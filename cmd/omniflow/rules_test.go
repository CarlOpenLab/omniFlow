package main

import (
	"testing"

	"github.com/CarlOpenLab/omniflow/internal/model"
	"github.com/CarlOpenLab/omniflow/internal/store"
	"github.com/CarlOpenLab/omniflow/locales"
)

func TestRuleDialogCreatesUpdatesAndDeletesARule(t *testing.T) {
	a := newStoredTestApp(t)
	a.openNewRule(model.RuleMapRemote)
	a.ruleMethod = "GET"
	a.ruleHost = "api.example"
	a.rulePath = "/v1"
	a.ruleTarget = "http://localhost:3000"
	a.saveRuleDialog()

	if a.ruleOpen || a.ruleErr != "" {
		t.Fatalf("save left the dialog open=%v err=%q", a.ruleOpen, a.ruleErr)
	}
	if len(a.rules) != 1 {
		t.Fatalf("rules = %d, want one", len(a.rules))
	}
	rule := a.rules[0]
	if rule.Kind != model.RuleMapRemote || rule.Host != "api.example" || rule.Target != "http://localhost:3000" {
		t.Errorf("rule is %+v", rule)
	}
	if rule.ID == "" || rule.Name == "" || !rule.Enabled {
		t.Errorf("rule was not complete: %+v", rule)
	}

	a.editRule(rule)
	a.ruleTarget = "https://staging.example.com"
	a.saveRuleDialog()
	if len(a.rules) != 1 || a.rules[0].ID != rule.ID || a.rules[0].Target != "https://staging.example.com" {
		t.Errorf("editing made %+v", a.rules)
	}

	a.deleteRule(rule.ID)
	if len(a.rules) != 0 {
		t.Errorf("delete left %d rules", len(a.rules))
	}
	reopened, err := store.Open(a.store.Path())
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	if got := len(reopened.Data().Rules); got != 0 {
		t.Errorf("store kept %d rules", got)
	}
}

func TestRuleDialogValidation(t *testing.T) {
	a := newStoredTestApp(t)

	a.openNewRule(model.RuleMapRemote)
	a.saveRuleDialog()
	if a.ruleErr == "" || len(a.rules) != 0 {
		t.Fatalf("a rule without a host was saved: err=%q rules=%d", a.ruleErr, len(a.rules))
	}

	a.ruleHost = "api.example"
	a.ruleTarget = "not a url"
	a.saveRuleDialog()
	if a.ruleErr == "" || len(a.rules) != 0 {
		t.Fatalf("a rule with a bad target was saved: err=%q rules=%d", a.ruleErr, len(a.rules))
	}

	a.openNewRule(model.RuleResponseHeader)
	a.ruleHost = "api.example"
	a.saveRuleDialog()
	if a.ruleErr == "" || len(a.rules) != 0 {
		t.Fatalf("a header rule without a name was saved: err=%q rules=%d", a.ruleErr, len(a.rules))
	}

	a.ruleHeader = "Content-Length"
	a.saveRuleDialog()
	if a.ruleErr == "" || len(a.rules) != 0 {
		t.Fatalf("a framing header was accepted: err=%q rules=%d", a.ruleErr, len(a.rules))
	}
}

func TestRulesPageListsRules(t *testing.T) {
	a := newStoredTestApp(t)
	a.openNewRule(model.RuleMapRemote)
	a.ruleHost = "api.example"
	a.rulePath = "/v1"
	a.ruleTarget = "http://localhost:3000"
	a.ruleName = "Local API"
	a.saveRuleDialog()

	a.settingsOpen = true
	a.settingsPane = settingsPaneRules
	tt := newTester(t, a)
	tt.Frame()
	if !tt.HasText("Local API") {
		t.Fatalf("the Rules page does not list the rule; texts = %v", tt.Texts())
	}
	if !tt.HasText(locales.RuleNew()) {
		t.Errorf("the Rules page has no New Rule button")
	}
}

func TestTheFlowRulesSubmenuOpensTheEditor(t *testing.T) {
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
	if err := tt.ChooseMenuItem(locales.InspectorRules(), locales.RuleMapRemote()); err != nil {
		t.Fatalf("choose Map Remote: %v", err)
	}
	tt.Frame()

	if !a.ruleOpen || a.ruleKind != model.RuleMapRemote {
		t.Fatalf("the editor did not open: open=%v kind=%q", a.ruleOpen, a.ruleKind)
	}
	if a.ruleMethod != "POST" || a.ruleHost != "httpbin.org" || a.rulePath != "/anything" {
		t.Errorf("the rule was not prefilled from the flow: %+v", a.ruleHost)
	}
}

func TestThrottlePersists(t *testing.T) {
	a := newStoredTestApp(t)
	a.setThrottle(throttleFast3G)
	if a.throttle != throttleFast3G || a.prefs.Throttle != throttleFast3G {
		t.Errorf("throttle is %q, prefs %q", a.throttle, a.prefs.Throttle)
	}
	if got := normalizeThrottle("bogus"); got != throttleOff {
		t.Errorf("normalizeThrottle(bogus) = %q", got)
	}
	if throttleSpec(throttleSlow3G).BytesPerSec == 0 {
		t.Error("Slow 3G has no bandwidth cap")
	}
	reopened, err := store.Open(a.store.Path())
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	if got := reopened.Data().Prefs.Throttle; got != throttleFast3G {
		t.Errorf("store kept throttle %q", got)
	}
}
