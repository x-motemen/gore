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
	return &contLiner{State: rl}
}

func (cl *contLiner) promptString() string {
	if cl.buffer != "" {
		prompt := promptContinue
		for i := 0; i < cl.depth; i++ {
			prompt += indent
		}
		return prompt
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

func (cl *contLiner) Reindent() {
	var min, max int
	newDepth := cl.countDepth()

	if newDepth > cl.depth {
		min, max = cl.depth, newDepth
	} else {
		min, max = newDepth, cl.depth
	}

	lines := strings.Split(cl.buffer, "\n")
	lastLine := lines[len(lines)-1]

	prompt := promptDefault
	if len(lines) > 1 {
		prompt = promptContinue
	}
	fmt.Printf("\x1b[1A\r%s", prompt)

	cl.printIndent(min)
	fmt.Print(lastLine)
	cl.printIndent(max - min)
	fmt.Print("\n")

	cl.depth = newDepth
}

func (cl *contLiner) countDepth() int {
	reader := bytes.NewBufferString(cl.buffer)
	sc := new(scanner.Scanner)
	sc.Init(reader)

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

func (cl *contLiner) printIndent(count int) {
	for i := 0; i < count; i++ {
		fmt.Print(indent)
	}
}
