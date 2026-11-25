package main

import (
	"fmt"
	"image"
	"path/filepath"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/widget"

	"github.com/vita-workflow/vita-decrypt/internal/desktop"
)

// notKeyMessage is shown when the chosen private key file isn't one.
const notKeyMessage = "This isn’t a private key file. Choose the file whose name ends in “-private-key.txt”."

// resultIndent lines a file's progress bar and result up with its name, past the file icon.
const resultIndent = 27

func (app *App) layoutOpenScreen(gtx layout.Context) layout.Dimensions {
	app.handleOpenClicks(gtx)
	return column(gtx,
		layout.Rigid(fullWidth(app.hero)),
		spacer(20),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if app.open.isDone {
				return app.card(gtx, layout.UniformInset(24), app.doneSummary)
			}
			return app.card(gtx, layout.Inset{Top: 24, Right: 24, Bottom: 20, Left: 24}, app.taskCard)
		}),
	)
}

// hero is the forest-green banner at the top of the open screen.
func (app *App) hero(gtx layout.Context) layout.Dimensions {
	colors := app.colors
	recording := op.Record(gtx.Ops)
	dims := symmetricInset(23, 24).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return column(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return row(gtx,
					layout.Rigid(app.icon("wifi-off", 15, 2, colors.fern)),
					spacer(7),
					layout.Rigid(app.text(styleEyebrow, colors.fern, "Works offline")),
				)
			}),
			spacer(9),
			layout.Rigid(app.text(styleHero, colors.forestForeground, "Open your sealed files")),
			spacer(7),
			layout.Rigid(app.text(styleBody, colors.forestForeground, "Your private key never leaves this computer.")),
		)
	})
	call := recording.Stop()
	size := image.Pt(gtx.Constraints.Max.X, max(dims.Size.Y, pixels(gtx, 154)))
	box{background: colors.forest, radius: 20}.paint(gtx, size)
	const shieldSize = 74
	shieldPixels := pixels(gtx, shieldSize)
	shieldCorner := image.Pt(size.X-pixels(gtx, 24)-shieldPixels, size.Y-pixels(gtx, 18)-shieldPixels)
	offset(gtx, shieldCorner, app.icon("shield-check", shieldSize, 1.2, blend(colors.fern, colors.forest, 0.18)))
	call.Add(gtx.Ops)
	return layout.Dimensions{Size: size}
}

// step is a numbered step heading.
func (app *App) step(number, title, subtitle string) layout.Widget {
	colors := app.colors
	return func(gtx layout.Context) layout.Dimensions {
		return rowTop(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return squareBadge(gtx, 25, box{background: colors.surfaceAlt, radius: 13},
					app.text(styleBadge, colors.primaryText, number))
			}),
			spacer(12),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return column(gtx,
					layout.Rigid(app.text(styleSection, colors.foreground, title)),
					spacer(1),
					layout.Rigid(app.text(styleSmall, colors.muted, subtitle)),
				)
			}),
		)
	}
}

func (app *App) taskCard(gtx layout.Context) layout.Dimensions {
	state := &app.open
	children := []layout.FlexChild{
		layout.Rigid(app.step("1", "Your private key file", "The key stays on this computer.")),
		spacer(12),
		layout.Rigid(fullWidth(app.keyZone)),
	}
	if state.hasKeyError {
		children = append(children, spacer(10), layout.Rigid(app.messageText(dangerMessage, notKeyMessage)))
	}
	children = append(children,
		spacer(20), layout.Rigid(app.divider), spacer(20),
		layout.Rigid(app.step("2", "Sealed files", "Choose one file or several at once.")),
		spacer(12),
	)
	children = append(children, app.sealedFilesArea()...)
	if state.filesMessage != "" {
		children = append(children, spacer(10), layout.Rigid(app.messageText(dangerMessage, state.filesMessage)))
	}
	children = append(children, spacer(20), layout.Rigid(fullWidth(app.openFooter)))
	return column(gtx, children...)
}

// sealedFilesArea is the drop zone, or the chosen files and a link to add more.
func (app *App) sealedFilesArea() []layout.FlexChild {
	state := &app.open
	if len(state.files) == 0 {
		zone := dropZone{
			icon: fileUpIcon,
			text: dropText("Drop sealed files here (names end in .age)",
				"Choose the sealed files (names end in .age)"),
			button: &state.chooseFilesButton,
			choose: "Choose files…",
		}
		return []layout.FlexChild{layout.Rigid(fullWidth(func(gtx layout.Context) layout.Dimensions {
			return app.dropZone(gtx, zone)
		}))}
	}
	children := []layout.FlexChild{layout.Rigid(fullWidth(app.fileList))}
	if !state.isWorking {
		children = append(children, spacer(4), layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return app.button(gtx, &state.addFilesButton, button{kind: linkButton, text: "Add more files…"})
		}))
	}
	return children
}

// openFooter is the note about where files are saved, and the Open button.
func (app *App) openFooter(gtx layout.Context) layout.Dimensions {
	state := &app.open
	fileCount := len(state.files)
	text := "Open files"
	switch {
	case state.isWorking:
		text = "Opening…"
	case fileCount == 1:
		text = "Open 1 file"
	case fileCount > 1:
		text = fmt.Sprintf("Open %d files", fileCount)
	}
	start := button{
		kind: primaryButton, isLarge: true, text: text, isBusy: state.isWorking,
		isDisabled: state.isWorking || fileCount == 0 || state.privateKey == nil,
	}
	return row(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, pixels(gtx, 350))
			note := "Each file is saved next to its sealed copy, without “.age” in the name."
			return app.text(styleCaption, app.colors.muted, note)(gtx)
		}),
		spacer(24),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return app.button(gtx, &state.startButton, start) }),
	)
}

// dropText is a drop zone's wording, which can't mention dropping where the window doesn't accept drops.
func dropText(drop, choose string) string {
	if desktop.CanDrop {
		return drop
	}
	return choose
}

func (app *App) keyZone(gtx layout.Context) layout.Dimensions {
	state := &app.open
	if state.privateKey == nil {
		return app.dropZone(gtx, dropZone{
			icon:   keyIcon,
			text:   dropText("Drop your private key file here", "Choose your private key file"),
			button: &state.chooseKeyButton,
			choose: chooseFileText,
		})
	}
	detail := "Private key"
	if state.privateKey.Name != "" {
		detail = "Key pair: " + state.privateKey.Name
	}
	chosen := chosenFile{
		icon: keyIcon, path: state.keyPath, detail: detail,
		changeButton: &state.changeKeyButton, isChangeDisabled: state.isWorking,
	}
	decoration := box{background: app.colors.surfaceAlt, border: app.colors.border, borderWidth: 1, radius: 12}
	if app.isDragging {
		decoration.border, decoration.borderWidth = app.colors.primary, 2
	}
	return decorate(gtx, decoration, symmetricInset(10, 13), minHeight(44, app.chosenFileRow(chosen)))
}

// chosenFile is a chosen file shown with its icon, name, a line about it, and a Change button.
type chosenFile struct {
	icon, path, detail string
	changeButton       *widget.Clickable
	isChangeDisabled   bool
}

func (app *App) chosenFileRow(chosen chosenFile) layout.Widget {
	colors := app.colors
	return func(gtx layout.Context) layout.Dimensions {
		return row(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return squareBadge(gtx, 34, box{background: colors.surface, radius: 9},
					app.icon(chosen.icon, 18, 2, colors.primaryText))
			}),
			spacer(12),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				name := label{
					style: styleSmallBold, color: colors.foreground, text: filepath.Base(chosen.path), maxLines: 1,
				}
				return column(gtx,
					layout.Rigid(app.label(name)),
					layout.Rigid(app.text(styleCaption, colors.muted, chosen.detail)),
				)
			}),
			spacer(12),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				change := button{kind: linkButton, text: "Change", isDisabled: chosen.isChangeDisabled}
				return app.button(gtx, chosen.changeButton, change)
			}),
		)
	}
}

// dropZone is the design's dashed target with a Choose button.
type dropZone struct {
	icon, text string
	button     *widget.Clickable
	choose     string // the button's text
}

func (app *App) dropZone(gtx layout.Context, zone dropZone) layout.Dimensions {
	colors := app.colors
	decoration := box{
		background: colors.surfaceAlt, border: colors.borderStrong, borderWidth: 1.5, radius: 14, isDashed: true,
	}
	foreground := colors.muted
	if app.isDragging {
		decoration.border, decoration.borderWidth, decoration.isDashed = colors.primary, 2, false
		foreground = colors.foreground
	}
	padding := layout.UniformInset(17.5)
	return decorate(gtx, decoration, padding, minHeight(61, func(gtx layout.Context) layout.Dimensions {
		return row(gtx,
			layout.Rigid(app.icon(zone.icon, 22, 1.8, colors.primaryText)),
			spacer(12),
			layout.Flexed(1, app.text(styleSmall, foreground, zone.text)),
			spacer(12),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return app.button(gtx, zone.button, button{kind: secondaryButton, text: zone.choose})
			}),
		)
	}))
}

// fileList draws the sealed files in a rounded, bordered list.
func (app *App) fileList(gtx layout.Context) layout.Dimensions {
	files := app.open.files
	colors := app.colors
	children := make([]layout.FlexChild, 0, len(files))
	for i, file := range files {
		isLast := i == len(files)-1
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if isLast {
				return app.fileRow(gtx, file)
			}
			return app.withRuleBelow(gtx, func(gtx layout.Context) layout.Dimensions { return app.fileRow(gtx, file) })
		}))
	}
	recording := op.Record(gtx.Ops)
	dims := layout.UniformInset(1).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return column(gtx, children...)
	})
	call := recording.Stop()
	box{background: colors.surfaceAlt, radius: 12}.paint(gtx, dims.Size)
	call.Add(gtx.Ops)
	border := box{border: colors.border, borderWidth: 1, radius: 12}
	if app.isDragging {
		border.border, border.borderWidth = colors.primary, 2
	}
	border.paint(gtx, dims.Size)
	return dims
}

func (app *App) fileRow(gtx layout.Context, file *sealedFile) layout.Dimensions {
	state := &app.open
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return symmetricInset(10, 12).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{layout.Rigid(fullWidth(app.fileHeader(file)))}
		below := layout.Inset{Top: 7, Bottom: 1, Left: resultIndent}
		if state.isWorking && file.result == nil {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return below.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return app.progressBar(gtx, float32(file.bytesRead.Load())/float32(max(file.sizeBytes, 1)))
				})
			}))
		}
		if file.result != nil {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return below.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return app.fileResult(gtx, file)
				})
			}))
		}
		return column(gtx, children...)
	})
}

// fileHeader is a file's icon, name, size, and remove button.
func (app *App) fileHeader(file *sealedFile) layout.Widget {
	colors := app.colors
	return func(gtx layout.Context) layout.Dimensions {
		items := []layout.FlexChild{
			layout.Rigid(app.icon("file-lock-2", 17, 2, colors.primaryText)),
			spacer(10),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				name := label{
					style: styleSmallBold, color: colors.foreground, text: filepath.Base(file.path), maxLines: 1,
				}
				return column(gtx,
					layout.Rigid(app.label(name)),
					layout.Rigid(app.text(styleCaption, colors.muted, formatSize(file.sizeBytes))),
				)
			}),
		}
		if !app.open.isWorking && file.result == nil {
			items = append(items, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return app.smallIconButton(gtx, &file.removeButton, "x", 16)
			}))
		}
		return row(gtx, items...)
	}
}

// progressBar is a thin bar filled to fraction, which is between 0 and 1.
func (app *App) progressBar(gtx layout.Context, fraction float32) layout.Dimensions {
	fraction = min(max(fraction, 0), 1)
	size := image.Pt(gtx.Constraints.Max.X, pixels(gtx, 3))
	box{background: app.colors.border, radius: 2}.paint(gtx, size)
	if filled := int(float32(size.X) * fraction); filled > 0 {
		rounded := clip.UniformRRect(image.Rectangle{Max: size}, size.Y/2).Push(gtx.Ops)
		fillRect(gtx, image.Rect(0, 0, filled, size.Y), app.colors.primary)
		rounded.Pop()
	}
	return layout.Dimensions{Size: size}
}

// fileResult is what happened to a file, with a link to the saved file.
func (app *App) fileResult(gtx layout.Context, file *sealedFile) layout.Dimensions {
	result := file.result
	style := app.messageStyle(result.kind)
	items := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: 1}.Layout(gtx, app.icon(style.icon, 15, 2, style.foreground))
		}),
		spacer(6),
		layout.Flexed(1, app.text(styleCaptionTight, style.foreground, result.text)),
	}
	if result.outputPath != "" {
		items = append(items, spacer(8), layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return app.textLink(gtx, &file.showButton, styleCaptionTight, app.colors.primaryText, showInFolderText)
		}))
	}
	return rowTop(gtx, items...)
}

// doneSummary is the card shown after opening: how many files opened, each file's result, and Open more files.
func (app *App) doneSummary(gtx layout.Context) layout.Dimensions {
	state := &app.open
	colors := app.colors
	summary := summarize(state.files)
	style := app.messageStyle(summary.kind)
	return column(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return rowTop(gtx,
				layout.Rigid(app.icon(style.icon, 26, 2, style.foreground)),
				spacer(12),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return column(gtx,
						layout.Rigid(app.text(styleSection, colors.foreground, summary.title)),
						spacer(1),
						layout.Rigid(app.text(styleSmall, colors.muted, summary.subtitle)),
					)
				}),
			)
		}),
		spacer(18),
		layout.Rigid(fullWidth(app.fileList)),
		spacer(18),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return app.button(gtx, &state.openMoreButton, button{kind: secondaryButton, text: "Open more files"})
		}),
	)
}
