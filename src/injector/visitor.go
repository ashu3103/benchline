package injector

import (
	"go/ast"
	"go/token"
	"strings"
)

var (
	funcTestParamName = "t"
	tOnlyMethods = map[string]bool{
		"Parallel": true,
		"Deadline": true,
	}
	tParamFuncs = map[string]string{}
)

type FileVisitor struct {
	Err    error
	FDecls []*ast.FuncDecl
	IDecls []ast.Decl
}

type FunctionVisitor struct {
	Err    error
}
type BodyVisitor struct {}

func wrapInBenchmarkLoop(fn *ast.FuncDecl)  {
	methodName := "Loop"

	forStmt := &ast.ForStmt{
		Init: nil,
		Cond: &ast.CallExpr{
		Args: nil,
		Fun: &ast.SelectorExpr{
				X: &ast.Ident{
					Name: funcTestParamName,
					Obj: nil,
				},
				Sel: &ast.Ident{
					Name: methodName,
					Obj: nil,
				},
			},
		},
		Post: nil,
		Body: fn.Body,
	}

	fn.Body = &ast.BlockStmt{
		List: []ast.Stmt{forStmt},
	}
}

func searchTMethods(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
    if !ok {
        return false
    }
    sel, ok := call.Fun.(*ast.SelectorExpr)
    if !ok {
        return false
    }
    ident, ok := sel.X.(*ast.Ident)
    if !ok {
        return false
    }
    return ident.Name == funcTestParamName && tOnlyMethods[sel.Sel.Name]
}

func checkAndModifyParamList(paramList []*ast.Field) bool {
	for _, param := range paramList {
		if starExp, ok := param.Type.(*ast.StarExpr); ok {
			if selExp, ok := starExp.X.(*ast.SelectorExpr); ok {
				pkg, ok := selExp.X.(*ast.Ident);
				if ok && pkg.Name == "testing" && selExp.Sel.Name == "T" {
					selExp.Sel.Name = "B"
					funcTestParamName = param.Names[0].Name
					return true
				}
			}
		}
	}
	return false
}

func (v *FileVisitor) Visit(node ast.Node) (w ast.Visitor) {
	if node == nil || v.Err != nil {
		return nil
	}

	switch decl := node.(type) {
	/* include the function declaration nodes that have *testing.T in their parameter list */
	case *ast.FuncDecl:
		/* check the parameter list of the function */
		if !checkAndModifyParamList(decl.Type.Params.List) {
			return nil
		}

		/* check the function name; if TestXxx make it Benchmark else change the */
		if strings.HasPrefix(decl.Name.Name, TESTING_FUNCTION_PREFIX) {
			decl.Name.Name = strings.ReplaceAll(decl.Name.Name, TESTING_FUNCTION_PREFIX, BENCHMARK_FUNCTION_PREFIX)
			v.FDecls = append(v.FDecls, decl)
		} else {
			tParamFuncs[decl.Name.Name] = BENCHMARK_FUNCTION_PREFIX + decl.Name.Name
			decl.Name.Name = BENCHMARK_FUNCTION_PREFIX + decl.Name.Name
			v.FDecls = append([]*ast.FuncDecl{decl}, v.FDecls...)
		}
	/* include the import declaration */
	case *ast.GenDecl:
		if decl.Tok == token.IMPORT {
			v.IDecls = append(v.IDecls, decl)
		}
	}

	return nil
}

func (v *FunctionVisitor) Visit(node ast.Node) (w ast.Visitor) {
	if node == nil || v.Err != nil {
		return nil
	}

	switch s := node.(type) {
	/* stmt */
	case *ast.AssignStmt:
		// TODO: remove t.Deadline()
		return v
	case *ast.ExprStmt:
		if searchTMethods(s.X) {
			s.X = &ast.CallExpr{
				Fun: ast.NewIdent(NOOP_FUNCTION_NAME),
				Args: nil,
			}
			return nil
		}
	/* expr */
	case *ast.CallExpr:
		switch fn := s.Fun.(type) {
		case *ast.FuncLit:
			for _, field := range fn.Type.Params.List {
				star, ok := field.Type.(*ast.StarExpr)
				if !ok {
					continue
				}
				sel, ok := star.X.(*ast.SelectorExpr)
				if !ok {
					continue
				}
				pkg, ok := sel.X.(*ast.Ident)
				if !ok {
					continue
				}
				if pkg.Name == "testing" && sel.Sel.Name == "T" {
					sel.Sel.Name = "B"
				}
			}
			ast.Walk(v, fn.Body)
		case *ast.Ident:
			if value, exists := tParamFuncs[fn.Name]; exists {
				fn.Name = value
			}
		}
		return nil
	}

	return v
}