package main

import (
	"testing"
)

func noError(t *testing.T, err error) {
	if err != nil {
		t.Fatal(err)
	}
}
