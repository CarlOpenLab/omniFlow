package main

import (
	"github.com/egoist/mygo/ui"

	"github.com/CarlOpenLab/omniflow/internal/uitokens"
	"github.com/CarlOpenLab/omniflow/locales"
)

// view builds the window: the sidebar over the material, the floating card
// that holds the workbench, the quiet line along its foot, and the dialogs.
func (a *app) view(c *ui.Context) {
	applyTheme(c, a.prefs.Theme)
	pal := paletteFor(c)
	vibrant := c.Vibrancy()
	a.shortcuts(c)
	// A curl command pasted into the URL field opens as a request before the
	// window is built around it, rather than in the middle of the paste.
	if cmd := a.pendingCurl; cmd != "" {
		a.pendingCurl = ""
		a.importCurl(cmd)
	}

	row := ui.Row(c).Fill().AlignItems(ui.Stretch)
	a.backdrop(c, pal, row, vibrant)
	row.Children(func() {
		a.sidebar(c, pal)
		main := ui.Column(c).Grow(1).MinWidth(0).
			Padding(uitokens.CardGap, uitokens.CardGap, uitokens.CardGap, uitokens.CardGap).
			DragWindow()
		main.Children(func() {
			a.card(c, pal)
		})
		a.settingsDialog(c, pal)
		a.caDialog(c, pal)
		a.envDialog(c, pal)
	})
}

// backdrop paints the window's ground: a veil of the palette's ground over
// the system's material, so the sidebar's text keeps its contrast while the
// frost reads through. Without a material, the ground is painted whole as
// the acrylic fallback gradient.
func (a *app) backdrop(c *ui.Context, pal *uitokens.Palette, row ui.Element, vibrant bool) {
	if vibrant {
		c.Root().Background(pal.GroundColor(true))
		return
	}
	c.Root().LinearGradient(uitokens.Acrylic.Base(pal.Dark))
	row.LinearGradient(uitokens.Acrylic.Glow(pal.Dark))
}

// card is the workspace: the one layer whose opacity the person chooses. Over
// the material it is translucent, and everything on it is a wash, so the
// frost comes through everywhere but its text.
func (a *app) card(c *ui.Context, pal *uitokens.Palette) {
	card := ui.Column(c).Fill().MinWidth(0).MinHeight(0).
		Radius(cardRadius()).Clip().
		Background(pal.CardFill(a.cardOpacity()))
	raised(card, pal)
	card.Children(func() {
		a.cardHeader(c, pal)
		ui.Box(c).Grow(1).MinHeight(0).Children(func() {
			switch a.mode {
			case modeInspector:
				a.inspector(c, pal)
			default:
				a.composer(c, pal)
			}
		})
		a.footer(c, pal)
	})
}

// modeSwitcher is the window's own segmented controller: two views of the
// same traffic, one being drafted and one being watched.
func (a *app) modeSwitcher(c *ui.Context, pal *uitokens.Palette) {
	track := ui.Row(c).Shrink(0).Padding(2).Radius(9).Gap(1).Background(pal.Chip)
	track.Children(func() {
		items := []struct {
			mode     viewMode
			id       string
			icon     *ui.SVG
			text     string
			shortcut string
		}{
			{modeComposer, "composer", iconCompose, locales.ComposerMode(), "⌘1"},
			{modeInspector, "inspector", iconInspect, locales.InspectorMode(), "⌘2"},
		}
		for _, it := range items {
			item := it
			selected := a.mode == item.mode
			b := ui.ButtonBase(c.Key("mode:"+item.id)).Height(26).Padding(0, 11).Radius(7).Gap(6).
				Center().Label(item.text).Tooltip(item.text + " " + item.shortcut)
			col := pal.TextMuted
			if selected {
				col = pal.Text
				b.Background(pal.WashSelected).Border(1, pal.EdgeStrong)
			}
			if b.Hovered() && !selected {
				col = pal.Text
			}
			b.Children(func() {
				ui.Icon(c, item.icon).Size(12, 12).TextColor(col)
				ui.Text(c, item.text).FontSize(11.5).FontWeight(600).TextColor(col)
			})
			if b.Clicked() {
				a.mode = item.mode
			}
		}
	})
}

func (a *app) cardHeader(c *ui.Context, pal *uitokens.Palette) {
	row := ui.Row(c).Height(uitokens.HeaderH).Shrink(0).Padding(0, 12, 0, 16).Gap(10).
		BorderWidth(0, 0, 1, 0).BorderColor(pal.Divider).DragWindow()
	row.Children(func() {
		if a.mode == modeInspector {
			a.inspectorTitle(c, pal)
		} else {
			a.composerTitle(c, pal)
		}
		ui.Spacer(c)
		a.modeSwitcher(c, pal)
		ui.Box(c).Width(1).Height(20).Background(pal.Divider).Margin(4, 6)
		if a.mode == modeComposer {
			a.saveButton(c, pal)
			a.requestMenu(c, pal)
			if b := iconButton(c, pal, iconPlus, locales.ComposerNewRequest(), 28); b.Clicked() {
				a.newRequest()
			}
		}
		if b := iconButton(c, pal, iconGear, locales.CommonSettings(), 28); b.Clicked() {
			a.settingsOpen = true
		}
	})
}

// composerTitle names the request on screen, renamed in place by a double
// click, and says whether the collection has it as it is.
func (a *app) composerTitle(c *ui.Context, pal *uitokens.Palette) {
	ui.Column(c).Gap(1).Justify(ui.Center).MinWidth(0).Shrink(1).Children(func() {
		name := a.req.DisplayName()
		title := ui.EditableText(c.Key("request-name"), &name).FontSize(13.5).FontWeight(700).
			TextColor(pal.Text).MaxWidth(420).Label(locales.ComposerRequestName()).
			Tooltip(locales.ComposerRenameHint())
		if title.Changed() {
			a.renameRequest(name)
		}
		ui.Row(c).Gap(5).MinWidth(0).Children(func() {
			ui.Text(c, a.req.Method+" "+shortHost(a.req.URL)).FontSize(10.5).TextColor(pal.TextMuted).
				SingleLine().Ellipsis("…").MinWidth(0)
			switch {
			case a.savedSel == "":
				ui.Text(c, locales.ComposerNotSaved()).FontSize(10.5).TextColor(pal.TextFaint).Shrink(0)
			case a.edited():
				ui.Text(c, locales.ComposerEdited()).FontSize(10.5).FontWeight(600).TextColor(pal.Warning).Shrink(0)
			default:
				ui.Text(c, locales.ComposerSaved()).FontSize(10.5).TextColor(pal.TextFaint).Shrink(0)
			}
		})
	})
}

func (a *app) inspectorTitle(c *ui.Context, pal *uitokens.Palette) {
	subtitle := ""
	switch {
	case a.proxyRunning():
		subtitle = locales.InspectorRecording("127.0.0.1:"+a.proxyPort, len(a.flows))
	case a.proxyErr != "":
		subtitle = locales.InspectorStopped(a.proxyErr)
	case len(a.flows) > 0:
		subtitle = locales.InspectorPausedCount(len(a.flows))
	default:
		subtitle = locales.InspectorPausedEmpty()
	}
	ui.Column(c).Gap(1).Justify(ui.Center).MinWidth(0).Shrink(1).Children(func() {
		ui.Text(c, locales.InspectorTitle()).FontSize(13.5).FontWeight(700).TextColor(pal.Text).SingleLine()
		ui.Text(c, subtitle).FontSize(10.5).TextColor(pal.TextMuted).SingleLine().Ellipsis("…")
	})
}

// saveButton saves the request, and carries a dot while the composer holds
// what the collection does not.
func (a *app) saveButton(c *ui.Context, pal *uitokens.Palette) {
	edited := a.edited()
	tip := locales.SaveTooltipNew()
	if edited && a.savedSel != "" {
		tip = locales.SaveTooltipChanges()
	}
	b := ui.ButtonBase(c.Key("save")).Height(28).Padding(0, 9).Radius(7).Gap(6).
		Label(locales.CommonSave()).Tooltip(tip)
	if b.Hovered() {
		b.Background(pal.RowHover)
	}
	col := pal.TextMuted
	if edited {
		col = pal.Text
	}
	b.Children(func() {
		ui.Box(c).Size(13, 13).Shrink(0).Children(func() {
			ui.Icon(c, iconSave).Size(13, 13).TextColor(col)
			if edited {
				ui.Box(c).Absolute().Top(-4).Right(-4).Size(7, 7).Radius(4).Background(pal.Accent)
			}
		})
		ui.Text(c, locales.CommonSave()).FontSize(12).FontWeight(550).TextColor(col)
	})
	if b.Clicked() {
		a.saveRequest()
	}
}

// requestMenu holds what is done to the request as a whole, beside Save.
func (a *app) requestMenu(c *ui.Context, pal *uitokens.Palette) {
	saved := a.savedSel
	var act func()
	iconButton(c, pal, iconMore, locales.CommonMore(), 28).Menu(func(m *ui.Menu) {
		if m.Item(locales.CommonCopyCurl()).Shortcut(ui.Cmd|ui.Shift, ui.KeyC).Chosen() {
			act = func() { a.copyCurl(c) }
		}
		if m.Item(locales.CommonCopyURL()).Disabled(a.req.URL == "").Chosen() {
			act = func() { a.copyText(c, locales.CommonURLCopied(), a.req.URL) }
		}
		m.Separator()
		if m.Item(locales.CommonDuplicate()).Disabled(saved == "").Chosen() {
			act = func() { a.duplicateRequest(saved) }
		}
		if m.Item(locales.CommonDelete()).Disabled(saved == "").Chosen() {
			act = func() { a.deleteRequest(saved) }
		}
	})
	if act != nil {
		act()
	}
}

func (a *app) footer(c *ui.Context, pal *uitokens.Palette) {
	row := ui.Row(c).Height(30).Shrink(0).Padding(0, 12).Gap(10).
		BorderWidth(1, 0, 0, 0).BorderColor(pal.Divider)
	row.Children(func() {
		ui.Row(c).Gap(8).Grow(1).MinWidth(0).Children(func() {
			if notice := a.currentNotice(c); notice != "" {
				statusDot(c, pal.Accent, 6)
				ui.Text(c, notice).FontSize(10.5).TextColor(pal.TextMuted).SingleLine().Grow(1).
					MinWidth(0).Ellipsis("…")
				if a.undo != nil {
					undo := ui.ButtonBase(c.Key("undo")).Height(20).Shrink(0).Padding(0, 7).
						Radius(5).Label(locales.CommonUndo()).Tooltip(locales.CommonUndoTooltip())
					if undo.Hovered() {
						undo.Background(pal.RowHover)
					}
					undo.Children(func() {
						ui.Text(c, locales.CommonUndo()).FontSize(10.5).FontWeight(600).TextColor(pal.Accent)
					})
					if undo.Clicked() {
						a.runUndo()
					}
				}
				return
			}
			col, text := pal.TextFaint, locales.FooterProxyPaused()
			if a.proxyRunning() {
				col = pal.Success
				text = locales.FooterProxyRunning(a.proxyPort, len(a.flows))
			}
			statusDot(c, col, 6)
			ui.Text(c, text).FontSize(10.5).TextColor(pal.TextFaint).SingleLine().Grow(1).MinWidth(0).
				Ellipsis("…")
		})
		if a.mode == modeComposer {
			if a.sending {
				shortcutHint(c, pal, "⌘.", locales.CommonCancel())
			} else {
				shortcutHint(c, pal, "⌘↩", locales.ComposerSend())
			}
			shortcutHint(c, pal, "⌘S", locales.CommonSave())
			shortcutHint(c, pal, "⌘2", locales.InspectorMode())
		} else {
			shortcutHint(c, pal, "↑↓", locales.FooterMove())
			shortcutHint(c, pal, "⌘F", locales.FooterFilter())
			shortcutHint(c, pal, "⌘1", locales.ComposerMode())
		}
	})
}

// shortcutHint is a key and what it does, for the foot of the card.
func shortcutHint(c *ui.Context, pal *uitokens.Palette, keys, label string) {
	ui.Row(c).Gap(5).Shrink(0).Children(func() {
		kbd(c, pal, keys)
		ui.Text(c, label).FontSize(10).TextColor(pal.TextFaint)
	})
}

// shortHost is the URL's host, for the header's second line.
func shortHost(raw string) string {
	s := raw
	for _, prefix := range []string{"https://", "http://"} {
		if len(s) >= len(prefix) && s[:len(prefix)] == prefix {
			s = s[len(prefix):]
			break
		}
	}
	if s == "" {
		return locales.ComposerNoURL()
	}
	for i := 0; i < len(s); i++ {
		if s[i] == '/' || s[i] == '?' {
			return s[:i]
		}
	}
	return s
}
