package overseer

import "testing"

func TestIngestReviewResultNormalizesContradictoryVerdict(t *testing.T) {
	packet, err := BuildPacket(BuildInput{
		Producer: DefaultProducer("test"),
		Subject: Subject{
			Kind:     "commit",
			Ref:      "HEAD",
			SHA:      "abc123",
			DiffHash: "sha256:diff",
		},
		RepoState: RepoState{GitRoot: ".", Branch: "main"},
		ChangedFiles: []ChangedFile{{
			Path:   "internal/cli/init.go",
			Status: "modified",
		}},
		Budget: DefaultContextBudget(),
	})
	if err != nil {
		t.Fatalf("BuildPacket returned error: %v", err)
	}

	stored := StoredRun{
		Packet: packet,
		Run:    NewDeterministicReviewRun(packet, "2026-06-09T00:00:00Z"),
	}

	stored, err = IngestReviewResult(stored, ReviewResultInput{
		Verdict:  "findings_recorded",
		Findings: []ReviewFinding{},
	}, "2026-06-09T01:00:00Z")
	if err != nil {
		t.Fatalf("IngestReviewResult returned error: %v", err)
	}
	if stored.Run.Verdict != "reviewed_no_findings" {
		t.Fatalf("verdict = %q, want reviewed_no_findings", stored.Run.Verdict)
	}
}
