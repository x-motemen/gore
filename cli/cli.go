package cli

import (
	"flag"
	"fmt"
	"io"
	"runtime"

	"github.com/x-motemen/gore"
)

var revision = "HEAD"

const (
	exitCodeOK = iota
	exitCodeErr
)

type cli struct {
	outWriter, errWriter io.Writer
}

func (c *cli) run(args []string) int {
	g, err := c.parseArgs(args)
	if err != nil {
		if err != flag.ErrHelp {
			return exitCodeErr
		}
		return exitCodeOK
	}
	if err := g.Run(); err != nil {
		fmt.Fprintf(c.errWriter, "gore: %s\n", err)
		return exitCodeErr
	}
	return exitCodeOK
}

func (c *cli) parseArgs(args []string) (*gore.Gore, error) {
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
`, gore.Version, revision, runtime.Version())
		fs.PrintDefaults()
	}

	var autoImport bool
	fs.BoolVar(&autoImport, "autoimport", false, "formats and adjusts imports automatically")

	var extFiles string
	fs.StringVar(&extFiles, "context", "", "import packages, functions, variables and constants from external golang source files")

	var packageName string
	fs.StringVar(&packageName, "pkg", "", "the package where the session will be run inside")

	var showVersion bool
	fs.BoolVar(&showVersion, "version", false, "print gore version")

	err := fs.Parse(args)
	if err != nil {
		return nil, err
	}

	if showVersion {
		fmt.Fprintf(c.outWriter, "gore %s (rev: %s/%s)\n", gore.Version, revision, runtime.Version())
		return nil, flag.ErrHelp
	}

	return gore.New(
		gore.AutoImport(autoImport),
		gore.ExtFiles(extFiles),
		gore.PackageName(packageName),
		gore.OutWriter(c.outWriter),
		gore.ErrWriter(c.errWriter),
	), nil
}
