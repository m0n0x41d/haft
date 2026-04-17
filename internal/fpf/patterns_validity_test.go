package fpf

import (
	"testing"
	"time"
)

// TestPatternFilesNotPastValidUntil applies VER-08 (Valid-until as Lifecycle
// Trigger) to the FPF pattern set itself. Each pattern file under patterns/
// declares a **Valid-until:** YYYY-MM-DD date in its header. When the date
// passes, this test fails — forcing review of the pattern file's content
// (attribution, Core markers, micro-content currency) before the date is
// extended in source.
//
// This closes a self-application gap: VER-02 / VER-08 prescribe valid_until
// for evidence and decisions; without applying the same rule to the patterns
// that prescribe it, the project violates its own X-DESIGNRUN distinction —
// the pattern set becomes a permanent "design-time" artifact masquerading as
// always-current truth.
//
// Refresh procedure when this test fails:
//  1. Open the failing pattern file (e.g. patterns/frame.md).
//  2. Review attribution against current FPF spec + slideument sources.
//  3. Verify Core markers still match desired hint behavior.
//  4. Add new patterns / refresh existing ones if FPF/seminar work has shipped.
//  5. Bump **Valid-until:** to a new date (typically 6 months out).
func TestPatternFilesNotPastValidUntil(t *testing.T) {
	metas, err := LoadPatternFileMetadata()
	if err != nil {
		t.Fatalf("load pattern file metadata: %v", err)
	}
	if len(metas) == 0 {
		t.Fatal("no pattern files found; expected at least one *.md under patterns/")
	}

	now := time.Now().UTC()
	missingValidUntil := make([]string, 0)
	pastValidUntil := make([]string, 0)

	for _, m := range metas {
		if m.ValidUntil == "" {
			missingValidUntil = append(missingValidUntil, m.Filename)
			continue
		}
		date, err := time.Parse("2006-01-02", m.ValidUntil)
		if err != nil {
			t.Errorf("%s: malformed Valid-until %q (want YYYY-MM-DD): %v", m.Filename, m.ValidUntil, err)
			continue
		}
		// End-of-day UTC: a file dated 2026-10-18 is valid through that day.
		expiry := date.Add(24 * time.Hour).Add(-time.Second)
		if now.After(expiry) {
			pastValidUntil = append(pastValidUntil, m.Filename+" ("+m.ValidUntil+")")
		}
	}

	if len(missingValidUntil) > 0 {
		t.Errorf("pattern files missing **Valid-until:** declaration:\n  %v\nAdd `**Valid-until:** YYYY-MM-DD` near the file header.", missingValidUntil)
	}
	if len(pastValidUntil) > 0 {
		t.Errorf("pattern files past their valid_until date:\n  %v\nReview content (attribution, Core markers, currency) and bump the date in source.", pastValidUntil)
	}
}
