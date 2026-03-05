package injector

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

const (
	BENCHMARK_PREFIX          = "benchmark_"
	TESTING_FUNCTION_PREFIX   = "Test"
	BENCHMARK_FUNCTION_PREFIX = "Benchmark"
)

func checkPermissions(file string) error {
	dir := filepath.Dir(file)

	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("cannot stat directory %s: %w", dir, err)
	}

	perms := info.Mode().Perm()
	currentUID := os.Getuid()
	currentGID := os.Getgid()

	dirStat := info.Sys().(*syscall.Stat_t)
	ownerUID := int(dirStat.Uid)
	ownerGID := int(dirStat.Gid)

	var isPermitted bool
	switch {
	case currentUID == 0: // root can always write
		isPermitted = true
	case currentUID == ownerUID: // check owner bits
		isPermitted = perms&0200 != 0
	case currentGID == ownerGID: // check group bits
		isPermitted = perms&0020 != 0
	default:
		isPermitted = perms&0002 != 0
	}

	if !isPermitted {
		return fmt.Errorf("no write permissions in %s (mode: %s)", dir, perms)
	}

	return nil
}

func modifyName(fn *ast.FuncDecl) error {
	oldName := fn.Name.Name

	if !strings.HasPrefix(oldName, TESTING_FUNCTION_PREFIX) {
		return fmt.Errorf("unexpected function declaration, not a testing function")
	}

	newName := strings.ReplaceAll(oldName, TESTING_FUNCTION_PREFIX, BENCHMARK_FUNCTION_PREFIX)
	fn.Name.Name = newName

	return nil
}

func modifyParamAndType(fn *ast.FuncDecl) error {
	done := 0
	/* modify parameter type */
	fieldList := fn.Type.Params.List
	for _, field := range fieldList {
		if len(field.Names) == 1 {
			/* filter field type of struct */
			if starExpr, ok := field.Type.(*ast.StarExpr); ok {
				if selExpr, ok := starExpr.X.(*ast.SelectorExpr); ok {
					pkg, ok := selExpr.X.(*ast.Ident);
					if ok && pkg.Name == "testing" && selExpr.Sel.Name == "T" {
						if done == 1 {
							return fmt.Errorf("parameter list is ambiguos, cannot modify this function")
						}
						selExpr.Sel.Name = "B"
						done = 1
					}
				}
			}
		}
	}

	if done == 0 {
		return fmt.Errorf("no parameter matches the testing.T type")
	}

	return nil
}

func modifyFunction(fn *ast.FuncDecl) error {
	/* clear all docs */
	fn.Doc = nil

	/* change name */
	err := modifyName(fn)
	if err != nil {
		return err
	}

	/* change parameter name and type */
	err = modifyParamAndType(fn)
	if err != nil {
		return err
	}

	return nil
}

func inject(node *ast.File) error {
	/* out of all decls select the FuncDecl with name TestXxx */
	var filteredFuncDecls []ast.Decl
	var filteredImportDecls []ast.Decl

	for _, decl := range node.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		// keep if its a testing function, drop everything else
		if ok && strings.HasPrefix(fn.Name.Name, TESTING_FUNCTION_PREFIX) {
			/* change body of the filtered Testing functions */
			err := modifyFunction(fn)

			if err != nil {
				log.Printf("error: %v while modifying function %v...skipping", err, fn.Name.Name)
			} else {
				filteredFuncDecls = append(filteredFuncDecls, fn)
			}
		}

		// keep the import declarations
		gen, ok := decl.(*ast.GenDecl)
		if ok && gen.Tok == token.IMPORT {
			filteredImportDecls = append(filteredImportDecls, gen)
		}
	}

	/* clear any comments in benchmarks */
	clear(node.Comments)

	/* replace the decls in the node */
	node.Decls = filteredImportDecls
	node.Decls = append(node.Decls, filteredFuncDecls...)
	return nil
}

// parse the file
func InjectBenchmark(file string) {
	/* Check for file permissions */
	err := checkPermissions(file)
	if err != nil {
		panic(err)
	}

	/* create AST by parsing the test source file */
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, file, nil, 0)
	if err != nil {
		panic(err)
	}

	/* modify the AST nodes in-place */
	err = inject(node)
	if err != nil {
		panic(err)
	}

	/* create a new file for becnhmarks */
	outFile := filepath.Join(filepath.Dir(file), BENCHMARK_PREFIX+filepath.Base(file))
	out, err := os.Create(outFile)
	if err != nil {
		panic(err)
	}
	defer out.Close() // this ensures file is closed at the end of the InjectBecnhmark function

	/* write the generated becnhmark code to the file */
	_, err = out.WriteString("// --- Generated Benchmark Code ---\n")
	if err != nil {
		panic(err)
	}

	err = printer.Fprint(out, fset, node)
	if err != nil {
		panic(err)
	}
}
