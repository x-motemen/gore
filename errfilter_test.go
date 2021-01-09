package gore

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestErrFilter(t *testing.T) {
	testCases := []struct {
		id, src, expected string
	}{
		{
			"simple",
			"foobar",
			"foobar",
		},
		{
			"new line",
			"foobar\nbaz\nqux\n",
			"foobar\nbaz\nqux\n",
		},
		{
			"command-line-arguments",
			"# command-line-arguments foo\nbar\nbuild command-line-arguments: baz\nqux",
			"bar\nbaz\nqux",
		},
		{
			"gore_session.go",
			"/tmp/gore_session.go:10:24: undefined: foo",
			"undefined: foo",
		},
		{
			"command-line-arguments and gore_session.go",
			"# command-line-arguments foo\n/tmp/gore_session.go:10:24: undefined: foo",
			"undefined: foo",
		},
		{
			"no module dependencies warning",
			"warning: pattern \"all\" matched no module dependencies\nwarning: pattern \"all\" matched no module depend",
			"warning: pattern \"all\" matched no module depend",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.id, func(t *testing.T) {
			out := new(bytes.Buffer)
			w := newErrFilter(out)
			_, err := w.Write([]byte(tc.src))
			require.NoError(t, err)
			err = w.Close()
			require.NoError(t, err)
			require.Equal(t, tc.expected, out.String())
		})
	}
}
