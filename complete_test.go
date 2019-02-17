package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motemen/gore/gocode"
)

func TestSession_completeCode(t *testing.T) {
	if gocode.Available() == false {
		t.Skipf("gocode unavailable")
	}

	s, err := NewSession()
	require.NoError(t, err)

	keep, cands, err := s.completeCode("", 0, true)
	require.NoError(t, err)

	assert.Equal(t, 0, keep)
	assert.Contains(t, cands, "main(")
	assert.NotContains(t, cands, printerName+"(")

	err = actionImport(s, "fmt")
	require.NoError(t, err)

	keep, cands, err = s.completeCode("fmt.p", 5, true)
	require.NoError(t, err)

	assert.Equal(t, 4, keep)
	assert.Contains(t, cands, "Println(")
}
