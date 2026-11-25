package desktop

import (
	"errors"
	"path/filepath"
	"testing"
)

// withoutDesktop points the session bus at a socket that doesn't exist and empties PATH, so the real Linux
// functions find neither the portal, zenity, nor xdg-open.
func withoutDesktop(t *testing.T) {
	t.Helper()
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path="+filepath.Join(t.TempDir(), "missing"))
	t.Setenv("PATH", t.TempDir())
}

func TestDialogs_WithoutADesktopAreUnavailable(t *testing.T) {
	withoutDesktop(t)
	if path, err := OpenFile(Dialog{Title: "Open"}); path != "" || !errors.Is(err, ErrUnavailable) {
		t.Fatalf("OpenFile = %q, %v; want ErrUnavailable", path, err)
	}
	if paths, err := OpenFiles(Dialog{Title: "Open"}); paths != nil || !errors.Is(err, ErrUnavailable) {
		t.Fatalf("OpenFiles = %q, %v; want ErrUnavailable", paths, err)
	}
	if path, err := ChooseFolder(Dialog{Title: "Choose"}); path != "" || !errors.Is(err, ErrUnavailable) {
		t.Fatalf("ChooseFolder = %q, %v; want ErrUnavailable", path, err)
	}
}

func TestReveal_WithoutADesktopFails(t *testing.T) {
	withoutDesktop(t)
	if err := Reveal(filepath.Join(t.TempDir(), "x.age")); err == nil {
		t.Fatal("Reveal succeeded without a file manager or xdg-open")
	}
}

func TestDrop_IsNotSupported(t *testing.T) {
	if CanDrop {
		t.Fatal("CanDrop is true on Linux")
	}
	if err := AttachDrop(0, DropHandler{}); err != nil {
		t.Fatalf("AttachDrop returned %v", err)
	}
	SetOwner(0)
}
