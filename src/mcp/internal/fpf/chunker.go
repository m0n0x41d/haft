package fpf

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// SpecChunk represents a section of the FPF spec.
type SpecChunk struct {
	ID      int
	Heading string // full heading text, e.g. "3.1. WLNK — Weak Link"
	Level   int    // heading level (1-6)
	Body    string // content under this heading (without the heading line itself)
}

// ChunkMarkdown splits a markdown document into chunks by headings.
// Each chunk contains the heading and all content until the next heading
// of the same or higher level.
func ChunkMarkdown(r io.Reader) ([]SpecChunk, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024) // 10MB buffer for huge lines

	var chunks []SpecChunk
	var currentHeading string
	var currentLevel int
	var currentBody strings.Builder
	id := 0

	flush := func() {
		body := strings.TrimSpace(currentBody.String())
		if currentHeading != "" && body != "" {
			chunks = append(chunks, SpecChunk{
				ID:      id,
				Heading: currentHeading,
				Level:   currentLevel,
				Body:    body,
			})
			id++
		}
		currentBody.Reset()
	}

	for scanner.Scan() {
		line := scanner.Text()
		if level, heading, ok := parseMarkdownHeading(line); ok {
			flush()
			currentHeading = heading
			currentLevel = level
		} else {
			if currentHeading != "" {
				currentBody.WriteString(line)
				currentBody.WriteString("\n")
			}
		}
	}
	flush()

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning markdown: %w", err)
	}
	return chunks, nil
}

func parseMarkdownHeading(line string) (level int, text string, ok bool) {
	trimmed := strings.TrimLeft(line, " ")
	if !strings.HasPrefix(trimmed, "#") {
		return 0, "", false
	}
	i := 0
	for i < len(trimmed) && trimmed[i] == '#' {
		i++
	}
	if i > 6 || i >= len(trimmed) || trimmed[i] != ' ' {
		return 0, "", false
	}
	return i, strings.TrimSpace(trimmed[i+1:]), true
}
