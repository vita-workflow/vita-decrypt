package main

import (
	"embed"
	"encoding/xml"
	"errors"
	"fmt"
	"image"
	"image/color"
	"io/fs"
	"math"
	"path"
	"strconv"
	"strings"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op/clip"
)

// Lucide icons (https://lucide.dev), ISC license (icons/LICENSE). Each is a 24×24 stroke drawing with round caps and
// joins, which is how Gio strokes.
//
//go:embed icons/*.svg
var iconFiles embed.FS

// iconViewBox is the width and height of every Lucide icon's view box.
const iconViewBox = 24

// icon is a parsed Lucide drawing: subpaths of cubic segments in the 24-unit view box.
type icon struct{ subpaths []subpath }

type subpath struct {
	start    f32.Point
	curves   []cubicCurve
	isClosed bool
}

// cubicCurve is one cubic Bézier segment, continuing from the end of the one before it.
type cubicCurve struct{ control1, control2, end f32.Point }

// iconsByName holds every embedded icon, keyed by its file name without .svg.
var iconsByName = mustParseIcons(iconFiles)

// mustParseIcons parses the embedded icons, panicking if one is broken: that's a build mistake, not a runtime error.
func mustParseIcons(files fs.FS) map[string]*icon {
	icons, err := parseIcons(files)
	if err != nil {
		panic(err)
	}
	return icons
}

// parseIcons parses every icons/*.svg file in files.
func parseIcons(files fs.FS) (map[string]*icon, error) {
	paths, err := fs.Glob(files, "icons/*.svg")
	if err != nil {
		return nil, fmt.Errorf("listing icons: %w", err)
	}
	icons := make(map[string]*icon, len(paths))
	for _, iconPath := range paths {
		data, err := fs.ReadFile(files, iconPath)
		if err != nil {
			return nil, fmt.Errorf("reading icon: %w", err)
		}
		parsed, err := parseIcon(string(data))
		if err != nil {
			return nil, fmt.Errorf("icon %s: %w", iconPath, err)
		}
		icons[strings.TrimSuffix(path.Base(iconPath), ".svg")] = parsed
	}
	return icons, nil
}

// loadIcon returns the named icon, panicking for a name that isn't embedded so a typo fails at once.
func loadIcon(name string) *icon {
	found, ok := iconsByName[name]
	if !ok {
		panic("no icon " + name)
	}
	return found
}

// layout strokes the icon at sizeDp with Lucide's stroke width (in view box units; Lucide's default is 2).
func (icon *icon) layout(gtx layout.Context, sizeDp, strokeWidth float32, stroke color.NRGBA) layout.Dimensions {
	size := pixelsFloat(gtx, sizeDp)
	scale := size / iconViewBox
	var path clip.Path
	path.Begin(gtx.Ops)
	for _, subpath := range icon.subpaths {
		path.MoveTo(subpath.start.Mul(scale))
		for _, curve := range subpath.curves {
			path.CubeTo(curve.control1.Mul(scale), curve.control2.Mul(scale), curve.end.Mul(scale))
		}
		if subpath.isClosed {
			path.Close()
		}
	}
	paintClip(gtx, clip.Stroke{Path: path.End(), Width: strokeWidth * scale}.Op(), stroke)
	sizePixels := int(math.Ceil(float64(size)))
	return layout.Dimensions{Size: image.Pt(sizePixels, sizePixels)}
}

// parseIcon reads the elements Lucide uses: path, circle, rect, line, and polyline.
func parseIcon(svg string) (*icon, error) {
	parsed := &icon{}
	decoder := xml.NewDecoder(strings.NewReader(svg))
	for {
		token, err := decoder.Token()
		if err != nil {
			break // the end of the document, or the end of what can be read of it
		}
		element, ok := token.(xml.StartElement)
		if !ok || element.Name.Local == "svg" {
			continue
		}
		subpaths, err := parseShape(element)
		if err != nil {
			return nil, err
		}
		parsed.subpaths = append(parsed.subpaths, subpaths...)
	}
	if len(parsed.subpaths) == 0 {
		return nil, errors.New("no shapes")
	}
	return parsed, nil
}

// svgAttributes are an element's attribute values by local name.
type svgAttributes map[string]string

// number is the attribute as a number, or 0 if it's missing or not a number.
func (attributes svgAttributes) number(name string) float32 {
	value, _ := strconv.ParseFloat(attributes[name], 32) // a missing or malformed number means 0, as browsers draw it
	return float32(value)
}

// cornerRadius is a rect's rx and ry, either of which defaults to the other.
func (attributes svgAttributes) cornerRadius() f32.Point {
	radius := f32.Pt(attributes.number("rx"), attributes.number("ry"))
	if _, ok := attributes["ry"]; !ok {
		radius.Y = radius.X
	}
	if _, ok := attributes["rx"]; !ok {
		radius.X = radius.Y
	}
	return radius
}

// parseShape turns one SVG shape element into subpaths.
func parseShape(element xml.StartElement) ([]subpath, error) {
	attributes := svgAttributes{}
	for _, attribute := range element.Attr {
		attributes[attribute.Name.Local] = attribute.Value
	}
	var builder pathBuilder
	switch element.Name.Local {
	case "path":
		if err := builder.parse(attributes["d"]); err != nil {
			return nil, err
		}
	case "circle":
		radius := attributes.number("r")
		builder.ellipse(f32.Pt(attributes.number("cx"), attributes.number("cy")), f32.Pt(radius, radius))
	case "rect":
		topLeft := f32.Pt(attributes.number("x"), attributes.number("y"))
		size := f32.Pt(attributes.number("width"), attributes.number("height"))
		builder.roundedRect(topLeft, size, attributes.cornerRadius())
	case "line":
		builder.moveTo(f32.Pt(attributes.number("x1"), attributes.number("y1")))
		builder.lineTo(f32.Pt(attributes.number("x2"), attributes.number("y2")))
	case "polyline":
		builder.polyline(parseNumbers(attributes["points"]))
	default:
		return nil, fmt.Errorf("unsupported element %s", element.Name.Local)
	}
	builder.finish()
	return builder.subpaths, nil
}
