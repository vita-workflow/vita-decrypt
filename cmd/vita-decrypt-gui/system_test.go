package main

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/vita-workflow/vita-decrypt/internal/desktop"
)

func TestNativeSystem_RunsBackgroundWorkOnAnotherGoroutine(t *testing.T) {
	native := nativeSystem()
	if native.openFile == nil || native.openFiles == nil || native.chooseFolder == nil || native.reveal == nil {
		t.Fatal("the native system is missing a function")
	}
	done := make(chan struct{})
	native.runInBackground(func() { close(done) })
	<-done
}

func TestChooseOne(t *testing.T) {
	failure := errors.New("portal timed out")
	tests := []struct {
		name      string
		chosen    string
		err       error
		wantPaths []string
	}{
		{"chosen", "/home/someone/notes.txt", nil, []string{"/home/someone/notes.txt"}},
		{"cancelled", "", nil, nil},
		{"failed", "", failure, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var asked desktop.Dialog
			pick := func(dialog desktop.Dialog) (string, error) { asked = dialog; return test.chosen, test.err }
			paths, err := chooseOne(pick, keyDialog)()
			if !slices.Equal(paths, test.wantPaths) || !errors.Is(err, test.err) {
				t.Errorf("got %q, %v", paths, err)
			}
			if asked.Title != keyDialog.Title {
				t.Errorf("asked with dialog %q", asked.Title)
			}
		})
	}
}

func TestRunDialog(t *testing.T) {
	tests := []struct {
		name        string
		paths       []string
		err         error
		wantPaths   []string
		wantMessage string
	}{
		{"chosen", []string{"a", "b"}, nil, []string{"a", "b"}, ""},
		{"cancelled", nil, nil, nil, ""},
		{"failed", nil, desktop.ErrUnavailable, nil, dialogError(desktop.ErrUnavailable)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			harness := newHarness(t)
			var gotPaths []string
			var gotMessage string
			harness.app.runDialog(func() ([]string, error) { return test.paths, test.err },
				func(paths []string) { gotPaths = paths },
				func(message string) { gotMessage = message })
			if gotPaths != nil || gotMessage != "" {
				t.Fatal("the results were handed over before the next frame")
			}
			harness.frame()
			if !slices.Equal(gotPaths, test.wantPaths) || gotMessage != test.wantMessage {
				t.Errorf("got paths %q and message %q", gotPaths, gotMessage)
			}
		})
	}
}

func TestSettings_RememberTheTheme(t *testing.T) {
	isolateSettings(t)
	if loadDarkTheme() {
		t.Error("the dark theme is chosen before anything was saved")
	}
	saveDarkTheme(true)
	if !loadDarkTheme() {
		t.Error("the dark theme wasn't remembered")
	}
	saveDarkTheme(false)
	if loadDarkTheme() {
		t.Error("the light theme wasn't remembered")
	}
}

func TestSettings_WorkWithoutASettingsFolder(t *testing.T) {
	for _, variable := range []string{"HOME", "XDG_CONFIG_HOME", "AppData"} {
		t.Setenv(variable, "")
	}
	if themeSettingsPath() != "" {
		t.Fatalf("settings path %q without a settings folder", themeSettingsPath())
	}
	saveDarkTheme(true)
	if loadDarkTheme() {
		t.Error("the dark theme was remembered with nowhere to save it")
	}
}

func TestSettings_IgnoreAFolderThatCantBeCreated(t *testing.T) {
	isolateSettings(t)
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, variable := range []string{"HOME", "XDG_CONFIG_HOME", "AppData"} {
		t.Setenv(variable, blocker)
	}
	saveDarkTheme(true)
	if loadDarkTheme() {
		t.Error("the dark theme was remembered although it couldn't be saved")
	}
}
