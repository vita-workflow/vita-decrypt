package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"filippo.io/age"
	"gioui.org/io/clipboard"
	"gioui.org/io/transfer"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/vita-workflow/vita-decrypt/internal/desktop"
	"github.com/vita-workflow/vita-decrypt/internal/vita"
)

// openState is the open screen: a private key and the sealed files to open with it.
type openState struct {
	privateKey   *vita.PrivateKey
	keyPath      string
	hasKeyError  bool
	filesMessage string // why some chosen files weren't added

	files     []*sealedFile
	isWorking bool
	isDone    bool

	chooseKeyButton, changeKeyButton, chooseFilesButton, addFilesButton widget.Clickable
	startButton, openMoreButton                                         widget.Clickable
	pasteTag                                                            int
}

type sealedFile struct {
	path         string
	sizeBytes    int64
	bytesRead    atomic.Int64 // written by the worker
	result       *fileResult
	removeButton widget.Clickable
	showButton   widget.Clickable
}

// fileResult is what happened to one file.
type fileResult struct {
	kind       messageKind
	text       string
	outputPath string // the saved file, on success
}

const (
	sealedExtension = ".age"
	notSealedFormat = "“%s” isn’t a sealed file. Sealed file names end in .age."
	// pastedPathsLimit is the most clipboard text read as pasted file paths.
	pastedPathsLimit = 1 << 20
	// progressInterval is how often a file's progress bar redraws at most.
	progressInterval = 50 * time.Millisecond
)

var (
	keyDialog = desktop.Dialog{
		Title:   "Choose your private key file",
		Filters: []desktop.Filter{{Name: "Private key files", Patterns: []string{"*.txt"}}},
	}
	sealedDialog = desktop.Dialog{
		Title:   "Choose sealed files",
		Filters: []desktop.Filter{{Name: "Sealed files", Patterns: []string{"*" + sealedExtension}}},
	}
)

// addPaths takes files from a drop, a paste, or a dialog: names ending in .age are sealed files, and anything else is
// tried as the private key.
func (app *App) addPaths(paths []string) {
	state := &app.open
	if state.isWorking {
		return
	}
	if state.isDone {
		app.resetFiles()
	}
	app.screen = openScreen
	var sealed, others []string
	for _, path := range paths {
		if strings.HasSuffix(strings.ToLower(path), sealedExtension) {
			sealed = append(sealed, path)
		} else {
			others = append(others, path)
		}
	}
	if len(others) > 0 {
		app.loadKey(others[0])
	}
	app.addSealed(sealed)
	if len(others) > 1 {
		state.filesMessage = fmt.Sprintf(notSealedFormat, filepath.Base(others[1]))
	}
}

func (app *App) loadKey(path string) {
	state := &app.open
	privateKey, err := vita.ReadPrivateKey(path)
	if err != nil {
		state.privateKey, state.keyPath, state.hasKeyError = nil, "", true
		return
	}
	state.privateKey, state.keyPath, state.hasKeyError = &privateKey, path, false
}

func (app *App) addSealed(paths []string) {
	state := &app.open
	state.filesMessage = ""
	for _, path := range paths {
		if state.hasFile(path) {
			continue
		}
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			state.filesMessage = fmt.Sprintf("Can’t read “%s”.", filepath.Base(path))
			continue
		}
		if _, err := vita.DecryptedName(path); err != nil {
			state.filesMessage = fmt.Sprintf(notSealedFormat, filepath.Base(path))
			continue
		}
		state.files = append(state.files, &sealedFile{path: path, sizeBytes: info.Size()})
	}
}

func (state *openState) hasFile(path string) bool {
	return slices.ContainsFunc(state.files, func(file *sealedFile) bool { return file.path == path })
}

func (app *App) resetFiles() {
	state := &app.open
	state.files, state.isDone, state.filesMessage = nil, false, ""
}

// requestPaste asks for the clipboard, which arrives in receivePaste.
func (app *App) requestPaste(gtx layout.Context) {
	gtx.Execute(clipboard.ReadCmd{Tag: &app.open.pasteTag})
}

// receivePaste adds the files named in pasted text.
func (app *App) receivePaste(gtx layout.Context) {
	for {
		received, ok := gtx.Event(transfer.TargetFilter{Target: &app.open.pasteTag, Type: textMIME})
		if !ok {
			return
		}
		data, ok := received.(transfer.DataEvent)
		if !ok {
			continue
		}
		if paths := desktop.PathsFromText(readClipboardText(data, pastedPathsLimit)); len(paths) > 0 {
			app.addPaths(paths)
		} else if !app.open.isWorking {
			app.open.filesMessage = "The clipboard doesn’t hold any files. " +
				"Copy the files (or their full paths), then paste again."
		}
	}
}

// fromDialog wraps what a dialog does with its paths, since the list may have started opening, or finished, while
// the dialog was up.
func (app *App) fromDialog(add func(paths []string)) func(paths []string) {
	return func(paths []string) {
		if app.open.isWorking {
			return
		}
		if app.open.isDone {
			app.resetFiles()
		}
		add(paths)
	}
}

// failOpenDialog shows why a dialog on the open screen didn't open.
func (app *App) failOpenDialog(message string) {
	if !app.open.isWorking {
		app.open.filesMessage = message
	}
}

// handleOpenClicks acts on the open screen's buttons.
func (app *App) handleOpenClicks(gtx layout.Context) {
	state := &app.open
	if state.chooseKeyButton.Clicked(gtx) || state.changeKeyButton.Clicked(gtx) {
		app.runDialog(chooseOne(app.system.openFile, keyDialog),
			app.fromDialog(func(paths []string) { app.loadKey(paths[0]) }), app.failOpenDialog)
	}
	if state.chooseFilesButton.Clicked(gtx) || state.addFilesButton.Clicked(gtx) {
		chooseFiles := func() ([]string, error) { return app.system.openFiles(sealedDialog) }
		app.runDialog(chooseFiles, app.fromDialog(app.addSealed), app.failOpenDialog)
	}
	if state.startButton.Clicked(gtx) {
		app.startOpening()
	}
	if state.openMoreButton.Clicked(gtx) {
		app.resetFiles()
	}
	canRemove := !state.isWorking && !state.isDone
	state.files = slices.DeleteFunc(state.files, func(file *sealedFile) bool {
		return canRemove && file.removeButton.Clicked(gtx)
	})
	for _, file := range state.files {
		if file.showButton.Clicked(gtx) && file.result != nil && file.result.outputPath != "" {
			app.showInFolder(file.result.outputPath)
		}
	}
}

// startOpening decrypts the files one at a time in the background.
func (app *App) startOpening() {
	state := &app.open
	if state.privateKey == nil || len(state.files) == 0 || state.isWorking {
		return
	}
	state.isWorking = true
	identities := state.privateKey.Identities
	files := state.files
	app.workers.Add(1)
	app.system.runInBackground(func() {
		defer app.workers.Done()
		for _, file := range files {
			if app.ctx.Err() != nil {
				return
			}
			outputPath, err := vita.Decrypt(app.ctx, identities, file.path, app.progressReporter(&file.bytesRead))
			result := outcome(file.path, outputPath, err)
			app.post(func() { file.result = result })
		}
		app.post(func() {
			state.isWorking = false
			state.isDone = true
			app.content.Position = layout.Position{}
		})
	})
}

// progressReporter returns a progress callback that stores the bytes done in counter and redraws, at most every
// progressInterval. It runs on the worker goroutine.
func (app *App) progressReporter(counter *atomic.Int64) func(bytesDone int64) {
	var lastRedraw time.Time
	return func(bytesDone int64) {
		counter.Store(bytesDone)
		if now := time.Now(); now.Sub(lastRedraw) > progressInterval {
			lastRedraw = now
			app.window.Invalidate()
		}
	}
}

// outcome explains what happened to the sealed file input in the design's words; outputPath is where it was saved.
func outcome(input, outputPath string, err error) *fileResult {
	var noMatch *age.NoIdentityMatchError
	switch {
	case err == nil:
		return &fileResult{kind: successMessage, text: "Saved as " + filepath.Base(outputPath), outputPath: outputPath}
	case errors.Is(err, fs.ErrExist):
		name, _ := vita.DecryptedName(filepath.Base(input)) // input was checked when it was added
		return &fileResult{
			kind: warningMessage, text: name + " already exists in this folder. Move or rename it, then try again.",
		}
	case errors.As(err, &noMatch):
		return &fileResult{
			kind: dangerMessage, text: "This key can’t open this file. It may be sealed to a different key pair.",
		}
	case errors.Is(err, vita.ErrDamaged):
		return &fileResult{
			kind: dangerMessage, text: "This file is damaged or was changed after it was sealed. Nothing was saved.",
		}
	case errors.Is(err, vita.ErrCannotDecrypt):
		return &fileResult{
			kind: dangerMessage, text: "This isn’t a file Vita sealed, or it’s damaged. Nothing was saved.",
		}
	case errors.Is(err, vita.ErrNotSealedName):
		return &fileResult{kind: dangerMessage, text: "This isn’t a sealed file. Sealed file names end in .age."}
	default:
		return &fileResult{kind: dangerMessage, text: "This file couldn’t be opened: " + err.Error()}
	}
}

// openSummary is the heading shown after opening.
type openSummary struct {
	kind            messageKind
	title, subtitle string
}

// summarize says how many of files were opened.
func summarize(files []*sealedFile) openSummary {
	opened := 0
	for _, file := range files {
		if file.result != nil && file.result.kind == successMessage {
			opened++
		}
	}
	total := len(files)
	switch {
	case opened == 0 && total == 1:
		return openSummary{dangerMessage, "The file wasn’t opened.", "Nothing was saved. The reason is below."}
	case opened == 0:
		return openSummary{dangerMessage, "No files were opened.", "Nothing was saved. The reasons are below."}
	case opened == total && total == 1:
		return openSummary{successMessage, "Your file was opened.", "It was saved next to its sealed copy."}
	case opened == total:
		return openSummary{successMessage, fmt.Sprintf("All %d files opened.", total), savedNextToCopies}
	}
	return openSummary{successMessage, fmt.Sprintf("%d of %d files opened.", opened, total), savedNextToCopies}
}

const savedNextToCopies = "Your files were saved next to their sealed copies."

// formatSize writes a file size the way file managers do, in powers of 1000.
func formatSize(sizeBytes int64) string {
	if sizeBytes == 1 {
		return "1 byte"
	}
	if sizeBytes < 1000 {
		return fmt.Sprintf("%d bytes", sizeBytes)
	}
	value := float64(sizeBytes) / 1000
	units := []string{"KB", "MB", "GB", "TB"}
	index := 0
	for value >= 999.5 && index < len(units)-1 {
		value /= 1000
		index++
	}
	if index == 0 || value >= 99.95 {
		return fmt.Sprintf("%.0f %s", value, units[index])
	}
	return fmt.Sprintf("%.1f %s", value, units[index])
}
