package main

import (
	"os"

	"github.com/motemen/gore"
)

func main() {
	if gore.Run(os.Args[1:]) != nil {
		os.Exit(1)
	}
}
