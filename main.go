package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"

	"go/ast"
	"go/build"
	"go/parser"
	"go/printer"
	"go/scanner"
	"go/token"
	"golang.org/x/tools/go/ast/astutil"
	_ "golang.org/x/tools/go/gcimporter"
	"golang.org/x/tools/go/types"

	"github.com/peterh/liner"
)

const printerName = "__gore_p"

var debug = false

func debugf(format string, args ...interface{}) {
	if !debug {
		return
	}

	_, file, line, ok := runtime.Caller(1)
	if ok {
		format = fmt.Sprintf("%s:%d %s", filepath.Base(file), line, format)
	}

	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

func errorf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
}

func infof(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

var gorootSrc = filepath.Join(filepath.Clean(runtime.GOROOT()), "src")

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

const (
	promptDefault  = "gore> "
	promptContinue = "..... "
)

type contLiner struct {
	*liner.State
	buffer string
}

func newContLiner() *contLiner {
	rl := liner.NewLiner()
	return &contLiner{State: rl}
}

func (cl *contLiner) promptString() string {
	if cl.buffer != "" {
		return promptContinue
	}

	return promptDefault
}

func (cl *contLiner) Prompt() (string, error) {
	line, err := cl.State.Prompt(cl.promptString())
	if err == io.EOF {
		if cl.buffer != "" {
			// cancel line continuation
			cl.Accepted()
			fmt.Println()
			err = nil
		}
	} else if err == nil {
		if cl.buffer != "" {
			cl.buffer = cl.buffer + "\n" + line
		} else {
			cl.buffer = line
		}
	}

	return cl.buffer, err
}

func (cl *contLiner) Accepted() {
	cl.State.AppendHistory(cl.buffer)
	cl.buffer = ""
}

type command struct {
	name     string
	action   func(*Session, string) error
	complete func(string) []string
}

// TODO
// - :edit
// - :undo
// - :reset
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

func main() {
	s := NewSession()

	rl := newContLiner()
	defer rl.Close()

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
						if !strings.HasPrefix(post, " ") && command.complete != nil {
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

		// code completion
		source, err := s.source(true)
		if err != nil {
			errorf("source: %s", err)
			return "", nil, ""
		}

		p := strings.LastIndex(source, "}")
		editSource := source[0:p] + line + source[p:]
		cursor := len(source[0:p]) + pos

		gocode := exec.Command("gocode", "-f=json", "autocomplete", fmt.Sprintf("%d", cursor))
		in, err := gocode.StdinPipe()
		if err != nil {
			errorf("gocode: %s", err)
			return "", nil, ""
		}
		_, err = in.Write([]byte(editSource))
		if err != nil {
			errorf("gocode: %s", err)
			return "", nil, ""
		}
		err = in.Close()
		if err != nil {
			errorf("gocode: %s", err)
			return "", nil, ""
		}
		out, err := gocode.Output()
		if err != nil {
			errorf("gocode: %s", err)
			return "", nil, ""
		}
		debugf("gocode :: %s", out)

		result := []json.RawMessage{}
		err = json.Unmarshal(out, &result)
		if err != nil {
			errorf("gocode: %s", err)
			return "", nil, ""
		}

		if len(result) < 2 {
			debugf("gocode :: %#v", result)
			return "", nil, ""
		}

		type entry struct {
			Class string `json:"class"`
			Name  string `json:"name"`
			Type  string `json:"type"`
		}

		var c int
		err = json.Unmarshal(result[0], &c)
		if err != nil {
			errorf("gocode: %s", err)
			return "", nil, ""
		}

		entries := []entry{}
		err = json.Unmarshal(result[1], &entries)
		if err != nil {
			errorf("gocode: %s", err)
			return "", nil, ""
		}

		debugf("%d %#v", c, entries)

		cands := make([]string, 0, len(entries))
		for _, e := range entries {
			cands = append(cands, e.Name[c:])
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
}

type Session struct {
	FilePath string
	File     *ast.File
	Fset     *token.FileSet
	Types    *types.Config

	mainBody         *ast.BlockStmt
	storedBodyLength int
}

const initialSource = `
package main

import "fmt"

func ` + printerName + `(xx ...interface{}) {
	for _, x := range xx {
		fmt.Printf("%#v\n", x)
	}
}

func main() {
}
`

func NewSession() *Session {
	var err error

	s := &Session{}
	s.Fset = token.NewFileSet()
	s.Types = &types.Config{
		Packages: make(map[string]*types.Package),
	}

	s.FilePath, err = tempFile()
	if err != nil {
		panic(err)
	}

	s.File, err = parser.ParseFile(s.Fset, "gore_session.go", initialSource, parser.Mode(0))
	if err != nil {
		panic(err)
	}

	mainFunc := s.File.Scope.Lookup("main").Decl.(*ast.FuncDecl)
	s.mainBody = mainFunc.Body

	return s
}

func (s *Session) BuildRunFile() error {
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

// TODO do not run after :print
func actionPrint(s *Session, _ string) error {
	source, err := s.source(true)
	if err != nil {
		return err
	}

	fmt.Println(source)

	return nil
}

// TODO do not run after :write
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

var (
	rxDeclaredNotUsed = regexp.MustCompile(`^([a-zA-Z0-9_]+) declared but not used`)
	rxImportedNotUsed = regexp.MustCompile(`^(".+") imported but not used`)
)

// quickFixFile tries to fix the source AST so that it compiles well.
func (s *Session) quickFixFile() error {
	const maxAttempts = 10

	for i := 0; i < maxAttempts; i++ {
		_, err := s.Types.Check("_quickfix", s.Fset, []*ast.File{s.File}, nil)
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
				if !isPureExpr(expr) {
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

var pureBuiltinFuncs = map[string]bool{
	"len":    true,
	"make":   true,
	"cap":    true,
	"append": true,
	"imag":   true,
	"real":   true,
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
func isPureExpr(expr ast.Expr) bool {
	if expr == nil {
		return true
	}

	switch expr := expr.(type) {
	case *ast.Ident:
		return true
	case *ast.BasicLit:
		return true
	case *ast.BinaryExpr:
		return isPureExpr(expr.X) && isPureExpr(expr.Y)
	case *ast.CallExpr:
		ident, ok := expr.Fun.(*ast.Ident)
		if !ok {
			return false
		}
		if !pureBuiltinFuncs[ident.Name] {
			return false
		}
		for _, arg := range expr.Args {
			if !isPureExpr(arg) {
				return false
			}
		}
		return true
	case *ast.CompositeLit:
		return true
	case *ast.FuncLit:
		return true
	case *ast.IndexExpr:
		return isPureExpr(expr.X) && isPureExpr(expr.Index)
	case *ast.SelectorExpr:
		return isPureExpr(expr.X)
	case *ast.SliceExpr:
		return isPureExpr(expr.Low) && isPureExpr(expr.High) && isPureExpr(expr.Max)
	case *ast.StarExpr:
		return isPureExpr(expr.X)
	case *ast.TypeAssertExpr:
		return true
	case *ast.UnaryExpr:
		return isPureExpr(expr.X)
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

	err := s.BuildRunFile()
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
