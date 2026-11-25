package main

import (
	"image/color"
	"path/filepath"

	"gioui.org/layout"

	"github.com/vita-workflow/vita-decrypt/internal/vita"
)

func (app *App) layoutSealScreen(gtx layout.Context) layout.Dimensions {
	state := &app.seal
	app.handleSealClicks(gtx)
	key, isValid := state.recipient()
	if !state.isWorking && state.resultFor != resultKey(key, state.file) {
		state.result = nil // the key or file changed since, so the result no longer describes them
	}
	children := []layout.FlexChild{
		layout.Rigid(app.heading("Seal a file",
			"Test a key pair: seal a file to its public key the way Vita does, then open it with the private key.")),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return app.card(gtx, layout.UniformInset(24), func(gtx layout.Context) layout.Dimensions {
				return app.sealForm(gtx, key, isValid)
			})
		}),
	}
	if state.result != nil {
		children = append(children, spacer(16), layout.Rigid(app.sealResult))
	}
	return column(gtx, children...)
}

func (app *App) sealForm(gtx layout.Context, key string, isValid bool) layout.Dimensions {
	state := &app.seal
	errorMessage, helper := state.sealKeyNote(key, isValid)
	keyField := field{
		label: publicKeyLabel, editor: &state.keyEditor, isMultiline: true, errorMessage: errorMessage, helper: helper,
		placeholder: publicKeyPlaceholder,
		actions:     []layout.Widget{app.pasteButton(&state.paste), app.chooseKeyButton(&state.chooseKeyButton)},
	}
	children := []layout.FlexChild{
		layout.Rigid(fullWidth(func(gtx layout.Context) layout.Dimensions { return app.field(gtx, keyField) })),
		spacer(20),
		layout.Rigid(app.text(styleLabel, app.colors.foreground, "File to seal")),
		spacer(7),
		layout.Rigid(fullWidth(app.sealFileArea)),
	}
	if state.fileError != "" {
		children = append(children, spacer(10), layout.Rigid(app.messageText(dangerMessage, state.fileError)))
	}
	children = append(children, spacer(22), layout.Rigid(fullWidth(func(gtx layout.Context) layout.Dimensions {
		return alignEnd(gtx, func(gtx layout.Context) layout.Dimensions {
			start := button{
				kind: primaryButton, isLarge: true, text: "Seal file", isBusy: state.isWorking,
				isDisabled: !isValid || state.file == "" || state.isWorking,
			}
			return app.button(gtx, &state.startButton, start)
		})
	})))
	return column(gtx, children...)
}

// sealFileArea is the drop zone, or the chosen file and, while sealing, its progress.
func (app *App) sealFileArea(gtx layout.Context) layout.Dimensions {
	state := &app.seal
	colors := app.colors
	if state.file == "" {
		return app.dropZone(gtx, dropZone{
			icon:   fileUpIcon,
			text:   dropText("Drop the file to seal here", "Choose the file to seal"),
			button: &state.chooseFileButton,
			choose: chooseFileText,
		})
	}
	chosen := chosenFile{
		icon: fileUpIcon, path: state.file,
		detail: formatSize(state.sizeBytes) + " · saves " + filepath.Base(vita.SealedName(state.file)) +
			" next to it",
		changeButton: &state.changeFileButton, isChangeDisabled: state.isWorking,
	}
	decoration := box{background: colors.surfaceAlt, border: colors.border, borderWidth: 1, radius: 12}
	return decorate(gtx, decoration, symmetricInset(10, 13), func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{layout.Rigid(fullWidth(app.chosenFileRow(chosen)))}
		if state.isWorking {
			children = append(children, spacer(9), layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return app.progressBar(gtx, float32(state.bytesRead.Load())/float32(max(state.sizeBytes, 1)))
			}))
		}
		return column(gtx, children...)
	})
}

// sealResult says how sealing went; on success it offers to open the sealed file or show it.
func (app *App) sealResult(gtx layout.Context) layout.Dimensions {
	state := &app.seal
	result := state.result
	if result.outputPath == "" {
		return app.messageText(result.kind, result.text)(gtx)
	}
	return app.message(gtx, successMessage, func(gtx layout.Context, foreground color.NRGBA) layout.Dimensions {
		return column(gtx,
			layout.Rigid(app.text(styleSmallBold, foreground, result.text)),
			spacer(2),
			layout.Rigid(app.text(styleSmallTall, foreground,
				"Open it with your private key to finish the test. The original file is unchanged.")),
			spacer(7),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return row(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return app.textLink(gtx, &state.openNowButton, styleCaption, foreground, "Open it now")
					}),
					spacer(16),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return app.textLink(gtx, &state.showButton, styleCaption, foreground, showInFolderText)
					}),
				)
			}),
		)
	})
}
