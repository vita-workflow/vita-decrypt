package main

import (
	"image"
	"image/color"
	"path/filepath"

	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/vita-workflow/vita-decrypt/internal/vita"
)

// handleCreatedClicks acts on the created screen's buttons and reports whether Done went back to the form.
func (app *App) handleCreatedClicks(gtx layout.Context) bool {
	state := &app.create
	keyPair := state.created
	if state.copyPublicKeyButton.Clicked(gtx) {
		copyText(gtx, keyPair.PublicKey)
		state.publicKeyCopiedAt = gtx.Now
	}
	if state.copyFingerprintButton.Clicked(gtx) {
		copyText(gtx, vita.Fingerprint(keyPair.PublicKey))
		state.fingerprintCopiedAt = gtx.Now
	}
	if state.showPrivateKeyButton.Clicked(gtx) {
		app.showInFolder(keyPair.PrivateKeyPath)
	}
	if state.showPublicKeyButton.Clicked(gtx) {
		app.showInFolder(keyPair.PublicKeyPath)
	}
	if state.doneButton.Clicked(gtx) && state.backedUpCheckbox.Value {
		state.created = nil
		state.nameEditor.SetText("")
		state.nameError.clear()
		state.folderError = ""
		app.content.Position = layout.Position{}
		return true
	}
	return false
}

// layoutCreatedScreen shows the key pair just created.
func (app *App) layoutCreatedScreen(gtx layout.Context) layout.Dimensions {
	if app.handleCreatedClicks(gtx) {
		return app.layoutCreateScreen(gtx)
	}
	return column(gtx,
		layout.Rigid(app.createdHeading),
		layout.Rigid(app.backupWarning),
		spacer(14),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			padding := layout.Inset{Top: 18, Right: 24, Bottom: 24, Left: 24}
			return app.card(gtx, padding, func(gtx layout.Context) layout.Dimensions {
				return column(gtx,
					app.createdSection(17, true, app.publicKeySection),
					app.createdSection(17, true, app.fingerprintSection),
					app.createdSection(0, false, app.backedUpSection),
				)
			})
		}),
	)
}

// createdSection is one section of the created card, with its bottom padding and, between sections, a rule.
func (app *App) createdSection(bottomDp unit.Dp, hasRule bool, content layout.Widget) layout.FlexChild {
	return layout.Rigid(fullWidth(func(gtx layout.Context) layout.Dimensions {
		padded := func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: bottomDp}.Layout(gtx, content)
		}
		if !hasRule {
			return padded(gtx)
		}
		return app.withRuleBelow(gtx, padded)
	}))
}

func (app *App) createdHeading(gtx layout.Context) layout.Dimensions {
	colors := app.colors
	keyPair := app.create.created
	return layout.Inset{Top: 2, Bottom: 22}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return row(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				badge := box{background: colors.successFill, border: colors.successBorder, borderWidth: 1, radius: 14}
				return squareBadge(gtx, 44, badge, app.icon("shield-check", 25, 2, colors.success))
			}),
			spacer(13),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return column(gtx,
					layout.Rigid(app.text(styleTitle, colors.foreground, "Key pair "+keyPair.Name+" created")),
					spacer(2),
					layout.Rigid(app.text(styleBody, colors.muted,
						"Your key files are saved. Add the public key to Vita next.")),
				)
			}),
		)
	})
}

// backupWarning asks the person to back up the private key file, and shows where it is.
func (app *App) backupWarning(gtx layout.Context) layout.Dimensions {
	state := &app.create
	return app.message(gtx, warningMessage, func(gtx layout.Context, foreground color.NRGBA) layout.Dimensions {
		where := label{
			style: styleCaption, color: foreground, breaksAnywhere: true,
			text: displayPath(state.created.PrivateKeyPath, string(filepath.Separator)),
		}
		return column(gtx,
			layout.Rigid(app.text(styleSmallBold, foreground, "Back up your private key file now.")),
			spacer(2),
			layout.Rigid(app.text(styleSmallTall, foreground,
				"Vita can’t recover it, and without it your sealed files can’t be opened.")),
			spacer(7),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return rowTop(gtx,
					layout.Flexed(1, app.label(where)),
					spacer(16),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						show := &state.showPrivateKeyButton
						return app.textLink(gtx, show, styleCaption, foreground, showInFolderText)
					}),
				)
			}),
		)
	})
}

func (app *App) publicKeySection(gtx layout.Context) layout.Dimensions {
	state := &app.create
	colors := app.colors
	copyPublicKey := button{kind: primaryButton, icon: copyIcon, text: "Copy public key"}
	if isCopyConfirmed(gtx, state.publicKeyCopiedAt) {
		copyPublicKey.icon, copyPublicKey.text = checkIcon, "Copied"
	}
	return layout.Inset{Top: 2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return column(gtx,
			layout.Rigid(app.text(styleLabel, colors.foreground, publicKeyLabel)),
			spacer(7),
			layout.Rigid(fullWidth(app.publicKeyBox)),
			spacer(10),
			layout.Rigid(fullWidth(func(gtx layout.Context) layout.Dimensions {
				return row(gtx,
					layout.Flexed(1, app.publicKeyFileNote),
					spacer(12),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return app.button(gtx, &state.copyPublicKeyButton, copyPublicKey)
					}),
				)
			})),
		)
	})
}

// publicKeyBox shows the public key, read-only.
func (app *App) publicKeyBox(gtx layout.Context) layout.Dimensions {
	colors := app.colors
	size := image.Pt(gtx.Constraints.Max.X, pixels(gtx, 72))
	box{background: colors.surfaceAlt, border: colors.borderStrong, borderWidth: 1, radius: 10}.paint(gtx, size)
	offset(gtx, image.Pt(pixels(gtx, 12), pixels(gtx, 11)), func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints = layout.Constraints{Max: image.Pt(size.X-pixels(gtx, 24), size.Y-pixels(gtx, 22))}
		key := label{
			style: styleSmallTall, color: colors.muted, text: app.create.created.PublicKey, maxLines: 2,
			breaksAnywhere: true,
		}
		return key.layout(gtx, app.shaper)
	})
	return layout.Dimensions{Size: size}
}

// publicKeyFileNote says where the public key is saved.
func (app *App) publicKeyFileNote(gtx layout.Context) layout.Dimensions {
	state := &app.create
	colors := app.colors
	fileName := label{
		style: styleCaption, color: colors.foreground, text: filepath.Base(state.created.PublicKeyPath), maxLines: 1,
	}
	return column(gtx,
		layout.Rigid(app.text(styleCaption, colors.muted,
			"Paste this into Vita when you add the key pair. It’s also saved in")),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return row(gtx,
				layout.Rigid(app.label(fileName)),
				spacer(8),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					show := &state.showPublicKeyButton
					return app.textLink(gtx, show, styleCaption, colors.primaryText, showInFolderText)
				}),
			)
		}),
	)
}

func (app *App) fingerprintSection(gtx layout.Context) layout.Dimensions {
	state := &app.create
	colors := app.colors
	fingerprint := vita.Fingerprint(state.created.PublicKey)
	return layout.Inset{Top: 16}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return column(gtx,
			layout.Rigid(app.text(styleLabel, colors.foreground, fingerprintLabel)),
			spacer(5),
			layout.Rigid(fullWidth(app.fingerprintRow(fingerprint, &state.copyFingerprintButton,
				state.fingerprintCopiedAt, layout.Inset{}))),
			spacer(3),
			layout.Rigid(app.text(styleCaption, colors.muted,
				"After you add the key pair, check that Vita shows this same fingerprint.")),
		)
	})
}

func (app *App) backedUpSection(gtx layout.Context) layout.Dimensions {
	state := &app.create
	return layout.Inset{Top: 16}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return row(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.W.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return app.checkbox(gtx, &state.backedUpCheckbox, "I’ve backed up my private key file")
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				done := button{kind: secondaryButton, text: "Done", isDisabled: !state.backedUpCheckbox.Value}
				return app.button(gtx, &state.doneButton, done)
			}),
		)
	})
}
