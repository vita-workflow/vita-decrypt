package desktop

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
)

// fakeExitError is an error from a program that exited with code.
type fakeExitError struct{ code int }

func (exit fakeExitError) Error() string { return fmt.Sprintf("exit status %d", exit.code) }

func (exit fakeExitError) ExitCode() int { return exit.code }

func TestZenityArguments(t *testing.T) {
	ageFilters := []Filter{{Name: "age files", Patterns: []string{"*.age", "*.AGE"}}}
	tests := []struct {
		name          string
		dialog        Dialog
		kind          selection
		wantArguments []string
	}{
		{
			name:          "one file",
			dialog:        Dialog{Title: "Open"},
			kind:          singleFile,
			wantArguments: []string{"--file-selection", "--title=Open"},
		},
		{
			name:          "several files are separated by newlines",
			dialog:        Dialog{Title: "Open"},
			kind:          multipleFiles,
			wantArguments: []string{"--file-selection", "--title=Open", "--multiple", "--separator=\n"},
		},
		{
			name:          "a folder",
			dialog:        Dialog{Title: "Choose"},
			kind:          singleFolder,
			wantArguments: []string{"--file-selection", "--title=Choose", "--directory"},
		},
		{
			name:          "the starting folder gets one trailing slash",
			dialog:        Dialog{Title: "Open", Folder: "/home/a/"},
			kind:          singleFile,
			wantArguments: []string{"--file-selection", "--title=Open", "--filename=/home/a/"},
		},
		{
			name:   "filters end with All files",
			dialog: Dialog{Title: "Open", Filters: ageFilters},
			kind:   singleFile,
			wantArguments: []string{
				"--file-selection", "--title=Open",
				"--file-filter=age files | *.age *.AGE", "--file-filter=All files | *",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if arguments := zenityArguments(test.dialog, test.kind); !reflect.DeepEqual(arguments, test.wantArguments) {
				t.Fatalf("zenityArguments = %q, want %q", arguments, test.wantArguments)
			}
		})
	}
}

func TestZenityError(t *testing.T) {
	errCrashed := errors.New("signal: killed")
	tests := []struct {
		name    string
		err     error
		wantErr error
	}{
		{name: "exit code 1 is a cancel", err: fakeExitError{code: 1}, wantErr: nil},
		{name: "another exit code is an error", err: fakeExitError{code: 5}, wantErr: fakeExitError{code: 5}},
		{name: "a wrapped cancel is a cancel", err: fmt.Errorf("running: %w", fakeExitError{code: 1}), wantErr: nil},
		{name: "an error without an exit code is an error", err: errCrashed, wantErr: errCrashed},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := zenityError(test.err); !errors.Is(err, test.wantErr) || (err == nil) != (test.wantErr == nil) {
				t.Fatalf("zenityError(%v) = %v, want %v", test.err, err, test.wantErr)
			}
		})
	}
}

func TestShowZenityDialog_ReadsOneChoicePerLine(t *testing.T) {
	programs := &fakePrograms{
		installed: map[string]string{"zenity": "/usr/bin/zenity"},
		output:    []byte("/home/a/x.age\n/home/a/y z.age\n"),
	}
	paths, err := fakeSystem(nil, programs).showZenityDialog(Dialog{}, multipleFiles)
	if want := []string{"/home/a/x.age", "/home/a/y z.age"}; err != nil || !reflect.DeepEqual(paths, want) {
		t.Fatalf("showZenityDialog = %q, %v; want %q", paths, err, want)
	}
}
