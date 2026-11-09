package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestMapRemoteSendsTheRequestToAnotherOrigin(t *testing.T) {
	var gotPath, gotQuery, gotHost string
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery, gotHost = r.URL.Path, r.URL.RawQuery, r.Host
		w.Header().Set("Content-Type", "text/plain")
		_, _ = io.WriteString(w, "from target")
	}))
	defer target.Close()

	p, addr := newTestProxy(t)
	p.SetRules([]Rule{{
		Kind: RuleMapRemote, Host: "origin.example", PathPrefix: "/api", Target: target.URL + "/base",
	}})
	flows := make(chan Flow, 2)
	p.SetOnFlow(func(f Flow) { flows <- f })

	proxyURL, _ := url.Parse("http://" + addr)
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}
	resp, err := client.Get("http://origin.example/api/thing?x=1")
	if err != nil {
		t.Fatalf("request through proxy: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if string(body) != "from target" {
		t.Fatalf("client body %q", body)
	}
	if gotPath != "/base/api/thing" || gotQuery != "x=1" {
		t.Errorf("target saw %s?%s", gotPath, gotQuery)
	}
	if !strings.HasPrefix(gotHost, "127.0.0.1:") {
		t.Errorf("target saw Host %q", gotHost)
	}

	f := waitFlow(t, flows)
	if !f.Mapped {
		t.Error("the flow does not say it was mapped")
	}
	if want := target.URL + "/base/api/thing?x=1"; f.MappedTo != want {
		t.Errorf("MappedTo = %q, want %q", f.MappedTo, want)
	}
	if f.URL != "http://origin.example/api/thing?x=1" {
		t.Errorf("the flow lost the URL the client asked for: %q", f.URL)
	}
}

func TestHeaderRulesRewriteRequestAndResponse(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Debug"); got != "yes" {
			t.Errorf("target saw X-Debug %q", got)
		}
		if got := r.Header.Get("X-Remove"); got != "" {
			t.Errorf("target still saw X-Remove %q", got)
		}
		w.Header().Set("X-Origin", "1")
		w.Header().Set("X-Keep", "kept")
		_, _ = io.WriteString(w, "ok")
	}))
	defer target.Close()

	p, addr := newTestProxy(t)
	p.SetRules([]Rule{
		{Kind: RuleMapRemote, Host: "rewrite.example", Target: target.URL},
		{Kind: RuleRequestHeader, Host: "rewrite.example", Header: "X-Debug", Target: "yes"},
		{Kind: RuleRequestHeader, Host: "rewrite.example", Header: "X-Remove", Remove: true},
		{Kind: RuleResponseHeader, Host: "rewrite.example", Header: "X-Debug", Target: "done"},
		{Kind: RuleResponseHeader, Host: "rewrite.example", Header: "X-Debug", Target: "old"},
		{Kind: RuleResponseHeader, Host: "rewrite.example", Header: "X-Origin", Remove: true},
	})
	proxyURL, _ := url.Parse("http://" + addr)
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}

	req, _ := http.NewRequest(http.MethodGet, "http://rewrite.example/", nil)
	req.Header.Set("X-Remove", "1")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request through proxy: %v", err)
	}
	resp.Body.Close()
	if got := resp.Header.Get("X-Debug"); got != "done" {
		t.Errorf("client saw X-Debug %q", got)
	}
	if got := resp.Header.Get("X-Origin"); got != "" {
		t.Errorf("client still saw X-Origin %q", got)
	}
	if got := resp.Header.Get("X-Keep"); got != "kept" {
		t.Errorf("the untouched header was lost: %q", got)
	}
}

func TestThrottleAddsLatencyAndSlowsTheBody(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, strings.Repeat("x", 1200))
	}))
	defer target.Close()

	p, addr := newTestProxy(t)
	p.SetThrottle(Throttle{Latency: 60 * time.Millisecond, BytesPerSec: 4000})
	proxyURL, _ := url.Parse("http://" + addr)
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}

	start := time.Now()
	resp, err := client.Get(target.URL + "/")
	if err != nil {
		t.Fatalf("request through proxy: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	elapsed := time.Since(start)
	if len(body) != 1200 {
		t.Fatalf("client body is %d bytes", len(body))
	}
	// 60 ms of latency plus 1200 bytes at 4000 B/s is at least 360 ms.
	if elapsed < 300*time.Millisecond {
		t.Errorf("the throttled request took %v, want at least 300ms", elapsed)
	}
}
