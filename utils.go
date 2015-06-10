package main

import (
	"bytes"

	"go/printer"
	"go/token"
)

func showNode(fset *token.FileSet, node interface{}) string {
	var buf bytes.Buffer
	printer.Fprint(&buf, fset, node)
	return buf.String()
}
