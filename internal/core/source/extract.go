package source

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/m0n0x41d/haft/internal/core/carrier"
)

// Mixed-case suffixes such as A.19.SelectorMechanism, C.2.2a and G.Core
// are source-owned identities and must not be lost to an all-uppercase grammar.
var patternStart = regexp.MustCompile(`^## ([A-Z][A-Za-z0-9]*(?:\.[A-Za-z0-9]+)+)[ \t]+[-–—][ \t]+(.+?)[ \t]*$`)
var patternEnd = regexp.MustCompile(`^### ([A-Z][A-Za-z0-9]*(?:\.[A-Za-z0-9]+)+):End[ \t]*$`)

func extract(f File) ([]indexedUnit, map[string][]carrier.Diagnostic) {
	units := []indexedUnit{}
	bad := map[string][]carrier.Diagnostic{}
	var current *indexedUnit
	startOffset := 0
	offset := 0
	lineNo := 0
	fenceChar := byte(0)
	fenceLength := 0
	closed := map[string]bool{}
	add := func(ref, code, msg string) { bad[ref] = append(bad[ref], diag(code, f.Path, msg+" ("+ref+")")) }
	for offset < len(f.Raw) {
		lineNo++
		end := bytes.IndexByte(f.Raw[offset:], '\n')
		if end < 0 {
			end = len(f.Raw)
		} else {
			end += offset + 1
		}
		line := string(bytes.TrimSuffix(bytes.TrimSuffix(f.Raw[offset:end], []byte{'\n'}), []byte{'\r'}))
		if char, n, tail, ok := fenceMarker(line); ok {
			if fenceChar == 0 {
				fenceChar = char
				fenceLength = n
			} else if char == fenceChar && n >= fenceLength && strings.TrimSpace(tail) == "" {
				fenceChar = 0
				fenceLength = 0
			}
			offset = end
			continue
		}
		if fenceChar != 0 {
			offset = end
			continue
		}
		if m := patternStart.FindStringSubmatch(line); m != nil {
			if current != nil {
				add(current.ref, "missing_pattern_end", "Pattern has no matching End before next pattern")
			}
			current = &indexedUnit{ref: m[1], kind: "pattern", title: m[2], publication: f.Path, start: lineNo}
			startOffset = offset
		} else if m := patternEnd.FindStringSubmatch(line); m != nil {
			ref := m[1]
			if current == nil {
				code := "unmatched_pattern_end"
				if closed[ref] {
					code = "duplicate_pattern_end"
				}
				add(ref, code, "End does not close one open pattern")
			} else if current.ref != ref {
				add(current.ref, "mismatched_pattern_end", "Pattern encountered another identity's End")
				add(ref, "unmatched_pattern_end", "End does not match the open identity")
				current = nil
			} else {
				current.end = lineNo
				current.raw = f.Raw[startOffset:end]
				units = append(units, *current)
				closed[ref] = true
				current = nil
			}
		}
		offset = end
	}
	if current != nil {
		add(current.ref, "missing_pattern_end", "Pattern has no matching End before end of publication")
	}
	return units, bad
}

// CommonMark-style fenced examples may be indented by up to three spaces.
func fenceMarker(line string) (byte, int, string, bool) {
	n := 0
	for n < len(line) && line[n] == ' ' {
		n++
	}
	if n > 3 || n == len(line) {
		return 0, 0, "", false
	}
	line = line[n:]
	char := line[0]
	if char != '`' && char != '~' {
		return 0, 0, "", false
	}
	n = 0
	for n < len(line) && line[n] == char {
		n++
	}
	if n < 3 {
		return 0, 0, "", false
	}
	if char == '`' && strings.ContainsRune(line[n:], '`') {
		return 0, 0, "", false
	}
	return char, n, line[n:], true
}
