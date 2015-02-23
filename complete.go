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

	s.clearQuickFix()

	source, err := s.source(false)
	if err != nil {
		errorf("source: %s", err)
		return "", nil, ""
	}

	p := strings.LastIndex(source, "}")
	editSource := source[0:p] + line + source[p:]
	cursor := len(source[0:p]) + pos

	cands, err := gocode.complete(editSource, cursor)
	if err != nil {
		errorf("gocode: %s", err)
		return "", nil, ""
	}

	return line[0:pos], cands, ""

}
