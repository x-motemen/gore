package gore

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func (s *Session) initGoMod() error {
	replaces := getModReplaces()
	tempModule := filepath.Base(s.tempDir)
	goModPath := filepath.Join(s.tempDir, "go.mod")

	mod := "module " + tempModule + "\n" + strings.Join(replaces, "\n")
	return ioutil.WriteFile(goModPath, []byte(mod), 0644)
}

func getModReplaces() (replaces []string) {
	modules, err := goListAll()
	if err != nil {
		return
	}
	for _, m := range modules {
		if m.Main || m.Replace != nil {
			replaces = append(replaces, "replace "+m.Path+" => "+strconv.Quote(m.Dir))
		}
	}
	return
}

type goModule struct {
	Path, Dir, Version string
	Main               bool
	Replace            *goModule
}

func goListAll() ([]*goModule, error) {
	cmd := exec.Command("go", "list", "-json", "-m", "all")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	d := json.NewDecoder(bytes.NewReader(out))
	var ms []*goModule
	for {
		m := new(goModule)
		if err := d.Decode(m); err != nil {
			if err == io.EOF {
				return ms, nil
			}
			return nil, err
		}
		ms = append(ms, m)
	}
}
