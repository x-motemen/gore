package gore

import "strings"

type commandName string

var commandAbbrReplacer = strings.NewReplacer("[", "", "]", "")

func (s commandName) String() string {
	return commandAbbrReplacer.Replace(string(s))
}

func (s commandName) matches(t string) bool {
	prefix, rest, _ := strings.Cut(string(s), "[")
	abbr, _, _ := strings.Cut(rest, "]")
	return strings.HasPrefix(t, prefix) &&
		strings.HasPrefix(abbr, t[len(prefix):])
}

func (s commandName) matchesPrefix(t string) bool {
	return strings.HasPrefix(string(s), t) || s.matches(t)
}
