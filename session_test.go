package main

import (
	"bytes"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	printerPkgs = []struct {
		path string
		code string
	}{
		{"fmt", `fmt.Printf("%#v\n", x)`},
	}
}

func TestRun_import(t *testing.T) {
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

func TestRun_QuickFix_evaluated_but_not_used(t *testing.T) {
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

func TestRun_QuickFix_used_as_value(t *testing.T) {
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

func TestRun_FixImports(t *testing.T) {
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

func TestIncludePackage(t *testing.T) {
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	s, err := NewSession(stdout, stderr)
	defer s.Clear()
	require.NoError(t, err)

	err = s.includePackage("github.com/motemen/gore/gocode")
	require.NoError(t, err)

	err = s.Eval("Completer{}")
	require.NoError(t, err)
}

func TestRun_Copy(t *testing.T) {
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	s, err := NewSession(stdout, stderr)
	defer s.Clear()
	require.NoError(t, err)

	codes := []string{
		`a := []string{"hello", "world"}`,
		`b := []string{"goodbye", "world"}`,
		`copy(a, b)`,
		`if (a[0] != "goodbye") {
			panic("should be copied")
		}`,
	}

	for _, code := range codes {
		err := s.Eval(code)
		require.NoError(t, err)
	}

	assert.Equal(t, `[]string{"hello", "world"}
[]string{"goodbye", "world"}
2
`, stdout.String())
	assert.Equal(t, "", stderr.String())
}

func TestRun_Const(t *testing.T) {
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

	assert.Equal(t, "0\n1\n", stdout.String())
	assert.Equal(t, "", stderr.String())
}

func TestRun_NotUsed(t *testing.T) {
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

func TestRun_MultipleValues(t *testing.T) {
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

func TestRun_Func(t *testing.T) {
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	s, err := NewSession(stdout, stderr)
	defer s.Clear()
	require.NoError(t, err)

	codes := []string{
		`func f() int { return 100 }`,
		`func g() string { return "hello, world" }`,
		`func h() int { return "foo" }`,
		`f() + len(g())`,
		`func f() int { return 200 }`,
		`f() * len(g())`,
		`func f() string { return 100 }`,
		`f() | len(g())`,
	}

	for _, code := range codes {
		_ = s.Eval(code)
	}

	assert.Equal(t, "112\n2400\n204\n", stdout.String())
	assert.Equal(t, `cannot use "foo" (type string) as type int in return argument
invalid operation: f() + len(g()) (mismatched types string and int)
invalid operation: f() * len(g()) (mismatched types string and int)
cannot use 100 (type int) as type string in return argument
`, stderr.String())
}

func TestRun_TokenError(t *testing.T) {
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	s, err := NewSession(stdout, stderr)
	defer s.Clear()
	require.NoError(t, err)

	codes := []string{
		`foo\`,
		`ba # r`,
		`$ + 3`,
		`~1`,
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
invalid token: "~"
`, stderr.String())
}

func TestRun_CompileError(t *testing.T) {
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
	assert.Equal(t, `undefined: foo
invalid argument f() (type int) for len
invalid operation: f() + g() (mismatched types int and string)
`, stderr.String())
}
