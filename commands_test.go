package main

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestActionDoc(t *testing.T) {
	_, err := exec.LookPath("godoc")
	if err != nil {
		t.Skipf("godoc not found: %s", err)
	}

	s, err := NewSession()
	require.NoError(t, err)

	err = actionImport(s, "encoding/json")
	require.NoError(t, err)
	err = actionImport(s, "fmt")
	require.NoError(t, err)

	test := func() {
		err = actionDoc(s, "fmt")
		require.NoError(t, err)

		err = actionDoc(s, "fmt.Print")
		require.NoError(t, err)

		err = actionDoc(s, "json.NewEncoder(nil).Encode")
		require.NoError(t, err)
	}

	test()

	// test :doc works after some code

	s.Eval("a := 1")
	s.Eval("fmt.Print()")

	test()
}

func TestActionImport(t *testing.T) {
	s, err := NewSession()
	require.NoError(t, err)

	require.NoError(t, actionImport(s, "encoding/json fmt"))

	require.NoError(t, s.Eval("fmt.Print"))
	require.NoError(t, s.Eval("json.Encoder{}"))
}
