package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/CarlOpenLab/omniflow/internal/httpx"
	"github.com/CarlOpenLab/omniflow/internal/uitokens"
)

func TestFindMatchesIgnoresCaseAndKeepsText(t *testing.T) {
	text := "Hello world, hello WORLD"
	matches := findMatches(text, "hello", 100)
	if len(matches) != 2 {
		t.Fatalf("found %d matches, want 2", len(matches))
	}
	if got := text[matches[0].start:matches[0].end]; got != "Hello" {
		t.Errorf("the first match is %q, want %q", got, "Hello")
	}
	if got := text[matches[1].start:matches[1].end]; got != "hello" {
		t.Errorf("the second match is %q, want %q", got, "hello")
	}
}

func TestFindMatchesFoldsUnicodeCase(t *testing.T) {
	text := "Ärger und ärger"
	matches := findMatches(text, "ÄRGER", 100)
	if len(matches) != 2 {
		t.Fatalf("found %d matches, want 2", len(matches))
	}
	if got := text[matches[0].start:matches[0].end]; got != "Ärger" {
		t.Errorf("the first match is %q, want %q", got, "Ärger")
	}
	if got := text[matches[1].start:matches[1].end]; got != "ärger" {
		t.Errorf("the second match is %q, want %q", got, "ärger")
	}
}

func TestFindMatchesDoesNotOverlapAndHonorsTheLimit(t *testing.T) {
	if got := findMatches("aaaa", "aa", 100); len(got) != 2 {
		t.Errorf("found %d matches of a repeated query, want 2 non-overlapping ones", len(got))
	}
	if got := findMatches("aaaa", "a", 2); len(got) != 2 {
		t.Errorf("found %d matches with a limit of 2, want 2", len(got))
	}
}

func TestFindMatchesWithoutAQueryFindsNothing(t *testing.T) {
	if got := findMatches("some text", "", 100); len(got) != 0 {
		t.Errorf("an empty query found %d matches", len(got))
	}
	if got := findMatches("", "query", 100); len(got) != 0 {
		t.Errorf("an empty text found %d matches", len(got))
	}
}

func TestFindLineColCountsLinesAndRunes(t *testing.T) {
	text := "{\n  \"a\": 1\néx\n}"
	at := strings.Index(text, `"a"`)
	if line, col := findLineCol(text, at); line != 1 || col != 2 {
		t.Errorf(`"a" is at line %d, column %d, want line 1, column 2`, line, col)
	}
	at = strings.Index(text, "x")
	if line, col := findLineCol(text, at); line != 2 || col != 1 {
		t.Errorf(`"x" is at line %d, column %d, want line 2, column 1`, line, col)
	}
}

func TestFindScrollOffsetPlacesTheMatchInTheCodeLayout(t *testing.T) {
	text := "one\ntwo\nthree"
	at := strings.Index(text, "three")
	x, y := findScrollOffset(text, at, 7)
	if x != codePadLeft {
		t.Errorf("x is %v, want the code's left padding %v", x, float32(codePadLeft))
	}
	if want := float32(codePadTop + 2*codeLineHeight); y != want {
		t.Errorf("y is %v, want %v", y, want)
	}
}

func TestHighlightMatchesKeepsTheTextAndColors(t *testing.T) {
	pal := uitokens.For(false)
	spans := []ui.Span{
		{Text: `{"name":`, Color: pal.CodePunct},
		{Text: `"alice"`, Color: pal.CodeString},
		{Text: `,"other":"name"}`, Color: pal.CodePunct},
	}
	text := spans[0].Text + spans[1].Text + spans[2].Text
	matches := findMatches(text, "name", 10)
	if len(matches) != 2 {
		t.Fatalf("found %d matches, want 2", len(matches))
	}
	highlighted := highlightMatches(spans, matches, 0, pal)

	var rebuilt strings.Builder
	for _, s := range highlighted {
		rebuilt.WriteString(s.Text)
	}
	if rebuilt.String() != text {
		t.Fatalf("the highlighted spans changed the text:\n%s", rebuilt.String())
	}

	// "alice" is between the matches and keeps its string color, without a
	// highlight.
	var alice *ui.Span
	for i := range highlighted {
		if highlighted[i].Text == `"alice"` {
			alice = &highlighted[i]
		}
	}
	if alice == nil {
		t.Fatal("the untouched string span is gone")
	}
	if alice.Color != pal.CodeString {
		t.Errorf("the untouched string lost its color")
	}
	if alice.Background != (ui.Color{}) {
		t.Errorf("the untouched string is highlighted")
	}

	// Both matches are highlighted, and the current one differently.
	var backgrounds []ui.Color
	for _, s := range highlighted {
		if strings.Contains(s.Text, "name") {
			backgrounds = append(backgrounds, s.Background)
		}
	}
	if len(backgrounds) != 2 {
		t.Fatalf("%d match spans are highlighted, want 2", len(backgrounds))
	}
	for _, bg := range backgrounds {
		if bg == (ui.Color{}) {
			t.Error("a match has no background")
		}
	}
	if backgrounds[0] == backgrounds[1] {
		t.Error("the current match and the others have the same background")
	}
}

func TestHighlightMatchesSpansPieceBoundaries(t *testing.T) {
	pal := uitokens.For(true)
	spans := []ui.Span{
		{Text: "ab", Color: pal.CodeKey},
		{Text: "cd", Color: pal.CodeString},
	}
	matches := []findMatch{{start: 1, end: 3}} // "bc"
	highlighted := highlightMatches(spans, matches, 0, pal)
	var rebuilt strings.Builder
	lit := 0
	for _, s := range highlighted {
		rebuilt.WriteString(s.Text)
		if s.Background != (ui.Color{}) {
			lit += len(s.Text)
		}
	}
	if rebuilt.String() != "abcd" {
		t.Fatalf("the highlighted spans changed the text to %q", rebuilt.String())
	}
	if lit != 2 {
		t.Errorf("%d characters are highlighted, want 2", lit)
	}
}

func TestHighlightMatchesWithoutMatchesReturnsTheSpans(t *testing.T) {
	pal := uitokens.For(false)
	spans := []ui.Span{{Text: "plain", Color: pal.CodeText}}
	got := highlightMatches(spans, nil, -1, pal)
	if !slices.Equal(got, spans) {
		t.Errorf("spans without matches changed: %#v", got)
	}
}

func TestFindStateSearchesAgainWhenTheTextOrQueryChanges(t *testing.T) {
	f := &findState{open: true, query: "a"}
	if !f.sync("aaa") {
		t.Fatal("the first search did not report a change")
	}
	if len(f.matches) != 3 || f.current != 0 || !f.moved {
		t.Fatalf("searched %d matches, current %d, moved %v, want 3, 0, true",
			len(f.matches), f.current, f.moved)
	}

	f.current = 2
	f.moved = false
	f.sync("aaa")
	if f.current != 2 || f.moved {
		t.Errorf("an unchanged search moved the current match to %d (moved %v)", f.current, f.moved)
	}

	f.sync("banana")
	if len(f.matches) != 3 || f.current != 0 || !f.moved {
		t.Errorf("new text found %d matches, current %d, moved %v", len(f.matches), f.current, f.moved)
	}

	f.query = "n"
	f.moved = false
	f.sync("banana")
	if len(f.matches) != 2 || f.current != 0 || !f.moved {
		t.Errorf("a new query found %d matches, current %d, moved %v", len(f.matches), f.current, f.moved)
	}

	f.current = 9
	f.sync("banana")
	if f.current != 1 {
		t.Errorf("the current match is %d after it went past the matches, want 1", f.current)
	}
}

func TestFindStateNamesTheCurrentMatch(t *testing.T) {
	f := &findState{query: "ab"}
	f.sync("ab xx ab")
	m, ok := f.currentMatch()
	if !ok || m.start != 0 || m.end != 2 {
		t.Errorf("current match is %+v, %v, want the first ab", m, ok)
	}
	f.current = 1
	if m, ok = f.currentMatch(); !ok || m.start != 6 || m.end != 8 {
		t.Errorf("current match is %+v, %v, want the second ab", m, ok)
	}
	f.query = "zzz"
	f.sync("ab xx ab")
	if _, ok = f.currentMatch(); ok {
		t.Error("a search without matches named one")
	}
}

func TestFindSpansKeepTheColorsAndHighlightTheMatches(t *testing.T) {
	pal := uitokens.For(false)
	text := `{"name":"alice","other":"name"}`
	matches := findMatches(text, "name", 10)
	spans := findSpans(text, matches, 1, pal)
	var rebuilt strings.Builder
	lit := 0
	for _, s := range spans {
		rebuilt.WriteString(s.Text)
		if s.Background != (ui.Color{}) {
			lit += len(s.Text)
		}
	}
	if rebuilt.String() != text {
		t.Fatalf("the colored spans changed the text:\n%s", rebuilt.String())
	}
	if lit == 0 {
		t.Error("the matches are not highlighted in the colored text")
	}

	plain := findSpans("hello world", findMatches("hello world", "world", 10), 0, pal)
	if len(plain) == 0 || plain[0].Color != pal.CodeText {
		t.Errorf("plain text is not drawn in the code's color: %#v", plain)
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
