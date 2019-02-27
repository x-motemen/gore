package main

import (
	"flag"
	"fmt"
	"io"
	"runtime"
)

type cli struct {
	outWriter, errWriter io.Writer
}

func (c *cli) run(args []string) error {
	g, err := c.parseArgs(args)
	if err != nil {
		return err
	}
	return g.run()
}

func (c *cli) parseArgs(args []string) (*gore, error) {
	g := &gore{outWriter: c.outWriter, errWriter: c.errWriter}
	fs := flag.NewFlagSet("gore", flag.ContinueOnError)
	fs.SetOutput(c.errWriter)
	fs.Usage = func() {
		fs.SetOutput(c.outWriter)
		defer fs.SetOutput(c.errWriter)
		fmt.Fprintf(c.outWriter, `gore - A Go REPL

Version: %s (rev: %s/%s)

Synopsis:
    %% gore

Options:
`, version, revision, runtime.Version())
		fs.PrintDefaults()
	}

	fs.BoolVar(&g.autoImport, "autoimport", false, "formats and adjusts imports automatically")
	fs.StringVar(&g.extFiles, "context", "", "import packages, functions, variables and constants from external golang source files")
	fs.StringVar(&g.packageName, "pkg", "", "the package where the session will be run inside")

	err := fs.Parse(args)
	if err != nil {
		return nil, err
	}
	return g, nil
}
