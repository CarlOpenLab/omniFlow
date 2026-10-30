package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/CarlOpenLab/omniflow/internal/model"
	"github.com/CarlOpenLab/omniflow/internal/uitokens"
)

// --- environments ---------------------------------------------------------

// activeEnv is the environment {{variables}} are filled in from, if any.
func (a *app) activeEnv() (*model.Environment, bool) {
	return a.envByID(a.prefs.Environment)
}

func (a *app) envByID(id string) (*model.Environment, bool) {
	if id == "" {
		return nil, false
	}
	for i := range a.envs {
		if a.envs[i].ID == id {
			return &a.envs[i], true
		}
	}
	return nil, false
}

// envValues are the active environment's variables, by name.
func (a *app) envValues() map[string]string {
	if e, ok := a.activeEnv(); ok {
		return e.Values()
	}
	return nil
}

// resolve fills a request's {{variables}} in from the environment in use,
// and names the ones it could not.
func (a *app) resolve(r model.Request) (model.Request, []string) {
	res := model.Resolver{Values: a.envValues()}
	out := r.Resolved(&res)
	return out, res.Missing
}

// missingIn are the names of missing that are still written in s.
func missingIn(s string, missing []string) []string {
	var out []string
	for _, p := range model.Placeholders(s) {
		if slices.Contains(missing, p.Name) && !slices.Contains(out, p.Name) {
			out = append(out, p.Name)
		}
	}
	return out
}

// braces writes names as their placeholders, for a sentence.
func braces(names []string) string {
	parts := make([]string, len(names))
	for i, n := range names {
		parts[i] = "{{" + n + "}}"
	}
	return strings.Join(parts, ", ")
}

// missingMessage says why variables were not filled in, and where to fill
// them.
func (a *app) missingMessage(names []string) string {
	verb := "is"
	if len(names) > 1 {
		verb = "are"
	}
	if e, ok := a.activeEnv(); ok {
		return fmt.Sprintf("%s %s not set in “%s”. Add it under Environments, or choose another.",
			braces(names), verb, e.Name)
	}
	return fmt.Sprintf("%s %s not set: no environment is in use. Choose one in the sidebar to fill it in.",
		braces(names), verb)
}

func (a *app) setActiveEnv(id string) {
	if id == a.prefs.Environment {
		return
	}
	a.prefs.Environment = id
	a.savePrefs()
	if e, ok := a.envByID(id); ok {
		a.setNotice("Using “" + e.Name + "”")
	} else {
		a.setNotice("No environment in use")
	}
}

func (a *app) persistEnvs() {
	if a.store != nil {
		_ = a.store.SetEnvironments(a.envs)
	}
}

// openEnvs opens the manager on the environment in use, or the first.
func (a *app) openEnvs() {
	a.envEdit = a.prefs.Environment
	if _, ok := a.envByID(a.envEdit); !ok && len(a.envs) > 0 {
		a.envEdit = a.envs[0].ID
	}
	a.envOpen = true
}

// addEnv makes a new environment, with the variables of the one being
// edited as a start when there is one: environments differ in their values
// far more often than in their names.
func (a *app) addEnv() {
	e := model.NewEnvironment(uniqueEnvName(a.envs, "New Environment"))
	if from, ok := a.envByID(a.envEdit); ok {
		var vars []model.KV
		for _, kv := range from.Vars {
			if strings.TrimSpace(kv.Key) != "" {
				vars = append(vars, model.KV{ID: model.NewID("kv"), Enabled: kv.Enabled, Key: kv.Key})
			}
		}
		if len(vars) > 0 {
			e.Vars = vars
		}
	}
	a.envs = append(a.envs, e)
	a.envEdit = e.ID
	if len(a.envs) == 1 {
		// The first environment is the one in use: there is no other.
		a.prefs.Environment = e.ID
		a.savePrefs()
	}
	a.persistEnvs()
}

func (a *app) duplicateEnv(id string) {
	i := slices.IndexFunc(a.envs, func(e model.Environment) bool { return e.ID == id })
	if i < 0 {
		return
	}
	cp := a.envs[i].Clone()
	cp.ID, cp.Name = model.NewID("env"), uniqueEnvName(a.envs, a.envs[i].Name+" Copy")
	for j := range cp.Vars {
		cp.Vars[j].ID = model.NewID("kv")
	}
	a.envs = slices.Insert(a.envs, i+1, cp)
	a.envEdit = cp.ID
	a.persistEnvs()
}

// deleteEnv takes an environment away, with Undo in the notice.
func (a *app) deleteEnv(id string) {
	i := slices.IndexFunc(a.envs, func(e model.Environment) bool { return e.ID == id })
	if i < 0 {
		return
	}
	removed, wasActive := a.envs[i], a.prefs.Environment == id
	a.envs = slices.Delete(a.envs, i, i+1)
	if wasActive {
		a.prefs.Environment = ""
		a.savePrefs()
	}
	if a.envEdit == id {
		a.envEdit = ""
		if len(a.envs) > 0 {
			a.envEdit = a.envs[min(i, len(a.envs)-1)].ID
		}
	}
	a.persistEnvs()
	a.setUndo("Deleted “"+removed.Name+"”", func() {
		a.envs = slices.Insert(a.envs, min(i, len(a.envs)), removed)
		if wasActive {
			a.prefs.Environment = removed.ID
			a.savePrefs()
		}
		a.envEdit = removed.ID
		a.persistEnvs()
	})
}

func uniqueEnvName(envs []model.Environment, base string) string {
	name := base
	for n := 2; slices.ContainsFunc(envs, func(e model.Environment) bool { return e.Name == name }); n++ {
		name = fmt.Sprintf("%s %d", base, n)
	}
	return name
}

// --- the picker -----------------------------------------------------------

// envPicker is the sidebar's row choosing the environment in use.
func (a *app) envPicker(c *ui.Context, pal *uitokens.Palette) {
	label, col := "No Environment", pal.TextMuted
	if e, ok := a.activeEnv(); ok {
		label, col = e.Name, pal.Text
	}
	b := ui.ButtonBase(c.Key("env-picker")).Height(30).Padding(0, 8).Radius(7).Gap(7).
		Label("Environment").Tooltip("The environment {{variables}} are filled in from")
	b.Background(pal.Chip)
	if b.Hovered() {
		b.Background(pal.WashSelected)
	}
	b.Children(func() {
		dot := pal.TextFaint
		if _, ok := a.activeEnv(); ok {
			dot = pal.Success
		}
		statusDot(c, dot, 7)
		ui.Text(c, label).FontSize(11.5).FontWeight(600).TextColor(col).SingleLine().Ellipsis("…").
			Grow(1).MinWidth(0)
		ui.Icon(c, iconChevron).Size(11, 11).TextColor(pal.TextFaint)
	})
	var act func()
	b.Menu(func(m *ui.Menu) {
		if m.Item("No Environment").Checked(a.prefs.Environment == "").Chosen() {
			act = func() { a.setActiveEnv("") }
		}
		if len(a.envs) > 0 {
			m.Separator()
		}
		for _, e := range a.envs {
			id := e.ID
			if m.Item(e.Name).Checked(a.prefs.Environment == id).Chosen() {
				act = func() { a.setActiveEnv(id) }
			}
		}
		m.Separator()
		if m.Item("Manage Environments…").Shortcut(ui.Cmd, ui.KeyE).Chosen() {
			act = a.openEnvs
		}
	})
	if act != nil {
		act()
	}
}

// --- the manager ----------------------------------------------------------

// envDialog edits the environments: a list on the left, the chosen one's
// name and variables on the right. Edits are kept as they are made, and
// written out when the dialog closes.
func (a *app) envDialog(c *ui.Context, pal *uitokens.Palette) {
	if a.envWasOpen && !a.envOpen {
		a.persistEnvs()
	}
	a.envWasOpen = a.envOpen
	ui.DialogBase(c, &a.envOpen, func(backdrop, panel ui.Element) {
		backdrop.Background(pal.Overlay)
		panel.Width(720).Height(480).Radius(uitokens.CardRadius).Background(pal.Popover).
			Border(1, pal.EdgeHigh).Shadow(0, 30, 80, 0, pal.FloatShadow).Clip().
			InsetShadow(0, 1, 0, 0, ui.RGBA(255, 255, 255, 0.08))
		ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
			a.envList(c, pal)
			ui.Column(c).Grow(1).MinWidth(0).Padding(18, 20).Gap(10).Children(func() {
				a.envEditor(c, pal)
			})
		})
	})
}

func (a *app) envList(c *ui.Context, pal *uitokens.Palette) {
	var act func()
	ui.Column(c).Width(210).Shrink(0).Background(pal.Chip).BorderWidth(0, 1, 0, 0).
		BorderColor(pal.Divider).Children(func() {
		ui.Row(c).Height(54).Shrink(0).Padding(0, 10, 0, 16).Gap(6).Children(func() {
			ui.Text(c, "Environments").FontSize(14).FontWeight(700).TextColor(pal.Text).Grow(1)
			if b := iconButton(c, pal, iconPlus, "New Environment", 26); b.Clicked() {
				act = a.addEnv
			}
		})
		ui.Scroll(c).Grow(1).MinHeight(0).Children(func() {
			ui.Column(c).FillWidth().Padding(0, 8, 8, 8).Gap(2).Children(func() {
				for _, e := range a.envs {
					env := e
					chosen := a.envEdit == env.ID
					row := ui.ButtonBase(c.Key("env:"+env.ID)).Height(30).Padding(0, 8).Radius(7).Gap(7).
						Label(env.Name)
					if chosen {
						row.Background(pal.AccentSoft)
					} else if row.Hovered() {
						row.Background(pal.RowHover)
					}
					row.Children(func() {
						dot := ui.Transparent
						if a.prefs.Environment == env.ID {
							dot = pal.Success
						}
						statusDot(c, dot, 6)
						nameCol := pal.Text
						if chosen {
							nameCol = pal.Accent
						}
						ui.Text(c, env.Name).FontSize(11.5).FontWeight(550).TextColor(nameCol).
							SingleLine().Ellipsis("…").Grow(1).MinWidth(0)
						ui.Text(c, fmt.Sprint(len(env.Values()))).FontSize(10).TextColor(pal.TextFaint)
					})
					row.ContextMenu(func(m *ui.Menu) {
						if m.Item("Use").Disabled(a.prefs.Environment == env.ID).Chosen() {
							act = func() { a.setActiveEnv(env.ID) }
						}
						if m.Item("Duplicate").Chosen() {
							act = func() { a.duplicateEnv(env.ID) }
						}
						m.Separator()
						if m.Item("Delete").Chosen() {
							act = func() { a.deleteEnv(env.ID) }
						}
					})
					if row.Clicked() {
						a.envEdit = env.ID
					}
				}
			})
		})
	})
	if act != nil {
		act()
	}
}

func (a *app) envEditor(c *ui.Context, pal *uitokens.Palette) {
	e, ok := a.envByID(a.envEdit)
	if !ok {
		ui.Box(c).Grow(1).MinHeight(0).Children(func() {
			emptyState(c, pal, iconLayers, "No environments yet",
				"An environment holds values that differ between places a request goes, as a host or a key. Write {{name}} in a URL, a header or a body, and the environment in use fills it in.")
		})
		ui.Row(c).Height(32).Gap(8).Justify(ui.End).Children(func() {
			if ui.Button(c.Key("env-done-empty"), "Done").Clicked() {
				a.envOpen = false
			}
			if ui.PrimaryButton(c.Key("env-first"), "New Environment").Clicked() {
				a.addEnv()
			}
		})
		return
	}
	ui.Row(c).Height(34).Shrink(0).Gap(10).Children(func() {
		ui.TextInput(c.Key("env-name:"+e.ID), &e.Name).Height(30).Grow(1).MinWidth(0).FontSize(13).
			FontWeight(650).Padding(0, 10).Background(pal.Chip).Border(1, pal.Edge).Label("Environment name")
		active := a.prefs.Environment == e.ID
		use := ui.Button(c.Key("env-use"), "").Label("Use this environment").Disabled(active)
		use.Children(func() {
			if active {
				ui.Icon(c, iconCheck).Size(12, 12).TextColor(pal.Success)
				ui.Text(c, "In Use")
			} else {
				ui.Text(c, "Use")
			}
		})
		if use.Clicked() {
			a.setActiveEnv(e.ID)
		}
	})
	if strings.TrimSpace(e.Name) == "" {
		e.Name = "Untitled"
	}
	ui.Text(c, "Write {{name}} in a request's URL, headers, body or auth. Built in: "+
		braces(model.DynamicVariables)+".").FontSize(10.5).TextColor(pal.TextFaint).LineHeight(1.4)
	ui.Scroll(c).Grow(1).MinHeight(0).Children(func() {
		if len(e.Vars) == 0 {
			e.Vars = []model.KV{model.NewKV()}
		}
		kvEditor(c, pal, &e.Vars, "variable", "value", "No variables yet.")
	})
	ui.Row(c).Height(32).Shrink(0).Gap(8).Children(func() {
		if b := iconAction(c, pal, iconTrash, "Delete"); b.Clicked() {
			a.deleteEnv(e.ID)
		}
		ui.Spacer(c)
		if ui.PrimaryButton(c.Key("env-done"), "Done").Clicked() {
			a.envOpen = false
		}
	})
}

// --- placeholders in text inputs -------------------------------------------

// varRanges colors the {{variables}} of a field: in the accent when the
// environment in use fills them in, in the danger color when nothing does.
func (a *app) varRanges(pal *uitokens.Palette, s string) []ui.TextRange {
	ps := model.Placeholders(s)
	if len(ps) == 0 {
		return nil
	}
	values := a.envValues()
	out := make([]ui.TextRange, 0, len(ps))
	for _, p := range ps {
		col := pal.Accent
		if _, ok := values[p.Name]; !ok && !model.Dynamic(p.Name) {
			col = pal.Danger
		}
		out = append(out, ui.TextRange{Start: p.Start, End: p.End, Color: col, Weight: 650})
	}
	return out
}

// varTip says what a field's variables are worth, for its tooltip.
func (a *app) varTip(s string) string {
	ps := model.Placeholders(s)
	if len(ps) == 0 {
		return ""
	}
	values := a.envValues()
	var lines []string
	seen := map[string]bool{}
	for _, p := range ps {
		if seen[p.Name] {
			continue
		}
		seen[p.Name] = true
		switch v, ok := values[p.Name]; {
		case ok:
			lines = append(lines, "{{"+p.Name+"}} = "+ellipsize(v, 60))
		case model.Dynamic(p.Name):
			lines = append(lines, "{{"+p.Name+"}} is made as the request goes")
		default:
			lines = append(lines, "{{"+p.Name+"}} is not set")
		}
	}
	return strings.Join(lines, "\n")
}

func ellipsize(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return s
}
