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

	result, err := gocode.query(source, cursor)
	if err != nil {
		if gocode.unavailable {
			t.Skipf("gocode unavailable: %s", err)
		} else {
			noError(t, err)
		}
	}

	if result.pos != 1 {
		t.Errorf("result.pos should == 1, got: %v", result.pos)
	}

	found := false
	for _, e := range result.entries {
		if e.Name == "Println" {
			found = true
			break
		}
	}
	if !found {
		t.Logf(`result must contain "Println": %#v`, result.entries)
	}
}
