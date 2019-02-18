package main

import (
	"bytes"
	"io"
)

type errFilter struct {
	w   io.Writer
	buf []byte
}

func newErrFilter(w io.Writer) *errFilter {
	return &errFilter{w, nil}
}

func (w *errFilter) Write(p []byte) (n int, err error) {
	var m, i int
	for {
		i = bytes.IndexRune(p, '\n')
		if i < 0 {
			break
		}
		if len(w.buf) > 0 {
			m, err = w.w.Write(w.replace(w.buf))
			n += m
			w.buf = nil
			if err != nil {
				return
			}
		}
		m, err = w.w.Write(w.replace(p[:i+1]))
		n += m
		if err != nil {
			return
		}
		p = p[i+1:]
	}
	w.buf = p
	return
}

func (w *errFilter) replace(p []byte) []byte {
	if bytes.HasPrefix(p, []byte("# command-line-arguments")) {
		return nil
	}
	if i := bytes.Index(p, []byte("gore_session.go")); i >= 0 {
		if j := bytes.IndexRune(p[i:], ' '); j >= 0 {
			return p[i+j+1:]
		}
		return p
	}
	return p
}
