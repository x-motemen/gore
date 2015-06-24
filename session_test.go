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
		`(4 & (1 << 1))`,
		`1`,
	}

	for _, code := range codes {
		err := s.Eval(code)
		noError(t, err)
	}
}

func TestRun_QuickFix_used_as_value(t *testing.T) {
	s, err := NewSession()
	noError(t, err)

	codes := []string{
		`:import log`,
		`a := 1`,
		`log.SetPrefix("")`,
	}

	for _, code := range codes {
		err := s.Eval(code)
		noError(t, err)
	}
}

func TestRun_FixImports(t *testing.T) {
	s, err := NewSession()
	noError(t, err)

	autoimport := true
	flagAutoImport = &autoimport

	codes := []string{
		`filepath.Join("a", "b")`,
	}

	for _, code := range codes {
		err := s.Eval(code)
		noError(t, err)
	}
}
