package gore

import (
	"bytes"
	"os"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	printerPkgs = printerPkgs[1:]
}

func TestSessionEval_import(t *testing.T) {
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	s, err := NewSession(stdout, stderr)
	defer s.Clear()
	require.NoError(t, err)

	codes := []string{
		":import encoding/json",
		"b, err := json.Marshal(nil)",
		"string(b)",
	}

	for _, code := range codes {
		err := s.Eval(code)
		require.NoError(t, err)
	}

	assert.Equal(t, `[]byte{0x6e, 0x75, 0x6c, 0x6c}
<nil>
"null"
`, stdout.String())
	assert.Equal(t, "", stderr.String())
}

func TestSessionEval_QuickFix_evaluated_but_not_used(t *testing.T) {
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	s, err := NewSession(stdout, stderr)
	defer s.Clear()
	require.NoError(t, err)

	codes := []string{
		`[]byte("")`,
		`make([]int, 0)`,
		`1+1`,
		`func() {}`,
		`(4 & (1 << 1))`,
		`1`,
	}

	for _, code := range codes {
		err := s.Eval(code)
		require.NoError(t, err)
	}

	r := regexp.MustCompile(`0x[0-9a-f]+`)
	assert.Equal(t, `[]byte{}
[]int{}
2
(func())(...)
0
1
`, r.ReplaceAllString(stdout.String(), "..."))
	assert.Equal(t, "", stderr.String())
}

func TestSessionEval_QuickFix_used_as_value(t *testing.T) {
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	s, err := NewSession(stdout, stderr)
	defer s.Clear()
	require.NoError(t, err)

	codes := []string{
		`:import log`,
		`a := 1`,
		`log.SetPrefix("")`,
	}

	for _, code := range codes {
		err := s.Eval(code)
		require.NoError(t, err)
	}

	assert.Equal(t, "1\n", stdout.String())
	assert.Equal(t, "", stderr.String())
}

func TestSessionEval_QuickFix_no_new_variables(t *testing.T) {
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	s, err := NewSession(stdout, stderr)
	defer s.Clear()
	require.NoError(t, err)

	codes := []string{
		`var a, b int`,
		`a := 2`,
		`b := a * 2`,
		`a := 3`,
		`c := a * b`,
		`c := b * c`,
		`b := c * a`,
		`a * b * c`,
	}

	for _, code := range codes {
		err := s.Eval(code)
		require.NoError(t, err)
	}

	assert.Equal(t, `2
4
3
12
48
144
20736
`, stdout.String())
	assert.Equal(t, "", stderr.String())
}

func TestSessionEval_AutoImport(t *testing.T) {
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	s, err := NewSession(stdout, stderr)
	defer s.Clear()
	require.NoError(t, err)
	s.autoImport = true

	codes := []string{
		`filepath.Join("a", "b")`,
	}

	for _, code := range codes {
		err := s.Eval(code)
		require.NoError(t, err)
	}

	assert.Equal(t, "\"a/b\"\n", stdout.String())
	assert.Equal(t, "", stderr.String())
}

func TestSession_IncludePackage(t *testing.T) {
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	s, err := NewSession(stdout, stderr)
	defer s.Clear()
	require.NoError(t, err)

	err = s.includePackage("github.com/x-motemen/gore/gocode")
	require.NoError(t, err)

	err = s.Eval("Completer{}")
	require.NoError(t, err)
}

func TestSessionEval_Copy(t *testing.T) {
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	s, err := NewSession(stdout, stderr)
	defer s.Clear()
	require.NoError(t, err)

	codes := []string{
		`a := []string{"hello", "world"}`,
		`b := []string{"goodbye", "world"}`,
		`copy(a, b)`,
		`a[0]`,
	}

	for _, code := range codes {
		err := s.Eval(code)
		require.NoError(t, err)
	}

	assert.Equal(t, `[]string{"hello", "world"}
[]string{"goodbye", "world"}
2
"goodbye"
`, stdout.String())
	assert.Equal(t, "", stderr.String())
}

func TestSessionEval_Const(t *testing.T) {
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	s, err := NewSession(stdout, stderr)
	defer s.Clear()
	require.NoError(t, err)

	codes := []string{
		`const ( a = iota; b )`,
		`a`,
		`b`,
	}

	for _, code := range codes {
		err := s.Eval(code)
		require.NoError(t, err)
	}

	assert.Equal(t, "0\n1\n0\n1\n", stdout.String())
	assert.Equal(t, "", stderr.String())
}

func TestSessionEval_Declarations(t *testing.T) {
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	s, err := NewSession(stdout, stderr)
	defer s.Clear()
	require.NoError(t, err)

	codes := []string{
		`var a, b int = 10, 20`,
		`var (x int = 10; y string = "hello"; z uint64; w error; v func(string)int)`,
	}

	for _, code := range codes {
		err := s.Eval(code)
		require.NoError(t, err)
	}

	assert.Equal(t, `10
20
10
"hello"
0x0
<nil>
(func(string) int)(nil)
`, stdout.String())
	assert.Equal(t, "", stderr.String())
}

func TestSessionEval_NotUsed(t *testing.T) {
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	s, err := NewSession(stdout, stderr)
	defer s.Clear()
	require.NoError(t, err)

	codes := []string{
		`f := func() []int { return []int{1, 2, 3} }`,
		`len(f())`,
		`3`,
		`len(f()) + len(f())`,
		`var x int`,
		`g := func() int { x++; return 128 }`,
		`g() + g()`,
		`g() * g()`,
		`x`,
	}

	for _, code := range codes {
		_ = s.Eval(code)
	}

	r := regexp.MustCompile(`0x[0-9a-f]+`)
	assert.Equal(t, `(func() []int)(...)
3
3
6
(func() int)(...)
256
16384
4
`, r.ReplaceAllString(stdout.String(), "..."))
	assert.Equal(t, "", stderr.String())
}

func TestSessionEval_MultipleValues(t *testing.T) {
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	s, err := NewSession(stdout, stderr)
	defer s.Clear()
	require.NoError(t, err)

	codes := []string{
		`var err error`,
		`:import fmt`,
		`fmt.Print()`,
		`fmt.Print()`,
		`:import io`,
		`_, err = func() (int, error) { return 0, io.EOF }()`,
		`err.Error()`,
		`var x int`,
		`x, err = 10, fmt.Errorf("test")`,
		`x`,
		`err.Error()`,
	}

	for _, code := range codes {
		_ = s.Eval(code)
	}

	assert.Equal(t, `0
<nil>
0
<nil>
&errors.errorString{s:"EOF"}
"EOF"
10
&errors.errorString{s:"test"}
10
"test"
`, stdout.String())
	assert.Equal(t, "", stderr.String())
}

func TestSessionEval_Struct(t *testing.T) {
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	s, err := NewSession(stdout, stderr)
	defer s.Clear()
	require.NoError(t, err)

	codes := []string{
		`type X struct { v int }`,
		`func (x *X) add(v int) { x.v += v }`,
		`var x X`,
		`x`,
		`x.add(1)`,
		`x`,
		`x.add(2)`,
		`x`,
		`type Y X; type Z Y;`,
		`func (z *Z) sub(v int) { z.v -= v }`,
		`var z Z`,
		`z.sub(3)`,
		`z`,
	}

	for _, code := range codes {
		_ = s.Eval(code)
	}

	assert.Contains(t, stdout.String(), `main.X{v:0}
main.X{v:1}
main.X{v:3}
main.Z{v:-3}`)
	assert.Equal(t, ``, stderr.String())
}

func TestSessionEval_Func(t *testing.T) {
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	s, err := NewSession(stdout, stderr)
	defer s.Clear()
	require.NoError(t, err)

	codes := []string{
		`func f() int { return 100 }`,
		`func g() string { return "hello, world" }`,
		`func h() int { s := ""; return s }`,
		`f() + len(g())`,
		`func f() int { return 200 }`,
		`f() * len(g())`,
		`func f() string { i := 100; return i }`,
		`f() | len(g())`,
	}

	for _, code := range codes {
		_ = s.Eval(code)
	}

	assert.Equal(t, "112\n2400\n204\n", stdout.String())
	assert.Regexp(t, `cannot use s \((?:variable of )?type string\) as type int in return (?:argument|statement)
invalid operation: f\(\) \+ len\(g\(\)\) \(mismatched types string and int\)
invalid operation: f\(\) \* len\(g\(\)\) \(mismatched types string and int\)
cannot use i \((?:variable of )?type int\) as type string in return (?:argument|statement)
`, stderr.String())
}

func TestSessionEval_TokenError(t *testing.T) {
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	s, err := NewSession(stdout, stderr)
	defer s.Clear()
	require.NoError(t, err)

	codes := []string{
		`foo\`,
		`ba # r`,
		`$ + 3`,
		"`foo",
		"`foo\nbar`",
	}

	for _, code := range codes {
		_ = s.Eval(code)
	}

	assert.Equal(t, "\"foo\\nbar\"\n", stdout.String())
	assert.Equal(t, `invalid token: "\\"
invalid token: "#"
invalid token: "$"
`, stderr.String())
}

func TestSessionEval_CompileError(t *testing.T) {
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	s, err := NewSession(stdout, stderr)
	defer s.Clear()
	require.NoError(t, err)

	codes := []string{
		`foo`,
		`func f() int { return 100 }`,
		`func g() string { return "hello" }`,
		`len(f())`,
		`len(g())`,
		`f() + g()`,
		`f() + len(g())`,
	}

	for _, code := range codes {
		_ = s.Eval(code)
	}

	assert.Equal(t, "5\n105\n", stdout.String())
	assert.Regexp(t, `undefined: foo
invalid argument:? f\(\) \((?:value of )?type int\) for len
invalid operation: f\(\) \+ g\(\) \(mismatched types int and string\)
`, stderr.String())
}

func TestSession_ExtraFiles(t *testing.T) {
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	tempDir, _ := os.MkdirTemp("", "gore-")
	defer chdir(tempDir)()
	defer os.RemoveAll(tempDir)
	require.NoError(t, os.WriteFile("test.go", []byte(`package test

// V is a value
var V = 42
`), 0o644))
	s, err := NewSession(stdout, stderr)
	defer s.Clear()
	require.NoError(t, err)

	s.includeFiles([]string{"test.go"})
	codes := []string{
		`V`,
		`:type V`,
		`:doc V`,
	}

	for _, code := range codes {
		_ = s.Eval(code)
	}

	assert.Contains(t, stdout.String(), `42
int
package builtin`)
	assert.Equal(t, ``, stderr.String())
}
