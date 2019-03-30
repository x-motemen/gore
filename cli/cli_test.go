package cli

import (
	"bytes"
	"flag"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/motemen/gore"
)

func TestCliRun_Version(t *testing.T) {
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	c := &cli{stdout, stderr}
	err := c.run([]string{"-version"})
	require.Equal(t, err, flag.ErrHelp)

	assert.Contains(t, stdout.String(), "gore "+gore.Version)
	assert.Equal(t, "", stderr.String())
}

func TestCliRun_Help(t *testing.T) {
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	c := &cli{stdout, stderr}
	err := c.run([]string{"-help"})
	require.Equal(t, err, flag.ErrHelp)

	assert.Contains(t, stdout.String(), "gore -")
	assert.Contains(t, stdout.String(), "Options:")
	assert.Equal(t, "", stderr.String())
}

func TestCliRun_Unknown(t *testing.T) {
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	c := &cli{stdout, stderr}
	err := c.run([]string{"-foobar"})
	require.Error(t, err)

	assert.Contains(t, stdout.String(), "gore -")
	assert.Contains(t, stderr.String(), "flag provided but not defined: -foobar")
}
