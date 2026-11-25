package desktop

import (
	"errors"
	"fmt"
	"strings"
)

// zenityCancelledExitCode is the exit code zenity uses when the person closes the dialog without choosing.
const zenityCancelledExitCode = 1

// exitCoder is an error that carries a program's exit code, such as *exec.ExitError.
type exitCoder interface {
	error
	ExitCode() int
}

// showZenityDialog runs zenity. It returns ErrUnavailable if zenity isn't installed.
func (system linuxSystem) showZenityDialog(dialog Dialog, kind selection) ([]string, error) {
	program, err := system.findProgram("zenity")
	if err != nil {
		return nil, ErrUnavailable
	}
	output, err := system.runProgram(program, zenityArguments(dialog, kind)...)
	if err != nil {
		return nil, zenityError(err)
	}
	return nonEmptyLines(string(output)), nil
}

// zenityError is the error to return for a failed zenity run: none if the person cancelled.
func zenityError(err error) error {
	var exit exitCoder
	if errors.As(err, &exit) && exit.ExitCode() == zenityCancelledExitCode {
		return nil
	}
	return fmt.Errorf("zenity: %w", err)
}

func zenityArguments(dialog Dialog, kind selection) []string {
	arguments := []string{"--file-selection", "--title=" + dialog.Title}
	switch kind {
	case singleFile:
	case multipleFiles:
		arguments = append(arguments, "--multiple", "--separator=\n")
	case singleFolder:
		arguments = append(arguments, "--directory")
	}
	if dialog.Folder != "" {
		// A trailing slash makes zenity start inside the folder instead of selecting it in its parent.
		arguments = append(arguments, "--filename="+strings.TrimSuffix(dialog.Folder, "/")+"/")
	}
	for _, filter := range withAllFiles(dialog.Filters, "*") {
		arguments = append(arguments, "--file-filter="+filter.Name+" | "+strings.Join(filter.Patterns, " "))
	}
	return arguments
}
