package main

import (
	"go/parser"
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

	normalizeNodePos(f)

	formatted := showNode(fset, f)
	if formatted == src {
		t.Fatalf("formatted source must differ from original after normalizeNode: %s", formatted)
	}

	t.Log(formatted)
}
