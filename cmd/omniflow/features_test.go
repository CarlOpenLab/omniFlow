package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/CarlOpenLab/omniflow/internal/model"
)

// A curl command pasted into the URL field opens as the request it
// describes, rather than being inserted as text.
func TestCurlPasteOpensAsRequest(t *testing.T) {
	a := newTestApp(t)
	a.req.URL = ""
	tt := newTester(t, a)
	if err := tt.Click("Request URL"); err != nil {
		t.Fatalf("click the URL field: %v", err)
	}
	tt.SetClipboard(`curl 'https://httpbin.org/post' -X POST -H 'Content-Type: application/json' --data-raw '{"hello":"world"}'`)
	tt.Key(ui.Cmd, ui.KeyV)
	tt.Frame()
	if a.req.Method != "POST" || a.req.URL != "https://httpbin.org/post" {
		t.Fatalf("imported %s %s, want POST https://httpbin.org/post", a.req.Method, a.req.URL)
	}
	if a.req.BodyKind != model.BodyJSON || a.req.Body != `{"hello":"world"}` {
		t.Errorf("imported body %q as %q", a.req.Body, a.req.BodyKind)
	}
	if !tt.HasText("Imported from cURL") {
		t.Errorf("the notice does not say the command was imported: %q", tt.Texts())
	}
}

// The environment in use fills the placeholders of what a request sends, and
// the names it cannot fill are named rather than sent as braces.
func TestEnvironmentFillsPlaceholders(t *testing.T) {
	a := newTestApp(t)
	a.envs = []model.Environment{{
		ID: "env1", Name: "Staging",
		Vars: []model.KV{
			{ID: "v1", Enabled: true, Key: "host", Value: "staging.example.com"},
			{ID: "v2", Enabled: false, Key: "token", Value: "off"},
		},
	}}
	a.prefs.Environment = "env1"
	a.req.URL = "https://{{host}}/v1/things"
	a.req.Headers = []model.KV{{ID: "h1", Enabled: true, Key: "X-Key", Value: "{{token}}"}}
	a.req.BodyKind = model.BodyJSON
	a.req.Body = `{"at":"{{$timestamp}}","key":"{{token}}"}`
	resolved, missing := a.resolve(a.req)
	if resolved.URL != "https://staging.example.com/v1/things" {
		t.Errorf("the URL resolved to %q", resolved.URL)
	}
	if strings.Contains(resolved.Body, "{{") && !strings.Contains(resolved.Body, "{{token}}") {
		t.Errorf("a dynamic variable was not filled in: %q", resolved.Body)
	}
	if !slices.Contains(missing, "token") || slices.Contains(missing, "host") {
		t.Errorf("missing is %q, want token and not host", missing)
	}
	// A URL with a hole in it is not sent: the pane says why and where to
	// fill it in.
	a.req.URL = "https://{{host}}/{{missing}}/things"
	a.send()
	if a.sending || a.respErr == "" || !strings.Contains(a.respErr, "{{missing}}") {
		t.Fatalf("sending a URL with a missing variable said %q", a.respErr)
	}
	if !strings.Contains(a.respErr, "Staging") {
		t.Errorf("the message does not name the environment: %q", a.respErr)
	}
}

// The environment manager opens from the sidebar and shows the variables of
// the environment chosen.
func TestEnvironmentDialogOpens(t *testing.T) {
	a := newTestApp(t)
	a.envs = []model.Environment{{
		ID: "env1", Name: "Staging",
		Vars: []model.KV{{ID: "v1", Enabled: true, Key: "host", Value: "staging.example.com"}},
	}}
	a.prefs.Environment = "env1"
	tt := newTester(t, a)
	if !tt.HasText("Staging") {
		t.Fatal("the sidebar does not name the environment in use")
	}
	if err := tt.Click("Environment"); err != nil {
		t.Fatalf("open the environment menu: %v", err)
	}
	if err := tt.ChooseMenuItem("Manage Environments…"); err != nil {
		t.Fatalf("choose Manage Environments: %v", err)
	}
	tt.Frame()
	if !a.envOpen {
		t.Fatal("the manager did not open")
	}
	for _, want := range []string{"Environments", "VARIABLE", "Add row", "Done"} {
		if !tt.HasText(want) {
			t.Errorf("the manager is missing %q", want)
		}
	}
	if a.envEdit != "env1" {
		t.Errorf("the manager opened on %q, want env1", a.envEdit)
	}
	a.envOpen = false
	tt.Frame()
}

// The status column's heading filters the list, and Clear Filters brings the
// whole capture back.
func TestStatusFilterFromTheColumnHead(t *testing.T) {
	a := newTestApp(t)
	seedFlows(a)
	a.mode = modeInspector
	tt := newTester(t, a)
	if err := tt.Click("STATUS"); err != nil {
		t.Fatalf("click the status head: %v", err)
	}
	if err := tt.ChooseMenuItem("2xx · Success"); err != nil {
		t.Fatalf("choose 2xx: %v", err)
	}
	tt.Frame()
	if a.statusFilter != "2xx" {
		t.Fatalf("the filter is %q", a.statusFilter)
	}
	if got := len(a.visibleFlows()); got != 3 {
		t.Errorf("2xx shows %d flows, want 3", got)
	}
	a.statusFilter = "err"
	if got := len(a.visibleFlows()); got != 1 {
		t.Errorf("err shows %d flows, want 1", got)
	}
	a.clearFilters()
	if got := len(a.visibleFlows()); got != 4 {
		t.Errorf("clearing the filters shows %d flows, want 4", got)
	}
}

// The appearance settings steer the two layers: the material's strength and
// the card's opacity.
func TestAppearanceSettings(t *testing.T) {
	a := newTestApp(t)
	a.settingsOpen = true
	tt := newTester(t, a)
	for _, want := range []string{"Card opacity", "Material", "Deep", "Timeout", "Stop at redirects"} {
		if !tt.HasText(want) {
			t.Errorf("settings is missing %q", want)
		}
	}
	if err := tt.Click("Deep"); err != nil {
		t.Fatalf("choose Deep: %v", err)
	}
	if a.prefs.Blur != "deep" {
		t.Errorf("the material is %q, want deep", a.prefs.Blur)
	}
	if err := tt.Click("None"); err != nil {
		t.Fatalf("choose None: %v", err)
	}
	if a.prefs.Blur != "none" {
		t.Errorf("the material is %q, want none", a.prefs.Blur)
	}
	a.prefs.CardOpacity = 0.55
	if got := a.cardOpacity(); got != 0.55 {
		t.Errorf("the card opacity is %v, want 0.55", got)
	}
}

// A removal with Undo in the notice puts the thing back when it is clicked.
func TestUndoRestoresADeletedRequest(t *testing.T) {
	a := newTestApp(t)
	tt := newTester(t, a)
	a.deleteRequest(a.requests[0].ID)
	tt.Frame()
	if len(a.requests) != 2 {
		t.Fatalf("the collection holds %d requests after the delete", len(a.requests))
	}
	if !tt.HasText("Deleted") {
		t.Fatal("the notice does not say what was deleted")
	}
	if err := tt.Click("Undo"); err != nil {
		t.Fatalf("click Undo: %v", err)
	}
	tt.Frame()
	if len(a.requests) != 3 {
		t.Errorf("Undo left %d requests, want 3", len(a.requests))
	}
}

// The row check box is a wash, but it still turns a row out of the request.
func TestWashCheckboxTogglesARow(t *testing.T) {
	a := newTestApp(t)
	a.req.Params = []model.KV{{ID: "p1", Enabled: true, Key: "page", Value: "2"}}
	tt := newTester(t, a)
	if err := tt.Click("Send this row"); err != nil {
		t.Fatalf("click the check box: %v", err)
	}
	tt.Frame()
	if a.req.Params[0].Enabled {
		t.Error("the row is still sent")
	}
	if err := tt.Click("Send this row"); err != nil {
		t.Fatalf("click the check box again: %v", err)
	}
	tt.Frame()
	if !a.req.Params[0].Enabled {
		t.Error("the row was not turned back on")
	}
}
