package analyzer

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"io"
	"slices"
	"strings"

	p "github/ashu3103/benchline/src/parser"
)

// struct to store uses of a variable
type VarUseChain struct {
	Obj   types.Object
	Decl  ast.Stmt
	Id    *ast.Ident
	Uses  []ast.Stmt
}

type DefUseChain struct {
	FuncDecl    *ast.FuncDecl
	Chains      map[types.Object]*VarUseChain
	Info        *types.Info
	Fset        *token.FileSet
	EscapedVars []p.EscapeInfo
	Escaped     bool
	StmtStack   []ast.Stmt
}

func NewDefUseChain(node *ast.FuncDecl, info *types.Info, fset *token.FileSet, ev []p.EscapeInfo) *DefUseChain {
	return &DefUseChain{
		FuncDecl:    node,
		Info:        info,
		Fset:        fset,
		EscapedVars: ev,
		Chains:      make(map[types.Object]*VarUseChain),
	}
}

func (du *DefUseChain) isEscaped(id *ast.Ident) bool {
	startLine := du.Fset.Position(du.FuncDecl.Body.Lbrace).Line
	endLine   := du.Fset.Position(du.FuncDecl.Body.Rbrace).Line
	evs := p.FilterEscapedVariables(du.EscapedVars, startLine, endLine)

	for _, ev := range evs {
		if du.varMatch(id, ev) {
			du.Escaped = true
			return true
		}
	}
	return false
}

func (du *DefUseChain) varMatch(id *ast.Ident, ev p.EscapeInfo) bool {
	line := du.Fset.Position(id.Pos()).Line
	col  := du.Fset.Position(id.Pos()).Column

	if id.Name == ev.VarName && line == ev.Line && ev.Col == col {
		return true 
	}

	return false
}

type FunctionBodyVisitor struct {
	*DefUseChain
	Pushed  bool
}

func NewFunctionBodyVisitor(defUseChain *DefUseChain) *FunctionBodyVisitor {
	return &FunctionBodyVisitor{
		DefUseChain: defUseChain,
		Pushed: false,
	}
}

func CreateDefUseChains(node *ast.FuncDecl, info *types.Info, fset *token.FileSet, ev []p.EscapeInfo) *DefUseChain {
	defUseChain := NewDefUseChain(node, info, fset, ev)
	funcBodyVis := NewFunctionBodyVisitor(defUseChain)
	ast.Walk(funcBodyVis, node)

	if funcBodyVis.Escaped {
		return nil
	}

	return funcBodyVis.DefUseChain
}

func (v *FunctionBodyVisitor) Visit(n ast.Node) (w ast.Visitor) {
	if n == nil || v.Escaped {
		return nil
	}

	switch node := n.(type) {
	/* ---- handle definitions ---- */
	case *ast.AssignStmt:
		v.StmtStack = append(v.StmtStack, node)
		v.Pushed = true
		v.collectStructDef(node)
	case *ast.DeclStmt:
		v.StmtStack = append(v.StmtStack, node)
		v.Pushed = true
		v.collectStructDef(node)
	
	/* ---- handle uses ---- */
	case *ast.DeferStmt, *ast.ExprStmt, *ast.LabeledStmt, *ast.IncDecStmt,
			*ast.SendStmt, *ast.ReturnStmt:
		v.StmtStack = append(v.StmtStack, node.(ast.Stmt))
	case *ast.Ident:
		obj := v.Info.Uses[node]
		if obj == nil { return nil }

		if v.isEscaped(node) {
			v.Escaped = true
		}

		if chain, tracked := v.Chains[obj]; tracked {
			if stmt := v.currentLeaf(); stmt != nil {
				chain.Uses = appendUniq(chain.Uses, stmt)
			}
		}
		return nil
	}

	return v
}

func DumpDefUseChain(w io.Writer, du *DefUseChain) {
	if du == nil || len(du.Chains) == 0 {
		fmt.Fprintf(w, "no def-use chains found\n")
		return
	}

	totalChains := len(du.Chains)

	// Pre-compute widest variable name for alignment.
	maxNameLen := 0
	for obj := range du.Chains {
		if l := len(obj.Name()); l > maxNameLen {
			maxNameLen = l
		}
	}

	fmt.Fprintf(w, "┌─ def-use chains  (%d variable(s))\n", totalChains)

	duInd := 0
	for obj := range du.Chains {
		chain := du.Chains[obj]

		// Variable header
		pos := obj.Pos()
		if duInd == (totalChains - 1) {
			fmt.Fprintf(w, "│   └ \"%-*s\"  {%s}  (declared at %s)\n",
				maxNameLen, obj.Name(),
				formatType(obj.Type()),
				formatPos(du.Fset.Position(pos)),
			)
		} else {
			fmt.Fprintf(w, "│   ├ %-*s  {%s}  (declared at %s)\n",
				maxNameLen, obj.Name(),
				formatType(obj.Type()),
				formatPos(du.Fset.Position(pos)),
			)
		}

		// Use sites.
		if len(chain.Uses) == 0 {
			fmt.Fprintf(w, "│      └ no uses\n")
		} else {
			for i, use := range chain.Uses {
				connector := "├"
				if i == len(chain.Uses)-1 {
					connector = "└"
				}
				fmt.Fprintf(w, "│      %s [%*d]  use  →  %s\n",
					connector,
					len(fmt.Sprint(len(chain.Uses))), i+1,
					formatStmt(use),
				)
			}
		}

		if duInd < totalChains - 1 {
			fmt.Fprintf(w, "│\n")
		}
	}

	fmt.Fprintf(w, "│\n")
	fmt.Fprintf(w, "└───────\n")
}

// formatStmt produces a compact single-line representation of a statement
// for display. Falls back to the Go node type name if printing fails.
func formatStmt(stmt ast.Stmt) string {
	if stmt == nil {
		return "<nil>"
	}
	var buf bytes.Buffer
	if err := format.Node(&buf, token.NewFileSet(), stmt); err != nil {
		return fmt.Sprintf("<%T>", stmt)
	}
	// Collapse newlines so the output stays on one line.
	line := strings.ReplaceAll(buf.String(), "\n", " ")
	line = strings.Join(strings.Fields(line), " ")
	if len(line) > 60 {
		line = line[:57] + "..."
	}
	return line
}

func formatType(objType types.Type) string {
	var result string

	typ := objType.String()

	s := strings.Split(typ, ".")
	if len(s) >= 2 {
		typ = s[1]

	}

	result = result + typ

	return result
}

func formatPos(pos token.Position) string {
	p := pos.String()
	s := strings.Split(p, "/")
	
	return s[len(s) - 1]
}

// collect definition site of a struct variable
func (v *FunctionBodyVisitor) collectStructDef(node ast.Stmt) {
	switch n := node.(type) {
	case *ast.AssignStmt:
		if n.Tok != token.DEFINE { return }  // token must be :=
		for _, lhs := range n.Lhs {
			l, ok := lhs.(*ast.Ident)
			if !ok { continue }
			obj := v.Info.Defs[l]
			if obj == nil { continue }

			if v.isEscaped(l) {
				v.Escaped = true
				return
			}

			if isStructType(obj.Type()) {
				v.Chains[obj] = &VarUseChain{
					Obj: obj,
					Decl: v.currentLeaf(),
					Id: l,
				}
			}
		}
	case *ast.DeclStmt:
		gen, ok := n.Decl.(*ast.GenDecl)     // *GenDecl with CONST, TYPE, or VAR token
		if !ok { return }

		for _, spec := range gen.Specs {
        vs, ok := spec.(*ast.ValueSpec)
        if !ok { continue }
        for _, name := range vs.Names {
			obj := v.Info.Defs[name]
            if obj == nil { continue }

			if v.isEscaped(name) {
				v.Escaped = true
				return
			}

            if isStructType(obj.Type()) {
                v.Chains[obj] = &VarUseChain{
					Obj: obj,
					Decl: v.currentLeaf(),
					Id: name,
				}
            }
        }
    }

	default:
		return
	}
}

// return the top of the stack
func (v *FunctionBodyVisitor) currentLeaf() ast.Stmt {
	if len(v.StmtStack) == 0 { return nil }
	return v.StmtStack[len(v.StmtStack) - 1]
}

// check if the type is of an aggregate
func isStructType(t types.Type) bool {
    _, ok := t.Underlying().(*types.Struct)
    return ok
}

// append if not already present
func appendUniq(stmts []ast.Stmt, s ast.Stmt) []ast.Stmt {
	if slices.Contains(stmts, s) { return stmts }
	return append(stmts, s)
}
