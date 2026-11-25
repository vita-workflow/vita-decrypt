package desktop

import (
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestPathsFromText(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("these are Unix paths; paths_windows_test.go has the Windows ones")
	}
	tests := []struct {
		name      string
		text      string
		wantPaths []string
	}{
		{name: "an absolute path is kept", text: "/home/a/Report.age", wantPaths: []string{"/home/a/Report.age"}},
		{
			name:      "a file URI is decoded",
			text:      "file:///home/a/Photo%20archive.zip.age",
			wantPaths: []string{"/home/a/Photo archive.zip.age"},
		},
		{name: "a localhost URI is local", text: "file://localhost/home/a/b.age", wantPaths: []string{"/home/a/b.age"}},
		{name: "a URI on another host is skipped", text: "file://other-host/x.age", wantPaths: nil},
		{name: "a web URL is skipped", text: "https://example.com/x.age", wantPaths: nil},
		{name: "a relative path is skipped", text: "relative/x.age", wantPaths: nil},
		{name: "a comment is skipped", text: "# copied files", wantPaths: nil},
		{name: "plain words are skipped", text: "just some words", wantPaths: nil},
		{name: "quotes are removed", text: `"/home/a/quoted name.age"`, wantPaths: []string{"/home/a/quoted name.age"}},
		{name: "a path is cleaned", text: "/home/a/../a/clean.age", wantPaths: []string{"/home/a/clean.age"}},
		{name: "a URI with a fragment is skipped", text: "file:///home/a/a#b.age", wantPaths: nil},
		{name: "a URI with a query is skipped", text: "file:///home/a/a?b.age", wantPaths: nil},
		{name: "an empty query is skipped", text: "file:///home/a/a?", wantPaths: nil},
		{name: "an escaped # is kept", text: "file:///home/a/a%23c.age", wantPaths: []string{"/home/a/a#c.age"}},
		{name: "a drive letter is part of a Unix path", text: "file:///m:/x.age", wantPaths: []string{"/m:/x.age"}},
		{name: "an opaque URI is skipped", text: "file:home/a/x.age", wantPaths: nil},
		{name: "a URI without a path is skipped", text: "file://", wantPaths: nil},
		{name: "a URI that doesn't parse is skipped", text: "file:///home/%zz", wantPaths: nil},
		{name: "spaces around a line are trimmed", text: "  /home/a/x.age \t", wantPaths: []string{"/home/a/x.age"}},
		{
			name: "a copied file list keeps its order and skips the rest",
			text: "# copied files\n/home/a/Report.pdf.age\nfile:///home/a/Photo%20archive.zip.age\r\n" +
				"file://localhost/home/a/b.age\nfile://other-host/x.age\nhttps://example.com/x.age\n" +
				"relative/x.age\n\n" +
				"\"/home/a/quoted name.age\"\n/home/a/../a/clean.age\n" +
				"file:///home/a/a#b.age\nfile:///home/a/a?b.age\nfile:///home/a/a%23c.age\nfile:///m:/x.age",
			wantPaths: []string{
				"/home/a/Report.pdf.age",
				"/home/a/Photo archive.zip.age",
				"/home/a/b.age",
				"/home/a/quoted name.age",
				"/home/a/clean.age",
				"/home/a/a#c.age",
				"/m:/x.age",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if paths := PathsFromText(test.text); !reflect.DeepEqual(paths, test.wantPaths) {
				t.Fatalf("PathsFromText(%q) = %q, want %q", test.text, paths, test.wantPaths)
			}
		})
	}
}

func TestHasDriveLetter(t *testing.T) {
	tests := []struct {
		name          string
		path          string
		wantHasLetter bool
	}{
		{name: "an upper-case drive has one", path: "/C:/Users", wantHasLetter: true},
		{name: "a lower-case drive has one", path: "/z:/", wantHasLetter: true},
		{name: "a drive alone has one", path: "/c:", wantHasLetter: true},
		{name: "a Unix path has none", path: "/home/a", wantHasLetter: false},
		{name: "a digit isn't a drive", path: "/1:/x", wantHasLetter: false},
		{name: "a symbol isn't a drive", path: "/@:/x", wantHasLetter: false},
		{name: "a letter after Z isn't a drive", path: "/[:/x", wantHasLetter: false},
		{name: "a letter after z isn't a drive", path: "/{:/x", wantHasLetter: false},
		{name: "a path without a leading slash has none", path: "C:/x", wantHasLetter: false},
		{name: "a short path has none", path: "/C", wantHasLetter: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if hasLetter := hasDriveLetter(test.path); hasLetter != test.wantHasLetter {
				t.Fatalf("hasDriveLetter(%q) = %v, want %v", test.path, hasLetter, test.wantHasLetter)
			}
		})
	}
}

// FuzzPathsFromText checks that pasted text never makes PathsFromText panic, and that every path it returns is
// absolute and clean.
func FuzzPathsFromText(f *testing.F) {
	f.Fuzz(func(t *testing.T, text string) {
		for _, path := range PathsFromText(text) {
			checkCleanAbsolute(t, path)
		}
	})
}

// FuzzPathFromURI checks that pathFromURI never panics, and that every path it accepts is absolute and clean.
func FuzzPathFromURI(f *testing.F) {
	f.Fuzz(func(t *testing.T, uri string) {
		if path, ok := pathFromURI(uri); ok {
			checkCleanAbsolute(t, path)
		}
	})
}

func checkCleanAbsolute(t *testing.T, path string) {
	t.Helper()
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		t.Fatalf("%q isn't a clean absolute path", path)
	}
}
