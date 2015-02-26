package main

import (
	"strings"

	"github.com/motemen/gore/gocode"
)

func (s *Session) completeWord(line string, pos int) (string, []string, string) {
	if strings.HasPrefix(line, ":") {
		// complete commands
		if !strings.Contains(line[0:pos], " ") {
			pre, post := line[0:pos], line[pos:]

			result := []string{}
			for _, command := range commands {
				name := ":" + command.name
				if strings.HasPrefix(name, pre) {
					// having complete means that this command takes an argument (for now)
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
			if command.complete == nil {
				continue
			}

			cmdPrefix := ":" + command.name + " "
			if strings.HasPrefix(line, cmdPrefix) && pos >= len(cmdPrefix) {
				return cmdPrefix, command.complete(s, line[len(cmdPrefix):pos]), ""
			}
		}

		return "", nil, ""
	}

	if gocode.Available() == false {
		return "", nil, ""
	}

	// code completion
	pos, cands, err := s.completeCode(line, pos, true)
	if err != nil {
		errorf("completeCode: %s", err)
		return "", nil, ""
	}

	return line[0:pos], cands, ""
}

// completeCode does code completion within the session using gocode (https://github.com/nsf/gocode).
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
		if exprMode && e.Class == "func" {
			cand = cand + "("
		}
		candidates = append(candidates, cand)
	}

	return
}
