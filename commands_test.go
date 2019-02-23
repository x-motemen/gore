package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAction_Doc(t *testing.T) {
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	s, err := NewSession(stdout, stderr)
	defer s.Clear()
	require.NoError(t, err)

	err = s.Eval(":import encoding/json")
	require.NoError(t, err)
	err = s.Eval(":import fmt")
	require.NoError(t, err)

	test := func() {
		err = s.Eval(":doc fmt")
		require.NoError(t, err)

		err = s.Eval(":doc fmt.Print")
		require.NoError(t, err)

		err = s.Eval(":doc json.NewEncoder(nil).Encode")
		require.NoError(t, err)
	}

	test()

	// test :doc works after some code

	s.Eval("a := 1")
	s.Eval("fmt.Print()")

	test()

	assert.Contains(t, stdout.String(), "package fmt")
	assert.Contains(t, stdout.String(), "func Printf")
	assert.Equal(t, "", stderr.String())
}

func TestAction_Import(t *testing.T) {
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	s, err := NewSession(stdout, stderr)
	defer s.Clear()
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
}

func TestAction_Clear(t *testing.T) {
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	s, err := NewSession(stdout, stderr)
	defer s.Clear()
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
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	s, err := NewSession(stdout, stderr)
	defer s.Clear()
	require.NoError(t, err)

	err = s.Eval(":help")
	require.NoError(t, err)

	assert.Contains(t, stdout.String(), "show this help")
	assert.Contains(t, stdout.String(), "quit the session")
	assert.Equal(t, "", stderr.String())
}
