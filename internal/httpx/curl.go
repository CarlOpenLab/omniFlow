package httpx

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/CarlOpenLab/omniflow/internal/model"
)

// Curl writes the spec as a curl command that sends the same request, and
// is sent the same way: following redirects, or skipping certificate checks,
// as the spec does. The composer's default User-Agent is left to curl's own.
func Curl(spec Spec) (string, error) {
	p, err := prepare(spec)
	if err != nil {
		return "", err
	}
	var flags []string
	if !spec.NoRedirects {
		flags = append(flags, "--location")
	}
	if spec.Insecure {
		flags = append(flags, "--insecure")
	}
	return curlCommand(flags, p.method, p.url.String(), p.headers, p.body), nil
}

// CurlCommand writes a request as a curl command in the quoting a POSIX
// shell reads back exactly: the method and the URL on the first line, then
// a header or the body to a line.
func CurlCommand(method, rawURL string, headers []Header, body []byte) string {
	return curlCommand(nil, method, rawURL, headers, body)
}

func curlCommand(flags []string, method, rawURL string, headers []Header, body []byte) string {
	method = strings.ToUpper(method)
	first := "curl "
	for _, f := range flags {
		first += f + " "
	}
	switch {
	case method == http.MethodHead:
		first += "--head "
	case method == http.MethodGet && len(body) == 0, method == http.MethodPost && len(body) > 0:
		// What curl does without being told.
	default:
		first += "-X " + method + " "
	}
	lines := []string{first + shellQuote(rawURL)}
	for _, h := range headers {
		lines = append(lines, "-H "+shellQuote(h.Key+": "+h.Value))
	}
	if len(body) > 0 {
		lines = append(lines, "--data-raw "+shellQuote(string(body)))
	}
	return strings.Join(lines, " \\\n  ")
}

// shellQuote quotes s for a POSIX shell: in single quotes when it is text,
// and in ANSI-C quotes with escapes when it holds bytes a terminal would
// not carry, as a binary body does.
func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	if utf8.ValidString(s) && !strings.ContainsRune(s, 0) {
		return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
	}
	var b strings.Builder
	b.WriteString("$'")
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '\\' || c == '\'':
			b.WriteByte('\\')
			b.WriteByte(c)
		case c >= 0x20 && c < 0x7f:
			b.WriteByte(c)
		default:
			fmt.Fprintf(&b, `\x%02x`, c)
		}
	}
	b.WriteByte('\'')
	return b.String()
}

// LooksLikeCurl reports whether text is a curl command, as one pasted from
// a browser's "Copy as cURL" into the URL field.
func LooksLikeCurl(text string) bool {
	fields := strings.Fields(text)
	return len(fields) > 1 && isCurl(fields[0])
}

func isCurl(word string) bool {
	return word == "curl" || word == "curl.exe" || strings.HasSuffix(word, "/curl")
}

// valueFlags are curl's own options that take a value, which an import
// passes over together with it.
var valueFlags = map[string]bool{
	"-o": true, "--output": true, "-m": true, "--max-time": true, "--connect-timeout": true,
	"-x": true, "--proxy": true, "-U": true, "--proxy-user": true, "--proxy-header": true,
	"-w": true, "--write-out": true, "-E": true, "--cert": true, "--key": true, "--cert-type": true,
	"--key-type": true, "--cacert": true, "--capath": true, "--pinnedpubkey": true,
	"--resolve": true, "--connect-to": true, "--retry": true, "--retry-delay": true,
	"--retry-max-time": true, "-r": true, "--range": true, "-c": true, "--cookie-jar": true,
	"-D": true, "--dump-header": true, "--limit-rate": true, "-K": true, "--config": true,
	"--interface": true, "--max-redirs": true, "--max-filesize": true, "-z": true, "--time-cond": true,
	"-Y": true, "--speed-limit": true, "-y": true, "--speed-time": true, "--noproxy": true,
	"--unix-socket": true, "--abstract-unix-socket": true, "--request-target": true,
	"--tls-max": true, "--ciphers": true, "--proto": true, "--proto-redir": true,
	"--trace": true, "--trace-ascii": true, "--stderr": true, "--output-dir": true,
	"--expect100-timeout": true, "--keepalive-time": true, "--local-port": true, "--dns-servers": true,
	"--preproxy": true, "--socks4": true, "--socks5": true, "--socks5-hostname": true,
	"--aws-sigv4": true, "--variable": true, "--netrc-file": true, "-Q": true, "--quote": true,
}

// ParseCurl reads a curl command into a request, as a browser's "Copy as
// cURL" writes one for a POSIX shell. Options that only steer curl itself
// are passed over; what the composer cannot carry, as a file to upload, is
// left out and named in the notes.
func ParseCurl(cmd string) (model.Request, []string, error) {
	words, err := shellWords(cmd)
	if err != nil {
		return model.Request{}, nil, err
	}
	if len(words) == 0 || !isCurl(words[0]) {
		return model.Request{}, nil, errors.New("That is not a curl command")
	}
	r := model.NewRequest()
	r.Headers = nil
	var (
		rawURL, method string
		data, query    []string
		notes          []string
		get, head      bool
		jsonBody       bool
	)
	note := func(s string) {
		if !slices.Contains(notes, s) {
			notes = append(notes, s)
		}
	}
	header := func(k, v string) {
		r.Headers = append(r.Headers, model.KV{ID: model.NewID("kv"), Enabled: true, Key: k, Value: v})
	}
	for i := 1; i < len(words); i++ {
		w := words[i]
		name, value, attached := w, "", false
		switch {
		case strings.HasPrefix(w, "--"):
			if k, v, ok := strings.Cut(w, "="); ok {
				name, value, attached = k, v, true
			}
		case len(w) > 2 && w[0] == '-' && strings.IndexByte("XHdbuAeFT", w[1]) >= 0:
			// A short option with its value joined on, as -XPOST.
			name, value, attached = w[:2], w[2:], true
		}
		arg := func() string {
			if attached {
				return value
			}
			if i+1 < len(words) {
				i++
				return words[i]
			}
			return ""
		}
		switch name {
		case "-X", "--request":
			method = strings.ToUpper(arg())
		case "-H", "--header":
			h := arg()
			if k, v, ok := strings.Cut(h, ":"); ok {
				k = strings.TrimSpace(k)
				switch strings.ToLower(k) {
				case "content-length", "accept-encoding":
					// Counted, and decoded, by the composer itself.
				default:
					header(k, strings.TrimSpace(v))
				}
			} else if k, ok := strings.CutSuffix(strings.TrimSpace(h), ";"); ok {
				header(k, "") // curl's way of writing an empty header
			}
		case "-d", "--data", "--data-ascii", "--data-binary", "--data-raw":
			v := arg()
			if name != "--data-raw" && strings.HasPrefix(v, "@") {
				note("data read from a file (" + v + ")")
				continue
			}
			data = append(data, v)
		case "--data-urlencode":
			if d := urlencodeData(arg(), note); d != "" {
				data = append(data, d)
			}
		case "--json":
			data = append(data, arg())
			jsonBody = true
		case "--url-query":
			query = append(query, arg())
		case "-u", "--user":
			user, pass, _ := strings.Cut(arg(), ":")
			r.AuthType, r.User, r.Pass = model.AuthBasic, user, pass
		case "--oauth2-bearer":
			r.AuthType, r.Token = model.AuthBearer, arg()
		case "-A", "--user-agent":
			header("User-Agent", arg())
		case "-e", "--referer":
			header("Referer", arg())
		case "-b", "--cookie":
			if v := arg(); strings.Contains(v, "=") {
				header("Cookie", v)
			} else {
				note("cookies read from a file (" + v + ")")
			}
		case "-I", "--head":
			head = true
		case "-G", "--get":
			get = true
		case "--url":
			rawURL = arg()
		case "-F", "--form", "--form-string":
			arg()
			note("multipart form fields (-F)")
		case "-T", "--upload-file":
			arg()
			note("an uploaded file (-T)")
		default:
			if strings.HasPrefix(w, "-") && len(w) > 1 {
				if valueFlags[name] && !attached {
					i++
				}
				continue
			}
			if rawURL == "" {
				rawURL = w
			}
		}
	}
	if rawURL == "" {
		return model.Request{}, nil, errors.New("That curl command has no URL")
	}
	body := strings.Join(data, "&")
	if get && body != "" {
		query, body = append(query, body), ""
	}
	if len(query) > 0 {
		sep := "?"
		if strings.Contains(rawURL, "?") {
			sep = "&"
		}
		rawURL += sep + strings.Join(query, "&")
	}
	switch {
	case method != "":
	case head:
		method = http.MethodHead
	case body != "":
		method = http.MethodPost
	default:
		method = http.MethodGet
	}
	r.Method, r.URL = method, rawURL
	if body != "" {
		setBody(&r, body, jsonBody)
	}
	if len(r.Headers) == 0 {
		r.Headers = []model.KV{model.NewKV()}
	}
	return r, notes, nil
}

// urlencodeData is one --data-urlencode: the content URL-encoded, after a
// name= when it has one, as curl writes it.
func urlencodeData(v string, note func(string)) string {
	i := strings.IndexAny(v, "=@")
	switch {
	case i < 0:
		return url.QueryEscape(v)
	case v[i] == '@':
		note("data read from a file (" + v + ")")
		return ""
	case i == 0:
		return url.QueryEscape(v[1:])
	}
	return v[:i] + "=" + url.QueryEscape(v[i+1:])
}

// setBody gives an imported request its body in the kind that sends the
// same bytes under the same type: JSON when the type says so, a form when
// writing its fields out again gives back what curl sends, and text, with
// curl's own type, otherwise.
func setBody(r *model.Request, body string, jsonBody bool) {
	ct, accept := "", false
	for _, h := range r.Headers {
		switch {
		case strings.EqualFold(h.Key, "Content-Type"):
			ct = strings.ToLower(h.Value)
		case strings.EqualFold(h.Key, "Accept"):
			accept = true
		}
	}
	add := func(k, v string) {
		r.Headers = append(r.Headers, model.KV{ID: model.NewID("kv"), Enabled: true, Key: k, Value: v})
	}
	switch {
	case jsonBody:
		// --json sends its type and asks for JSON back; the composer adds
		// the type itself.
		r.BodyKind, r.Body = model.BodyJSON, body
		if !accept {
			add("Accept", "application/json")
		}
	case strings.Contains(ct, "json"):
		r.BodyKind, r.Body = model.BodyJSON, body
	case ct == "" || strings.Contains(ct, "x-www-form-urlencoded"):
		form := model.Request{BodyKind: model.BodyForm, Body: body}
		form.Normalize()
		if encoded, _ := encodeBody(Spec{BodyKind: model.BodyForm, Form: form.Form}); len(form.Form) > 0 && string(encoded) == body {
			r.BodyKind, r.Form = model.BodyForm, form.Form
			return
		}
		r.BodyKind, r.Body = model.BodyText, body
		if ct == "" {
			add("Content-Type", "application/x-www-form-urlencoded")
		}
	default:
		r.BodyKind, r.Body = model.BodyText, body
	}
}

var errUnclosed = errors.New("That curl command has a quote that is never closed")

// shellWords splits a command line as a POSIX shell does: words apart by
// blanks, '…' taken as it is, "…" with its backslash escapes, $'…' with C's,
// and a backslash at the end of a line joining it to the next.
func shellWords(s string) ([]string, error) {
	var words []string
	var cur strings.Builder
	inWord := false
	for i := 0; i < len(s); {
		switch c := s[i]; {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			if inWord {
				words = append(words, cur.String())
				cur.Reset()
				inWord = false
			}
			i++
		case c == '\\':
			switch {
			case strings.HasPrefix(s[i+1:], "\n"):
				i += 2
			case strings.HasPrefix(s[i+1:], "\r\n"):
				i += 3
			case i+1 < len(s):
				cur.WriteByte(s[i+1])
				inWord = true
				i += 2
			default:
				i++
			}
		case c == '\'':
			end := strings.IndexByte(s[i+1:], '\'')
			if end < 0 {
				return nil, errUnclosed
			}
			cur.WriteString(s[i+1 : i+1+end])
			inWord = true
			i += end + 2
		case c == '$' && strings.HasPrefix(s[i+1:], "'"):
			next, err := ansiC(s, i+2, &cur)
			if err != nil {
				return nil, err
			}
			inWord = true
			i = next
		case c == '"':
			j := i + 1
			for ; ; j++ {
				if j >= len(s) {
					return nil, errUnclosed
				}
				if s[j] == '"' {
					break
				}
				if s[j] == '\\' && j+1 < len(s) && strings.IndexByte("\"\\$`\n", s[j+1]) >= 0 {
					if s[j+1] != '\n' {
						cur.WriteByte(s[j+1])
					}
					j++
					continue
				}
				cur.WriteByte(s[j])
			}
			inWord = true
			i = j + 1
		default:
			cur.WriteByte(c)
			inWord = true
			i++
		}
	}
	if inWord {
		words = append(words, cur.String())
	}
	return words, nil
}

// ansiC reads a $'…' word from just inside its quote, writing what its
// escapes stand for, and returns where the word ends.
func ansiC(s string, i int, b *strings.Builder) (int, error) {
	for i < len(s) {
		c := s[i]
		if c == '\'' {
			return i + 1, nil
		}
		if c != '\\' || i+1 >= len(s) {
			b.WriteByte(c)
			i++
			continue
		}
		e := s[i+1]
		i += 2
		switch e {
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		case 'a':
			b.WriteByte(7)
		case 'b':
			b.WriteByte(8)
		case 'e', 'E':
			b.WriteByte(27)
		case 'f':
			b.WriteByte(12)
		case 'v':
			b.WriteByte(11)
		case '\\', '\'', '"', '?':
			b.WriteByte(e)
		case 'x':
			n, used := hexValue(s[i:], 2)
			b.WriteByte(byte(n))
			i += used
		case 'u', 'U':
			width := 4
			if e == 'U' {
				width = 8
			}
			n, used := hexValue(s[i:], width)
			var buf [utf8.UTFMax]byte
			b.Write(buf[:utf8.EncodeRune(buf[:], rune(n))])
			i += used
		case '0', '1', '2', '3', '4', '5', '6', '7':
			end := i - 1
			for end < len(s) && end < i+2 && s[end] >= '0' && s[end] <= '7' {
				end++
			}
			n, _ := strconv.ParseUint(s[i-1:end], 8, 8)
			b.WriteByte(byte(n))
			i = end
		default:
			b.WriteByte('\\')
			b.WriteByte(e)
		}
	}
	return 0, errUnclosed
}

// hexValue reads up to width hex digits from the start of s.
func hexValue(s string, width int) (uint64, int) {
	n := 0
	for n < len(s) && n < width && strings.IndexByte("0123456789abcdefABCDEF", s[n]) >= 0 {
		n++
	}
	v, _ := strconv.ParseUint(s[:n], 16, 32)
	return v, n
}
