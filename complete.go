package gore

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/x-motemen/gore/gocode"
)

func (s *Session) completeWord(line string, pos int) (string, []string, string) {
	prefix, suffix := line[:pos], line[pos:]

	if strings.TrimSpace(prefix) == "" {
		return prefix, []string{indent}, suffix
	}

	if strings.HasPrefix(strings.TrimSpace(prefix), ":") {
		return s.completeCommand(prefix, suffix)
	}

	pos, candidates, err := s.completeCode(line, pos, true)
	if err != nil {
		errorf("completeCode: %s", err)
		return "", nil, ""
	}

	return line[:pos], candidates, ""
}

func (s *Session) completeCommand(prefix, suffix string) (string, []string, string) {
	commas, prefix := cutPrefixFunc(prefix, func(c rune) bool {
		return c == ':' || unicode.IsSpace(c)
	})
	prefix, arg := cutPrefixFunc(prefix, func(c rune) bool {
		return !unicode.IsSpace(c)
	})

	if arg == "" {
		var candidates []string
		for _, command := range commands {
			if command.name.matchesPrefix(prefix) {
				name := fmt.Sprint(command.name)
				if command.arg != "" && !hasPrefixFunc(suffix, unicode.IsSpace) {
					name += " "
				}
				candidates = append(candidates, name)
			}
		}
		return commas, candidates, suffix
	}

	spaces, arg := cutPrefixFunc(arg, unicode.IsSpace)
	for _, command := range commands {
		if command.name.matches(prefix) {
			if command.complete == nil {
				break
			}
			return commas + prefix + spaces, command.complete(s, arg), ""
		}
	}

	return "", nil, ""
}

// completeCode does code completion within the session using gocode.
// in and pos specifies the current input and the cursor position (0 <= pos <= len(in)) respectively.
// If exprMode is set to true, the completion is done as an expression (e.g. appends "(" to functions).
// Return value keep specifies how many characters of in should be kept and candidates are what follow in[0:keep].
func (s *Session) completeCode(in string, pos int, exprMode bool) (keep int, candidates []string, err error) {
	if !gocode.Available() {
		return
	}

	s.clearQuickFix()

	source, err := s.source(false)
	if err != nil {
		return
	}

	// Kind of dirty hack :/
	p := strings.LastIndex(source, "}")
	editingSource := source[0:p] + in + source[p:]
	cursor := len(source[0:p]) + pos

	result, err := gocode.Query([]byte(editingSource), cursor)
	if err != nil {
		return
	}

	keep = pos - result.Cursor
	candidates = make([]string, 0, len(result.Candidates))
	for _, e := range result.Candidates {
		cand := e.Name
		if cand == printerName && e.Class == "func" {
			continue
		}
		if exprMode && e.Class == "func" {
			cand += "("
		}
		candidates = append(candidates, cand)
	}

	return
}

func hasPrefixFunc(src string, f func(rune) bool) bool {
	for _, r := range src {
		return f(r)
	}
	return false
}

func cutPrefixFunc(src string, f func(rune) bool) (string, string) {
	for i, r := range src {
		if !f(r) {
			return src[:i], src[i:]
		}
	}
	return src, ""
}
