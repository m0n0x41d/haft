package specmigrationv2

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	profileadmissionsqlite "github.com/m0n0x41d/haft/internal/profileadmission/sqlite"
)

type foreignEmbeddedMigrationReview struct {
	admittedMigrationReview
}

type foreignEmbeddedPendingMigrationReview struct {
	PendingMigrationReview
}

func TestApplyBoundaryRejectsForeignTypeEmbeddingAnAdmittedReview(t *testing.T) {
	review := validAdmittedReviewFixture(t)
	if _, err := exactAdmittedMigrationReview(review); err != nil {
		t.Fatalf("package-owned review rejected: %v", err)
	}
	foreign := foreignEmbeddedMigrationReview{admittedMigrationReview: review}
	if _, err := exactAdmittedMigrationReview(foreign); err == nil {
		t.Fatal("foreign embedded review crossed the exact package-owned admission gate")
	}
}

func TestDryRunBoundaryRejectsForeignTypeEmbeddingPendingReview(t *testing.T) {
	missing, err := NewReviewMissingBasisSet([]ReviewMissingBasis{
		MissingHumanSemanticZeroReview,
	})
	if err != nil {
		t.Fatalf("NewReviewMissingBasisSet: %v", err)
	}
	pending, err := NewPendingMigrationReview(missing)
	if err != nil {
		t.Fatalf("NewPendingMigrationReview: %v", err)
	}
	foreign := foreignEmbeddedPendingMigrationReview{PendingMigrationReview: pending}
	if err := validateMigrationReviewResolution(foreign); err == nil {
		t.Fatal("foreign embedded pending review crossed the exact package-owned gate")
	}
}

func TestApplyBoundaryRejectsZeroCanonicalApplicability(t *testing.T) {
	root, err := NewApplyProjectRoot(filepath.Clean(t.TempDir()))
	if err != nil {
		t.Fatalf("NewApplyProjectRoot: %v", err)
	}
	var required profileadmissionsqlite.SoftwareSystemSpecMigrationRequired
	_, err = profileBindingFromRequired(required, root)
	if err == nil {
		t.Fatal("zero P0PA applicability crossed the migration effect gate")
	}
	var unavailable ProfileApplicabilityPreconditionError
	if !errors.As(err, &unavailable) {
		t.Fatalf("error = %T, want ProfileApplicabilityPreconditionError", err)
	}
	if unavailable.Precondition() != ProfileApplicabilityProofInvalid {
		t.Fatalf("precondition = %q, want proof_invalid", unavailable.Precondition())
	}
}

func validAdmittedReviewFixture(t *testing.T) admittedMigrationReview {
	t.Helper()
	reviewRef, err := newReviewRef("review:admitted-test")
	if err != nil {
		t.Fatalf("newReviewRef: %v", err)
	}
	packetDigest, err := NewPacketDigest(DigestBytes([]byte("packet")).String())
	if err != nil {
		t.Fatalf("NewPacketDigest: %v", err)
	}
	sourceDigest := SourceDigestOf([]byte("source"))
	carriers := []ReviewCarrierDigest{
		validReviewCarrierFixture(t, ReviewTargetSystemCarrier, ".context/target.md", "target"),
		validReviewCarrierFixture(t, ReviewSoftwareSystemCarrier, ".context/software.md", "software"),
		validReviewCarrierFixture(t, ReviewTermMapCarrier, ".context/terms.md", "terms"),
	}
	fpfRevision, err := newFPFRevision(strings.Repeat("a", 40))
	if err != nil {
		t.Fatalf("newFPFRevision: %v", err)
	}
	semanticCarrier, err := NewTargetCarrierID(".context/semantic-zero-pass.md")
	if err != nil {
		t.Fatalf("NewTargetCarrierID: %v", err)
	}
	root, err := NewApplyProjectRoot(filepath.Clean(t.TempDir()))
	if err != nil {
		t.Fatalf("NewApplyProjectRoot: %v", err)
	}
	speechActRef, err := NewSemanticReviewSpeechActRef("speech-act:admitted-test")
	if err != nil {
		t.Fatalf("NewSemanticReviewSpeechActRef: %v", err)
	}
	sourceCarrier, err := NewSourceCarrierID(".haft/specs/enabling-system.md")
	if err != nil {
		t.Fatalf("NewSourceCarrierID: %v", err)
	}
	return admittedMigrationReview{
		reviewRef:           reviewRef,
		admissionDigest:     DigestBytes([]byte("review admission")),
		speechActRef:        speechActRef,
		speechActDigest:     DigestBytes([]byte("review SpeechAct")),
		projectRoot:         root,
		packetDigest:        packetDigest,
		packetCarrierDigest: PacketCarrierDigest{value: DigestBytes([]byte("packet carrier"))},
		partitionAudit: PacketPartitionAuditBinding{
			schema: PacketPartitionAuditSchemaVersionV1,
			status: PacketPartitionAuditVerified,
			digest: PacketPartitionAuditDigest{value: DigestBytes([]byte("partition audit"))},
		},
		sourceCarrier:        sourceCarrier,
		sourceDigest:         sourceDigest,
		targetCarrierDigests: ReviewCarrierDigestSet{values: carriers},
		fpfRevision:          fpfRevision,
		semanticZeroPass: SemanticZeroPassBinding{
			carrier: semanticCarrier,
			digest:  DigestBytes([]byte("semantic zero-pass")),
		},
		lifecycleIntent: LifecycleIntent{values: []LifecycleIntentItem{
			{sectionRef: "SS.contract.001", operation: LifecycleActivate},
		}},
	}
}

func validReviewCarrierFixture(
	t *testing.T,
	role ReviewCarrierRole,
	path string,
	content string,
) ReviewCarrierDigest {
	t.Helper()
	carrier, err := NewTargetCarrierID(path)
	if err != nil {
		t.Fatalf("NewTargetCarrierID: %v", err)
	}
	return ReviewCarrierDigest{
		role:    role,
		carrier: carrier,
		digest:  DigestBytes([]byte(content)),
	}
}
