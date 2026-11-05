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
	if !a.find.open {
		t.Fatal("⌘F did not open the find bar")
	}
	tt.Type("hello")
	tt.Frame()
	if len(a.find.matches) != 2 {
		t.Fatalf("the find bar found %d matches, want 2", len(a.find.matches))
	}
	if !tt.HasText("1 of 2") {
		t.Errorf("the find bar does not say 1 of 2: %q", tt.Texts())
	}

	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if a.find.current != 1 {
		t.Errorf("Enter chose match %d, want 1", a.find.current)
	}
	if !tt.HasText("2 of 2") {
		t.Errorf("the find bar does not say 2 of 2: %q", tt.Texts())
	}

	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if a.find.open {
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
	if !a.find.open {
		t.Fatal("⌘F did not open the find bar in the inspector")
	}
	tt.Type("mygo")
	tt.Frame()
	if len(a.find.matches) != 1 {
		t.Fatalf("the find bar found %d matches, want 1", len(a.find.matches))
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
	if !a.find.open {
		t.Fatal("⌘F did not open the find bar")
	}

	a.respTab = 1 // Headers, where there is no body to search
	tt.Frame()
	if a.find.open {
		t.Error("the find bar stayed open over the headers")
	}

	a.respTab = 0
	tt.Key(ui.Cmd, ui.KeyF)
	if !a.find.open {
		t.Error("⌘F could not open the find bar again after the body returned")
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
	if a.find.open {
		t.Error("⌘F opened the find bar over an image body")
	}
}
