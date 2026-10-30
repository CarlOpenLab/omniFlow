package httpx

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/CarlOpenLab/omniflow/internal/model"
)

func TestBuildURLAddsEnabledParams(t *testing.T) {
	u, err := BuildURL("example.com/a?x=1", []model.KV{
		{Enabled: true, Key: "page", Value: "2"},
		{Enabled: false, Key: "draft", Value: "true"},
		{Enabled: true, Key: " ", Value: "ignored"},
	})
	if err != nil {
		t.Fatalf("BuildURL: %v", err)
	}
	if got, want := u.Scheme, "https"; got != want {
		t.Errorf("scheme %q, want %q", got, want)
	}
	if got, want := u.Host, "example.com"; got != want {
		t.Errorf("host %q, want %q", got, want)
	}
	// The written query comes first, as it was; the table's rows follow.
	if got, want := u.RawQuery, "x=1&page=2"; got != want {
		t.Errorf("query %q, want %q", got, want)
	}
}

func TestBuildURLKeepsTheQueryAsWritten(t *testing.T) {
	for raw, want := range map[string]string{
		// A signed URL breaks if its query is sorted or encoded again.
		"https://cdn.example.com/f?X-Sig=a%2Fb%3D&b=2&a=1": "X-Sig=a%2Fb%3D&b=2&a=1",
		"https://example.com/search?q=two words&tag=a|b":   "q=two%20words&tag=a%7Cb",
		"https://example.com/?":                            "",
		"https://example.com/plain":                        "",
	} {
		u, err := BuildURL(raw, nil)
		if err != nil {
			t.Fatalf("BuildURL(%q): %v", raw, err)
		}
		if u.RawQuery != want {
			t.Errorf("BuildURL(%q) query %q, want %q", raw, u.RawQuery, want)
		}
	}
}

func TestRedirectsAreFollowedCountedOrStopped(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/middle", http.StatusFound) })
	mux.HandleFunc("/middle", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/end", http.StatusMovedPermanently) })
	mux.HandleFunc("/end", func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "here") })
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	res := New().Do(t.Context(), Spec{URL: upstream.URL + "/start"})
	if res.Err != "" || res.Status != http.StatusOK || string(res.Body) != "here" {
		t.Fatalf("followed: %d %q %s", res.Status, res.Body, res.Err)
	}
	if res.Redirects != 2 || res.URL != upstream.URL+"/end" {
		t.Errorf("followed %d redirects to %q", res.Redirects, res.URL)
	}

	res = New().Do(t.Context(), Spec{URL: upstream.URL + "/start", NoRedirects: true})
	if res.Err != "" || res.Status != http.StatusFound {
		t.Fatalf("stopped: %d %s", res.Status, res.Err)
	}
	if res.Redirects != 0 || res.URL != "" {
		t.Errorf("a stopped redirect reports %d redirects to %q", res.Redirects, res.URL)
	}
}

func TestInsecureAcceptsASelfSignedCertificate(t *testing.T) {
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "ok")
	}))
	defer upstream.Close()

	client := New()
	if res := client.Do(t.Context(), Spec{URL: upstream.URL}); !strings.Contains(res.Err, "TLS verification failed") {
		t.Errorf("a self-signed certificate was accepted, or failed with %q", res.Err)
	}
	if res := client.Do(t.Context(), Spec{URL: upstream.URL, Insecure: true}); res.Err != "" || string(res.Body) != "ok" {
		t.Errorf("an insecure send failed: %q %q", res.Err, res.Body)
	}
	// The connection made without checks is not reused by a send that
	// wants them.
	if res := client.Do(t.Context(), Spec{URL: upstream.URL}); res.Err == "" {
		t.Error("a verifying send reused the unverified connection")
	}
}

func TestBuildURLRejectsWhatItCannotSend(t *testing.T) {
	for _, raw := range []string{"", "   ", "ftp://example.com/file", "https:///path"} {
		if _, err := BuildURL(raw, nil); err == nil {
			t.Errorf("BuildURL(%q) succeeded, want an error", raw)
		}
	}
}

func TestDoSendsRequestAndReadsAnswer(t *testing.T) {
	var seen struct {
		method, query, body, bearer, contentType string
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		seen.method = r.Method
		seen.query = r.URL.RawQuery
		seen.body = string(body)
		seen.bearer = r.Header.Get("Authorization")
		seen.contentType = r.Header.Get("Content-Type")
		w.Header().Set("X-One", "a")
		w.Header().Add("X-One", "b")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	defer upstream.Close()

	client := New()
	res := client.Do(t.Context(), Spec{
		Method: "post",
		URL:    upstream.URL + "/things",
		Params: []model.KV{{Enabled: true, Key: "page", Value: "2"}},
		Headers: []model.KV{
			{Enabled: true, Key: "X-Omni", Value: "yes"},
		},
		BodyKind: model.BodyJSON,
		Body:     `{"hello":"world"}`,
		AuthType: model.AuthBearer,
		Token:    "secret",
		Timeout:  5 * time.Second,
	})
	if res.Err != "" {
		t.Fatalf("Do: %s", res.Err)
	}
	if res.Status != http.StatusAccepted || res.StatusText != "Accepted" {
		t.Errorf("status %d %q", res.Status, res.StatusText)
	}
	if res.Proto == "" {
		t.Error("proto is empty")
	}
	if string(res.Body) != `{"ok":true}` {
		t.Errorf("body %q", res.Body)
	}
	if res.Duration <= 0 {
		t.Errorf("duration %v", res.Duration)
	}
	if seen.method != "POST" {
		t.Errorf("upstream method %q", seen.method)
	}
	if seen.query != "page=2" {
		t.Errorf("upstream query %q", seen.query)
	}
	if seen.body != `{"hello":"world"}` {
		t.Errorf("upstream body %q", seen.body)
	}
	if seen.bearer != "Bearer secret" {
		t.Errorf("upstream authorization %q", seen.bearer)
	}
	if seen.contentType != "application/json" {
		t.Errorf("upstream content type %q", seen.contentType)
	}
	if len(res.Headers) < 3 {
		t.Fatalf("headers %v", res.Headers)
	}
	for i := 1; i < len(res.Headers); i++ {
		if res.Headers[i-1].Key > res.Headers[i].Key {
			t.Errorf("headers are not sorted: %v", res.Headers)
		}
	}
}

func TestDoKeepsExplicitContentTypeAndBasicAuth(t *testing.T) {
	var contentType, authorization string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentType = r.Header.Get("Content-Type")
		authorization = r.Header.Get("Authorization")
	}))
	defer upstream.Close()

	res := New().Do(t.Context(), Spec{
		Method: "POST", URL: upstream.URL,
		Headers:  []model.KV{{Enabled: true, Key: "Content-Type", Value: "text/plain; charset=utf-8"}},
		BodyKind: model.BodyJSON, Body: `{}`,
		AuthType: model.AuthBasic, User: "carl", Pass: "hunter2",
	})
	if res.Err != "" {
		t.Fatalf("Do: %s", res.Err)
	}
	if contentType != "text/plain; charset=utf-8" {
		t.Errorf("content type %q", contentType)
	}
	if !strings.HasPrefix(authorization, "Basic ") {
		t.Errorf("authorization %q", authorization)
	}
}

func TestADeadlineIsReportedAsATimeout(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(150 * time.Millisecond)
		_, _ = io.WriteString(w, "late")
	}))
	defer upstream.Close()

	res := New().Do(t.Context(), Spec{URL: upstream.URL, Timeout: 20 * time.Millisecond})
	if res.Err == "" {
		t.Fatal("Do succeeded, want a timeout")
	}
	if !strings.Contains(res.Err, "timed out") {
		t.Errorf("error %q does not mention a timeout", res.Err)
	}
}

func TestErrorStatusIsStillAResult(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	}))
	defer upstream.Close()

	res := New().Do(t.Context(), Spec{URL: upstream.URL})
	if res.Err != "" {
		t.Fatalf("a 404 came back as an error: %s", res.Err)
	}
	if res.Status != http.StatusNotFound {
		t.Errorf("status %d", res.Status)
	}
}

func TestPrettyAndFormatting(t *testing.T) {
	pretty, ok := Pretty([]byte(`{"a":1}`))
	if !ok || !strings.Contains(pretty, "\n") {
		t.Errorf("Pretty returned %q, %v", pretty, ok)
	}
	if _, ok := Pretty([]byte(`<html></html>`)); ok {
		t.Error("HTML was reported as JSON")
	}
	if _, ok := Pretty([]byte(`{not json}`)); ok {
		t.Error("invalid JSON was reported as JSON")
	}
	if _, ok := Pretty(nil); ok {
		t.Error("an empty body was reported as JSON")
	}

	for in, want := range map[int64]string{
		512:        "512 B",
		1024:       "1.0 KB",
		1048576:    "1.0 MB",
		1073741824: "1.0 GB",
	} {
		if got := FormatBytes(in); got != want {
			t.Errorf("FormatBytes(%d) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[time.Duration]string{
		0:                       "—",
		12 * time.Millisecond:   "12 ms",
		1500 * time.Millisecond: "1.50 s",
	} {
		if got := FormatDuration(in); got != want {
			t.Errorf("FormatDuration(%v) = %q, want %q", in, got, want)
		}
	}
}

func TestFormBodyIsURLEncodedInTheTablesOrder(t *testing.T) {
	var body, contentType string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		body, contentType = string(raw), r.Header.Get("Content-Type")
	}))
	defer upstream.Close()

	res := New().Do(t.Context(), Spec{
		Method: "POST", URL: upstream.URL, BodyKind: model.BodyForm,
		Form: []model.KV{
			{Enabled: true, Key: "name", Value: "two words"},
			{Enabled: true, Key: "a&b", Value: "x=y"},
			{Enabled: false, Key: "draft", Value: "true"},
			{Enabled: true, Key: " ", Value: "ignored"},
		},
	})
	if res.Err != "" {
		t.Fatalf("Do: %s", res.Err)
	}
	if want := "name=two+words&a%26b=x%3Dy"; body != want {
		t.Errorf("form body %q, want %q", body, want)
	}
	if contentType != "application/x-www-form-urlencoded" {
		t.Errorf("content type %q", contentType)
	}
}

func TestWrittenUserAgentAndHostAreSent(t *testing.T) {
	var agent, host string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		agent, host = r.Header.Get("User-Agent"), r.Host
	}))
	defer upstream.Close()

	res := New().Do(t.Context(), Spec{URL: upstream.URL, Headers: []model.KV{
		{Enabled: true, Key: "User-Agent", Value: "probe/2"},
		{Enabled: true, Key: "Host", Value: "api.internal"},
	}})
	if res.Err != "" {
		t.Fatalf("Do: %s", res.Err)
	}
	if agent != "probe/2" {
		t.Errorf("user agent %q, want the one written", agent)
	}
	if host != "api.internal" {
		t.Errorf("host %q, want the one written", host)
	}

	if res := New().Do(t.Context(), Spec{URL: upstream.URL}); res.Err != "" {
		t.Fatalf("Do: %s", res.Err)
	}
	if agent != UserAgent {
		t.Errorf("default user agent %q, want %q", agent, UserAgent)
	}
}

func TestAuthTabReplacesAWrittenAuthorization(t *testing.T) {
	var values []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		values = r.Header.Values("Authorization")
	}))
	defer upstream.Close()

	res := New().Do(t.Context(), Spec{
		URL:      upstream.URL,
		Headers:  []model.KV{{Enabled: true, Key: "authorization", Value: "Bearer stale"}},
		AuthType: model.AuthBearer, Token: "fresh",
	})
	if res.Err != "" {
		t.Fatalf("Do: %s", res.Err)
	}
	if len(values) != 1 || values[0] != "Bearer fresh" {
		t.Errorf("authorization %q, want only the Auth tab's", values)
	}
}

func TestCanceledRequestSaysSo(t *testing.T) {
	arrived := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(arrived)
		<-r.Context().Done()
	}))
	defer upstream.Close()

	ctx, cancel := context.WithCancel(t.Context())
	go func() {
		<-arrived
		cancel()
	}()
	res := New().Do(ctx, Spec{URL: upstream.URL})
	if res.Err != "The request was canceled." {
		t.Errorf("error %q, want the cancel to be named", res.Err)
	}
}

func TestCurlWritesTheRequestTheComposerSends(t *testing.T) {
	got, err := Curl(Spec{
		Method: "post", URL: "api.example.com/v1/things",
		Params:   []model.KV{{Enabled: true, Key: "q", Value: "a b"}},
		Headers:  []model.KV{{Enabled: true, Key: "X-Trace", Value: "it's on"}, {Enabled: false, Key: "X-Off", Value: "1"}},
		BodyKind: model.BodyJSON, Body: `{"hello":"world"}`,
		AuthType: model.AuthBearer, Token: "secret",
	})
	if err != nil {
		t.Fatalf("Curl: %v", err)
	}
	want := `curl --location 'https://api.example.com/v1/things?q=a+b' \
  -H 'X-Trace: it'\''s on' \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer secret' \
  --data-raw '{"hello":"world"}'`
	if got != want {
		t.Errorf("Curl =\n%s\nwant\n%s", got, want)
	}
	stopped, _ := Curl(Spec{URL: "https://x.test/", NoRedirects: true, Insecure: true})
	if stopped != "curl --insecure 'https://x.test/'" {
		t.Errorf("a send that stops at redirects and skips checks is written %q", stopped)
	}
	if _, err := Curl(Spec{URL: "ftp://example.com"}); err == nil {
		t.Error("Curl wrote a command for a URL the composer cannot send")
	}
}

func TestCurlCommandMethodsAndQuoting(t *testing.T) {
	for _, tc := range []struct {
		method string
		body   []byte
		want   string
	}{
		{"GET", nil, `curl 'http://h/'`},
		{"HEAD", nil, `curl --head 'http://h/'`},
		{"DELETE", nil, `curl -X DELETE 'http://h/'`},
		{"POST", []byte("a=1"), `curl 'http://h/' \` + "\n  --data-raw 'a=1'"},
		{"GET", []byte("x"), `curl -X GET 'http://h/' \` + "\n  --data-raw 'x'"},
		{"PUT", []byte{0x00, 0xff, '\''}, `curl -X PUT 'http://h/' \` + "\n  --data-raw $'\\x00\\xff\\''"},
	} {
		if got := CurlCommand(tc.method, "http://h/", nil, tc.body); got != tc.want {
			t.Errorf("CurlCommand(%s, %q) =\n%s\nwant\n%s", tc.method, tc.body, got, tc.want)
		}
	}
}

func TestPhasesFollowOneAnother(t *testing.T) {
	res := Result{
		Duration: 284 * time.Millisecond, TTFB: 182 * time.Millisecond,
		DNS: 12 * time.Millisecond, Connect: 28 * time.Millisecond, TLS: 46 * time.Millisecond,
	}
	if got, want := res.Waiting(), 96*time.Millisecond; got != want {
		t.Errorf("Waiting = %v, want %v (the first byte less DNS, connect and TLS)", got, want)
	}
	if got, want := res.Download(), 102*time.Millisecond; got != want {
		t.Errorf("Download = %v, want %v", got, want)
	}
	if got := (Result{Duration: time.Second}).Download(); got != 0 {
		t.Errorf("a result with no first byte downloads for %v", got)
	}
}
