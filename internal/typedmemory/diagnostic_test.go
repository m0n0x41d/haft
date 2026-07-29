package typedmemory

import "testing"

func TestDiagnosticSeparatesContradictionFromMissingBasis(t *testing.T) {
	path, err := NewDiagnosticPath("changes[0].slots[EntityOfConcernSlot]")
	if err != nil {
		t.Fatalf("diagnostic path: %v", err)
	}
	rule, err := NewRuleRef("typeenv:fixture/rule/entity-of-concern-slot")
	if err != nil {
		t.Fatalf("rule ref: %v", err)
	}
	repair, err := NewRepairPointer("haft memory inspect --signature fixture")
	if err != nil {
		t.Fatalf("repair pointer: %v", err)
	}

	invalidWitness, _ := NewExpectedActualWitness(
		diagnosticState("by_reference"),
		diagnosticState("by_value"),
	)
	invalidBasis, _ := NewSnapshotRuleBasis(rule)
	invalidRepair, _ := NewRepairCandidate(
		RepairChangeInput,
		repair,
		diagnosticState("by_reference"),
		HumanChoiceNotClaimed,
	)
	invalid, err := NewInvalidDiagnosticWithDetails(
		DiagnosticReferenceModeMismatch,
		"known slot requires a reference filler",
		path,
		invalidWitness,
		invalidBasis,
		[]RepairCandidate{invalidRepair},
	)
	if err != nil {
		t.Fatalf("invalid diagnostic: %v", err)
	}
	if invalid.Posture() != DiagnosticInvalid {
		t.Fatalf("invalid posture = %q", invalid.Posture())
	}
	if _, ok := invalid.Repair(); ok {
		t.Fatal("known contradiction unexpectedly carries a missing-basis repair")
	}
	if len(invalid.RepairCandidates()) != 1 {
		t.Fatal("known contradiction did not retain a separate remediation candidate")
	}

	required := diagnosticState("active relation signature")
	missingWitness, _ := NewMissingBasisWitness(required, "signature is absent")
	missingBasis, _ := NewMissingRuntimeBasis(MissingRuntimeDeclaration, required)
	missingRepair, _ := NewRepairCandidate(
		RepairExtendTypeEnv,
		repair,
		required,
		HumanChoiceRequired,
	)
	underdetermined, err := NewUnderdeterminedDiagnosticWithDetails(
		DiagnosticSignatureNotActive,
		"relation signature is absent from the active TypeEnv",
		path,
		missingWitness,
		missingBasis,
		repair,
		[]RepairCandidate{missingRepair},
	)
	if err != nil {
		t.Fatalf("underdetermined diagnostic: %v", err)
	}
	if underdetermined.Posture() != DiagnosticUnderdetermined {
		t.Fatalf("underdetermined posture = %q", underdetermined.Posture())
	}
	if _, ok := underdetermined.Rule(); ok {
		t.Fatal("missing basis unexpectedly claims a known violated rule")
	}
}

func TestStructuredDiagnosticRetainsWitnessBasisAndHumanChoiceMarker(t *testing.T) {
	path, _ := NewDiagnosticPath("changes[0].signature")
	expected := diagnosticReference("typeenv:fixture/signature/U.EpistemeSlotRelation")
	actual := diagnosticState("absent")
	witness, err := NewExpectedActualWitness(expected, actual)
	if err != nil {
		t.Fatalf("NewExpectedActualWitness: %v", err)
	}
	rule, _ := NewRuleRef("typedmemory.validation-core.v1")
	basis, err := NewCoreValidatorBasis(rule)
	if err != nil {
		t.Fatalf("NewCoreValidatorBasis: %v", err)
	}
	pointer, _ := NewRepairPointer("choose-or-stage-the-required-signature")
	repair, err := NewRepairCandidate(
		RepairExtendTypeEnv,
		pointer,
		expected,
		HumanChoiceRequired,
	)
	if err != nil {
		t.Fatalf("NewRepairCandidate: %v", err)
	}
	diagnostic, err := NewInvalidDiagnosticWithDetails(
		DiagnosticSignatureContextMismatch,
		"signature does not admit the requested context",
		path,
		witness,
		basis,
		[]RepairCandidate{repair},
	)
	if err != nil {
		t.Fatalf("NewInvalidDiagnosticWithDetails: %v", err)
	}

	retained := diagnostic.Witness()
	if retained.Expected().Values()[0] != expected.Values()[0] {
		t.Fatal("structured expected value was not retained")
	}
	if retained.Actual().Kind() != DiagnosticDatumState {
		t.Fatalf("actual kind = %q", retained.Actual().Kind())
	}
	if diagnostic.GoverningBasis().Kind() != DiagnosticBasisCoreValidator {
		t.Fatalf("basis kind = %q", diagnostic.GoverningBasis().Kind())
	}
	repairs := diagnostic.RepairCandidates()
	if repairs[0].HumanChoiceRequirement() != HumanChoiceRequired {
		t.Fatalf("human-choice marker = %q", repairs[0].HumanChoiceRequirement())
	}
	repairs[0] = RepairCandidate{}
	if !diagnostic.RepairCandidates()[0].valid() {
		t.Fatal("caller mutation changed retained repair candidates")
	}
}

func TestDiagnosticPostureRejectsWrongGoverningBasisVariant(t *testing.T) {
	path, _ := NewDiagnosticPath("changes[0]")
	expected := diagnosticState("active declaration")
	actual := diagnosticState("absent")
	knownWitness, _ := NewExpectedActualWitness(expected, actual)
	missingWitness, _ := NewMissingBasisWitness(expected, "declaration was not compiled")
	rule, _ := NewRuleRef("typedmemory.validation-core.v1")
	knownBasis, _ := NewCoreValidatorBasis(rule)
	missingBasis, _ := NewMissingRuntimeBasis(MissingRuntimeDeclaration, expected)
	pointer, _ := NewRepairPointer("inspect-the-declaration")
	repair, _ := NewRepairCandidate(
		RepairInspectBasis,
		pointer,
		expected,
		HumanChoiceNotClaimed,
	)

	if _, err := NewInvalidDiagnosticWithDetails(
		DiagnosticMalformedValue,
		"known contradiction",
		path,
		knownWitness,
		missingBasis,
		[]RepairCandidate{repair},
	); err == nil {
		t.Fatal("Invalid accepted a missing governing basis")
	}
	if _, err := NewUnderdeterminedDiagnosticWithDetails(
		DiagnosticTypeRuleUnavailable,
		"missing basis",
		path,
		missingWitness,
		knownBasis,
		pointer,
		[]RepairCandidate{repair},
	); err == nil {
		t.Fatal("Underdetermined accepted a known governing basis")
	}
}

func TestKnownDeclarationBasisRetainsExactFPFSourceProvenance(t *testing.T) {
	provenance := typeEnvTestFPFProvenance(t, "fpf:rule/A.6.5", 0x5a)
	basis, err := NewKnownDeclarationBasis(provenance)
	if err != nil {
		t.Fatalf("NewKnownDeclarationBasis: %v", err)
	}
	path, _ := NewDiagnosticPath("changes[0].slots.EntityOfConcernSlot")
	expected := diagnosticState("by_reference")
	actual := diagnosticState("by_value")
	witness, _ := NewExpectedActualWitness(expected, actual)
	pointer, _ := NewRepairPointer("change-the-slot-filler-mode")
	repair, _ := NewRepairCandidate(
		RepairChangeInput,
		pointer,
		expected,
		HumanChoiceNotClaimed,
	)
	diagnostic, err := NewInvalidDiagnosticWithDetails(
		DiagnosticReferenceModeMismatch,
		"slot filler mode contradicts the compiled SlotSpec",
		path,
		witness,
		basis,
		[]RepairCandidate{repair},
	)
	if err != nil {
		t.Fatalf("NewInvalidDiagnosticWithDetails: %v", err)
	}

	retained, ok := diagnostic.GoverningBasis().(KnownDeclarationBasis)
	if !ok {
		t.Fatalf("basis = %T", diagnostic.GoverningBasis())
	}
	fpf, ok := retained.Provenance().(FPFSourceProvenance)
	if !ok {
		t.Fatalf("provenance = %T", retained.Provenance())
	}
	if fpf.Location().ContentHash() != provenance.Location().ContentHash() ||
		fpf.Location().LineRange() != provenance.Location().LineRange() ||
		fpf.CompilerRuleID() != provenance.CompilerRuleID() {
		t.Fatal("exact FPF source provenance changed through diagnostic projection")
	}
}

func TestCodecIssueProjectionRetainsStructuredWitnessAndCoreBasis(t *testing.T) {
	path, _ := NewDiagnosticPath("claim_graph.nodes.node-1")
	expected := diagnosticState("unique ClaimNodeID")
	actual := diagnosticReference("node-1")
	witness, _ := NewExpectedActualWitness(expected, actual)
	pointer, _ := NewRepairPointer("change-candidate-at:claim_graph.nodes.node-1")
	repair, _ := NewRepairCandidate(
		RepairChangeInput,
		pointer,
		expected,
		HumanChoiceNotClaimed,
	)
	issue, err := NewCodecIssueWithDetails(
		DiagnosticClaimGraphDuplicateNode,
		"duplicate claim node",
		path,
		witness,
		[]RepairCandidate{repair},
	)
	if err != nil {
		t.Fatalf("NewCodecIssueWithDetails: %v", err)
	}
	diagnostics := diagnosticsFromCodecIssues([]CodecIssue{issue})
	if len(diagnostics) != 1 {
		t.Fatalf("diagnostics = %d", len(diagnostics))
	}
	projected := diagnostics[0]
	if projected.Witness().Actual().Values()[0] != "node-1" {
		t.Fatal("codec issue projection lost its actual witness")
	}
	if projected.GoverningBasis().Kind() != DiagnosticBasisCoreValidator {
		t.Fatalf("basis kind = %q", projected.GoverningBasis().Kind())
	}
}

func TestMissingBasisRepairDispositionUsesTypedBasisNotOnlyDiagnosticCode(t *testing.T) {
	required := diagnosticState("exact active basis")
	pointer, _ := NewRepairPointer("recover-the-basis")
	snapshotBasis, _ := NewMissingRuntimeBasis(MissingRuntimeSnapshot, required)
	snapshotRepair := defaultMissingBasisRepair(
		DiagnosticTypeRuleUnavailable,
		snapshotBasis,
		pointer,
		required,
	)
	if snapshotRepair.Kind() != RepairRefreshSnapshot ||
		snapshotRepair.HumanChoiceRequirement() != HumanChoiceNotClaimed {
		t.Fatalf(
			"snapshot repair = %q/%q",
			snapshotRepair.Kind(),
			snapshotRepair.HumanChoiceRequirement(),
		)
	}

	typeEnv, err := NewTypeEnvRef(typeEnvTestDigest(t, 0x6b))
	if err != nil {
		t.Fatalf("NewTypeEnvRef: %v", err)
	}
	typeEnvBasis, _ := NewMissingTypeEnvDeclarationBasis(typeEnv, required)
	typeEnvRepair := defaultMissingBasisRepair(
		DiagnosticTypeRuleUnavailable,
		typeEnvBasis,
		pointer,
		required,
	)
	if typeEnvRepair.Kind() != RepairExtendTypeEnv ||
		typeEnvRepair.HumanChoiceRequirement() != HumanChoiceRequired {
		t.Fatalf(
			"TypeEnv repair = %q/%q",
			typeEnvRepair.Kind(),
			typeEnvRepair.HumanChoiceRequirement(),
		)
	}
}

func TestSourceOnlyMissingDeclarationBasisRetainsExactCoverageEvidence(t *testing.T) {
	typeEnv, err := NewTypeEnvRef(typeEnvTestDigest(t, 0x7c))
	if err != nil {
		t.Fatalf("NewTypeEnvRef: %v", err)
	}
	unit := typeEnvTestSourceUnitID(t, "pattern-section:C.2.1")
	subject, err := SourceUnitCoverage(unit)
	if err != nil {
		t.Fatalf("SourceUnitCoverage: %v", err)
	}
	coverage, err := NewSourceOnlyCoverageEntry(
		subject,
		typeEnvTestSourceLocation(t, 0x7d),
		"relation profile is structurally incomplete",
	)
	if err != nil {
		t.Fatalf("NewSourceOnlyCoverageEntry: %v", err)
	}
	missing, err := NewSourceOnlyTypeEnvDeclarationBasis(
		typeEnv,
		diagnosticReference("U.EpistemeSlotRelation"),
		coverage,
	)
	if err != nil {
		t.Fatalf("NewSourceOnlyTypeEnvDeclarationBasis: %v", err)
	}
	retained, ok := missing.Coverage()
	if !ok {
		t.Fatal("source-only basis lost coverage evidence")
	}
	if retained.Posture() != CoverageSourceOnly ||
		retained.Source().ContentHash() != coverage.Source().ContentHash() {
		t.Fatal("source-only basis changed exact coverage provenance")
	}
}

func TestValidVerdictRejectsZeroInputsAndInvalidSemanticChange(t *testing.T) {
	if _, err := newValidVerdict(
		MemoryChangeSet{},
		ValidatedMemoryChangeSet{},
		nil,
	); err == nil {
		t.Fatal("newValidVerdict accepted zero ValidatedMemoryChangeSet")
	}
	if (AdmissionBatch{}).IsValid() {
		t.Fatal("zero AdmissionBatch is admissible")
	}
	fixture := newValidationFixture(t)
	if _, err := newValidVerdict(
		fixture.changeSet,
		newValidatedMemoryChangeSet(
			[]ValidatedMemoryChange{ValidatedDeclareEntity{}},
		),
		nil,
	); err == nil {
		t.Fatal("newValidVerdict accepted an invalid validated-change variant")
	}
}

func TestValidVerdictCarriesOnlySealedAdmissionBatch(t *testing.T) {
	fixture := newValidationFixture(t)
	verdict := ValidateMemoryChangeSet(
		fixture.environment,
		fixture.registry,
		fixture.snapshot,
		fixture.changeSet,
	)
	valid, ok := verdict.(Valid)
	if !ok {
		t.Fatalf("verdict = %T; want Valid", verdict)
	}
	batch := valid.AdmissionBatch()
	if !batch.IsValid() {
		t.Fatal("successful validation did not produce an admissible batch")
	}
	if len(batch.ChangeSet().Changes()) != len(valid.ChangeSet().Changes()) {
		t.Fatal("admission batch changed the validated effect set")
	}
	basis := batch.Basis()
	if basis == nil ||
		basis.TypeEnv() != fixture.environment.Ref() ||
		basis.GraphRevision() != fixture.snapshot.GraphRevision() {
		t.Fatal("admission batch did not seal the exact validation basis")
	}
	requestDigest, err := fixture.changeSet.Digest()
	if err != nil {
		t.Fatalf("candidate digest: %v", err)
	}
	if batch.RequestDigest() != requestDigest ||
		batch.SemanticChangeDigest() != valid.SemanticChangeDigest() ||
		len(batch.CanonicalEnvelopeBytes()) == 0 ||
		!batch.AdmissionEnvelopeDigest().valid() {
		t.Fatal("admission batch did not retain its distinct request, semantic, and envelope identities")
	}
	if batch.AdmissionEnvelopeDigest() == batch.SemanticChangeDigest() ||
		batch.AdmissionEnvelopeDigest() == batch.RequestDigest() {
		t.Fatal("admission envelope digest collapsed into a request or semantic digest")
	}
	const expectedEnvelopeDigest = "sha256:8ad49a3edd2d1f2f24cfdb931a780c98ffaa517642db39fc9409e6d78b44b0c7"
	if batch.AdmissionEnvelopeDigest().String() != expectedEnvelopeDigest {
		t.Fatalf(
			"admission envelope digest = %s; want %s",
			batch.AdmissionEnvelopeDigest().String(),
			expectedEnvelopeDigest,
		)
	}
	retained := batch.CanonicalEnvelopeBytes()
	retained[0] ^= 0xff
	if string(retained) == string(batch.CanonicalEnvelopeBytes()) {
		t.Fatal("caller mutation changed retained canonical admission-envelope bytes")
	}
}

func TestAdmissionBatchRejectsTamperedRequestSemanticBasisAndEnvelope(t *testing.T) {
	fixture := newValidationFixture(t)
	verdict := ValidateMemoryChangeSet(
		fixture.environment,
		fixture.registry,
		fixture.snapshot,
		fixture.changeSet,
	)
	valid, ok := verdict.(Valid)
	if !ok {
		t.Fatalf("verdict = %T; want Valid", verdict)
	}
	original := valid.AdmissionBatch()

	cases := map[string]func(AdmissionBatch) AdmissionBatch{
		"candidate": func(batch AdmissionBatch) AdmissionBatch {
			batch.candidate = MemoryChangeSet{}
			return batch
		},
		"semantic change set": func(batch AdmissionBatch) AdmissionBatch {
			batch.changeSet = ValidatedMemoryChangeSet{}
			return batch
		},
		"basis": func(batch AdmissionBatch) AdmissionBatch {
			batch.basis = nil
			return batch
		},
		"request digest": func(batch AdmissionBatch) AdmissionBatch {
			batch.requestDigest = SHA256Digest{}
			return batch
		},
		"semantic digest": func(batch AdmissionBatch) AdmissionBatch {
			batch.semanticChangeDigest = SHA256Digest{}
			return batch
		},
		"canonical envelope": func(batch AdmissionBatch) AdmissionBatch {
			batch.canonicalEnvelope = append([]byte(nil), batch.canonicalEnvelope...)
			batch.canonicalEnvelope[0] ^= 0xff
			return batch
		},
		"envelope digest": func(batch AdmissionBatch) AdmissionBatch {
			batch.admissionEnvelopeDigest = SHA256Digest{}
			return batch
		},
	}
	for name, tamper := range cases {
		t.Run(name, func(t *testing.T) {
			if tamper(original).IsValid() {
				t.Fatal("tampered AdmissionBatch remained valid")
			}
		})
	}
}

func TestValidVerdictRejectsStructurallyUncorrelatedCandidate(t *testing.T) {
	fixture := newValidationFixture(t)
	verdict := ValidateMemoryChangeSet(
		fixture.environment,
		fixture.registry,
		fixture.snapshot,
		fixture.changeSet,
	)
	valid, ok := verdict.(Valid)
	if !ok {
		t.Fatalf("verdict = %T; want Valid", verdict)
	}
	batch := valid.AdmissionBatch()
	mismatched := copyMemoryChangeSet(fixture.changeSet)
	change, ok := mismatched.changes[0].(InstantiateRelation)
	if !ok {
		t.Fatalf("fixture change = %T; want InstantiateRelation", mismatched.changes[0])
	}
	assertion, err := NewAssertionID("assertion:uncorrelated-admission-envelope")
	if err != nil {
		t.Fatalf("NewAssertionID: %v", err)
	}
	relation := change.relation
	relation.assertion = assertion
	mismatched.changes[0] = InstantiateRelation{relation: relation}
	if !mismatched.valid() {
		t.Fatal("mismatch fixture is not a strong candidate")
	}
	if _, err := newValidVerdict(mismatched, batch.changeSet, batch.basis); err == nil {
		t.Fatal("newValidVerdict accepted a candidate unrelated to the sealed semantic change and basis")
	}
}
