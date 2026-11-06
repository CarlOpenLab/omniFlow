package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/CarlOpenLab/omniflow/internal/httpx"
)

// textBody builds a plain text body of about size bytes whose last line
// says where it ends.
func textBody(size int) []byte {
	var b strings.Builder
	for i := 0; b.Len() < size; i++ {
		fmt.Fprintf(&b, "line %06d: the quick brown fox jumps over the lazy dog\n", i)
	}
	b.WriteString("END OF BODY")
	return []byte(b.String())
}

func TestLargeTextBodyIsShownWhole(t *testing.T) {
	a := newTestApp(t)
	a.pretty = false
	body := textBody(2 << 20)
	a.resp = &httpx.Result{
		Status:  200,
		Headers: []httpx.Header{{Key: "Content-Type", Value: "text/plain"}},
		Body:    body,
	}
	tt := newTester(t, a)
	tt.Frame()

	st := a.body.st
	if st == nil {
		t.Fatal("the response body has no editor")
	}
	if got := st.Buffer.Len(); got != len(body) {
		t.Fatalf("the editor holds %d bytes of the body, want all %d", got, len(body))
	}
	last := st.Buffer.LineCount() - 1
	if got := st.Buffer.Line(last); got != "END OF BODY" {
		t.Fatalf("the editor's last line is %q, want the body's end", got)
	}

	st.ScrollTo(last)
	tt.Frame()
	first, shownLast := st.VisibleLines()
	if first > last || shownLast < last {
		t.Errorf("scrolled to the end, lines %d..%d of %d are shown", first, shownLast, last)
	}
}

func TestCanFindOnlyInTextBodies(t *testing.T) {
	a := newTestApp(t)
	if a.canFind() {
		t.Error("the composer with no response can find")
	}
	a.resp = &httpx.Result{Status: 200, Body: []byte("hello")}
	if !a.canFind() {
		t.Error("the composer's text response cannot be found")
	}
	a.respTab = 1
	if a.canFind() {
		t.Error("the composer's headers tab can find")
	}
	a.respTab = 0
	a.resp.Headers = []httpx.Header{{Key: "Content-Type", Value: "image/png"}}
	a.resp.Body = []byte("\x89PNG\r\n\x1a\n")
	if a.canFind() {
		t.Error("an image body can find")
	}
	a.resp.Headers = []httpx.Header{{Key: "Content-Type", Value: "application/json"}}
	a.resp.Body = []byte(`{"a":1}`)
	if !a.canFind() {
		t.Error("the composer's JSON response cannot be found")
	}
	a.resp.Body = nil
	if a.canFind() {
		t.Error("an empty body can find")
	}

	a.mode = modeInspector
	seedFlows(a)
	if a.canFind() {
		t.Error("the inspector with no flow chosen can find")
	}
	a.flowSelID = 3 // a POST whose request and response are JSON
	if !a.canFind() {
		t.Error("the inspector's request body cannot be found")
	}
	a.detailTab = 1
	if !a.canFind() {
		t.Error("the inspector's response body cannot be found")
	}
	a.detailSub = 1
	if a.canFind() {
		t.Error("the inspector's headers can find")
	}
}

func TestPrettyBodyIsFormattedOnce(t *testing.T) {
	a := newTestApp(t)
	a.pretty = true
	calls := 0
	prev := prettyJSON
	prettyJSON = func(body []byte) (string, bool) {
		calls++
		return "{\n  \"a\": 1\n}", true
	}
	defer func() { prettyJSON = prev }()

	a.resp = &httpx.Result{
		Status:  200,
		Headers: []httpx.Header{{Key: "Content-Type", Value: "application/json"}},
		Body:    []byte(`{"a":1}`),
	}
	tt := newTester(t, a)
	tt.Frame()
	tt.Frame()
	if calls != 1 {
		t.Fatalf("the body was formatted %d times over three frames, want once", calls)
	}
}
