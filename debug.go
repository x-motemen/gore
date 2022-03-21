//go:build debug
// +build debug

package gore

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

func debugf(format string, args ...interface{}) {
	_, file, line, ok := runtime.Caller(1)

	if ok {
		format = fmt.Sprintf("%s:%d %s", filepath.Base(file), line, format)
	}

	fmt.Fprintf(os.Stderr, format+"\n", args...)
}
