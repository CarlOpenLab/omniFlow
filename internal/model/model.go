// Package model holds OmniFlow's saved data: requests, history and the
// preferences the window reads.
package model

import (
	"fmt"
	"net/url"
	"slices"
	"strings"
	"sync/atomic"
	"time"
)

// KV is one row of a query, header or form table. A row the user unchecked
// stays in the list but is not sent.
type KV struct {
	ID      string `json:"id"`
	Enabled bool   `json:"enabled"`
	Key     string `json:"key"`
	Value   string `json:"value"`
}

// body kinds a request can carry.
const (
	BodyNone = "none"
	BodyJSON = "json"
	BodyText = "text"
	BodyForm = "form"
)

// auth kinds a request can use.
const (
	AuthNone   = "none"
	AuthBearer = "bearer"
	AuthBasic  = "basic"
)

// Request is a draft in the composer or one saved in a collection.
type Request struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Method   string `json:"method"`
	URL      string `json:"url"`
	Params   []KV   `json:"params"`
	Headers  []KV   `json:"headers"`
	BodyKind string `json:"bodyKind"`
	Body     string `json:"body"`
	// Form is the body's fields when BodyKind is BodyForm, sent URL-encoded.
	Form     []KV   `json:"form,omitempty"`
	AuthType string `json:"authType"`
	Token    string `json:"token"`
	User     string `json:"user"`
	Pass     string `json:"pass"`

	Saved     bool      `json:"saved"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Clone returns a deep copy, so a saved request loads without sharing its
// slices with the collection.
func (r Request) Clone() Request {
	out := r
	out.Params = append([]KV(nil), r.Params...)
	out.Headers = append([]KV(nil), r.Headers...)
	out.Form = append([]KV(nil), r.Form...)
	return out
}

// NewRequest is the empty composer: GET, no rows, no body.
func NewRequest() Request {
	return Request{
		ID:       NewID("req"),
		Name:     "Untitled Request",
		Method:   "GET",
		BodyKind: BodyNone,
		AuthType: AuthNone,
		Params:   []KV{NewKV()},
		Headers:  []KV{NewKV()},
		Form:     []KV{NewKV()},
	}
}

// SameContent reports whether two requests would send the same thing under
// the same name. Ids, timestamps and rows left blank do not count, so a row
// added and not yet written in does not mark a saved request as edited.
func (r Request) SameContent(o Request) bool {
	return r.DisplayName() == o.DisplayName() &&
		r.Method == o.Method && r.URL == o.URL &&
		r.BodyKind == o.BodyKind && r.Body == o.Body &&
		r.AuthType == o.AuthType && r.Token == o.Token && r.User == o.User && r.Pass == o.Pass &&
		sameRows(r.Params, o.Params) && sameRows(r.Headers, o.Headers) && sameRows(r.Form, o.Form)
}

// sameRows compares the rows that hold something, in order.
func sameRows(a, b []KV) bool {
	return slices.EqualFunc(filled(a), filled(b), func(x, y KV) bool {
		return x.Enabled == y.Enabled && x.Key == y.Key && x.Value == y.Value
	})
}

func filled(rows []KV) []KV {
	out := make([]KV, 0, len(rows))
	for _, kv := range rows {
		if strings.TrimSpace(kv.Key) != "" || strings.TrimSpace(kv.Value) != "" {
			out = append(out, kv)
		}
	}
	return out
}

// Normalize brings a request saved by an earlier version up to this one: a
// form body was once written as key=value text, and is now a table of rows.
func (r *Request) Normalize() {
	if r.BodyKind != BodyForm || len(filled(r.Form)) > 0 || strings.TrimSpace(r.Body) == "" {
		return
	}
	// Split by hand rather than with url.ParseQuery, whose map would lose
	// the order the fields were written in.
	var rows []KV
	for _, part := range strings.Split(strings.TrimSpace(r.Body), "&") {
		if part == "" {
			continue
		}
		k, v, _ := strings.Cut(part, "=")
		key, err := url.QueryUnescape(k)
		if err != nil {
			return
		}
		value, err := url.QueryUnescape(v)
		if err != nil {
			return
		}
		rows = append(rows, KV{ID: NewID("kv"), Enabled: true, Key: key, Value: value})
	}
	r.Form, r.Body = rows, ""
}

// DisplayName is the name a row shows: the saved name, or the request's
// path when the draft has none worth showing.
func (r Request) DisplayName() string {
	if strings.TrimSpace(r.Name) != "" && r.Name != "Untitled Request" {
		return r.Name
	}
	u := strings.TrimSpace(r.URL)
	if u == "" {
		return "Untitled Request"
	}
	if i := strings.Index(u, "://"); i >= 0 {
		u = u[i+3:]
	}
	if i := strings.IndexAny(u, "?#"); i >= 0 {
		u = u[:i]
	}
	if len(u) > 48 {
		u = u[:47] + "…"
	}
	return u
}

// HistoryEntry is one sent request, kept so it can be loaded again.
type HistoryEntry struct {
	ID       string    `json:"id"`
	Method   string    `json:"method"`
	URL      string    `json:"url"`
	Status   int       `json:"status"`
	Duration int64     `json:"durationMs"`
	At       time.Time `json:"at"`
	// Request is the whole request as it was sent: its rows, body and
	// auth. Entries written by earlier versions have only the method and
	// the URL.
	Request *Request `json:"request,omitempty"`
	// BodyDropped says the body was too large to keep with the request.
	BodyDropped bool `json:"bodyDropped,omitempty"`
}

// MockRule answers a matching request from a file instead of the network.
// Path is the URL's path only, so a mock keeps working while the query
// changes. Status and ContentType are the captured response's.
type MockRule struct {
	ID          string    `json:"id"`
	Enabled     bool      `json:"enabled"`
	Name        string    `json:"name"`
	Method      string    `json:"method"`
	Host        string    `json:"host"`
	Path        string    `json:"path"`
	Status      int       `json:"status"`
	ContentType string    `json:"contentType,omitempty"`
	File        string    `json:"file"`
	CreatedAt   time.Time `json:"createdAt"`
}

// Kinds of proxy rule. A Map Remote sends a matching request to another
// origin; the header kinds add, set or remove one header.
const (
	RuleMapRemote      = "mapRemote"
	RuleRequestHeader  = "requestHeader"
	RuleResponseHeader = "responseHeader"
)

// ProxyRule changes matching traffic as it passes through the proxy. Method,
// Host and PathPrefix narrow the match; an empty one matches everything.
// Host accepts a leading "*." for a subdomain wildcard.
type ProxyRule struct {
	ID         string    `json:"id"`
	Enabled    bool      `json:"enabled"`
	Name       string    `json:"name"`
	Kind       string    `json:"kind"`
	Method     string    `json:"method,omitempty"`
	Host       string    `json:"host,omitempty"`
	PathPrefix string    `json:"pathPrefix,omitempty"`
	Target     string    `json:"target,omitempty"`
	Header     string    `json:"header,omitempty"`
	Remove     bool      `json:"remove,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
}

// Prefs are the settings the window stores. A field added later reads as
// its zero value from an older file, so each is named for that zero to be
// the default: StopAtRedirects rather than FollowRedirects.
type Prefs struct {
	Theme     string `json:"theme"`     // "", light or dark
	ProxyPort string `json:"proxyPort"` // loopback port
	// Language is the language the app shows: "", "en" or "zh-Hans";
	// "" follows the system's preferred languages.
	Language       string `json:"language,omitempty"`
	ProxyAutostart bool   `json:"proxyAutostart"`
	DecryptHTTPS   bool   `json:"decryptHTTPS"`
	FollowFlows    bool   `json:"followFlows"`
	KeepHistory    bool   `json:"keepHistory"`
	// LANAccess lets a device on the local network use the proxy, and starts
	// the setup page that installs its certificate.
	LANAccess bool `json:"lanAccess,omitempty"`

	// Environment is the id of the environment the composer resolves
	// {{variables}} from; "" for none.
	Environment     string `json:"environment,omitempty"`
	TimeoutSec      int    `json:"timeoutSec,omitempty"` // 0 for DefaultTimeout
	StopAtRedirects bool   `json:"stopAtRedirects,omitempty"`
	SkipTLSVerify   bool   `json:"skipTLSVerify,omitempty"`

	// The splits between the panes, in percent of the height; 0 for the
	// window's own.
	ComposerSplit float32 `json:"composerSplit,omitempty"`
	InspectSplit  float32 `json:"inspectSplit,omitempty"`

	// CardOpacity is how opaque the workspace card is over the window's
	// material, 0.30 to 1; 0 for the default. Blur is the material's
	// strength: none, subtle, medium or deep; "" for subtle.
	CardOpacity float64 `json:"cardOpacity,omitempty"`
	Blur        string  `json:"blur,omitempty"`

	// Throttle is the capture's simulated network: "", slow3g, fast3g or 4g.
	Throttle string `json:"throttle,omitempty"`
}

// DefaultTimeout is how long the composer waits for an answer unless the
// settings say otherwise.
const DefaultTimeout = 30 * time.Second

// Timeout is how long the composer waits for an answer.
func (p Prefs) Timeout() time.Duration {
	if p.TimeoutSec <= 0 {
		return DefaultTimeout
	}
	return time.Duration(p.TimeoutSec) * time.Second
}

// DefaultPrefs is the first run: acrylic over the window's material, the
// proxy on 9090 with HTTPS decryption ready to start.
func DefaultPrefs() Prefs {
	return Prefs{
		Theme:          "",
		ProxyPort:      "9090",
		ProxyAutostart: true,
		DecryptHTTPS:   true,
		FollowFlows:    true,
		KeepHistory:    true,
	}
}

// Data is everything stored in the app's JSON file.
type Data struct {
	Prefs        Prefs          `json:"prefs"`
	Requests     []Request      `json:"requests"`
	History      []HistoryEntry `json:"history"`
	Environments []Environment  `json:"environments"`
	Mocks        []MockRule     `json:"mocks,omitempty"`
	Rules        []ProxyRule    `json:"rules,omitempty"`
}

var idSeq atomic.Uint64

// NewID makes an id that is unique within a run and unlikely to collide
// with one saved by an earlier run.
func NewID(prefix string) string {
	return fmt.Sprintf("%s_%x_%x", prefix, time.Now().UnixNano(), idSeq.Add(1))
}

// NewKV returns an enabled empty row with a new id.
func NewKV() KV {
	return KV{ID: NewID("kv"), Enabled: true}
}

// SeedRequests is what a fresh install's collection holds: three requests
// that show the three things the composer can do.
func SeedRequests() []Request {
	now := time.Now()
	jsonHeaders := []KV{{ID: NewID("kv"), Enabled: true, Key: "Content-Type", Value: "application/json"}}
	seed := []Request{
		{
			ID: NewID("req"), Name: "GitHub — Repository", Method: "GET",
			URL:      "https://api.github.com/repos/egoist/mygo",
			BodyKind: BodyNone, AuthType: AuthNone, Saved: true, UpdatedAt: now,
		},
		{
			ID: NewID("req"), Name: "HTTPBin — Echo JSON", Method: "POST",
			URL:      "https://httpbin.org/post",
			Headers:  append([]KV(nil), jsonHeaders...),
			BodyKind: BodyJSON, Body: "{\n  \"hello\": \"world\",\n  \"from\": \"OmniFlow\"\n}",
			AuthType: AuthNone, Saved: true, UpdatedAt: now,
		},
		{
			ID: NewID("req"), Name: "Postman Echo — Query", Method: "GET",
			URL: "https://postman-echo.com/get",
			Params: []KV{
				{ID: NewID("kv"), Enabled: true, Key: "utm_source", Value: "omniflow"},
				{ID: NewID("kv"), Enabled: true, Key: "limit", Value: "5"},
			},
			BodyKind: BodyNone, AuthType: AuthNone, Saved: true, UpdatedAt: now,
		},
	}
	return seed
}
