package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/x-motemen/gore"
)

func TestCliRun_Version(t *testing.T) {
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	c := &cli{stdout, stderr}
	code := c.run([]string{"-version"})
	require.Equal(t, exitCodeOK, code)

	assert.Contains(t, stdout.String(), "gore "+gore.Version)
	assert.Equal(t, "", stderr.String())
}

func TestCliRun_Help(t *testing.T) {
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	c := &cli{stdout, stderr}
	code := c.run([]string{"-help"})
	require.Equal(t, exitCodeOK, code)

	assert.Contains(t, stdout.String(), "gore -")
	assert.Contains(t, stdout.String(), "Options:")
	assert.Equal(t, "", stderr.String())
}

func TestCliRun_Unknown(t *testing.T) {
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	c := &cli{stdout, stderr}
	code := c.run([]string{"-foobar"})
	require.Equal(t, exitCodeErr, code)

	assert.Contains(t, stdout.String(), "gore -")
	assert.Contains(t, stderr.String(), "flag provided but not defined: -foobar")
}
