package decisionbinding

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/testsupport/kerneldbfixture"
)

const decisionBindingTransactionWatchdog = 30 * time.Second

func TestDecisionBindingServicePersistsExactTwoPhaseClosure(t *testing.T) {
	fixture := openDecisionBindingServiceFixture(t)
	input := decisionServiceInputFixture()

	result, err := fixture.service.Bind(context.Background(), input)
	if err != nil {
		t.Fatalf("Bind: %v", err)
	}
	if result.DecisionRef() == "" || result.Title() != input.SelectedTitle {
		t.Fatalf("result identity = (%q, %q)", result.DecisionRef(), result.Title())
	}
	if result.SpeechActRef() == "" || result.EffectDigest() == "" {
		t.Fatal("successful binding omitted SpeechAct or effect identity")
	}
	if result.ExactReplay() {
		t.Fatal("first institutional effect was reported as a replay")
	}
	if len(result.Warnings()) != 0 {
		t.Fatalf("binding warnings = %v", result.Warnings())
	}
	if _, err := os.Stat(result.FilePath()); err != nil {
		t.Fatalf("Markdown projection: %v", err)
	}

	assertDecisionBindingRowCount(t, fixture.database, "decision_binding_contents", 1)
	assertDecisionBindingRowCount(t, fixture.database, "speech_acts", 1)
	assertDecisionBindingRowCount(t, fixture.database, "decision_record_instituted_effects", 1)
	assertDecisionBindingRowCount(t, fixture.database, "artifacts", 1)
	assertDecisionBindingRowCount(t, fixture.database, "affected_files", 1)
	assertDecisionBindingRowCount(t, fixture.database, "artifact_links", 0)
	if fixture.captureCount != 1 {
		t.Fatalf("manual SpeechAct capture count = %d, want 1", fixture.captureCount)
	}
	replayed, err := fixture.service.Resume(context.Background(), result.DecisionRef())
	if err != nil {
		t.Fatalf("Resume completed decision: %v", err)
	}
	if !replayed.ExactReplay() || replayed.EffectDigest() != result.EffectDigest() {
		t.Fatal("completed decision did not recover as the same exact institutional effect")
	}
	if fixture.captureCount != 1 {
		t.Fatalf("completed replay recaptured SpeechAct: count = %d", fixture.captureCount)
	}

	var kind string
	var createdAt string
	var updatedAt string
	var fileHash string
	err = fixture.database.QueryRow(
		`SELECT artifact.kind, artifact.created_at, artifact.updated_at,
			COALESCE(file.file_hash, '')
		 FROM artifacts artifact
		 JOIN affected_files file ON file.artifact_id = artifact.id
		 WHERE artifact.id = ?`,
		result.DecisionRef(),
	).Scan(&kind, &createdAt, &updatedAt, &fileHash)
	if err != nil {
		t.Fatalf("read instituted artifact closure: %v", err)
	}
	wantInstant := fixture.occurredAt.Format(time.RFC3339Nano)
	if kind != string(artifact.KindDecisionRecord) ||
		createdAt != wantInstant ||
		updatedAt != wantInstant ||
		fileHash != "" {
		t.Fatalf(
			"artifact closure = (%q, %q, %q, %q), want DecisionRecord at %q with empty baseline hash",
			kind,
			createdAt,
			updatedAt,
			fileHash,
			wantInstant,
		)
	}
}

func TestDecisionBindingServiceKeepsSpecRefsOutOfArtifactForeignKeyGraph(t *testing.T) {
	fixture := openDecisionBindingServiceFixture(t)
	input := decisionServiceInputFixture()
	input.SectionRefs = []string{"TS.decision-binding.001"}

	result, err := fixture.service.Bind(context.Background(), input)
	if err != nil {
		t.Fatalf("Bind spec-linked decision: %v", err)
	}
	decision, err := fixture.store.Get(context.Background(), result.DecisionRef())
	if err != nil {
		t.Fatalf("load spec-linked decision: %v", err)
	}
	fields := decision.UnmarshalDecisionFields()
	if len(fields.SectionRefs) != 1 || fields.SectionRefs[0] != "TS.decision-binding.001" {
		t.Fatalf("structured section refs = %#v", fields.SectionRefs)
	}
	links, err := fixture.store.GetLinks(context.Background(), result.DecisionRef())
	if err != nil {
		t.Fatalf("load artifact links: %v", err)
	}
	for _, link := range links {
		if link.Ref == "TS.decision-binding.001" {
			t.Fatalf("SpecSection cross-carrier ref leaked into Artifact target FK: %#v", link)
		}
	}
}

func TestDecisionBindingServiceEffectFailureRollsBackArtifactAndReusesSpeechAct(t *testing.T) {
	fixture := openDecisionBindingServiceFixture(t)
	_, err := fixture.database.Exec(`CREATE TRIGGER test_abort_decision_effect
		BEFORE INSERT ON decision_record_instituted_effects
		BEGIN SELECT RAISE(ABORT, 'injected effect failure'); END`)
	if err != nil {
		t.Fatalf("install effect failure: %v", err)
	}

	partial, err := fixture.service.Bind(
		context.Background(),
		decisionServiceInputFixture(),
	)
	if err == nil || !strings.Contains(err.Error(), "SpeechAct is durable") {
		t.Fatalf("effect failure = %v", err)
	}
	if partial.DecisionRef() == "" || partial.SpeechActRef() == "" {
		t.Fatal("failed effect did not return its resumable DecisionRecord and SpeechAct refs")
	}
	assertDecisionBindingRowCount(t, fixture.database, "decision_binding_contents", 1)
	assertDecisionBindingRowCount(t, fixture.database, "speech_acts", 1)
	assertDecisionBindingRowCount(t, fixture.database, "artifacts", 0)
	assertDecisionBindingRowCount(t, fixture.database, "affected_files", 0)
	assertDecisionBindingRowCount(t, fixture.database, "decision_record_instituted_effects", 0)
	if fixture.captureCount != 1 {
		t.Fatalf("capture count after failed effect = %d, want 1", fixture.captureCount)
	}

	if _, err := fixture.database.Exec("DROP TRIGGER test_abort_decision_effect"); err != nil {
		t.Fatalf("remove effect failure: %v", err)
	}
	resumed, err := fixture.service.Resume(context.Background(), partial.DecisionRef())
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if resumed.DecisionRef() != partial.DecisionRef() || resumed.SpeechActRef() != partial.SpeechActRef() {
		t.Fatal("resume changed the exact decision or performed SpeechAct identity")
	}
	if fixture.captureCount != 1 {
		t.Fatalf("resume recaptured an already-performed SpeechAct: count = %d", fixture.captureCount)
	}
	assertDecisionBindingRowCount(t, fixture.database, "artifacts", 1)
	assertDecisionBindingRowCount(t, fixture.database, "decision_record_instituted_effects", 1)
}

func TestDecisionBindingServiceGuardRunsAfterDurableSourceBeforeEffectAndResume(t *testing.T) {
	fixture := openDecisionBindingServiceFixture(t)
	guardFailure := errors.New("injected checked-ledger revalidation failure")
	guardCalls := 0
	fixture.service.postSourcePreEffectGuard = func(ctx context.Context) error {
		guardCalls++
		counts := struct {
			speechActs int
			effects    int
			artifacts  int
		}{}
		err := fixture.database.QueryRowContext(
			ctx,
			`SELECT
				(SELECT COUNT(*) FROM speech_acts),
				(SELECT COUNT(*) FROM decision_record_instituted_effects),
				(SELECT COUNT(*) FROM artifacts)`,
		).Scan(
			&counts.speechActs,
			&counts.effects,
			&counts.artifacts,
		)
		if err != nil {
			return fmt.Errorf("guard could not read the closed source transaction: %w", err)
		}
		if counts.speechActs != 1 || counts.effects != 0 || counts.artifacts != 0 {
			return fmt.Errorf(
				"guard order = speech_acts:%d effects:%d artifacts:%d",
				counts.speechActs,
				counts.effects,
				counts.artifacts,
			)
		}
		if guardCalls == 1 {
			return guardFailure
		}
		return nil
	}
	ctx, cancel := context.WithTimeout(
		context.Background(),
		decisionBindingTransactionWatchdog,
	)
	defer cancel()

	partial, err := fixture.service.Bind(ctx, decisionServiceInputFixture())
	if !errors.Is(err, guardFailure) || !strings.Contains(err.Error(), "checked post-source/pre-effect guard") {
		t.Fatalf("guard failure = %v", err)
	}
	if partial.DecisionRef() == "" || partial.SpeechActRef() == "" || partial.EffectDigest() != "" {
		t.Fatal("guard failure did not preserve the durable act without instituting an effect")
	}
	if guardCalls != 1 || fixture.captureCount != 1 {
		t.Fatalf("first attempt calls = guard:%d capture:%d, want 1/1", guardCalls, fixture.captureCount)
	}
	assertDecisionBindingRowCount(t, fixture.database, "decision_binding_contents", 1)
	assertDecisionBindingRowCount(t, fixture.database, "speech_acts", 1)
	assertDecisionBindingRowCount(t, fixture.database, "artifacts", 0)
	assertDecisionBindingRowCount(t, fixture.database, "affected_files", 0)
	assertDecisionBindingRowCount(t, fixture.database, "decision_record_instituted_effects", 0)

	resumed, err := fixture.service.Resume(ctx, partial.DecisionRef())
	if err != nil {
		t.Fatalf("Resume after guard recovery: %v", err)
	}
	if resumed.DecisionRef() != partial.DecisionRef() || resumed.SpeechActRef() != partial.SpeechActRef() {
		t.Fatal("resume changed the exact decision or durable SpeechAct source")
	}
	if guardCalls != 2 || fixture.captureCount != 1 {
		t.Fatalf("resume calls = guard:%d capture:%d, want 2/1", guardCalls, fixture.captureCount)
	}
	assertDecisionBindingRowCount(t, fixture.database, "artifacts", 1)
	assertDecisionBindingRowCount(t, fixture.database, "decision_record_instituted_effects", 1)
}

func TestOpenDecisionBindingServiceRequiresPostSourcePreEffectGuard(t *testing.T) {
	fixture := openDecisionBindingServiceFixture(t)
	_, err := OpenDecisionBindingService(
		fixture.database,
		fixture.store,
		fixture.service.haftDir,
		nil,
	)
	if err == nil || !strings.Contains(err.Error(), "requires a checked post-source/pre-effect guard") {
		t.Fatalf("missing guard admission = %v", err)
	}
}

func TestDecisionBindingServiceRejectedCaptureLeavesOnlyInertContent(t *testing.T) {
	fixture := openDecisionBindingServiceFixture(t)
	fixture.service.capture = func(
		context.Context,
		authority.PreparedManualSpeechAct,
	) (authority.VerifiedSpeechActSource, error) {
		fixture.captureCount++
		return authority.VerifiedSpeechActSource{}, errors.New("operator rejected the review")
	}

	partial, err := fixture.service.Bind(
		context.Background(),
		decisionServiceInputFixture(),
	)
	if err == nil || !strings.Contains(err.Error(), "remains inert and can be resumed") {
		t.Fatalf("capture rejection = %v", err)
	}
	if partial.DecisionRef() == "" || partial.SpeechActRef() != "" || partial.EffectDigest() != "" {
		t.Fatal("capture rejection result confused inert content with performed act or effect")
	}
	assertDecisionBindingRowCount(t, fixture.database, "decision_binding_contents", 1)
	assertDecisionBindingRowCount(t, fixture.database, "speech_acts", 0)
	assertDecisionBindingRowCount(t, fixture.database, "artifacts", 0)
	assertDecisionBindingRowCount(t, fixture.database, "decision_record_instituted_effects", 0)
}

func TestDecisionBindingServiceRevalidatesSourcePinsOnSingleConnectionTransaction(t *testing.T) {
	fixture := openDecisionBindingServiceFixture(t)
	problem := &artifact.Artifact{
		Meta: artifact.Meta{
			ID:        "prob-20260715-transaction-source-a1b2c3d4",
			Kind:      artifact.KindProblemCard,
			Version:   1,
			Status:    artifact.StatusActive,
			Context:   "decision service source pin",
			Mode:      artifact.ModeDeep,
			Title:     "Decision binding freshness source",
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		},
		Body:           "The decision service must revalidate this exact source through its active transaction.",
		StructuredData: "{}",
	}
	if err := fixture.store.Create(context.Background(), problem); err != nil {
		t.Fatalf("seed source artifact: %v", err)
	}
	input := decisionServiceInputFixture()
	input.ProblemRef = problem.Meta.ID
	input.ChoiceResult.ProblemRefs = []string{problem.Meta.ID}
	ctx, cancel := context.WithTimeout(
		context.Background(),
		decisionBindingTransactionWatchdog,
	)
	defer cancel()

	result, err := fixture.service.Bind(ctx, input)
	if err != nil {
		t.Fatalf("Bind with transaction-local source pin: %v", err)
	}
	if ctx.Err() != nil {
		t.Fatalf("single-connection institution exhausted its context: %v", ctx.Err())
	}
	if result.DecisionRef() == "" || result.EffectDigest() == "" {
		t.Fatal("single-connection transaction did not institute the exact decision")
	}
	assertDecisionBindingRowCount(t, fixture.database, "decision_record_instituted_effects", 1)
	assertDecisionBindingRowCount(t, fixture.database, "artifact_links", 1)
}

type decisionBindingServiceFixture struct {
	service      *DecisionBindingService
	database     *sql.DB
	store        *artifact.Store
	occurredAt   time.Time
	captureCount int
	guardCount   int
}

func openDecisionBindingServiceFixture(t *testing.T) *decisionBindingServiceFixture {
	t.Helper()
	root := t.TempDir()
	haftDir := filepath.Join(root, ".haft")
	if err := os.MkdirAll(haftDir, 0o755); err != nil {
		t.Fatalf("create .haft: %v", err)
	}
	ledger, err := kerneldbfixture.OpenCurrentStore(
		filepath.Join(haftDir, "haft.db"),
	)
	if err != nil {
		t.Fatalf("open current kernel test store: %v", err)
	}
	t.Cleanup(func() { _ = ledger.Close() })
	database := ledger.GetRawDB()
	database.SetMaxOpenConns(1)
	database.SetMaxIdleConns(1)
	store := artifact.NewStore(database)
	base := time.Now().UTC().Round(0).Add(-10 * time.Second)
	startedAt := base
	occurredAt := base.Add(time.Second)
	endedAt := base.Add(2 * time.Second)
	fixture := &decisionBindingServiceFixture{
		database:   database,
		store:      store,
		occurredAt: occurredAt,
	}
	guard := func(context.Context) error {
		fixture.guardCount++
		return nil
	}
	service, err := OpenDecisionBindingService(database, store, haftDir, guard)
	if err != nil {
		t.Fatalf("OpenDecisionBindingService: %v", err)
	}
	fixture.service = service
	if database.Stats().MaxOpenConnections != 1 {
		t.Fatal("service fixture did not exercise the checked single-connection ledger topology")
	}
	service.capture = func(
		_ context.Context,
		prepared authority.PreparedManualSpeechAct,
	) (authority.VerifiedSpeechActSource, error) {
		fixture.captureCount++
		return authority.CaptureVerifiedSpeechActForTestFixture(
			t,
			prepared,
			startedAt,
			occurredAt,
			endedAt,
		)
	}
	service.now = func() time.Time { return endedAt.Add(time.Second) }
	return fixture
}

func decisionServiceInputFixture() artifact.DecideInput {
	input := decisionInputFixture()
	input.ProblemRef = ""
	input.ProblemRefs = nil
	input.PortfolioRef = ""
	input.ChoiceResult.ProblemRefs = nil
	input.ChoiceResult.PortfolioRef = ""
	input.SectionRefs = nil
	input.DecisionSubjectRef = ""
	input.BindingTargets = nil
	input.GovernanceTargets = nil
	input.DriftWatchTargets = nil
	input.SpecBindingPreflight = nil
	input.AffectedFiles = []string{"internal/decisionbinding/service.go"}
	return input
}

func assertDecisionBindingRowCount(
	t *testing.T,
	database *sql.DB,
	table string,
	want int,
) {
	t.Helper()
	allowed := map[string]bool{
		"decision_binding_contents":          true,
		"speech_acts":                        true,
		"decision_record_instituted_effects": true,
		"artifacts":                          true,
		"artifact_links":                     true,
		"affected_files":                     true,
	}
	if !allowed[table] {
		t.Fatalf("test row-count table %q is not allowed", table)
	}
	query := "SELECT COUNT(*) FROM " + table
	got := 0
	if err := database.QueryRow(query).Scan(&got); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	if got != want {
		t.Fatalf("%s rows = %d, want %d", table, got, want)
	}
}
