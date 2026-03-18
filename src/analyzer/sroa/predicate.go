package analyzer

import (
	"go/ast"
	"go/token"
	"go/types"
)

type VerdictMessage int

const (
	VERDICT_SCALAR_REPLACABLE VerdictMessage = iota
	VERDICT_ADDRESS_TAKEN_AGGREGATE
	VERDICT_USE_AGGREGATE_FIELD
	VERDICT_USE_AGGREGATE_WHOLE
)

func (v *VerdictMessage) String() string {
	switch *v {
	case VERDICT_ADDRESS_TAKEN_AGGREGATE:
		return "ADDRESS_TAKEN_AGGREGATE"
	case VERDICT_SCALAR_REPLACABLE:
		return "SCALAR_REPLACABLE"
	case VERDICT_USE_AGGREGATE_WHOLE:
		return "USE_AGGREGATE_AS_WHOLE"
	case VERDICT_USE_AGGREGATE_FIELD:
		return "USE_AGGREGATE_FIELD"
	}

	return ""
}

type Verdict struct {
	Obj           types.Object
	Promotable    bool
	ViolationMsg  string   // empty if promotable
}

func CheckViolation(r *DefUseChain) []Verdict {
	verdicts := make([]Verdict, 0)

	for obj, chain := range r.Chains {
		promotable, msg := checkChainViolation(obj, chain, r.Info)
		verdicts = append(verdicts, Verdict{
			Obj: obj,
			Promotable: promotable,
			ViolationMsg: msg,
		})
	}

	return verdicts
}

func checkChainViolation(obj types.Object, chain *VarUseChain, info *types.Info) (bool, string) {
	for _, stmt := range chain.Uses {
		if reason := violates(obj, stmt, info); reason != VERDICT_SCALAR_REPLACABLE {
			return false, reason.String()
		}
	}
	return true, ""
}

func violates(obj types.Object, stmt ast.Stmt, info *types.Info) VerdictMessage {
	stmtVisitor := NewStmtVisitor(obj, info)
	ast.Walk(stmtVisitor, stmt)
	return stmtVisitor.reason
}

type StmtVisitor struct {
	obj    types.Object
	info   *types.Info
	unwind bool
	reason VerdictMessage
}

func NewStmtVisitor(obj types.Object, info *types.Info) *StmtVisitor {
	return &StmtVisitor{
		obj: obj,
		info: info,
		unwind: false,
		reason: VERDICT_SCALAR_REPLACABLE,
	}
}

func (v *StmtVisitor) Visit(node ast.Node) (w ast.Visitor) {
	if node == nil && v.unwind == true {
		return nil
	}

	switch n := node.(type) {
	case *ast.UnaryExpr:
		if n.Op == token.ADD && exprHasObject(n.X, v.obj, v.info) {
			v.unwind = true
			v.reason = VERDICT_ADDRESS_TAKEN_AGGREGATE
		}
	case *ast.SelectorExpr:
		if isExprTypeStruct(n, v.info) && exprHasObject(n.X, v.obj, v.info) {
			v.unwind = true
			v.reason = VERDICT_USE_AGGREGATE_FIELD
		}
	/* base case */
	case *ast.Ident:
		if v.info.Uses[n] == v.obj {
			v.unwind = true
			v.reason = VERDICT_USE_AGGREGATE_WHOLE
		}
	}
	
	return v
}

func exprHasObject(e ast.Expr, obj types.Object, info *types.Info) bool {
	hasObj := false
	ast.Inspect(e, func(n ast.Node) bool {

		if e == nil || hasObj {
			return false
		}

		switch exp := e.(type) {
		case *ast.Ident:
			if rootObj(exp, info) == obj {
				hasObj = true
				return false
			}
		}
		return true
	})
	return hasObj
}

func isExprTypeStruct(e ast.Expr, info *types.Info) bool {
	typ := info.Types[e].Type.Underlying()
	if _, ok := typ.(*types.Struct); ok {
		return true
	}
	return false
}

// func isExprIdent(e ast.Expr) bool {
// 	if _, ok := e.(*ast.Ident); ok {
// 		return true
// 	}
// 	return false
// }

// func isBasicType(t types.Type) bool {
//     _, ok := t.Underlying().(*types.Basic)
//     return ok
// }

func rootObj(exp ast.Expr, info *types.Info) types.Object {
	ident, ok := exp.(*ast.Ident)
	if !ok { return nil }
	return info.Uses[ident]
}