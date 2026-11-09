package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"

	"github.com/CarlOpenLab/omniflow/internal/httpx"
	"github.com/CarlOpenLab/omniflow/internal/model"
	"github.com/CarlOpenLab/omniflow/internal/proxy"
	"github.com/CarlOpenLab/omniflow/internal/store"
	"github.com/CarlOpenLab/omniflow/internal/sysproxy"
	"github.com/CarlOpenLab/omniflow/locales"
)

type viewMode int

const (
	modeComposer viewMode = iota
	modeInspector
)

// respProblem is a message the app itself puts in the response pane when a
// request cannot go. The problem is kept by name, not by the sentence it was
// written as, so the pane can say it in the language on screen now.
type respProblem int

const (
	respOK respProblem = iota
	respCanceled
	respEnterURL
	respMissingVars
)

// proxyProblem is the same idea for the inspector's own reason the proxy is
// not listening.
type proxyProblem int

const (
	proxyOK proxyProblem = iota
	proxyCAUnavailable
	proxyBadPort
)

// systemProxyManager is the app's handle on the machine's system proxy.
// Tests replace it so no test changes the machine's real settings.
type systemProxyManager interface {
	Enable(port int, captureHTTPS bool) error
	Restore() error
	Recover() error
	Reset()
}

// app is the window's state. It lives on the UI thread; the proxy publishes
// to it through Window.Update, as everything off the main thread must.
type app struct {
	win *mygo.Window
	// frameStub stands in for the window when the view is drawn without one,
	// as the headless tester does.
	frameStub windowFrame
	store     *store.Store

	prefs    model.Prefs
	requests []model.Request
	history  []model.HistoryEntry
	envs     []model.Environment
	mocks    []model.MockRule
	rules    []model.ProxyRule

	client *httpx.Client
	ca     *proxy.CA
	prx    *proxy.Proxy

	mode viewMode

	// composer
	req      model.Request
	savedSel string
	// drafts are saved requests with edits not saved yet, by id: switching
	// to another request keeps them here rather than throwing them away.
	drafts        map[string]model.Request
	reqTab        int
	respTab       int
	sending       bool
	sendSeq       uint64
	sendCancel    context.CancelFunc
	resp          *httpx.Result
	respErr       string
	respProblem   respProblem
	respMissing   []string
	pretty        bool
	wrapBody      bool
	composerSplit float32
	urlFocus      ui.Handle
	methodOpen    bool
	jsonChecked   jsonCheck
	// preview is the last body drawn as an image, decoded once.
	preview bodyPreview
	// body is the editor showing the text body on show: the composer's
	// response, or the inspector's request or response body. findBody asks
	// it to open its find bar as it is built, on ⌘F.
	body     bodyEditor
	findBody bool
	// bodyTexts is the body on show, raw and formatted, read once.
	bodyTexts bodyTexts
	// pendingCurl is a curl command pasted into the URL field, opened as a
	// request in the next frame rather than in the middle of the paste.
	pendingCurl string
	sideFilter  string
	// timeoutText is the settings' timeout field, in seconds, as typed.
	timeoutText string

	// environments
	envOpen, envWasOpen bool
	envEdit             string // the environment the dialog shows, by id

	// inspector: flows are kept oldest first, as the list shows them, so a
	// new one is appended rather than copying the whole capture in front.
	flows         []proxy.Flow
	capturedBytes int64
	flowSelID     uint64
	flowFilter    string
	searchBodies  bool
	domainFilter  string
	statusFilter  string
	follow        bool
	// flowRev changes whenever the capture does, so the filter's result can be
	// reused within a frame and between frames that changed nothing.
	flowRev       uint64
	flowCache     string
	flowCached    []int
	detailTab     int
	detailSub     int
	inspectSplit  float32
	flowList      ui.ListState
	searchFocus   ui.Handle
	proxyPort     string
	proxyErr      string
	proxyProblem  proxyProblem
	mitm          bool
	autoProxy     bool
	// sysproxy owns the machine's proxy settings while capture runs;
	// sysproxyBlocked keeps a corrupt backup from being replaced by a fresh
	// snapshot until the person explicitly turns the setting back on.
	sysproxy        systemProxyManager
	sysproxyBlocked bool
	lastFlowAt      time.Time
	throttle        string
	lanAccess       bool
	// device setup: the page a phone opens, and the server behind it.
	deviceSetupOpen bool
	setupSrv        *http.Server
	setupURL        string
	setupErr        string

	// dialogs
	settingsOpen bool
	settingsPane int
	// rule dialog: ruleID is empty for a new rule, and ruleErr is the reason
	// Save did not go through.
	ruleOpen   bool
	ruleID     string
	ruleKind   string
	ruleName   string
	ruleMethod string
	ruleHost   string
	rulePath   string
	ruleTarget string
	ruleHeader string
	ruleValue  string
	ruleRemove bool
	ruleErr    string
	caOpen     bool
	caWasOpen  bool
	caTrust    trustState

	notice   string
	noticeAt time.Time
	// undo puts back what the notice says was just removed, while it shows.
	undo func()
}

func newApp() *app {
	a := &app{
		client:        httpx.New(),
		pretty:        true,
		composerSplit: 44,
		inspectSplit:  46,
		detailTab:     0,
		detailSub:     0,
	}
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		dir = filepath.Join(os.TempDir(), "omniflow")
	}
	st, err := store.Open(filepath.Join(dir, "omniflow.json"))
	if err != nil {
		a.setNotice(locales.NoticeSettingsReadFailed(err.Error()))
		st, _ = store.Open(filepath.Join(os.TempDir(), "omniflow.json"))
	}
	a.store = st
	data := st.Data()
	a.prefs = data.Prefs
	a.requests = data.Requests
	a.history = data.History
	a.envs = data.Environments
	a.mocks = data.Mocks
	a.rules = data.Rules
	a.throttle = normalizeThrottle(a.prefs.Throttle)
	a.proxyPort = a.prefs.ProxyPort
	a.mitm = a.prefs.DecryptHTTPS
	a.autoProxy = !a.prefs.DisableAutoProxy
	a.follow = a.prefs.FollowFlows
	a.lanAccess = a.prefs.LANAccess
	a.timeoutText = strconv.Itoa(int(a.prefs.Timeout() / time.Second))
	if a.prefs.ComposerSplit > 0 {
		a.composerSplit = a.prefs.ComposerSplit
	}
	if a.prefs.InspectSplit > 0 {
		a.inspectSplit = a.prefs.InspectSplit
	}

	if len(a.requests) > 0 {
		a.req = a.requests[0].Clone()
		a.savedSel = a.req.ID
	} else {
		a.req = model.NewRequest()
	}

	ca, err := proxy.LoadOrCreateCA(dir)
	if err != nil {
		a.setNotice(locales.NoticeCACreateFailed(err.Error()))
	} else {
		a.ca = ca
		a.prx = proxy.New(ca)
		a.applyMocks()
		a.applyRules()
		a.applyThrottle()
		a.prx.SetOnFlow(func(f proxy.Flow) {
			if a.win == nil {
				return
			}
			a.win.Update(func() { a.addFlow(f) })
		})
	}
	if st != nil {
		// The backup lives beside the rest of the app's data, and a
		// leftover one is put back before a new capture can start.
		a.sysproxy = sysproxy.New(filepath.Dir(st.Path()))
		a.recoverSystemProxy()
	}
	return a
}

// setAppLocale is mygo.App.SetLocale, named so tests can watch the language
// without a running app.
var setAppLocale = mygo.App.SetLocale

// normalizeLanguage reads a stored language the app does not ship as "":
// follow the system, as a value a person did not choose.
func normalizeLanguage(tag string) string {
	switch tag {
	case "en", "zh-Hans":
		return tag
	}
	return ""
}

// applyLanguage makes the app show the stored language, or the system's
// when it stores none.
func (a *app) applyLanguage() {
	setAppLocale(normalizeLanguage(a.prefs.Language))
}

// setLanguage stores and shows a language at once: "" follows the system.
func (a *app) setLanguage(tag string) {
	tag = normalizeLanguage(tag)
	if a.prefs.Language == tag {
		return
	}
	a.prefs.Language = tag
	a.savePrefs()
	setAppLocale(tag)
}

func (a *app) applyPreferences() {
	a.applyLanguage()
	applyAppearance(a.prefs.Theme)
}

// persistOnClose keeps what changes too often to write as it changes: where
// the panes were split, and the environments being edited.
func (a *app) persistOnClose() {
	a.prefs.ComposerSplit, a.prefs.InspectSplit = a.composerSplit, a.inspectSplit
	a.savePrefs()
	if a.envOpen {
		a.persistEnvs()
	}
}

// noticeLife is how long a notice stays in the card's foot.
const noticeLife = 7 * time.Second

func (a *app) setNotice(text string) {
	a.notice, a.noticeAt, a.undo = text, time.Time{}, nil
}

// setUndo is a notice of something just removed, with Undo beside it for as
// long as the notice shows.
func (a *app) setUndo(text string, undo func()) {
	a.setNotice(text)
	a.undo = undo
}

// respProblemText is the response pane's message about the request itself,
// in the language on screen now; a result's own error stays as written.
func (a *app) respProblemText() string {
	switch a.respProblem {
	case respCanceled:
		return locales.ErrRequestCanceled()
	case respEnterURL:
		return locales.ErrEnterURL()
	case respMissingVars:
		return a.missingMessage(a.respMissing)
	}
	return a.respErr
}

// proxyProblemText is the inspector's own reason the proxy is not listening,
// in the language on screen now; the proxy's error stays as written.
func (a *app) proxyProblemText() string {
	switch a.proxyProblem {
	case proxyCAUnavailable:
		return locales.NoticeProxyCAUnavailable()
	case proxyBadPort:
		return locales.ErrPort()
	}
	return a.proxyErr
}

// runUndo puts back what the notice offered to.
func (a *app) runUndo() {
	if u := a.undo; u != nil {
		a.undo = nil
		u()
	}
}

// currentNotice is the notice still on show. It is timed by the frame's
// clock from the first frame that shows it, and asks for the frame that
// takes it down: nothing else may draw one in a window left alone.
func (a *app) currentNotice(c *ui.Context) string {
	if a.notice == "" {
		return ""
	}
	now := c.Now()
	if a.noticeAt.IsZero() {
		a.noticeAt = now
	}
	left := noticeLife - now.Sub(a.noticeAt)
	if left <= 0 {
		a.notice, a.undo = "", nil
		return ""
	}
	c.After(left)
	return a.notice
}

func (a *app) update(fn func()) {
	if a.win != nil {
		a.win.Update(fn)
	}
}

// shortcuts reads the window's chords. They are not menu items only, so
// they work wherever the focus is, except inside a dialog.
func (a *app) shortcuts(c *ui.Context) {
	if a.settingsOpen || a.caOpen || a.envOpen || a.methodOpen {
		return
	}
	// A find belongs to the body it was opened over; leaving that body
	// drops its editor, bar and all, so ⌘F opens anew on the next one.
	if !a.canFind() {
		a.body = bodyEditor{}
	}
	if c.Shortcut(ui.Cmd, ui.Key1) {
		a.mode = modeComposer
	}
	if c.Shortcut(ui.Cmd, ui.Key2) {
		a.mode = modeInspector
	}
	if c.Shortcut(ui.Cmd, ui.KeyN) {
		a.newRequest()
	}
	if c.Shortcut(ui.Cmd, ui.KeyS) {
		a.saveRequest()
	}
	if c.Shortcut(ui.Cmd, ui.KeyComma) {
		a.settingsOpen = true
	}
	if c.Shortcut(ui.Cmd, ui.KeyE) {
		a.openEnvs()
	}
	// ⌘F finds in the body on show, as Safari does; the traffic filter is
	// under ⇧⌘F so the two do not take the same chord. The body's editor
	// opens its bar as it is built, from the flag.
	if c.Shortcut(ui.Cmd, ui.KeyF) {
		if a.canFind() {
			a.findBody = true
		}
	}
	if c.Shortcut(ui.Cmd|ui.Shift, ui.KeyF) {
		if a.mode == modeInspector {
			a.searchFocus.Focus()
		}
	}
	if c.Shortcut(ui.Cmd|ui.Shift, ui.KeyC) {
		a.copyCurl(c)
	}
	switch a.mode {
	case modeComposer:
		if c.Shortcut(ui.Cmd, ui.KeyEnter) {
			a.send()
		}
		// ⌘. stops what is under way, as it does across macOS.
		if c.Shortcut(ui.Cmd, ui.KeyPeriod) {
			a.cancelSend()
		}
	case modeInspector:
		// The arrows walk the capture while no field has taken them.
		if c.Shortcut(0, ui.KeyUp) {
			a.moveFlowSelection(-1)
		}
		if c.Shortcut(0, ui.KeyDown) {
			a.moveFlowSelection(1)
		}
	}
}

// --- requests -------------------------------------------------------------

func (a *app) newRequest() {
	a.open(model.NewRequest(), "")
	a.urlFocus.Focus()
}

func (a *app) loadRequest(r model.Request) {
	if r.ID == a.savedSel {
		return
	}
	req := r.Clone()
	if d, ok := a.drafts[r.ID]; ok {
		req = d.Clone()
	}
	req.Saved = true
	a.open(req, r.ID)
}

// open puts a request in the composer, keeping the edits of the one it
// replaces as a draft, and clears the answer that belonged to that one.
func (a *app) open(r model.Request, savedID string) {
	a.stashDraft()
	a.cancelSend()
	a.req = r
	a.savedSel = savedID
	a.reqTab = 0
	a.respTab = 0
	a.resp = nil
	a.respErr, a.respProblem, a.respMissing = "", respOK, nil
	a.methodOpen = false
}

// stashDraft keeps the composer's edits to a saved request, or forgets the
// draft once there are none.
func (a *app) stashDraft() {
	if a.savedSel == "" {
		return
	}
	if a.edited() {
		if a.drafts == nil {
			a.drafts = map[string]model.Request{}
		}
		a.drafts[a.savedSel] = a.req.Clone()
	} else {
		delete(a.drafts, a.savedSel)
	}
}

// savedRequest is the collection's copy of the request in the composer.
func (a *app) savedRequest() (model.Request, bool) {
	if a.savedSel == "" {
		return model.Request{}, false
	}
	for _, r := range a.requests {
		if r.ID == a.savedSel {
			return r, true
		}
	}
	return model.Request{}, false
}

// edited reports whether the composer holds what the collection does not:
// edits to a saved request, or a new request once it has a URL.
func (a *app) edited() bool {
	if saved, ok := a.savedRequest(); ok {
		return !a.req.SameContent(saved)
	}
	return strings.TrimSpace(a.req.URL) != ""
}

// hasDraft reports whether a saved request has edits not saved yet.
func (a *app) hasDraft(id string) bool {
	if id == a.savedSel {
		return a.edited()
	}
	_, ok := a.drafts[id]
	return ok
}

// renameRequest names the request in the composer. A saved request takes
// the name at once, as a file renamed in Finder does, without saving any
// other edit along with it.
func (a *app) renameRequest(name string) {
	name = strings.TrimSpace(name)
	if name == "" || name == a.req.DisplayName() {
		return
	}
	a.req.Name = name
	for i := range a.requests {
		if a.requests[i].ID == a.savedSel {
			a.requests[i].Name = name
			a.requests[i].UpdatedAt = time.Now()
			a.persistRequests()
			a.setNotice(locales.NoticeRenamed(name))
			return
		}
	}
}

// duplicateRequest saves a copy of a saved request below it, and opens it.
func (a *app) duplicateRequest(id string) {
	for i, r := range a.requests {
		if r.ID != id {
			continue
		}
		cp := r.Clone()
		cp.ID = model.NewID("req")
		cp.Name = locales.ComposerCopyName(displayRequestName(r))
		cp.Saved, cp.UpdatedAt = true, time.Now()
		a.requests = limitRequests(slices.Insert(a.requests, i+1, cp))
		a.persistRequests()
		a.loadRequest(cp)
		a.setNotice(locales.NoticeDuplicated(cp.Name))
		return
	}
}

func (a *app) persistRequests() {
	if a.store != nil {
		_ = a.store.SetRequests(a.requests)
	}
}

func (a *app) saveRequest() {
	if strings.TrimSpace(a.req.URL) == "" {
		a.setNotice(locales.NoticeNeedsURL())
		return
	}
	now := time.Now()
	name := strings.TrimSpace(a.req.Name)
	if name == "" || name == "Untitled Request" {
		name = a.req.DisplayName()
	}
	a.req.Name = name
	a.req.UpdatedAt = now
	a.req.Saved = true

	if a.savedSel != "" {
		found := false
		for i := range a.requests {
			if a.requests[i].ID == a.savedSel {
				a.requests[i] = a.req.Clone()
				found = true
				break
			}
		}
		if !found {
			a.req.ID = model.NewID("req")
			a.savedSel = a.req.ID
			a.requests = append([]model.Request{a.req.Clone()}, a.requests...)
		}
	} else {
		a.req.ID = model.NewID("req")
		a.savedSel = a.req.ID
		a.requests = append([]model.Request{a.req.Clone()}, a.requests...)
	}
	a.requests = limitRequests(a.requests)
	delete(a.drafts, a.savedSel)
	a.persistRequests()
	a.setNotice(locales.NoticeSaved(name))
}

func limitRequests(in []model.Request) []model.Request {
	const maxRequests = 300
	if len(in) > maxRequests {
		return in[:maxRequests]
	}
	return in
}

// deleteRequest takes a request out of the collection, with Undo in the
// notice to put it back where it was, its draft and all.
func (a *app) deleteRequest(id string) {
	idx := slices.IndexFunc(a.requests, func(r model.Request) bool { return r.ID == id })
	if idx < 0 {
		return
	}
	removed := a.requests[idx]
	draft, hadDraft := a.drafts[id]
	wasOpen := a.savedSel == id
	if wasOpen && a.edited() {
		draft, hadDraft = a.req.Clone(), true
	}
	a.requests = slices.Delete(a.requests, idx, idx+1)
	delete(a.drafts, id)
	if wasOpen {
		// Nothing is left to keep a draft of.
		a.savedSel = ""
		a.newRequest()
	}
	a.persistRequests()
	a.setUndo(locales.NoticeDeletedRequest(displayRequestName(removed)), func() {
		a.requests = slices.Insert(a.requests, min(idx, len(a.requests)), removed)
		if hadDraft {
			if a.drafts == nil {
				a.drafts = map[string]model.Request{}
			}
			a.drafts[id] = draft
		}
		a.persistRequests()
		if wasOpen {
			a.loadRequest(removed)
		}
		a.setNotice(locales.NoticeRestored(displayRequestName(removed)))
	})
}

// loadHistory opens a sent request again as a new draft: the whole request
// as it was written, {{variables}} and all, when the entry kept it, and the
// method and URL when it was written by an earlier version.
func (a *app) loadHistory(h model.HistoryEntry) {
	r := model.NewRequest()
	if h.Request != nil {
		r = h.Request.Clone()
		r.ID, r.Name, r.Saved = model.NewID("req"), "Untitled Request", false
	} else {
		r.Method, r.URL = h.Method, h.URL
	}
	a.open(r, "")
	if h.BodyDropped {
		a.setNotice(locales.NoticeBodyTooLarge())
	}
}

func (a *app) removeHistory(id string) {
	idx := slices.IndexFunc(a.history, func(h model.HistoryEntry) bool { return h.ID == id })
	if idx < 0 {
		return
	}
	removed := a.history[idx]
	a.history = slices.Delete(a.history, idx, idx+1)
	a.persistHistory()
	a.setUndo(locales.NoticeRemovedHistory(), func() {
		a.history = slices.Insert(a.history, min(idx, len(a.history)), removed)
		a.persistHistory()
	})
}

func (a *app) persistHistory() {
	if a.store != nil {
		_ = a.store.SetHistory(a.history)
	}
}

// historyBodyMax is the largest body a history entry keeps; a larger one is
// left out so the history file stays small.
const historyBodyMax = 256 << 10

// addHistory keeps a sent request: as it was written, to open again, under
// the URL it went to, to read in the list.
func (a *app) addHistory(sent model.Request, sentURL string, res httpx.Result) {
	if !a.prefs.KeepHistory {
		return
	}
	snap := sent.Clone()
	snap.ID, snap.Saved, snap.UpdatedAt = "", false, time.Time{}
	dropped := len(snap.Body) > historyBodyMax
	if dropped {
		snap.Body = ""
	}
	entry := model.HistoryEntry{
		ID:          model.NewID("h"),
		Method:      sent.Method,
		URL:         sentURL,
		Status:      res.Status,
		Duration:    res.Duration.Milliseconds(),
		At:          time.Now(),
		Request:     &snap,
		BodyDropped: dropped,
	}
	a.history = append([]model.HistoryEntry{entry}, a.history...)
	out := make([]model.HistoryEntry, 0, min(len(a.history), 120))
	seen := map[string]bool{}
	for _, h := range a.history {
		key := h.Method + " " + h.URL
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, h)
		if len(out) == 120 {
			break
		}
	}
	a.history = out
	a.persistHistory()
}

func (a *app) clearHistory() {
	cleared := a.history
	a.history = []model.HistoryEntry{}
	a.persistHistory()
	a.setUndo(locales.NoticeHistoryCleared(), func() {
		// Anything sent since stays on top.
		a.history = append(a.history, cleared...)
		a.persistHistory()
	})
}

func (a *app) send() {
	if a.sending {
		return
	}
	// A request that cannot go replaces the last answer with why, which an
	// answer left on screen would hide.
	if strings.TrimSpace(a.req.URL) == "" {
		a.resp, a.respErr = nil, ""
		a.respProblem, a.respMissing = respEnterURL, nil
		a.setNotice(a.respProblemText())
		return
	}
	resolved, missing := a.resolve(a.req)
	// A URL with a hole in it cannot be sent; a header or a body can, as
	// it may mean its braces, and the pane warns of it below the URL.
	if urlMissing := missingIn(resolved.URL, missing); len(urlMissing) > 0 {
		a.resp, a.respErr = nil, ""
		a.respProblem, a.respMissing = respMissingVars, urlMissing
		a.setNotice(a.respProblemText())
		return
	}
	u, err := httpx.BuildURL(resolved.URL, resolved.Params)
	if err != nil {
		a.resp, a.respErr = nil, err.Error()
		a.respProblem, a.respMissing = respOK, nil
		a.setNotice(err.Error())
		return
	}
	sent, sentURL := a.req.Clone(), u.String()
	spec := a.sendSpec(resolved)
	ctx, cancel := context.WithCancel(context.Background())
	a.sendSeq++
	seq := a.sendSeq
	a.sending, a.sendCancel = true, cancel
	a.resp = nil
	a.respErr = ""
	a.respProblem, a.respMissing = respOK, nil
	a.pretty = true
	a.respTab = 0

	go func() {
		res := a.client.Do(ctx, spec)
		cancel()
		// The window may have closed while the request was in flight; unlike
		// the proxy's callback, this goroutine is not owned by it.
		if a.win == nil {
			return
		}
		a.win.Update(func() { a.finishSend(seq, sent, sentURL, res) })
	}()
	if len(missing) > 0 {
		a.setNotice(locales.NoticeSentMissing(braces(missing)))
	}
}

// finishSend shows the answer to the send numbered seq. An answer to a send
// that was canceled, or replaced by a newer one, is dropped: it would land on
// a request that did not ask for it.
func (a *app) finishSend(seq uint64, sent model.Request, sentURL string, res httpx.Result) {
	if seq != a.sendSeq || !a.sending {
		return
	}
	a.sending, a.sendCancel = false, nil
	a.resp = &res
	a.respErr = res.Err
	a.respProblem, a.respMissing = respOK, nil
	a.addHistory(sent, sentURL, res)
	if res.Err != "" {
		a.setNotice(locales.NoticeRequestFailed(sentURL))
	}
}

// cancelSend stops the request in flight. Its answer is dropped when it
// comes, and the pane says the request was canceled.
func (a *app) cancelSend() {
	if !a.sending {
		return
	}
	if a.sendCancel != nil {
		a.sendCancel()
	}
	a.sendSeq++
	a.sending, a.sendCancel = false, nil
	a.resp = nil
	a.respErr = ""
	a.respProblem, a.respMissing = respCanceled, nil
}

func specOf(r model.Request) httpx.Spec {
	return httpx.Spec{
		Method:   r.Method,
		URL:      r.URL,
		Params:   r.Params,
		Headers:  r.Headers,
		BodyKind: r.BodyKind,
		Body:     r.Body,
		Form:     r.Form,
		AuthType: r.AuthType,
		Token:    r.Token,
		User:     r.User,
		Pass:     r.Pass,
	}
}

// sendSpec is a resolved request with the settings that steer how it goes:
// how long to wait, and whether to follow redirects and check certificates.
func (a *app) sendSpec(r model.Request) httpx.Spec {
	s := specOf(r)
	s.Timeout = a.prefs.Timeout()
	s.NoRedirects = a.prefs.StopAtRedirects
	s.Insecure = a.prefs.SkipTLSVerify
	return s
}

// curlOf writes a request as curl sends it from here: its {{variables}}
// filled in from the environment in use, under the composer's settings.
func (a *app) curlOf(r model.Request) (string, error) {
	resolved, _ := a.resolve(r)
	return httpx.Curl(a.sendSpec(resolved))
}

// sendFlowToComposer turns a captured exchange into a draft, so the thing
// on screen can be replayed, edited and sent from the composer.
func (a *app) sendFlowToComposer(f proxy.Flow) {
	a.open(requestFromFlow(f), "")
	a.mode = modeComposer
	a.setNotice(locales.NoticeCapturedOpened())
}

// saveFlowAsRequest puts a captured exchange into the collection, already
// saved, and opens it so it can be changed and sent again.
func (a *app) saveFlowAsRequest(f proxy.Flow) {
	r := requestFromFlow(f)
	r.ID = model.NewID("req")
	r.Name = flowRequestName(f)
	r.Saved, r.UpdatedAt = true, time.Now()
	a.requests = limitRequests(append([]model.Request{r.Clone()}, a.requests...))
	a.persistRequests()
	a.open(r.Clone(), r.ID)
	a.mode = modeComposer
	a.setNotice(locales.NoticeFlowSaved(r.Name))
}

// requestFromFlow turns a captured exchange into a request: its method, URL,
// replayable headers, body and the body kind its content type describes.
func requestFromFlow(f proxy.Flow) model.Request {
	r := model.NewRequest()
	r.Name = flowRequestName(f)
	r.Method = f.Method
	r.URL = f.URL
	r.Headers = nil
	for _, h := range f.ReqHeaders {
		if replayable(h.Key) {
			r.Headers = append(r.Headers, model.KV{
				ID: model.NewID("kv"), Enabled: true, Key: h.Key, Value: h.Value,
			})
		}
	}
	if len(f.ReqBody) > 0 {
		r.Body = string(f.ReqBody)
		ct := strings.ToLower(headerValue(f.ReqHeaders, "Content-Type"))
		switch {
		case strings.Contains(ct, "json"):
			r.BodyKind = model.BodyJSON
		case strings.Contains(ct, "x-www-form-urlencoded"):
			r.BodyKind, r.Form = model.BodyForm, nil
			r.Normalize()
			if len(r.Form) == 0 {
				// Not a form after all: keep it as the text it is.
				r.BodyKind = model.BodyText
			}
		default:
			r.BodyKind = model.BodyText
		}
	}
	return r
}

// flowRequestName is the name a captured exchange takes in the collection:
// its method, host and path, without the query or an overlong tail.
func flowRequestName(f proxy.Flow) string {
	path := f.Path
	if i := strings.IndexByte(path, '?'); i >= 0 {
		path = path[:i]
	}
	name := f.Method + " " + f.Host + path
	if runes := []rune(name); len(runes) > 64 {
		name = string(runes[:63]) + "…"
	}
	return name
}

// replayable reports whether a captured header belongs in a request sent
// again: those describing the hop or the old connection, and the encodings
// the inspector asked for, do not.
func replayable(key string) bool {
	switch strings.ToLower(key) {
	case "host", "content-length", "connection", "accept-encoding", "proxy-connection",
		"keep-alive", "transfer-encoding", "te", "upgrade":
		return false
	}
	return true
}

func headerValue(headers []proxy.HeaderPair, key string) string {
	for _, h := range headers {
		if strings.EqualFold(h.Key, key) {
			return h.Value
		}
	}
	return ""
}

// curlText is what Copy as cURL copies: the composer's request, or the
// inspector's chosen flow.
func (a *app) curlText() (string, bool) {
	if a.mode == modeInspector {
		f := a.selectedFlow()
		if f == nil {
			a.setNotice(locales.NoticeChooseFlowCurl())
			return "", false
		}
		return flowCurl(*f), true
	}
	cmd, err := a.curlOf(a.req)
	if err != nil {
		a.setNotice(err.Error())
		return "", false
	}
	return cmd, true
}

func (a *app) copyCurl(c *ui.Context) {
	if cmd, ok := a.curlText(); ok {
		a.copyText(c, locales.NoticeCurlCopied(), cmd)
	}
}

// copyCurlPlain is copyCurl for the menu, which has no view to copy from.
func (a *app) copyCurlPlain() {
	if cmd, ok := a.curlText(); ok {
		a.copyPlain(locales.NoticeCurlCopied(), cmd)
	}
}

// flowCurl writes a captured request as the curl command that sends it
// again.
func flowCurl(f proxy.Flow) string {
	var headers []httpx.Header
	for _, h := range f.ReqHeaders {
		if replayable(h.Key) {
			headers = append(headers, httpx.Header{Key: h.Key, Value: h.Value})
		}
	}
	return httpx.CurlCommand(f.Method, f.URL, headers, f.ReqBody)
}

// --- proxy ----------------------------------------------------------------

// systemTrustCheck is the live Root CA trust query. Tests replace it so the
// app's own tests never depend on what this machine trusts.
var systemTrustCheck = systemTrusts

// recoverSystemProxy puts back the system proxy a previous run left behind.
// A corrupt backup blocks automatic control for this session; the person
// turns the setting off and on again to take a fresh snapshot.
func (a *app) recoverSystemProxy() {
	if a.sysproxy == nil {
		return
	}
	if err := a.sysproxy.Recover(); err != nil {
		if errors.Is(err, sysproxy.ErrCorruptBackup) {
			a.sysproxyBlocked = true
			a.setNotice(locales.NoticeSystemProxyCorrupt())
			return
		}
		a.setNotice(locales.NoticeSystemProxyFailed(err.Error()))
	}
}

// syncSystemProxy makes the machine follow the capture: while it runs and
// automatic proxy control is on, the system proxy points at OmniFlow. Any
// other state puts the machine's own settings back.
func (a *app) syncSystemProxy() {
	if a.sysproxy == nil {
		return
	}
	if !a.autoProxy || !a.proxyRunning() {
		if err := a.sysproxy.Restore(); err != nil {
			a.setNotice(locales.NoticeSystemProxyFailed(err.Error()))
		}
		return
	}
	if a.sysproxyBlocked {
		return
	}
	port, err := strconv.Atoi(strings.TrimSpace(a.proxyPort))
	if err != nil {
		return
	}
	trusted := false
	if a.ca != nil {
		ok, known := systemTrustCheck(a.ca.CertPath())
		switch {
		case known && ok:
			a.caTrust, trusted = trustYes, true
		case known:
			a.caTrust = trustNo
		default:
			a.caTrust = trustUnknown
		}
	}
	if err := a.sysproxy.Enable(port, a.mitm && trusted); err != nil {
		a.setNotice(locales.NoticeSystemProxyFailed(err.Error()))
	}
}

// setAutoProxy is the settings switch: the next sync either takes the
// machine's proxy or gives it back. Turning it on is also the explicit
// retry that takes a fresh snapshot after a corrupt backup.
func (a *app) setAutoProxy(on bool) {
	a.autoProxy = on
	a.prefs.DisableAutoProxy = !on
	a.savePrefs()
	if a.sysproxy != nil {
		a.sysproxy.Reset()
	}
	a.sysproxyBlocked = false
	a.syncSystemProxy()
}

func (a *app) startProxy() {
	if a.prx == nil {
		a.proxyErr = ""
		a.proxyProblem = proxyCAUnavailable
		a.setNotice(a.proxyProblemText())
		a.syncSystemProxy()
		return
	}
	if !validPort(a.proxyPort) {
		a.proxyErr = ""
		a.proxyProblem = proxyBadPort
		a.setNotice(a.proxyProblemText())
		a.syncSystemProxy()
		return
	}
	a.prx.SetMITM(a.mitm)
	addr, err := a.prx.StartOn(a.proxyListenHost(), strings.TrimSpace(a.proxyPort))
	if err != nil {
		a.proxyErr = err.Error()
		a.proxyProblem = proxyOK
		a.setNotice(err.Error())
		a.syncSystemProxy()
		return
	}
	a.proxyErr = ""
	a.proxyProblem = proxyOK
	if _, port, err := splitHostPort(addr); err == nil {
		a.proxyPort = port
	}
	a.setNotice(locales.NoticeProxyListening(addr))
	a.syncSystemProxy()
}

func (a *app) stopProxy() {
	var restoreErr error
	if a.sysproxy != nil {
		restoreErr = a.sysproxy.Restore()
	}
	if a.prx != nil {
		a.prx.Stop()
		a.setNotice(locales.NoticeProxyStopped())
	}
	if restoreErr != nil {
		a.setNotice(locales.NoticeSystemProxyFailed(restoreErr.Error()))
	}
}

func (a *app) toggleProxy() {
	if a.prx != nil && a.prx.Running() {
		a.stopProxy()
		return
	}
	a.startProxy()
}

func (a *app) restartProxy() {
	was := a.proxyRunning()
	a.stopProxy()
	if was {
		a.startProxy()
	}
}

func (a *app) proxyRunning() bool {
	return a.prx != nil && a.prx.Running()
}

// proxyListenHost is where the proxy listens: loopback alone, or every
// interface when a device on the local network is being captured.
func (a *app) proxyListenHost() string {
	if a.lanAccess {
		return "0.0.0.0"
	}
	return "127.0.0.1"
}

func validPort(s string) bool {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	return err == nil && n >= 1 && n <= 65535
}

// setProxyPort is a port field's edit: saved once it is a port at all, and
// applied to a running proxy when the field is submitted.
func (a *app) setProxyPort(submitted bool) {
	if !validPort(a.proxyPort) {
		return
	}
	a.prefs.ProxyPort = strings.TrimSpace(a.proxyPort)
	a.savePrefs()
	if submitted && a.proxyRunning() {
		a.restartProxy()
	}
}

func splitHostPort(addr string) (string, string, error) {
	i := strings.LastIndex(addr, ":")
	if i < 0 {
		return addr, "", fmt.Errorf("no port in %q", addr)
	}
	return addr[:i], addr[i+1:], nil
}

// --- flows ----------------------------------------------------------------

// The capture keeps at most maxFlows flows, and maxCaptured bytes of their
// bodies; past either the oldest go, as the least likely to be read again.
const (
	maxFlows    = 3000
	maxCaptured = 512 << 20
)

func flowBytes(f proxy.Flow) int64 { return int64(len(f.ReqBody) + len(f.RespBody)) }

func (a *app) addFlow(f proxy.Flow) {
	a.flows = append(a.flows, f)
	a.flowRev++
	a.capturedBytes += flowBytes(f)
	a.trimFlows()
	a.lastFlowAt = time.Now()
	if a.follow {
		a.flowSelID = f.ID
	}
}

// trimFlows lets the oldest flows go while the capture is over its limits,
// keeping the newest one whatever its size.
func (a *app) trimFlows() {
	drop := 0
	for n := len(a.flows); n-drop > maxFlows || (a.capturedBytes > maxCaptured && n-drop > 1); drop++ {
		a.capturedBytes -= flowBytes(a.flows[drop])
		// Let go of its bodies now, not when the array is next grown.
		a.flows[drop] = proxy.Flow{}
	}
	a.flows = a.flows[drop:]
}

// clearFlows empties the capture, with Undo in the notice to bring it back
// ahead of whatever has come since.
func (a *app) clearFlows() {
	cleared, sel := a.flows, a.flowSelID
	a.flows, a.capturedBytes = nil, 0
	a.flowRev++
	a.flowSelID = 0
	a.domainFilter, a.flowFilter, a.statusFilter = "", "", ""
	if len(cleared) == 0 {
		return
	}
	a.setUndo(locales.NoticeSessionCleared(), func() {
		a.flows = append(cleared, a.flows...)
		a.flowRev++
		a.capturedBytes = 0
		for _, f := range a.flows {
			a.capturedBytes += flowBytes(f)
		}
		a.trimFlows()
		if a.flowSelID == 0 {
			a.flowSelID = sel
		}
	})
}

// exportHAR writes the capture as an HTTP Archive, the file other
// inspectors read. It is built on this thread, from the capture as it is
// now, and written where the save panel says.
func (a *app) exportHAR() {
	n := len(a.flows)
	if n == 0 {
		a.setNotice(locales.NoticeNoTraffic())
		return
	}
	data, err := proxy.HAR(a.flows, "OmniFlow", appVersion())
	if err != nil {
		a.setNotice(locales.NoticeHARPrepareFailed(err.Error()))
		return
	}
	go func() {
		dest, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
			Parent:      a.win,
			Title:       locales.ExportHAR(),
			DefaultPath: "omniflow.har",
			ButtonLabel: locales.ExportHARButton(),
		})
		if err != nil {
			a.update(func() { a.setNotice(locales.NoticeHARExportFailed(err.Error())) })
			return
		}
		if dest == "" {
			return
		}
		if err := os.WriteFile(dest, data, 0o644); err != nil {
			a.update(func() { a.setNotice(locales.NoticeHARExportFailed(err.Error())) })
			return
		}
		a.update(func() { a.setNotice(locales.NoticeExported(n, dest)) })
	}()
}

// filtering reports whether the list shows less than the whole capture.
func (a *app) filtering() bool {
	return a.domainFilter != "" || a.statusFilter != "" || strings.TrimSpace(a.flowFilter) != ""
}

func (a *app) clearFilters() {
	a.domainFilter, a.flowFilter, a.statusFilter = "", "", ""
}

// statusClass is the group a flow's status is filtered by: 2xx to 5xx, or
// "err" for a flow that got no answer.
func statusClass(status int) string {
	switch {
	case status <= 0:
		return "err"
	case status < 200:
		return "1xx"
	case status < 300:
		return "2xx"
	case status < 400:
		return "3xx"
	case status < 500:
		return "4xx"
	}
	return "5xx"
}

func (a *app) matchesFilter(f proxy.Flow) bool {
	if a.domainFilter != "" && !strings.EqualFold(f.Host, a.domainFilter) {
		return false
	}
	if a.statusFilter != "" && statusClass(f.Status) != a.statusFilter {
		return false
	}
	q := strings.ToLower(strings.TrimSpace(a.flowFilter))
	if q == "" {
		return true
	}
	hay := strings.ToLower(f.Method + " " + f.Host + " " + f.Path + " " + f.ContentType)
	if strings.Contains(hay, q) {
		return true
	}
	return a.searchBodies && len(q) >= 2 && (bodyContains(f.ReqBody, q) || bodyContains(f.RespBody, q))
}

// bodyContains reports whether a body holds query, which is already lower
// case. Only the beginning of a very large body is read, so filtering stays
// responsive.
func bodyContains(body []byte, query string) bool {
	const maxSearchBody = 512 << 10
	if len(body) == 0 || query == "" {
		return false
	}
	if len(body) > maxSearchBody {
		body = body[:maxSearchBody]
	}
	return bytes.Contains(bytes.ToLower(body), []byte(query))
}

// visibleFlows are the indices of the flows the filters let through,
// oldest first, as the list shows them.
func (a *app) visibleFlows() []int {
	key := fmt.Sprintf("%d|%d|%t|%s|%s|%s",
		a.flowRev, len(a.flows), a.searchBodies, a.domainFilter, a.statusFilter, a.flowFilter)
	if key == a.flowCache {
		return a.flowCached
	}
	out := make([]int, 0, len(a.flows))
	for i := range a.flows {
		if a.matchesFilter(a.flows[i]) {
			out = append(out, i)
		}
	}
	a.flowCache, a.flowCached = key, out
	return out
}

// selectFlow chooses a flow by hand, which stops following new ones.
func (a *app) selectFlow(id uint64) {
	a.flowSelID = id
	if a.follow {
		a.follow = false
		a.prefs.FollowFlows = false
		a.savePrefs()
	}
}

// moveFlowSelection moves the choice through the flows on screen, in the
// order they are listed: by delta rows, from the first or last row when
// nothing is chosen.
func (a *app) moveFlowSelection(delta int) {
	ordered := a.visibleFlows()
	if len(ordered) == 0 {
		return
	}
	at := -1
	for i, idx := range ordered {
		if a.flows[idx].ID == a.flowSelID {
			at = i
			break
		}
	}
	switch {
	case at < 0 && delta > 0:
		at = 0
	case at < 0:
		at = len(ordered) - 1
	default:
		at = min(max(at+delta, 0), len(ordered)-1)
	}
	a.selectFlow(a.flows[ordered[at]].ID)
	a.flowList.ScrollIntoView(at)
}

func (a *app) selectedFlow() *proxy.Flow {
	for i := range a.flows {
		if a.flows[i].ID == a.flowSelID {
			return &a.flows[i]
		}
	}
	return nil
}

type domainCount struct {
	Host  string
	Count int
}

func (a *app) domains() []domainCount {
	counts := map[string]int{}
	for _, f := range a.flows {
		counts[f.Host]++
	}
	out := make([]domainCount, 0, len(counts))
	for host, n := range counts {
		out = append(out, domainCount{Host: host, Count: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Host < out[j].Host
	})
	if len(out) > 80 {
		out = out[:80]
	}
	return out
}

// --- settings & certificate ----------------------------------------------

func (a *app) setTheme(mode string) {
	a.prefs.Theme = mode
	applyAppearance(mode)
	a.savePrefs()
}

func (a *app) savePrefs() {
	if a.store != nil {
		_ = a.store.SetPrefs(a.prefs)
	}
}

func (a *app) exportCA() {
	if a.ca == nil {
		a.setNotice(locales.NoticeCAUnavailable())
		return
	}
	go func() {
		path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
			Parent:      a.win,
			Title:       locales.CaSaveDialogTitle(),
			DefaultPath: "OmniFlow-Root-CA.pem",
			ButtonLabel: locales.CommonSave(),
			Filters:     []mygo.FileFilter{{Name: locales.CaFilterName(), Extensions: []string{"pem", "crt", "cer"}}},
		})
		if err != nil {
			a.update(func() { a.setNotice(locales.NoticeCASaveFailed(err.Error())) })
			return
		}
		if path == "" {
			return
		}
		if err := os.WriteFile(path, a.ca.CertPEM(), 0o644); err != nil {
			a.update(func() { a.setNotice(locales.NoticeCASaveFailed(err.Error())) })
			return
		}
		a.update(func() { a.setNotice(locales.NoticeCASaved(path)) })
	}()
}

func (a *app) revealCA() {
	if a.ca == nil {
		return
	}
	if err := revealPath(a.ca.CertPath()); err != nil {
		a.setNotice(locales.NoticeCARevealFailed(err.Error()))
	}
}

// dataDir is where the collection, the history and the certificate live.
func (a *app) dataDir() string {
	if a.store == nil {
		return locales.CommonUnavailable()
	}
	return filepath.Dir(a.store.Path())
}

func (a *app) revealData() {
	if a.store == nil {
		return
	}
	if err := revealPath(a.store.Path()); err != nil {
		a.setNotice(locales.NoticeDataRevealFailed(err.Error()))
	}
}

// appVersion is the packaged app's version; a development run has none.
func appVersion() string {
	if v := mygo.App.Version(); v != "" {
		return v
	}
	return locales.CommonDevelopmentBuild()
}

// displayRequestName is a request's name as the interface shows it: a
// request still carrying the stored default reads in the app's language.
func displayRequestName(r model.Request) string {
	if r.DisplayName() == "Untitled Request" {
		return locales.ComposerUntitledRequest()
	}
	return r.DisplayName()
}

func (a *app) copyText(c *ui.Context, label, text string) {
	c.WriteClipboard(text)
	a.setNotice(label)
}

func (a *app) copyPlain(label, text string) {
	_ = mygo.Clipboard.Write(transfer.TextData(text))
	a.setNotice(label)
}

// --- menu -----------------------------------------------------------------

// appMenuItem is the application menu. On macOS it is written here rather
// than left to the default role, because the default has no room for Check
// for Updates…; the Settings item stays in the File menu, where it already
// was. Everywhere else the default menu is kept, and the Updates page's
// button is the way to check.
func (a *app) appMenuItem() *mygo.MenuItem {
	if runtime.GOOS != "darwin" {
		return &mygo.MenuItem{Role: mygo.RoleAppMenu}
	}
	return &mygo.MenuItem{Label: "OmniFlow", Submenu: []*mygo.MenuItem{
		{Role: mygo.RoleAbout},
		updaterMenuItem(),
		mygo.Separator(),
		{Role: mygo.RoleServices},
		mygo.Separator(),
		{Role: mygo.RoleHide},
		{Role: mygo.RoleHideOthers},
		{Role: mygo.RoleUnhide},
		mygo.Separator(),
		{Role: mygo.RoleQuit},
	}}
}

func (a *app) installMenu() {
	mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
		a.appMenuItem(),
		{Label: locales.MenuFile(), Submenu: []*mygo.MenuItem{
			{Label: locales.ComposerNewRequest(), Accelerator: "CmdOrCtrl+N", Click: func(*mygo.MenuItem, *mygo.Window) {
				a.update(func() { a.newRequest() })
			}},
			{Label: locales.MenuSaveRequest(), Accelerator: "CmdOrCtrl+S", Click: func(*mygo.MenuItem, *mygo.Window) {
				a.update(a.saveRequest)
			}},
			mygo.Separator(),
			{Label: locales.MenuSendRequest(), Accelerator: "CmdOrCtrl+Enter", Click: func(*mygo.MenuItem, *mygo.Window) {
				a.update(func() {
					if a.mode == modeComposer {
						a.send()
					}
				})
			}},
			{Label: locales.MenuCancelRequest(), Accelerator: "CmdOrCtrl+.", Click: func(*mygo.MenuItem, *mygo.Window) {
				a.update(a.cancelSend)
			}},
			{Label: locales.CommonCopyCurl(), Accelerator: "CmdOrCtrl+Shift+C", Click: func(*mygo.MenuItem, *mygo.Window) {
				a.update(func() { a.copyCurlPlain() })
			}},
			mygo.Separator(),
			{Label: locales.CommonSettingsMenu(), Accelerator: "CmdOrCtrl+,", Click: func(*mygo.MenuItem, *mygo.Window) {
				a.update(func() { a.settingsOpen = true })
			}},
			mygo.Separator(),
			{Role: mygo.RoleClose},
		}},
		{Role: mygo.RoleEditMenu},
		{Label: locales.MenuView(), Submenu: []*mygo.MenuItem{
			{Label: locales.ComposerMode(), Accelerator: "CmdOrCtrl+1", Click: func(*mygo.MenuItem, *mygo.Window) {
				a.update(func() { a.mode = modeComposer })
			}},
			{Label: locales.MenuProxyInspector(), Accelerator: "CmdOrCtrl+2", Click: func(*mygo.MenuItem, *mygo.Window) {
				a.update(func() { a.mode = modeInspector })
			}},
			mygo.Separator(),
			{Label: locales.MenuEnvironments(), Accelerator: "CmdOrCtrl+E", Click: func(*mygo.MenuItem, *mygo.Window) {
				a.update(a.openEnvs)
			}},
		}},
		{Label: locales.MenuProxy(), Submenu: []*mygo.MenuItem{
			{Label: locales.MenuStartStop(), Accelerator: "CmdOrCtrl+Shift+P", Click: func(*mygo.MenuItem, *mygo.Window) {
				a.update(a.toggleProxy)
			}},
			{Label: locales.InspectorClearSession(), Click: func(*mygo.MenuItem, *mygo.Window) {
				a.update(a.clearFlows)
			}},
			{Label: locales.MenuExportHAR(), Accelerator: "CmdOrCtrl+Shift+E", Click: func(*mygo.MenuItem, *mygo.Window) {
				a.update(a.exportHAR)
			}},
			mygo.Separator(),
			{Label: locales.MenuExportCA(), Click: func(*mygo.MenuItem, *mygo.Window) { a.exportCA() }},
		}},
		{Role: mygo.RoleWindowMenu},
	}))
}

// windowOptions is the frame the window opens in: an inset title bar, the
// material the settings chose, and a size that fits the two workbenches. The
// window is transparent so the system's material is the background, and the
// ground is drawn as a veil over it.
func (a *app) windowOptions(content mygo.Content) mygo.WindowOptions {
	return framedWindow(mygo.WindowOptions{
		Title:           "OmniFlow",
		Width:           1280,
		Height:          820,
		MinWidth:        1020,
		MinHeight:       620,
		TitleBarHeight:  52,
		Vibrancy:        a.blurInUse().vibrancy(),
		Transparent:     true,
		BackgroundColor: "light-dark(#e6ecf5, #141418)",
		StateKey:        "main",
		Content:         content,
	})
}
