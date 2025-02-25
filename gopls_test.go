package gore

import "testing"

func TestDiffString(t *testing.T) {
	testCases := []struct {
		s, t    string
		i, j, k int
	}{
		{"", "", 0, 0, 0},
		{"\n", "\n", 1, 1, 1},
		{"abc", "", 0, 3, 0},
		{"", "abc", 0, 0, 3},
		{"abc", "abc", 3, 3, 3},
		{"abc", "abc\n", 3, 3, 4},
		{"abc\n", "abc", 0, 4, 3},
		{"abc\n", "", 0, 4, 0},
		{"", "abc\n", 0, 0, 4},
		{"abc\n", "abc\n", 4, 4, 4},
		{"abc\n", "abc\ndef\n", 4, 4, 8},
		{"abc\ndef\n", "abc\n", 4, 8, 4},
		{"abc\ndef\n", "abc\ndef\n", 8, 8, 8},
		{"abc\ndef\n", "abc\nghidef\n", 4, 4, 7},
		{"abc\ndef\n", "abc\nghi\ndef\n", 4, 4, 8},
		{"abc\ndef\n", "abc\ndef\nghi\n", 8, 8, 12},
		{"abc\nghi\ndef\n", "abc\ndef\n", 4, 8, 4},
		{"abc\ndef\n", "ghi\ndef\n", 0, 4, 4},
		{"abc\ndef\n", "defbc\n", 0, 8, 6},
		{"abc\ndef\n", "def\nabc\n", 0, 0, 4},
		{"abc\ndef\n", "abcdefabc\n", 0, 0, 6},
		{"abc\nf\ndef\n", "abc\ndef\n", 4, 6, 4},
		{"abc;def\nghi\n", "abc;abc\ndef\n", 4, 4, 8},
		{"abc\ndef\nghi\n", "def\nghi\nabc\n", 0, 0, 8},
		{"abc\ndef\nghi\n", "ghi\nabc\ndef\n", 0, 0, 4},
		{"abc\ndef\nghi\njkl\n", "abc\njkl\nghi\ndef\n", 4, 4, 12},
		{"abc\ndef\nghi\njkl\n", "jkl\nabc\ndef\nghi\n", 0, 0, 4},
		{"abc\ndef\nghi\njkl\n", "abc\nghi\njkl\ndef\n", 4, 4, 12},
		{"abc\ndef\nghi\njkl\n", "abc\ndef\nghi\njkl\n", 16, 16, 16},
	}
	for _, tc := range testCases {
		i, j, k := diffString(tc.s, tc.t)
		if i != tc.i || j != tc.j || k != tc.k {
			t.Errorf("diffString(%q, %q) = %d, %d, %d; want %d, %d, %d",
				tc.s, tc.t, i, j, k, tc.i, tc.j, tc.k)
		}
		if tc.s[:i] != tc.t[:i] || i < j && tc.s[i:j] == tc.t[i:k] {
			t.Errorf("diffString(%q, %q): %q != %q or %q == %q",
				tc.s, tc.t, tc.s[:i], tc.t[:i], tc.s[i:j], tc.t[i:k])
		}
	}
}

func TestGetFromPos(t *testing.T) {
	testCases := []struct {
		source     string
		pos        int
		line, char uint32
	}{
		{"", 0, 0, 0},
		{"\n", 0, 0, 0},
		{"\n", 1, 1, 0},
		{"abc", 0, 0, 0},
		{"abc", 1, 0, 1},
		{"abc", 2, 0, 2},
		{"abc", 3, 0, 3},
		{"abc\ndef\n", 0, 0, 0},
		{"abc\ndef\n", 3, 0, 3},
		{"abc\ndef\n", 4, 1, 0},
		{"abc\ndef\n", 7, 1, 3},
		{"abc\ndef\n", 8, 2, 0},
	}
	for _, tc := range testCases {
		pos := getPos(tc.source, tc.pos)
		if pos.Line != tc.line || pos.Character != tc.char {
			t.Errorf("getPos(%q, %d) = %d, %d; want %d, %d",
				tc.source, tc.pos, pos.Line, pos.Character, tc.line, tc.char)
		}
		p := fromPos(tc.source, pos)
		if p != tc.pos {
			t.Errorf("fromPos(%q, %v) = %d; want %d",
				tc.source, pos, p, tc.pos)
		}
	}
}
