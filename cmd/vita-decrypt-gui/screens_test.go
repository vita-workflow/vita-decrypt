package main

import (
	"fmt"
	"image"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gioui.org/gpu/headless"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"filippo.io/age"

	"github.com/vita-workflow/vita-decrypt/internal/vita"
)

// screenFixture is the key pair and files the rendered screens show.
type screenFixture struct {
	folder  string
	keyPair vita.KeyPair
	sealed  []string
	plain   string
}

type screenState struct {
	name  string
	setup func(app *App)
}

// TestScreens renders every screen and state in both themes to PNG files in $VITA_GUI_SHOTS, to compare against the
// design by eye. It needs a GPU (or a software OpenGL), so it only runs when that variable is set.
func TestScreens(t *testing.T) {
	outputFolder := os.Getenv("VITA_GUI_SHOTS")
	if outputFolder == "" {
		t.Skip("set VITA_GUI_SHOTS to a folder to render the screens")
	}
	if err := os.MkdirAll(outputFolder, 0o750); err != nil { //nolint:gosec // the tester chose this folder
		t.Fatal(err)
	}
	isolateSettings(t)
	scale := float32(1)
	if os.Getenv("VITA_GUI_SCALE") == "2" {
		scale = 2
	}
	size := image.Pt(int(960*scale), int(740*scale))
	window, err := headless.NewWindow(size.X, size.Y)
	if err != nil {
		t.Skipf("no headless GPU: %v", err)
	}
	defer window.Release()
	output, err := os.OpenRoot(outputFolder)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = output.Close() }() // closing a folder handle has nothing to report

	states := screenStates(t, newScreenFixture(t))
	for _, isDark := range []bool{false, true} {
		mode := lightTheme
		if isDark {
			mode = darkTheme
		}
		for _, state := range states {
			app := newApp(&fakeWindow{}, (&fakeSystem{}).system())
			app.setDark(isDark)
			state.setup(app)
			screenshot := renderScreen(t, window, app, size, scale)
			if err := writePNG(output, state.name+"-"+mode+".png", screenshot); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func newScreenFixture(t *testing.T) screenFixture {
	t.Helper()
	folder := t.TempDir()
	keyPair, err := vita.CreateKeyPair(folder, "family-vault", false)
	if err != nil {
		t.Fatal(err)
	}
	fixture := screenFixture{folder: folder, keyPair: keyPair}
	files := []struct {
		name      string
		sizeBytes int64
	}{
		{"tax-return-2025.pdf.age", 2_400_000},
		{"passport-scan.jpg.age", 182_000},
		{"family-photos.zip.age", 54_300_000},
	}
	for _, file := range files {
		path := filepath.Join(folder, file.name)
		writeZeros(t, path, file.sizeBytes)
		fixture.sealed = append(fixture.sealed, path)
	}
	fixture.plain = filepath.Join(t.TempDir(), "tax-return-2025.pdf")
	writeZeros(t, fixture.plain, 180_000)
	return fixture
}

// writeZeros writes a file of sizeBytes zeros, sparsely where the file system allows.
func writeZeros(t *testing.T, path string, sizeBytes int64) {
	t.Helper()
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(path, sizeBytes); err != nil {
		t.Fatal(err)
	}
}

// renderScreen draws two frames of app, since the first settles scroll lengths and recorded sizes, and screenshots
// the second.
func renderScreen(t *testing.T, window *headless.Window, app *App, size image.Point, scale float32) *image.RGBA {
	t.Helper()
	var ops op.Ops
	for range 2 {
		ops.Reset()
		app.frame(layout.Context{
			Ops:         &ops,
			Now:         time.Unix(1_800_000_000, 0),
			Metric:      unit.Metric{PxPerDp: scale, PxPerSp: scale},
			Constraints: layout.Exact(size),
		})
		if err := window.Frame(&ops); err != nil {
			t.Fatal(err)
		}
	}
	screenshot := image.NewRGBA(image.Rectangle{Max: size})
	if err := window.Screenshot(screenshot); err != nil {
		t.Fatal(err)
	}
	return screenshot
}

func screenStates(t *testing.T, fixture screenFixture) []screenState {
	t.Helper()
	states := openScreenStates(fixture)
	states = append(states, createScreenStates(t, fixture)...)
	states = append(states, checkScreenStates(fixture)...)
	states = append(states, sealScreenStates(fixture)...)
	return append(states, screenState{"about", func(app *App) { app.isAboutOpen = true }})
}

func openScreenStates(fixture screenFixture) []screenState {
	files := fixture.sealed
	keyPath := fixture.keyPair.PrivateKeyPath
	opened := func(index int) *fileResult {
		return outcome(files[index], files[index][:len(files[index])-len(sealedExtension)], nil)
	}
	return []screenState{
		{"open-empty", func(*App) {}},
		{"open-drag", func(app *App) { app.isDragging = true }},
		{"open-key-error", func(app *App) { app.open.hasKeyError = true }},
		{"open-ready", func(app *App) { app.loadKey(keyPath); app.addSealed(files) }},
		{"open-working", func(app *App) {
			app.loadKey(keyPath)
			app.addSealed(files)
			app.open.isWorking = true
			app.open.files[0].result = opened(0)
			app.open.files[1].bytesRead.Store(120_000)
		}},
		{"open-done", func(app *App) {
			app.loadKey(keyPath)
			app.addSealed(files)
			app.open.isDone = true
			app.open.files[0].result = opened(0)
			app.open.files[1].result = outcome(files[1], "", fmt.Errorf("x: %w", &age.NoIdentityMatchError{}))
			app.open.files[2].result = outcome(files[2], "", fmt.Errorf("x: %w", fs.ErrExist))
		}},
		{"open-none", func(app *App) {
			app.loadKey(keyPath)
			app.addSealed(files[:1])
			app.open.isDone = true
			app.open.files[0].result = outcome(files[0], "", fmt.Errorf("x: %w", vita.ErrDamaged))
		}},
	}
}

func createScreenStates(t *testing.T, fixture screenFixture) []screenState {
	t.Helper()
	return []screenState{
		{"create-form", func(app *App) { app.screen = createScreen; app.create.nameEditor.SetText("family-vault") }},
		{"create-invalid", func(app *App) {
			app.screen = createScreen
			app.create.nameEditor.SetText("-bad name")
			app.create.isClassic = true
		}},
		{"create-exists", func(app *App) {
			app.screen = createScreen
			app.create.folder = fixture.folder
			app.create.nameEditor.SetText("family-vault")
			app.createKeyPair()
		}},
		{"create-done", func(app *App) {
			app.screen = createScreen
			app.create.folder = t.TempDir()
			app.create.nameEditor.SetText("family-vault")
			app.createKeyPair()
			if app.create.created == nil {
				t.Fatal("key pair not created")
			}
		}},
	}
}

func checkScreenStates(fixture screenFixture) []screenState {
	publicKey := fixture.keyPair.PublicKey
	return []screenState{
		{"check-empty", func(app *App) { app.screen = checkScreen }},
		{"check-invalid", func(app *App) { app.screen = checkScreen; app.check.keyEditor.SetText("hello") }},
		{"check-match", func(app *App) {
			app.screen = checkScreen
			app.check.keyEditor.SetText(publicKey)
			app.check.expectedEditor.SetText(vita.Fingerprint(publicKey))
		}},
		{"check-mismatch", func(app *App) {
			app.screen = checkScreen
			app.check.keyEditor.SetText(publicKey)
			app.check.expectedEditor.SetText("D6B3 1A36 E35D 662F")
		}},
		{"check-private-file", func(app *App) {
			app.screen = checkScreen
			app.check.keyFileError.set(
				"That’s your private key file. Choose the public key file (its name ends in -public-key.txt).", "")
		}},
	}
}

func sealScreenStates(fixture screenFixture) []screenState {
	publicKey, plain := fixture.keyPair.PublicKey, fixture.plain
	ready := func(app *App) {
		app.screen = sealScreen
		app.seal.keyEditor.SetText(publicKey)
		app.setSealFile(plain)
	}
	return []screenState{
		{"seal-empty", func(app *App) { app.screen = sealScreen }},
		{"seal-invalid", func(app *App) { app.screen = sealScreen; app.seal.keyEditor.SetText("age1nope") }},
		{"seal-ready", ready},
		{"seal-working", func(app *App) {
			ready(app)
			app.seal.isWorking = true
			app.seal.bytesRead.Store(app.seal.sizeBytes / 3)
		}},
		{"seal-done", func(app *App) {
			ready(app)
			app.seal.resultFor = resultKey(publicKey, plain)
			app.seal.result = sealOutcome(plain, vita.SealedName(plain), nil)
		}},
		{"seal-exists", func(app *App) {
			ready(app)
			app.seal.resultFor = resultKey(publicKey, plain)
			app.seal.result = sealOutcome(plain, "", fmt.Errorf("x: %w", fs.ErrExist))
		}},
	}
}

// writePNG saves screenshot as name in output.
func writePNG(output *os.Root, name string, screenshot image.Image) error {
	file, err := output.Create(name)
	if err != nil {
		return err
	}
	if err := png.Encode(file, screenshot); err != nil {
		_ = file.Close() // the encoding error is the one worth reporting
		return err
	}
	return file.Close()
}
