// Package vita holds what the vita-decrypt command and the Vita Decrypt app share: decrypting sealed files, sealing
// them, creating key pairs, and the fingerprints Vita shows for public keys.
package vita

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"

	"filippo.io/age"
)

// fingerprintBytes is how many bytes of the SHA-256 a fingerprint keeps.
const fingerprintBytes = 8

// canonicalRecipient is the form Vita fingerprints and stores: the recipient trimmed and lowercased.
func canonicalRecipient(recipient string) string {
	return strings.ToLower(strings.TrimSpace(recipient))
}

// Fingerprint returns the fingerprint Vita shows for a public key (an age recipient): the SHA-256 of the canonical
// recipient's UTF-8 bytes, first 8 bytes, as uppercase hex in four groups of four. Vita computes the same value;
// testdata/fingerprint-vectors.json holds the shared test vectors.
func Fingerprint(recipient string) string {
	sum := sha256.Sum256([]byte(canonicalRecipient(recipient)))
	digits := strings.ToUpper(hex.EncodeToString(sum[:fingerprintBytes]))
	return digits[0:4] + " " + digits[4:8] + " " + digits[8:12] + " " + digits[12:16]
}

// CheckRecipient reports whether recipient, once canonical, is a public key age can seal to: hybrid (age1pq1…) or
// X25519 (age1…).
func CheckRecipient(recipient string) error {
	_, err := parseRecipient(recipient)
	return err
}

// parseRecipient parses recipient, once canonical, as a hybrid or X25519 public key.
func parseRecipient(recipient string) (age.Recipient, error) {
	canonical := canonicalRecipient(recipient)
	var parsed age.Recipient
	var err error
	switch {
	case strings.HasPrefix(canonical, "age1pq1"):
		parsed, err = age.ParseHybridRecipient(canonical)
	case strings.HasPrefix(canonical, "age1"):
		parsed, err = age.ParseX25519Recipient(canonical)
	default:
		err = errors.New("it should start with age1")
	}
	if err != nil {
		return nil, errors.New("not a valid age public key: " + err.Error())
	}
	return parsed, nil
}

// ErrPrivateKeyFile reports a private key file given where a public key file was expected.
var ErrPrivateKeyFile = errors.New("this is a private key file; choose the public key file instead")

// ReadPublicKey reads the public key from a public key file: one age recipient, with any number of blank lines and
// lines starting with #, as CreateKeyPair writes and age -R reads.
func ReadPublicKey(path string) (string, error) {
	data, err := readKeyFile(path)
	if err != nil {
		return "", err
	}
	return parsePublicKey(data)
}

// parsePublicKey parses the contents of a public key file.
func parsePublicKey(data []byte) (string, error) {
	if len(data) > maxKeyFileBytes {
		return "", errors.New("the file is too large to be a public key file")
	}
	if strings.Contains(strings.ToUpper(string(data)), "AGE-SECRET-KEY-") {
		return "", ErrPrivateKeyFile
	}
	var keys []string
	for line := range strings.SplitSeq(string(data), "\n") {
		if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
			keys = append(keys, line)
		}
	}
	switch {
	case len(keys) == 0:
		return "", errors.New("the file holds no public key")
	case len(keys) > 1:
		return "", errors.New("the file holds more than one public key")
	}
	if err := CheckRecipient(keys[0]); err != nil {
		return "", err
	}
	return keys[0], nil
}
