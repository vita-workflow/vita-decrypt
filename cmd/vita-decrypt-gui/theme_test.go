package main

import (
	"image/color"
	"testing"
)

func TestRGB(t *testing.T) {
	if got, want := rgb(0x2e6f40), (color.NRGBA{R: 0x2e, G: 0x6f, B: 0x40, A: 0xff}); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestWithAlpha(t *testing.T) {
	tests := []struct {
		opacity float32
		want    uint8
	}{
		{0, 0},
		{0.5, 128},
		{1, 255},
	}
	for _, test := range tests {
		if got := withAlpha(rgb(0x123456), test.opacity); got.A != test.want || got.R != 0x12 {
			t.Errorf("withAlpha(%v) = %v, want alpha %d", test.opacity, got, test.want)
		}
	}
}

func TestBlend(t *testing.T) {
	white, black := rgb(0xffffff), rgb(0x000000)
	tests := []struct {
		amount float32
		want   color.NRGBA
	}{
		{0, black},
		{1, white},
		{0.5, rgb(0x808080)},
	}
	for _, test := range tests {
		if got := blend(white, black, test.amount); got != test.want {
			t.Errorf("blend(white, black, %v) = %v, want %v", test.amount, got, test.want)
		}
	}
}

func TestTranslucent(t *testing.T) {
	base := rgb(0xfbfcf8)
	tests := []struct {
		name      string
		top       color.NRGBA
		opacity   float32
		wantAlpha uint8
	}{
		{"same color keeps the opacity", base, 0.4, 102},
		{"fully opaque", rgb(0x2e6f40), 1, 255},
		{"invisible", rgb(0x2e6f40), 0, 0},
	}
	for _, test := range tests {
		got := translucent(test.top, base, test.opacity)
		if got.A != test.wantAlpha || got.R != test.top.R {
			t.Errorf("%s: got %v, want alpha %d", test.name, got, test.wantAlpha)
		}
	}
	// Over a light base, a dark color at half opacity needs more than half alpha in linear light to look as dark.
	if got := translucent(rgb(0x000000), rgb(0xffffff), 0.5); got.A <= 128 {
		t.Errorf("black at half opacity over white has alpha %d, want more than 128", got.A)
	}
}

func TestLinearLight(t *testing.T) {
	tests := []struct {
		channel uint8
		want    float64
	}{
		{0, 0},
		{10, 10.0 / 255 / 12.92},
		{255, 1},
	}
	for _, test := range tests {
		if got := linearLight(test.channel); got < test.want-1e-9 || got > test.want+1e-9 {
			t.Errorf("linearLight(%d) = %v, want %v", test.channel, got, test.want)
		}
	}
}

func TestButtonColors(t *testing.T) {
	app := &App{colors: lightPalette}
	palette := lightPalette
	faded := func(original color.NRGBA) color.NRGBA { return blend(original, palette.surface, disabledOpacity) }
	tests := []struct {
		name       string
		kind       buttonKind
		isHovered  bool
		isDisabled bool
		want       buttonColors
	}{
		{"primary", primaryButton, false, false, buttonColors{palette.primary, palette.onPrimary, color.NRGBA{}}},
		{
			"primary hovered", primaryButton, true, false,
			buttonColors{palette.primaryHover, palette.onPrimary, color.NRGBA{}},
		},
		{
			"secondary", secondaryButton, false, false,
			buttonColors{palette.surface, palette.foreground, palette.borderStrong},
		},
		{
			"secondary hovered", secondaryButton, true, false,
			buttonColors{palette.surfaceAlt, palette.foreground, palette.borderStrong},
		},
		{"link", linkButton, false, false, buttonColors{foreground: palette.primaryText}},
		{"icon", iconButton, false, false, buttonColors{palette.surfaceAlt, palette.primaryText, palette.border}},
		{
			"disabled primary", primaryButton, false, true,
			buttonColors{faded(palette.primary), faded(palette.onPrimary), color.NRGBA{}},
		},
		{"disabled link", linkButton, false, true, buttonColors{foreground: faded(palette.primaryText)}},
	}
	for _, test := range tests {
		if got := app.buttonColors(test.kind, test.isHovered, test.isDisabled); got != test.want {
			t.Errorf("%s: got %v, want %v", test.name, got, test.want)
		}
	}
}

func TestMessageStyle(t *testing.T) {
	app := &App{colors: darkPalette}
	tests := []struct {
		kind messageKind
		want messageStyle
	}{
		{successMessage, messageStyle{
			darkPalette.success, darkPalette.successFill, darkPalette.successBorder,
			"circle-check",
		}},
		{warningMessage, messageStyle{
			darkPalette.warning, darkPalette.warningFill, darkPalette.warningBorder,
			"triangle-alert",
		}},
		{dangerMessage, messageStyle{darkPalette.danger, darkPalette.dangerFill, darkPalette.dangerBorder, "circle-x"}},
	}
	for _, test := range tests {
		if got := app.messageStyle(test.kind); got != test.want {
			t.Errorf("messageStyle(%d) = %v, want %v", test.kind, got, test.want)
		}
	}
	defer func() {
		if recover() == nil {
			t.Error("an unknown message kind didn't panic")
		}
	}()
	app.messageStyle(messageKind(99))
}
