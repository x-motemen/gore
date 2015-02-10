package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/bobappleyard/readline"
)

const appName = "gore"

func debugf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

func main() {
	prompt := "> "
	readline.Completer = func(q, ctx string) []string {
		debugf("q=%q ctx=%q", q, ctx)
		return []string{}
	}

	s := NewSession()

	for {
		line, err := readline.String(prompt)
		if err == io.EOF {
			break
		} else if err != nil {
			fmt.Fprintf(os.Stderr, "fatal: %s", err)
			os.Exit(1)
		}

		readline.AddHistory(line)

		v, err := s.Eval(line)
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Printf("%#v\n", v)
		}
	}
}

type Session struct {
	FilePath string
	File     *ast.File
	Fset     *token.FileSet
	MainBody *ast.BlockStmt
}

const initialSource = `
package main

import "github.com/k0kubun/pp"

func p(xx ...interface{}) {
	pp.Println(xx...)
}

func main() {
}
`

func NewSession() *Session {
	var err error

	s := &Session{}
	s.Fset = token.NewFileSet()

	// s.FilePath, err = tempFile()
	s.FilePath = "_tmp/session.go"
	if err != nil {
		panic(err)
	}

	s.File, err = parser.ParseFile(s.Fset, "session.go", initialSource, parser.ParseComments)
	if err != nil {
		panic(err)
	}

	mainFunc := s.File.Decls[len(s.File.Decls)-1].(*ast.FuncDecl)
	s.MainBody = mainFunc.Body

	return s
}

func (s *Session) RunFile() error {
	f, err := os.Create(s.FilePath)
	if err != nil {
		return err
	}
	printer.Fprint(f, s.Fset, s.File)

	return goRun(s.FilePath)
}

func tempFile() (string, error) {
	dir, err := ioutil.TempDir("", appName)
	if err != nil {
		return "", err
	}

	err = os.MkdirAll(dir, 0755)
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "session.go"), nil
}

func goRun(file string) error {
	debugf("go run %s", file)

	cmd := exec.Command("go", "run", file)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (s *Session) Eval(in string) (interface{}, error) {
	debugf("eval %q", in)

	expr, err := parser.ParseExpr(in)
	if err == nil {
		normalizeNode(expr)
		stmt := &ast.ExprStmt{
			X: &ast.CallExpr{
				Fun:  ast.NewIdent("p"),
				Args: []ast.Expr{expr},
			},
		}
		s.MainBody.List = append(s.MainBody.List, stmt)
	} else {
		debugf("%s", err)
	}

	err = s.RunFile()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// if failed with status 2, remove the last statement
			if st, ok := exitErr.ProcessState.Sys().(syscall.WaitStatus); ok {
				if st.ExitStatus() == 2 {
					s.MainBody.List = s.MainBody.List[0 : len(s.MainBody.List)-1]
				}
			}
		}
	}

	return nil, err
}

func normalizeNode(node ast.Node) {
	// TODO remove token.Pos information
}
