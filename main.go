/*
Yet another Go REPL that works nicely. Featured with line editing, code completion and more.

Usage

When started, a prompt is shown waiting for input. Enter any statement or expression to proceed.
If an expression is given or any variables are assigned or defined, their data will be pretty-printed.

Some special functionalities are provided as commands, which starts with colons:

	:import <package path>  Imports a package
	:print                  Prints current source code
	:write [<filename>]     Writes out current code
	:doc <target>           Shows documentation for an expression or package name given
	:help                   Lists commands
	:quit                   Quit the session
*/
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"go/ast"
	"go/build"
	"go/importer"
	"go/parser"
	"go/printer"
	"go/scanner"
	"go/token"
	"go/types"

	"golang.org/x/tools/imports"

	"github.com/mitchellh/go-homedir"
	"github.com/motemen/go-quickfix"
)

const version = "0.2.6"
const printerName = "__gore_p"

var (
	flagAutoImport = flag.Bool("autoimport", false, "formats and adjusts imports automatically")
	flagExtFiles   = flag.String("context", "",
		"import packages, functions, variables and constants from external golang source files")
	flagPkg = flag.String("pkg", "", "specify a package where the session will be run inside")
)

func main() {
	flag.Parse()

	s, err := NewSession()
	if err != nil {
		panic(err)
	}

	fmt.Printf("gore version %s  :help for help\n", version)

	if *flagExtFiles != "" {
		extFiles := strings.Split(*flagExtFiles, ",")
		s.includeFiles(extFiles)
	}

	if *flagPkg != "" {
		err := s.includePackage(*flagPkg)
		if err != nil {
			errorf("-pkg: %s", err)
			os.Exit(1)
		}
	}

	rl := newContLiner()
	defer rl.Close()

	var historyFile string
	home, err := homeDir()
	if err != nil {
		errorf("home: %s", err)
	} else {
		historyFile = filepath.Join(home, "history")

		f, err := os.Open(historyFile)
		if err != nil {
			if !os.IsNotExist(err) {
				errorf("%s", err)
			}
		} else {
			_, err := rl.ReadHistory(f)
			if err != nil {
				errorf("while reading history: %s", err)
			}
		}
	}

	rl.SetWordCompleter(s.completeWord)

	for {
		in, err := rl.Prompt()
		if err != nil {
			if err == io.EOF {
				break
			}
			fmt.Fprintf(os.Stderr, "fatal: %s", err)
			os.Exit(1)
		}

		if in == "" {
			continue
		}

		if err := rl.Reindent(); err != nil {
			fmt.Fprintf(os.Stderr, "error: %s\n", err)
			rl.Clear()
			continue
		}

		err = s.Eval(in)
		if err != nil {
			if err == ErrContinue {
				continue
			} else if err == ErrQuit {
				break
			}
			fmt.Println(err)
		}
		rl.Accepted()
	}

	if historyFile != "" {
		err := os.MkdirAll(filepath.Dir(historyFile), 0755)
		if err != nil {
			errorf("%s", err)
		} else {
			f, err := os.Create(historyFile)
			if err != nil {
				errorf("%s", err)
			} else {
				_, err := rl.WriteHistory(f)
				if err != nil {
					errorf("while saving history: %s", err)
				}
			}
		}
	}
}

func homeDir() (home string, err error) {
	home = os.Getenv("GORE_HOME")
	if home != "" {
		return
	}

	home, err = homedir.Dir()
	if err != nil {
		return
	}

	home = filepath.Join(home, ".gore")
	return
}

type Session struct {
	FilePath       string
	File           *ast.File
	Fset           *token.FileSet
	Types          *types.Config
	TypeInfo       types.Info
	ExtraFilePaths []string
	ExtraFiles     []*ast.File

	mainBody         *ast.BlockStmt
	storedBodyLength int
}

const initialSourceTemplate = `
package main

import %q

func ` + printerName + `(xx ...interface{}) {
	for _, x := range xx {
		%s
	}
}

func main() {
}
`

// printerPkgs is a list of packages that provides
// pretty printing function. Preceding first.
var printerPkgs = []struct {
	path string
	code string
}{
	{"github.com/k0kubun/pp", `pp.Println(x)`},
	{"github.com/davecgh/go-spew/spew", `spew.Printf("%#v\n", x)`},
	{"fmt", `fmt.Printf("%#v\n", x)`},
}

func NewSession() (*Session, error) {
	var err error

	s := &Session{
		Fset: token.NewFileSet(),
		Types: &types.Config{
			Importer: importer.Default(),
		},
	}

	s.FilePath, err = tempFile()
	if err != nil {
		return nil, err
	}

	var initialSource string
	for _, pp := range printerPkgs {
		_, err := s.Types.Importer.Import(pp.path)
		if err == nil {
			initialSource = fmt.Sprintf(initialSourceTemplate, pp.path, pp.code)
			break
		}
		debugf("could not import %q: %s", pp.path, err)
	}

	if initialSource == "" {
		return nil, fmt.Errorf(`Could not load pretty printing package (even "fmt"; something is wrong)`)
	}

	s.File, err = parser.ParseFile(s.Fset, "gore_session.go", initialSource, parser.Mode(0))
	if err != nil {
		return nil, err
	}

	s.mainBody = s.mainFunc().Body

	return s, nil
}

func (s *Session) mainFunc() *ast.FuncDecl {
	return s.File.Scope.Lookup("main").Decl.(*ast.FuncDecl)
}

func (s *Session) Run() error {
	f, err := os.Create(s.FilePath)
	if err != nil {
		return err
	}

	err = printer.Fprint(f, s.Fset, s.File)
	if err != nil {
		return err
	}

	return goRun(append(s.ExtraFilePaths, s.FilePath))
}

func tempFile() (string, error) {
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		return "", err
	}

	err = os.MkdirAll(dir, 0755)
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "gore_session.go"), nil
}

func goRun(files []string) error {
	args := append([]string{"run"}, files...)
	debugf("go %s", strings.Join(args, " "))
	cmd := exec.Command("go", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (s *Session) evalExpr(in string) (ast.Expr, error) {
	expr, err := parser.ParseExpr(in)
	if err != nil {
		return nil, err
	}

	stmt := &ast.ExprStmt{
		X: &ast.CallExpr{
			Fun:  ast.NewIdent(printerName),
			Args: []ast.Expr{expr},
		},
	}

	s.appendStatements(stmt)

	return expr, nil
}

func isNamedIdent(expr ast.Expr, name string) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == name
}

func (s *Session) evalStmt(in string) error {
	src := fmt.Sprintf("package P; func F() { %s }", in)
	f, err := parser.ParseFile(s.Fset, "stmt.go", src, parser.Mode(0))
	if err != nil {
		return err
	}

	enclosingFunc := f.Scope.Lookup("F").Decl.(*ast.FuncDecl)
	stmts := enclosingFunc.Body.List

	if len(stmts) > 0 {
		debugf("evalStmt :: %s", showNode(s.Fset, stmts))
		lastStmt := stmts[len(stmts)-1]
		// print last assigned/defined values
		if assign, ok := lastStmt.(*ast.AssignStmt); ok {
			vs := []ast.Expr{}
			for _, v := range assign.Lhs {
				if !isNamedIdent(v, "_") {
					vs = append(vs, v)
				}
			}
			if len(vs) > 0 {
				printLastValues := &ast.ExprStmt{
					X: &ast.CallExpr{
						Fun:  ast.NewIdent(printerName),
						Args: vs,
					},
				}
				stmts = append(stmts, printLastValues)
			}
		}
	}

	s.appendStatements(stmts...)

	return nil
}

func (s *Session) appendStatements(stmts ...ast.Stmt) {
	s.mainBody.List = append(s.mainBody.List, stmts...)
}

type Error string

const (
	ErrContinue Error = "<continue input>"
	ErrQuit     Error = "<quit session>"
)

func (e Error) Error() string {
	return string(e)
}

func (s *Session) source(space bool) (string, error) {
	normalizeNodePos(s.mainFunc())

	var config *printer.Config
	if space {
		config = &printer.Config{
			Mode:     printer.UseSpaces,
			Tabwidth: 4,
		}
	} else {
		config = &printer.Config{
			Tabwidth: 8,
		}
	}

	var buf bytes.Buffer
	err := config.Fprint(&buf, s.Fset, s.File)
	return buf.String(), err
}

func (s *Session) reset() error {
	source, err := s.source(false)
	if err != nil {
		return err
	}

	file, err := parser.ParseFile(s.Fset, "gore_session.go", source, parser.Mode(0))
	if err != nil {
		return err
	}

	s.File = file
	s.mainBody = s.mainFunc().Body

	return nil
}

func (s *Session) Eval(in string) error {
	debugf("eval >>> %q", in)

	s.clearQuickFix()
	s.storeMainBody()

	var commandRan bool
	for _, command := range commands {
		arg := strings.TrimPrefix(in, ":"+command.name)
		if arg == in {
			continue
		}

		if arg == "" || strings.HasPrefix(arg, " ") {
			arg = strings.TrimSpace(arg)
			err := command.action(s, arg)
			if err != nil {
				if err == ErrQuit {
					return err
				}
				errorf("%s: %s", command.name, err)
			}
			commandRan = true
			break
		}
	}

	if commandRan {
		s.doQuickFix()
		return nil
	}

	if _, err := s.evalExpr(in); err != nil {
		debugf("expr :: err = %s", err)

		err := s.evalStmt(in)
		if err != nil {
			debugf("stmt :: err = %s", err)

			if _, ok := err.(scanner.ErrorList); ok {
				return ErrContinue
			}
		}
	}

	if *flagAutoImport {
		s.fixImports()
	}
	s.doQuickFix()

	err := s.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// if failed with status 2, remove the last statement
			if st, ok := exitErr.ProcessState.Sys().(syscall.WaitStatus); ok {
				if st.ExitStatus() == 2 {
					debugf("got exit status 2, popping out last input")
					s.restoreMainBody()
				}
			}
		}
		errorf("%s", err)
	}

	return err
}

// storeMainBody stores current state of code so that it can be restored
// actually it saves the length of statements inside main()
func (s *Session) storeMainBody() {
	s.storedBodyLength = len(s.mainBody.List)
}

func (s *Session) restoreMainBody() {
	s.mainBody.List = s.mainBody.List[0:s.storedBodyLength]
}

// includeFiles imports packages and funcsions from multiple golang source
func (s *Session) includeFiles(files []string) {
	for _, file := range files {
		s.includeFile(file)
	}
}

func (s *Session) includeFile(file string) {
	content, err := ioutil.ReadFile(file)
	if err != nil {
		errorf("%s", err)
		return
	}

	if err = s.importPackages(content); err != nil {
		errorf("%s", err)
		return
	}

	if err = s.importFile(content); err != nil {
		errorf("%s", err)
	}

	infof("added file %s", file)
}

// importPackages includes packages defined on external file into main file
func (s *Session) importPackages(src []byte) error {
	astf, err := parser.ParseFile(s.Fset, "", src, parser.Mode(0))
	if err != nil {
		return err
	}

	for _, imt := range astf.Imports {
		debugf("import package: %s", imt.Path.Value)
		actionImport(s, imt.Path.Value)
	}

	return nil
}

// importFile adds external golang file to goRun target to use its function
func (s *Session) importFile(src []byte) error {
	// Don't need to same directory
	tmp, err := ioutil.TempFile(filepath.Dir(s.FilePath), "gore_extarnal_")
	if err != nil {
		return err
	}

	ext := tmp.Name() + ".go"

	f, err := parser.ParseFile(s.Fset, ext, src, parser.Mode(0))
	if err != nil {
		return err
	}

	// rewrite to package main
	f.Name.Name = "main"

	// remove func main()
	for i, decl := range f.Decls {
		if funcDecl, ok := decl.(*ast.FuncDecl); ok {
			if isNamedIdent(funcDecl.Name, "main") {
				f.Decls = append(f.Decls[0:i], f.Decls[i+1:]...)
				// main() removed from this file, we may have to
				// remove some unsed import's
				quickfix.QuickFix(s.Fset, []*ast.File{f})
				break
			}
		}
	}

	out, err := os.Create(ext)
	if err != nil {
		return err
	}
	defer out.Close()

	err = printer.Fprint(out, s.Fset, f)
	if err != nil {
		return err
	}

	debugf("import file: %s", ext)
	s.ExtraFilePaths = append(s.ExtraFilePaths, ext)
	s.ExtraFiles = append(s.ExtraFiles, f)

	return nil
}

// fixImports formats and adjusts imports for the current AST.
func (s *Session) fixImports() error {

	var buf bytes.Buffer
	err := printer.Fprint(&buf, s.Fset, s.File)
	if err != nil {
		return err
	}

	formatted, err := imports.Process("", buf.Bytes(), nil)
	if err != nil {
		return err
	}

	s.File, err = parser.ParseFile(s.Fset, "", formatted, parser.Mode(0))
	if err != nil {
		return err
	}
	s.mainBody = s.mainFunc().Body

	return nil
}

func (s *Session) includePackage(path string) error {
	pkg, err := build.Import(path, ".", 0)
	if err != nil {
		var err2 error
		pkg, err2 = build.ImportDir(path, 0)
		if err2 != nil {
			return err // return package path import error, not directory import error as build.Import can also import directories if "./foo" is specified
		}
	}

	files := make([]string, len(pkg.GoFiles))
	for i, f := range pkg.GoFiles {
		files[i] = filepath.Join(pkg.Dir, f)
	}
	s.includeFiles(files)

	return nil
}
