package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

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
