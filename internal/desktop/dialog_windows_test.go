package desktop

import (
	"errors"
	"reflect"
	"syscall"
	"testing"

	"golang.org/x/sys/windows"
)

// fakeFileDialog records how runFileDialog sets it up, and answers with chosen or chooseErr.
type fakeFileDialog struct {
	defaultOptions uint32
	showResult     uintptr
	chosen         []string
	chooseErr      error

	setOptionsTo uint32
	title        string
	fileTypes    []Filter // each filterSpec, with its patterns as one string
	folder       string
	shownOver    uintptr
	isShown      bool
}

func (dialog *fakeFileDialog) options() uint32 { return dialog.defaultOptions }

func (dialog *fakeFileDialog) setOptions(options uint32) { dialog.setOptionsTo = options }

func (dialog *fakeFileDialog) setTitle(title *uint16) { dialog.title = windows.UTF16PtrToString(title) }

func (dialog *fakeFileDialog) setFileTypes(fileTypes []filterSpec) {
	for _, spec := range fileTypes {
		dialog.fileTypes = append(dialog.fileTypes, Filter{
			Name:     windows.UTF16PtrToString(spec.name),
			Patterns: []string{windows.UTF16PtrToString(spec.patterns)},
		})
	}
}

func (dialog *fakeFileDialog) setFolder(path *uint16) { dialog.folder = windows.UTF16PtrToString(path) }

func (dialog *fakeFileDialog) show(owner uintptr) uintptr {
	dialog.isShown, dialog.shownOver = true, owner
	return dialog.showResult
}

func (dialog *fakeFileDialog) results() ([]string, error) { return dialog.chosen, dialog.chooseErr }

func (dialog *fakeFileDialog) result() (string, error) {
	if dialog.chooseErr != nil {
		return "", dialog.chooseErr
	}
	return dialog.chosen[0], nil
}

func TestRunFileDialog(t *testing.T) {
	errRead := errors.New("can't read the choice")
	chosen := []string{`C:\a\x.age`, `C:\a\y.age`}
	tests := []struct {
		name      string
		kind      selection
		dialog    *fakeFileDialog
		wantPaths []string
		wantErr   error
	}{
		{name: "one file returns it", kind: singleFile, dialog: &fakeFileDialog{chosen: chosen}, wantPaths: chosen[:1]},
		{
			name:      "several files return them all",
			kind:      multipleFiles,
			dialog:    &fakeFileDialog{chosen: chosen},
			wantPaths: chosen,
		},
		{name: "one folder", kind: singleFolder, dialog: &fakeFileDialog{chosen: chosen}, wantPaths: chosen[:1]},
		{
			name:   "a cancelled dialog returns nothing",
			kind:   singleFile,
			dialog: &fakeFileDialog{showResult: upperHalf | hresultCancelled, chosen: chosen},
		},
		{
			name:    "a dialog that fails to show is an error",
			kind:    singleFile,
			dialog:  &fakeFileDialog{showResult: hresultFail},
			wantErr: syscall.Errno(hresultFail),
		},
		{
			name:    "a choice that can't be read is an error",
			kind:    singleFile,
			dialog:  &fakeFileDialog{chooseErr: errRead},
			wantErr: errRead,
		},
		{
			name:    "choices that can't be read are an error",
			kind:    multipleFiles,
			dialog:  &fakeFileDialog{chooseErr: errRead},
			wantErr: errRead,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			paths, err := runFileDialog(test.dialog, Dialog{Title: "Open"}, test.kind, 42)
			if !reflect.DeepEqual(paths, test.wantPaths) || !errors.Is(err, test.wantErr) {
				t.Fatalf("runFileDialog = %q, %v; want %q, %v", paths, err, test.wantPaths, test.wantErr)
			}
			if test.dialog.shownOver != 42 {
				t.Fatalf("shown over window %d, want 42", test.dialog.shownOver)
			}
		})
	}
}

func TestRunFileDialog_SetsTheDialogUp(t *testing.T) {
	ageFilters := []Filter{{Name: "age files", Patterns: []string{"*.age", "*.AGE"}}}
	dialog := Dialog{Title: "Open a file", Filters: ageFilters, Folder: `C:\a`}
	tests := []struct {
		name          string
		kind          selection
		wantOptions   uint32
		wantFileTypes []Filter
	}{
		{
			name:        "one file must exist",
			kind:        singleFile,
			wantOptions: 1 | optionForceFileSystem | optionPathMustExist | optionFileMustExist,
			wantFileTypes: []Filter{
				{Name: "age files", Patterns: []string{"*.age;*.AGE"}},
				{Name: allFilesName, Patterns: []string{"*.*"}},
			},
		},
		{
			name:        "several files must exist",
			kind:        multipleFiles,
			wantOptions: 1 | optionForceFileSystem | optionPathMustExist | optionFileMustExist | optionMultipleChoice,
			wantFileTypes: []Filter{
				{Name: "age files", Patterns: []string{"*.age;*.AGE"}},
				{Name: allFilesName, Patterns: []string{"*.*"}},
			},
		},
		{
			name:        "a folder has no file types",
			kind:        singleFolder,
			wantOptions: 1 | optionForceFileSystem | optionPathMustExist | optionPickFolders,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fake := &fakeFileDialog{defaultOptions: 1, chosen: []string{`C:\a\x.age`}}
			if _, err := runFileDialog(fake, dialog, test.kind, 0); err != nil {
				t.Fatal(err)
			}
			if fake.setOptionsTo != test.wantOptions {
				t.Fatalf("options %#x, want %#x", fake.setOptionsTo, test.wantOptions)
			}
			if !reflect.DeepEqual(fake.fileTypes, test.wantFileTypes) {
				t.Fatalf("file types %v, want %v", fake.fileTypes, test.wantFileTypes)
			}
			if fake.title != dialog.Title || fake.folder != dialog.Folder {
				t.Fatalf("title %q and folder %q, want %q and %q", fake.title, fake.folder, dialog.Title, dialog.Folder)
			}
		})
	}
}

func TestRunFileDialog_LeavesDefaultsAlone(t *testing.T) {
	fake := &fakeFileDialog{chosen: []string{`C:\a\x.age`}}
	if _, err := runFileDialog(fake, Dialog{}, singleFile, 0); err != nil {
		t.Fatal(err)
	}
	if fake.title != "" || fake.fileTypes != nil || fake.folder != "" {
		t.Fatalf("set title %q, file types %v, folder %q on a dialog without them",
			fake.title, fake.fileTypes, fake.folder)
	}
}

func TestRunFileDialog_RefusesNUL(t *testing.T) {
	tests := []struct {
		name   string
		dialog Dialog
	}{
		{name: "a NUL in the title", dialog: Dialog{Title: "a\x00b"}},
		{name: "a NUL in a filter name", dialog: Dialog{Filters: []Filter{{Name: "a\x00b", Patterns: []string{"*"}}}}},
		{name: "a NUL in a pattern", dialog: Dialog{Filters: []Filter{{Name: "a", Patterns: []string{"*\x00"}}}}},
		{name: "a NUL in the folder", dialog: Dialog{Folder: "C:\\a\x00b"}},
	}
	for _, test := range tests {
		t.Run(test.name+" is an error and shows nothing", func(t *testing.T) {
			fake := &fakeFileDialog{chosen: []string{`C:\a\x.age`}}
			paths, err := runFileDialog(fake, test.dialog, singleFile, 0)
			if paths != nil || !errors.Is(err, syscall.EINVAL) || fake.isShown {
				t.Fatalf("runFileDialog = %q, %v, shown %v; want EINVAL, not shown", paths, err, fake.isShown)
			}
		})
	}
}

func TestCollectPaths(t *testing.T) {
	errItem := errors.New("can't read item 1")
	items := []string{`C:\a`, `C:\b`, `C:\c`}
	pathAt := func(index uint32) (string, error) {
		if index == 1 {
			return "", errItem
		}
		return items[index], nil
	}
	tests := []struct {
		name      string
		count     uint32
		wantPaths []string
		wantErr   error
	}{
		{name: "no items return nothing", count: 0, wantPaths: nil},
		{name: "items are read until the count", count: 1, wantPaths: items[:1]},
		{name: "a failure returns only the error", count: 3, wantErr: errItem},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			paths, err := collectPaths(test.count, pathAt)
			if !reflect.DeepEqual(paths, test.wantPaths) || !errors.Is(err, test.wantErr) {
				t.Fatalf("collectPaths(%d) = %q, %v; want %q, %v", test.count, paths, err, test.wantPaths, test.wantErr)
			}
		})
	}
}
