package runner

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

/* 
   execute the go test benchmark command for a directory and flush the
   results in a log file
*/
func RunBecnhmarkTests(projDir string, logFile string) {
	info, err := os.Stat(projDir)
	if err != nil {
		panic(fmt.Errorf("cannot stat %s: %v\n", projDir, err))
	}
	if !info.IsDir() {
		panic(fmt.Errorf("%s is not a directory\n", projDir))
	}

	absDir, err := filepath.Abs(projDir)
	if err != nil {
		panic(fmt.Errorf("cannot resolve absolute path: %v\n", err))
	}

	/* open the log file */
	lfile, err := os.Create(logFile)
	if err != nil {
		panic(fmt.Errorf("cannot create log file: %v\n", err))
	}
	defer lfile.Close()

	/* execute the go test command */
	cmd := exec.Command("go", "test", "-v", "-bench=.", "-benchmem", "-run=^$", "./...")
	cmd.Dir = absDir
	cmd.Stdout = lfile
	cmd.Stderr = lfile

	if err := cmd.Run(); err != nil {
		panic(fmt.Errorf("benchmark failed: %v\n", err))
	}
}