package inventory_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// Nothing in this package may touch binary floating point.
//
// The behaviour tests can only check the arithmetic they happen to exercise. A
// float is a property of the source: one `float64` anywhere on the valuation
// path puts a representation error into every figure that passes through it, and
// the test that would have caught it is the one nobody thought to write. So the
// package is read and asked directly.
func TestPackage_UsesNoFloatingPoint(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("list the package source: %v", err)
	}

	scanned := 0

	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}

		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		scanned++

		ast.Inspect(file, func(node ast.Node) bool {
			switch n := node.(type) {
			case *ast.Ident:
				if n.Name == "float32" || n.Name == "float64" {
					t.Errorf("%s uses %s; money is whole minor units and quantities are integers", path, n.Name)
				}
			case *ast.BasicLit:
				if n.Kind == token.FLOAT {
					t.Errorf("%s holds the floating-point literal %s", path, n.Value)
				}
			}

			return true
		})
	}

	if scanned == 0 {
		t.Fatal("no source files were scanned, so this test proves nothing")
	}
}
