package briefing

import (
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"strings"
	"testing"
)

// TestRenderCLI_StatusOrder_Fixed is the structural guard against regression
// to map iteration. It asserts:
// (a) ModuleStatusOrder slice contents match expected canonical order,
// (b) render_cli.go source references ModuleStatusOrder (declaration + use),
// (c) render_cli.go source does not use `range payload.Status` directly.
//
// AC-017 / REQ-BR-063.
func TestRenderCLI_StatusOrder_Fixed(t *testing.T) {
	// (a) Slice contents assertion
	want := []string{"weather", "journal", "date", "mantra"}
	if !reflect.DeepEqual(ModuleStatusOrder, want) {
		t.Fatalf("ModuleStatusOrder = %v, want %v", ModuleStatusOrder, want)
	}

	// (b) + (c) Source-level assertions via AST and string search
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "render_cli.go", nil, parser.AllErrors)
	if err != nil {
		t.Fatalf("parse render_cli.go: %v", err)
	}

	var references int
	var rangeOnStatus int

	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.Ident:
			if x.Name == "ModuleStatusOrder" {
				references++
			}
		case *ast.RangeStmt:
			// Detect `for _, x := range payload.Status`
			if sel, ok := x.X.(*ast.SelectorExpr); ok {
				if sel.Sel != nil && sel.Sel.Name == "Status" {
					rangeOnStatus++
				}
			}
		}
		return true
	})

	// (b) ModuleStatusOrder must appear at least twice (declaration + use site)
	if references < 2 {
		t.Errorf("ModuleStatusOrder must be referenced in render_cli.go (decl + use), got %d reference(s)", references)
	}

	// (c) Direct range over payload.Status is forbidden in render_cli.go
	if rangeOnStatus > 0 {
		t.Errorf("render_cli.go must not range payload.Status directly; found %d such range statement(s)", rangeOnStatus)
	}

	_ = strings.Join // keep import alive for potential future expansion
}
