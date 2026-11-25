package main

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vita-workflow/vita-decrypt/internal/desktop"
	"github.com/vita-workflow/vita-decrypt/internal/vita"
)

func newKeyPair(t *testing.T) vita.KeyPair {
	t.Helper()
	keyPair, err := vita.CreateKeyPair(t.TempDir(), "test-pair", true)
	if err != nil {
		t.Fatal(err)
	}
	return keyPair
}

func TestCheckScreen_CopiesThePastedKeysFingerprint(t *testing.T) {
	keyPair := newKeyPair(t)
	fingerprint := vita.Fingerprint(keyPair.PublicKey)
	harness := newHarness(t)
	harness.showScreen(checkScreen)
	state := &harness.app.check

	harness.click(&state.pasteKey.button)
	harness.paste("  " + keyPair.PublicKey + "\n")
	if state.keyEditor.Text() != keyPair.PublicKey {
		t.Fatalf("pasted %q, want the trimmed key", state.keyEditor.Text())
	}
	harness.click(&state.pasteExpected.button)
	harness.paste(strings.ToLower(fingerprint))
	harness.explore()

	harness.click(&state.copyButton)
	if copied := harness.copied(); copied != fingerprint {
		t.Errorf("copied %q, want %q", copied, fingerprint)
	}
	if state.copiedAt != harness.now {
		t.Error("the copy wasn't confirmed")
	}
	harness.now = harness.now.Add(copyConfirmation)
	harness.frame()
}

func TestCheckScreen_DoesNotCopyAnInvalidKey(t *testing.T) {
	harness := newHarness(t)
	harness.showScreen(checkScreen)
	harness.app.check.keyEditor.SetText("age1nope")
	harness.click(&harness.app.check.copyButton)
	if !harness.app.check.copiedAt.IsZero() {
		t.Error("an invalid key's fingerprint was copied")
	}
}

func TestChoosePublicKey(t *testing.T) {
	keyPair := newKeyPair(t)
	other := writeFile(t, filepath.Join(t.TempDir(), "notes.txt"), "hello")
	tests := []struct {
		name        string
		chosen      string
		dialogError error
		wantKey     string
		wantError   string
	}{
		{"public key file", keyPair.PublicKeyPath, nil, keyPair.PublicKey, ""},
		{"private key file", keyPair.PrivateKeyPath, nil, "", "That’s your private key file."},
		{"other file", other, nil, "", "That file doesn’t hold a public key: "},
		{"cancelled", "", nil, "", ""},
		{"no file chooser", "", desktop.ErrUnavailable, "", "This computer has no file chooser."},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			harness := newHarness(t)
			harness.showScreen(checkScreen)
			harness.system.chosenFile, harness.system.dialogError = test.chosen, test.dialogError
			state := &harness.app.check
			harness.click(&state.chooseKeyButton)
			if state.keyEditor.Text() != test.wantKey {
				t.Errorf("key %q, want %q", state.keyEditor.Text(), test.wantKey)
			}
			message := state.keyFileError.messageFor(state.keyEditor.Text())
			if !strings.HasPrefix(message, test.wantError) || (test.wantError == "") != (message == "") {
				t.Errorf("error %q, want it to start %q", message, test.wantError)
			}
			if len(harness.system.dialogs) != 1 || harness.system.dialogs[0].Title != publicKeyDialog.Title {
				t.Errorf("dialogs %+v", harness.system.dialogs)
			}
		})
	}
}

func TestInputError_LastsUntilTheInputChanges(t *testing.T) {
	var problem inputError
	if problem.messageFor("") != "" {
		t.Error("a new inputError has a message")
	}
	problem.set("bad name", "-bad")
	if problem.messageFor("-bad") != "bad name" || problem.messageFor("-bad2") != "" {
		t.Error("the message doesn't follow the input")
	}
	problem.clear()
	if problem.messageFor("-bad") != "" {
		t.Error("clear didn't clear the message")
	}
}

func TestCompactKey(t *testing.T) {
	if got := compactKey(" age1abc\n def\tghi "); got != "age1abcdefghi" {
		t.Errorf("got %q", got)
	}
}

func TestSameFingerprint(t *testing.T) {
	tests := []struct {
		first, second string
		want          bool
	}{
		{"D6B3 1A36 E35D 662F", "d6b31a36e35d662f", true},
		{"D6B3 1A36 E35D 662F", " D6B3  1A36\nE35D 662F ", true},
		{"D6B3 1A36 E35D 662F", "D6B3 1A36 E35D 6620", false},
		{"", "", true},
	}
	for _, test := range tests {
		if got := sameFingerprint(test.first, test.second); got != test.want {
			t.Errorf("sameFingerprint(%q, %q) = %v", test.first, test.second, got)
		}
	}
}

func TestKeyFingerprint(t *testing.T) {
	keyPair := newKeyPair(t)
	tests := []struct {
		name, key, want string
	}{
		{"empty", "", ""},
		{"invalid", "age1nope", ""},
		{"valid", keyPair.PublicKey, vita.Fingerprint(keyPair.PublicKey)},
	}
	for _, test := range tests {
		if got := keyFingerprint(test.key); got != test.want {
			t.Errorf("%s: got %q, want %q", test.name, got, test.want)
		}
	}
}

func TestInvalidKeyMessage(t *testing.T) {
	tests := []struct{ key, want string }{
		{"hello", "This isn’t a valid public key. Vita public keys start with age1."},
		{"AGE1abc", "This isn’t a valid public key. Check that you copied all of it."},
	}
	for _, test := range tests {
		if got := invalidKeyMessage(test.key); got != test.want {
			t.Errorf("invalidKeyMessage(%q) = %q, want %q", test.key, got, test.want)
		}
	}
}

func TestIsCopyConfirmed(t *testing.T) {
	harness := newHarness(t)
	copiedAt := harness.now
	tests := []struct {
		name     string
		copiedAt time.Time
		now      time.Time
		want     bool
	}{
		{"never copied", time.Time{}, copiedAt, false},
		{"just copied", copiedAt, copiedAt.Add(time.Second), true},
		{"long ago", copiedAt, copiedAt.Add(copyConfirmation), false},
	}
	for _, test := range tests {
		harness.now = test.now
		if got := isCopyConfirmed(harness.context(), test.copiedAt); got != test.want {
			t.Errorf("%s: got %v, want %v", test.name, got, test.want)
		}
	}
}
