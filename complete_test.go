package main

import (
	"testing"

	"github.com/motemen/gore/gocode"
)

func TestSession_completeCode(t *testing.T) {
	if gocode.Available() == false {
		t.Skipf("gocode unavailable")
	}

	s, err := NewSession()
	noError(t, err)

	keep, cands, err := s.completeCode("", 0, true)
	if err != nil {
		noError(t, err)
	}

	if keep != 0 {
		t.Errorf("keep should be == 0: got %v", keep)
	}

	stringsContain(t, cands, "main(")

	err = actionImport(s, "fmt")
	noError(t, err)

	keep, cands, err = s.completeCode("fmt.p", 5, true)
	if err != nil {
		noError(t, err)
	}

	if keep != 4 {
		t.Errorf("keep should be == 4: got %v", keep)
	}

	stringsContain(t, cands, "Println(")
}
