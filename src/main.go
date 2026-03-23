package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github/ashu3103/benchline/src/analyzer/sroa"
	"github/ashu3103/benchline/src/loader"
	"github/ashu3103/benchline/src/parser"
	"github/ashu3103/benchline/src/runner"
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("%v", r)
		}
	}()
		
	var dir string
	fmt.Scanln(&dir)

	cfg := &loader.LoadConfig{
		Dir: dir,
		Patterns: []string{"./..."},
		BenchmarkOnly: true,
	}

	currentDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	currentDir = filepath.Dir(currentDir)
	currentDir = filepath.Join(currentDir, "logs")

	// ----- Load Packages ------
	pkgs, err := loader.LoadPackages(cfg)
	if err != nil {
		panic(err)
	}
	loadPkgLog := filepath.Join(currentDir, "loaded_package.log")
	loadLogFile, err := os.Create(loadPkgLog)
	if err != nil {
		panic(err)
	}
	loader.DumpLoadedPackages(loadLogFile, pkgs)
	loadLogFile.Close()

	// ----- Run Test ------
	rCfg := runner.NewRunConfig(dir, "", nil, nil, "")
	rCfg.AddGcFlag("-N", "")
	rCfg.AddGcFlag("-l", "")
	rCfg.AddGcFlag("-m", "")

	r := runner.NewRunner(rCfg)
	rResult := r.Test()

	if rResult.Err != nil {
		panic(rResult.Err)
	}

	// ---- Parse Escape Logs ------
	escapeMap := parser.ParseEscapesFromString(rResult.Output)

	escapeMapLog := filepath.Join(currentDir, "escape_map.log")
	escapeLogFile, err := os.Create(escapeMapLog)
	if err != nil {
		panic(err)
	}
	parser.DumpEscapeMap(escapeLogFile, escapeMap)
	escapeLogFile.Close()

	// ---- Create Def Use Chain ------
	for _, pkg := range pkgs {
		// find the escaped variable list
		canonicalPkgName := loader.CanonicalPkgName(pkg.Pkg.ID)
		fileMap := escapeMap[canonicalPkgName]

		for _, fn := range pkg.TargetFuncs {
			rel, err := filepath.Rel(dir, fn.FileName)
			if err != nil {
				// error
			}
			escapeVars := fileMap[rel]

			defUseChain := analyzer.CreateDefUseChains(fn.Decl, pkg.Info, pkg.Fset, escapeVars)
			if defUseChain == nil || len(defUseChain.Chains) == 0 {
				continue
			}

			pathFile := strings.ReplaceAll(strings.Split(rel, ".")[0], "/", "_")
			canonicalPkgName = strings.ReplaceAll(canonicalPkgName, "/", "_")
			defUseLog := filepath.Join(currentDir, fmt.Sprintf("%s_%s_%s_def_use.log", canonicalPkgName, pathFile, fn.Decl.Name.Name))
			defUseLogFile, err := os.Create(defUseLog)
			if err != nil {
				panic(err)
			}
			verdictLog := filepath.Join(currentDir, fmt.Sprintf("%s_%s_%s_verdict.log", canonicalPkgName, pathFile, fn.Decl.Name.Name))
			verdictLogFile, err := os.Create(verdictLog)
			if err != nil {
				panic(err)
			}

			analyzer.DumpDefUseChain(defUseLogFile, defUseChain)
			verdicts := analyzer.CheckViolation(defUseChain)
			analyzer.DumpVerdicts(verdictLogFile, verdicts)

			defUseLogFile.Close()
			verdictLogFile.Close()
		}
	}
}