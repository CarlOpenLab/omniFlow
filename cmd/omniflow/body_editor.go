package main

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"

	"github.com/CarlOpenLab/omniflow/internal/uitokens"
	"github.com/CarlOpenLab/omniflow/locales"
)

// The body's box: its padding and line height.
const (
	codePadTop     = 10
	codePadLeft    = 14
	codeLineHeight = 18
)

// canFind reports whether the body on show is text the find can search: in
// the composer, the response's body; in the inspector, the chosen flow's
// request or response body.
func (a *app) canFind() bool {
	switch a.mode {
	case modeComposer:
		if a.resp == nil || a.respTab != 0 || len(a.resp.Body) == 0 {
			return false
		}
		return shapeOf(a.resp.Body, contentTypeOf(a.resp.Headers)) == shapeText
	case modeInspector:
		f := a.selectedFlow()
		if f == nil || a.detailSub != 0 {
			return false
		}
		body, contentType := f.RespBody, f.ContentType
		if a.detailTab == 0 {
			body, contentType = f.ReqBody, headerValue(f.ReqHeaders, "Content-Type")
		}
		return len(body) > 0 && shapeOf(body, contentType) == shapeText
	}
	return false
}

// bodyEditor is the code editor showing the text body on show, and the key
// of the body it holds: a new body gets a new editor, while the same one
// keeps its place, its selection and its find.
type bodyEditor struct {
	key string
	st  *editor.State
}

// state returns the editor of key's text, making it when the key changed.
func (b *bodyEditor) state(key, text string, isJSON bool, pal *uitokens.Palette) *editor.State {
	if b.st != nil && b.key == key {
		return b.st
	}
	st := &editor.State{
		Buffer:          editor.NewBuffer(text),
		ReadOnly:        true,
		Font:            ui.Font{Family: "monospace", Size: 11.5},
		LineHeight:      codeLineHeight / 11.5,
		TabSize:         2,
		LineNumbers:     editor.LineNumbersOff,
		NoFolding:       true,
		NoActiveLine:    true,
		NoIndentGuides:  true,
		NoScrollPastEnd: true,
		SoftWrap:        false,
		Label:           locales.ComposerTabBody(),
		Theme:           bodyEditorTheme(pal),
	}
	if isJSON {
		st.Buffer.SetLanguage(editor.LanguageFor("body.json"))
	}
	b.key, b.st = key, st
	return st
}

// bodyEditorTheme colors the body's editor as the app's code view: its
// paper and text, and its JSON colors.
func bodyEditorTheme(pal *uitokens.Palette) *editor.Theme {
	th := &editor.Theme{
		Background:        pal.CodeBg,
		Text:              pal.CodeText,
		Selection:         pal.Selection,
		InactiveSelection: pal.Selection.Alpha(0.55),
		Cursor:            pal.Accent,
		Match:             pal.Warning.Alpha(0.22),
		CurrentMatch:      pal.Warning.Alpha(0.5),
		// A body is read, not edited: the bracket around the resting caret
		// would ring the body's first character as a control would.
		MatchingBracket: pal.CodeBg.Alpha(0.001),
	}
	th.Tokens[editor.TokenProperty] = editor.TokenStyle{Color: pal.CodeKey}
	th.Tokens[editor.TokenString] = editor.TokenStyle{Color: pal.CodeString}
	th.Tokens[editor.TokenNumber] = editor.TokenStyle{Color: pal.CodeNumber}
	th.Tokens[editor.TokenConstant] = editor.TokenStyle{Color: pal.CodeLiteral}
	th.Tokens[editor.TokenKeyword] = editor.TokenStyle{Color: pal.CodeLiteral}
	th.Tokens[editor.TokenPunctuation] = editor.TokenStyle{Color: pal.CodePunct}
	th.Tokens[editor.TokenOperator] = editor.TokenStyle{Color: pal.CodePunct}
	return th
}

// wrapToggle is the body's soft-wrap switch: long lines fold at the width
// instead of running off to the right.
func wrapToggle(c *ui.Context, pal *uitokens.Palette, on *bool) ui.Element {
	b := ui.ButtonBase(c.Key("wrap")).Height(24).Padding(0, 8).Radius(6).Gap(4).
		Label(locales.ComposerWrap()).Tooltip(locales.ComposerWrapHint())
	if *on {
		b.Background(pal.AccentSoft)
	} else if b.Hovered() {
		b.Background(pal.RowHover)
	}
	b.Children(func() {
		col := pal.TextMuted
		if *on {
			col = pal.Accent
		}
		ui.Icon(c, iconWrap).Size(11, 11).TextColor(col)
		ui.Text(c, locales.ComposerWrap()).FontSize(10.5).FontWeight(600).TextColor(col)
	})
	if b.Clicked() {
		*on = !*on
	}
	return b
}

// codeView shows text in the body's virtualized editor: only the lines in
// view are laid out, so a body of any size scrolls, selects and searches as
// one of a few. note, when there is one, says why the body ends early.
func (a *app) codeView(c *ui.Context, pal *uitokens.Palette, key, text string, isJSON bool, note string) ui.Element {
	col := ui.Column(c).Grow(1).MinHeight(0).Background(pal.CodeBg)
	col.Children(func() {
		if note != "" {
			ui.Row(c).Height(24).Shrink(0).Padding(0, codePadLeft).Children(func() {
				ui.Text(c, note).FontSize(10.5).FontWeight(600).TextColor(pal.Warning)
			})
		}
		if strings.TrimSpace(text) == "" {
			ui.Text(c, locales.CommonEmptyBody()).Font("monospace").FontSize(11.5).
				TextColor(pal.TextFaint).Padding(codePadTop, codePadLeft)
			return
		}
		st := a.body.state(key, text, isJSON, pal)
		// The editor reads this each frame, so the switch folds the body
		// where it stands rather than rebuilding it and losing its place.
		st.SoftWrap = a.wrapBody
		if a.findBody {
			st.Run(editor.Find)
			a.findBody = false
		}
		editor.View(c, st).Grow(1).MinHeight(0)
	})
	return col
}
