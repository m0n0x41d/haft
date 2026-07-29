package specmigrationv2

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/db"
	_ "modernc.org/sqlite"
)

type reviewAdmissionFixture struct {
	root              ApplyProjectRoot
	carrier           FinalCandidatePacketCarrier
	structural        StructuralRequest
	service           ReviewAdmissionService
	database          *sql.DB
	targetSystemBytes []byte
}

func newReviewAdmissionFixture(t *testing.T) reviewAdmissionFixture {
	t.Helper()
	rootPath, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	applyFixture := newApplyE2EFixture(t, rootPath)
	targetSystemBytes := []byte("reviewed target-system bytes\n")
	termMapBytes := []byte("reviewed term-map bytes\n")
	semanticBytes := []byte("reviewed semantic zero-pass bytes\n")
	targetSystemCarrier := mustReviewAdmissionTargetCarrier(t, ".context/review/target-system.md")
	softwareCarrier := mustReviewAdmissionTargetCarrier(t, ".context/review/software-system.md")
	termMapCarrier := mustReviewAdmissionTargetCarrier(t, ".context/review/term-map.md")
	semanticCarrier := mustReviewAdmissionTargetCarrier(t, ".context/review/semantic-zero-pass.md")
	writeReviewAdmissionFile(t, rootPath, targetSystemCarrier.String(), targetSystemBytes)
	writeReviewAdmissionFile(t, rootPath, softwareCarrier.String(), applyFixture.targetBytes)
	writeReviewAdmissionFile(t, rootPath, termMapCarrier.String(), termMapBytes)
	writeReviewAdmissionFile(t, rootPath, semanticCarrier.String(), semanticBytes)

	fpfRoot := filepath.Join(rootPath, "data", "FPF")
	if err := os.MkdirAll(fpfRoot, 0o755); err != nil {
		t.Fatalf("mkdir FPF fixture: %v", err)
	}
	runApplyE2EGit(t, fpfRoot, "init")
	runApplyE2EGit(t, fpfRoot, "config", "user.email", "test@example.com")
	runApplyE2EGit(t, fpfRoot, "config", "user.name", "Haft Test")
	if err := os.WriteFile(filepath.Join(fpfRoot, "FPF-Spec.md"), []byte("# FPF fixture\n"), 0o600); err != nil {
		t.Fatalf("write FPF fixture: %v", err)
	}
	runApplyE2EGit(t, fpfRoot, "add", "FPF-Spec.md")
	runApplyE2EGit(t, fpfRoot, "commit", "-m", "fixture FPF")
	fpfRevision := strings.TrimSpace(string(runApplyE2EGit(t, fpfRoot, "rev-parse", "HEAD")))
	if len(fpfRevision) != 40 {
		t.Fatalf("FPF fixture revision length = %d, want 40", len(fpfRevision))
	}

	basis, err := NewFinalCandidateReviewBasis(FinalCandidateReviewBasisInput{
		CarrierDigests: []ReviewCarrierDigestInput{
			{Role: ReviewTargetSystemCarrier, Carrier: targetSystemCarrier, Digest: DigestBytes(targetSystemBytes)},
			{Role: ReviewSoftwareSystemCarrier, Carrier: softwareCarrier, Digest: DigestBytes(applyFixture.targetBytes)},
			{Role: ReviewTermMapCarrier, Carrier: termMapCarrier, Digest: DigestBytes(termMapBytes)},
		},
		FPFRevision: fpfRevision,
		SemanticZeroPass: SemanticZeroPassInput{
			Carrier: semanticCarrier,
			Digest:  DigestBytes(semanticBytes),
		},
		LifecycleIntent: []LifecycleIntentInput{
			{SectionRef: "SS.alpha.001", Operation: LifecycleActivate},
		},
	})
	if err != nil {
		t.Fatalf("NewFinalCandidateReviewBasis: %v", err)
	}
	carrier, err := FinalizePacketCarrier(applyFixture.structural.packet, basis)
	if err != nil {
		t.Fatalf("FinalizePacketCarrier: %v", err)
	}

	databasePath := filepath.Join(rootPath, ".haft", "haft.db")
	store, err := db.NewStore(databasePath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close store before review service: %v", err)
	}
	dsn := databasePath + "?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	database, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open review database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	service, err := NewReviewAdmissionService(database)
	if err != nil {
		t.Fatalf("NewReviewAdmissionService: %v", err)
	}
	return reviewAdmissionFixture{
		root:              applyFixture.root,
		carrier:           carrier,
		structural:        applyFixture.structural,
		service:           service,
		database:          database,
		targetSystemBytes: targetSystemBytes,
	}
}

func mustReviewAdmissionTargetCarrier(t *testing.T, raw string) TargetCarrierID {
	t.Helper()
	carrier, err := NewTargetCarrierID(raw)
	if err != nil {
		t.Fatalf("NewTargetCarrierID: %v", err)
	}
	return carrier
}

func writeReviewAdmissionFile(t *testing.T, root string, carrier string, content []byte) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(carrier))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir review carrier: %v", err)
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write review carrier: %v", err)
	}
}
