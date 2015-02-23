package main

import (
	"strings"
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
				return cmdPrefix, command.complete(line[len(cmdPrefix):pos]), ""
			}
		}

		return "", nil, ""
	}

	if gocode.unavailable {
		return "", nil, ""
	}

	// code completion
	pos, cands, err := s.completeCode(line, pos)
	if err != nil {
		errorf("completeCode: %s", err)
		return "", nil, ""
	}

	return line[0:pos], cands, ""
}

// completeCode does code completion using gocode (https://github.com/nsf/gocode).
func (s *Session) completeCode(in string, pos int) (int, []string, error) {
	s.clearQuickFix()

	source, err := s.source(false)
	if err != nil {
		return 0, nil, err
	}

	// Kind of dirty hack :/
	p := strings.LastIndex(source, "}")
	editingSource := source[0:p] + in + source[p:]
	cursor := len(source[0:p]) + pos

	result, err := gocode.query(editingSource, cursor)
	if err != nil {
		return 0, nil, err
	}

	cands := make([]string, 0, len(result.entries))
	for _, e := range result.entries {
		cand := e.Name
		if e.Class == "func" {
			cand = cand + "("
		}
		cands = append(cands, cand)
	}
	return pos - result.pos, cands, nil
}
