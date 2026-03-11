package injector

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"log"
	"os"
	"path/filepath"
	"syscall"

	"golang.org/x/tools/imports"
)

const (
	BENCHMARK_PREFIX          = "benchmark_"
	TESTING_FUNCTION_PREFIX   = "Test"
	BENCHMARK_FUNCTION_PREFIX = "Benchmark"

	NOOP_FUNCTION_NAME        = "BENCHLINE_NOOP"
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

func transformFile(node *ast.File) error {
	/* clear any comments in benchmarks */
	node.Comments = nil

	fileVisitor := &FileVisitor{}
	for _, decl := range node.Decls {
		ast.Walk(fileVisitor, decl)
	}

	/* replace the decls in the node */
	node.Decls = fileVisitor.IDecls

	for _, fdecl := range fileVisitor.FDecls {
		node.Decls = append(node.Decls, fdecl)
	}

	node.Decls = append(node.Decls, &ast.FuncDecl{
		Name: ast.NewIdent(NOOP_FUNCTION_NAME),
		Type: &ast.FuncType{
            Params:  &ast.FieldList{},
            Results: nil,
        },
        Body: &ast.BlockStmt{
            List: nil,
        },
	})

	funcVisitor := &FunctionVisitor{}
	for _, decl := range node.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Name.Name != NOOP_FUNCTION_NAME {
			ast.Walk(funcVisitor, fn)
			if funcVisitor.Err != nil {
				return funcVisitor.Err
			}
			/* wrap the function body in b.Loop */
			wrapInBenchmarkLoop(fn)
		}
	}

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
	err = transformFile(node)
	if err != nil {
		panic(err)
	}

	/* reconstruct ast back to source go code */
	var buf bytes.Buffer
	err = printer.Fprint(&buf, fset, node)
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

	/* remove unused imports and adds missing ones */
	result, err := imports.Process(outFile, buf.Bytes(), nil)
	if err != nil {
		panic(err)
	}

	/* write the generated benchmark code to the file */
	err = os.WriteFile(outFile, result, 0666)
	if err != nil {
		log.Fatal(err)
	}
}
