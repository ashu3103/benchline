package loader

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
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
	SelectedFuncs []*ast.FuncDecl
}

// LoadPackages loads Go packages using go/packages and returns them ready for analysis.
// It requires the packages to be loadable with full type information.
func LoadPackages(cfg *LoadConfig) ([]*LoadedPackage, error) {
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

	var loaded []*LoadedPackage
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

		selected := selectFuncs(pkg, cfg)
		if len(selected) == 0 {
			continue
		}

		loaded = append(loaded, &LoadedPackage{
			Pkg:           pkg,
			Fset:          pkg.Fset,
			Info:          pkg.TypesInfo,
			TypesPkg:      pkg.Types,
			SelectedFuncs: selected,
		})
	}

	return loaded, nil
}

// selectFuncs walks the parsed AST files and returns function declarations
// that pass the filter.
func selectFuncs(pkg *packages.Package, cfg *LoadConfig) []*ast.FuncDecl {
	var selected []*ast.FuncDecl

	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}

			name := fn.Name.Name

			if cfg.BenchmarkOnly && !strings.HasPrefix(name, "Benchmark") {
				continue
			}

			if cfg.FuncFilter != nil && !cfg.FuncFilter(name) {
				continue
			}

			selected = append(selected, fn)
		}
	}

	return selected
}