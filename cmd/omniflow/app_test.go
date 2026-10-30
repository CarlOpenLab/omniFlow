package main

import (
	"bytes"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/CarlOpenLab/omniflow/internal/httpx"
	"github.com/CarlOpenLab/omniflow/internal/model"
	"github.com/CarlOpenLab/omniflow/internal/proxy"
	"github.com/CarlOpenLab/omniflow/internal/uitokens"
)

func seedFlows(a *app) {
	start := time.Date(2026, 10, 30, 15, 26, 12, 0, time.Local)
	// Oldest first, as the capture keeps them.
	a.flows = []proxy.Flow{
		{ID: 1, At: start, Method: "GET", Scheme: "https", Host: "cdn.example.net",
			Path: "/assets/app.js", URL: "https://cdn.example.net/assets/app.js", TLS: true,
			Status: 0, ContentType: "text/javascript", Err: "dial tcp: lookup cdn.example.net: no such host",
			Duration: 1 * time.Second},
		{ID: 2, At: start.Add(time.Second), Method: "GET", Scheme: "http", Host: "example.com",
			Path: "/", URL: "http://example.com/", Status: 200, StatusText: "OK",
			ContentType: "text/html", RespBody: []byte("<h1>Example</h1>"), RespSize: 2400,
			Duration: 96 * time.Millisecond},
		{ID: 3, At: start.Add(2 * time.Second), Method: "POST", Scheme: "https", Host: "httpbin.org",
			Path: "/anything", URL: "https://httpbin.org/anything", TLS: true,
			Status: 201, StatusText: "Created", ContentType: "application/json",
			ReqHeaders: []proxy.HeaderPair{{Key: "Content-Type", Value: "application/json"}},
			ReqBody:    []byte("{\"hello\":\"world\"}"),
			RespBody:   []byte("{\"json\":{\"hello\":\"world\"},\"method\":\"POST\"}"),
			RespSize:   1284, Duration: 512 * time.Millisecond},
		{ID: 4, At: start.Add(3 * time.Second), Method: "GET", Scheme: "https", Host: "api.github.com",
			Path: "/repos/egoist/mygo", URL: "https://api.github.com/repos/egoist/mygo", TLS: true,
			Status: 200, StatusText: "OK", ContentType: "application/json; charset=utf-8",
			ReqHeaders:  []proxy.HeaderPair{{Key: "Accept", Value: "application/json"}, {Key: "User-Agent", Value: "OmniFlow/0.1"}},
			RespHeaders: []proxy.HeaderPair{{Key: "Content-Type", Value: "application/json; charset=utf-8"}, {Key: "Server", Value: "GitHub.com"}},
			RespBody:    []byte("{\n  \"id\": 1234,\n  \"name\": \"mygo\",\n  \"owner\": {\"login\": \"egoist\"},\n  \"stargazers_count\": 842\n}"),
			RespSize:    38712, Duration: 284 * time.Millisecond},
	}
}

func newTester(t *testing.T, a *app) *ui.Tester {
	t.Helper()
	tt := ui.NewTester(a.view, 1280, 820)
	tt.SetTitleBar(ui.TitleBar{Height: 52, Left: 78})
	return tt
}

func TestComposerSurfaces(t *testing.T) {
	a := newTestApp(t)
	tt := newTester(t, a)
	for _, want := range []string{
		"Composer", "COLLECTIONS", "GitHub — Repository", "HTTPBin — Echo JSON",
		"Params", "Headers", "Body", "Auth", "Response", "Nothing sent yet", "Send",
	} {
		if !tt.HasText(want) {
			t.Errorf("composer is missing %q", want)
		}
	}
	if a.mode != modeComposer {
		t.Fatalf("starts in mode %v", a.mode)
	}
}

func TestModeSwitch(t *testing.T) {
	a := newTestApp(t)
	tt := newTester(t, a)
	if err := tt.Click("Inspector"); err != nil {
		t.Fatalf("click Inspector: %v", err)
	}
	if a.mode != modeInspector {
		t.Fatalf("Inspector click left mode %v", a.mode)
	}
	if !tt.HasText("The proxy is paused") || !tt.HasText("Start Capture") {
		t.Errorf("inspector empty state is missing")
	}
	if err := tt.Click("Composer"); err != nil {
		t.Fatalf("click Composer: %v", err)
	}
	if a.mode != modeComposer {
		t.Fatalf("Composer click left mode %v", a.mode)
	}
}

func TestRequestTabs(t *testing.T) {
	a := newTestApp(t)
	a.req = a.requests[0].Clone()
	tt := newTester(t, a)
	if err := tt.Click("Headers"); err != nil {
		t.Fatalf("click Headers: %v", err)
	}
	if a.reqTab != 1 {
		t.Fatalf("Headers tab left reqTab %d", a.reqTab)
	}
	if !tt.HasText("No headers yet.") {
		t.Errorf("headers tab did not show its empty state")
	}
}

func TestFlowClickAndReplay(t *testing.T) {
	a := newTestApp(t)
	seedFlows(a)
	a.mode = modeInspector
	a.follow = false
	a.flowSelID = 0
	tt := newTester(t, a)
	label := "GET api.github.com/repos/egoist/mygo"
	if err := tt.Click(label); err != nil {
		t.Fatalf("click flow: %v", err)
	}
	if a.flowSelID != 4 {
		t.Fatalf("clicking the row selected %d, want 4", a.flowSelID)
	}
	if a.follow {
		t.Errorf("a manual choice should stop following")
	}
	if !tt.HasText("Send to Composer") {
		t.Errorf("detail pane did not open")
	}
	// A double click replays the exchange in the composer.
	if err := tt.Click(label); err != nil {
		t.Fatalf("double click flow: %v", err)
	}
	tt.Frame()
	if a.mode != modeComposer {
		t.Fatalf("double click left mode %v", a.mode)
	}
	if a.req.URL != "https://api.github.com/repos/egoist/mygo" {
		t.Fatalf("replayed URL is %q", a.req.URL)
	}
}

func TestSettingsDialog(t *testing.T) {
	a := newTestApp(t)
	a.settingsOpen = true
	tt := newTester(t, a)
	for _, want := range []string{
		"Settings", "APPEARANCE", "Theme", "Acrylic background", "CAPTURE", "Proxy port",
		"DATA", "Keep request history", "Show in Finder",
	} {
		if !tt.HasText(want) {
			t.Errorf("settings is missing %q", want)
		}
	}
	if err := tt.Click("Done"); err != nil {
		t.Fatalf("close settings: %v", err)
	}
	if a.settingsOpen {
		t.Errorf("Done left the settings open")
	}
}

func TestCADialog(t *testing.T) {
	a := newTestApp(t)
	a.caOpen = true
	tt := newTester(t, a)
	for _, want := range []string{"Root CA Certificate", "Save Certificate…", "Reveal in Finder"} {
		if !tt.HasText(want) {
			t.Errorf("certificate dialog is missing %q", want)
		}
	}
	if !strings.Contains(a.ca.CertPath(), "omniflow-ca.pem") {
		t.Errorf("certificate path is %q", a.ca.CertPath())
	}
	if err := tt.Click("Done"); err != nil {
		t.Fatalf("close certificate dialog: %v", err)
	}
	if a.caOpen {
		t.Errorf("Done left the certificate dialog open")
	}
}

func TestFlowFilterAndEmptyState(t *testing.T) {
	a := newTestApp(t)
	seedFlows(a)
	a.mode = modeInspector
	a.follow = false
	tt := newTester(t, a)
	a.flowFilter = "definitely-no-such-flow"
	tt.Frame()
	if !tt.HasText("No flows match this filter") {
		t.Errorf("filtered list has no empty state")
	}
	if err := tt.Click("Clear Filters"); err != nil {
		t.Fatalf("clear filters: %v", err)
	}
	if a.flowFilter != "" || a.domainFilter != "" {
		t.Errorf("filters are %q / %q, want empty", a.flowFilter, a.domainFilter)
	}
}

func TestTimingPhasesRunEndToEnd(t *testing.T) {
	ms := time.Millisecond
	res := &httpx.Result{Duration: 284 * ms, TTFB: 182 * ms, DNS: 12 * ms, Connect: 28 * ms, TLS: 46 * ms}
	want := []struct {
		name       string
		start, dur time.Duration
	}{
		{"DNS", 0, 12 * ms},
		{"Connect", 12 * ms, 28 * ms},
		{"TLS", 40 * ms, 46 * ms},
		{"Waiting", 86 * ms, 96 * ms},
		{"Download", 182 * ms, 102 * ms},
	}
	got := timingPhases(res, uitokens.For(false))
	if len(got) != len(want) {
		t.Fatalf("%d phases, want %d", len(got), len(want))
	}
	for i, w := range want {
		if got[i].name != w.name || got[i].start != w.start || got[i].dur != w.dur {
			t.Errorf("phase %d is %s at %v for %v, want %s at %v for %v",
				i, got[i].name, got[i].start, got[i].dur, w.name, w.start, w.dur)
		}
	}
}

func TestNoticeLeavesOnItsOwn(t *testing.T) {
	a := newTestApp(t)
	tt := newTester(t, a)
	a.setNotice("Response copied")
	tt.Frame()
	if !tt.HasText("Response copied") {
		t.Fatal("the notice did not show")
	}
	tt.Advance(noticeLife - time.Second)
	if !tt.HasText("Response copied") {
		t.Error("the notice left early")
	}
	// No input comes: the frame that takes the notice down is the view's own.
	tt.Advance(2 * time.Second)
	if tt.HasText("Response copied") {
		t.Error("the notice stayed in a window left alone")
	}
}

func TestCancelDropsTheLateAnswer(t *testing.T) {
	release := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer upstream.Close()
	defer close(release)

	a := newTestApp(t)
	a.req.URL = upstream.URL
	tt := newTester(t, a)
	a.send()
	seq := a.sendSeq
	tt.Frame()
	if !a.sending || !tt.HasText("Waiting for the server") {
		t.Fatal("the pane is not waiting for the answer")
	}
	tt.Key(ui.Cmd, ui.KeyPeriod)
	if a.sending {
		t.Fatal("⌘. did not cancel the request")
	}
	if !tt.HasText("Request canceled") {
		t.Error("the pane does not say the request was canceled")
	}
	// The canceled request's answer comes after all, and is dropped.
	a.finishSend(seq, a.req, a.req.URL, httpx.Result{Status: http.StatusOK})
	if a.resp != nil {
		t.Error("a canceled request's answer was shown")
	}
	if len(a.history) != 0 {
		t.Error("a canceled request went into the history")
	}
}

func TestHistoryReopensTheWholeRequest(t *testing.T) {
	a := newTestApp(t)
	sent := a.requests[1].Clone() // a POST with a header and a JSON body
	a.addHistory(sent, sent.URL, httpx.Result{Status: http.StatusOK, Duration: 40 * time.Millisecond})
	a.loadHistory(a.history[0])
	if a.req.Method != "POST" || a.req.Body != sent.Body || a.req.BodyKind != model.BodyJSON {
		t.Errorf("history opened %s with body %q (%s)", a.req.Method, a.req.Body, a.req.BodyKind)
	}
	if len(a.req.Headers) != 1 || a.req.Headers[0].Key != "Content-Type" {
		t.Errorf("history opened headers %+v", a.req.Headers)
	}
	if a.savedSel != "" || a.req.ID == sent.ID {
		t.Error("a request from history should open as a new draft")
	}

	// An entry written before history kept whole requests opens its method
	// and URL.
	a.loadHistory(model.HistoryEntry{ID: "old", Method: "DELETE", URL: "https://example.com/things/1"})
	if a.req.Method != "DELETE" || a.req.URL != "https://example.com/things/1" || a.req.Body != "" {
		t.Errorf("an old entry opened %s %s with body %q", a.req.Method, a.req.URL, a.req.Body)
	}
}

func TestEditsSurviveSwitchingRequests(t *testing.T) {
	a := newTestApp(t)
	first, second := a.requests[0], a.requests[1]
	edited := first.URL + "?edited=1"
	a.req.URL = edited
	if !a.edited() {
		t.Fatal("an edited URL did not mark the request as edited")
	}
	a.loadRequest(second)
	if !a.hasDraft(first.ID) {
		t.Fatal("switching away dropped the edits")
	}
	a.loadRequest(first)
	if a.req.URL != edited {
		t.Fatalf("coming back opened %q, want the draft %q", a.req.URL, edited)
	}
	a.saveRequest()
	if a.edited() || a.hasDraft(first.ID) {
		t.Error("the request still reads as edited after saving")
	}
	if a.requests[0].URL != edited {
		t.Errorf("the collection has %q, want %q", a.requests[0].URL, edited)
	}
}

func TestRenameInPlace(t *testing.T) {
	a := newTestApp(t)
	tt := newTester(t, a)
	// A double click: two clicks while the tester's clock stands still.
	for range 2 {
		if err := tt.Click("Request name"); err != nil {
			t.Fatalf("click the title: %v", err)
		}
	}
	tt.Type("Repository details")
	tt.Key(0, ui.KeyEnter)
	if a.req.Name != "Repository details" {
		t.Fatalf("the request is named %q", a.req.Name)
	}
	if a.requests[0].Name != "Repository details" {
		t.Errorf("the collection still names it %q", a.requests[0].Name)
	}
	if a.edited() {
		t.Error("a rename alone left the request edited")
	}
}

func TestDuplicateFromTheRowsMenu(t *testing.T) {
	a := newTestApp(t)
	tt := newTester(t, a)
	if err := tt.RightClick("Postman Echo — Query"); err != nil {
		t.Fatalf("right click: %v", err)
	}
	if menu := tt.Menu(); !slices.Contains(menu, "Duplicate") || !slices.Contains(menu, "Copy as cURL") {
		t.Fatalf("the row's menu is %q", menu)
	}
	if err := tt.ChooseMenuItem("Duplicate"); err != nil {
		t.Fatalf("choose Duplicate: %v", err)
	}
	tt.Frame()
	if len(a.requests) != 4 || a.requests[3].Name != "Postman Echo — Query Copy" {
		t.Fatalf("the collection is %d requests, the last %q", len(a.requests), a.requests[len(a.requests)-1].Name)
	}
	if a.savedSel != a.requests[3].ID {
		t.Error("the copy did not open")
	}
	if a.requests[3].URL != a.requests[2].URL || len(a.requests[3].Params) != 2 {
		t.Error("the copy is not the same request")
	}
}

func TestArrowsWalkTheCapture(t *testing.T) {
	a := newTestApp(t)
	seedFlows(a)
	a.mode = modeInspector
	a.follow = false
	a.flowSelID = 0
	tt := newTester(t, a)
	// The list shows the oldest flow first.
	for _, step := range []struct {
		key  ui.Key
		want uint64
	}{{ui.KeyDown, 1}, {ui.KeyDown, 2}, {ui.KeyDown, 3}, {ui.KeyUp, 2}, {ui.KeyUp, 1}, {ui.KeyUp, 1}} {
		tt.Key(0, step.key)
		if a.flowSelID != step.want {
			t.Fatalf("after %v the flow chosen is %d, want %d", step.key, a.flowSelID, step.want)
		}
	}
}

func TestFormBodyIsATable(t *testing.T) {
	a := newTestApp(t)
	a.req.Method = "POST"
	a.req.BodyKind = model.BodyForm
	a.req.Form = []model.KV{{ID: "f1", Enabled: true, Key: "name", Value: "Omni Flow"}}
	a.reqTab = 2
	tt := newTester(t, a)
	for _, want := range []string{"Body · Form", "FIELD", "x-www-form-urlencoded", "1 field"} {
		if !tt.HasText(want) {
			t.Errorf("the form body is missing %q", want)
		}
	}
	cmd, err := httpx.Curl(specOf(a.req))
	if err != nil {
		t.Fatalf("Curl: %v", err)
	}
	for _, want := range []string{"--data-raw 'name=Omni+Flow'", "Content-Type: application/x-www-form-urlencoded"} {
		if !strings.Contains(cmd, want) {
			t.Errorf("the form sends\n%s\nwithout %q", cmd, want)
		}
	}
}

func TestPortIsCheckedBeforeTheProxyStarts(t *testing.T) {
	a := newTestApp(t)
	a.mode = modeInspector
	a.proxyPort = "99999"
	tt := newTester(t, a)
	if !tt.HasText(errPort) {
		t.Error("the sidebar does not flag the port")
	}
	a.startProxy()
	if a.proxyRunning() || a.proxyErr != errPort {
		t.Errorf("the proxy started on %s, or said %q", a.proxyPort, a.proxyErr)
	}
}

func TestCopyAsCurl(t *testing.T) {
	a := newTestApp(t)
	tt := newTester(t, a)
	tt.Key(ui.Cmd|ui.Shift, ui.KeyC)
	if got, want := tt.Clipboard(), "curl --location 'https://api.github.com/repos/egoist/mygo'"; got != want {
		t.Errorf("the composer copied %q, want %q", got, want)
	}

	// The inspector copies the flow chosen.
	seedFlows(a)
	a.mode = modeInspector
	a.follow = false
	a.flowSelID = 3
	tt.Frame()
	tt.Key(ui.Cmd|ui.Shift, ui.KeyC)
	got := tt.Clipboard()
	for _, want := range []string{"curl 'https://httpbin.org/anything'", "-H 'Content-Type: application/json'", `--data-raw '{"hello":"world"}'`} {
		if !strings.Contains(got, want) {
			t.Errorf("the flow copied as\n%s\nwithout %q", got, want)
		}
	}
}

// tinyPNG is a 3×2 picture.
func tinyPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 3, 2))
	for i := range img.Pix {
		img.Pix[i] = 0xff
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode: %v", err)
	}
	return buf.Bytes()
}

func TestBodiesShowInTheirShape(t *testing.T) {
	pic := tinyPNG(t)
	for _, tc := range []struct {
		body []byte
		ct   string
		want bodyShape
	}{
		{nil, "", shapeText},
		{[]byte(`{"a":1}`), "application/json", shapeText},
		{[]byte("plain words"), "", shapeText},
		{[]byte("<p>caf\xe9</p>"), "text/html; charset=iso-8859-1", shapeText},
		{[]byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`), "image/svg+xml", shapeText},
		{pic, "image/png", shapeImage},
		{[]byte{0x00, 0x01, 0x02, 0xff}, "application/octet-stream", shapeBinary},
		{[]byte{0x00, 0x01, 0x02, 0xff}, "", shapeBinary},
	} {
		if got := shapeOf(tc.body, tc.ct); got != tc.want {
			t.Errorf("shapeOf(%q, %q) = %d, want %d", tc.body, tc.ct, got, tc.want)
		}
	}
	for _, tc := range []struct{ url, ct, want string }{
		{"https://example.com/assets/logo?v=2", "image/png", "logo.png"},
		{"https://example.com/photo", "image/jpeg", "photo.jpg"},
		{"https://example.com/", "application/json; charset=utf-8", "body.json"},
		{"https://example.com/files/report.pdf", "application/pdf", "report.pdf"},
		{"example.com", "", "body"},
	} {
		if got := fileNameFor(tc.url, tc.ct); got != tc.want {
			t.Errorf("fileNameFor(%q, %q) = %q, want %q", tc.url, tc.ct, got, tc.want)
		}
	}
}

func TestImageBodyIsShownAsAPicture(t *testing.T) {
	a := newTestApp(t)
	pic := tinyPNG(t)
	a.resp = &httpx.Result{
		Status: http.StatusOK, StatusText: "OK", Proto: "HTTP/1.1",
		Headers: []httpx.Header{{Key: "Content-Type", Value: "image/png"}},
		Body:    pic, Size: int64(len(pic)), Duration: 30 * time.Millisecond, TTFB: 20 * time.Millisecond,
	}
	tt := newTester(t, a)
	for _, want := range []string{"3 × 2 px", "Save Body…"} {
		if !tt.HasText(want) {
			t.Errorf("the picture is missing %q", want)
		}
	}
	if tt.HasText("Copy Response") {
		t.Error("a picture offers to be copied as text")
	}
	if a.preview.bmp == nil {
		t.Error("the picture was not decoded")
	}
}
