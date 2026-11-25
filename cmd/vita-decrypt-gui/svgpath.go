package main

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"gioui.org/f32"
)

// pathBuilder collects subpaths of cubic curves, the one form Gio strokes every SVG shape in.
type pathBuilder struct {
	subpaths []subpath
	current  *subpath
	pen      f32.Point
}

// finish ends the current subpath, if any.
func (builder *pathBuilder) finish() {
	if builder.current != nil {
		builder.subpaths = append(builder.subpaths, *builder.current)
		builder.current = nil
	}
}

func (builder *pathBuilder) moveTo(point f32.Point) {
	builder.finish()
	builder.current = &subpath{start: point}
	builder.pen = point
}

func (builder *pathBuilder) cubeTo(control1, control2, end f32.Point) {
	if builder.current == nil {
		builder.moveTo(builder.pen)
	}
	builder.current.curves = append(builder.current.curves, cubicCurve{control1, control2, end})
	builder.pen = end
}

func (builder *pathBuilder) lineTo(end f32.Point) {
	builder.cubeTo(interpolate(builder.pen, end, 1.0/3), interpolate(builder.pen, end, 2.0/3), end)
}

// close draws a line back to the start of the current subpath and ends it.
func (builder *pathBuilder) close() {
	if builder.current == nil {
		return
	}
	start := builder.current.start
	if builder.pen != start {
		builder.lineTo(start)
	}
	builder.current.isClosed = true
	builder.finish()
	builder.pen = start
}

// polyline draws lines through coordinates, which alternate x and y. A trailing odd coordinate is ignored.
func (builder *pathBuilder) polyline(coordinates []float32) {
	for i := 0; i+1 < len(coordinates); i += 2 {
		point := f32.Pt(coordinates[i], coordinates[i+1])
		if i == 0 {
			builder.moveTo(point)
		} else {
			builder.lineTo(point)
		}
	}
}

// ellipse draws a closed ellipse.
func (builder *pathBuilder) ellipse(center, radius f32.Point) {
	builder.moveTo(f32.Pt(center.X+radius.X, center.Y))
	builder.arcTo(svgArc{radius: radius, isSweep: true, end: f32.Pt(center.X-radius.X, center.Y)})
	builder.arcTo(svgArc{radius: radius, isSweep: true, end: f32.Pt(center.X+radius.X, center.Y)})
	builder.close()
}

// roundedRect draws a closed rectangle whose corners are quarter ellipses, the way SVG's rect element does.
func (builder *pathBuilder) roundedRect(topLeft, size, radius f32.Point) {
	radius = f32.Pt(min(radius.X, size.X/2), min(radius.Y, size.Y/2))
	left, top, right, bottom := topLeft.X, topLeft.Y, topLeft.X+size.X, topLeft.Y+size.Y
	corner := func(end f32.Point) { builder.arcTo(svgArc{radius: radius, isSweep: true, end: end}) }
	builder.moveTo(f32.Pt(left+radius.X, top))
	builder.lineTo(f32.Pt(right-radius.X, top))
	corner(f32.Pt(right, top+radius.Y))
	builder.lineTo(f32.Pt(right, bottom-radius.Y))
	corner(f32.Pt(right-radius.X, bottom))
	builder.lineTo(f32.Pt(left+radius.X, bottom))
	corner(f32.Pt(left, bottom-radius.Y))
	builder.lineTo(f32.Pt(left, top+radius.Y))
	corner(f32.Pt(left+radius.X, top))
	builder.close()
}

// svgArc is an SVG elliptical arc command, which runs from the pen to end.
type svgArc struct {
	radius          f32.Point
	rotationDegrees float32
	isLarge         bool // take the longer way around
	isSweep         bool // go clockwise
	end             f32.Point
}

// arcTo adds an elliptical arc from the pen as cubic segments of at most 90°.
func (builder *pathBuilder) arcTo(arc svgArc) {
	start := builder.pen
	if arc.radius.X == 0 || arc.radius.Y == 0 || start == arc.end {
		builder.lineTo(arc.end)
		return
	}
	ellipse := centerArc(start, arc)
	segmentCount := int(math.Ceil(math.Abs(ellipse.sweepAngle) / (math.Pi / 2)))
	step := ellipse.sweepAngle / float64(segmentCount)
	handle := 4.0 / 3 * math.Tan(step/4)
	for i := range segmentCount {
		fromX, fromY, fromTangentX, fromTangentY := ellipse.at(ellipse.startAngle + float64(i)*step)
		toX, toY, toTangentX, toTangentY := ellipse.at(ellipse.startAngle + float64(i+1)*step)
		end := f32.Pt(float32(toX), float32(toY))
		if i == segmentCount-1 {
			end = arc.end
		}
		builder.cubeTo(
			f32.Pt(float32(fromX+handle*fromTangentX), float32(fromY+handle*fromTangentY)),
			f32.Pt(float32(toX-handle*toTangentX), float32(toY-handle*toTangentY)),
			end)
	}
}

// centeredArc is an elliptical arc in center parameterization, in float64 for accuracy.
type centeredArc struct {
	centerX, centerY float64
	radiusX, radiusY float64
	cos, sin         float64 // of the ellipse's rotation
	startAngle       float64
	sweepAngle       float64 // signed: negative runs counterclockwise
}

// at is the point at angle on the ellipse and the tangent (derivative) there.
func (ellipse centeredArc) at(angle float64) (pointX, pointY, tangentX, tangentY float64) {
	unrotatedX, unrotatedY := ellipse.radiusX*math.Cos(angle), ellipse.radiusY*math.Sin(angle)
	derivativeX, derivativeY := -ellipse.radiusX*math.Sin(angle), ellipse.radiusY*math.Cos(angle)
	cos, sin := ellipse.cos, ellipse.sin
	return cos*unrotatedX - sin*unrotatedY + ellipse.centerX, sin*unrotatedX + cos*unrotatedY + ellipse.centerY,
		cos*derivativeX - sin*derivativeY, sin*derivativeX + cos*derivativeY
}

// centerArc converts an arc from start to center parameterization (SVG 1.1 appendix F.6.5), scaling the radii up if
// they're too small to reach the end.
func centerArc(start f32.Point, arc svgArc) centeredArc {
	radiusX, radiusY := math.Abs(float64(arc.radius.X)), math.Abs(float64(arc.radius.Y))
	rotation := float64(arc.rotationDegrees) * math.Pi / 180
	cos, sin := math.Cos(rotation), math.Sin(rotation)
	halfX, halfY := float64(start.X-arc.end.X)/2, float64(start.Y-arc.end.Y)/2
	primeX := cos*halfX + sin*halfY
	primeY := -sin*halfX + cos*halfY
	if reach := primeX*primeX/(radiusX*radiusX) + primeY*primeY/(radiusY*radiusY); reach > 1 {
		scale := math.Sqrt(reach)
		radiusX, radiusY = radiusX*scale, radiusY*scale
	}
	numerator := radiusX*radiusX*radiusY*radiusY - radiusX*radiusX*primeY*primeY - radiusY*radiusY*primeX*primeX
	denominator := radiusX*radiusX*primeY*primeY + radiusY*radiusY*primeX*primeX
	coefficient := math.Sqrt(math.Max(0, numerator/denominator))
	if arc.isLarge == arc.isSweep {
		coefficient = -coefficient
	}
	centerPrimeX := coefficient * radiusX * primeY / radiusY
	centerPrimeY := -coefficient * radiusY * primeX / radiusX
	startX, startY := (primeX-centerPrimeX)/radiusX, (primeY-centerPrimeY)/radiusY
	endX, endY := (-primeX-centerPrimeX)/radiusX, (-primeY-centerPrimeY)/radiusY
	sweepAngle := angleBetween(startX, startY, endX, endY)
	if !arc.isSweep && sweepAngle > 0 {
		sweepAngle -= 2 * math.Pi
	} else if arc.isSweep && sweepAngle < 0 {
		sweepAngle += 2 * math.Pi
	}
	return centeredArc{
		centerX:    cos*centerPrimeX - sin*centerPrimeY + float64(start.X+arc.end.X)/2,
		centerY:    sin*centerPrimeX + cos*centerPrimeY + float64(start.Y+arc.end.Y)/2,
		radiusX:    radiusX,
		radiusY:    radiusY,
		cos:        cos,
		sin:        sin,
		startAngle: angleBetween(1, 0, startX, startY),
		sweepAngle: sweepAngle,
	}
}

// angleBetween is the signed angle from one vector to another, in radians.
func angleBetween(fromX, fromY, toX, toY float64) float64 {
	return math.Atan2(fromX*toY-fromY*toX, fromX*toX+fromY*toY)
}

// errNoProgress means path data has numbers where the current command takes none.
var errNoProgress = errors.New("numbers after a command that takes none")

// parse reads SVG path data: M L H V C S Q T A Z in both cases.
func (builder *pathBuilder) parse(data string) error {
	parser := pathParser{builder: builder, scanner: pathScanner{source: data}}
	for {
		parser.scanner.skipSeparators()
		if parser.scanner.isAtEnd() {
			return nil
		}
		start := parser.scanner.position
		if letter := parser.scanner.peek(); isASCIILetter(letter) {
			parser.command = letter
			parser.scanner.position++
		} else if parser.command == 0 {
			return fmt.Errorf("path data starts without a command: %q", data)
		}
		upper := parser.command &^ 0x20
		if err := parser.runCommand(upper); err != nil {
			return fmt.Errorf("path %q: %w", data, err)
		}
		if parser.scanner.position == start {
			return fmt.Errorf("path %q at %d: %w", data, start, errNoProgress)
		}
		parser.lastCommand = upper
	}
}

func isASCIILetter(char byte) bool {
	return (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z')
}

// pathParser is the state of reading one path's data.
type pathParser struct {
	builder     *pathBuilder
	scanner     pathScanner
	command     byte      // the command being repeated, as written
	lastCommand byte      // the previous command, in upper case
	lastControl f32.Point // the previous curve's last control point, reflected by S and T
	origin      f32.Point // what the current command's coordinates are relative to
}

// runCommand reads one set of the current command's arguments and draws it. upper is the command in upper case.
func (parser *pathParser) runCommand(upper byte) error {
	isRelative := parser.command >= 'a'
	parser.origin = f32.Point{}
	if isRelative {
		parser.origin = parser.builder.pen
	}
	switch upper {
	case 'Z':
		parser.builder.close()
		return nil
	case 'M':
		return parser.moveTo(isRelative)
	case 'L':
		return parser.lineTo()
	case 'H':
		return parser.horizontalLineTo()
	case 'V':
		return parser.verticalLineTo()
	case 'C':
		return parser.cubicTo()
	case 'S':
		return parser.smoothCubicTo()
	case 'Q':
		return parser.quadraticTo(false)
	case 'T':
		return parser.quadraticTo(true)
	case 'A':
		return parser.arcTo()
	}
	return fmt.Errorf("unsupported path command %c", parser.command)
}

// point reads a coordinate pair.
func (parser *pathParser) point() (f32.Point, error) {
	horizontal, err := parser.scanner.number()
	if err != nil {
		return f32.Point{}, err
	}
	vertical, err := parser.scanner.number()
	return f32.Pt(horizontal, vertical).Add(parser.origin), err
}

func (parser *pathParser) moveTo(isRelative bool) error {
	point, err := parser.point()
	if err != nil {
		return err
	}
	parser.builder.moveTo(point)
	// Further pairs after a moveto are implicit linetos.
	parser.command = 'L'
	if isRelative {
		parser.command = 'l'
	}
	return nil
}

func (parser *pathParser) lineTo() error {
	point, err := parser.point()
	if err != nil {
		return err
	}
	parser.builder.lineTo(point)
	return nil
}

func (parser *pathParser) horizontalLineTo() error {
	horizontal, err := parser.scanner.number()
	if err != nil {
		return err
	}
	parser.builder.lineTo(f32.Pt(horizontal+parser.origin.X, parser.builder.pen.Y))
	return nil
}

func (parser *pathParser) verticalLineTo() error {
	vertical, err := parser.scanner.number()
	if err != nil {
		return err
	}
	parser.builder.lineTo(f32.Pt(parser.builder.pen.X, vertical+parser.origin.Y))
	return nil
}

func (parser *pathParser) cubicTo() error {
	control1, err := parser.point()
	if err != nil {
		return err
	}
	return parser.finishCubic(control1)
}

// smoothCubicTo reflects the previous cubic's second control point to make the first.
func (parser *pathParser) smoothCubicTo() error {
	control1 := parser.builder.pen
	if parser.lastCommand == 'C' || parser.lastCommand == 'S' {
		control1 = parser.builder.pen.Mul(2).Sub(parser.lastControl)
	}
	return parser.finishCubic(control1)
}

// finishCubic reads a cubic's second control point and end, and draws it.
func (parser *pathParser) finishCubic(control1 f32.Point) error {
	control2, err := parser.point()
	if err != nil {
		return err
	}
	end, err := parser.point()
	if err != nil {
		return err
	}
	parser.builder.cubeTo(control1, control2, end)
	parser.lastControl = control2
	return nil
}

// quadraticTo draws a quadratic curve as the equivalent cubic. A smooth one reflects the previous quadratic's control
// point instead of reading one.
func (parser *pathParser) quadraticTo(isSmooth bool) error {
	control := parser.builder.pen
	switch {
	case !isSmooth:
		var err error
		if control, err = parser.point(); err != nil {
			return err
		}
	case parser.lastCommand == 'Q' || parser.lastCommand == 'T':
		control = parser.builder.pen.Mul(2).Sub(parser.lastControl)
	}
	end, err := parser.point()
	if err != nil {
		return err
	}
	start := parser.builder.pen
	parser.builder.cubeTo(interpolate(start, control, 2.0/3), interpolate(end, control, 2.0/3), end)
	parser.lastControl = control
	return nil
}

func (parser *pathParser) arcTo() error {
	scanner := &parser.scanner
	var arc svgArc
	var err error
	if arc.radius.X, err = scanner.number(); err != nil {
		return err
	}
	if arc.radius.Y, err = scanner.number(); err != nil {
		return err
	}
	if arc.rotationDegrees, err = scanner.number(); err != nil {
		return err
	}
	if arc.isLarge, err = scanner.arcFlag(); err != nil {
		return err
	}
	if arc.isSweep, err = scanner.arcFlag(); err != nil {
		return err
	}
	if arc.end, err = parser.point(); err != nil {
		return err
	}
	parser.builder.arcTo(arc)
	return nil
}

// pathScanner reads the numbers and flags of SVG path data.
type pathScanner struct {
	source   string
	position int
}

func (scanner *pathScanner) isAtEnd() bool { return scanner.position >= len(scanner.source) }

// peek is the byte at the current position. The scanner must not be at the end.
func (scanner *pathScanner) peek() byte { return scanner.source[scanner.position] }

// skipSeparators skips whitespace and commas.
func (scanner *pathScanner) skipSeparators() {
	for !scanner.isAtEnd() && strings.IndexByte(" \t\r\n,", scanner.peek()) >= 0 {
		scanner.position++
	}
}

// skipSign skips a plus or minus sign, if there is one.
func (scanner *pathScanner) skipSign() {
	if !scanner.isAtEnd() && (scanner.peek() == '-' || scanner.peek() == '+') {
		scanner.position++
	}
}

// number reads a number. It stops where the number can't continue, so "1.5.5" is two numbers, as SVG allows.
func (scanner *pathScanner) number() (float32, error) {
	scanner.skipSeparators()
	start := scanner.position
	scanner.skipSign()
	hasDot, hasExponent := false, false
	for !scanner.isAtEnd() {
		char := scanner.peek()
		switch {
		case char >= '0' && char <= '9':
		case char == '.' && !hasDot && !hasExponent:
			hasDot = true
		case (char == 'e' || char == 'E') && !hasExponent:
			hasExponent = true
			scanner.position++
			scanner.skipSign()
			continue
		default:
			return scanner.parseFloat(start)
		}
		scanner.position++
	}
	return scanner.parseFloat(start)
}

// parseFloat parses the source from start to the current position.
func (scanner *pathScanner) parseFloat(start int) (float32, error) {
	value, err := strconv.ParseFloat(scanner.source[start:scanner.position], 32)
	if err != nil {
		return 0, fmt.Errorf("bad number at %d", start)
	}
	return float32(value), nil
}

// arcFlag reads an arc's 0 or 1 flag.
func (scanner *pathScanner) arcFlag() (bool, error) {
	scanner.skipSeparators()
	if scanner.isAtEnd() || (scanner.peek() != '0' && scanner.peek() != '1') {
		return false, fmt.Errorf("bad arc flag at %d", scanner.position)
	}
	scanner.position++
	return scanner.source[scanner.position-1] == '1', nil
}

// parseNumbers reads a list of numbers separated by whitespace or commas, stopping at the first that isn't one.
func parseNumbers(list string) []float32 {
	scanner := pathScanner{source: list}
	var numbers []float32
	for scanner.skipSeparators(); !scanner.isAtEnd(); scanner.skipSeparators() {
		value, err := scanner.number()
		if err != nil {
			break
		}
		numbers = append(numbers, value)
	}
	return numbers
}
