# benchline
A benchmark analysis pipeline for Go projects. It discovers Go benchmark tests, executes them using the standard go test tooling, and extracts detailed memory usage and performance statistics.

### Aim

The aim is to automate the process of executing the traditionl go test tools over a go directory and extract the relevent metrics of interest out of the test results (typically a log file).

**Key Features**
- Auto-discovery of go test files across directories and sub-directories
- For each test file found, transform any unit test (`TestXxx`) into a benchmark test (`BenchmarkXxx`) by injecting go code into the file.
- Convert the test results into exportable file formats like JSON

**Metrics of Interest**

At present, we're primarily focused on the following metrics:-
- `bytes of heap allocated`
- `total number of heap allocations`

These measurements are particularly relevant for analyzing and benchmarking behaviors related to escape analysis in experiments and studies.

### Architecture

Each target go project directory has to go through 4 stages of this pipeline, currently the pipeline is sequential, however it can be upgraded to a parallel one ith some stalling.

#### Finder

The **finder** stage will take a go project directory, detect all the test files and return 
a list of paths to the test path

This will first check the standard test subdirectories like - ('test/' or 'tests/') and if no
such subdirectory is found, perform a DFS over the whole project directory to find test files
(typically the files ending with '_test.go')

Additionally we can exclude the directories which can contain static (large) data (think about 
media files)

#### Injector

This stage is the core of `benchline`, it scans the whole Go test file and try to inject benchmark test function 
for every unit test function found.

After injecting the code, it will check if the modified file is syntactically correct using a 
Go syntax checker binary (native), if syntax not correct, skip the file and no benchmark tests
skip the file.

#### Runner

The **runner** package is pretty simple, it will just take the benchmark codes and execute
the traditional `go test...` command over the project directory and output the result in a log
file which will then be passed to scrapper.

Command: `go test -bench=. -benchmem -run=^$ ./...`

The command is executed over the root go project dirctory

#### Extractor

There are two main responsibility of this stage is to -
- parse the log file generated from the runner stage and extract the relevent metrics (like bytes allocated and number of allocations)
- structure the extracted metrics and dump them into exportable file formats like JSON

### Usage

Suppose we have a pool of go projects (in a directory)

Run:
```
cd src/
./benchline.py </path/to/pool>
```

For now the tool will execute in default configuration (TODO: will make it configurable based on user inputs)

### Why `BenchmarkXxx`?

Simply because we want our metrics of interest and these metrics directly reflect whether values escape to the heap (higher allocations) or stay on the stack (zero allocations).

This will allow us to build an accurate and reliable benchmarking tool for escape analysis
