package typedmemory

import (
	"bytes"
	"testing"
)

func TestRelationalAssertionV3PreservesThreeExplicitModalitiesWithoutInferringObtaining(
	t *testing.T,
) {
	fixture := newValidationFixture(t)
	testCases := []struct {
		name     string
		modality AssertionModality
		kind     AssertionModalityKind
	}{
		{
			name:     "positive without occurrence designation",
			modality: NewAffirmsObtaining(),
			kind:     AssertionModalityAffirmsObtaining,
		},
		{
			name:     "negative",
			modality: NewDeniesObtaining(),
			kind:     AssertionModalityDeniesObtaining,
		},
		{
			name:     "unknown",
			modality: NewObtainingUnknown(),
			kind:     AssertionModalityObtainingUnknown,
		},
	}

	digests := make(map[string]struct{}, len(testCases))
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			changeSet := relationalAssertionTestChangeSet(t, fixture, testCase.modality)
			verdict := ValidateMemoryChangeSet(
				fixture.environment,
				fixture.registry,
				fixture.snapshot,
				changeSet,
			)
			valid, ok := verdict.(Valid)
			if !ok {
				t.Fatalf("verdict = %T (%s); want Valid", verdict, verdict.Kind())
			}
			changes := valid.ChangeSet().Changes()
			if len(changes) != 1 {
				t.Fatalf("validated changes = %d; want 1", len(changes))
			}
			validated, ok := changes[0].(ValidatedRelationalAssertion)
			if !ok {
				t.Fatalf(
					"validated change = %T; want ValidatedRelationalAssertion",
					changes[0],
				)
			}
			assertion := validated.Assertion()
			if assertion.Modality().Kind() != testCase.kind {
				t.Fatalf(
					"validated modality = %q; want %q",
					assertion.Modality().Kind(),
					testCase.kind,
				)
			}
			if positive, isPositive := assertion.Modality().(AffirmsObtaining); isPositive &&
				positive.HasOccurrenceDesignation() {
				t.Fatal("positive assertion inferred an occurrence designation")
			}
			if !valid.AdmissionBatch().IsValid() {
				t.Fatal("pure validation did not seal its exact structural basis")
			}

			canonical, err := assertion.CanonicalBytes()
			if err != nil {
				t.Fatalf("RelationalAssertion.CanonicalBytes(): %v", err)
			}
			decoded, err := DecodeCanonicalRelationalAssertion(canonical)
			if err != nil {
				t.Fatalf("DecodeCanonicalRelationalAssertion(): %v", err)
			}
			reencoded, err := decoded.CanonicalBytes()
			if err != nil {
				t.Fatalf("decoded CanonicalBytes(): %v", err)
			}
			if !bytes.Equal(reencoded, canonical) || decoded.Modality().Kind() != testCase.kind {
				t.Fatal("v3 canonical round trip changed bytes or modality")
			}
			if _, err := DecodeCanonicalRelationInstance(canonical); err == nil {
				t.Fatal("legacy v2 decoder accepted disjoint v3 assertion bytes")
			}
			digest, err := assertion.Digest()
			if err != nil {
				t.Fatalf("RelationalAssertion.Digest(): %v", err)
			}
			digests[digest.String()] = struct{}{}
		})
	}
	if len(digests) != len(testCases) {
		t.Fatalf("three modalities produced only %d canonical digests", len(digests))
	}
}

func TestRelationalAssertionV3RejectsMissingOrUnsealedModality(t *testing.T) {
	fixture := newValidationFixture(t)
	legacy := fixture.changeSet.Changes()[0].(InstantiateRelation).Relation()
	base := RelationalAssertionCandidateInput{
		Assertion:  legacy.Assertion(),
		Signature:  legacy.Signature(),
		Slice:      legacy.Slice(),
		Bindings:   legacy.Bindings(),
		Provenance: legacy.Provenance(),
	}

	for name, modality := range map[string]AssertionModality{
		"missing":         nil,
		"zero positive":   AffirmsObtaining{},
		"zero negative":   DeniesObtaining{},
		"zero unresolved": ObtainingUnknown{},
	} {
		t.Run(name, func(t *testing.T) {
			input := base
			input.Modality = modality
			if _, err := NewRelationalAssertionCandidate(input); err == nil {
				t.Fatal("RelationalAssertionCandidate accepted a missing or unsealed modality")
			}
		})
	}
}

func TestRelationalAssertionV3DoesNotAliasFrozenV2CanonicalCarriers(t *testing.T) {
	fixture := newValidationFixture(t)
	legacyCandidate := fixture.changeSet.Changes()[0].(InstantiateRelation).Relation()
	legacyCandidateBytes, err := canonicalRelationInstantiation(legacyCandidate)
	if err != nil {
		t.Fatalf("canonicalRelationInstantiation(): %v", err)
	}
	v3ChangeSet := relationalAssertionTestChangeSet(t, fixture, NewObtainingUnknown())
	v3Candidate := v3ChangeSet.Changes()[0].(AssertRelation).Assertion()
	v3CandidateBytes, err := v3Candidate.CanonicalBytes()
	if err != nil {
		t.Fatalf("RelationalAssertionCandidate.CanonicalBytes(): %v", err)
	}
	if !bytes.Contains(legacyCandidateBytes, []byte("relation-instantiation.v2")) ||
		!bytes.Contains(v3CandidateBytes, []byte(relationalAssertionCandidateCanonicalDomain)) ||
		bytes.Equal(legacyCandidateBytes, v3CandidateBytes) {
		t.Fatal("candidate v2 and v3 canonical domains are not disjoint")
	}

	legacyStrong := canonicalDecodeRelationFixture(t)
	legacyStrongBytes, err := legacyStrong.CanonicalBytes()
	if err != nil {
		t.Fatalf("legacy RelationInstance.CanonicalBytes(): %v", err)
	}
	if _, err := DecodeCanonicalRelationalAssertion(legacyStrongBytes); err == nil {
		t.Fatal("v3 decoder accepted frozen validated-relation-instance.v2 bytes")
	}
	decodedLegacy, err := DecodeCanonicalRelationInstance(legacyStrongBytes)
	if err != nil {
		t.Fatalf("legacy v2 decoder no longer reads frozen bytes: %v", err)
	}
	reencodedLegacy, err := decodedLegacy.CanonicalBytes()
	if err != nil {
		t.Fatalf("decoded legacy CanonicalBytes(): %v", err)
	}
	if !bytes.Equal(reencodedLegacy, legacyStrongBytes) {
		t.Fatal("legacy v2 decoder rewrote canonical bytes")
	}
}

func relationalAssertionTestChangeSet(
	t *testing.T,
	fixture validationFixture,
	modality AssertionModality,
) MemoryChangeSet {
	t.Helper()
	legacy := fixture.changeSet.Changes()[0].(InstantiateRelation).Relation()
	assertion, err := NewRelationalAssertionCandidate(RelationalAssertionCandidateInput{
		Assertion:  legacy.Assertion(),
		Signature:  legacy.Signature(),
		Slice:      legacy.Slice(),
		Modality:   modality,
		Bindings:   legacy.Bindings(),
		Provenance: legacy.Provenance(),
	})
	if err != nil {
		t.Fatalf("NewRelationalAssertionCandidate(): %v", err)
	}
	change, err := NewAssertRelation(assertion)
	if err != nil {
		t.Fatalf("NewAssertRelation(): %v", err)
	}
	changeSet, err := NewMemoryChangeSet([]MemoryChange{change})
	if err != nil {
		t.Fatalf("NewMemoryChangeSet(): %v", err)
	}
	return changeSet
}
