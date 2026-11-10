package main

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/CarlOpenLab/omniflow/internal/model"
	"github.com/CarlOpenLab/omniflow/internal/proxy"
	"github.com/CarlOpenLab/omniflow/internal/store"
	"github.com/CarlOpenLab/omniflow/locales"
)

func newStoredTestApp(t *testing.T) *app {
	t.Helper()
	a := newTestApp(t)
	st, err := store.Open(filepath.Join(t.TempDir(), "omniflow.json"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	a.store = st
	return a
}

func TestSaveFlowAsRequestAddsItToTheCollection(t *testing.T) {
	a := newStoredTestApp(t)
	before := len(a.requests)
	f := proxy.Flow{
		Method: "POST", Host: "api.example", Path: "/login?next=%2F",
		URL: "https://api.example/login?next=%2F", Status: 200,
		ReqHeaders: []proxy.HeaderPair{{Key: "Content-Type", Value: "application/json"}},
		ReqBody:    []byte(`{"user":"ada"}`),
	}

	a.saveFlowAsRequest(f)

	if got := len(a.requests); got != before+1 {
		t.Fatalf("collection has %d requests, want %d", got, before+1)
	}
	if a.mode != modeComposer || a.savedSel == "" {
		t.Errorf("the saved request is not open in the Composer: mode=%v saved=%q", a.mode, a.savedSel)
	}
	if !a.req.Saved || a.req.Method != "POST" || a.req.BodyKind != "json" {
		t.Errorf("saved request is %+v", a.req)
	}
	if a.req.Name != "POST api.example/login" {
		t.Errorf("name = %q", a.req.Name)
	}
}

func TestMockFlowWritesAFileAndUpdatesTheSameRule(t *testing.T) {
	a := newStoredTestApp(t)
	flow := proxy.Flow{
		Method: "GET", Host: "api.example", Path: "/users?page=2", Status: 200,
		ContentType: "application/json", RespBody: []byte(`{"users":[]}`),
	}

	a.mockFlow(flow)
	if len(a.mocks) != 1 {
		t.Fatalf("mocks = %d, want one", len(a.mocks))
	}
	rule := a.mocks[0]
	if rule.Path != "/users" || !strings.HasSuffix(rule.File, ".json") {
		t.Errorf("rule is %+v", rule)
	}
	if body, err := os.ReadFile(rule.File); err != nil || string(body) != `{"users":[]}` {
		t.Errorf("mock file is %q, %v", body, err)
	}

	flow.RespBody = []byte(`{"users":[{"id":1}]}`)
	a.mockFlow(flow)
	if len(a.mocks) != 1 {
		t.Fatalf("a second capture made %d mocks, want the same one", len(a.mocks))
	}
	if body, _ := os.ReadFile(a.mocks[0].File); string(body) != `{"users":[{"id":1}]}` {
		t.Errorf("mock file was not updated: %q", body)
	}

	// The rules are part of the saved data, not only of this run.
	reopened, err := store.Open(a.store.Path())
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	if got := len(reopened.Data().Mocks); got != 1 {
		t.Errorf("store kept %d mocks, want one", got)
	}

	a.deleteMock(a.mocks[0].ID)
	if len(a.mocks) != 0 {
		t.Errorf("delete left %d mocks", len(a.mocks))
	}
	if _, err := os.Stat(rule.File); !os.IsNotExist(err) {
		t.Errorf("the mock file still exists: %v", err)
	}
}

// A mock keeps the response headers it was made from, so CORS and the rest
// of the server's contract survive the replacement.
func TestMockFlowKeepsTheCapturedResponseHeaders(t *testing.T) {
	a := newStoredTestApp(t)
	flow := proxy.Flow{
		Method: "GET", Host: "api.example", Path: "/users", Status: 200,
		ContentType: "application/json", RespBody: []byte(`{}`),
		RespHeaders: []proxy.HeaderPair{
			{Key: "Access-Control-Allow-Origin", Value: "https://app.example"},
			{Key: "X-Trace", Value: "abc"},
		},
	}
	a.mockFlow(flow)
	if len(a.mocks) != 1 {
		t.Fatalf("mocks = %d, want one", len(a.mocks))
	}
	want := []model.Header{
		{Key: "Access-Control-Allow-Origin", Value: "https://app.example"},
		{Key: "X-Trace", Value: "abc"},
	}
	if !slices.Equal(a.mocks[0].RespHeaders, want) {
		t.Fatalf("headers = %+v, want %+v", a.mocks[0].RespHeaders, want)
	}

	// Capturing the same request again refreshes the headers too.
	flow.RespHeaders = []proxy.HeaderPair{{Key: "X-Trace", Value: "def"}}
	a.mockFlow(flow)
	if got := a.mocks[0].RespHeaders; !slices.Equal(got, []model.Header{{Key: "X-Trace", Value: "def"}}) {
		t.Fatalf("headers after recapture = %+v", got)
	}
}

func TestTheTrafficFilterCanSearchBodies(t *testing.T) {
	a := newTestApp(t)
	a.flows = []proxy.Flow{{
		Method: "GET", Host: "api.example", Path: "/users",
		RespBody: []byte(`{"name":"Ada Lovelace"}`),
	}}
	a.flowFilter = "ada"

	if a.matchesFilter(a.flows[0]) {
		t.Fatal("a body matched while body search was off")
	}
	a.searchBodies = true
	if !a.matchesFilter(a.flows[0]) {
		t.Fatal("the response body did not match while body search was on")
	}
}

func TestSettingsListsMocks(t *testing.T) {
	a := newStoredTestApp(t)
	a.mockFlow(proxy.Flow{
		Method: "GET", Host: "api.example", Path: "/users", Status: 200,
		ContentType: "application/json", RespBody: []byte(`{"users":[]}`),
	})
	a.settingsOpen = true
	a.settingsPane = settingsPaneMocks

	tt := newTester(t, a)
	tt.Frame()
	if !tt.HasText(a.mocks[0].Name) {
		t.Fatalf("the Mocks page does not list %q; texts = %v", a.mocks[0].Name, tt.Texts())
	}
}

// seedMockRule puts one mock rule and its file in the app, without going
// through a capture.
func seedMockRule(t *testing.T, a *app, body string) model.MockRule {
	t.Helper()
	file := filepath.Join(t.TempDir(), "users.json")
	if err := os.WriteFile(file, []byte(body), 0o644); err != nil {
		t.Fatalf("write mock: %v", err)
	}
	rule := model.MockRule{
		ID: "m1", Enabled: true, Name: "GET api.example/users",
		Method: "GET", Host: "api.example", Path: "/users",
		Status: 200, ContentType: "application/json", File: file,
		CreatedAt: time.Now(),
	}
	a.mocks = []model.MockRule{rule}
	a.persistMocks()
	return rule
}

// The Inspector keeps the mock rules beside the traffic, so a rule made from
// a response can be found and changed without leaving the capture.
func TestTheInspectorShowsTheMockRulesPanel(t *testing.T) {
	a := newStoredTestApp(t)
	rule := seedMockRule(t, a, `{"users":[]}`)
	a.mode = modeInspector

	tt := newTester(t, a)
	tt.Frame()
	if !tt.HasText(locales.SettingsMocks()) {
		t.Fatalf("the Inspector has no mocks panel: %v", tt.Texts())
	}
	if !tt.HasText(rule.Name) {
		t.Errorf("the panel does not list %q: %v", rule.Name, tt.Texts())
	}
}

// Clicking a rule in the panel opens the dialog that edits its response.
func TestClickingAMockOpensTheResponseEditor(t *testing.T) {
	a := newStoredTestApp(t)
	rule := seedMockRule(t, a, `{"users":[]}`)
	a.mode = modeInspector

	tt := newTester(t, a)
	tt.Frame()
	if err := tt.Click(rule.Name); err != nil {
		t.Fatalf("click the mock rule: %v", err)
	}
	tt.Frame()
	if !tt.HasText(locales.MockEditTitle()) {
		t.Fatalf("the response editor did not open: %v", tt.Texts())
	}
	for _, want := range []string{locales.MockEditStatus(), locales.MockEditContentType(), locales.CommonSave()} {
		if !tt.HasText(want) {
			t.Errorf("the editor is missing %q", want)
		}
	}
}

// The dialog edits the response body itself: Save writes what the editor
// holds back to the file the proxy serves.
func TestSavingTheMockEditorWritesTheBody(t *testing.T) {
	a := newStoredTestApp(t)
	rule := seedMockRule(t, a, `{"users":[]}`)
	a.mode = modeInspector

	tt := newTester(t, a)
	tt.Frame()
	if err := tt.Click(rule.Name); err != nil {
		t.Fatalf("click the mock rule: %v", err)
	}
	tt.Frame()
	if err := tt.Click(locales.MockEditBodyLabel()); err != nil {
		t.Fatalf("click the body editor: %v", err)
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type(`{"users":[{"id":1}]}`)
	tt.Frame()
	if err := tt.Click(locales.CommonSave()); err != nil {
		t.Fatalf("save the mock: %v", err)
	}
	tt.Frame()

	body, err := os.ReadFile(rule.File)
	if err != nil {
		t.Fatalf("read the mock file: %v", err)
	}
	if string(body) != `{"users":[{"id":1}]}` {
		t.Errorf("the mock file is %q", body)
	}
	if tt.HasText(locales.MockEditTitle()) {
		t.Error("the editor stayed open after saving")
	}
}

// Status and content type are edited beside the body and saved with it.
func TestTheMockEditorSavesStatusAndContentType(t *testing.T) {
	a := newStoredTestApp(t)
	rule := seedMockRule(t, a, `{"users":[]}`)
	a.mode = modeInspector

	tt := newTester(t, a)
	tt.Frame()
	if err := tt.Click(rule.Name); err != nil {
		t.Fatalf("click the mock rule: %v", err)
	}
	tt.Frame()
	if err := tt.Click(locales.MockEditStatusLabel()); err != nil {
		t.Fatalf("click the status field: %v", err)
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("418")
	if err := tt.Click(locales.MockEditContentTypeLabel()); err != nil {
		t.Fatalf("click the content type field: %v", err)
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("application/problem+json")
	tt.Frame()
	if err := tt.Click(locales.CommonSave()); err != nil {
		t.Fatalf("save the mock: %v", err)
	}
	tt.Frame()

	if a.mocks[0].Status != 418 {
		t.Errorf("status = %d, want 418", a.mocks[0].Status)
	}
	if a.mocks[0].ContentType != "application/problem+json" {
		t.Errorf("content type = %q", a.mocks[0].ContentType)
	}
	if body, _ := os.ReadFile(rule.File); string(body) != `{"users":[]}` {
		t.Errorf("editing the fields changed the body: %q", body)
	}
}

// A status that is not one keeps the dialog open with the reason under the
// fields; nothing is written.
func TestTheMockEditorKeepsItselfOpenOnABadStatus(t *testing.T) {
	a := newStoredTestApp(t)
	rule := seedMockRule(t, a, `{"users":[]}`)
	a.mode = modeInspector

	tt := newTester(t, a)
	tt.Frame()
	if err := tt.Click(rule.Name); err != nil {
		t.Fatalf("click the mock rule: %v", err)
	}
	tt.Frame()
	if err := tt.Click(locales.MockEditStatusLabel()); err != nil {
		t.Fatalf("click the status field: %v", err)
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("abc")
	tt.Frame()
	if err := tt.Click(locales.CommonSave()); err != nil {
		t.Fatalf("save the mock: %v", err)
	}
	tt.Frame()

	if a.mocks[0].Status != 200 {
		t.Errorf("status = %d, want the captured 200", a.mocks[0].Status)
	}
	if !tt.HasText(locales.MockEditTitle()) {
		t.Error("the editor closed on a bad status")
	}
	if !tt.HasText(locales.MockEditInvalidStatus()) {
		t.Errorf("the editor does not say what is wrong: %v", tt.Texts())
	}
}

// A body that is not text is not loaded into the dialog; Save keeps the file
// as it was, so an image rule can change its status without being corrupted.
func TestTheMockEditorLeavesABinaryBodyInItsFile(t *testing.T) {
	a := newStoredTestApp(t)
	file := filepath.Join(t.TempDir(), "logo.png")
	body := []byte{0x89, 'P', 'N', 'G', 0x00, 0x01, 0x02, 0xff}
	if err := os.WriteFile(file, body, 0o644); err != nil {
		t.Fatalf("write mock: %v", err)
	}
	a.mocks = []model.MockRule{{
		ID: "m1", Enabled: true, Name: "GET api.example/logo",
		Method: "GET", Host: "api.example", Path: "/logo",
		Status: 200, ContentType: "image/png", File: file, CreatedAt: time.Now(),
	}}
	a.mode = modeInspector

	tt := newTester(t, a)
	tt.Frame()
	if err := tt.Click(a.mocks[0].Name); err != nil {
		t.Fatalf("click the mock rule: %v", err)
	}
	tt.Frame()
	if !tt.HasText(locales.MockEditBinary()) {
		t.Fatalf("the editor does not say the body is binary: %v", tt.Texts())
	}
	if err := tt.Click(locales.CommonSave()); err != nil {
		t.Fatalf("save the mock: %v", err)
	}
	tt.Frame()

	got, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read the mock file: %v", err)
	}
	if !bytes.Equal(got, body) {
		t.Errorf("the binary body was changed: %v", got)
	}
}

// A body past what the dialog holds is refused with a note, not read into
// memory, and saving the fields leaves the file whole.
func TestTheMockEditorLeavesAnOversizedBodyInItsFile(t *testing.T) {
	a := newStoredTestApp(t)
	file := filepath.Join(t.TempDir(), "large.txt")
	body := strings.Repeat("x", mockEditMax+1)
	if err := os.WriteFile(file, []byte(body), 0o644); err != nil {
		t.Fatalf("write mock: %v", err)
	}
	a.mocks = []model.MockRule{{
		ID: "m1", Enabled: true, Name: "GET api.example/large",
		Method: "GET", Host: "api.example", Path: "/large",
		Status: 200, ContentType: "text/plain", File: file, CreatedAt: time.Now(),
	}}
	a.mode = modeInspector

	tt := newTester(t, a)
	tt.Frame()
	if err := tt.Click(a.mocks[0].Name); err != nil {
		t.Fatalf("click the mock rule: %v", err)
	}
	tt.Frame()
	if !tt.HasText(locales.MockEditTooLarge()) {
		t.Fatalf("the editor does not say the body is too large: %v", tt.Texts())
	}
	if err := tt.Click(locales.CommonSave()); err != nil {
		t.Fatalf("save the mock: %v", err)
	}
	tt.Frame()

	info, err := os.Stat(file)
	if err != nil {
		t.Fatalf("stat the mock file: %v", err)
	}
	if info.Size() != int64(len(body)) {
		t.Errorf("the body was rewritten to %d bytes, want %d", info.Size(), len(body))
	}
}

// Making a mock from a response opens its editor at once: the point of
// mocking is to change what the server said.
func TestMockingAResponseOpensTheEditor(t *testing.T) {
	a := newStoredTestApp(t)
	a.mode = modeInspector
	a.mockFlow(proxy.Flow{
		Method: "GET", Host: "api.example", Path: "/users", Status: 200,
		ContentType: "application/json", RespBody: []byte(`{"users":[]}`),
	})

	tt := newTester(t, a)
	tt.Frame()
	if !tt.HasText(locales.MockEditTitle()) {
		t.Fatalf("mocking a response did not open its editor: %v", tt.Texts())
	}
}

func TestTheFlowMenuSavesARequestAndMakesAMock(t *testing.T) {
	a := newStoredTestApp(t)
	a.mode = modeInspector
	seedFlows(a)
	a.follow = false
	tt := newTester(t, a)
	tt.Frame()

	if err := tt.RightClick("/anything"); err != nil {
		t.Fatalf("right-click the flow: %v", err)
	}
	tt.Frame()
	menu := tt.Menu()
	for _, item := range []string{locales.InspectorSaveAsRequest(), locales.InspectorMockResponse()} {
		if !slices.Contains(menu, item) {
			t.Fatalf("the flow menu has no %q: %v", item, menu)
		}
	}

	if err := tt.ChooseMenuItem(locales.InspectorSaveAsRequest()); err != nil {
		t.Fatalf("choose Save as Request: %v", err)
	}
	tt.Frame()
	if a.mode != modeComposer || len(a.requests) != 4 {
		t.Errorf("Save as Request left mode=%v requests=%d", a.mode, len(a.requests))
	}

	a.mode = modeInspector
	tt.Frame()
	if err := tt.RightClick("/anything"); err != nil {
		t.Fatalf("right-click the flow again: %v", err)
	}
	tt.Frame()
	if err := tt.ChooseMenuItem(locales.InspectorMockResponse()); err != nil {
		t.Fatalf("choose Mock This Response: %v", err)
	}
	tt.Frame()
	if len(a.mocks) != 1 {
		t.Fatalf("the flow menu made %d mocks, want one", len(a.mocks))
	}
}
