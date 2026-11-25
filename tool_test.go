package main

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"filippo.io/age"

	"github.com/vita-workflow/vita-decrypt/internal/vita"
)

func TestKeygenThenDecrypt(t *testing.T) {
	tests := []struct {
		name      string
		arguments []string
		isHybrid  bool
	}{
		{"hybrid key pair decrypts what age sealed", []string{"--name", "Family-Only"}, true},
		{"classic key pair decrypts what age sealed", []string{"--name", "Family-Only", "--classic"}, false},
		{"classic option before the name works the same", []string{"--classic", "--name", "Family-Only"}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			folder := t.TempDir()
			report, err := keygen(test.arguments, folder)
			if err != nil {
				t.Fatal(err)
			}
			privatePath := filepath.Join(folder, "Family-Only-private-key.txt")
			publicPath := filepath.Join(folder, "Family-Only-public-key.txt")
			assertPrivateKeyMode(t, privatePath)

			recipient := publicKeyFrom(t, report)
			if strings.HasPrefix(recipient, "age1pq1") != test.isHybrid {
				t.Fatalf("unexpected recipient %q", recipient[:12])
			}
			wantReport := "Private key written to " + privatePath + ". Keep it safe: Vita can't recover it.\n" +
				"Public key written to " + publicPath + ".\n" +
				"Public key:\n" + recipient + "\nFingerprint: " + vita.Fingerprint(recipient) + "\n"
			if report != wantReport {
				t.Errorf("keygen reported %q, want %q", report, wantReport)
			}

			sealed := sealWithAge(t, folder, recipient, []byte("quarterly report"))
			if _, err := decrypt([]string{privatePath, sealed}); err != nil {
				t.Fatal(err)
			}
			if got := readFile(t, strings.TrimSuffix(sealed, ".age")); got != "quarterly report" {
				t.Fatalf("decrypted %q", got)
			}
		})
	}
}

func TestKeygen_RefusesInvalidNames(t *testing.T) {
	for _, name := range []string{"", "-lead", "has space", strings.Repeat("a", 41)} {
		t.Run("name "+name+" returns ErrInvalidName", func(t *testing.T) {
			folder := t.TempDir()
			if _, err := keygen([]string{"--name", name}, folder); !errors.Is(err, vita.ErrInvalidName) {
				t.Errorf("name %q: %v", name, err)
			}
			assertFolderEmpty(t, folder)
		})
	}
}

func TestKeygen_NeverOverwrites(t *testing.T) {
	folder := t.TempDir()
	if _, err := keygen([]string{"--name", "Ops"}, folder); err != nil {
		t.Fatal(err)
	}
	privatePath := filepath.Join(folder, "Ops-private-key.txt")
	before := readFile(t, privatePath)
	if _, err := keygen([]string{"--name", "Ops"}, folder); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("a second keygen returned %v, want fs.ErrExist", err)
	}
	if after := readFile(t, privatePath); after != before {
		t.Fatal("key file changed")
	}
}

func TestDecrypt_Refuses(t *testing.T) {
	tests := []struct {
		name string
		// arguments gives decrypt's arguments, from the private key paths and the sealed file.
		arguments func(right, wrong, sealed string) []string
		// prepare changes the fixture before decrypt runs.
		prepare func(t *testing.T, sealed string)
		wantErr error
		// wantOutput is what the file next to the sealed one must hold afterwards; empty means it must not exist.
		wantOutput string
	}{
		{
			name:      "wrong key returns ErrCannotDecrypt",
			arguments: func(_, wrong, sealed string) []string { return []string{wrong, sealed} },
			wantErr:   vita.ErrCannotDecrypt,
		},
		{
			name:      "tampered file returns ErrDamaged and leaves no plaintext",
			arguments: func(right, _, sealed string) []string { return []string{right, sealed} },
			prepare: func(t *testing.T, sealed string) {
				t.Helper()
				data := []byte(readFile(t, sealed))
				data[len(data)-10] ^= 0xFF
				writeFile(t, sealed, string(data))
			},
			wantErr: vita.ErrDamaged,
		},
		{
			name:      "existing output returns fs.ErrExist and is kept",
			arguments: func(right, _, sealed string) []string { return []string{right, sealed} },
			prepare: func(t *testing.T, sealed string) {
				t.Helper()
				writeFile(t, strings.TrimSuffix(sealed, ".age"), "keep me")
			},
			wantErr:    fs.ErrExist,
			wantOutput: "keep me",
		},
		{
			name: "file without .age returns ErrNotSealedName",
			arguments: func(right, _, sealed string) []string {
				return []string{right, strings.TrimSuffix(sealed, ".age")}
			},
			wantErr: vita.ErrNotSealedName,
		},
		{
			name:      "missing key file returns fs.ErrNotExist",
			arguments: func(right, _, sealed string) []string { return []string{right + ".missing", sealed} },
			wantErr:   fs.ErrNotExist,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			folder := t.TempDir()
			right, wrong, sealed := createRightAndWrongKeys(t, folder)
			if test.prepare != nil {
				test.prepare(t, sealed)
			}

			report, err := decrypt(test.arguments(right, wrong, sealed))
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("got %v, want %v", err, test.wantErr)
			}
			if report != "" {
				t.Errorf("a failed decrypt reported %q", report)
			}
			assertNoPlaintextLeft(t, folder, strings.TrimSuffix(sealed, ".age"), test.wantOutput)
		})
	}
}

// createRightAndWrongKeys makes the key pairs Right and Wrong in folder, and a file sealed to Right. It returns the
// paths of the two private key files and of the sealed file.
func createRightAndWrongKeys(t *testing.T, folder string) (right, wrong, sealed string) {
	t.Helper()
	if _, err := keygen([]string{"--name", "Right"}, folder); err != nil {
		t.Fatal(err)
	}
	if _, err := keygen([]string{"--name", "Wrong", "--classic"}, folder); err != nil {
		t.Fatal(err)
	}
	recipient, err := vita.ReadPublicKey(filepath.Join(folder, "Right-public-key.txt"))
	if err != nil {
		t.Fatal(err)
	}
	// Large enough for several 64 KiB chunks, so damage lands after plaintext could have been written.
	sealed = sealWithAge(t, folder, recipient, bytes.Repeat([]byte("x"), 300*1024))
	return filepath.Join(folder, "Right-private-key.txt"), filepath.Join(folder, "Wrong-private-key.txt"), sealed
}

func TestSealThenDecrypt(t *testing.T) {
	tests := []struct {
		name string
		// publicKey gives the PUBLIC_KEY argument from the public key file's path and the key it holds.
		publicKey func(path, key string) string
	}{
		{"public key file seals", func(path, _ string) string { return path }},
		{"public key itself seals", func(_, key string) string { return key }},
		{"public key in capitals with spaces seals", func(_, key string) string { return " " + strings.ToUpper(key) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			folder := t.TempDir()
			if _, err := keygen([]string{"--name", "Test"}, folder); err != nil {
				t.Fatal(err)
			}
			publicPath := filepath.Join(folder, "Test-public-key.txt")
			key, err := vita.ReadPublicKey(publicPath)
			if err != nil {
				t.Fatal(err)
			}
			input := filepath.Join(folder, "notes.txt")
			writeFile(t, input, "test note")

			report, err := seal([]string{test.publicKey(publicPath, key), input})
			if err != nil {
				t.Fatal(err)
			}
			wantReport := "Sealed to " + input + ".age (public key fingerprint " + vita.Fingerprint(key) + ")\n"
			if report != wantReport {
				t.Errorf("seal reported %q, want %q", report, wantReport)
			}
			removeFile(t, input)
			assertDecryptsToNote(t, filepath.Join(folder, "Test-private-key.txt"), input)
		})
	}
}

// assertDecryptsToNote decrypts input.age with the private key file and checks it gives back the test note.
func assertDecryptsToNote(t *testing.T, keyPath, input string) {
	t.Helper()
	report, err := decrypt([]string{keyPath, input + ".age"})
	if err != nil {
		t.Fatal(err)
	}
	if report != "Decrypted to "+input+"\n" {
		t.Errorf("decrypt reported %q", report)
	}
	if got := readFile(t, input); got != "test note" {
		t.Fatalf("decrypted %q", got)
	}
}

func TestSeal_RefusesPrivateKeyFile(t *testing.T) {
	folder := t.TempDir()
	if _, err := keygen([]string{"--name", "Test", "--classic"}, folder); err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(folder, "notes.txt")
	writeFile(t, input, "test note")
	_, err := seal([]string{filepath.Join(folder, "Test-private-key.txt"), input})
	if !errors.Is(err, vita.ErrPrivateKeyFile) {
		t.Errorf("sealing to a private key file returned %v", err)
	}
	if _, err := os.Stat(input + ".age"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a sealed file was written: %v", err)
	}
}

// TestNoNetworkPackages keeps the tool offline: no package in its build graph may be a network package.
func TestNoNetworkPackages(t *testing.T) {
	output, err := exec.CommandContext(t.Context(), "go", "list", "-deps", ".").Output()
	if err != nil {
		t.Skipf("go list unavailable: %v", err)
	}
	for _, packagePath := range strings.Fields(string(output)) {
		isNetwork := packagePath == "net" || strings.HasPrefix(packagePath, "net/") ||
			strings.HasPrefix(packagePath, "crypto/tls")
		if isNetwork {
			t.Errorf("network package %s is in the build graph", packagePath)
		}
	}
}

// publicKeyFrom finds the public key in keygen's report.
func publicKeyFrom(t *testing.T, report string) string {
	t.Helper()
	for line := range strings.SplitSeq(report, "\n") {
		if strings.HasPrefix(line, "age1") {
			return strings.TrimSpace(line)
		}
	}
	t.Fatal("no public key in keygen's report")
	return ""
}

// sealWithAge seals plaintext to recipient with the age library itself, as report.pdf.age in folder.
func sealWithAge(t *testing.T, folder, recipient string, plaintext []byte) string {
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
	path := filepath.Join(folder, "report.pdf.age")
	writeFile(t, path, sealed.String())
	return path
}

// assertPrivateKeyMode checks that only the owner can read the private key file at path.
func assertPrivateKeyMode(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	// Windows has no Unix modes: Go reports 0666 for any writable file there, and the file gets its folder's
	// permissions instead.
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Errorf("key file mode %v, want 0600", info.Mode().Perm())
	}
}

// assertNoPlaintextLeft checks that no temporary file is left in folder, and that output holds wantOutput, or doesn't
// exist when wantOutput is empty.
func assertNoPlaintextLeft(t *testing.T, folder, output, wantOutput string) {
	t.Helper()
	got, err := os.ReadFile(output)
	switch {
	case wantOutput == "" && err == nil:
		t.Error("partial output was left behind")
	case wantOutput != "" && string(got) != wantOutput:
		t.Errorf("output holds %q, %v; want %q", got, err, wantOutput)
	}
	entries, err := os.ReadDir(folder)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".vita-decrypt-") {
			t.Errorf("temporary file %s was left behind", entry.Name())
		}
	}
}

func assertFolderEmpty(t *testing.T, folder string) {
	t.Helper()
	entries, err := os.ReadDir(folder)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		t.Errorf("%s was written", entry.Name())
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

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func removeFile(t *testing.T, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
}
