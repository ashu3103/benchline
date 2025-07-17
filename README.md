# benchline
A benchmark analysis pipeline for Go projects. It discovers Go benchmark tests, executes them using the standard go test tooling, and extracts detailed memory usage and performance statistics.

## Key Features
- Auto-discovery of benchmark functions across directories and sub-packages.
- Captures memory, compute related metric along with the number of tests successfully performed.
- Convert the analysis into exportable file formats like JSON/CSV
