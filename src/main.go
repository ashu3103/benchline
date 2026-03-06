package main

import (
	"log"
	"fmt"

	"github/ashu3103/benchline/src/injector"
	// "github/ashu3103/benchline/src/finder"
	// "github/ashu3103/benchline/src/runner"
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("%v", r)
		}
	}()
		
	var dir string
	fmt.Scanln(&dir)

	// /* find the test files */
	// files := finder.FindTestFiles(dir)
	
	/* inject the benchmark in dir */
	injector.InjectBenchmark(dir)
	// for _, f := range files {
	// }

	/* run go test */
	// runner.RunBecnhmarkTests(dir, "out.log")
}