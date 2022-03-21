package gore

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func chdir(dir string) func() {
	d, _ := os.Getwd()
	os.Chdir(dir)
	return func() { os.Chdir(d) }
}

func gomodSetup(t *testing.T) func() {
	tempDir, err := os.MkdirTemp("", "gore-")
	require.NoError(t, err)
	mod1Dir := filepath.Join(tempDir, "mod1")
	require.NoError(t, os.Mkdir(mod1Dir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(mod1Dir, "go.mod"), []byte(`module mod1
`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(mod1Dir, "mod1.go"), []byte(`package mod1

const Value = 10
`), 0o600))

	mod2Dir := filepath.Join(tempDir, "mod2")
	require.NoError(t, os.Mkdir(mod2Dir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(mod2Dir, "go.mod"), []byte(fmt.Sprintf(`module mod2

replace mod1 => %s

require mod1 v0.0.0-00010101000000-000000000000
`, strconv.Quote(mod1Dir))), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(mod2Dir, "mod2.go"), []byte(`package mod2

import "mod1"

func Foo() int {
	return mod1.Value
}
`), 0o600))

	mod3Dir := filepath.Join(mod2Dir, "mod3")
	require.NoError(t, os.Mkdir(mod3Dir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(mod3Dir, "mod3.go"), []byte(`package mod3

func Bar() string {
	return "mod3"
}
`), 0o600))

	mod4Dir := filepath.Join(mod2Dir, "mod4")
	require.NoError(t, os.Mkdir(mod4Dir, 0o700))

	restore := chdir(mod2Dir)
	return func() {
		defer os.RemoveAll(tempDir)
		defer restore()
	}
}

func TestSessionEval_Gomod(t *testing.T) {
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	defer gomodSetup(t)()
	s, err := NewSession(stdout, stderr)
	defer s.Clear()
	require.NoError(t, err)

	codes := []string{
		`:i mod2`,
		`mod2.Foo()`,
		`mod2.Foo() + mod2.Foo()`,
		`:clear`,
		`:i mod2`,
		`mod2.Foo()`,
		`:i mod1`,
		`3 * mod1.Value`,
	}

	for _, code := range codes {
		_ = s.Eval(code)
	}

	assert.Equal(t, "10\n20\n10\n30\n", stdout.String())
	assert.Equal(t, ``, stderr.String())
}

func TestSessionEval_Gomod_AutoImport(t *testing.T) {
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	defer gomodSetup(t)()
	s, err := NewSession(stdout, stderr)
	defer s.Clear()
	require.NoError(t, err)
	s.autoImport = true

	codes := []string{
		`mod2.Foo()`,
		`mod2.Foo() + mod2.Foo()`,
		`:clear`,
		`mod2.Foo()`,
		`3 * mod1.Value`,
		`:t mod2.Foo`,
		`:d mod2.Foo`,
		`mod3.Bar()`,
		`:t mod3.Bar`,
	}

	for _, code := range codes {
		_ = s.Eval(code)
	}

	assert.Equal(t, `10
20
10
30
func() int
package mod2 // import "mod2"

func Foo() int
"mod3"
func() string
`, stdout.String())
	assert.Equal(t, ``, stderr.String())
}

func TestSessionEval_Gomod_DeepDir(t *testing.T) {
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	defer gomodSetup(t)()
	require.NoError(t, os.Mkdir("tmp", 0o700))
	require.NoError(t, os.Chdir("tmp"))
	s, err := NewSession(stdout, stderr)
	defer s.Clear()
	require.NoError(t, err)

	codes := []string{
		`:i mod2`,
		`mod2.Foo()`,
		`mod2.Foo() + mod2.Foo()`,
		`:clear`,
		`:i mod2`,
		`mod2.Foo()`,
		`:i mod1`,
		`3 * mod1.Value`,
	}

	for _, code := range codes {
		_ = s.Eval(code)
	}

	assert.Equal(t, "10\n20\n10\n30\n", stdout.String())
	assert.Equal(t, ``, stderr.String())
}

func TestSessionEval_Gomod_Outside(t *testing.T) {
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	tempDir, _ := os.MkdirTemp("", "gore-")
	defer chdir(tempDir)()
	defer os.RemoveAll(tempDir)
	s, err := NewSession(stdout, stderr)
	defer s.Clear()
	require.NoError(t, err)

	codes := []string{
		`:i github.com/x-motemen/gore`,
		`gore.Session{}`,
	}

	for _, code := range codes {
		_ = s.Eval(code)
	}

	assert.Equal(t, ``, stderr.String())
}

func TestSessionEval_Gomod_CompleteImport(t *testing.T) {
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	defer gomodSetup(t)()
	s, err := NewSession(stdout, stderr)
	defer s.Clear()
	require.NoError(t, err)

	pre, cands, post := s.completeWord(":i ", 3)
	assert.Equal(t, ":i ", pre)
	assert.Subset(t, cands, []string{"mod2", "mod1"})
	assert.Equal(t, post, "")

	pre, cands, post = s.completeWord(":i mod2/", 8)
	assert.Equal(t, ":i ", pre)
	assert.Subset(t, cands, []string{"mod2/mod3", "mod2/mod4"})
	assert.Equal(t, post, "")
}
