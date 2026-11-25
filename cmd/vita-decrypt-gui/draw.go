package main

import (
	"image"
	"image/color"
	"math"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
)

// pixels converts a length in design pixels (dp) to whole device pixels.
func pixels(gtx layout.Context, lengthDp float32) int {
	return int(math.Round(float64(lengthDp * gtx.Metric.PxPerDp)))
}

// pixelsFloat converts a length in design pixels (dp) to device pixels.
func pixelsFloat(gtx layout.Context, lengthDp float32) float32 { return lengthDp * gtx.Metric.PxPerDp }

// fillRect paints area with fill.
func fillRect(gtx layout.Context, area image.Rectangle, fill color.NRGBA) {
	defer clip.Rect(area).Push(gtx.Ops).Pop()
	paint.ColorOp{Color: fill}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
}

// paintClip paints fill inside the clip shape.
func paintClip(gtx layout.Context, shape clip.Op, fill color.NRGBA) {
	defer shape.Push(gtx.Ops).Pop()
	paint.ColorOp{Color: fill}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
}

// quarterCircleControl is how far along the tangent a cubic Bézier's control points sit to draw a quarter circle.
const quarterCircleControl = 0.5522847

// roundedRectPath is a rounded rectangle from topLeft to bottomRight with corner radius, in pixels.
func roundedRectPath(ops *op.Ops, topLeft, bottomRight f32.Point, radius float32) clip.PathSpec {
	shortSide := min(bottomRight.X-topLeft.X, bottomRight.Y-topLeft.Y)
	radius = max(0, min(radius, shortSide/2))
	control := radius * (1 - quarterCircleControl)
	left, top, right, bottom := topLeft.X, topLeft.Y, bottomRight.X, bottomRight.Y
	var path clip.Path
	path.Begin(ops)
	path.MoveTo(f32.Pt(left+radius, top))
	path.LineTo(f32.Pt(right-radius, top))
	path.CubeTo(f32.Pt(right-control, top), f32.Pt(right, top+control), f32.Pt(right, top+radius))
	path.LineTo(f32.Pt(right, bottom-radius))
	path.CubeTo(f32.Pt(right, bottom-control), f32.Pt(right-control, bottom), f32.Pt(right-radius, bottom))
	path.LineTo(f32.Pt(left+radius, bottom))
	path.CubeTo(f32.Pt(left+control, bottom), f32.Pt(left, bottom-control), f32.Pt(left, bottom-radius))
	path.LineTo(f32.Pt(left, top+radius))
	path.CubeTo(f32.Pt(left, top+control), f32.Pt(left+control, top), f32.Pt(left+radius, top))
	path.Close()
	return path.End()
}

// box is a CSS-like box decoration: background, border, and corner radius, all in dp.
type box struct {
	background  color.NRGBA
	border      color.NRGBA
	borderWidth float32
	radius      float32
	isDashed    bool
}

// paint draws the box over size (in pixels) at the current offset.
func (box box) paint(gtx layout.Context, size image.Point) {
	radius := pixelsFloat(gtx, box.radius)
	full := f32.Pt(float32(size.X), float32(size.Y))
	if box.background.A > 0 {
		paintClip(gtx, clip.Outline{Path: roundedRectPath(gtx.Ops, f32.Point{}, full, radius)}.Op(), box.background)
	}
	if box.borderWidth <= 0 || box.border.A == 0 {
		return
	}
	width := pixelsFloat(gtx, box.borderWidth)
	half := f32.Pt(width/2, width/2)
	var outline clip.PathSpec
	if box.isDashed {
		pattern := dashPattern{3 * width, 2 * width}
		outline = dashedRoundedRectPath(gtx.Ops, half, full.Sub(half), radius-width/2, pattern)
	} else {
		outline = roundedRectPath(gtx.Ops, half, full.Sub(half), radius-width/2)
	}
	paintClip(gtx, clip.Stroke{Path: outline, Width: width}.Op(), box.border)
}

// dashPattern is the length of each dash and of the gap after it, in pixels.
type dashPattern struct{ dash, gap float32 }

// dashedRoundedRectPath is the outline of a rounded rectangle as dashes, spaced evenly around the whole perimeter.
func dashedRoundedRectPath(
	ops *op.Ops, topLeft, bottomRight f32.Point, radius float32, pattern dashPattern,
) clip.PathSpec {
	outline := newPolyline(roundedRectOutline(topLeft, bottomRight, radius))
	total := outline.length()
	// Stretch the gaps so the pattern closes without a short dash.
	dashCount := int(math.Max(1, math.Floor(float64(total/(pattern.dash+pattern.gap)))))
	period := total / float32(dashCount)

	var path clip.Path
	path.Begin(ops)
	segment := 1
	for dashIndex := range dashCount {
		dashStart := float32(dashIndex) * period
		dashEnd := dashStart + pattern.dash
		segment = outline.segmentAt(dashStart, segment)
		path.MoveTo(outline.pointAt(dashStart, segment))
		for segment < len(outline.points)-1 && outline.distances[segment] < dashEnd {
			path.LineTo(outline.points[segment])
			segment++
		}
		path.LineTo(outline.pointAt(min(dashEnd, total), segment))
	}
	return path.End()
}

// roundedRectOutline samples a rounded rectangle's outline as a closed polyline, clockwise from the top right corner.
func roundedRectOutline(topLeft, bottomRight f32.Point, radius float32) []f32.Point {
	const stepsPerCorner = 8
	var points []f32.Point
	corner := func(centerX, centerY float32, startAngle float64) {
		for step := 0; step <= stepsPerCorner; step++ {
			angle := startAngle + float64(step)/stepsPerCorner*math.Pi/2
			points = append(points, f32.Pt(
				centerX+radius*float32(math.Cos(angle)), centerY+radius*float32(math.Sin(angle))))
		}
	}
	corner(bottomRight.X-radius, topLeft.Y+radius, -math.Pi/2)
	corner(bottomRight.X-radius, bottomRight.Y-radius, 0)
	corner(topLeft.X+radius, bottomRight.Y-radius, math.Pi/2)
	corner(topLeft.X+radius, topLeft.Y+radius, math.Pi)
	return append(points, points[0])
}

// polyline is a run of points with the distance along the line at each one.
type polyline struct {
	points    []f32.Point
	distances []float32
}

func newPolyline(points []f32.Point) polyline {
	distances := make([]float32, len(points))
	for i := 1; i < len(points); i++ {
		distances[i] = distances[i-1] + distance(points[i-1], points[i])
	}
	return polyline{points: points, distances: distances}
}

func (line polyline) length() float32 { return line.distances[len(line.distances)-1] }

// segmentAt advances segment, starting from the given one, to the segment whose end is at or past along.
func (line polyline) segmentAt(along float32, segment int) int {
	for segment < len(line.points)-1 && line.distances[segment] < along {
		segment++
	}
	return segment
}

// pointAt is the point at distance along, which lies on the segment ending at points[segment].
func (line polyline) pointAt(along float32, segment int) f32.Point {
	segmentLength := line.distances[segment] - line.distances[segment-1]
	if segmentLength <= 0 {
		return line.points[segment]
	}
	fraction := (along - line.distances[segment-1]) / segmentLength
	return interpolate(line.points[segment-1], line.points[segment], fraction)
}

// distance is the straight-line distance between two points.
func distance(from, to f32.Point) float32 {
	difference := to.Sub(from)
	return float32(math.Hypot(float64(difference.X), float64(difference.Y)))
}

// interpolate is the point fraction of the way from from to to.
func interpolate(from, to f32.Point, fraction float32) f32.Point {
	return from.Add(to.Sub(from).Mul(fraction))
}
