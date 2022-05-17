package gore

import (
	"bytes"
	"encoding/json"
	"go/build"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func (s *Session) initGoMod() error {
	tempModule := filepath.Base(s.tempDir)
	goModPath := filepath.Join(s.tempDir, "go.mod")
	directives := s.listModuleDirectives()
	mod := "module " + tempModule + "\n" + strings.Join(directives, "\n")
	return os.WriteFile(goModPath, []byte(mod), 0o644)
}

func (s *Session) listModuleDirectives() []string {
	var directives []string
	for i, pp := range printerPkgs {
		if pp.path == "fmt" {
			continue
		}
		// Check local module caches.
		found := lookupGoModule(pp.path, pp.version)
		if found {
			for _, r := range pp.requires {
				if !lookupGoModule(r.path, r.version) {
					found = false
					break
				}
			}
		}
		if found || canAccessGoproxy() {
			// Specifying the version of the printer package improves startup
			// performance by skipping module version fetching. Also allows to
			// use gore in offline environment.
			directives = append(directives, "require "+pp.path+" "+pp.version)
			for _, r := range pp.requires {
				directives = append(directives, "require "+r.path+" "+r.version)
			}
		} else {
			// If there is no module cache and no network connection, use fmt package.
			printerPkgs = printerPkgs[i+1:]
		}
		// only the first printer is checked (assuming printerPkgs[1] is fmt)
		break
	}
	modules, err := goListAll()
	if err != nil {
		return directives
	}
	for _, m := range modules {
		if m.Main || m.Replace != nil {
			directives = append(directives, "replace "+m.Path+" => "+strconv.Quote(m.Dir))
			s.requiredModules = append(s.requiredModules, m.Path)
		}
	}
	return directives
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

func canAccessGoproxy() bool {
	var host string
	if url, err := url.Parse(getGoproxy()); err != nil {
		host = "proxy.golang.org"
	} else {
		host = url.Hostname()
	}
	addr := net.JoinHostPort(host, "80")
	dialer := net.Dialer{Timeout: 5 * time.Second}
	conn, err := dialer.Dial("tcp", addr)
	if err != nil {
		return false
	}
	defer conn.Close()
	return true
}

func getGoproxy() string {
	if goproxy := os.Getenv("GOPROXY"); goproxy != "" {
		return goproxy
	}
	return "https://proxy.golang.org/"
}
