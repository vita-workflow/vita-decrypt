package desktop

import (
	"reflect"
	"testing"
)

func TestPathsFromText_WindowsPaths(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		wantPaths []string
	}{
		{name: "a drive path is kept", text: `C:\Users\a\x.age`, wantPaths: []string{`C:\Users\a\x.age`}},
		{name: "quotes are removed", text: `"C:\Users\a\y z.age"`, wantPaths: []string{`C:\Users\a\y z.age`}},
		{name: "a file URI is decoded", text: "file:///C:/a/q%20r.age", wantPaths: []string{`C:\a\q r.age`}},
		{name: "plain words are skipped", text: "not a path", wantPaths: nil},
		{
			name: "a copied file list keeps its order",
			text: "C:\\Users\\a\\x.age\r\n\"C:\\Users\\a\\y z.age\"\r\n" +
				"file:///C:/Users/a/q%20r.age\r\nnot a path",
			wantPaths: []string{`C:\Users\a\x.age`, `C:\Users\a\y z.age`, `C:\Users\a\q r.age`},
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
