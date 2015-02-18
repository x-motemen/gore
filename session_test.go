package main

import (
	"testing"
)

func TestRun_import(t *testing.T) {
	s, err := NewSession()
	noError(t, err)

	codes := []string{
		":import encoding/json",
		"b, err := json.Marshal(nil)",
		"string(b)",
	}

	for _, code := range codes {
		err := s.Eval(code)
		noError(t, err)
	}
}

func TestRun_QuickFix_evaluated_but_not_used(t *testing.T) {
	s, err := NewSession()
	noError(t, err)

	codes := []string{
		`[]byte("")`,
		`make([]int, 0)`,
		`1+1`,
		`func() {}`,
		`1`,
	}

	for _, code := range codes {
		err := s.Eval(code)
		noError(t, err)
	}
}
