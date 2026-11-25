package main

import (
	"image"
	"image/color"
	"math"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"
)

type buttonKind int

const (
	primaryButton buttonKind = iota
	secondaryButton
	linkButton
	iconButton
)

// button is the design's .button.
type button struct {
	kind       buttonKind
	isLarge    bool
	isDisabled bool
	isBusy     bool // shows a spinner before the text
	icon       string
	iconSizeDp float32 // 16 if zero
	text       string
}

// buttonColors are a button's colors in its current state.
type buttonColors struct{ background, foreground, border color.NRGBA }

// buttonMetrics are a button's padding and minimum height, in dp.
type buttonMetrics struct{ paddingY, paddingX, minHeight float32 }

// disabledOpacity is how strongly a disabled button's colors show over the card.
const disabledOpacity = 0.46

func (app *App) button(gtx layout.Context, clickable *widget.Clickable, button button) layout.Dimensions {
	colors := app.buttonColors(button.kind, !button.isDisabled && clickable.Hovered(), button.isDisabled)
	draw := func(gtx layout.Context) layout.Dimensions { return app.paintButton(gtx, clickable, button, colors) }
	if button.isDisabled {
		return draw(gtx)
	}
	return app.clickable(gtx, clickable, 10, draw)
}

func (app *App) buttonColors(kind buttonKind, isHovered, isDisabled bool) buttonColors {
	palette := app.colors
	var colors buttonColors
	switch kind {
	case primaryButton:
		colors = buttonColors{background: palette.primary, foreground: palette.onPrimary}
		if isHovered {
			colors.background = palette.primaryHover
		}
	case secondaryButton:
		colors = buttonColors{background: palette.surface, foreground: palette.foreground, border: palette.borderStrong}
		if isHovered {
			colors.background = palette.surfaceAlt
		}
	case linkButton:
		colors.foreground = palette.primaryText
	case iconButton:
		colors = buttonColors{background: palette.surfaceAlt, foreground: palette.primaryText, border: palette.border}
	}
	if isDisabled {
		fade := func(original color.NRGBA) color.NRGBA {
			if original.A == 0 {
				return original
			}
			return blend(original, palette.surface, disabledOpacity)
		}
		colors = buttonColors{fade(colors.background), fade(colors.foreground), fade(colors.border)}
	}
	return colors
}

func (button button) metrics() buttonMetrics {
	switch {
	case button.kind == linkButton:
		return buttonMetrics{paddingY: 4, paddingX: 4, minHeight: 32}
	case button.kind == iconButton:
		return buttonMetrics{paddingY: 7, paddingX: 7, minHeight: 36}
	case button.isLarge:
		return buttonMetrics{paddingY: 11, paddingX: 18, minHeight: 44}
	}
	return buttonMetrics{paddingY: 7, paddingX: 14, minHeight: 36}
}

// paintButton draws the button's box with its content centered in it.
func (app *App) paintButton(
	gtx layout.Context, clickable *widget.Clickable, button button, colors buttonColors,
) layout.Dimensions {
	gtx.Constraints.Min = image.Point{}
	recording := op.Record(gtx.Ops)
	contentDims := app.buttonContent(gtx, button, colors.foreground)
	call := recording.Stop()
	metrics := button.metrics()
	borderWidth := pixels(gtx, 1)
	width := contentDims.Size.X + 2*pixels(gtx, metrics.paddingX) + 2*borderWidth
	height := max(contentDims.Size.Y+2*pixels(gtx, metrics.paddingY)+2*borderWidth, pixels(gtx, metrics.minHeight))
	if button.kind == iconButton {
		width = max(width, pixels(gtx, 36))
	}
	size := image.Pt(width, height)
	pressShift := 0
	if button.kind == primaryButton && clickable.Pressed() && !button.isDisabled {
		pressShift = pixels(gtx, 1)
	}
	defer op.Offset(image.Pt(0, pressShift)).Push(gtx.Ops).Pop()
	box{background: colors.background, border: colors.border, borderWidth: 1, radius: 10}.paint(gtx, size)
	shift := op.Offset(image.Pt((width-contentDims.Size.X)/2, (height-contentDims.Size.Y)/2)).Push(gtx.Ops)
	call.Add(gtx.Ops)
	shift.Pop()
	return layout.Dimensions{Size: size}
}

// buttonContent is the spinner or icon, then the text.
func (app *App) buttonContent(gtx layout.Context, button button, foreground color.NRGBA) layout.Dimensions {
	var items []layout.FlexChild
	switch {
	case button.isBusy:
		items = append(items, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return spinner(gtx, 15, foreground)
		}))
	case button.icon != "":
		iconSize := button.iconSizeDp
		if iconSize == 0 {
			iconSize = 16
		}
		items = append(items, layout.Rigid(app.icon(button.icon, iconSize, 2, foreground)))
	}
	if button.text != "" {
		if len(items) > 0 {
			items = append(items, spacer(7))
		}
		items = append(items, layout.Rigid(app.text(styleButton, foreground, button.text)))
	}
	return row(gtx, items...)
}

// spinner is a three-quarter ring that turns while something is working.
func spinner(gtx layout.Context, sizeDp float32, stroke color.NRGBA) layout.Dimensions {
	size := pixelsFloat(gtx, sizeDp)
	strokeWidth := pixelsFloat(gtx, 2)
	radius := (size - strokeWidth) / 2
	const turnsPerSecond = 1.25
	angle := float32(math.Mod(float64(gtx.Now.UnixMilli())/1000*2*math.Pi*turnsPerSecond, 2*math.Pi))
	middle := f32.Pt(size/2, size/2)
	defer op.Affine(f32.Affine2D{}.Rotate(middle, angle)).Push(gtx.Ops).Pop()
	var path clip.Path
	path.Begin(gtx.Ops)
	path.MoveTo(f32.Pt(middle.X+radius, middle.Y))
	path.ArcTo(middle, middle, 1.5*math.Pi)
	paintClip(gtx, clip.Stroke{Path: path.End(), Width: strokeWidth}.Op(), stroke)
	gtx.Execute(op.InvalidateCmd{})
	sizePixels := int(math.Ceil(float64(size)))
	return layout.Dimensions{Size: image.Pt(sizePixels, sizePixels)}
}

// ghostStyle is a borderless control that tints on hover: navigation items, the About link, and small icon buttons.
type ghostStyle struct {
	minHeightDp, paddingXDp, radius float32
	background, hoverBackground     color.NRGBA
	isFullWidth                     bool
}

func (app *App) ghostButton(
	gtx layout.Context, clickable *widget.Clickable, style ghostStyle, content layout.Widget,
) layout.Dimensions {
	return app.clickable(gtx, clickable, style.radius, func(gtx layout.Context) layout.Dimensions {
		if !style.isFullWidth {
			gtx.Constraints.Min.X = 0
		}
		gtx.Constraints.Min.Y = 0
		recording := op.Record(gtx.Ops)
		padding := layout.Inset{Left: unit.Dp(style.paddingXDp), Right: unit.Dp(style.paddingXDp)}
		dims := padding.Layout(gtx, minHeight(style.minHeightDp, content))
		call := recording.Stop()
		if style.isFullWidth {
			dims.Size.X = gtx.Constraints.Max.X
		}
		background := style.background
		if clickable.Hovered() && style.hoverBackground.A > 0 {
			background = style.hoverBackground
		}
		box{background: background, radius: style.radius}.paint(gtx, dims.Size)
		call.Add(gtx.Ops)
		return dims
	})
}

// smallIconButton is the design's .icon-button: 32×32, muted, tinted on hover.
func (app *App) smallIconButton(
	gtx layout.Context, clickable *widget.Clickable, name string, sizeDp float32,
) layout.Dimensions {
	foreground := app.colors.muted
	if clickable.Hovered() {
		foreground = app.colors.foreground
	}
	style := ghostStyle{minHeightDp: 32, radius: 8, hoverBackground: app.colors.surface}
	return app.ghostButton(gtx, clickable, style, func(gtx layout.Context) layout.Dimensions {
		return center(gtx, image.Pt(pixels(gtx, 32), pixels(gtx, 32)), app.icon(name, sizeDp, 2, foreground))
	})
}

// textLink is the design's underlined inline link.
func (app *App) textLink(
	gtx layout.Context, clickable *widget.Clickable, style textStyle, foreground color.NRGBA, content string,
) layout.Dimensions {
	link := label{style: style, color: foreground, text: content, isUnderlined: true}
	return app.clickable(gtx, clickable, 3, app.label(link))
}
