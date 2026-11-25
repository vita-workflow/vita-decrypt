//go:build !linux && !windows && !(darwin && cgo)

package desktop

// CanDrop reports whether files can be dropped on the window.
const CanDrop = false

// SetOwner does nothing here.
func SetOwner(view uintptr) {}

// AttachDrop does nothing here.
func AttachDrop(view uintptr, handler DropHandler) error { return nil }

// Reveal is unavailable here.
func Reveal(path string) error { return ErrUnavailable }

func showDialog(dialog Dialog, kind selection) ([]string, error) { return nil, ErrUnavailable }
