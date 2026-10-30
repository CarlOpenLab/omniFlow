// Package proxy is OmniFlow's capture proxy: a local HTTP proxy that logs
// every request passing through it, and decrypts HTTPS with a certificate
// authority of its own when the person has installed it.
package proxy

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// MaxBody is how much of a body is kept for the inspector. The full body
// still reaches the client; only the copy on screen is cut.
const MaxBody = 4 << 20

// Proxy is a capturing HTTP proxy listening on loopback.
type Proxy struct {
	ca *CA

	mu        sync.Mutex
	srv       *http.Server
	ln        net.Listener
	addr      string
	mitm      bool
	onFlow    func(Flow)
	conns     map[net.Conn]struct{}
	transport *http.Transport
	nextID    atomic.Uint64
}

// New returns a proxy that signs with ca. Call Start before using it, and
// SetOnFlow to receive what it captures.
func New(ca *CA) *Proxy {
	return &Proxy{
		ca:    ca,
		mitm:  true,
		conns: map[net.Conn]struct{}{},
		transport: &http.Transport{
			Proxy:               nil,
			ForceAttemptHTTP2:   true,
			MaxIdleConns:        128,
			MaxIdleConnsPerHost: 8,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
			TLSClientConfig:     &tls.Config{MinVersion: tls.VersionTLS12},
		},
	}
}

// SetOnFlow sets the callback run once for each exchange, from the proxy's
// goroutine. The window hands these to Window.Update.
func (p *Proxy) SetOnFlow(fn func(Flow)) {
	p.mu.Lock()
	p.onFlow = fn
	p.mu.Unlock()
}

// SetMITM turns HTTPS decryption on or off. Off, CONNECT is a plain tunnel:
// it still works, but the inspector shows nothing inside it.
func (p *Proxy) SetMITM(on bool) {
	p.mu.Lock()
	p.mitm = on
	p.mu.Unlock()
}

// MITM reports whether HTTPS is decrypted.
func (p *Proxy) MITM() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.mitm
}

// Start listens on 127.0.0.1:port. It is safe to call when running.
func (p *Proxy) Start(port string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.srv != nil {
		return p.addr, nil
	}
	if strings.TrimSpace(port) == "" {
		port = "9090"
	}
	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", port))
	if err != nil {
		return "", fmt.Errorf("the proxy could not listen on 127.0.0.1:%s: %w", port, err)
	}
	srv := &http.Server{
		Handler:           p,
		ReadHeaderTimeout: 15 * time.Second,
		// Idle connections from a browser stay open; the proxy is a daemon
		// for as long as the window says so.
		IdleTimeout: 5 * time.Minute,
	}
	p.srv = srv
	p.ln = ln
	p.addr = ln.Addr().String()
	go func() {
		_ = srv.Serve(ln)
	}()
	return p.addr, nil
}

// Stop closes the listener and every connection that passed through it,
// including the hijacked ones the HTTP server no longer tracks.
func (p *Proxy) Stop() {
	p.mu.Lock()
	srv := p.srv
	conns := make([]net.Conn, 0, len(p.conns))
	for c := range p.conns {
		conns = append(conns, c)
	}
	p.conns = map[net.Conn]struct{}{}
	p.srv = nil
	p.ln = nil
	p.addr = ""
	p.mu.Unlock()

	for _, c := range conns {
		_ = c.Close()
	}
	if srv != nil {
		_ = srv.Close()
	}
}

// Addr is the address it listens on, "" when stopped.
func (p *Proxy) Addr() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.addr
}

// Running reports whether it is listening.
func (p *Proxy) Running() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.srv != nil
}

// ServeHTTP is the proxy's handler: absolute-form requests are forwarded,
// CONNECT is either decrypted or tunneled.
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		p.handleConnect(w, r)
		return
	}
	p.handleHTTP(w, r)
}

func (p *Proxy) handleHTTP(w http.ResponseWriter, r *http.Request) {
	scheme := r.URL.Scheme
	if scheme == "" {
		scheme = "http"
	}
	resp, flow, body, err := p.forward(r, scheme, r.URL.Host, false)
	if err != nil {
		http.Error(w, "OmniFlow proxy: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	removeHopHeaders(resp.Header)
	copyHeader(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	if _, copyErr := io.Copy(w, body); copyErr != nil {
		flow.Err = copyErr.Error()
	}
	p.finishFlow(flow, body, resp)
}

func (p *Proxy) handleConnect(w http.ResponseWriter, r *http.Request) {
	target := hostPort(r.Host, "443")
	if !p.MITM() {
		p.tunnel(w, r, target)
		return
	}
	p.mitmConnect(w, r, target)
}

// tunnel is CONNECT without decryption: the bytes are copied both ways and
// none of them is seen.
func (p *Proxy) tunnel(w http.ResponseWriter, r *http.Request, target string) {
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "OmniFlow proxy: this connection cannot be hijacked", http.StatusInternalServerError)
		return
	}
	client, _, err := hj.Hijack()
	if err != nil {
		return
	}
	untrack := p.track(client)
	defer untrack()
	defer client.Close()

	upstream, err := net.DialTimeout("tcp", target, 12*time.Second)
	if err != nil {
		_, _ = client.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
		return
	}
	defer upstream.Close()
	_, _ = client.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))

	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(upstream, client); done <- struct{}{} }()
	go func() { _, _ = io.Copy(client, upstream); done <- struct{}{} }()
	<-done
}

// mitmConnect answers CONNECT, then speaks TLS with a certificate for the
// host and reads the requests inside it as ordinary HTTP/1.1.
func (p *Proxy) mitmConnect(w http.ResponseWriter, r *http.Request, target string) {
	start := time.Now()
	host, _, err := net.SplitHostPort(target)
	if err != nil {
		host = target
	}
	cert, err := p.ca.Leaf(host)
	if err != nil {
		http.Error(w, "OmniFlow proxy: cannot sign a certificate for "+host, http.StatusBadGateway)
		return
	}
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "OmniFlow proxy: this connection cannot be hijacked", http.StatusInternalServerError)
		return
	}
	raw, rw, err := hj.Hijack()
	if err != nil {
		return
	}
	untrack := p.track(raw)
	defer untrack()
	defer raw.Close()
	if _, err := rw.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		return
	}
	if err := rw.Flush(); err != nil {
		return
	}
	var conn net.Conn = raw
	if n := rw.Reader.Buffered(); n > 0 {
		conn = &bufferedConn{Conn: raw, r: io.MultiReader(io.LimitReader(rw.Reader, int64(n)), raw)}
	}
	tlsConn := tls.Server(conn, &tls.Config{
		Certificates: []tls.Certificate{*cert},
		MinVersion:   tls.VersionTLS12,
		// HTTP/2 is deliberately not offered: the inspector reads HTTP/1.1,
		// and a browser that negotiates h2 would fill the list with nothing.
		NextProtos: []string{"http/1.1"},
	})
	_ = tlsConn.SetDeadline(time.Now().Add(15 * time.Second))
	if err := tlsConn.Handshake(); err != nil {
		if certRejected(err) {
			// Without this the request would vanish: say why, where the
			// person looks for it.
			p.emit(Flow{
				ID: p.nextID.Add(1), At: start, Method: http.MethodConnect, Scheme: "https",
				Host: host, URL: "https://" + target, TLS: true, Proto: r.Proto,
				Duration: time.Since(start), Err: rejectedMessage(host),
			})
		}
		return
	}
	_ = tlsConn.SetDeadline(time.Time{})
	br := bufio.NewReaderSize(tlsConn, 32*1024)

	for {
		_ = tlsConn.SetReadDeadline(time.Now().Add(3 * time.Minute))
		req, err := http.ReadRequest(br)
		if err != nil {
			return
		}
		_ = tlsConn.SetReadDeadline(time.Time{})
		req.URL.Scheme = "https"
		req.URL.Host = target
		if req.Host == "" {
			req.Host = target
		}
		resp, flow, body, ferr := p.forward(req, "https", target, true)
		if ferr != nil {
			writeProxyError(tlsConn, ferr)
			return
		}
		flow.TLS = true
		removeHopHeaders(resp.Header)
		// The client speaks HTTP/1.1 to us even when the upstream answered
		// over HTTP/2; a copied version line would leave it unable to tell
		// where a chunked body ends.
		resp.Proto, resp.ProtoMajor, resp.ProtoMinor = "HTTP/1.1", 1, 1
		if resp.Uncompressed || resp.ContentLength < 0 {
			resp.Header.Del("Content-Length")
			resp.ContentLength = -1
			// A response of unknown length needs chunked framing here:
			// without it the client cannot see where the body ends while
			// the connection stays open for the next request.
			resp.TransferEncoding = []string{"chunked"}
		}
		resp.Body = body
		if werr := resp.Write(tlsConn); werr != nil {
			_ = resp.Body.Close()
			return
		}
		_ = resp.Body.Close()
		p.finishFlow(flow, body, resp)

		if req.Close || resp.Close {
			return
		}
	}
}

// forward sends one request upstream and returns the response with a body
// reader that captures what passes through it for the inspector.
func (p *Proxy) forward(in *http.Request, scheme, authority string, isTLS bool) (*http.Response, *Flow, *captureReader, error) {
	start := time.Now()
	flow := &Flow{
		ID:     p.nextID.Add(1),
		At:     start,
		Method: in.Method,
		Scheme: scheme,
		TLS:    isTLS,
		Path:   in.URL.RequestURI(),
		Proto:  in.Proto,
	}
	if flow.Path == "" {
		flow.Path = "/"
	}
	host := in.URL.Hostname()
	if host == "" {
		h, _, _ := net.SplitHostPort(authority)
		if h == "" {
			h = authority
		}
		host = h
	}
	flow.Host = host
	flow.URL = scheme + "://" + host + flow.Path
	if !strings.HasPrefix(flow.Path, "/") {
		flow.Path = "/" + flow.Path
		flow.URL = scheme + "://" + host + flow.Path
	}
	flow.ReqHeaders = headerPairs(in.Header)
	flow.ReqSize = in.ContentLength

	out := in.Clone(context.Background())
	out.RequestURI = ""
	out.URL.Scheme = scheme
	out.URL.Host = authority
	out.Host = in.Host
	removeHopHeaders(out.Header)
	// Keep the client's own encodings, minus the ones the inspector cannot
	// decode (Brotli, zstd). With none left, Go asks upstream for gzip and
	// decompresses it for a client that did not ask to compress.
	if ae := usableEncoding(in.Header.Get("Accept-Encoding")); ae != "" {
		out.Header.Set("Accept-Encoding", ae)
	} else {
		out.Header.Del("Accept-Encoding")
	}

	reqBody, truncated, restored := captureBody(in.Body, MaxBody)
	out.Body = restored
	flow.ReqBody = reqBody
	flow.ReqTruncated = truncated
	if flow.ReqSize < 0 {
		flow.ReqSize = int64(len(reqBody))
	}

	resp, err := p.transport.RoundTrip(out)
	if err != nil {
		return nil, flow, nil, err
	}
	flow.Status = resp.StatusCode
	flow.StatusText = http.StatusText(resp.StatusCode)
	flow.RespHeaders = headerPairs(resp.Header)
	flow.ContentType = resp.Header.Get("Content-Type")
	flow.RespSize = resp.ContentLength
	flow.Duration = time.Since(start)

	body := &captureReader{r: resp.Body, limit: MaxBody}
	return resp, flow, body, nil
}

// finishFlow completes a flow's body sizes and hands it to the window. It
// is called after the body has passed through captureReader.
func (p *Proxy) finishFlow(flow *Flow, body *captureReader, resp *http.Response) {
	if body != nil {
		flow.RespBody = body.bytes()
		flow.RespTruncated = body.truncated
		if flow.RespSize < 0 {
			flow.RespSize = body.total
		}
	}
	// A compressed body is kept for forwarding untouched; the inspector
	// wants to read it, so decode a copy here.
	flow.RespBody = decodeForDisplay(flow.RespBody, resp.Header.Get("Content-Encoding"))
	flow.RespHeaders = stripHidden(flow.RespHeaders)
	p.emit(*flow)
}

// emit hands a finished flow to the window.
func (p *Proxy) emit(f Flow) {
	p.mu.Lock()
	fn := p.onFlow
	p.mu.Unlock()
	if fn != nil {
		fn(f)
	}
}

// certRejected reports whether a handshake failed because the client would
// not trust the certificate it was shown, as one without the Root CA does:
// it says so with an alert, which Go reports as a remote error.
func certRejected(err error) bool {
	msg := err.Error()
	for _, alert := range []string{"bad certificate", "unknown certificate", "unsupported certificate",
		"expired certificate", "revoked certificate"} {
		if strings.Contains(msg, "remote error: tls: "+alert) {
			return true
		}
	}
	return false
}

// rejectedMessage says what to do about a client that refused the
// certificate OmniFlow signed for host.
func rejectedMessage(host string) string {
	return "The client refused OmniFlow's certificate for " + host +
		". Install the Root CA on it and trust it, or turn off Decrypt HTTPS. " +
		"Apps that pin their certificates refuse it even then."
}

func (p *Proxy) track(c net.Conn) func() {
	p.mu.Lock()
	p.conns[c] = struct{}{}
	p.mu.Unlock()
	return func() {
		p.mu.Lock()
		delete(p.conns, c)
		p.mu.Unlock()
	}
}

func writeProxyError(w io.Writer, err error) {
	msg := "OmniFlow proxy: " + err.Error()
	_, _ = fmt.Fprintf(w, "HTTP/1.1 502 Bad Gateway\r\nContent-Type: text/plain; charset=utf-8\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s", len(msg), msg)
}

func hostPort(host, fallbackPort string) string {
	if _, _, err := net.SplitHostPort(host); err == nil {
		return host
	}
	return net.JoinHostPort(strings.Trim(host, "[]"), fallbackPort)
}

// bufferedConn reads what the HTTP server had already buffered before
// handing the connection over, then reads the connection itself.
type bufferedConn struct {
	net.Conn
	r io.Reader
}

func (c *bufferedConn) Read(b []byte) (int, error) { return c.r.Read(b) }

// captureReader passes a body through while keeping the first limit bytes
// for the inspector. Reading goes on after the limit so the client gets the
// whole body; the copy on screen is cut.
type captureReader struct {
	r         io.Reader
	limit     int64
	buf       bytes.Buffer
	truncated bool
	total     int64
}

func (c *captureReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	if n > 0 {
		c.total += int64(n)
		if left := c.limit - int64(c.buf.Len()); left > 0 {
			take := int64(n)
			if take > left {
				take = left
				c.truncated = true
			}
			c.buf.Write(p[:take])
		} else {
			c.truncated = true
		}
	}
	return n, err
}

func (c *captureReader) bytes() []byte { return c.buf.Bytes() }

func (c *captureReader) Close() error {
	if closer, ok := c.r.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

// captureBody reads a request body into memory, up to limit, and returns a
// reader that replays it whole: what was captured plus what was left.
func captureBody(r io.ReadCloser, limit int64) ([]byte, bool, io.ReadCloser) {
	if r == nil {
		return nil, false, nil
	}
	buf, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return buf, false, io.NopCloser(io.MultiReader(bytes.NewReader(buf), r))
	}
	truncated := int64(len(buf)) > limit
	kept := buf
	if truncated {
		kept = buf[:limit]
	}
	return kept, truncated, io.NopCloser(io.MultiReader(bytes.NewReader(buf), r))
}

// headerPairs flattens headers in name order, one pair per value.
func headerPairs(h http.Header) []HeaderPair {
	if len(h) == 0 {
		return nil
	}
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]HeaderPair, 0, len(keys))
	for _, k := range keys {
		for _, v := range h[k] {
			out = append(out, HeaderPair{Key: k, Value: v})
		}
	}
	return out
}

// stripHidden drops the headers that describe the proxy hop rather than
// the response, so the inspector shows what the server said.
func stripHidden(in []HeaderPair) []HeaderPair {
	out := in[:0]
	for _, h := range in {
		switch http.CanonicalHeaderKey(h.Key) {
		case "Proxy-Connection", "Proxy-Authenticate", "Proxy-Authorization", "Keep-Alive":
			continue
		}
		out = append(out, h)
	}
	return out
}

var hopHeaders = []string{
	"Connection", "Proxy-Connection", "Keep-Alive", "Proxy-Authenticate",
	"Proxy-Authorization", "Te", "Trailer", "Transfer-Encoding", "Upgrade",
}

func removeHopHeaders(h http.Header) {
	for _, name := range h.Values("Connection") {
		for _, f := range strings.Split(name, ",") {
			if f = strings.TrimSpace(f); f != "" {
				h.Del(f)
			}
		}
	}
	for _, name := range hopHeaders {
		h.Del(name)
	}
}

func copyHeader(dst, src http.Header) {
	for k, values := range src {
		for _, v := range values {
			dst.Add(k, v)
		}
	}
}

// displayMax is the largest a decoded body grows for the viewer, so a
// compressed bomb cannot take the window down.
const displayMax = 8 << 20

// usableEncoding is the client's Accept-Encoding without the encodings the
// inspector cannot decode. An empty result lets the transport choose gzip
// and hand the client plain bytes.
func usableEncoding(header string) string {
	if strings.TrimSpace(header) == "" {
		return ""
	}
	parts := strings.Split(header, ",")
	kept := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name := part
		if i := strings.IndexByte(name, ';'); i >= 0 {
			name = name[:i]
		}
		switch strings.ToLower(strings.TrimSpace(name)) {
		case "br", "zstd":
			continue
		}
		kept = append(kept, part)
	}
	return strings.Join(kept, ", ")
}

// decodeForDisplay returns the body a person can read: gzip and deflate
// decoded, anything else untouched. Brotli and zstd stay off the request's
// Accept-Encoding, so almost every body is readable here.
func decodeForDisplay(body []byte, encoding string) []byte {
	enc := strings.ToLower(strings.TrimSpace(encoding))
	switch enc {
	case "gzip", "x-gzip":
		zr, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return body
		}
		defer zr.Close()
		out, err := io.ReadAll(io.LimitReader(zr, displayMax))
		if err != nil && len(out) == 0 {
			return body
		}
		return out
	case "deflate":
		zr, err := zlib.NewReader(bytes.NewReader(body))
		if err != nil {
			return body
		}
		defer zr.Close()
		out, err := io.ReadAll(io.LimitReader(zr, displayMax))
		if err != nil && len(out) == 0 {
			return body
		}
		return out
	}
	return body
}
