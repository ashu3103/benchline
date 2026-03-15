package runner

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// RunConfig holds the configuration for a Go toolchain command.
type RunConfig struct {
	dir     string
	file    string
	gcFlags map[string]string
	args    map[string]string
	pattern string
}

// NewRunConfig creates a new RunConfig.
// dir is required and resolved to an absolute path; returns nil on failure.
// Nil flags/args are initialised to empty maps. Empty pattern defaults to "./...".
func NewRunConfig(dir, outFile string, flags, args map[string]string, pattern string) *RunConfig {
	if dir == "" {
		return nil
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil
	}

	if flags == nil {
		flags = make(map[string]string)
	}
	if args == nil {
		args = make(map[string]string)
	}
	if pattern == "" {
		pattern = "./..."
	}

	return &RunConfig{
		dir:     absDir,
		file:    outFile,
		gcFlags: flags,
		args:    args,
		pattern: pattern,
	}
}

// AddArg adds or overwrites a command-line argument (e.g. "v" → "-v").
func (r *RunConfig) AddArg(key, value string) {
	r.args[key] = value
}

// RemoveArg removes a command-line argument by key.
func (r *RunConfig) RemoveArg(key string) {
	delete(r.args, key)
}

// AddGcFlag adds or overwrites a gcflag (e.g. "-N" → "-gcflags=-N").
func (r *RunConfig) AddGcFlag(key, value string) {
	r.gcFlags[key] = value
}

// RemoveGcFlag removes a gcflag by key.
func (r *RunConfig) RemoveGcFlag(key string) {
	delete(r.gcFlags, key)
}

// gcFlagsArg serialises gcFlags into a single -gcflags="..." argument,
// or returns "" when there are no flags.
func (r *RunConfig) gcFlagsArg() string {
	if len(r.gcFlags) == 0 {
		return ""
	}

	parts := make([]string, 0, len(r.gcFlags))
	for k, v := range r.gcFlags {
		if v == "" {
			parts = append(parts, k)
		} else {
			parts = append(parts, fmt.Sprintf("%s=%s", k, v))
		}
	}
	return fmt.Sprintf("-gcflags=%s", strings.Join(parts, " "))
}

// buildCommand assembles the full argument slice for the given sub-command.
func (r *RunConfig) buildCommand(subCmd string) []string {
	cmdArgs := []string{subCmd}

	if gcf := r.gcFlagsArg(); gcf != "" {
		cmdArgs = append(cmdArgs, gcf)
	}

	for k, v := range r.args {
		if v == "" {
			cmdArgs = append(cmdArgs, fmt.Sprintf("-%s", k))
		} else {
			cmdArgs = append(cmdArgs, fmt.Sprintf("-%s=%s", k, v))
		}
	}

	cmdArgs = append(cmdArgs, r.pattern)
	return cmdArgs
}

// ----------------------------------------------------------------------------

// RunResult holds the output produced by a Runner command.
type RunResult struct {
	File   string // set when output was redirected to a file
	Output string // set when output was captured in memory
	Err    error
}

// ----------------------------------------------------------------------------

// Runner executes Go toolchain commands using a RunConfig.
type Runner struct {
	cfg *RunConfig
}

// NewRunner creates a Runner bound to the given RunConfig.
func NewRunner(cfg *RunConfig) *Runner {
	return &Runner{cfg: cfg}
}

// run is the shared execution helper for Test, Build, and Run.
func (r *Runner) run(subCmd string) *RunResult {
	result := &RunResult{}

	cmd := exec.Command("go", r.cfg.buildCommand(subCmd)...)
	cmd.Dir = r.cfg.dir

	if r.cfg.file != "" {
		f, err := os.Create(r.cfg.file)
		if err != nil {
			result.Err = err
			return result
		}
		defer f.Close()

		cmd.Stdout = f
		cmd.Stderr = f
		result.File = r.cfg.file
		result.Err = cmd.Run()
	} else {
		out, err := cmd.CombinedOutput()
		result.Output = string(out)
		result.Err = err
	}

	return result
}

// Test executes `go test [gcflags] [args] <pattern>`.
func (r *Runner) Test() *RunResult { return r.run("test") }

// Build executes `go build [gcflags] [args] <pattern>`.
func (r *Runner) Build() *RunResult { return r.run("build") }

// Run executes `go run [gcflags] [args] <pattern>`.
func (r *Runner) Run() *RunResult { return r.run("run") }