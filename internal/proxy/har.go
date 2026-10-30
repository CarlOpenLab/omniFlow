package proxy

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

// The HTTP Archive 1.2 format, as far as a captured flow fills it in.
type (
	harLog struct {
		Log harBody `json:"log"`
	}
	harBody struct {
		Version string     `json:"version"`
		Creator harCreator `json:"creator"`
		Entries []harEntry `json:"entries"`
	}
	harCreator struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	harEntry struct {
		StartedDateTime string      `json:"startedDateTime"`
		Time            float64     `json:"time"`
		Request         harRequest  `json:"request"`
		Response        harResponse `json:"response"`
		Cache           struct{}    `json:"cache"`
		Timings         harTimings  `json:"timings"`
		Error           string      `json:"_error,omitempty"`
	}
	harRequest struct {
		Method      string    `json:"method"`
		URL         string    `json:"url"`
		HTTPVersion string    `json:"httpVersion"`
		Cookies     []harPair `json:"cookies"`
		Headers     []harPair `json:"headers"`
		QueryString []harPair `json:"queryString"`
		PostData    *harPost  `json:"postData,omitempty"`
		HeadersSize int       `json:"headersSize"`
		BodySize    int64     `json:"bodySize"`
	}
	harResponse struct {
		Status      int        `json:"status"`
		StatusText  string     `json:"statusText"`
		HTTPVersion string     `json:"httpVersion"`
		Cookies     []harPair  `json:"cookies"`
		Headers     []harPair  `json:"headers"`
		Content     harContent `json:"content"`
		RedirectURL string     `json:"redirectURL"`
		HeadersSize int        `json:"headersSize"`
		BodySize    int64      `json:"bodySize"`
	}
	harPair struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}
	harPost struct {
		MimeType string `json:"mimeType"`
		Text     string `json:"text"`
	}
	harContent struct {
		Size     int64  `json:"size"`
		MimeType string `json:"mimeType"`
		Text     string `json:"text,omitempty"`
		Encoding string `json:"encoding,omitempty"`
		Comment  string `json:"comment,omitempty"`
	}
	harTimings struct {
		Send    float64 `json:"send"`
		Wait    float64 `json:"wait"`
		Receive float64 `json:"receive"`
	}
)

// HAR writes flows as an HTTP Archive, the file browsers' developer tools
// and other proxies open, in the order given. Bodies that are not text go in
// base64, as the format allows for a response; the inspector's cut is noted.
func HAR(flows []Flow, creator, version string) ([]byte, error) {
	out := harLog{Log: harBody{
		Version: "1.2",
		Creator: harCreator{Name: creator, Version: version},
		Entries: make([]harEntry, 0, len(flows)),
	}}
	for _, f := range flows {
		out.Log.Entries = append(out.Log.Entries, harOf(f))
	}
	return json.MarshalIndent(out, "", "  ")
}

func harOf(f Flow) harEntry {
	ms := float64(f.Duration) / float64(time.Millisecond)
	proto := f.Proto
	if proto == "" {
		proto = "HTTP/1.1"
	}
	e := harEntry{
		StartedDateTime: f.At.Format("2006-01-02T15:04:05.000Z07:00"),
		Time:            ms,
		Request: harRequest{
			Method: f.Method, URL: f.URL, HTTPVersion: proto,
			Cookies: []harPair{}, Headers: harPairs(f.ReqHeaders), QueryString: queryPairs(f.URL),
			HeadersSize: -1, BodySize: max(f.ReqSize, 0),
		},
		Response: harResponse{
			Status: f.Status, StatusText: f.StatusText, HTTPVersion: proto,
			Cookies: []harPair{}, Headers: harPairs(f.RespHeaders),
			RedirectURL: headerOf(f.RespHeaders, "Location"),
			HeadersSize: -1, BodySize: -1,
		},
		Timings: harTimings{Wait: ms},
		Error:   f.Err,
	}
	if len(f.ReqBody) > 0 {
		// postData has no encoding of its own: a body that is not text keeps
		// its type and size only.
		post := &harPost{MimeType: headerOf(f.ReqHeaders, "Content-Type")}
		if utf8.Valid(f.ReqBody) {
			post.Text = string(f.ReqBody)
		}
		e.Request.PostData = post
	}
	if f.Status > 0 {
		e.Response.BodySize = max(f.RespSize, 0)
		c := harContent{Size: int64(len(f.RespBody)), MimeType: f.ContentType}
		switch {
		case len(f.RespBody) == 0:
		case utf8.Valid(f.RespBody):
			c.Text = string(f.RespBody)
		default:
			c.Text, c.Encoding = base64.StdEncoding.EncodeToString(f.RespBody), "base64"
		}
		if f.RespTruncated {
			c.Comment = "OmniFlow kept only the beginning of this body."
		}
		e.Response.Content = c
	}
	return e
}

func harPairs(headers []HeaderPair) []harPair {
	out := make([]harPair, 0, len(headers))
	for _, h := range headers {
		out = append(out, harPair{Name: h.Key, Value: h.Value})
	}
	return out
}

// queryPairs lists a URL's query in its own order, as the request sent it.
func queryPairs(raw string) []harPair {
	out := []harPair{}
	u, err := url.Parse(raw)
	if err != nil || u.RawQuery == "" {
		return out
	}
	for _, part := range strings.Split(u.RawQuery, "&") {
		if part == "" {
			continue
		}
		k, v, _ := strings.Cut(part, "=")
		if uk, err := url.QueryUnescape(k); err == nil {
			k = uk
		}
		if uv, err := url.QueryUnescape(v); err == nil {
			v = uv
		}
		out = append(out, harPair{Name: k, Value: v})
	}
	return out
}

func headerOf(headers []HeaderPair, key string) string {
	key = http.CanonicalHeaderKey(key)
	for _, h := range headers {
		if http.CanonicalHeaderKey(h.Key) == key {
			return h.Value
		}
	}
	return ""
}
