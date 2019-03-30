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
	if err := g.Run(); err != nil {
		fmt.Fprintf(c.errWriter, "gore: %s\n", err)
		return err
	}
	return nil
}

func (c *cli) parseArgs(args []string) (*Gore, error) {
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
		fmt.Fprintf(c.outWriter, "gore %s (rev: %s/%s)\n", version, revision, runtime.Version())
		return nil, flag.ErrHelp
	}

	return New(
		AutoImport(autoImport),
		ExtFiles(extFiles),
		PackageName(packageName),
		OutWriter(c.outWriter),
		ErrWriter(c.errWriter),
	), nil
}
