package runner

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func RunBecnhmarkTests(projDir string) {
	info, err := os.Stat(projDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot stat %s: %v\n", projDir, err)
		os.Exit(1)
	}
	if !info.IsDir() {
		fmt.Fprintf(os.Stderr, "%s is not a directory\n", projDir)
		os.Exit(1)
	}

	absDir, err := filepath.Abs(projDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot resolve absolute path: %v\n", err)
		os.Exit(1)
	}

	cmd := exec.Command("go", "test", "-v", "-bench=.", "-benchmem", "-run=^$", "./...")
	cmd.Dir = absDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "benchmark failed: %v\n", err)
		os.Exit(1)
	}
}