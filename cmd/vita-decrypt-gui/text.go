package main

import (
	"bytes"
	_ "embed"
	"image"
	"image/color"
	"math"
	"strings"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/text"
	fontapi "github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/font/opentype"
	"golang.org/x/image/math/fixed"
)

// The design's two variable fonts, both under the SIL Open Font License (fonts/*-OFL.txt).
var (
	//go:embed fonts/Fraunces.ttf
	frauncesTTF []byte
	//go:embed fonts/Inter.ttf
	interTTF []byte
)

// interTypeface is the design's body font, which editors use directly as well as through the text styles.
const interTypeface = "Inter"

// fontInstance is one fixed setting of a variable font's axes, registered with the shaper as its own typeface. A
// browser picks the optical size from the font size; Gio doesn't, so each size the design uses gets an instance.
type fontInstance struct {
	typeface    font.Typeface
	weight      font.Weight
	data        []byte
	weightAxis  float32
	opticalSize float32
}

var fontInstances = []fontInstance{
	{"FrauncesHero", font.SemiBold, frauncesTTF, 600, 32},
	{"FrauncesTitle", font.SemiBold, frauncesTTF, 600, 28},
	{"FrauncesMark", font.Bold, frauncesTTF, 700, 25},
	{"FrauncesSmall", font.SemiBold, frauncesTTF, 600, 10},
	{interTypeface, font.Normal, interTTF, 400, 14},
	{interTypeface, font.SemiBold, interTTF, 600, 14},
	{"InterSection", font.SemiBold, interTTF, 600, 16},
	{"InterDisplay", font.SemiBold, interTTF, 600, 24},
}

// variableFace is a go-text face with its variations applied, in the form Gio's shaper accepts.
type variableFace struct{ parsed *fontapi.Face }

// Face returns the face with its variations applied.
func (face variableFace) Face() *fontapi.Face { return face.parsed }

// newShaper returns a shaper that knows only the embedded fonts, so the app looks the same on every computer.
func newShaper() *text.Shaper {
	collection := make([]font.FontFace, 0, len(fontInstances))
	for _, instance := range fontInstances {
		// Gio tells faces apart by their parsed font, so every instance needs its own parse.
		face, err := fontapi.ParseTTF(bytes.NewReader(instance.data))
		if err != nil {
			panic("embedded font: " + err.Error())
		}
		face.SetVariations([]fontapi.Variation{
			{Tag: opentype.MustNewTag("wght"), Value: instance.weightAxis},
			{Tag: opentype.MustNewTag("opsz"), Value: instance.opticalSize},
		})
		collection = append(collection, font.FontFace{
			Font: font.Font{Typeface: instance.typeface, Weight: instance.weight},
			Face: variableFace{face},
		})
	}
	return text.NewShaper(text.NoSystemFonts(), text.WithCollection(collection))
}

// textStyle is one of the design's type styles.
type textStyle struct {
	font         font.Font
	sizeDp       float32
	lineHeightDp float32
	trackingEm   float32
	isUppercase  bool
}

// interStyle is a text style in the Inter instance for body text.
func interStyle(weight font.Weight, sizeDp, lineHeightDp float32) textStyle {
	return textStyle{
		font: font.Font{Typeface: interTypeface, Weight: weight}, sizeDp: sizeDp, lineHeightDp: lineHeightDp,
	}
}

var (
	styleHero = textStyle{
		font:   font.Font{Typeface: "FrauncesHero", Weight: font.SemiBold},
		sizeDp: 32, lineHeightDp: 38, trackingEm: -0.045,
	}
	styleTitle = textStyle{
		font:   font.Font{Typeface: "FrauncesTitle", Weight: font.SemiBold},
		sizeDp: 28, lineHeightDp: 34, trackingEm: -0.045,
	}
	styleWordmark = textStyle{
		font: font.Font{Typeface: "FrauncesMark", Weight: font.Bold}, sizeDp: 25, lineHeightDp: 25, trackingEm: -0.02,
	}
	styleWordmarkSmall = textStyle{
		font:   font.Font{Typeface: "FrauncesSmall", Weight: font.SemiBold},
		sizeDp: 10, lineHeightDp: 10, trackingEm: 0.18, isUppercase: true,
	}
	styleSection = textStyle{
		font: font.Font{Typeface: "InterSection", Weight: font.SemiBold}, sizeDp: 16, lineHeightDp: 22,
	}
	styleFingerprint = textStyle{
		font:   font.Font{Typeface: "InterDisplay", Weight: font.SemiBold},
		sizeDp: 24, lineHeightDp: 32, trackingEm: 0.06,
	}
	styleEyebrow = textStyle{
		font:   font.Font{Typeface: interTypeface, Weight: font.SemiBold},
		sizeDp: 12, lineHeightDp: 16, trackingEm: 0.18, isUppercase: true,
	}
	stylePill = textStyle{
		font:   font.Font{Typeface: interTypeface, Weight: font.SemiBold},
		sizeDp: 10, lineHeightDp: 16, trackingEm: 0.08, isUppercase: true,
	}
	styleBody         = interStyle(font.Normal, 14, 20)
	styleBodyLoose    = interStyle(font.Normal, 14, 21)
	styleButton       = interStyle(font.SemiBold, 14, 20)
	styleNavigation   = interStyle(font.SemiBold, 14, 20)
	styleSmall        = interStyle(font.Normal, 13, 20)
	styleSmallTall    = interStyle(font.Normal, 13, 19)
	styleSmallBold    = interStyle(font.SemiBold, 13, 19)
	styleCaption      = interStyle(font.Normal, 12, 18)
	styleCaptionTight = interStyle(font.Normal, 12, 17)
	styleLegal        = interStyle(font.Normal, 12, 19)
	styleLabel        = interStyle(font.SemiBold, 12, 16)
	styleStrong       = interStyle(font.SemiBold, 14, 20)
	styleAbout        = interStyle(font.Normal, 16, 22)
	styleBadge        = interStyle(font.SemiBold, 12, 16)
)

// label is a run of text in one style.
type label struct {
	style          textStyle
	color          color.NRGBA
	text           string
	maxLines       int  // 0 means unlimited
	breaksAnywhere bool // break long words anywhere, like overflow-wrap: anywhere
	isUnderlined   bool
	isCentered     bool // center each line in the available width
}

// shapedLine is one line of a label's glyphs, in pixels.
type shapedLine struct {
	glyphs  []text.Glyph
	ascent  float32
	descent float32
	width   float32
}

// layout shapes and paints the label. Its height is a whole number of CSS-style line boxes, each glyph line centered in
// its box, and letter spacing is applied after every glyph as browsers do.
func (label label) layout(gtx layout.Context, shaper *text.Shaper) layout.Dimensions {
	scale := gtx.Metric.PxPerDp
	lineHeight := label.style.lineHeightDp * scale
	lines := label.shape(gtx, shaper)

	var width float32
	for _, line := range lines {
		width = max(width, line.width)
	}
	if label.isCentered {
		width = float32(gtx.Constraints.Max.X)
	}
	var lastBaseline float32
	for index, line := range lines {
		lastBaseline = float32(index)*lineHeight + (lineHeight-(line.ascent+line.descent))/2 + line.ascent
		left := fixedToFloat(line.glyphs[0].X)
		if label.isCentered {
			left = (width - line.width) / 2
		}
		label.paintLine(gtx, shaper, line, f32.Pt(left, lastBaseline))
	}
	height := int(math.Ceil(float64(float32(max(len(lines), 1)) * lineHeight)))
	roundedWidth := min(max(int(math.Ceil(float64(width))), gtx.Constraints.Min.X), gtx.Constraints.Max.X)
	return layout.Dimensions{
		Size:     image.Pt(roundedWidth, height),
		Baseline: height - int(math.Round(float64(lastBaseline))),
	}
}

// shape breaks the label's text into lines that fit the constraints.
func (label label) shape(gtx layout.Context, shaper *text.Shaper) []shapedLine {
	size := label.style.sizeDp * gtx.Metric.PxPerDp
	tracking := label.style.trackingEm * size
	content := label.text
	if label.style.isUppercase {
		content = strings.ToUpper(content)
	}
	params := text.Parameters{
		Font:     label.style.font,
		PxPerEm:  fixed.Int26_6(size*64 + 0.5),
		MaxWidth: gtx.Constraints.Max.X,
		MaxLines: label.maxLines,
	}
	if label.maxLines > 0 {
		params.Truncator = "…"
	}
	if label.breaksAnywhere {
		params.WrapPolicy = text.WrapGraphemes
	}
	shaper.LayoutString(params, content)

	var lines []shapedLine
	var current shapedLine
	finishLine := func() {
		if len(current.glyphs) > 0 {
			first, last := current.glyphs[0], current.glyphs[len(current.glyphs)-1]
			current.width = fixedToFloat(last.X+last.Advance-first.X) + tracking*float32(len(current.glyphs))
			lines = append(lines, current)
		}
		current = shapedLine{}
	}
	for glyph, ok := shaper.NextGlyph(); ok; glyph, ok = shaper.NextGlyph() {
		if len(current.glyphs) == 0 {
			current.ascent, current.descent = fixedToFloat(glyph.Ascent), fixedToFloat(glyph.Descent)
		}
		glyph.X += fixed.Int26_6(tracking * 64 * float32(len(current.glyphs)))
		current.glyphs = append(current.glyphs, glyph)
		if glyph.Flags&text.FlagLineBreak != 0 {
			finishLine()
		}
	}
	finishLine()
	return lines
}

// paintLine paints one line with its left edge at origin.X and its baseline at origin.Y, then the underline if any.
func (label label) paintLine(gtx layout.Context, shaper *text.Shaper, line shapedLine, origin f32.Point) {
	scale := gtx.Metric.PxPerDp
	glyphOrigin := f32.Pt(origin.X, float32(math.Round(float64(origin.Y))))
	transform := op.Affine(f32.Affine2D{}.Offset(glyphOrigin)).Push(gtx.Ops)
	paintClip(gtx, clip.Outline{Path: shaper.Shape(line.glyphs)}.Op(), label.color)
	transform.Pop()
	if !label.isUnderlined {
		return
	}
	tracking := label.style.trackingEm * label.style.sizeDp * scale
	top := int(math.Round(float64(origin.Y + 2*scale)))
	right := int(math.Ceil(float64(origin.X + line.width - tracking)))
	fillRect(gtx, image.Rect(int(origin.X), top, right, top+max(1, int(scale+0.5))), label.color)
}

// fixedToFloat converts a 26.6 fixed-point value to a float.
func fixedToFloat(value fixed.Int26_6) float32 { return float32(value) / 64 }
