package gore

import "io"

// Option for Gore
type Option func(*Gore)

// AutoImport option
func AutoImport(autoImport bool) Option {
	return func(g *Gore) {
		g.autoImport = autoImport
	}
}

// ExtFiles option
func ExtFiles(extFiles string) Option {
	return func(g *Gore) {
		g.extFiles = extFiles
	}
}

// PackageName option
func PackageName(packageName string) Option {
	return func(g *Gore) {
		g.packageName = packageName
	}
}

// OutWriter option
func OutWriter(outWriter io.Writer) Option {
	return func(g *Gore) {
		g.outWriter = outWriter
	}
}

// ErrWriter option
func ErrWriter(errWriter io.Writer) Option {
	return func(g *Gore) {
		g.errWriter = errWriter
	}
}
