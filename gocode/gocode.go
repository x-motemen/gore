// Package gocode is an interface to github.com/nsf/gocode.
package gocode

import (
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
)

// DefaultCompleter is a default Completer with gocode's path set to "gocode".
var DefaultCompleter = Completer{
	GocodePath: "gocode",
}

// Completer is the interface to gocode that this package provides.
type Completer struct {
	// The path to gocode
	GocodePath  string
	unavailable bool
}

// Result represents a completion result of Query().
type Result struct {
	// Cursor position within Candidates
	Cursor int
	// The list of Candidates
	Candidates []Candidate
}

// Candidate is resulting entries from gocode.
type Candidate struct {
	// One of "package", "func", "type", "var", "const".
	Class string `json:"class"`
	// The name of the candidate.
	Name string `json:"name"`
	// The type (in Go) of the candidate.
	Type string `json:"type"`
}

// Query asks gocode for completion of Go code source for a cursor position cursor.
func Query(source []byte, cursor int) (*Result, error) {
	return DefaultCompleter.Query(source, cursor)
}

// Available checks if gocode executable is available or not.
func Available() bool {
	return DefaultCompleter.Available()
}

// Query asks gocode for completion of Go code source for a cursor position cursor.
func (c *Completer) Query(source []byte, cursor int) (*Result, error) {
	cmd := exec.Command(c.GocodePath, "-f=json", "autocomplete", fmt.Sprintf("%d", cursor))

	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	err = writeClose(in, source)
	if err != nil {
		return nil, err
	}

	out, err := cmd.Output()
	if err != nil {
		if _, ok := err.(*exec.Error); ok {
			// cannot invoke gocode
			c.unavailable = true
		}
		return nil, err
	}

	var result Result
	err = json.Unmarshal(out, &result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

// Available checks if gocode executable is available or not.
func (c *Completer) Available() bool {
	if c.unavailable {
		return false
	}

	_, err := exec.LookPath(c.GocodePath)
	if err != nil {
		c.unavailable = true
		return false
	}

	return true
}

func writeClose(w io.WriteCloser, buf []byte) error {
	_, err := w.Write(buf)
	if err != nil {
		return err
	}
	return w.Close()
}

// UnmarshalJSON decodes JSON bytes text to Result.
func (r *Result) UnmarshalJSON(text []byte) error {
	result := []json.RawMessage{}

	err := json.Unmarshal(text, &result)
	if err != nil {
		return err
	}

	if len(result) < 2 {
		return nil
	}

	err = json.Unmarshal(result[0], &r.Cursor)
	if err != nil {
		return err
	}

	r.Candidates = make([]Candidate, 0)
	return json.Unmarshal(result[1], &r.Candidates)
}
