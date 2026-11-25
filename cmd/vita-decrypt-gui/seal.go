package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync/atomic"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/vita-workflow/vita-decrypt/internal/desktop"
	"github.com/vita-workflow/vita-decrypt/internal/vita"
)

// sealState is the seal screen, which seals a file to a public key the way Vita does, so a new key pair can be tested
// before anything real is sealed to it.
type sealState struct {
	keyEditor    widget.Editor
	paste        paster
	keyFileError inputError // why the chosen public key file couldn't be read
	file         string
	sizeBytes    int64
	fileError    string
	bytesRead    atomic.Int64 // bytes sealed so far, written by the worker
	isWorking    bool
	result       *fileResult
	resultFor    string // the key and file the result is for

	chooseKeyButton, chooseFileButton, changeFileButton, startButton widget.Clickable
	showButton, openNowButton                                        widget.Clickable
}

var fileToSealDialog = desktop.Dialog{Title: "Choose a file to seal"}

// setSealFile chooses the file to seal, from a dialog or a drop.
func (app *App) setSealFile(path string) {
	state := &app.seal
	if state.isWorking {
		return
	}
	state.result = nil
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		state.fileError = fmt.Sprintf("Can’t read “%s”. Choose a file, not a folder.", filepath.Base(path))
		return
	}
	state.file, state.sizeBytes, state.fileError = path, info.Size(), ""
}

// recipient is the public key in the field with its spaces and line breaks taken out, and whether it's valid.
func (state *sealState) recipient() (string, bool) {
	key := compactKey(state.keyEditor.Text())
	return key, key != "" && vita.CheckRecipient(key) == nil
}

// resultKey identifies the key and file a result is for.
func resultKey(key, path string) string { return key + "\x00" + path }

// startSealing seals the chosen file in the background.
func (app *App) startSealing() {
	state := &app.seal
	key, isValid := state.recipient()
	if !isValid || state.file == "" || state.isWorking {
		return
	}
	state.isWorking, state.result = true, nil
	state.bytesRead.Store(0)
	path := state.file
	state.resultFor = resultKey(key, path)
	app.workers.Add(1)
	app.system.runInBackground(func() {
		defer app.workers.Done()
		outputPath, err := vita.Seal(app.ctx, key, path, app.progressReporter(&state.bytesRead))
		result := sealOutcome(path, outputPath, err)
		app.post(func() { state.isWorking, state.result = false, result })
	})
}

// sealOutcome explains what happened to the file input; outputPath is the sealed file.
func sealOutcome(input, outputPath string, err error) *fileResult {
	switch {
	case err == nil:
		return &fileResult{kind: successMessage, text: "Sealed as " + filepath.Base(outputPath), outputPath: outputPath}
	case errors.Is(err, fs.ErrExist):
		return &fileResult{
			kind: warningMessage,
			text: filepath.Base(vita.SealedName(input)) +
				" already exists in this folder. Move or rename it, then try again.",
		}
	default:
		return &fileResult{kind: dangerMessage, text: "The file wasn’t sealed: " + err.Error()}
	}
}

// handleSealClicks acts on the seal screen's buttons.
func (app *App) handleSealClicks(gtx layout.Context) {
	state := &app.seal
	state.paste.update(gtx, &state.keyEditor)
	if state.chooseKeyButton.Clicked(gtx) {
		app.choosePublicKey(&state.keyEditor, &state.keyFileError)
	}
	if (state.chooseFileButton.Clicked(gtx) || state.changeFileButton.Clicked(gtx)) && !state.isWorking {
		app.runDialog(chooseOne(app.system.openFile, fileToSealDialog),
			func(paths []string) { app.setSealFile(paths[0]) },
			func(message string) { state.fileError = message })
	}
	if state.startButton.Clicked(gtx) {
		app.startSealing()
	}
	result := state.result
	if result == nil || result.outputPath == "" {
		return
	}
	if state.showButton.Clicked(gtx) {
		app.showInFolder(result.outputPath)
	}
	if state.openNowButton.Clicked(gtx) {
		// The open screen takes it from here: the sealed file is listed and only the private key is missing.
		state.result = nil
		app.screen = openScreen
		app.content.Position = layout.Position{}
		app.addPaths([]string{result.outputPath})
	}
}

// sealKeyNote is the public key field's error and helper text.
func (state *sealState) sealKeyNote(key string, isValid bool) (errorMessage, helper string) {
	helper = "Paste the public key, or choose your public key file (its name ends in -public-key.txt)."
	if fileError := state.keyFileError.messageFor(state.keyEditor.Text()); fileError != "" {
		return fileError, helper
	}
	switch {
	case key != "" && !isValid:
		return "This isn’t a valid public key. Vita public keys start with age1; " +
			"check that you copied all of it.", helper
	case isValid:
		return "", "Fingerprint " + vita.Fingerprint(key) + ". Check that it’s the one Vita shows for this key pair."
	}
	return "", helper
}
