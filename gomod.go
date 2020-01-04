package gore

import (
	"bytes"
	"encoding/json"
	"go/build"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func (s *Session) initGoMod() error {
	tempModule := filepath.Base(s.tempDir)
	goModPath := filepath.Join(s.tempDir, "go.mod")
	directives := getModuleDirectives()

	mod := "module " + tempModule + "\n" + strings.Join(directives, "\n")
	return ioutil.WriteFile(goModPath, []byte(mod), 0644)
}

func getModuleDirectives() (directives []string) {
	modules, err := goListAll()
	if err != nil {
		return
	}
	for _, m := range modules {
		if m.Main || m.Replace != nil {
			directives = append(directives, "replace "+m.Path+" => "+strconv.Quote(m.Dir))
		}
	}
	for _, pp := range printerPkgs {
		if pp.path == "fmt" {
			continue
		}
		found := lookupGoModule(pp.path, pp.version)
		for _, r := range pp.requires {
			if found && !lookupGoModule(r.path, r.version) {
				found = false
				break
			}
		}
		if found {
			// Specifying the version of the printer package improves startup
			// performance by skipping module version fetching. Also allows to
			// use gore in offline environment.
			directives = append(directives, "require "+pp.path+" "+pp.version)
			for _, r := range pp.requires {
				directives = append(directives, "require "+r.path+" "+r.version)
			}
			break
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

func lookupGoModule(pkg, version string) bool {
	modDir := filepath.Join(build.Default.GOPATH, "pkg/mod", pkg+"@"+version)
	fi, err := os.Stat(modDir)
	return err == nil && fi.IsDir()
}
