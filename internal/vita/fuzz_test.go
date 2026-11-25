package vita

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The seed corpus for each fuzz test is in testdata/fuzz/<test name>.

// fingerprintPattern is the shape of every fingerprint: four groups of four uppercase hex digits.
var fingerprintPattern = regexp.MustCompile(`^[0-9A-F]{4} [0-9A-F]{4} [0-9A-F]{4} [0-9A-F]{4}$`)

func FuzzOutputName(f *testing.F) {
	f.Fuzz(func(t *testing.T, input string) {
		output, err := DecryptedName(input)
		if err != nil {
			if !errors.Is(err, ErrNotSealedName) || output != "" {
				t.Fatalf("failure returned %q, %v", output, err)
			}
			return
		}
		if SealedName(output) != input || filepath.Base(input) == ".age" {
			t.Errorf("DecryptedName(%q) = %q", input, output)
		}
	})
}

func FuzzFingerprint(f *testing.F) {
	f.Fuzz(func(t *testing.T, recipient string) {
		fingerprint := Fingerprint(recipient)
		if !fingerprintPattern.MatchString(fingerprint) {
			t.Fatalf("Fingerprint(%q) = %q", recipient, fingerprint)
		}
		// Vita stores the canonical form, so it must fingerprint the same as what was typed or pasted.
		if again := Fingerprint(" \t" + canonicalRecipient(recipient) + "\r\n"); again != fingerprint {
			t.Errorf("the canonical form of %q fingerprints as %q, not %q", recipient, again, fingerprint)
		}
	})
}

func FuzzCheckRecipient(f *testing.F) {
	f.Fuzz(func(t *testing.T, recipient string) {
		parsed, err := parseRecipient(recipient)
		if (err == nil) != (CheckRecipient(recipient) == nil) {
			t.Fatalf("parseRecipient and CheckRecipient disagree about %q", recipient)
		}
		if err != nil {
			if parsed != nil || !strings.HasPrefix(err.Error(), "not a valid age public key: ") {
				t.Errorf("failure returned %v, %v", parsed, err)
			}
			return
		}
		// A valid public key has exactly one encoding, so encoding it again gives back its canonical form.
		encoded, ok := parsed.(fmt.Stringer)
		if !ok || encoded.String() != canonicalRecipient(recipient) {
			t.Errorf("%q parsed as %v", recipient, parsed)
		}
	})
}

func FuzzParsePublicKey(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		key, err := parsePublicKey(data)
		if err != nil {
			if key != "" {
				t.Fatalf("failure returned %q", key)
			}
			return
		}
		if CheckRecipient(key) != nil || key != strings.TrimSpace(key) || strings.HasPrefix(key, "#") {
			t.Errorf("returned %q", key)
		}
		if strings.Contains(strings.ToUpper(string(data)), "AGE-SECRET-KEY-") {
			t.Error("a private key file was read as a public key")
		}
	})
}

func FuzzParsePrivateKey(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte, fileName string) {
		privateKey, err := parsePrivateKey(data, fileName)
		if err != nil {
			if privateKey.Identities != nil || privateKey.Name != "" {
				t.Fatalf("failure returned %+v", privateKey)
			}
			return
		}
		if len(privateKey.Identities) == 0 {
			t.Error("no identities")
		}
		if privateKey.Name != "" && !IsValidName(privateKey.Name) {
			t.Errorf("invalid name %q", privateKey.Name)
		}
	})
}
