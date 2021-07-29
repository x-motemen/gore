package gore

import (
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/require"
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
	require.NoError(t, err)

	normalizeNodePos(f)

	formatted := showNode(fset, f)
	if formatted == src {
		t.Fatalf("formatted source must differ from original after normalizeNode: %s", formatted)
	}
}
