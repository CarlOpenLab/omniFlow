package main

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/CarlOpenLab/omniflow/internal/httpx"
)

func TestCommandFFindsInTheResponseBody(t *testing.T) {
	a := newTestApp(t)
	a.pretty = false
	a.resp = &httpx.Result{Status: 200, Body: []byte("hello world, hello again")}
	tt := newTester(t, a)

	tt.Key(ui.Cmd, ui.KeyF)
	tt.Frame()
	tt.Type("hello")
	tt.Frame()
	if a.body.st == nil {
		t.Fatal("⌘F opened no find on the response body")
	}
	count, current := a.body.st.FindMatches()
	if count != 2 || current != 0 {
		t.Fatalf("the find found %d matches, at %d, want 2 at 0", count, current)
	}
	if !tt.HasText("1 of 2") {
		t.Errorf("the find bar does not say 1 of 2: %q", tt.Texts())
	}

	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if _, current = a.body.st.FindMatches(); current != 1 {
		t.Errorf("Enter chose match %d, want 1", current)
	}
	if !tt.HasText("2 of 2") {
		t.Errorf("the find bar does not say 2 of 2: %q", tt.Texts())
	}

	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if tt.HasText("1 of 2") || tt.HasText("2 of 2") {
		t.Error("Escape left the find bar open")
	}
}

func TestInspectorFindsInTheBodyAndFiltersWithShiftCommandF(t *testing.T) {
	a := newTestApp(t)
	seedFlows(a)
	a.mode = modeInspector
	a.flowSelID = 4 // a JSON response that mentions mygo once
	a.detailTab = 1
	a.detailSub = 0
	tt := newTester(t, a)

	tt.Key(ui.Cmd, ui.KeyF)
	tt.Frame()
	tt.Type("mygo")
	tt.Frame()
	if a.body.st == nil {
		t.Fatal("⌘F opened no find on the inspector's body")
	}
	if count, _ := a.body.st.FindMatches(); count != 1 {
		t.Fatalf("the find found %d matches, want 1", count)
	}
	if !tt.HasText("1 of 1") {
		t.Errorf("the find bar does not say 1 of 1: %q", tt.Texts())
	}

	tt.Key(ui.Cmd|ui.Shift, ui.KeyF)
	tt.Frame()
	tt.Type("github")
	tt.Frame()
	if a.flowFilter != "github" {
		t.Errorf("⇧⌘F did not focus the traffic filter: it holds %q", a.flowFilter)
	}
}

func TestFindClosesWhenTheBodyIsNoLongerThere(t *testing.T) {
	a := newTestApp(t)
	a.pretty = false
	a.resp = &httpx.Result{Status: 200, Body: []byte("hello")}
	tt := newTester(t, a)

	tt.Key(ui.Cmd, ui.KeyF)
	tt.Frame()
	if a.body.st == nil {
		t.Fatal("⌘F opened no find on the body")
	}

	a.respTab = 1 // Headers, where there is no body to search
	tt.Frame()
	if a.body.st != nil {
		t.Error("the body's editor stayed over the headers")
	}

	a.respTab = 0
	tt.Key(ui.Cmd, ui.KeyF)
	tt.Frame()
	if a.body.st == nil {
		t.Error("⌘F could not open find again after the body returned")
	}
}

func TestCommandFDoesNothingWithoutATextBody(t *testing.T) {
	a := newTestApp(t)
	a.resp = &httpx.Result{
		Status:  200,
		Headers: []httpx.Header{{Key: "Content-Type", Value: "image/png"}},
		Body:    []byte("\x89PNG\r\n\x1a\n"),
	}
	tt := newTester(t, a)

	tt.Key(ui.Cmd, ui.KeyF)
	tt.Frame()
	if a.body.st != nil {
		t.Error("⌘F opened a find over an image body")
	}
}
