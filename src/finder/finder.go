package finder

import (
	"os"
	"io/fs"
	"path/filepath"
	"strings"
	"fmt"
)

var excludedDirs = map[string]bool{
	"vendor":       true,
	"node_modules": true,
	"testdata":     true,
	"doc":          true,
	"docs":         true,
	"assets":       true,
	"static":       true,
	"public":       true,
	"images":       true,
	"img":          true,
	".git":         true,
	".github":      true,
	".idea":        true,
	".vscode":      true,
}

func FindTestFiles(root string) []string {
	info, err := os.Stat(root)
	if err != nil {
		panic(fmt.Errorf("cannot stat path %s: %w", root, err))
	}
	if !info.IsDir() {
		panic(fmt.Errorf("%s is not a directory", root))
	}

	var testFiles []string

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			if excludedDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		if strings.HasSuffix(d.Name(), "_test.go") {
			testFiles = append(testFiles, path)
		}

		return nil
	})

	if err != nil {
		panic(fmt.Errorf("error walking directory %s: %w", root, err))
	}

	return testFiles
}