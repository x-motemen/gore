package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"text/tabwriter"
	"time"
	"unicode"

	"go/ast"
	"go/build"
	"go/types"
	"golang.org/x/tools/go/ast/astutil"
)

type command struct {
	name     string
	action   func(*Session, string) error
	complete func(*Session, string) []string
	arg      string
	document string
}

// TODO
// - :edit
// - :undo
// - :reset
// - :type
var commands []command

func init() {
	commands = []command{
		{
			name:     "import",
			action:   actionImport,
			complete: completeImport,
			arg:      "<package>",
			document: "import a package",
		},
		{
			name:     "print",
			action:   actionPrint,
			document: "print current source",
		},
		{
			name:     "write",
			action:   actionWrite,
			complete: nil, // TODO implement
			arg:      "[<file>]",
			document: "write out current source",
		},
		{
			name:     "doc",
			action:   actionDoc,
			complete: completeDoc,
			arg:      "<expr or pkg>",
			document: "show documentation",
		},
		{
			name:     "help",
			action:   actionHelp,
			document: "show this help",
		},
		{
			name:     "quit",
			action:   actionQuit,
			document: "quit the session",
		},
	}
}

func actionImport(s *Session, arg string) error {
	if arg == "" {
		return fmt.Errorf("arg required")
	}

	if strings.Contains(arg, " ") {
		for _, v := range strings.Fields(arg) {
			if v == "" {
				continue
			}
			if err := actionImport(s, v); err != nil {
				return err
			}
		}

		return nil
	}

	path := strings.Trim(arg, `"`)

	// check if the package specified by path is importable
	_, err := s.Types.Importer.Import(path)
	if err != nil {
		return err
	}

	astutil.AddImport(s.Fset, s.File, path)

	return nil
}

var gorootSrc = filepath.Join(filepath.Clean(runtime.GOROOT()), "src")

func completeImport(s *Session, prefix string) []string {
	result := []string{}
	seen := map[string]bool{}

	p := strings.LastIndexFunc(prefix, unicode.IsSpace) + 1

	d, fn := path.Split(prefix[p:])
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
					result = append(result, prefix[:p]+r)
					seen[r] = true
				}
			}
		}
	}

	return result
}

func completeDoc(s *Session, prefix string) []string {
	pos, cands, err := s.completeCode(prefix, len(prefix), false)
	if err != nil {
		errorf("completeCode: %s", err)
		return nil
	}

	result := make([]string, 0, len(cands))
	for _, c := range cands {
		result = append(result, prefix[0:pos]+c)
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

func actionDoc(s *Session, in string) error {
	s.clearQuickFix()

	s.storeMainBody()
	defer s.restoreMainBody()

	expr, err := s.evalExpr(in)
	if err != nil {
		return err
	}

	s.TypeInfo = types.Info{
		Types:  make(map[ast.Expr]types.TypeAndValue),
		Uses:   make(map[*ast.Ident]types.Object),
		Defs:   make(map[*ast.Ident]types.Object),
		Scopes: make(map[ast.Node]*types.Scope),
	}
	_, err = s.Types.Check("_tmp", s.Fset, []*ast.File{s.File}, &s.TypeInfo)
	if err != nil {
		debugf("typecheck error (ignored): %s", err)
	}

	// :doc patterns:
	// - "json" -> "encoding/json" (package name)
	// - "json.Encoder" -> "encoding/json", "Encoder" (package member)
	// - "json.NewEncoder(nil).Encode" -> "encoding/json", "Decode" (package type member)
	var docObj types.Object
	if sel, ok := expr.(*ast.SelectorExpr); ok {
		// package member, package type member
		docObj = s.TypeInfo.ObjectOf(sel.Sel)
	} else if t := s.TypeInfo.TypeOf(expr); t != nil && t != types.Typ[types.Invalid] {
		for {
			if pt, ok := t.(*types.Pointer); ok {
				t = pt.Elem()
			} else {
				break
			}
		}
		switch t := t.(type) {
		case *types.Named:
			docObj = t.Obj()
		case *types.Basic:
			// builtin types
			docObj = types.Universe.Lookup(t.Name())
		}
	} else if ident, ok := expr.(*ast.Ident); ok {
		// package name
		mainScope := s.TypeInfo.Scopes[s.mainFunc().Type]
		_, docObj = mainScope.LookupParent(ident.Name, ident.NamePos)
	}

	if docObj == nil {
		return fmt.Errorf("cannot determine the document location")
	}

	debugf("doc :: obj=%#v", docObj)

	var pkgPath, objName string
	if pkgName, ok := docObj.(*types.PkgName); ok {
		pkgPath = pkgName.Imported().Path()
	} else {
		if pkg := docObj.Pkg(); pkg != nil {
			pkgPath = pkg.Path()
		} else {
			pkgPath = "builtin"
		}
		objName = docObj.Name()
	}

	debugf("doc :: %q %q", pkgPath, objName)

	args := []string{pkgPath}
	if objName != "" {
		args = append(args, objName)
	}

	godoc := exec.Command("godoc", args...)
	godoc.Stderr = os.Stderr

	// TODO just use PAGER?
	if pagerCmd := os.Getenv("GORE_PAGER"); pagerCmd != "" {
		r, err := godoc.StdoutPipe()
		if err != nil {
			return err
		}

		pager := exec.Command(pagerCmd)
		pager.Stdin = r
		pager.Stdout = os.Stdout
		pager.Stderr = os.Stderr

		err = pager.Start()
		if err != nil {
			return err
		}

		err = godoc.Run()
		if err != nil {
			return err
		}

		return pager.Wait()
	} else {
		godoc.Stdout = os.Stdout
		return godoc.Run()
	}
}

func actionHelp(s *Session, _ string) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 8, 4, ' ', 0)
	for _, command := range commands {
		cmd := ":" + command.name
		if command.arg != "" {
			cmd = cmd + " " + command.arg
		}
		w.Write([]byte("    " + cmd + "\t" + command.document + "\n"))
	}
	w.Flush()

	return nil
}

func actionQuit(s *Session, _ string) error {
	return ErrQuit
}
