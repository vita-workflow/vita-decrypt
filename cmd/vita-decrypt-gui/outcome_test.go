package main

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

func TestOutcome_NamesTheExistingFile(t *testing.T) {
	input := filepath.Join(t.TempDir(), "report.pdf.age")
	result := outcome(input, "", fmt.Errorf("won't overwrite: %w", fs.ErrExist))
	if result.kind != warningMessage || !strings.HasPrefix(result.text, "report.pdf already exists") {
		t.Errorf("got %v %q", result.kind, result.text)
	}
}
