package gore

import (
	"bytes"
	"cmp"
	"encoding/json"
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
		// Check whether the printer package and its entire build closure are
		// available in the local module cache, or can be fetched from the proxy.
		if s.canBuildOffline(pp.path, pp.version) || canAccessGoproxy() {
			// Specifying the version of the printer package improves startup
			// performance by skipping module version fetching. Also allows to
			// use gore in offline environment.
			directives = append(directives, "require "+pp.path+" "+pp.version)
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

// canBuildOffline reports whether path@version and its entire build closure
// are already in the local module cache, by asking the go toolchain to resolve
// them with the network disabled. This resolves the real transitive closure, so
// it does not rely on a hand-maintained list of dependencies.
func (s *Session) canBuildOffline(path, version string) bool {
	defer os.Remove(filepath.Join(s.tempDir, "go.sum"))
	mod := "module " + filepath.Base(s.tempDir) + "\nrequire " + path + " " + version + "\n"
	if err := os.WriteFile(filepath.Join(s.tempDir, "go.mod"), []byte(mod), 0o644); err != nil {
		return false
	}
	cmd := exec.Command("go", "list", "-deps", path)
	cmd.Dir = s.tempDir
	cmd.Env = append(os.Environ(), "GOPROXY=off", "GOFLAGS=-mod=mod")
	return cmd.Run() == nil
}

func canAccessGoproxy() bool {
	entries := strings.FieldsFunc(
		cmp.Or(os.Getenv("GOPROXY"), "https://proxy.golang.org,direct"),
		func(r rune) bool { return r == ',' || r == '|' })
	if len(entries) == 0 {
		return false
	}
	entry := strings.TrimSpace(entries[0])
	switch entry {
	case "", "off", "direct":
		return false
	}
	u, err := url.Parse(entry)
	if err != nil {
		return false
	}
	if u.Scheme == "file" {
		return true
	}
	host := u.Hostname()
	if host == "" {
		return false
	}
	port := u.Port()
	if port == "" {
		if u.Scheme == "http" {
			port = "80"
		} else {
			port = "443"
		}
	}
	addr := net.JoinHostPort(host, port)
	dialer := net.Dialer{Timeout: 5 * time.Second}
	conn, err := dialer.Dial("tcp", addr)
	if err != nil {
		return false
	}
	defer conn.Close()
	return true
}
