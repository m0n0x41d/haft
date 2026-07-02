package fpf

import (
	"strings"
	"testing"
)

func TestParsePatternFilePreservesSourceRefInChunk(t *testing.T) {
	idCounter := PatternChunkIDBase
	chunks := parsePatternFile(strings.Join([]string{
		"## CHR-10: Boundary Norm Square (L / A / D / E)",
		"**Trigger:** Reviewing a boundary statement.",
		"**Source:** Levenchuk FPF A.6.B Boundary Norm Square, adapted for haft",
		"**Core:** characterize",
		"",
		"Decompose a mixed boundary formulation.",
	}, "\n"), &idCounter)

	if len(chunks) != 1 {
		t.Fatalf("chunks = %#v", chunks)
	}
	if !strings.Contains(chunks[0].Body, "Source: Levenchuk FPF A.6.B") {
		t.Fatalf("body did not preserve source ref: %q", chunks[0].Body)
	}
	if !patternUseStringsContain(chunks[0].Queries, "Levenchuk FPF A.6.B Boundary Norm Square, adapted for haft") {
		t.Fatalf("queries did not preserve source ref: %#v", chunks[0].Queries)
	}
}
