package main

import (
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"

	"github.com/CarlOpenLab/omniflow/internal/uitokens"
)

// maxFindMatches bounds a search, so a common letter in a large body does
// not make the highlight spans heavy.
const maxFindMatches = 5000

// The code view's box: its padding and line height, which place a match as
// the view scrolls to it.
const (
	codePadTop     = 10
	codePadLeft    = 14
	codeLineHeight = 18
)

// findMatch is one occurrence of a find's query, as byte offsets in the
// text it was found in.
type findMatch struct {
	start, end int
}

// findState is a body's find bar and what it found, kept while the app
// runs. The bar itself writes open, query and current.
type findState struct {
	open    bool
	query   string
	current int
	matches []findMatch
	// text and searched are what the matches were found in and for, so a
	// search runs only when one of them changed.
	text     string
	searched string
	// moved asks the view to scroll to the current match.
	moved bool
}

// sync searches text again when it or the query changed, choosing the
// first match; otherwise it keeps the current one within the matches.
func (f *findState) sync(text string) bool {
	if f.text == text && f.searched == f.query {
		f.clamp()
		return false
	}
	f.text, f.searched = text, f.query
	f.matches = findMatches(text, f.query, maxFindMatches)
	f.current = 0
	f.moved = true
	return true
}

// clamp keeps the current match among the matches there are.
func (f *findState) clamp() {
	if len(f.matches) == 0 {
		f.current = 0
		return
	}
	f.current = max(0, min(f.current, len(f.matches)-1))
}

// currentMatch is the match the find bar is on, and whether there is one.
func (f *findState) currentMatch() (findMatch, bool) {
	if f.current < 0 || f.current >= len(f.matches) {
		return findMatch{}, false
	}
	return f.matches[f.current], true
}

// findMatches finds the occurrences of query in text, ignoring case, at
// most limit of them, from left to right and without overlaps. It compares
// runes folded to lower case; the offsets are those of the text as written.
func findMatches(text, query string, limit int) []findMatch {
	if text == "" || query == "" || limit <= 0 {
		return nil
	}
	runes, at := foldRunes(text)
	want, _ := foldRunes(query)
	if len(want) == 0 {
		return nil
	}
	var matches []findMatch
	for i := 0; i+len(want) <= len(runes); {
		if equalRunes(runes[i:i+len(want)], want) {
			matches = append(matches, findMatch{start: at[i], end: at[i+len(want)]})
			if len(matches) == limit {
				break
			}
			i += len(want)
			continue
		}
		i++
	}
	return matches
}

// foldRunes lowercases s rune by rune, and gives the byte offset of each
// folded rune, with the length of s after the last.
func foldRunes(s string) ([]rune, []int) {
	runes := make([]rune, 0, len(s))
	at := make([]int, 1, len(s)+1)
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		runes = append(runes, unicode.ToLower(r))
		i += size
		at = append(at, i)
	}
	return runes, at
}

func equalRunes(a, b []rune) bool {
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// findLineCol is where byte offset at falls in text: its line from 0, and
// its column in runes from the start of that line.
func findLineCol(text string, at int) (line, col int) {
	at = max(0, min(at, len(text)))
	before := text[:at]
	line = strings.Count(before, "\n")
	start := strings.LastIndexByte(before, '\n') + 1
	return line, utf8.RuneCountInString(text[start:at])
}

// findScrollOffset is where a match at byte offset at sits in the code
// view's content, given the width of one monospaced character.
func findScrollOffset(text string, at int, charWidth float32) (x, y float32) {
	line, col := findLineCol(text, at)
	return codePadLeft + float32(col)*charWidth, codePadTop + float32(line)*codeLineHeight
}

// highlightMatches colors the matches in spans, which lay out text in
// order: every match takes the find's background, the current one a
// heavier one, and everything else keeps the span's color and text.
func highlightMatches(spans []ui.Span, matches []findMatch, current int, pal *uitokens.Palette) []ui.Span {
	if len(matches) == 0 {
		return spans
	}
	other, active := pal.Warning.Alpha(0.18), pal.Warning.Alpha(0.45)
	var out []ui.Span
	pos := 0
	for _, sp := range spans {
		start, end := pos, pos+len(sp.Text)
		pos = end
		// The first match that can reach into this span; those are sorted.
		i := sort.Search(len(matches), func(i int) bool { return matches[i].end > start })
		if i == len(matches) || matches[i].start >= end {
			out = append(out, sp)
			continue
		}
		cur := start
		for ; i < len(matches) && matches[i].start < end; i++ {
			a, b := max(matches[i].start, start), min(matches[i].end, end)
			if a >= b {
				continue
			}
			if a > cur {
				out = append(out, spanPart(sp, sp.Text[cur-start:a-start]))
			}
			part := spanPart(sp, sp.Text[a-start:b-start])
			if i == current {
				part.Background = active
			} else {
				part.Background = other
			}
			out = append(out, part)
			cur = b
		}
		if cur < end {
			out = append(out, spanPart(sp, sp.Text[cur-start:]))
		}
	}
	return out
}

// spanPart is a span of the same style with a piece of its text.
func spanPart(s ui.Span, text string) ui.Span {
	s.Text = text
	return s
}

// findSpans lays text out as the code view does, with the matches
// highlighted: JSON keeps its colors, and other text is one plain run.
func findSpans(text string, matches []findMatch, current int, pal *uitokens.Palette) []ui.Span {
	base, ok := jsonSpans(text, pal)
	if !ok {
		base = []ui.Span{{Text: text, Color: pal.CodeText}}
	}
	return highlightMatches(base, matches, current, pal)
}

// scrollToFind scrolls a body to the match the find bar is on, with the
// match a little below and right of the view's corner.
func (a *app) scrollToFind(c *ui.Context, text string, scroll *ui.ScrollState) {
	m, ok := a.find.currentMatch()
	if !ok {
		return
	}
	width, _ := c.MeasureText(0, ui.Span{Text: "MMMMMMMMMM", Font: "monospace", Size: 11.5})
	x, y := findScrollOffset(text, m.start, width/10)
	scroll.X = max(0, x-120)
	scroll.Y = max(0, y-72)
}

// canFind reports whether the body on show is text the find bar can search:
// in the composer, the response's body; in the inspector, the chosen flow's
// request or response body.
func (a *app) canFind() bool {
	switch a.mode {
	case modeComposer:
		if a.resp == nil || a.respTab != 0 || len(a.resp.Body) == 0 {
			return false
		}
		return shapeOf(a.resp.Body, contentTypeOf(a.resp.Headers)) == shapeText
	case modeInspector:
		f := a.selectedFlow()
		if f == nil || a.detailSub != 0 {
			return false
		}
		body, contentType := f.RespBody, f.ContentType
		if a.detailTab == 0 {
			body, contentType = f.ReqBody, headerValue(f.ReqHeaders, "Content-Type")
		}
		return len(body) > 0 && shapeOf(body, contentType) == shapeText
	}
	return false
}
