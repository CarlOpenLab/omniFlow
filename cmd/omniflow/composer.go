package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/CarlOpenLab/omniflow/internal/httpx"
	"github.com/CarlOpenLab/omniflow/internal/model"
	"github.com/CarlOpenLab/omniflow/internal/uitokens"
	"github.com/CarlOpenLab/omniflow/locales"
)

// composer is the active half of the app: a request on top, its answer
// below, and the split between them where a person expects it.
func (a *app) composer(c *ui.Context, pal *uitokens.Palette) {
	col := ui.Column(c).Fill().MinHeight(0)
	col.Children(func() {
		a.urlBar(c, pal)
		ui.SplitVerticalPercent(c, &a.composerSplit, func() {
			a.requestEditor(c, pal)
		}, func() {
			a.responsePane(c, pal)
		}).Grow(1).MinHeight(0)
	})
}

func (a *app) urlBar(c *ui.Context, pal *uitokens.Palette) {
	row := ui.Row(c).Height(56).Shrink(0).Padding(0, 12, 0, 12).Gap(8).
		BorderWidth(0, 0, 1, 0).BorderColor(pal.Divider)
	row.Children(func() {
		// Method: a token that opens the verb list.
		method := ui.ButtonBase(c.Key("method")).Width(108).Height(34).Shrink(0).Radius(8).
			Padding(0, 10).Gap(6).Label(locales.ComposerMethodLabel(a.req.Method)).
			Tooltip(locales.ComposerMethodTooltip())
		method.Background(pal.Chip)
		if method.Hovered() {
			method.Background(pal.WashSelected)
		}
		method.BorderWidth(0, 0, 0, 0)
		method.Children(func() {
			col := uitokens.MethodColor(pal, a.req.Method)
			ui.Text(c, a.req.Method).FontSize(11).FontWeight(750).LetterSpacing(0.3).
				TextColor(col).Grow(1)
			ui.Icon(c, iconChevron).Size(12, 12).TextColor(pal.TextMuted)
		})
		if method.Clicked() {
			a.methodOpen = !a.methodOpen
		}
		a.methodMenu(c, pal, method)

		// URL
		in := ui.TextInput(c.Key("url"), &a.req.URL).Height(34).Grow(1).MinWidth(0).
			FontSize(12.5).Padding(0, 12).Radius(8).
			Background(pal.Chip).Border(1, pal.Edge).Label(locales.ComposerRequestURL()).
			Placeholder("https://api.example.com/v1/things")
		in = in.Bind(&a.urlFocus)
		if in.Focused() {
			in.Border(1, pal.Accent)
		}
		in.TextRanges(a.varRanges(pal, a.req.URL)...)
		if tip := a.varTip(a.req.URL); tip != "" {
			in.Tooltip(tip)
		}
		// A curl command is pasted here far more often than a URL; open it as
		// the request it describes instead of inserting it as text.
		in.OnPaste(func(text string) bool {
			if !httpx.LooksLikeCurl(text) {
				return false
			}
			a.pendingCurl = text
			return true
		})
		if in.Submitted() {
			a.send()
		}

		// Send. While a request is out it stays put and says so; Cancel is in
		// the response pane, away from where a second click on Send lands.
		send := ui.PrimaryButton(c.Key("send"), "").Height(34).Shrink(0).Padding(0, 16).
			Radius(8).Gap(6).Label(locales.ComposerSend()).Tooltip(locales.ComposerSendTooltip()).Disabled(a.sending)
		send.Children(func() {
			if a.sending {
				ui.Spinner(c).Size(13, 13)
				ui.Text(c, locales.ComposerSending()).FontSize(12).FontWeight(650)
			} else {
				ui.Icon(c, iconSend).Size(13, 13)
				ui.Text(c, locales.ComposerSend()).FontSize(12).FontWeight(650)
			}
		})
		if send.Clicked() {
			a.send()
		}
	})
}

// methodMenu is the verb list, a floating panel over the method token.
// Popovers are one of the few layers painted solid, so the list keeps its
// contrast over the frost.
func (a *app) methodMenu(c *ui.Context, pal *uitokens.Palette, anchor ui.Element) {
	ui.PopoverBase(c, anchor, &a.methodOpen, func(panel ui.Element) {
		panel.Width(156).Padding(5).Radius(11).Background(pal.Popover).
			Border(1, pal.EdgeHigh).Shadow(0, 12, 32, 0, pal.FloatShadow)
		panel.Children(func() {
			for _, m := range methodList {
				name := m
				selected := a.req.Method == name
				item := ui.ButtonBase(c.Key("verb:"+name)).Height(28).Padding(0, 8).
					Radius(6).Gap(8).Label(name)
				if selected {
					item.Background(pal.AccentSoft)
				} else if item.Hovered() {
					item.Background(pal.RowHover)
				}
				item.Children(func() {
					ui.Text(c, name).FontSize(11).FontWeight(700).
						TextColor(uitokens.MethodColor(pal, name)).Grow(1)
					if selected {
						ui.Icon(c, iconCheck).Size(12, 12).TextColor(pal.Accent)
					}
				})
				if item.Clicked() {
					a.req.Method = name
					a.methodOpen = false
				}
			}
		})
	})
}

func (a *app) requestEditor(c *ui.Context, pal *uitokens.Palette) {
	pane := ui.Column(c).Fill().MinHeight(0)
	pane.Children(func() {
		labels := []string{
			countLabel(locales.ComposerTabParams(), enabledCount(a.req.Params)),
			countLabel(locales.ComposerTabHeaders(), enabledCount(a.req.Headers)),
			locales.ComposerTabBody(),
			locales.ComposerTabAuth(),
		}
		if a.req.BodyKind != model.BodyNone {
			labels[2] = locales.ComposerBodyKind(locales.ComposerTabBody(), bodyKindLabel(a.req.BodyKind))
		}
		if a.req.AuthType != model.AuthNone {
			labels[3] = locales.ComposerAuthKind(locales.ComposerTabAuth(), authLabel(a.req.AuthType))
		}
		lineTabs(c, pal, &a.reqTab, labels)
		ui.Box(c).Grow(1).MinHeight(0).Children(func() {
			switch a.reqTab {
			case 0:
				a.paramsTab(c, pal)
			case 1:
				a.headersTab(c, pal)
			case 2:
				a.bodyTab(c, pal)
			default:
				a.authTab(c, pal)
			}
		})
	})
}

// countLabel is a tab's name with how many rows it sends, when it sends any.
func countLabel(name string, n int) string {
	if n == 0 {
		return name
	}
	return locales.ComposerTabCount(name, n)
}

func bodyKindLabel(kind string) string {
	switch kind {
	case model.BodyJSON:
		return locales.ComposerKindJSON()
	case model.BodyForm:
		return locales.ComposerKindForm()
	}
	return locales.ComposerKindText()
}

func authLabel(kind string) string {
	if kind == model.AuthBasic {
		return "Basic"
	}
	return "Bearer"
}

func enabledCount(rows []model.KV) int {
	n := 0
	for _, r := range rows {
		if r.Enabled && strings.TrimSpace(r.Key) != "" {
			n++
		}
	}
	return n
}

// tabCaption is the line over a tab's table: what it holds, and how it is
// sent.
func tabCaption(c *ui.Context, pal *uitokens.Palette, title, how string, extra func()) {
	ui.Row(c).Gap(6).Children(func() {
		ui.Text(c, title).FontSize(11.5).FontWeight(650).TextColor(pal.TextMuted)
		ui.Text(c, how).FontSize(10.5).TextColor(pal.TextFaint)
		ui.Spacer(c)
		if extra != nil {
			extra()
		}
	})
}

func (a *app) paramsTab(c *ui.Context, pal *uitokens.Palette) {
	ui.Scroll(c).Grow(1).Children(func() {
		col := ui.Column(c).Padding(12, 14).Gap(8)
		col.Children(func() {
			tabCaption(c, pal, locales.ComposerQueryParams(), locales.ComposerQueryParamsHint(), nil)
			kvEditor(c, pal, &a.req.Params, locales.ComposerParamPlaceholder(), locales.CommonValuePlaceholder(), locales.ComposerNoParams())
		})
	})
}

func (a *app) headersTab(c *ui.Context, pal *uitokens.Palette) {
	ui.Scroll(c).Grow(1).Children(func() {
		col := ui.Column(c).Padding(12, 14).Gap(8)
		col.Children(func() {
			tabCaption(c, pal, locales.ComposerRequestHeaders(), locales.ComposerHeadersHint(), func() {
				if a.req.AuthType != model.AuthNone {
					ui.Text(c, locales.ComposerAuthAddedByTab()).FontSize(10.5).
						TextColor(pal.TextFaint)
				}
			})
			kvEditor(c, pal, &a.req.Headers, locales.CommonHeaderPlaceholder(), locales.CommonValuePlaceholder(), locales.ComposerNoHeaders())
		})
	})
}

func (a *app) bodyTab(c *ui.Context, pal *uitokens.Palette) {
	kinds := []string{model.BodyNone, model.BodyJSON, model.BodyText, model.BodyForm}
	idx := indexOf(kinds, a.req.BodyKind)
	if idx < 0 {
		idx = 0
	}
	segments := []string{locales.ComposerBodyNone(), locales.ComposerBodyJSON(), locales.ComposerBodyText(), locales.ComposerBodyForm()}
	col := ui.Column(c).Fill().MinHeight(0).Padding(12, 14).Gap(8)
	col.Children(func() {
		row := ui.Row(c).Height(30).Gap(10)
		row.Children(func() {
			segmented(c, pal, &idx, segments)
			ui.Spacer(c)
			switch a.req.BodyKind {
			case model.BodyJSON:
				a.jsonState(c, pal)
				if b := iconAction(c, pal, iconSpark, locales.ComposerFormat()); b.Clicked() {
					a.req.Body = formatJSON(a.req.Body)
				}
			case model.BodyForm:
				ui.Text(c, locales.ComposerFieldCount(enabledCount(a.req.Form))).FontSize(10.5).TextColor(pal.TextFaint)
			}
			if a.req.BodyKind == model.BodyJSON || a.req.BodyKind == model.BodyText {
				ui.Text(c, locales.ComposerCharCount(len([]rune(a.req.Body)))).
					FontSize(10.5).TextColor(pal.TextFaint)
			}
		})
		if idx != indexOf(kinds, a.req.BodyKind) {
			a.req.BodyKind = kinds[idx]
		}
		if a.req.BodyKind == model.BodyNone {
			ui.Box(c).Grow(1).MinHeight(0).Children(func() {
				emptyState(c, pal, iconCode, locales.ComposerNoBody(), locales.ComposerNoBodyHint())
			})
			return
		}
		hint := ""
		switch a.req.BodyKind {
		case model.BodyJSON:
			hint = locales.ComposerJSONHint()
		case model.BodyForm:
			hint = locales.ComposerFormHint()
		default:
			hint = locales.ComposerTextHint()
		}
		hintColor := pal.TextFaint
		if a.req.Method == "GET" || a.req.Method == "HEAD" {
			hint = locales.ComposerNoBodyMethodHint()
			hintColor = pal.Warning
		}
		ui.Text(c, hint).FontSize(10.5).TextColor(hintColor)
		if a.req.BodyKind == model.BodyForm {
			if len(a.req.Form) == 0 {
				a.req.Form = []model.KV{model.NewKV()}
			}
			ui.Scroll(c).Grow(1).MinHeight(0).Children(func() {
				kvEditor(c, pal, &a.req.Form, locales.CommonFieldPlaceholder(), locales.CommonValuePlaceholder(), locales.ComposerNoFormFields())
			})
			return
		}
		ui.TextArea(c.Key("body"), &a.req.Body).Grow(1).MinHeight(0).
			Font("monospace").FontSize(11.5).Padding(8, 10).Radius(8).
			Background(pal.CodeBg).Border(1, pal.CodeEdge).Placeholder("{\n  \"hello\": \"world\"\n}")
	})
}

// jsonCheck is the last JSON body checked, so the check runs as the body
// changes rather than in every frame. Where the parser stopped is kept as
// numbers, so the message can be written in the language on screen now.
type jsonCheck struct {
	body   string
	ok     bool
	line   int
	column int
}

// jsonState says whether the JSON body parses, and where it stops if not.
func (a *app) jsonState(c *ui.Context, pal *uitokens.Palette) {
	if strings.TrimSpace(a.req.Body) == "" {
		return
	}
	if a.jsonChecked.body != a.req.Body {
		check := checkJSON(a.req.Body)
		check.body = a.req.Body
		a.jsonChecked = check
	}
	col, icon := pal.Success, iconValid
	if !a.jsonChecked.ok {
		col, icon = pal.Danger, iconWarning
	}
	msg := locales.ComposerJSONValid()
	if !a.jsonChecked.ok {
		msg = locales.ComposerJSONInvalid()
		if a.jsonChecked.line > 0 {
			msg = locales.ComposerJSONInvalidAt(a.jsonChecked.line, a.jsonChecked.column)
		}
	}
	ui.Row(c).Gap(5).Shrink(0).Children(func() {
		ui.Icon(c, icon).Size(12, 12).TextColor(col)
		ui.Text(c, msg).FontSize(10.5).FontWeight(600).TextColor(col)
	})
}

// checkJSON reports whether s is one JSON document, and if not, where the
// parser gave up, as a line and column a person can find.
func checkJSON(s string) jsonCheck {
	if json.Valid([]byte(s)) {
		return jsonCheck{ok: true}
	}
	var v any
	err := json.Unmarshal([]byte(s), &v)
	var syntax *json.SyntaxError
	if errors.As(err, &syntax) {
		line, col := lineCol(s, int(syntax.Offset))
		return jsonCheck{line: line, column: col}
	}
	return jsonCheck{}
}

// lineCol turns a byte offset into a 1-based line and column of runes.
func lineCol(s string, offset int) (int, int) {
	offset = min(max(offset, 0), len(s))
	before := s[:offset]
	line := strings.Count(before, "\n") + 1
	start := strings.LastIndexByte(before, '\n') + 1
	return line, len([]rune(before[start:])) + 1
}

func (a *app) authTab(c *ui.Context, pal *uitokens.Palette) {
	types := []string{model.AuthNone, model.AuthBearer, model.AuthBasic}
	idx := indexOf(types, a.req.AuthType)
	if idx < 0 {
		idx = 0
	}
	col := ui.Column(c).Padding(14, 16).Gap(12)
	col.Children(func() {
		segmented(c, pal, &idx, []string{locales.ComposerAuthNone(), "Bearer", "Basic"})
		if idx != indexOf(types, a.req.AuthType) {
			a.req.AuthType = types[idx]
		}
		switch a.req.AuthType {
		case model.AuthBearer:
			ui.Text(c, locales.ComposerBearerHint()).
				FontSize(10.5).TextColor(pal.TextFaint)
			a.labelInput(c, pal, locales.ComposerToken(), &a.req.Token, false)
		case model.AuthBasic:
			ui.Text(c, locales.ComposerBasicHint()).
				FontSize(10.5).TextColor(pal.TextFaint)
			a.labelInput(c, pal, locales.ComposerUsername(), &a.req.User, false)
			a.labelInput(c, pal, locales.ComposerPassword(), &a.req.Pass, true)
		default:
			ui.Text(c, locales.ComposerNoAuth()).FontSize(11).TextColor(pal.TextFaint)
		}
	})
}

func (a *app) labelInput(c *ui.Context, pal *uitokens.Palette, label string, value *string, secret bool) {
	ui.Row(c).Gap(10).Children(func() {
		ui.Text(c, label).Width(80).FontSize(11.5).TextColor(pal.TextMuted)
		in := ui.TextInput(c.Key(label), value).Height(30).Grow(1).FontSize(11.5).
			Font("monospace").Padding(0, 9).Background(pal.Chip).Border(1, pal.Edge)
		if tip := a.varTip(*value); tip != "" {
			in.Tooltip(tip)
		}
		if secret {
			in.Password()
		}
	})
}

func indexOf(list []string, v string) int {
	for i, s := range list {
		if s == v {
			return i
		}
	}
	return -1
}

func formatJSON(s string) string {
	var buf bytes.Buffer
	if err := json.Indent(&buf, []byte(s), "", "  "); err != nil {
		return s
	}
	return buf.String()
}

// responsePane is the answer, or the state of waiting for one.
func (a *app) responsePane(c *ui.Context, pal *uitokens.Palette) {
	pane := ui.Column(c).Fill().MinHeight(0)
	pane.Children(func() {
		a.responseHeader(c, pal)
		if a.sending {
			ui.Box(c).Grow(1).MinHeight(0).Children(func() {
				ui.Column(c).Fill().Center().Gap(10).Children(func() {
					ui.Spinner(c).Size(22, 22).TextColor(pal.Accent)
					ui.Text(c, locales.ComposerWaiting()).FontSize(12).TextColor(pal.TextMuted)
					cancel := ui.ButtonBase(c.Key("cancel-send")).Height(28).Padding(0, 10).
						Radius(7).Gap(6).Label(locales.CommonCancel()).
						Tooltip(locales.ComposerCancelTooltip())
					if cancel.Hovered() {
						cancel.Background(pal.RowHover)
					}
					cancel.Children(func() {
						ui.Text(c, locales.CommonCancel()).FontSize(11.5).TextColor(pal.TextMuted)
						kbd(c, pal, "⌘.")
					})
					if cancel.Clicked() {
						a.cancelSend()
					}
				})
			})
			return
		}
		if a.resp == nil {
			ui.Box(c).Grow(1).MinHeight(0).Children(func() {
				msg := a.respProblemText()
				switch {
				case msg == "":
					emptyState(c, pal, iconSend, locales.ComposerNothingSent(), locales.ComposerNothingSentHint())
				case a.respProblem == respCanceled:
					a.canceledState(c, pal)
				default:
					a.errorState(c, pal, msg)
				}
			})
			return
		}
		resp := a.resp
		if resp.Err != "" && resp.Status == 0 {
			ui.Box(c).Grow(1).MinHeight(0).Children(func() {
				a.errorState(c, pal, resp.Err)
			})
			return
		}
		if resp.Err != "" {
			ui.Row(c).Height(28).Shrink(0).Padding(0, 12).Gap(6).Background(pal.Warning.Alpha(0.1)).
				Children(func() {
					ui.Icon(c, iconWarning).Size(12, 12).TextColor(pal.Warning)
					ui.Text(c, resp.Err).FontSize(10.5).TextColor(pal.Warning).SingleLine().Ellipsis("…")
				})
		}
		prettyText, isJSON := httpx.Pretty(resp.Body)
		labels := []string{locales.ComposerTabBody(), countLabel(locales.ComposerTabHeaders(), len(resp.Headers)), locales.ComposerTiming()}
		lineTabs(c, pal, &a.respTab, labels)
		ui.Box(c).Grow(1).MinHeight(0).Children(func() {
			switch a.respTab {
			case 0:
				text := string(resp.Body)
				if isJSON && a.pretty {
					text = prettyText
				}
				note := ""
				if resp.Truncated {
					note = locales.ComposerTruncated()
				}
				a.bodyView(c, pal, fmt.Sprintf("resp:%p", resp), a.req.URL, resp.Body,
					contentTypeOf(resp.Headers), text, note)
			case 1:
				headerList(c, pal, resp.Headers, locales.ComposerNoResponseHeaders())
			default:
				timingRows(c, pal, resp)
			}
		})
	})
}

// contentTypeOf is a response's Content-Type, "" when it names none.
func contentTypeOf(headers []httpx.Header) string {
	for _, h := range headers {
		if strings.EqualFold(h.Key, "Content-Type") {
			return h.Value
		}
	}
	return ""
}

func (a *app) responseHeader(c *ui.Context, pal *uitokens.Palette) {
	row := ui.Row(c).Height(40).Shrink(0).Padding(0, 12).Gap(8).
		BorderWidth(0, 0, 1, 0).BorderColor(pal.Divider)
	row.Children(func() {
		ui.Text(c, locales.ComposerResponse()).FontSize(11.5).FontWeight(650).TextColor(pal.TextMuted)
		resp := a.resp
		if resp != nil && resp.Status > 0 {
			statusBadge(c, pal, resp.Status)
			ui.Text(c, resp.StatusText).FontSize(11).FontWeight(600).
				TextColor(uitokens.StatusColor(pal, resp.Status)).SingleLine()
			ui.Text(c, resp.Proto).FontSize(10).TextColor(pal.TextFaint)
			ui.Box(c).Width(1).Height(14).Background(pal.Divider)
			ui.Text(c, httpx.FormatDuration(resp.Duration)).FontSize(10.5).Font("monospace").
				TextColor(pal.TextMuted).Tooltip(locales.ComposerTotalTime())
			ui.Text(c, httpx.FormatBytes(resp.Size)).FontSize(10.5).Font("monospace").
				TextColor(pal.TextMuted).Tooltip(locales.ComposerBodySize())
		}
		ui.Spacer(c)
		if resp != nil && resp.Status > 0 && a.respTab == 0 {
			_, isJSON := httpx.Pretty(resp.Body)
			if isJSON {
				pretty := ui.ButtonBase(c.Key("pretty")).Height(24).Padding(0, 8).Radius(6).Gap(4).
					Label(locales.ComposerPrettyJSON()).Tooltip(locales.ComposerPrettyJSONHint())
				if a.pretty {
					pretty.Background(pal.AccentSoft)
				} else if pretty.Hovered() {
					pretty.Background(pal.RowHover)
				}
				pretty.Children(func() {
					col := pal.TextMuted
					if a.pretty {
						col = pal.Accent
					}
					ui.Icon(c, iconCode).Size(11, 11).TextColor(col)
					ui.Text(c, locales.ComposerPretty()).FontSize(10.5).FontWeight(600).TextColor(col)
				})
				if pretty.Clicked() {
					a.pretty = !a.pretty
				}
			}
			if shapeOf(resp.Body, contentTypeOf(resp.Headers)) == shapeText {
				if b := iconButton(c, pal, iconCopy, locales.ComposerCopyResponse(), 24); b.Clicked() {
					text := string(resp.Body)
					if prettyText, ok := httpx.Pretty(resp.Body); ok && a.pretty {
						text = prettyText
					}
					a.copyText(c, locales.ComposerResponseCopied(), text)
				}
			}
		}
	})
}

func (a *app) errorState(c *ui.Context, pal *uitokens.Palette, msg string) {
	ui.Column(c).Fill().Center().Gap(10).Padding(24).Children(func() {
		ui.Box(c).Size(48, 48).Radius(13).Center().Background(pal.Danger.Alpha(0.1)).Children(func() {
			ui.Icon(c, iconWarning).Size(22, 22).TextColor(pal.Danger)
		})
		ui.Text(c, locales.ComposerRequestFailed()).FontSize(13.5).FontWeight(650).
			TextColor(pal.Text)
		ui.Text(c, msg).FontSize(11.5).TextColor(pal.TextMuted).TextAlign(ui.Center).
			MaxWidth(440).LineHeight(1.4).Selectable()
		if quietButton(c, pal, "retry", locales.ComposerTryAgain()).Clicked() {
			a.send()
		}
	})
}

// canceledState is the pane after a request was called off: nothing went
// wrong, so it says so quietly.
func (a *app) canceledState(c *ui.Context, pal *uitokens.Palette) {
	ui.Column(c).Fill().Center().Gap(10).Padding(24).Children(func() {
		ui.Box(c).Size(48, 48).Radius(13).Center().Background(pal.Chip).Children(func() {
			ui.Icon(c, iconCancel).Size(22, 22).TextColor(pal.TextMuted)
		})
		ui.Text(c, locales.ComposerRequestCanceled()).FontSize(13.5).FontWeight(650).TextColor(pal.Text)
		ui.Text(c, locales.ComposerCanceledHint()).
			FontSize(11.5).TextColor(pal.TextMuted).TextAlign(ui.Center).MaxWidth(440).LineHeight(1.4)
		if quietButton(c, pal, "send-again", locales.ComposerSendAgain()).Clicked() {
			a.send()
		}
	})
}
