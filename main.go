package main

import "os"

func main() {
	if err := Run(os.Args[1:]); err != nil {
		os.Exit(1)
	}
}
