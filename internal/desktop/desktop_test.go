package desktop

import (
	"errors"
	"testing"
)

func TestFirstPath(t *testing.T) {
	errDialog := errors.New("the dialog failed")
	tests := []struct {
		name     string
		paths    []string
		err      error
		wantPath string
		wantErr  error
	}{
		{name: "one path returns it", paths: []string{"/home/a/x.age"}, wantPath: "/home/a/x.age"},
		{
			name:     "several paths return the first",
			paths:    []string{"/home/a/x.age", "/home/a/y.age"},
			wantPath: "/home/a/x.age",
		},
		{name: "no paths return nothing", paths: nil, wantPath: ""},
		{name: "an error returns no path", paths: []string{"/home/a/x.age"}, err: errDialog, wantErr: errDialog},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path, err := firstPath(test.paths, test.err)
			if path != test.wantPath || !errors.Is(err, test.wantErr) {
				t.Fatalf("firstPath = %q, %v; want %q, %v", path, err, test.wantPath, test.wantErr)
			}
		})
	}
}
