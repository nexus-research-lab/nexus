// 共享字符串原语只允许 internal/infra/textutil 持有一份；protocol 不能导入任何
// internal 包，因此保留自身副本。门禁按函数形状识别私有复制，名称不同也会被拒绝。
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
)

var helperCopyExempt = []string{"internal/infra/textutil", "internal/protocol"}

func checkSharedHelperCopies(root string) error {
	var violations []string
	err := filepath.WalkDir(filepath.Join(root, "internal"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		relative, _ := filepath.Rel(root, path)
		for _, exempt := range helperCopyExempt {
			if strings.HasPrefix(filepath.ToSlash(relative), exempt+"/") {
				return nil
			}
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || fn.Body == nil {
				continue
			}
			if shared := sharedHelperShape(fn); shared != "" {
				violations = append(violations, fmt.Sprintf("%s: %s 复制了 textutil.%s", fset.Position(fn.Pos()), fn.Name.Name, shared))
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if len(violations) > 0 {
		return fmt.Errorf("禁止私有复制共享字符串原语：\n%s", strings.Join(violations, "\n"))
	}
	return nil
}

// sharedHelperShape 返回 fn 等价复制的 textutil 函数名；不匹配时返回空串。
func sharedHelperShape(fn *ast.FuncDecl) string {
	params, results := fn.Type.Params.List, fn.Type.Results
	if len(params) != 1 || len(params[0].Names) != 1 || results == nil || len(results.List) != 1 ||
		exprText(results.List[0].Type) != "string" || !trimsSpace(fn.Body) {
		return ""
	}
	body := fn.Body.List
	switch exprText(params[0].Type) {
	case "...string":
		if loop, ok := body[0].(*ast.RangeStmt); ok && len(body) == 2 && returnsEmptyString(body[1]) && returnsFirstNonBlank(loop.Body) {
			return "FirstNonEmpty"
		}
	case "*string":
		if guard, ok := body[0].(*ast.IfStmt); ok && len(body) == 2 && exprText(guard.Cond) == params[0].Names[0].Name+" == nil" {
			return "PointerValue"
		}
	case "any", "interface{}":
		if len(body) > 3 || !returnsTrimmed(body[len(body)-1]) {
			return ""
		}
		if len(body) == 3 {
			guard, ok := body[1].(*ast.IfStmt)
			if !ok || len(guard.Body.List) != 1 || !returnsEmptyString(guard.Body.List[0]) {
				return ""
			}
		}
		found := false
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			if assertion, ok := node.(*ast.TypeAssertExpr); ok && exprText(assertion.Type) == "string" {
				found = true
			}
			return !found
		})
		if found {
			return "AnyString"
		}
	}
	return ""
}

// returnsFirstNonBlank 只接受“裁剪后非空即返回”的循环体；附加过滤条件属于领域规则。
func returnsFirstNonBlank(loop *ast.BlockStmt) bool {
	if len(loop.List) == 0 || len(loop.List) > 2 {
		return false
	}
	guard, ok := loop.List[len(loop.List)-1].(*ast.IfStmt)
	if !ok || guard.Else != nil || len(guard.Body.List) != 1 {
		return false
	}
	condition, ok := guard.Cond.(*ast.BinaryExpr)
	return ok && condition.Op == token.NEQ && exprText(condition.Y) == `""`
}

func returnsTrimmed(statement ast.Stmt) bool {
	ret, ok := statement.(*ast.ReturnStmt)
	if !ok || len(ret.Results) != 1 {
		return false
	}
	call, ok := ret.Results[0].(*ast.CallExpr)
	return ok && exprText(call.Fun) == "strings.TrimSpace"
}

func trimsSpace(body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(node ast.Node) bool {
		if call, ok := node.(*ast.CallExpr); ok && exprText(call.Fun) == "strings.TrimSpace" {
			found = true
		}
		return !found
	})
	return found
}

func returnsEmptyString(statement ast.Stmt) bool {
	ret, ok := statement.(*ast.ReturnStmt)
	return ok && len(ret.Results) == 1 && exprText(ret.Results[0]) == `""`
}

func exprText(expr ast.Expr) string {
	switch typed := expr.(type) {
	case *ast.Ident:
		return typed.Name
	case *ast.BasicLit:
		return typed.Value
	case *ast.StarExpr:
		return "*" + exprText(typed.X)
	case *ast.Ellipsis:
		return "..." + exprText(typed.Elt)
	case *ast.SelectorExpr:
		return exprText(typed.X) + "." + typed.Sel.Name
	case *ast.InterfaceType:
		if len(typed.Methods.List) == 0 {
			return "interface{}"
		}
	case *ast.BinaryExpr:
		return exprText(typed.X) + " " + typed.Op.String() + " " + exprText(typed.Y)
	}
	return ""
}
