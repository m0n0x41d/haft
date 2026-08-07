package specmigrationv2

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/authority"
)

func TestPrepareMigrationReviewAdmissionBindsStablePreTTYContent(t *testing.T) {
	fixture := newReviewAdmissionFixture(t)
	audit, err := AuditPacketCandidate(fixture.carrier, fixture.structural)
	if err != nil {
		t.Fatalf("AuditPacketCandidate: %v", err)
	}

	first, err := PrepareMigrationReviewAdmission(fixture.carrier, audit)
	if err != nil {
		t.Fatalf("PrepareMigrationReviewAdmission first: %v", err)
	}
	second, err := PrepareMigrationReviewAdmission(fixture.carrier, audit)
	if err != nil {
		t.Fatalf("PrepareMigrationReviewAdmission second: %v", err)
	}

	if first.ReviewContentRef() != second.ReviewContentRef() {
		t.Fatal("same exact packet and audit produced another review-content ref")
	}
	if !first.ReviewContentDigest().Equal(second.ReviewContentDigest()) {
		t.Fatal("same exact packet and audit produced another review-content digest")
	}
	if first.AdmissionRef().String() != second.AdmissionRef().String() {
		t.Fatal("same exact pre-TTY subject produced another stable admission ref")
	}
	if first.SpeechActRef() != second.SpeechActRef() {
		t.Fatal("same exact pre-TTY subject produced another SpeechAct ref")
	}
	if first.ReviewDigest() != second.ReviewDigest() {
		t.Fatal("same exact prepared review produced another review digest")
	}
	for _, want := range []string{
		"Migration effects reviewed for the later invocation",
		"install the reviewed SoftwareSystemSpec at .haft/specs/software-system.md",
		"move the current EnablingSystemSpec source .haft/specs/enabling-system.md to archive .haft/migration-archive/enabling-system.md",
		"preserve explicit lineage for 1 source sections through 1 dispositions and 1 lineage entries",
		"ES.alpha.001 -> target section SS.alpha.001",
		"What this acceptance does not do",
		"Run `haft spec migrate` again later",
		"To accept, type exactly: ACCEPT REVIEWED MIGRATION",
	} {
		if !strings.Contains(first.ReviewText(), want) {
			t.Fatalf("semantic review card omitted %q", want)
		}
	}
	for _, forbidden := range []string{
		"sha256:",
		"review_content_ref:",
		"packet_digest:",
		"review-admission:",
		"speech-act:",
	} {
		if strings.Contains(first.ReviewText(), forbidden) {
			t.Fatalf("human semantic review card exposed machine binding %q", forbidden)
		}
	}
	if strings.Contains(first.ReviewText(), "speech_act_digest") ||
		strings.Contains(first.ReviewText(), "admission_digest") {
		t.Fatal("pre-TTY review text fabricated a future SpeechAct or admission digest")
	}
}

func TestMigrationReviewEffectSummaryShowsEveryDispositionDestination(t *testing.T) {
	fixture := newPacketPartitionAuditFixture(t, TargetDigest{})

	summary, err := migrationReviewEffectSummary(fixture.packet)
	if err != nil {
		t.Fatalf("migrationReviewEffectSummary: %v", err)
	}

	if summary.AuditCounts.SourceSections != 8 ||
		summary.AuditCounts.TopLevelDispositions != 8 ||
		summary.AuditCounts.SplitSections != 7 ||
		summary.AuditCounts.SplitLeaves != 69 ||
		summary.AuditCounts.WholeSectionOutcomes != 1 ||
		summary.AuditCounts.LineageEntries != 70 {
		t.Fatalf("unexpected review audit counts: %+v", summary.AuditCounts)
	}
	if len(summary.SourceDispositions) != 8 {
		t.Fatalf("review disposition count = %d, want 8", len(summary.SourceDispositions))
	}
	first := summary.SourceDispositions[0]
	if first.SourceSection != "ES.audit0.001" ||
		len(first.Destinations) != 1 ||
		first.Destinations[0] != "history archive .haft/migration-archive/enabling-system.md" {
		t.Fatalf("history disposition = %+v", first)
	}
	second := summary.SourceDispositions[1]
	for _, want := range []string{
		"outside PSS carrier .github/workflows/ci.yml",
		"outside PSS carrier AGENTS.md",
		"target section SS.audit.001",
	} {
		if !slices.Contains(second.Destinations, want) {
			t.Fatalf("split disposition omitted %q: %+v", want, second)
		}
	}
}

func TestPrepareMigrationReviewAdmissionRejectsAuditFromAnotherCarrier(t *testing.T) {
	fixture := newReviewAdmissionFixture(t)
	audit, err := AuditPacketCandidate(fixture.carrier, fixture.structural)
	if err != nil {
		t.Fatalf("AuditPacketCandidate: %v", err)
	}
	changed := audit
	changed.packetCarrierDigest = PacketCarrierDigest{value: DigestBytes([]byte("another packet carrier"))}

	_, err = PrepareMigrationReviewAdmission(fixture.carrier, changed)
	if err == nil || !strings.Contains(err.Error(), "exact final-candidate packet carrier") {
		t.Fatalf("mismatched audit error = %v", err)
	}
}

func TestPreparedMigrationReviewAcceptanceContentIsStrictCanonicalJSON(t *testing.T) {
	fixture := newReviewAdmissionFixture(t)
	audit, err := AuditPacketCandidate(fixture.carrier, fixture.structural)
	if err != nil {
		t.Fatalf("AuditPacketCandidate: %v", err)
	}
	prepared, err := PrepareMigrationReviewAdmission(fixture.carrier, audit)
	if err != nil {
		t.Fatalf("PrepareMigrationReviewAdmission: %v", err)
	}

	dto, err := decodeMigrationReviewAcceptanceContent(prepared.state.content.canonical)
	if err != nil {
		t.Fatalf("decode exact acceptance content: %v", err)
	}
	if dto.Schema != migrationReviewAcceptanceContentSchemaV2 {
		t.Fatalf("acceptance schema = %q", dto.Schema)
	}
	if dto.ReviewContentRef != prepared.ReviewContentRef() {
		t.Fatal("acceptance DTO lost its stable content ref")
	}
	if dto.PacketCarrierDigest != fixture.carrier.CarrierDigest().String() {
		t.Fatal("acceptance DTO lost the packet-carrier digest")
	}
	if len(dto.TargetCarrierDigests) != 3 || len(dto.LifecycleIntent) == 0 {
		t.Fatal("acceptance DTO lost target carriers or lifecycle intent")
	}

	tampered := append([]byte{}, prepared.state.content.canonical...)
	tampered = append(tampered, byte('\n'))
	if _, err := decodeMigrationReviewAcceptanceContent(tampered); err == nil {
		t.Fatal("strict decoder accepted non-canonical trailing bytes")
	}
}

func TestReviewAdmissionServiceRejectsUnverifiedSourceWithoutAnyWrite(t *testing.T) {
	fixture := newReviewAdmissionFixture(t)
	audit, err := AuditPacketCandidate(fixture.carrier, fixture.structural)
	if err != nil {
		t.Fatalf("AuditPacketCandidate: %v", err)
	}
	prepared, err := PrepareMigrationReviewAdmission(fixture.carrier, audit)
	if err != nil {
		t.Fatalf("PrepareMigrationReviewAdmission: %v", err)
	}

	_, err = fixture.service.Admit(
		context.Background(),
		prepared,
		authority.VerifiedSpeechActSource{},
	)
	if err == nil || !strings.Contains(err.Error(), "package-verified generic SpeechAct source") {
		t.Fatalf("unverified generic source error = %v", err)
	}
	assertMigrationReviewTableCounts(t, fixture, 0, 0, 0, 0, 0)
}

func TestReviewAdmissionServiceResolveCurrentRequiresExactV2Closure(t *testing.T) {
	fixture := newReviewAdmissionFixture(t)
	audit, err := AuditPacketCandidate(fixture.carrier, fixture.structural)
	if err != nil {
		t.Fatalf("AuditPacketCandidate: %v", err)
	}

	_, err = fixture.service.ResolveCurrentForAudit(
		context.Background(),
		fixture.carrier,
		audit,
	)
	if !errors.Is(err, ErrNoCurrentSemanticReviewAdmission) {
		t.Fatalf("missing exact v2 admission error = %v", err)
	}
}

func TestReviewAdmissionServiceFailsClosedWithoutExactSourceTrigger(t *testing.T) {
	fixture := newReviewAdmissionFixture(t)
	_, err := fixture.database.Exec(
		"DROP TRIGGER migration_review_admissions_v2_exact_sources",
	)
	if err != nil {
		t.Fatalf("drop exact-source trigger: %v", err)
	}

	_, err = NewReviewAdmissionService(fixture.database)
	if err == nil || !strings.Contains(err.Error(), "migration_review_admissions_v2_exact_sources") {
		t.Fatalf("weakened v2 schema error = %v", err)
	}
}

func TestReviewAdmissionServiceFailsClosedWithoutSemanticLiteralSchemaV42(t *testing.T) {
	fixture := newReviewAdmissionFixture(t)
	_, err := fixture.database.Exec("DELETE FROM schema_version WHERE version = 42")
	if err != nil {
		t.Fatalf("remove semantic-literal schema version: %v", err)
	}

	_, err = NewReviewAdmissionService(fixture.database)
	if err == nil || !strings.Contains(err.Error(), "schema version 42 is unavailable") {
		t.Fatalf("schema-41 semantic-review service error = %v", err)
	}
}

func TestReviewAdmissionServiceFailsClosedWithLegacyDigestTriggerAtSchemaV42(t *testing.T) {
	fixture := newReviewAdmissionFixture(t)
	_, err := fixture.database.Exec(
		"DROP TRIGGER migration_review_admissions_v2_exact_sources",
	)
	if err != nil {
		t.Fatalf("drop semantic-literal exact-source trigger: %v", err)
	}
	_, err = fixture.database.Exec(`CREATE TRIGGER migration_review_admissions_v2_exact_sources
		BEFORE INSERT ON migration_review_admissions_v2
		BEGIN
			SELECT RAISE(ABORT, 'legacy digest-transcription protocol');
		END`)
	if err != nil {
		t.Fatalf("install legacy-named exact-source trigger: %v", err)
	}

	_, err = NewReviewAdmissionService(fixture.database)
	if err == nil || !strings.Contains(err.Error(), "does not seal semantic-literal protocol v42") {
		t.Fatalf("legacy digest-trigger service error = %v", err)
	}
}

func TestMigrationReviewDurableClosureRejectsSelfConsistentArbitraryReviewText(t *testing.T) {
	fixture := newReviewAdmissionFixture(t)
	audit, err := AuditPacketCandidate(fixture.carrier, fixture.structural)
	if err != nil {
		t.Fatalf("AuditPacketCandidate: %v", err)
	}
	prepared, err := PrepareMigrationReviewAdmission(fixture.carrier, audit)
	if err != nil {
		t.Fatalf("PrepareMigrationReviewAdmission: %v", err)
	}
	dto, err := decodeMigrationReviewAcceptanceContent(prepared.state.content.canonical)
	if err != nil {
		t.Fatalf("decode acceptance content: %v", err)
	}
	row := migrationReviewContentRowFixture(t, prepared, dto)
	row.admissionReviewText = "self-consistent but arbitrary review text"
	row.sourceReviewText = row.admissionReviewText

	err = validateMigrationReviewContentRow(dto, row)
	if err == nil || !strings.Contains(err.Error(), "canonical review text") {
		t.Fatalf("arbitrary durable review text error = %v", err)
	}
}

func TestMigrationReviewDurableClosureRejectsAlternateAdmissionPolicy(t *testing.T) {
	pins, err := canonicalMigrationReviewProtocolPins()
	if err != nil {
		t.Fatalf("canonical protocol pins: %v", err)
	}
	row := migrationReviewProtocolRowFixture(pins)
	row.admissionContextPolicyRef = "context-policy:generic-accept:v1"
	protocol, err := migrationReviewProtocolPinsFromRow(row)
	if err != nil {
		t.Fatalf("parse alternate admission protocol: %v", err)
	}
	record := migrationReviewAdmissionRecordV2{protocol: protocol}

	err = validateMigrationReviewProtocolRow(record, row)
	if err == nil || !strings.Contains(err.Error(), "sealed protocol") {
		t.Fatalf("alternate admission policy error = %v", err)
	}
}

func TestMigrationReviewDurableClosureRejectsAlternateGenericSourceProtocol(t *testing.T) {
	pins, err := canonicalMigrationReviewProtocolPins()
	if err != nil {
		t.Fatalf("canonical protocol pins: %v", err)
	}
	tests := []struct {
		name   string
		mutate func(*migrationReviewAdmissionV2Row)
	}{
		{
			name: "generic ACCEPT policy",
			mutate: func(row *migrationReviewAdmissionV2Row) {
				row.sourceContextPolicyRef = "context-policy:generic-accept:v1"
			},
		},
		{
			name: "generic ACCEPT method description",
			mutate: func(row *migrationReviewAdmissionV2Row) {
				row.sourceMethodRef = "method:generic-accept"
			},
		},
		{
			name: "generic ACCEPT method",
			mutate: func(row *migrationReviewAdmissionV2Row) {
				row.sourceMethodDescRef = "method-description:generic-accept:v1"
			},
		},
		{
			name: "another registered act type",
			mutate: func(row *migrationReviewAdmissionV2Row) {
				row.sourceActTypeRef = "speech-act-type:authorize"
			},
		},
		{
			name: "another instituted effect rule",
			mutate: func(row *migrationReviewAdmissionV2Row) {
				row.policyEffectRuleRef = "institution-rule:accept-institutes-generic-review:v1"
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			row := migrationReviewProtocolRowFixture(pins)
			test.mutate(&row)
			record := migrationReviewAdmissionRecordV2{protocol: pins}
			err := validateMigrationReviewProtocolRow(record, row)
			if err == nil || !strings.Contains(err.Error(), "sealed protocol") {
				t.Fatalf("alternate source protocol error = %v", err)
			}
		})
	}
}

func migrationReviewContentRowFixture(
	t *testing.T,
	prepared PreparedMigrationReviewAdmission,
	dto migrationReviewAcceptanceContentJSONV2,
) migrationReviewAdmissionV2Row {
	t.Helper()
	carriers, err := marshalMigrationReviewFragment(dto.TargetCarrierDigests)
	if err != nil {
		t.Fatalf("encode carrier digests: %v", err)
	}
	lifecycle, err := marshalMigrationReviewFragment(dto.LifecycleIntent)
	if err != nil {
		t.Fatalf("encode lifecycle intent: %v", err)
	}
	return migrationReviewAdmissionV2Row{
		contentRef:                 dto.ReviewContentRef,
		contentDigest:              prepared.ReviewContentDigest().String(),
		contentRoot:                dto.ProjectRoot,
		packetDigest:               dto.PacketDigest,
		contentPacketCarrierDigest: dto.PacketCarrierDigest,
		auditSchema:                dto.PartitionAuditSchema,
		auditStatus:                dto.PartitionAuditStatus,
		auditDigest:                dto.PartitionAuditDigest,
		sourceCarrier:              dto.SourceCarrier,
		sourceDigest:               dto.SourceDigest,
		targetCarrierDigestsJSON:   string(carriers),
		fpfRevision:                dto.FPFRevision,
		semanticZeroPassCarrier:    dto.SemanticZeroPassCarrier,
		semanticZeroPassDigest:     dto.SemanticZeroPassDigest,
		lifecycleIntentJSON:        string(lifecycle),
		admissionReviewText:        prepared.ReviewText(),
		sourceReviewText:           prepared.ReviewText(),
	}
}

func migrationReviewProtocolRowFixture(
	pins migrationReviewProtocolPins,
) migrationReviewAdmissionV2Row {
	return migrationReviewAdmissionV2Row{
		admissionContextPolicyRef:  pins.contextPolicyRef,
		admissionContextPolicyHash: pins.contextPolicyDigest.String(),
		admissionActTypeRef:        pins.actTypeRef,
		admissionMethodRef:         pins.methodRef,
		admissionMethodDescRef:     pins.methodDescriptionRef,
		admissionMethodDescHash:    pins.methodDescriptionDigest.String(),
		admissionBoundedContextRef: pins.boundedContextRef,
		admissionEffectRuleRef:     pins.effectRuleRef,
		sourceContextPolicyRef:     pins.contextPolicyRef,
		sourceContextPolicyHash:    pins.contextPolicyDigest.String(),
		sourceActTypeRef:           pins.actTypeRef,
		sourceMethodRef:            pins.methodRef,
		sourceMethodDescRef:        pins.methodDescriptionRef,
		sourceMethodDescHash:       pins.methodDescriptionDigest.String(),
		sourceBoundedContextRef:    pins.boundedContextRef,
		policyRef:                  pins.contextPolicyRef,
		policyDigest:               pins.contextPolicyDigest.String(),
		policyBoundedContextRef:    pins.boundedContextRef,
		policyActTypeRef:           pins.actTypeRef,
		policyEffectRuleRef:        pins.effectRuleRef,
		methodRef:                  pins.methodRef,
		methodDescriptionRef:       pins.methodDescriptionRef,
		methodDescriptionDigest:    pins.methodDescriptionDigest.String(),
	}
}

func assertMigrationReviewTableCounts(
	t *testing.T,
	fixture reviewAdmissionFixture,
	legacyActs int,
	legacyAdmissions int,
	contents int,
	admissions int,
	effects int,
) {
	t.Helper()
	wants := map[string]int{
		"migration_review_speech_acts":         legacyActs,
		"migration_review_admissions":          legacyAdmissions,
		"migration_review_acceptance_contents": contents,
		"migration_review_admissions_v2":       admissions,
		"migration_review_instituted_effects":  effects,
	}
	for table, want := range wants {
		var got int
		query := "SELECT COUNT(*) FROM " + table
		if err := fixture.database.QueryRow(query).Scan(&got); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if got != want {
			t.Fatalf("%s count = %d, want %d", table, got, want)
		}
	}
}
