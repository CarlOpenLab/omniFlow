package httpx

import (
	"slices"
	"strings"
	"testing"

	"github.com/CarlOpenLab/omniflow/internal/model"
)

// headerOf is a header the import wrote, "" when it wrote none.
func headerOf(r model.Request, key string) string {
	for _, h := range r.Headers {
		if strings.EqualFold(h.Key, key) {
			return h.Value
		}
	}
	return ""
}

func TestParseCurlReadsABrowsersCopy(t *testing.T) {
	cmd := `curl 'https://api.example.com/v1/items?limit=5&b=2' \
  -H 'accept: application/json' \
  -H 'authorization: Bearer abc' \
  -H 'accept-encoding: gzip, deflate, br' \
  -H 'content-type: application/json' \
  --data-raw '{"name":"it'\''s"}' \
  --compressed`
	r, notes, err := ParseCurl(cmd)
	if err != nil {
		t.Fatalf("ParseCurl: %v", err)
	}
	if r.Method != "POST" || r.URL != "https://api.example.com/v1/items?limit=5&b=2" {
		t.Errorf("request %s %s", r.Method, r.URL)
	}
	if r.BodyKind != model.BodyJSON || r.Body != `{"name":"it's"}` {
		t.Errorf("body %s %q", r.BodyKind, r.Body)
	}
	if headerOf(r, "authorization") != "Bearer abc" || headerOf(r, "accept-encoding") != "" {
		t.Errorf("headers %+v", r.Headers)
	}
	if len(notes) != 0 {
		t.Errorf("notes %q", notes)
	}
}

func TestParseCurlBodies(t *testing.T) {
	for _, tc := range []struct {
		cmd      string
		method   string
		url      string
		kind     string
		body     string
		form     []string
		ctHeader string
	}{
		// A form that writes out again as it came becomes a table.
		{`curl -d 'a=1' -d 'b=two+words' https://x.test/f`, "POST", "https://x.test/f", model.BodyForm, "", []string{"a=1", "b=two words"}, ""},
		// One that would not is sent as the text it is, with curl's type.
		{`curl -d 'a=two%20words' https://x.test/f`, "POST", "https://x.test/f", model.BodyText, "a=two%20words", nil, "application/x-www-form-urlencoded"},
		{`curl -X PUT https://x.test/j -H "Content-Type: application/json" -d '{"a":1}'`, "PUT", "https://x.test/j", model.BodyJSON, `{"a":1}`, nil, "application/json"},
		{`curl --json '{"a":1}' https://x.test/j`, "POST", "https://x.test/j", model.BodyJSON, `{"a":1}`, nil, ""},
		{`curl -H 'Content-Type: text/csv' --data-binary $'a,b\n1,2' https://x.test/c`, "POST", "https://x.test/c", model.BodyText, "a,b\n1,2", nil, "text/csv"},
		{`curl -G -d q=1 --data-urlencode 'tag=a b' 'https://x.test/s?a=0'`, "GET", "https://x.test/s?a=0&q=1&tag=a+b", model.BodyNone, "", nil, ""},
		{`curl -I https://x.test/h`, "HEAD", "https://x.test/h", model.BodyNone, "", nil, ""},
		{`curl -XDELETE https://x.test/d`, "DELETE", "https://x.test/d", model.BodyNone, "", nil, ""},
		{`curl -sSL -o out.txt --max-time 5 -k https://x.test/q`, "GET", "https://x.test/q", model.BodyNone, "", nil, ""},
	} {
		r, _, err := ParseCurl(tc.cmd)
		if err != nil {
			t.Errorf("ParseCurl(%s): %v", tc.cmd, err)
			continue
		}
		if r.Method != tc.method || r.URL != tc.url || r.BodyKind != tc.kind || r.Body != tc.body {
			t.Errorf("ParseCurl(%s) = %s %s, %s %q", tc.cmd, r.Method, r.URL, r.BodyKind, r.Body)
		}
		var form []string
		for _, kv := range r.Form {
			if kv.Key != "" {
				form = append(form, kv.Key+"="+kv.Value)
			}
		}
		if !slices.Equal(form, tc.form) {
			t.Errorf("ParseCurl(%s) form %q, want %q", tc.cmd, form, tc.form)
		}
		if got := headerOf(r, "Content-Type"); got != tc.ctHeader {
			t.Errorf("ParseCurl(%s) content type %q, want %q", tc.cmd, got, tc.ctHeader)
		}
	}
}

func TestParseCurlAuthQuotingAndNotes(t *testing.T) {
	r, notes, err := ParseCurl(`curl -u 'carl:pa:ss' -A probe/1 -b 'id=7' -F 'file=@a.png' $'https://x.test/\x41é' -H $'X-Note: a\\tb'`)
	if err != nil {
		t.Fatalf("ParseCurl: %v", err)
	}
	if r.AuthType != model.AuthBasic || r.User != "carl" || r.Pass != "pa:ss" {
		t.Errorf("auth %s %q %q", r.AuthType, r.User, r.Pass)
	}
	if r.URL != "https://x.test/Aé" {
		t.Errorf("URL %q", r.URL)
	}
	if headerOf(r, "User-Agent") != "probe/1" || headerOf(r, "Cookie") != "id=7" || headerOf(r, "X-Note") != `a\tb` {
		t.Errorf("headers %+v", r.Headers)
	}
	if !slices.Contains(notes, "multipart form fields (-F)") {
		t.Errorf("notes %q", notes)
	}

	for _, bad := range []string{`curl 'https://x.test`, `wget https://x.test`, `curl -H 'A: b'`, ``} {
		if _, _, err := ParseCurl(bad); err == nil {
			t.Errorf("ParseCurl(%q) succeeded", bad)
		}
	}
	if !LooksLikeCurl("  curl https://x.test\n") || LooksLikeCurl("curl") || LooksLikeCurl("https://x.test") {
		t.Error("LooksLikeCurl misjudged a paste")
	}
}

func TestCurlRoundTrips(t *testing.T) {
	spec := Spec{
		Method: "PATCH", URL: "https://x.test/items/1?v=2",
		Headers:  []model.KV{{Enabled: true, Key: "X-Quote", Value: `it's "here"`}},
		BodyKind: model.BodyJSON, Body: "{\n  \"a\": \"b'c\"\n}",
		AuthType: model.AuthBearer, Token: "t0k",
	}
	cmd, err := Curl(spec)
	if err != nil {
		t.Fatalf("Curl: %v", err)
	}
	r, _, err := ParseCurl(cmd)
	if err != nil {
		t.Fatalf("ParseCurl(%s): %v", cmd, err)
	}
	if r.Method != "PATCH" || r.URL != "https://x.test/items/1?v=2" || r.Body != spec.Body || r.BodyKind != model.BodyJSON {
		t.Errorf("round trip gave %s %s %s %q", r.Method, r.URL, r.BodyKind, r.Body)
	}
	if headerOf(r, "X-Quote") != `it's "here"` || headerOf(r, "Authorization") != "Bearer t0k" {
		t.Errorf("round trip headers %+v", r.Headers)
	}
}
