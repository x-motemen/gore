package cli

import "os"

// Run gore.
func Run(args []string) int {
	return (&cli{
		outWriter: os.Stdout,
		errWriter: os.Stderr,
	}).run(args)
}
