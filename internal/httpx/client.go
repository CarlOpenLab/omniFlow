// Package httpx sends the composer's requests and reports everything worth
// showing about the answer: status, headers, body, and where the time went.
package httpx

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/CarlOpenLab/omniflow/internal/model"
)

// MaxBody is how much of a response body is kept for the window. A download
// larger than this is shown cut, with its size still reported in full.
const MaxBody = 8 << 20

// Header is one header row, in the order the response listed it.
type Header struct{ Key, Value string }

// Result is a completed attempt: the response, or why there is none.
type Result struct {
	Status     int
	StatusText string
	Proto      string
	Headers    []Header
	Body       []byte
	Truncated  bool

	Duration time.Duration
	TTFB     time.Duration
	DNS      time.Duration
	Connect  time.Duration
	TLS      time.Duration
	Size     int64

	// Redirects is how many redirects were followed to the answer, and URL
	// where they led, "" when none were.
	Redirects int
	URL       string

	Err string
}

// Waiting is the time between the connection being ready and the first
// byte of the answer: the request going up and the server's own work. The
// phases before it run one after another, so each is taken off in turn.
func (r Result) Waiting() time.Duration {
	if r.TTFB <= 0 {
		return 0
	}
	return max(0, r.TTFB-r.DNS-r.Connect-r.TLS)
}

// Download is the time from the first byte of the answer to its last.
func (r Result) Download() time.Duration {
	if r.TTFB <= 0 {
		return 0
	}
	return max(0, r.Duration-r.TTFB)
}

// Spec is what the composer sends: the same fields its tabs edit.
type Spec struct {
	Method   string
	URL      string
	Params   []model.KV
	Headers  []model.KV
	BodyKind string
	Body     string
	Form     []model.KV
	AuthType string
	Token    string
	User     string
	Pass     string
	Timeout  time.Duration
	// NoRedirects answers with a redirect itself rather than following it.
	NoRedirects bool
	// Insecure accepts any certificate, as a local server's self-signed one.
	Insecure bool
}

// maxRedirects is how many redirects a request follows before giving up.
const maxRedirects = 10

// UserAgent is sent when a request names no User-Agent of its own.
const UserAgent = "OmniFlow/0.1"

// prepared is a spec resolved into what goes on the wire. Do sends it and
// Curl writes it out, so the command a person copies is the request the
// composer sends.
type prepared struct {
	method  string
	url     *url.URL
	headers []Header // in the order written, the added ones last
	body    []byte   // nil when the method or the body kind sends none
}

// prepare resolves the URL, the body and the headers the tabs imply: a
// Content-Type for the body unless a header names one, and the Auth tab's
// Authorization in place of any written by hand.
func prepare(spec Spec) (prepared, error) {
	u, err := BuildURL(spec.URL, spec.Params)
	if err != nil {
		return prepared{}, err
	}
	p := prepared{method: strings.ToUpper(strings.TrimSpace(spec.Method)), url: u}
	if p.method == "" {
		p.method = http.MethodGet
	}
	contentType := ""
	if p.method != http.MethodGet && p.method != http.MethodHead {
		p.body, contentType = encodeBody(spec)
	}
	auth := authorization(spec)
	hasContentType := false
	for _, h := range spec.Headers {
		if !h.Enabled || strings.TrimSpace(h.Key) == "" {
			continue
		}
		if auth != "" && strings.EqualFold(h.Key, "Authorization") {
			continue
		}
		if strings.EqualFold(h.Key, "Content-Type") {
			hasContentType = true
		}
		p.headers = append(p.headers, Header{Key: h.Key, Value: h.Value})
	}
	if p.body != nil && !hasContentType {
		p.headers = append(p.headers, Header{Key: "Content-Type", Value: contentType})
	}
	if auth != "" {
		p.headers = append(p.headers, Header{Key: "Authorization", Value: auth})
	}
	return p, nil
}

// encodeBody is what the spec's body kind sends, and the Content-Type that
// goes with it unless a header names another.
func encodeBody(spec Spec) ([]byte, string) {
	switch spec.BodyKind {
	case model.BodyJSON:
		if spec.Body != "" {
			return []byte(spec.Body), "application/json"
		}
	case model.BodyText:
		if spec.Body != "" {
			return []byte(spec.Body), "text/plain; charset=utf-8"
		}
	case model.BodyForm:
		const formType = "application/x-www-form-urlencoded"
		// Written out in the table's order: url.Values would sort the keys.
		var fields []string
		for _, kv := range spec.Form {
			if kv.Enabled && strings.TrimSpace(kv.Key) != "" {
				fields = append(fields, url.QueryEscape(kv.Key)+"="+url.QueryEscape(kv.Value))
			}
		}
		if len(fields) > 0 {
			return []byte(strings.Join(fields, "&")), formType
		}
		// A form still written as text, from before the table.
		if spec.Body != "" {
			return []byte(spec.Body), formType
		}
	}
	return nil, ""
}

// authorization is the header value the Auth tab adds, "" for none.
func authorization(spec Spec) string {
	switch spec.AuthType {
	case model.AuthBearer:
		if spec.Token != "" {
			return "Bearer " + spec.Token
		}
	case model.AuthBasic:
		return "Basic " + base64.StdEncoding.EncodeToString([]byte(spec.User+":"+spec.Pass))
	}
	return ""
}

// Client shares its transports between sends, so repeated sends reuse
// connections: one that verifies certificates, and one that does not for the
// requests that ask it not to, so that no connection made without checks is
// reused by a request that wants them.
type Client struct {
	secure, insecure *http.Transport
}

// New returns a client with TLS verification on and no system proxy: the
// composer talks to the URL on screen, not through OmniFlow's own proxy.
func New() *Client {
	tr := &http.Transport{
		Proxy:                 nil,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          64,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
	}
	insecure := tr.Clone()
	insecure.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: true}
	return &Client{secure: tr, insecure: insecure}
}

// BuildURL resolves the URL the composer holds: a missing scheme becomes
// https, and enabled query rows are added to whatever query it already has.
func BuildURL(raw string, params []model.KV) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("Enter a URL to send")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("That URL cannot be parsed: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("%s is not an HTTP or HTTPS URL", u.Scheme)
	}
	if u.Host == "" {
		return nil, errors.New("That URL has no host")
	}
	// The query stays as it was written, in its order and its encoding, as
	// a signed URL needs it; the rows of the table follow it, in theirs.
	fields := []string{}
	if q := cleanQuery(u.RawQuery); q != "" {
		fields = append(fields, q)
	}
	for _, p := range params {
		if !p.Enabled || strings.TrimSpace(p.Key) == "" {
			continue
		}
		fields = append(fields, url.QueryEscape(p.Key)+"="+url.QueryEscape(p.Value))
	}
	u.RawQuery = strings.Join(fields, "&")
	u.ForceQuery = false
	return u, nil
}

// cleanQuery escapes what a query cannot carry as it is, as a space typed
// into the URL, and leaves the rest of it, escapes included, alone.
func cleanQuery(raw string) string {
	var b strings.Builder
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		if c <= ' ' || c >= 0x7f || strings.IndexByte(`"<>\^`+"`"+`{|}`, c) >= 0 {
			fmt.Fprintf(&b, "%%%02X", c)
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

// Do sends the spec and waits for the whole answer, up to the timeout. A
// response with a 4xx or 5xx status is a result, not an error.
func (c *Client) Do(ctx context.Context, spec Spec) Result {
	start := time.Now()
	p, err := prepare(spec)
	if err != nil {
		return Result{Err: err.Error(), Duration: time.Since(start)}
	}
	timeout := spec.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var body io.Reader
	if p.body != nil {
		body = bytes.NewReader(p.body)
	}
	req, err := http.NewRequestWithContext(ctx, p.method, p.url.String(), body)
	if err != nil {
		return Result{Err: err.Error(), Duration: time.Since(start)}
	}
	for _, h := range p.headers {
		// Go sends the request's Host field, never a Host header.
		if strings.EqualFold(h.Key, "Host") {
			req.Host = h.Value
			continue
		}
		req.Header.Add(h.Key, h.Value)
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", UserAgent)
	}

	tr := c.secure
	if spec.Insecure {
		tr = c.insecure
	}
	redirects := 0
	hc := &http.Client{Transport: tr, CheckRedirect: func(_ *http.Request, via []*http.Request) error {
		if spec.NoRedirects {
			return http.ErrUseLastResponse
		}
		if len(via) >= maxRedirects {
			return fmt.Errorf("stopped after %d redirects", maxRedirects)
		}
		redirects++
		return nil
	}}

	var tm timings
	trace := &httptrace.ClientTrace{
		DNSStart:             func(httptrace.DNSStartInfo) { tm.mark(&tm.dnsStart) },
		DNSDone:              func(httptrace.DNSDoneInfo) { tm.finish(&tm.dnsStart, &tm.dns) },
		ConnectStart:         func(string, string) { tm.mark(&tm.connStart) },
		ConnectDone:          func(string, string, error) { tm.finish(&tm.connStart, &tm.conn) },
		TLSHandshakeStart:    tm.markPtr(&tm.tlsStart),
		TLSHandshakeDone:     func(tls.ConnectionState, error) { tm.finish(&tm.tlsStart, &tm.tls) },
		GotFirstResponseByte: func() { tm.set(&tm.ttfb, time.Since(start)) },
	}
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))

	resp, err := hc.Do(req)
	if err != nil {
		return Result{Err: friendlyError(err), Duration: time.Since(start), TTFB: tm.get(&tm.ttfb)}
	}
	defer resp.Body.Close()

	raw, readErr := io.ReadAll(io.LimitReader(resp.Body, MaxBody+1))
	res := Result{
		Status:     resp.StatusCode,
		StatusText: strings.TrimSpace(strings.TrimPrefix(resp.Status, fmt.Sprint(resp.StatusCode))),
		Proto:      resp.Proto,
		Headers:    sortedHeaders(resp.Header),
		Duration:   time.Since(start),
		TTFB:       tm.get(&tm.ttfb),
		DNS:        tm.get(&tm.dns),
		Connect:    tm.get(&tm.conn),
		TLS:        tm.get(&tm.tls),
		Size:       resp.ContentLength,
		Redirects:  redirects,
	}
	if redirects > 0 {
		res.URL = resp.Request.URL.String()
	}
	if len(raw) > MaxBody {
		raw = raw[:MaxBody]
		res.Truncated = true
	}
	res.Body = raw
	if res.Size < 0 {
		res.Size = int64(len(raw))
	}
	if readErr != nil {
		res.Err = friendlyError(readErr)
	}
	return res
}

// sortedHeaders flattens a header map into rows, one per value, in name
// order, as the inspector lists them.
func sortedHeaders(h http.Header) []Header {
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]Header, 0, len(keys))
	for _, k := range keys {
		for _, v := range h[k] {
			out = append(out, Header{Key: k, Value: v})
		}
	}
	return out
}

// friendlyError says what went wrong in terms a person can act on.
func friendlyError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	switch {
	case errors.Is(err, context.Canceled):
		return "The request was canceled."
	case errors.Is(err, context.DeadlineExceeded) || strings.Contains(msg, "context deadline exceeded"):
		return "The request timed out. The server took longer than its timeout to answer."
	case strings.Contains(msg, "no such host"):
		return "That host could not be found. Check the spelling, or your network."
	case strings.Contains(msg, "connection refused"):
		return "The connection was refused. Is the server running on that port?"
	case strings.Contains(msg, "certificate"):
		return "TLS verification failed: " + msg
	}
	return msg
}

// Pretty returns the body re-indented as JSON when it is JSON, and whether
// it is.
func Pretty(body []byte) (string, bool) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return "", false
	}
	if trimmed[0] != '{' && trimmed[0] != '[' {
		return "", false
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, trimmed, "", "  "); err != nil {
		return "", false
	}
	return buf.String(), true
}

// FormatBytes writes a size as a person reads it: 512 B, 4.2 KB, 1.8 MB.
func FormatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGT"[exp])
}

// FormatDuration writes a duration in the unit that reads best: ms below a
// second, seconds above it.
func FormatDuration(d time.Duration) string {
	if d <= 0 {
		return "—"
	}
	if d < time.Second {
		return fmt.Sprintf("%.0f ms", float64(d)/float64(time.Millisecond))
	}
	return fmt.Sprintf("%.2f s", d.Seconds())
}

// timing holds the phases of a request that httptrace reports. The trace's
// callbacks may run on the transport's goroutine, so the fields are guarded.
type timings struct {
	mu                            sync.Mutex
	dnsStart, connStart, tlsStart time.Time
	dns, conn, tls, ttfb          time.Duration
}

func (t *timings) mark(at *time.Time) {
	t.mu.Lock()
	*at = time.Now()
	t.mu.Unlock()
}

func (t *timings) markPtr(at *time.Time) func() {
	return func() { t.mark(at) }
}

func (t *timings) finish(from *time.Time, to *time.Duration) {
	t.mu.Lock()
	if !from.IsZero() {
		*to = time.Since(*from)
	}
	t.mu.Unlock()
}

func (t *timings) set(to *time.Duration, v time.Duration) {
	t.mu.Lock()
	*to = v
	t.mu.Unlock()
}

func (t *timings) get(from *time.Duration) time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	return *from
}
