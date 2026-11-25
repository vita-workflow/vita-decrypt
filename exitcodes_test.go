package main

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/vita-workflow/vita-decrypt/internal/vita"
)

// The placeholders a test case's arguments and wanted output may use, replaced by the fixture's values.
const (
	publicKeyPlaceholder   = "<public key>"
	fingerprintPlaceholder = "<fingerprint>"
)

// TestRun_ExitCodes runs every command through run, as main does, and checks each exit code: 0 with the report on
// stdout, 1 with "error:" on stderr, and 2 with the usage text on stderr.
func TestRun_ExitCodes(t *testing.T) {
	tests := []struct {
		name      string
		arguments []string
		wantCode  int
		// wantStdoutPrefix is how stdout must start for exit code 0.
		wantStdoutPrefix string
		// wantStderr is what stderr must contain for exit code 1.
		wantStderr string
		// wantFile, if not empty, must exist afterwards.
		wantFile string
	}{
		{
			name: "keygen creates a key pair and exits 0", arguments: []string{"keygen", "--name", "Ops"},
			wantCode: exitSuccess, wantStdoutPrefix: "Private key written to Ops-private-key.txt. Keep it safe",
			wantFile: "Ops-public-key.txt",
		},
		{
			name: "classic keygen exits 0", arguments: []string{"keygen", "--classic", "--name", "Ops"},
			wantCode: exitSuccess, wantStdoutPrefix: "Private key written to Ops-private-key.txt.",
			wantFile: "Ops-private-key.txt",
		},
		{
			name: "decrypt exits 0", arguments: []string{"decrypt", "Test-private-key.txt", "report.pdf.age"},
			wantCode: exitSuccess, wantStdoutPrefix: "Decrypted to report.pdf\n", wantFile: "report.pdf",
		},
		{
			name: "seal to a public key file exits 0", arguments: []string{"seal", "Test-public-key.txt", "letter.txt"},
			wantCode:         exitSuccess,
			wantStdoutPrefix: "Sealed to letter.txt.age (public key fingerprint " + fingerprintPlaceholder + ")\n",
			wantFile:         "letter.txt.age",
		},
		{
			name:             "seal to the public key itself exits 0",
			arguments:        []string{"seal", publicKeyPlaceholder, "letter.txt"},
			wantCode:         exitSuccess,
			wantStdoutPrefix: "Sealed to letter.txt.age (public key fingerprint " + fingerprintPlaceholder + ")\n",
			wantFile:         "letter.txt.age",
		},
		{
			name:      "fingerprint of a public key exits 0",
			arguments: []string{"fingerprint", "age1lvyvwawkr0mcnnnncaghunadrqkmuf9e6507x9y920xxpp866cnql7dp2z"},
			wantCode:  exitSuccess, wantStdoutPrefix: "CE94 50DF DFF5 9E9D\n",
		},
		{
			name: "fingerprint of a public key file exits 0", arguments: []string{"fingerprint", "Test-public-key.txt"},
			wantCode: exitSuccess, wantStdoutPrefix: fingerprintPlaceholder + "\n",
		},

		{name: "no arguments exits 2", arguments: nil, wantCode: exitUsage},
		{name: "unknown command exits 2", arguments: []string{"bogus"}, wantCode: exitUsage},
		{name: "keygen without a name value exits 2", arguments: []string{"keygen", "--name"}, wantCode: exitUsage},
		{name: "keygen with an unknown option exits 2", arguments: []string{"keygen", "--bogus"}, wantCode: exitUsage},
		{name: "keygen with a bare name exits 2", arguments: []string{"keygen", "Ops"}, wantCode: exitUsage},
		{
			name:      "decrypt with one argument exits 2",
			arguments: []string{"decrypt", "report.pdf.age"}, wantCode: exitUsage,
		},
		{
			name:      "decrypt with three arguments exits 2",
			arguments: []string{"decrypt", "a", "b", "c"}, wantCode: exitUsage,
		},
		{name: "seal without a file exits 2", arguments: []string{"seal", "age1notarealkey"}, wantCode: exitUsage},
		{name: "fingerprint without a key exits 2", arguments: []string{"fingerprint"}, wantCode: exitUsage},
		{name: "fingerprint with two keys exits 2", arguments: []string{"fingerprint", "a", "b"}, wantCode: exitUsage},

		{
			name: "keygen with an invalid name exits 1", arguments: []string{"keygen", "--name", "has space"},
			wantCode: exitFailure, wantStderr: vita.ErrInvalidName.Error(),
		},
		{
			name: "keygen over an existing key pair exits 1", arguments: []string{"keygen", "--name", "Test"},
			wantCode: exitFailure, wantStderr: "won't overwrite",
		},
		{
			name:      "decrypt of a file without .age exits 1",
			arguments: []string{"decrypt", "Test-private-key.txt", "letter.txt"},
			wantCode:  exitFailure, wantStderr: "error: expected a file ending in .age\n",
		},
		{
			name:      "decrypt with a missing key file exits 1",
			arguments: []string{"decrypt", "missing.txt", "report.pdf.age"},
			wantCode:  exitFailure, wantStderr: "missing.txt",
		},
		{
			name:      "decrypt with the wrong key exits 1",
			arguments: []string{"decrypt", "Other-private-key.txt", "report.pdf.age"},
			wantCode:  exitFailure, wantStderr: "error: this key can't open the file",
		},
		{
			name:      "decrypt over an existing file exits 1",
			arguments: []string{"decrypt", "Test-private-key.txt", "notes.txt.age"},
			wantCode:  exitFailure, wantStderr: "won't overwrite existing notes.txt",
		},
		{
			name: "seal to an invalid public key exits 1", arguments: []string{"seal", "age1notarealkey", "letter.txt"},
			wantCode: exitFailure, wantStderr: "error: not a valid age public key",
		},
		{
			name:       "seal to a private key file exits 1",
			arguments:  []string{"seal", "Test-private-key.txt", "letter.txt"},
			wantCode:   exitFailure,
			wantStderr: "error: reading the public key from Test-private-key.txt: " + vita.ErrPrivateKeyFile.Error(),
		},
		{
			name: "seal of a missing file exits 1", arguments: []string{"seal", "Test-public-key.txt", "missing.txt"},
			wantCode: exitFailure, wantStderr: "missing.txt",
		},
		{
			name:      "seal over an existing sealed file exits 1",
			arguments: []string{"seal", "Test-public-key.txt", "notes.txt"},
			wantCode:  exitFailure, wantStderr: "won't overwrite existing notes.txt.age",
		},
		{
			name: "fingerprint of an invalid public key exits 1", arguments: []string{"fingerprint", "age1notarealkey"},
			wantCode: exitFailure, wantStderr: "error: not a valid age public key",
		},
		{
			name: "fingerprint of a missing file exits 1", arguments: []string{"fingerprint", "missing.txt"},
			wantCode: exitFailure, wantStderr: "error: reading the public key from missing.txt: ",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			publicKey := createCommandFixture(t)
			placeholders := strings.NewReplacer(
				publicKeyPlaceholder, publicKey, fingerprintPlaceholder, vita.Fingerprint(publicKey))
			arguments := make([]string, len(test.arguments))
			for index, argument := range test.arguments {
				arguments[index] = placeholders.Replace(argument)
			}

			var stdout, stderr bytes.Buffer
			if code := run(arguments, &stdout, &stderr); code != test.wantCode {
				t.Fatalf("exit code %d, want %d; stderr %q", code, test.wantCode, stderr.String())
			}
			assertCommandOutput(t, test.wantCode, stdout.String(), stderr.String(),
				placeholders.Replace(test.wantStdoutPrefix), test.wantStderr)
			if test.wantFile != "" {
				if _, err := os.Stat(test.wantFile); err != nil {
					t.Errorf("%s wasn't written: %v", test.wantFile, err)
				}
			}
		})
	}
}

// assertCommandOutput checks that only the stream wantCode calls for was written, and that it holds what's wanted.
func assertCommandOutput(t *testing.T, wantCode int, stdout, stderr, wantStdoutPrefix, wantStderr string) {
	t.Helper()
	switch wantCode {
	case exitSuccess:
		if !strings.HasPrefix(stdout, wantStdoutPrefix) || stderr != "" {
			t.Errorf("stdout %q, stderr %q; want stdout starting %q", stdout, stderr, wantStdoutPrefix)
		}
	case exitFailure:
		if stdout != "" || !strings.HasPrefix(stderr, "error: ") || !strings.Contains(stderr, wantStderr) {
			t.Errorf("stdout %q, stderr %q; want an error containing %q", stdout, stderr, wantStderr)
		}
	case exitUsage:
		if stdout != "" || stderr != usageText {
			t.Errorf("stdout %q, stderr %q; want the usage text", stdout, stderr)
		}
	}
}

// createCommandFixture fills the current folder with what the commands work on, and returns the public key of the
// Test key pair:
//   - the key pairs Test and Other;
//   - report.pdf.age, sealed to Test, with no report.pdf next to it;
//   - notes.txt and notes.txt.age, sealed to Test, so neither decrypting nor sealing it again can write anything;
//   - letter.txt, not sealed.
func createCommandFixture(t *testing.T) string {
	t.Helper()
	keyPair, err := vita.CreateKeyPair(".", "Test", true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := vita.CreateKeyPair(".", "Other", true); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"report.pdf", "notes.txt", "letter.txt"} {
		writeFile(t, name, "contents of "+name)
	}
	for _, name := range []string{"report.pdf", "notes.txt"} {
		if _, err := vita.Seal(context.Background(), keyPair.PublicKey, name, nil); err != nil {
			t.Fatal(err)
		}
	}
	removeFile(t, "report.pdf")
	return keyPair.PublicKey
}
