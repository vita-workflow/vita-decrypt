package desktop

import (
	"os/exec"

	"github.com/godbus/dbus/v5"
)

// This file only connects linuxSystem to the real session bus and programs. The logic is in system_linux.go,
// portal_linux.go, and zenity_linux.go.

// CanDrop reports whether files can be dropped on the window. Gio owns the X11 and Wayland event loops, so on Linux
// they can't; the GUI accepts pasted files instead.
const CanDrop = false

// AttachDrop does nothing on Linux; see CanDrop.
func AttachDrop(view uintptr, handler DropHandler) error { return nil }

// SetOwner does nothing on Linux: the portal and zenity dialogs aren't tied to a window.
func SetOwner(view uintptr) {}

// Reveal opens the file manager at path's folder, selecting path where the file manager supports it.
func Reveal(path string) error { return realSystem().reveal(path) }

func showDialog(dialog Dialog, kind selection) ([]string, error) {
	return realSystem().showDialog(dialog, kind)
}

func realSystem() linuxSystem {
	return linuxSystem{
		connectBus:   connectSessionBus,
		findProgram:  exec.LookPath,
		runProgram:   runProgram,
		startProgram: startProgram,
	}
}

// connectSessionBus connects to the running session bus. Unlike dbus.ConnectSessionBus, it never starts one with
// dbus-launch, which would leave a bus daemon behind.
func connectSessionBus() (busConnection, error) {
	connection, err := dbus.SessionBusPrivateNoAutoStartup()
	if err != nil {
		return nil, err
	}
	if err := connection.Auth(nil); err != nil {
		_ = connection.Close() // the Auth error is the one worth reporting
		return nil, err
	}
	if err := connection.Hello(); err != nil {
		_ = connection.Close() // the Hello error is the one worth reporting
		return nil, err
	}
	return connection, nil
}

// runProgram runs a program and returns what it printed.
func runProgram(path string, arguments ...string) ([]byte, error) {
	// The program is zenity, found by exec.LookPath, and the arguments are built by zenityArguments. No shell is
	// involved, so nothing in them is interpreted.
	return exec.Command(path, arguments...).Output() //nolint:gosec // see the comment above
}

// startProgram starts a program without waiting for it to finish.
func startProgram(path string, arguments ...string) error {
	// The program is xdg-open, and its one argument is the folder to open. No shell is involved.
	command := exec.Command(path, arguments...) //nolint:gosec // see the comment above
	if err := command.Start(); err != nil {
		return err
	}
	go func() { _ = command.Wait() }() // reap it; some xdg-open fallbacks wait for the file manager to close
	return nil
}
