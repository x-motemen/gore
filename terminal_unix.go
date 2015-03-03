// +build !windows

package main

import (
	"fmt"
)

func cursorUp() {
	fmt.Print("\x1b[1A")
}

func eraseInLine() {
	fmt.Print("\x1b[0K")
}
