package vita

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"filippo.io/age"
)

var (
	// ErrNotSealedName reports an input whose name doesn't end in .age.
	ErrNotSealedName = errors.New("expected a file ending in .age")
	// ErrCannotDecrypt reports a file age refused to decrypt: the identities don't match its recipients
	// (errors.As finds an *age.NoIdentityMatchError), or it isn't an age file.
	ErrCannotDecrypt = errors.New("this key can't open the file")
	// ErrDamaged reports a file whose contents failed authentication part way through.
	ErrDamaged = errors.New("the file is damaged or was changed; nothing was written")
)

// sealedSuffix ends the name of every sealed file.
const sealedSuffix = ".age"

// tempFilePattern names the temporary files Decrypt and Seal write before publishing them.
const tempFilePattern = ".vita-decrypt-*"

// outputFileMode is the mode of an output written by the copy fallback in publish, the same mode os.CreateTemp gives
// the temporary file it copies, so decrypted contents stay private to the user.
const outputFileMode = 0o600

// kindError is an error that matches a sentinel with errors.Is and wraps its cause.
type kindError struct {
	kind  error
	cause error
}

func (kindError *kindError) Error() string {
	return kindError.kind.Error() + ": " + kindError.cause.Error()
}

// Is matches the sentinel itself, not an error wrapping it, so == is the comparison wanted here.
func (kindError *kindError) Is(target error) bool { return target == kindError.kind }

func (kindError *kindError) Unwrap() error { return kindError.cause }

// existsError reports an output that is already there; it matches fs.ErrExist.
type existsError struct{ path string }

func (existsError *existsError) Error() string { return "won't overwrite existing " + existsError.path }

// Is matches fs.ErrExist itself, so == is the comparison wanted here.
func (*existsError) Is(target error) bool { return target == fs.ErrExist }

// DecryptedName is where Decrypt saves input: its name without .age, in the same folder.
func DecryptedName(input string) (string, error) {
	if !strings.HasSuffix(input, sealedSuffix) || len(filepath.Base(input)) == len(sealedSuffix) {
		return "", ErrNotSealedName
	}
	return strings.TrimSuffix(input, sealedSuffix), nil
}

// Decrypt decrypts FILE.age with identities and writes FILE next to it, returning FILE. It writes to a temporary file
// and moves it into place only after age has authenticated every chunk, so a tampered or truncated file never leaves
// partial plaintext behind, and it never overwrites an existing file. progress, if not nil, is told how many bytes of
// input have been read. Cancelling ctx stops the decryption and removes the temporary file; the caller must wait for
// Decrypt to return before exiting, or the partial plaintext stays behind.
func Decrypt(ctx context.Context, identities []age.Identity, input string, progress func(read int64)) (string, error) {
	output, err := DecryptedName(input)
	if err != nil {
		return "", err
	}
	if _, err := os.Lstat(output); err == nil {
		return "", &existsError{output}
	}

	inputFile, err := os.Open(input)
	if err != nil {
		return "", err
	}
	defer closeQuietly(inputFile)
	plaintext, err := age.Decrypt(&countingReader{ctx: ctx, source: inputFile, report: progress}, identities...)
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err != nil {
		return "", &kindError{ErrCannotDecrypt, err}
	}

	tempFile, err := os.CreateTemp(filepath.Dir(output), tempFilePattern)
	if err != nil {
		return "", err
	}
	defer removeQuietly(tempFile.Name())
	if err := copyPlaintext(ctx, tempFile, plaintext, output); err != nil {
		closeQuietly(tempFile)
		return "", err
	}
	if err := tempFile.Close(); err != nil {
		return "", err
	}
	if err := publish(tempFile.Name(), output); err != nil {
		return "", err
	}
	return output, nil
}

// copyPlaintext copies the decrypted plaintext to destination. Only a failed read means the file is damaged; a failed
// write (a full disk) or a cancel is reported as itself.
func copyPlaintext(ctx context.Context, destination io.Writer, plaintext io.Reader, output string) error {
	writer := &recordingWriter{destination: destination}
	_, err := io.Copy(writer, plaintext)
	switch {
	case err == nil:
		return nil
	case ctx.Err() != nil:
		return ctx.Err()
	case writer.err != nil:
		return fmt.Errorf("writing %s; nothing was kept: %w", output, writer.err)
	}
	return &kindError{ErrDamaged, err}
}

// countingReader reports progress and stops reading once ctx is cancelled.
type countingReader struct {
	ctx       context.Context
	source    io.Reader
	bytesRead int64
	report    func(bytesRead int64) // may be nil
}

func (reader *countingReader) Read(buffer []byte) (int, error) {
	if err := reader.ctx.Err(); err != nil {
		return 0, err
	}
	count, err := reader.source.Read(buffer)
	reader.bytesRead += int64(count)
	if reader.report != nil {
		reader.report(reader.bytesRead)
	}
	return count, err
}

// recordingWriter remembers the error its destination returned, to tell write failures from read failures in io.Copy.
type recordingWriter struct {
	destination io.Writer
	err         error
}

func (writer *recordingWriter) Write(buffer []byte) (int, error) {
	count, err := writer.destination.Write(buffer)
	if err != nil {
		writer.err = err
	}
	return count, err
}

// linkFile is os.Link; tests replace it to stand in for a file system without hard links.
var linkFile = os.Link

// publish puts the finished file at tempPath at outputPath without ever replacing an existing file, even one that
// appeared after Decrypt or Seal checked. A hard link does this atomically. File systems without hard links (FAT32 and
// exFAT drives, some network shares) get an exclusive create and a copy instead, and a failed copy is removed.
func publish(tempPath, outputPath string) error {
	err := linkFile(tempPath, outputPath)
	if err == nil {
		return nil
	}
	if errors.Is(err, fs.ErrExist) {
		return &existsError{outputPath}
	}
	return copyExclusive(tempPath, outputPath)
}

// copyExclusive copies sourcePath to a new file at destinationPath, failing with fs.ErrExist rather than replace one.
func copyExclusive(sourcePath, destinationPath string) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer closeQuietly(source)
	destination, err := os.OpenFile(destinationPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, outputFileMode)
	if errors.Is(err, fs.ErrExist) {
		return &existsError{destinationPath}
	}
	if err != nil {
		return err
	}
	_, err = io.Copy(destination, source)
	if err == nil {
		err = destination.Sync()
	}
	if closeErr := destination.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		removeQuietly(destinationPath)
		return fmt.Errorf("writing %s; nothing was kept: %w", destinationPath, err)
	}
	return nil
}

// closeQuietly closes a file that was only read, or one being abandoned because writing it already failed. Neither
// close can lose data the caller still wants, and in the second case the write error is the one worth reporting.
func closeQuietly(file io.Closer) {
	_ = file.Close() // see the doc comment: there is nothing useful to report
}

// removeQuietly removes a file this package created and no longer needs: a temporary file, or an output it couldn't
// finish. The caller is either already returning the error that matters or has published the file under its real
// name, so a failed removal isn't reported on top of that.
func removeQuietly(path string) {
	_ = os.Remove(path) // best effort, see the doc comment
}
