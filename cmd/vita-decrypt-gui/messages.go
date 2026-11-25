package main

import (
	"fmt"
	"image/color"

	"gioui.org/layout"
)

type messageKind int

const (
	successMessage messageKind = iota
	warningMessage
	dangerMessage
)

// messageStyle is how a kind of message looks.
type messageStyle struct {
	foreground, fill, border color.NRGBA
	icon                     string
}

func (app *App) messageStyle(kind messageKind) messageStyle {
	colors := app.colors
	switch kind {
	case successMessage:
		return messageStyle{colors.success, colors.successFill, colors.successBorder, "circle-check"}
	case warningMessage:
		return messageStyle{colors.warning, colors.warningFill, colors.warningBorder, "triangle-alert"}
	case dangerMessage:
		return messageStyle{colors.danger, colors.dangerFill, colors.dangerBorder, "circle-x"}
	}
	panic(fmt.Sprintf("unknown message kind %d", kind))
}

// messageBody lays out a message's content in the message's text color.
type messageBody func(gtx layout.Context, foreground color.NRGBA) layout.Dimensions

// message is the design's .message bar.
func (app *App) message(gtx layout.Context, kind messageKind, body messageBody) layout.Dimensions {
	style := app.messageStyle(kind)
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	decoration := box{background: style.fill, border: style.border, borderWidth: 1, radius: 10}
	return decorate(gtx, decoration, symmetricInset(11, 12), func(gtx layout.Context) layout.Dimensions {
		return rowTop(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: 1}.Layout(gtx, app.icon(style.icon, 18, 1.8, style.foreground))
			}),
			spacer(10),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return body(gtx, style.foreground) }),
		)
	})
}

// messageText is a message bar holding one sentence.
func (app *App) messageText(kind messageKind, content string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return app.message(gtx, kind, func(gtx layout.Context, foreground color.NRGBA) layout.Dimensions {
			return app.text(styleSmallTall, foreground, content)(gtx)
		})
	}
}
