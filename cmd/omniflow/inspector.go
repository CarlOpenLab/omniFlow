package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/CarlOpenLab/omniflow/internal/httpx"
	"github.com/CarlOpenLab/omniflow/internal/proxy"
	"github.com/CarlOpenLab/omniflow/internal/uitokens"
	"github.com/CarlOpenLab/omniflow/locales"
)

// inspector is the passive half: live traffic on top, the selected
// exchange below it, and the tools that start, filter and clear the capture.
func (a *app) inspector(c *ui.Context, pal *uitokens.Palette) {
	col := ui.Column(c).Fill().MinHeight(0)
	col.Children(func() {
		a.captureToolbar(c, pal)
		ui.SplitVerticalPercent(c, &a.inspectSplit, func() {
			a.flowListPane(c, pal)
		}, func() {
			a.flowDetailPane(c, pal)
		}).Grow(1).MinHeight(0)
	})
}

func (a *app) captureToolbar(c *ui.Context, pal *uitokens.Palette) {
	row := ui.Row(c).Height(42).Shrink(0).Padding(0, 12).Gap(8).
		BorderWidth(0, 0, 1, 0).BorderColor(pal.Divider)
	row.Children(func() {
		running := a.proxyRunning()
		rec := ui.ButtonBase(c.Key("capture")).Height(28).Shrink(0).Padding(0, 10).
			Radius(7).Gap(6).Label(locales.InspectorToggleCapture()).
			Tooltip(locales.InspectorToggleCaptureHint())
		if running {
			rec.Background(pal.Danger.Alpha(0.11))
		} else {
			rec.Background(pal.AccentSoft)
		}
		if rec.Hovered() {
			rec.Opacity(0.85)
		}
		rec.Children(func() {
			if running {
				statusDot(c, pal.Danger, 7)
				ui.Text(c, locales.InspectorRecordingStatus()).FontSize(11.5).FontWeight(650).TextColor(pal.Danger)
			} else {
				ui.Icon(c, iconPlay).Size(11, 11).TextColor(pal.Accent)
				ui.Text(c, locales.InspectorStartCapture()).FontSize(11.5).FontWeight(650).TextColor(pal.Accent)
			}
		})
		if rec.Clicked() {
			a.toggleProxy()
		}

		addr := "127.0.0.1:" + a.proxyPort
		addrChip := ui.ButtonBase(c.Key("addr")).Height(28).Shrink(0).Padding(0, 9).
			Radius(7).Gap(6).Label(locales.InspectorCopyAddr()).Tooltip(locales.InspectorCopyAddrHint(addr))
		if addrChip.Hovered() {
			addrChip.Background(pal.RowHover)
		}
		addrChip.Background(pal.Chip)
		addrChip.Children(func() {
			ui.Icon(c, iconGlobe).Size(11, 11).TextColor(pal.TextFaint)
			ui.Text(c, addr).Font("monospace").FontSize(10.5).TextColor(pal.TextMuted)
		})
		if addrChip.Clicked() {
			a.copyText(c, locales.InspectorAddrCopied(addr), addr)
		}
		if text := a.proxyProblemText(); text != "" {
			ui.Row(c).Height(28).Padding(0, 8).Radius(7).Background(pal.Danger.Alpha(0.1)).
				Children(func() {
					ui.Icon(c, iconWarning).Size(11, 11).TextColor(pal.Danger)
					ui.Text(c, text).FontSize(10.5).TextColor(pal.Danger).SingleLine().
						Ellipsis("…").MaxWidth(280)
				})
		}
		ui.Spacer(c)

		if a.filtering() && len(a.flows) > 0 {
			ui.Text(c, locales.InspectorFilterCount(len(a.visibleFlows()), len(a.flows))).FontSize(10.5).
				Font("monospace").TextColor(pal.TextFaint).Shrink(0)
		}
		searchField(c, pal, &a.searchFocus, &a.flowFilter, locales.InspectorFilterTraffic()).
			Width(230).Shrink(0)

		follow := ui.ButtonBase(c.Key("follow")).Height(28).Width(28).Radius(7).Center().
			Label(locales.InspectorFollow()).Tooltip(locales.InspectorFollowHint())
		if a.follow {
			follow.Background(pal.AccentSoft)
		} else if follow.Hovered() {
			follow.Background(pal.RowHover)
		}
		follow.Children(func() {
			col := pal.TextFaint
			if a.follow {
				col = pal.Accent
			}
			ui.Icon(c, iconDown).Size(12, 12).TextColor(col)
		})
		if follow.Clicked() {
			a.follow = !a.follow
			if a.follow {
				if visible := a.visibleFlows(); len(visible) > 0 {
					a.flowSelID = a.flows[visible[len(visible)-1]].ID
				}
			}
			a.prefs.FollowFlows = a.follow
			a.savePrefs()
		}
		if b := iconButton(c, pal, iconExport, locales.InspectorExportHAR(), 28); b.Clicked() {
			a.exportHAR()
		}
		if b := iconButton(c, pal, iconTrash, locales.InspectorClearSession(), 28); b.Clicked() {
			a.clearFlows()
		}
		if b := iconButton(c, pal, iconShield, locales.CaTitle(), 28); b.Clicked() {
			a.caOpen = true
		}
	})
}

func (a *app) flowListPane(c *ui.Context, pal *uitokens.Palette) {
	pane := ui.Column(c).Fill().MinHeight(0)
	pane.Children(func() {
		a.flowListHeader(c, pal)
		ui.Box(c).Grow(1).MinHeight(0).Children(func() {
			if len(a.flows) == 0 {
				a.captureEmpty(c, pal)
				return
			}
			ordered := a.visibleFlows()
			if len(ordered) == 0 {
				ui.Column(c).Fill().Center().Gap(8).Children(func() {
					ui.Icon(c, iconFilter).Size(20, 20).TextColor(pal.TextFaint)
					ui.Text(c, locales.InspectorNoMatch()).FontSize(12).TextColor(pal.TextMuted)
					if quietButton(c, pal, "clear-filters", locales.InspectorClearFilters()).Clicked() {
						a.clearFilters()
					}
				})
				return
			}
			a.flowList.Key = func(i int) any { return a.flows[ordered[i]].ID }
			a.flowList.Label = func(i int) string {
				f := a.flows[ordered[i]]
				return f.Method + " " + f.Host + f.Path
			}
			a.flowList.FollowEnd = a.follow
			a.flowList.FollowAnimation = &ui.Smooth
			a.flowList.Selected = nil
			ui.List(c, &a.flowList, len(ordered), func(i int) {
				a.flowRow(c, pal, &a.flows[ordered[i]], ordered[i])
			}).Grow(1).MinHeight(0)
		})
	})
}

func (a *app) flowListHeader(c *ui.Context, pal *uitokens.Palette) {
	row := ui.Row(c).Height(28).Shrink(0).Padding(0, 12).Gap(8).
		BorderWidth(0, 0, 1, 0).BorderColor(pal.Divider)
	row.Children(func() {
		label := func(text string, w float32, align ui.Align) {
			ui.Text(c, strings.ToUpper(text)).Width(w).Shrink(0).FontSize(9).
				FontWeight(700).LetterSpacing(0.6).TextColor(pal.TextFaint).TextAlign(align)
		}
		label(locales.InspectorColMethod(), 52, ui.Start)
		a.statusHead(c, pal)
		label(locales.InspectorColHost(), 160, ui.Start)
		ui.Text(c, strings.ToUpper(locales.InspectorColPath())).Grow(1).MinWidth(0).FontSize(9).
			FontWeight(700).LetterSpacing(0.6).TextColor(pal.TextFaint)
		label(locales.InspectorColSize(), 58, ui.End)
		label(locales.InspectorColTime(), 54, ui.End)
		ui.Text(c, strings.ToUpper(locales.InspectorColAt())).Width(54).Shrink(0).FontSize(9).FontWeight(700).
			LetterSpacing(0.6).TextColor(pal.TextFaint).TextAlign(ui.End)
	})
}

// statusHead is the Status column's heading and the list's status filter:
// the chosen class is named in the accent.
func (a *app) statusHead(c *ui.Context, pal *uitokens.Palette) {
	label, col := strings.ToUpper(locales.InspectorColStatus()), pal.TextFaint
	if a.statusFilter != "" {
		label, col = statusFilterHead(a.statusFilter), pal.Accent
	}
	b := ui.ButtonBase(c.Key("status-filter")).Height(22).Width(52).Shrink(0).Padding(0, 6).
		Radius(5).Gap(3).Label(locales.InspectorFilterByStatus()).
		Tooltip(locales.InspectorFilterByStatus())
	if a.statusFilter != "" {
		b.Background(pal.AccentSoft)
	} else if b.Hovered() {
		b.Background(pal.RowHover)
	}
	b.Children(func() {
		ui.Text(c, label).FontSize(9).FontWeight(700).LetterSpacing(0.6).TextColor(col)
		ui.Icon(c, iconChevron).Size(8, 8).TextColor(col)
	})
	b.Menu(func(m *ui.Menu) {
		groups := []struct {
			value string
			label func() string
		}{
			{"", locales.InspectorStatusAll},
			{"2xx", locales.InspectorStatus2xx},
			{"3xx", locales.InspectorStatus3xx},
			{"4xx", locales.InspectorStatus4xx},
			{"5xx", locales.InspectorStatus5xx},
			{"err", locales.InspectorStatusErr},
		}
		for _, g := range groups {
			if m.Item(g.label()).Checked(a.statusFilter == g.value).Chosen() {
				a.statusFilter = g.value
			}
		}
	})
}

// statusFilterHead is the status heading while a class is chosen: the
// class in the accent, or ERR when no response came back.
func statusFilterHead(value string) string {
	if value == "err" {
		return locales.CommonErrorShort()
	}
	return strings.ToUpper(value)
}

func (a *app) flowRow(c *ui.Context, pal *uitokens.Palette, f *proxy.Flow, index int) {
	selected := a.flowSelID == f.ID
	row := ui.ButtonBase(c.Key(fmt.Sprintf("flow:%d", f.ID))).Height(29).Shrink(0).
		Padding(0, 12).Gap(8).Label(f.Method + " " + f.Host + f.Path)
	if selected {
		row.Background(pal.AccentSoft)
	} else if row.Hovered() {
		row.Background(pal.RowHover)
	}
	row.BorderWidth(0, 0, 1, 0).BorderColor(pal.Divider.Alpha(0.5))
	row.Children(func() {
		if selected {
			ui.Box(c).Absolute().Left(0).Top(5).Bottom(5).Width(2).Radius(1).
				Background(pal.Accent)
		}
		ui.Text(c, f.Method).Width(52).Shrink(0).FontSize(9).FontWeight(750).
			TextColor(uitokens.MethodColor(pal, f.Method))
		statusColor := uitokens.StatusColor(pal, f.Status)
		statusText := fmt.Sprint(f.Status)
		if f.Status == 0 {
			// No answer came back: the proxy could not reach the server.
			statusText, statusColor = locales.CommonErrorShort(), pal.Danger
		}
		ui.Text(c, statusText).Width(52).Shrink(0).FontSize(10).FontWeight(650).
			TextColor(statusColor)
		host := ui.Row(c).Width(160).Shrink(0).Gap(4)
		host.Children(func() {
			if f.TLS {
				ui.Icon(c, iconLock).Size(9, 9).TextColor(pal.TextFaint)
			}
			ui.Text(c, f.Host).FontSize(10.5).TextColor(pal.TextMuted).SingleLine().
				Ellipsis("…").Grow(1).MinWidth(0)
		})
		path := f.Path
		if path == "" {
			path = "/"
		}
		ui.Text(c, path).Grow(1).MinWidth(0).Font("monospace").FontSize(10.5).
			TextColor(pal.Text).SingleLine().Ellipsis("…")
		size := "—"
		if f.Status > 0 {
			size = httpx.FormatBytes(f.RespSize)
		}
		ui.Text(c, size).Width(58).Shrink(0).Font("monospace").
			FontSize(9.5).TextColor(pal.TextFaint).TextAlign(ui.End)
		durCol := pal.TextMuted
		if f.Duration > 2*time.Second {
			durCol = pal.Danger
		} else if f.Duration > 800*time.Millisecond {
			durCol = pal.Warning
		}
		ui.Text(c, httpx.FormatDuration(f.Duration)).Width(54).Shrink(0).Font("monospace").
			FontSize(9.5).TextColor(durCol).TextAlign(ui.End)
		ui.Text(c, f.At.Format("15:04:05")).Width(54).Shrink(0).Font("monospace").
			FontSize(9.5).TextColor(pal.TextFaint).TextAlign(ui.End)
	})
	// A right click chooses the row too, so the menu acts on the flow shown.
	if row.RightClicked() && !selected {
		a.selectFlow(f.ID)
	}
	row.ContextMenu(func(m *ui.Menu) {
		a.flowMenu(c, m, f)
	})
	if row.RowDoubleClicked() || row.DoubleClicked() {
		a.sendFlowToComposer(*f)
		return
	}
	if row.Clicked() {
		a.selectFlow(f.ID)
	}
}

// flowMenu is what a flow's row and its detail offer to do with it.
func (a *app) flowMenu(c *ui.Context, m *ui.Menu, f *proxy.Flow) {
	if m.Item(locales.InspectorSendToComposer()).Chosen() {
		a.sendFlowToComposer(*f)
	}
	m.Separator()
	if m.Item(locales.CommonCopyURL()).Chosen() {
		a.copyText(c, locales.CommonURLCopied(), f.URL)
	}
	if m.Item(locales.CommonCopyCurl()).Shortcut(ui.Cmd|ui.Shift, ui.KeyC).Chosen() {
		a.copyText(c, locales.ComposerCurlCopied(), flowCurl(*f))
	}
	m.Separator()
	if a.domainFilter == "" {
		if m.Item(locales.InspectorShowOnly(f.Host)).Chosen() {
			a.domainFilter = f.Host
		}
	} else if m.Item(locales.SidebarShowAllDomains()).Chosen() {
		a.domainFilter = ""
	}
}

func (a *app) captureEmpty(c *ui.Context, pal *uitokens.Palette) {
	ui.Column(c).Fill().Center().Gap(10).Padding(24).Children(func() {
		ui.Box(c).Size(50, 50).Radius(14).Center().Background(pal.AccentSoft).Children(func() {
			ui.Icon(c, iconInspect).Size(23, 23).TextColor(pal.Accent)
		})
		title := locales.InspectorWaiting()
		body := locales.InspectorWaitingHint("127.0.0.1:" + a.proxyPort)
		if !a.proxyRunning() {
			title = locales.InspectorProxyPausedTitle()
			body = locales.InspectorPausedHint("127.0.0.1:" + a.proxyPort)
		}
		ui.Text(c, title).FontSize(13.5).FontWeight(650).TextColor(pal.Text)
		ui.Text(c, body).FontSize(11.5).TextColor(pal.TextMuted).TextAlign(ui.Center).
			MaxWidth(430).LineHeight(1.4)
		ui.Row(c).Gap(8).Children(func() {
			if !a.proxyRunning() {
				if ui.PrimaryButton(c.Key("start-empty"), locales.InspectorStartCapture()).Clicked() {
					a.startProxy()
				}
			}
			if quietButton(c, pal, "ca-empty", locales.InspectorRootCA()).Clicked() {
				a.caOpen = true
			}
		})
	})
}

func (a *app) flowDetailPane(c *ui.Context, pal *uitokens.Palette) {
	f := a.selectedFlow()
	if f == nil {
		ui.Box(c).Fill().Children(func() {
			emptyState(c, pal, iconGlobe, locales.InspectorSelectFlow(), locales.InspectorSelectFlowHint())
		})
		return
	}
	col := ui.Column(c).Fill().MinHeight(0)
	col.Children(func() {
		a.flowDetailHeader(c, pal, f)
		lineTabs(c, pal, &a.detailTab, []string{locales.InspectorTabRequest(), locales.InspectorTabResponse()})
		a.flowDetailBody(c, pal, f)
	})
}

func (a *app) flowDetailHeader(c *ui.Context, pal *uitokens.Palette, f *proxy.Flow) {
	row := ui.Row(c).Height(42).Shrink(0).Padding(0, 12).Gap(8).
		BorderWidth(0, 0, 1, 0).BorderColor(pal.Divider)
	row.Children(func() {
		methodBadge(c, pal, f.Method)
		if f.Status > 0 {
			statusBadge(c, pal, f.Status)
		}
		ui.Column(c).Gap(0).Justify(ui.Center).Grow(1).MinWidth(0).Children(func() {
			ui.Text(c, f.Host+f.Path).Font("monospace").FontSize(11.5).TextColor(pal.Text).
				SingleLine().Ellipsis("…")
			ui.Text(c, f.URL).FontSize(9.5).TextColor(pal.TextFaint).SingleLine().Ellipsis("…")
		})
		if f.Err != "" {
			ui.Row(c).Center().Height(20).Padding(0, 6).Radius(5).
				Background(pal.Danger.Alpha(0.12)).Children(func() {
				ui.Text(c, locales.InspectorProxyError()).FontSize(9.5).FontWeight(650).TextColor(pal.Danger)
			})
		}
		ui.Text(c, httpx.FormatDuration(f.Duration)).Font("monospace").FontSize(10).
			TextColor(pal.TextMuted).Tooltip(locales.ComposerTotalTime())
		if f.Status > 0 {
			ui.Text(c, httpx.FormatBytes(f.RespSize)).Font("monospace").FontSize(10).
				TextColor(pal.TextFaint).Tooltip(locales.InspectorResponseSize())
		}
		iconButton(c, pal, iconCopy, locales.CommonCopy(), 26).Menu(func(m *ui.Menu) {
			if m.Item(locales.CommonCopyURL()).Chosen() {
				a.copyText(c, locales.CommonURLCopied(), f.URL)
			}
			if m.Item(locales.CommonCopyCurl()).Shortcut(ui.Cmd|ui.Shift, ui.KeyC).Chosen() {
				a.copyText(c, locales.ComposerCurlCopied(), flowCurl(*f))
			}
			m.Separator()
			if m.Item(locales.InspectorCopyReqHeaders()).Disabled(len(f.ReqHeaders) == 0).Chosen() {
				a.copyText(c, locales.InspectorReqHeadersCopied(), headersText(f.ReqHeaders))
			}
			if m.Item(locales.InspectorCopyRespHeaders()).Disabled(len(f.RespHeaders) == 0).Chosen() {
				a.copyText(c, locales.InspectorRespHeadersCopied(), headersText(f.RespHeaders))
			}
		})
		send := ui.ButtonBase(c.Key("to-composer")).Height(26).Padding(0, 10).Radius(7).Gap(6).
			Label(locales.InspectorSendToComposer()).Tooltip(locales.InspectorReplayHint())
		send.Background(pal.AccentSoft)
		if send.Hovered() {
			send.Opacity(0.85)
		}
		send.Children(func() {
			ui.Icon(c, iconCompose).Size(11, 11).TextColor(pal.Accent)
			ui.Text(c, locales.InspectorSendToComposer()).FontSize(10.5).FontWeight(650).TextColor(pal.Accent)
		})
		if send.Clicked() {
			a.sendFlowToComposer(*f)
		}
	})
}

func (a *app) flowDetailBody(c *ui.Context, pal *uitokens.Palette, f *proxy.Flow) {
	headers := f.RespHeaders
	body := f.RespBody
	truncated := f.RespTruncated
	contentType := f.ContentType
	if a.detailTab == 0 {
		headers = f.ReqHeaders
		body = f.ReqBody
		truncated = f.ReqTruncated
		contentType = headerValue(f.ReqHeaders, "Content-Type")
	}
	shape := shapeOf(body, contentType)
	bodyKey := fmt.Sprintf("flow:%d:%d", f.ID, a.detailTab)
	text, isJSON := a.shownBody(bodyKey, body)
	column := ui.Column(c).Fill().MinHeight(0)
	column.Children(func() {
		sub := ui.Row(c).Height(34).Shrink(0).Padding(0, 12).Gap(8).
			BorderWidth(0, 0, 1, 0).BorderColor(pal.Divider.Alpha(0.7))
		sub.Children(func() {
			// The tabs name the side shown: the request's own words for
			// one, the answer's for the other.
			bodyLabel := locales.ComposerTabBody()
			headersLabel := locales.ComposerTabHeaders()
			if a.detailTab == 1 {
				bodyLabel = locales.ComposerTabRespBody()
				headersLabel = locales.ComposerTabRespHeaders()
			}
			if len(body) > 0 {
				bodyLabel += " · " + httpx.FormatBytes(int64(len(body)))
			}
			segmented(c, pal, &a.detailSub, []string{bodyLabel, countLabel(headersLabel, len(headers))})
			ui.Spacer(c)
			if contentType != "" {
				ui.Row(c).Center().Height(20).Padding(0, 7).Radius(5).Background(pal.Chip).
					Children(func() {
						ui.Text(c, shortContentType(contentType)).FontSize(9.5).
							TextColor(pal.TextFaint).SingleLine()
					})
			}
			if a.detailSub == 0 && shape == shapeText {
				if isJSON {
					pretty := ui.ButtonBase(c.Key("flow-pretty")).Height(22).Padding(0, 7).Radius(6).
						Label(locales.ComposerPrettyJSON())
					if a.pretty {
						pretty.Background(pal.AccentSoft)
					}
					pretty.Children(func() {
						col := pal.TextFaint
						if a.pretty {
							col = pal.Accent
						}
						ui.Text(c, locales.ComposerPretty()).FontSize(10).FontWeight(600).TextColor(col)
					})
					if pretty.Clicked() {
						a.pretty = !a.pretty
					}
				}
				if b := iconButton(c, pal, iconCopy, locales.InspectorCopyBody(), 24); b.Clicked() {
					a.copyText(c, locales.InspectorBodyCopied(), text)
				}
			} else if a.detailSub == 1 {
				if b := iconButton(c, pal, iconCopy, locales.InspectorCopyHeaders(), 24); b.Clicked() {
					a.copyText(c, locales.InspectorHeadersCopied(), headersText(headers))
				}
			}
		})
		ui.Column(c).Grow(1).MinHeight(0).Children(func() {
			if a.detailSub == 0 {
				if a.detailTab == 1 && f.Err != "" {
					ui.Row(c).Height(28).Shrink(0).Padding(0, 12).Gap(6).Background(pal.Danger.Alpha(0.08)).
						Children(func() {
							ui.Icon(c, iconWarning).Size(12, 12).TextColor(pal.Danger)
							ui.Text(c, f.Err).FontSize(10.5).TextColor(pal.Danger).SingleLine().
								Ellipsis("…").MinWidth(0).Selectable()
						})
				}
				note := ""
				if truncated {
					note = locales.InspectorTruncated()
				}
				key := fmt.Sprintf("%s:%v", bodyKey, a.pretty)
				ui.Box(c).Grow(1).MinHeight(0).Children(func() {
					a.bodyView(c, pal, key, f.URL, body, contentType, text, isJSON, note)
				})
				return
			}
			rows := make([]headerRow, 0, len(headers))
			for _, h := range headers {
				rows = append(rows, headerRow{Key: h.Key, Value: h.Value})
			}
			keyValueList(c, pal, rows, locales.InspectorNoHeaders())
		})
	})
}

func headersText(headers []proxy.HeaderPair) string {
	var b strings.Builder
	for _, h := range headers {
		b.WriteString(h.Key)
		b.WriteString(": ")
		b.WriteString(h.Value)
		b.WriteByte('\n')
	}
	return b.String()
}

func shortContentType(ct string) string {
	ct = strings.TrimSpace(ct)
	if i := strings.Index(ct, ";"); i > 0 {
		ct = ct[:i]
	}
	if len(ct) > 38 {
		ct = ct[:37] + "…"
	}
	return ct
}
