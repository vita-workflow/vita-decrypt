package main

import (
	"log"

	gioapp "gioui.org/app"

	"github.com/vita-workflow/vita-decrypt/internal/desktop"
)

// attachView lets files be dropped on the window.
func (app *App) attachView(viewEvent gioapp.ViewEvent) {
	view, ok := viewEvent.(gioapp.AppKitViewEvent)
	if !ok || !view.Valid() {
		return
	}
	handler := app.dropHandler()
	app.window.Run(func() {
		if err := desktop.AttachDrop(view.View, handler); err != nil {
			log.Print(err)
		}
	})
}
