package gore

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/printer"
	"go/scanner"
	"go/token"
	"go/types"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unicode"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/imports"

	"github.com/motemen/go-quickfix"
)

// Session ...
type Session struct {
	tempDir         string
	tempFilePath    string
	file            *ast.File
	fset            *token.FileSet
	types           *types.Config
	typeInfo        types.Info
	extraFilePaths  []string
	extraFiles      []*ast.File
	autoImport      bool
	requiredModules []string
	mainBody        *ast.BlockStmt
	lastStmts       []ast.Stmt
	lastDecls       []ast.Decl
	stdout          io.Writer
	stderr          io.Writer
}

const printerName = "__gore_p"

const initialSourceTemplate = `
package main

import %q

func ` + printerName + `(xs ...interface{}) {
	for _, x := range xs {
		%s
	}
}

func main() {
}
`

// printerPkgs is a list of packages that provides pretty printing function
// when changing this, read listModuleDirectives carefully
var printerPkgs = []struct {
	path, version string
	requires      []pathVersion
	code          string
}{
	{
		path: "github.com/k0kubun/pp/v3", version: "v3.1.0", code: `pp.Println(x)`,
		requires: []pathVersion{{"github.com/mattn/go-colorable", "v0.1.12"}},
	},
	{path: "fmt", code: `fmt.Printf("%#v\n", x)`},
}

type pathVersion struct {
	path, version string
}

// NewSession creates a new Session.
func NewSession(stdout, stderr io.Writer) (*Session, error) {
	var err error

	s := &Session{stdout: stdout, stderr: stderr}

	s.tempDir, err = os.MkdirTemp("", "gore-")
	if err != nil {
		return s, err
	}
	s.tempFilePath = filepath.Join(s.tempDir, "gore_session.go")

	if err = s.init(); err != nil {
		return s, err
	}

	return s, nil
}

type pkgsImporter struct {
	dir string
}

func (i *pkgsImporter) Import(path string) (*types.Package, error) {
	pkgs, err := packages.Load(&packages.Config{
		Mode:       packages.NeedTypes | packages.NeedDeps,
		Dir:        i.dir,
		BuildFlags: []string{"-mod=mod"},
	}, path)
	if err != nil {
		return nil, err
	}

	if len(pkgs) == 0 {
		return nil, fmt.Errorf("path %s not found", path)
	}

	return pkgs[0].Types, nil
}

func (s *Session) init() (err error) {
	s.fset = token.NewFileSet()
	s.types = &types.Config{Importer: &pkgsImporter{dir: s.tempDir}}
	s.typeInfo = types.Info{}
	s.extraFilePaths = nil
	s.extraFiles = nil

	s.initGoMod() // this should be before printer load for printer package requirements

	var initialSource string
	for _, pp := range printerPkgs {
		_, err = packages.Load(
			&packages.Config{
				Dir:        s.tempDir,
				BuildFlags: []string{"-mod=mod"},
			},
			pp.path,
		)
		if err == nil {
			initialSource = fmt.Sprintf(initialSourceTemplate, pp.path, pp.code)
			break
		}
		debugf("could not import %q: %s", pp.path, err)
	}

	if initialSource == "" {
		return fmt.Errorf("could not load 'fmt' package: %w", err)
	}

	s.file, err = parser.ParseFile(s.fset, "gore_session.go", initialSource, parser.Mode(0))
	if err != nil {
		return err
	}

	s.mainBody = s.mainFunc().Body

	s.lastStmts = nil
	s.lastDecls = nil
	return nil
}

func (s *Session) mainFunc() *ast.FuncDecl {
	return s.file.Scope.Lookup("main").Decl.(*ast.FuncDecl)
}

// Run the session.
func (s *Session) Run() error {
	f, err := os.Create(s.tempFilePath)
	if err != nil {
		return err
	}
	defer f.Close()

	err = printer.Fprint(f, s.fset, s.file)
	if err != nil {
		return err
	}

	return s.goRun(append(s.extraFilePaths, s.tempFilePath))
}

func (s *Session) goRun(files []string) error {
	args := append([]string{"run", "-mod=mod"}, files...)
	debugf("go %s", strings.Join(args, " "))
	cmd := exec.Command("go", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = s.stdout
	cmd.Dir = s.tempDir
	ef := newErrFilter(s.stderr)
	cmd.Stderr = ef
	defer ef.Close()
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
	f, err := parser.ParseFile(s.fset, "stmt.go", src, parser.Mode(0))
	if err != nil {
		return err
	}

	enclosingFunc := f.Scope.Lookup("F").Decl.(*ast.FuncDecl)

	debugf("evalStmt :: %s", showNode(s.fset, enclosingFunc.Body.List))
	var stmts []ast.Stmt

	for _, stmt := range enclosingFunc.Body.List {
		switch stmt := stmt.(type) {
		case *ast.AssignStmt:
			if stmt := buildPrintStmt(stmt.Lhs); stmt != nil {
				stmts = append(stmts, stmt)
			}
		case *ast.DeclStmt:
			if decl, ok := stmt.Decl.(*ast.GenDecl); ok {
				if decl.Tok == token.TYPE {
					s.file.Decls = append(s.file.Decls, decl)
					continue
				} else if stmt := buildPrintStmtOfDecl(decl); stmt != nil {
					stmts = append(stmts, stmt)
				}
			}
		}
		s.appendStatements(stmt)
	}
	s.appendStatements(stmts...)

	return nil
}

func buildPrintStmt(exprs []ast.Expr) ast.Stmt {
	vs := make([]ast.Expr, 0, len(exprs))
	for _, v := range exprs {
		if !isNamedIdent(v, "_") {
			vs = append(vs, v)
		}
	}
	if len(vs) == 0 {
		return nil
	}
	return &ast.ExprStmt{
		X: &ast.CallExpr{
			Fun:  ast.NewIdent(printerName),
			Args: vs,
		},
	}
}

func buildPrintStmtOfDecl(decl *ast.GenDecl) ast.Stmt {
	var cnt int
	for _, s := range decl.Specs {
		if vs, ok := s.(*ast.ValueSpec); ok {
			cnt += len(vs.Values)
		}
	}
	if cnt == 0 {
		return nil
	}
	exprs := make([]ast.Expr, 0, cnt)
	for _, s := range decl.Specs {
		if vs, ok := s.(*ast.ValueSpec); ok {
			for _, name := range vs.Names {
				exprs = append(exprs, ast.Expr(name))
			}
		}
	}
	return buildPrintStmt(exprs)
}

func (s *Session) evalFunc(in string) error {
	src := fmt.Sprintf("package P; %s", in)
	f, err := parser.ParseFile(s.fset, "func.go", src, parser.Mode(0))
	if err != nil {
		return err
	}
	if len(f.Decls) != 1 {
		return errors.New("eval func error")
	}
	newDecl, ok := f.Decls[0].(*ast.FuncDecl)
	if !ok {
		return errors.New("eval func error")
	}
	for i, d := range s.file.Decls {
		if d, ok := d.(*ast.FuncDecl); ok && d.Name.String() == newDecl.Name.String() {
			s.file.Decls = append(s.file.Decls[:i], s.file.Decls[i+1:]...)
			break
		}
	}
	s.file.Decls = append(s.file.Decls, newDecl)
	return nil
}

func (s *Session) parseTokens(in string) error {
	var scanner scanner.Scanner
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(in))
	scanner.Init(file, []byte(in), nil, 0)
	for {
		_, tok, lit := scanner.Scan()
		if tok == token.EOF {
			break
		}
		if tok == token.ILLEGAL {
			return fmt.Errorf("invalid token: %q", string(lit))
		}
	}
	return nil
}

func (s *Session) appendStatements(stmts ...ast.Stmt) {
	s.mainBody.List = append(s.mainBody.List, stmts...)
}

// Error ...
type Error string

// Errors
const (
	ErrContinue Error = "<continue input>"
	ErrQuit     Error = "<quit session>"
	ErrCmdRun   Error = "<command failed>"
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
	err := config.Fprint(&buf, s.fset, s.file)
	return buf.String(), err
}

func (s *Session) reset() error {
	source, err := s.source(false)
	if err != nil {
		return err
	}

	file, err := parser.ParseFile(s.fset, "gore_session.go", source, parser.Mode(0))
	if err != nil {
		return err
	}

	s.file = file
	s.mainBody = s.mainFunc().Body

	return nil
}

// Eval the input.
func (s *Session) Eval(in string) error {
	debugf("eval >>> %q", in)

	s.clearQuickFix()
	s.storeCode()

	if strings.HasPrefix(strings.TrimSpace(in), ":") {
		err := s.invokeCommand(in)
		if err != nil && err != ErrQuit {
			fmt.Fprintf(s.stderr, "%s\n", err)
		}
		return err
	}

	if _, err := s.evalExpr(in); err != nil {
		debugf("expr :: err = %s", err)

		err := s.evalStmt(in)
		if err != nil {
			debugf("stmt :: err = %s", err)

			err := s.evalFunc(in)
			if err != nil {
				debugf("func :: err = %s", err)

				if err := s.parseTokens(in); err != nil {
					fmt.Fprintf(s.stderr, "%s\n", err)
					return err
				}

				return ErrContinue
			}
		}
	}

	if s.autoImport {
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
					s.restoreCode()
				}
			}
		}
		debugf("%s", err)
		err = ErrCmdRun
	}

	return err
}

func (s *Session) invokeCommand(in string) (err error) {
	in = strings.TrimLeftFunc(in, func(c rune) bool {
		return c == ':' || unicode.IsSpace(c)
	})
	tokens := strings.Fields(in)
	if len(tokens) == 0 {
		return
	}
	cmd := tokens[0]
	arg := strings.TrimSpace(strings.TrimPrefix(in, cmd))
	for _, command := range commands {
		if !command.name.matches(cmd) {
			continue
		}
		err = command.action(s, arg)
		if err != nil {
			if err == ErrQuit {
				return
			}
			err = fmt.Errorf("%s: %s", command.name, err)
		}
		return
	}
	return fmt.Errorf("command not found: %s", cmd)
}

// storeCode stores current state of code so that it can be restored
func (s *Session) storeCode() {
	s.lastStmts = s.mainBody.List
	if len(s.lastDecls) != len(s.file.Decls) {
		s.lastDecls = make([]ast.Decl, len(s.file.Decls))
	}
	copy(s.lastDecls, s.file.Decls)
}

// restoreCode restores the previous code
func (s *Session) restoreCode() {
	s.mainBody.List = s.lastStmts
	decls := make([]ast.Decl, 0, len(s.file.Decls))
	for _, d := range s.file.Decls {
		if d, ok := d.(*ast.FuncDecl); ok && d.Name.String() != "main" {
			for _, ld := range s.lastDecls {
				if ld, ok := ld.(*ast.FuncDecl); ok && ld.Name.String() == d.Name.String() {
					decls = append(decls, ld)
					break
				}
			}
			continue
		}
		decls = append(decls, d)
	}
	s.file.Decls = decls
}

// includeFiles imports packages and funcsions from multiple golang source
func (s *Session) includeFiles(files []string) {
	for _, file := range files {
		s.includeFile(file)
	}
}

func (s *Session) includeFile(file string) {
	content, err := os.ReadFile(file)
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
	astf, err := parser.ParseFile(s.fset, "", src, parser.Mode(0))
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
	tmp, err := os.CreateTemp(s.tempDir, "gore_external_*.go")
	if err != nil {
		return err
	}

	f, err := parser.ParseFile(s.fset, tmp.Name(), src, parser.Mode(0))
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
				// remove some unused import's
				quickfix.QuickFix(s.fset, []*ast.File{f})
				break
			}
		}
	}

	out, err := os.Create(tmp.Name())
	if err != nil {
		return err
	}
	defer out.Close()

	err = printer.Fprint(out, s.fset, f)
	if err != nil {
		return err
	}

	debugf("import file: %s", tmp.Name())
	s.extraFilePaths = append(s.extraFilePaths, tmp.Name())
	s.extraFiles = append(s.extraFiles, f)

	return nil
}

// fixImports formats and adjusts imports for the current AST.
func (s *Session) fixImports() error {
	// Fix against error: no required module provides package ...; try 'go get -d ...'
	for _, path := range s.requiredModules {
		cmd := exec.Command("go", "get", "-d", path)
		cmd.Dir = s.tempDir
		if err := cmd.Run(); err != nil {
			debugf("failed to go get -d %q: %s", path, err)
		}
	}
	s.requiredModules = nil

	var buf bytes.Buffer
	err := printer.Fprint(&buf, s.fset, s.file)
	if err != nil {
		return err
	}

	formatted, err := imports.Process("", buf.Bytes(), nil)
	if err != nil {
		return err
	}

	s.file, err = parser.ParseFile(s.fset, "", formatted, parser.Mode(0))
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

// Clear the temporary directory.
func (s *Session) Clear() error {
	return os.RemoveAll(s.tempDir)
}
