//go:build !windows && !darwin

package main

import gioapp "gioui.org/app"

// attachView does nothing: Gio owns the X11 and Wayland windows, so files are pasted instead of dropped.
func (*App) attachView(gioapp.ViewEvent) {}
