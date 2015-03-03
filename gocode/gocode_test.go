package gocode

import (
	"strings"

	"testing"
)

func TestUnavailable(t *testing.T) {
	c := &Completer{GocodePath: "./no-such-bin"}
	if c.Available() {
		t.Error("should not be available: %#v", c)
	}
}

func TestQuery(t *testing.T) {
	if !Available() {
		t.Skip("gocode unavailable")
	}

	source := `package P

import "fmt"

func F() {
	fmt.P_
}
`
	cursor := strings.Index(source, "_")

	result, err := Query([]byte(source), cursor)
	if err != nil {
		t.Errorf("error should not occur: %s", err)
	}

	if result.Cursor != 1 {
		t.Errorf("result.Cursor should == 1, got: %v", result.Cursor)
	}

	found := false
	for _, e := range result.Candidates {
		if e.Name == "Println" {
			found = true
			break
		}
	}
	if !found {
		t.Logf(`result must contain "Println": %#v`, result.Candidates)
	}
}
