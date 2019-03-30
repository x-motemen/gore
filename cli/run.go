package cli

import (
	"flag"
	"os"
)

// Run gore.
func Run(args []string) error {
	err := (&cli{outWriter: os.Stdout, errWriter: os.Stderr}).run(args)
	if err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	return nil
}
