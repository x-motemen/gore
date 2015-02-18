package main

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"

	"testing"
)

func TestNormalizeNodePos(t *testing.T) {
	src := `package P

import "fmt"

func F() {
	fmt.
		Println(
		1,
	)
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "t.go", src, 0)
	noError(t, err)

	if formatted := showNode(fset, f); formatted != src {
		t.Fatalf("formatted source must equal original: %s", formatted)
	}

	normalizeNodePos(f)

	formatted := showNode(fset, f)
	if formatted == src {
		t.Fatalf("formatted source must differ from original after normalizeNode: %s", formatted)
	}

	t.Log(formatted)
}

func showNode(fset *token.FileSet, node ast.Node) string {
	var buf bytes.Buffer
	printer.Fprint(&buf, fset, node)
	return buf.String()
}
