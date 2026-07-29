package specmigrationv2

import "testing"

func TestReviewAnalysisDiagnosticsAcceptExactBindings(t *testing.T) {
	review := validAdmittedReviewFixture(t)
	softwareBinding, found := softwareReviewBinding(review)
	if !found {
		t.Fatal("review fixture has no SoftwareSystemSpec binding")
	}
	analysis := structuralAnalysis{
		packetID:      mustMigrationPacketIDForReview(t, "review-diagnostics"),
		packetDigest:  review.packetDigest,
		sourceCarrier: review.sourceCarrier,
		sourceDigest:  review.sourceDigest,
		targetCarrier: mustTargetCarrierForReview(t, ".haft/specs/software-system.md"),
		targetDigest:  TargetDigest{value: softwareBinding.digest},
	}
	analysis.sourceProvenance = reviewAnalysisProvenanceFixture(t, review, analysis.sourceCarrier)
	diagnostics := reviewAnalysisDiagnostics(review, analysis).Values()
	if len(diagnostics) != 0 {
		t.Fatalf("exact review diagnostics = %#v, want none", diagnostics)
	}
}

func TestReviewAnalysisDiagnosticsPreserveExactMismatchKinds(t *testing.T) {
	review := validAdmittedReviewFixture(t)
	analysis := structuralAnalysis{
		packetID:      mustMigrationPacketIDForReview(t, "review-diagnostics-mismatch"),
		packetDigest:  mustPacketDigestForReview(t, "other-packet"),
		sourceCarrier: mustSourceCarrierForReview(t, ".context/source.md"),
		sourceDigest:  SourceDigestOf([]byte("other-source")),
		targetCarrier: mustTargetCarrierForReview(t, ".context/other-software.md"),
		targetDigest:  TargetDigestOf([]byte("other-software")),
	}
	analysis.sourceProvenance = reviewAnalysisProvenanceFixture(t, review, analysis.sourceCarrier)
	diagnostics := reviewAnalysisDiagnostics(review, analysis).Values()
	want := map[DiagnosticCode]bool{
		DiagnosticReviewPacketDigestMismatch: false,
		DiagnosticReviewSourceDigestMismatch: false,
		DiagnosticReviewTargetDigestMismatch: false,
	}
	for _, diagnostic := range diagnostics {
		if _, expected := want[diagnostic.Code()]; expected {
			want[diagnostic.Code()] = true
		}
	}
	for code, found := range want {
		if !found {
			t.Fatalf("missing diagnostic %q in %#v", code, diagnostics)
		}
	}
}

func TestReviewAnalysisDiagnosticsDistinguishTargetDigestMismatch(t *testing.T) {
	review := validAdmittedReviewFixture(t)
	analysis := structuralAnalysis{
		packetID:      mustMigrationPacketIDForReview(t, "review-target-digest"),
		packetDigest:  review.packetDigest,
		sourceCarrier: review.sourceCarrier,
		sourceDigest:  review.sourceDigest,
		targetCarrier: mustTargetCarrierForReview(t, ".haft/specs/software-system.md"),
		targetDigest:  TargetDigestOf([]byte("other-software")),
	}
	analysis.sourceProvenance = reviewAnalysisProvenanceFixture(t, review, analysis.sourceCarrier)
	diagnostics := reviewAnalysisDiagnostics(review, analysis).Values()
	if len(diagnostics) != 1 || diagnostics[0].Code() != DiagnosticReviewTargetDigestMismatch {
		t.Fatalf("diagnostics = %#v, want target digest mismatch", diagnostics)
	}
}

func reviewAnalysisProvenanceFixture(
	t *testing.T,
	review admittedMigrationReview,
	carrier SourceCarrierID,
) DesignatedSourceProvenance {
	t.Helper()
	provenance, _ := effectGitProvenanceFixture(
		t,
		review.projectRoot,
		carrier,
		review.sourceDigest,
	)
	return provenance
}

func mustMigrationPacketIDForReview(t *testing.T, raw string) MigrationPacketID {
	t.Helper()
	value, err := NewMigrationPacketID(raw)
	if err != nil {
		t.Fatalf("NewMigrationPacketID: %v", err)
	}
	return value
}

func mustPacketDigestForReview(t *testing.T, raw string) PacketDigest {
	t.Helper()
	value, err := NewPacketDigest(DigestBytes([]byte(raw)).String())
	if err != nil {
		t.Fatalf("NewPacketDigest: %v", err)
	}
	return value
}

func mustSourceCarrierForReview(t *testing.T, raw string) SourceCarrierID {
	t.Helper()
	value, err := NewSourceCarrierID(raw)
	if err != nil {
		t.Fatalf("NewSourceCarrierID: %v", err)
	}
	return value
}

func mustTargetCarrierForReview(t *testing.T, raw string) TargetCarrierID {
	t.Helper()
	value, err := NewTargetCarrierID(raw)
	if err != nil {
		t.Fatalf("NewTargetCarrierID: %v", err)
	}
	return value
}
