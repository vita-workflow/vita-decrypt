package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"filippo.io/age"

	"gioui.org/io/key"

	"github.com/vita-workflow/vita-decrypt/internal/desktop"
	"github.com/vita-workflow/vita-decrypt/internal/vita"
)

// sealedFixture is a key pair and files sealed to it, whose originals have been removed so they can be opened again.
type sealedFixture struct {
	folder   string
	keyPair  vita.KeyPair
	sealed   []string
	contents map[string]string // each sealed file's original content
}

func newSealedFixture(t *testing.T, names ...string) sealedFixture {
	t.Helper()
	folder := t.TempDir()
	keyPair, err := vita.CreateKeyPair(folder, "test-pair", true)
	if err != nil {
		t.Fatal(err)
	}
	fixture := sealedFixture{folder: folder, keyPair: keyPair, contents: map[string]string{}}
	for _, name := range names {
		plain := filepath.Join(folder, name)
		content := "the content of " + name
		if err := os.WriteFile(plain, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		sealed, err := vita.Seal(context.Background(), keyPair.PublicKey, plain, func(int64) {})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(plain); err != nil {
			t.Fatal(err)
		}
		fixture.sealed = append(fixture.sealed, sealed)
		fixture.contents[sealed] = content
	}
	return fixture
}

func writeFile(t *testing.T, path, content string) string {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestOpenScreen_OpensChosenFiles(t *testing.T) {
	fixture := newSealedFixture(t, "first.txt", "second.txt")
	harness := newHarness(t)
	state := &harness.app.open

	harness.system.chosenFile = fixture.keyPair.PrivateKeyPath
	harness.click(&state.chooseKeyButton)
	if state.privateKey == nil || state.keyPath != fixture.keyPair.PrivateKeyPath {
		t.Fatalf("the chosen key wasn't loaded: %q", state.keyPath)
	}
	harness.system.chosenFiles = fixture.sealed
	harness.click(&state.chooseFilesButton)
	if len(state.files) != 2 {
		t.Fatalf("%d files listed, want 2", len(state.files))
	}
	wantDialogs := []string{keyDialog.Title, sealedDialog.Title}
	for i, dialog := range harness.system.dialogs {
		if dialog.Title != wantDialogs[i] {
			t.Errorf("dialog %d is %q, want %q", i, dialog.Title, wantDialogs[i])
		}
	}

	harness.click(&state.startButton)
	harness.frame()
	if state.isWorking || !state.isDone {
		t.Fatalf("opening didn't finish: working %v, done %v", state.isWorking, state.isDone)
	}
	checkOpened(t, state.files, fixture.contents)
	harness.explore()

	harness.click(&state.files[1].showButton)
	if len(harness.system.revealed) != 1 || harness.system.revealed[0] != state.files[1].result.outputPath {
		t.Errorf("revealed %q, want the second opened file", harness.system.revealed)
	}
	harness.click(&state.openMoreButton)
	if len(state.files) != 0 || state.isDone || state.privateKey == nil {
		t.Error("Open more files didn't clear the list and keep the key")
	}
}

// checkOpened checks that every file was opened, to its original content.
func checkOpened(t *testing.T, files []*sealedFile, contents map[string]string) {
	t.Helper()
	for _, file := range files {
		if file.result == nil || file.result.kind != successMessage {
			t.Fatalf("%s: %+v", file.path, file.result)
		}
		content, err := os.ReadFile(file.result.outputPath)
		if err != nil || string(content) != contents[file.path] {
			t.Errorf("%s opened as %q (%v), want %q", file.path, content, err, contents[file.path])
		}
	}
}

func TestOpenScreen_ReportsFilesThatFail(t *testing.T) {
	fixture := newSealedFixture(t, "exists.txt", "fine.txt")
	writeFile(t, filepath.Join(fixture.folder, "exists.txt"), "already here")
	harness := newHarness(t)
	state := &harness.app.open
	harness.app.addPaths(append([]string{fixture.keyPair.PrivateKeyPath}, fixture.sealed...))
	harness.frame()
	harness.click(&state.startButton)
	harness.frame()
	if got := state.files[0].result; got == nil || got.kind != warningMessage {
		t.Errorf("the file whose output exists got %+v, want a warning", got)
	}
	if got := summarize(state.files); got.title != "1 of 2 files opened." {
		t.Errorf("summary %q", got.title)
	}
}

func TestOpenScreen_RemovesAndChangesBeforeOpening(t *testing.T) {
	fixture := newSealedFixture(t, "first.txt", "second.txt")
	harness := newHarness(t)
	state := &harness.app.open
	harness.app.addPaths(append([]string{fixture.keyPair.PrivateKeyPath}, fixture.sealed...))
	harness.frame()
	harness.explore()

	harness.click(&state.files[0].removeButton)
	if len(state.files) != 1 || state.files[0].path != fixture.sealed[1] {
		t.Errorf("removing the first file left %d files", len(state.files))
	}
	harness.system.chosenFiles = fixture.sealed[:1]
	harness.click(&state.addFilesButton)
	if len(state.files) != 2 {
		t.Errorf("adding a file left %d files, want 2", len(state.files))
	}
	harness.system.chosenFile = "" // cancelled
	harness.click(&state.changeKeyButton)
	if state.keyPath != fixture.keyPair.PrivateKeyPath {
		t.Error("cancelling the key dialog changed the key")
	}
}

func TestOpenScreen_ShowsWhyADialogDidntOpen(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"no file chooser", desktop.ErrUnavailable, "This computer has no file chooser."},
		{"other error", errors.New("portal timed out"), "The file chooser didn’t open: portal timed out"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			harness := newHarness(t)
			harness.system.dialogError = test.err
			harness.click(&harness.app.open.chooseFilesButton)
			if message := harness.app.open.filesMessage; !strings.HasPrefix(message, test.want) {
				t.Errorf("message %q, want it to start %q", message, test.want)
			}
		})
	}
}

func TestOpenScreen_PastesFiles(t *testing.T) {
	fixture := newSealedFixture(t, "notes.txt")
	harness := newHarness(t)
	harness.press("V", key.ModShortcut)
	harness.paste(fixture.keyPair.PrivateKeyPath + "\n" + fixture.sealed[0])
	state := &harness.app.open
	if state.privateKey == nil || len(state.files) != 1 {
		t.Fatalf("pasting set key %v and %d files, want both", state.privateKey != nil, len(state.files))
	}

	harness.press("V", key.ModShortcut)
	harness.paste("just some words")
	if !strings.HasPrefix(state.filesMessage, "The clipboard doesn’t hold any files.") {
		t.Errorf("pasting words said %q", state.filesMessage)
	}
}

func TestAddPaths(t *testing.T) {
	fixture := newSealedFixture(t, "notes.txt")
	folder := t.TempDir()
	notKey := writeFile(t, filepath.Join(folder, "notes.txt"), "not a key")
	tests := []struct {
		name        string
		paths       []string
		wantFiles   int
		wantKey     bool
		wantKeyErr  bool
		wantMessage string
	}{
		{"key and file", []string{fixture.keyPair.PrivateKeyPath, fixture.sealed[0]}, 1, true, false, ""},
		{
			// A file of its own, because on Windows and macOS notes.txt.AGE would find notes.txt.age.
			"upper case extension",
			[]string{writeFile(t, filepath.Join(folder, "report.AGE"), "sealed")},
			0, false, false,
			"“report.AGE” isn’t a sealed file. Sealed file names end in .age.",
		},
		{"not a key", []string{notKey}, 0, false, true, ""},
		{
			"two other files",
			[]string{fixture.keyPair.PrivateKeyPath, notKey},
			0, true, false,
			"“notes.txt” isn’t a sealed file. Sealed file names end in .age.",
		},
		{"missing file", []string{filepath.Join(folder, "gone.age")}, 0, false, false, "Can’t read “gone.age”."},
		{
			"folder",
			[]string{mkdir(t, filepath.Join(folder, "photos.age"))},
			0, false, false,
			"Can’t read “photos.age”.",
		},
		{"duplicate", []string{fixture.sealed[0], fixture.sealed[0]}, 1, false, false, ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			harness := newHarness(t)
			harness.app.screen = sealScreen
			harness.app.addPaths(test.paths)
			state := &harness.app.open
			if harness.app.screen != openScreen {
				t.Error("adding files didn't show the open screen")
			}
			if len(state.files) != test.wantFiles || (state.privateKey != nil) != test.wantKey ||
				state.hasKeyError != test.wantKeyErr || state.filesMessage != test.wantMessage {
				t.Errorf("got %d files, key %v, key error %v, message %q", len(state.files), state.privateKey != nil,
					state.hasKeyError, state.filesMessage)
			}
		})
	}
}

func mkdir(t *testing.T, path string) string {
	t.Helper()
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAddSealed_RejectsNamesWithoutTheExtension(t *testing.T) {
	harness := newHarness(t)
	plain := writeFile(t, filepath.Join(t.TempDir(), "notes.txt"), "plain")
	harness.app.addSealed([]string{plain})
	if want := fmt.Sprintf(notSealedFormat, "notes.txt"); harness.app.open.filesMessage != want {
		t.Errorf("message %q, want %q", harness.app.open.filesMessage, want)
	}
}

func TestOpenScreen_IgnoresNewFilesWhileWorkingAndStartsOverWhenDone(t *testing.T) {
	fixture := newSealedFixture(t, "first.txt", "second.txt")
	harness := newHarness(t)
	app := harness.app
	app.addPaths([]string{fixture.keyPair.PrivateKeyPath, fixture.sealed[0]})

	app.open.isWorking = true
	app.addPaths(fixture.sealed[1:])
	app.fromDialog(app.addSealed)(fixture.sealed[1:])
	app.failOpenDialog("ignored")
	app.startOpening()
	if len(app.open.files) != 1 || app.open.filesMessage != "" {
		t.Errorf("while working: %d files, message %q", len(app.open.files), app.open.filesMessage)
	}

	app.open.isWorking, app.open.isDone = false, true
	app.fromDialog(app.addSealed)(fixture.sealed[1:])
	if len(app.open.files) != 1 || app.open.files[0].path != fixture.sealed[1] || app.open.isDone {
		t.Error("a dialog after opening didn't start a new list")
	}
	app.open.isDone = true
	app.addPaths(fixture.sealed[:1])
	if len(app.open.files) != 1 || app.open.files[0].path != fixture.sealed[0] || app.open.isDone {
		t.Error("a drop after opening didn't start a new list")
	}
}

func TestStartOpening_StopsWhenTheWindowCloses(t *testing.T) {
	fixture := newSealedFixture(t, "notes.txt")
	harness := newHarness(t)
	harness.app.addPaths([]string{fixture.keyPair.PrivateKeyPath, fixture.sealed[0]})
	harness.app.cancel()
	harness.app.startOpening()
	harness.frame()
	if harness.app.open.files[0].result != nil {
		t.Error("a file was opened after the window closed")
	}
}

func TestProgressReporter_StoresProgressAndRedrawsAtMostEveryInterval(t *testing.T) {
	harness := newHarness(t)
	var counter atomic.Int64
	report := harness.app.progressReporter(&counter)
	before := harness.window.invalidations.Load()
	report(10)
	report(20)
	if counter.Load() != 20 {
		t.Errorf("stored %d, want 20", counter.Load())
	}
	if redraws := harness.window.invalidations.Load() - before; redraws != 1 {
		t.Errorf("redrew %d times for two quick reports, want 1", redraws)
	}
}

func TestOutcome(t *testing.T) {
	input := filepath.Join(t.TempDir(), "report.pdf.age")
	tests := []struct {
		name     string
		err      error
		wantKind messageKind
		wantText string
	}{
		{"opened", nil, successMessage, "Saved as report.pdf"},
		{"exists", fmt.Errorf("x: %w", fs.ErrExist), warningMessage, "report.pdf already exists in this folder."},
		{
			"wrong key", fmt.Errorf("x: %w", &age.NoIdentityMatchError{}), dangerMessage,
			"This key can’t open this file.",
		},
		{"damaged", fmt.Errorf("x: %w", vita.ErrDamaged), dangerMessage, "This file is damaged"},
		{"not sealed", fmt.Errorf("x: %w", vita.ErrCannotDecrypt), dangerMessage, "This isn’t a file Vita sealed"},
		{"bad name", fmt.Errorf("x: %w", vita.ErrNotSealedName), dangerMessage, "This isn’t a sealed file."},
		{"other", errors.New("disk full"), dangerMessage, "This file couldn’t be opened: disk full"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := outcome(input, strings.TrimSuffix(input, ".age"), test.err)
			if result.kind != test.wantKind || !strings.HasPrefix(result.text, test.wantText) {
				t.Errorf("got %v %q, want %v %q…", result.kind, result.text, test.wantKind, test.wantText)
			}
		})
	}
}

func TestSummarize(t *testing.T) {
	opened := &fileResult{kind: successMessage}
	failed := &fileResult{kind: dangerMessage}
	files := func(results ...*fileResult) []*sealedFile {
		var list []*sealedFile
		for _, result := range results {
			list = append(list, &sealedFile{result: result})
		}
		return list
	}
	tests := []struct {
		name      string
		files     []*sealedFile
		wantKind  messageKind
		wantTitle string
	}{
		{"one failed", files(failed), dangerMessage, "The file wasn’t opened."},
		{"all failed", files(failed, nil), dangerMessage, "No files were opened."},
		{"one opened", files(opened), successMessage, "Your file was opened."},
		{"all opened", files(opened, opened, opened), successMessage, "All 3 files opened."},
		{"some opened", files(opened, failed), successMessage, "1 of 2 files opened."},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := summarize(test.files); got.kind != test.wantKind || got.title != test.wantTitle {
				t.Errorf("got %v %q, want %v %q", got.kind, got.title, test.wantKind, test.wantTitle)
			}
		})
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		sizeBytes int64
		want      string
	}{
		{0, "0 bytes"},
		{1, "1 byte"},
		{999, "999 bytes"},
		{1000, "1 KB"},
		{1500, "2 KB"},
		{182_000, "182 KB"},
		{999_499, "999 KB"},
		{999_500, "1.0 MB"},
		{2_400_000, "2.4 MB"},
		{54_300_000, "54.3 MB"},
		{5_000_000_000_000_000, "5000 TB"},
	}
	for _, test := range tests {
		if got := formatSize(test.sizeBytes); got != test.want {
			t.Errorf("formatSize(%d) = %q, want %q", test.sizeBytes, got, test.want)
		}
	}
}
