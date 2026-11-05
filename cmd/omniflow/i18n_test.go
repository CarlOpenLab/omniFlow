package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/egoist/mygo/i18n"

	"github.com/CarlOpenLab/omniflow/internal/model"
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
		"设置", "外观", "语言", "主题", "代理端口", "数据", "关于",
	} {
		if !tt.HasText(want) {
			t.Errorf("the Chinese settings are missing %q", want)
		}
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
