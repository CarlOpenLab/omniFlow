package main

import (
	"bytes"
	"mime"
	"os"
	"path"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/CarlOpenLab/omniflow/internal/httpx"
	"github.com/CarlOpenLab/omniflow/internal/uitokens"
	"github.com/CarlOpenLab/omniflow/locales"
)

// bodyShape is how a body is shown: read as text, looked at as a picture,
// or kept as bytes to save.
type bodyShape int

const (
	shapeText bodyShape = iota
	shapeImage
	shapeBinary
)

// shapeOf decides from the body's type and its first bytes. A type that
// names text wins over the bytes, so a page in Latin-1 still reads as one;
// an image is only a picture if it decodes, which the view finds out.
func shapeOf(body []byte, contentType string) bodyShape {
	ct := strings.ToLower(contentType)
	switch {
	case len(body) == 0:
		return shapeText
	case strings.HasPrefix(ct, "image/") && !strings.Contains(ct, "svg"):
		return shapeImage
	case textual(ct) && bytes.IndexByte(body[:min(len(body), sniffLen)], 0) < 0:
		return shapeText
	case looksBinary(body):
		return shapeBinary
	}
	return shapeText
}

// sniffLen is how much of a body is read to tell text from bytes.
const sniffLen = 8 << 10

func textual(ct string) bool {
	if strings.HasPrefix(ct, "text/") {
		return true
	}
	for _, s := range []string{"json", "xml", "javascript", "x-www-form-urlencoded", "yaml", "graphql"} {
		if strings.Contains(ct, s) {
			return true
		}
	}
	return false
}

// looksBinary reports whether the start of a body is not UTF-8 text.
func looksBinary(body []byte) bool {
	sample := body[:min(len(body), sniffLen)]
	if bytes.IndexByte(sample, 0) >= 0 {
		return true
	}
	if utf8.Valid(sample) {
		return false
	}
	if len(body) > len(sample) {
		// The cut may have fallen inside a character.
		for i := 1; i <= 3 && i < len(sample); i++ {
			if utf8.Valid(sample[:len(sample)-i]) {
				return false
			}
		}
	}
	return true
}

// bodyPreview is a body decoded as a picture, kept while the same body is on
// screen: it is decoded once, not in every frame, and the toolkit keeps its
// texture only while it shows.
type bodyPreview struct {
	key string
	bmp *ui.Bitmap
	err error
}

func (a *app) previewImage(key string, body []byte) (*ui.Bitmap, error) {
	if a.preview.key != key {
		bmp, err := ui.DecodeBitmap(body)
		a.preview = bodyPreview{key: key, bmp: bmp, err: err}
	}
	return a.preview.bmp, a.preview.err
}

// bodyView shows a body in the shape it has: highlighted text, a picture
// with its size, or a note that it is bytes, with a way to save them. key
// names the body for the picture's cache; name is the file it saves as.
func (a *app) bodyView(c *ui.Context, pal *uitokens.Palette, key, name string, body []byte, contentType, text, note string) {
	switch shapeOf(body, contentType) {
	case shapeImage:
		if bmp, err := a.previewImage(key, body); err == nil {
			a.imageView(c, pal, bmp, body, contentType, name, note)
			return
		}
		a.binaryView(c, pal, body, contentType, name, note)
	case shapeBinary:
		a.binaryView(c, pal, body, contentType, name, note)
	default:
		codeView(c, pal, text, note)
	}
}

func (a *app) imageView(c *ui.Context, pal *uitokens.Palette, bmp *ui.Bitmap, body []byte, contentType, name, note string) {
	w, h := bmp.Size()
	ui.Column(c).Fill().MinHeight(0).Background(pal.CodeBg).Children(func() {
		ui.Box(c).Grow(1).MinHeight(0).Padding(16).Center().Children(func() {
			ui.Image(c, bmp).Fill().Fit(ui.ScaleDown).Label(locales.BodyImageLabel())
		})
		a.bodyFoot(c, pal, locales.BodyImageMeta(shortContentType(contentType), w, h,
			httpx.FormatBytes(int64(len(body)))), body, contentType, name, note)
	})
}

func (a *app) binaryView(c *ui.Context, pal *uitokens.Palette, body []byte, contentType, name, note string) {
	ui.Column(c).Fill().MinHeight(0).Background(pal.CodeBg).Children(func() {
		ui.Box(c).Grow(1).MinHeight(0).Children(func() {
			kind := shortContentType(contentType)
			if kind == "" {
				kind = locales.BodyBinaryData()
			}
			emptyState(c, pal, iconFile, locales.BodyBinaryTitle(),
				locales.BodyBinaryHint(kind, httpx.FormatBytes(int64(len(body)))))
		})
		a.bodyFoot(c, pal, "", body, contentType, name, note)
	})
}

// bodyFoot is the line under a picture or bytes: what they are, a note when
// the body was cut, and Save.
func (a *app) bodyFoot(c *ui.Context, pal *uitokens.Palette, caption string, body []byte, contentType, name, note string) {
	ui.Row(c).Height(36).Shrink(0).Padding(0, 12).Gap(8).BorderWidth(1, 0, 0, 0).
		BorderColor(pal.CodeEdge).Children(func() {
		ui.Text(c, caption).FontSize(10.5).TextColor(pal.TextMuted).SingleLine().Ellipsis("…").
			Grow(1).MinWidth(0)
		if note != "" {
			ui.Text(c, note).FontSize(10.5).TextColor(pal.Warning).SingleLine().Ellipsis("…").MinWidth(0)
		}
		if b := iconAction(c, pal, iconSave, locales.BodySave()); b.Clicked() {
			a.saveBody(body, fileNameFor(name, contentType))
		}
	})
}

// fileNameFor is the name a saved body is offered under: the last part of
// the URL's path, given the extension its type has when it has none.
func fileNameFor(rawURL, contentType string) string {
	p := rawURL
	if i := strings.IndexAny(p, "?#"); i >= 0 {
		p = p[:i]
	}
	if i := strings.Index(p, "://"); i >= 0 {
		p = p[i+3:]
	}
	name := ""
	if i := strings.IndexByte(p, '/'); i >= 0 {
		name = path.Base(p[i:])
	}
	if name == "" || name == "/" || name == "." {
		name = "body"
	}
	if path.Ext(name) == "" {
		name += extensionFor(contentType)
	}
	return name
}

// commonExtensions are the extensions people expect for the usual types;
// the mime package lists all of a type's in order, so JPEG's first is .jfif.
var commonExtensions = map[string]string{
	"application/json":         ".json",
	"application/octet-stream": ".bin",
	"application/pdf":          ".pdf",
	"application/zip":          ".zip",
	"image/gif":                ".gif",
	"image/jpeg":               ".jpg",
	"image/png":                ".png",
	"image/webp":               ".webp",
	"text/html":                ".html",
	"text/plain":               ".txt",
}

func extensionFor(contentType string) string {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return ""
	}
	if ext, ok := commonExtensions[mediaType]; ok {
		return ext
	}
	if exts, _ := mime.ExtensionsByType(mediaType); len(exts) > 0 {
		return exts[0]
	}
	return ""
}

// saveBody asks where to write a body and writes it there.
func (a *app) saveBody(body []byte, name string) {
	data := bytes.Clone(body)
	go func() {
		dest, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
			Parent:      a.win,
			Title:       locales.BodySaveDialogTitle(),
			DefaultPath: name,
			ButtonLabel: locales.CommonSave(),
		})
		if err != nil {
			a.update(func() { a.setNotice(locales.BodySaveFailed(err.Error())) })
			return
		}
		if dest == "" {
			return
		}
		if err := os.WriteFile(dest, data, 0o644); err != nil {
			a.update(func() { a.setNotice(locales.BodySaveFailed(err.Error())) })
			return
		}
		a.update(func() { a.setNotice(locales.BodySaved(dest)) })
	}()
}
