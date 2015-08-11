package main

import (
	"reflect"

	"go/ast"
	"go/token"
)

// normalizeNodePos resets all position information of node and its descendants.
func normalizeNodePos(node ast.Node) {
	ast.Inspect(node, func(node ast.Node) bool {
		if node == nil {
			return true
		}

		if node.Pos() == token.NoPos && node.End() == token.NoPos {
			return true
		}

		pv := reflect.ValueOf(node)
		if pv.Kind() != reflect.Ptr {
			return true
		}

		v := pv.Elem()
		if v.Kind() != reflect.Struct {
			return true
		}

		for i := 0; i < v.NumField(); i++ {
			f := v.Field(i)
			ft := f.Type()
			if f.CanSet() && ft.PkgPath() == "go/token" && ft.Name() == "Pos" && f.Int() != 0 {
				f.SetInt(1)
			}
		}

		return true
	})
}
