package parser

import (
	"bufio"
	"os"
	"fmt"
	"io"
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
		case currentPkg != "" && strings.Contains(line, "escapes to heap"):
			file, info, err := parseEscapeLine(line)
			if err != nil {
				// TODO: log error
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
 
	varName, _, found := strings.Cut(message, " escapes to heap")
	if !found {
		return "", EscapeInfo{}, fmt.Errorf("\"escapes to heap\" not found in message: %s", message)
	}

	l, err := strconv.Atoi(locParts[1])
	if err != nil {
		return "", EscapeInfo{}, fmt.Errorf("parsed line number: %s is not an integer", locParts[1])
	}

	c, err := strconv.Atoi(locParts[1])
	if err != nil {
		return "", EscapeInfo{}, fmt.Errorf("parsed line number: %s is not an integer", locParts[2])
	}

	return locParts[0], EscapeInfo{
		Line:     l,
		Col:      c,
		VarName: strings.TrimSpace(varName),
		Filename: locParts[0],
	}, nil
}