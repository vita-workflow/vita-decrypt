// Command vita-decrypt opens files that Vita sealed with age, and creates the key pairs they are sealed to. It can also
// seal a file the way Vita does, to test a key pair.
//
// It works offline: it imports no network package (TestNoNetworkPackages enforces this), so a private key never
// leaves the machine it runs on.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	"github.com/vita-workflow/vita-decrypt/internal/vita"
)

const usageText = `usage:
  vita-decrypt keygen --name NAME [--classic]   create a key pair; writes NAME-private-key.txt and
                                                 NAME-public-key.txt
  vita-decrypt decrypt KEYFILE FILE.age          decrypt FILE.age next to it, without the .age suffix
  vita-decrypt seal PUBLIC_KEY FILE              seal FILE to a public key, as Vita does; writes FILE.age
  vita-decrypt fingerprint PUBLIC_KEY            print the fingerprint Vita shows for a public key

PUBLIC_KEY is the key itself (age1...) or a public key file.
`

// Exit codes, documented in the README.
const (
	exitSuccess = 0
	exitFailure = 1
	exitUsage   = 2
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run executes one command, prints what it reports, and returns the process exit code.
func run(arguments []string, stdout, stderr io.Writer) int {
	report, err := runCommand(arguments)
	// The exit code reports the command's outcome; a terminal that can't be written to has nowhere to report its own
	// failure, so print errors are discarded.
	switch {
	case errors.Is(err, errUsage):
		_, _ = fmt.Fprint(stderr, usageText)
		return exitUsage
	case err != nil:
		_, _ = fmt.Fprintln(stderr, "error:", err)
		return exitFailure
	}
	_, _ = fmt.Fprint(stdout, report)
	return exitSuccess
}

// errUsage reports a malformed command line; run prints the usage text for it.
var errUsage = errors.New("usage")

// runCommand runs the command named by the first argument and returns what to print on success.
func runCommand(arguments []string) (string, error) {
	if len(arguments) == 0 {
		return "", errUsage
	}
	command, commandArguments := arguments[0], arguments[1:]
	switch command {
	case "keygen":
		return keygen(commandArguments, ".")
	case "decrypt":
		return decrypt(commandArguments)
	case "seal":
		return seal(commandArguments)
	case "fingerprint":
		return fingerprint(commandArguments)
	}
	return "", errUsage
}

// keygen creates a key pair in folder and reports the public key and its fingerprint for pasting into Vita.
func keygen(arguments []string, folder string) (string, error) {
	name, isClassic, err := parseKeygenArguments(arguments)
	if err != nil {
		return "", err
	}
	keyPair, err := vita.CreateKeyPair(folder, name, isClassic)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Private key written to %s. Keep it safe: Vita can't recover it.\n", keyPair.PrivateKeyPath) +
		fmt.Sprintf("Public key written to %s.\n", keyPair.PublicKeyPath) +
		fmt.Sprintf("Public key:\n%s\nFingerprint: %s\n", keyPair.PublicKey, vita.Fingerprint(keyPair.PublicKey)), nil
}

// parseKeygenArguments reads keygen's --name NAME and --classic options, in any order. A missing name is left for
// vita.CreateKeyPair to refuse.
func parseKeygenArguments(arguments []string) (name string, isClassic bool, err error) {
	for remaining := arguments; len(remaining) > 0; {
		option := remaining[0]
		remaining = remaining[1:]
		switch option {
		case "--name":
			if len(remaining) == 0 {
				return "", false, errUsage
			}
			name, remaining = remaining[0], remaining[1:]
		case "--classic":
			isClassic = true
		default:
			return "", false, errUsage
		}
	}
	return name, isClassic, nil
}

// fingerprint reports the fingerprint of PUBLIC_KEY.
func fingerprint(arguments []string) (string, error) {
	if len(arguments) != 1 {
		return "", errUsage
	}
	key, err := resolvePublicKey(arguments[0])
	if err != nil {
		return "", err
	}
	return vita.Fingerprint(key) + "\n", nil
}

// resolvePublicKey reads a PUBLIC_KEY argument: the key itself if it starts with age1, else a public key file.
func resolvePublicKey(argument string) (string, error) {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(argument)), "age1") {
		return argument, vita.CheckRecipient(argument)
	}
	key, err := vita.ReadPublicKey(argument)
	if err != nil {
		return "", fmt.Errorf("reading the public key from %s: %w", argument, err)
	}
	return key, nil
}

// seal encrypts FILE to PUBLIC_KEY and writes FILE.age next to it, to test a key pair.
func seal(arguments []string) (string, error) {
	if len(arguments) != 2 {
		return "", errUsage
	}
	key, err := resolvePublicKey(arguments[0])
	if err != nil {
		return "", err
	}
	// Ctrl+C stops the sealing cleanly, so its temporary file doesn't outlive it.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	output, err := vita.Seal(ctx, key, arguments[1], nil)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Sealed to %s (public key fingerprint %s)\n", output, vita.Fingerprint(key)), nil
}

// decrypt decrypts FILE.age with the identities in KEYFILE and writes FILE next to it.
func decrypt(arguments []string) (string, error) {
	if len(arguments) != 2 {
		return "", errUsage
	}
	keyPath, input := arguments[0], arguments[1]
	if _, err := vita.DecryptedName(input); err != nil {
		return "", err
	}
	key, err := vita.ReadPrivateKey(keyPath)
	if err != nil {
		return "", err
	}
	// Ctrl+C stops the decryption cleanly, so its temporary file doesn't outlive it.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	output, err := vita.Decrypt(ctx, key.Identities, input, nil)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Decrypted to %s\n", output), nil
}
