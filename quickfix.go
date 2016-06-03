package main

import (
	"strings"

	"go/ast"
	"go/types"
	"golang.org/x/tools/go/ast/astutil"

	"github.com/motemen/go-quickfix"
)

// doQuickFix tries to fix the source AST so that it compiles well.
func (s *Session) doQuickFix() error {
	const maxAttempts = 10

	s.reset()

quickFixAttempt:
	for i := 0; i < maxAttempts; i++ {
		s.TypeInfo = types.Info{
			Types: make(map[ast.Expr]types.TypeAndValue),
		}

		files := s.ExtraFiles
		files = append(files, s.File)

		config := quickfix.Config{
			Fset:     s.Fset,
			Files:    files,
			TypeInfo: &s.TypeInfo,
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
				nodepath, _ := astutil.PathEnclosingInterval(s.File, err.Pos, err.Pos)

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
							stmts = append(stmts, &ast.ExprStmt{expr})
						}

						s.mainBody.List = append(stmts, s.mainBody.List[i+1:]...)
						continue quickFixAttempt
					}
				}
			}
		}

		debugf("quickFix :: give up: %#v", err)
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
					break
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

	debugf("clearQuickFix :: %s", showNode(s.Fset, s.mainBody))
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
		tv := s.TypeInfo.Types[expr.Fun]
		for _, arg := range expr.Args {
			if s.isPureExpr(arg) == false {
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
