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

func TestIncludePackage(t *testing.T) {
	s, err := NewSession()
	noError(t, err)

	err = s.includePackage("github.com/motemen/gore/gocode")
	noError(t, err)

	err = s.Eval("Completer{}")
	noError(t, err)
}

func TestRun_Copy(t *testing.T) {
	s, err := NewSession()
	noError(t, err)

	codes := []string{
		`a := []string{"hello", "world"}`,
		`b := []string{"goodbye", "world"}`,
		`copy(a, b)`,
		`if (a[0] != "goodbye") {
			panic("should be copied")
		}`,
	}

	for _, code := range codes {
		err := s.Eval(code)
		noError(t, err)
	}
}

func TestRun_Const(t *testing.T) {
	s, err := NewSession()
	noError(t, err)

	codes := []string{
		`const ( a = iota; b )`,
		`a`,
		`b`,
	}

	for _, code := range codes {
		err := s.Eval(code)
		noError(t, err)
	}
}
