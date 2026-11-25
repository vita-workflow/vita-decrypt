package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vita-workflow/vita-decrypt/internal/vita"
)

func TestSealScreen_SealsAFileThenOpensIt(t *testing.T) {
	keyPair := newKeyPair(t)
	plain := writeFile(t, filepath.Join(t.TempDir(), "notes.txt"), "seal me")
	harness := newHarness(t)
	harness.showScreen(sealScreen)
	state := &harness.app.seal

	harness.click(&state.paste.button)
	harness.paste(keyPair.PublicKey)
	harness.system.chosenFile = plain
	harness.click(&state.chooseFileButton)
	if state.file != plain || state.sizeBytes != int64(len("seal me")) {
		t.Fatalf("file %q of %d bytes, want %q", state.file, state.sizeBytes, plain)
	}
	if dialog := harness.system.dialogs[0]; dialog.Title != fileToSealDialog.Title {
		t.Errorf("dialog %q", dialog.Title)
	}
	harness.explore()

	harness.click(&state.startButton)
	harness.frame()
	result := state.result
	if state.isWorking || result == nil || result.kind != successMessage {
		t.Fatalf("sealing didn't finish: %+v", result)
	}
	if result.outputPath != vita.SealedName(plain) {
		t.Errorf("sealed to %q", result.outputPath)
	}
	harness.explore()

	harness.click(&state.showButton)
	if len(harness.system.revealed) != 1 || harness.system.revealed[0] != result.outputPath {
		t.Errorf("revealed %q", harness.system.revealed)
	}
	harness.click(&state.startButton)
	harness.frame()
	if state.result == nil || state.result.kind != warningMessage {
		t.Errorf("sealing again gave %+v, want a warning that the sealed file exists", state.result)
	}

	state.result = result
	harness.click(&state.openNowButton)
	if harness.app.screen != openScreen || len(harness.app.open.files) != 1 ||
		harness.app.open.files[0].path != result.outputPath || state.result != nil {
		t.Error("Open it now didn't hand the sealed file to the open screen")
	}
}

func TestSealScreen_ForgetsTheResultWhenTheKeyChanges(t *testing.T) {
	keyPair := newKeyPair(t)
	harness := newHarness(t)
	harness.showScreen(sealScreen)
	state := &harness.app.seal
	state.keyEditor.SetText(keyPair.PublicKey)
	harness.app.setSealFile(writeFile(t, filepath.Join(t.TempDir(), "notes.txt"), "seal me"))
	harness.click(&state.startButton)
	harness.frame()
	if state.result == nil {
		t.Fatal("nothing was sealed")
	}
	state.keyEditor.SetText(newKeyPair(t).PublicKey)
	harness.frame()
	if state.result != nil {
		t.Error("the result stayed after the key changed")
	}
}

func TestSealScreen_DoesNotStartWithoutAValidKeyAndFile(t *testing.T) {
	keyPair := newKeyPair(t)
	plain := writeFile(t, filepath.Join(t.TempDir(), "notes.txt"), "seal me")
	tests := []struct {
		name, key, file string
		isWorking       bool
	}{
		{"no key", "", plain, false},
		{"invalid key", "age1nope", plain, false},
		{"no file", keyPair.PublicKey, "", false},
		{"already working", keyPair.PublicKey, plain, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			harness := newHarness(t)
			state := &harness.app.seal
			state.keyEditor.SetText(test.key)
			state.file, state.isWorking = test.file, test.isWorking
			harness.app.startSealing()
			if state.result != nil || state.resultFor != "" {
				t.Error("sealing started")
			}
			if _, err := os.Stat(vita.SealedName(plain)); err == nil {
				t.Error("a sealed file was written")
			}
		})
	}
}

func TestSetSealFile(t *testing.T) {
	folder := t.TempDir()
	plain := writeFile(t, filepath.Join(folder, "notes.txt"), "seal me")
	tests := []struct {
		name, path, wantFile, wantError string
	}{
		{"file", plain, plain, ""},
		{"folder", folder, "", "Choose a file, not a folder."},
		{"missing", filepath.Join(folder, "gone.txt"), "", "Can’t read “gone.txt”."},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			harness := newHarness(t)
			harness.app.setSealFile(test.path)
			state := &harness.app.seal
			if state.file != test.wantFile || !strings.Contains(state.fileError, test.wantError) ||
				(test.wantError == "") != (state.fileError == "") {
				t.Errorf("file %q, error %q", state.file, state.fileError)
			}
		})
	}
	harness := newHarness(t)
	harness.app.seal.isWorking = true
	harness.app.setSealFile(plain)
	if harness.app.seal.file != "" {
		t.Error("the file changed while sealing")
	}
}

func TestSealScreen_ShowsWhyTheFileDialogDidntOpen(t *testing.T) {
	harness := newHarness(t)
	harness.showScreen(sealScreen)
	harness.system.dialogError = errors.New("portal timed out")
	harness.click(&harness.app.seal.chooseFileButton)
	if harness.app.seal.fileError != "The file chooser didn’t open: portal timed out" {
		t.Errorf("file error %q", harness.app.seal.fileError)
	}
}

func TestSealOutcome(t *testing.T) {
	input := filepath.Join(t.TempDir(), "notes.txt")
	tests := []struct {
		name     string
		err      error
		wantKind messageKind
		wantText string
	}{
		{"sealed", nil, successMessage, "Sealed as notes.txt.age"},
		{"exists", fmt.Errorf("x: %w", fs.ErrExist), warningMessage, "notes.txt.age already exists in this folder."},
		{"other", errors.New("disk full"), dangerMessage, "The file wasn’t sealed: disk full"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := sealOutcome(input, input+".age", test.err)
			if result.kind != test.wantKind || !strings.HasPrefix(result.text, test.wantText) {
				t.Errorf("got %v %q, want %v %q…", result.kind, result.text, test.wantKind, test.wantText)
			}
		})
	}
}

func TestSealKeyNote(t *testing.T) {
	keyPair := newKeyPair(t)
	tests := []struct {
		name, key, fileError  string
		wantError, wantHelper string
	}{
		{"empty", "", "", "", "Paste the public key"},
		{"invalid", "age1nope", "", "This isn’t a valid public key.", "Paste the public key"},
		{"valid", keyPair.PublicKey, "", "", "Fingerprint " + vita.Fingerprint(keyPair.PublicKey)},
		{
			"unreadable file", "", "That’s your private key file.", "That’s your private key file.",
			"Paste the public key",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var state sealState
			state.keyEditor.SetText(test.key)
			if test.fileError != "" {
				state.keyFileError.set(test.fileError, test.key)
			}
			key, isValid := state.recipient()
			errorMessage, helper := state.sealKeyNote(key, isValid)
			if !strings.HasPrefix(errorMessage, test.wantError) || (test.wantError == "") != (errorMessage == "") ||
				!strings.HasPrefix(helper, test.wantHelper) {
				t.Errorf("got error %q and helper %q", errorMessage, helper)
			}
		})
	}
}
