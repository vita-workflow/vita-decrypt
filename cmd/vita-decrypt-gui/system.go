package main

import (
	"errors"

	"github.com/vita-workflow/vita-decrypt/internal/desktop"
)

// system is what the app asks of the operating system outside its window: file dialogs, the file manager, and
// background work. Tests replace it so nothing native opens.
type system struct {
	openFile        func(dialog desktop.Dialog) (string, error)
	openFiles       func(dialog desktop.Dialog) ([]string, error)
	chooseFolder    func(dialog desktop.Dialog) (string, error)
	reveal          func(path string) error
	runInBackground func(task func())
}

// nativeSystem is the real operating system.
func nativeSystem() system {
	return system{
		openFile:        desktop.OpenFile,
		openFiles:       desktop.OpenFiles,
		chooseFolder:    desktop.ChooseFolder,
		reveal:          desktop.Reveal,
		runInBackground: func(task func()) { go task() },
	}
}

// runDialog runs choose off the event loop, then on the event loop hands the chosen paths to then, or why the dialog
// didn't open to fail. Nothing is called if the dialog was cancelled.
func (app *App) runDialog(choose func() ([]string, error), then func(paths []string), fail func(message string)) {
	app.system.runInBackground(func() {
		paths, err := choose()
		app.post(func() {
			switch {
			case err != nil:
				fail(dialogError(err))
			case len(paths) > 0:
				then(paths)
			}
		})
	})
}

// chooseOne adapts a dialog that picks one path, which is "" when cancelled, to runDialog.
func chooseOne(pick func(dialog desktop.Dialog) (string, error), dialog desktop.Dialog) func() ([]string, error) {
	return func() ([]string, error) {
		path, err := pick(dialog)
		if path == "" {
			return nil, err
		}
		return []string{path}, err
	}
}

func dialogError(err error) string {
	if errors.Is(err, desktop.ErrUnavailable) {
		return "This computer has no file chooser. Copy the files in your file manager, then press Ctrl+V here."
	}
	return "The file chooser didn’t open: " + err.Error()
}

// showInFolder opens the file manager at path, off the event loop.
func (app *App) showInFolder(path string) {
	app.system.runInBackground(func() {
		_ = app.system.reveal(path) // there's nothing more to do if the file manager doesn't open
	})
}
