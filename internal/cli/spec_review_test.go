package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/project/specflow"
	"github.com/m0n0x41d/haft/internal/projectledger"
)

func TestRunSpecReviewJSONReturnsAdvisoryPacket(t *testing.T) {
	root := newSpecReviewCLIProject(t)
	restore := enterTestProjectRoot(t, root)
	defer restore()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	restoreJSON := stubSpecReviewJSON(t, true)
	defer restoreJSON()

	err := runSpecReview(cmd, nil)
	if err != nil {
		t.Fatalf("runSpecReview returned error: %v", err)
	}

	var packet specReviewPacket
	if err := json.Unmarshal(output.Bytes(), &packet); err != nil {
		t.Fatalf("decode review JSON: %v\n%s", err, output.String())
	}
	if packet.ReviewKind != specflow.ReviewKindSpecSemantic {
		t.Fatalf("review_kind = %q, want %q", packet.ReviewKind, specflow.ReviewKindSpecSemantic)
	}
	if packet.Authority != specflow.ReviewAuthority {
		t.Fatalf("authority = %q, want %q", packet.Authority, specflow.ReviewAuthority)
	}
	if packet.AuthorityBoundary.Publication != "not_publication" {
		t.Fatalf("authority boundary = %+v", packet.AuthorityBoundary)
	}
	if packet.AuthorityBoundary.ClaimTruth != "not_claim_truth" {
		t.Fatalf("claim truth boundary = %+v", packet.AuthorityBoundary)
	}
	if packet.Profile.ID != specflow.ReviewProfileSemanticV2 {
		t.Fatalf("profile.id = %q, want %q", packet.Profile.ID, specflow.ReviewProfileSemanticV2)
	}
	if specReviewModelDisposition(packet.Profile, "value_slice") != specflow.ReviewModelDispositionAbstain {
		t.Fatalf("profile value_slice disposition = %+v", packet.Profile)
	}
	if packet.Summary.CheckedSections != 2 {
		t.Fatalf("checked_sections = %d, want 2", packet.Summary.CheckedSections)
	}
	if packet.Summary.ExplicitClaims != 1 {
		t.Fatalf("explicit_claims = %d, want 1", packet.Summary.ExplicitClaims)
	}
	section := specReviewSectionByID(
		t,
		packet.ReviewPacket,
		"TS.environment.001",
	)
	if section.ClaimRegister.ExplicitClaims != 1 {
		t.Fatalf("section claim register = %+v, want one explicit claim", section.ClaimRegister)
	}
	if section.SystemFrame.Kind != "target_system" || section.SystemFrame.Source == "" {
		t.Fatalf("section system frame = %+v, want typed target_system frame", section.SystemFrame)
	}
	if section.StateReading.Profile != specflow.ReviewProfileSemanticV2 {
		t.Fatalf("state_reading = %+v, want spec semantic review profile", section.StateReading)
	}
	if section.StateReading.Bearer == "" || section.StateReading.Frame == "" || section.StateReading.Use == "" || section.StateReading.ReopenCondition == "" {
		t.Fatalf("state_reading must qualify bearer/frame/use/reopen condition: %+v", section.StateReading)
	}
	if len(section.Claims) != 1 || section.Claims[0].Class != specflow.ReviewClaimClassLaw {
		t.Fatalf("section claims = %#v, want one L claim", section.Claims)
	}
	if strings.Contains(output.String(), `"status":"ready"`) || strings.Contains(output.String(), `"verdict":"pass"`) {
		t.Fatalf("review JSON must not expose ready/pass authority: %s", output.String())
	}
	if packet.SpecEditionCarrierDelta.Posture !=
		"not_applicable_no_sql_editions" ||
		packet.SpecEditionCarrierDelta.Judgment !=
			specEditionCarrierDeltaJudgment {
		t.Fatalf(
			"carrier delta without SQL editions = %#v",
			packet.SpecEditionCarrierDelta,
		)
	}
}

func TestRunSpecReviewSummaryNamesAdvisoryBoundary(t *testing.T) {
	root := newSpecReviewCLIProject(t)
	restore := enterTestProjectRoot(t, root)
	defer restore()

	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)

	restoreJSON := stubSpecReviewJSON(t, false)
	defer restoreJSON()

	err := runSpecReview(cmd, nil)
	if err != nil {
		t.Fatalf("runSpecReview returned error: %v", err)
	}

	result := output.String()
	if !strings.Contains(result, "advisory_only") {
		t.Fatalf("summary missing advisory boundary:\n%s", result)
	}
	if !strings.Contains(result, "evidence=not_evidence approval=not_approval rebaseline=not_rebaseline gate_decision=not_gate_decision spec_use_admission=not_spec_use_admission claim_truth=not_claim_truth global_truth=not_global_truth publication=not_publication") {
		t.Fatalf("summary missing authority disclaimer:\n%s", result)
	}
	if !strings.Contains(result, "state_readings: per-section profile names bearer, frame, use, and reopen_condition") {
		t.Fatalf("summary missing qualified state readings line:\n%s", result)
	}
	if !strings.Contains(result, "claims: explicit=1 declared=1") {
		t.Fatalf("summary missing claim register counts:\n%s", result)
	}
	if !strings.Contains(result, "profile: spec_semantic_review_v2; value_slice=abstain") {
		t.Fatalf("summary missing profile boundary:\n%s", result)
	}
	if !strings.Contains(
		result,
		"spec_edition_carrier_delta: not_applicable_no_sql_editions",
	) || !strings.Contains(result, specEditionCarrierDeltaJudgment) {
		t.Fatalf("summary missing carrier-delta boundary:\n%s", result)
	}
}

func TestBuildSpecReviewPacketReadsCurrentSQLEditionsBeforeCarriers(t *testing.T) {
	root := setupSpecSyncProject(t)
	database := openSpecSyncDB(t, root)
	defer database.Close()
	store := specflow.NewSQLiteSpecSectionEditionStore(database.GetRawDB())
	section := project.SpecSection{
		ID:            "TS.sql.001",
		Spec:          "target-system",
		SystemFrame:   project.SystemReferenceFrame{ID: "target_system", Kind: "target_system", Source: "declared"},
		Kind:          "acceptance",
		StatementType: "definition",
		ClaimLayer:    "object",
		Owner:         "haft",
		Status:        "active",
		DocumentKind:  "target-system",
		Path:          ".haft/specs/target-system.md",
	}
	edition := specflow.NewSpecSectionEdition("qnt_5eec5eec", section, specflow.SpecSectionSourceSQL, time.Now().UTC())
	if err := store.PutCurrent(edition); err != nil {
		t.Fatalf("seed SQL spec section edition: %v", err)
	}

	packet, err := buildSpecReviewPacket(root)
	if err != nil {
		t.Fatalf("buildSpecReviewPacket: %v", err)
	}
	if packet.Summary.CheckedSections != 1 {
		t.Fatalf("checked_sections = %d, want SQL edition only", packet.Summary.CheckedSections)
	}
	if _, ok := findSpecReviewSection(packet.ReviewPacket, "TS.sql.001"); !ok {
		t.Fatalf("review packet did not include SQL section: %#v", packet.Sections)
	}
	if _, ok := findSpecReviewSection(packet.ReviewPacket, "TS.sync.001"); ok {
		t.Fatalf("review packet included carrier section despite SQL editions: %#v", packet.Sections)
	}
	if packet.SpecEditionCarrierDelta.Posture != "differs" ||
		packet.SpecEditionCarrierDelta.Judgment !=
			specEditionCarrierDeltaJudgment ||
		len(packet.SpecEditionCarrierDelta.Sections) == 0 {
		t.Fatalf(
			"SQL/carrier delta = %#v",
			packet.SpecEditionCarrierDelta,
		)
	}
}

func TestBuildSpecReviewPacketKeepsMalformedCarrierAsObservation(
	t *testing.T,
) {
	root := setupSpecSyncProject(t)
	database := openSpecSyncDB(t, root)
	defer database.Close()
	store := specflow.NewSQLiteSpecSectionEditionStore(database.GetRawDB())
	section := project.SpecSection{
		ID:            "TS.sql.001",
		Spec:          "target-system",
		SystemFrame:   project.SystemReferenceFrame{ID: "target_system", Kind: "target_system", Source: "declared"},
		Kind:          "acceptance",
		StatementType: "definition",
		ClaimLayer:    "object",
		Owner:         "haft",
		Status:        "active",
		DocumentKind:  "target-system",
		Path:          ".haft/specs/target-system.md",
	}
	if err := store.PutCurrent(specflow.NewSpecSectionEdition(
		"qnt_5eec5eec",
		section,
		specflow.SpecSectionSourceSQL,
		time.Now().UTC(),
	)); err != nil {
		t.Fatalf("seed SQL spec section edition: %v", err)
	}
	writeSpecCheckCLIFile(
		t,
		filepath.Join(root, ".haft", "specs", "target-system.md"),
		"```yaml spec-section\nid: [broken\n```\n",
	)

	packet, err := buildSpecReviewPacket(root)
	if err != nil {
		t.Fatalf("buildSpecReviewPacket: %v", err)
	}
	observation := packet.SpecEditionCarrierDelta.CarrierObservation
	if observation.Posture != "parsed_with_findings" ||
		observation.FindingCount == 0 ||
		!slices.Contains(observation.FindingCodes, "spec_section_invalid_yaml") {
		t.Fatalf("malformed carrier observation = %#v", observation)
	}
	if packet.Summary.CheckedSections != 1 ||
		packet.SpecEditionCarrierDelta.Judgment !=
			specEditionCarrierDeltaJudgment {
		t.Fatalf("canonical SQL review was displaced: %#v", packet)
	}
}

func TestHandleQuintQuerySpecReviewReturnsPacket(t *testing.T) {
	fixture := newBoundSpecQueryTestProject(t)
	result, err := handleQuintQuery(context.Background(), fixture.store, nil, fixture.haftDir, map[string]any{
		"action": "spec_review",
	})
	if err != nil {
		t.Fatalf("handleQuintQuery spec_review returned error: %v", err)
	}

	var packet specReviewPacket
	if err := json.Unmarshal([]byte(result), &packet); err != nil {
		t.Fatalf("decode MCP spec_review packet: %v\n%s", err, result)
	}
	if packet.Authority != specflow.ReviewAuthority {
		t.Fatalf("authority = %q, want %q", packet.Authority, specflow.ReviewAuthority)
	}
	if packet.Summary.CheckedSections == 0 {
		t.Fatalf("checked_sections = 0, want active sections")
	}
}

func newBoundSpecQueryTestProject(t *testing.T) checkTestProject {
	t.Helper()
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve physical test home: %v", err)
	}
	t.Setenv("HOME", home)
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve physical project root: %v", err)
	}
	haftDir := filepath.Join(root, ".haft")
	if err := os.MkdirAll(haftDir, 0o755); err != nil {
		t.Fatalf("create .haft directory: %v", err)
	}
	config, err := project.Create(haftDir, root)
	if err != nil {
		t.Fatalf("create project config: %v", err)
	}
	databasePath, err := config.DBPath()
	if err != nil {
		t.Fatalf("resolve project database path: %v", err)
	}
	database, err := openCurrentKernelTestStore(databasePath)
	if err != nil {
		t.Fatalf("create current project database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := projectledger.BindInitialized(
		context.Background(),
		root,
		time.Date(2026, time.July, 22, 0, 0, 0, 0, time.UTC),
	); err != nil {
		t.Fatalf("bind initialized spec query fixture: %v", err)
	}
	rawDatabase := database.GetRawDB()
	fixture := checkTestProject{
		root:    root,
		haftDir: haftDir,
		store:   artifact.NewStore(rawDatabase),
		db:      rawDatabase,
	}
	writeCheckTestSpecCarriers(t, fixture)
	return fixture
}

func newSpecReviewCLIProject(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	specDir := filepath.Join(root, ".haft", "specs")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}

	writeSpecCheckCLIFile(t, filepath.Join(specDir, "target-system.md"), reviewCLISpecSectionWithClaims(
		"TS.environment.001",
		"target-system",
		"target.environment",
		"Test target environment",
		"definition",
		"object",
	))
	writeSpecCheckCLIFile(t, filepath.Join(specDir, "enabling-system.md"), reviewCLISpecSection(
		"ES.agent-policy.001",
		"enabling-system",
		"enabling.agent_policy",
		"Test agent policy",
		"duty",
		"work",
	))
	writeSpecCheckCLIFile(t, filepath.Join(specDir, "term-map.md"), validCLITermMapCarrier())

	return root
}

func reviewCLISpecSectionWithClaims(
	id string,
	spec string,
	kind string,
	title string,
	statementType string,
	claimLayer string,
) string {
	return "## " + id + " " + title + "\n\n" +
		"```yaml spec-section\n" +
		"id: " + id + "\n" +
		"spec: " + spec + "\n" +
		"kind: " + kind + "\n" +
		"title: " + title + "\n" +
		"statement_type: " + statementType + "\n" +
		"claim_layer: " + claimLayer + "\n" +
		"owner: human\n" +
		"status: active\n" +
		"valid_until: 2099-01-01\n" +
		"claims:\n" +
		"  - id: " + id + ".L1\n" +
		"    class: L\n" +
		"    statement: This section defines the target environment.\n" +
		"    governing_pattern_refs:\n" +
		"      - A.6.B\n" +
		"evidence_required:\n" +
		"  - kind: review\n" +
		"    description: Human confirms this section still holds.\n" +
		"```\n"
}

func reviewCLISpecSection(
	id string,
	spec string,
	kind string,
	title string,
	statementType string,
	claimLayer string,
) string {
	return "## " + id + " " + title + "\n\n" +
		"```yaml spec-section\n" +
		"id: " + id + "\n" +
		"spec: " + spec + "\n" +
		"kind: " + kind + "\n" +
		"title: " + title + "\n" +
		"statement_type: " + statementType + "\n" +
		"claim_layer: " + claimLayer + "\n" +
		"owner: human\n" +
		"status: active\n" +
		"valid_until: 2099-01-01\n" +
		"evidence_required:\n" +
		"  - kind: review\n" +
		"    description: Human confirms this section still holds.\n" +
		"```\n"
}

func stubSpecReviewJSON(t *testing.T, value bool) func() {
	t.Helper()

	previous := specReviewJSON
	specReviewJSON = value

	return func() {
		specReviewJSON = previous
	}
}

func specReviewSectionByID(t *testing.T, packet specflow.ReviewPacket, sectionID string) specflow.ReviewSection {
	t.Helper()
	section, ok := findSpecReviewSection(packet, sectionID)
	if ok {
		return section
	}

	t.Fatalf("section %q not found in %#v", sectionID, packet.Sections)
	return specflow.ReviewSection{}
}

func findSpecReviewSection(packet specflow.ReviewPacket, sectionID string) (specflow.ReviewSection, bool) {
	for _, section := range packet.Sections {
		if section.SectionID == sectionID {
			return section, true
		}
	}

	return specflow.ReviewSection{}, false
}

func specReviewModelDisposition(profile specflow.ReviewProfile, name string) string {
	for _, input := range profile.ModelInputs {
		if input.Name != name {
			continue
		}

		return input.Disposition
	}

	return ""
}
