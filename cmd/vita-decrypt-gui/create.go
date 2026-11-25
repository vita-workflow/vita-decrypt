package main

import (
	"errors"
	"image"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"

	"github.com/vita-workflow/vita-decrypt/internal/desktop"
	"github.com/vita-workflow/vita-decrypt/internal/vita"
)

// createState is the create screen, which makes a key pair, then shows it.
type createState struct {
	nameEditor  widget.Editor
	isClassic   bool
	folder      string
	nameError   inputError // why the last attempt failed, until the name changes
	folderError string

	created             *vita.KeyPair
	publicKeyCopiedAt   time.Time
	fingerprintCopiedAt time.Time
	backedUpCheckbox    widget.Bool

	postQuantumButton, classicButton, changeFolderButton, createButton widget.Clickable
	copyPublicKeyButton, copyFingerprintButton                         widget.Clickable
	showPrivateKeyButton, showPublicKeyButton, doneButton              widget.Clickable
}

func (state *createState) init() {
	state.nameEditor.SingleLine = true
	state.nameEditor.Submit = true
	state.folder = defaultKeyFolder()
}

// defaultKeyFolder is Documents in the home folder if there is one, else the home folder.
func defaultKeyFolder() string {
	home, err := os.UserHomeDir()
	if err != nil {
		workingFolder, _ := os.Getwd() // "" if even this fails, which saves in the working folder
		return workingFolder
	}
	documents := filepath.Join(home, "Documents")
	if info, err := os.Stat(documents); err == nil && info.IsDir() {
		return documents
	}
	return home
}

// displayPath shows a path inside the home folder relative to it, like "Documents / Vita keys".
func displayPath(path, separator string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	relative, err := filepath.Rel(home, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return path
	}
	if relative == "." {
		return "Home folder"
	}
	return strings.Join(strings.Split(relative, string(filepath.Separator)), separator)
}

const nameRule = "Use 1–40 letters, digits, or hyphens, starting with a letter or digit."

func (app *App) createKeyPair() {
	state := &app.create
	name := strings.TrimSpace(state.nameEditor.Text())
	if !vita.IsValidName(name) {
		return
	}
	keyPair, err := vita.CreateKeyPair(state.folder, name, state.isClassic)
	switch {
	case err == nil:
		state.created = &keyPair
		state.backedUpCheckbox.Value = false
		state.publicKeyCopiedAt, state.fingerprintCopiedAt = time.Time{}, time.Time{}
		app.content.Position = layout.Position{}
	case errors.Is(err, fs.ErrExist):
		state.nameError.set(vita.PrivateKeyFileName(name)+" or "+vita.PublicKeyFileName(name)+
			" already exists in this folder. Choose another name or folder.", name)
	default:
		state.folderError = "The key pair wasn’t created: " + err.Error()
	}
}

// handleCreateClicks acts on the create form's buttons and the name field's Enter key.
func (app *App) handleCreateClicks(gtx layout.Context, isNameValid bool) {
	state := &app.create
	for {
		received, ok := state.nameEditor.Update(gtx)
		if !ok {
			break
		}
		if _, ok := received.(widget.SubmitEvent); ok && isNameValid {
			app.createKeyPair()
		}
	}
	if state.postQuantumButton.Clicked(gtx) {
		state.isClassic = false
	}
	if state.classicButton.Clicked(gtx) {
		state.isClassic = true
	}
	if state.changeFolderButton.Clicked(gtx) {
		dialog := desktop.Dialog{Title: "Choose where to save the key files", Folder: state.folder}
		app.runDialog(chooseOne(app.system.chooseFolder, dialog),
			func(paths []string) { state.folder, state.folderError = paths[0], "" },
			func(message string) { state.folderError = message })
	}
	if state.createButton.Clicked(gtx) {
		app.createKeyPair()
	}
}

func (app *App) layoutCreateScreen(gtx layout.Context) layout.Dimensions {
	state := &app.create
	if state.created != nil {
		return app.layoutCreatedScreen(gtx)
	}
	name := strings.TrimSpace(state.nameEditor.Text())
	isValid := vita.IsValidName(name)
	app.handleCreateClicks(gtx, isValid)
	if state.created != nil {
		return app.layoutCreatedScreen(gtx)
	}
	nameError := state.nameError.messageFor(name)
	if nameError == "" && name != "" && !isValid {
		nameError = nameRule
	}
	savedName := name
	if !isValid {
		savedName = "name"
	}
	nameField := field{
		label: "Name", editor: &state.nameEditor, placeholder: "e.g. family-vault", errorMessage: nameError,
		helper: "1–40 letters, digits, or hyphens, starting with a letter or digit. Use the same name in Vita.",
	}
	return column(gtx,
		layout.Rigid(app.heading("Create a key pair",
			"Vita seals files to the public key. Only the private key file can open them.")),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return app.card(gtx, layout.UniformInset(24), func(gtx layout.Context) layout.Dimensions {
				return column(gtx,
					layout.Rigid(fullWidth(func(gtx layout.Context) layout.Dimensions {
						return app.field(gtx, nameField)
					})),
					spacer(20),
					layout.Rigid(fullWidth(func(gtx layout.Context) layout.Dimensions {
						return column(gtx,
							layout.Rigid(app.text(styleLabel, app.colors.foreground, "Key type")),
							spacer(7),
							layout.Rigid(fullWidth(app.keyOptions)),
						)
					})),
					spacer(20),
					layout.Rigid(fullWidth(app.folderField(savedName))),
					spacer(22),
					layout.Rigid(fullWidth(func(gtx layout.Context) layout.Dimensions {
						return alignEnd(gtx, func(gtx layout.Context) layout.Dimensions {
							create := button{
								kind: primaryButton, isLarge: true, text: "Create key pair", isDisabled: !isValid,
							}
							return app.button(gtx, &state.createButton, create)
						})
					})),
				)
			})
		}),
	)
}

// keyOption is one of the key types to choose from.
type keyOption struct {
	button        *widget.Clickable
	isSelected    bool
	isRecommended bool
	title         string
	description   string
}

func (app *App) keyOptionContent(option keyOption) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		var items []layout.FlexChild
		if option.isRecommended {
			items = append(items,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return app.pill(gtx, "Recommended") }),
				spacer(5))
		}
		items = append(items,
			layout.Rigid(app.text(styleStrong, app.colors.foreground, option.title)),
			spacer(5),
			layout.Rigid(app.text(styleCaptionTight, app.colors.muted, option.description)),
		)
		return column(gtx, items...)
	}
}

// keyOptions is the two-column choice of key type, both cards as tall as the taller one.
func (app *App) keyOptions(gtx layout.Context) layout.Dimensions {
	state := &app.create
	gap := pixels(gtx, 12)
	width := (gtx.Constraints.Max.X - gap) / 2
	options := []keyOption{
		{
			&state.postQuantumButton, !state.isClassic, true, "Post-quantum",
			"Protects against future quantum computers. Works with Vita and age 1.3 or later.",
		},
		{&state.classicButton, state.isClassic, false, "Classic", "For older age tools. Shorter public key."},
	}
	// The text leaves room for the padding on both sides and the check mark.
	textWidth := width - pixels(gtx, 30) - pixels(gtx, 26)
	// Measure both, then draw both at the taller height.
	height := pixels(gtx, 112)
	for _, option := range options {
		recording := op.Record(gtx.Ops)
		measure := gtx
		measure.Constraints = layout.Constraints{Max: image.Pt(textWidth, gtx.Constraints.Max.Y)}
		dims := app.keyOptionContent(option)(measure)
		recording.Stop()
		height = max(height, dims.Size.Y+pixels(gtx, 30))
	}
	for i, option := range options {
		offset(gtx, image.Pt(i*(width+gap), 0), func(gtx layout.Context) layout.Dimensions {
			return app.keyOptionCard(gtx, option, image.Pt(width, height), textWidth)
		})
	}
	return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, height)}
}

func (app *App) keyOptionCard(gtx layout.Context, option keyOption, size image.Point, textWidth int) layout.Dimensions {
	colors := app.colors
	return app.clickable(gtx, option.button, 14, func(gtx layout.Context) layout.Dimensions {
		decoration := box{background: colors.surface, border: colors.borderStrong, borderWidth: 1, radius: 14}
		if option.isSelected {
			decoration.border, decoration.borderWidth = colors.primary, 2
		} else if option.button.Hovered() {
			decoration.background = colors.surfaceAlt
		}
		decoration.paint(gtx, size)
		padding := pixels(gtx, 15)
		offset(gtx, image.Pt(padding, padding), func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints = layout.Constraints{Max: image.Pt(textWidth, size.Y)}
			return app.keyOptionContent(option)(gtx)
		})
		if option.isSelected {
			const checkSize = 21
			corner := image.Pt(size.X-pixels(gtx, 10)-pixels(gtx, checkSize), pixels(gtx, 10))
			offset(gtx, corner, func(gtx layout.Context) layout.Dimensions {
				return squareBadge(gtx, checkSize, box{background: colors.primary, radius: 11},
					app.icon(checkIcon, 13, 2.5, colors.onPrimary))
			})
		}
		return layout.Dimensions{Size: size}
	})
}

func (app *App) folderField(savedName string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		state := &app.create
		colors := app.colors
		children := []layout.FlexChild{
			layout.Rigid(app.text(styleLabel, colors.foreground, "Save the key files in")),
			spacer(7),
			layout.Rigid(fullWidth(app.folderPicker)),
			spacer(7),
			layout.Rigid(app.text(styleCaption, colors.muted,
				"Saves "+vita.PrivateKeyFileName(savedName)+" and "+vita.PublicKeyFileName(savedName))),
		}
		if state.folderError != "" {
			children = append(children, spacer(10), layout.Rigid(app.messageText(dangerMessage, state.folderError)))
		}
		return column(gtx, children...)
	}
}

// folderPicker is the chosen folder with a Change button.
func (app *App) folderPicker(gtx layout.Context) layout.Dimensions {
	state := &app.create
	colors := app.colors
	decoration := box{background: colors.surface, border: colors.borderStrong, borderWidth: 1, radius: 10}
	padding := layout.Inset{Top: 7, Right: 8, Bottom: 7, Left: 12}
	return decorate(gtx, decoration, padding, minHeight(34, func(gtx layout.Context) layout.Dimensions {
		folder := label{
			style: styleSmall, color: colors.foreground, text: displayPath(state.folder, " / "), maxLines: 1,
		}
		return row(gtx,
			layout.Rigid(app.icon("folder-open", 18, 2, colors.primaryText)),
			spacer(10),
			layout.Flexed(1, app.label(folder)),
			spacer(10),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return app.button(gtx, &state.changeFolderButton, button{kind: secondaryButton, text: "Change…"})
			}),
		)
	}))
}
