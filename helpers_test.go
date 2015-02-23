package main

import (
	"testing"
)

func noError(t *testing.T, err error) {
	if err != nil {
		t.Fatal(err)
	}
}

func stringsContain(t *testing.T, ss []string, it string) {
	for _, s := range ss {
		if s == it {
			return
		}
	}

	t.Errorf("should contain %q: %v", it, ss)
}
