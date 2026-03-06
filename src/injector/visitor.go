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
	filteredDecls []ast.Decl
	tParamFuncs = map[string]bool{}
)

type FunctionSignatureVisitor struct {}
type FunctionVisitor struct {}
type BodyVisitor struct {}

func wrapInBenchmarkLoop(fn *ast.FuncDecl)  {
	methodName := "Loop"

	bLoopExpr := &ast.CallExpr{
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
	}

	forStmt := &ast.ForStmt{
		Init: nil,
		Cond: bLoopExpr,
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

func (v *FunctionSignatureVisitor) Visit(node ast.Node) (w ast.Visitor) {
	if node == nil {
		return nil
	}

	switch decl := node.(type) {
	/* include the function declaration nodes that have *testing.T in their parameter list */
	case *ast.FuncDecl:
		hasTestingParam := false

		/* check the parameter list of the function */
		for _, param := range decl.Type.Params.List {
			if starExp, ok := param.Type.(*ast.StarExpr); ok {
				if selExp, ok := starExp.X.(*ast.SelectorExpr); ok {
					pkg, ok := selExp.X.(*ast.Ident);
					if ok && pkg.Name == "testing" && selExp.Sel.Name == "T" {
						selExp.Sel.Name = "B"
						funcTestParamName = param.Names[0].Name
						hasTestingParam = true
					}
				}
			}
		}

		if !hasTestingParam {
			return nil
		}

		/* check the function name; if TestXxx make it Benchmark else change the */
		if strings.HasPrefix(decl.Name.Name, TESTING_FUNCTION_PREFIX) {
			decl.Name.Name = strings.ReplaceAll(decl.Name.Name, TESTING_FUNCTION_PREFIX, BENCHMARK_FUNCTION_PREFIX)
		} else {
			tParamFuncs[decl.Name.Name] = true
			decl.Name.Name = BENCHMARK_FUNCTION_PREFIX + decl.Name.Name
		}

		filteredDecls = append(filteredDecls, decl)
	/* include the import declaration */
	case *ast.GenDecl:
		if decl.Tok == token.IMPORT {
			filteredDecls = append(filteredDecls, decl)
		}
	default:
		// logs
	}

	return nil
}

func (v *FunctionVisitor) Visit(node ast.Node) (w ast.Visitor) {
	if node == nil {
		return nil
	}

	fn, ok := node.(*ast.FuncDecl)
	if !ok || fn.Name.Name == NOOP_FUNCTION_NAME {
		return nil
	}

	/* walk the body of the function */
	bodyVisitor := &BodyVisitor{}
	ast.Walk(bodyVisitor, fn.Body)

	/* wrap the function body in b.Loop */
	wrapInBenchmarkLoop(fn)
	return nil
}

func (v *BodyVisitor) Visit(node ast.Node) (w ast.Visitor) {
	if node == nil {
		return nil
	}

	switch s := node.(type) {
	/* stmt */
	case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.SelectStmt, *ast.TypeSwitchStmt:
		return v
	case *ast.CaseClause, *ast.CommClause:
		return v
	case *ast.BlockStmt, *ast.GoStmt, *ast.DeferStmt:
		return v
	case *ast.AssignStmt:
		// TODO: remove t.Deadline()
		return v
	case *ast.ExprStmt:
		if searchTMethods(s.X) {
			s.X = &ast.CallExpr{
				Fun: ast.NewIdent(NOOP_FUNCTION_NAME),
				Args: nil,
			}
		} else {
			return v
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
			if tParamFuncs[fn.Name] {
				fn.Name = BENCHMARK_FUNCTION_PREFIX + fn.Name
			}
		}
		return nil
	/* ignore other nodes */
	default:
		return nil
	}

	return nil
}