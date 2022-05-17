package gore

import (
	"go/printer"
	"go/token"
	"strings"
)

func showNode(fset *token.FileSet, node interface{}) string {
	var sb strings.Builder
	printer.Fprint(&sb, fset, node)
	return sb.String()
}
