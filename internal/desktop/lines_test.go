//go:build linux || darwin

package desktop

import (
	"reflect"
	"testing"
)

func TestNonEmptyLines(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		wantLines []string
	}{
		{name: "empty text has no lines", text: "", wantLines: nil},
		{name: "only newlines have no lines", text: "\n\n", wantLines: nil},
		{name: "one line without a newline is kept", text: "/a", wantLines: []string{"/a"}},
		{name: "a trailing newline is dropped", text: "/a\n", wantLines: []string{"/a"}},
		{name: "empty lines between are dropped", text: "/a\n\n/b c\n", wantLines: []string{"/a", "/b c"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if lines := nonEmptyLines(test.text); !reflect.DeepEqual(lines, test.wantLines) {
				t.Fatalf("nonEmptyLines(%q) = %q, want %q", test.text, lines, test.wantLines)
			}
		})
	}
}
