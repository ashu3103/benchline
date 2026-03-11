package analyzer

import (
	"io"
	"go/ast"
	"go/token"
	"go/types"
)

type StructVar struct {
	VarType    *types.Var
	StructType *types.Struct
	NamedType  *types.Named
	DeclNode   ast.Node
	Fields     []*types.Var
}

// finds all local variables of struct type within a function
type StructVarCollector struct {
	info *types.Info
	fset *token.FileSet
	pkg  *types.Package
}

func NewStructVarCollector(info *types.Info, fset *token.FileSet, pkg *types.Package) *StructVarCollector {
	return &StructVarCollector{
		info: info,
		fset: fset,
		pkg: pkg,
	}
}

func DumpStructVar(w io.Writer, sv []*StructVar) {

}

// Collect returns all struct-typed local variables
func (c *StructVarCollector) Collect(funcDecl *ast.FuncDecl) []*StructVar {
	if funcDecl.Body == nil {
		return nil
	}

	paramSet := c.buildParamSet(funcDecl)
	var results []*StructVar

	seen := map[*types.Var]bool{}

	ast.Inspect(funcDecl.Body, func(n ast.Node) bool {

		switch node := n.(type) {
		// var v T or var v T = expr
		case *ast.GenDecl:
			if node.Tok != token.VAR {
				return true
			}

			for _, spec := range node.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, nameIdent := range vs.Names {
					obj := c.info.Defs[nameIdent]
					if obj == nil {
						continue
					}

					v, ok := obj.(*types.Var)
					if !ok || paramSet[v] || seen[v] {
						continue
					}

					seen[v] = true
					if sv := c.toStructVar(v, node); sv != nil {
						results = append(results, sv)
					}
				}
			}

		// v := expr, v, w := expr1, expr2
		case *ast.AssignStmt:
			if node.Tok != token.DEFINE {
				return true
			}
			for _, lhs := range node.Lhs {
				ident, ok := lhs.(*ast.Ident)
				if !ok || ident.Name == "_" {
					continue
				}
				obj := c.info.Defs[ident]
				if obj == nil {
					continue
				}
				v, ok := obj.(*types.Var)
				if !ok || paramSet[v] || seen[v] {
					continue
				}
				seen[v] = true
				if sv := c.toStructVar(v, node); sv != nil {
					results = append(results, sv)
				}
			}

		// for k, v := range ...
		case *ast.RangeStmt:
			if node.Tok != token.DEFINE {
				return true
			}
			for _, expr := range []ast.Expr{node.Key, node.Value} {
				if expr == nil {
					continue
				}
				ident, ok := expr.(*ast.Ident)
				if !ok || ident.Name == "_" {
					continue
				}
				obj := c.info.Defs[ident]
				if obj == nil {
					continue
				}
				v, ok := obj.(*types.Var)
				if !ok || paramSet[v] || seen[v] {
					continue
				}
				seen[v] = true
				if sv := c.toStructVar(v, node); sv != nil {
					results = append(results, sv)
				}
			}
		}

		return true
	})

	return results
}

func (c *StructVarCollector) toStructVar(v *types.Var, decl ast.Node) *StructVar {
	typ := v.Type()

	// ignore pointers
	if _, isPtr := typ.(*types.Pointer); isPtr {
		return nil
	}

	var named *types.Named
	if n, ok := typ.(*types.Named); ok {
		named = n
	}

	underlying := typ.Underlying()
	st, ok := underlying.(*types.Struct)
	if !ok {
		return nil
	}

	// Collect fields.
	fields := make([]*types.Var, st.NumFields())
	for i := range fields {
		fields[i] = st.Field(i)
	}

	return &StructVar{
		VarType: v,
		StructType: st,
		NamedType: named,
		DeclNode: decl,
		Fields: fields,
	}
}

func (c *StructVarCollector) buildParamSet(funcDecl *ast.FuncDecl) map[*types.Var]bool {
	set := map[*types.Var]bool{}
	obj := c.info.Defs[funcDecl.Name]
	if obj == nil {
		return set
	}
	fn, ok := obj.(*types.Func)
	if !ok {
		return set
	}
	sig := fn.Type().(*types.Signature)

	for i := 0; i < sig.Params().Len(); i++ {
		set[sig.Params().At(i)] = true
	}
	for i := 0; i < sig.Results().Len(); i++ {
		set[sig.Results().At(i)] = true
	}
	// Also include receiver
	if sig.Recv() != nil {
		set[sig.Recv()] = true
	}
	return set
}
