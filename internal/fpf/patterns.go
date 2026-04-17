package fpf

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

//go:embed patterns/*.md
var embeddedPatterns embed.FS

//go:embed fpf-routes.json
var embeddedRoutes []byte

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
var coreRe = regexp.MustCompile(`^\*\*Core:\*\* (.+)$`)
var validUntilRe = regexp.MustCompile(`^\*\*Valid-until:\*\* (\d{4}-\d{2}-\d{2})\b`)

// PatternFileMetadata captures file-level annotations parsed from the top of
// each pattern markdown file.
type PatternFileMetadata struct {
	Filename   string
	ValidUntil string // YYYY-MM-DD when this file's patterns require review
}

// LoadPatternFileMetadata reads each pattern file and extracts its file-level
// metadata (currently just Valid-until). Used by the self-application test
// that fails when a pattern file's review date has passed.
func LoadPatternFileMetadata() ([]PatternFileMetadata, error) {
	entries, err := embeddedPatterns.ReadDir("patterns")
	if err != nil {
		return nil, err
	}
	var out []PatternFileMetadata
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := embeddedPatterns.ReadFile("patterns/" + e.Name())
		if err != nil {
			return nil, err
		}
		meta := PatternFileMetadata{Filename: e.Name()}
		// Only scan the first 20 lines for file-level annotations.
		lines := strings.Split(string(data), "\n")
		scanLimit := min(len(lines), 20)
		for i := 0; i < scanLimit; i++ {
			if m := validUntilRe.FindStringSubmatch(lines[i]); m != nil {
				meta.ValidUntil = m[1]
				break
			}
		}
		out = append(out, meta)
	}
	return out, nil
}

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

		if m := coreRe.FindStringSubmatch(line); m != nil {
			current.corePhases = parseCorePhases(m[1])
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
	patternID  string
	heading    string
	trigger    string
	specRef    string
	corePhases []string
	body       []string
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

// parseCorePhases parses a **Core:** marker value. `true` means the pattern is
// core for the phase of the file it lives in (inferred from filename). A
// comma-separated list of phase names means it's core for those phases
// specifically (allowing cross-phase citation, e.g. CHR-01 core in both frame
// and characterize).
func parseCorePhases(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.EqualFold(raw, "false") {
		return nil
	}
	if strings.EqualFold(raw, "true") {
		return []string{"__self__"}
	}
	var phases []string
	for p := range strings.SplitSeq(raw, ",") {
		p = strings.ToLower(strings.TrimSpace(p))
		if p != "" {
			phases = append(phases, p)
		}
	}
	return phases
}

// --- Phase hints: compact pattern citations injected into tool responses -----

type phaseHintCache struct {
	once  sync.Once
	hints map[string]string
	err   error
}

var phaseHintCacheInst phaseHintCache

// PhaseHint returns a compact nudge for the given reasoning phase, listing the
// core pattern IDs for that phase with their headings, plus retrieval hints.
// Returns empty string for non-reasoning or unknown phases. Descriptions are
// derived from the embedded pattern files — renaming a pattern heading
// propagates automatically.
func PhaseHint(phase string) string {
	phase = strings.ToLower(strings.TrimSpace(phase))
	phaseHintCacheInst.once.Do(func() {
		phaseHintCacheInst.hints, phaseHintCacheInst.err = buildPhaseHints()
	})
	if phaseHintCacheInst.err != nil {
		// Fall back to empty rather than surface build errors in every response.
		return ""
	}
	return phaseHintCacheInst.hints[phase]
}

func buildPhaseHints() (map[string]string, error) {
	// Parse embedded pattern files, keeping track of which phase each pattern
	// is declared core for.
	type entry struct {
		id      string
		heading string
		file    string
	}
	byPhase := map[string][]entry{}

	entries, err := embeddedPatterns.ReadDir("patterns")
	if err != nil {
		return nil, fmt.Errorf("read embedded patterns: %w", err)
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		fileName := strings.TrimSuffix(e.Name(), ".md")
		data, err := embeddedPatterns.ReadFile("patterns/" + e.Name())
		if err != nil {
			return nil, fmt.Errorf("read pattern %s: %w", e.Name(), err)
		}
		sections := collectPatternSections(string(data))
		for _, s := range sections {
			if len(s.corePhases) == 0 {
				continue
			}
			for _, p := range s.corePhases {
				phase := p
				if phase == "__self__" {
					phase = fileName
				}
				byPhase[phase] = append(byPhase[phase], entry{
					id:      s.patternID,
					heading: s.heading,
					file:    fileName,
				})
			}
		}
	}

	// Keywords for the retrieval example query per phase are derived from the
	// first few matchers of the corresponding phase-* route in fpf-routes.json.
	// This keeps the hint aligned with actual routing semantics — rename a
	// matcher and the hint follows.
	phaseQueryKeywords := buildPhaseQueryKeywords()

	result := map[string]string{}
	for phase, entries := range byPhase {
		// Stable order — by file, then by ID
		sort.SliceStable(entries, func(i, j int) bool {
			if entries[i].file != entries[j].file {
				return fileOrder(entries[i].file) < fileOrder(entries[j].file)
			}
			return entries[i].id < entries[j].id
		})

		var pairs []string
		for _, e := range entries {
			pairs = append(pairs, fmt.Sprintf("%s (%s)", e.id, shortDescription(e.heading)))
		}

		first := ""
		if len(entries) > 0 {
			first = entries[0].id
		}

		kw := phaseQueryKeywords[phase]
		if kw == "" {
			kw = phase
		}

		var b strings.Builder
		b.WriteString("\n── FPF patterns for ")
		b.WriteString(phaseDisplayName(phase))
		b.WriteString(" ──\n")
		b.WriteString(strings.Join(pairs, ", "))
		b.WriteString("\n")
		if first != "" {
			b.WriteString(fmt.Sprintf("Retrieve: haft_query(action=\"fpf\", query=\"%s\") | Full phase: haft_query(action=\"fpf\", query=\"%s\")\n", first, kw))
		}
		result[phase] = b.String()
	}
	return result, nil
}

// collectPatternSections parses a pattern file string and returns one entry
// per ## section (similar to parsePatternFile but without SpecChunk conversion).
func collectPatternSections(content string) []*patternSection {
	lines := strings.Split(content, "\n")
	var sections []*patternSection
	var current *patternSection
	for _, line := range lines {
		if m := patternHeadingRe.FindStringSubmatch(line); m != nil {
			if current != nil {
				sections = append(sections, current)
			}
			current = &patternSection{patternID: m[1], heading: m[2]}
			continue
		}
		if current == nil {
			continue
		}
		if m := coreRe.FindStringSubmatch(line); m != nil {
			current.corePhases = parseCorePhases(m[1])
			continue
		}
		if m := triggerRe.FindStringSubmatch(line); m != nil {
			current.trigger = m[1]
			continue
		}
	}
	if current != nil {
		sections = append(sections, current)
	}
	return sections
}

// shortDescription extracts a terse label from a pattern heading — the part
// before the first colon, parenthesis, or slash, lowercased.
func shortDescription(heading string) string {
	h := heading
	for _, sep := range []string{":", "("} {
		if idx := strings.Index(h, sep); idx > 0 {
			h = h[:idx]
		}
	}
	h = strings.TrimSpace(h)
	// Keep it short — first 3-4 words
	words := strings.Fields(h)
	if len(words) > 4 {
		words = words[:4]
	}
	return strings.ToLower(strings.Join(words, " "))
}

func phaseDisplayName(phase string) string {
	switch phase {
	case "frame":
		return "framing"
	case "characterize":
		return "characterization"
	case "explore":
		return "exploration"
	case "compare":
		return "comparison"
	case "decide":
		return "deciding"
	case "verify":
		return "verification"
	default:
		return phase
	}
}

// buildPhaseQueryKeywords derives per-phase example query keywords from the
// embedded routes file. For each route with id "phase-<name>", take the first
// 4 matchers and join them. If the route or matchers are missing, the phase
// falls back to its own name — the hint still works, just less descriptive.
func buildPhaseQueryKeywords() map[string]string {
	result := map[string]string{
		"frame":        "frame",
		"characterize": "characterize",
		"explore":      "explore",
		"compare":      "compare",
		"decide":       "decide",
		"verify":       "verify",
	}
	routes, err := ParseRoutes(strings.NewReader(string(embeddedRoutes)))
	if err != nil {
		return result
	}
	const take = 4
	for _, r := range routes {
		if !strings.HasPrefix(r.ID, "phase-") {
			continue
		}
		phase := strings.TrimPrefix(r.ID, "phase-")
		if _, ok := result[phase]; !ok {
			continue
		}
		n := min(len(r.Matchers), take)
		if n > 0 {
			result[phase] = strings.Join(r.Matchers[:n], " ")
		}
	}
	return result
}

func fileOrder(name string) int {
	switch name {
	case "frame":
		return 0
	case "characterize":
		return 1
	case "explore":
		return 2
	case "compare":
		return 3
	case "decide":
		return 4
	case "verify":
		return 5
	case "cross-cutting":
		return 6
	default:
		return 99
	}
}
