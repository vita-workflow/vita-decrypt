package desktop

import (
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
)

// PathsFromText reads file paths from pasted text: one per line, as absolute paths or file:// URIs, the forms file
// managers put on the clipboard when files are copied. Lines that aren't paths are skipped.
func PathsFromText(text string) []string {
	var paths []string
	for _, line := range strings.FieldsFunc(text, isLineBreak) {
		line = strings.TrimSpace(line)
		if len(line) >= 2 && line[0] == '"' && line[len(line)-1] == '"' {
			line = line[1 : len(line)-1]
		}
		switch {
		case line == "" || strings.HasPrefix(line, "#"):
			continue
		case strings.HasPrefix(line, "file://"):
			if path, ok := pathFromURI(line); ok {
				paths = append(paths, path)
			}
		case filepath.IsAbs(line):
			paths = append(paths, filepath.Clean(line))
		}
	}
	return paths
}

func isLineBreak(character rune) bool { return character == '\n' || character == '\r' }

// pathFromURI turns a local file:// URI into a path.
func pathFromURI(uri string) (string, bool) {
	parsed, err := url.Parse(uri)
	if err != nil || parsed.Scheme != "file" || (parsed.Host != "" && parsed.Host != "localhost") ||
		parsed.Path == "" || parsed.Opaque != "" || parsed.RawQuery != "" || parsed.Fragment != "" ||
		parsed.ForceQuery {
		// A query or fragment means an unescaped ? or #; dropping it would name some other file.
		return "", false
	}
	path := parsed.Path
	// file:///C:/Users/... on Windows.
	if runtime.GOOS == "windows" && hasDriveLetter(path) {
		path = path[1:]
	}
	path = filepath.FromSlash(path)
	if !filepath.IsAbs(path) {
		return "", false
	}
	return filepath.Clean(path), true
}

// hasDriveLetter reports whether a URI path starts with a Windows drive, as in /C:/Users.
func hasDriveLetter(path string) bool {
	return len(path) >= 3 && path[0] == '/' && path[2] == ':' && isASCIILetter(path[1])
}

func isASCIILetter(character byte) bool {
	return 'a' <= character && character <= 'z' || 'A' <= character && character <= 'Z'
}
