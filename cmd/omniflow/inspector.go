package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/CarlOpenLab/omniflow/internal/httpx"
	"github.com/CarlOpenLab/omniflow/internal/proxy"
	"github.com/CarlOpenLab/omniflow/internal/uitokens"
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
			Radius(7).Gap(6).Label("Start or stop capture").Tooltip("Start or stop the capture proxy")
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
				ui.Text(c, "Recording").FontSize(11.5).FontWeight(650).TextColor(pal.Danger)
			} else {
				ui.Icon(c, iconPlay).Size(11, 11).TextColor(pal.Accent)
				ui.Text(c, "Start Capture").FontSize(11.5).FontWeight(650).TextColor(pal.Accent)
			}
		})
		if rec.Clicked() {
			a.toggleProxy()
		}

		addr := "127.0.0.1:" + a.proxyPort
		addrChip := ui.ButtonBase(c.Key("addr")).Height(28).Shrink(0).Padding(0, 9).
			Radius(7).Gap(6).Label("Copy proxy address").Tooltip("Copy " + addr)
		if addrChip.Hovered() {
			addrChip.Background(pal.RowHover)
		}
		addrChip.Background(pal.Chip)
		addrChip.Children(func() {
			ui.Icon(c, iconGlobe).Size(11, 11).TextColor(pal.TextFaint)
			ui.Text(c, addr).Font("monospace").FontSize(10.5).TextColor(pal.TextMuted)
		})
		if addrChip.Clicked() {
			a.copyText(c, "Proxy address copied: "+addr, addr)
		}
		if a.proxyErr != "" {
			ui.Row(c).Height(28).Padding(0, 8).Radius(7).Background(pal.Danger.Alpha(0.1)).
				Children(func() {
					ui.Icon(c, iconWarning).Size(11, 11).TextColor(pal.Danger)
					ui.Text(c, a.proxyErr).FontSize(10.5).TextColor(pal.Danger).SingleLine().
						Ellipsis("…").MaxWidth(280)
				})
		}
		ui.Spacer(c)

		if a.filtering() && len(a.flows) > 0 {
			ui.Text(c, fmt.Sprintf("%d of %d", len(a.visibleFlows()), len(a.flows))).FontSize(10.5).
				Font("monospace").TextColor(pal.TextFaint).Shrink(0)
		}
		searchField(c, pal, &a.searchFocus, &a.flowFilter, "Filter traffic").
			Width(230).Shrink(0)

		follow := ui.ButtonBase(c.Key("follow")).Height(28).Width(28).Radius(7).Center().
			Label("Follow new flows").Tooltip("Follow new flows: choose each one as it arrives")
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
		if b := iconButton(c, pal, iconExport, "Export HAR", 28); b.Clicked() {
			a.exportHAR()
		}
		if b := iconButton(c, pal, iconTrash, "Clear Session", 28); b.Clicked() {
			a.clearFlows()
		}
		if b := iconButton(c, pal, iconShield, "Root CA Certificate", 28); b.Clicked() {
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
					ui.Text(c, "No flows match this filter").FontSize(12).TextColor(pal.TextMuted)
					if quietButton(c, pal, "clear-filters", "Clear Filters").Clicked() {
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
		label("Method", 52, ui.Start)
		a.statusHead(c, pal)
		label("Host", 160, ui.Start)
		ui.Text(c, strings.ToUpper("Path")).Grow(1).MinWidth(0).FontSize(9).
			FontWeight(700).LetterSpacing(0.6).TextColor(pal.TextFaint)
		label("Size", 58, ui.End)
		label("Time", 54, ui.End)
		ui.Text(c, strings.ToUpper("At")).Width(54).Shrink(0).FontSize(9).FontWeight(700).
			LetterSpacing(0.6).TextColor(pal.TextFaint).TextAlign(ui.End)
	})
}

// statusHead is the Status column's heading and the list's status filter:
// the chosen class is named in the accent.
func (a *app) statusHead(c *ui.Context, pal *uitokens.Palette) {
	label, col := "STATUS", pal.TextFaint
	if a.statusFilter != "" {
		label, col = strings.ToUpper(a.statusFilter), pal.Accent
	}
	b := ui.ButtonBase(c.Key("status-filter")).Height(22).Width(52).Shrink(0).Padding(0, 6).
		Radius(5).Gap(3).Label("Filter by status").Tooltip("Filter by status")
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
		groups := []struct{ value, label string }{
			{"", "All Statuses"},
			{"2xx", "2xx · Success"},
			{"3xx", "3xx · Redirect"},
			{"4xx", "4xx · Client Error"},
			{"5xx", "5xx · Server Error"},
			{"err", "Errors · no response"},
		}
		for _, g := range groups {
			if m.Item(g.label).Checked(a.statusFilter == g.value).Chosen() {
				a.statusFilter = g.value
			}
		}
	})
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
			statusText, statusColor = "ERR", pal.Danger
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
	if m.Item("Send to Composer").Chosen() {
		a.sendFlowToComposer(*f)
	}
	m.Separator()
	if m.Item("Copy URL").Chosen() {
		a.copyText(c, "URL copied", f.URL)
	}
	if m.Item("Copy as cURL").Shortcut(ui.Cmd|ui.Shift, ui.KeyC).Chosen() {
		a.copyText(c, "cURL command copied", flowCurl(*f))
	}
	m.Separator()
	if a.domainFilter == "" {
		if m.Item("Show Only " + f.Host).Chosen() {
			a.domainFilter = f.Host
		}
	} else if m.Item("Show All Domains").Chosen() {
		a.domainFilter = ""
	}
}

func (a *app) captureEmpty(c *ui.Context, pal *uitokens.Palette) {
	ui.Column(c).Fill().Center().Gap(10).Padding(24).Children(func() {
		ui.Box(c).Size(50, 50).Radius(14).Center().Background(pal.AccentSoft).Children(func() {
			ui.Icon(c, iconInspect).Size(23, 23).TextColor(pal.Accent)
		})
		title := "Waiting for traffic"
		body := "Point an app or device at 127.0.0.1:" + a.proxyPort + ", or press Start Capture. HTTPS needs the Root CA installed and trusted."
		if !a.proxyRunning() {
			title = "The proxy is paused"
			body = "Start the capture, then set a client's HTTP proxy to 127.0.0.1:" + a.proxyPort + ". The flows appear here as they pass."
		}
		ui.Text(c, title).FontSize(13.5).FontWeight(650).TextColor(pal.Text)
		ui.Text(c, body).FontSize(11.5).TextColor(pal.TextMuted).TextAlign(ui.Center).
			MaxWidth(430).LineHeight(1.4)
		ui.Row(c).Gap(8).Children(func() {
			if !a.proxyRunning() {
				if ui.PrimaryButton(c.Key("start-empty"), "Start Capture").Clicked() {
					a.startProxy()
				}
			}
			if quietButton(c, pal, "ca-empty", "Root CA…").Clicked() {
				a.caOpen = true
			}
		})
	})
}

func (a *app) flowDetailPane(c *ui.Context, pal *uitokens.Palette) {
	f := a.selectedFlow()
	if f == nil {
		ui.Box(c).Fill().Children(func() {
			emptyState(c, pal, iconGlobe, "Select a flow",
				"Click a row above to read its request and response. Double-click sends it to the Composer.")
		})
		return
	}
	col := ui.Column(c).Fill().MinHeight(0)
	col.Children(func() {
		a.flowDetailHeader(c, pal, f)
		lineTabs(c, pal, &a.detailTab, []string{"Request", "Response"})
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
				ui.Text(c, "proxy error").FontSize(9.5).FontWeight(650).TextColor(pal.Danger)
			})
		}
		ui.Text(c, httpx.FormatDuration(f.Duration)).Font("monospace").FontSize(10).
			TextColor(pal.TextMuted).Tooltip("Total time")
		if f.Status > 0 {
			ui.Text(c, httpx.FormatBytes(f.RespSize)).Font("monospace").FontSize(10).
				TextColor(pal.TextFaint).Tooltip("Response size")
		}
		iconButton(c, pal, iconCopy, "Copy", 26).Menu(func(m *ui.Menu) {
			if m.Item("Copy URL").Chosen() {
				a.copyText(c, "URL copied", f.URL)
			}
			if m.Item("Copy as cURL").Shortcut(ui.Cmd|ui.Shift, ui.KeyC).Chosen() {
				a.copyText(c, "cURL command copied", flowCurl(*f))
			}
			m.Separator()
			if m.Item("Copy Request Headers").Disabled(len(f.ReqHeaders) == 0).Chosen() {
				a.copyText(c, "Request headers copied", headersText(f.ReqHeaders))
			}
			if m.Item("Copy Response Headers").Disabled(len(f.RespHeaders) == 0).Chosen() {
				a.copyText(c, "Response headers copied", headersText(f.RespHeaders))
			}
		})
		send := ui.ButtonBase(c.Key("to-composer")).Height(26).Padding(0, 10).Radius(7).Gap(6).
			Label("Send to Composer").Tooltip("Replay this request in the Composer")
		send.Background(pal.AccentSoft)
		if send.Hovered() {
			send.Opacity(0.85)
		}
		send.Children(func() {
			ui.Icon(c, iconCompose).Size(11, 11).TextColor(pal.Accent)
			ui.Text(c, "Send to Composer").FontSize(10.5).FontWeight(650).TextColor(pal.Accent)
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
	column := ui.Column(c).Fill().MinHeight(0)
	column.Children(func() {
		sub := ui.Row(c).Height(34).Shrink(0).Padding(0, 12).Gap(8).
			BorderWidth(0, 0, 1, 0).BorderColor(pal.Divider.Alpha(0.7))
		sub.Children(func() {
			bodyLabel := "Body"
			if len(body) > 0 {
				bodyLabel += " · " + httpx.FormatBytes(int64(len(body)))
			}
			segmented(c, pal, &a.detailSub, []string{bodyLabel, countLabel("Headers", len(headers))})
			ui.Spacer(c)
			if contentType != "" {
				ui.Row(c).Center().Height(20).Padding(0, 7).Radius(5).Background(pal.Chip).
					Children(func() {
						ui.Text(c, shortContentType(contentType)).FontSize(9.5).
							TextColor(pal.TextFaint).SingleLine()
					})
			}
			if a.detailSub == 0 && shape == shapeText {
				if _, isJSON := httpx.Pretty(body); isJSON {
					pretty := ui.ButtonBase(c.Key("flow-pretty")).Height(22).Padding(0, 7).Radius(6).
						Label("Pretty JSON")
					if a.pretty {
						pretty.Background(pal.AccentSoft)
					}
					pretty.Children(func() {
						col := pal.TextFaint
						if a.pretty {
							col = pal.Accent
						}
						ui.Text(c, "Pretty").FontSize(10).FontWeight(600).TextColor(col)
					})
					if pretty.Clicked() {
						a.pretty = !a.pretty
					}
				}
				if b := iconButton(c, pal, iconCopy, "Copy Body", 24); b.Clicked() {
					a.copyText(c, "Body copied", a.displayFlowBody(body))
				}
			} else if a.detailSub == 1 {
				if b := iconButton(c, pal, iconCopy, "Copy Headers", 24); b.Clicked() {
					a.copyText(c, "Headers copied", headersText(headers))
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
					note = "The body is larger than the inspector keeps; this is the beginning."
				}
				key := fmt.Sprintf("flow:%d:%d", f.ID, a.detailTab)
				ui.Box(c).Grow(1).MinHeight(0).Children(func() {
					a.bodyView(c, pal, key, f.URL, body, contentType, a.displayFlowBody(body), note)
				})
				return
			}
			rows := make([]headerRow, 0, len(headers))
			for _, h := range headers {
				rows = append(rows, headerRow{Key: h.Key, Value: h.Value})
			}
			keyValueList(c, pal, rows, "This message carries no headers.")
		})
	})
}

func (a *app) displayFlowBody(body []byte) string {
	text := string(body)
	if a.pretty {
		if pretty, ok := httpx.Pretty(body); ok {
			return pretty
		}
	}
	return text
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
