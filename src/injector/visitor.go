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
)

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

func (v *FunctionVisitor) Visit(node ast.Node) (w ast.Visitor) {
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
		if strings.HasPrefix(decl.Name.Name, BENCHMARK_FUNCTION_PREFIX) {
			decl.Name.Name = strings.ReplaceAll(decl.Name.Name, TESTING_FUNCTION_PREFIX, BENCHMARK_FUNCTION_PREFIX)
		} else {
			decl.Name.Name = BENCHMARK_FUNCTION_PREFIX + decl.Name.Name
		}
		
		/* walk the body of the function */
		bodyVisitor := &BodyVisitor{}
		ast.Walk(bodyVisitor, decl.Body)

		/* wrap the function body in b.Loop */
		wrapInBenchmarkLoop(decl)
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

func (v *BodyVisitor) Visit(node ast.Node) (w ast.Visitor) {
	return nil
}