package main

import (
	"os/exec"
	"testing"
)

func TestActionDoc(t *testing.T) {
	_, err := exec.LookPath("godoc")
	if err != nil {
		t.Skipf("godoc not found: %s", err)
	}

	s, err := NewSession()
	noError(t, err)

	err = actionImport(s, "encoding/json")
	noError(t, err)
	err = actionImport(s, "fmt")
	noError(t, err)

	test := func() {
		err = actionDoc(s, "fmt")
		noError(t, err)

		err = actionDoc(s, "fmt.Print")
		noError(t, err)

		err = actionDoc(s, "json.NewEncoder(nil).Encode")
		noError(t, err)
	}

	test()

	// test :doc works after some code

	s.Eval("a := 1")
	s.Eval("fmt.Print()")

	test()
}
