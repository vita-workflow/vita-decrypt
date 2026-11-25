package main

import (
	"errors"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"gioui.org/f32"
)

func TestParseIcon(t *testing.T) {
	const header = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24">`
	tests := []struct {
		name         string
		svg          string
		wantSubpaths int
		wantClosed   bool
		wantStart    f32.Point
	}{
		{"path", header + `<path d="M1 2 L3 4"/></svg>`, 1, false, f32.Pt(1, 2)},
		{"circle", header + `<circle cx="12" cy="12" r="10"/></svg>`, 1, true, f32.Pt(22, 12)},
		{"rect", header + `<rect x="2" y="3" width="20" height="18" rx="2"/></svg>`, 1, true, f32.Pt(4, 3)},
		{"rect with ry only", header + `<rect width="10" height="10" ry="3"/></svg>`, 1, true, f32.Pt(3, 0)},
		{"square rect", header + `<rect x="1" y="1" width="4" height="4"/></svg>`, 1, true, f32.Pt(1, 1)},
		{"line", header + `<line x1="5" y1="6" x2="7" y2="8"/></svg>`, 1, false, f32.Pt(5, 6)},
		{"polyline", header + `<polyline points="1 2 3 4 5 6 7"/></svg>`, 1, false, f32.Pt(1, 2)},
		{"several shapes", header + `<path d="M0 0 L1 1 M2 2 L3 3"/><line x2="1"/></svg>`, 3, false, f32.Pt(0, 0)},
		{"truncated document", header + `<path d="M1 1 L2 2"/>`, 1, false, f32.Pt(1, 1)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed, err := parseIcon(test.svg)
			if err != nil {
				t.Fatal(err)
			}
			first := parsed.subpaths[0]
			if len(parsed.subpaths) != test.wantSubpaths || first.isClosed != test.wantClosed ||
				!near(first.start, test.wantStart) {
				t.Errorf("got %d subpaths, the first closed %v starting at %v", len(parsed.subpaths),
					first.isClosed, first.start)
			}
		})
	}
}

func TestParseIcon_Errors(t *testing.T) {
	tests := []struct {
		name, svg, want string
	}{
		{"empty", "", "no shapes"},
		{"no shapes", `<svg></svg>`, "no shapes"},
		{"unsupported element", `<svg><ellipse rx="1"/></svg>`, "unsupported element ellipse"},
		{"bad path", `<svg><path d="1 2"/></svg>`, "starts without a command"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseIcon(test.svg)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Errorf("error %v, want one mentioning %q", err, test.want)
			}
		})
	}
}

func TestCornerRadius(t *testing.T) {
	tests := []struct {
		name       string
		attributes svgAttributes
		want       f32.Point
	}{
		{"neither", svgAttributes{}, f32.Pt(0, 0)},
		{"rx", svgAttributes{"rx": "2"}, f32.Pt(2, 2)},
		{"ry", svgAttributes{"ry": "3"}, f32.Pt(3, 3)},
		{"both", svgAttributes{"rx": "2", "ry": "3"}, f32.Pt(2, 3)},
		{"malformed", svgAttributes{"rx": "wide"}, f32.Pt(0, 0)},
	}
	for _, test := range tests {
		if got := test.attributes.cornerRadius(); got != test.want {
			t.Errorf("%s: got %v, want %v", test.name, got, test.want)
		}
	}
}

func TestParseIcons(t *testing.T) {
	const valid = `<svg><path d="M0 0 L1 1"/></svg>`
	icons, err := parseIcons(fstest.MapFS{
		"icons/plus.svg":  {Data: []byte(valid)},
		"icons/notes.txt": {Data: []byte("not an icon")},
	})
	if err != nil || len(icons) != 1 || icons["plus"] == nil {
		t.Errorf("got %v, %v; want just plus", icons, err)
	}

	_, err = parseIcons(fstest.MapFS{"icons/broken.svg": {Data: []byte("<svg></svg>")}})
	if err == nil || !strings.Contains(err.Error(), "icons/broken.svg") {
		t.Errorf("a broken icon gave %v", err)
	}
	_, err = parseIcons(fstest.MapFS{"icons/locked.svg": {Data: []byte(valid), Mode: fs.ModeDir}})
	if err == nil {
		t.Error("an unreadable icon gave no error")
	}
	_, err = parseIcons(globFailure{})
	if err == nil {
		t.Error("a file system that can't be listed gave no error")
	}
}

// globFailure is a file system that fails every listing.
type globFailure struct{}

func (globFailure) Open(string) (fs.File, error) { return nil, fs.ErrPermission }

func (globFailure) Glob(string) ([]string, error) { return nil, errors.New("cannot list") }

func TestMustParseIcons_PanicsOnABrokenIcon(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("a broken icon didn't panic")
		}
	}()
	mustParseIcons(fstest.MapFS{"icons/broken.svg": {Data: []byte("<svg></svg>")}})
}

func TestLoadIcon(t *testing.T) {
	if loadIcon(checkIcon) == nil {
		t.Error("an embedded icon wasn't found")
	}
	defer func() {
		if recover() == nil {
			t.Error("an unknown icon didn't panic")
		}
	}()
	loadIcon("no-such-icon")
}

func FuzzParseIcon(f *testing.F) {
	for _, name := range []string{"check", "copy", "key-round", "file-up"} {
		data, err := iconFiles.ReadFile("icons/" + name + ".svg")
		if err != nil {
			f.Fatal(err)
		}
		f.Add(string(data))
	}
	f.Fuzz(func(t *testing.T, svg string) {
		parsed, err := parseIcon(svg)
		if err != nil {
			if parsed != nil {
				t.Errorf("parseIcon(%q) returned an icon with error %v", svg, err)
			}
			return
		}
		if len(parsed.subpaths) == 0 {
			t.Errorf("parseIcon(%q) returned an icon with no shapes", svg)
		}
	})
}
