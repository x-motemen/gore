package main

import (
	"testing"
)

func TestSession_completeCode(t *testing.T) {
	s, err := NewSession()
	noError(t, err)

	err = actionImport(s, "fmt")
	noError(t, err)

	keep, cands, err := s.completeCode("fmt.p", 5, true)
	if err != nil {
		if gocode.unavailable {
			t.Skipf("gocode unavailable: %s", err)
		} else {
			noError(t, err)
		}
	}

	if keep != 4 {
		t.Errorf("keep should be == 4: got %v", keep)
	}

	stringsContain(t, cands, "Println(")
}
