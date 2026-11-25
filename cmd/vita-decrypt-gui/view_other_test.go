//go:build !windows && !darwin

package main

import (
	gioapp "gioui.org/app"
	"gioui.org/io/event"
)

// viewEvents are the view events a test window sends before its first frame. Attaching an X11 view does nothing, so
// it's safe to send one.
var viewEvents = []event.Event{gioapp.X11ViewEvent{}}
