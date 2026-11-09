package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"

	"github.com/CarlOpenLab/omniflow/internal/uitokens"
	"github.com/CarlOpenLab/omniflow/locales"
)

// The pages of Settings, in the order the list beside them shows.
const (
	settingsPaneAppearance = iota
	settingsPaneCapture
	settingsPaneRules
	settingsPaneMocks
	settingsPaneData
	settingsPaneUpdates
	settingsPaneAbout
)

type settingsPage struct {
	id    int
	title func() string
	icon  *ui.SVG
}

var settingsPages = []settingsPage{
	{settingsPaneAppearance, locales.SettingsAppearance, iconSun},
	{settingsPaneCapture, locales.SettingsCapture, iconGlobe},
	{settingsPaneRules, locales.SettingsRules, iconShield},
	{settingsPaneMocks, locales.SettingsMocks, iconServer},
	{settingsPaneData, locales.SettingsData, iconFolder},
	{settingsPaneUpdates, locales.SettingsUpdates, iconRefresh},
	{settingsPaneAbout, locales.SettingsAbout, iconInfo},
}

// settingsDialog is the one place appearance, capture, data and updates are
// configured: a page list beside the page it has chosen, rather than every
// setting stacked in one column.
func (a *app) settingsDialog(c *ui.Context, pal *uitokens.Palette) {
	ui.DialogBase(c, &a.settingsOpen, func(backdrop, panel ui.Element) {
		backdrop.Background(pal.Overlay)
		panel.Width(760).Height(560).Radius(uitokens.CardRadius).Background(pal.Popover).
			Border(1, pal.EdgeHigh).Shadow(0, 30, 80, 0, pal.FloatShadow).
			InsetShadow(0, 1, 0, 0, ui.RGBA(255, 255, 255, 0.08)).
			Clip()

		panel.Children(func() {
			// The window names itself once; each page names itself again in
			// the header above its settings.
			head := ui.Row(c).Height(52).Shrink(0).Padding(0, 16).Gap(8)
			head.Children(func() {
				ui.Text(c, locales.SettingsTitle()).FontSize(15).FontWeight(700).
					TextColor(pal.Text).Grow(1)
				if b := iconButton(c, pal, iconX, locales.SettingsClose(), 26); b.Clicked() {
					a.settingsOpen = false
				}
			})

			row := ui.Row(c).Grow(1).MinHeight(0).AlignItems(ui.Stretch)
			row.Children(func() {
				// Settings: the pages down the left, the chosen one on a card
				// of its own at the right.
				ui.Column(c).Width(184).Shrink(0).Padding(8, 8, 8, 8).Gap(2).
					Children(func() {
						for _, page := range settingsPages {
							a.settingsNav(c, pal, page)
						}
					})
				ui.Column(c).Grow(1).MinWidth(0).Padding(8, 8, 8, 8).Children(func() {
					card := ui.Column(c).Fill().MinWidth(0).Radius(10).Clip().
						Background(pal.Card).Border(1, pal.Edge)
					card.Children(func() {
						// The page names itself in a header that stays while
						// the settings below scroll.
						ui.Row(c).Height(44).Shrink(0).Padding(0, 18).Children(func() {
							ui.Text(c, settingsPageTitle(a.settingsPane)).FontSize(14).
								FontWeight(650).TextColor(pal.Text)
						})
						ui.Scroll(c).Grow(1).MinHeight(0).Children(func() {
							ui.Column(c).FillWidth().Padding(18).Gap(14).Children(func() {
								switch a.settingsPane {
								case settingsPaneCapture:
									a.settingsCapturePage(c, pal)
								case settingsPaneRules:
									a.settingsRulesPage(c, pal)
								case settingsPaneMocks:
									a.settingsMocksPage(c, pal)
								case settingsPaneData:
									a.settingsDataPage(c, pal)
								case settingsPaneUpdates:
									a.settingsUpdatesPage(c, pal)
								case settingsPaneAbout:
									a.settingsAboutPage(c, pal)
								default:
									a.settingsAppearancePage(c, pal)
								}
							})
						})
					})
				})
			})

			ui.Row(c).Height(52).Shrink(0).Padding(0, 16).Justify(ui.End).
				BorderWidth(1, 0, 0, 0).BorderColor(pal.Divider).Children(func() {
				if ui.PrimaryButton(c.Key("settings-done"), locales.CommonDone()).Clicked() {
					a.settingsOpen = false
				}
			})
		})
	})
}

// settingsPageTitle is the name of a page, for the header above its settings.
func settingsPageTitle(pane int) string {
	for _, page := range settingsPages {
		if page.id == pane {
			return page.title()
		}
	}
	return locales.SettingsTitle()
}

// settingsNav is one page in the window's own list; the chosen page is raised
// off the column and keeps its icon in the accent.
func (a *app) settingsNav(c *ui.Context, pal *uitokens.Palette, page settingsPage) {
	selected := a.settingsPane == page.id
	item := ui.ButtonBase(c.Key(fmt.Sprintf("settings-page:%d", page.id))).Height(32).
		Shrink(0).Padding(0, 10).Gap(10).Radius(7).Label(page.title())
	if selected {
		item.Background(pal.AccentSoft)
	} else if item.Hovered() {
		item.Background(pal.RowHover)
	}
	item.Children(func() {
		iconColor := pal.TextMuted
		if selected {
			iconColor = pal.Accent
		}
		ui.Icon(c, page.icon).Size(14, 14).TextColor(iconColor)
		ui.Text(c, page.title()).FontSize(12.5).TextColor(pal.Text).SingleLine()
	})
	if item.Clicked() {
		a.settingsPane = page.id
	}
}

// settingsGroup puts a page's settings on one soft panel, so a page reads as
// a few groups rather than a white field of rows.
func settingsGroup(c *ui.Context, pal *uitokens.Palette, fn func()) {
	ui.Column(c).Radius(10).Background(pal.Chip).Padding(12, 16).Children(fn)
}

// settingsNote is the quiet line under a group: what the settings above it
// mean, in the page's own words.
func settingsNote(c *ui.Context, pal *uitokens.Palette, text string) {
	ui.Text(c, text).FontSize(10.5).TextColor(pal.TextFaint).LineHeight(1.45)
}

func (a *app) settingsAppearancePage(c *ui.Context, pal *uitokens.Palette) {
	settingsGroup(c, pal, func() {
		ui.Form(c, func() {
			mode := themeMode(a.prefs.Theme)
			labels := make([]string, len(themes))
			current := 0
			for i, t := range themes {
				labels[i] = t.label()
				if t.mode == mode {
					current = i
				}
			}
			tidx := current
			ui.Field(c, locales.SettingsTheme(), func() {
				segmentedKey(c, pal, "theme:", &tidx, labels)
			})
			if tidx != current && tidx >= 0 && tidx < len(themes) {
				a.setTheme(themes[tidx].mode)
			}

			// Language: the system's, or one of the languages the app is
			// written in, each named in itself.
			langTags := []string{"", "en", "zh-Hans"}
			langLabels := []string{
				locales.LanguageSystem(),
				i18n.Autonym("en"),
				i18n.Autonym("zh-Hans"),
			}
			langNow := 0
			for i, tag := range langTags {
				if tag == normalizeLanguage(a.prefs.Language) {
					langNow = i
				}
			}
			lidx := langNow
			ui.Field(c, locales.SettingsLanguage(), func() {
				segmentedKey(c, pal, "lang:", &lidx, langLabels)
			})
			if lidx != langNow && lidx >= 0 && lidx < len(langTags) {
				a.setLanguage(langTags[lidx])
			}
		})
	})

	// The two layers a person can steer: how opaque the card is, and how
	// strong the material behind it is. Both repaint the window as they
	// change, with no Done to press.
	settingsGroup(c, pal, func() {
		ui.Form(c, func() {
			opacity := a.cardOpacity()
			ui.Field(c, locales.SettingsCardOpacity(), func() {
				ui.Row(c).Gap(10).Children(func() {
					ui.Slider(c.Key("settings-opacity"), &opacity, uitokens.OpacityMin, uitokens.OpacityMax).
						Step(0.05).Grow(1).MinWidth(0).Label(locales.SettingsCardOpacity())
					ui.Text(c, fmt.Sprintf("%.0f%%", opacity*100)).Width(38).Shrink(0).
						FontSize(11).Font("monospace").TextColor(pal.TextMuted).TextAlign(ui.End)
				})
			})
			if opacity != a.cardOpacity() {
				a.prefs.CardOpacity = opacity
				a.savePrefs()
				if a.win != nil {
					a.win.Invalidate()
				}
			}

			blurLabels := make([]string, len(blurs))
			blurNow := 0
			for i, b := range blurs {
				blurLabels[i] = b.label()
				if b.id == a.blurInUse() {
					blurNow = i
				}
			}
			bidx := blurNow
			ui.Field(c, locales.SettingsMaterial(), func() {
				segmented(c, pal, &bidx, blurLabels)
			})
			if bidx != blurNow && bidx >= 0 && bidx < len(blurs) {
				a.setBlur(blurs[bidx].id)
			}
		})
	})

	// One background: the window's material is the system's own, not a
	// palette of washes. The row shows how it reads in this appearance.
	stops := uitokens.Acrylic.Light
	if pal.Dark {
		stops = uitokens.Acrylic.Dark
	}
	bg := ui.Row(c).Height(50).Padding(0, 10).Gap(10).Radius(10).Background(pal.Control).
		Border(1, pal.Edge)
	bg.Children(func() {
		chip := ui.Column(c).Width(54).Height(32).Shrink(0).Radius(7).Clip().Border(1, pal.Edge)
		chip.Gradient(ui.Hex(stops[0]), ui.Hex(stops[len(stops)-1]), 135)
		chip.Children(func() {
			ui.Box(c).Width(24).Height(14).Radius(4).Margin(9, 0, 0, 15).
				Background(pal.Card.Alpha(0.90)).Shadow(0, 1, 2, 0, pal.CardShadow)
		})
		ui.Column(c).Gap(1).Justify(ui.Center).Children(func() {
			ui.Text(c, locales.SettingsAcrylicTitle()).FontSize(11.5).FontWeight(600).TextColor(pal.Text)
			ui.Text(c, locales.SettingsAcrylicHint()).FontSize(10).TextColor(pal.TextFaint)
		})
	})
}

func (a *app) settingsCapturePage(c *ui.Context, pal *uitokens.Palette) {
	before := a.prefs
	settingsGroup(c, pal, func() {
		ui.Form(c, func() {
			ui.Field(c, locales.SettingsProxyPort(), func() {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Spacer(c)
					hint, hintColor := locales.CommonEnterToApply(), pal.TextFaint
					if !validPort(a.proxyPort) {
						hint, hintColor = locales.ErrPort(), pal.Danger
					}
					ui.Text(c, hint).FontSize(10).TextColor(hintColor)
					in := ui.TextInput(c.Key("settings-port"), &a.proxyPort).Width(86).Height(26).
						FontSize(11.5).Font("monospace").Padding(0, 8).Background(pal.CodeBg).
						Border(1, pal.Edge)
					if !validPort(a.proxyPort) {
						in.Border(1, pal.Danger)
					}
					if in.Submitted() || in.Changed() {
						a.setProxyPort(in.Submitted())
					}
				})
			})

			timeoutSec, timeoutErr := strconv.Atoi(strings.TrimSpace(a.timeoutText))
			timeoutOK := timeoutErr == nil && timeoutSec >= 1 && timeoutSec <= 600
			ui.Field(c, locales.SettingsTimeout(), func() {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Spacer(c)
					ui.Text(c, locales.SettingsSeconds()).FontSize(10).TextColor(pal.TextFaint)
					in := ui.TextInput(c.Key("settings-timeout"), &a.timeoutText).Width(64).Height(26).
						FontSize(11.5).Font("monospace").Padding(0, 8).Background(pal.CodeBg).
						Border(1, pal.Edge).Label(locales.SettingsTimeoutLabel())
					if !timeoutOK {
						in.Border(1, pal.Danger)
					}
					if in.Changed() || in.Submitted() {
						if timeoutOK && a.prefs.TimeoutSec != timeoutSec {
							a.prefs.TimeoutSec = timeoutSec
							a.savePrefs()
						}
					}
				})
			})
		})
	})

	settingsGroup(c, pal, func() {
		ui.Form(c, func() {
			a.settingsSwitchField(c, pal, locales.SettingsStopRedirects(),
				&a.prefs.StopAtRedirects, "switch:redirects")
			a.settingsSwitchField(c, pal, locales.SettingsSkipTLS(),
				&a.prefs.SkipTLSVerify, "switch:tls")
			a.settingsSwitchField(c, pal, locales.SettingsAutostart(),
				&a.prefs.ProxyAutostart, "switch:autostart")
			a.settingsSwitchField(c, pal, locales.SettingsDecrypt(), &a.mitm, "switch:decrypt")
		})
	})
	if a.mitm != a.prefs.DecryptHTTPS {
		a.prefs.DecryptHTTPS = a.mitm
		if a.prx != nil {
			a.prx.SetMITM(a.mitm)
		}
	}
	if a.prefs != before {
		a.savePrefs()
	}

	caRow := ui.Row(c).Height(32).Gap(8).Padding(0, 2)
	caRow.Children(func() {
		ui.Icon(c, iconShield).Size(13, 13).TextColor(pal.TextMuted)
		ui.Text(c, locales.CaTitle()).FontSize(11.5).TextColor(pal.TextMuted).Grow(1)
		if ui.Button(c.Key("settings-ca"), locales.SettingsManage()).Clicked() {
			a.caOpen = true
		}
	})
}

// settingsMocksPage lists the Map Local rules a captured response made. Each
// one can be switched off without deleting its file, revealed in the file
// manager, or removed with the file.
func (a *app) settingsMocksPage(c *ui.Context, pal *uitokens.Palette) {
	if len(a.mocks) == 0 {
		settingsGroup(c, pal, func() {
			ui.Column(c).Gap(4).Children(func() {
				ui.Text(c, locales.SettingsMocksEmpty()).FontSize(11.5).TextColor(pal.TextMuted)
				ui.Text(c, locales.SettingsMocksHint()).FontSize(10.5).TextColor(pal.TextFaint)
			})
		})
		return
	}

	var toggleID, revealID, deleteID string
	var toggleOn bool
	settingsGroup(c, pal, func() {
		for i := range a.mocks {
			rule := a.mocks[i]
			if i > 0 {
				ui.Box(c).Height(1).Background(pal.Divider)
			}
			row := ui.Row(c).Height(44).Gap(8).AlignItems(ui.Center)
			row.Children(func() {
				ui.Column(c).Grow(1).MinWidth(0).Gap(1).Justify(ui.Center).Children(func() {
					ui.Text(c, rule.Name).FontSize(11.5).FontWeight(600).TextColor(pal.Text).
						SingleLine().Ellipsis("…")
					ui.Text(c, fmt.Sprintf("%d · %s", rule.Status, shortenPath(rule.File, 52))).
						FontSize(9.5).Font("monospace").TextColor(pal.TextFaint).
						SingleLine().Ellipsis("…")
				})
				enabled := rule.Enabled
				ui.Switch(c.Key("mock:"+rule.ID), &enabled)
				if enabled != rule.Enabled {
					toggleID, toggleOn = rule.ID, enabled
				}
				if b := iconButton(c, pal, iconFolder, locales.SettingsMockReveal(), 24); b.Clicked() {
					revealID = rule.ID
				}
				if b := iconButton(c, pal, iconTrash, locales.SettingsMockDelete(), 24); b.Clicked() {
					deleteID = rule.ID
				}
			})
		}
	})

	if toggleID != "" {
		a.setMockEnabled(toggleID, toggleOn)
	}
	if revealID != "" {
		for _, rule := range a.mocks {
			if rule.ID == revealID {
				a.revealMock(rule)
				break
			}
		}
	}
	if deleteID != "" {
		a.deleteMock(deleteID)
	}

	ui.Row(c).Gap(8).Children(func() {
		ui.Text(c, locales.SettingsMocksHint()).FontSize(10.5).TextColor(pal.TextFaint).Grow(1)
		if ui.Button(c.Key("mocks-dir"), locales.SettingsMocksFolder()).Clicked() {
			a.revealMocksDir()
		}
	})
}

// settingsSwitchField is a switch with its label down the form's label column:
// the whole row toggles, as a switch's label does across the platform.
func (a *app) settingsSwitchField(c *ui.Context, pal *uitokens.Palette, label string, value *bool, key string) {
	ui.Field(c, label, func() {
		ui.Row(c).Gap(8).Children(func() {
			ui.Spacer(c)
			// The field's label names the switch for assistive technology; a
			// label of its own would name the whole row instead, and the row's
			// middle is not where a switch is.
			ui.Switch(c.Key(key), value)
		})
	})
}

func (a *app) settingsDataPage(c *ui.Context, pal *uitokens.Palette) {
	before := a.prefs
	settingsGroup(c, pal, func() {
		ui.Form(c, func() {
			a.settingsSwitchField(c, pal, locales.SettingsKeepHistory(),
				&a.prefs.KeepHistory, "switch:history")
		})
	})
	if a.prefs != before {
		a.savePrefs()
	}

	dataRow := ui.Row(c).Height(40).Gap(8)
	dataRow.Children(func() {
		ui.Icon(c, iconFolder).Size(13, 13).TextColor(pal.TextMuted)
		ui.Column(c).Gap(0).Grow(1).MinWidth(0).Justify(ui.Center).Children(func() {
			ui.Text(c, locales.SettingsDataHint()).FontSize(11.5).TextColor(pal.TextMuted)
			ui.Text(c, shortenPath(a.dataDir(), 56)).FontSize(10).Font("monospace").
				TextColor(pal.TextFaint).SingleLine().Ellipsis("…")
		})
		if ui.Button(c.Key("settings-data"), locales.SettingsShowInFinder()).Clicked() {
			a.revealData()
		}
	})
}

func (a *app) settingsUpdatesPage(c *ui.Context, pal *uitokens.Palette) {
	prefs := loadUpdatePrefs()
	settingsGroup(c, pal, func() {
		ui.Form(c, func() {
			ui.Field(c, locales.UpdateVersion(), func() {
				ui.Text(c, appVersion()).Font("monospace").FontSize(11.5).
					TextColor(pal.TextMuted).Selectable()
			})
			ui.Field(c, locales.UpdateLastCheck(), func() {
				ui.Text(c, lastCheckLabel(prefs.lastCheck)).Font("monospace").FontSize(11.5).
					TextColor(pal.TextMuted).Selectable()
			})
			if !prefs.available {
				ui.Field(c, locales.UpdateAutoCheck(), func() {
					ui.Text(c, locales.UpdateUnavailable()).FontSize(11.5).TextColor(pal.TextMuted)
				})
				ui.Field(c, locales.UpdateAutoDownload(), func() {
					ui.Text(c, locales.UpdateUnavailable()).FontSize(11.5).TextColor(pal.TextMuted)
				})
				return
			}
			// Each switch is drawn from the plugin's state and pushed back
			// when clicked, so the page reads what is on disk every frame and
			// keeps no second copy of it.
			checks := prefs.automaticChecks
			ui.Field(c, locales.UpdateAutoCheck(), func() {
				ui.Row(c).Gap(8).Children(func() {
					ui.Spacer(c)
					ui.Switch(c.Key("switch:update-checks"), &checks)
				})
			})
			if checks != prefs.automaticChecks {
				saveUpdatePrefs(checks, prefs.automaticDownloads)
			}
			downloads := prefs.automaticDownloads
			ui.Field(c, locales.UpdateAutoDownload(), func() {
				ui.Row(c).Gap(8).Children(func() {
					ui.Spacer(c)
					ui.Switch(c.Key("switch:update-downloads"), &downloads)
				})
			})
			if downloads != prefs.automaticDownloads {
				saveUpdatePrefs(prefs.automaticChecks, downloads)
			}
		})
	})

	if !prefs.available {
		settingsNote(c, pal, locales.UpdateUnavailableNote())
		return
	}
	check := ui.Row(c).Gap(8)
	check.Children(func() {
		ui.Spacer(c)
		if ui.Button(c.Key("update-check"), locales.UpdateCheckNow()).Clicked() {
			checkForUpdatesNow()
		}
	})
	settingsNote(c, pal, locales.UpdateDailyNote())
	settingsNote(c, pal, locales.UpdateFoundNote())
}

func (a *app) settingsAboutPage(c *ui.Context, pal *uitokens.Palette) {
	ui.Column(c).Gap(6).Children(func() {
		ui.Text(c, "OmniFlow").FontSize(19).FontWeight(700).TextColor(pal.Text)
		ui.Text(c, appVersion()).FontSize(11.5).Font("monospace").TextColor(pal.TextMuted)
		ui.Text(c, locales.SettingsAboutLine(appVersion())).FontSize(11.5).
			TextColor(pal.TextFaint).LineHeight(1.5)
	})
}

// caDialog explains the one certificate OmniFlow asks a person to trust,
// and offers the file to install.
func (a *app) caDialog(c *ui.Context, pal *uitokens.Palette) {
	// Ask whether this Mac trusts the certificate when the dialog opens,
	// and only then: the answer changes only when a person installs it.
	if a.caOpen && !a.caWasOpen {
		a.checkTrust()
	}
	a.caWasOpen = a.caOpen
	ui.DialogBase(c, &a.caOpen, func(backdrop, panel ui.Element) {
		backdrop.Background(pal.Overlay)
		panel.Width(540).Padding(22).Radius(uitokens.CardRadius).Background(pal.Popover).
			Border(1, pal.EdgeHigh).Shadow(0, 30, 80, 0, pal.FloatShadow).
			InsetShadow(0, 1, 0, 0, ui.RGBA(255, 255, 255, 0.08))

		head := ui.Row(c).Height(34).Gap(10)
		head.Children(func() {
			ui.Box(c).Size(34, 34).Radius(9).Center().Background(pal.AccentSoft).Children(func() {
				ui.Icon(c, iconShield).Size(18, 18).TextColor(pal.Accent)
			})
			ui.Column(c).Gap(0).Grow(1).Justify(ui.Center).Children(func() {
				ui.Text(c, locales.CaTitle()).FontSize(15.5).FontWeight(700).TextColor(pal.Text)
				ui.Text(c, locales.CaSubtitle()).FontSize(10.5).
					TextColor(pal.TextFaint)
			})
			if b := iconButton(c, pal, iconX, locales.CaClose(), 26); b.Clicked() {
				a.caOpen = false
			}
		})
		ui.Box(c).Height(14)
		ui.Text(c, locales.CaIntro()).
			FontSize(11.5).TextColor(pal.TextMuted).LineHeight(1.5)
		ui.Box(c).Height(12)

		steps := []struct{ n, title, body string }{
			{"1", locales.CaStep1Title(), locales.CaStep1Body()},
			{"2", locales.CaStep2Title(), locales.CaStep2Body()},
			{"3", locales.CaStep3Title(), locales.CaStep3Body(a.proxyPort)},
			{"4", locales.CaStep4Title(), locales.CaStep4Body()},
		}
		for _, s := range steps {
			step := s
			row := ui.Row(c).AlignItems(ui.Start).Gap(9).Padding(0, 0, 6, 0)
			row.Children(func() {
				ui.Box(c).Size(18, 18).Shrink(0).Radius(9).Center().Background(pal.Chip).
					Children(func() {
						ui.Text(c, step.n).FontSize(9).FontWeight(750).TextColor(pal.TextMuted)
					})
				ui.Column(c).Gap(1).Grow(1).Children(func() {
					ui.Text(c, step.title).FontSize(11.5).FontWeight(650).TextColor(pal.Text)
					ui.Text(c, step.body).FontSize(10.5).TextColor(pal.TextFaint).LineHeight(1.4)
				})
			})
		}
		ui.Box(c).Height(6)
		path := locales.CommonUnavailable()
		if a.ca != nil {
			path = a.ca.CertPath()
		}
		pathRow := ui.Row(c).Height(34).Gap(8).Padding(0, 10).Radius(8).Background(pal.CodeBg).
			Border(1, pal.CodeEdge)
		pathRow.Children(func() {
			ui.Icon(c, iconKey).Size(12, 12).TextColor(pal.TextFaint)
			ui.Text(c, shortenPath(path, 64)).Font("monospace").FontSize(10).
				TextColor(pal.TextMuted).Grow(1).MinWidth(0).SingleLine().Ellipsis("…")
			if b := iconButton(c, pal, iconCopy, locales.CaCopyPath(), 24); b.Clicked() {
				a.copyText(c, locales.CaPathCopied(), path)
			}
		})
		ui.Box(c).Height(8)
		trustRow := ui.Row(c).Height(24).Gap(8)
		trustRow.Children(func() {
			col, text := pal.TextFaint, locales.CaTrustUnknown()
			switch a.caTrust {
			case trustChecking:
				col, text = pal.TextMuted, locales.CaTrustChecking()
			case trustYes:
				col, text = pal.Success, locales.CaTrustYes()
			case trustNo:
				col, text = pal.Warning, locales.CaTrustNo()
			}
			statusDot(c, col, 6)
			ui.Text(c, text).FontSize(10.5).TextColor(col).Grow(1).SingleLine().Ellipsis("…")
			if a.caTrust != trustChecking {
				if ui.Button(c.Key("ca-recheck"), locales.CaCheckAgain()).Clicked() {
					a.checkTrust()
				}
			}
		})
		ui.Box(c).Height(10)
		ui.Row(c).Height(32).Gap(8).Justify(ui.End).Children(func() {
			if ui.Button(c.Key("ca-reveal"), locales.CaReveal()).Clicked() {
				a.revealCA()
			}
			if ui.Button(c.Key("ca-cancel"), locales.CommonDone()).Clicked() {
				a.caOpen = false
			}
			if ui.PrimaryButton(c.Key("ca-save"), locales.CaSave()).Clicked() {
				a.exportCA()
			}
		})
	})
}

func shortenPath(p string, n int) string {
	if len(p) <= n {
		return p
	}
	return "…" + p[len(p)-n+1:]
}
