package main

import (
	"regexp"
	"strings"

	"go/ast"
	"go/token"
	"golang.org/x/tools/go/types"
)

var (
	rxDeclaredNotUsed = regexp.MustCompile(`^([a-zA-Z0-9_]+) declared but not used`)
	rxImportedNotUsed = regexp.MustCompile(`^(".+") imported but not used`)
)

// doQuickFix tries to fix the source AST so that it compiles well.
func (s *Session) doQuickFix() error {
	const maxAttempts = 100

	for i := 0; i < maxAttempts; i++ {
		s.TypeInfo = types.Info{
			Types: make(map[ast.Expr]types.TypeAndValue),
		}

		files := s.IncludeFiles
		files = append(files, s.File)

		_, err := s.Types.Check("_quickfix", s.Fset, files, &s.TypeInfo)
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
		tv := s.TypeInfo.Types[expr.Fun]
		if tv.IsType() || tv.IsBuiltin() {
			return true
		}
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
