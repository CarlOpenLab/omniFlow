package main

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/CarlOpenLab/omniflow/internal/uitokens"
)

func jsonClass(pal *uitokens.Palette, col ui.Color) string {
	switch col {
	case pal.CodeKey:
		return "key"
	case pal.CodeString:
		return "string"
	case pal.CodeNumber:
		return "number"
	case pal.CodeLiteral:
		return "literal"
	}
	return ""
}

func TestJSONSpansColorTokensAndKeepTheText(t *testing.T) {
	pal := uitokens.For(false)
	text := "{\n  \"key\": \"value\",\n  \"n\": 12.5,\n  \"yes\": true,\n  \"no\": null\n}"
	spans, ok := jsonSpans(text, pal)
	if !ok {
		t.Fatal("valid JSON was not highlighted")
	}
	var rebuilt strings.Builder
	classes := map[string]string{}
	for _, s := range spans {
		rebuilt.WriteString(s.Text)
		if class := jsonClass(pal, s.Color); class != "" {
			classes[s.Text] = class
		}
	}
	if rebuilt.String() != text {
		t.Errorf("the spans changed the document:\n%s", rebuilt.String())
	}

	want := map[string]string{
		`"key"`:   "key",
		`"value"`: "string",
		"12.5":    "number",
		"true":    "literal",
		"null":    "literal",
	}
	for token, class := range want {
		if got := classes[token]; got != class {
			t.Errorf("%q is colored as %q, want %q", token, got, class)
		}
	}

	if _, ok := jsonSpans("<html>not json</html>", pal); ok {
		t.Error("HTML was highlighted as JSON")
	}
	if _, ok := jsonSpans(`{"unterminated":`, pal); ok {
		t.Error("invalid JSON was highlighted")
	}
}

func TestJSONSpansSkipHugeBodies(t *testing.T) {
	pal := uitokens.For(true)
	large := `{"filler":"` + strings.Repeat("x", highlightLimit) + `"}`
	if _, ok := jsonSpans(large, pal); ok {
		t.Error("a body over the limit was highlighted")
	}
}
