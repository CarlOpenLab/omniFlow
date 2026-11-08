package main

import (
	"testing"
	"time"

	"github.com/CarlOpenLab/omniflow/internal/model"
	"github.com/CarlOpenLab/omniflow/locales"
)

func TestHistoryRowRemovesOneEntry(t *testing.T) {
	a := newTestApp(t)
	a.history = []model.HistoryEntry{
		{ID: "h1", Method: "GET", URL: "https://example.com/one", Status: 200, At: time.Now()},
		{ID: "h2", Method: "GET", URL: "https://example.com/two", Status: 200, At: time.Now()},
	}
	tt := newTester(t, a)
	tt.Frame()

	row, ok := tt.Find("example.com/one")
	if !ok {
		t.Fatalf("the history row is not on screen; texts = %v", tt.Texts())
	}
	tt.Move(row.X+row.W/2, row.Y+row.H/2)
	tt.Frame()

	remove, ok := tt.Find(locales.CommonRemoveHistory())
	if !ok {
		t.Fatalf("the hovered row has no remove button; texts = %v", tt.Texts())
	}
	tt.ClickAt(remove.X+remove.W/2, remove.Y+remove.H/2)
	tt.Frame()

	if len(a.history) != 1 || a.history[0].ID != "h2" {
		t.Fatalf("history after remove is %+v, want only h2", a.history)
	}
}
