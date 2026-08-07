package projecttypeenvstore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/recordmapping"
	"github.com/m0n0x41d/haft/internal/recordmembershipregistration"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestRegistrationPolicyClosureRoundTripsAndFailsClosed(t *testing.T) {
	t.Run("round trip", func(t *testing.T) {
		ctx := context.Background()
		database, store := openStoreFixture(t, ctx)
		fixture := newRegistrationPolicyRuntimeStoreFixture(t)

		err := store.PutRuntimeEvaluationBasisResolvedClosure(
			ctx,
			fixture.basis,
			nil,
			[]projecttypeenv.RegistrationPolicyArtifact{fixture.policy},
		)
		if err != nil {
			t.Fatalf("PutRuntimeEvaluationBasisResolvedClosure(): %v", err)
		}
		loaded, err := store.GetRuntimeEvaluationBasisArtifact(
			ctx,
			fixture.basis.Ref(),
		)
		if err != nil {
			t.Fatalf("GetRuntimeEvaluationBasisArtifact(): %v", err)
		}
		if err := loaded.VerifyResolvedClosure(); err != nil {
			t.Fatalf("loaded VerifyResolvedClosure(): %v", err)
		}
		if !bytes.Equal(loaded.CanonicalBytes(), fixture.basis.CanonicalBytes()) {
			t.Fatal("X canonical bytes changed across persistence")
		}
		if pins := loaded.RegistrationPolicyPins(); len(pins) != 1 ||
			pins[0].Registration() != fixture.policy.Ref() {
			t.Fatalf("loaded registration-policy pins = %#v", pins)
		}
		if got := registrationPolicyRowCount(t, ctx, database); got != 1 {
			t.Fatalf("registration-policy row count = %d, want 1", got)
		}
	})

	t.Run("missing closure writes nothing", func(t *testing.T) {
		ctx := context.Background()
		database, store := openStoreFixture(t, ctx)
		fixture := newRegistrationPolicyRuntimeStoreFixture(t)

		err := store.PutRuntimeEvaluationBasisResolvedClosure(
			ctx,
			fixture.basis,
			nil,
			nil,
		)
		if !errors.Is(err, ErrRuntimeClosureRequired) {
			t.Fatalf("missing registration-policy closure error = %v", err)
		}
		if got := artifactRowCount(t, ctx, database); got != 0 {
			t.Fatalf("artifact row count = %d, want 0", got)
		}
		if got := registrationPolicyRowCount(t, ctx, database); got != 0 {
			t.Fatalf("registration-policy row count = %d, want 0", got)
		}
	})

	t.Run("duplicate closure writes nothing", func(t *testing.T) {
		ctx := context.Background()
		database, store := openStoreFixture(t, ctx)
		fixture := newRegistrationPolicyRuntimeStoreFixture(t)

		err := store.PutRuntimeEvaluationBasisResolvedClosure(
			ctx,
			fixture.basis,
			nil,
			[]projecttypeenv.RegistrationPolicyArtifact{
				fixture.policy,
				fixture.policy,
			},
		)
		if !errors.Is(err, ErrClosureInconsistent) {
			t.Fatalf("duplicate registration-policy closure error = %v", err)
		}
		if got := artifactRowCount(t, ctx, database); got != 0 {
			t.Fatalf("artifact row count = %d, want 0", got)
		}
		if got := registrationPolicyRowCount(t, ctx, database); got != 0 {
			t.Fatalf("registration-policy row count = %d, want 0", got)
		}
	})

	corruptions := []struct {
		name   string
		column string
		value  func(registrationPolicyRuntimeStoreFixture) any
	}{
		{
			name:   "canonical bytes",
			column: "canonical_bytes",
			value: func(fixture registrationPolicyRuntimeStoreFixture) any {
				return append(fixture.policy.CanonicalBytes(), 0x00)
			},
		},
		{
			name:   "digest metadata",
			column: "artifact_digest",
			value: func(registrationPolicyRuntimeStoreFixture) any {
				return testSHA256Digest(t, strings.Repeat("f", 64)).String()
			},
		},
		{
			name:   "schema metadata",
			column: "canonical_schema_version",
			value: func(registrationPolicyRuntimeStoreFixture) any {
				return "forged.registration-policy/v9"
			},
		},
	}
	for _, corruption := range corruptions {
		t.Run("corrupt "+corruption.name, func(t *testing.T) {
			ctx := context.Background()
			database, store := openStoreFixture(t, ctx)
			fixture := newRegistrationPolicyRuntimeStoreFixture(t)
			err := store.PutRuntimeEvaluationBasisResolvedClosure(
				ctx,
				fixture.basis,
				nil,
				[]projecttypeenv.RegistrationPolicyArtifact{fixture.policy},
			)
			if err != nil {
				t.Fatalf("PutRuntimeEvaluationBasisResolvedClosure(): %v", err)
			}
			statement := `UPDATE project_typeenv_registration_policies SET ` +
				corruption.column + ` = ? WHERE registration_ref = ?`
			_, err = database.ExecContext(
				ctx,
				statement,
				corruption.value(fixture),
				fixture.policy.Ref().String(),
			)
			if err != nil {
				t.Fatalf("inject registration-policy corruption: %v", err)
			}
			_, err = store.GetRuntimeEvaluationBasisArtifact(
				ctx,
				fixture.basis.Ref(),
			)
			if !errors.Is(err, ErrArtifactIntegrity) {
				t.Fatalf("corrupt registration-policy read error = %v", err)
			}
		})
	}
}

func TestSchemaV1MigrationPreservesRowsAndAddsRegistrationPolicies(t *testing.T) {
	ctx := context.Background()
	database := openDatabaseFixture(t)
	if _, err := database.ExecContext(ctx, createSchemaVersionTable); err != nil {
		t.Fatalf("create schema version table: %v", err)
	}
	for _, statement := range schemaMigrations[0].statements {
		if _, err := database.ExecContext(ctx, statement); err != nil {
			t.Fatalf("apply v1 fixture statement: %v", err)
		}
	}
	if _, err := database.ExecContext(
		ctx,
		`INSERT INTO project_typeenv_artifact_store_schema (singleton, version)
		 VALUES (1, 1)`,
	); err != nil {
		t.Fatalf("seed v1 schema version: %v", err)
	}
	if _, err := database.ExecContext(
		ctx,
		`INSERT INTO project_typeenv_runtime_mechanisms (
			artifact_ref,
			edition,
			artifact_digest,
			canonical_schema_version,
			canonical_bytes
		) VALUES (?, ?, ?, ?, ?)`,
		"artifact:legacy-runtime-mechanism",
		"1.0.0",
		testSHA256Digest(t, strings.Repeat("e", 64)).String(),
		"haft.runtime-mechanism-artifact/v1",
		[]byte("legacy-row"),
	); err != nil {
		t.Fatalf("seed v1 runtime-mechanism row: %v", err)
	}

	if _, err := New(ctx, database); err != nil {
		t.Fatalf("New(migrate v1 to v2): %v", err)
	}
	var version int
	if err := database.QueryRowContext(
		ctx,
		`SELECT version
		 FROM project_typeenv_artifact_store_schema
		 WHERE singleton = 1`,
	).Scan(&version); err != nil {
		t.Fatalf("read migrated schema version: %v", err)
	}
	if version != CurrentSchemaVersion {
		t.Fatalf("migrated schema version = %d, want %d", version, CurrentSchemaVersion)
	}
	var legacyRows int
	if err := database.QueryRowContext(
		ctx,
		`SELECT COUNT(*)
		 FROM project_typeenv_runtime_mechanisms
		 WHERE artifact_ref = 'artifact:legacy-runtime-mechanism'`,
	).Scan(&legacyRows); err != nil {
		t.Fatalf("count preserved v1 rows: %v", err)
	}
	if legacyRows != 1 {
		t.Fatalf("preserved v1 row count = %d, want 1", legacyRows)
	}
	if got := registrationPolicyRowCount(t, ctx, database); got != 0 {
		t.Fatalf("new registration-policy row count = %d, want 0", got)
	}
}

func TestSchemaV1MigrationPreservesLegacyXAndCButRequiresRebuild(t *testing.T) {
	ctx := context.Background()
	database := openDatabaseFixture(t)
	if _, err := database.ExecContext(ctx, createSchemaVersionTable); err != nil {
		t.Fatalf("create schema version table: %v", err)
	}
	for _, statement := range schemaMigrations[0].statements {
		if _, err := database.ExecContext(ctx, statement); err != nil {
			t.Fatalf("apply v1 fixture statement: %v", err)
		}
	}
	if _, err := database.ExecContext(
		ctx,
		`INSERT INTO project_typeenv_artifact_store_schema (singleton, version)
		 VALUES (1, 1)`,
	); err != nil {
		t.Fatalf("seed v1 schema version: %v", err)
	}
	fixture := newLegacyRuntimeBasisCompositeFixture(t)
	seedLegacyProjectTypeEnvArtifact(
		t,
		ctx,
		database,
		ArtifactRuntimeBasis,
		fixture.basisRef.String(),
		fixture.basisDigest.String(),
		legacyRuntimeBasisCanonicalSchema,
		legacyRuntimeBasisCanonicalSchema,
		fixture.basisCanonical,
	)
	seedLegacyProjectTypeEnvArtifact(
		t,
		ctx,
		database,
		ArtifactCompositeTypeEnv,
		fixture.composite.Ref().String(),
		fixture.composite.Digest().String(),
		compositeArtifactCanonicalSchema,
		fixture.composite.LowererSchemaVersion(),
		fixture.composite.CanonicalBytes(),
	)

	store, err := New(ctx, database)
	if err != nil {
		t.Fatalf("New(migrate real v1 X/C): %v", err)
	}
	assertPreservedLegacyProjectTypeEnvArtifact(
		t,
		ctx,
		database,
		ArtifactRuntimeBasis,
		fixture.basisRef.String(),
		fixture.basisDigest.String(),
		legacyRuntimeBasisCanonicalSchema,
		legacyRuntimeBasisCanonicalSchema,
		fixture.basisCanonical,
	)
	assertPreservedLegacyProjectTypeEnvArtifact(
		t,
		ctx,
		database,
		ArtifactCompositeTypeEnv,
		fixture.composite.Ref().String(),
		fixture.composite.Digest().String(),
		compositeArtifactCanonicalSchema,
		fixture.composite.LowererSchemaVersion(),
		fixture.composite.CanonicalBytes(),
	)

	_, err = store.GetRuntimeEvaluationBasisArtifact(ctx, fixture.basisRef)
	if !errors.Is(err, ErrRuntimeBasisRebuildRequired) {
		t.Fatalf("legacy X read error = %v, want rebuild-required", err)
	}
	if errors.Is(err, ErrArtifactIntegrity) {
		t.Fatalf("valid legacy X was misclassified as corrupt: %v", err)
	}
	if !strings.Contains(err.Error(), "rebuild X and every bound C") {
		t.Fatalf("legacy X rebuild error lacks C repair scope: %v", err)
	}
	composite, err := store.GetProjectTypeEnvCompositeArtifact(
		ctx,
		fixture.composite.Ref(),
	)
	if err != nil {
		t.Fatalf("GetProjectTypeEnvCompositeArtifact(legacy-bound C): %v", err)
	}
	if composite.RuntimeEvaluationBasisRef() != fixture.basisRef {
		t.Fatalf(
			"preserved C binds X %q, want %q",
			composite.RuntimeEvaluationBasisRef().String(),
			fixture.basisRef.String(),
		)
	}
	if !bytes.Equal(composite.CanonicalBytes(), fixture.composite.CanonicalBytes()) {
		t.Fatal("legacy-bound C bytes changed across migration")
	}
}

type legacyRuntimeBasisCompositeFixture struct {
	basisRef       projecttypeenv.RuntimeEvaluationBasisRef
	basisDigest    typedmemory.SHA256Digest
	basisCanonical []byte
	composite      projecttypeenv.ProjectTypeEnvCompositeArtifact
}

func newLegacyRuntimeBasisCompositeFixture(
	t *testing.T,
) legacyRuntimeBasisCompositeFixture {
	t.Helper()
	basisCanonical := legacyRuntimeBasisV1CanonicalFixture()
	if !bytes.Contains(
		basisCanonical,
		[]byte(legacyRuntimeBasisCanonicalSchema),
	) {
		t.Fatal("legacy X fixture does not carry the v1 artifact domain")
	}
	if bytes.Contains(basisCanonical, []byte("registration_policy")) {
		t.Fatal("legacy X fixture unexpectedly carries registration policy")
	}
	basisDigest := canonicalFixtureDigest(t, basisCanonical)
	basisRef, err := projecttypeenv.ParseRuntimeEvaluationBasisRef(
		"runtime-evaluation-basis:" + basisDigest.String(),
	)
	if err != nil {
		t.Fatalf("ParseRuntimeEvaluationBasisRef(legacy X): %v", err)
	}
	current := newArtifactClosureFixture(t)
	compositeCanonical := bytes.Replace(
		current.composite.CanonicalBytes(),
		[]byte(current.runtime.Ref().String()),
		[]byte(basisRef.String()),
		1,
	)
	if bytes.Equal(compositeCanonical, current.composite.CanonicalBytes()) {
		t.Fatal("legacy C fixture retained the current X ref")
	}
	composite, err := projecttypeenv.DecodeProjectTypeEnvCompositeArtifact(
		compositeCanonical,
	)
	if err != nil {
		t.Fatalf("DecodeProjectTypeEnvCompositeArtifact(legacy-bound C): %v", err)
	}
	return legacyRuntimeBasisCompositeFixture{
		basisRef:       basisRef,
		basisDigest:    basisDigest,
		basisCanonical: append([]byte(nil), basisCanonical...),
		composite:      composite,
	}
}

func legacyRuntimeBasisV1CanonicalFixture() []byte {
	buffer := bytes.Buffer{}
	appendLengthPrefixedFixture(
		&buffer,
		[]byte("haft.fpf.projecttypeenv.runtime-evaluation-basis.canonical.v1"),
	)
	appendLengthPrefixedFixture(
		&buffer,
		[]byte(legacyRuntimeBasisCanonicalSchema),
	)
	appendLengthPrefixedFixture(&buffer, []byte(`{"pins":[]}`))
	return append([]byte(nil), buffer.Bytes()...)
}

func appendLengthPrefixedFixture(buffer *bytes.Buffer, value []byte) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	buffer.Write(length[:])
	buffer.Write(value)
}

func canonicalFixtureDigest(
	t *testing.T,
	canonical []byte,
) typedmemory.SHA256Digest {
	t.Helper()
	sum := sha256.Sum256(canonical)
	hexDigest := hex.EncodeToString(sum[:])
	return testSHA256Digest(t, hexDigest)
}

func seedLegacyProjectTypeEnvArtifact(
	t *testing.T,
	ctx context.Context,
	database *sql.DB,
	kind ArtifactKind,
	ref string,
	digest string,
	canonicalSchema string,
	producerSchema string,
	canonical []byte,
) {
	t.Helper()
	_, err := database.ExecContext(
		ctx,
		`INSERT INTO project_typeenv_artifacts (
			artifact_kind,
			artifact_ref,
			artifact_digest,
			canonical_schema_version,
			producer_schema_version,
			canonical_bytes
		) VALUES (?, ?, ?, ?, ?, ?)`,
		string(kind),
		ref,
		digest,
		canonicalSchema,
		producerSchema,
		canonical,
	)
	if err != nil {
		t.Fatalf("seed legacy %s %q: %v", kind, ref, err)
	}
}

func assertPreservedLegacyProjectTypeEnvArtifact(
	t *testing.T,
	ctx context.Context,
	database *sql.DB,
	kind ArtifactKind,
	ref string,
	wantDigest string,
	wantCanonicalSchema string,
	wantProducerSchema string,
	wantCanonical []byte,
) {
	t.Helper()
	var gotDigest string
	var gotCanonicalSchema string
	var gotProducerSchema string
	var gotCanonical []byte
	err := database.QueryRowContext(
		ctx,
		`SELECT
			artifact_digest,
			canonical_schema_version,
			producer_schema_version,
			canonical_bytes
		 FROM project_typeenv_artifacts
		 WHERE artifact_kind = ? AND artifact_ref = ?`,
		string(kind),
		ref,
	).Scan(
		&gotDigest,
		&gotCanonicalSchema,
		&gotProducerSchema,
		&gotCanonical,
	)
	if err != nil {
		t.Fatalf("read preserved legacy %s %q: %v", kind, ref, err)
	}
	if gotDigest != wantDigest ||
		gotCanonicalSchema != wantCanonicalSchema ||
		gotProducerSchema != wantProducerSchema ||
		!bytes.Equal(gotCanonical, wantCanonical) {
		t.Fatalf("legacy %s %q metadata or bytes changed during migration", kind, ref)
	}
}

type registrationPolicyRuntimeStoreFixture struct {
	policy projecttypeenv.RegistrationPolicyArtifact
	basis  projecttypeenv.RuntimeEvaluationBasisArtifact
}

func newRegistrationPolicyRuntimeStoreFixture(
	t *testing.T,
) registrationPolicyRuntimeStoreFixture {
	t.Helper()
	evaluator := registrationPolicyStoreMechanismFixture(
		t,
		recordmembershipregistration.EvaluatorMechanism,
		"haft.member-of.project-record-carrier/v1",
		"haft.runtime.record-membership-evaluator",
		"a1",
	)
	delivery := registrationPolicyStoreMechanismFixture(
		t,
		recordmembershipregistration.SourceDeliveryBoundaryMechanism,
		"haft.deliver.project-record-membership/v1",
		"haft.runtime.record-membership-delivery",
		"a2",
	)
	manifest, err := recordmapping.NewMappingManifestRef(
		"mapping.project-record",
		"1.0.0",
		testSHA256Digest(t, strings.Repeat("b1", 32)),
	)
	if err != nil {
		t.Fatalf("recordmapping.NewMappingManifestRef(): %v", err)
	}
	adapter, err := recordmapping.NewAdapterVersion(
		"haft-record-adapter/1.0.0",
	)
	if err != nil {
		t.Fatalf("recordmapping.NewAdapterVersion(): %v", err)
	}
	mapping, err := recordmembershipregistration.NewAcceptedMapping(
		recordmembershipregistration.AcceptedMappingInput{
			Manifest: manifest,
			Adapter:  adapter,
		},
	)
	if err != nil {
		t.Fatalf("recordmembershipregistration.NewAcceptedMapping(): %v", err)
	}
	policy, err := recordmembershipregistration.SealRegistrationArtifactV1(
		recordmembershipregistration.RegistrationArtifactInputV1{
			Evaluator:      evaluator,
			SourceDelivery: delivery,
			Mappings: []recordmembershipregistration.AcceptedMapping{
				mapping,
			},
		},
	)
	if err != nil {
		t.Fatalf("SealRegistrationArtifactV1(): %v", err)
	}
	pin, err := projecttypeenv.NewRegistrationPolicyPin(policy)
	if err != nil {
		t.Fatalf("projecttypeenv.NewRegistrationPolicyPin(): %v", err)
	}
	basis, err := projecttypeenv.SealRuntimeEvaluationBasisWithPins(
		[]projecttypeenv.RuntimeEvaluationBasisPin{pin},
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("projecttypeenv.SealRuntimeEvaluationBasisWithPins(): %v", err)
	}
	return registrationPolicyRuntimeStoreFixture{
		policy: policy,
		basis:  basis,
	}
}

func registrationPolicyStoreMechanismFixture(
	t *testing.T,
	role recordmembershipregistration.MechanismRole,
	ruleRaw string,
	artifactRaw string,
	digestText string,
) recordmembershipregistration.MechanismCoordinate {
	t.Helper()
	rule, err := typedmemory.NewRuleRef(ruleRaw)
	if err != nil {
		t.Fatalf("typedmemory.NewRuleRef(): %v", err)
	}
	artifact, err := typedmemory.NewCarrierRef(artifactRaw)
	if err != nil {
		t.Fatalf("typedmemory.NewCarrierRef(): %v", err)
	}
	edition, err := typedmemory.NewCarrierEdition("1.0.0")
	if err != nil {
		t.Fatalf("typedmemory.NewCarrierEdition(): %v", err)
	}
	coordinate, err := recordmembershipregistration.NewMechanismCoordinate(
		recordmembershipregistration.MechanismCoordinateInput{
			Role:     role,
			Rule:     rule,
			Artifact: artifact,
			Edition:  edition,
			Digest:   testSHA256Digest(t, strings.Repeat(digestText, 32)),
		},
	)
	if err != nil {
		t.Fatalf("recordmembershipregistration.NewMechanismCoordinate(): %v", err)
	}
	return coordinate
}

func registrationPolicyRowCount(
	t *testing.T,
	ctx context.Context,
	database *sql.DB,
) int {
	t.Helper()
	var count int
	if err := database.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM project_typeenv_registration_policies`,
	).Scan(&count); err != nil {
		t.Fatalf("count registration-policy artifacts: %v", err)
	}
	return count
}
