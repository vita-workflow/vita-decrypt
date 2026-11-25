package main

import (
	"image"
	"image/color"
	"time"

	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/widget"
)

// Icon names and texts used in more than one place.
const (
	checkIcon  = "check"
	copyIcon   = "copy"
	keyIcon    = "key-round"
	fileUpIcon = "file-up"

	chooseFileText   = "Choose file…"
	showInFolderText = "Show in folder"
)

// text is a widget for content in one style and color.
func (app *App) text(style textStyle, foreground color.NRGBA, content string) layout.Widget {
	return app.label(label{style: style, color: foreground, text: content})
}

// label is a widget for a label with more options than text has.
func (app *App) label(label label) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions { return label.layout(gtx, app.shaper) }
}

// icon is a widget for the named icon.
func (*App) icon(name string, sizeDp, strokeWidth float32, stroke color.NRGBA) layout.Widget {
	drawing := loadIcon(name)
	return func(gtx layout.Context) layout.Dimensions { return drawing.layout(gtx, sizeDp, strokeWidth, stroke) }
}

// clickable makes content respond to clickable, with a pointer cursor and a focus ring of the given corner radius.
func (app *App) clickable(
	gtx layout.Context, clickable *widget.Clickable, radius float32, content layout.Widget,
) layout.Dimensions {
	dims := clickable.Layout(gtx, content)
	showPointer(gtx, dims.Size)
	if gtx.Focused(clickable) {
		app.focusRing(gtx, dims.Size, radius)
	}
	return dims
}

// showPointer shows the pointing-hand cursor over size.
func showPointer(gtx layout.Context, size image.Point) {
	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
	pointer.CursorPointer.Add(gtx.Ops)
}

// focusRing draws the design's focus indicator: a 2 dp gap in the surface color, then a 2 dp ring, outside size.
func (app *App) focusRing(gtx layout.Context, size image.Point, radius float32) {
	outset := pixels(gtx, 4)
	defer op.Offset(image.Pt(-outset, -outset)).Push(gtx.Ops).Pop()
	outer := size.Add(image.Pt(2*outset, 2*outset))
	box{border: app.colors.surface, borderWidth: 4, radius: radius + 4}.paint(gtx, outer)
	box{border: app.colors.ring, borderWidth: 2, radius: radius + 4}.paint(gtx, outer)
}

// card is the design's .card: surface, 1 dp border, 14 dp radius, full width.
func (app *App) card(gtx layout.Context, padding layout.Inset, content layout.Widget) layout.Dimensions {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	decoration := box{background: app.colors.surface, border: app.colors.border, borderWidth: 1, radius: 14}
	return decorate(gtx, decoration, padding, fullWidth(content))
}

// checkbox is the design's .checkbox.
func (app *App) checkbox(gtx layout.Context, checkbox *widget.Bool, content string) layout.Dimensions {
	colors := app.colors
	dims := checkbox.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return row(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				size := image.Pt(pixels(gtx, 19), pixels(gtx, 19))
				decoration := box{background: colors.surface, border: colors.borderStrong, borderWidth: 1, radius: 5}
				if checkbox.Value {
					decoration.background, decoration.border = colors.primary, colors.primary
				}
				decoration.paint(gtx, size)
				if checkbox.Value {
					center(gtx, size, app.icon(checkIcon, 14, 2, colors.onPrimary))
				}
				if gtx.Focused(checkbox) {
					app.focusRing(gtx, size, 5)
				}
				return layout.Dimensions{Size: size}
			}),
			spacer(9),
			layout.Rigid(app.text(styleSmall, colors.foreground, content)),
		)
	})
	showPointer(gtx, dims.Size)
	return dims
}

// pill is the design's small uppercase tag.
func (app *App) pill(gtx layout.Context, content string) layout.Dimensions {
	return decorate(gtx, box{background: app.colors.surfaceAlt, radius: 999}, symmetricInset(2, 7),
		app.text(stylePill, app.colors.primaryText, content))
}

// heading is a page header: a Fraunces title and a muted line under it.
func (app *App) heading(title, subtitle string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: 2, Bottom: 22}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return column(gtx,
				layout.Rigid(app.text(styleTitle, app.colors.foreground, title)),
				spacer(7),
				layout.Rigid(app.text(styleBody, app.colors.muted, subtitle)),
			)
		})
	}
}

// copyConfirmation is how long a button shows "Copied" after copying.
const copyConfirmation = 2 * time.Second

// isCopyConfirmed reports whether a copy confirmation started at copiedAt is still showing, scheduling a redraw for
// when it ends.
func isCopyConfirmed(gtx layout.Context, copiedAt time.Time) bool {
	if copiedAt.IsZero() {
		return false
	}
	end := copiedAt.Add(copyConfirmation)
	if !gtx.Now.Before(end) {
		return false
	}
	gtx.Execute(op.InvalidateCmd{At: end})
	return true
}

// divider is a 1 dp horizontal rule.
func (app *App) divider(gtx layout.Context) layout.Dimensions {
	size := image.Pt(gtx.Constraints.Max.X, max(1, pixels(gtx, 1)))
	fillRect(gtx, image.Rectangle{Max: size}, app.colors.border)
	return layout.Dimensions{Size: size}
}

// withRuleBelow lays content out with a 1 dp rule under it.
func (app *App) withRuleBelow(gtx layout.Context, content layout.Widget) layout.Dimensions {
	dims := content(gtx)
	ruleWidth := max(1, pixels(gtx, 1))
	fillRect(gtx, image.Rect(0, dims.Size.Y, dims.Size.X, dims.Size.Y+ruleWidth), app.colors.border)
	dims.Size.Y += ruleWidth
	return dims
}

// wasAreaPressed registers area as a click target for tag and reports whether it was pressed since the last frame.
func wasAreaPressed(gtx layout.Context, tag event.Tag, area image.Rectangle) bool {
	isPressed := false
	for {
		received, ok := gtx.Event(pointer.Filter{Target: tag, Kinds: pointer.Press})
		if !ok {
			break
		}
		if press, ok := received.(pointer.Event); ok && press.Kind == pointer.Press {
			isPressed = true
		}
	}
	defer clip.Rect(area).Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, tag)
	return isPressed
}

// swallowClicks stops clicks in size from reaching anything drawn before it.
func swallowClicks(gtx layout.Context, tag event.Tag, size image.Point) {
	for {
		if _, ok := gtx.Event(pointer.Filter{Target: tag, Kinds: pointer.Press | pointer.Release}); !ok {
			break
		}
	}
	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, tag)
}
