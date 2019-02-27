package gore

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
	if err := g.run(); err != nil {
		fmt.Fprintf(c.errWriter, "gore: %s\n", err)
		return err
	}
	return nil
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

	var showVersion bool
	fs.BoolVar(&showVersion, "version", false, "print gore version")

	err := fs.Parse(args)
	if err != nil {
		return nil, err
	}

	if showVersion {
		fmt.Fprintf(c.errWriter, "gore %s (rev: %s/%s)\n", version, revision, runtime.Version())
		return nil, flag.ErrHelp
	}

	return g, nil
}
