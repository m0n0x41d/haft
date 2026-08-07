package runtimemechanism

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestSealRuntimeMechanismArtifactV1PermutationInvariant(t *testing.T) {
	artifactRef := mustCarrierRef(t, "haft.runtime/default")
	edition := mustEdition(t, "1.2.3")
	codec := mustCodecEntry(t, "haft.canonical-json")
	member := mustRuleEntry(t, "haft.rule.member-of", NewMemberOfEntry)
	visibility := mustRuleEntry(t, "haft.rule.candidate-visibility", NewCandidateVisibilityEntry)

	left, err := SealRuntimeMechanismArtifactV1(
		artifactRef,
		edition,
		[]RuntimeMechanismEntryV1{member, codec, visibility},
	)
	if err != nil {
		t.Fatalf("SealRuntimeMechanismArtifactV1(left): %v", err)
	}
	right, err := SealRuntimeMechanismArtifactV1(
		artifactRef,
		edition,
		[]RuntimeMechanismEntryV1{visibility, member, codec},
	)
	if err != nil {
		t.Fatalf("SealRuntimeMechanismArtifactV1(right): %v", err)
	}

	if left.Identity() != right.Identity() {
		t.Fatalf("permutations produced different identities")
	}
	if !bytes.Equal(left.CanonicalBytes(), right.CanonicalBytes()) {
		t.Fatalf("permutations produced different canonical bytes")
	}
	if err := left.Verify(); err != nil {
		t.Fatalf("Verify(): %v", err)
	}

	canonicalCopy := left.CanonicalBytes()
	canonicalCopy[0] ^= 0xff
	if bytes.Equal(canonicalCopy, left.CanonicalBytes()) {
		t.Fatalf("CanonicalBytes returned mutable storage")
	}
	entryCopy := left.Entries()
	entryCopy[0] = RuntimeMechanismEntryV1{}
	if runtimeMechanismEntryCanonicalKey(left.Entries()[0]) == "" {
		t.Fatalf("Entries returned mutable storage")
	}
}

func TestDecodeRuntimeMechanismArtifactV1RejectsMutationAndIdentityMismatch(t *testing.T) {
	artifact := mustArtifact(
		t,
		[]RuntimeMechanismEntryV1{
			mustRuleEntry(t, "haft.rule.member-of", NewMemberOfEntry),
		},
	)
	mutated := artifact.CanonicalBytes()
	mutated[8] ^= 0x01
	if _, err := DecodeRuntimeMechanismArtifactV1(mutated); err == nil {
		t.Fatalf("DecodeRuntimeMechanismArtifactV1 accepted mutated bytes")
	}

	other := mustArtifact(
		t,
		[]RuntimeMechanismEntryV1{
			mustRuleEntry(t, "haft.rule.kind-definedness", NewKindDefinednessEntry),
		},
	)
	if _, err := VerifyRuntimeMechanismArtifactV1(
		artifact.Identity(),
		other.CanonicalBytes(),
	); err == nil {
		t.Fatalf("VerifyRuntimeMechanismArtifactV1 accepted an identity mismatch")
	}

	corrupted := artifact
	corrupted.entries = []RuntimeMechanismEntryV1{
		mustRuleEntry(t, "haft.rule.other", NewMemberOfEntry),
	}
	if err := corrupted.Verify(); err == nil {
		t.Fatalf("Verify accepted stored entries that differ from canonical bytes")
	}
}

func TestDecodeRuntimeMechanismArtifactV1RejectsMalformedTrailingAndNoncanonical(t *testing.T) {
	entry := mustRuleEntry(t, "haft.rule.member-of", NewMemberOfEntry)
	artifact := mustArtifact(t, []RuntimeMechanismEntryV1{entry})
	canonical := artifact.CanonicalBytes()

	malformed := [][]byte{
		canonical[:7],
		canonical[:len(canonical)-1],
		append(append([]byte(nil), canonical...), 0x00),
	}
	for index, input := range malformed {
		if _, err := DecodeRuntimeMechanismArtifactV1(input); err == nil {
			t.Fatalf("malformed input %d was accepted", index)
		}
	}

	artifactRef := mustCarrierRef(t, "haft.runtime/default")
	edition := mustEdition(t, "1.2.3")
	carrier := mustRuleEntry(
		t,
		"haft.rule.shared",
		NewCarrierMembershipDeliveryEntry,
	)
	evaluator := mustRuleEntry(t, "haft.rule.other", NewMemberOfEntry)
	unsorted, err := encodeRuntimeMechanismArtifactV1(
		artifactRef,
		edition,
		[]RuntimeMechanismEntryV1{evaluator, carrier},
	)
	if err != nil {
		t.Fatalf("encodeRuntimeMechanismArtifactV1(unsorted): %v", err)
	}
	if _, err := DecodeRuntimeMechanismArtifactV1(unsorted); err == nil {
		t.Fatalf("DecodeRuntimeMechanismArtifactV1 accepted noncanonical entry order")
	}
}

func TestRuntimeMechanismArtifactV1BoundsAndExactEdition(t *testing.T) {
	artifactRef := mustCarrierRef(t, "haft.runtime/default")
	edition := mustEdition(t, "1.2.3")
	entries := make([]RuntimeMechanismEntryV1, MaximumArtifactEntries+1)
	if _, err := SealRuntimeMechanismArtifactV1(artifactRef, edition, entries); err == nil {
		t.Fatalf("SealRuntimeMechanismArtifactV1 accepted too many entries")
	}

	longRule := mustRuleRef(t, strings.Repeat("r", MaximumRuleRefBytes+1))
	if _, err := NewMemberOfEntry(longRule); err == nil {
		t.Fatalf("NewMemberOfEntry accepted an overlong RuleRef")
	}

	longArtifact := mustCarrierRef(t, strings.Repeat("a", MaximumCoordinateBytes+1))
	entry := mustRuleEntry(t, "haft.rule.member-of", NewMemberOfEntry)
	if _, err := SealRuntimeMechanismArtifactV1(
		longArtifact,
		edition,
		[]RuntimeMechanismEntryV1{entry},
	); err == nil {
		t.Fatalf("SealRuntimeMechanismArtifactV1 accepted an overlong artifact reference")
	}

	oversized := make([]byte, MaximumArtifactBytes+1)
	if _, err := DecodeRuntimeMechanismArtifactV1(oversized); err == nil {
		t.Fatalf("DecodeRuntimeMechanismArtifactV1 accepted oversized bytes")
	}

	invalidEditions := []string{"latest", "1", "01.2.3", "1.2.3 - 2.0.0"}
	for _, raw := range invalidEditions {
		invalid := mustEdition(t, raw)
		if _, err := SealRuntimeMechanismArtifactV1(
			artifactRef,
			invalid,
			[]RuntimeMechanismEntryV1{entry},
		); err == nil {
			t.Fatalf("SealRuntimeMechanismArtifactV1 accepted edition %q", raw)
		}
	}

	buildEdition := mustEdition(t, "build-20260717.1.release")
	if _, err := SealRuntimeMechanismArtifactV1(
		artifactRef,
		buildEdition,
		[]RuntimeMechanismEntryV1{entry},
	); err != nil {
		t.Fatalf("SealRuntimeMechanismArtifactV1(build edition): %v", err)
	}
}

func TestRuntimeMechanismArtifactV1ContractSensitivity(t *testing.T) {
	rule := mustRuleRef(t, "haft.rule.shared")
	member, err := NewMemberOfEntry(rule)
	if err != nil {
		t.Fatalf("NewMemberOfEntry(): %v", err)
	}
	definedness, err := NewKindDefinednessEntry(rule)
	if err != nil {
		t.Fatalf("NewKindDefinednessEntry(): %v", err)
	}
	memberArtifact := mustArtifact(t, []RuntimeMechanismEntryV1{member})
	definednessArtifact := mustArtifact(t, []RuntimeMechanismEntryV1{definedness})

	if memberArtifact.Identity().Digest() == definednessArtifact.Identity().Digest() {
		t.Fatalf("distinct invocation contracts produced the same digest")
	}
	if bytes.Equal(memberArtifact.CanonicalBytes(), definednessArtifact.CanonicalBytes()) {
		t.Fatalf("distinct invocation contracts produced the same bytes")
	}
}

func TestRuntimeMechanismArtifactV1SupportsReferenceSchemeEvaluatorContracts(
	t *testing.T,
) {
	testCases := []struct {
		name        string
		contract    InvocationContract
		constructor ruleEntryConstructor
	}{
		{
			name:        "reference designation resolution",
			contract:    InvocationContractReferenceDesignationResolution,
			constructor: NewReferenceDesignationResolutionEntry,
		},
		{
			name:        "claim interpretation",
			contract:    InvocationContractClaimInterpretation,
			constructor: NewClaimInterpretationEntry,
		},
		{
			name:        "claim measurement",
			contract:    InvocationContractClaimMeasurement,
			constructor: NewClaimMeasurementEntry,
		},
		{
			name:        "claim evaluation",
			contract:    InvocationContractClaimEvaluation,
			constructor: NewClaimEvaluationEntry,
		},
		{
			name:        "episteme constitution evaluation",
			contract:    InvocationContractEpistemeConstitutionEvaluation,
			constructor: NewEpistemeConstitutionEvaluationEntry,
		},
	}
	entries := make([]RuntimeMechanismEntryV1, 0, len(testCases))
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			entry := mustRuleEntry(
				t,
				"haft.rule."+testCase.contract.String(),
				testCase.constructor,
			)
			if entry.Role() != RuntimeMechanismRoleEvaluator {
				t.Fatalf("role = %q; want evaluator", entry.Role())
			}
			if entry.Contract() != testCase.contract {
				t.Fatalf(
					"contract = %q; want %q",
					entry.Contract(),
					testCase.contract,
				)
			}
			parsed, err := parseInvocationContract(testCase.contract.String())
			if err != nil || parsed != testCase.contract {
				t.Fatalf("parse contract = %q, %v", parsed, err)
			}
			entries = append(entries, entry)
		})
	}
	artifact := mustArtifact(t, entries)
	decoded, err := DecodeRuntimeMechanismArtifactV1(artifact.CanonicalBytes())
	if err != nil {
		t.Fatalf("DecodeRuntimeMechanismArtifactV1(): %v", err)
	}
	if decoded.Identity() != artifact.Identity() {
		t.Fatal("reference-scheme evaluator contracts changed artifact identity on round trip")
	}
}

func TestRuntimeMechanismArtifactV1AllowsSameRuleAcrossCompatibleRoles(t *testing.T) {
	rule := mustRuleRef(t, "haft.rule.shared")
	evaluator, err := NewMemberOfEntry(rule)
	if err != nil {
		t.Fatalf("NewMemberOfEntry(): %v", err)
	}
	membership, err := NewCarrierMembershipDeliveryEntry(rule)
	if err != nil {
		t.Fatalf("NewCarrierMembershipDeliveryEntry(): %v", err)
	}
	artifact := mustArtifact(
		t,
		[]RuntimeMechanismEntryV1{evaluator, membership},
	)
	if len(artifact.Entries()) != 2 {
		t.Fatalf("entry count = %d; want 2", len(artifact.Entries()))
	}
}

func TestRuntimeMechanismArtifactV1AllowsSameRuleAcrossDistinctContracts(t *testing.T) {
	rule := mustRuleRef(t, "haft.rule.shared")
	member, err := NewMemberOfEntry(rule)
	if err != nil {
		t.Fatalf("NewMemberOfEntry(): %v", err)
	}
	definedness, err := NewKindDefinednessEntry(rule)
	if err != nil {
		t.Fatalf("NewKindDefinednessEntry(): %v", err)
	}
	left := mustArtifact(t, []RuntimeMechanismEntryV1{member, definedness})
	right := mustArtifact(t, []RuntimeMechanismEntryV1{definedness, member})
	if left.Identity() != right.Identity() {
		t.Fatalf("distinct contracts changed identity under permutation")
	}
	if len(left.Entries()) != 2 {
		t.Fatalf("entry count = %d; want 2", len(left.Entries()))
	}
}

func TestRuntimeMechanismArtifactV1RejectsDeterministicDuplicate(t *testing.T) {
	member := mustRuleEntry(t, "haft.rule.shared", NewMemberOfEntry)
	leftError := sealDuplicate(t, []RuntimeMechanismEntryV1{member, member})
	rightError := sealDuplicate(t, []RuntimeMechanismEntryV1{member, member})
	if leftError.Error() != rightError.Error() {
		t.Fatalf(
			"duplicate diagnostic is not deterministic:\nleft:  %s\nright: %s",
			leftError,
			rightError,
		)
	}
	if leftError.Kind() != EntryConflictDuplicate {
		t.Fatalf("conflict kind = %q; want %q", leftError.Kind(), EntryConflictDuplicate)
	}
}

func sealDuplicate(t *testing.T, entries []RuntimeMechanismEntryV1) *EntryConflictError {
	t.Helper()
	_, err := SealRuntimeMechanismArtifactV1(
		mustCarrierRef(t, "haft.runtime/default"),
		mustEdition(t, "1.2.3"),
		entries,
	)
	var conflict *EntryConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("error = %v; want EntryConflictError", err)
	}
	return conflict
}

func mustArtifact(
	t *testing.T,
	entries []RuntimeMechanismEntryV1,
) RuntimeMechanismArtifactV1 {
	t.Helper()
	artifact, err := SealRuntimeMechanismArtifactV1(
		mustCarrierRef(t, "haft.runtime/default"),
		mustEdition(t, "1.2.3"),
		entries,
	)
	if err != nil {
		t.Fatalf("SealRuntimeMechanismArtifactV1(): %v", err)
	}
	return artifact
}

func mustCodecEntry(
	t *testing.T,
	id string,
) RuntimeMechanismEntryV1 {
	t.Helper()
	codecID, err := typedmemory.NewCodecID(id)
	if err != nil {
		t.Fatalf("NewCodecID(): %v", err)
	}
	version, err := typedmemory.NewCanonicalizationVersion("canonical-v1")
	if err != nil {
		t.Fatalf("NewCanonicalizationVersion(): %v", err)
	}
	digest := mustDigest(t, strings.Repeat("a", 64))
	ref, err := typedmemory.NewCodecRef(codecID, version, digest)
	if err != nil {
		t.Fatalf("NewCodecRef(): %v", err)
	}
	entry, err := NewCodecCanonicalizationEntry(ref)
	if err != nil {
		t.Fatalf("NewCodecCanonicalizationEntry(): %v", err)
	}
	return entry
}

type ruleEntryConstructor func(typedmemory.RuleRef) (RuntimeMechanismEntryV1, error)

func mustRuleEntry(
	t *testing.T,
	raw string,
	constructor ruleEntryConstructor,
) RuntimeMechanismEntryV1 {
	t.Helper()
	entry, err := constructor(mustRuleRef(t, raw))
	if err != nil {
		t.Fatalf("rule entry constructor(%q): %v", raw, err)
	}
	return entry
}

func mustRuleRef(t *testing.T, raw string) typedmemory.RuleRef {
	t.Helper()
	ref, err := typedmemory.NewRuleRef(raw)
	if err != nil {
		t.Fatalf("NewRuleRef(%q): %v", raw, err)
	}
	return ref
}

func mustCarrierRef(t *testing.T, raw string) typedmemory.CarrierRef {
	t.Helper()
	ref, err := typedmemory.NewCarrierRef(raw)
	if err != nil {
		t.Fatalf("NewCarrierRef(%q): %v", raw, err)
	}
	return ref
}

func mustEdition(t *testing.T, raw string) typedmemory.CarrierEdition {
	t.Helper()
	edition, err := typedmemory.NewCarrierEdition(raw)
	if err != nil {
		t.Fatalf("NewCarrierEdition(%q): %v", raw, err)
	}
	return edition
}

func mustDigest(t *testing.T, hexValue string) typedmemory.SHA256Digest {
	t.Helper()
	digest, err := typedmemory.NewSHA256Digest(fmt.Sprintf("sha256:%s", hexValue))
	if err != nil {
		t.Fatalf("NewSHA256Digest(): %v", err)
	}
	return digest
}
