//go:build linux || darwin

package desktop

import "strings"

// nonEmptyLines splits the newline-separated paths that zenity and the macOS glue code return.
func nonEmptyLines(text string) []string {
	var lines []string
	for line := range strings.SplitSeq(text, "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}
