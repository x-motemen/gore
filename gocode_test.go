package main

import (
	"strings"

	"testing"
)

func TestComplete(t *testing.T) {
	source := `package P

import "fmt"

func F() {
	fmt.P_
}
`
	cursor := strings.Index(source, "_")

	cand, err := gocode.complete(source, cursor)
	if err != nil {
		if gocode.unavailable {
			t.Skipf("gocode unavailable: %s", err)
		} else {
			noError(t, err)
		}
	}

	if strings.Contains(strings.Join(cand, "\000"), "rintln(") == false {
		t.Errorf(`Result must contain "rintln(": %+v`, cand)
	}
}
