package main

import (
	"errors"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"testing"
	"time"

	gioapp "gioui.org/app"
	"gioui.org/io/key"
	"gioui.org/op"
	"gioui.org/unit"
)

func TestRun_DrawsFramesUntilTheWindowCloses(t *testing.T) {
	isolateSettings(t)
	framesDrawn := 0
	closed := errors.New("closed")
	window := &fakeWindow{events: append(viewEvents,
		gioapp.FrameEvent{
			Now: time.Unix(1_800_000_000, 0), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Size: image.Pt(960, 740),
			Frame: func(*op.Ops) { framesDrawn++ },
		},
		gioapp.DestroyEvent{Err: closed},
	)}
	app := newApp(window, (&fakeSystem{}).system())
	if err := app.run(); !errors.Is(err, closed) {
		t.Errorf("run returned %v, want the window's error", err)
	}
	if framesDrawn != 1 {
		t.Errorf("drew %d frames, want 1", framesDrawn)
	}
	if app.ctx.Err() == nil {
		t.Error("closing the window didn't cancel the work in progress")
	}
}

func TestPost_RunsTasksBeforeTheNextFrame(t *testing.T) {
	harness := newHarness(t)
	before := harness.window.invalidations.Load()
	ran := false
	harness.app.post(func() { ran = true })
	if ran {
		t.Fatal("the task ran before the next frame")
	}
	if harness.window.invalidations.Load() == before {
		t.Error("posting didn't ask for a frame")
	}
	harness.frame()
	if !ran {
		t.Error("the task didn't run in the next frame")
	}
}

func TestSidebar_SwitchesScreens(t *testing.T) {
	harness := newHarness(t)
	for _, target := range []screen{createScreen, checkScreen, sealScreen, openScreen} {
		harness.showScreen(target)
	}
}

func TestScreenContent_PanicsForAnUnknownScreen(t *testing.T) {
	harness := newHarness(t)
	harness.app.screen = screenCount
	defer func() {
		if recover() == nil {
			t.Error("an unknown screen didn't panic")
		}
	}()
	harness.frame()
}

func TestThemeToggle_SwitchesAndRemembersTheTheme(t *testing.T) {
	harness := newHarness(t)
	if harness.app.isDark {
		t.Fatal("the theme starts dark without a saved choice")
	}
	harness.click(&harness.app.darkThemeButton)
	if !harness.app.isDark || harness.app.colors != darkPalette || !loadDarkTheme() {
		t.Error("the dark theme wasn't chosen and saved")
	}
	if restarted := newApp(&fakeWindow{}, (&fakeSystem{}).system()); !restarted.isDark {
		t.Error("the dark theme wasn't restored at the next start")
	}
	harness.click(&harness.app.lightThemeButton)
	if harness.app.isDark || harness.app.colors != lightPalette || loadDarkTheme() {
		t.Error("the light theme wasn't chosen and saved")
	}
}

func TestAboutDialog_Closes(t *testing.T) {
	tests := []struct {
		name  string
		close func(harness *uiHarness)
	}{
		{"close button", func(harness *uiHarness) { harness.click(&harness.app.aboutCloseButton) }},
		{"close icon", func(harness *uiHarness) { harness.click(&harness.app.aboutCloseIcon) }},
		{"escape", func(harness *uiHarness) { harness.press(key.NameEscape, 0) }},
		{"click outside", func(harness *uiHarness) { harness.clickAt(image.Pt(5, 5)) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			harness := newHarness(t)
			harness.click(&harness.app.aboutButton)
			if !harness.app.isAboutOpen {
				t.Fatal("About didn't open")
			}
			test.close(harness)
			if harness.app.isAboutOpen {
				t.Error("About didn't close")
			}
		})
	}
}

func TestAboutDialog_StaysOpenWhenClickedInside(t *testing.T) {
	harness := newHarness(t)
	harness.click(&harness.app.aboutButton)
	harness.clickAt(image.Pt(harness.size.X/2, harness.size.Y/2))
	if !harness.app.isAboutOpen {
		t.Error("clicking the dialog closed it")
	}
	harness.press(key.NameEscape, 0)
	harness.press(key.NameEscape, 0) // a second Escape has nothing to close
	if harness.app.isAboutOpen {
		t.Error("Escape didn't close About")
	}
}

func TestDropHandler_HighlightsAndRoutesDroppedFiles(t *testing.T) {
	harness := newHarness(t)
	handler := harness.app.dropHandler()
	handler.Enter()
	harness.frame()
	if !harness.app.isDragging {
		t.Error("dragging over the window didn't highlight the drop zones")
	}
	handler.Leave()
	harness.frame()
	if harness.app.isDragging {
		t.Error("leaving the window didn't clear the highlight")
	}

	plain := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(plain, []byte("notes"), 0o600); err != nil {
		t.Fatal(err)
	}
	harness.showScreen(sealScreen)
	handler.Enter()
	handler.Drop([]string{plain})
	harness.frame()
	if harness.app.isDragging || harness.app.seal.file != plain {
		t.Errorf("a drop on the seal screen chose %q, want %q", harness.app.seal.file, plain)
	}

	sealed := filepath.Join(t.TempDir(), "notes.txt.age")
	if err := os.WriteFile(sealed, []byte("sealed"), 0o600); err != nil {
		t.Fatal(err)
	}
	harness.showScreen(checkScreen)
	handler.Drop([]string{sealed})
	harness.frame()
	if harness.app.screen != openScreen || len(harness.app.open.files) != 1 {
		t.Errorf("a drop elsewhere went to screen %d with %d files, want the open screen with 1",
			harness.app.screen, len(harness.app.open.files))
	}
}

func TestLayoutContent_ScrollsTallScreens(t *testing.T) {
	harness := newHarness(t)
	harness.size = image.Pt(720, 300)
	harness.showScreen(createScreen)
	harness.frame()
	position := harness.app.content.Position
	if position.Length <= harness.size.Y {
		t.Errorf("the create screen is %d px tall in a %d px window, want it to scroll",
			position.Length, harness.size.Y)
	}
}

// TestScreenStates_LayOutInBothThemes lays out every state TestScreens renders, in both themes, without a GPU. In the
// light theme it also moves the mouse and keyboard focus over each, so every hover and focus style is drawn; the dark
// theme takes the same paths with other colors.
func TestScreenStates_LayOutInBothThemes(t *testing.T) {
	fixture := newScreenFixture(t)
	for _, isDark := range []bool{false, true} {
		for _, state := range screenStates(t, fixture) {
			t.Run(fmt.Sprintf("%s dark %v", state.name, isDark), func(t *testing.T) {
				harness := newHarness(t)
				harness.app.setDark(isDark)
				state.setup(harness.app)
				harness.frame()
				if !isDark {
					harness.explore()
				}
				if harness.app.isDark != isDark {
					t.Error("laying out the screen changed the theme")
				}
			})
		}
	}
}
