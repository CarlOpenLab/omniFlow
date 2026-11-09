package proxy

import (
	"time"
)

// HeaderPair is one header line of a captured request or response.
type HeaderPair struct{ Key, Value string }

// Flow is one request the proxy carried: what it was, what came back, and
// how long the exchange took. Bodies are cut at MaxBody; Truncated says so.
type Flow struct {
	ID     uint64    `json:"id"`
	At     time.Time `json:"at"`
	Method string    `json:"method"`
	Scheme string    `json:"scheme"`
	Host   string    `json:"host"`
	Path   string    `json:"path"`
	URL    string    `json:"url"`
	TLS    bool      `json:"tls"`
	Proto  string    `json:"proto"`

	ReqHeaders   []HeaderPair `json:"reqHeaders"`
	ReqBody      []byte       `json:"reqBody"`
	ReqTruncated bool         `json:"reqTruncated"`
	ReqSize      int64        `json:"reqSize"`

	Status        int          `json:"status"`
	StatusText    string       `json:"statusText"`
	RespHeaders   []HeaderPair `json:"respHeaders"`
	RespBody      []byte       `json:"respBody"`
	RespTruncated bool         `json:"respTruncated"`
	RespSize      int64        `json:"respSize"`
	ContentType   string       `json:"contentType"`

	Duration time.Duration `json:"duration"`
	Err      string        `json:"err"`
	// Mocked says a Map Local rule answered this exchange instead of the
	// network.
	Mocked bool `json:"mocked,omitempty"`
	// Mapped says a Map Remote rule sent the exchange to another origin.
	Mapped   bool   `json:"mapped,omitempty"`
	MappedTo string `json:"mappedTo,omitempty"`
}
