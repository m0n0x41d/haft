package fpf

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// LoadPatternChunks reads all .md files from the patterns directory and
// converts each ## section into a SpecChunk suitable for indexing alongside
// the FPF spec sections.
func LoadPatternChunks(patternsDir string) ([]SpecChunk, error) {
	entries, err := os.ReadDir(patternsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // no patterns directory = no patterns
		}
		return nil, err
	}

	var chunks []SpecChunk
	idCounter := 90000 // offset to avoid collision with spec chunk IDs

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(patternsDir, entry.Name()))
		if err != nil {
			continue
		}

		parsed := parsePatternFile(string(data), &idCounter)
		chunks = append(chunks, parsed...)
	}

	return chunks, nil
}

var patternHeadingRe = regexp.MustCompile(`^## ([A-Z]+-[A-Z0-9]+(?:-[A-Z0-9]+)*): (.+)$`)
var triggerRe = regexp.MustCompile(`^\*\*Trigger:\*\* (.+)$`)
var specRe = regexp.MustCompile(`^\*\*(?:Spec|Source):\*\* (.+)$`)

func parsePatternFile(content string, idCounter *int) []SpecChunk {
	lines := strings.Split(content, "\n")
	var chunks []SpecChunk
	var current *patternSection

	for _, line := range lines {
		if m := patternHeadingRe.FindStringSubmatch(line); m != nil {
			if current != nil {
				chunks = append(chunks, current.toChunk(idCounter))
			}
			current = &patternSection{
				patternID: m[1],
				heading:   m[2],
			}
			continue
		}

		if current == nil {
			continue
		}

		if m := triggerRe.FindStringSubmatch(line); m != nil {
			current.trigger = m[1]
			continue
		}

		if m := specRe.FindStringSubmatch(line); m != nil {
			current.specRef = m[1]
			continue
		}

		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "**") {
			current.body = append(current.body, trimmed)
		}
	}

	if current != nil {
		chunks = append(chunks, current.toChunk(idCounter))
	}

	return chunks
}

type patternSection struct {
	patternID string
	heading   string
	trigger   string
	specRef   string
	body      []string
}

func (p *patternSection) toChunk(idCounter *int) SpecChunk {
	*idCounter++
	bodyText := strings.Join(p.body, " ")

	// Extract keywords from trigger
	triggerWords := extractKeywords(p.trigger)

	return SpecChunk{
		ID:        *idCounter,
		Heading:   p.heading,
		Level:     2,
		Body:      bodyText,
		Summary:   p.trigger,
		PatternID: p.patternID,
		Keywords:  triggerWords,
		Queries:   []string{p.trigger},
	}
}

func extractKeywords(text string) []string {
	// Simple: split on common separators, keep meaningful words
	text = strings.ToLower(text)
	text = strings.NewReplacer(
		";", " ", ",", " ", "(", " ", ")", " ",
		".", " ", "?", " ", "!", " ",
	).Replace(text)

	words := strings.Fields(text)
	stopwords := map[string]bool{
		"a": true, "an": true, "the": true, "is": true, "are": true,
		"or": true, "and": true, "to": true, "for": true, "of": true,
		"in": true, "on": true, "at": true, "by": true, "with": true,
		"from": true, "that": true, "this": true, "it": true, "be": true,
		"as": true, "was": true, "not": true, "but": true, "if": true,
	}

	var keywords []string
	seen := map[string]bool{}
	for _, w := range words {
		if len(w) < 3 || stopwords[w] || seen[w] {
			continue
		}
		seen[w] = true
		keywords = append(keywords, w)
	}

	return keywords
}
