package main

import (
	"log"

	gioapp "gioui.org/app"

	"github.com/vita-workflow/vita-decrypt/internal/desktop"
)

// attachView makes dialogs modal to the window and lets files be dropped on it.
func (app *App) attachView(viewEvent gioapp.ViewEvent) {
	view, ok := viewEvent.(gioapp.Win32ViewEvent)
	if !ok || !view.Valid() {
		return
	}
	desktop.SetOwner(view.HWND)
	handler := app.dropHandler()
	app.window.Run(func() {
		if err := desktop.AttachDrop(view.HWND, handler); err != nil {
			log.Print(err)
		}
	})
}
