package main

import (
	"bytes"
	"fmt"
	"go/ast"

	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
	"golang.org/x/tools/go/types"
)

func (s *Session) dumpSSA() (string, error) {
	pkg := types.NewPackage("goresession", "")
	if err := s.doQuickFix(); err != nil {
		debugf("quickfix SSA: %v", err)
		return "", nil
	}
	files := []*ast.File{s.File}
	ssapkg, _, err := ssautil.BuildPackage(s.Types, s.Fset, pkg, files, ssa.SanityCheckFunctions)
	if err != nil {
		fmt.Print(err) // type error in some package
		return "", err
	}
	var b bytes.Buffer
	ssa.WriteFunction(&b, ssapkg.Func("main"))
	debugf("ssa :: %s", b.String())
	return b.String(), nil
}
