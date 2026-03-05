package injector

import (
	"os"
	"path/filepath"
	"go/parser"
	"go/printer"
	"go/token"
)

const (
	BENCHMARK_PREFIX = "benchmark_"
)

// parse the file
func InjectBenchmark(file string) {
	/* create AST by parsing the test source file */
	fset := token.FileSet{}
	node, err := parser.ParseFile(&fset, file, nil, 0)
	if err != nil {
		panic(err)
	}

	/* TODO: modify the AST nodes in-place */

	/* create a new file for becnhmarks */
	outFile := filepath.Join(filepath.Dir(file), BENCHMARK_PREFIX + filepath.Base(file))
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

	err = printer.Fprint(out, &fset, node)
	if err != nil {
		panic(err)
	}
}