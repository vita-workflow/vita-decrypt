package desktop

import (
	"errors"
	"net/url"
	"path/filepath"

	"github.com/godbus/dbus/v5"
)

// linuxSystem is everything the Linux dialogs and Reveal need from the computer. desktop_linux.go fills it with the
// real session bus and programs; tests fill it with fakes.
type linuxSystem struct {
	connectBus   func() (busConnection, error)
	findProgram  func(name string) (string, error)
	runProgram   func(path string, arguments ...string) ([]byte, error)
	startProgram func(path string, arguments ...string) error
}

// busConnection is the part of a *dbus.Conn that the portal and the file manager calls use.
type busConnection interface {
	Names() []string
	AddMatchSignal(options ...dbus.MatchOption) error
	Signal(channel chan<- *dbus.Signal)
	Object(destination string, path dbus.ObjectPath) dbus.BusObject
	Close() error
}

// The org.freedesktop.FileManager1 D-Bus API, which file managers provide to show a file selected.
const (
	fileManagerDestination = "org.freedesktop.FileManager1"
	fileManagerPath        = dbus.ObjectPath("/org/freedesktop/FileManager1")
	showItemsMethod        = "org.freedesktop.FileManager1.ShowItems"
)

// showDialog tries the XDG desktop portal, which every current desktop provides and which works inside sandboxes,
// then zenity.
func (system linuxSystem) showDialog(dialog Dialog, kind selection) ([]string, error) {
	paths, err := system.showPortalDialog(dialog, kind)
	if !errors.Is(err, errNoPortal) {
		return paths, err
	}
	return system.showZenityDialog(dialog, kind)
}

// showPortalDialog asks the desktop portal. It returns errNoPortal if there is no portal to ask.
func (system linuxSystem) showPortalDialog(dialog Dialog, kind selection) ([]string, error) {
	connection, err := system.connectBus()
	if err != nil {
		return nil, errNoPortal
	}
	defer func() { _ = connection.Close() }() // the answer is already in hand; a failed close changes nothing
	return askPortal(connection, dialog, kind)
}

// reveal asks the file manager to show path selected, and if that fails, opens path's folder with xdg-open.
func (system linuxSystem) reveal(path string) error {
	if err := system.showItems(path); err == nil {
		return nil
	}
	return system.startProgram("xdg-open", filepath.Dir(path))
}

func (system linuxSystem) showItems(path string) error {
	connection, err := system.connectBus()
	if err != nil {
		return err
	}
	defer func() { _ = connection.Close() }() // the call's own result is what matters
	uri := url.URL{Scheme: "file", Path: path}
	return connection.Object(fileManagerDestination, fileManagerPath).
		Call(showItemsMethod, 0, []string{uri.String()}, "").Err
}
