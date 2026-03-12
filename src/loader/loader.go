package loader

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"io"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"
)

// LoadConfig controls which functions are analyzed.
type LoadConfig struct {
	Dir string
	Patterns []string
	BenchmarkOnly bool
	FuncFilter func(funcName string) bool
	BuildFlags []string
}

// LoadedPackage wraps a loaded package with the data needed for SROA analysis.
type LoadedPackage struct {
	Pkg      *packages.Package
	Fset     *token.FileSet
	Info     *types.Info
	TypesPkg *types.Package
	// SelectedFuncs []*ast.FuncDecl
}

type globalIndex struct {
	decl   map[*types.Func]*ast.FuncDecl
	pkg    map[*types.Func]*packages.Package
	dirAbs string
}

// LoadPackages loads Go packages using go/packages and returns them ready for analysis.
// It requires the packages to be loadable with full type information.
func LoadPackages(cfg *LoadConfig) ([]*LoadedPackage, error) {
	// dirAbs, err := filepath.Abs(cfg.Dir)
	// if err != nil {
	// 	return nil, fmt.Errorf("resolving dir: %w", err)
	// }

	pkgCfg := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedSyntax |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedTypesSizes |
			packages.NeedImports,
		Dir:        cfg.Dir,
		BuildFlags: cfg.BuildFlags,
		// Include test files so we can analyze benchmark functions.
		Tests: true,
	}

	pkgs, err := packages.Load(pkgCfg, cfg.Patterns...)
	if err != nil {
		return nil, fmt.Errorf("packages.Load: %w", err)
	}

	// idx := buildGlobalIndex(pkgs, dirAbs)

	var loaded []*LoadedPackage
	visited := make(map[string]int, 0)

	for _, pkg := range pkgs {
		if len(pkg.Errors) > 0 {
			// Report but don't abort — partial results are useful.
			for _, e := range pkg.Errors {
				fmt.Printf("warning: package %s: %v\n", pkg.PkgPath, e)
			}
		}
		if pkg.TypesInfo == nil {
			continue
		}

		// Skip external test packages e.g. "add_test [add.test]"
		if strings.Contains(pkg.ID, "_test [") {
			continue
		}
		// Skip test binary e.g. "add.test"
		if strings.HasSuffix(pkg.ID, ".test") && !strings.Contains(pkg.ID, "[") {
			continue
		}

		// entryPoints := collectEntryPoints(pkg, cfg)
		// if len(entryPoints) == 0 {
		// 	continue
		// }

		// selected := expandCallees(entryPoints, pkg, idx)
		// if len(selected) == 0 {
		// 	continue
		// }

		// Normalize ID — "add [add.test]" → "add"
		baseID := pkg.ID
		if idx := strings.Index(baseID, " ["); idx != -1 {
			baseID = baseID[:idx]
		}

		_, exists := visited[baseID]

		if !exists {
			loaded = append(loaded, &LoadedPackage{
				Pkg:           pkg,
				Fset:          pkg.Fset,
				Info:          pkg.TypesInfo,
				TypesPkg:      pkg.Types,
				// SelectedFuncs: selected,
			})
			visited[baseID] = len(loaded) - 1
		} else if strings.Contains(pkg.ID, "[") {
			loaded[visited[baseID]] = &LoadedPackage{
				Pkg:           pkg,
				Fset:          pkg.Fset,
				Info:          pkg.TypesInfo,
				TypesPkg:      pkg.Types,
			}
		}
		// if strings.Contains(pkg.ID, "[") {
		// 	existing.Pkg  = pkg
		// 	existing.Info = pkg.TypesInfo
		// } else if  {
		// 	// plain variant — only set if we haven't seen augmented yet
		// 	existing.Pkg  = pkg
		// 	existing.Info = pkg.TypesInfo
		// }
	}

	return loaded, nil
}

// Dump the loaded packages
func DumpLoadedPackages(w io.Writer, loaded []*LoadedPackage) {
	if len(loaded) == 0 {
		fmt.Fprintf(w, "no package loaded")
	}

	for _, lp := range loaded {
		fmt.Fprintf(w, "package %s (%s)\n", lp.Pkg.PkgPath, lp.Pkg.Name)

		// if len(lp.SelectedFuncs) == 0 {
		// 	fmt.Fprintf(w, "  no selected functions")
		// 	continue
		// }

		// for i, fn := range lp.SelectedFuncs {
		// 	pos := lp.Fset.Position(fn.Pos())
		// 	filename := pos.Filename
		// 	if rel := trimDirPrefix(filename, lp.Pkg); rel != "" {
		// 		filename = rel
		// 	}
		// 	fmt.Fprintf(
		// 		w, "  [%d] %-30s %s:%d\n",
		// 		i + 1, fn.Name.Name, filename, pos.Line,
		// 	)
		// }
	}

}

func trimDirPrefix(filename string, pkg interface{ String() string }) string {
	// Use the last two path components: dir/file.go — enough to be unambiguous
	// without requiring os.Getwd or module root resolution.
	sep := '/'
	// Find second-to-last separator.
	idx := -1
	count := 0
	for i := len(filename) - 1; i >= 0; i-- {
		if rune(filename[i]) == sep {
			count++
			if count == 2 {
				idx = i
				break
			}
		}
	}
	if idx >= 0 {
		return filename[idx+1:]
	}
	return ""
}

// inDir reports whether the file containing pos is rooted under dirAbs.
func (idx *globalIndex) inDir(tfn *types.Func) bool {
	pos := tfn.Pos()
	if !pos.IsValid() {
		return false
	}
	// types.Func carries a *token.FileSet position; we need the filename.
	// We recover it from the package's Fset via the owning package.
	pkg, ok := idx.pkg[tfn]
	if !ok {
		return false
	}
	filename := pkg.Fset.Position(pos).Filename
	abs, err := filepath.Abs(filename)
	if err != nil {
		return false
	}
	return strings.HasPrefix(abs, idx.dirAbs+string(filepath.Separator)) ||
		abs == idx.dirAbs
}

// collectEntryPoints returns the benchmark (or filtered) function declarations
// from a single package. These are the roots for callee expansion.
func collectEntryPoints(pkg *packages.Package, cfg *LoadConfig) []*ast.FuncDecl {
	var entries []*ast.FuncDecl
	for _, file := range pkg.Syntax {
		for _, d := range file.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			name := fn.Name.Name
			if cfg.BenchmarkOnly && !strings.HasPrefix(name, "Benchmark") {
				continue
			}
			entries = append(entries, fn)
		}
	}
	return entries
}

// buildGlobalIndex indexes every function declaration across all loaded packages.
func buildGlobalIndex(pkgs []*packages.Package, dirAbs string) *globalIndex {
	idx := &globalIndex{
		decl: make(map[*types.Func]*ast.FuncDecl),
		pkg: make(map[*types.Func]*packages.Package),
		dirAbs: dirAbs,
	}

	for _, pkg := range pkgs {
		if pkg.TypesInfo == nil {
			continue
		}

		for _, file := range pkg.Syntax {
			for _, d := range file.Decls {
				fn, ok := d.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}

				obj, ok := pkg.TypesInfo.Defs[fn.Name]
				if !ok || obj == nil {
					continue
				}
				tfn, ok := obj.(*types.Func)
				if !ok {
					continue
				}
				idx.decl[tfn] = fn
				idx.pkg[tfn] = pkg
			}
		}
	}

	return idx
}

// expandCallees performs a BFS starting from the entry-point declarations.
// For every call expression it finds in a function body, it resolves the callee
// via TypesInfo, checks whether the callee's source file is under Config.Dir,
// and if so enqueues it. The returned slice contains both the entry points and
// all transitively reachable in-dir callees, deduplicated.
//
// The owning package of a callee may differ from the entry-point package
// (e.g. BenchmarkNewLoda is in loda_test, NewLoda is in loda). The index
// carries the correct TypesInfo for each function so we always resolve
// identifiers against the right package.
func expandCallees(
	entryPoints []*ast.FuncDecl,
	entryPkg *packages.Package,
	idx *globalIndex,
) []*ast.FuncDecl {
	// visited tracks types.Func pointers we have already enqueued.
	visited := make(map[*types.Func]bool)
	// result is the ordered output slice (entry points first).
	var result []*ast.FuncDecl

	// queue items pair a declaration with the TypesInfo of the package that
	// owns it — needed to resolve call expressions inside that declaration.
	type queueItem struct {
		decl    *ast.FuncDecl
		ownerPkg *packages.Package
	}
	queue := make([]queueItem, 0, len(entryPoints))

	// Seed the queue with entry points. Entry points belong to entryPkg.
	for _, fn := range entryPoints {
		obj, ok := entryPkg.TypesInfo.Defs[fn.Name]
		if !ok || obj == nil {
			// Still include the entry point itself even if we can't resolve it.
			result = append(result, fn)
			continue
		}
		tfn, ok := obj.(*types.Func)
		if !ok {
			result = append(result, fn)
			continue
		}
		if !visited[tfn] {
			visited[tfn] = true
			result = append(result, fn)
			queue = append(queue, queueItem{fn, entryPkg})
		}
	}

	// BFS expansion.
	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]

		// Walk the function body looking for call expressions.
		ast.Inspect(item.decl.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			// Resolve the callee identifier. We handle three shapes:
			//   f(...)           → *ast.Ident
			//   pkg.F(...)       → *ast.SelectorExpr
			//   x.Method(...)    → *ast.SelectorExpr (method call)
			var calleeObj types.Object
			switch fun := call.Fun.(type) {
			case *ast.Ident:
				calleeObj = item.ownerPkg.TypesInfo.Uses[fun]
			case *ast.SelectorExpr:
				calleeObj = item.ownerPkg.TypesInfo.Uses[fun.Sel]
			}
			if calleeObj == nil {
				return true
			}

			tfn, ok := calleeObj.(*types.Func)
			if !ok || visited[tfn] {
				return true
			}

			// Only include callees whose source lives under Config.Dir.
			if !idx.inDir(tfn) {
				return true
			}

			calleePkg, hasPkg := idx.pkg[tfn]
			calleeDecl, hasDecl := idx.decl[tfn]
			if !hasPkg || !hasDecl {
				// The function is in-dir but we have no AST for it
				// (e.g. a function literal stored in a variable). Skip.
				return true
			}

			visited[tfn] = true
			result = append(result, calleeDecl)
			queue = append(queue, queueItem{calleeDecl, calleePkg})
			return true
		})
	}

	return result
}