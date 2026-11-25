package main

import (
	"image"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/transfer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/vita-workflow/vita-decrypt/internal/desktop"
)

// fakeWindow stands in for a Gio window: it hands out scripted events and counts redraw requests.
type fakeWindow struct {
	events        []event.Event
	invalidations atomic.Int64
}

func (window *fakeWindow) Event() event.Event {
	next := window.events[0]
	window.events = window.events[1:]
	return next
}

func (window *fakeWindow) Invalidate() { window.invalidations.Add(1) }

func (*fakeWindow) Run(task func()) { task() }

// fakeSystem records what the app asks of the operating system and answers with canned results. Background work runs
// at once, on the calling goroutine.
type fakeSystem struct {
	dialogs  []desktop.Dialog
	revealed []string

	chosenFile   string
	chosenFiles  []string
	chosenFolder string
	dialogError  error
}

func (fake *fakeSystem) system() system {
	return system{
		openFile: func(dialog desktop.Dialog) (string, error) {
			fake.dialogs = append(fake.dialogs, dialog)
			return fake.chosenFile, fake.dialogError
		},
		openFiles: func(dialog desktop.Dialog) ([]string, error) {
			fake.dialogs = append(fake.dialogs, dialog)
			return fake.chosenFiles, fake.dialogError
		},
		chooseFolder: func(dialog desktop.Dialog) (string, error) {
			fake.dialogs = append(fake.dialogs, dialog)
			return fake.chosenFolder, fake.dialogError
		},
		reveal: func(path string) error {
			fake.revealed = append(fake.revealed, path)
			return nil
		},
		runInBackground: func(task func()) { task() },
	}
}

// uiHarness drives an App frame by frame, headlessly, through a real input router.
type uiHarness struct {
	t      *testing.T
	app    *App
	window *fakeWindow
	system *fakeSystem
	router input.Router
	ops    op.Ops
	now    time.Time
	size   image.Point
}

// isolateSettings points the home and settings folders, on every system, at an empty temporary folder.
func isolateSettings(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	for _, variable := range []string{"HOME", "XDG_CONFIG_HOME", "AppData"} {
		t.Setenv(variable, home)
	}
}

func newHarness(t *testing.T) *uiHarness {
	t.Helper()
	isolateSettings(t)
	fake := &fakeSystem{}
	window := &fakeWindow{}
	harness := &uiHarness{
		t: t, window: window, system: fake,
		app:  newApp(window, fake.system()),
		now:  time.Unix(1_800_000_000, 0),
		size: image.Pt(960, 740),
	}
	harness.frame()
	return harness
}

func (harness *uiHarness) context() layout.Context {
	harness.ops.Reset()
	return layout.Context{
		Ops:         &harness.ops,
		Now:         harness.now,
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(harness.size),
		Source:      harness.router.Source(),
	}
}

// frame lays out one frame and hands it to the router, so the next frame sees the events queued in between.
func (harness *uiHarness) frame() {
	harness.app.frame(harness.context())
	harness.router.Frame(&harness.ops)
}

// click clicks clickable, then lays out a frame for the click to be seen, and another for its results.
func (harness *uiHarness) click(clickable *widget.Clickable) {
	harness.t.Helper()
	clickable.Click()
	harness.frame()
	harness.frame()
}

// press presses a key, with modifiers.
func (harness *uiHarness) press(name key.Name, modifiers key.Modifiers) {
	harness.router.Queue(key.Event{Name: name, Modifiers: modifiers, State: key.Press})
	harness.frame()
	harness.frame()
}

// clickAt presses and releases the primary mouse button at position.
func (harness *uiHarness) clickAt(position image.Point) {
	point := f32.Pt(float32(position.X), float32(position.Y))
	harness.router.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: point},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: point},
	)
	harness.frame()
	harness.frame()
}

// explore hovers the mouse over the window on a grid, then moves the keyboard focus through the focusable widgets, so
// every hover and focus style on the screen is drawn.
func (harness *uiHarness) explore() {
	const columnStep, rowStep, focusSteps = 64, 32, 30
	for row := rowStep / 2; row < harness.size.Y; row += rowStep {
		for column := columnStep / 2; column < harness.size.X; column += columnStep {
			harness.router.Queue(pointer.Event{
				Kind: pointer.Move, Source: pointer.Mouse, Position: f32.Pt(float32(column), float32(row)),
			})
			harness.frame()
		}
	}
	for range focusSteps {
		harness.router.MoveFocus(key.FocusForward)
		harness.frame()
	}
	harness.router.Queue(pointer.Event{Kind: pointer.Leave, Source: pointer.Mouse})
	harness.frame()
}

// paste answers the app's clipboard request with content.
func (harness *uiHarness) paste(content string) {
	harness.t.Helper()
	if !harness.router.ClipboardRequested() {
		harness.t.Fatal("the app didn't ask for the clipboard")
	}
	harness.router.Queue(transfer.DataEvent{
		Type: textMIME,
		Open: func() io.ReadCloser { return io.NopCloser(strings.NewReader(content)) },
	})
	harness.frame()
	harness.frame()
}

// copied is what the app last put on the clipboard.
func (harness *uiHarness) copied() string {
	harness.t.Helper()
	mime, content, ok := harness.router.WriteClipboard()
	if !ok || mime != textMIME {
		harness.t.Fatalf("nothing was copied (type %q)", mime)
	}
	return string(content)
}

// showScreen switches to target with its sidebar button.
func (harness *uiHarness) showScreen(target screen) {
	harness.click(&harness.app.navigationButtons[target])
	if harness.app.screen != target {
		harness.t.Fatalf("screen is %d, want %d", harness.app.screen, target)
	}
}
