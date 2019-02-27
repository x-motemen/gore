package gore

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommandName(t *testing.T) {
	testCases := []struct {
		name    string
		str     string
		target  string
		matches bool
		prefix  bool
	}{
		{"foobar", "foobar", "foobarr", false, false},
		{"foobar", "foobar", "foobar", true, true},
		{"foobar", "foobar", "foo", false, true},
		{"foobar", "foobar", "", false, true},
		{"foo[bar]", "foobar", "foobarr", false, false},
		{"foo[bar]", "foobar", "foobar", true, true},
		{"foo[bar]", "foobar", "foob", true, true},
		{"foo[bar]", "foobar", "foo", true, true},
		{"foo[bar]", "foobar", "fo", false, true},
		{"foo[bar]", "foobar", "", false, true},
		{"foo[bar]", "foobar", "foo[bar]", false, false},
		{"foo[bar]", "foobar", "foobar]", false, false},
		{"foo[bar]", "foobar", "foo[]", false, false},
		{"foo[bar]", "foobar", "foo[", false, false},
		{"foo[bar]", "foobar", "[", false, false},
		{"[bar]", "bar", "foobar", false, false},
		{"[bar]", "bar", "bar", true, true},
		{"[bar]", "bar", "bra", false, false},
		{"[bar]", "bar", "ba", true, true},
		{"[bar]", "bar", "", true, true},
	}
	for _, tc := range testCases {
		t.Run(tc.name+"/"+tc.target, func(t *testing.T) {
			cn := commandName(tc.name)
			assert.Equal(t, tc.str, fmt.Sprint(cn))
			assert.Equal(t, tc.matches, cn.matches(tc.target))
			assert.Equal(t, tc.prefix, cn.matchesPrefix(tc.target))
		})
	}
}
