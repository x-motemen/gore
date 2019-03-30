package main

import (
	"os"

	"github.com/motemen/gore/cli"
)

func main() {
	if cli.Run(os.Args[1:]) != nil {
		os.Exit(1)
	}
}
