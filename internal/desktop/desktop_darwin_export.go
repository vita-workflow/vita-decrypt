//go:build darwin && cgo

package desktop

// #include <stdlib.h>
import "C"

// drops receives the drag events for the one window. AttachDrop sets its handler on the main thread, where AppKit
// also calls the functions below.
var drops = newDropQueue()

//export vitaDragEntered
func vitaDragEntered() { drops.enter() }

//export vitaDragExited
func vitaDragExited() { drops.leave() }

//export vitaDrop
func vitaDrop(paths *C.char) { drops.drop(C.GoString(paths)) }
