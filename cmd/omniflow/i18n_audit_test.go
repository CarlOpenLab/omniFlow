package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// uiStringArg lists the argument positions, in the order the call passes
// them, that hold the words a person reads. A function missing from the map
// is not scanned.
var uiStringArg = map[string][]int{
	"Text":          {1},
	"Button":        {1},
	"PrimaryButton": {1},
	"Label":         {0},
	"Tooltip":       {0},
	"Item":          {0},
	"iconButton":    {3},
	"iconAction":    {3},
	"quietButton":   {3},
	"emptyState":    {3, 4},
	"sectionTitle":  {2},
	"settingsLabel": {2},
	"settingSwitch": {2},
	"setNotice":     {0},
	"setUndo":       {0},
}

// unlocalizedUIStrings are the literals that are not copy: technical markers,
// brand and addresses a person reads as data, not as a sentence. An empty
// label is not copy either: the widget draws its children instead.
var unlocalizedUIStrings = []string{
	"",
	"OmniFlow",
	"127.0.0.1",
	"—",
	"JSON",
}

// TestNoUnlocalizedUIStrings fails while the interface still carries a
// literal where a catalog message belongs, so the audit cannot be skipped by
// accident.
func TestNoUnlocalizedUIStrings(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("list package files: %v", err)
	}
	var findings []string
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, raw, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			name := calleeName(call.Fun)
			for _, i := range uiStringArg[name] {
				if i >= len(call.Args) {
					continue
				}
				lit, ok := call.Args[i].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				value, err := strconv.Unquote(lit.Value)
				if err != nil || slices.Contains(unlocalizedUIStrings, value) {
					continue
				}
				pos := fset.Position(lit.Pos())
				findings = append(findings, fmt.Sprintf("%s:%d: %s(%q)", filepath.Base(path), pos.Line, name, value))
			}
			return true
		})
	}
	sort.Strings(findings)
	if len(findings) > 0 {
		t.Fatalf("interface strings not taken from the catalogs:\n%s", strings.Join(findings, "\n"))
	}
}

// calleeName is the name a call goes by: Text for ui.Text and for a Text
// chosen any other way, Label for a .Label method, iconButton for itself.
func calleeName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return e.Sel.Name
	}
	return ""
}
