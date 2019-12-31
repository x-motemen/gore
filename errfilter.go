package gore

import (
	"bytes"
	"io"

	"golang.org/x/text/transform"
)

func newErrFilter(w io.Writer) io.WriteCloser {
	return transform.NewWriter(w, &errTransformer{})
}

type errTransformer struct{}

func (w *errTransformer) Transform(dst, src []byte, atEOF bool) (nDst, nSrc int, err error) {
	var i int
	for {
		if atEOF {
			if i = len(src) - 1; i < 0 {
				break
			}
		} else {
			if i = bytes.IndexByte(src, '\n'); i < 0 {
				err = transform.ErrShortSrc
				break
			}
		}
		res := replaceErrMsg(src[:i+1])
		if nDst+len(res) > len(dst) {
			err = transform.ErrShortDst
			break
		}
		src = src[i+1:]
		nSrc += i + 1
		nDst += copy(dst[nDst:], res)
		if len(src) == 0 {
			break
		}
	}
	return
}

func (w *errTransformer) Reset() {}

func replaceErrMsg(p []byte) []byte {
	if bytes.HasPrefix(p, []byte("# command-line-arguments")) {
		return nil
	}
	if cs := "build command-line-arguments: "; bytes.HasPrefix(p, []byte(cs)) {
		return p[len(cs):]
	}
	if bytes.HasPrefix(p, []byte(`warning: pattern "all" matched no module dependencies`)) {
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
