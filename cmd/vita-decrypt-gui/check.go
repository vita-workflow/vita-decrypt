package main

import (
	"errors"
	"strings"
	"time"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/vita-workflow/vita-decrypt/internal/desktop"
	"github.com/vita-workflow/vita-decrypt/internal/vita"
)

// checkState is the check screen, which shows a public key's fingerprint and compares it with the one Vita shows.
type checkState struct {
	keyEditor, expectedEditor   widget.Editor
	pasteKey, pasteExpected     paster
	copyButton, chooseKeyButton widget.Clickable
	copiedAt                    time.Time
	keyFileError                inputError // why the chosen public key file couldn't be read
}

func (state *checkState) init() {
	state.expectedEditor.SingleLine = true
}

const (
	publicKeyLabel       = "Public key"
	publicKeyPlaceholder = "Paste a public key (it starts with age1)"
	fingerprintLabel     = "Fingerprint"
)

var publicKeyDialog = desktop.Dialog{
	Title:   "Choose a public key file",
	Filters: []desktop.Filter{{Name: "Public key files", Patterns: []string{"*.txt"}}},
}

// compactKey is a pasted key with its spaces and line breaks taken out.
func compactKey(text string) string { return strings.Join(strings.Fields(text), "") }

// sameFingerprint compares fingerprints ignoring spaces and case.
func sameFingerprint(first, second string) bool {
	return strings.EqualFold(compactKey(first), compactKey(second))
}

// keyFingerprint is the fingerprint of key, or "" if key isn't a valid public key.
func keyFingerprint(key string) string {
	if key == "" || vita.CheckRecipient(key) != nil {
		return ""
	}
	return vita.Fingerprint(key)
}

func (app *App) layoutCheckScreen(gtx layout.Context) layout.Dimensions {
	state := &app.check
	key := compactKey(state.keyEditor.Text())
	fingerprint := keyFingerprint(key)
	if state.copyButton.Clicked(gtx) && fingerprint != "" {
		copyText(gtx, fingerprint)
		state.copiedAt = gtx.Now
	}
	state.pasteKey.update(gtx, &state.keyEditor)
	state.pasteExpected.update(gtx, &state.expectedEditor)
	if state.chooseKeyButton.Clicked(gtx) {
		app.choosePublicKey(&state.keyEditor, &state.keyFileError)
	}
	return column(gtx,
		layout.Rigid(app.heading("Check a fingerprint", "Make sure the key in Vita is the one you created.")),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return app.card(gtx, layout.UniformInset(24), app.checkFields)
		}),
		spacer(16),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return app.card(gtx, layout.UniformInset(24), func(gtx layout.Context) layout.Dimensions {
				return app.checkResult(gtx, key, fingerprint)
			})
		}),
	)
}

func (app *App) checkFields(gtx layout.Context) layout.Dimensions {
	state := &app.check
	keyField := field{
		label: publicKeyLabel, editor: &state.keyEditor, isMultiline: true, placeholder: publicKeyPlaceholder,
		errorMessage: state.keyFileError.messageFor(state.keyEditor.Text()),
		actions:      []layout.Widget{app.pasteButton(&state.pasteKey), app.chooseKeyButton(&state.chooseKeyButton)},
	}
	expectedField := field{
		label: "Fingerprint Vita shows (optional)", editor: &state.expectedEditor,
		placeholder: "e.g. D6B3 1A36 E35D 662F", actions: []layout.Widget{app.pasteButton(&state.pasteExpected)},
	}
	return column(gtx,
		layout.Rigid(fullWidth(func(gtx layout.Context) layout.Dimensions { return app.field(gtx, keyField) })),
		spacer(16),
		layout.Rigid(fullWidth(func(gtx layout.Context) layout.Dimensions { return app.field(gtx, expectedField) })),
	)
}

// checkResult is the fingerprint and whether it matches, or why there isn't one.
func (app *App) checkResult(gtx layout.Context, key, fingerprint string) layout.Dimensions {
	colors := app.colors
	switch {
	case key == "":
		return minHeight(72, func(gtx layout.Context) layout.Dimensions {
			return row(gtx,
				layout.Rigid(app.icon("fingerprint", 27, 2, colors.primaryText)),
				spacer(13),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return column(gtx,
						layout.Rigid(app.text(styleSection, colors.muted, "Your fingerprint will appear here")),
						spacer(1),
						layout.Rigid(app.text(styleSmall, colors.muted, "Paste the public key above to check it.")),
					)
				}),
			)
		})(gtx)
	case fingerprint == "":
		return app.messageText(dangerMessage, invalidKeyMessage(key))(gtx)
	}
	children := []layout.FlexChild{
		layout.Rigid(app.text(styleLabel, colors.foreground, fingerprintLabel)),
		layout.Rigid(fullWidth(app.fingerprintRow(fingerprint, &app.check.copyButton, app.check.copiedAt,
			layout.Inset{Top: 5, Bottom: 3}))),
	}
	if expected := strings.TrimSpace(app.check.expectedEditor.Text()); expected != "" {
		kind := dangerMessage
		message := "The fingerprints don’t match. Don’t seal files to this key until you find out why."
		if sameFingerprint(expected, fingerprint) {
			kind, message = successMessage, "The fingerprints match."
		}
		children = append(children, spacer(12), layout.Rigid(app.messageText(kind, message)))
	}
	return column(gtx, children...)
}

// invalidKeyMessage explains why key isn't a valid public key.
func invalidKeyMessage(key string) string {
	if !strings.HasPrefix(strings.ToLower(key), "age1") {
		return "This isn’t a valid public key. Vita public keys start with age1."
	}
	return "This isn’t a valid public key. Check that you copied all of it."
}

// fingerprintRow is a fingerprint with a button that copies it.
func (app *App) fingerprintRow(
	fingerprint string, copyButton *widget.Clickable, copiedAt time.Time, padding layout.Inset,
) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		icon := copyIcon
		if isCopyConfirmed(gtx, copiedAt) {
			icon = checkIcon
		}
		return row(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return padding.Layout(gtx, app.text(styleFingerprint, app.colors.foreground, fingerprint))
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return app.button(gtx, copyButton, button{kind: iconButton, icon: icon, iconSizeDp: 17})
			}),
		)
	}
}

// choosePublicKey lets the person pick a public key file and puts its key in editor. If the file can't be read,
// problem says why.
func (app *App) choosePublicKey(editor *widget.Editor, problem *inputError) {
	choose := chooseOne(app.system.openFile, publicKeyDialog)
	app.runDialog(choose, func(paths []string) {
		key, err := vita.ReadPublicKey(paths[0])
		switch {
		case errors.Is(err, vita.ErrPrivateKeyFile):
			problem.set("That’s your private key file. Choose the public key file (its name ends in -public-key.txt).",
				editor.Text())
		case err != nil:
			problem.set("That file doesn’t hold a public key: "+err.Error(), editor.Text())
		default:
			editor.SetText(key)
			problem.clear()
		}
	}, func(message string) { problem.set(message, editor.Text()) })
}

// chooseKeyButton is the Choose file… button beside a public key field.
func (app *App) chooseKeyButton(clickable *widget.Clickable) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		choose := button{kind: linkButton, icon: "folder-open", iconSizeDp: 15, text: chooseFileText}
		return app.button(gtx, clickable, choose)
	}
}
