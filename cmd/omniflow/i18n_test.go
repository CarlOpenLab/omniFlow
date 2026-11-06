package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/i18n"

	"github.com/CarlOpenLab/omniflow/internal/httpx"
	"github.com/CarlOpenLab/omniflow/internal/model"
	"github.com/CarlOpenLab/omniflow/locales"
)

// TestMain pins the language the app shows, so the English assertions in
// every test do not change with the language of the machine running them.
// Tests that check a translation swap the locale themselves.
func TestMain(m *testing.M) {
	i18n.SetLocaleFunc(func() string { return "en" })
	os.Exit(m.Run())
}

// catalogKeys reads a catalog's message keys, leaving out the metadata keys
// that start with @.
func catalogKeys(t *testing.T, path string) []string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var msgs map[string]json.RawMessage
	if err := json.Unmarshal(raw, &msgs); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	keys := make([]string, 0, len(msgs))
	for key := range msgs {
		if !strings.HasPrefix(key, "@") {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

// captureLocale swaps the app's locale setter for one that records tags, so
// tests can watch the language without a running app.
func captureLocale(t *testing.T) *[]string {
	t.Helper()
	got := &[]string{}
	old := setAppLocale
	setAppLocale = func(tag string) { *got = append(*got, tag) }
	t.Cleanup(func() { setAppLocale = old })
	return got
}

func TestApplyLanguage(t *testing.T) {
	got := captureLocale(t)
	a := newTestApp(t)
	a.prefs.Language = "zh-Hans"
	a.applyLanguage()
	a.prefs.Language = ""
	a.applyLanguage()
	// A value the app does not ship, from a hand-edited file, follows the
	// system rather than panicking the locale setter.
	a.prefs.Language = "de"
	a.applyLanguage()
	if want := []string{"zh-Hans", "", ""}; !slices.Equal(*got, want) {
		t.Fatalf("applyLanguage called the locale setter with %q, want %q", *got, want)
	}
}

func TestLanguageRow(t *testing.T) {
	got := captureLocale(t)
	a := newTestApp(t)
	a.settingsOpen = true
	tt := newTester(t, a)
	for _, want := range []string{"Language", "Follow System", "English", "简体中文"} {
		if !tt.HasText(want) {
			t.Errorf("the language row is missing %q", want)
		}
	}
	if err := tt.Click("简体中文"); err != nil {
		t.Fatalf("choose 简体中文: %v", err)
	}
	if a.prefs.Language != "zh-Hans" {
		t.Errorf("prefs.Language is %q, want zh-Hans", a.prefs.Language)
	}
	if len(*got) == 0 || (*got)[len(*got)-1] != "zh-Hans" {
		t.Errorf("the locale setter got %q, want a final zh-Hans", *got)
	}
}

func TestChineseComposer(t *testing.T) {
	i18n.SetLocaleFunc(func() string { return "zh-Hans" })
	t.Cleanup(func() { i18n.SetLocaleFunc(func() string { return "en" }) })
	a := newTestApp(t)
	tt := newTester(t, a)
	for _, want := range []string{
		"发送", "集合", "历史", "参数", "请求头", "请求体", "认证", "响应", "尚未发送",
	} {
		if !tt.HasText(want) {
			t.Errorf("the Chinese composer is missing %q", want)
		}
	}
}

func TestChineseInspector(t *testing.T) {
	i18n.SetLocaleFunc(func() string { return "zh-Hans" })
	t.Cleanup(func() { i18n.SetLocaleFunc(func() string { return "en" }) })
	a := newTestApp(t)
	seedFlows(a)
	a.mode = modeInspector
	a.follow = false
	a.flowSelID = 0
	tt := newTester(t, a)
	for _, want := range []string{
		"抓包", "开始抓包", "方法", "状态", "主机", "路径", "大小", "耗时",
		"全部流量", "选择一条流量",
	} {
		if !tt.HasText(want) {
			t.Errorf("the Chinese inspector is missing %q", want)
		}
	}
}

func TestChineseSettingsAndDialogs(t *testing.T) {
	i18n.SetLocaleFunc(func() string { return "zh-Hans" })
	t.Cleanup(func() { i18n.SetLocaleFunc(func() string { return "en" }) })
	a := newTestApp(t)
	a.settingsOpen = true
	tt := newTester(t, a)
	for _, want := range []string{
		"设置", "外观", "语言", "主题", "数据", "更新", "关于",
	} {
		if !tt.HasText(want) {
			t.Errorf("the Chinese settings are missing %q", want)
		}
	}
	clickSettingsPage(t, tt, settingsPaneCapture)
	if !tt.HasText("代理端口") {
		t.Errorf("the Chinese capture page is missing 代理端口: %q", tt.Texts())
	}

	a.settingsOpen = false
	a.envs = []model.Environment{{
		ID: "env1", Name: "Staging",
		Vars: []model.KV{{ID: "e1", Enabled: true, Key: "host", Value: "staging.example.com"}},
	}}
	a.envEdit, a.envOpen = "env1", true
	tt.Frame()
	for _, want := range []string{"环境", "添加行"} {
		if !tt.HasText(want) {
			t.Errorf("the Chinese environment manager is missing %q", want)
		}
	}

	a.envOpen = false
	a.caOpen = true
	tt.Frame()
	if !tt.HasText("根证书") {
		t.Error("the Chinese certificate dialog is missing 根证书")
	}
}

func TestChineseStatusFilterMenu(t *testing.T) {
	i18n.SetLocaleFunc(func() string { return "zh-Hans" })
	t.Cleanup(func() { i18n.SetLocaleFunc(func() string { return "en" }) })
	a := newTestApp(t)
	seedFlows(a)
	a.mode = modeInspector
	tt := newTester(t, a)
	if err := tt.Click("状态"); err != nil {
		t.Fatalf("click the status head: %v", err)
	}
	labels := tt.Menu()
	for _, want := range []string{
		"全部状态", "2xx · 成功", "3xx · 重定向", "4xx · 客户端错误", "5xx · 服务器错误", "错误 · 无响应",
	} {
		if !slices.Contains(labels, want) {
			t.Errorf("the Chinese status menu is missing %q: %q", want, labels)
		}
	}
}

// The app's own messages are written when a problem happens but read in
// whatever language is on screen now.
func TestCanceledStateFollowsLanguage(t *testing.T) {
	a := newTestApp(t)
	a.sending = true
	a.cancelSend() // under English, as the test app starts
	i18n.SetLocaleFunc(func() string { return "zh-Hans" })
	t.Cleanup(func() { i18n.SetLocaleFunc(func() string { return "en" }) })
	tt := newTester(t, a)
	if !tt.HasText("请求已取消") {
		t.Errorf("the canceled request is not in Chinese: %q", tt.Texts())
	}
	if tt.HasText("请求未完成") {
		t.Error("a canceled request is shown as an error")
	}
}

func TestMissingVariableMessageFollowsLanguage(t *testing.T) {
	a := newTestApp(t)
	a.req.URL = "https://{{host}}/things"
	a.send() // under English, as the test app starts
	i18n.SetLocaleFunc(func() string { return "zh-Hans" })
	t.Cleanup(func() { i18n.SetLocaleFunc(func() string { return "en" }) })
	tt := newTester(t, a)
	if !tt.HasText("未设置") {
		t.Errorf("the missing-variable message is not in Chinese: %q", tt.Texts())
	}
}

func TestProxyProblemFollowsLanguage(t *testing.T) {
	a := newTestApp(t)
	a.proxyPort = "0"
	a.startProxy() // under English, as the test app starts
	// The sidebar's inline check would print the message in the current
	// language anyway; make the port valid again so only the stored problem
	// is left on screen. The notice may keep its old language by design.
	a.proxyPort = "9090"
	a.notice = ""
	i18n.SetLocaleFunc(func() string { return "zh-Hans" })
	t.Cleanup(func() { i18n.SetLocaleFunc(func() string { return "en" }) })
	a.mode = modeInspector
	tt := newTester(t, a)
	if !tt.HasText("请使用 1 到 65535 之间的端口。") {
		t.Errorf("the port problem is not in Chinese: %q", tt.Texts())
	}
	if tt.HasText("Use a port from 1 to 65535.") {
		t.Error("the port problem kept the language it was written in")
	}
}

func TestJSONValidationFollowsLanguage(t *testing.T) {
	a := newTestApp(t)
	a.reqTab = 2
	a.req.BodyKind = model.BodyJSON
	a.req.Body = `{"a": }`
	tt := newTester(t, a) // checked and written under English
	i18n.SetLocaleFunc(func() string { return "zh-Hans" })
	t.Cleanup(func() { i18n.SetLocaleFunc(func() string { return "en" }) })
	tt.Frame()
	if !tt.HasText("JSON 无效") {
		t.Errorf("the JSON check is not in Chinese: %q", tt.Texts())
	}
	if tt.HasText("Invalid JSON") {
		t.Error("the JSON check kept the language it was written in")
	}
}

func TestChineseTabCount(t *testing.T) {
	i18n.SetLocaleFunc(func() string { return "zh-Hans" })
	t.Cleanup(func() { i18n.SetLocaleFunc(func() string { return "en" }) })
	a := newTestApp(t)
	a.req.Headers = []model.KV{
		{ID: "h1", Enabled: true, Key: "Accept", Value: "application/json"},
		{ID: "h2", Enabled: true, Key: "User-Agent", Value: "OmniFlow/0.1"},
	}
	tt := newTester(t, a)
	if !tt.HasText("请求头（2）") {
		t.Errorf("the Chinese header count is missing: %q", tt.Texts())
	}
}

// The tabs over an answer name the answer, not the request: Chinese has a
// word for each side.
func TestChineseResponseTabs(t *testing.T) {
	i18n.SetLocaleFunc(func() string { return "zh-Hans" })
	t.Cleanup(func() { i18n.SetLocaleFunc(func() string { return "en" }) })
	a := newTestApp(t)
	a.resp = &httpx.Result{
		Status: 200, StatusText: "OK", Proto: "HTTP/1.1",
		Headers: []httpx.Header{
			{Key: "Content-Type", Value: "application/json"},
			{Key: "Server", Value: "GitHub.com"},
		},
		Body: []byte(`{"a":1}`),
	}
	tt := newTester(t, a)
	if !tt.HasText("响应体") {
		t.Errorf("the composer's answer body tab is not 响应体: %q", tt.Texts())
	}
	if !tt.HasText("响应头（2）") {
		t.Errorf("the composer's answer headers tab is not 响应头（2）: %q", tt.Texts())
	}
	if !tt.HasText("请求体") {
		t.Error("the request body tab stopped saying 请求体")
	}

	a.mode = modeInspector
	seedFlows(a)
	a.follow = false
	a.flowSelID = 4
	a.detailTab = 1
	tt.Frame()
	bodyTab := "响应体 · " + httpx.FormatBytes(int64(len(a.flows[3].RespBody)))
	if !tt.HasText(bodyTab) {
		t.Errorf("the inspector's answer body tab is not %q: %q", bodyTab, tt.Texts())
	}
	if !tt.HasText("响应头（2）") {
		t.Errorf("the inspector's answer headers tab is not 响应头（2）: %q", tt.Texts())
	}
	a.detailTab = 0
	tt.Frame()
	if !tt.HasText("请求头（2）") {
		t.Errorf("the inspector's request headers tab is not 请求头（2）: %q", tt.Texts())
	}
}

func TestHistorySentEnglishKeepsComma(t *testing.T) {
	at := time.Date(2026, 1, 2, 15, 4, 5, 0, time.Local)
	if got := locales.HistorySent(at); !strings.Contains(got, "Jan 2, 15:04:05") {
		t.Errorf("the English history tooltip is %q, want the date and time with the comma it had", got)
	}
}

func TestCatalogParity(t *testing.T) {
	dir := filepath.Join("..", "..", "locales")
	en := catalogKeys(t, filepath.Join(dir, "en.json"))
	zh := catalogKeys(t, filepath.Join(dir, "zh-Hans.json"))
	if slices.Equal(en, zh) {
		return
	}
	for _, key := range en {
		if !slices.Contains(zh, key) {
			t.Errorf("zh-Hans.json is missing %q", key)
		}
	}
	for _, key := range zh {
		if !slices.Contains(en, key) {
			t.Errorf("zh-Hans.json has %q, which en.json does not", key)
		}
	}
}
