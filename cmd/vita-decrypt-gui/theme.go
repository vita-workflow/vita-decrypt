package main

import (
	"image/color"
	"math"
)

// palette is one color mode of the Vita Decrypt design, matching Vita's colors.
type palette struct {
	canvas, surface, surfaceAlt                color.NRGBA
	foreground, muted, border, borderStrong    color.NRGBA
	primary, onPrimary, primaryHover           color.NRGBA
	primaryText, ring                          color.NRGBA
	forest, forestDeep, forestForeground, fern color.NRGBA
	success, successFill, successBorder        color.NRGBA
	warning, warningFill, warningBorder        color.NRGBA
	danger, dangerFill, dangerBorder           color.NRGBA
	scrollThumb                                color.NRGBA
}

// rgb is the opaque color written in CSS as #rrggbb.
func rgb(hex uint32) color.NRGBA {
	return color.NRGBA{R: uint8(hex >> 16 & 0xff), G: uint8(hex >> 8 & 0xff), B: uint8(hex & 0xff), A: 0xff}
}

// withAlpha returns base at opacity (0–1).
func withAlpha(base color.NRGBA, opacity float32) color.NRGBA {
	base.A = uint8(float32(base.A)*opacity + 0.5)
	return base
}

//nolint:dupl // The light and dark palettes are parallel tables of the same fields; looking alike is the point.
var lightPalette = palette{
	canvas: rgb(0xf4f7f1), surface: rgb(0xfbfcf8), surfaceAlt: rgb(0xeef4ea),
	foreground: rgb(0x1a3d25), muted: rgb(0x4a6655), border: rgb(0xd3e2d8), borderStrong: rgb(0x6f8a78),
	primary: rgb(0x2e6f40), onPrimary: rgb(0xfbfdf9), primaryHover: rgb(0x256532),
	primaryText: rgb(0x2e6f40), ring: rgb(0x2e6f40),
	forest: rgb(0x1a3d25), forestDeep: rgb(0x143024), forestForeground: rgb(0xeef4ea), fern: rgb(0x62d778),
	success: rgb(0x0e700e), successFill: rgb(0xf1faf1), successBorder: rgb(0x9fd89f),
	warning: rgb(0xbc4b09), warningFill: rgb(0xfff9f5), warningBorder: rgb(0xfdcfb4),
	danger: rgb(0xb10e1c), dangerFill: rgb(0xfdf3f4), dangerBorder: rgb(0xeeacb2),
	scrollThumb: withAlpha(rgb(0x6f8a78), 0.45),
}

//nolint:dupl // The light and dark palettes are parallel tables of the same fields; looking alike is the point.
var darkPalette = palette{
	canvas: rgb(0x101511), surface: rgb(0x171e19), surfaceAlt: rgb(0x1f2921),
	foreground: rgb(0xedf6ed), muted: rgb(0xa3c9b0), border: rgb(0x243d2e), borderStrong: rgb(0x5d7a68),
	primary: rgb(0x4fbe64), onPrimary: rgb(0x101511), primaryHover: rgb(0x62d778),
	primaryText: rgb(0x4fbe64), ring: rgb(0x4fbe64),
	forest: rgb(0x143024), forestDeep: rgb(0x0d1f16), forestForeground: rgb(0xeef4ea), fern: rgb(0x74e389),
	success: rgb(0x54b054), successFill: rgb(0x052505), successBorder: rgb(0x107c10),
	warning: rgb(0xfaa06b), warningFill: rgb(0x4a1e04), warningBorder: rgb(0xf7630c),
	danger: rgb(0xdc626d), dangerFill: rgb(0x3b0509), dangerBorder: rgb(0xc50f1f),
	scrollThumb: withAlpha(rgb(0x5d7a68), 0.6),
}

// appIconGreen is the Vita app icon's fixed background, the same in both modes.
var appIconGreen = rgb(0x2e6f40)

// blend is CSS's color-mix in sRGB: amount of top over base, opaque. Gio blends translucent colors in linear light,
// which looks lighter than the design, so the design's opacities are mixed here instead.
func blend(top, base color.NRGBA, amount float32) color.NRGBA {
	channel := func(over, under uint8) uint8 {
		return uint8(float32(over)*amount + float32(under)*(1-amount) + 0.5)
	}
	return color.NRGBA{R: channel(top.R, base.R), G: channel(top.G, base.G), B: channel(top.B, base.B), A: 255}
}

// translucent returns top with the alpha that, blended in linear light over base, gives what CSS shows for top at
// opacity over base. It's exact over base and close over colors near it.
func translucent(top, base color.NRGBA, opacity float32) color.NRGBA {
	luminanceSum := func(value color.NRGBA) float64 {
		return linearLight(value.R) + linearLight(value.G) + linearLight(value.B)
	}
	want := blend(top, base, opacity)
	difference := luminanceSum(top) - luminanceSum(base)
	if math.Abs(difference) < 1e-6 {
		return withAlpha(top, opacity)
	}
	alpha := (luminanceSum(want) - luminanceSum(base)) / difference
	return withAlpha(top, float32(min(max(alpha, 0), 1)))
}

// linearLight converts an sRGB channel to linear light (0–1).
func linearLight(channel uint8) float64 {
	value := float64(channel) / 255
	if value <= 0.04045 {
		return value / 12.92
	}
	return math.Pow((value+0.055)/1.055, 2.4)
}
