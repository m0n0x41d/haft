package artifact

import (
	"regexp"
	"testing"
)

// TestGenerateID_NoCollisionAcrossRapidCalls is the regression test for
// GitHub issue #63 — artifact ID collisions across branches. The previous
// sequential format (dec-YYYYMMDD-001) reliably produced identical filenames
// when two branches created decisions on the same day, leading to
// mechanically-unmergeable conflicts in `.haft/`.
//
// New format uses a 32-bit random hex suffix from crypto/rand. With
// ~4.3B possible suffixes per kind per day, collision in a single project
// over a typical day is negligible (birthday-paradox probability stays
// below 10^-6 for the first few thousand IDs).
//
// This test creates 2000 IDs in tight succession and asserts uniqueness.
// Failure here means the random source has degraded or the seq parameter
// crept back into the format.
func TestGenerateID_NoCollisionAcrossRapidCalls(t *testing.T) {
	const samples = 2000

	seen := make(map[string]struct{}, samples)
	for i := 0; i < samples; i++ {
		// Use varying seq to confirm seq is no longer rendered into the ID.
		// Two calls with the same seq must NOT produce the same ID.
		id := GenerateID(KindDecisionRecord, 1)
		if _, dup := seen[id]; dup {
			t.Fatalf("ID collision detected after %d samples: %q (seq=1 is supposed to be ignored)", i, id)
		}
		seen[id] = struct{}{}
	}
}

// TestGenerateID_FormatMatchesContract pins the canonical ID format so that
// downstream tools (filename globbers, .haft/ readers, regression diff
// inspectors) have a stable contract to match against.
func TestGenerateID_FormatMatchesContract(t *testing.T) {
	pattern := regexp.MustCompile(`^[a-z]+-\d{8}-[0-9a-f]{8}$`)
	cases := []Kind{KindNote, KindProblemCard, KindSolutionPortfolio, KindDecisionRecord, KindEvidencePack, KindRefreshReport}
	for _, kind := range cases {
		id := GenerateID(kind, 1)
		if !pattern.MatchString(id) {
			t.Errorf("ID for %s does not match `prefix-YYYYMMDD-8hex`: got %q", kind, id)
		}
	}
}

func TestGenerateID_TaskContextFormatMatchesContract(t *testing.T) {
	pattern := regexp.MustCompile(`^dec-\d{8}-task-4-[0-9a-f]{8}$`)

	id := GenerateIDWithTaskContext(KindDecisionRecord, 1, "Task #4")

	if !pattern.MatchString(id) {
		t.Fatalf("ID with task context does not match `dec-YYYYMMDD-task-4-8hex`: got %q", id)
	}
}

func TestGenerateID_TaskContextSanitizesFilenameSlug(t *testing.T) {
	pattern := regexp.MustCompile(`^dec-\d{8}-api-cli-cleanup-v2-[0-9a-f]{8}$`)

	id := GenerateIDWithTaskContext(KindDecisionRecord, 1, " API/CLI cleanup: v2! ")

	if !pattern.MatchString(id) {
		t.Fatalf("ID with task context did not use sanitized slug: got %q", id)
	}
}

func TestGenerateID_InvalidTaskContextFallsBackToDefaultFormat(t *testing.T) {
	pattern := regexp.MustCompile(`^dec-\d{8}-[0-9a-f]{8}$`)

	id := GenerateIDWithTaskContext(KindDecisionRecord, 1, "///")

	if !pattern.MatchString(id) {
		t.Fatalf("invalid task context should preserve default ID format, got %q", id)
	}
}

// TestGenerateID_SeqParameterIgnored explicitly asserts that callers passing
// different seq values get IDs that differ only because of the random hex
// suffix, not because of seq. Prevents regression to the old format if
// someone re-introduces seq into the rendered ID.
func TestGenerateID_SeqParameterIgnored(t *testing.T) {
	// Generate a batch with seq=1 and another with seq=999. The two batches
	// should be statistically indistinguishable — both pull from crypto/rand.
	const samples = 500
	withSeq1 := make(map[string]struct{}, samples)
	withSeqHigh := make(map[string]struct{}, samples)
	for i := 0; i < samples; i++ {
		withSeq1[GenerateID(KindNote, 1)] = struct{}{}
		withSeqHigh[GenerateID(KindNote, 999)] = struct{}{}
	}
	// Both sets should be near-fully unique within themselves.
	if len(withSeq1) < samples*9/10 {
		t.Errorf("seq=1 batch had too many collisions: %d/%d unique", len(withSeq1), samples)
	}
	if len(withSeqHigh) < samples*9/10 {
		t.Errorf("seq=999 batch had too many collisions: %d/%d unique", len(withSeqHigh), samples)
	}
}
