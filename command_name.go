package gore

import "strings"

type commandName string

func (s commandName) String() string {
	var b strings.Builder
	for _, c := range s {
		if c != '[' && c != ']' {
			b.WriteRune(c)
		}
	}
	return b.String()
}

func (s commandName) matches(t string) bool {
	var grouping bool
	for i, j := 0, 0; j <= len(t); i, j = i+1, j+1 {
		if i >= len(s) {
			return j == len(t)
		}
		if s[i] == '[' {
			grouping = true
			i++
		}
		if j == len(t) {
			break
		}
		if s[i] == ']' {
			return false
		}
		if s[i] != t[j] {
			return false
		}
	}
	return grouping
}

func (s commandName) matchesPrefix(t string) bool {
	for i, j := 0, 0; j < len(t); i, j = i+1, j+1 {
		if i >= len(s) {
			return false
		}
		if s[i] == '[' {
			i++
		}
		if s[i] == ']' {
			return false
		}
		if s[i] != t[j] {
			return false
		}
	}
	return true
}
