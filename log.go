package gore

import (
	"fmt"
	"os"
)

func errorf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
}

func infof(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}
