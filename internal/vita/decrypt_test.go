package vita

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"filippo.io/age"
)

func TestDecryptedName(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantOutput string
		wantErr    error
	}{
		{"sealed file loses .age", "report.pdf.age", "report.pdf", nil},
		{
			"path keeps its folder", filepath.Join("folder", "report.pdf.age"),
			filepath.Join("folder", "report.pdf"), nil,
		},
		{"only the last .age goes", "notes.age.age", "notes.age", nil},
		{"name without .age returns ErrNotSealedName", "report.pdf", "", ErrNotSealedName},
		{"capital .AGE returns ErrNotSealedName", "report.pdf.AGE", "", ErrNotSealedName},
		{"bare .age returns ErrNotSealedName", ".age", "", ErrNotSealedName},
		{"bare .age in a folder returns ErrNotSealedName", filepath.Join("folder", ".age"), "", ErrNotSealedName},
		{"empty name returns ErrNotSealedName", "", "", ErrNotSealedName},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			output, err := DecryptedName(test.input)
			if output != test.wantOutput || !errors.Is(err, test.wantErr) {
				t.Errorf("got %q, %v; want %q, %v", output, err, test.wantOutput, test.wantErr)
			}
		})
	}
}

// decryptFixture is a folder holding the key pairs Right and Wrong, and a.bin.age sealed to Right.
type decryptFixture struct {
	folder   string
	sealed   string
	right    PrivateKey
	wrong    PrivateKey
	keyFiles []string
}

func createDecryptFixture(t *testing.T) decryptFixture {
	t.Helper()
	folder := t.TempDir()
	rightPair, right := createKeyPair(t, folder, "Right", true)
	wrongPair, wrong := createKeyPair(t, folder, "Wrong", false)
	// Large enough for several 64 KiB chunks, so damage lands after plaintext could have been written.
	sealed := sealWithAge(t, folder, "a.bin.age", rightPair.PublicKey, bytes.Repeat([]byte("x"), 300*1024))
	keyFiles := []string{
		rightPair.PrivateKeyPath, rightPair.PublicKeyPath, wrongPair.PrivateKeyPath, wrongPair.PublicKeyPath,
	}
	for index, path := range keyFiles {
		keyFiles[index] = filepath.Base(path)
	}
	return decryptFixture{folder: folder, sealed: sealed, right: right, wrong: wrong, keyFiles: keyFiles}
}

// The app words each outcome differently, so each failure must be told apart.
func TestDecrypt_Failures(t *testing.T) {
	tests := []struct {
		name string
		// prepare sets the fixture up for the case and returns the file to decrypt.
		prepare     func(t *testing.T, fixture decryptFixture) string
		isWrongKey  bool
		isCancelled bool
		wantErr     error
		// wantNoIdentityMatch is whether errors.As must find an *age.NoIdentityMatchError.
		wantNoIdentityMatch bool
		// wantFiles are the files besides the key files the folder must hold afterwards.
		wantFiles []string
	}{
		{
			name:       "wrong key returns ErrCannotDecrypt with NoIdentityMatchError",
			prepare:    func(_ *testing.T, fixture decryptFixture) string { return fixture.sealed },
			isWrongKey: true, wantErr: ErrCannotDecrypt, wantNoIdentityMatch: true, wantFiles: []string{"a.bin.age"},
		},
		{
			name: "tampered file returns ErrDamaged and leaves nothing",
			prepare: func(t *testing.T, fixture decryptFixture) string {
				t.Helper()
				data := []byte(readFile(t, fixture.sealed))
				data[len(data)-10] ^= 0xFF
				writeFile(t, fixture.sealed, string(data))
				return fixture.sealed
			},
			wantErr: ErrDamaged, wantFiles: []string{"a.bin.age"},
		},
		{
			name: "file that isn't age returns ErrCannotDecrypt without NoIdentityMatchError",
			prepare: func(t *testing.T, fixture decryptFixture) string {
				t.Helper()
				path := filepath.Join(fixture.folder, "notes.txt.age")
				writeFile(t, path, "just text")
				return path
			},
			wantErr: ErrCannotDecrypt, wantFiles: []string{"a.bin.age", "notes.txt.age"},
		},
		{
			name: "existing output returns fs.ErrExist and is kept",
			prepare: func(t *testing.T, fixture decryptFixture) string {
				t.Helper()
				writeFile(t, filepath.Join(fixture.folder, "a.bin"), "keep me")
				return fixture.sealed
			},
			wantErr: fs.ErrExist, wantFiles: []string{"a.bin.age", "a.bin"},
		},
		{
			name: "name without .age returns ErrNotSealedName",
			prepare: func(_ *testing.T, fixture decryptFixture) string {
				return filepath.Join(fixture.folder, "report.pdf")
			},
			wantErr: ErrNotSealedName, wantFiles: []string{"a.bin.age"},
		},
		{
			name:    "bare .age returns ErrNotSealedName",
			prepare: func(_ *testing.T, fixture decryptFixture) string { return filepath.Join(fixture.folder, ".age") },
			wantErr: ErrNotSealedName, wantFiles: []string{"a.bin.age"},
		},
		{
			name: "missing file returns fs.ErrNotExist",
			prepare: func(_ *testing.T, fixture decryptFixture) string {
				return filepath.Join(fixture.folder, "missing.bin.age")
			},
			wantErr: fs.ErrNotExist, wantFiles: []string{"a.bin.age"},
		},
		{
			name:        "cancelled context returns context.Canceled",
			prepare:     func(_ *testing.T, fixture decryptFixture) string { return fixture.sealed },
			isCancelled: true, wantErr: context.Canceled, wantFiles: []string{"a.bin.age"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := createDecryptFixture(t)
			input := test.prepare(t, fixture)
			identities := fixture.right.Identities
			if test.isWrongKey {
				identities = fixture.wrong.Identities
			}
			output, err := Decrypt(testContext(t, test.isCancelled), identities, input, nil)
			var noMatch *age.NoIdentityMatchError
			if output != "" || !errors.Is(err, test.wantErr) || errors.As(err, &noMatch) != test.wantNoIdentityMatch {
				t.Errorf("got %q, %v; want %v", output, err, test.wantErr)
			}
			if errors.Is(err, fs.ErrExist) && readFile(t, filepath.Join(fixture.folder, "a.bin")) != "keep me" {
				t.Error("the existing file changed")
			}
			assertFolderHolds(t, fixture.folder, append(test.wantFiles, fixture.keyFiles...)...)
		})
	}
}

// The CLI prints these messages, so they must name the kind of failure and then its cause.
func TestDecrypt_FailureMessages(t *testing.T) {
	fixture := createDecryptFixture(t)
	_, err := Decrypt(t.Context(), fixture.wrong.Identities, fixture.sealed, nil)
	if err == nil || !strings.HasPrefix(err.Error(), ErrCannotDecrypt.Error()+": ") {
		t.Errorf("got %v, want it to start with %q", err, ErrCannotDecrypt.Error()+": ")
	}
}

func TestDecrypt_ReadOnlyFolderFails(t *testing.T) {
	fixture := createDecryptFixture(t)
	makeReadOnly(t, fixture.folder)
	_, err := Decrypt(t.Context(), fixture.right.Identities, fixture.sealed, nil)
	if !errors.Is(err, fs.ErrPermission) {
		t.Errorf("got %v, want fs.ErrPermission", err)
	}
	assertFolderHolds(t, fixture.folder, append(fixture.keyFiles, "a.bin.age")...)
}

// TestDecrypt_CancelLeavesNothing cancels part way through: no output and no temporary file may remain.
func TestDecrypt_CancelLeavesNothing(t *testing.T) {
	fixture := createDecryptFixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var lastRead int64
	_, err := Decrypt(ctx, fixture.right.Identities, fixture.sealed, func(read int64) {
		lastRead = read
		if read > 200*1024 {
			cancel()
		}
	})
	if !errors.Is(err, context.Canceled) || errors.Is(err, ErrDamaged) {
		t.Fatalf("got %v, want context.Canceled and not ErrDamaged", err)
	}
	if lastRead <= 200*1024 {
		t.Errorf("progress stopped at %d bytes", lastRead)
	}
	assertFolderHolds(t, fixture.folder, append(fixture.keyFiles, "a.bin.age")...)
}

// replaceLinkFile makes publish call link instead of os.Link until the test ends.
func replaceLinkFile(t *testing.T, link func(oldPath, newPath string) error) {
	t.Helper()
	linkFile = link
	t.Cleanup(func() { linkFile = os.Link })
}

// TestDecryptWithoutHardLinks stands in for FAT32, exFAT, and network shares, where os.Link fails: Decrypt must still
// write the file, leave no temporary file, and refuse to overwrite.
func TestDecryptWithoutHardLinks(t *testing.T) {
	replaceLinkFile(t, func(oldPath, newPath string) error {
		return &os.LinkError{Op: "link", Old: oldPath, New: newPath, Err: errors.ErrUnsupported}
	})
	fixture := createDecryptFixture(t)

	output, err := Decrypt(t.Context(), fixture.right.Identities, fixture.sealed, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, output); got != strings.Repeat("x", 300*1024) {
		t.Fatal("the fallback wrote the wrong plaintext")
	}
	assertFolderHolds(t, fixture.folder, append(fixture.keyFiles, "a.bin.age", "a.bin")...)

	// The output appears after Decrypt's own check, so only the exclusive create can stop the overwrite.
	writeFile(t, output, "keep me")
	if err := publish(fixture.sealed, output); !errors.Is(err, fs.ErrExist) {
		t.Errorf("the fallback overwrote an existing file: %v", err)
	}
	if got := readFile(t, output); got != "keep me" {
		t.Error("existing file changed")
	}
}

// TestDecrypt_OutputAppearingLateIsKept creates the output after Decrypt's own check: the hard link must refuse to
// replace it, and the temporary file must go.
func TestDecrypt_OutputAppearingLateIsKept(t *testing.T) {
	replaceLinkFile(t, func(oldPath, newPath string) error {
		if err := os.WriteFile(newPath, []byte("keep me"), 0o600); err != nil {
			return err
		}
		return os.Link(oldPath, newPath)
	})
	fixture := createDecryptFixture(t)
	if _, err := Decrypt(t.Context(), fixture.right.Identities, fixture.sealed, nil); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("got %v, want fs.ErrExist", err)
	}
	if got := readFile(t, filepath.Join(fixture.folder, "a.bin")); got != "keep me" {
		t.Error("the file that appeared was replaced")
	}
	assertFolderHolds(t, fixture.folder, append(fixture.keyFiles, "a.bin.age", "a.bin")...)
}

func TestCopyExclusive_Failures(t *testing.T) {
	tests := []struct {
		name string
		// paths sets the case up in folder and returns the source and destination.
		paths   func(t *testing.T, folder string) (string, string)
		wantErr error
		// wantMessage, if not empty, must be in the error.
		wantMessage string
	}{
		{
			name: "missing source returns fs.ErrNotExist",
			paths: func(_ *testing.T, folder string) (string, string) {
				return filepath.Join(folder, "missing"), filepath.Join(folder, "output")
			},
			wantErr: fs.ErrNotExist,
		},
		{
			name: "missing destination folder returns fs.ErrNotExist",
			paths: func(t *testing.T, folder string) (string, string) {
				t.Helper()
				writeFile(t, filepath.Join(folder, "source"), "plaintext")
				return filepath.Join(folder, "source"), filepath.Join(folder, "missing", "output")
			},
			wantErr: fs.ErrNotExist,
		},
		{
			name: "existing destination returns fs.ErrExist",
			paths: func(t *testing.T, folder string) (string, string) {
				t.Helper()
				writeFile(t, filepath.Join(folder, "source"), "plaintext")
				writeFile(t, filepath.Join(folder, "output"), "keep me")
				return filepath.Join(folder, "source"), filepath.Join(folder, "output")
			},
			wantErr: fs.ErrExist,
		},
		{
			name: "failed copy removes the destination",
			paths: func(t *testing.T, folder string) (string, string) {
				t.Helper()
				source := filepath.Join(folder, "source")
				if err := os.Mkdir(source, 0o700); err != nil {
					t.Fatal(err)
				}
				return source, filepath.Join(folder, "output")
			},
			wantMessage: "nothing was kept",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			folder := t.TempDir()
			source, destination := test.paths(t, folder)
			err := copyExclusive(source, destination)
			if !isWantedError(err, test.wantErr, test.wantMessage) {
				t.Fatalf("got %v, want %v containing %q", err, test.wantErr, test.wantMessage)
			}
			if test.wantErr == nil {
				if _, err := os.Lstat(destination); !errors.Is(err, fs.ErrNotExist) {
					t.Errorf("the partial copy was left behind: %v", err)
				}
			}
		})
	}
}

func TestCopyPlaintext(t *testing.T) {
	tests := []struct {
		name         string
		isFailedRead bool
		writeBudget  int
		isCancelled  bool
		wantErr      error
	}{
		{name: "full copy succeeds", writeBudget: 100},
		{name: "failed write returns the write error, not ErrDamaged", writeBudget: 0, wantErr: errDiskFull},
		{name: "failed read returns ErrDamaged", isFailedRead: true, writeBudget: 100, wantErr: ErrDamaged},
		{
			name: "failed read after a cancel returns context.Canceled", isFailedRead: true, writeBudget: 100,
			isCancelled: true, wantErr: context.Canceled,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var plaintext io.Reader = strings.NewReader("plaintext")
			if test.isFailedRead {
				plaintext = failingReader{}
			}
			destination := &failingWriter{remainingBytes: test.writeBudget}
			err := copyPlaintext(testContext(t, test.isCancelled), destination, plaintext, "output")
			if !errors.Is(err, test.wantErr) || (test.wantErr == nil) != (err == nil) {
				t.Errorf("got %v, want %v", err, test.wantErr)
			}
			if errors.Is(err, errDiskFull) && (errors.Is(err, ErrDamaged) || !strings.Contains(err.Error(), "output")) {
				t.Errorf("a failed write was reported as %v", err)
			}
		})
	}
}
