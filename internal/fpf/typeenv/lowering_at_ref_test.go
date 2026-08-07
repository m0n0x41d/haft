package typeenv

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestLowerBaseTypeEnvArtifactWithCodecsAtRefBindsEveryRuntimeReference(
	t *testing.T,
) {
	t.Parallel()

	target := baseLoweringTargetRef(t, "c")
	artifacts := []struct {
		name     string
		artifact BaseTypeEnvArtifact
	}{
		{
			name:     "pinned base and ClaimGraph representation",
			artifact: claimGraphTestCompiledArtifact(t),
		},
		{
			name:     "closed relation with value and reference slots",
			artifact: baseLoweringRelationArtifact(t),
		},
	}
	for _, fixture := range artifacts {
		fixture := fixture
		t.Run(fixture.name, func(t *testing.T) {
			environment, _, err := LowerBaseTypeEnvArtifactWithCodecsAtRef(
				fixture.artifact,
				target,
			)
			if err != nil {
				t.Fatalf("LowerBaseTypeEnvArtifactWithCodecsAtRef: %v", err)
			}
			assertBaseLoweringRuntimeRefs(t, environment, target)
		})
	}
}

func TestLowerBaseTypeEnvArtifactWithCodecsAtRefPreservesArtifactIdentity(
	t *testing.T,
) {
	t.Parallel()

	artifact := claimGraphTestCompiledArtifact(t)
	beforeCanonical := artifact.CanonicalBytes()
	beforeDigest := artifact.Digest()
	beforeRef, beforeHasRef := artifact.TypeEnvRef()
	target := baseLoweringTargetRef(t, "d")

	environment, _, err := LowerBaseTypeEnvArtifactWithCodecsAtRef(artifact, target)
	if err != nil {
		t.Fatalf("LowerBaseTypeEnvArtifactWithCodecsAtRef: %v", err)
	}
	if environment.Ref() != target {
		t.Fatalf("environment ref = %q, want target %q", environment.Ref().String(), target.String())
	}
	if err := artifact.Verify(); err != nil {
		t.Fatalf("base artifact no longer verifies: %v", err)
	}
	afterRef, afterHasRef := artifact.TypeEnvRef()
	if afterHasRef != beforeHasRef || afterRef != beforeRef {
		t.Fatal("base artifact TypeEnv identity changed during target lowering")
	}
	if artifact.Digest() != beforeDigest {
		t.Fatal("base artifact digest changed during target lowering")
	}
	if !bytes.Equal(artifact.CanonicalBytes(), beforeCanonical) {
		t.Fatal("base artifact canonical bytes changed during target lowering")
	}
}

func TestLowerBaseTypeEnvArtifactWithCodecsAtRefIsMechanismNotCompositeProof(
	t *testing.T,
) {
	t.Parallel()

	artifact := claimGraphTestCompiledArtifact(t)
	baseRef, exists := artifact.TypeEnvRef()
	if !exists {
		t.Fatal("compiled base artifact has no TypeEnvRef")
	}
	// This target is intentionally only syntactically valid. It was not derived
	// from a base-plus-extension composite and carries no activation authority.
	unauthenticatedTarget := baseLoweringTargetRef(t, "a")
	if unauthenticatedTarget == baseRef {
		t.Fatal("fixture target unexpectedly equals the base artifact identity")
	}
	environment, _, err := LowerBaseTypeEnvArtifactWithCodecsAtRef(
		artifact,
		unauthenticatedTarget,
	)
	if err != nil {
		t.Fatalf("mechanism-only lowering rejected a valid target: %v", err)
	}
	if environment.Ref() != unauthenticatedTarget {
		t.Fatal("mechanism-only lowering did not bind the requested target")
	}
	artifactRef, exists := artifact.TypeEnvRef()
	if !exists || artifactRef != baseRef {
		t.Fatal("mechanism-only lowering relabelled the immutable base artifact")
	}
}

func TestLowerBaseTypeEnvArtifactWithCodecsAtRefRetainsClaimGraphCodec(
	t *testing.T,
) {
	t.Parallel()

	artifact := claimGraphTestCompiledArtifact(t)
	target := baseLoweringTargetRef(t, "e")
	environment, registry, err := LowerBaseTypeEnvArtifactWithCodecsAtRef(
		artifact,
		target,
	)
	if err != nil {
		t.Fatalf("LowerBaseTypeEnvArtifactWithCodecsAtRef: %v", err)
	}
	if len(environment.ValueBindings()) != 1 {
		t.Fatalf("value binding count = %d, want 1", len(environment.ValueBindings()))
	}
	binding := environment.ValueBindings()[0]
	if binding.ValueKind().TypeEnv() != target {
		t.Fatalf(
			"ClaimGraph ValueKind TypeEnv = %q, want %q",
			binding.ValueKind().TypeEnv().String(),
			target.String(),
		)
	}
	if !registry.Contains(binding.Codec()) {
		t.Fatal("target-lowered environment has no registered ClaimGraph codec")
	}
	assertClaimGraphCodecRoundTrip(t, environment, registry)
}

func TestLowerBaseTypeEnvArtifactWithCodecsAtRefVerifiesArtifactBeforeTarget(
	t *testing.T,
) {
	t.Parallel()

	zeroTarget := typedmemory.TypeEnvRef{}
	validArtifact := claimGraphTestCompiledArtifact(t)
	tamperedArtifact := validArtifact
	tamperedArtifact.canonical = append([]byte(nil), tamperedArtifact.canonical...)
	tamperedArtifact.canonical[0] ^= 0xff
	refTamperedArtifact := validArtifact
	refTamperedArtifact.ref = baseLoweringTargetRef(t, "b")
	coverageOnly := baseLoweringCoverageOnlyArtifact(t)

	tests := []struct {
		name      string
		artifact  BaseTypeEnvArtifact
		wantError string
		notError  string
	}{
		{
			name:      "zero artifact precedes zero target",
			artifact:  BaseTypeEnvArtifact{},
			wantError: "artifact",
			notError:  "target TypeEnv",
		},
		{
			name:      "tampered artifact precedes zero target",
			artifact:  tamperedArtifact,
			wantError: "canonical bytes do not match",
			notError:  "target TypeEnv",
		},
		{
			name:      "tampered artifact identity precedes zero target",
			artifact:  refTamperedArtifact,
			wantError: "artifact TypeEnvRef does not match canonical bytes",
			notError:  "target TypeEnv",
		},
		{
			name:      "coverage-only posture precedes zero target",
			artifact:  coverageOnly,
			wantError: "coverage-only artifact has no runtime TypeEnv identity",
			notError:  "target TypeEnv",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			_, _, err := LowerBaseTypeEnvArtifactWithCodecsAtRef(
				test.artifact,
				zeroTarget,
			)
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("error = %v, want %q", err, test.wantError)
			}
			if strings.Contains(err.Error(), test.notError) {
				t.Fatalf("error = %v, artifact-first check was bypassed", err)
			}
		})
	}

	_, _, err := LowerBaseTypeEnvArtifactWithCodecsAtRef(validArtifact, zeroTarget)
	if err == nil || !strings.Contains(err.Error(), "target TypeEnv reference is required") {
		t.Fatalf("zero target error = %v", err)
	}
}

func TestLowerBaseTypeEnvArtifactWithCodecsKeepsBaseIdentityBehavior(
	t *testing.T,
) {
	t.Parallel()

	artifact := claimGraphTestCompiledArtifact(t)
	baseRef, exists := artifact.TypeEnvRef()
	if !exists {
		t.Fatal("compiled base artifact has no TypeEnvRef")
	}
	legacyEnvironment, legacyRegistry, err := LowerBaseTypeEnvArtifactWithCodecs(artifact)
	if err != nil {
		t.Fatalf("LowerBaseTypeEnvArtifactWithCodecs: %v", err)
	}
	explicitEnvironment, explicitRegistry, err := LowerBaseTypeEnvArtifactWithCodecsAtRef(
		artifact,
		baseRef,
	)
	if err != nil {
		t.Fatalf("LowerBaseTypeEnvArtifactWithCodecsAtRef(base): %v", err)
	}
	withoutCodecs, err := LowerBaseTypeEnvArtifact(artifact)
	if err != nil {
		t.Fatalf("LowerBaseTypeEnvArtifact: %v", err)
	}
	if legacyEnvironment.Ref() != baseRef || withoutCodecs.Ref() != baseRef {
		t.Fatal("legacy lowering no longer uses artifact-derived base identity")
	}
	if !reflect.DeepEqual(legacyEnvironment, explicitEnvironment) ||
		!reflect.DeepEqual(legacyEnvironment, withoutCodecs) {
		t.Fatal("legacy base lowering differs from common verified lowerer")
	}
	if !reflect.DeepEqual(legacyRegistry, explicitRegistry) {
		t.Fatal("legacy base codec registry differs from common verified lowerer")
	}
}

func TestLowerBaseTypeEnvArtifactWithCodecsAtRefIsDeterministic(t *testing.T) {
	t.Parallel()

	artifact := claimGraphTestCompiledArtifact(t)
	target := baseLoweringTargetRef(t, "f")
	firstEnvironment, firstRegistry, err := LowerBaseTypeEnvArtifactWithCodecsAtRef(
		artifact,
		target,
	)
	if err != nil {
		t.Fatalf("first lowering: %v", err)
	}
	secondEnvironment, secondRegistry, err := LowerBaseTypeEnvArtifactWithCodecsAtRef(
		artifact,
		target,
	)
	if err != nil {
		t.Fatalf("second lowering: %v", err)
	}
	if !reflect.DeepEqual(firstEnvironment, secondEnvironment) {
		t.Fatal("repeated target lowering produced different TypeEnvs")
	}
	if !reflect.DeepEqual(firstRegistry, secondRegistry) {
		t.Fatal("repeated target lowering produced different codec registries")
	}
}

func assertBaseLoweringRuntimeRefs(
	t *testing.T,
	environment typedmemory.TypeEnv,
	target typedmemory.TypeEnvRef,
) {
	t.Helper()
	if environment.Ref() != target {
		t.Fatalf("environment ref = %q, want %q", environment.Ref().String(), target.String())
	}
	for _, definition := range environment.EntitySetDefinitions() {
		if definition.Ref().TypeEnv() != target {
			t.Fatalf("EntitySet %q is not target-bound", definition.Ref().String())
		}
	}
	for _, definition := range environment.KindSignatureDefinitions() {
		if definition.Ref().TypeEnv() != target ||
			definition.ValueKind().TypeEnv() != target ||
			definition.EntitySet().TypeEnv() != target {
			t.Fatalf("KindSignature %q is not target-bound", definition.Ref().String())
		}
	}
	for _, definition := range environment.RefKindDefinitions() {
		if definition.Ref().TypeEnv() != target || definition.ValueKind().TypeEnv() != target {
			t.Fatalf("RefKind %q is not target-bound", definition.Ref().String())
		}
	}
	for _, signature := range environment.RelationSignatures() {
		if signature.Ref().TypeEnv() != target {
			t.Fatalf("signature %q is not target-bound", signature.Ref().String())
		}
		for _, slot := range signature.Slots() {
			switch slotTarget := slot.Target().(type) {
			case typedmemory.ValueSlotTarget:
				if slotTarget.ValueKind().TypeEnv() != target {
					t.Fatalf("value slot %q is not target-bound", slot.SlotKind().String())
				}
			case typedmemory.ReferenceSlotTarget:
				if slotTarget.ValueKind().TypeEnv() != target ||
					slotTarget.ReferenceKind().TypeEnv() != target {
					t.Fatalf("reference slot %q is not target-bound", slot.SlotKind().String())
				}
			default:
				t.Fatalf("slot %q has unknown target %T", slot.SlotKind().String(), slot.Target())
			}
		}
	}
	for _, binding := range environment.ValueBindings() {
		if binding.ValueKind().TypeEnv() != target {
			t.Fatalf("value binding %q is not target-bound", binding.ValueKind().String())
		}
	}
	for _, constraint := range environment.Constraints() {
		var ref typedmemory.RelationSignatureRef
		switch value := constraint.(type) {
		case typedmemory.SlotGroupConstraint:
			ref = value.Signature()
		case typedmemory.SlotCardinalityConstraint:
			ref = value.Signature()
		case typedmemory.ReferenceSlotSubsetConstraint:
			ref = value.Signature()
		case typedmemory.ReferenceSlotPartitionConstraint:
			ref = value.Signature()
		case typedmemory.KindDisjointConstraint:
			continue
		default:
			t.Fatalf("constraint %q has unknown variant %T", constraint.ID().String(), constraint)
		}
		if ref.TypeEnv() != target {
			t.Fatalf("constraint %q is not target-bound", constraint.ID().String())
		}
	}
}

func baseLoweringRelationArtifact(t *testing.T) BaseTypeEnvArtifact {
	t.Helper()
	revision, err := typedmemory.NewSourceRevision("fixture-revision")
	if err != nil {
		t.Fatalf("NewSourceRevision: %v", err)
	}
	compiler, err := typedmemory.NewCompilerSchemaVersion(baseTypeEnvCompilerSchema)
	if err != nil {
		t.Fatalf("NewCompilerSchemaVersion: %v", err)
	}
	rootUnit := linkerSourceUnit(
		"fixture:base-lowering:root",
		"Fixture.BaseLowering.Root",
		"Fixture.BaseLowering",
		10,
	)
	referenceUnit := linkerSourceUnit(
		"fixture:base-lowering:reference-slot",
		"Fixture.BaseLowering.ReferenceSlot",
		"Fixture.BaseLowering",
		20,
	)
	valueUnit := linkerSourceUnit(
		"fixture:base-lowering:value-slot",
		"Fixture.BaseLowering.ValueSlot",
		"Fixture.BaseLowering",
		30,
	)
	artifact, err := linkStructuralDeclarations(
		revision,
		compiler,
		[]StructuralDeclaration{
			RelationRootDeclaration{
				source:      rootUnit,
				owner:       rootUnit.ParentPatternID,
				subjectKind: "Fixture.Subject",
				relation:    "Fixture.TargetBoundRelation",
			},
			SlotDeclarationFragment{
				source:      referenceUnit,
				owner:       referenceUnit.ParentPatternID,
				slotKind:    "ReferenceSlot",
				valueKind:   "Fixture.ReferenceValue",
				reference:   ByReferenceEvidence{refKind: "Fixture.ReferenceValueRef"},
				cardinality: BoundedCardinalityEvidence{minimum: 1, maximum: 1},
			},
			SlotDeclarationFragment{
				source:      valueUnit,
				owner:       valueUnit.ParentPatternID,
				slotKind:    "ValueSlot",
				valueKind:   "Fixture.Value",
				reference:   ByValueEvidence{},
				cardinality: BoundedCardinalityEvidence{minimum: 0, maximum: 1},
			},
		},
		nil,
	)
	if err != nil {
		t.Fatalf("linkStructuralDeclarations: %v", err)
	}
	return artifact
}

func baseLoweringCoverageOnlyArtifact(t *testing.T) BaseTypeEnvArtifact {
	t.Helper()
	fixture := newArtifactFixture(t)
	location := fixture.location(t, "base-lowering-coverage-gap", 1, 2)
	subject, err := typedmemory.SourceUnitCoverage(location.UnitID())
	if err != nil {
		t.Fatalf("SourceUnitCoverage: %v", err)
	}
	entry, err := typedmemory.NewSourceOnlyCoverageEntry(
		subject,
		location,
		"base_lowering_test_gap",
	)
	if err != nil {
		t.Fatalf("NewSourceOnlyCoverageEntry: %v", err)
	}
	coverage, err := typedmemory.NewCoverageManifest([]typedmemory.CoverageEntry{entry})
	if err != nil {
		t.Fatalf("NewCoverageManifest: %v", err)
	}
	ir, err := NewCoverageOnlyLinkedTypeEnvIR(
		fixture.revision,
		fixture.compiler,
		coverage,
		"no executable declarations",
	)
	if err != nil {
		t.Fatalf("NewCoverageOnlyLinkedTypeEnvIR: %v", err)
	}
	artifact, err := SealBaseTypeEnv(ir)
	if err != nil {
		t.Fatalf("SealBaseTypeEnv: %v", err)
	}
	return artifact
}

func baseLoweringTargetRef(t *testing.T, fill string) typedmemory.TypeEnvRef {
	t.Helper()
	digest, err := typedmemory.NewSHA256Digest("sha256:" + strings.Repeat(fill, 64))
	if err != nil {
		t.Fatalf("NewSHA256Digest: %v", err)
	}
	ref, err := typedmemory.NewTypeEnvRef(digest)
	if err != nil {
		t.Fatalf("NewTypeEnvRef: %v", err)
	}
	return ref
}
