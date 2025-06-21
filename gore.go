package gore

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Version of gore.
const Version = "0.6.1"

// Gore ...
type Gore struct {
	autoImport           bool
	extFiles             string
	packageName          string
	outWriter, errWriter io.Writer
}

// New Gore
func New(opts ...Option) *Gore {
	g := &Gore{outWriter: os.Stdout, errWriter: os.Stderr}
	for _, opt := range opts {
		opt(g)
	}
	return g
}

// Run ...
func (g *Gore) Run() error {
	s, err := NewSession(g.outWriter, g.errWriter)
	defer s.Clear()
	if err != nil {
		return err
	}
	s.autoImport = g.autoImport

	if err := s.initCompleter(); err != nil {
		debugf("failed to initialize gopls completer: %s", err)
	}

	fmt.Fprintf(g.errWriter, "gore version %s  :help for help\n", Version)

	if g.extFiles != "" {
		extFiles := strings.Split(g.extFiles, ",")
		s.includeFiles(extFiles)
	}

	if g.packageName != "" {
		if err := s.includePackage(g.packageName); err != nil {
			return err
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

	rl.SetWordCompleter(func(str string, pos int) (string, []string, string) {
		return s.completeWord(str, len(string([]rune(str)[:pos])))
	})

	for {
		in, err := rl.Prompt()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		if in == "" {
			continue
		}

		if err := rl.Reindent(); err != nil {
			fmt.Fprintf(g.errWriter, "error: %s\n", err)
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
		err := os.MkdirAll(filepath.Dir(historyFile), 0o755)
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

	return nil
}

func homeDir() (home string, err error) {
	home = os.Getenv("GORE_HOME")
	if home != "" {
		return
	}

	var baseDir string

	baseDir = os.Getenv("XDG_DATA_HOME")
	if baseDir != "" {
		home = filepath.Join(baseDir, "gore")

		return
	}

	baseDir, err = os.UserHomeDir()
	if err != nil {
		return
	}

	home = filepath.Join(baseDir, ".gore")
	return
}
