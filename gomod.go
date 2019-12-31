package gore

import (
	"bufio"
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func (s *Session) initGoMod() error {
	hasMod, replaces, err := getModReplaces()
	if err != nil || !hasMod {
		return err
	}

	tempModule := filepath.Base(s.tempDir)
	goModPath := filepath.Join(s.tempDir, "go.mod")

	mod := "module " + tempModule + "\n" + strings.Join(replaces, "\n")

	return ioutil.WriteFile(goModPath, []byte(mod), 0644)
}

func getModReplaces() (hasMod bool, replaces []string, err error) {
	pwd, err := os.Getwd()
	if err != nil {
		return
	}

	file, err := os.Open(filepath.Join(pwd, "go.mod"))
	if err != nil {
		if os.IsNotExist(err) {
			err = nil
		}
		return
	}
	defer file.Close()

	out, err := exec.Command("go", "list", "-m", "all").Output()
	s := bufio.NewScanner(bytes.NewReader(out))

	s.Scan()
	module := s.Text()
	if module == "" {
		return
	}

	hasMod = true
	replaces = append(replaces, "replace "+module+" => "+strconv.Quote(pwd))

	for s.Scan() {
		replace := s.Text()
		if strings.Contains(replace, "=>") {
			replaces = append(replaces, "replace "+replace)
		}
	}

	return
}
