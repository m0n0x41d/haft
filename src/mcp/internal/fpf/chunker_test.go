package fpf

import (
	"strings"
	"testing"
)

func TestChunkMarkdown_BasicSections(t *testing.T) {
	input := `# Title

Some intro text.

## Section One

Content of section one.
More content here.

## Section Two

Content of section two.

### Subsection 2.1

Nested content.
`
	chunks, err := ChunkMarkdown(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(chunks) != 4 {
		t.Fatalf("expected 4 chunks, got %d", len(chunks))
	}

	tests := []struct {
		heading string
		level   int
		bodyContains string
	}{
		{"Title", 1, "Some intro text."},
		{"Section One", 2, "Content of section one."},
		{"Section Two", 2, "Content of section two."},
		{"Subsection 2.1", 3, "Nested content."},
	}

	for i, tt := range tests {
		if chunks[i].Heading != tt.heading {
			t.Errorf("chunk[%d].Heading = %q, want %q", i, chunks[i].Heading, tt.heading)
		}
		if chunks[i].Level != tt.level {
			t.Errorf("chunk[%d].Level = %d, want %d", i, chunks[i].Level, tt.level)
		}
		if !strings.Contains(chunks[i].Body, tt.bodyContains) {
			t.Errorf("chunk[%d].Body should contain %q, got %q", i, tt.bodyContains, chunks[i].Body)
		}
	}
}

func TestChunkMarkdown_EmptyBodiesSkipped(t *testing.T) {
	input := `## Empty Section
## Has Content

Real content here.
`
	chunks, err := ChunkMarkdown(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk (empty body skipped), got %d", len(chunks))
	}

	if chunks[0].Heading != "Has Content" {
		t.Errorf("expected heading 'Has Content', got %q", chunks[0].Heading)
	}
}

func TestChunkMarkdown_ContentBeforeFirstHeadingSkipped(t *testing.T) {
	input := `This content has no heading above it.

## First Real Section

Section body.
`
	chunks, err := ChunkMarkdown(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0].Heading != "First Real Section" {
		t.Errorf("expected 'First Real Section', got %q", chunks[0].Heading)
	}
}

func TestChunkMarkdown_SixLevelHeadings(t *testing.T) {
	input := `###### Deep heading

Deep content.
`
	chunks, err := ChunkMarkdown(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0].Level != 6 {
		t.Errorf("expected level 6, got %d", chunks[0].Level)
	}
}

func TestChunkMarkdown_NotAHeading(t *testing.T) {
	input := `## Real heading

##NotAHeading because no space.
#######TooManyHashes
`
	chunks, err := ChunkMarkdown(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	// The non-headings should be in the body
	if !strings.Contains(chunks[0].Body, "##NotAHeading") {
		t.Error("non-heading line should be in body")
	}
}

func TestParseMarkdownHeading(t *testing.T) {
	tests := []struct {
		line    string
		wantOK  bool
		wantLvl int
		wantTxt string
	}{
		{"# Title", true, 1, "Title"},
		{"## Section", true, 2, "Section"},
		{"### Sub", true, 3, "Sub"},
		{"###### Deep", true, 6, "Deep"},
		{"####### TooDeep", false, 0, ""},
		{"##NoSpace", false, 0, ""},
		{"Not a heading", false, 0, ""},
		{"", false, 0, ""},
		{"  ## Indented", true, 2, "Indented"},
	}

	for _, tt := range tests {
		level, text, ok := parseMarkdownHeading(tt.line)
		if ok != tt.wantOK {
			t.Errorf("parseMarkdownHeading(%q): ok=%v, want %v", tt.line, ok, tt.wantOK)
		}
		if ok {
			if level != tt.wantLvl {
				t.Errorf("parseMarkdownHeading(%q): level=%d, want %d", tt.line, level, tt.wantLvl)
			}
			if text != tt.wantTxt {
				t.Errorf("parseMarkdownHeading(%q): text=%q, want %q", tt.line, text, tt.wantTxt)
			}
		}
	}
}

func TestChunkMarkdown_IDsAreSequential(t *testing.T) {
	input := `## A
Content A.
## B
Content B.
## C
Content C.
`
	chunks, err := ChunkMarkdown(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for i, c := range chunks {
		if c.ID != i {
			t.Errorf("chunk[%d].ID = %d, want %d", i, c.ID, i)
		}
	}
}
