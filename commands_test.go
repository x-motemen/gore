package gore

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAction_Type(t *testing.T) {
	var stdout, stderr strings.Builder
	s, err := NewSession(&stdout, &stderr)
	t.Cleanup(func() { s.Clear() })
	require.NoError(t, err)

	codes := []string{
		`:type "hello"`,
		":type 128",
		":type 3.14",
		"func f() []int { return nil }",
		":t f",
		":t f()",
		":i fmt encoding/json",
		":t fmt.Sprint",
		":t fmt.Println",
		":t json.NewEncoder",
		":t x",
		":t fmt",
	}

	for _, code := range codes {
		_ = s.Eval(code)
	}

	assert.Equal(t, `string
int
float64
func() []int
[]int
func(a ...any) string
func(a ...any) (n int, err error)
func(w io.Writer) *encoding/json.Encoder
`, stdout.String())
	assert.Equal(t, `type: cannot get type: x
type: cannot get type: fmt
`, stderr.String())
}

func TestAction_Doc(t *testing.T) {
	var stdout, stderr strings.Builder
	s, err := NewSession(&stdout, &stderr)
	t.Cleanup(func() { s.Clear() })
	require.NoError(t, err)

	err = s.Eval(":import encoding/json")
	require.NoError(t, err)
	err = s.Eval(":i fmt")
	require.NoError(t, err)

	test := func() {
		err = s.Eval(":doc fmt")
		require.NoError(t, err)

		err = s.Eval(":doc fmt.Print")
		require.NoError(t, err)

		err = s.Eval(":d json.NewEncoder(nil).Encode")
		require.NoError(t, err)
	}

	test()

	// test :doc works after some code

	err = s.Eval("a := 1")
	require.NoError(t, err)

	err = s.Eval("fmt.Print()")
	require.NoError(t, err)

	test()

	assert.Contains(t, stdout.String(), "package fmt")
	assert.Contains(t, stdout.String(), "func Printf")
	assert.Equal(t, "", stderr.String())
}

func TestAction_Import(t *testing.T) {
	var stdout, stderr strings.Builder
	s, err := NewSession(&stdout, &stderr)
	t.Cleanup(func() { s.Clear() })
	require.NoError(t, err)

	err = s.Eval(":import encoding/json fmt")
	require.NoError(t, err)

	err = s.Eval("fmt.Print")
	require.NoError(t, err)

	err = s.Eval("json.Encoder{}")
	require.NoError(t, err)

	assert.Contains(t, stdout.String(), "(func(...interface {}) (int, error))")
	assert.Contains(t, stdout.String(), "json.Encoder")
	assert.Equal(t, "", stderr.String())

	err = s.Eval(":import invalid")
	require.Error(t, err)

	err = s.Eval("fmt.Sprint")
	require.NoError(t, err)
	assert.Equal(t, "import: could not import \"invalid\"\n", stderr.String())
}

func TestAction_ImportAlias(t *testing.T) {
	var stdout, stderr strings.Builder
	s, err := NewSession(&stdout, &stderr)
	t.Cleanup(func() { s.Clear() })
	require.NoError(t, err)

	// An aliased import can be referenced by its alias.
	err = s.Eval(":import js encoding/json")
	require.NoError(t, err)
	err = s.Eval("js.Marshal(1)")
	require.NoError(t, err)

	source, err := s.source(false)
	require.NoError(t, err)
	assert.Contains(t, source, `js "encoding/json"`)

	// Two standard packages are still imported as two packages, not aliased.
	err = s.Eval(":import strings strconv")
	require.NoError(t, err)
	err = s.Eval(`strconv.Itoa(len(strings.Fields("a b")))`)
	require.NoError(t, err)
	source, err = s.source(false)
	require.NoError(t, err)
	assert.Contains(t, source, `"strings"`)
	assert.Contains(t, source, `"strconv"`)

	// Packages sharing a name can be imported together with aliases, and an
	// alias survives evaluations that do not reference it.
	err = s.Eval(":import tt text/template ht html/template")
	require.NoError(t, err)
	err = s.Eval(`tt.New("x")`)
	require.NoError(t, err)
	err = s.Eval(`ht.New("y")`)
	require.NoError(t, err)
	assert.Equal(t, "", stderr.String())
}

func TestAction_Print(t *testing.T) {
	var stdout, stderr strings.Builder
	s, err := NewSession(&stdout, &stderr)
	t.Cleanup(func() { s.Clear() })
	require.NoError(t, err)

	err = s.Eval("x := 1")
	require.NoError(t, err)

	err = s.Eval(":print")
	require.NoError(t, err)

	out := stdout.String()
	assert.Contains(t, out, "package main")
	assert.Contains(t, out, "func main()")
	assert.Contains(t, out, "x := 1")
	assert.Equal(t, "", stderr.String())
}

func TestAction_Clear(t *testing.T) {
	var stdout, stderr strings.Builder
	s, err := NewSession(&stdout, &stderr)
	t.Cleanup(func() { s.Clear() })
	require.NoError(t, err)

	codes := []string{
		`x := 10`,
		`x`,
		`:clear`,
		`x := "foo"`,
		`x`,
		`:clear`,
		`x`,
	}

	for _, code := range codes {
		_ = s.Eval(code)
	}

	assert.Equal(t, `10
10
"foo"
"foo"
`, stdout.String())
	assert.Equal(t, "undefined: x\n", stderr.String())
}

func TestAction_Help(t *testing.T) {
	var stdout, stderr strings.Builder
	s, err := NewSession(&stdout, &stderr)
	t.Cleanup(func() { s.Clear() })
	require.NoError(t, err)

	err = s.Eval(": :  :   help  ")
	require.NoError(t, err)

	assert.Contains(t, stdout.String(), ":import [<alias>] <package>")
	assert.Contains(t, stdout.String(), ":write [<file>]")
	assert.Contains(t, stdout.String(), "show this help")
	assert.Contains(t, stdout.String(), "quit the session")
	assert.Equal(t, "", stderr.String())

	err = s.Eval(":h")
	require.NoError(t, err)
}

func TestAction_Quit(t *testing.T) {
	var stdout, stderr strings.Builder
	s, err := NewSession(&stdout, &stderr)
	t.Cleanup(func() { s.Clear() })
	require.NoError(t, err)

	err = s.Eval(" :\t: quit")
	require.Equal(t, ErrQuit, err)

	assert.Equal(t, "", stdout.String())
	assert.Equal(t, "", stderr.String())

	err = s.Eval(":q")
	require.Equal(t, ErrQuit, err)
}

func TestAction_CommandNotFound(t *testing.T) {
	var stdout, stderr strings.Builder
	s, err := NewSession(&stdout, &stderr)
	t.Cleanup(func() { s.Clear() })
	require.NoError(t, err)

	err = s.Eval(":::")
	require.NoError(t, err)

	err = s.Eval(":foo")
	require.Error(t, err)

	err = s.Eval(":ii")
	require.Error(t, err)

	err = s.Eval(":docc")
	require.Error(t, err)

	err = s.Eval(":help]")
	require.Error(t, err)

	assert.Equal(t, "", stdout.String())
	assert.Equal(t, `command not found: foo
command not found: ii
command not found: docc
command not found: help]
`, stderr.String())
}

func TestAction_ArgumentRequired(t *testing.T) {
	var stdout, stderr strings.Builder
	s, err := NewSession(&stdout, &stderr)
	t.Cleanup(func() { s.Clear() })
	require.NoError(t, err)

	err = s.Eval(":import")
	require.Error(t, err)

	err = s.Eval(":type")
	require.Error(t, err)

	err = s.Eval(":doc")
	require.Error(t, err)

	assert.Equal(t, "", stdout.String())
	assert.Equal(t, `import: argument is required
type: argument is required
doc: argument is required
`, stderr.String())
}
