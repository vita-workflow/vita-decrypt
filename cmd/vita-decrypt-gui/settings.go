package main

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	lightTheme = "light"
	darkTheme  = "dark"
)

// themeSettingsPath is the file that remembers the chosen theme, or "" if there's no settings folder.
func themeSettingsPath() string {
	folder, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(folder, "vita-decrypt", "theme")
}

// loadDarkTheme reports whether the dark theme was chosen last time.
func loadDarkTheme() bool {
	path := themeSettingsPath()
	if path == "" {
		return false
	}
	content, err := os.ReadFile(path)
	return err == nil && strings.TrimSpace(string(content)) == darkTheme
}

// saveDarkTheme remembers the chosen theme. It's only a preference, so failing to save it is ignored.
func saveDarkTheme(isDark bool) {
	path := themeSettingsPath()
	if path == "" {
		return
	}
	theme := lightTheme
	if isDark {
		theme = darkTheme
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err == nil {
		_ = os.WriteFile(path, []byte(theme+"\n"), 0o600) // the app works the same without the saved preference
	}
}
