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
	replaces, err := getModReplaces()
	if err != nil {
		return err
	}

	tempModule := filepath.Base(s.tempDir)
	goModPath := filepath.Join(s.tempDir, "go.mod")

	mod := "module " + tempModule + "\n" + strings.Join(replaces, "\n")

	return ioutil.WriteFile(goModPath, []byte(mod), 0644)
}

func getModReplaces() (replaces []string, err error) {
	pwd, err := os.Getwd()
	if err != nil {
		return
	}

	root := findModuleRoot(pwd)
	if root == "" {
		return
	}

	out, err := exec.Command("go", "list", "-m", "all").Output()
	s := bufio.NewScanner(bytes.NewReader(out))

	s.Scan()
	module := s.Text()
	if module == "" {
		return
	}

	replaces = append(replaces, "replace "+module+" => "+strconv.Quote(root))

	for s.Scan() {
		replace := s.Text()
		if strings.Contains(replace, "=>") {
			replaces = append(replaces, "replace "+replace)
		}
	}

	return
}

func findModuleRoot(dir string) string {
	for {
		if fi, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil && !fi.IsDir() {
			return dir
		}
		d := filepath.Dir(dir)
		if d == dir {
			return ""
		}
		dir = d
	}
}
