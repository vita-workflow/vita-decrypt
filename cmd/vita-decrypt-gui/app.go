package main

import (
	"context"
	"fmt"
	"image"
	"sync"

	gioapp "gioui.org/app"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/text"
	"gioui.org/widget"

	"github.com/vita-workflow/vita-decrypt/internal/desktop"
)

// screen is one of the pages the sidebar switches between.
type screen int

const (
	openScreen screen = iota
	createScreen
	checkScreen
	sealScreen
)

// screenCount is how many screens there are, one per sidebar item.
const screenCount = 4

// window is what the app needs from a Gio window, so tests can drive it without one.
type window interface {
	Event() event.Event
	Invalidate()
	Run(task func())
}

// App is the whole GUI. Only the event loop goroutine touches it; other goroutines hand it work through post.
type App struct {
	window window
	shaper *text.Shaper
	colors palette
	system system

	isDark bool
	screen screen

	navigationButtons                 [screenCount]widget.Clickable
	lightThemeButton, darkThemeButton widget.Clickable
	aboutButton                       widget.Clickable
	content                           layout.List
	isAboutOpen                       bool
	aboutCloseButton, aboutCloseIcon  widget.Clickable
	scrimTag, dialogTag               int // tags for clicks on the about scrim and dialog

	isDragging bool

	open   openState
	create createState
	check  checkState
	seal   sealState

	pendingMutex sync.Mutex
	pendingTasks []func()

	// ctx is cancelled when the window closes; workers counts the goroutines that must finish cleaning up first.
	ctx     context.Context
	cancel  context.CancelFunc
	workers sync.WaitGroup
}

func newApp(window window, system system) *App {
	app := &App{window: window, shaper: newShaper(), system: system}
	app.ctx, app.cancel = context.WithCancel(context.Background())
	app.content.Axis = layout.Vertical
	app.setDark(loadDarkTheme())
	app.create.init()
	app.check.init()
	return app
}

// run handles the window's events until it closes.
func (app *App) run() error {
	var ops op.Ops
	for {
		switch windowEvent := app.window.Event().(type) {
		case gioapp.DestroyEvent:
			// Stop any decryption and let it remove its temporary file before the process exits.
			app.cancel()
			app.workers.Wait()
			return windowEvent.Err
		case gioapp.ViewEvent:
			app.attachView(windowEvent)
		case gioapp.FrameEvent:
			gtx := gioapp.NewContext(&ops, windowEvent)
			app.frame(gtx)
			windowEvent.Frame(gtx.Ops)
		}
	}
}

// post runs task on the event loop before the next frame. Any goroutine may call it.
func (app *App) post(task func()) {
	app.pendingMutex.Lock()
	app.pendingTasks = append(app.pendingTasks, task)
	app.pendingMutex.Unlock()
	app.window.Invalidate()
}

func (app *App) runPending() {
	app.pendingMutex.Lock()
	tasks := app.pendingTasks
	app.pendingTasks = nil
	app.pendingMutex.Unlock()
	for _, task := range tasks {
		task()
	}
}

// dropHandler routes files dropped on the window; its functions run on the window's thread.
func (app *App) dropHandler() desktop.DropHandler {
	return desktop.DropHandler{
		Enter: func() { app.post(func() { app.isDragging = true }) },
		Leave: func() { app.post(func() { app.isDragging = false }) },
		Drop: func(paths []string) {
			app.post(func() {
				app.isDragging = false
				if app.screen == sealScreen {
					app.setSealFile(paths[0])
				} else {
					app.addPaths(paths)
				}
			})
		},
	}
}

func (app *App) setDark(isDark bool) {
	app.isDark = isDark
	if isDark {
		app.colors = darkPalette
	} else {
		app.colors = lightPalette
	}
}

// frame lays out one frame of the whole window.
func (app *App) frame(gtx layout.Context) {
	app.runPending()
	app.handleKeys(gtx)
	fillRect(gtx, image.Rectangle{Max: gtx.Constraints.Max}, app.colors.canvas)

	underneath := gtx
	if app.isAboutOpen {
		underneath = gtx.Disabled()
	}
	layout.Flex{}.Layout(underneath,
		layout.Rigid(app.sidebar),
		layout.Flexed(1, app.layoutContent),
	)
	if app.isAboutOpen {
		app.aboutDialog(gtx)
	}
}

// handleKeys handles Escape for the about dialog and Ctrl/Cmd+V for pasted files on the open screen.
func (app *App) handleKeys(gtx layout.Context) {
	area := clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops)
	event.Op(gtx.Ops, app)
	area.Pop()
	filters := []event.Filter{key.Filter{Name: key.NameEscape}}
	// Asking for Ctrl+V here takes it from every text field, because this runs before they do, so ask only on the
	// open screen, which has none.
	if app.screen == openScreen && !app.isAboutOpen {
		filters = append(filters, key.Filter{Name: "V", Required: key.ModShortcut})
	}
	for {
		received, ok := gtx.Event(filters...)
		if !ok {
			break
		}
		press, ok := received.(key.Event)
		if !ok || press.State != key.Press {
			continue
		}
		switch {
		case press.Name == key.NameEscape && app.isAboutOpen:
			app.isAboutOpen = false
		case press.Name == "V":
			app.requestPaste(gtx)
		}
	}
	app.receivePaste(gtx)
}

// layoutContent lays out the current screen in a scrolling column, at most 640 dp wide and centered.
func (app *App) layoutContent(gtx layout.Context) layout.Dimensions {
	size := gtx.Constraints.Max
	gtx.Constraints = layout.Exact(size)
	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
	dims := app.content.Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
		return layout.UniformInset(32).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			width := min(gtx.Constraints.Max.X, pixels(gtx, 640))
			margin := (gtx.Constraints.Max.X - width) / 2
			gtx.Constraints.Min.X, gtx.Constraints.Max.X = width, width
			defer op.Offset(image.Pt(margin, 0)).Push(gtx.Ops).Pop()
			screenDims := app.screenContent(gtx)
			screenDims.Size.X += 2 * margin
			return screenDims
		})
	})
	app.scrollThumb(gtx, size)
	return dims
}

func (app *App) screenContent(gtx layout.Context) layout.Dimensions {
	switch app.screen {
	case openScreen:
		return app.layoutOpenScreen(gtx)
	case createScreen:
		return app.layoutCreateScreen(gtx)
	case checkScreen:
		return app.layoutCheckScreen(gtx)
	case sealScreen:
		return app.layoutSealScreen(gtx)
	}
	panic(fmt.Sprintf("unknown screen %d", app.screen))
}

// scrollThumb shows where the content is scrolled when it's taller than the window.
func (app *App) scrollThumb(gtx layout.Context, view image.Point) {
	position := app.content.Position
	total := position.Length
	if total <= view.Y || total == 0 {
		return
	}
	height := max(view.Y*view.Y/total, pixels(gtx, 32))
	top := position.Offset * (view.Y - height) / max(total-view.Y, 1)
	width := pixels(gtx, 6)
	defer op.Offset(image.Pt(view.X-width-pixels(gtx, 3), top)).Push(gtx.Ops).Pop()
	box{background: app.colors.scrollThumb, radius: 3}.paint(gtx, image.Pt(width, height))
}
