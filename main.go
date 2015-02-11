package main

// TODO:
// - "declared and not used" error
// - import

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"go/ast"
	"go/parser"
	"go/printer"
	"go/scanner"
	"go/token"
	"golang.org/x/tools/go/ast/astutil"

	"github.com/bobappleyard/readline"
)

const appName = "gore"

func debugf(format string, args ...interface{}) {
	_, file, line, ok := runtime.Caller(1)
	if ok {
		format = fmt.Sprintf("%s:%d %s", filepath.Base(file), line, format)
	}

	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

func main() {
	readline.Completer = func(q, ctx string) []string {
		debugf("q=%q ctx=%q", q, ctx)
		return []string{}
	}

	s := NewSession()

	rl := readline.NewReader()
	line := ""

	for {
		buf := make([]byte, 8192)
		_, err := rl.Read(buf) // TODO: check n
		if err == io.EOF {
			break
		} else if err != nil {
			fmt.Fprintf(os.Stderr, "fatal: %s", err)
			os.Exit(1)
		}

		p := bytes.IndexByte(buf, '\x00')
		if line == "" {
			line = string(buf[0:p])
		} else {
			line = line + "\n" + string(buf[0:p])
		}

		v, err := s.Eval(line)
		if err != nil {
			fmt.Println(err)
		}

		if err == nil || err != ErrContinue {
			fmt.Printf("%#v\n", v)
			readline.AddHistory(line)
			rl = readline.NewReader()
			line = ""
		}
	}
}

type Session struct {
	FilePath   string
	File       *ast.File
	Fset       *token.FileSet
	MainBody   *ast.BlockStmt
	ImportDecl *ast.GenDecl

	Source         *bytes.Buffer
	LastBodyLength int
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
	s.Source = bytes.NewBufferString(initialSource)

	// s.FilePath, err = tempFile()
	s.FilePath = "_tmp/session.go"
	if err != nil {
		panic(err)
	}

	s.File, err = parser.ParseFile(s.Fset, "session.go", initialSource, parser.Mode(0))
	if err != nil {
		panic(err)
	}

	mainFunc := s.File.Decls[len(s.File.Decls)-1].(*ast.FuncDecl)
	s.MainBody = mainFunc.Body
	s.ImportDecl = s.File.Decls[0].(*ast.GenDecl)

	return s
}

func (s *Session) BuildRunFile() error {
	s.Source = new(bytes.Buffer)
	debugf("specs :: %#v", s.File.Decls[0])
	printer.Fprint(s.Source, s.Fset, s.File)

	f, err := os.Create(s.FilePath)
	if err != nil {
		return err
	}

	_, err = f.Write(s.Source.Bytes())
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	printer.Fprint(&buf, s.Fset, s.MainBody.List)
	debugf("%q", buf.String())

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

func (s *Session) injectExpr(in string) error {
	expr, err := parser.ParseExpr(in)
	if err != nil {
		return err
	}

	normalizeNode(expr)
	stmt := &ast.ExprStmt{
		X: &ast.CallExpr{
			Fun:  ast.NewIdent("p"),
			Args: []ast.Expr{expr},
		},
	}
	s.MainBody.List = append(s.MainBody.List, stmt)
	return nil
}

func (s *Session) injectStmt(in string) error {
	src := fmt.Sprintf("package X; func X() { %s }", in)
	f, err := parser.ParseFile(s.Fset, "stmt.go", src, parser.Mode(0))

	var enclosingFunc *ast.FuncDecl
	if f != nil {
		enclosingFunc = f.Decls[0].(*ast.FuncDecl)
	}

	if err != nil {
		debugf("%#v", enclosingFunc.Body.List[0])
		return err
	}

	s.MainBody.List = append(s.MainBody.List, enclosingFunc.Body.List...)

	return nil
}

type Error string

const (
	ErrContinue Error = "<continue input>"
)

func (e Error) Error() string {
	return string(e)
}

func (s *Session) handleImport(in string) bool {
	if !strings.HasPrefix(in, "import ") {
		return false
	}

	path := in[len("import "):]
	path = strings.Trim(path, `"`)

	astutil.AddNamedImport(s.Fset, s.File, "_", path)

	return true
}

func (s *Session) Eval(in string) (interface{}, error) {
	debugf("eval >>> %q", in)

	imported := s.handleImport(in)

	if !imported {
		if err := s.injectExpr(in); err != nil {
			debugf("expr err = %s", err)

			err := s.injectStmt(in)
			if err != nil {
				if _, ok := err.(scanner.ErrorList); ok {
					return nil, ErrContinue
				}
				debugf("stmt err = %s", err)
			}
		}
	}

	err := s.BuildRunFile()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// if failed with status 2, remove the last statement
			if st, ok := exitErr.ProcessState.Sys().(syscall.WaitStatus); ok {
				if st.ExitStatus() == 2 {
					debugf("got exit status 2, popping out last input")
					s.MainBody.List = s.MainBody.List[0:s.LastBodyLength]
				}
			}
		}
	} else {
		s.LastBodyLength = len(s.MainBody.List)
	}

	return nil, err
}

func normalizeNode(node ast.Node) {
	// TODO remove token.Pos information
}
