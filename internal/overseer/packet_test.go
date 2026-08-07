package overseer

import "testing"

func TestBuildPacket_StableHashIgnoresCreationTime(t *testing.T) {
	input := BuildInput{
		CreatedAt: "2026-06-09T00:00:00Z",
		Producer:  DefaultProducer("test"),
		Subject: Subject{
			Kind:      "commit",
			Ref:       "HEAD",
			SHA:       "abc123",
			ParentSHA: "def456",
			DiffHash:  "sha256:diff",
		},
		RepoState: RepoState{GitRoot: ".", Branch: "main"},
		ChangedFiles: []ChangedFile{{
			Path:      "internal/cli/init.go",
			Status:    "modified",
			Language:  "go",
			DiffStats: DiffStats{Added: 2, Deleted: 1},
			Governance: ChangedFileGovernance{
				ModuleState: "covered",
				AffectedDecisions: []ArtifactRef{{
					ID:    "dec-1",
					Title: "Init stays opt-in",
				}},
				AffectedInvariants: []InvariantRef{{
					ID:        "dec-1#invariant-1",
					Text:      "Plain haft init behavior must not change.",
					SourceRef: "dec-1",
				}},
			},
		}},
		Budget: DefaultContextBudget(),
	}

	first, err := BuildPacket(input)
	if err != nil {
		t.Fatalf("BuildPacket returned error: %v", err)
	}

	input.CreatedAt = "2026-06-09T00:01:00Z"
	second, err := BuildPacket(input)
	if err != nil {
		t.Fatalf("BuildPacket returned error: %v", err)
	}

	if first.PacketHash != second.PacketHash {
		t.Fatalf("packet hash changed with created_at: %s vs %s", first.PacketHash, second.PacketHash)
	}
	if first.PacketID != second.PacketID {
		t.Fatalf("packet ID changed with created_at: %s vs %s", first.PacketID, second.PacketID)
	}
}

func TestBuildPacket_RiskAndReviewRequestStayAdvisory(t *testing.T) {
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
			Governance: ChangedFileGovernance{
				ModuleState: "blind",
			},
		}},
		Budget: DefaultContextBudget(),
	})
	if err != nil {
		t.Fatalf("BuildPacket returned error: %v", err)
	}

	if packet.Risk.Level != "medium" {
		t.Fatalf("risk level = %q, want medium", packet.Risk.Level)
	}
	if packet.Risk.LLMReview != "eligible" {
		t.Fatalf("llm_review = %q, want eligible", packet.Risk.LLMReview)
	}
	if packet.ReviewRequest.Authority != "advisory_only" {
		t.Fatalf("authority = %q, want advisory_only", packet.ReviewRequest.Authority)
	}
	if contains(packet.ReviewRequest.Modes, "invariant_conformance") == false {
		t.Fatalf("review modes = %v, want invariant_conformance", packet.ReviewRequest.Modes)
	}

	finding := AdvisoryFindingDefaults(ReviewFinding{ID: "rf-1"})
	if finding.SupportPosture != "advisory_unverified" {
		t.Fatalf("support posture = %q, want advisory_unverified", finding.SupportPosture)
	}
	if finding.CountsForREff {
		t.Fatalf("advisory finding must not count for R_eff")
	}
}

func TestBuildPacket_BudgetTruncatesChangedFiles(t *testing.T) {
	files := []ChangedFile{
		{Path: "a.go", Status: "modified"},
		{Path: "b.go", Status: "modified"},
	}

	packet, err := BuildPacket(BuildInput{
		Producer: DefaultProducer("test"),
		Subject: Subject{
			Kind:     "commit",
			Ref:      "HEAD",
			SHA:      "abc123",
			DiffHash: "sha256:diff",
		},
		RepoState:    RepoState{GitRoot: ".", Branch: "main"},
		ChangedFiles: files,
		Budget: ContextBudget{
			MaxPacketBytes:        24000,
			MaxChangedFilesListed: 1,
			MaxInlineDiffBytes:    12000,
			MaxArtifactRefs:       12,
			FullSourcePolicy:      "fetch_on_demand",
			OmissionPolicy:        "summarize_and_handle",
		},
	})
	if err != nil {
		t.Fatalf("BuildPacket returned error: %v", err)
	}

	if got := len(packet.ChangedFiles); got != 1 {
		t.Fatalf("changed files = %d, want 1", got)
	}
	if got := len(packet.Omissions); got != 1 {
		t.Fatalf("omissions = %d, want 1", got)
	}
	if packet.Omissions[0].Kind != "changed_files_truncated" {
		t.Fatalf("omission kind = %q, want changed_files_truncated", packet.Omissions[0].Kind)
	}
}

func TestBuildPacket_AssessesRiskBeforeChangedFileTruncation(t *testing.T) {
	files := []ChangedFile{
		{Path: "a.md", Status: "modified"},
		{Path: "internal/cli/init.go", Status: "modified"},
	}

	packet, err := BuildPacket(BuildInput{
		Producer: DefaultProducer("test"),
		Subject: Subject{
			Kind:     "commit",
			Ref:      "HEAD",
			SHA:      "abc123",
			DiffHash: "sha256:diff",
		},
		RepoState:    RepoState{GitRoot: ".", Branch: "main"},
		ChangedFiles: files,
		Budget: ContextBudget{
			MaxPacketBytes:        24000,
			MaxChangedFilesListed: 1,
			MaxInlineDiffBytes:    12000,
			MaxArtifactRefs:       12,
			FullSourcePolicy:      "fetch_on_demand",
			OmissionPolicy:        "summarize_and_handle",
		},
	})
	if err != nil {
		t.Fatalf("BuildPacket returned error: %v", err)
	}

	if got := len(packet.ChangedFiles); got != 1 {
		t.Fatalf("changed files = %d, want truncated packet context", got)
	}
	if packet.Risk.LLMReview != "eligible" {
		t.Fatalf("risk should include truncated governed init surface, got %+v", packet.Risk)
	}
	if !contains(packet.ReviewRequest.Modes, "invariant_conformance") {
		t.Fatalf("review modes = %v, want invariant_conformance", packet.ReviewRequest.Modes)
	}
}

func TestBuildPacket_SpecCarrierChangeIsReviewEligible(t *testing.T) {
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
			Path:   ".haft/specs/target-system.md",
			Status: "modified",
		}},
		Budget: DefaultContextBudget(),
	})
	if err != nil {
		t.Fatalf("BuildPacket returned error: %v", err)
	}

	if packet.Risk.LLMReview != "eligible" {
		t.Fatalf("spec carrier risk = %+v, want review eligible", packet.Risk)
	}
	if !contains(packet.ReviewRequest.Modes, "spec_conformance") {
		t.Fatalf("review modes = %v, want spec_conformance", packet.ReviewRequest.Modes)
	}
}

func contains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}
