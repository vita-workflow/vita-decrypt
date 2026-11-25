package vita

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"filippo.io/age"
)

// errDiskFull is what failingWriter returns once its budget is spent.
var errDiskFull = errors.New("disk full")

// failingWriter accepts writes until remainingBytes would go below zero, then fails every write with errDiskFull.
type failingWriter struct{ remainingBytes int }

func (writer *failingWriter) Write(buffer []byte) (int, error) {
	if len(buffer) > writer.remainingBytes {
		return 0, errDiskFull
	}
	writer.remainingBytes -= len(buffer)
	return len(buffer), nil
}

// errBadRead is what failingReader returns.
var errBadRead = errors.New("bad read")

// failingReader fails every read with errBadRead.
type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errBadRead }

// createKeyPair creates a key pair in folder and reads back its private key.
func createKeyPair(t *testing.T, folder, name string, isClassic bool) (KeyPair, PrivateKey) {
	t.Helper()
	keyPair, err := CreateKeyPair(folder, name, isClassic)
	if err != nil {
		t.Fatal(err)
	}
	privateKey, err := ReadPrivateKey(keyPair.PrivateKeyPath)
	if err != nil {
		t.Fatal(err)
	}
	return keyPair, privateKey
}

// sealWithAge seals plaintext to recipient with the age library itself, as the file name in folder.
func sealWithAge(t *testing.T, folder, name, recipient string, plaintext []byte) string {
	t.Helper()
	recipients, err := age.ParseRecipients(strings.NewReader(recipient))
	if err != nil {
		t.Fatal(err)
	}
	var sealed bytes.Buffer
	encryptor, err := age.Encrypt(&sealed, recipients...)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := encryptor.Write(plaintext); err != nil {
		t.Fatal(err)
	}
	if err := encryptor.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(folder, name)
	writeFile(t, path, sealed.String())
	return path
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// assertFolderHolds checks that folder holds exactly the files named, so nothing (no temporary file, no partial
// output) was left behind.
func assertFolderHolds(t *testing.T, folder string, wantNames ...string) {
	t.Helper()
	entries, err := os.ReadDir(folder)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	slices.Sort(names)
	slices.Sort(wantNames)
	if !slices.Equal(names, wantNames) {
		t.Errorf("folder holds %q, want %q", names, wantNames)
	}
}

// isWantedError reports whether err is a failure matching wantErr, unless that is nil, and containing wantMessage.
func isWantedError(err, wantErr error, wantMessage string) bool {
	return err != nil && (wantErr == nil || errors.Is(err, wantErr)) && strings.Contains(err.Error(), wantMessage)
}

// testContext returns the test's context, already cancelled if isCancelled.
func testContext(t *testing.T, isCancelled bool) context.Context {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	if isCancelled {
		cancel()
	}
	return ctx
}

// makeReadOnly stops new files being created in folder until the test ends. It skips the test where folder modes
// don't stop that: on Windows, and for a user (root) whom they don't apply to.
func makeReadOnly(t *testing.T, folder string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("Windows ignores Unix modes on folders")
	}
	info, err := os.Stat(folder)
	if err != nil {
		t.Fatal(err)
	}
	const writeBits = 0o222
	if err := os.Chmod(folder, info.Mode()&^writeBits); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(folder, info.Mode()); err != nil {
			t.Error(err)
		}
	})
	probe, err := os.CreateTemp(folder, "probe-*")
	if err == nil {
		if err := probe.Close(); err != nil {
			t.Error(err)
		}
		if err := os.Remove(probe.Name()); err != nil {
			t.Error(err)
		}
		t.Skip("folder modes don't apply to this user")
	}
}
