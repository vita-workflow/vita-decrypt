package main

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

// spacer is empty space of size dp in a row or column.
func spacer(size unit.Dp) layout.FlexChild {
	return layout.Rigid(layout.Spacer{Width: size, Height: size}.Layout)
}

// column lays children out vertically.
func column(gtx layout.Context, children ...layout.FlexChild) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// row lays children out horizontally, centered vertically.
func row(gtx layout.Context, children ...layout.FlexChild) layout.Dimensions {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
}

// rowTop lays children out horizontally, aligned to the top.
func rowTop(gtx layout.Context, children ...layout.FlexChild) layout.Dimensions {
	return layout.Flex{Alignment: layout.Start}.Layout(gtx, children...)
}

// alignEnd lays content out at the end (right) of the available width.
func alignEnd(gtx layout.Context, content layout.Widget) layout.Dimensions {
	return layout.Flex{Spacing: layout.SpaceStart}.Layout(gtx, layout.Rigid(content))
}

// fullWidth makes content take the full available width.
func fullWidth(content layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return content(gtx)
	}
}

// minHeight makes content at least heightDp tall, centering it vertically.
func minHeight(heightDp float32, content layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		recording := op.Record(gtx.Ops)
		dims := content(gtx)
		call := recording.Stop()
		want := pixels(gtx, heightDp)
		if dims.Size.Y >= want {
			call.Add(gtx.Ops)
			return dims
		}
		shift := op.Offset(image.Pt(0, (want-dims.Size.Y)/2)).Push(gtx.Ops)
		call.Add(gtx.Ops)
		shift.Pop()
		return layout.Dimensions{Size: image.Pt(dims.Size.X, want)}
	}
}

// symmetricInset is CSS's two-value padding: vertical on the top and bottom, horizontal on the left and right.
func symmetricInset(vertical, horizontal unit.Dp) layout.Inset {
	return layout.Inset{Top: vertical, Right: horizontal, Bottom: vertical, Left: horizontal}
}

// decorate lays out content inside padding, then paints decoration behind it.
func decorate(gtx layout.Context, decoration box, padding layout.Inset, content layout.Widget) layout.Dimensions {
	recording := op.Record(gtx.Ops)
	dims := padding.Layout(gtx, content)
	call := recording.Stop()
	decoration.paint(gtx, dims.Size)
	call.Add(gtx.Ops)
	return dims
}

// offset draws content moved by shift pixels.
func offset(gtx layout.Context, shift image.Point, content layout.Widget) {
	defer op.Offset(shift).Push(gtx.Ops).Pop()
	content(gtx)
}

// center lays content out centered in size (pixels).
func center(gtx layout.Context, size image.Point, content layout.Widget) layout.Dimensions {
	gtx.Constraints = layout.Exact(size)
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = image.Point{}
		return content(gtx)
	})
}

// squareBadge paints decoration over a sizeDp square and centers content in it.
func squareBadge(gtx layout.Context, sizeDp float32, decoration box, content layout.Widget) layout.Dimensions {
	size := image.Pt(pixels(gtx, sizeDp), pixels(gtx, sizeDp))
	decoration.paint(gtx, size)
	return center(gtx, size, content)
}
