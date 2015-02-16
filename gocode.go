package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
)

var gocode = gocodeCompleter{
	gocodePath: "gocode",
}

type gocodeCompleter struct {
	gocodePath string
	invalid    bool
}

type gocodeResult struct {
	pos     int
	entries []gocodeResultEntry
}

type gocodeResultEntry struct {
	Class string `json:"class"`
	Name  string `json:"name"`
	Type  string `json:"type"`
}

func (r *gocodeResult) UnmarshalJSON(text []byte) error {
	result := make([]json.RawMessage, 0)

	err := json.Unmarshal(text, &result)
	if err != nil {
		return err
	}

	if len(result) < 2 {
		return nil
	}

	err = json.Unmarshal(result[0], &r.pos)
	if err != nil {
		return err
	}

	r.entries = make([]gocodeResultEntry, 0)
	return json.Unmarshal(result[1], &r.entries)
}

func (c *gocodeCompleter) complete(source string, cursor int) ([]string, error) {
	cmd := exec.Command(c.gocodePath, "-f=json", "autocomplete", fmt.Sprintf("%d", cursor))

	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	err = writeCloseString(in, source)
	if err != nil {
		return nil, err
	}

	out, err := cmd.Output()
	if err != nil {
		if _, ok := err.(*exec.Error); ok {
			// cannot invoke gocode
			c.invalid = true
		}
		return nil, err
	}

	debugf("gocode :: %s", out)

	var result gocodeResult
	err = json.Unmarshal(out, &result)
	if err != nil {
		return nil, err
	}

	cands := make([]string, 0, len(result.entries))
	for _, e := range result.entries {
		cand := e.Name[result.pos:]
		if e.Class == "func" {
			cand = cand + "("
		}
		cands = append(cands, cand)
	}

	return cands, nil
}

func writeCloseString(w io.WriteCloser, s string) error {
	_, err := io.WriteString(w, s)
	if err != nil {
		return err
	}
	return w.Close()
}
