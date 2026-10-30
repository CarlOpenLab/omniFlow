package model

import (
	"crypto/rand"
	"fmt"
	mrand "math/rand/v2"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Environment is a named set of variables. A request writes {{name}} where a
// value differs between environments, as a host or a key does between
// staging and production, and the environment in use fills it in.
type Environment struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Vars []KV   `json:"vars"`
}

// NewEnvironment is an environment with one empty row to write in.
func NewEnvironment(name string) Environment {
	return Environment{ID: NewID("env"), Name: name, Vars: []KV{NewKV()}}
}

// Clone returns a deep copy.
func (e Environment) Clone() Environment {
	out := e
	out.Vars = append([]KV(nil), e.Vars...)
	return out
}

// Values are the variables that are on and named, by name. A name given
// twice takes the later value, as a row added below another overrides it.
func (e Environment) Values() map[string]string {
	out := make(map[string]string, len(e.Vars))
	for _, kv := range e.Vars {
		if name := strings.TrimSpace(kv.Key); kv.Enabled && name != "" {
			out[name] = kv.Value
		}
	}
	return out
}

// varPattern is a {{name}} placeholder, with room for spaces inside the
// braces. A name starting with $ is one the app makes up as it sends.
var varPattern = regexp.MustCompile(`\{\{\s*([A-Za-z0-9_.\-$]+)\s*\}\}`)

// Placeholder is one {{name}} in a text, in runes, as text inputs count.
type Placeholder struct {
	Name       string
	Start, End int
}

// Placeholders finds the {{name}}s in s.
func Placeholders(s string) []Placeholder {
	if !strings.Contains(s, "{{") {
		return nil
	}
	var out []Placeholder
	for _, m := range varPattern.FindAllStringSubmatchIndex(s, -1) {
		out = append(out, Placeholder{
			Name:  s[m[2]:m[3]],
			Start: utf8.RuneCountInString(s[:m[0]]),
			End:   utf8.RuneCountInString(s[:m[1]]),
		})
	}
	return out
}

// dynamic are the variables the app makes up as each request goes, rather
// than ones an environment holds.
var dynamic = map[string]func() string{
	"$timestamp":    func() string { return strconv.FormatInt(time.Now().Unix(), 10) },
	"$isoTimestamp": func() string { return time.Now().UTC().Format(time.RFC3339) },
	"$uuid":         newUUID,
	"$guid":         newUUID,
	"$randomInt":    func() string { return strconv.Itoa(mrand.IntN(1001)) },
}

// Dynamic reports whether a variable is one the app makes up, as a time or
// an id.
func Dynamic(name string) bool { _, ok := dynamic[name]; return ok }

// DynamicVariables are the names the app fills in by itself, as a person
// reads them.
var DynamicVariables = []string{"$timestamp", "$isoTimestamp", "$uuid", "$randomInt"}

// Resolver fills placeholders in: from the environment's values, or with a
// value made up for a dynamic one. It notes the names it could not fill.
type Resolver struct {
	Values  map[string]string
	Missing []string
	// probe fills nothing in, so Missing collects every name used.
	probe bool
}

// Resolve fills in the placeholders of s; one it cannot fill stays as it is.
func (r *Resolver) Resolve(s string) string {
	if !strings.Contains(s, "{{") {
		return s
	}
	return varPattern.ReplaceAllStringFunc(s, func(m string) string {
		name := varPattern.FindStringSubmatch(m)[1]
		if v, ok := r.value(name); ok {
			return v
		}
		if !slices.Contains(r.Missing, name) {
			r.Missing = append(r.Missing, name)
		}
		return m
	})
}

func (r *Resolver) value(name string) (string, bool) {
	if r.probe {
		return "", false
	}
	if v, ok := r.Values[name]; ok {
		return v, true
	}
	if make, ok := dynamic[name]; ok {
		return make(), true
	}
	return "", false
}

// newUUID is a random (version 4) UUID.
func newUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// Resolved is the request with its placeholders filled in, in what is sent:
// the URL, the rows that are on, the body its kind sends and the auth its
// type adds. Rows that are off keep theirs, so they never count as missing.
func (r Request) Resolved(res *Resolver) Request {
	out := r.Clone()
	out.URL = res.Resolve(out.URL)
	for _, rows := range [][]KV{out.Params, out.Headers} {
		resolveRows(res, rows)
	}
	switch out.BodyKind {
	case BodyJSON, BodyText:
		out.Body = res.Resolve(out.Body)
	case BodyForm:
		resolveRows(res, out.Form)
		out.Body = res.Resolve(out.Body)
	}
	switch out.AuthType {
	case AuthBearer:
		out.Token = res.Resolve(out.Token)
	case AuthBasic:
		out.User, out.Pass = res.Resolve(out.User), res.Resolve(out.Pass)
	}
	return out
}

func resolveRows(res *Resolver, rows []KV) {
	for i := range rows {
		if rows[i].Enabled {
			rows[i].Key = res.Resolve(rows[i].Key)
			rows[i].Value = res.Resolve(rows[i].Value)
		}
	}
}

// Variables are the names the request's placeholders use, in what it
// sends, each once, in the order they come.
func (r Request) Variables() []string {
	probe := Resolver{probe: true}
	r.Resolved(&probe)
	return probe.Missing
}
