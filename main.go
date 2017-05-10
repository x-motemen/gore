/*
Yet another Go REPL that works nicely. Featured with line editing, code completion and more.

Usage

When started, a prompt is shown waiting for input. Enter any statement or expression to proceed.
If an expression is given or any variables are assigned or defined, their data will be pretty-printed.

Some special functionalities are provided as commands, which starts with colons:

	:import <package path>  Imports a package
	:print                  Prints current source code
	:write [<filename>]     Writes out current code
	:doc <target>           Shows documentation for an expression or package name given
	:help                   Lists commands
	:quit                   Quit the session
*/
package main

import (
	"flag"

	"github.com/motemen/gore/console"
)

func main() {
	flagAutoImport := flag.Bool("autoimport", false, "formats and adjusts imports automatically")
	flagExtFiles := flag.String("context", "",
		"import packages, functions, variables and constants from external golang source files")
	flagPkg := flag.String("pkg", "", "specify a package where the session will be run inside")
	flag.Parse()

	console.Run(*flagExtFiles, *flagPkg, *flagAutoImport)
}
