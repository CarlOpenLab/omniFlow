package main

// Rules change traffic as it passes through the proxy: a Map Remote sends a
// request to another origin, and a header rule adds, sets or removes one
// request or response header. The rules are made from a captured exchange or
// from Settings → Rules, and the proxy reads them for every request.

import (
	"net/url"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/CarlOpenLab/omniflow/internal/model"
	"github.com/CarlOpenLab/omniflow/internal/proxy"
	"github.com/CarlOpenLab/omniflow/internal/uitokens"
	"github.com/CarlOpenLab/omniflow/locales"
)

func (a *app) applyRules() {
	if a.prx == nil {
		return
	}
	rules := make([]proxy.Rule, 0, len(a.rules))
	for _, r := range a.rules {
		if !r.Enabled {
			continue
		}
		rules = append(rules, proxy.Rule{
			Kind:       r.Kind,
			Method:     r.Method,
			Host:       r.Host,
			PathPrefix: r.PathPrefix,
			Target:     r.Target,
			Header:     r.Header,
			Remove:     r.Remove,
		})
	}
	a.prx.SetRules(rules)
}

func (a *app) persistRules() {
	if a.store != nil {
		_ = a.store.SetRules(a.rules)
	}
}

// limitRules keeps the list bounded; rules are small, so only the oldest is
// dropped.
func (a *app) limitRules() {
	const maxRules = 200
	for len(a.rules) > maxRules {
		a.rules = a.rules[:len(a.rules)-1]
	}
}

// openNewRule opens the rule dialog with kind's fields and no match.
func (a *app) openNewRule(kind string) {
	if kind == "" {
		kind = model.RuleMapRemote
	}
	a.ruleOpen = true
	a.ruleID = ""
	a.ruleKind = kind
	a.ruleName = ""
	a.ruleMethod = ""
	a.ruleHost = ""
	a.rulePath = "/"
	a.ruleTarget = ""
	a.ruleHeader = ""
	a.ruleValue = ""
	a.ruleRemove = false
	a.ruleErr = ""
}

// openRuleFromFlow opens the dialog with the captured request's match, so the
// rule starts on the thing on screen.
func (a *app) openRuleFromFlow(kind string, f proxy.Flow) {
	a.openNewRule(kind)
	a.ruleMethod = f.Method
	a.ruleHost = f.Host
	path := f.Path
	if i := strings.IndexByte(path, '?'); i >= 0 {
		path = path[:i]
	}
	if path == "" {
		path = "/"
	}
	a.rulePath = path
}

func (a *app) editRule(r model.ProxyRule) {
	a.ruleOpen = true
	a.ruleID = r.ID
	a.ruleKind = r.Kind
	a.ruleName = r.Name
	a.ruleMethod = r.Method
	a.ruleHost = r.Host
	a.rulePath = r.PathPrefix
	if a.rulePath == "" {
		a.rulePath = "/"
	}
	a.ruleTarget, a.ruleHeader, a.ruleValue = "", r.Header, ""
	if r.Kind == model.RuleMapRemote {
		a.ruleTarget = r.Target
	} else {
		a.ruleValue = r.Target
	}
	a.ruleRemove = r.Remove
	a.ruleErr = ""
}

// saveRuleDialog validates the form, writes the rule and hands it to the
// proxy. The dialog stays open on a bad field, with the reason under it.
func (a *app) saveRuleDialog() {
	kind := a.ruleKind
	if kind == "" {
		kind = model.RuleMapRemote
	}
	host := strings.TrimSpace(a.ruleHost)
	path := strings.TrimSpace(a.rulePath)
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if host == "" {
		a.ruleErr = locales.RuleNeedsHost()
		return
	}
	target := strings.TrimSpace(a.ruleTarget)
	header := strings.TrimSpace(a.ruleHeader)
	value := a.ruleValue
	if kind == model.RuleMapRemote {
		u, err := url.Parse(target)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			a.ruleErr = locales.RuleNeedsTarget()
			return
		}
	} else {
		if header == "" {
			a.ruleErr = locales.RuleNeedsHeader()
			return
		}
		if protectedRuleHeader(header) {
			a.ruleErr = locales.RuleProtectedHeader()
			return
		}
		if a.ruleRemove {
			value = ""
		}
	}

	name := strings.TrimSpace(a.ruleName)
	if name == "" {
		name = defaultRuleName(kind, a.ruleMethod, host, path, target, header, a.ruleRemove)
	}
	name = shortText(name, 80)
	rule := model.ProxyRule{
		Enabled:    true,
		Name:       name,
		Kind:       kind,
		Method:     a.ruleMethod,
		Host:       host,
		PathPrefix: path,
		Target:     target,
		Header:     header,
		Remove:     a.ruleRemove,
	}
	if kind != model.RuleMapRemote {
		rule.Target = value
	}

	now := time.Now()
	if a.ruleID != "" {
		found := false
		for i := range a.rules {
			if a.rules[i].ID != a.ruleID {
				continue
			}
			rule.ID, rule.Enabled, rule.CreatedAt = a.rules[i].ID, a.rules[i].Enabled, a.rules[i].CreatedAt
			a.rules[i] = rule
			found = true
			break
		}
		if !found {
			rule.ID, rule.CreatedAt = model.NewID("rule"), now
			a.rules = append([]model.ProxyRule{rule}, a.rules...)
		}
	} else {
		rule.ID, rule.CreatedAt = model.NewID("rule"), now
		a.rules = append([]model.ProxyRule{rule}, a.rules...)
	}

	a.limitRules()
	a.persistRules()
	a.applyRules()
	a.ruleOpen = false
	a.ruleErr = ""
	a.setNotice(locales.NoticeRuleSaved(name))
}

func (a *app) deleteRule(id string) {
	for i := range a.rules {
		if a.rules[i].ID != id {
			continue
		}
		name := a.rules[i].Name
		a.rules = append(a.rules[:i], a.rules[i+1:]...)
		a.persistRules()
		a.applyRules()
		a.setNotice(locales.NoticeRuleDeleted(name))
		return
	}
}

func (a *app) setRuleEnabled(id string, enabled bool) {
	for i := range a.rules {
		if a.rules[i].ID != id {
			continue
		}
		a.rules[i].Enabled = enabled
		a.persistRules()
		a.applyRules()
		return
	}
}

func protectedRuleHeader(name string) bool {
	switch strings.ToLower(name) {
	case "content-length", "transfer-encoding", "connection", "content-encoding",
		"proxy-connection", "keep-alive", "te", "trailer", "upgrade":
		return true
	}
	return false
}

func defaultRuleName(kind, method, host, path, target, header string, remove bool) string {
	scope := host + path
	if method != "" {
		scope = method + " " + scope
	}
	switch kind {
	case model.RuleMapRemote:
		return "Map " + scope + " → " + target
	case model.RuleRequestHeader:
		if remove {
			return "Remove request " + header + " on " + scope
		}
		return "Request " + header + " on " + scope
	default:
		if remove {
			return "Remove response " + header + " on " + scope
		}
		return "Response " + header + " on " + scope
	}
}

func ruleKindLabel(kind string) string {
	switch kind {
	case model.RuleRequestHeader:
		return locales.RuleRequestHeader()
	case model.RuleResponseHeader:
		return locales.RuleResponseHeader()
	}
	return locales.RuleMapRemote()
}

func ruleKindFromLabel(label string) string {
	switch label {
	case locales.RuleRequestHeader():
		return model.RuleRequestHeader
	case locales.RuleResponseHeader():
		return model.RuleResponseHeader
	}
	return model.RuleMapRemote
}

func ruleKindOptions() []string {
	return []string{locales.RuleMapRemote(), locales.RuleRequestHeader(), locales.RuleResponseHeader()}
}

func ruleMethodOptions() []string {
	return []string{
		locales.RuleAnyMethod(), "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS",
	}
}

func ruleMethodLabel(method string) string {
	if method == "" {
		return locales.RuleAnyMethod()
	}
	return method
}

func ruleMethodFromLabel(label string) string {
	if label == locales.RuleAnyMethod() {
		return ""
	}
	return label
}

func ruleSubtitle(r model.ProxyRule) string {
	scope := r.Method
	if scope == "" {
		scope = locales.RuleAnyMethod()
	}
	scope += " " + r.Host + r.PathPrefix
	switch r.Kind {
	case model.RuleMapRemote:
		return scope + " → " + r.Target
	case model.RuleRequestHeader, model.RuleResponseHeader:
		if r.Remove {
			return scope + " · " + locales.RuleRemoveHeader(r.Header)
		}
		return scope + " · " + r.Header + ": " + r.Target
	}
	return scope
}

func shortText(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return s
}

// --- rule dialog ----------------------------------------------------------

func (a *app) ruleDialog(c *ui.Context, pal *uitokens.Palette) {
	ui.DialogBase(c, &a.ruleOpen, func(backdrop, panel ui.Element) {
		backdrop.Background(pal.Overlay)
		panel.Width(620).Radius(uitokens.CardRadius).Background(pal.Popover).
			Border(1, pal.EdgeHigh).Shadow(0, 30, 80, 0, pal.FloatShadow).Clip().
			InsetShadow(0, 1, 0, 0, ui.RGBA(255, 255, 255, 0.08)).Padding(20)
		panel.Children(func() {
			ui.Row(c).Height(30).Children(func() {
				ui.Text(c, ruleKindLabel(a.ruleKind)).FontSize(15).FontWeight(700).
					TextColor(pal.Text).Grow(1)
				if b := iconButton(c, pal, iconX, locales.CommonCancel(), 26); b.Clicked() {
					a.ruleOpen = false
				}
			})
			ui.Column(c).Gap(12).Children(func() {
				ruleField(c, pal, locales.RuleName(), &a.ruleName, locales.RuleNameHint(), 0, false)
				ruleRow(c, func() {
					kind := ruleKindLabel(a.ruleKind)
					ruleSelectField(c, pal, locales.RuleKind(), &kind, ruleKindOptions(), 220)
					if kind != ruleKindLabel(a.ruleKind) {
						a.ruleKind = ruleKindFromLabel(kind)
					}
					method := ruleMethodLabel(a.ruleMethod)
					ruleSelectField(c, pal, locales.RuleMethod(), &method, ruleMethodOptions(), 170)
					if method != ruleMethodLabel(a.ruleMethod) {
						a.ruleMethod = ruleMethodFromLabel(method)
					}
				})
				ruleRow(c, func() {
					ruleField(c, pal, locales.RuleHost(), &a.ruleHost, locales.RuleHostHint(), 0, false)
					ruleField(c, pal, locales.RulePathPrefix(), &a.rulePath, "/", 170, false)
				})
				if a.ruleKind == model.RuleMapRemote {
					ruleField(c, pal, locales.RuleTarget(), &a.ruleTarget, locales.RuleTargetHint(), 0, false)
				} else {
					ruleRow(c, func() {
						ruleField(c, pal, locales.RuleHeader(), &a.ruleHeader, locales.RuleHeaderHint(), 0, false)
						ruleField(c, pal, locales.RuleValue(), &a.ruleValue, locales.RuleValueHint(), 0, a.ruleRemove)
					})
					ui.Row(c).Height(26).Gap(8).Children(func() {
						ui.Text(c, locales.RuleRemove()).FontSize(11.5).TextColor(pal.TextMuted).Grow(1)
						ui.Switch(c.Key("rule-remove"), &a.ruleRemove).Label(locales.RuleRemove())
					})
				}
			})
			if a.ruleErr != "" {
				ui.Text(c, a.ruleErr).FontSize(11).TextColor(pal.Danger)
			}
			ui.Row(c).Height(32).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c.Key("rule-cancel"), locales.CommonCancel()).Clicked() {
					a.ruleOpen = false
				}
				if ui.PrimaryButton(c.Key("rule-save"), locales.CommonSave()).Clicked() {
					a.saveRuleDialog()
				}
			})
		})
	})
}

// ruleRow lays two fields side by side.
func ruleRow(c *ui.Context, fn func()) {
	ui.Row(c).Gap(12).AlignItems(ui.End).Children(fn)
}

// ruleField is a label over a text input. A width of zero fills the row.
func ruleField(c *ui.Context, pal *uitokens.Palette, label string, value *string, placeholder string, width float32, readOnly bool) ui.Element {
	var in ui.Element
	col := ui.Column(c).Gap(4).MinWidth(0)
	if width > 0 {
		col.Shrink(0)
	} else {
		col.Grow(1)
	}
	col.Children(func() {
		ui.Text(c, label).FontSize(10.5).TextColor(pal.TextFaint)
		in = ui.TextInput(c, value).Height(30).FontSize(11.5).Padding(0, 10).
			Background(pal.CodeBg).Border(1, pal.Edge).Label(label).Placeholder(placeholder).
			ReadOnly(readOnly)
		if width > 0 {
			in.Width(width)
		} else {
			in.FillWidth()
		}
	})
	return in
}

// ruleSelectField is a label over a select. The label is the value shown and
// chosen; the caller maps it back to the rule's own value.
func ruleSelectField(c *ui.Context, pal *uitokens.Palette, label string, value *string, options []string, width float32) {
	ui.Column(c).Gap(4).Shrink(0).Children(func() {
		ui.Text(c, label).FontSize(10.5).TextColor(pal.TextFaint)
		ui.Select(c, value, options).Width(width).Height(30).Label(label)
	})
}

// --- rules settings page --------------------------------------------------

func (a *app) settingsRulesPage(c *ui.Context, pal *uitokens.Palette) {
	top := ui.Row(c).Height(32).Gap(8)
	top.Children(func() {
		ui.Text(c, locales.SettingsRulesHint()).FontSize(10.5).TextColor(pal.TextFaint).Grow(1)
		if ui.PrimaryButton(c.Key("rule-new"), locales.RuleNew()).Clicked() {
			a.openNewRule(model.RuleMapRemote)
		}
	})

	if len(a.rules) == 0 {
		settingsGroup(c, pal, func() {
			ui.Text(c, locales.SettingsRulesEmpty()).FontSize(11.5).TextColor(pal.TextMuted)
		})
		return
	}

	var toggleID, deleteID, editID string
	var toggleOn bool
	settingsGroup(c, pal, func() {
		for i := range a.rules {
			r := a.rules[i]
			if i > 0 {
				ui.Box(c).Height(1).Background(pal.Divider)
			}
			row := ui.Row(c).Height(48).Gap(8).AlignItems(ui.Center).Cursor(ui.CursorPointer)
			row.Children(func() {
				ruleKindBadge(c, pal, r.Kind)
				ui.Column(c).Grow(1).MinWidth(0).Gap(1).Justify(ui.Center).Children(func() {
					ui.Text(c, r.Name).FontSize(11.5).FontWeight(600).TextColor(pal.Text).
						SingleLine().Ellipsis("…")
					ui.Text(c, ruleSubtitle(r)).FontSize(9.5).Font("monospace").
						TextColor(pal.TextFaint).SingleLine().Ellipsis("…")
				})
				enabled := r.Enabled
				ui.Switch(c.Key("rule:"+r.ID), &enabled)
				if enabled != r.Enabled {
					toggleID, toggleOn = r.ID, enabled
				}
				if b := iconButton(c, pal, iconTrash, locales.RuleDelete(), 24); b.Clicked() {
					deleteID = r.ID
				}
			})
			if row.Clicked() {
				editID = r.ID
			}
		}
	})

	if toggleID != "" {
		a.setRuleEnabled(toggleID, toggleOn)
	}
	if editID != "" {
		for _, r := range a.rules {
			if r.ID == editID {
				a.editRule(r)
				break
			}
		}
	}
	if deleteID != "" {
		a.deleteRule(deleteID)
	}
}

func ruleKindBadge(c *ui.Context, pal *uitokens.Palette, kind string) {
	label, col := "MAP", pal.Accent
	if kind == model.RuleRequestHeader {
		label, col = "REQ", pal.Info
	} else if kind == model.RuleResponseHeader {
		label, col = "RES", pal.Success
	}
	ui.Row(c).Width(36).Height(18).Center().Radius(4).Background(col.Alpha(0.14)).Children(func() {
		ui.Text(c, label).FontSize(8.5).FontWeight(750).LetterSpacing(0.3).TextColor(col)
	})
}

// --- flow actions ---------------------------------------------------------

// ruleMenu fills the Rules submenu of a flow's context menu.
func (a *app) ruleMenu(m *ui.Menu, f proxy.Flow) {
	if m.Item(locales.RuleMapRemote()).Chosen() {
		a.openRuleFromFlow(model.RuleMapRemote, f)
	}
	if m.Item(locales.RuleRequestHeader()).Chosen() {
		a.openRuleFromFlow(model.RuleRequestHeader, f)
	}
	if m.Item(locales.RuleResponseHeader()).Chosen() {
		a.openRuleFromFlow(model.RuleResponseHeader, f)
	}
}
