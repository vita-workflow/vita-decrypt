// Package desktop connects the GUI to the operating system: file and folder dialogs, showing a file in the file
// manager, and files dropped on the window. None of it uses the network.
//
// The platform files (desktop_linux.go, desktop_windows.go, desktop_darwin*.go, and desktop_other.go) only make the
// system calls. Everything that decides something lives in files that tests can reach.
package desktop

import "errors"

// ErrUnavailable means this computer has no way to show the requested dialog.
var ErrUnavailable = errors.New("no file dialog is available on this computer")

// Filter narrows a file dialog to names matching Patterns, such as "*.age".
type Filter struct {
	Name     string
	Patterns []string
}

// Dialog describes a file dialog. Every call blocks until the person closes it, so call it from a goroutine. A
// cancelled dialog returns no paths and no error.
type Dialog struct {
	Title   string
	Filters []Filter
	Folder  string // where the dialog starts; empty for the system default
}

// DropHandler receives files dragged over and dropped on the window. Its functions are called one at a time, in
// order, but not necessarily on the window's thread, and must return quickly.
type DropHandler struct {
	Enter func()
	Leave func()
	Drop  func(paths []string)
}

// selection is what a dialog lets the person choose. Each platform's showDialog takes one.
type selection int

const (
	singleFile selection = iota
	multipleFiles
	singleFolder
)

// OpenFile asks for one existing file. It returns "" if the person cancels.
func OpenFile(dialog Dialog) (string, error) { return firstPath(showDialog(dialog, singleFile)) }

// OpenFiles asks for one or more existing files.
func OpenFiles(dialog Dialog) ([]string, error) { return showDialog(dialog, multipleFiles) }

// ChooseFolder asks for a folder. It returns "" if the person cancels.
func ChooseFolder(dialog Dialog) (string, error) { return firstPath(showDialog(dialog, singleFolder)) }

// firstPath is the one path a single-choice dialog returned, or "" if the person cancelled or the dialog failed.
func firstPath(paths []string, err error) (string, error) {
	if err != nil || len(paths) == 0 {
		return "", err
	}
	return paths[0], nil
}
