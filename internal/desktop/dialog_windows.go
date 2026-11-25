package desktop

import (
	"strings"

	"golang.org/x/sys/windows"
)

// Options for IFileDialog::SetOptions, from FILEOPENDIALOGOPTIONS. The names in the comments are the Win32 ones.
const (
	optionPickFolders     = 0x20   // FOS_PICKFOLDERS
	optionForceFileSystem = 0x40   // FOS_FORCEFILESYSTEM
	optionMultipleChoice  = 0x200  // FOS_ALLOWMULTISELECT
	optionPathMustExist   = 0x800  // FOS_PATHMUSTEXIST
	optionFileMustExist   = 0x1000 // FOS_FILEMUSTEXIST
)

// readingChoice describes the step that reads what the person chose, for errors.
const readingChoice = "reading the choice"

// fileDialog is the shell's IFileOpenDialog, as runFileDialog uses it. shellFileDialog, in desktop_windows.go, is the
// real one.
type fileDialog interface {
	options() uint32
	setOptions(options uint32)
	setTitle(title *uint16)
	setFileTypes(fileTypes []filterSpec)
	setFolder(path *uint16)
	show(owner uintptr) uintptr
	results() ([]string, error)
	result() (string, error)
}

// filterSpec is a COMDLG_FILTERSPEC: a filter's name and its patterns separated by semicolons.
type filterSpec struct{ name, patterns *uint16 }

// runFileDialog sets the dialog up as dialog and kind ask, shows it over the owner window, and returns the chosen
// paths. A cancelled dialog returns no paths and no error.
func runFileDialog(fileOpenDialog fileDialog, dialog Dialog, kind selection, owner uintptr) ([]string, error) {
	if err := configureFileDialog(fileOpenDialog, dialog, kind); err != nil {
		return nil, err
	}
	result := fileOpenDialog.show(owner)
	if isHRESULT(result, hresultCancelled) {
		return nil, nil
	}
	if err := checkHRESULT(result, "showing the file dialog"); err != nil {
		return nil, err
	}
	if kind == multipleFiles {
		return fileOpenDialog.results()
	}
	path, err := fileOpenDialog.result()
	if err != nil {
		return nil, err
	}
	return []string{path}, nil
}

func configureFileDialog(fileOpenDialog fileDialog, dialog Dialog, kind selection) error {
	fileOpenDialog.setOptions(dialogOptions(fileOpenDialog.options(), kind))
	if dialog.Title != "" {
		title, err := windows.UTF16PtrFromString(dialog.Title)
		if err != nil {
			return err
		}
		fileOpenDialog.setTitle(title)
	}
	if kind != singleFolder && len(dialog.Filters) > 0 {
		fileTypes, err := filterSpecs(dialog.Filters)
		if err != nil {
			return err
		}
		fileOpenDialog.setFileTypes(fileTypes)
	}
	if dialog.Folder != "" {
		folder, err := windows.UTF16PtrFromString(dialog.Folder)
		if err != nil {
			return err
		}
		fileOpenDialog.setFolder(folder)
	}
	return nil
}

// dialogOptions adds the options for kind to the dialog's default options.
func dialogOptions(defaults uint32, kind selection) uint32 {
	options := defaults | optionForceFileSystem | optionPathMustExist
	switch kind {
	case singleFile:
		options |= optionFileMustExist
	case multipleFiles:
		options |= optionMultipleChoice | optionFileMustExist
	case singleFolder:
		options |= optionPickFolders
	}
	return options
}

// filterSpecs is the file type list for IFileDialog::SetFileTypes.
func filterSpecs(filters []Filter) ([]filterSpec, error) {
	var specs []filterSpec
	for _, filter := range withAllFiles(filters, "*.*") {
		name, err := windows.UTF16PtrFromString(filter.Name)
		if err != nil {
			return nil, err
		}
		patterns, err := windows.UTF16PtrFromString(strings.Join(filter.Patterns, ";"))
		if err != nil {
			return nil, err
		}
		specs = append(specs, filterSpec{name: name, patterns: patterns})
	}
	return specs, nil
}

// collectPaths reads count chosen items with pathAt, stopping at the first error.
func collectPaths(count uint32, pathAt func(index uint32) (string, error)) ([]string, error) {
	var paths []string // not sized by count, which comes from the shell
	for index := range count {
		path, err := pathAt(index)
		if err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	return paths, nil
}
