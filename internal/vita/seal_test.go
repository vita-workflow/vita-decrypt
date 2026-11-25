package vita

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"filippo.io/age"
)

func TestSealThenDecrypt(t *testing.T) {
	tests := []struct {
		name      string
		isClassic bool
	}{
		{"hybrid public key seals what its private key decrypts", false},
		{"classic public key seals what its private key decrypts", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			folder := t.TempDir()
			keyPair, privateKey := createKeyPair(t, folder, "Test", test.isClassic)
			plaintext := strings.Repeat("seal me ", 40*1024)
			input := filepath.Join(folder, "notes.txt")
			writeFile(t, input, plaintext)
			var bytesRead int64
			recipient := " " + strings.ToUpper(keyPair.PublicKey) + "\n"
			sealed, err := Seal(t.Context(), recipient, input, func(read int64) { bytesRead = read })
			if err != nil {
				t.Fatal(err)
			}
			if sealed != SealedName(input) || bytesRead != int64(len(plaintext)) {
				t.Errorf("sealed to %s after reading %d bytes", sealed, bytesRead)
			}
			if err := os.Remove(input); err != nil {
				t.Fatal(err)
			}
			assertDecryptsTo(t, privateKey, sealed, plaintext)
		})
	}
}

// assertDecryptsTo decrypts sealed with privateKey and checks the result holds wantPlaintext.
func assertDecryptsTo(t *testing.T, privateKey PrivateKey, sealed, wantPlaintext string) {
	t.Helper()
	output, err := Decrypt(t.Context(), privateKey.Identities, sealed, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, output); got != wantPlaintext {
		t.Fatal("the sealed file didn't decrypt to the original")
	}
}

func TestSealedName(t *testing.T) {
	if got := SealedName(filepath.Join("folder", "report.pdf")); got != filepath.Join("folder", "report.pdf.age") {
		t.Errorf("SealedName: %q", got)
	}
}

func TestSeal_Failures(t *testing.T) {
	tests := []struct {
		name string
		// prepare sets the case up in folder and returns the file to seal.
		prepare     func(t *testing.T, folder string) string
		recipient   string // empty means the fixture's public key
		isCancelled bool
		wantErr     error
		// wantMessage, if not empty, must be in the error.
		wantMessage string
		// wantFiles are the files besides the key files the folder must hold afterwards.
		wantFiles []string
	}{
		{
			name: "invalid public key fails", prepare: notesFile, recipient: "age1notarealkey",
			wantMessage: "not a valid age public key", wantFiles: []string{"notes.txt"},
		},
		{
			name: "existing sealed file returns fs.ErrExist and is kept",
			prepare: func(t *testing.T, folder string) string {
				t.Helper()
				writeFile(t, filepath.Join(folder, "notes.txt.age"), "keep me")
				return notesFile(t, folder)
			},
			wantErr: fs.ErrExist, wantFiles: []string{"notes.txt", "notes.txt.age"},
		},
		{
			name:    "missing file returns fs.ErrNotExist",
			prepare: func(_ *testing.T, folder string) string { return filepath.Join(folder, "missing.txt") },
			wantErr: fs.ErrNotExist,
		},
		{name: "folder fails", prepare: innerFolder, wantMessage: "isn't a file", wantFiles: []string{"inner"}},
		{
			name: "cancelled context returns context.Canceled", prepare: notesFile, isCancelled: true,
			wantErr: context.Canceled, wantFiles: []string{"notes.txt"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			folder := t.TempDir()
			keyPair, err := CreateKeyPair(folder, "Test", true)
			if err != nil {
				t.Fatal(err)
			}
			input := test.prepare(t, folder)
			recipient := test.recipient
			if recipient == "" {
				recipient = keyPair.PublicKey
			}
			output, err := Seal(testContext(t, test.isCancelled), recipient, input, nil)
			if output != "" || !isWantedError(err, test.wantErr, test.wantMessage) {
				t.Errorf("got %q, %v; want %v containing %q", output, err, test.wantErr, test.wantMessage)
			}
			if errors.Is(err, fs.ErrExist) && readFile(t, filepath.Join(folder, "notes.txt.age")) != "keep me" {
				t.Error("the existing sealed file changed")
			}
			keyFiles := []string{filepath.Base(keyPair.PrivateKeyPath), filepath.Base(keyPair.PublicKeyPath)}
			assertFolderHolds(t, folder, append(test.wantFiles, keyFiles...)...)
		})
	}
}

// innerFolder makes the folder inner in folder and returns its path.
func innerFolder(t *testing.T, folder string) string {
	t.Helper()
	inner := filepath.Join(folder, "inner")
	if err := os.Mkdir(inner, 0o700); err != nil {
		t.Fatal(err)
	}
	return inner
}

// notesFile writes notes.txt in folder and returns its path.
func notesFile(t *testing.T, folder string) string {
	t.Helper()
	path := filepath.Join(folder, "notes.txt")
	writeFile(t, path, "notes")
	return path
}

func TestSeal_ReadOnlyFolderFails(t *testing.T) {
	folder := t.TempDir()
	keyPair, err := CreateKeyPair(folder, "Test", true)
	if err != nil {
		t.Fatal(err)
	}
	input := notesFile(t, folder)
	makeReadOnly(t, folder)
	if _, err := Seal(t.Context(), keyPair.PublicKey, input, nil); !errors.Is(err, fs.ErrPermission) {
		t.Errorf("got %v, want fs.ErrPermission", err)
	}
	assertFolderHolds(t, folder,
		filepath.Base(keyPair.PrivateKeyPath), filepath.Base(keyPair.PublicKeyPath), "notes.txt")
}

// TestSeal_OutputAppearingLateIsKept creates the sealed file after Seal's own check: the hard link must refuse to
// replace it, and the temporary file must go.
func TestSeal_OutputAppearingLateIsKept(t *testing.T) {
	replaceLinkFile(t, func(oldPath, newPath string) error {
		if err := os.WriteFile(newPath, []byte("keep me"), 0o600); err != nil {
			return err
		}
		return os.Link(oldPath, newPath)
	})
	folder := t.TempDir()
	keyPair, err := CreateKeyPair(folder, "Test", true)
	if err != nil {
		t.Fatal(err)
	}
	input := notesFile(t, folder)
	if _, err := Seal(t.Context(), keyPair.PublicKey, input, nil); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("got %v, want fs.ErrExist", err)
	}
	if got := readFile(t, SealedName(input)); got != "keep me" {
		t.Error("the file that appeared was replaced")
	}
	assertFolderHolds(t, folder,
		filepath.Base(keyPair.PrivateKeyPath), filepath.Base(keyPair.PublicKeyPath), "notes.txt", "notes.txt.age")
}

// TestEncrypt_WriteFailures fails the destination at each stage of age's output: the header, a full chunk while
// copying, and the last chunk on close.
func TestEncrypt_WriteFailures(t *testing.T) {
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		// writeBudget is how many bytes the destination accepts, out of the totalBytes age writes.
		writeBudget    func(totalBytes int) int
		plaintextBytes int
	}{
		{"failed header write fails", func(int) int { return 0 }, 10},
		{"failed chunk write while copying fails", func(totalBytes int) int { return totalBytes / 2 }, 200 * 1024},
		{"failed last chunk write on close fails", func(totalBytes int) int { return totalBytes - 1 }, 10},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plaintext := make([]byte, test.plaintextBytes)
			var sealed bytes.Buffer
			if err := encrypt(&sealed, identity.Recipient(), bytes.NewReader(plaintext)); err != nil {
				t.Fatal(err)
			}
			destination := &failingWriter{remainingBytes: test.writeBudget(sealed.Len())}
			err := encrypt(destination, identity.Recipient(), bytes.NewReader(plaintext))
			if !errors.Is(err, errDiskFull) {
				t.Errorf("got %v, want errDiskFull", err)
			}
		})
	}
}
