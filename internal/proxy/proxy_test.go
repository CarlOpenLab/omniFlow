package proxy

import (
	"compress/gzip"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func newTestProxy(t *testing.T) (*Proxy, string) {
	t.Helper()
	ca, err := LoadOrCreateCA(t.TempDir())
	if err != nil {
		t.Fatalf("create CA: %v", err)
	}
	p := New(ca)
	addr, err := p.Start("0")
	if err != nil {
		t.Fatalf("start proxy: %v", err)
	}
	t.Cleanup(p.Stop)
	return p, addr
}

func waitFlow(t *testing.T, flows <-chan Flow) Flow {
	t.Helper()
	select {
	case f := <-flows:
		return f
	case <-time.After(5 * time.Second):
		t.Fatal("the proxy reported no flow")
		return Flow{}
	}
}

func TestCaptureHTTP(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if r.Method != http.MethodPost || string(body) != `{"hello":"world"}` {
			t.Errorf("upstream saw %s %q", r.Method, body)
		}
		if got := r.Header.Get("X-Omni"); got != "yes" {
			t.Errorf("upstream saw X-Omni %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	defer upstream.Close()

	p, addr := newTestProxy(t)
	flows := make(chan Flow, 2)
	p.SetOnFlow(func(f Flow) { flows <- f })

	proxyURL, _ := url.Parse("http://" + addr)
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}
	req, _ := http.NewRequest(http.MethodPost, upstream.URL+"/anything?x=1", strings.NewReader(`{"hello":"world"}`))
	req.Header.Set("X-Omni", "yes")
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request through proxy: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated || string(body) != `{"ok":true}` {
		t.Fatalf("client got %d %q", resp.StatusCode, body)
	}

	f := waitFlow(t, flows)
	if f.Method != "POST" || f.Status != 201 {
		t.Errorf("flow is %s %d", f.Method, f.Status)
	}
	if f.TLS {
		t.Errorf("plain HTTP flow is marked TLS")
	}
	if !strings.Contains(f.URL, "/anything?x=1") {
		t.Errorf("flow URL is %q", f.URL)
	}
	if string(f.ReqBody) != `{"hello":"world"}` {
		t.Errorf("captured request body %q", f.ReqBody)
	}
	if string(f.RespBody) != `{"ok":true}` {
		t.Errorf("captured response body %q", f.RespBody)
	}
	if f.ContentType != "application/json" {
		t.Errorf("content type %q", f.ContentType)
	}
}

func TestCaptureHTTPSWithOwnCA(t *testing.T) {
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"secure":true}`)
	}))
	defer upstream.Close()

	ca, err := LoadOrCreateCA(t.TempDir())
	if err != nil {
		t.Fatalf("create CA: %v", err)
	}
	p := New(ca)
	// The test's upstream certificate is its own; trust it just here.
	p.transport.TLSClientConfig.InsecureSkipVerify = true
	addr, err := p.Start("0")
	if err != nil {
		t.Fatalf("start proxy: %v", err)
	}
	defer p.Stop()
	flows := make(chan Flow, 2)
	p.SetOnFlow(func(f Flow) { flows <- f })

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(ca.CertPEM()) {
		t.Fatal("cannot trust the proxy CA")
	}
	proxyURL, _ := url.Parse("http://" + addr)
	client := &http.Client{Transport: &http.Transport{
		Proxy:           http.ProxyURL(proxyURL),
		TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12},
	}}
	resp, err := client.Get(upstream.URL + "/secure?yes=1")
	if err != nil {
		t.Fatalf("HTTPS request through proxy: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || string(body) != `{"secure":true}` {
		t.Fatalf("client got %d %q", resp.StatusCode, body)
	}

	f := waitFlow(t, flows)
	if !f.TLS || f.Scheme != "https" {
		t.Errorf("flow is not TLS: %+v", f)
	}
	if f.Status != 200 {
		t.Errorf("flow status %d", f.Status)
	}
	if !strings.Contains(f.Path, "/secure?yes=1") {
		t.Errorf("flow path %q", f.Path)
	}
	if string(f.RespBody) != `{"secure":true}` {
		t.Errorf("captured HTTPS body %q", f.RespBody)
	}
}

func TestCaptureDecodesGzipForViewer(t *testing.T) {
	var gz []byte
	{
		var b strings.Builder
		zw := gzip.NewWriter(&b)
		_, _ = io.WriteString(zw, `{"compressed":true}`)
		_ = zw.Close()
		gz = []byte(b.String())
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Content-Length", strconv.Itoa(len(gz)))
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			t.Errorf("proxy asked for %q", r.Header.Get("Accept-Encoding"))
		}
		_, _ = w.Write(gz)
	}))
	defer upstream.Close()

	p, addr := newTestProxy(t)
	flows := make(chan Flow, 2)
	p.SetOnFlow(func(f Flow) { flows <- f })
	proxyURL, _ := url.Parse("http://" + addr)
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}
	// The transport asks for gzip and transparently decodes it; the proxy
	// must have decoded its own copy for the inspector too.
	resp, err := client.Get(upstream.URL + "/compressed")
	if err != nil {
		t.Fatalf("request through proxy: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(body) != `{"compressed":true}` {
		t.Fatalf("client body %q", body)
	}
	f := waitFlow(t, flows)
	if string(f.RespBody) != `{"compressed":true}` {
		t.Errorf("inspector body %q, want decoded JSON", f.RespBody)
	}
}

func TestCaptureChunkedResponse(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, _ := w.(http.Flusher)
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "first-")
		if flusher != nil {
			flusher.Flush()
		}
		time.Sleep(20 * time.Millisecond)
		_, _ = io.WriteString(w, "second")
	}))
	defer upstream.Close()

	p, addr := newTestProxy(t)
	flows := make(chan Flow, 2)
	p.SetOnFlow(func(f Flow) { flows <- f })
	proxyURL, _ := url.Parse("http://" + addr)
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}
	resp, err := client.Get(upstream.URL + "/chunked")
	if err != nil {
		t.Fatalf("request through proxy: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(body) != "first-second" {
		t.Fatalf("client body %q", body)
	}
	f := waitFlow(t, flows)
	if string(f.RespBody) != "first-second" {
		t.Errorf("inspector body %q", f.RespBody)
	}
}

func TestARefusedCertificateBecomesAFlow(t *testing.T) {
	p, addr := newTestProxy(t)
	flows := make(chan Flow, 2)
	p.SetOnFlow(func(f Flow) { flows <- f })

	// A client that has not installed the Root CA: its handshake fails, and
	// the proxy says so rather than dropping the request without a trace.
	proxyURL, _ := url.Parse("http://" + addr)
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}
	if _, err := client.Get("https://untrusting.example/"); err == nil {
		t.Fatal("a client without the Root CA trusted the proxy's certificate")
	}
	f := waitFlow(t, flows)
	if f.Method != http.MethodConnect || f.Host != "untrusting.example" || f.Status != 0 || !f.TLS {
		t.Errorf("flow is %s %s %d tls=%v", f.Method, f.Host, f.Status, f.TLS)
	}
	if !strings.Contains(f.Err, "Root CA") {
		t.Errorf("the flow does not say what to do: %q", f.Err)
	}
}

func TestAMockAnswersFromItsFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "users.json")
	if err := os.WriteFile(file, []byte(`{"mocked":true}`), 0o644); err != nil {
		t.Fatalf("write mock: %v", err)
	}

	p, addr := newTestProxy(t)
	p.SetMocks([]Mock{{
		Method: "GET", Host: "mock.example", Path: "/users",
		Status: http.StatusCreated, ContentType: "application/json", File: file,
	}})
	flows := make(chan Flow, 2)
	p.SetOnFlow(func(f Flow) { flows <- f })

	proxyURL, _ := url.Parse("http://" + addr)
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}
	// The host does not resolve: only the mock can answer it. The query is
	// not part of the rule's match.
	resp, err := client.Get("http://mock.example/users?page=2")
	if err != nil {
		t.Fatalf("request through proxy: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated || string(body) != `{"mocked":true}` {
		t.Fatalf("client got %d %q", resp.StatusCode, body)
	}
	if got := resp.Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("content type %q", got)
	}

	f := waitFlow(t, flows)
	if f.Status != http.StatusCreated || string(f.RespBody) != `{"mocked":true}` {
		t.Errorf("flow is %d %q", f.Status, f.RespBody)
	}
	if !f.Mocked {
		t.Error("the flow does not say a mock answered it")
	}
	if f.Host != "mock.example" || f.Path != "/users?page=2" {
		t.Errorf("flow is %s%s", f.Host, f.Path)
	}
}
