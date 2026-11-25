package vita

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"filippo.io/age"
)

// namePattern matches Vita's key pair name rule: 1–40 ASCII letters, digits, and hyphens, starting with a letter or
// digit.
var namePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]{0,39}$`)

// ErrInvalidName reports a key pair name that breaks Vita's rule.
var ErrInvalidName = errors.New("name must be 1-40 letters, digits or hyphens, starting with a letter or digit")

// IsValidName reports whether name follows Vita's key pair name rule.
func IsValidName(name string) bool { return namePattern.MatchString(name) }

// privateKeyFileSuffix ends every private key file this tool writes, and publicKeyFileSuffix every public key file.
const (
	privateKeyFileSuffix = "-private-key.txt"
	publicKeyFileSuffix  = "-public-key.txt"
)

// The modes key files are created with. Only the owner may read a private key; a public key is meant to be shared.
const (
	privateKeyFileMode = 0o600
	publicKeyFileMode  = 0o644
)

// PrivateKeyFileName is the file a key pair's private key is saved in.
func PrivateKeyFileName(name string) string { return name + privateKeyFileSuffix }

// PublicKeyFileName is the file a key pair's public key is saved in.
func PublicKeyFileName(name string) string { return name + publicKeyFileSuffix }

// KeyPair is a newly created key pair.
type KeyPair struct {
	Name           string
	PrivateKeyPath string // the private key file
	PublicKeyPath  string // the public key file
	PublicKey      string
}

// CreateKeyPair creates a key pair in folder: hybrid post-quantum (ML-KEM-768 + X25519) unless isClassic, then X25519.
// The private key goes to NAME-private-key.txt with mode 0600, and the public key to NAME-public-key.txt, which age -R
// also reads. Neither file is ever overwritten: if either exists, nothing is written.
func CreateKeyPair(folder, name string, isClassic bool) (KeyPair, error) {
	if !IsValidName(name) {
		return KeyPair{}, ErrInvalidName
	}
	identity, recipient, err := generateKeys(isClassic)
	if err != nil {
		return KeyPair{}, err
	}

	privatePath := filepath.Join(folder, PrivateKeyFileName(name))
	publicPath := filepath.Join(folder, PublicKeyFileName(name))
	if _, err := os.Lstat(publicPath); err == nil {
		return KeyPair{}, fmt.Errorf("won't overwrite %s: %w", publicPath, fs.ErrExist)
	}
	if err := writeNew(privatePath, privateKeyFileMode, privateKeyFileContent(name, identity, recipient)); err != nil {
		return KeyPair{}, err
	}
	if err := writeNew(publicPath, publicKeyFileMode, publicKeyFileContent(name, recipient)); err != nil {
		// Nobody has seen this private key yet, so removing it leaves no half-made key pair behind.
		removeQuietly(privatePath)
		return KeyPair{}, err
	}
	return KeyPair{Name: name, PrivateKeyPath: privatePath, PublicKeyPath: publicPath, PublicKey: recipient}, nil
}

// generateKeys creates a new identity, hybrid unless isClassic, and returns it with its recipient, both encoded.
func generateKeys(isClassic bool) (identity, recipient string, err error) {
	if isClassic {
		classicIdentity, err := age.GenerateX25519Identity()
		if err != nil {
			return "", "", err
		}
		return classicIdentity.String(), classicIdentity.Recipient().String(), nil
	}
	hybridIdentity, err := age.GenerateHybridIdentity()
	if err != nil {
		return "", "", err
	}
	return hybridIdentity.String(), hybridIdentity.Recipient().String(), nil
}

// privateKeyFileContent is what a private key file holds: the key pair's name, its public key as a comment, and the
// identity.
func privateKeyFileContent(name, identity, recipient string) string {
	return fmt.Sprintf("%s%s\n# public key: %s\n%s\n", nameComment, name, recipient, identity)
}

// publicKeyFileContent is what a public key file holds: comments and the recipient, the format age -R reads.
func publicKeyFileContent(name, recipient string) string {
	return fmt.Sprintf("%s%s\n# fingerprint: %s\n%s\n", nameComment, name, Fingerprint(recipient), recipient)
}

// writeNew creates path holding content. It fails with fs.ErrExist rather than replace a file, and removes a file it
// couldn't finish writing.
func writeNew(path string, mode os.FileMode, content string) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("won't overwrite %s: %w", path, err)
	}
	if err != nil {
		return err
	}
	if _, err := file.WriteString(content); err != nil {
		closeQuietly(file)
		removeQuietly(path)
		return err
	}
	if err := file.Close(); err != nil {
		removeQuietly(path)
		return err
	}
	return nil
}

// nameComment starts the line of a key file that names its key pair.
const nameComment = "# Vita key pair: "

// PrivateKey is a private key file that has been read.
type PrivateKey struct {
	Identities []age.Identity
	// Name is the key pair's name: from the file's "# Vita key pair:" line, else from a NAME-private-key.txt file
	// name, else empty.
	Name string
}

// maxKeyFileBytes is the largest key file ReadPrivateKey and ReadPublicKey read. A key file is a few hundred bytes (a
// hybrid public key is about 2 KB); the limit keeps a large file chosen by mistake from being read into memory.
const maxKeyFileBytes = 1 << 20

// ReadPrivateKey reads the identities in a private key file.
func ReadPrivateKey(path string) (PrivateKey, error) {
	data, err := readKeyFile(path)
	if err != nil {
		return PrivateKey{}, err
	}
	return parsePrivateKey(data, filepath.Base(path))
}

// parsePrivateKey parses the contents of a private key file called fileName.
func parsePrivateKey(data []byte, fileName string) (PrivateKey, error) {
	if len(data) > maxKeyFileBytes {
		return PrivateKey{}, errors.New("reading the private key: the file is too large to be a private key file")
	}
	identities, err := age.ParseIdentities(strings.NewReader(string(data)))
	if err != nil {
		return PrivateKey{}, fmt.Errorf("reading the private key: %w", err)
	}
	return PrivateKey{Identities: identities, Name: keyPairName(string(data), fileName)}, nil
}

// keyPairName is the name of the key pair in a private key file: from its first valid "# Vita key pair:" line, else
// from a NAME-private-key.txt file name, else empty.
func keyPairName(content, fileName string) string {
	for line := range strings.SplitSeq(content, "\n") {
		if name, ok := strings.CutPrefix(strings.TrimSpace(line), nameComment); ok && IsValidName(name) {
			return name
		}
	}
	if name, ok := strings.CutSuffix(fileName, privateKeyFileSuffix); ok && IsValidName(name) {
		return name
	}
	return ""
}

// readKeyFile reads a key file whole, but no more than one byte over maxKeyFileBytes, which is enough for the caller
// to tell that the file is too large.
func readKeyFile(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer closeQuietly(file)
	return io.ReadAll(io.LimitReader(file, maxKeyFileBytes+1))
}
