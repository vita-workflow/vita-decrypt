package main

import (
	"image/color"
	"io"
	"strings"

	"gioui.org/font"
	"gioui.org/io/clipboard"
	"gioui.org/io/key"
	"gioui.org/io/transfer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
)

// textMIME is the clipboard type for plain text.
const textMIME = "application/text"

// field is the design's labelled text input.
type field struct {
	label        string
	editor       *widget.Editor
	placeholder  string
	helper       string
	errorMessage string
	isMultiline  bool
	actions      []layout.Widget // small buttons at the right of the label, such as Paste
}

func (app *App) field(gtx layout.Context, field field) layout.Dimensions {
	colors := app.colors
	title := app.text(styleLabel, colors.foreground, field.label)
	if len(field.actions) > 0 {
		labelText := title
		title = func(gtx layout.Context) layout.Dimensions {
			items := []layout.FlexChild{layout.Flexed(1, labelText)}
			for _, action := range field.actions {
				items = append(items, spacer(6), layout.Rigid(action))
			}
			return row(gtx, items...)
		}
	}
	children := []layout.FlexChild{
		layout.Rigid(fullWidth(title)),
		spacer(7),
		layout.Rigid(fullWidth(func(gtx layout.Context) layout.Dimensions { return app.input(gtx, field) })),
	}
	note, noteColor := field.helper, colors.muted
	if field.errorMessage != "" {
		note, noteColor = field.errorMessage, colors.danger
	}
	if note != "" {
		children = append(children, spacer(7), layout.Rigid(app.text(styleCaption, noteColor, note)))
	}
	return column(gtx, children...)
}

// input is the field's bordered editor with its placeholder.
func (app *App) input(gtx layout.Context, field field) layout.Dimensions {
	colors := app.colors
	border := colors.borderStrong
	if field.errorMessage != "" {
		border = colors.danger
	}
	minHeightDp := float32(36)
	if field.isMultiline {
		minHeightDp = 84
	}
	editor := field.editor
	decoration := box{background: colors.surface, border: border, borderWidth: 1, radius: 10}
	dims := decorate(gtx, decoration, symmetricInset(8, 12), func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		gtx.Constraints.Min.Y = pixels(gtx, minHeightDp-16)
		if field.isMultiline {
			// Long keys scroll inside the box instead of growing it.
			const visibleLines = 7
			gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, pixels(gtx, visibleLines*styleBody.lineHeightDp))
		}
		return layout.Stack{}.Layout(gtx,
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				if editor.Len() > 0 {
					return layout.Dimensions{}
				}
				faint := blend(colors.muted, colors.surface, 0.78)
				placeholder := label{style: styleBody, color: faint, text: field.placeholder}
				return placeholder.layout(gtx, app.shaper)
			}),
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return editor.Layout(gtx, app.shaper, font.Font{Typeface: interTypeface}, unit.Sp(14),
					editorColor(gtx, colors.foreground), editorColor(gtx, withAlpha(colors.primary, 0.3)))
			}),
		)
	})
	if gtx.Focused(editor) {
		app.focusRing(gtx, dims.Size, 10)
	}
	return dims
}

// editorColor records a color for an editor's text or selection.
func editorColor(gtx layout.Context, fill color.NRGBA) op.CallOp {
	recording := op.Record(gtx.Ops)
	paint.ColorOp{Color: fill}.Add(gtx.Ops)
	return recording.Stop()
}

// paster fills an editor from the clipboard when its Paste button is clicked, for people who don't know the keyboard
// shortcut or whose right-click menu has no Paste.
type paster struct {
	button widget.Clickable
	tag    int // receives the clipboard text
}

// pastedKeyLimit is the most clipboard text a Paste button reads, far more than any key.
const pastedKeyLimit = 64 << 10

// update asks for the clipboard text when the button was clicked and puts what arrives in editor, replacing its text.
// It must run every frame the button is shown.
func (paster *paster) update(gtx layout.Context, editor *widget.Editor) {
	if paster.button.Clicked(gtx) {
		gtx.Execute(clipboard.ReadCmd{Tag: &paster.tag})
	}
	for {
		received, ok := gtx.Event(transfer.TargetFilter{Target: &paster.tag, Type: textMIME})
		if !ok {
			return
		}
		if data, ok := received.(transfer.DataEvent); ok {
			editor.SetText(strings.TrimSpace(readClipboardText(data, pastedKeyLimit)))
			gtx.Execute(key.FocusCmd{Tag: editor})
		}
	}
}

// readClipboardText reads at most limitBytes of clipboard text.
func readClipboardText(data transfer.DataEvent, limitBytes int64) string {
	reader := data.Open()
	content, _ := io.ReadAll(io.LimitReader(reader, limitBytes)) // a failed read pastes what arrived before it
	_ = reader.Close()                                           // the clipboard is in memory; closing can't lose data
	return string(content)
}

// pasteButton is the Paste button beside a field's label.
func (app *App) pasteButton(paster *paster) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return app.button(gtx, &paster.button,
			button{kind: linkButton, icon: "clipboard-paste", iconSizeDp: 15, text: "Paste"})
	}
}

// copyText puts content on the clipboard.
func copyText(gtx layout.Context, content string) {
	gtx.Execute(clipboard.WriteCmd{Type: textMIME, Data: io.NopCloser(strings.NewReader(content))})
}

// inputError is why something the person entered didn't work, shown until they change it.
type inputError struct {
	message string
	input   string // what was entered when it failed
}

func (problem *inputError) set(message, input string) {
	problem.message, problem.input = message, input
}

func (problem *inputError) clear() { *problem = inputError{} }

// messageFor is the message if input is still what failed, else "".
func (problem *inputError) messageFor(input string) string {
	if problem.message == "" || problem.input != input {
		return ""
	}
	return problem.message
}
