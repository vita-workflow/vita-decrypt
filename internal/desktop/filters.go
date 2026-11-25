//go:build linux || windows

package desktop

// allFilesName names the filter that every filtered dialog adds last, so a file with an unexpected name can still be
// chosen.
const allFilesName = "All files"

// withAllFiles is filters followed by an "All files" filter matching allPattern, the platform's match-anything
// pattern. A dialog without filters stays without them.
func withAllFiles(filters []Filter, allPattern string) []Filter {
	if len(filters) == 0 {
		return nil
	}
	combined := make([]Filter, 0, len(filters)+1)
	combined = append(combined, filters...)
	return append(combined, Filter{Name: allFilesName, Patterns: []string{allPattern}})
}
