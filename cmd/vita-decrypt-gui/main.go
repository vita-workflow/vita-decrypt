// Command vita-decrypt-gui is the desktop version of vita-decrypt: it opens files Vita sealed and creates the key pairs
// they're sealed to, offline. It can also seal a file the way Vita does, to test a key pair.
package main

import (
	"log"
	"os"

	gioapp "gioui.org/app"
	"gioui.org/unit"
)

// version is set at build time with -ldflags "-X main.version=…".
var version = "dev"

func main() {
	window := new(gioapp.Window)
	window.Option(
		gioapp.Title("Vita Decrypt"),
		gioapp.Size(unit.Dp(960), unit.Dp(740)),
		gioapp.MinSize(unit.Dp(720), unit.Dp(560)),
	)
	app := newApp(window, nativeSystem())
	go func() {
		if err := app.run(); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	gioapp.Main()
}
