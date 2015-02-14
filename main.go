package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"go/ast"
	"go/parser"
	"go/printer"
	"go/scanner"
	"go/token"
	"golang.org/x/tools/go/ast/astutil"
	_ "golang.org/x/tools/go/gcimporter"
	"golang.org/x/tools/go/types"

	"github.com/mitchellh/go-homedir"
)

const printerName = "__gore_p"

func main() {
	s, err := NewSession()
	if err != nil {
		panic(err)
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

	rl.SetWordCompleter(func(line string, pos int) (string, []string, string) {
		if strings.HasPrefix(line, ":") {
			// complete commands
			if !strings.Contains(line[0:pos], " ") {
				pre, post := line[0:pos], line[pos:]

				result := []string{}
				for _, command := range commands {
					name := ":" + command.name
					if strings.HasPrefix(name, pre) {
						// having complete means that this command takes an argument (for now)
						if !strings.HasPrefix(post, " ") && command.arg != "" {
							name = name + " "
						}
						result = append(result, name)
					}
				}
				return "", result, post
			}

			// complete command arguments
			for _, command := range commands {
				if command.complete == nil {
					continue
				}

				cmdPrefix := ":" + command.name + " "
				if strings.HasPrefix(line, cmdPrefix) && pos >= len(cmdPrefix) {
					return cmdPrefix, command.complete(line[len(cmdPrefix):pos]), ""
				}
			}

			return "", nil, ""
		}

		if gocode.invalid {
			return "", nil, ""
		}

		// code completion

		s.clearQuickFix()

		source, err := s.source(false)
		if err != nil {
			errorf("source: %s", err)
			return "", nil, ""
		}

		p := strings.LastIndex(source, "}")
		editSource := source[0:p] + line + source[p:]
		cursor := len(source[0:p]) + pos

		cands, err := gocode.complete(editSource, cursor)
		if err != nil {
			errorf("gocode: %s", err)
			return "", nil, ""
		}

		return line[0:pos], cands, ""
	})

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

		err = s.Run(in)
		if err != nil {
			if err == ErrContinue {
				continue
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
	FilePath string
	File     *ast.File
	Fset     *token.FileSet
	Types    *types.Config
	TypeInfo types.Info

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
			Packages: make(map[string]*types.Package),
		},
	}

	s.FilePath, err = tempFile()
	if err != nil {
		return nil, err
	}

	var initialSource string
	for _, pp := range printerPkgs {
		_, err := types.DefaultImport(s.Types.Packages, pp.path)
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

	mainFunc := s.File.Scope.Lookup("main").Decl.(*ast.FuncDecl)
	s.mainBody = mainFunc.Body

	return s, nil
}

func (s *Session) RunFile() error {
	f, err := os.Create(s.FilePath)
	if err != nil {
		return err
	}

	err = printer.Fprint(f, s.Fset, s.File)
	if err != nil {
		return err
	}

	return goRun(s.FilePath)
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
			Fun:  ast.NewIdent(printerName),
			Args: []ast.Expr{expr},
		},
	}

	s.appendStatements(stmt)

	return nil
}

func isNamedIdent(expr ast.Expr, name string) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == name
}

func (s *Session) injectStmt(in string) error {
	src := fmt.Sprintf("package P; func F() { %s }", in)
	f, err := parser.ParseFile(s.Fset, "stmt.go", src, parser.Mode(0))
	if err != nil {
		return err
	}

	enclosingFunc := f.Scope.Lookup("F").Decl.(*ast.FuncDecl)
	stmts := enclosingFunc.Body.List

	if len(stmts) > 0 {
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
)

func (e Error) Error() string {
	return string(e)
}

// TODO normalize position
func (s *Session) source(space bool) (string, error) {
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

var (
	rxDeclaredNotUsed = regexp.MustCompile(`^([a-zA-Z0-9_]+) declared but not used`)
	rxImportedNotUsed = regexp.MustCompile(`^(".+") imported but not used`)
)

// quickFixFile tries to fix the source AST so that it compiles well.
func (s *Session) quickFixFile() error {
	const maxAttempts = 10

	for i := 0; i < maxAttempts; i++ {
		s.TypeInfo = types.Info{
			Types: make(map[ast.Expr]types.TypeAndValue),
		}
		_, err := s.Types.Check("_quickfix", s.Fset, []*ast.File{s.File}, &s.TypeInfo)
		if err == nil {
			break
		}

		debugf("quickFix :: err = %#v", err)

		if err, ok := err.(types.Error); ok {
			// Handle these situations:
			// - "%s declared but not used"
			// - "%q imported but not used"
			// - "%s used as value"
			if m := rxDeclaredNotUsed.FindStringSubmatch(err.Msg); m != nil {
				ident := m[1]
				debugf("quickFix :: declared but not used -> %s", ident)
				// insert "_ = x" to supress "declared but not used" error
				stmt := &ast.AssignStmt{
					Lhs: []ast.Expr{ast.NewIdent("_")},
					Tok: token.ASSIGN,
					Rhs: []ast.Expr{ast.NewIdent(ident)},
				}
				s.appendStatements(stmt)
			} else if m := rxImportedNotUsed.FindStringSubmatch(err.Msg); m != nil {
				path := m[1] // quoted string, but it's okay because this will be compared to ast.BasicLit.Value.
				debugf("quickFix :: imported but not used -> %s", path)

				for _, imp := range s.File.Imports {
					debugf("%s vs %s", imp.Path.Value, path)
					if imp.Path.Value == path {
						// make this import spec anonymous one
						imp.Name = ast.NewIdent("_")
						break
					}
				}
			} else if strings.HasSuffix(err.Msg, " used as value") {
				// if last added statement is p(expr), unwrap that expr
				mainLen := len(s.mainBody.List)
				if mainLen-s.storedBodyLength == 1 {
					// just one statement added
					lastStmt := s.mainBody.List[mainLen-1]
					if es, ok := lastStmt.(*ast.ExprStmt); ok {
						if call, ok := es.X.(*ast.CallExpr); ok && isNamedIdent(call.Fun, printerName) {
							s.restoreMainBody()
							for _, expr := range call.Args {
								s.appendStatements(&ast.ExprStmt{X: expr})
							}
						}
					}
				} else {
					debugf("quickFix :: give up")
					break
				}
			} else {
				debugf("quickFix :: give up")
				break
			}
		} else {
			return err
		}
	}

	return nil
}

func (s *Session) clearQuickFix() {
	// make all import specs explicit (i.e. no "_").
	for _, imp := range s.File.Imports {
		imp.Name = nil
	}

	for i := 0; i < len(s.mainBody.List); {
		stmt := s.mainBody.List[i]

		// remove "_ = x" stmt
		if assign, ok := stmt.(*ast.AssignStmt); ok && len(assign.Lhs) == 1 {
			if isNamedIdent(assign.Lhs[0], "_") {
				s.mainBody.List = append(s.mainBody.List[0:i], s.mainBody.List[i+1:]...)
				continue
			}
		}

		// remove expressions just for printing out
		// i.e. what causes "evaluated but not used."
		if exprs := printedExprs(stmt); exprs != nil {
			allPure := true
			for _, expr := range exprs {
				if !s.isPureExpr(expr) {
					allPure = false
				}
			}
			if allPure {
				s.mainBody.List = append(s.mainBody.List[0:i], s.mainBody.List[i+1:]...)
				continue
			}

			// strip (possibly impure) printing expression to expression
			var trailing []ast.Stmt
			s.mainBody.List, trailing = s.mainBody.List[0:i], s.mainBody.List[i+1:]
			for _, expr := range exprs {
				if !isNamedIdent(expr, "_") {
					s.mainBody.List = append(s.mainBody.List, &ast.ExprStmt{X: expr})
				}
			}

			s.mainBody.List = append(s.mainBody.List, trailing...)
			continue
		}

		i++
	}
}

// printedExprs returns arguments of statement stmt of form "p(x...)"
func printedExprs(stmt ast.Stmt) []ast.Expr {
	st, ok := stmt.(*ast.ExprStmt)
	if !ok {
		return nil
	}

	// first check whether the expr is p(_) form
	call, ok := st.X.(*ast.CallExpr)
	if !ok {
		return nil
	}

	if !isNamedIdent(call.Fun, printerName) {
		return nil
	}

	return call.Args
}

// TODO: use types.Universe?
var pureBuiltinFuncs = map[string]bool{
	"len":    true,
	"make":   true,
	"cap":    true,
	"append": true,
	"imag":   true,
	"real":   true,

	// below are actually not builtin functions
	"int":        true,
	"bool":       true,
	"int8":       true,
	"int16":      true,
	"int32":      true,
	"int64":      true,
	"uint":       true,
	"uint8":      true,
	"uint16":     true,
	"uint32":     true,
	"uint64":     true,
	"uintptr":    true,
	"float32":    true,
	"float64":    true,
	"complex64":  true,
	"complex128": true,
	"string":     true,
}

// isPureExpr checks if an expression expr is "pure", which means
// removing this expression will no affect the entire program.
// - identifiers ("x")
// - selectors ("x.y")
// - slices ("a[n:m]")
// - literals ("1")
// - type conversion ("int(1)")
// - type assertion ("x.(int)")
// - call of some built-in functions: len, make, cap, append, imag, real
func (s *Session) isPureExpr(expr ast.Expr) bool {
	if expr == nil {
		return true
	}

	switch expr := expr.(type) {
	case *ast.Ident:
		return true
	case *ast.BasicLit:
		return true
	case *ast.BinaryExpr:
		return s.isPureExpr(expr.X) && s.isPureExpr(expr.Y)
	case *ast.CallExpr:
		if ident, ok := expr.Fun.(*ast.Ident); ok {
			if !pureBuiltinFuncs[ident.Name] {
				return false
			}
			for _, arg := range expr.Args {
				if !s.isPureExpr(arg) {
					return false
				}
			}
			return true
		}
		tv := s.TypeInfo.Types[expr.Fun]
		debugf("%s: %#v", astutil.NodeDescription(expr), tv)
	case *ast.CompositeLit:
		return true
	case *ast.FuncLit:
		return true
	case *ast.IndexExpr:
		return s.isPureExpr(expr.X) && s.isPureExpr(expr.Index)
	case *ast.SelectorExpr:
		return s.isPureExpr(expr.X)
	case *ast.SliceExpr:
		return s.isPureExpr(expr.Low) && s.isPureExpr(expr.High) && s.isPureExpr(expr.Max)
	case *ast.StarExpr:
		return s.isPureExpr(expr.X)
	case *ast.TypeAssertExpr:
		return true
	case *ast.UnaryExpr:
		return s.isPureExpr(expr.X)
	}

	return false
}

func (s *Session) Run(in string) error {
	debugf("run >>> %q", in)

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
				errorf("%s: %s", command.name, err)
			}
			commandRan = true
			break
		}
	}

	if commandRan {
		s.quickFixFile()
		return nil
	}

	if !commandRan {
		if err := s.injectExpr(in); err != nil {
			debugf("expr :: err = %s", err)

			err := s.injectStmt(in)
			if err != nil {
				debugf("stmt :: err = %s", err)

				if _, ok := err.(scanner.ErrorList); ok {
					return ErrContinue
				}
			}
		}
	}

	s.quickFixFile()

	err := s.RunFile()
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

func normalizeNode(node ast.Node) {
	// TODO remove token.Pos information
}
