package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"go/build"
	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/types"
)

type command struct {
	name     string
	action   func(*Session, string) error
	complete func(string) []string
}

// TODO
// - :edit
// - :undo
// - :reset
// - :type
// - :doc
// - :help
var commands = []command{
	{
		name:     "import",
		action:   actionImport,
		complete: completeImport,
	},
	{
		name:     "print",
		action:   actionPrint,
		complete: nil,
	},
	{
		name:     "write",
		action:   actionWrite,
		complete: nil, // TODO implement
	},
}

func actionImport(s *Session, arg string) error {
	if arg == "" {
		return fmt.Errorf("arg required")
	}

	path := strings.Trim(arg, `"`)

	// check if the package specified by path is importable
	_, err := types.DefaultImport(s.Types.Packages, path)
	if err != nil {
		return err
	}

	astutil.AddImport(s.Fset, s.File, path)

	return nil
}

func completeImport(prefix string) []string {
	result := []string{}
	seen := map[string]bool{}

	d, fn := path.Split(prefix)
	for _, srcDir := range build.Default.SrcDirs() {
		dir := filepath.Join(srcDir, d)

		if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
			if err != nil && !os.IsNotExist(err) {
				errorf("Stat %s: %s", dir, err)
			}
			continue
		}

		entries, err := ioutil.ReadDir(dir)
		if err != nil {
			errorf("ReadDir %s: %s", dir, err)
			continue
		}
		for _, fi := range entries {
			if !fi.IsDir() {
				continue
			}

			name := fi.Name()
			if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") || name == "testdata" {
				continue
			}

			if strings.HasPrefix(name, fn) {
				r := path.Join(d, name)
				if srcDir != gorootSrc {
					// append "/" if this directory is not a repository
					// e.g. does not have VCS directory such as .git or .hg
					// TODO: do not append "/" to subdirectories of repos
					var isRepo bool
					for _, vcsDir := range []string{".git", ".hg", ".svn", ".bzr"} {
						_, err := os.Stat(filepath.Join(srcDir, filepath.FromSlash(r), vcsDir))
						if err == nil {
							isRepo = true
							break
						}
					}
					if !isRepo {
						r = r + "/"
					}
				}

				if !seen[r] {
					result = append(result, r)
					seen[r] = true
				}
			}
		}
	}

	return result
}

func actionPrint(s *Session, _ string) error {
	source, err := s.source(true)
	if err != nil {
		return err
	}

	fmt.Println(source)

	return nil
}

func actionWrite(s *Session, filename string) error {
	source, err := s.source(false)
	if err != nil {
		return err
	}

	if filename == "" {
		filename = fmt.Sprintf("gore_session_%s.go", time.Now().Format("20060102_150405"))
	}

	err = ioutil.WriteFile(filename, []byte(source), 0644)
	if err != nil {
		return err
	}

	infof("Source wrote to %s", filename)

	return nil
}
