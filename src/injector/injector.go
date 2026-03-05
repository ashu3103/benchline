package injector

import (
	"fmt"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"syscall"
)

const (
	BENCHMARK_PREFIX = "benchmark_"
	MINIMUM_PERMISSION = 0o666
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
	case currentUID == 0:     // root can always write
		isPermitted = true
	case currentUID == ownerUID:  // check owner bits
		isPermitted = perms&0200 != 0
	case currentGID == ownerGID:  // check group bits
		isPermitted = perms&0020 != 0
	default:
		isPermitted = perms&0002 != 0
	}

	if !isPermitted {
		return fmt.Errorf("no write permissions in %s (mode: %s)", dir, perms)
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