package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

// ---------------------------------------------------------------------------
// Ripgrep availability detection and command builder.
// Used by grep.go and glob.go as a fast backend with Go fallback.
// ---------------------------------------------------------------------------

// rgAvailable checks if ripgrep (rg) is on PATH. Detected once at startup.
var rgAvailable = sync.OnceValue(func() bool {
	_, err := exec.LookPath("rg")
	return err == nil
})

// rgMatch represents a single match from ripgrep JSON output.
type rgMatch struct {
	Path       string
	LineNumber int
	LineText   string
	Submatches []rgSubmatch
}

type rgSubmatch struct {
	Match string
	Start int
	End   int
}

// rgSearchParams configures a ripgrep search.
type rgSearchParams struct {
	Pattern    string
	Path       string // search root
	Glob       string // --glob filter
	FileType   string // --type filter
	OutputMode string // "content" | "files_with_matches" | "count"
	ContextA   int    // -A lines after
	ContextB   int    // -B lines before
	ContextC   int    // -C context lines
	Multiline  bool
	CaseInsens bool // -i
	HeadLimit  int  // max results
	Offset     int  // skip first N
}

// rgSearch executes ripgrep and returns structured matches.
func rgSearch(params rgSearchParams) ([]rgMatch, error) {
	args := buildRgArgs(params)
	cmd := exec.Command("rg", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		// Exit code 1 = no matches (not an error)
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		// Exit code 2 = actual error
		return nil, fmt.Errorf("rg: %s", strings.TrimSpace(stderr.String()))
	}

	return parseRgJSON(stdout.Bytes())
}

// rgFiles lists files matching a glob pattern using ripgrep.
func rgFiles(path, glob string) ([]string, error) {
	args := []string{"--files"}
	if glob != "" {
		args = append(args, "--glob", glob)
	}
	if path != "" {
		args = append(args, path)
	}

	cmd := exec.Command("rg", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil, nil // no files
		}
		return nil, fmt.Errorf("rg --files: %s", strings.TrimSpace(stderr.String()))
	}

	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil, nil
	}
	return lines, nil
}

func buildRgArgs(p rgSearchParams) []string {
	args := []string{"--json", "-n", "-H"}

	if p.CaseInsens {
		args = append(args, "-i")
	}
	if p.Multiline {
		args = append(args, "-U", "--multiline-dotall")
	}
	if p.Glob != "" {
		args = append(args, "--glob", p.Glob)
	}
	if p.FileType != "" {
		args = append(args, "--type", p.FileType)
	}
	if p.ContextA > 0 {
		args = append(args, "-A", fmt.Sprintf("%d", p.ContextA))
	}
	if p.ContextB > 0 {
		args = append(args, "-B", fmt.Sprintf("%d", p.ContextB))
	}
	if p.ContextC > 0 {
		args = append(args, "-C", fmt.Sprintf("%d", p.ContextC))
	}

	args = append(args, p.Pattern)

	if p.Path != "" {
		args = append(args, p.Path)
	}

	return args
}

// rgJSONMessage represents a single line of ripgrep JSON output.
type rgJSONMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type rgJSONMatch struct {
	Path struct {
		Text string `json:"text"`
	} `json:"path"`
	Lines struct {
		Text string `json:"text"`
	} `json:"lines"`
	LineNumber int `json:"line_number"`
	Submatches []struct {
		Match struct {
			Text string `json:"text"`
		} `json:"match"`
		Start int `json:"start"`
		End   int `json:"end"`
	} `json:"submatches"`
}

type rgJSONContext struct {
	Path struct {
		Text string `json:"text"`
	} `json:"path"`
	Lines struct {
		Text string `json:"text"`
	} `json:"lines"`
	LineNumber int `json:"line_number"`
}

func parseRgJSON(data []byte) ([]rgMatch, error) {
	var matches []rgMatch

	for _, line := range bytes.Split(data, []byte("\n")) {
		if len(line) == 0 {
			continue
		}

		var msg rgJSONMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			continue // skip malformed lines
		}

		switch msg.Type {
		case "match":
			var m rgJSONMatch
			if err := json.Unmarshal(msg.Data, &m); err != nil {
				continue
			}
			match := rgMatch{
				Path:       m.Path.Text,
				LineNumber: m.LineNumber,
				LineText:   strings.TrimRight(m.Lines.Text, "\n"),
			}
			for _, sm := range m.Submatches {
				match.Submatches = append(match.Submatches, rgSubmatch{
					Match: sm.Match.Text,
					Start: sm.Start,
					End:   sm.End,
				})
			}
			matches = append(matches, match)

		case "context":
			var c rgJSONContext
			if err := json.Unmarshal(msg.Data, &c); err != nil {
				continue
			}
			matches = append(matches, rgMatch{
				Path:       c.Path.Text,
				LineNumber: c.LineNumber,
				LineText:   strings.TrimRight(c.Lines.Text, "\n"),
			})
		}
	}

	return matches, nil
}

// formatRgResults formats matches according to output mode.
func formatRgResults(matches []rgMatch, mode string, headLimit, offset int) string {
	// Apply offset
	if offset > 0 {
		if offset >= len(matches) {
			return "No matches (offset beyond results)."
		}
		matches = matches[offset:]
	}

	// Apply limit
	if headLimit > 0 && headLimit < len(matches) {
		matches = matches[:headLimit]
	}

	switch mode {
	case "files_with_matches":
		seen := make(map[string]bool)
		var files []string
		for _, m := range matches {
			if !seen[m.Path] {
				seen[m.Path] = true
				files = append(files, m.Path)
			}
		}
		if len(files) == 0 {
			return "No matches found."
		}
		return strings.Join(files, "\n")

	case "count":
		counts := make(map[string]int)
		var order []string
		for _, m := range matches {
			if _, ok := counts[m.Path]; !ok {
				order = append(order, m.Path)
			}
			counts[m.Path]++
		}
		var lines []string
		for _, path := range order {
			lines = append(lines, fmt.Sprintf("%s:%d", path, counts[path]))
		}
		if len(lines) == 0 {
			return "No matches found."
		}
		return strings.Join(lines, "\n")

	default: // "content"
		if len(matches) == 0 {
			return "No matches found."
		}
		var lines []string
		for _, m := range matches {
			lines = append(lines, fmt.Sprintf("%s:%d:%s", m.Path, m.LineNumber, m.LineText))
		}
		return strings.Join(lines, "\n")
	}
}
