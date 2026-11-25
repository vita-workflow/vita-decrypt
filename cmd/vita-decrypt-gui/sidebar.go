package main

import (
	"image"

	"gioui.org/layout"
	"gioui.org/widget"
)

// navigationItem is a sidebar entry; its index is its screen.
type navigationItem struct{ icon, text string }

var navigationItems = [screenCount]navigationItem{
	openScreen:   {"file-lock-2", "Open a sealed file"},
	createScreen: {keyIcon, "Create a key pair"},
	checkScreen:  {"fingerprint", "Check a fingerprint"},
	sealScreen:   {"lock", "Seal a file"},
}

// wordmark is the Sprout icon, "Vita", and a small "DECRYPT".
func (app *App) wordmark(gtx layout.Context) layout.Dimensions {
	colors := app.colors
	return minHeight(34, func(gtx layout.Context) layout.Dimensions {
		return row(gtx,
			layout.Rigid(app.icon("sprout", 22, 1.8, colors.primaryText)),
			spacer(8),
			layout.Rigid(app.text(styleWordmark, colors.foreground, "Vita")),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: 7, Left: 2}.Layout(gtx, app.text(styleWordmarkSmall, colors.muted, "Decrypt"))
			}),
		)
	})(gtx)
}

// handleSidebarClicks switches screens and themes and opens About.
func (app *App) handleSidebarClicks(gtx layout.Context) {
	for i := range app.navigationButtons {
		if app.navigationButtons[i].Clicked(gtx) {
			app.screen = screen(i)
			app.content.Position = layout.Position{}
		}
	}
	if app.lightThemeButton.Clicked(gtx) && app.isDark {
		app.setDark(false)
		saveDarkTheme(false)
	}
	if app.darkThemeButton.Clicked(gtx) && !app.isDark {
		app.setDark(true)
		saveDarkTheme(true)
	}
	if app.aboutButton.Clicked(gtx) {
		app.isAboutOpen = true
	}
}

func (app *App) sidebar(gtx layout.Context) layout.Dimensions {
	colors := app.colors
	size := image.Pt(pixels(gtx, 240), gtx.Constraints.Max.Y)
	fillRect(gtx, image.Rectangle{Max: size}, colors.surface)
	ruleWidth := max(1, pixels(gtx, 1))
	fillRect(gtx, image.Rect(size.X-ruleWidth, 0, size.X, size.Y), colors.border)
	app.handleSidebarClicks(gtx)

	gtx.Constraints = layout.Exact(image.Pt(size.X-ruleWidth, size.Y))
	layout.Inset{Top: 28, Right: 18, Bottom: 20, Left: 18}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{layout.Rigid(app.wordmark), spacer(34)}
		for i, item := range navigationItems {
			if i > 0 {
				children = append(children, spacer(7))
			}
			children = append(children, layout.Rigid(app.navigationButton(screen(i), item)))
		}
		children = append(children,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Dimensions{Size: gtx.Constraints.Min}
			}),
			layout.Rigid(app.themeToggle),
			spacer(14),
			layout.Rigid(app.aboutLink),
		)
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
	return layout.Dimensions{Size: size}
}

func (app *App) navigationButton(target screen, item navigationItem) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		colors := app.colors
		style := ghostStyle{
			minHeightDp: 46, paddingXDp: 14, radius: 14, hoverBackground: colors.surfaceAlt, isFullWidth: true,
		}
		foreground := colors.foreground
		if app.screen == target {
			style.background, style.hoverBackground, foreground = colors.primary, colors.primary, colors.onPrimary
		}
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return app.ghostButton(gtx, &app.navigationButtons[target], style, func(gtx layout.Context) layout.Dimensions {
			return row(gtx,
				layout.Rigid(app.icon(item.icon, 20, 1.8, foreground)),
				spacer(12),
				layout.Rigid(app.text(styleNavigation, foreground, item.text)),
			)
		})
	}
}

func (app *App) themeToggle(gtx layout.Context) layout.Dimensions {
	gtx.Constraints.Min.X = 0
	decoration := box{background: app.colors.surfaceAlt, border: app.colors.border, borderWidth: 1, radius: 999}
	return decorate(gtx, decoration, layout.UniformInset(4), func(gtx layout.Context) layout.Dimensions {
		return row(gtx,
			layout.Rigid(app.themeOption(&app.lightThemeButton, "sun", !app.isDark)),
			layout.Rigid(app.themeOption(&app.darkThemeButton, "moon", app.isDark)),
		)
	})
}

func (app *App) themeOption(clickable *widget.Clickable, icon string, isSelected bool) layout.Widget {
	colors := app.colors
	return func(gtx layout.Context) layout.Dimensions {
		return app.clickable(gtx, clickable, 14, func(gtx layout.Context) layout.Dimensions {
			size := image.Pt(pixels(gtx, 34), pixels(gtx, 28))
			foreground := colors.muted
			if isSelected {
				box{background: colors.primary, radius: 14}.paint(gtx, size)
				foreground = colors.onPrimary
			}
			return center(gtx, size, app.icon(icon, 16, 2, foreground))
		})
	}
}

func (app *App) aboutLink(gtx layout.Context) layout.Dimensions {
	colors := app.colors
	foreground := colors.muted
	if app.aboutButton.Hovered() {
		foreground = colors.foreground
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	style := ghostStyle{
		minHeightDp: 38, paddingXDp: 10, radius: 10, hoverBackground: colors.surfaceAlt, isFullWidth: true,
	}
	return app.ghostButton(gtx, &app.aboutButton, style, func(gtx layout.Context) layout.Dimensions {
		return row(gtx,
			layout.Rigid(app.icon("info", 18, 2, foreground)),
			spacer(10),
			layout.Rigid(app.text(styleAbout, foreground, "About")),
		)
	})
}
