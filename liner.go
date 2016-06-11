package main

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"text/scanner"

	"github.com/peterh/liner"
)

const (
	promptDefault  = "gore> "
	promptContinue = "..... "
	indent         = "    "
)

type contLiner struct {
	*liner.State
	buffer string
	depth  int
}

func newContLiner() *contLiner {
	rl := liner.NewLiner()
	rl.SetCtrlCAborts(true)
	return &contLiner{State: rl}
}

func (cl *contLiner) promptString() string {
	if cl.buffer != "" {
		return promptContinue + strings.Repeat(indent, cl.depth)
	}

	return promptDefault
}

func (cl *contLiner) Prompt() (string, error) {
	line, err := cl.State.Prompt(cl.promptString())
	if err == io.EOF {
		if cl.buffer != "" {
			// cancel line continuation
			cl.Accepted()
			fmt.Println()
			err = nil
		}
	} else if err == liner.ErrPromptAborted {
		err = nil
		if cl.buffer != "" {
			cl.Accepted()
		} else {
			fmt.Println("(^D to quit)")
		}
	} else if err == nil {
		if cl.buffer != "" {
			cl.buffer = cl.buffer + "\n" + line
		} else {
			cl.buffer = line
		}
	}

	return cl.buffer, err
}

func (cl *contLiner) Accepted() {
	cl.State.AppendHistory(cl.buffer)
	cl.buffer = ""
}

func (cl *contLiner) Clear() {
	cl.buffer = ""
	cl.depth = 0
}

var errUnmatchedBraces = fmt.Errorf("unmatched braces")

func (cl *contLiner) Reindent() error {
	oldDepth := cl.depth
	cl.depth = cl.countDepth()

	if cl.depth < 0 {
		return errUnmatchedBraces
	}

	if cl.depth < oldDepth {
		lines := strings.Split(cl.buffer, "\n")
		if len(lines) > 1 {
			lastLine := lines[len(lines)-1]

			cursorUp()
			fmt.Printf("\r%s%s", cl.promptString(), lastLine)
			eraseInLine()
			fmt.Print("\n")
		}
	}

	return nil
}

func (cl *contLiner) countDepth() int {
	reader := bytes.NewBufferString(cl.buffer)
	sc := new(scanner.Scanner)
	sc.Init(reader)
	sc.Error = func(_ *scanner.Scanner, msg string) {
		debugf("scanner: %s", msg)
	}

	depth := 0
	for {
		switch sc.Scan() {
		case '{':
			depth++
		case '}':
			depth--
		case scanner.EOF:
			return depth
		}
	}
}
