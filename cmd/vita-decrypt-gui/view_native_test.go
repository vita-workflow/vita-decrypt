//go:build windows || darwin

package main

import "gioui.org/io/event"

// viewEvents are the view events a test window sends before its first frame: none here, since attaching a view
// registers the window with the system for drops, which needs a real window.
var viewEvents []event.Event
