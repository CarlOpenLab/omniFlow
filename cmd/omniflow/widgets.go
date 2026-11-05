package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/CarlOpenLab/omniflow/internal/httpx"
	"github.com/CarlOpenLab/omniflow/internal/model"
	"github.com/CarlOpenLab/omniflow/internal/uitokens"
	"github.com/CarlOpenLab/omniflow/locales"
)

func paletteFor(c *ui.Context) *uitokens.Palette {
	return uitokens.For(c.Theme().Dark)
}

// applyAppearance asks the app for an appearance: system, light or dark.
func applyAppearance(mode string) {
	switch mode {
	case "light":
		mygo.Theme.SetSource(mygo.ThemeLight)
	case "dark":
		mygo.Theme.SetSource(mygo.ThemeDark)
	default:
		mygo.Theme.SetSource(mygo.ThemeSystem)
	}
}

// applyTheme makes the native widgets read as the app's: their surfaces,
// signal colors and sizes, in the appearance the frame already has.
//
// Background is transparent: the sidebar sits on the window's material and
// the workbench on the card, so a framework control that paints the theme's
// background would cut an opaque block out of the frost. Only the dialogs,
// which are solid Popover, draw the framework's surfaces on purpose.
func applyTheme(c *ui.Context, mode string) {
	t := *c.Theme()
	pal := uitokens.For(t.Dark)
	switch mode {
	case "light":
		t.Dark = false
		pal = uitokens.For(false)
	case "dark":
		t.Dark = true
		pal = uitokens.For(true)
	}
	t.Background = ui.Transparent
	t.Surface = pal.Control
	t.SurfaceHover = pal.ControlHover
	t.SurfacePressed = pal.Chip
	t.Border = pal.Edge
	t.Text = pal.Text
	t.TextMuted = pal.TextMuted
	t.Accent = pal.Accent
	t.AccentHover = pal.AccentHover
	t.AccentPressed = pal.AccentHover
	t.AccentText = pal.OnAccent
	t.Danger = pal.Danger
	t.Warning = pal.Warning
	t.Success = pal.Success
	t.Selection = pal.Selection
	t.Focus = pal.Accent.Alpha(0.85)
	t.Scrollbar = pal.TextFaint.Alpha(0.35)
	t.Inverse = pal.Popover
	t.InverseText = pal.Text
	t.Radius = 8
	t.Spacing = 4
	t.FontSize = 13
	c.SetTheme(&t)
}

// raised lifts the workspace card: in the light, a hairline and two soft
// shadows; in the dark, a lit top edge over three tight shadows, where a
// shadow alone would disappear. All of it is translucent — the card lets the
// material through, and a solid edge would ring it.
func raised(el ui.Element, pal *uitokens.Palette) {
	if pal.Dark {
		el.Border(1, pal.Text.Alpha(0.12))
		el.BorderLinearGradient(ui.LinearGradient{
			Angle: 180,
			From:  pal.Text.Alpha(0.18),
			To:    pal.Text.Alpha(0.06),
		})
		el.Shadow(0, 1, 1, 0, ui.RGBA(0, 0, 0, 0.20))
		el.Shadow(0, 2, 2, -1, ui.RGBA(0, 0, 0, 0.20))
		el.Shadow(0, 4, 6, -3, ui.RGBA(0, 0, 0, 0.24))
		return
	}
	el.Border(1, pal.CardBorder)
	el.Shadow(0, 1, 2, 0, pal.CardShadow)
	el.Shadow(0, 1, 6, 0, pal.CardShadow2)
}

// iconButton is a square quiet control in the chrome.
func iconButton(c *ui.Context, pal *uitokens.Palette, icon *ui.SVG, caption string, size float32) ui.Element {
	b := ui.ButtonBase(c.Key(caption)).Size(size, size).Shrink(0).Radius(7).Center().
		Label(caption).Tooltip(caption)
	if b.Hovered() {
		b.Background(pal.RowHover)
	}
	b.Children(func() {
		ui.Icon(c, icon).Size(size*0.5, size*0.5).TextColor(pal.TextMuted)
	})
	return b
}

// iconAction is the same control with a label beside it, for toolbars where
// a word is clearer than a tooltip.
func iconAction(c *ui.Context, pal *uitokens.Palette, icon *ui.SVG, label string) ui.Element {
	b := ui.ButtonBase(c.Key(label)).Height(28).Padding(0, 9).Radius(7).Gap(6).
		Label(label).Tooltip(label)
	if b.Hovered() {
		b.Background(pal.RowHover)
	}
	b.Children(func() {
		ui.Icon(c, icon).Size(13, 13).TextColor(pal.TextMuted)
		ui.Text(c, label).FontSize(12).FontWeight(550).TextColor(pal.TextMuted)
	})
	return b
}

// kbd writes a shortcut the way a keyboard does.
func kbd(c *ui.Context, pal *uitokens.Palette, text string) ui.Element {
	return ui.Row(c).Center().Height(18).Shrink(0).Padding(0, 5).Radius(4).
		Background(pal.Kbd).Border(1, pal.KbdBorder).Children(func() {
		ui.Text(c, text).FontSize(10).FontWeight(600).TextColor(pal.TextFaint)
	})
}

func methodBadge(c *ui.Context, pal *uitokens.Palette, method string) ui.Element {
	col := uitokens.MethodColor(pal, method)
	return ui.Row(c).Center().Height(20).Shrink(0).Padding(0, 6).Radius(5).
		Background(col.Alpha(0.13)).Children(func() {
		ui.Text(c, method).FontSize(9.5).FontWeight(750).LetterSpacing(0.35).TextColor(col)
	})
}

func statusBadge(c *ui.Context, pal *uitokens.Palette, status int) ui.Element {
	col := uitokens.StatusColor(pal, status)
	text := "—"
	if status > 0 {
		text = fmt.Sprint(status)
	}
	return ui.Row(c).Center().Height(20).Shrink(0).Padding(0, 6).Radius(5).
		Background(col.Alpha(0.13)).Children(func() {
		ui.Text(c, text).FontSize(10).FontWeight(700).TextColor(col)
	})
}

func statusDot(c *ui.Context, col ui.Color, size float32) ui.Element {
	return ui.Box(c).Size(size, size).Shrink(0).Radius(size / 2).Background(col)
}

// lineTabs is a strip of underlined tabs, the composer's request and
// response sections.
func lineTabs(c *ui.Context, pal *uitokens.Palette, sel *int, labels []string) ui.Element {
	row := ui.Row(c).Height(uitokens.TabH).Shrink(0).Padding(12, 0, 12, 0).Gap(2).
		BorderWidth(0, 0, 1, 0).BorderColor(pal.Divider)
	row.Children(func() {
		for i := range labels {
			idx := i
			selected := i == *sel
			tab := ui.ButtonBase(c.Key("line:"+labels[i])).Height(uitokens.TabH).
				Padding(0, 10).Center().Label(labels[i])
			col := pal.TextMuted
			if selected {
				col = pal.Text
			}
			if tab.Hovered() && !selected {
				col = pal.Text
			}
			tab.Children(func() {
				ui.Text(c, labels[i]).FontSize(12.5).FontWeight(600).TextColor(col)
				if selected {
					ui.Box(c).Absolute().Left(7).Right(7).Bottom(0).Height(2).Radius(1).
						Background(pal.Accent)
				}
			})
			if tab.Clicked() {
				*sel = idx
			}
		}
	})
	return row
}

// segmented is the small choice control used inside a panel: a trough with
// one lifted segment, as the prototype's mode switch.
func segmented(c *ui.Context, pal *uitokens.Palette, sel *int, labels []string) ui.Element {
	return segmentedKey(c, pal, "seg:", sel, labels)
}

// segmentedKey is segmented with its widgets keyed under prefix, so two
// rows with the same label in one language do not share a key.
func segmentedKey(c *ui.Context, pal *uitokens.Palette, prefix string, sel *int, labels []string) ui.Element {
	track := ui.Row(c).Shrink(0).Padding(2).Radius(8).Gap(1).Background(pal.Chip)
	track.Children(func() {
		for i := range labels {
			idx := i
			selected := i == *sel
			b := ui.ButtonBase(c.Key(prefix+labels[i])).Height(24).Padding(0, 10).
				Radius(6).Center().Label(labels[i]).Background(ui.Transparent)
			col := pal.TextMuted
			if selected {
				col = pal.Text
				b.Background(pal.WashSelected).Border(1, pal.EdgeStrong)
			} else if b.Hovered() {
				col = pal.Text
				b.Background(pal.RowHover)
			}
			b.Children(func() {
				ui.Text(c, labels[i]).FontSize(11.5).FontWeight(600).TextColor(col)
			})
			if b.Clicked() {
				*sel = idx
			}
		}
	})
	return track
}

// sectionTitle is a sidebar group's small caps heading.
func sectionTitle(c *ui.Context, pal *uitokens.Palette, title string, count int) ui.Element {
	row := ui.Row(c).Height(28).Grow(1).Shrink(0).Padding(4, 0, 0, 10).Gap(6)
	row.Children(func() {
		ui.Text(c, strings.ToUpper(title)).FontSize(9.5).FontWeight(750).
			LetterSpacing(0.8).TextColor(pal.TextMuted)
		if count > 0 {
			ui.Text(c, fmt.Sprint(count)).FontSize(9.5).FontWeight(600).TextColor(pal.TextFaint)
		}
	})
	return row
}

// emptyState is what a pane says before there is anything to show.
func emptyState(c *ui.Context, pal *uitokens.Palette, icon *ui.SVG, title, body string) ui.Element {
	return ui.Column(c).Fill().Center().Gap(10).Padding(24).Children(func() {
		ui.Box(c).Size(52, 52).Radius(14).Center().Background(pal.AccentSoft).Children(func() {
			ui.Icon(c, icon).Size(24, 24).TextColor(pal.Accent)
		})
		ui.Text(c, title).FontSize(14).FontWeight(650).TextColor(pal.Text)
		ui.Text(c, body).FontSize(12).TextColor(pal.TextMuted).TextAlign(ui.Center).
			MaxWidth(360).LineHeight(1.45)
	})
}

// searchField is a filter box drawn in washes: the framework's SearchField
// paints the theme's surface, a solid block over the frost.
func searchField(c *ui.Context, pal *uitokens.Palette, focus *ui.Handle, value *string, placeholder string) ui.Element {
	row := ui.Row(c).Height(28).Padding(0, 8).Gap(6).Radius(7).
		Background(pal.Chip).Border(1, pal.Edge)
	var in ui.Element
	row.Children(func() {
		ui.Icon(c, iconSearch).Size(11, 11).TextColor(pal.TextFaint)
		in = ui.TextInputBase(c.Key("flow-search"), value).Grow(1).MinWidth(0).
			FontSize(11.5).Placeholder(placeholder).Label(locales.InspectorFilterTraffic())
		in = in.Bind(focus)
		if *value != "" {
			if b := iconButton(c, pal, iconX, locales.CommonClearFilter(), 16); b.Clicked() {
				*value = ""
				in.Focus()
			}
		}
	})
	switch {
	case in.Focused():
		row.Border(1, pal.Accent)
	case row.Hovered():
		row.Border(1, pal.EdgeStrong)
	}
	row.Tooltip(locales.InspectorFilterHint())
	return row
}

// quietButton is a labelled control drawn in washes, for the card: the
// framework's Button paints an opaque surface, which would cut into the
// frost the card lets through.
func quietButton(c *ui.Context, pal *uitokens.Palette, key, label string) ui.Element {
	b := ui.ButtonBase(c.Key(key)).Height(28).Padding(0, 12).Radius(7).Gap(6).Label(label)
	b.Background(pal.Chip).Border(1, pal.Edge)
	if b.Hovered() {
		b.Background(pal.WashSelected).Border(1, pal.EdgeStrong)
	}
	b.Children(func() {
		ui.Text(c, label).FontSize(11.5).FontWeight(600).TextColor(pal.TextMuted)
	})
	return b
}

// washCheckbox is a check box drawn in washes: the framework's own paints the
// theme's background when it is off, which would cut a solid square out of a
// translucent card. The box is a wash at rest and under the pointer, and the
// accent when it is on.
func washCheckbox(c *ui.Context, pal *uitokens.Palette, on *bool, caption string) ui.Element {
	b := ui.CheckboxBase(c, on).Size(18, 18).Shrink(0).Center().Label(caption).Tooltip(caption)
	b.Children(func() {
		box := ui.Box(c).Size(15, 15).Radius(4)
		if *on {
			box.Background(pal.Accent).Border(1, pal.Accent)
		} else if b.Hovered() {
			box.Background(pal.WashSelected).Border(1, pal.EdgeStrong)
		} else {
			box.Background(pal.Chip).Border(1, pal.EdgeStrong)
		}
		if *on {
			box.Children(func() {
				ui.Icon(c, iconCheck).Size(10, 10).TextColor(pal.OnAccent)
			})
		}
	})
	return b
}

// kvKeyWidth is the key column of a row table, shared by its heads and rows.
const kvKeyWidth = 180

// kvEditor is a grouped inset table of query, header or form rows.
func kvEditor(c *ui.Context, pal *uitokens.Palette, rows *[]model.KV, keyHint, valueHint, empty string) ui.Element {
	panel := ui.Column(c).FillWidth().Radius(10).Background(pal.Chip).
		Border(1, pal.Edge).Clip()
	panel.Children(func() {
		if len(*rows) == 0 {
			ui.Row(c).Height(40).Padding(0, 12).Children(func() {
				ui.Text(c, empty).FontSize(12).TextColor(pal.TextFaint)
			})
		} else {
			kvHeads(c, pal, keyHint, valueHint)
		}
		for i := range *rows {
			idx := i
			kv := &(*rows)[i]
			row := ui.Row(c.Key(kv.ID)).Height(36).Shrink(0).Padding(0, 8, 0, 6).Gap(6).
				BorderWidth(0, 0, 1, 0).BorderColor(pal.Divider)
			if row.Hovered() {
				row.Background(pal.RowHover)
			}
			// A row left out of the request reads as left out.
			text := pal.Text
			if !kv.Enabled {
				text = pal.TextFaint
			}
			row.Children(func() {
				washCheckbox(c, pal, &kv.Enabled, locales.KvSendRow())
				ui.TextInputBase(c.Key(kv.ID+"-k"), &kv.Key).Placeholder(keyHint).
					Width(kvKeyWidth).Shrink(0).FontSize(12).Font("monospace").Padding(2, 4).TextColor(text)
				ui.Box(c).Width(1).Height(18).Shrink(0).Background(pal.Divider)
				ui.TextInputBase(c.Key(kv.ID+"-v"), &kv.Value).Placeholder(valueHint).
					Grow(1).MinWidth(0).FontSize(12).Font("monospace").Padding(2, 4).TextColor(text)
				removeButton(c, pal, locales.KvRemoveRow()).OnClick(func() {
					r := *rows
					if idx >= 0 && idx < len(r) {
						*rows = append(r[:idx], r[idx+1:]...)
					}
				})
			})
		}
		add := ui.Row(c).Height(34).Shrink(0).Padding(0, 10).Gap(6).Cursor(ui.CursorPointer)
		if add.Hovered() {
			add.Background(pal.RowHover)
		}
		add.Children(func() {
			ui.Icon(c, iconPlus).Size(11, 11).TextColor(pal.TextFaint)
			ui.Text(c, locales.KvAddRow()).FontSize(11.5).TextColor(pal.TextFaint)
		})
		if add.Clicked() {
			*rows = append(*rows, model.NewKV())
		}
	})
	return panel
}

// kvHeads names a row table's columns, over the inputs they head.
func kvHeads(c *ui.Context, pal *uitokens.Palette, keyHint, valueHint string) {
	ui.Row(c).Height(26).Shrink(0).Padding(0, 8, 0, 6).Gap(6).
		BorderWidth(0, 0, 1, 0).BorderColor(pal.Divider).Children(func() {
		head := func(text string) ui.Element {
			return ui.Text(c, strings.ToUpper(text)).FontSize(9).FontWeight(700).LetterSpacing(0.6).
				TextColor(pal.TextFaint).Padding(0, 4)
		}
		ui.Box(c).Width(20).Shrink(0)
		head(keyHint).Width(kvKeyWidth).Shrink(0)
		ui.Box(c).Width(1).Shrink(0)
		head(valueHint).Grow(1).MinWidth(0)
		ui.Box(c).Width(24).Shrink(0)
	})
}

// removeButton is a row's quiet delete, red only under the pointer.
func removeButton(c *ui.Context, pal *uitokens.Palette, caption string) ui.Element {
	b := ui.ButtonBase(c.Key(caption)).Size(24, 24).Shrink(0).Radius(7).Center().
		Label(caption).Tooltip(caption)
	col := pal.TextFaint
	if b.Hovered() {
		b.Background(pal.Danger.Alpha(0.1))
		col = pal.Danger
	}
	b.Children(func() {
		ui.Icon(c, iconTrash).Size(12, 12).TextColor(col)
	})
	return b
}

// headerRow is one line of a header table.
type headerRow struct{ Key, Value string }

// headerList shows response or request headers as a grouped, readable list.
func headerList(c *ui.Context, pal *uitokens.Palette, headers []httpx.Header, empty string) ui.Element {
	rows := make([]headerRow, 0, len(headers))
	for _, h := range headers {
		rows = append(rows, headerRow{Key: h.Key, Value: h.Value})
	}
	return keyValueList(c, pal, rows, empty)
}

func keyValueList(c *ui.Context, pal *uitokens.Palette, rows []headerRow, empty string) ui.Element {
	scroll := ui.Scroll(c).Grow(1).Background(pal.CodeBg)
	scroll.Children(func() {
		col := ui.Column(c).Padding(10, 12).Gap(0)
		col.Children(func() {
			if len(rows) == 0 {
				ui.Text(c, empty).FontSize(12).TextColor(pal.TextFaint).Padding(6, 2)
				return
			}
			for _, row := range rows {
				r := ui.Row(c).AlignItems(ui.Start).Padding(6, 8, 7, 8).Gap(12).
					BorderWidth(0, 0, 1, 0).BorderColor(pal.CodeEdge)
				r.Children(func() {
					ui.Text(c, row.Key).Width(190).Shrink(0).Font("monospace").FontSize(11.5).
						TextColor(pal.TextMuted).Selectable()
					ui.Text(c, row.Value).Grow(1).MinWidth(0).Font("monospace").FontSize(11.5).
						TextColor(pal.CodeText).Selectable()
				})
			}
		})
	})
	return scroll
}

// viewLimit is the most text a body view lays out. A longer body shows its
// beginning, and Copy and Save still take all of it.
const viewLimit = 1 << 20

// clipText cuts text to viewLimit at the end of a line, and says so in the
// note, joined to any it had.
func clipText(text, note string) (string, string) {
	if len(text) <= viewLimit {
		return text, note
	}
	cut := strings.LastIndexByte(text[:viewLimit], '\n')
	if cut < viewLimit/2 {
		cut = viewLimit
		for cut > 0 && !utf8.RuneStart(text[cut]) {
			cut--
		}
	}
	more := locales.CommonShowingFirst(httpx.FormatBytes(int64(cut)), httpx.FormatBytes(int64(len(text))))
	if note != "" {
		more = note + " " + more
	}
	return text[:cut], more
}

// codeView is a selectable monospaced document on the inspector's paper.
func codeView(c *ui.Context, pal *uitokens.Palette, text string, note string) ui.Element {
	text, note = clipText(text, note)
	scroll := ui.ScrollBoth(c).Grow(1).Background(pal.CodeBg)
	scroll.Children(func() {
		ui.Column(c).Padding(10, 14, 12, 14).Gap(0).Children(func() {
			if note != "" {
				ui.Row(c).Height(24).Padding(0, 0, 2, 0).Children(func() {
					ui.Text(c, note).FontSize(10.5).FontWeight(600).TextColor(pal.Warning)
				})
			}
			if strings.TrimSpace(text) == "" {
				ui.Text(c, locales.CommonEmptyBody()).Font("monospace").FontSize(11.5).
					TextColor(pal.TextFaint).Padding(2, 0)
				return
			}
			if spans, ok := jsonSpans(text, pal); ok {
				ui.RichText(c, spans...).Font("monospace").FontSize(11.5).
					FixedLineHeight(18).Selectable().NoWrap()
				return
			}
			ui.Text(c, text).Font("monospace").FontSize(11.5).FixedLineHeight(18).
				TextColor(pal.CodeText).Selectable().NoWrap()
		})
	})
	return scroll
}

// highlightLimit keeps the span list small enough to lay out smoothly; a
// larger body is shown in one plain run.
const highlightLimit = 192 << 10

// jsonSpans colors a valid JSON document the way an editor does: keys and
// strings, numbers and literals apart from the punctuation around them.
// Everything else is left in the punctuation color.
func jsonSpans(text string, pal *uitokens.Palette) ([]ui.Span, bool) {
	if len(text) > highlightLimit || !strings.ContainsAny(text, "{[]") || !json.Valid([]byte(text)) {
		return nil, false
	}
	spans := make([]ui.Span, 0, 64)
	plain := 0
	flush := func(end int) {
		if end > plain {
			spans = append(spans, ui.Span{Text: text[plain:end], Color: pal.CodePunct})
		}
	}
	add := func(start, end int, col ui.Color) {
		flush(start)
		spans = append(spans, ui.Span{Text: text[start:end], Color: col})
		plain = end
	}
	for i := 0; i < len(text); {
		switch c := text[i]; {
		case c == '"':
			end := i + 1
			for end < len(text) {
				if text[end] == '\\' {
					end += 2
					continue
				}
				if text[end] == '"' {
					end++
					break
				}
				end++
			}
			if end > len(text) {
				end = len(text)
			}
			col := pal.CodeString
			j := end
			for j < len(text) && (text[j] == ' ' || text[j] == '\t' || text[j] == '\n' || text[j] == '\r') {
				j++
			}
			if j < len(text) && text[j] == ':' {
				col = pal.CodeKey
			}
			add(i, end, col)
			i = end
		case c == '-' || (c >= '0' && c <= '9'):
			end := i + 1
			for end < len(text) {
				d := text[end]
				if (d >= '0' && d <= '9') || d == '.' || d == 'e' || d == 'E' || d == '+' || d == '-' {
					end++
					continue
				}
				break
			}
			add(i, end, pal.CodeNumber)
			i = end
		case strings.HasPrefix(text[i:], "true"):
			add(i, i+4, pal.CodeLiteral)
			i += 4
		case strings.HasPrefix(text[i:], "false"):
			add(i, i+5, pal.CodeLiteral)
			i += 5
		case strings.HasPrefix(text[i:], "null"):
			add(i, i+4, pal.CodeLiteral)
			i += 4
		default:
			i++
		}
	}
	flush(len(text))
	return spans, true
}

// timingPhase is one stretch of a request's time, placed where it began.
type timingPhase struct {
	name       string
	start, dur time.Duration
	col        ui.Color
}

// timingPhases lays a request's phases end to end, as they happen: each
// starts where the one before it ended, so the bars read as a waterfall and
// the download begins at the first byte.
func timingPhases(res *httpx.Result, pal *uitokens.Palette) []timingPhase {
	var at time.Duration
	next := func(name string, d time.Duration, col ui.Color) timingPhase {
		p := timingPhase{name: name, start: at, dur: max(d, 0), col: col}
		at += p.dur
		return p
	}
	return []timingPhase{
		next("DNS", res.DNS, pal.Info),
		next(locales.TimingConnect(), res.Connect, pal.Accent),
		next("TLS", res.TLS, uitokens.MethodColor(pal, "PATCH")),
		next(locales.TimingWaiting(), res.Waiting(), pal.Warning),
		next(locales.TimingDownload(), res.Download(), pal.Success),
	}
}

// timingRows draws where a request's time went, as a waterfall against the
// total.
func timingRows(c *ui.Context, pal *uitokens.Palette, res *httpx.Result) ui.Element {
	total := float32(res.Duration)
	if total <= 0 {
		total = 1
	}
	pct := func(d time.Duration) float32 { return min(float32(d)/total*100, 100) }
	bar := func(name string, start, dur time.Duration, col ui.Color, value string, strong bool) {
		ui.Row(c).Height(24).Gap(10).AlignItems(ui.Center).Children(func() {
			label := ui.Text(c, name).Width(78).Shrink(0).FontSize(11.5).TextColor(pal.TextMuted)
			if strong {
				label.FontWeight(650).TextColor(pal.Text)
			}
			ui.Box(c).Grow(1).Height(7).Radius(4).Background(pal.Chip).Clip().Children(func() {
				if dur > 0 {
					ui.Box(c).Absolute().Top(0).Bottom(0).LeftPercent(pct(start)).
						WidthPercent(pct(dur)).MinWidth(2).Radius(4).Background(col)
				}
			})
			ui.Text(c, value).Width(72).Shrink(0).FontSize(11).Font("monospace").
				TextColor(pal.CodeText).TextAlign(ui.End)
		})
	}
	// A connection that was open already spends nothing before the request;
	// a new one to an address skips only DNS, and plain HTTP only TLS.
	reused := res.DNS == 0 && res.Connect == 0 && res.TLS == 0
	col := ui.Column(c).Fill().Padding(14, 16).Gap(10)
	col.Children(func() {
		for i, p := range timingPhases(res, pal) {
			value := httpx.FormatDuration(p.dur)
			if reused && i < 3 {
				value = locales.TimingReused()
			}
			bar(p.name, p.start, p.dur, p.col, value, false)
		}
		ui.Box(c).Height(1).Shrink(0).Background(pal.Divider).Margin(2, 0)
		bar(locales.TimingTotal(), 0, res.Duration, pal.TextMuted, httpx.FormatDuration(res.Duration), true)
		ui.Spacer(c)
		note := locales.TimingNote()
		if reused {
			note = locales.TimingReusedNote()
		}
		ui.Text(c, note).FontSize(11).TextColor(pal.TextFaint).LineHeight(1.4).MaxWidth(460)
	})
	return col
}

// methodList is the verbs the composer's selector offers.
var methodList = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}
