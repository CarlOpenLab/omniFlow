package proxy

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

func TestHARWritesFlowsAsAnArchive(t *testing.T) {
	at := time.Date(2026, 10, 30, 15, 26, 12, 345e6, time.UTC)
	flows := []Flow{
		{
			At: at, Method: "POST", URL: "https://api.example.com/items?b=2&a=x%20y", Proto: "HTTP/1.1",
			ReqHeaders: []HeaderPair{{Key: "Content-Type", Value: "application/json"}},
			ReqBody:    []byte(`{"hello":"world"}`), ReqSize: 17,
			Status: 201, StatusText: "Created", ContentType: "application/json",
			RespHeaders: []HeaderPair{{Key: "Location", Value: "/items/7"}},
			RespBody:    []byte(`{"id":7}`), RespSize: 8, Duration: 284 * time.Millisecond,
		},
		{At: at, Method: "GET", URL: "https://cdn.example.com/p.png", Status: 200, ContentType: "image/png",
			RespBody: []byte{0x89, 'P', 'N', 'G', 0x00, 0xff}, RespSize: 6, RespTruncated: true},
		{At: at, Method: "CONNECT", URL: "https://pinned.example.com", Err: "refused"},
	}
	raw, err := HAR(flows, "OmniFlow", "0.1.0")
	if err != nil {
		t.Fatalf("HAR: %v", err)
	}
	var doc struct {
		Log struct {
			Version string
			Creator struct{ Name, Version string }
			Entries []struct {
				StartedDateTime string
				Time            float64
				Request         struct {
					Method      string
					QueryString []struct{ Name, Value string }
					PostData    *struct{ MimeType, Text string }
				}
				Response struct {
					Status      int
					RedirectURL string
					Content     struct {
						Size                     int64
						MimeType, Text, Encoding string
						Comment                  string
					}
				}
				Error string `json:"_error"`
			}
		}
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("the archive is not JSON: %v", err)
	}
	log := doc.Log
	if log.Version != "1.2" || log.Creator.Name != "OmniFlow" || len(log.Entries) != 3 {
		t.Fatalf("log %+v", log)
	}
	first := log.Entries[0]
	if first.StartedDateTime != "2026-10-30T15:26:12.345Z" || first.Time != 284 {
		t.Errorf("first entry at %s for %v ms", first.StartedDateTime, first.Time)
	}
	if q := first.Request.QueryString; len(q) != 2 || q[0].Name != "b" || q[1].Value != "x y" {
		t.Errorf("query %+v", q)
	}
	if p := first.Request.PostData; p == nil || p.Text != `{"hello":"world"}` || p.MimeType != "application/json" {
		t.Errorf("post data %+v", p)
	}
	if first.Response.Status != 201 || first.Response.RedirectURL != "/items/7" || first.Response.Content.Text != `{"id":7}` {
		t.Errorf("response %+v", first.Response)
	}
	pic := log.Entries[1].Response.Content
	if pic.Encoding != "base64" || pic.Comment == "" {
		t.Errorf("binary content %+v", pic)
	}
	if b, _ := base64.StdEncoding.DecodeString(pic.Text); string(b) != "\x89PNG\x00\xff" {
		t.Errorf("binary content decodes to %q", b)
	}
	if failed := log.Entries[2]; failed.Error != "refused" || failed.Response.Status != 0 {
		t.Errorf("failed entry %+v", failed)
	}
}
