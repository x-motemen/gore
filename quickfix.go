package gore

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/ast/astutil"

	"github.com/motemen/go-quickfix"
)

// doQuickFix tries to fix the source AST so that it compiles well.
func (s *Session) doQuickFix() error {
	const maxAttempts = 10

	s.reset()

quickFixAttempt:
	for i := 0; i < maxAttempts; i++ {
		s.typeInfo = types.Info{
			Types: make(map[ast.Expr]types.TypeAndValue),
		}

		config := quickfix.Config{
			Fset:     s.fset,
			Files:    append(s.extraFiles, s.file),
			TypeInfo: &s.typeInfo,
			Dir:      s.tempDir,
		}
		_, err := config.QuickFixOnce()
		if err == nil {
			break
		}

		debugf("quickFix :: err = %#v", err)

		errList, ok := err.(quickfix.ErrorList)
		if !ok {
			continue
		}

		// (try to) fix gore-specific remaining errors
		for _, err := range errList {
			err, ok := err.(types.Error)
			if !ok {
				continue
			}

			// "... used as value":
			//
			// convert
			//   __gore_pp(funcWithSideEffectReturningNoValue())
			// to
			//   funcWithSideEffectReturningNoValue()
			if strings.HasSuffix(err.Msg, " used as value") {
				nodepath, _ := astutil.PathEnclosingInterval(s.file, err.Pos, err.Pos)

				for _, node := range nodepath {
					stmt, ok := node.(ast.Stmt)
					if !ok {
						continue
					}

					for i := range s.mainBody.List {
						if s.mainBody.List[i] != stmt {
							continue
						}

						exprs := printedExprs(stmt)

						stmts := s.mainBody.List[0:i]
						for _, expr := range exprs {
							stmts = append(stmts, &ast.ExprStmt{X: expr})
						}

						s.mainBody.List = append(stmts, s.mainBody.List[i+1:]...)
						continue quickFixAttempt
					}
				}
			}
		}

		debugf("quickFix :: give up: %#v", err)
		break
	}

	return nil
}

func (s *Session) clearQuickFix() {
	// make all import specs explicit (i.e. no "_").
	for _, imp := range s.file.Imports {
		imp.Name = nil
	}

	for i := 0; i < len(s.mainBody.List); {
		stmt := s.mainBody.List[i]

		// remove assignment statement if it is omittable.
		if assign, ok := stmt.(*ast.AssignStmt); ok && s.isPureAssignStmt(assign) {
			s.mainBody.List = append(s.mainBody.List[0:i], s.mainBody.List[i+1:]...)
			continue
		}

		// remove expressions just for printing out
		// i.e. what causes "evaluated but not used."
		if exprs := printedExprs(stmt); exprs != nil {
			allPure := true
			for _, expr := range exprs {
				if !s.isPureExpr(expr) {
					allPure = false
					break
				}
			}

			if allPure {
				s.mainBody.List = append(s.mainBody.List[0:i], s.mainBody.List[i+1:]...)
				continue
			}

			// convert possibly impure expressions to blank assignment
			var trailing []ast.Stmt
			s.mainBody.List, trailing = s.mainBody.List[0:i], s.mainBody.List[i+1:]
			for _, expr := range exprs {
				if !s.isPureExpr(expr) {
					t := s.typeInfo.TypeOf(expr)
					var lhs []ast.Expr
					if t, ok := t.(*types.Tuple); ok {
						lhs = make([]ast.Expr, t.Len())
						for i := 0; i < t.Len(); i++ {
							lhs[i] = ast.NewIdent("_")
						}
					} else {
						lhs = []ast.Expr{ast.NewIdent("_")}
					}
					s.mainBody.List = append(s.mainBody.List, &ast.AssignStmt{
						Lhs: lhs, Tok: token.ASSIGN, Rhs: []ast.Expr{expr},
					})
				}
			}

			s.mainBody.List = append(s.mainBody.List, trailing...)
			continue
		}

		i++
	}

	debugf("clearQuickFix :: %s", showNode(s.fset, s.mainBody))
}

// isPureAssignStmt returns assignment is pure and omittable.
func (s *Session) isPureAssignStmt(stmt *ast.AssignStmt) bool {
	for _, lhs := range stmt.Lhs {
		if !isNamedIdent(lhs, "_") {
			return false
		}
	}
	for _, expr := range stmt.Rhs {
		if !s.isPureExpr(expr) {
			return false
		}
	}
	return true
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

var pureBuiltinFuncNames = map[string]bool{
	"append":  true,
	"cap":     true,
	"complex": true,
	"imag":    true,
	"len":     true,
	"make":    true,
	"new":     true,
	"real":    true,
}

// isPureExpr checks if an expression expr is "pure", which means
// removing this expression will no affect the entire program.
// - identifiers ("x")
// - types
// - selectors ("x.y")
// - slices ("a[n:m]")
// - literals ("1")
// - type conversion ("int(1)")
// - type assertion ("x.(int)")
// - call of some built-in functions as listed in pureBuiltinFuncNames
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
		tv := s.typeInfo.Types[expr.Fun]
		for _, arg := range expr.Args {
			if !s.isPureExpr(arg) {
				return false
			}
		}

		if tv.IsType() {
			return true
		}

		if tv.IsBuiltin() {
			if ident, ok := expr.Fun.(*ast.Ident); ok {
				if pureBuiltinFuncNames[ident.Name] {
					return true
				}
			}
		}

		return false
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
	case *ast.ParenExpr:
		return s.isPureExpr(expr.X)

	case *ast.InterfaceType:
		return true
	case *ast.ArrayType:
		return true
	case *ast.ChanType:
		return true
	case *ast.KeyValueExpr:
		return true
	case *ast.MapType:
		return true
	case *ast.StructType:
		return true
	case *ast.FuncType:
		return true

	case *ast.Ellipsis:
		return true

	case *ast.BadExpr:
		return false
	}

	return false
}
