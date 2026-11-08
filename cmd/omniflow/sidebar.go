package main

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/CarlOpenLab/omniflow/internal/httpx"
	"github.com/CarlOpenLab/omniflow/internal/model"
	"github.com/CarlOpenLab/omniflow/internal/uitokens"
	"github.com/CarlOpenLab/omniflow/locales"
)

// sidebar is the column beside the card: the collection or the captured
// domains for the mode on screen, and the quiet foot where settings live. It
// sits directly on the wash: no edge line, and its title row is only a drag
// surface.
func (a *app) sidebar(c *ui.Context, pal *uitokens.Palette) {
	col := ui.Column(c).Width(uitokens.SidebarWidth).Shrink(0).MinHeight(0)
	col.Children(func() {
		// The title bar's left end belongs to the traffic lights, not a brand.
		ui.Box(c).Height(uitokens.TopBar).Shrink(0).DragWindow()
		ui.Scroll(c).Grow(1).MinHeight(0).Children(func() {
			body := ui.Column(c).FillWidth().Padding(8, 8, 8, 8).Gap(2)
			body.Children(func() {
				if a.mode == modeInspector {
					a.inspectorSidebar(c, pal)
				} else {
					a.composerSidebar(c, pal)
				}
			})
		})
		a.sidebarFooter(c, pal)
	})
}

func (a *app) composerSidebar(c *ui.Context, pal *uitokens.Palette) {
	// A row's menu can take the row out of the list being built; what it
	// chose runs once the list is done.
	var act func()

	// Environment: it changes what every request sends, so it sits first.
	envHead := ui.Row(c).Height(30).Padding(4, 0, 0, 0)
	envHead.Children(func() {
		sectionTitle(c, pal, locales.SidebarEnvironment(), 0)
		if b := iconButton(c, pal, iconLayers, locales.SidebarManageEnvs(), 24); b.Clicked() {
			a.openEnvs()
		}
	})
	a.envPicker(c, pal)
	ui.Box(c).Height(10)

	// Collections
	header := ui.Row(c).Height(30).Padding(4, 0, 0, 0)
	header.Children(func() {
		sectionTitle(c, pal, locales.SidebarCollections(), len(a.requests))
		if b := iconButton(c, pal, iconPlus, locales.ComposerNewRequest(), 24); b.Clicked() {
			a.newRequest()
		}
	})
	for _, r := range a.requests {
		if f := a.requestRow(c, pal, r); f != nil {
			act = f
		}
	}
	if len(a.requests) == 0 {
		ui.Row(c).Height(30).Padding(4, 10).Children(func() {
			ui.Text(c, locales.SidebarNoRequests()).FontSize(11.5).TextColor(pal.TextFaint)
		})
	}

	ui.Box(c).Height(10)
	hist := ui.Row(c).Height(30).Padding(4, 0, 0, 0)
	hist.Children(func() {
		sectionTitle(c, pal, locales.SidebarHistory(), len(a.history))
		if len(a.history) > 0 {
			if b := iconButton(c, pal, iconTrash, locales.SidebarClearHistory(), 24); b.Clicked() {
				a.clearHistory()
			}
		}
	})
	limit := min(len(a.history), 30)
	for _, h := range a.history[:limit] {
		if f := a.historyRow(c, pal, h); f != nil {
			act = f
		}
	}
	if len(a.history) == 0 {
		ui.Row(c).Height(30).Padding(4, 10).Children(func() {
			text := locales.SidebarHistoryEmpty()
			if !a.prefs.KeepHistory {
				text = locales.SidebarHistoryOff()
			}
			ui.Text(c, text).FontSize(11.5).TextColor(pal.TextFaint)
		})
	}
	if act != nil {
		act()
	}
}

// requestRow is a saved request: its verb and name, a dot while it has
// edits not saved, and Delete under the pointer. It returns what its menu
// chose, for after the list.
func (a *app) requestRow(c *ui.Context, pal *uitokens.Palette, r model.Request) (act func()) {
	selected := a.savedSel == r.ID
	row := ui.ButtonBase(c.Key("req:"+r.ID)).Height(30).Padding(0, 8).Radius(7).Gap(7).
		Label(displayRequestName(r)).Tooltip(r.Method + " " + r.URL)
	if selected {
		row.Background(pal.AccentSoft)
	} else if row.Hovered() {
		row.Background(pal.RowHover)
	}
	hovered := row.Hovered()
	row.Children(func() {
		col := uitokens.MethodColor(pal, r.Method)
		ui.Text(c, r.Method).Width(40).Shrink(0).FontSize(9).FontWeight(750).
			LetterSpacing(0.2).TextColor(col)
		nameColor := pal.Text
		if selected {
			nameColor = pal.Accent
		}
		ui.Text(c, displayRequestName(r)).FontSize(11.5).FontWeight(550).TextColor(nameColor).
			SingleLine().Ellipsis("…").Grow(1).MinWidth(0)
		switch {
		case hovered:
			del := iconButton(c, pal, iconTrash, locales.SidebarDeleteRequest(displayRequestName(r)), 20)
			del.OnClick(func() { a.deleteRequest(r.ID) })
		case a.hasDraft(r.ID):
			ui.Box(c).Size(6, 6).Shrink(0).Radius(3).Background(pal.Warning).Margin(0, 7).
				Label(locales.SidebarEdited()).Tooltip(locales.SidebarEditedHint())
		}
	})
	row.ContextMenu(func(m *ui.Menu) {
		if m.Item(locales.CommonOpen()).Disabled(selected).Chosen() {
			act = func() { a.loadRequest(r) }
		}
		if m.Item(locales.CommonDuplicate()).Chosen() {
			act = func() { a.duplicateRequest(r.ID) }
		}
		m.Separator()
		if m.Item(locales.CommonCopyURL()).Disabled(r.URL == "").Chosen() {
			act = func() { a.copyText(c, locales.CommonURLCopied(), r.URL) }
		}
		if m.Item(locales.CommonCopyCurl()).Chosen() {
			act = func() { a.copySpecCurl(c, specOf(r)) }
		}
		m.Separator()
		if m.Item(locales.CommonDelete()).Chosen() {
			act = func() { a.deleteRequest(r.ID) }
		}
	})
	if row.Clicked() {
		a.loadRequest(r)
	}
	return act
}

// copySpecCurl copies a request as a curl command, or says why it cannot.
func (a *app) copySpecCurl(c *ui.Context, spec httpx.Spec) {
	cmd, err := httpx.Curl(spec)
	if err != nil {
		a.setNotice(err.Error())
		return
	}
	a.copyText(c, locales.ComposerCurlCopied(), cmd)
}

// historyRow is a request sent before: its verb, URL and the status it
// got. It returns what its menu chose, for after the list.
func (a *app) historyRow(c *ui.Context, pal *uitokens.Palette, h model.HistoryEntry) (act func()) {
	tip := h.Method + " " + h.URL
	if !h.At.IsZero() {
		tip += "\n" + locales.HistorySent(h.At)
		if h.Duration > 0 {
			tip += " · " + locales.HistoryDuration(int(h.Duration))
		}
	}
	row := ui.ButtonBase(c.Key("hist:"+h.ID)).Height(27).Padding(0, 8).Radius(7).Gap(7).
		Label(h.Method + " " + h.URL).Tooltip(tip)
	if row.Hovered() {
		row.Background(pal.RowHover)
	}
	hovered := row.Hovered()
	row.Children(func() {
		ui.Text(c, h.Method).Width(40).Shrink(0).FontSize(9).FontWeight(750).
			TextColor(uitokens.MethodColor(pal, h.Method))
		ui.Text(c, prettyURL(h.URL)).FontSize(11).TextColor(pal.TextMuted).
			SingleLine().Ellipsis("…").Grow(1).MinWidth(0)
		if h.Status > 0 {
			ui.Text(c, fmt.Sprint(h.Status)).FontSize(9.5).FontWeight(600).
				TextColor(uitokens.StatusColor(pal, h.Status))
		} else {
			ui.Text(c, locales.CommonErrorShort()).FontSize(9.5).FontWeight(600).TextColor(pal.Danger)
		}
		if hovered {
			del := iconButton(c, pal, iconTrash, locales.CommonRemoveHistory(), 20)
			del.OnClick(func() { a.removeHistory(h.ID) })
		}
	})
	row.ContextMenu(func(m *ui.Menu) {
		if m.Item(locales.CommonOpen()).Chosen() {
			act = func() { a.loadHistory(h) }
		}
		m.Separator()
		if m.Item(locales.CommonCopyURL()).Chosen() {
			act = func() { a.copyText(c, locales.CommonURLCopied(), h.URL) }
		}
		if m.Item(locales.CommonCopyCurl()).Disabled(h.Request == nil).Chosen() {
			act = func() { a.copySpecCurl(c, specOf(*h.Request)) }
		}
		m.Separator()
		if m.Item(locales.CommonRemoveHistory()).Chosen() {
			act = func() { a.removeHistory(h.ID) }
		}
	})
	if row.Clicked() {
		a.loadHistory(h)
	}
	return act
}

// prettyURL is a URL without its scheme, for the narrow rows of the sidebar.
func prettyURL(raw string) string {
	s := raw
	for _, prefix := range []string{"https://", "http://"} {
		if strings.HasPrefix(s, prefix) {
			s = s[len(prefix):]
			break
		}
	}
	if s == "" {
		return locales.SidebarEmptyURL()
	}
	return s
}

func (a *app) inspectorSidebar(c *ui.Context, pal *uitokens.Palette) {
	// Capture
	sectionTitle(c, pal, locales.SidebarCapture(), 0)
	panel := ui.Column(c).FillWidth().Radius(10).Background(pal.Chip).
		Padding(10, 10, 8, 10).Gap(8)
	panel.Children(func() {
		row := ui.Row(c).Height(26).Gap(8)
		row.Children(func() {
			col := pal.TextFaint
			label := locales.InspectorPaused()
			if a.proxyRunning() {
				col = pal.Success
				label = locales.InspectorRecordingStatus()
			}
			statusDot(c, col, 8)
			ui.Text(c, label).FontSize(12).FontWeight(650).TextColor(pal.Text).Grow(1)
			toggle := ui.ButtonBase(c.Key("capture-toggle")).Height(24).Padding(0, 8).Radius(6).Gap(5).
				Label(locales.SidebarProxyToggle()).Tooltip(locales.SidebarProxyToggleHint())
			if toggle.Hovered() {
				toggle.Background(pal.RowHover)
			}
			toggle.Children(func() {
				if a.proxyRunning() {
					ui.Icon(c, iconStop).Size(11, 11).TextColor(pal.Danger)
					ui.Text(c, locales.SidebarStop()).FontSize(11).FontWeight(600).TextColor(pal.Danger)
				} else {
					ui.Icon(c, iconPlay).Size(11, 11).TextColor(pal.Success)
					ui.Text(c, locales.SidebarStart()).FontSize(11).FontWeight(600).TextColor(pal.Success)
				}
			})
			if toggle.Clicked() {
				a.toggleProxy()
			}
		})
		port := ui.Row(c).Height(26).Gap(8)
		port.Children(func() {
			ui.Text(c, locales.CommonPort()).FontSize(11.5).TextColor(pal.TextMuted).Width(34)
			in := ui.TextInput(c.Key("proxy-port"), &a.proxyPort).Height(24).Width(68).
				FontSize(11.5).Font("monospace").Padding(2, 6).Radius(6).
				Background(pal.Chip).Border(1, pal.Edge).Tooltip(locales.CommonEnterToApply())
			if !validPort(a.proxyPort) {
				in.Border(1, pal.Danger)
			}
			if in.Submitted() || in.Changed() {
				a.setProxyPort(in.Submitted())
			}
			ui.Spacer(c)
			ui.Text(c, "127.0.0.1").FontSize(10.5).Font("monospace").TextColor(pal.TextFaint)
		})
		if !validPort(a.proxyPort) {
			ui.Text(c, locales.ErrPort()).FontSize(10).TextColor(pal.Danger)
		}
		decrypt := ui.Row(c).Height(24).Gap(8)
		decrypt.Children(func() {
			ui.Text(c, locales.SidebarDecryptHTTPS()).FontSize(11.5).TextColor(pal.TextMuted).Grow(1)
			ui.Switch(c.Key("mitm"), &a.mitm).Label(locales.SidebarDecryptHTTPS())
		})
		if a.mitm != a.prefs.DecryptHTTPS {
			a.prefs.DecryptHTTPS = a.mitm
			a.savePrefs()
			if a.prx != nil {
				a.prx.SetMITM(a.mitm)
			}
		}
		caRow := ui.Row(c).Height(24).Gap(6).Padding(0, 4).Margin(0, -4).Radius(6).
			Cursor(ui.CursorPointer).Label(locales.CaTitle())
		if caRow.Hovered() {
			caRow.Background(pal.RowHover)
		}
		caRow.Children(func() {
			ui.Icon(c, iconShield).Size(12, 12).TextColor(pal.TextMuted)
			ui.Text(c, locales.CaTitle()).FontSize(11.5).TextColor(pal.TextMuted).Grow(1)
			ui.Icon(c, iconRight).Size(11, 11).TextColor(pal.TextFaint)
		})
		if caRow.Clicked() {
			a.caOpen = true
		}
	})

	ui.Box(c).Height(10)
	// Domains
	domains := a.domains()
	header := ui.Row(c).Height(30).Padding(4, 0, 0, 0)
	header.Children(func() {
		sectionTitle(c, pal, locales.SidebarDomains(), len(domains))
		if a.domainFilter != "" {
			if b := iconButton(c, pal, iconX, locales.SidebarShowAllDomains(), 24); b.Clicked() {
				a.domainFilter = ""
			}
		}
	})
	allRow := ui.ButtonBase(c.Key("domain:all")).Height(28).Padding(0, 8).Radius(7).Gap(7).
		Label(locales.SidebarAllTraffic())
	if a.domainFilter == "" {
		allRow.Background(pal.AccentSoft)
	} else if allRow.Hovered() {
		allRow.Background(pal.RowHover)
	}
	allRow.Children(func() {
		ui.Icon(c, iconGlobe).Size(12, 12).TextColor(pal.TextMuted)
		ui.Text(c, locales.SidebarAllTraffic()).FontSize(11.5).FontWeight(550).TextColor(pal.Text).Grow(1)
		ui.Text(c, fmt.Sprint(len(a.flows))).FontSize(10).TextColor(pal.TextFaint)
	})
	if allRow.Clicked() {
		a.domainFilter = ""
	}
	for _, d := range domains {
		domain := d
		selected := strings.EqualFold(a.domainFilter, domain.Host)
		row := ui.ButtonBase(c.Key("domain:"+domain.Host)).Height(28).Padding(0, 8).Radius(7).Gap(7).
			Label(domain.Host).Tooltip(domain.Host)
		if selected {
			row.Background(pal.AccentSoft)
		} else if row.Hovered() {
			row.Background(pal.RowHover)
		}
		row.Children(func() {
			col := pal.TextFaint
			if selected {
				col = pal.Accent
			}
			ui.Icon(c, iconServer).Size(12, 12).TextColor(col)
			ui.Text(c, domain.Host).FontSize(11.5).TextColor(pal.Text).SingleLine().
				Ellipsis("…").Grow(1).MinWidth(0)
			ui.Text(c, fmt.Sprint(domain.Count)).FontSize(10).TextColor(pal.TextFaint)
		})
		if row.Clicked() {
			if selected {
				a.domainFilter = ""
			} else {
				a.domainFilter = domain.Host
			}
		}
	}
	if len(domains) == 0 {
		ui.Row(c).Height(30).Padding(4, 10).Children(func() {
			ui.Text(c, locales.SidebarNoHosts()).FontSize(11.5).TextColor(pal.TextFaint)
		})
	}

	ui.Box(c).Height(12)
	note := ui.Column(c).FillWidth().Radius(9).Background(pal.Chip).
		Padding(9, 10).Gap(3)
	note.Children(func() {
		ui.Text(c, locales.SidebarPointDevice()).FontSize(10.5).FontWeight(650).
			TextColor(pal.Text)
		ui.Text(c, locales.SidebarProxyHint("127.0.0.1:"+a.proxyPort)).
			FontSize(10).TextColor(pal.TextMuted).LineHeight(1.4)
	})
}

// themes are the appearances a person can choose, as prefs store them.
var themes = []struct {
	mode  string
	label func() string
}{
	{"system", locales.ThemeSystem},
	{"light", locales.ThemeLight},
	{"dark", locales.ThemeDark},
}

// themeMode is the stored appearance, with the first run's empty one read as
// the system's.
func themeMode(stored string) string {
	if stored == "" {
		return "system"
	}
	return stored
}

func (a *app) sidebarFooter(c *ui.Context, pal *uitokens.Palette) {
	// No hairline above the foot: the wash is the only separation the sidebar
	// gets.
	row := ui.Row(c).Height(uitokens.FootH).Shrink(0).Padding(0, 10).Gap(4)
	row.Children(func() {
		settings := ui.ButtonBase(c.Key("settings")).Height(26).Padding(0, 8).Radius(7).Gap(6).
			Label(locales.CommonSettings()).Tooltip(locales.CommonSettingsTooltip())
		if settings.Hovered() {
			settings.Background(pal.RowHover)
		}
		settings.Children(func() {
			ui.Icon(c, iconGear).Size(13, 13).TextColor(pal.TextMuted)
			ui.Text(c, locales.CommonSettings()).FontSize(11.5).FontWeight(550).TextColor(pal.TextMuted)
		})
		if settings.Clicked() {
			a.settingsOpen = true
		}
		ui.Spacer(c)
		a.appearanceMenu(c, pal)
	})
}

// appearanceMenu shows the appearance in use and switches it, without a
// trip to Settings.
func (a *app) appearanceMenu(c *ui.Context, pal *uitokens.Palette) {
	mode := themeMode(a.prefs.Theme)
	icon, label := iconMonitor, locales.ThemeSystem()
	switch mode {
	case "light":
		icon, label = iconSun, locales.ThemeLight()
	case "dark":
		icon, label = iconMoon, locales.ThemeDark()
	}
	chip := ui.ButtonBase(c.Key("appearance")).Height(24).Padding(0, 8).Radius(6).Gap(5).
		Label(locales.CommonAppearance()).Tooltip(locales.CommonAppearanceTooltip(label))
	if chip.Hovered() {
		chip.Background(pal.RowHover)
	}
	chip.Children(func() {
		ui.Icon(c, icon).Size(12, 12).TextColor(pal.TextMuted)
		ui.Text(c, label).FontSize(10.5).TextColor(pal.TextMuted)
		ui.Icon(c, iconChevron).Size(10, 10).TextColor(pal.TextFaint)
	})
	chip.Menu(func(m *ui.Menu) {
		for _, t := range themes {
			if m.Item(t.label()).Checked(mode == t.mode).Chosen() {
				a.setTheme(t.mode)
			}
		}
		m.Separator()
		if m.Item(locales.CommonSettingsMenu()).Shortcut(ui.Cmd, ui.KeyComma).Chosen() {
			a.settingsOpen = true
		}
	})
}
