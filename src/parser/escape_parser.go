package parser

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
)

type PkgFileEscapeMap map[string]map[string][]EscapeInfo

type EscapeInfo struct {
	Line     int
	Col      int
	Filename string
	VarName  string
}

func FilterEscapedVariables(items []EscapeInfo, lo, hi int) []EscapeInfo {
	// find first index where Value >= lo
    left := sort.Search(len(items), func(i int) bool {
        return items[i].Line >= lo
    })

    // find first index where Value > hi
    right := sort.Search(len(items), func(i int) bool {
        return items[i].Line > hi
    })

	return items[left:right]
}

// ParseEscapesFromString parses the output of `go test -gcflags="-m"` from
// an in-memory string.
func ParseEscapesFromString(output string) PkgFileEscapeMap {
	return parseEscapes(strings.NewReader(output))
}
 
// ParseEscapesFromFile parses the output of `go test -gcflags="-m"` from a
// file, reading it in chunks of bufSize bytes at a time. Lines that span a
// chunk boundary are carried over so no diagnostic is lost.
func ParseEscapesFromFile(path string, bufSize int) (PkgFileEscapeMap, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
 
	return parseEscapes(NewChunkedReader(f, bufSize)), nil
}

// add item to PkgFileInfoMap
func (e PkgFileEscapeMap) add(pkg, file string, info EscapeInfo) {
	if e[pkg] == nil {
		e[pkg] = make(map[string][]EscapeInfo)
	}
	e[pkg][file] = append(e[pkg][file], info)
}

// parses the output of escape logs
func parseEscapes(r io.Reader) PkgFileEscapeMap {
	result := make(PkgFileEscapeMap)
	currentPkg := ""

	scanner := bufio.NewScanner(r)
	// iterate line by line
	for scanner.Scan() {
		line := scanner.Text()

		switch {
		case strings.HasPrefix(line, "# "):
			currentPkg = parsePackageHeader(line)

			// ignore test executables
			if strings.HasSuffix(currentPkg, ".test") {
				currentPkg = ""
			}
		case currentPkg != "" && (strings.Contains(line, "escapes to heap") || strings.Contains(line, "moved to heap:")):
			file, info, err := parseEscapeLine(line)
			if err != nil || isLiteralEscape(info.VarName) || isFieldEscape(info.VarName) {
				// TODO: log error
				continue
			}

			if strings.HasSuffix(file, "_test.go") {
				continue
			}

			result.add(currentPkg, file, info)
		}
	}

	return result
}

// parsePackageHeader extracts the package path from a "# <pkg> [optional]" line.
func parsePackageHeader(line string) string {
	pkg := strings.TrimPrefix(line, "# ")
	if idx := strings.IndexByte(pkg, '['); idx != -1 {
		pkg = strings.TrimSpace(pkg[:idx])
	}
	return pkg
}

// parseEscapeLine parses a diagnostic line of the form:
//
//	./main.go:10:14: c escapes to heap
//	add/add_test.go:7:14: *c escapes to heap
//
// Returns (file, EscapeInfo, true) on success.
func parseEscapeLine(line string) (string, EscapeInfo, error) {
	// format:  <file>:<line>:<col>: <message>
	colonSpace := strings.Index(line, ": ")
	if colonSpace == -1 {
		return "", EscapeInfo{}, fmt.Errorf("string: \"%s\" not of the format <file>:<line>:<col>: <message>", line)
	}
 
	location := line[:colonSpace]  // "./main.go:10:14"
	message := line[colonSpace+2:] // "c escapes to heap"
 
	locParts := strings.SplitN(location, ":", 3)
	if len(locParts) != 3 {
		return "", EscapeInfo{}, fmt.Errorf("location: \"%s\" is not of the form <file>:<line>:<col>", location)
	}
 
	var varName string
    switch {
    case strings.Contains(message, " escapes to heap"):
        varName, _, _ = strings.Cut(message, " escapes to heap")
    case strings.HasPrefix(message, "moved to heap: "):
        _, varName, _ = strings.Cut(message, "moved to heap: ")
    default:
        return "", EscapeInfo{}, nil
    }

	varName = strings.TrimSpace(varName)
    if varName == "" {
        return "", EscapeInfo{}, nil
    }

	l, err := strconv.Atoi(locParts[1])
	if err != nil {
		return "", EscapeInfo{}, fmt.Errorf("parsed line number: %s is not an integer", locParts[1])
	}

	c, err := strconv.Atoi(locParts[2])
	if err != nil {
		return "", EscapeInfo{}, fmt.Errorf("parsed column number: %s is not an integer", locParts[2])
	}

	return locParts[0], EscapeInfo{
		Line:     l,
		Col:      c,
		VarName: varName,
		Filename: locParts[0],
	}, nil
}

func DumpEscapeMap(w io.Writer, escapeMap PkgFileEscapeMap) {
    if escapeMap == nil {
        fmt.Fprintf(w, "no packages found\n")
        return
    }

    pkgs := sortedKeys(escapeMap)

    for pi, pkg := range pkgs {
        // Package header
        files := escapeMap[pkg]

        total := 0
		for _, infos := range files {
			total += len(infos)
		}

        fmt.Fprintf(w, "┌─ package %s  (%d escape(s))\n", pkg, total)

        if files == nil {
            fmt.Fprintf(w, "│   no escaped variables found\n")
        } else {
            // Pre-compute the widest "file:line:col" in this package
            // so every variable name starts at the same column.
            maxLocLen := 0
            for file, infos := range files {
                for _, info := range infos {
					loc := fmt.Sprintf("%s:%d:%d", file, info.Line, info.Col)
                    if l := len(loc); l > maxLocLen {
                        maxLocLen = l
                    }
                }
            }

            fileNames := sortedKeys(files)
            for i, file := range fileNames {
				hasNextFile := false
                infos := files[file]

				if i == len(fileNames) - 1 {
					hasNextFile = false
					fmt.Fprintf(w, "│   └ %s  (%d escape(s))\n", file, len(infos))
				} else {
					hasNextFile = true
					fmt.Fprintf(w, "│   ├ %s  (%d escape(s))\n", file, len(infos))
				}

                for j, info := range infos {
                    loc := fmt.Sprintf("%s:%d:%d", file, info.Line, info.Col)
                    connector := "├"
                    if j == len(infos)-1 {
                        connector = "└"
                    }
					if !hasNextFile {
						fmt.Fprintf(w, "│      %s [%*d]  %-*s  →  \"%s\"\n",
							connector,
							len(fmt.Sprint(len(infos))), j+1, // index width matches total digits
							maxLocLen, loc,
							info.VarName,
						)
					} else {
						fmt.Fprintf(w, "│   │  %s [%*d]  %-*s  →  \"%s\"\n",
							connector,
							len(fmt.Sprint(len(infos))), j+1, // index width matches total digits
							maxLocLen, loc,
							info.VarName,
						)
					}
                }
            }
        }

        // Separator between packages
        if pi < len(pkgs)-1 {
            fmt.Fprintf(w, "│\n")
        }
		fmt.Fprintf(w, "└───────\n")
    }
}

// sortedKeys returns the keys of any map[string]V in sorted order.
func sortedKeys[V any](m map[string]V) []string {
    keys := make([]string, 0, len(m))
    for k := range m {
        keys = append(keys, k)
    }
    sort.Strings(keys)
    return keys
}

// isLiteralEscape reports whether a variable field from a `-gcflags="-m"`
// diagnostic represents a literal, anonymous allocation, or transient
// expression rather than a named variable, and should be filtered out.
func isLiteralEscape(v string) bool {
	if v == "" {
		return false
	}

	switch v[0] {
	case '"', '`', '\'': // string or rune literal
		return true
	case '&': // address-of:  &x, &T{}
		return true
	case '.': // variadic:  ... argument
		return true
	case '(': // type conversion:  (T)(expr)
		return true
	case '~': // compiler-generated temporary:  ~r0, ~r1
		return true
	}
 
	// Composite literals end with "{" in the compiler's output.
	if strings.HasSuffix(v, "{") {
		return true
	}
 
	// Explicit keyword forms.
	for _, prefix := range []string{
		"new(",          // new(T)
		"make(",         // make([]T, n), make(chan T), make(map[K]V)
		"func literal",  // anonymous function value
	} {
		if strings.HasPrefix(v, prefix) {
			return true
		}
	}

	return false
}

func isFieldEscape(v string) bool {
	if strings.Contains(v, ".") { return true }
	return false
}