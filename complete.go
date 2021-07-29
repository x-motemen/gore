package gore

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/x-motemen/gore/gocode"
)

func (s *Session) completeWord(line string, pos int) (string, []string, string) {
	if strings.HasPrefix(strings.TrimSpace(line), ":") {
		// complete commands
		var idx int
		in := strings.TrimLeftFunc(line[:pos], func(c rune) bool {
			if c == ':' || unicode.IsSpace(c) {
				idx++
				return true
			}
			return false
		})
		var cmd string
		if tokens := strings.Fields(in); len(tokens) > 0 {
			cmd = tokens[0]
		}

		if !strings.Contains(in, " ") {
			pre, post := line[:idx], line[pos:]
			var result []string
			for _, command := range commands {
				name := pre + fmt.Sprint(command.name)
				if cmd == "" || command.name.matchesPrefix(cmd) {
					if !strings.HasPrefix(post, " ") && command.arg != "" {
						name = name + " "
					}
					result = append(result, name)
				}
			}
			return "", result, post
		}

		// complete command arguments
		for _, command := range commands {
			if command.complete == nil || !command.name.matches(cmd) {
				continue
			}
			cmdPrefix := line[:idx] + cmd + " "
			return cmdPrefix, command.complete(s, line[len(cmdPrefix):pos]), ""
		}

		return "", nil, ""
	}

	if !gocode.Available() {
		return "", nil, ""
	}

	if strings.TrimSpace(line[:pos]) == "" {
		return "", []string{line[:pos] + indent}, line[pos:]
	}

	// code completion
	pos, cands, err := s.completeCode(line, pos, true)
	if err != nil {
		errorf("completeCode: %s", err)
		return "", nil, ""
	}

	return line[0:pos], cands, ""
}

// completeCode does code completion within the session using gocode.
// in and pos specifies the current input and the cursor position (0 <= pos <= len(in)) respectively.
// If exprMode is set to true, the completion is done as an expression (e.g. appends "(" to functions).
// Return value keep specifies how many characters of in should be kept and candidates are what follow in[0:keep].
func (s *Session) completeCode(in string, pos int, exprMode bool) (keep int, candidates []string, err error) {
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
			cand = cand + "("
		}
		candidates = append(candidates, cand)
	}

	return
}
