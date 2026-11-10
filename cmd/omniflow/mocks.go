package main

// Mocks are captured responses kept as files and served back by the proxy
// instead of the network: a Map Local rule made from what was on screen.

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/CarlOpenLab/omniflow/internal/model"
	"github.com/CarlOpenLab/omniflow/internal/proxy"
	"github.com/CarlOpenLab/omniflow/internal/uitokens"
	"github.com/CarlOpenLab/omniflow/locales"
)

// applyMocks hands the enabled rules to the proxy.
func (a *app) applyMocks() {
	if a.prx == nil {
		return
	}
	mocks := make([]proxy.Mock, 0, len(a.mocks))
	for _, r := range a.mocks {
		if !r.Enabled || r.Method == "" || r.Host == "" || r.Path == "" || r.File == "" {
			continue
		}
		mocks = append(mocks, proxy.Mock{
			Method:      r.Method,
			Host:        r.Host,
			Path:        r.Path,
			Status:      r.Status,
			ContentType: r.ContentType,
			File:        r.File,
			Headers:     mockHeaderPairs(r.RespHeaders),
		})
	}
	a.prx.SetMocks(mocks)
}

func (a *app) persistMocks() {
	if a.store != nil {
		_ = a.store.SetMocks(a.mocks)
	}
}

// mockHeaders saves a capture's response headers with the rule.
func mockHeaders(headers []proxy.HeaderPair) []model.Header {
	if len(headers) == 0 {
		return nil
	}
	out := make([]model.Header, len(headers))
	for i, h := range headers {
		out[i] = model.Header{Key: h.Key, Value: h.Value}
	}
	return out
}

// mockHeaderPairs hands a rule's saved headers to the proxy.
func mockHeaderPairs(headers []model.Header) []proxy.HeaderPair {
	if len(headers) == 0 {
		return nil
	}
	out := make([]proxy.HeaderPair, len(headers))
	for i, h := range headers {
		out[i] = proxy.HeaderPair{Key: h.Key, Value: h.Value}
	}
	return out
}

// mockFlow turns a captured response into a Map Local rule: the body is
// written into a file the person can edit, and the proxy answers the same
// method, host and path from it.
func (a *app) mockFlow(f proxy.Flow) {
	if f.Status == 0 {
		a.setNotice(locales.NoticeMockNeedsResponse())
		return
	}
	if a.store == nil {
		a.setNotice(locales.CommonUnavailable())
		return
	}
	path := f.Path
	if i := strings.IndexByte(path, '?'); i >= 0 {
		path = path[:i]
	}
	if path == "" {
		path = "/"
	}
	name := f.Method + " " + f.Host + path

	for i := range a.mocks {
		r := &a.mocks[i]
		if r.Method != f.Method || !strings.EqualFold(r.Host, f.Host) || r.Path != path {
			continue
		}
		if err := os.WriteFile(r.File, f.RespBody, 0o644); err != nil {
			a.setNotice(locales.NoticeMockWriteFailed(err.Error()))
			return
		}
		r.Enabled, r.Name = true, name
		r.Status, r.ContentType = f.Status, f.ContentType
		r.RespHeaders = mockHeaders(f.RespHeaders)
		a.persistMocks()
		a.applyMocks()
		a.setNotice(mockNotice(name, f.RespTruncated, true))
		a.openMockEditor(*r)
		return
	}

	dir := filepath.Join(a.dataDir(), "mocks")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		a.setNotice(locales.NoticeMockWriteFailed(err.Error()))
		return
	}
	id := model.NewID("mock")
	file := filepath.Join(dir, id+mockFileExt(f.ContentType))
	if err := os.WriteFile(file, f.RespBody, 0o644); err != nil {
		a.setNotice(locales.NoticeMockWriteFailed(err.Error()))
		return
	}
	a.mocks = append([]model.MockRule{{
		ID:          id,
		Enabled:     true,
		Name:        name,
		Method:      f.Method,
		Host:        f.Host,
		Path:        path,
		Status:      f.Status,
		ContentType: f.ContentType,
		File:        file,
		RespHeaders: mockHeaders(f.RespHeaders),
		CreatedAt:   time.Now(),
	}}, a.mocks...)
	a.limitMocks()
	a.persistMocks()
	a.applyMocks()
	a.setNotice(mockNotice(name, f.RespTruncated, false))
	a.openMockEditor(a.mocks[0])
}

// mockNotice says what the new or updated mock answered, and warns when the
// captured body was cut before the rule was made.
func mockNotice(name string, truncated, updated bool) string {
	if truncated {
		return locales.NoticeMockTruncated()
	}
	if updated {
		return locales.NoticeMockUpdated(name)
	}
	return locales.NoticeMockCreated(name)
}

// limitMocks keeps the list bounded, and takes the oldest rule's file with
// it so the mock folder does not collect orphans.
func (a *app) limitMocks() {
	const maxMocks = 200
	for len(a.mocks) > maxMocks {
		last := a.mocks[len(a.mocks)-1]
		a.mocks = a.mocks[:len(a.mocks)-1]
		if last.File != "" {
			_ = os.Remove(last.File)
		}
	}
}

func (a *app) deleteMock(id string) {
	for i := range a.mocks {
		if a.mocks[i].ID != id {
			continue
		}
		file, name := a.mocks[i].File, a.mocks[i].Name
		a.mocks = append(a.mocks[:i], a.mocks[i+1:]...)
		a.persistMocks()
		a.applyMocks()
		if a.mockEditID == id {
			a.mockOpen, a.mockEditID, a.mockBody = false, "", nil
		}
		if file != "" {
			_ = os.Remove(file)
		}
		a.setNotice(locales.NoticeMockDeleted(name))
		return
	}
}

func (a *app) setMockEnabled(id string, enabled bool) {
	for i := range a.mocks {
		if a.mocks[i].ID != id {
			continue
		}
		a.mocks[i].Enabled = enabled
		a.persistMocks()
		a.applyMocks()
		return
	}
}

func (a *app) revealMock(r model.MockRule) {
	if err := revealPath(r.File); err != nil {
		a.setNotice(locales.NoticeMockRevealFailed(err.Error()))
	}
}

func (a *app) revealMocksDir() {
	if a.store == nil {
		return
	}
	if err := revealPath(filepath.Join(a.dataDir(), "mocks")); err != nil {
		a.setNotice(locales.NoticeMockRevealFailed(err.Error()))
	}
}

// --- editing a mock's response --------------------------------------------

// mockEditMax is the largest body the dialog holds as text. A larger file is
// edited in its own editor rather than copied here.
const mockEditMax = 8 << 20

// mockRule is the rule the dialog edits, by id.
func (a *app) mockRule(id string) (model.MockRule, bool) {
	for _, r := range a.mocks {
		if r.ID == id {
			return r, true
		}
	}
	return model.MockRule{}, false
}

// openMockEditor loads a rule's response into the dialog: its status and
// content type as typed, and its body in a buffer kept across frames so the
// caret, the selection and undo survive a redraw. A body that is not text, or
// is larger than the dialog should hold, is left in its file.
func (a *app) openMockEditor(rule model.MockRule) {
	a.mockOpen = true
	a.mockEditID = rule.ID
	a.mockStatus = strconv.Itoa(rule.Status)
	a.mockContent = rule.ContentType
	a.mockErr, a.mockNote, a.mockEditable = "", "", true

	// A body too large for the dialog is not read at all: the file can be
	// hundreds of megabytes, and copying it here only to refuse it would
	// stall the window.
	var data []byte
	info, err := os.Stat(rule.File)
	switch {
	case err != nil:
		a.mockNote = locales.MockEditReadFailed(err.Error())
	case info.Size() > mockEditMax:
		a.mockEditable = false
		a.mockNote = locales.MockEditTooLarge()
	default:
		data, err = os.ReadFile(rule.File)
		if err != nil {
			a.mockNote = locales.MockEditReadFailed(err.Error())
			data = nil
		} else if shapeOf(data, rule.ContentType) != shapeText {
			a.mockEditable = false
			a.mockNote = locales.MockEditBinary()
			data = nil
		}
	}
	if a.mockBody == nil {
		a.mockBody = ui.NewTextBuffer("")
	}
	if a.mockEditable {
		a.mockBody.Set(string(data))
	} else {
		a.mockBody.Set("")
	}
}

// saveMockDialog writes what the dialog holds: the body, when it is one the
// dialog edits, and the status and content type beside it. A field the proxy
// cannot answer with keeps the dialog open, with the reason under it.
func (a *app) saveMockDialog() {
	status, err := strconv.Atoi(strings.TrimSpace(a.mockStatus))
	if err != nil || status < 100 || status > 599 {
		a.mockErr = locales.MockEditInvalidStatus()
		return
	}
	rule, ok := a.mockRule(a.mockEditID)
	if !ok {
		a.mockOpen, a.mockEditID, a.mockBody = false, "", nil
		return
	}
	if a.mockEditable && a.mockBody != nil {
		if err := os.WriteFile(rule.File, []byte(a.mockBody.String()), 0o644); err != nil {
			a.mockErr = locales.NoticeMockWriteFailed(err.Error())
			return
		}
	}
	for i := range a.mocks {
		if a.mocks[i].ID != rule.ID {
			continue
		}
		a.mocks[i].Status = status
		a.mocks[i].ContentType = strings.TrimSpace(a.mockContent)
		rule = a.mocks[i]
		break
	}
	a.persistMocks()
	a.applyMocks()
	a.mockOpen, a.mockEditID, a.mockBody = false, "", nil
	a.mockErr = ""
	a.setNotice(locales.NoticeMockUpdated(rule.Name))
}

// mockRulesPanel is the column beside the capture: the Map Local rules made
// from responses, each ready to switch off or open for editing.
func (a *app) mockRulesPanel(c *ui.Context, pal *uitokens.Palette) {
	col := ui.Column(c).Width(300).Shrink(0).MinHeight(0).
		BorderWidth(0, 0, 0, 1).BorderColor(pal.Divider)
	col.Children(func() {
		head := ui.Row(c).Height(42).Shrink(0).Padding(0, 12).Gap(8).
			BorderWidth(0, 0, 1, 0).BorderColor(pal.Divider)
		head.Children(func() {
			ui.Icon(c, iconSpark).Size(12, 12).TextColor(pal.Accent)
			ui.Text(c, locales.SettingsMocks()).FontSize(12).FontWeight(650).
				TextColor(pal.Text).Grow(1).SingleLine()
			if n := len(a.mocks); n > 0 {
				ui.Row(c).Height(18).Padding(0, 6).Radius(4).Center().
					Background(pal.Chip).Children(func() {
					ui.Text(c, fmt.Sprint(n)).FontSize(9.5).Font("monospace").
						TextColor(pal.TextMuted)
				})
			}
			if b := iconButton(c, pal, iconFolder, locales.SettingsMocksFolder(), 24); b.Clicked() {
				a.revealMocksDir()
			}
		})

		if len(a.mocks) == 0 {
			ui.Column(c).Fill().Center().Gap(8).Padding(16).Children(func() {
				ui.Box(c).Size(38, 38).Radius(11).Center().Background(pal.AccentSoft).
					Children(func() {
						ui.Icon(c, iconSpark).Size(17, 17).TextColor(pal.Accent)
					})
				ui.Text(c, locales.SettingsMocksEmpty()).FontSize(11.5).FontWeight(600).
					TextColor(pal.TextMuted)
				ui.Text(c, locales.SettingsMocksHint()).FontSize(10.5).TextColor(pal.TextFaint).
					TextAlign(ui.Center).LineHeight(1.45).MaxWidth(230)
			})
			return
		}

		var toggleID, editID, deleteID string
		var toggleOn bool
		ui.Scroll(c).Grow(1).MinHeight(0).Children(func() {
			ui.Column(c).FillWidth().Children(func() {
				for i := range a.mocks {
					rule := a.mocks[i]
					if i > 0 {
						ui.Box(c).Height(1).Background(pal.Divider)
					}
					row := ui.Row(c).Height(50).Padding(0, 12).Gap(8).
						AlignItems(ui.Center).Cursor(ui.CursorPointer)
					if row.Hovered() {
						row.Background(pal.RowHover)
					}
					row.Children(func() {
						enabled := rule.Enabled
						ui.Switch(c.Key("panel-mock:"+rule.ID), &enabled)
						if enabled != rule.Enabled {
							toggleID, toggleOn = rule.ID, enabled
						}
						ui.Column(c).Grow(1).MinWidth(0).Gap(1).Justify(ui.Center).
							Children(func() {
								ui.Text(c, rule.Name).FontSize(11).FontWeight(600).
									TextColor(pal.Text).SingleLine().Ellipsis("…")
								ui.Text(c, mockSubtitle(rule)).FontSize(9.5).Font("monospace").
									TextColor(pal.TextFaint).SingleLine().Ellipsis("…")
							})
						if b := iconButton(c, pal, iconTrash, locales.SettingsMockDelete(), 24); b.Clicked() {
							deleteID = rule.ID
						}
					})
					if row.Clicked() {
						editID = rule.ID
					}
				}
			})
		})

		// A click on the switch is the switch's, not the row's: it must not
		// also open the editor.
		if toggleID != "" {
			a.setMockEnabled(toggleID, toggleOn)
		} else if editID != "" {
			if rule, ok := a.mockRule(editID); ok {
				a.openMockEditor(rule)
			}
		}
		if deleteID != "" {
			a.deleteMock(deleteID)
		}
	})
}

// mockSubtitle is the line under a rule's name: what it answers with.
func mockSubtitle(r model.MockRule) string {
	ct := shortContentType(r.ContentType)
	if ct == "" {
		return fmt.Sprintf("%d", r.Status)
	}
	return fmt.Sprintf("%d · %s", r.Status, ct)
}

// mockDialog edits a mock's response where it lives: status, content type
// and body, saved back into the file the proxy serves.
func (a *app) mockDialog(c *ui.Context, pal *uitokens.Palette) {
	ui.DialogBase(c, &a.mockOpen, func(backdrop, panel ui.Element) {
		backdrop.Background(pal.Overlay)
		panel.Width(760).Height(560).Radius(uitokens.CardRadius).Background(pal.Popover).
			Border(1, pal.EdgeHigh).Shadow(0, 30, 80, 0, pal.FloatShadow).Clip().
			InsetShadow(0, 1, 0, 0, ui.RGBA(255, 255, 255, 0.08)).Padding(20)
		panel.Children(func() {
			rule, ok := a.mockRule(a.mockEditID)
			head := ui.Row(c).Height(38).Gap(8)
			head.Children(func() {
				ui.Column(c).Grow(1).MinWidth(0).Gap(1).Justify(ui.Center).Children(func() {
					ui.Text(c, locales.MockEditTitle()).FontSize(15).FontWeight(700).
						TextColor(pal.Text)
					if ok {
						ui.Text(c, rule.Method+" "+rule.Host+rule.Path).Font("monospace").
							FontSize(10).TextColor(pal.TextFaint).SingleLine().Ellipsis("…")
					}
				})
				if b := iconButton(c, pal, iconX, locales.CommonCancel(), 26); b.Clicked() {
					a.mockOpen = false
				}
			})
			if !ok {
				return
			}

			ui.Column(c).Gap(12).Grow(1).MinHeight(0).Children(func() {
				ui.Row(c).Gap(12).AlignItems(ui.End).Children(func() {
					mockField(c, pal, locales.MockEditStatus(), locales.MockEditStatusLabel(),
						&a.mockStatus, "200", 110)
					mockField(c, pal, locales.MockEditContentType(), locales.MockEditContentTypeLabel(),
						&a.mockContent, "application/json", 0)
				})
				ui.Row(c).Height(26).Gap(8).Children(func() {
					ui.Text(c, locales.MockEditBody()).FontSize(11.5).FontWeight(600).
						TextColor(pal.TextMuted)
					ui.Spacer(c)
					if b := iconButton(c, pal, iconFolder, locales.SettingsMockReveal(), 24); b.Clicked() {
						a.revealMock(rule)
					}
				})
				ui.Column(c).Grow(1).MinHeight(0).Radius(8).Clip().
					Background(pal.CodeBg).Border(1, pal.Edge).Children(func() {
					if a.mockEditable && a.mockBody != nil {
						ui.TextAreaBufferBase(c, a.mockBody).Fill().Padding(10, 12).
							Font("monospace").FontSize(12).Label(locales.MockEditBodyLabel())
						return
					}
					ui.Column(c).Fill().Center().Gap(10).Padding(24).Children(func() {
						ui.Text(c, a.mockNote).FontSize(11.5).TextColor(pal.TextMuted).
							TextAlign(ui.Center).MaxWidth(420)
						if ui.Button(c.Key("mock-reveal"), locales.SettingsMockReveal()).Clicked() {
							a.revealMock(rule)
						}
					})
				})
				if a.mockNote != "" && a.mockEditable {
					ui.Text(c, a.mockNote).FontSize(10.5).TextColor(pal.Warning)
				}
				if a.mockErr != "" {
					ui.Text(c, a.mockErr).FontSize(11).TextColor(pal.Danger)
				}
				ui.Row(c).Height(32).Gap(8).Justify(ui.End).Children(func() {
					if ui.Button(c.Key("mock-cancel"), locales.CommonCancel()).Clicked() {
						a.mockOpen = false
					}
					if ui.PrimaryButton(c.Key("mock-save"), locales.CommonSave()).Clicked() {
						a.saveMockDialog()
					}
				})
			})
		})
	})
}

// mockField is a label over a text input. A width of zero fills the row.
// The input's own label names it for assistive technology, apart from the
// text above it, so a reader hears which value is being edited.
func mockField(c *ui.Context, pal *uitokens.Palette, label, a11y string, value *string, placeholder string, width float32) {
	col := ui.Column(c).Gap(4).MinWidth(0)
	if width > 0 {
		col.Shrink(0)
	} else {
		col.Grow(1)
	}
	col.Children(func() {
		ui.Text(c, label).FontSize(10.5).TextColor(pal.TextFaint)
		in := ui.TextInput(c, value).Height(30).FontSize(11.5).Padding(0, 10).
			Background(pal.CodeBg).Border(1, pal.Edge).Label(a11y).Placeholder(placeholder)
		if width > 0 {
			in.Width(width)
		} else {
			in.FillWidth()
		}
	})
}

// mockFileExt gives the captured body an extension an editor understands.
func mockFileExt(contentType string) string {
	ct := strings.ToLower(contentType)
	switch {
	case strings.Contains(ct, "json"):
		return ".json"
	case strings.Contains(ct, "html"):
		return ".html"
	case strings.Contains(ct, "xml"):
		return ".xml"
	case strings.Contains(ct, "javascript"):
		return ".js"
	case strings.Contains(ct, "css"):
		return ".css"
	case strings.Contains(ct, "svg"):
		return ".svg"
	case strings.Contains(ct, "png"):
		return ".png"
	case strings.Contains(ct, "jpeg"), strings.Contains(ct, "jpg"):
		return ".jpg"
	case strings.Contains(ct, "gif"):
		return ".gif"
	case strings.HasPrefix(ct, "text/"):
		return ".txt"
	}
	return ".bin"
}
