package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motemen/gore/gocode"
)

func TestSession_completeWord(t *testing.T) {
	if gocode.Available() == false {
		t.Skipf("gocode unavailable")
	}

	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	s, err := NewSession(stdout, stderr)
	defer s.Clear()
	require.NoError(t, err)

	pre, cands, post := s.completeWord("", 0)
	assert.Equal(t, "", pre)
	assert.Equal(t, []string{"    "}, cands)
	assert.Equal(t, post, "")

	pre, cands, post = s.completeWord("    x", 4)
	assert.Equal(t, "", pre)
	assert.Equal(t, []string{"        "}, cands)
	assert.Equal(t, post, "x")

	err = actionImport(s, "fmt")
	require.NoError(t, err)

	pre, cands, post = s.completeWord("fmt.p", 5)
	assert.Equal(t, "fmt.", pre)
	assert.Contains(t, cands, "Println(")
	assert.Equal(t, post, "")
}
