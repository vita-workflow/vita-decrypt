package vita

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"filippo.io/age"
)

func TestCreateKeyPair(t *testing.T) {
	tests := []struct {
		name           string
		isClassic      bool
		wantPublicHead string
		wantSecretHead string
	}{
		{"hybrid key pair decrypts what age sealed", false, "age1pq1", "AGE-SECRET-KEY-PQ-1"},
		{"classic key pair decrypts what age sealed", true, "age1", "AGE-SECRET-KEY-1"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			folder := t.TempDir()
			keyPair, err := CreateKeyPair(folder, "Family-Only", test.isClassic)
			if err != nil {
				t.Fatal(err)
			}
			if keyPair.Name != "Family-Only" ||
				keyPair.PrivateKeyPath != filepath.Join(folder, "Family-Only-private-key.txt") ||
				keyPair.PublicKeyPath != filepath.Join(folder, "Family-Only-public-key.txt") {
				t.Errorf("key pair %+v", keyPair)
			}
			if !strings.HasPrefix(keyPair.PublicKey, test.wantPublicHead) ||
				strings.HasPrefix(keyPair.PublicKey, "age1pq1") == test.isClassic {
				t.Errorf("public key %.12s…", keyPair.PublicKey)
			}
			assertKeyFiles(t, keyPair, test.wantSecretHead)
			assertDecryptsWhatAgeSealed(t, folder, keyPair)
		})
	}
}

// assertDecryptsWhatAgeSealed reads the key pair's private key file back and decrypts a file age sealed to its
// public key, checking the name read and the progress reported.
func assertDecryptsWhatAgeSealed(t *testing.T, folder string, keyPair KeyPair) {
	t.Helper()
	privateKey, err := ReadPrivateKey(keyPair.PrivateKeyPath)
	if err != nil {
		t.Fatal(err)
	}
	if privateKey.Name != keyPair.Name {
		t.Errorf("read name %q", privateKey.Name)
	}
	sealed := sealWithAge(t, folder, "report.pdf.age", keyPair.PublicKey, []byte("quarterly report"))
	var bytesRead int64
	output, err := Decrypt(t.Context(), privateKey.Identities, sealed, func(read int64) { bytesRead = read })
	if err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, output); got != "quarterly report" {
		t.Fatalf("decrypted %q", got)
	}
	if info, err := os.Stat(sealed); err != nil || bytesRead != info.Size() {
		t.Errorf("progress reached %d bytes; the sealed file: %v, %v", bytesRead, info, err)
	}
}

// assertKeyFiles checks the contents and modes of a key pair's two files.
func assertKeyFiles(t *testing.T, keyPair KeyPair, wantSecretHead string) {
	t.Helper()
	privateLines := strings.Split(readFile(t, keyPair.PrivateKeyPath), "\n")
	if len(privateLines) != 4 || privateLines[0] != "# Vita key pair: "+keyPair.Name ||
		privateLines[1] != "# public key: "+keyPair.PublicKey ||
		!strings.HasPrefix(privateLines[2], wantSecretHead) || privateLines[3] != "" {
		t.Errorf("the private key file holds %d lines, starting %q", len(privateLines), privateLines[0])
	}
	wantPublic := "# Vita key pair: " + keyPair.Name + "\n# fingerprint: " + Fingerprint(keyPair.PublicKey) + "\n" +
		keyPair.PublicKey + "\n"
	if got := readFile(t, keyPair.PublicKeyPath); got != wantPublic {
		t.Errorf("the public key file holds %q, want %q", got, wantPublic)
	}
	if publicKey, err := ReadPublicKey(keyPair.PublicKeyPath); err != nil || publicKey != keyPair.PublicKey {
		t.Errorf("the public key file reads as %q, %v", publicKey, err)
	}

	// Windows has no Unix modes: Go reports 0666 for any writable file there, and the file gets its folder's
	// permissions instead.
	if runtime.GOOS == "windows" {
		return
	}
	if info, err := os.Stat(keyPair.PrivateKeyPath); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("private key file: %v, %v; want mode 0600", info, err)
	}
	// The umask can take permissions away from 0644, but never add them.
	if info, err := os.Stat(keyPair.PublicKeyPath); err != nil || info.Mode().Perm()&^0o644 != 0 {
		t.Errorf("public key file: %v, %v; want mode 0644 or less", info, err)
	}
}

func TestCreateKeyPair_RefusesInvalidNames(t *testing.T) {
	for _, name := range []string{"", "-lead", "has space", "dot.dot", "slash/name", strings.Repeat("a", 41)} {
		t.Run("name "+name+" returns ErrInvalidName", func(t *testing.T) {
			folder := t.TempDir()
			if _, err := CreateKeyPair(folder, name, true); !errors.Is(err, ErrInvalidName) {
				t.Errorf("name %q: %v", name, err)
			}
			assertFolderHolds(t, folder)
		})
	}
}

func TestCreateKeyPair_NeverOverwrites(t *testing.T) {
	tests := []struct {
		name         string
		existingFile string
	}{
		{"existing private key file returns fs.ErrExist and is kept", "Ops-private-key.txt"},
		{"existing public key file returns fs.ErrExist and is kept", "Ops-public-key.txt"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			folder := t.TempDir()
			writeFile(t, filepath.Join(folder, test.existingFile), "keep me")
			if _, err := CreateKeyPair(folder, "Ops", false); !errors.Is(err, fs.ErrExist) {
				t.Fatalf("got %v, want fs.ErrExist", err)
			}
			if got := readFile(t, filepath.Join(folder, test.existingFile)); got != "keep me" {
				t.Error("the existing file changed")
			}
			// Neither file of the key pair may be left without the other.
			assertFolderHolds(t, folder, test.existingFile)
		})
	}
}

func TestCreateKeyPair_MissingFolderFails(t *testing.T) {
	folder := filepath.Join(t.TempDir(), "missing")
	if _, err := CreateKeyPair(folder, "Ops", true); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("got %v, want fs.ErrNotExist", err)
	}
}

func TestCreateKeyPair_ReadOnlyFolderFails(t *testing.T) {
	folder := t.TempDir()
	makeReadOnly(t, folder)
	if _, err := CreateKeyPair(folder, "Ops", true); !errors.Is(err, fs.ErrPermission) {
		t.Errorf("got %v, want fs.ErrPermission", err)
	}
	assertFolderHolds(t, folder)
}

func TestIsValidName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		isValid bool
	}{
		{"one letter is valid", "a", true},
		{"one digit is valid", "7", true},
		{"letters, digits and hyphens are valid", "Family-Only-2", true},
		{"40 characters are valid", strings.Repeat("a", 40), true},
		{"empty is invalid", "", false},
		{"41 characters are invalid", strings.Repeat("a", 41), false},
		{"leading hyphen is invalid", "-lead", false},
		{"space is invalid", "has space", false},
		{"dot is invalid", "dot.dot", false},
		{"underscore is invalid", "snake_case", false},
		{"non-ASCII letter is invalid", "café", false},
		{"trailing newline is invalid", "Ops\n", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := IsValidName(test.input); got != test.isValid {
				t.Errorf("IsValidName(%q) = %v", test.input, got)
			}
		})
	}
}

func TestPrivateKeyFileName(t *testing.T) {
	if got := PrivateKeyFileName("Ops"); got != "Ops-private-key.txt" {
		t.Errorf("PrivateKeyFileName: %q", got)
	}
	if got := PublicKeyFileName("Ops"); got != "Ops-public-key.txt" {
		t.Errorf("PublicKeyFileName: %q", got)
	}
}

func TestReadPrivateKey(t *testing.T) {
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	secret := identity.String() + "\n"
	tests := []struct {
		name     string
		fileName string
		content  string
		wantName string
		wantErr  bool
	}{
		{"name comment names the key pair", "key.txt", "# Vita key pair: Ops\n" + secret, "Ops", false},
		{
			"name comment with a Windows line ending names the key pair", "key.txt",
			"# Vita key pair: Ops\r\n" + secret, "Ops", false,
		},
		{
			"name comment wins over the file name", "Dev-private-key.txt",
			"# Vita key pair: Ops\n" + secret, "Ops", false,
		},
		{
			"age-keygen file named like ours takes the file name", "Shared-private-key.txt",
			"# created: now\n" + secret, "Shared", false,
		},
		{
			"invalid name comment falls back to the file name", "Shared-private-key.txt",
			"# Vita key pair: a b\n" + secret, "Shared", false,
		},
		{"invalid name in the file name gives no name", "a.b-private-key.txt", secret, "", false},
		{"file without a name gives no name", "key.txt", secret, "", false},
		{"file without a key fails", "notes-private-key.txt", "shopping list\n", "", true},
		{"empty file fails", "key.txt", "", "", true},
		{"public key file fails", "key.txt", identity.Recipient().String() + "\n", "", true},
		{
			"file over the size limit fails", "big.txt",
			"# " + strings.Repeat("x", maxKeyFileBytes) + "\n" + secret, "", true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), test.fileName)
			writeFile(t, path, test.content)
			privateKey, err := ReadPrivateKey(path)
			if (err != nil) != test.wantErr || privateKey.Name != test.wantName {
				t.Fatalf("got %q, %v", privateKey.Name, err)
			}
			if err != nil && strings.Contains(err.Error(), "AGE-SECRET-KEY") {
				t.Errorf("the error shows the private key: %v", err)
			}
			if err == nil && (len(privateKey.Identities) != 1 || privateKey.Identities[0] == nil) {
				t.Errorf("read %d identities", len(privateKey.Identities))
			}
		})
	}
}

func TestReadPrivateKey_UnreadableFileFails(t *testing.T) {
	folder := t.TempDir()
	if _, err := ReadPrivateKey(filepath.Join(folder, "missing.txt")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("missing file: %v", err)
	}
	if _, err := ReadPrivateKey(folder); err == nil {
		t.Error("a folder was read as a private key")
	}
}
