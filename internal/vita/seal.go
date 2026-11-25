package vita

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"filippo.io/age"
)

// SealedName is where Seal saves input: its name with .age added, in the same folder.
func SealedName(input string) string { return input + sealedSuffix }

// Seal encrypts input to the public key recipient with age, as Vita does, and writes FILE.age next to it, returning
// that path. Like Decrypt, it writes to a temporary file first, never overwrites an existing file, reports progress
// (bytes of input read) if progress isn't nil, and stops and cleans up when ctx is cancelled.
func Seal(ctx context.Context, recipient, input string, progress func(read int64)) (string, error) {
	parsedRecipient, err := parseRecipient(recipient)
	if err != nil {
		return "", err
	}
	output := SealedName(input)
	if _, err := os.Lstat(output); err == nil {
		return "", &existsError{output}
	}
	inputFile, err := openRegularFile(input)
	if err != nil {
		return "", err
	}
	defer closeQuietly(inputFile)

	tempFile, err := os.CreateTemp(filepath.Dir(output), tempFilePattern)
	if err != nil {
		return "", err
	}
	defer removeQuietly(tempFile.Name())
	err = encrypt(tempFile, parsedRecipient, &countingReader{ctx: ctx, source: inputFile, report: progress})
	if closeErr := tempFile.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", fmt.Errorf("sealing %s; nothing was kept: %w", filepath.Base(input), err)
	}
	if err := publish(tempFile.Name(), output); err != nil {
		return "", err
	}
	return output, nil
}

// openRegularFile opens path for reading, refusing a folder or a device.
func openRegularFile(path string) (*os.File, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		closeQuietly(file)
		return nil, err
	}
	if !info.Mode().IsRegular() {
		closeQuietly(file)
		return nil, errors.New(path + " isn't a file")
	}
	return file, nil
}

// encrypt writes plaintext to destination, encrypted to recipient.
func encrypt(destination io.Writer, recipient age.Recipient, plaintext io.Reader) error {
	encryptor, err := age.Encrypt(destination, recipient)
	if err != nil {
		return err
	}
	if _, err := io.Copy(encryptor, plaintext); err != nil {
		return err
	}
	return encryptor.Close()
}
