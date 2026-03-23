package analyzer

import (
	"fmt"
	"io"
	"sort"

	"go/ast"
	"go/token"
	"go/types"
)

type VerdictMessage int

const (
	VERDICT_SCALAR_REPLACABLE VerdictMessage = iota
	VERDICT_ADDRESS_TAKEN_AGGREGATE
	VERDICT_USE_AGGREGATE_WHOLE
	VERDICT_ADDRESS_TAKEN_METHOD_RECIEVER
	VERDICT_PASSED_AS_WHOLE
	VERDICT_ANONYMOUS
)

func (v *VerdictMessage) String() string {
	switch *v {
	case VERDICT_ADDRESS_TAKEN_AGGREGATE:
		return "ADDRESS_TAKEN_AGGREGATE"
	case VERDICT_SCALAR_REPLACABLE:
		return "SCALAR_REPLACABLE"
	case VERDICT_USE_AGGREGATE_WHOLE:
		return "USE_AGGREGATE_AS_WHOLE"
	case VERDICT_ADDRESS_TAKEN_METHOD_RECIEVER:
		return "ADDRESS_TAKEN_METHOD_RECIEVER"
	case VERDICT_PASSED_AS_WHOLE:
		return "PASSED_AS_WHOLE"
	case VERDICT_ANONYMOUS:
		return "ANONYMOUS"
	}

	return ""
}

type Verdict struct {
	Obj          types.Object
	Id           *ast.Ident
	Fset         *token.FileSet
	Info         *types.Info
	Promotable   bool
	ViolationMsg string // empty if promotable
	UseNum       int
}

func CheckViolation(r *DefUseChain) []Verdict {
	verdicts := make([]Verdict, 0)

	for obj, chain := range r.Chains {
		promotable, msg, useNum := checkChainViolation(obj, chain, r.Info)
		verdicts = append(verdicts, Verdict{
			Obj:          obj,
			Id:           chain.Id,
			Fset:         r.Fset,
			Info:         r.Info,
			Promotable:   promotable,
			ViolationMsg: msg,
			UseNum:       useNum,
		})
	}

	return verdicts
}

func checkChainViolation(obj types.Object, chain *VarUseChain, info *types.Info) (bool, string, int) {
	for i, stmt := range chain.Uses {
		if reason := violates(obj, stmt, info); reason != VERDICT_SCALAR_REPLACABLE {
			return false, reason.String(), i + 1
		}
	}
	return true, "", 0
}

func violates(obj types.Object, stmt ast.Stmt, info *types.Info) VerdictMessage {
	stmtVisitor := NewStmtVisitor(obj, info)
	ast.Walk(stmtVisitor, stmt)
	return stmtVisitor.reason
}

type StmtVisitor struct {
	obj     types.Object
	info    *types.Info
	violate bool
	reason  VerdictMessage
}

func NewStmtVisitor(obj types.Object, info *types.Info) *StmtVisitor {
	return &StmtVisitor{
		obj:     obj,
		info:    info,
		violate: false,
		reason:  VERDICT_SCALAR_REPLACABLE,
	}
}

func (v *StmtVisitor) Visit(node ast.Node) (w ast.Visitor) {
	if node == nil || v.violate == true {
		return nil
	}

	switch n := node.(type) {
	/* ---- handle statements ---- */
	case *ast.AssignStmt:
		for _, rhs := range n.Rhs {
			if ctx := v.checkExpr(rhs, ctxMayViolate); ctx != ctxSafe {
				v.violate = true
				return nil
			}
		}
		return nil
	case *ast.ReturnStmt:
		for _, r := range n.Results {
			if ctx := v.checkExpr(r, ctxMayViolate); ctx != ctxSafe {
                v.violate = true
				return nil
            }
		}
		return nil
	case *ast.ExprStmt:
		if ctx := v.checkExpr(n.X, ctxMayViolate); ctx != ctxSafe {
			v.violate = true
		}
		return nil
	case *ast.SendStmt:
        if ctx := v.checkExpr(n.Value, ctxMayViolate); ctx != ctxSafe {
            v.violate = true
        }
        return nil
	}

	return v
}

type exprContext int

const (
	ctxSafe exprContext = iota
	ctxMayViolate
	ctxViolate	
)

func (v *StmtVisitor) checkExpr(expr ast.Expr, ctx exprContext) exprContext {
	switch e := expr.(type) {
	case *ast.Ident:
		if v.info.Uses[e] != v.obj || ctx == ctxSafe {
            return ctxSafe
        }
        // found a: return what the caller passed so it propagates up
		switch ctx {
		case ctxMayViolate:   // used as whole
			v.reason = VERDICT_USE_AGGREGATE_WHOLE
		case ctxViolate:
			v.reason = VERDICT_ANONYMOUS
		}
		v.violate = true
        return ctxViolate
	case *ast.StarExpr:
        // dereference: heap indirection (safe)
        return v.checkExpr(e.X, ctxSafe)
	case *ast.SelectorExpr:
		// a.b: field access, downgrade ctxMayViolate to ctxSafe
        // but preserve ctxViolate (e.g &a.b: & came from above)
        inner := ctxSafe
        switch ctx {
        case ctxViolate:
            inner = ctxViolate  // &a.b — & propagates inward
        case ctxMayViolate:
            inner = ctxSafe     // a.b as whole read — field access is safe
        default:
            inner = ctxSafe
        }
        return v.checkExpr(e.X, inner)
	case *ast.IndexExpr:
		result := v.checkExpr(e.Index, ctxMayViolate)
        if result == ctxViolate {
			return ctxViolate
		}

        collType := v.info.Types[e.X].Type.Underlying()
        switch collType.(type) {
        case *types.Slice, *types.Map:
            // elements on heap: chain broken, ctx cannot reach a
            return v.checkExpr(e.X, ctxMayViolate)
        case *types.Array:
            // elements inline: propagate ctx inward
            return v.checkExpr(e.X, ctx)
        default:
            return v.checkExpr(e.X, ctx)
        }
	case *ast.ParenExpr:
		return v.checkExpr(e.X, ctx)
	case *ast.BinaryExpr:
		if v.checkExpr(e.X, ctx) == ctxViolate {
            return ctxViolate
        }
        if v.checkExpr(e.Y, ctx) == ctxViolate {
			return ctxViolate
        }
        return ctxSafe
	case *ast.CallExpr:
		for _, arg := range e.Args {
			if v.checkExpr(arg, ctxMayViolate) == ctxViolate {
				if v.reason == VERDICT_USE_AGGREGATE_WHOLE {
					v.reason = VERDICT_PASSED_AS_WHOLE
				}
				return ctxViolate
			}
		}

		sel, ok := e.Fun.(*ast.SelectorExpr)
        if !ok {
            return v.checkExpr(e.Fun, ctxMayViolate)
        }

        if selection, ok := v.info.Selections[sel]; ok {
            fn, ok := selection.Obj().(*types.Func)
            if !ok {
                return ctxViolate
            }
            sig := fn.Type().(*types.Signature)
            _, isPtr := sig.Recv().Type().(*types.Pointer)

            if selection.Indirect() {
                // implicit deref — heap indirection
                return v.checkExpr(sel.X, ctx)
            }

            if isPtr {
                // pointer receiver — pass ctxViolate inward
                res := v.checkExpr(sel.X, ctxViolate)
				if res == ctxViolate && v.reason == VERDICT_ANONYMOUS {
					v.reason = VERDICT_ADDRESS_TAKEN_METHOD_RECIEVER
				}
				return res
            } else {
				if _, ok := sel.X.(*ast.Ident); ok {
					return v.checkExpr(sel.X, ctxSafe)
				}

                // value receiver — chain broken
                return v.checkExpr(sel.X, ctxMayViolate)
            }
        } else {
            return v.checkExpr(e.Fun, ctx)
        }
	case *ast.UnaryExpr:
		if e.Op != token.AND {
			return v.checkExpr(e.X, ctx)
		}

		result := v.checkExpr(e.X, ctxViolate)
		switch result {
		case ctxViolate:
			if v.reason == VERDICT_ANONYMOUS {
				v.reason = VERDICT_ADDRESS_TAKEN_AGGREGATE
			}
			return ctxViolate
		}
		return ctxSafe
	case *ast.BasicLit:
		return ctxSafe
	case *ast.CompositeLit:
		for _, elt := range e.Elts {
			var val ast.Expr
			switch kv := elt.(type) {
			case *ast.KeyValueExpr:
				val = kv.Value
			default:
				val = elt
			}
			result := v.checkExpr(val, ctxMayViolate)
			switch result {
			case ctxViolate:
				return ctxViolate
			}
		}
		return ctxSafe
	}

	return ctxViolate
}

func DumpVerdicts(w io.Writer, verdicts []Verdict) {
	if len(verdicts) == 0 {
		fmt.Fprintf(w, "no verdicts found\n")
		return
	}

	// Partition into promotable and non-promotable for summary.
	promotable, rejected := 0, 0
	for _, v := range verdicts {
		if v.Promotable {
			promotable++
		} else {
			rejected++
		}
	}

	// Sort by variable name for deterministic output.
	sorted := make([]Verdict, len(verdicts))
	copy(sorted, verdicts)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Obj.Name() < sorted[j].Obj.Name()
	})

	// Pre-compute widest name for alignment.
	maxNameLen := 0
	for _, v := range sorted {
		if l := len(v.Obj.Name()); l > maxNameLen {
			maxNameLen = l
		}
	}

	fmt.Fprintf(w, "┌─ verdicts  (%d total  ✓ %d promotable  ✗ %d rejected)\n",
		len(verdicts), promotable, rejected)

	for vi, v := range sorted {
		obj := v.Obj
		pos := v.Fset.Position(obj.Pos())

		// Resolve identifier type via types.Info if available.
		typeStr := obj.Type().String()
		if v.Info != nil {
			if tv, ok := v.Info.Defs[v.Id]; ok && tv != nil {
				typeStr = types.TypeString(tv.Type(), nil)
			}
		}

		// Package the object belongs to.
		pkgName := "<unknown>"
		if pkg := obj.Pkg(); pkg != nil {
			pkgName = pkg.Name()
		}

		// Verdict symbol and label.
		symbol, label := "✓", "PROMOTABLE"
		if !v.Promotable {
			symbol, label = "✗", "REJECTED  "
		}

		fmt.Fprintf(w, "│   %s  %-*s  %-10s  %s:%d:%d\n",
			symbol,
			maxNameLen, obj.Name(),
			pkgName,
			pos.Filename, pos.Line, pos.Column,
		)

		// Scope / object kind (Var, Const, Func, etc.)
		fmt.Fprintf(w, "│        type    : %s\n", typeStr)

		// Exported status.
		exported := "no"
		if obj.Exported() {
			exported = "yes"
		}
		fmt.Fprintf(w, "│        exported: %s\n", exported)

		// Verdict outcome.
		fmt.Fprintf(w, "│        verdict : %s", label)
		if !v.Promotable && v.ViolationMsg != "" {
			fmt.Fprintf(w, " —  %s [Violation Use: %d]", v.ViolationMsg, v.UseNum)
		}
		fmt.Fprintln(w)

		if vi < len(sorted)-1 {
			fmt.Fprintf(w, "│\n")
		}
	}

	fmt.Fprintf(w, "│\n")
	fmt.Fprintf(w, "└─────\n\n")
}
