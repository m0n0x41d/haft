package authority

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/testsupport/kerneldbfixture"
)

type authorityFixture struct {
	database            *sql.DB
	gate                *KernelGate
	now                 time.Time
	basis               AuthorityBasisExpectation
	envelope            AuthorizationEnvelope
	presentation        canonicalPresentation
	authorityResolution canonicalAuthorityResolution
	request             ResolveRequest
}

type authorityRowOverrides struct {
	permissionModality         string
	permissionSourceRef        string
	permissionSubjectRef       string
	permissionContentRef       string
	permissionActionKind       string
	permissionProjectRoot      string
	permissionMethodRef        string
	permissionValidFrom        string
	permissionValidUntil       string
	permissionSingleUseKey     string
	permissionPredicateRef     string
	permissionContextPolicyRef string
	projectBindingDigest       string
	envelopeDigest             string
	presentationDigest         string
	authorityResolutionDigest  string
}

func TestKernelGateResolveAdmitsExactCanonicalRecordWithoutWriting(t *testing.T) {
	fixture := newAuthorityFixture(t, authorityRowOverrides{})
	before := authorityMutationCounts(t, fixture.database)

	result, err := fixture.gate.Resolve(context.Background(), fixture.request)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if result.Kind() != ResolutionAdmitted {
		t.Fatalf("kind = %q, want admitted: %+v", result.Kind(), result)
	}
	presentation, ok := result.Presentation()
	if !ok || presentation.ID() != fixture.presentation.id {
		t.Fatalf("presentation = %+v, ok=%v", presentation, ok)
	}
	authorityResolutionID, ok := result.AuthorityResolutionID()
	if !ok || authorityResolutionID != fixture.authorityResolution.id {
		t.Fatalf("authority resolution = %+v, ok=%v", authorityResolutionID, ok)
	}
	after := authorityMutationCounts(t, fixture.database)
	if before != after {
		t.Fatalf("read-only Resolve mutated ledger: before=%v after=%v", before, after)
	}
}

func TestKernelGateResolveRejectsEveryExactBindingMismatch(t *testing.T) {
	fixture := newAuthorityFixture(t, authorityRowOverrides{})
	otherDigest := testDigest(t, '9')
	otherRef := func(raw string) string { return raw + ":other" }

	tests := []struct {
		name   string
		code   DenialCode
		mutate func(ResolveRequest) ResolveRequest
	}{
		{
			name: "speech act",
			code: DenialSpeechActMismatch,
			mutate: func(value ResolveRequest) ResolveRequest {
				value.basis.speechActRef = mustParse(t, NewSpeechActRef, otherRef(value.basis.speechActRef.String()))
				return value
			},
		},
		{
			name: "speech act digest",
			code: DenialSpeechActMismatch,
			mutate: func(value ResolveRequest) ResolveRequest {
				value.basis.speechActDigest = otherDigest
				return value
			},
		},
		{
			name: "authorization content",
			code: DenialAuthorizationContentMismatch,
			mutate: func(value ResolveRequest) ResolveRequest {
				value.basis.authorizationContentRef = mustParse(t, NewAuthorizationContentRef, otherRef(value.basis.authorizationContentRef.String()))
				return value
			},
		},
		{
			name: "authorization content digest",
			code: DenialAuthorizationContentMismatch,
			mutate: func(value ResolveRequest) ResolveRequest {
				value.basis.authorizationContentDigest = otherDigest
				return value
			},
		},
		{
			name: "permission",
			code: DenialPermissionMismatch,
			mutate: func(value ResolveRequest) ResolveRequest {
				value.basis.permissionRef = mustParse(t, NewPermissionRef, otherRef(value.basis.permissionRef.String()))
				return value
			},
		},
		{
			name: "permission digest",
			code: DenialPermissionMismatch,
			mutate: func(value ResolveRequest) ResolveRequest {
				value.basis.permissionDigest = otherDigest
				return value
			},
		},
		{
			name: "permission admission predicate",
			code: DenialPermissionMismatch,
			mutate: func(value ResolveRequest) ResolveRequest {
				value.basis.permissionPredicateRef = mustParse(
					t,
					NewProfileAdmissionPredicateRef,
					"predicate:profile-declaration-admission:other",
				)
				return value
			},
		},
		{
			name: "context policy",
			code: DenialContextPolicyMismatch,
			mutate: func(value ResolveRequest) ResolveRequest {
				value.basis.contextPolicyRef = mustParse(t, NewContextPolicyRef, otherRef(value.basis.contextPolicyRef.String()))
				return value
			},
		},
		{
			name: "context-policy digest",
			code: DenialContextPolicyMismatch,
			mutate: func(value ResolveRequest) ResolveRequest {
				value.basis.contextPolicyDigest = otherDigest
				return value
			},
		},
		{
			name: "action kind",
			code: DenialEnvelopeMismatch,
			mutate: func(value ResolveRequest) ResolveRequest {
				value.envelope.actionKind = mustParse(t, NewActionKind, "profile.declare.other")
				return value
			},
		},
		{
			name: "project root",
			code: DenialEnvelopeMismatch,
			mutate: func(value ResolveRequest) ResolveRequest {
				value.envelope.projectRoot = mustParse(t, NewProjectRoot, filepath.Join(value.envelope.projectRoot.String(), "other"))
				return value
			},
		},
		{
			name: "role assignment",
			code: DenialEnvelopeMismatch,
			mutate: func(value ResolveRequest) ResolveRequest {
				value.envelope.profileAuthor = mustParse(t, NewRoleAssignmentRef, otherRef(value.envelope.profileAuthor.String()))
				return value
			},
		},
		{
			name: "role assignment digest",
			code: DenialEnvelopeMismatch,
			mutate: func(value ResolveRequest) ResolveRequest {
				value.envelope.profileAuthorDigest = otherDigest
				return value
			},
		},
		{
			name: "method description",
			code: DenialEnvelopeMismatch,
			mutate: func(value ResolveRequest) ResolveRequest {
				value.envelope.methodDescription = mustParse(t, NewMethodDescriptionRef, otherRef(value.envelope.methodDescription.String()))
				return value
			},
		},
		{
			name: "method description digest",
			code: DenialEnvelopeMismatch,
			mutate: func(value ResolveRequest) ResolveRequest {
				value.envelope.methodDescriptionDigest = otherDigest
				return value
			},
		},
		{
			name: "method contract",
			code: DenialEnvelopeMismatch,
			mutate: func(value ResolveRequest) ResolveRequest {
				value.envelope.methodContract = mustParse(
					t,
					NewMethodContractRef,
					otherRef(value.envelope.methodContract.String()),
				)
				return value
			},
		},
		{
			name: "method contract digest",
			code: DenialEnvelopeMismatch,
			mutate: func(value ResolveRequest) ResolveRequest {
				value.envelope.methodContractDigest = otherDigest
				return value
			},
		},
		{
			name: "classifier version",
			code: DenialEnvelopeMismatch,
			mutate: func(value ResolveRequest) ResolveRequest {
				value.envelope.classifierVersion = mustParse(t, NewClassifierVersion, otherRef(value.envelope.classifierVersion.String()))
				return value
			},
		},
		{
			name: "policy version",
			code: DenialEnvelopeMismatch,
			mutate: func(value ResolveRequest) ResolveRequest {
				value.envelope.policyVersion = mustParse(t, NewPolicyVersion, otherRef(value.envelope.policyVersion.String()))
				return value
			},
		},
		{
			name: "session",
			code: DenialEnvelopeMismatch,
			mutate: func(value ResolveRequest) ResolveRequest {
				value.envelope.sessionRef = mustParse(t, NewSessionRef, otherRef(value.envelope.sessionRef.String()))
				return value
			},
		},
		{
			name: "allowed Work start",
			code: DenialEnvelopeMismatch,
			mutate: func(value ResolveRequest) ResolveRequest {
				value.envelope.allowedWorkWindow.from = value.envelope.allowedWorkWindow.from.Add(time.Minute)
				return value
			},
		},
		{
			name: "allowed Work end",
			code: DenialEnvelopeMismatch,
			mutate: func(value ResolveRequest) ResolveRequest {
				value.envelope.allowedWorkWindow.until = value.envelope.allowedWorkWindow.until.Add(-time.Minute)
				return value
			},
		},
		{
			name: "basis-observation start",
			code: DenialEnvelopeMismatch,
			mutate: func(value ResolveRequest) ResolveRequest {
				value.envelope.allowedBasisObservation.from = value.envelope.allowedBasisObservation.from.Add(time.Minute)
				return value
			},
		},
		{
			name: "basis-observation end",
			code: DenialEnvelopeMismatch,
			mutate: func(value ResolveRequest) ResolveRequest {
				value.envelope.allowedBasisObservation.until = value.envelope.allowedBasisObservation.until.Add(-time.Minute)
				return value
			},
		},
		{
			name: "validity start",
			code: DenialEnvelopeMismatch,
			mutate: func(value ResolveRequest) ResolveRequest {
				value.envelope.authorizationValidityWindow.from = value.envelope.authorizationValidityWindow.from.Add(-time.Minute)
				return value
			},
		},
		{
			name: "validity end",
			code: DenialEnvelopeMismatch,
			mutate: func(value ResolveRequest) ResolveRequest {
				value.envelope.authorizationValidityWindow.until = value.envelope.authorizationValidityWindow.until.Add(time.Minute)
				return value
			},
		},
		{
			name: "single-use key",
			code: DenialEnvelopeMismatch,
			mutate: func(value ResolveRequest) ResolveRequest {
				value.envelope.singleUseKey = mustParse(t, NewSingleUseKey, "use.other")
				return value
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := test.mutate(fixture.request)
			result, err := fixture.gate.resolveAt(context.Background(), request, fixture.now)
			if err != nil {
				t.Fatalf("Resolve failed: %v", err)
			}
			assertDeniedWithCode(t, result, test.code)
		})
	}
}

func TestKernelGateResolveRejectsMalformedCanonicalRows(t *testing.T) {
	tests := []struct {
		name      string
		overrides authorityRowOverrides
	}{
		{
			name: "permission modality",
			overrides: authorityRowOverrides{
				permissionModality: "MUST",
			},
		},
		{
			name: "permission source",
			overrides: authorityRowOverrides{
				permissionSourceRef: "work:other",
			},
		},
		{
			name: "permission subject",
			overrides: authorityRowOverrides{
				permissionSubjectRef: "role-assignment:other",
			},
		},
		{
			name: "permission content referent",
			overrides: authorityRowOverrides{
				permissionContentRef: "utterance-description:other",
			},
		},
		{
			name: "permission action",
			overrides: authorityRowOverrides{
				permissionActionKind: "profile.declare.other",
			},
		},
		{
			name: "permission project",
			overrides: authorityRowOverrides{
				permissionProjectRoot: "/tmp/other-project",
			},
		},
		{
			name: "permission method",
			overrides: authorityRowOverrides{
				permissionMethodRef: "method-description:other",
			},
		},
		{
			name: "permission validity start",
			overrides: authorityRowOverrides{
				permissionValidFrom: "2026-07-14T00:00:00Z",
			},
		},
		{
			name: "permission validity end",
			overrides: authorityRowOverrides{
				permissionValidUntil: "2099-07-14T00:00:00Z",
			},
		},
		{
			name: "permission single-use key",
			overrides: authorityRowOverrides{
				permissionSingleUseKey: "use.other",
			},
		},
		{
			name: "permission admission predicate",
			overrides: authorityRowOverrides{
				permissionPredicateRef: "predicate:other",
			},
		},
		{
			name: "permission context policy",
			overrides: authorityRowOverrides{
				permissionContextPolicyRef: "context-policy:other",
			},
		},
		{
			name: "envelope digest",
			overrides: authorityRowOverrides{
				envelopeDigest: testDigest(t, '8').String(),
			},
		},
		{
			name: "project-binding digest",
			overrides: authorityRowOverrides{
				projectBindingDigest: testDigest(t, '0').String(),
			},
		},
		{
			name: "presentation digest",
			overrides: authorityRowOverrides{
				presentationDigest: testDigest(t, '7').String(),
			},
		},
		{
			name: "authority-resolution digest",
			overrides: authorityRowOverrides{
				authorityResolutionDigest: testDigest(t, '6').String(),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := newAuthorityFixture(t, test.overrides)
			result, err := fixture.gate.resolveAt(context.Background(), fixture.request, fixture.now)
			if err != nil {
				t.Fatalf("Resolve failed: %v", err)
			}
			assertDeniedWithCode(t, result, DenialCanonicalRecordInvalid)
		})
	}
}

func TestKernelGateResolveChecksCurrentWindowsAndConsumption(t *testing.T) {
	fixture := newAuthorityFixture(t, authorityRowOverrides{})

	tests := []struct {
		name string
		now  time.Time
		code DenialCode
	}{
		{
			name: "before authorization",
			now:  fixture.envelope.authorizationValidityWindow.from.Add(-time.Nanosecond),
			code: DenialOutsideAuthorizationWindow,
		},
		{
			name: "before authority resolution",
			now:  fixture.authorityResolution.resolvedAt.Add(-time.Nanosecond),
			code: DenialResolutionNotEffective,
		},
		{
			name: "at authority-resolution expiry",
			now:  fixture.authorityResolution.validUntil,
			code: DenialResolutionExpired,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := fixture.gate.resolveAt(context.Background(), fixture.request, test.now)
			if err != nil {
				t.Fatalf("Resolve failed: %v", err)
			}
			assertDeniedWithCode(t, result, test.code)
		})
	}

	insertAuthorityUse(t, fixture, "use-record.one", "admission.one")
	before := authorityMutationCounts(t, fixture.database)
	result, err := fixture.gate.resolveAt(context.Background(), fixture.request, fixture.now)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	assertDeniedWithCode(t, result, DenialSingleUseAlreadyConsumed)
	after := authorityMutationCounts(t, fixture.database)
	if before != after {
		t.Fatalf("consumed-key Resolve mutated ledger: before=%v after=%v", before, after)
	}
}

func TestAuthorityUseUniqueConstraintRejectsACompetingRecord(t *testing.T) {
	fixture := newAuthorityFixture(t, authorityRowOverrides{})
	admissionID := "admission.competing"
	insertTestAdmission(t, fixture, admissionID)
	if err := insertAuthorityUseRaw(fixture, "use-record.first", admissionID); err != nil {
		t.Fatalf("first authority use: %v", err)
	}
	if err := insertAuthorityUseRaw(fixture, "use-record.second", admissionID); err == nil {
		t.Fatal("competing authority-use record reused one resolution and single-use key")
	}
	var count int
	if err := fixture.database.QueryRow("SELECT COUNT(*) FROM authority_uses").Scan(&count); err != nil {
		t.Fatalf("count authority uses: %v", err)
	}
	if count != 1 {
		t.Fatalf("authority use rows = %d, want 1", count)
	}
}

func TestAuthorityUseExactTupleTriggerRejectsMismatchedBindings(t *testing.T) {
	fixture := newAuthorityFixture(t, authorityRowOverrides{})
	admissionID := "admission.exact-tuple"
	insertTestAdmission(t, fixture, admissionID)

	wrongKey := fixture
	wrongKey.envelope.singleUseKey = mustParse(t, NewSingleUseKey, "profile-declaration.use.other")
	if err := insertAuthorityUseRaw(wrongKey, "use-record.wrong-key", admissionID); err == nil {
		t.Fatal("authority-use trigger accepted a single-use key from another envelope")
	}

	wrongResolution := fixture
	wrongResolution.authorityResolution.digest = testDigest(t, '0')
	if err := insertAuthorityUseRaw(wrongResolution, "use-record.wrong-resolution", admissionID); err == nil {
		t.Fatal("authority-use trigger accepted a mismatched authority-resolution digest")
	}
}

func TestKernelGateChecksSingleUseKeyIndependentlyOfResolutionRef(t *testing.T) {
	fixture := newAuthorityFixture(t, authorityRowOverrides{})
	admissionID := "admission.cross-key"
	insertTestAdmission(t, fixture, admissionID)
	if _, err := fixture.database.Exec("DROP TRIGGER authority_uses_exact_tuple"); err != nil {
		t.Fatalf("drop tuple-integrity trigger for corruption fixture: %v", err)
	}
	corrupt := fixture
	corrupt.authorityResolution.id = mustParse(
		t,
		NewAuthorityResolutionID,
		"authority-resolution.unrelated",
	)
	if _, err := fixture.database.Exec("PRAGMA foreign_keys = OFF"); err != nil {
		t.Fatalf("disable foreign keys for corruption fixture: %v", err)
	}
	if err := insertAuthorityUseRaw(corrupt, "use-record.cross-key", admissionID); err != nil {
		t.Fatalf("seed cross-key corruption fixture: %v", err)
	}
	if _, err := fixture.database.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("restore foreign keys after corruption fixture: %v", err)
	}

	result, err := fixture.gate.resolveAt(context.Background(), fixture.request, fixture.now)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	assertDeniedWithCode(t, result, DenialSingleUseAlreadyConsumed)
}

func TestCanonicalLedgerRejectsInsertOrReplaceOnEveryRecordTable(t *testing.T) {
	fixture := newAuthorityFixture(t, authorityRowOverrides{})
	admissionID := "admission.no-replace"
	insertAuthorityUse(t, fixture, "use-record.no-replace", admissionID)
	_, err := fixture.database.Exec(`INSERT INTO project_profile_projection_debt (
		event_id, debt_id, admission_id, admission_digest, project_root,
		ledger_revision, profile_payload_digest, projection_path,
		event_kind, reason_code, detail, expected_projection_digest,
		observed_projection_digest, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'opened', ?, ?, ?, '', ?)`,
		"projection-event.one",
		"projection-debt.one",
		admissionID,
		testDigestValue('f'),
		fixture.envelope.projectRoot.String(),
		1,
		testDigestValue('1'),
		".haft/project-profile.yaml",
		"write_failed",
		"test projection debt",
		testDigestValue('9'),
		formatAuthorityTime(fixture.now),
	)
	if err != nil {
		t.Fatalf("insert projection debt fixture: %v", err)
	}

	tables := []string{
		"authority_presentations",
		"authority_resolution_records",
		"profile_onboarding_work_records",
		"project_profile_admissions",
		"project_profile_revisions",
		"authority_uses",
		"project_profile_projection_debt",
	}
	for _, table := range tables {
		statement := "INSERT OR REPLACE INTO " + table + " SELECT * FROM " + table + " LIMIT 1"
		if _, err := fixture.database.Exec(statement); err == nil {
			t.Fatalf("append-only table %s accepted INSERT OR REPLACE", table)
		}
	}
}

func TestCanonicalLedgerRejectsInsertOrReplaceThroughSecondaryUniqueIdentity(t *testing.T) {
	fixture := newAuthorityFixture(t, authorityRowOverrides{})
	insertAuthorityUse(t, fixture, "use-record.secondary-unique", "admission.secondary-unique")

	for _, trigger := range []string{
		"authority_uses_exact_tuple",
		"project_profile_admissions_revision_cas",
		"project_profile_admissions_exact_authority",
		"project_profile_revisions_exact_admission",
	} {
		if _, err := fixture.database.Exec("DROP TRIGGER " + trigger); err != nil {
			t.Fatalf("drop %s: %v", trigger, err)
		}
	}

	tests := []struct {
		table       string
		firstColumn string
		replacement string
	}{
		{table: "authority_presentations", firstColumn: "presentation_id", replacement: "presentation.secondary"},
		{table: "authority_resolution_records", firstColumn: "authority_resolution_id", replacement: "authority-resolution.secondary"},
		{table: "authority_uses", firstColumn: "use_id", replacement: "use-record.secondary"},
		{table: "profile_onboarding_work_records", firstColumn: "work_record_ref", replacement: "work-record:secondary"},
		{table: "project_profile_admissions", firstColumn: "admission_id", replacement: "admission.secondary"},
		{table: "project_profile_revisions", firstColumn: "project_root", replacement: "/tmp/secondary-project"},
	}
	for _, test := range tests {
		t.Run(test.table, func(t *testing.T) {
			statement := replaceFirstColumnStatement(t, fixture.database, test.table, test.firstColumn)
			if _, err := fixture.database.Exec(statement, test.replacement); err == nil {
				t.Fatalf("append-only table %s accepted replacement through a secondary UNIQUE identity", test.table)
			} else if !strings.Contains(err.Error(), "append-only") &&
				!strings.Contains(err.Error(), "requires exact v38 authority") {
				t.Fatalf("replacement failed for the wrong reason: %v", err)
			}
		})
	}
}

func TestCanonicalLedgerTablesExposeNoHiddenRowIDReplacementPath(t *testing.T) {
	fixture := newAuthorityFixture(t, authorityRowOverrides{})
	insertAuthorityUse(t, fixture, "use-record.no-rowid", "admission.no-rowid")
	_, err := fixture.database.Exec(`INSERT INTO project_profile_projection_debt (
		event_id, debt_id, admission_id, admission_digest, project_root,
		ledger_revision, profile_payload_digest, projection_path,
		event_kind, reason_code, detail, expected_projection_digest,
		observed_projection_digest, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'opened', ?, ?, ?, '', ?)`,
		"projection-event.no-rowid",
		"projection-debt.no-rowid",
		"admission.no-rowid",
		testDigestValue('f'),
		fixture.envelope.projectRoot.String(),
		1,
		testDigestValue('1'),
		".haft/project-profile.yaml",
		"write_failed",
		"test projection debt",
		testDigestValue('9'),
		formatAuthorityTime(fixture.now),
	)
	if err != nil {
		t.Fatalf("insert projection debt fixture: %v", err)
	}

	for _, table := range []string{
		"authority_presentations",
		"authority_resolution_records",
		"authority_uses",
		"profile_onboarding_work_records",
		"project_profile_admissions",
		"project_profile_revisions",
		"project_profile_projection_debt",
	} {
		if _, err := fixture.database.Exec("SELECT rowid FROM " + table + " LIMIT 1"); err == nil {
			t.Fatalf("canonical table %s still exposes a hidden rowid", table)
		}
	}
}

func replaceFirstColumnStatement(
	t *testing.T,
	database *sql.DB,
	table string,
	firstColumn string,
) string {
	t.Helper()
	rows, err := database.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		t.Fatalf("inspect %s columns: %v", table, err)
	}
	defer rows.Close()
	columns := []string{}
	for rows.Next() {
		var ordinal int
		var name string
		var declaredType string
		var notNull int
		var defaultValue sql.NullString
		var primaryKey int
		if err := rows.Scan(&ordinal, &name, &declaredType, &notNull, &defaultValue, &primaryKey); err != nil {
			t.Fatalf("scan %s column: %v", table, err)
		}
		columns = append(columns, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate %s columns: %v", table, err)
	}
	if len(columns) == 0 || columns[0] != firstColumn {
		t.Fatalf("%s first column = %q, want %q", table, columns, firstColumn)
	}
	selectColumns := append([]string{"?"}, columns[1:]...)
	return "INSERT OR REPLACE INTO " + table + " (" +
		strings.Join(columns, ", ") + ") SELECT " +
		strings.Join(selectColumns, ", ") + " FROM " + table + " LIMIT 1"
}

func TestKernelGateResolveSeparatesMissingRecordFromDatabaseFailure(t *testing.T) {
	fixture := newAuthorityFixture(t, authorityRowOverrides{})
	missing := fixture.request
	missing.presentationID = mustParse(t, NewPresentationID, "presentation.missing")

	result, err := fixture.gate.resolveAt(context.Background(), missing, fixture.now)
	if err != nil {
		t.Fatalf("missing record Resolve failed: %v", err)
	}
	assertDeniedWithCode(t, result, DenialCanonicalRecordMissing)

	if err := fixture.database.Close(); err != nil {
		t.Fatalf("close database: %v", err)
	}
	result, err = fixture.gate.resolveAt(context.Background(), fixture.request, fixture.now)
	if err == nil {
		t.Fatal("closed database was conflated with NotAdmitted")
	}
	if result.Kind() != ResolutionInvalid {
		t.Fatalf("result kind = %q, want invalid on effect failure", result.Kind())
	}
}

func TestOpenKernelGateFailsClosedWithoutCanonicalSchema(t *testing.T) {
	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "empty.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()

	if _, err := OpenKernelGate(database); err == nil {
		t.Fatal("kernel gate opened without canonical authority schema")
	}
	if _, err := OpenKernelGate(nil); err == nil {
		t.Fatal("kernel gate opened with nil database")
	}
}

func TestOpenKernelGateRejectsAnIncompleteMigration34Schema(t *testing.T) {
	store, err := kerneldbfixture.OpenCurrentStore(
		filepath.Join(t.TempDir(), "authority.sqlite"),
	)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	database := store.GetRawDB()
	if _, err := database.Exec("DROP TRIGGER authority_uses_exact_tuple"); err != nil {
		t.Fatalf("damage migration fixture: %v", err)
	}
	if _, err := OpenKernelGate(database); err == nil {
		t.Fatal("OpenKernelGate accepted migration 34 with a missing exact-tuple trigger")
	}
}

func TestAuthorityResolutionCannotPredateAuthorizationValidity(t *testing.T) {
	now := time.Now().UTC().Round(0)
	basis := testAuthorityBasis(t)
	envelope := testAuthorizationEnvelope(t, now)
	presentation := testCanonicalPresentation(t, basis, envelope)
	resolution := testCanonicalAuthorityResolution(t, presentation, now)
	resolution.resolvedAt = envelope.authorizationValidityWindow.from.Add(-time.Nanosecond)
	resolution.digest = authorityResolutionDigest(resolution)
	if err := validateCanonicalAuthorityResolution(resolution, presentation); err == nil {
		t.Fatal("authority resolution predating the authorization window was accepted")
	}
}

func newAuthorityFixture(
	t *testing.T,
	overrides authorityRowOverrides,
) authorityFixture {
	t.Helper()
	now := time.Now().UTC().Round(0)
	return newAuthorityFixtureAt(t, overrides, now)
}

func newAuthorityFixtureAt(
	t *testing.T,
	overrides authorityRowOverrides,
	now time.Time,
) authorityFixture {
	if overrides != (authorityRowOverrides{}) {
		return newHistoricalAuthorityFixtureAt(t, overrides, now)
	}
	t.Helper()
	database := openFrozenLegacyAuthoritySchema43(t)
	gate, err := OpenKernelGate(database)
	if err != nil {
		t.Fatalf("OpenKernelGate: %v", err)
	}
	act := testVerifiedAuthorityActAt(t, now)
	canonicalNow := canonicalAuthorityTime(act.state.capture.state.endedAt)
	envelope := act.state.intent.state.content.state.envelope
	support := authorityFixture{
		database: database,
		now:      canonicalNow,
		envelope: envelope,
	}
	insertExactAuthoritySupport(t, support)
	writer, err := OpenAuthorityBasisWriter(database)
	if err != nil {
		t.Fatalf("OpenAuthorityBasisWriter: %v", err)
	}
	writer.now = func() time.Time { return canonicalNow }
	writeResult, err := writer.Record(context.Background(), act)
	if err != nil {
		t.Fatalf("record v38 authority fixture: %v", err)
	}
	recorded, ok := writeResult.RecordedAuthorityBasis()
	if !ok {
		t.Fatalf("v38 authority fixture write kind = %q", writeResult.Kind())
	}
	presentation := recorded.state.legacy.state.presentation
	authorityResolution := recorded.state.legacy.state.resolution
	request, err := profileDeclarationResolveRequestFromBasis(recorded)
	if err != nil {
		t.Fatalf("build strict profile declaration request: %v", err)
	}
	return authorityFixture{
		database:            database,
		gate:                gate,
		now:                 canonicalNow,
		basis:               presentation.basis,
		envelope:            presentation.envelope,
		presentation:        presentation,
		authorityResolution: authorityResolution,
		request:             request,
	}
}

// newHistoricalAuthorityFixtureAt explicitly installs a pre-v38 compatibility
// row for pure legacy parser/history tests. Current gate, admission, replay,
// and tamper tests must use newAuthorityFixtureAt and the real v38 writer.
func newHistoricalAuthorityFixtureAt(
	t *testing.T,
	overrides authorityRowOverrides,
	now time.Time,
) authorityFixture {
	t.Helper()
	database := openFrozenLegacyAuthoritySchema43(t)
	gate, err := OpenKernelGate(database)
	if err != nil {
		t.Fatalf("OpenKernelGate: %v", err)
	}
	basis := testAuthorityBasis(t)
	envelope := testAuthorizationEnvelope(t, now)
	presentation := testCanonicalPresentation(t, basis, envelope)
	authorityResolution := testCanonicalAuthorityResolution(t, presentation, now)
	request, err := NewResolveRequestBuilder(presentation.id, authorityResolution.id).
		ExpectBasis(basis).
		ExpectEnvelope(envelope).
		Build()
	if err != nil {
		t.Fatalf("build resolve request: %v", err)
	}
	fixture := authorityFixture{
		database:            database,
		gate:                gate,
		now:                 now,
		basis:               basis,
		envelope:            envelope,
		presentation:        presentation,
		authorityResolution: authorityResolution,
		request:             request,
	}
	insertExactAuthoritySupport(t, fixture)
	installHistoricalPreV38FixtureBoundary(t, database)
	insertCanonicalRecords(t, fixture, overrides)
	return fixture
}

func newHistoricalAuthorityFixture(
	t *testing.T,
	overrides authorityRowOverrides,
) authorityFixture {
	t.Helper()
	return newHistoricalAuthorityFixtureAt(t, overrides, time.Now().UTC().Round(0))
}

func installHistoricalPreV38FixtureBoundary(t *testing.T, database *sql.DB) {
	t.Helper()
	_, err := database.Exec("DROP TRIGGER authority_presentations_require_v38_basis")
	if err != nil {
		t.Fatalf("install historical presentation fixture boundary: %v", err)
	}
	_, err = database.Exec("DROP TRIGGER authority_resolution_records_require_v38_basis")
	if err != nil {
		t.Fatalf("install historical resolution fixture boundary: %v", err)
	}
}

func testAuthorityBasis(t *testing.T) AuthorityBasisExpectation {
	t.Helper()
	basis, err := NewAuthorityBasisExpectationBuilder().
		FromSpeechAct(
			mustParse(t, NewSpeechActRef, "work:speech-act:profile-declaration"),
			testDigest(t, 'a'),
		).
		DescribedBy(
			mustParse(t, NewAuthorizationContentRef, "utterance-description:profile-declaration"),
			testDigest(t, 'b'),
		).
		InstitutesPermission(
			mustParse(t, NewPermissionRef, "commitment:permission:profile-declaration"),
			testDigest(t, 'c'),
		).
		ScopedBy(mustParse(
			t,
			NewProfileAdmissionPredicateRef,
			"predicate:profile-declaration-admission:v1",
		)).
		UnderContextPolicy(
			mustParse(t, NewContextPolicyRef, "context-policy:profile-declaration:v1"),
			testDigest(t, 'd'),
		).
		Build()
	if err != nil {
		t.Fatalf("build authority basis: %v", err)
	}
	return basis
}

func testAuthorizationEnvelope(t *testing.T, now time.Time) AuthorizationEnvelope {
	t.Helper()
	validity := mustWindow(t, now.Add(-2*time.Hour), now.Add(2*time.Hour))
	work := mustWindow(t, now.Add(-30*time.Minute), now.Add(30*time.Minute))
	basis := mustWindow(t, now.Add(-45*time.Minute), now.Add(45*time.Minute))
	envelope, err := NewAuthorizationEnvelopeBuilder(
		ProfileDeclarationActionKind(),
		mustParse(t, NewProjectRoot, filepath.Join(t.TempDir(), "project")),
	).
		ForProfileAuthor(
			mustParse(t, NewRoleAssignmentRef, "role-assignment:profile-author"),
			testDigest(t, '6'),
		).
		ForMethodDescription(
			mustParse(t, NewMethodDescriptionRef, "method-description:profile-onboarding"),
			testDigest(t, '7'),
		).
		UnderMethodContract(
			mustParse(t, NewMethodContractRef, "method-contract:profile-onboarding:v1"),
			testDigest(t, '8'),
		).
		WithClassifier(
			mustParse(t, NewClassifierVersion, "classifier:v1"),
			mustParse(t, NewPolicyVersion, "profile-policy:v1"),
		).
		InSession(mustParse(t, NewSessionRef, "session:onboarding")).
		AllowWorkWithin(work).
		AllowBasisObservationWithin(basis).
		ValidWithin(validity).
		SingleUse(mustParse(t, NewSingleUseKey, "profile-declaration.use.one")).
		Build()
	if err != nil {
		t.Fatalf("build authorization envelope: %v", err)
	}
	return envelope
}

func testCanonicalPresentation(
	t *testing.T,
	basis AuthorityBasisExpectation,
	envelope AuthorizationEnvelope,
) canonicalPresentation {
	t.Helper()
	id := mustParse(t, NewPresentationID, "presentation.profile-declaration.one")
	digest, err := presentationDigest(id, basis, envelope)
	if err != nil {
		t.Fatalf("digest presentation: %v", err)
	}
	return canonicalPresentation{id: id, basis: basis, envelope: envelope, digest: digest}
}

func testCanonicalAuthorityResolution(
	t *testing.T,
	presentation canonicalPresentation,
	now time.Time,
) canonicalAuthorityResolution {
	t.Helper()
	authorityResolution := canonicalAuthorityResolution{
		id:                       mustParse(t, NewAuthorityResolutionID, "authority-resolution.profile-declaration.one"),
		presentationID:           presentation.id,
		presentationDigest:       presentation.digest,
		profileAuthorRef:         presentation.envelope.profileAuthor,
		profileAuthorDigest:      presentation.envelope.profileAuthorDigest,
		methodDescriptionRef:     presentation.envelope.methodDescription,
		methodDescriptionDigest:  presentation.envelope.methodDescriptionDigest,
		methodContractRef:        presentation.envelope.methodContract,
		methodContractDigest:     presentation.envelope.methodContractDigest,
		verifierIdentity:         mustParse(t, NewVerifierIdentity, "kernel-verifier:local-cli"),
		verifierVersion:          mustParse(t, NewVerifierVersion, "v1"),
		verificationPolicyRef:    mustParse(t, NewVerificationPolicyRef, "verification-policy:local-cli:v1"),
		verificationPolicyDigest: testDigest(t, '5'),
		resolvedAt:               now.Add(-time.Hour),
		validUntil:               now.Add(time.Hour),
	}
	authorityResolution.digest = authorityResolutionDigest(authorityResolution)
	return authorityResolution
}

func insertExactAuthoritySupport(t *testing.T, fixture authorityFixture) {
	t.Helper()
	contextRef := "context:profile-onboarding"
	roleRef := "role:profile-author"
	systemRef := "system:haft-kernel"
	methodDescriptionRef := fixture.envelope.methodDescription.String()
	methodDescriptionDigest := fixture.envelope.methodDescriptionDigest.String()
	methodContractRef := fixture.envelope.methodContract.String()
	methodContractDigest := fixture.envelope.methodContractDigest.String()
	validFrom := formatAuthorityTime(fixture.envelope.authorizationValidityWindow.from)
	validUntil := formatAuthorityTime(fixture.envelope.authorizationValidityWindow.until)
	sessionRef := fixture.envelope.sessionRef.String()

	_, err := fixture.database.Exec(`INSERT INTO profile_onboarding_method_descriptions (
		method_description_ref, described_method_ref, bounded_context_ref,
		source_revision, edition, required_role_ref, required_system_kind,
		state_plane_ref, affected_ref_kind, effect_witness_rule_ref,
		method_description_json, method_description_digest, recorded_at
	) VALUES (?, 'method:profile-onboarding', ?, 'fpf:revision', 'v1', ?,
		'U.System', 'state:profile', 'ProfileClassificationEpistemeV1',
		'rule:effect-witness', '{}', ?, ?)`,
		methodDescriptionRef,
		contextRef,
		roleRef,
		methodDescriptionDigest,
		validFrom,
	)
	if err != nil {
		t.Fatalf("insert exact MethodDescription support: %v", err)
	}
	_, err = fixture.database.Exec(`INSERT INTO profile_onboarding_method_contracts (
		method_contract_ref, edition, method_description_ref,
		method_description_digest, bounded_context_ref,
		role_admission_policy_ref, system_admission_policy_ref,
		parameter_spec_set_digest, accepted_result_kinds_json,
		required_occurrence_slots_json, occurrence_coverage_rule_refs_json,
		effect_state_witness_rule_ref, acceptance_standard_ref,
		acceptance_standard_edition, holder_equals_executed_within_rule_ref,
		method_contract_json, method_contract_digest, recorded_at
	) VALUES (?, 'v1', ?, ?, ?, 'policy:role-admission',
		'policy:system-admission', ?, '["CandidatePayloadProduced","ClassificationUnderdetermined"]',
		'["work_interval","basis_observation_window"]', '["rule:coverage"]',
		'rule:effect-witness', 'acceptance:profile-onboarding', 'v1',
		'rule:holder-is-executor', '{}', ?, ?)`,
		methodContractRef,
		methodDescriptionRef,
		methodDescriptionDigest,
		contextRef,
		testDigestValue('9'),
		methodContractDigest,
		validFrom,
	)
	if err != nil {
		t.Fatalf("insert exact MethodContract support: %v", err)
	}
	_, err = fixture.database.Exec(`INSERT INTO profile_onboarding_executor_system_admissions (
		system_admission_ref, system_ref, admitted_system_kind,
		bounded_context_ref, governing_pattern_ref,
		identity_basis_kind, identity_basis_system_ref,
		identity_basis_kernel_identity, identity_basis_kernel_version,
		identity_basis_designation_ref, identity_basis_designation_digest,
		acting_eligibility_basis_ref, acting_eligibility_basis_digest,
		session_ref, valid_from, valid_until,
		method_description_ref, method_description_digest,
		method_contract_ref, method_contract_digest,
		system_admission_policy_ref, system_admission_json,
		system_admission_digest, recorded_at
	) VALUES ('system-admission:profile-authority', ?, 'U.System', ?, 'A.1',
		'kernel_owned', ?, 'haft-kernel', 'v9', '', '',
		'eligibility:profile-authority', ?, ?, ?, ?, ?, ?, ?, ?,
		'policy:system-admission', '{}', ?, ?)`,
		systemRef,
		contextRef,
		systemRef,
		testDigestValue('a'),
		sessionRef,
		validFrom,
		validUntil,
		methodDescriptionRef,
		methodDescriptionDigest,
		methodContractRef,
		methodContractDigest,
		testDigestValue('b'),
		validFrom,
	)
	if err != nil {
		t.Fatalf("insert exact executor-system admission: %v", err)
	}
	_, err = fixture.database.Exec(`INSERT INTO profile_author_role_admissions (
		role_admission_ref, role_ref, bounded_context_ref,
		governing_pattern_ref, method_description_ref,
		method_description_digest, method_contract_ref,
		method_contract_digest, role_admission_policy_ref,
		role_admission_json, role_admission_digest, recorded_at
	) VALUES ('role-admission:profile-authority', ?, ?, 'A.2.1', ?, ?, ?, ?,
		'policy:role-admission', '{}', ?, ?)`,
		roleRef,
		contextRef,
		methodDescriptionRef,
		methodDescriptionDigest,
		methodContractRef,
		methodContractDigest,
		testDigestValue('c'),
		validFrom,
	)
	if err != nil {
		t.Fatalf("insert exact ProfileAuthor role admission: %v", err)
	}
	_, err = fixture.database.Exec(`INSERT INTO profile_author_assignment_support_carriers (
		assignment_justification_ref, assignment_rule_ref,
		assignment_rule_statement, bounded_context_ref,
		system_admission_ref, system_admission_digest,
		role_admission_ref, role_admission_digest,
		assignment_from, assignment_until, method_contract_ref,
		method_contract_digest, assignment_justification_json,
		assignment_justification_digest, assignment_provenance_ref,
		provenance_justification_ref, provenance_justification_digest,
		session_ref, kernel_identity, kernel_version,
		runtime_identity, runtime_version, provenance_recorded_at,
		assignment_provenance_json, assignment_provenance_digest, recorded_at
	) VALUES ('assignment-justification:profile-authority', 'rule:assignment',
		'exact pre-existing assignment support', ?,
		'system-admission:profile-authority', ?,
		'role-admission:profile-authority', ?, ?, ?, ?, ?, '{}', ?,
		'assignment-provenance:profile-authority',
		'assignment-justification:profile-authority', ?, ?,
		'haft-kernel', 'v9', 'codex', 'v1', ?, '{}', ?, ?)`,
		contextRef,
		testDigestValue('b'),
		testDigestValue('c'),
		validFrom,
		validUntil,
		methodContractRef,
		methodContractDigest,
		testDigestValue('d'),
		testDigestValue('d'),
		sessionRef,
		validFrom,
		testDigestValue('e'),
		validFrom,
	)
	if err != nil {
		t.Fatalf("insert exact assignment support carrier: %v", err)
	}
	_, err = fixture.database.Exec(`INSERT INTO profile_author_role_assignments (
		role_assignment_ref, holder_system_ref, admitted_role_ref,
		bounded_context_ref, valid_from, valid_until,
		system_admission_ref, system_admission_digest,
		role_admission_ref, role_admission_digest,
		assignment_justification_ref, assignment_justification_digest,
		assignment_provenance_ref, assignment_provenance_digest,
		role_assignment_json, role_assignment_digest, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, 'system-admission:profile-authority', ?,
		'role-admission:profile-authority', ?,
		'assignment-justification:profile-authority', ?,
		'assignment-provenance:profile-authority', ?, '{}', ?, ?)`,
		fixture.envelope.profileAuthor.String(),
		systemRef,
		roleRef,
		contextRef,
		validFrom,
		validUntil,
		testDigestValue('b'),
		testDigestValue('c'),
		testDigestValue('d'),
		testDigestValue('e'),
		fixture.envelope.profileAuthorDigest.String(),
		validFrom,
	)
	if err != nil {
		t.Fatalf("insert exact ProfileAuthor RoleAssignment: %v", err)
	}
}

func insertCanonicalRecords(
	t *testing.T,
	fixture authorityFixture,
	overrides authorityRowOverrides,
) {
	t.Helper()
	envelopeDigest, err := fixture.envelope.Digest()
	if err != nil {
		t.Fatalf("digest envelope: %v", err)
	}
	permissionModality := permissionModalityMay
	if overrides.permissionModality != "" {
		permissionModality = overrides.permissionModality
	}
	permissionSourceRef := fixture.basis.speechActRef.String()
	if overrides.permissionSourceRef != "" {
		permissionSourceRef = overrides.permissionSourceRef
	}
	permissionSubjectRef := fixture.envelope.profileAuthor.String()
	if overrides.permissionSubjectRef != "" {
		permissionSubjectRef = overrides.permissionSubjectRef
	}
	permissionContentRef := fixture.basis.authorizationContentRef.String()
	if overrides.permissionContentRef != "" {
		permissionContentRef = overrides.permissionContentRef
	}
	permissionActionKind := fixture.envelope.actionKind.String()
	if overrides.permissionActionKind != "" {
		permissionActionKind = overrides.permissionActionKind
	}
	permissionProjectRoot := fixture.envelope.projectRoot.String()
	if overrides.permissionProjectRoot != "" {
		permissionProjectRoot = overrides.permissionProjectRoot
	}
	permissionMethodRef := fixture.envelope.methodDescription.String()
	if overrides.permissionMethodRef != "" {
		permissionMethodRef = overrides.permissionMethodRef
	}
	permissionValidFrom := formatAuthorityTime(fixture.envelope.authorizationValidityWindow.from)
	if overrides.permissionValidFrom != "" {
		permissionValidFrom = overrides.permissionValidFrom
	}
	permissionValidUntil := formatAuthorityTime(fixture.envelope.authorizationValidityWindow.until)
	if overrides.permissionValidUntil != "" {
		permissionValidUntil = overrides.permissionValidUntil
	}
	permissionSingleUseKey := fixture.envelope.singleUseKey.String()
	if overrides.permissionSingleUseKey != "" {
		permissionSingleUseKey = overrides.permissionSingleUseKey
	}
	permissionPredicateRef := fixture.basis.permissionPredicateRef.String()
	if overrides.permissionPredicateRef != "" {
		permissionPredicateRef = overrides.permissionPredicateRef
	}
	permissionContextPolicyRef := fixture.basis.contextPolicyRef.String()
	if overrides.permissionContextPolicyRef != "" {
		permissionContextPolicyRef = overrides.permissionContextPolicyRef
	}
	storedEnvelopeDigest := envelopeDigest.String()
	if overrides.envelopeDigest != "" {
		storedEnvelopeDigest = overrides.envelopeDigest
	}
	storedPresentationDigest := fixture.presentation.digest.String()
	if overrides.presentationDigest != "" {
		storedPresentationDigest = overrides.presentationDigest
	}
	storedAuthorityResolutionDigest := fixture.authorityResolution.digest.String()
	if overrides.authorityResolutionDigest != "" {
		storedAuthorityResolutionDigest = overrides.authorityResolutionDigest
	}
	projectBindingDigest, err := ProjectBindingDigest(
		fixture.envelope.actionKind,
		fixture.envelope.projectRoot,
	)
	if err != nil {
		t.Fatalf("digest project binding: %v", err)
	}
	if overrides.projectBindingDigest != "" {
		projectBindingDigest = mustParse(t, NewDigest, overrides.projectBindingDigest)
	}
	if overrides.permissionModality != "" {
		if _, err := fixture.database.Exec("PRAGMA ignore_check_constraints = ON"); err != nil {
			t.Fatalf("disable CHECK constraints for malformed-row fixture: %v", err)
		}
		defer func() {
			if _, err := fixture.database.Exec("PRAGMA ignore_check_constraints = OFF"); err != nil {
				t.Fatalf("restore CHECK constraints after malformed-row fixture: %v", err)
			}
		}()
	}
	triggerBypassRequired := overrides.permissionSubjectRef != "" ||
		overrides.permissionActionKind != "" ||
		overrides.permissionProjectRoot != "" ||
		overrides.permissionMethodRef != "" ||
		overrides.permissionValidFrom != "" ||
		overrides.permissionValidUntil != "" ||
		overrides.permissionSingleUseKey != ""
	if triggerBypassRequired {
		_, err = fixture.database.Exec("DROP TRIGGER authority_presentations_exact_assignment_method")
		if err != nil {
			t.Fatalf("disable exact-presentation trigger for malformed-row fixture: %v", err)
		}
	}
	_, err = fixture.database.Exec(`INSERT INTO authority_presentations (
		presentation_id, speech_act_ref, speech_act_digest,
		authorization_content_ref, authorization_content_digest,
		permission_ref, permission_digest, permission_modality,
		permission_source_speech_act_ref,
		permission_subject_role_assignment_ref,
		permission_authorization_content_ref,
		permission_action_kind, permission_project_root,
		permission_method_description_ref,
		permission_valid_from, permission_valid_until,
		permission_single_use_key, permission_profile_admission_predicate_ref,
		permission_context_policy_ref,
		context_policy_ref,
		context_policy_digest, action_kind, project_root,
		profile_author_role_assignment_ref, profile_author_role_assignment_digest,
		method_description_ref, method_description_digest,
		method_contract_ref, method_contract_digest,
		classifier_version, policy_version, session_ref,
		allowed_work_from, allowed_work_until,
		basis_observation_from, basis_observation_until,
		valid_from, valid_until, single_use_key, project_binding_digest, envelope_digest,
		presentation_digest, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		fixture.presentation.id.String(),
		fixture.basis.speechActRef.String(),
		fixture.basis.speechActDigest.String(),
		fixture.basis.authorizationContentRef.String(),
		fixture.basis.authorizationContentDigest.String(),
		fixture.basis.permissionRef.String(),
		fixture.basis.permissionDigest.String(),
		permissionModality,
		permissionSourceRef,
		permissionSubjectRef,
		permissionContentRef,
		permissionActionKind,
		permissionProjectRoot,
		permissionMethodRef,
		permissionValidFrom,
		permissionValidUntil,
		permissionSingleUseKey,
		permissionPredicateRef,
		permissionContextPolicyRef,
		fixture.basis.contextPolicyRef.String(),
		fixture.basis.contextPolicyDigest.String(),
		fixture.envelope.actionKind.String(),
		fixture.envelope.projectRoot.String(),
		fixture.envelope.profileAuthor.String(),
		fixture.envelope.profileAuthorDigest.String(),
		fixture.envelope.methodDescription.String(),
		fixture.envelope.methodDescriptionDigest.String(),
		fixture.envelope.methodContract.String(),
		fixture.envelope.methodContractDigest.String(),
		fixture.envelope.classifierVersion.String(),
		fixture.envelope.policyVersion.String(),
		fixture.envelope.sessionRef.String(),
		formatAuthorityTime(fixture.envelope.allowedWorkWindow.from),
		formatAuthorityTime(fixture.envelope.allowedWorkWindow.until),
		formatAuthorityTime(fixture.envelope.allowedBasisObservation.from),
		formatAuthorityTime(fixture.envelope.allowedBasisObservation.until),
		formatAuthorityTime(fixture.envelope.authorizationValidityWindow.from),
		formatAuthorityTime(fixture.envelope.authorizationValidityWindow.until),
		fixture.envelope.singleUseKey.String(),
		projectBindingDigest.String(),
		storedEnvelopeDigest,
		storedPresentationDigest,
		formatAuthorityTime(fixture.now.Add(-time.Hour)),
	)
	if err != nil {
		t.Fatalf("insert authority presentation: %v", err)
	}
	_, err = fixture.database.Exec(`INSERT INTO authority_resolution_records (
		authority_resolution_id, presentation_id, presentation_digest,
		profile_author_role_assignment_ref, profile_author_role_assignment_digest,
		method_description_ref, method_description_digest,
		method_contract_ref, method_contract_digest,
		verifier_identity, verifier_version,
		verification_policy_ref, verification_policy_digest,
		resolved_at, valid_until, authority_resolution_digest, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		fixture.authorityResolution.id.String(),
		fixture.authorityResolution.presentationID.String(),
		storedPresentationDigest,
		fixture.authorityResolution.profileAuthorRef.String(),
		fixture.authorityResolution.profileAuthorDigest.String(),
		fixture.authorityResolution.methodDescriptionRef.String(),
		fixture.authorityResolution.methodDescriptionDigest.String(),
		fixture.authorityResolution.methodContractRef.String(),
		fixture.authorityResolution.methodContractDigest.String(),
		fixture.authorityResolution.verifierIdentity.String(),
		fixture.authorityResolution.verifierVersion.String(),
		fixture.authorityResolution.verificationPolicyRef.String(),
		fixture.authorityResolution.verificationPolicyDigest.String(),
		formatAuthorityTime(fixture.authorityResolution.resolvedAt),
		formatAuthorityTime(fixture.authorityResolution.validUntil),
		storedAuthorityResolutionDigest,
		formatAuthorityTime(fixture.now.Add(-time.Hour)),
	)
	if err != nil {
		t.Fatalf("insert authority-resolution record: %v", err)
	}
}

func insertAuthorityUse(t *testing.T, fixture authorityFixture, useID string, admissionID string) {
	t.Helper()
	insertTestAdmission(t, fixture, admissionID)
	if err := insertAuthorityUseRaw(fixture, useID, admissionID); err != nil {
		t.Fatalf("insert authority use: %v", err)
	}
	insertTestRevision(t, fixture, admissionID)
}

func insertAuthorityUseRaw(fixture authorityFixture, useID string, admissionID string) error {
	envelopeDigest, err := fixture.envelope.Digest()
	if err != nil {
		return err
	}
	projectBindingDigest, err := ProjectBindingDigest(
		fixture.envelope.actionKind,
		fixture.envelope.projectRoot,
	)
	if err != nil {
		return err
	}
	_, err = fixture.database.Exec(`INSERT INTO authority_uses (
		use_id, authority_resolution_ref, authority_resolution_digest,
		single_use_key, action_kind, project_root, project_binding_digest,
		envelope_digest, authority_record_ref, authority_record_digest,
		admission_request_digest, verifier_identity, verifier_version,
		committed_result_ref, committed_result_digest, consumed_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		useID,
		fixture.authorityResolution.id.String(),
		fixture.authorityResolution.digest.String(),
		fixture.envelope.singleUseKey.String(),
		fixture.envelope.actionKind.String(),
		fixture.envelope.projectRoot.String(),
		projectBindingDigest.String(),
		envelopeDigest.String(),
		fixture.presentation.id.String(),
		fixture.presentation.digest.String(),
		testDigestValue('e'),
		fixture.authorityResolution.verifierIdentity.String(),
		fixture.authorityResolution.verifierVersion.String(),
		admissionID,
		testDigestValue('f'),
		formatAuthorityTime(fixture.now),
	)
	return err
}

func insertTestAdmission(t *testing.T, fixture authorityFixture, admissionID string) {
	t.Helper()
	insertTestWorkRecord(t, fixture)
	projectBindingDigest, err := ProjectBindingDigest(
		fixture.envelope.actionKind,
		fixture.envelope.projectRoot,
	)
	if err != nil {
		t.Fatalf("digest project binding: %v", err)
	}
	const profilePayload = `{"scopes":["haft-software"]}`
	const receiptPayload = `{"kind":"profile-declaration"}`
	candidateProvenanceDigest := testDigestValue('2')
	candidateProvenance := fmt.Sprintf(
		`{"authority_basis_ref":%q,"classifier_version":%q,"observed_project_basis_digest":%q,"observed_project_basis_ref":%q,"outcome_assessment_digest":%q,"outcome_assessment_ref":%q,"payload_digest":%q,"policy_version":%q,"profile_author_role_assignment_digest":%q,"profile_author_role_assignment_ref":%q,"project_root":%q,"provenance_digest":%q,"session_ref":%q,"work_record_digest":%q,"work_record_ref":%q}`,
		fixture.presentation.id.String(),
		fixture.envelope.classifierVersion.String(),
		testDigestValue('3'),
		"observed-project-basis:profile-onboarding",
		testDigestValue('8'),
		"outcome-assessment:profile-onboarding",
		testDigestValue('1'),
		fixture.envelope.policyVersion.String(),
		fixture.envelope.profileAuthorDigest.String(),
		fixture.envelope.profileAuthor.String(),
		fixture.envelope.projectRoot.String(),
		candidateProvenanceDigest,
		fixture.envelope.sessionRef.String(),
		testDigestValue('5'),
		"work-record:profile-onboarding",
	)
	_, err = fixture.database.Exec(`INSERT INTO project_profile_admissions (
		admission_id, action_kind, project_root, project_binding_digest,
		profile_payload_json, candidate_provenance_json,
		candidate_provenance_digest, profile_author_role_assignment_ref,
		profile_author_role_assignment_digest, profile_payload_digest,
		observed_project_basis_ref, observed_project_basis_digest,
		work_record_ref, work_record_digest,
		outcome_assessment_ref, outcome_assessment_digest,
		authority_basis_ref, authority_basis_digest,
		authority_resolution_ref, authority_resolution_digest,
		receipt_json, receipt_digest, expected_ledger_revision,
		ledger_revision, single_use_key, admission_request_digest,
		admission_json, admission_digest, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		admissionID,
		fixture.envelope.actionKind.String(),
		fixture.envelope.projectRoot.String(),
		projectBindingDigest.String(),
		profilePayload,
		candidateProvenance,
		candidateProvenanceDigest,
		fixture.envelope.profileAuthor.String(),
		fixture.envelope.profileAuthorDigest.String(),
		testDigestValue('1'),
		"observed-project-basis:profile-onboarding",
		testDigestValue('3'),
		"work-record:profile-onboarding",
		testDigestValue('5'),
		"outcome-assessment:profile-onboarding",
		testDigestValue('8'),
		fixture.presentation.id.String(),
		fixture.presentation.digest.String(),
		fixture.authorityResolution.id.String(),
		fixture.authorityResolution.digest.String(),
		receiptPayload,
		testDigestValue('4'),
		0,
		1,
		fixture.envelope.singleUseKey.String(),
		testDigestValue('e'),
		`{"schema":"profile-admission-test"}`,
		testDigestValue('f'),
		formatAuthorityTime(fixture.now),
	)
	if err != nil {
		t.Fatalf("insert profile admission: %v", err)
	}
}

func insertTestWorkRecord(t *testing.T, fixture authorityFixture) {
	t.Helper()
	basisRef := "observed-project-basis:profile-onboarding"
	basisDigest := testDigestValue('3')
	workRecordRef := "work-record:profile-onboarding"
	workRef := "work:profile-onboarding"
	workDigest := testDigestValue('5')
	effectRef := "effect:profile-onboarding"
	effectDigest := testDigestValue('7')
	assessmentRef := "outcome-assessment:profile-onboarding"
	assessmentDigest := testDigestValue('8')
	workFrom := formatAuthorityTime(fixture.envelope.allowedWorkWindow.from)
	workUntil := formatAuthorityTime(fixture.envelope.allowedWorkWindow.until)
	basisFrom := formatAuthorityTime(fixture.envelope.allowedBasisObservation.from)
	basisUntil := formatAuthorityTime(fixture.envelope.allowedBasisObservation.until)
	recordedAt := formatAuthorityTime(fixture.now)
	parameterBindings := fmt.Sprintf(
		`[{"name":"classifier_version","value":%q},{"name":"policy_version","value":%q},{"name":"project_root","value":%q},{"name":"session_ref","value":%q}]`,
		fixture.envelope.classifierVersion.String(),
		fixture.envelope.policyVersion.String(),
		fixture.envelope.projectRoot.String(),
		fixture.envelope.sessionRef.String(),
	)
	inputsJSON := fmt.Sprintf(`[%q]`, basisRef)
	_, err := fixture.database.Exec(`INSERT INTO observed_project_bases (
		observed_project_basis_ref, project_root, observation_from,
		observation_until, detector_version, classifier_version,
		observed_project_basis_json, observed_project_basis_digest, recorded_at
	) VALUES (?, ?, ?, ?, 'detector:v1', ?, '{}', ?, ?)`,
		basisRef,
		fixture.envelope.projectRoot.String(),
		basisFrom,
		basisUntil,
		fixture.envelope.classifierVersion.String(),
		basisDigest,
		basisUntil,
	)
	if err != nil {
		t.Fatalf("insert observed project basis: %v", err)
	}
	_, err = fixture.database.Exec(`INSERT INTO profile_onboarding_work_records (
		work_record_ref, work_ref, project_root, enacts_method_ref,
		method_description_ref, method_description_digest,
		method_contract_ref, method_contract_digest, parameter_bindings_json,
		performed_by_role_assignment_ref, profile_author_role_assignment_ref,
		profile_author_role_assignment_digest, executed_within_ref,
		work_from, work_until, bounded_context_ref,
		basis_observation_from, basis_observation_until,
		observed_project_basis_ref, observed_project_basis_digest,
		inputs_json, outputs_json, resources_json, affected_ref_kind, affected_refs_json,
		state_plane_ref, pre_state_ref, post_state_ref, delta_predicate_ref,
		outcome_kind,
		profile_payload_digest, observed_basis_digest, missing_basis_digest,
		work_record_json, work_record_digest, recorded_at
	) VALUES (
		?, ?, ?, ?,
		?, ?, ?, ?, ?,
		?, ?, ?, ?,
		?, ?, ?,
		?, ?, ?, ?,
		?, ?, ?, ?, ?,
		?, ?, ?, '',
		'CandidatePayloadProduced', ?, ?, '', '{}', ?, ?
	)`,
		workRecordRef,
		workRef,
		fixture.envelope.projectRoot.String(),
		"method:profile-onboarding",
		fixture.envelope.methodDescription.String(),
		fixture.envelope.methodDescriptionDigest.String(),
		fixture.envelope.methodContract.String(),
		fixture.envelope.methodContractDigest.String(),
		parameterBindings,
		fixture.envelope.profileAuthor.String(),
		fixture.envelope.profileAuthor.String(),
		fixture.envelope.profileAuthorDigest.String(),
		"system:haft-kernel",
		workFrom,
		workUntil,
		"context:profile-onboarding",
		basisFrom,
		basisUntil,
		basisRef,
		basisDigest,
		inputsJSON,
		`["profile-candidate-payload"]`,
		`["repository"]`,
		"ProfileClassificationEpistemeV1",
		`["episteme:profile-classification"]`,
		"state:profile",
		"state:before-profile-classification",
		"state:after-profile-classification",
		testDigestValue('1'),
		basisDigest,
		workDigest,
		recordedAt,
	)
	if err != nil {
		t.Fatalf("insert profile-onboarding Work record: %v", err)
	}
	_, err = fixture.database.Exec(`INSERT INTO profile_onboarding_effects (
		effect_ref, work_record_ref, work_ref, work_record_digest,
		result_kind, output_ref, profile_payload_digest,
		observed_project_basis_ref, observed_project_basis_digest,
		missing_basis_digest, affected_entity_refs_json, state_plane_ref,
		pre_state_ref, post_state_ref, delta_predicate_ref,
		evidence_provenance_path_refs_json, effect_json, effect_digest, recorded_at
	) VALUES (?, ?, ?, ?, 'CandidatePayloadProduced',
		'profile-candidate-payload', ?, ?, ?, '',
		'["episteme:profile-classification"]', 'state:profile',
		'state:before-profile-classification', 'state:after-profile-classification', '',
		'[]', '{}', ?, ?)`,
		effectRef,
		workRecordRef,
		workRef,
		workDigest,
		testDigestValue('1'),
		basisRef,
		basisDigest,
		effectDigest,
		recordedAt,
	)
	if err != nil {
		t.Fatalf("insert profile-onboarding effect: %v", err)
	}
	_, err = fixture.database.Exec(`INSERT INTO profile_onboarding_outcome_assessments (
		outcome_assessment_ref, effect_ref, effect_digest,
		work_record_ref, work_ref, work_record_digest,
		acceptance_standard_ref, acceptance_standard_edition,
		comparator_ref, comparator_edition, verdict_kind,
		verdict_reason_ref, missing_basis_digest,
		evidence_provenance_path_refs_json, outcome_assessment_json,
		outcome_assessment_digest, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, 'acceptance:profile-onboarding', 'v1',
		'comparator:profile-onboarding', 'v1', 'passed', '', '', '[]', '{}', ?, ?)`,
		assessmentRef,
		effectRef,
		effectDigest,
		workRecordRef,
		workRef,
		workDigest,
		assessmentDigest,
		recordedAt,
	)
	if err != nil {
		t.Fatalf("insert profile-onboarding outcome assessment: %v", err)
	}
}

func insertTestRevision(t *testing.T, fixture authorityFixture, admissionID string) {
	t.Helper()
	const profilePayload = `{"scopes":["haft-software"]}`
	const receiptPayload = `{"kind":"profile-declaration"}`
	_, err := fixture.database.Exec(`INSERT INTO project_profile_revisions (
		project_root, ledger_revision, configured_profile_kind,
		profile_payload_json, profile_payload_digest,
		receipt_json, receipt_digest, admission_id, admission_digest, recorded_at
	) VALUES (?, ?, 'Declared', ?, ?, ?, ?, ?, ?, ?)`,
		fixture.envelope.projectRoot.String(),
		1,
		profilePayload,
		testDigestValue('1'),
		receiptPayload,
		testDigestValue('4'),
		admissionID,
		testDigestValue('f'),
		formatAuthorityTime(fixture.now),
	)
	if err != nil {
		t.Fatalf("insert project profile revision: %v", err)
	}
}

func authorityMutationCounts(t *testing.T, database *sql.DB) [5]int {
	t.Helper()
	tables := []string{
		"authority_uses",
		"profile_onboarding_work_records",
		"project_profile_admissions",
		"project_profile_revisions",
		"project_profile_projection_debt",
	}
	counts := [5]int{}
	for index, table := range tables {
		if err := database.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&counts[index]); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
	}
	return counts
}

func assertDeniedWithCode(t *testing.T, result Resolution, code DenialCode) {
	t.Helper()
	if result.Kind() != ResolutionNotAdmitted {
		t.Fatalf("kind = %q, want not_admitted", result.Kind())
	}
	denial, ok := result.NotAdmitted()
	if !ok {
		t.Fatal("NotAdmitted payload is missing")
	}
	for _, reason := range denial.Reasons() {
		if reason.Code() == code {
			return
		}
	}
	t.Fatalf("denial codes = %v, want %q", denial.Reasons(), code)
}

func mustParse[T any](
	t *testing.T,
	parse func(string) (T, error),
	raw string,
) T {
	t.Helper()
	value, err := parse(raw)
	if err != nil {
		t.Fatalf("parse %q: %v", raw, err)
	}
	return value
}

func mustWindow(t *testing.T, from time.Time, until time.Time) TimeWindow {
	t.Helper()
	value, err := NewTimeWindow(from, until)
	if err != nil {
		t.Fatalf("build time window: %v", err)
	}
	return value
}

func testDigest(t *testing.T, character rune) Digest {
	t.Helper()
	return mustParse(t, NewDigest, testDigestValue(character))
}

func testDigestValue(character rune) string {
	return "sha256:" + strings.Repeat(string(character), 64)
}
