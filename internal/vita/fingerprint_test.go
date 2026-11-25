package vita

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The vectors are shared with Vita, which must compute the same fingerprints and reject the same invalid keys.
func TestFingerprintMatchesSharedVectors(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/fingerprint-vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Vectors []struct {
			Description string `json:"description"`
			Input       string `json:"input"`
			Fingerprint string `json:"fingerprint"`
		} `json:"vectors"`
		InvalidRecipients []struct {
			Description string `json:"description"`
			Input       string `json:"input"`
		} `json:"invalidRecipients"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatal(err)
	}
	if len(file.Vectors) == 0 {
		t.Fatal("no vectors")
	}
	for _, vector := range file.Vectors {
		if got := Fingerprint(vector.Input); got != vector.Fingerprint {
			t.Errorf("%s: got %q, want %q", vector.Description, got, vector.Fingerprint)
		}
		if err := CheckRecipient(vector.Input); err != nil {
			t.Errorf("%s: rejected: %v", vector.Description, err)
		}
	}
	if len(file.InvalidRecipients) == 0 {
		t.Fatal("no invalid recipients")
	}
	for _, invalid := range file.InvalidRecipients {
		if CheckRecipient(invalid.Input) == nil {
			t.Errorf("%s: accepted", invalid.Description)
		}
	}
}

func TestFingerprint_OfEmptyInputIsTheEmptyHash(t *testing.T) {
	// SHA-256 of no bytes starts e3b0c44298fc1c14.
	if got := Fingerprint(" \n"); got != "E3B0 C442 98FC 1C14" {
		t.Errorf("got %q", got)
	}
}

func TestCheckRecipient(t *testing.T) {
	const classic = "age1lvyvwawkr0mcnnnncaghunadrqkmuf9e6507x9y920xxpp866cnql7dp2z"
	tests := []struct {
		name      string
		recipient string
		isValid   bool
	}{
		{"classic public key is valid", classic, true},
		{"classic public key in capitals with spaces is valid", "  " + strings.ToUpper(classic) + "\r\n", true},
		{"public key with a bad checksum is invalid", classic[:len(classic)-1] + "q", false},
		{"short public key is invalid", "age1notarealkey", false},
		{"short hybrid public key is invalid", "age1pq1notarealkey", false},
		{"private key is invalid", "AGE-SECRET-KEY-1QQQQ", false},
		{"SSH key is invalid", "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA", false},
		{"empty is invalid", "", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := CheckRecipient(test.recipient); (err == nil) != test.isValid {
				t.Errorf("CheckRecipient(%q) = %v", test.recipient, err)
			}
		})
	}
}

func TestReadPublicKey(t *testing.T) {
	keyPair, err := CreateKeyPair(t.TempDir(), "Test", false)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		content string
		wantKey string
		wantErr error
		// wantMessage, if not empty, must be in the error.
		wantMessage string
	}{
		{name: "bare public key is read", content: "\n  " + keyPair.PublicKey + "  \n\n", wantKey: keyPair.PublicKey},
		{name: "public key file is read", content: readFile(t, keyPair.PublicKeyPath), wantKey: keyPair.PublicKey},
		{
			name: "private key file returns ErrPrivateKeyFile", content: readFile(t, keyPair.PrivateKeyPath),
			wantErr: ErrPrivateKeyFile,
		},
		{name: "comments alone fail", content: "# only a comment\n", wantMessage: "holds no public key"},
		{
			name: "two public keys fail", content: keyPair.PublicKey + "\n" + keyPair.PublicKey + "\n",
			wantMessage: "more than one public key",
		},
		{name: "invalid public key fails", content: "age1notarealkey\n", wantMessage: "not a valid age public key"},
		{
			name: "file over the size limit fails", content: strings.Repeat("#", maxKeyFileBytes+1),
			wantMessage: "too large",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "key.txt")
			writeFile(t, path, test.content)
			key, err := ReadPublicKey(path)
			if key != test.wantKey || (test.wantKey == "") == (err == nil) {
				t.Fatalf("got %q, %v", key, err)
			}
			if (test.wantErr != nil && !errors.Is(err, test.wantErr)) ||
				(err != nil && !strings.Contains(err.Error(), test.wantMessage)) {
				t.Errorf("got %v, want %v containing %q", err, test.wantErr, test.wantMessage)
			}
		})
	}
}

func TestReadPublicKey_UnreadableFileFails(t *testing.T) {
	folder := t.TempDir()
	if _, err := ReadPublicKey(filepath.Join(folder, "missing.txt")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("missing file: %v", err)
	}
	if _, err := ReadPublicKey(folder); err == nil {
		t.Error("a folder was read as a public key")
	}
}
