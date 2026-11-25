package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gioui.org/io/key"

	"github.com/vita-workflow/vita-decrypt/internal/desktop"
	"github.com/vita-workflow/vita-decrypt/internal/vita"
)

func TestCreateScreen_CreatesAKeyPairAndGoesBackWhenBackedUp(t *testing.T) {
	harness := newHarness(t)
	harness.showScreen(createScreen)
	state := &harness.app.create
	folder := t.TempDir()
	harness.system.chosenFolder = folder
	harness.click(&state.changeFolderButton)
	if state.folder != folder {
		t.Fatalf("folder %q, want %q", state.folder, folder)
	}
	if dialog := harness.system.dialogs[0]; dialog.Folder == "" || dialog.Title == "" {
		t.Errorf("the folder dialog didn't start in the current folder: %+v", dialog)
	}
	harness.click(&state.classicButton)
	if !state.isClassic {
		t.Fatal("choosing classic didn't select it")
	}
	state.nameEditor.SetText("family-vault")
	harness.explore()
	harness.click(&state.createButton)
	created := state.created
	if created == nil {
		t.Fatalf("no key pair created; name error %q, folder error %q", state.nameError.message, state.folderError)
	}
	if created.PrivateKeyPath != filepath.Join(folder, vita.PrivateKeyFileName("family-vault")) {
		t.Errorf("private key saved at %q", created.PrivateKeyPath)
	}
	if _, err := os.Stat(created.PublicKeyPath); err != nil {
		t.Errorf("public key file: %v", err)
	}
	harness.explore()

	harness.click(&state.copyPublicKeyButton)
	if copied := harness.copied(); copied != created.PublicKey {
		t.Errorf("copied %q, want the public key", copied)
	}
	harness.click(&state.copyFingerprintButton)
	if copied := harness.copied(); copied != vita.Fingerprint(created.PublicKey) {
		t.Errorf("copied %q, want the fingerprint", copied)
	}
	harness.click(&state.showPrivateKeyButton)
	harness.click(&state.showPublicKeyButton)
	if want := []string{created.PrivateKeyPath, created.PublicKeyPath}; strings.Join(harness.system.revealed, "|") !=
		strings.Join(want, "|") {
		t.Errorf("revealed %q, want %q", harness.system.revealed, want)
	}

	harness.click(&state.doneButton)
	if state.created == nil {
		t.Fatal("Done went back before the private key was backed up")
	}
	state.backedUpCheckbox.Value = true
	harness.click(&state.doneButton)
	if state.created != nil || state.nameEditor.Text() != "" {
		t.Error("Done didn't go back to an empty form")
	}
	harness.click(&state.postQuantumButton)
	if state.isClassic {
		t.Error("choosing post-quantum didn't select it")
	}
}

func TestCreateScreen_CreatesOnEnter(t *testing.T) {
	harness := newHarness(t)
	harness.showScreen(createScreen)
	state := &harness.app.create
	state.folder = t.TempDir()
	state.isClassic = true
	state.nameEditor.SetText("family-vault")
	harness.router.Source().Execute(key.FocusCmd{Tag: &state.nameEditor})
	harness.frame()
	harness.press(key.NameReturn, 0)
	if state.created == nil {
		t.Error("Enter in the name field didn't create the key pair")
	}
}

func TestCreateScreen_ExplainsWhyNothingWasCreated(t *testing.T) {
	folder := t.TempDir()
	if _, err := vita.CreateKeyPair(folder, "taken", true); err != nil {
		t.Fatal(err)
	}
	notAFolder := writeFile(t, filepath.Join(folder, "file"), "")
	tests := []struct {
		name, folder, keyName string
		wantNameError         string
		wantFolderError       string
	}{
		{"name taken", folder, "taken", "taken-private-key.txt or taken-public-key.txt already exists", ""},
		{"folder is a file", notAFolder, "fresh", "", "The key pair wasn’t created: "},
		{"invalid name", folder, "-bad name", "", ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			harness := newHarness(t)
			harness.showScreen(createScreen)
			state := &harness.app.create
			state.folder, state.isClassic = test.folder, true
			state.nameEditor.SetText(test.keyName)
			harness.click(&state.createButton)
			if state.created != nil {
				t.Fatal("a key pair was created")
			}
			if !strings.HasPrefix(state.nameError.message, test.wantNameError) ||
				!strings.HasPrefix(state.folderError, test.wantFolderError) {
				t.Errorf("name error %q, folder error %q", state.nameError.message, state.folderError)
			}
		})
	}
}

func TestCreateScreen_ShowsWhyTheFolderDialogDidntOpen(t *testing.T) {
	harness := newHarness(t)
	harness.showScreen(createScreen)
	harness.system.dialogError = desktop.ErrUnavailable
	harness.click(&harness.app.create.changeFolderButton)
	if !strings.HasPrefix(harness.app.create.folderError, "This computer has no file chooser.") {
		t.Errorf("folder error %q", harness.app.create.folderError)
	}
}

// setHomeFolder makes os.UserHomeDir return folder, or fail if folder is empty, for the rest of the test. Windows
// reads USERPROFILE; Linux and macOS read HOME.
func setHomeFolder(t *testing.T, folder string) {
	t.Helper()
	t.Setenv("HOME", folder)
	t.Setenv("USERPROFILE", folder)
}

func TestDefaultKeyFolder(t *testing.T) {
	workingFolder, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	withDocuments := t.TempDir()
	mkdir(t, filepath.Join(withDocuments, "Documents"))
	withoutDocuments := t.TempDir()
	tests := []struct {
		name, home, want string
	}{
		{"documents", withDocuments, filepath.Join(withDocuments, "Documents")},
		{"home", withoutDocuments, withoutDocuments},
		{"no home", "", workingFolder},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setHomeFolder(t, test.home)
			if got := defaultKeyFolder(); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestDisplayPath(t *testing.T) {
	home := t.TempDir()
	setHomeFolder(t, home)
	outside := filepath.Join(filepath.Dir(home), "elsewhere")
	tests := []struct {
		name, path, want string
	}{
		{"home", home, "Home folder"},
		{"inside", filepath.Join(home, "Documents", "Vita keys"), "Documents / Vita keys"},
		{"outside", outside, outside},
		{"relative", "keys", "keys"},
	}
	for _, test := range tests {
		if got := displayPath(test.path, " / "); got != test.want {
			t.Errorf("%s: got %q, want %q", test.name, got, test.want)
		}
	}
	setHomeFolder(t, "")
	if got := displayPath(outside, " / "); got != outside {
		t.Errorf("without a home folder: got %q", got)
	}
}

func TestDialogError(t *testing.T) {
	if got := dialogError(errors.New("boom")); got != "The file chooser didn’t open: boom" {
		t.Errorf("got %q", got)
	}
}
