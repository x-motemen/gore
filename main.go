/*
Yet another Go REPL that works nicely. Featured with line editing, code completion and more.

Usage

When started, a prompt is shown waiting for input. Enter any statement or expression to proceed.
If an expression is given or any variables are assigned or defined, their data will be pretty-printed.

Some special functionalities are provided as commands, which starts with colons:

	:import <package path>  Imports a package
	:print                  Prints current source code
	:write [<filename>]     Writes out current code
	:doc <target>           Shows documentation for an expression or package name given
	:help                   Lists commands
	:quit                   Quit the session
*/
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mitchellh/go-homedir"
)

const version = "0.3.0"
const printerName = "__gore_p"

var (
	flagAutoImport = flag.Bool("autoimport", false, "formats and adjusts imports automatically")
	flagExtFiles   = flag.String("context", "",
		"import packages, functions, variables and constants from external golang source files")
	flagPkg = flag.String("pkg", "", "specify a package where the session will be run inside")
)

func main() {
	flag.Parse()

	s, err := NewSession(os.Stdout, os.Stderr)
	defer s.Clear()
	if err != nil {
		panic(err)
	}

	fmt.Fprintf(os.Stderr, "gore version %s  :help for help\n", version)

	if *flagExtFiles != "" {
		extFiles := strings.Split(*flagExtFiles, ",")
		s.includeFiles(extFiles)
	}

	if *flagPkg != "" {
		err := s.includePackage(*flagPkg)
		if err != nil {
			errorf("-pkg: %s", err)
			os.Exit(1)
		}
	}

	rl := newContLiner()
	defer rl.Close()

	var historyFile string
	home, err := homeDir()
	if err != nil {
		errorf("home: %s", err)
	} else {
		historyFile = filepath.Join(home, "history")

		f, err := os.Open(historyFile)
		if err != nil {
			if !os.IsNotExist(err) {
				errorf("%s", err)
			}
		} else {
			_, err := rl.ReadHistory(f)
			if err != nil {
				errorf("while reading history: %s", err)
			}
			f.Close()
		}
	}

	rl.SetWordCompleter(s.completeWord)

	for {
		in, err := rl.Prompt()
		if err != nil {
			if err == io.EOF {
				break
			}
			fmt.Fprintf(os.Stderr, "fatal: %s", err)
			os.Exit(1)
		}

		if in == "" {
			continue
		}

		if err := rl.Reindent(); err != nil {
			fmt.Fprintf(os.Stderr, "error: %s\n", err)
			rl.Clear()
			continue
		}

		err = s.Eval(in)
		if err != nil {
			if err == ErrContinue {
				continue
			} else if err == ErrQuit {
				break
			} else if err != ErrCmdRun {
				rl.Clear()
				continue
			}
		}
		rl.Accepted()
	}

	if historyFile != "" {
		err := os.MkdirAll(filepath.Dir(historyFile), 0755)
		if err != nil {
			errorf("%s", err)
		} else {
			f, err := os.Create(historyFile)
			if err != nil {
				errorf("%s", err)
			} else {
				_, err := rl.WriteHistory(f)
				if err != nil {
					errorf("while saving history: %s", err)
				}
				f.Close()
			}
		}
	}
}

func homeDir() (home string, err error) {
	home = os.Getenv("GORE_HOME")
	if home != "" {
		return
	}

	home, err = homedir.Dir()
	if err != nil {
		return
	}

	home = filepath.Join(home, ".gore")
	return
}
