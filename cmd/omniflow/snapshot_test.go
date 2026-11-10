package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"

	"github.com/CarlOpenLab/omniflow/internal/httpx"
	"github.com/CarlOpenLab/omniflow/internal/model"
	"github.com/CarlOpenLab/omniflow/internal/proxy"
	"github.com/CarlOpenLab/omniflow/internal/uitokens"
)

func newTestApp(t *testing.T) *app {
	t.Helper()
	dir := t.TempDir()
	ca, err := proxy.LoadOrCreateCA(dir)
	if err != nil {
		t.Fatalf("create CA: %v", err)
	}
	a := &app{
		client:        httpx.New(),
		prefs:         model.DefaultPrefs(),
		requests:      model.SeedRequests(),
		pretty:        true,
		composerSplit: 44,
		inspectSplit:  46,
		proxyPort:     "9090",
		mitm:          true,
		timeoutText:   "30",
		follow:        true,
		ca:            ca,
		prx:           proxy.New(ca),
	}
	a.req = a.requests[0].Clone()
	a.savedSel = a.req.ID
	return a
}

// snapshotPNG is a picture to show as a response body: a soft diagonal
// gradient, w by h pixels.
func snapshotPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			f := float64(x+y) / float64(w+h)
			img.Set(x, y, color.RGBA{R: uint8(59 + 120*f), G: uint8(108 + 60*f), B: 246, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode: %v", err)
	}
	return buf.Bytes()
}

func saveSnapshot(t *testing.T, tester *ui.Tester, path string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	defer f.Close()
	if err := png.Encode(f, tester.Image()); err != nil {
		t.Fatalf("encode %s: %v", path, err)
	}
}

// TestSnapshots renders the main surfaces to /tmp for visual review.
// Run with OMNIFLOW_SNAPSHOT=1 go test ./cmd/omniflow -run TestSnapshots.
func TestSnapshots(t *testing.T) {
	if os.Getenv("OMNIFLOW_SNAPSHOT") == "" {
		t.Skip("set OMNIFLOW_SNAPSHOT=1 to write snapshots")
	}
	out := os.Getenv("OMNIFLOW_SNAPSHOT_DIR")
	if out == "" {
		out = "/tmp"
	}
	a := newTestApp(t)

	tester := ui.NewTester(a.view, 1280, 820)
	tester.SetTitleBar(ui.TitleBar{Height: 52, Left: 78})
	// The headless frame has no system material, so the app draws its acrylic
	// fallback wash: the ground these snapshots review.
	tester.SetVibrancy(false)
	saveSnapshot(t, tester, out+"/omniflow-composer-light.png")
	tester.SetDark(true)
	saveSnapshot(t, tester, out+"/omniflow-composer-dark.png")
	tester.SetDark(false)

	a.mode = modeInspector
	seedFlows(a)
	a.flowSelID = 4
	a.follow = false
	a.detailTab = 1
	tester.Frame()
	saveSnapshot(t, tester, out+"/omniflow-inspector-light.png")

	tester.SetDark(true)
	saveSnapshot(t, tester, out+"/omniflow-inspector-dark.png")
	tester.SetDark(false)

	// The Inspector's mock rules panel, and the dialog that edits a mock's
	// response where it lives.
	mockFile := filepath.Join(t.TempDir(), "users.json")
	if err := os.WriteFile(mockFile, []byte("{\n  \"users\": []\n}"), 0o644); err != nil {
		t.Fatalf("write mock file: %v", err)
	}
	a.mocks = []model.MockRule{{
		ID: "mock_demo", Enabled: true, Name: "GET api.github.com/repos/egoist/mygo",
		Method: "GET", Host: "api.github.com", Path: "/repos/egoist/mygo",
		Status: 200, ContentType: "application/json", File: mockFile,
		CreatedAt: time.Now(),
	}}
	tester.Frame()
	saveSnapshot(t, tester, out+"/omniflow-inspector-mocks.png")
	a.openMockEditor(a.mocks[0])
	tester.Frame()
	saveSnapshot(t, tester, out+"/omniflow-mock-editor.png")
	a.mockOpen, a.mockEditID, a.mockBody = false, "", nil
	a.mocks = nil

	a.mode = modeComposer
	a.settingsOpen = true
	tester.Frame()
	saveSnapshot(t, tester, out+"/omniflow-settings-light.png")
	a.settingsOpen = false

	a.caOpen = true
	tester.Frame()
	saveSnapshot(t, tester, out+"/omniflow-ca-light.png")
	a.caOpen = false

	// The composer's editor tabs and a completed response.
	a.mode = modeComposer
	a.req = a.requests[1].Clone()
	a.savedSel = a.req.ID
	a.reqTab = 1
	tester.Frame()
	saveSnapshot(t, tester, out+"/omniflow-composer-headers.png")
	a.reqTab = 2
	tester.Frame()
	saveSnapshot(t, tester, out+"/omniflow-composer-body.png")

	a.req = a.requests[0].Clone()
	a.savedSel = a.req.ID
	a.req.Params = []model.KV{
		{ID: "p1", Enabled: true, Key: "page", Value: "2"},
		{ID: "p2", Enabled: true, Key: "sort", Value: "updated"},
		{ID: "p3", Enabled: false, Key: "draft", Value: "true"},
	}
	a.reqTab = 0
	a.resp = &httpx.Result{
		Status: 200, StatusText: "OK", Proto: "HTTP/1.1",
		Headers: []httpx.Header{
			{Key: "Content-Type", Value: "application/json; charset=utf-8"},
			{Key: "Cache-Control", Value: "public, max-age=60"},
			{Key: "X-RateLimit-Remaining", Value: "33"},
		},
		Body: []byte(`{"id":1234,"name":"mygo","owner":{"login":"egoist"},"stargazers_count":842}`),
		Size: 38712, Duration: 284 * time.Millisecond, TTFB: 182 * time.Millisecond,
		DNS: 12 * time.Millisecond, Connect: 28 * time.Millisecond, TLS: 46 * time.Millisecond,
	}
	a.respTab = 0
	tester.Frame()
	saveSnapshot(t, tester, out+"/omniflow-composer-response.png")
	a.respTab = 1
	tester.Frame()
	saveSnapshot(t, tester, out+"/omniflow-composer-response-headers.png")
	a.respTab = 2
	tester.Frame()
	saveSnapshot(t, tester, out+"/omniflow-composer-timing.png")

	// A picture, and bytes that are not text.
	a.respTab = 0
	a.resp.Headers = []httpx.Header{{Key: "Content-Type", Value: "image/png"}}
	a.resp.Body = snapshotPNG(t, 360, 220)
	tester.Frame()
	saveSnapshot(t, tester, out+"/omniflow-composer-image.png")
	a.resp.Headers = []httpx.Header{{Key: "Content-Type", Value: "application/octet-stream"}}
	a.resp.Body = []byte{0x00, 0x9c, 0xff, 0x10, 0x00}
	tester.Frame()
	saveSnapshot(t, tester, out+"/omniflow-composer-binary.png")

	a.resp = nil
	a.respErr = "dial tcp: lookup api.void.example: no such host"
	a.respProblem = respOK
	tester.Frame()
	saveSnapshot(t, tester, out+"/omniflow-composer-error.png")
	a.respErr = ""
	a.respProblem = respCanceled
	tester.Frame()
	saveSnapshot(t, tester, out+"/omniflow-composer-canceled.png")
	a.respProblem = respOK

	// A form body, written as a table.
	a.req = a.requests[1].Clone()
	a.savedSel = a.req.ID
	a.req.BodyKind = model.BodyForm
	a.req.Form = []model.KV{
		{ID: "f1", Enabled: true, Key: "name", Value: "OmniFlow"},
		{ID: "f2", Enabled: true, Key: "tags", Value: "http, proxy"},
		{ID: "f3", Enabled: false, Key: "debug", Value: "1"},
	}
	a.reqTab = 2
	tester.Frame()
	saveSnapshot(t, tester, out+"/omniflow-composer-form.png")
	a.req.BodyKind = model.BodyJSON
	a.req.Body = "{\n  \"hello\": \"world\",\n  \"from\": OmniFlow\n}"
	tester.Frame()
	saveSnapshot(t, tester, out+"/omniflow-composer-json-invalid.png")
	a.reqTab = 0

	a.methodOpen = true
	tester.Frame()
	saveSnapshot(t, tester, out+"/omniflow-composer-method.png")
	a.methodOpen = false

	// The environment manager, opened on an environment with variables the
	// request uses.
	a.mode = modeComposer
	a.envs = []model.Environment{{
		ID: "env1", Name: "Staging",
		Vars: []model.KV{
			{ID: "e1", Enabled: true, Key: "host", Value: "staging.example.com"},
			{ID: "e2", Enabled: true, Key: "region", Value: "eu-west-1"},
			{ID: "e3", Enabled: false, Key: "token", Value: "s3cret"},
		},
	}}
	a.prefs.Environment = "env1"
	a.envEdit, a.envOpen = "env1", true
	a.req = a.requests[0].Clone()
	a.req.URL = "https://{{host}}/v1/things?region={{region}}"
	a.savedSel = a.req.ID
	a.reqTab = 0
	tester.Frame()
	saveSnapshot(t, tester, out+"/omniflow-environments-light.png")
	tester.SetDark(true)
	tester.Frame()
	saveSnapshot(t, tester, out+"/omniflow-environments-dark.png")
	tester.SetDark(false)
	a.envOpen, a.prefs.Environment = false, ""
	a.req = a.requests[0].Clone()
	a.savedSel = a.req.ID

	a.settingsOpen = true
	tester.SetDark(true)
	tester.Frame()
	saveSnapshot(t, tester, out+"/omniflow-settings-dark.png")
	tester.SetDark(false)
	a.settingsOpen = false

	// A flow that never got an answer.
	a.mode = modeInspector
	a.flowSelID = 1
	a.detailTab = 1
	tester.Frame()
	saveSnapshot(t, tester, out+"/omniflow-inspector-error.png")
	a.flowSelID = 4

	// The minimum window, to check the layout holds together small.
	a.mode = modeInspector
	a.resp = nil
	tester.SetSize(1020, 620)
	tester.Frame()
	saveSnapshot(t, tester, out+"/omniflow-inspector-min.png")
	a.mode = modeComposer
	tester.Frame()
	saveSnapshot(t, tester, out+"/omniflow-composer-min.png")

	// The same surfaces in the app's other language, to review the copy in
	// the places it has to fit.
	i18n.SetLocaleFunc(func() string { return "zh-Hans" })
	defer i18n.SetLocaleFunc(func() string { return "en" })
	tester.SetSize(1280, 820)
	a.settingsOpen, a.caOpen, a.envOpen = false, false, false
	tester.Frame()
	saveSnapshot(t, tester, out+"/omniflow-composer-zh.png")

	a.mode = modeInspector
	tester.Frame()
	saveSnapshot(t, tester, out+"/omniflow-inspector-zh.png")

	a.mocks = []model.MockRule{{
		ID: "mock_demo", Enabled: true, Name: "GET api.github.com/repos/egoist/mygo",
		Method: "GET", Host: "api.github.com", Path: "/repos/egoist/mygo",
		Status: 200, ContentType: "application/json", File: mockFile,
		CreatedAt: time.Now(),
	}}
	tester.Frame()
	saveSnapshot(t, tester, out+"/omniflow-inspector-mocks-zh.png")
	a.openMockEditor(a.mocks[0])
	tester.Frame()
	saveSnapshot(t, tester, out+"/omniflow-mock-editor-zh.png")
	a.mockOpen, a.mockEditID, a.mockBody = false, "", nil
	a.mocks = nil

	a.settingsOpen = true
	tester.Frame()
	saveSnapshot(t, tester, out+"/omniflow-settings-zh.png")
	a.settingsOpen = false

	a.caOpen = true
	tester.Frame()
	saveSnapshot(t, tester, out+"/omniflow-ca-zh.png")
	a.caOpen = false

	a.mode = modeComposer
	a.envs = []model.Environment{{
		ID: "env1", Name: "预发布",
		Vars: []model.KV{{ID: "e1", Enabled: true, Key: "host", Value: "staging.example.com"}},
	}}
	a.envEdit, a.envOpen = "env1", true
	tester.Frame()
	saveSnapshot(t, tester, out+"/omniflow-environments-zh.png")
	a.envOpen = false
}

// TestStressLayers renders the window over a magenta ground with the card at
// 30%: wherever the frost should show through, a solid fill shows up as a
// patch that keeps the magenta out. Only code canvases and popovers may be
// solid. Run with OMNIFLOW_SNAPSHOT=1 to write the pictures.
func TestStressLayers(t *testing.T) {
	if os.Getenv("OMNIFLOW_SNAPSHOT") == "" {
		t.Skip("set OMNIFLOW_SNAPSHOT=1 to write snapshots")
	}
	out := os.Getenv("OMNIFLOW_SNAPSHOT_DIR")
	if out == "" {
		out = "/tmp"
	}
	a := newTestApp(t)
	a.prefs.CardOpacity = uitokens.OpacityMin
	seedFlows(a)
	a.flowSelID = 4

	// The shell without its ground: the magenta shows wherever a layer lets
	// the window through.
	shell := func(c *ui.Context) {
		applyTheme(c, a.prefs.Theme)
		pal := paletteFor(c)
		c.Root().Background(ui.Hex("#ff00ff"))
		ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
			a.sidebar(c, pal)
			main := ui.Column(c).Grow(1).MinWidth(0).
				Padding(uitokens.CardGap, uitokens.CardGap, uitokens.CardGap, uitokens.CardGap)
			main.Children(func() { a.card(c, pal) })
		})
	}
	tester := ui.NewTester(shell, 1280, 820)
	tester.SetTitleBar(ui.TitleBar{Height: 52, Left: 78})

	for _, dark := range []bool{false, true} {
		tester.SetDark(dark)
		a.mode = modeComposer
		a.req = a.requests[0].Clone()
		a.savedSel = a.req.ID
		a.reqTab = 2
		a.req.BodyKind = model.BodyJSON
		a.req.Body = "{\n  \"hello\": \"world\"\n}"
		tester.Frame()
		name := "light"
		if dark {
			name = "dark"
		}
		saveSnapshot(t, tester, out+"/omniflow-stress-composer-"+name+".png")

		a.mode = modeInspector
		a.detailTab = 1
		tester.Frame()
		saveSnapshot(t, tester, out+"/omniflow-stress-inspector-"+name+".png")
	}
}
