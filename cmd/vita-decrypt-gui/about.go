package main

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"
)

// scrimOpacity is how much of the forest color the scrim behind the About dialog shows.
const scrimOpacity = 0.45

// aboutDialog draws the scrim and the About dialog over the window.
func (app *App) aboutDialog(gtx layout.Context) {
	if app.aboutCloseButton.Clicked(gtx) || app.aboutCloseIcon.Clicked(gtx) {
		app.isAboutOpen = false
	}
	size := gtx.Constraints.Max
	// The scrim closes the dialog when clicked; the dialog itself swallows clicks.
	if wasAreaPressed(gtx, &app.scrimTag, image.Rectangle{Max: size}) {
		app.isAboutOpen = false
	}
	fillRect(gtx, image.Rectangle{Max: size}, translucent(app.colors.forestDeep, app.colors.canvas, scrimOpacity))

	width := min(pixels(gtx, 480), size.X-pixels(gtx, 32))
	recording := op.Record(gtx.Ops)
	dialog := gtx
	dialog.Constraints = layout.Constraints{Min: image.Pt(width, 0), Max: image.Pt(width, size.Y)}
	decoration := box{background: app.colors.surface, border: app.colors.border, borderWidth: 1, radius: 20}
	dims := decorate(dialog, decoration, layout.UniformInset(27), app.aboutContent)
	call := recording.Stop()
	corner := image.Pt((size.X-dims.Size.X)/2, max(0, (size.Y-dims.Size.Y)/2))
	defer op.Offset(corner).Push(gtx.Ops).Pop()
	swallowClicks(gtx, &app.dialogTag, dims.Size)
	call.Add(gtx.Ops)
}

func (app *App) aboutContent(gtx layout.Context) layout.Dimensions {
	colors := app.colors
	return column(gtx,
		layout.Rigid(fullWidth(app.aboutTopRow)),
		spacer(18),
		layout.Rigid(app.wordmark),
		spacer(3),
		layout.Rigid(app.text(styleCaption, colors.muted, "Version "+version)),
		spacer(18),
		layout.Rigid(app.text(styleBodyLoose, colors.foreground,
			"Vita Decrypt opens files that Vita sealed and creates the key pairs they’re sealed to. "+
				"It works offline and never sends anything over the internet.")),
		spacer(18),
		layout.Rigid(app.text(styleBodyLoose, colors.foreground,
			"Sealed files are standard age files, so other age tools can open them too.")),
		spacer(20),
		layout.Rigid(app.text(styleLegal, colors.muted, "© 2026 Vita Workflow")),
		layout.Rigid(app.text(styleLegal, colors.muted, "TAnderson Enterprises LLC, doing business as Vita Workflow")),
		spacer(22),
		layout.Rigid(fullWidth(func(gtx layout.Context) layout.Dimensions {
			return alignEnd(gtx, func(gtx layout.Context) layout.Dimensions {
				return app.button(gtx, &app.aboutCloseButton, button{kind: secondaryButton, text: "Close"})
			})
		})),
	)
}

// aboutTopRow is the app icon and the close button.
func (app *App) aboutTopRow(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Spacing: layout.SpaceBetween, Alignment: layout.Start}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			badge := box{background: appIconGreen, radius: 12}
			return squareBadge(gtx, 48, badge, app.icon("sprout", 32, 2, rgb(0xffffff)))
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return app.smallIconButton(gtx, &app.aboutCloseIcon, "x", 18)
		}),
	)
}
