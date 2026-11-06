package main

// The acrylic rules, checked statically: nothing drawn on the window's
// material — the sidebar, the workbench card — may fill with an opaque layer
// color or use a framework control that paints one. Such a fill looks right
// at 100% and shows as a solid patch over the frost the moment the card is
// turned down. Dialogs and the code canvas are opaque by design and are
// listed, each with its reason; adding a line to that list is meant to cost
// an explanation.

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"strings"
	"testing"
)

// paletteVar is the name the palette has in this package's views.
const paletteVar = "pal"

// opaqueFields are the palette's opaque layer colors.
var opaqueFields = map[string]bool{
	"Ground": true, "Card": true, "Popover": true, "CodeBg": true,
	"Control": true, "ControlHover": true,
}

// skinnedWidgets are framework controls that paint an opaque face of their
// own, or that our theme leaves without a visible line.
var skinnedWidgets = map[string]string{
	"ui.Checkbox": "use washCheckbox: the framework's box paints the theme's background, a solid square over the frost",
	"ui.Radio":    "write a wash radio like washCheckbox: the framework's dot paints the theme's background",
}

// acrylicOwners says where an opaque fill is the point: a dialog on its
// solid Popover, or a code canvas that must keep its contrast. The key is a
// file, or a file and the one function in it.
var acrylicOwners = map[string]string{
	"body.go:imageView":                  "an image body is read on the code canvas",
	"body.go:binaryView":                 "a binary body is read on the code canvas",
	"body_editor.go:codeView":            "bodies are read on the code canvas",
	"body_editor.go:bodyEditorTheme":     "the body editor's paper is the code canvas",
	"composer.go:methodMenu":             "the verb list is a popover, drawn solid by design",
	"composer.go:bodyTab":                "the JSON and text editor is a code canvas, so code keeps its contrast",
	"env.go:envDialog":                   "the environment manager is a dialog on its solid Popover",
	"settings.go:settingsDialog":         "settings is a dialog on its solid Popover",
	"settings.go:settingsAppearancePage": "a page of the settings dialog, whose card and preview sit on its solid Popover",
	"settings.go:settingsCapturePage":    "a page of the settings dialog, whose fields are drawn on its solid Popover",
	"settings.go:caDialog":               "the certificate dialog is a dialog on its solid Popover",
	"widgets.go:applyTheme":              "the faces the framework's own dialog controls read",
	"widgets.go:keyValueList":            "headers are read on the code canvas",
}

func TestAcrylicLayersUseWashes(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	used := map[string]bool{}
	for _, e := range entries {
		path := e.Name()
		if e.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			continue
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			owner := ""
			for _, key := range []string{path + ":" + fn.Name.Name, path} {
				if _, ok := acrylicOwners[key]; ok {
					owner = key
					break
				}
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				var what, fix string
				switch n := n.(type) {
				case *ast.SelectorExpr:
					x, ok := n.X.(*ast.Ident)
					if !ok || x.Name != paletteVar || !opaqueFields[n.Sel.Name] {
						return true
					}
					what = paletteVar + "." + n.Sel.Name
					fix = "panels and strips take no fill; controls rest on WashChip (ghost controls on nothing), hover RowHover, pressed and chosen WashSelected"
				case *ast.CallExpr:
					var b bytes.Buffer
					_ = printer.Fprint(&b, fset, n.Fun)
					hint, ok := skinnedWidgets[b.String()]
					if !ok {
						return true
					}
					what, fix = b.String(), hint
				default:
					return true
				}
				if owner != "" {
					used[owner] = true
					return true
				}
				t.Errorf("%s: %s fills or draws with %s over the material; %s."+
					" If it really is a dialog or a code canvas, add it to acrylicOwners with the reason",
					fset.Position(n.Pos()), fn.Name.Name, what, fix)
				return true
			})
		}
	}
	for key := range acrylicOwners {
		if !used[key] {
			t.Errorf("acrylicOwners[%q] no longer covers an opaque fill — remove the entry", key)
		}
	}
}
