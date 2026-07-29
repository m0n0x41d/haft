package projecttypeenv

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/runtimemechanism"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestRuntimeEvaluationBasisSealDerivesCanonicalPermutationInvariantIdentity(t *testing.T) {
	pins := runtimeEvaluationBasisFixturePins(t)
	reversed := []RuntimeEvaluationMechanismPin{pins[2], pins[1], pins[0]}

	first, err := SealRuntimeEvaluationBasis(pins)
	if err != nil {
		t.Fatalf("SealRuntimeEvaluationBasis(first): %v", err)
	}
	second, err := SealRuntimeEvaluationBasis(reversed)
	if err != nil {
		t.Fatalf("SealRuntimeEvaluationBasis(second): %v", err)
	}
	if first.Ref() != second.Ref() {
		t.Fatalf("permutation changed X-ref: %s != %s", first.Ref().String(), second.Ref().String())
	}
	if !bytes.Equal(first.CanonicalBytes(), second.CanonicalBytes()) {
		t.Fatal("permutation changed canonical X bytes")
	}
	if len(first.CanonicalBytes()) == 0 {
		t.Fatal("sealed X canonical bytes are empty")
	}
	if !strings.HasPrefix(first.Ref().String(), runtimeEvaluationBasisRefPrefix) {
		t.Fatalf("X-ref %q has the wrong domain", first.Ref().String())
	}
	canonicalText := string(first.CanonicalBytes())
	if !strings.Contains(canonicalText, runtimeEvaluationBasisCanonicalDomain) ||
		!strings.Contains(canonicalText, `"invocation_contract":"codec_canonicalization"`) ||
		!strings.Contains(canonicalText, `"invocation_contract":"member_of"`) ||
		!strings.Contains(canonicalText, `"invocation_contract":"carrier_membership_delivery"`) {
		t.Fatalf("canonical-v2 X omits domain or invocation contracts: %s", canonicalText)
	}
	legacy := bytes.Replace(
		first.CanonicalBytes(),
		[]byte("runtime-evaluation-basis-artifact.v2"),
		[]byte("runtime-evaluation-basis-artifact.v1"),
		1,
	)
	if _, err := DecodeRuntimeEvaluationBasisArtifact(legacy); err == nil {
		t.Fatal("canonical-v1 X domain was accepted after the schema identity bump")
	}
	if err := first.Verify(); err != nil {
		t.Fatalf("RuntimeEvaluationBasisArtifact.Verify(): %v", err)
	}
	if err := first.VerifyResolvedClosure(); err != nil {
		t.Fatalf("RuntimeEvaluationBasisArtifact.VerifyResolvedClosure(): %v", err)
	}

	decoded, err := DecodeRuntimeEvaluationBasisArtifact(first.CanonicalBytes())
	if err != nil {
		t.Fatalf("DecodeRuntimeEvaluationBasisArtifact(): %v", err)
	}
	if decoded.Ref() != first.Ref() {
		t.Fatalf("decoded X-ref = %s, want %s", decoded.Ref().String(), first.Ref().String())
	}
	if err := decoded.VerifyResolvedClosure(); err == nil ||
		!strings.Contains(err.Error(), "is not resolved") {
		t.Fatalf("decoded X transitive closure error = %v", err)
	}
	verified, err := VerifyRuntimeEvaluationBasisArtifact(first.Ref(), first.CanonicalBytes())
	if err != nil {
		t.Fatalf("VerifyRuntimeEvaluationBasisArtifact(): %v", err)
	}
	if verified.Ref() != first.Ref() {
		t.Fatalf("verified X-ref = %s, want %s", verified.Ref().String(), first.Ref().String())
	}
	parsed, err := ParseRuntimeEvaluationBasisRef(first.Ref().String())
	if err != nil {
		t.Fatalf("ParseRuntimeEvaluationBasisRef(): %v", err)
	}
	if parsed != first.Ref() {
		t.Fatalf("parsed X-ref = %s, want %s", parsed.String(), first.Ref().String())
	}
}

func TestRuntimeEvaluationBasisIdentityIsSensitiveToEveryPinCoordinate(t *testing.T) {
	basePins := runtimeEvaluationBasisFixturePins(t)
	base, err := SealRuntimeEvaluationBasis(basePins)
	if err != nil {
		t.Fatalf("SealRuntimeEvaluationBasis(base): %v", err)
	}

	tests := []struct {
		name string
		pin  RuntimeEvaluationMechanismPin
	}{
		{
			name: "semantic codec",
			pin: runtimeCodecMechanismPin(
				t,
				"haft.Codec.Other",
				"v1",
				0x11,
				"artifact:codec",
				"1.0.0",
				0x21,
			),
		},
		{
			name: "codec canonicalization version",
			pin: runtimeCodecMechanismPin(
				t,
				"haft.Codec.Text",
				"v2",
				0x11,
				"artifact:codec",
				"1.0.0",
				0x21,
			),
		},
		{
			name: "codec specification digest",
			pin: runtimeCodecMechanismPin(
				t,
				"haft.Codec.Text",
				"v1",
				0x12,
				"artifact:codec",
				"1.0.0",
				0x21,
			),
		},
		{
			name: "mechanism artifact",
			pin: runtimeCodecMechanismPin(
				t,
				"haft.Codec.Text",
				"v1",
				0x11,
				"artifact:codec-other",
				"1.0.0",
				0x21,
			),
		},
		{
			name: "mechanism edition",
			pin: runtimeCodecMechanismPin(
				t,
				"haft.Codec.Text",
				"v1",
				0x11,
				"artifact:codec",
				"1.0.1",
				0x21,
			),
		},
		{
			name: "mechanism digest",
			pin: runtimeCodecMechanismPin(
				t,
				"haft.Codec.Text",
				"v1",
				0x11,
				"artifact:codec",
				"1.0.0",
				0x22,
			),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			changedPins := append([]RuntimeEvaluationMechanismPin(nil), basePins...)
			changedPins[0] = test.pin
			changed, sealErr := SealRuntimeEvaluationBasis(changedPins)
			if sealErr != nil {
				t.Fatalf("SealRuntimeEvaluationBasis(changed): %v", sealErr)
			}
			if changed.Ref() == base.Ref() {
				t.Fatalf("changing %s did not change X-ref", test.name)
			}
		})
	}

	evaluator := runtimeEvaluatorMechanismPin(
		t,
		"haft.rule.shared/v1",
		"artifact:shared",
		"1.0.0",
		0x31,
	)
	carrierMembership := runtimeCarrierMembershipMechanismPin(
		t,
		"haft.rule.shared/v1",
		"artifact:shared",
		"1.0.0",
		0x31,
	)
	evaluatorBasis, err := SealRuntimeEvaluationBasis([]RuntimeEvaluationMechanismPin{evaluator})
	if err != nil {
		t.Fatalf("SealRuntimeEvaluationBasis(evaluator): %v", err)
	}
	carrierBasis, err := SealRuntimeEvaluationBasis([]RuntimeEvaluationMechanismPin{carrierMembership})
	if err != nil {
		t.Fatalf("SealRuntimeEvaluationBasis(carrier membership): %v", err)
	}
	if evaluatorBasis.Ref() == carrierBasis.Ref() {
		t.Fatal("changing a RuleRef mechanism role did not change X-ref")
	}
	otherRule := runtimeEvaluatorMechanismPin(
		t,
		"haft.rule.other/v1",
		"artifact:shared",
		"1.0.0",
		0x31,
	)
	otherRuleBasis, err := SealRuntimeEvaluationBasis(
		[]RuntimeEvaluationMechanismPin{otherRule},
	)
	if err != nil {
		t.Fatalf("SealRuntimeEvaluationBasis(other rule): %v", err)
	}
	if evaluatorBasis.Ref() == otherRuleBasis.Ref() {
		t.Fatal("changing a RuleRef did not change X-ref")
	}
}

func TestRuntimeEvaluationBasisRejectsDuplicateSemanticCoordinates(t *testing.T) {
	codec := runtimeCodecMechanismPin(
		t,
		"haft.Codec.Text",
		"v1",
		0x11,
		"artifact:codec",
		"1.0.0",
		0x21,
	)
	codecOtherMechanism := runtimeCodecMechanismPin(
		t,
		"haft.Codec.Text",
		"v1",
		0x11,
		"artifact:codec-other",
		"1.0.0",
		0x22,
	)
	evaluator := runtimeEvaluatorMechanismPin(
		t,
		"haft.rule.member/v1",
		"artifact:evaluator",
		"1.0.0",
		0x31,
	)
	evaluatorOtherMechanism := runtimeEvaluatorMechanismPin(
		t,
		"haft.rule.member/v1",
		"artifact:evaluator-other",
		"1.0.0",
		0x32,
	)
	carrierMembership := runtimeCarrierMembershipMechanismPin(
		t,
		"haft.rule.member/v1",
		"artifact:carrier-membership",
		"1.0.0",
		0x41,
	)
	carrierMembershipOtherMechanism := runtimeCarrierMembershipMechanismPin(
		t,
		"haft.rule.member/v1",
		"artifact:carrier-membership-other",
		"1.0.0",
		0x42,
	)

	tests := []struct {
		name string
		pins []RuntimeEvaluationMechanismPin
	}{
		{name: "exact duplicate codec", pins: []RuntimeEvaluationMechanismPin{codec, codec}},
		{name: "same codec different mechanism", pins: []RuntimeEvaluationMechanismPin{codec, codecOtherMechanism}},
		{name: "exact duplicate evaluator", pins: []RuntimeEvaluationMechanismPin{evaluator, evaluator}},
		{name: "same evaluator RuleRef different mechanism", pins: []RuntimeEvaluationMechanismPin{evaluator, evaluatorOtherMechanism}},
		{name: "exact duplicate carrier membership", pins: []RuntimeEvaluationMechanismPin{carrierMembership, carrierMembership}},
		{name: "same carrier-membership RuleRef different mechanism", pins: []RuntimeEvaluationMechanismPin{carrierMembership, carrierMembershipOtherMechanism}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, forwardErr := SealRuntimeEvaluationBasis(test.pins)
			if forwardErr == nil || !strings.Contains(forwardErr.Error(), "repeats semantic coordinate") {
				t.Fatalf("SealRuntimeEvaluationBasis() error = %v, want duplicate semantic coordinate", forwardErr)
			}
			reversed := []RuntimeEvaluationMechanismPin{test.pins[1], test.pins[0]}
			_, reverseErr := SealRuntimeEvaluationBasis(reversed)
			if reverseErr == nil || reverseErr.Error() != forwardErr.Error() {
				t.Fatalf(
					"duplicate diagnostic changed under permutation: forward=%v reverse=%v",
					forwardErr,
					reverseErr,
				)
			}
		})
	}
}

func TestRuntimeEvaluationBasisAllowsSameRuleRefAcrossDistinctRoles(t *testing.T) {
	rule := "haft.rule.shared/v1"
	ruleRef, err := typedmemory.NewRuleRef(rule)
	if err != nil {
		t.Fatalf("NewRuleRef(): %v", err)
	}
	evaluatorEntry := runtimeEvaluatorEntryFixture(
		t,
		RuntimeMechanismContractMemberOf,
		ruleRef,
	)
	membershipEntry, err := runtimemechanism.NewCarrierMembershipDeliveryEntry(ruleRef)
	if err != nil {
		t.Fatalf("NewCarrierMembershipDeliveryEntry(): %v", err)
	}
	sharedArtifact := runtimeMechanismArtifactFixture(
		t,
		"artifact:shared-rule-runtime",
		"1.0.0",
		0x51,
		[]runtimemechanism.RuntimeMechanismEntryV1{evaluatorEntry, membershipEntry},
	)
	sharedPin, err := NewRuntimeMechanismArtifactPinFromArtifact(sharedArtifact)
	if err != nil {
		t.Fatalf("NewRuntimeMechanismArtifactPinFromArtifact(): %v", err)
	}
	evaluator, err := NewEvaluatorRuntimeMechanismPin(EvaluatorRuntimeMechanismPinInput{
		Rule:             ruleRef,
		Contract:         RuntimeMechanismContractMemberOf,
		Mechanism:        sharedPin,
		ResolvedArtifact: &sharedArtifact,
	})
	if err != nil {
		t.Fatalf("NewEvaluatorRuntimeMechanismPin(): %v", err)
	}
	membership, err := NewCarrierMembershipRuntimeMechanismPin(
		CarrierMembershipRuntimeMechanismPinInput{
			Rule:             ruleRef,
			Mechanism:        sharedPin,
			ResolvedArtifact: &sharedArtifact,
		},
	)
	if err != nil {
		t.Fatalf("NewCarrierMembershipRuntimeMechanismPin(): %v", err)
	}
	forward, err := SealRuntimeEvaluationBasis(
		[]RuntimeEvaluationMechanismPin{evaluator, membership},
	)
	if err != nil {
		t.Fatalf("SealRuntimeEvaluationBasis(forward): %v", err)
	}
	reverse, err := SealRuntimeEvaluationBasis(
		[]RuntimeEvaluationMechanismPin{membership, evaluator},
	)
	if err != nil {
		t.Fatalf("SealRuntimeEvaluationBasis(reverse): %v", err)
	}
	if forward.Ref() != reverse.Ref() {
		t.Fatalf("role-aware permutation changed X-ref: %s != %s", forward.Ref(), reverse.Ref())
	}
	if !bytes.Equal(forward.CanonicalBytes(), reverse.CanonicalBytes()) {
		t.Fatal("role-aware permutation changed canonical X bytes")
	}
	canonical := string(forward.CanonicalBytes())
	if strings.Count(canonical, `"rule_ref":"`+rule+`"`) != 2 {
		t.Fatalf("canonical X does not retain both role-aware RuleRef pins: %s", canonical)
	}
	if !strings.Contains(canonical, `"kind":"evaluator","role":"evaluator"`) ||
		!strings.Contains(
			canonical,
			`"kind":"carrier_membership","role":"carrier_membership"`,
		) {
		t.Fatalf("canonical X collapsed runtime roles: %s", canonical)
	}
	decoded, err := DecodeRuntimeEvaluationBasisArtifact(forward.CanonicalBytes())
	if err != nil {
		t.Fatalf("DecodeRuntimeEvaluationBasisArtifact(role-aware): %v", err)
	}
	if decoded.Ref() != forward.Ref() || len(decoded.Pins()) != 2 {
		t.Fatalf(
			"decoded role-aware X = ref %s pins %d; want ref %s pins 2",
			decoded.Ref(),
			len(decoded.Pins()),
			forward.Ref(),
		)
	}

	changedMembership := runtimeCarrierMembershipMechanismPin(
		t,
		rule,
		"artifact:membership-specific-runtime",
		"1.0.0",
		0x52,
	)
	changed, err := SealRuntimeEvaluationBasis(
		[]RuntimeEvaluationMechanismPin{evaluator, changedMembership},
	)
	if err != nil {
		t.Fatalf("SealRuntimeEvaluationBasis(changed membership): %v", err)
	}
	if changed.Ref() == forward.Ref() {
		t.Fatal("changing one role-specific mechanism did not change canonical X identity")
	}

	conflictingMembership := runtimeCarrierMembershipMechanismPin(
		t,
		rule,
		"artifact:shared-rule-runtime",
		"1.0.0",
		0x53,
	)
	_, err = SealRuntimeEvaluationBasis(
		[]RuntimeEvaluationMechanismPin{evaluator, conflictingMembership},
	)
	if err == nil || !strings.Contains(err.Error(), "conflicting content digests") {
		t.Fatalf("shared artifact identity conflict error = %v", err)
	}
}

func TestRuntimeEvaluationBasisAllowsSameRuleRefAcrossDistinctContracts(t *testing.T) {
	rule := mustRuntimeRuleRef(t, "haft.rule.shared-contracts/v1")
	enumerationEntry := runtimeEvaluatorEntryFixture(
		t,
		RuntimeMechanismContractEntitySetEnumeration,
		rule,
	)
	memberOfEntry := runtimeEvaluatorEntryFixture(
		t,
		RuntimeMechanismContractMemberOf,
		rule,
	)
	resolved := runtimeMechanismArtifactFixture(
		t,
		"artifact:shared-contract-runtime",
		"1.0.0",
		0x5a,
		[]runtimemechanism.RuntimeMechanismEntryV1{enumerationEntry, memberOfEntry},
	)
	mechanism, err := NewRuntimeMechanismArtifactPinFromArtifact(resolved)
	if err != nil {
		t.Fatalf("NewRuntimeMechanismArtifactPinFromArtifact(): %v", err)
	}
	enumeration, err := NewEvaluatorRuntimeMechanismPin(EvaluatorRuntimeMechanismPinInput{
		Rule:             rule,
		Contract:         RuntimeMechanismContractEntitySetEnumeration,
		Mechanism:        mechanism,
		ResolvedArtifact: &resolved,
	})
	if err != nil {
		t.Fatalf("NewEvaluatorRuntimeMechanismPin(enumeration): %v", err)
	}
	memberOf, err := NewEvaluatorRuntimeMechanismPin(EvaluatorRuntimeMechanismPinInput{
		Rule:             rule,
		Contract:         RuntimeMechanismContractMemberOf,
		Mechanism:        mechanism,
		ResolvedArtifact: &resolved,
	})
	if err != nil {
		t.Fatalf("NewEvaluatorRuntimeMechanismPin(member_of): %v", err)
	}
	basis, err := SealRuntimeEvaluationBasis(
		[]RuntimeEvaluationMechanismPin{memberOf, enumeration},
	)
	if err != nil {
		t.Fatalf("SealRuntimeEvaluationBasis(): %v", err)
	}
	if len(basis.Pins()) != 2 {
		t.Fatalf("pin count = %d; want 2", len(basis.Pins()))
	}
	canonical := string(basis.CanonicalBytes())
	if !strings.Contains(canonical, `"invocation_contract":"entity_set_enumeration"`) ||
		!strings.Contains(canonical, `"invocation_contract":"member_of"`) {
		t.Fatalf("canonical X collapsed distinct contracts: %s", canonical)
	}
}

func TestRuntimeEvaluationBasisSealRequiresExactResolvedMechanismArtifact(t *testing.T) {
	rule := mustRuntimeRuleRef(t, "haft.rule.exact-resolution/v1")
	memberEntry := runtimeEvaluatorEntryFixture(
		t,
		RuntimeMechanismContractMemberOf,
		rule,
	)
	memberArtifact := runtimeMechanismArtifactFixture(
		t,
		"artifact:exact-resolution",
		"1.0.0",
		0x5b,
		[]runtimemechanism.RuntimeMechanismEntryV1{memberEntry},
	)
	memberIdentity, err := NewRuntimeMechanismArtifactPinFromArtifact(memberArtifact)
	if err != nil {
		t.Fatalf("NewRuntimeMechanismArtifactPinFromArtifact(): %v", err)
	}
	claimed, err := NewEvaluatorRuntimeMechanismPin(EvaluatorRuntimeMechanismPinInput{
		Rule:      rule,
		Contract:  RuntimeMechanismContractMemberOf,
		Mechanism: memberIdentity,
	})
	if err != nil {
		t.Fatalf("NewEvaluatorRuntimeMechanismPin(): %v", err)
	}
	if _, err := SealRuntimeEvaluationBasis(
		[]RuntimeEvaluationMechanismPin{claimed},
	); err == nil || !strings.Contains(err.Error(), "is not resolved") {
		t.Fatalf("unresolved seal error = %v", err)
	}
	if _, err := SealRuntimeEvaluationBasis(
		[]RuntimeEvaluationMechanismPin{claimed},
		memberArtifact,
	); err != nil {
		t.Fatalf("SealRuntimeEvaluationBasis(exact artifact): %v", err)
	}

	wrongContractEntry := runtimeEvaluatorEntryFixture(
		t,
		RuntimeMechanismContractKindDefinedness,
		rule,
	)
	wrongContractArtifact := runtimeMechanismArtifactFixture(
		t,
		"artifact:wrong-contract-resolution",
		"1.0.0",
		0x5c,
		[]runtimemechanism.RuntimeMechanismEntryV1{wrongContractEntry},
	)
	wrongIdentity, err := NewRuntimeMechanismArtifactPinFromArtifact(wrongContractArtifact)
	if err != nil {
		t.Fatalf("NewRuntimeMechanismArtifactPinFromArtifact(wrong): %v", err)
	}
	wrongClaim, err := NewEvaluatorRuntimeMechanismPin(EvaluatorRuntimeMechanismPinInput{
		Rule:      rule,
		Contract:  RuntimeMechanismContractMemberOf,
		Mechanism: wrongIdentity,
	})
	if err != nil {
		t.Fatalf("NewEvaluatorRuntimeMechanismPin(wrong claim): %v", err)
	}
	if _, err := SealRuntimeEvaluationBasis(
		[]RuntimeEvaluationMechanismPin{wrongClaim},
		wrongContractArtifact,
	); err == nil || !strings.Contains(err.Error(), "does not contain exact") {
		t.Fatalf("wrong-contract artifact error = %v", err)
	}
}

func TestRuntimeEvaluationBasisRejectsConflictingMechanismContent(t *testing.T) {
	codec := runtimeCodecMechanismPin(
		t,
		"haft.Codec.Text",
		"v1",
		0x11,
		"artifact:shared",
		"1.0.0",
		0x21,
	)
	evaluatorConflict := runtimeEvaluatorMechanismPin(
		t,
		"haft.rule.member/v1",
		"artifact:shared",
		"1.0.0",
		0x22,
	)
	_, err := SealRuntimeEvaluationBasis(
		[]RuntimeEvaluationMechanismPin{codec, evaluatorConflict},
	)
	if err == nil || !strings.Contains(err.Error(), "conflicting content digests") {
		t.Fatalf("SealRuntimeEvaluationBasis() error = %v, want conflicting content", err)
	}

	rule := mustRuntimeRuleRef(t, "haft.rule.member/v1")
	codecEntry, err := runtimemechanism.NewCodecCanonicalizationEntry(codec.Codec())
	if err != nil {
		t.Fatalf("NewCodecCanonicalizationEntry(): %v", err)
	}
	evaluatorEntry := runtimeEvaluatorEntryFixture(
		t,
		RuntimeMechanismContractMemberOf,
		rule,
	)
	shared := runtimeMechanismArtifactFixture(
		t,
		"artifact:shared",
		"1.0.0",
		0x21,
		[]runtimemechanism.RuntimeMechanismEntryV1{codecEntry, evaluatorEntry},
	)
	sharedPin, err := NewRuntimeMechanismArtifactPinFromArtifact(shared)
	if err != nil {
		t.Fatalf("NewRuntimeMechanismArtifactPinFromArtifact(): %v", err)
	}
	sharedCodec, err := NewCodecRuntimeMechanismPin(CodecRuntimeMechanismPinInput{
		Codec:            codec.Codec(),
		Mechanism:        sharedPin,
		ResolvedArtifact: &shared,
	})
	if err != nil {
		t.Fatalf("NewCodecRuntimeMechanismPin(): %v", err)
	}
	evaluatorSameContent, err := NewEvaluatorRuntimeMechanismPin(
		EvaluatorRuntimeMechanismPinInput{
			Rule:             rule,
			Contract:         RuntimeMechanismContractMemberOf,
			Mechanism:        sharedPin,
			ResolvedArtifact: &shared,
		},
	)
	if err != nil {
		t.Fatalf("NewEvaluatorRuntimeMechanismPin(): %v", err)
	}
	if _, err := SealRuntimeEvaluationBasis(
		[]RuntimeEvaluationMechanismPin{sharedCodec, evaluatorSameContent},
	); err != nil {
		t.Fatalf("same exact mechanism content should be reusable: %v", err)
	}
}

func mustRuntimeRuleRef(t *testing.T, raw string) typedmemory.RuleRef {
	t.Helper()
	ref, err := typedmemory.NewRuleRef(raw)
	if err != nil {
		t.Fatalf("NewRuleRef(%q): %v", raw, err)
	}
	return ref
}

func TestRuntimeEvaluationBasisUsesClosedExactEditionGrammar(t *testing.T) {
	accepted := []string{
		"1.0.0",
		"2.3.4-rc.1+build.7",
		"build-20260717.1",
		"build-20260717.2.arm64",
	}
	for _, editionRaw := range accepted {
		if err := runtimeEvaluationBasisEditionError(t, editionRaw); err != nil {
			t.Errorf("exact edition %q rejected: %v", editionRaw, err)
		}
	}
	rejected := []string{
		"latest",
		"main",
		"stable",
		"*",
		"1.x",
		"^1.2.3",
		">=1.2.3",
		"v1.2.3",
		"1.2",
		"1.2.3-01",
		"build-20260717.01",
	}
	for _, editionRaw := range rejected {
		err := runtimeEvaluationBasisEditionError(t, editionRaw)
		if err == nil || !strings.Contains(err.Error(), "must be an exact") {
			t.Errorf("moving or malformed edition %q error = %v", editionRaw, err)
		}
	}
	exactBoundaryEdition := "1.0.0+" +
		strings.Repeat("a", maximumRuntimeMechanismCoordinateBytes-len("1.0.0+"))
	if err := runtimeEvaluationBasisEditionError(t, exactBoundaryEdition); err != nil {
		t.Errorf("exact boundary edition rejected: %v", err)
	}
	oversizedExactEdition := exactBoundaryEdition + "a"
	if err := runtimeEvaluationBasisEditionError(t, oversizedExactEdition); err == nil ||
		!strings.Contains(err.Error(), "exceeds") {
		t.Errorf("oversized exact edition error = %v; want bounded rejection", err)
	}
}

func TestRuntimeEvaluationBasisMatchesEvaluatorRegistryCoordinateBounds(t *testing.T) {
	mechanism := runtimeMechanismArtifactPin(
		t,
		"artifact:evaluator",
		"1.0.0",
		0x90,
	)
	boundaryRule, err := typedmemory.NewRuleRef(
		strings.Repeat("r", maximumRuntimeMechanismRuleRefBytes),
	)
	if err != nil {
		t.Fatalf("NewRuleRef(boundary): %v", err)
	}
	if _, err := NewEvaluatorRuntimeMechanismPin(EvaluatorRuntimeMechanismPinInput{
		Rule:      boundaryRule,
		Contract:  RuntimeMechanismContractMemberOf,
		Mechanism: mechanism,
	}); err != nil {
		t.Fatalf("evaluator boundary RuleRef rejected: %v", err)
	}
	oversizedRule, err := typedmemory.NewRuleRef(
		strings.Repeat("r", maximumRuntimeMechanismRuleRefBytes+1),
	)
	if err != nil {
		t.Fatalf("NewRuleRef(oversized): %v", err)
	}
	if _, err := NewEvaluatorRuntimeMechanismPin(EvaluatorRuntimeMechanismPinInput{
		Rule:      oversizedRule,
		Contract:  RuntimeMechanismContractMemberOf,
		Mechanism: mechanism,
	}); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("oversized evaluator RuleRef error = %v; want bounded rejection", err)
	}

	boundaryArtifact, err := typedmemory.NewCarrierRef(
		strings.Repeat("a", maximumRuntimeMechanismCoordinateBytes),
	)
	if err != nil {
		t.Fatalf("NewCarrierRef(boundary): %v", err)
	}
	edition, err := typedmemory.NewCarrierEdition("1.0.0")
	if err != nil {
		t.Fatalf("NewCarrierEdition(): %v", err)
	}
	if _, err := NewRuntimeMechanismArtifactPin(RuntimeMechanismArtifactPinInput{
		Artifact: boundaryArtifact,
		Edition:  edition,
		Digest:   runtimeEvaluationBasisDigestFixture(t, 0x91),
	}); err != nil {
		t.Fatalf("boundary mechanism artifact rejected: %v", err)
	}
	oversizedArtifact, err := typedmemory.NewCarrierRef(
		strings.Repeat("a", maximumRuntimeMechanismCoordinateBytes+1),
	)
	if err != nil {
		t.Fatalf("NewCarrierRef(oversized): %v", err)
	}
	if _, err := NewRuntimeMechanismArtifactPin(RuntimeMechanismArtifactPinInput{
		Artifact: oversizedArtifact,
		Edition:  edition,
		Digest:   runtimeEvaluationBasisDigestFixture(t, 0x92),
	}); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("oversized mechanism artifact error = %v; want bounded rejection", err)
	}
}

func runtimeEvaluationBasisEditionError(t *testing.T, raw string) error {
	t.Helper()
	artifact, err := typedmemory.NewCarrierRef("artifact:codec")
	if err != nil {
		t.Fatalf("NewCarrierRef(): %v", err)
	}
	edition, err := typedmemory.NewCarrierEdition(raw)
	if err != nil {
		t.Fatalf("NewCarrierEdition(): %v", err)
	}
	_, err = NewRuntimeMechanismArtifactPin(RuntimeMechanismArtifactPinInput{
		Artifact: artifact,
		Edition:  edition,
		Digest:   runtimeEvaluationBasisDigestFixture(t, 0x21),
	})
	return err
}

func TestRuntimeEvaluationBasisEvaluatorRoleIsDistinctFromCarrierMembership(t *testing.T) {
	evaluator := runtimeEvaluatorMechanismPin(
		t,
		"haft.rule.evaluator/v1",
		"artifact:evaluator",
		"1.0.0",
		0x91,
	)
	membership := runtimeCarrierMembershipMechanismPin(
		t,
		"haft.rule.membership/v1",
		"artifact:membership",
		"1.0.0",
		0x92,
	)
	if evaluator.Role() != RuntimeMechanismRoleEvaluator {
		t.Fatalf("evaluator role = %q; want %q", evaluator.Role(), RuntimeMechanismRoleEvaluator)
	}
	if membership.Role() != RuntimeMechanismRoleCarrierMembership {
		t.Fatalf(
			"carrier-membership role = %q; want %q",
			membership.Role(),
			RuntimeMechanismRoleCarrierMembership,
		)
	}
	if evaluator.Role() == membership.Role() {
		t.Fatal("evaluator and carrier-membership roles collapsed")
	}

	artifact, err := SealRuntimeEvaluationBasis(
		[]RuntimeEvaluationMechanismPin{membership, evaluator},
	)
	if err != nil {
		t.Fatalf("SealRuntimeEvaluationBasis(): %v", err)
	}
	canonical := string(artifact.CanonicalBytes())
	if !strings.Contains(canonical, `"kind":"evaluator","role":"evaluator"`) {
		t.Fatalf("canonical X omits exact evaluator role: %s", canonical)
	}
	if !strings.Contains(
		canonical,
		`"kind":"carrier_membership","role":"carrier_membership"`,
	) {
		t.Fatalf("canonical X omits exact carrier-membership role: %s", canonical)
	}
}

func TestRuntimeEvaluationBasisDecodeRejectsUnknownAndForbiddenFields(t *testing.T) {
	for _, field := range []string{
		"project_id",
		"graph_revision",
		"profile",
		"authority",
		"stage",
		"composite_ref",
		"project_head",
	} {
		t.Run(field, func(t *testing.T) {
			payload := []byte(fmt.Sprintf(`{"pins":[],%q:"forbidden"}`, field))
			canonical := runtimeEvaluationBasisEnvelopeFixture(payload)
			_, err := DecodeRuntimeEvaluationBasisArtifact(canonical)
			if err == nil || !strings.Contains(err.Error(), "unknown field") {
				t.Fatalf("DecodeRuntimeEvaluationBasisArtifact() error = %v, want unknown field", err)
			}
		})
	}

	unknownKind := []byte(`{"pins":[{"kind":"head","role":"evaluator","rule_ref":"haft.rule/v1","mechanism":{"artifact_ref":"artifact:x","edition":"1.0.0","digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}]}`)
	_, err := DecodeRuntimeEvaluationBasisArtifact(runtimeEvaluationBasisEnvelopeFixture(unknownKind))
	if err == nil || !strings.Contains(err.Error(), "kind \"head\" is not supported") {
		t.Fatalf("unknown-kind error = %v", err)
	}

	wrongRole := []byte(`{"pins":[{"kind":"evaluator","role":"codec","rule_ref":"haft.rule/v1","mechanism":{"artifact_ref":"artifact:x","edition":"1.0.0","digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}]}`)
	_, err = DecodeRuntimeEvaluationBasisArtifact(runtimeEvaluationBasisEnvelopeFixture(wrongRole))
	if err == nil || !strings.Contains(err.Error(), "role \"codec\" is invalid") {
		t.Fatalf("wrong-role error = %v", err)
	}

	wrongCoordinateKind := []byte(`{"pins":[{"kind":"evaluator","role":"evaluator","codec_ref":{"id":"haft.Codec.Text","canonicalization_version":"v1","specification_digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},"mechanism":{"artifact_ref":"artifact:x","edition":"1.0.0","digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}]}`)
	_, err = DecodeRuntimeEvaluationBasisArtifact(runtimeEvaluationBasisEnvelopeFixture(wrongCoordinateKind))
	if err == nil || !strings.Contains(err.Error(), "requires only rule_ref") {
		t.Fatalf("wrong-coordinate-kind error = %v", err)
	}

	unknownNestedField := []byte(`{"pins":[{"kind":"evaluator","role":"evaluator","rule_ref":"haft.rule/v1","mechanism":{"artifact_ref":"artifact:x","edition":"1.0.0","digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","build_id":"forbidden"}}]}`)
	_, err = DecodeRuntimeEvaluationBasisArtifact(
		runtimeEvaluationBasisEnvelopeFixture(unknownNestedField),
	)
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("unknown nested-field error = %v", err)
	}
}

func TestRuntimeEvaluationBasisDecodeRejectsMalformedTrailingAndNoncanonicalBytes(t *testing.T) {
	artifact, err := SealRuntimeEvaluationBasis(runtimeEvaluationBasisFixturePins(t))
	if err != nil {
		t.Fatalf("SealRuntimeEvaluationBasis(): %v", err)
	}

	trailing := append(artifact.CanonicalBytes(), byte('x'))
	if _, err := DecodeRuntimeEvaluationBasisArtifact(trailing); err == nil || !strings.Contains(err.Error(), "trailing bytes") {
		t.Fatalf("trailing-byte error = %v", err)
	}

	malformed := artifact.CanonicalBytes()
	malformed[0] = 0xff
	if _, err := DecodeRuntimeEvaluationBasisArtifact(malformed); err == nil {
		t.Fatal("malformed envelope was accepted")
	}

	payload, err := decodeRuntimeEvaluationBasisEnvelope(artifact.CanonicalBytes())
	if err != nil {
		t.Fatalf("decodeRuntimeEvaluationBasisEnvelope(): %v", err)
	}
	noncanonicalPayload := append([]byte(" "), payload...)
	noncanonical := runtimeEvaluationBasisEnvelopeFixture(noncanonicalPayload)
	if _, err := DecodeRuntimeEvaluationBasisArtifact(noncanonical); err == nil || !strings.Contains(err.Error(), "not canonical") {
		t.Fatalf("noncanonical payload error = %v", err)
	}

	trailingJSONPayload := append(append([]byte(nil), payload...), []byte(` {"pins":[]}`)...)
	trailingJSON := runtimeEvaluationBasisEnvelopeFixture(trailingJSONPayload)
	if _, err := DecodeRuntimeEvaluationBasisArtifact(trailingJSON); err == nil || !strings.Contains(err.Error(), "trailing value") {
		t.Fatalf("trailing JSON error = %v", err)
	}

	unorderedPins := []RuntimeEvaluationMechanismPin{
		runtimeEvaluationBasisFixturePins(t)[2],
		runtimeEvaluationBasisFixturePins(t)[1],
		runtimeEvaluationBasisFixturePins(t)[0],
	}
	unorderedBasisPins := make([]RuntimeEvaluationBasisPin, 0, len(unorderedPins))
	for _, pin := range unorderedPins {
		unorderedBasisPins = append(unorderedBasisPins, pin)
	}
	encoded := runtimeEvaluationBasisCanonicalV1{
		Pins: runtimeEvaluationBasisPinsCanonical(unorderedBasisPins),
	}
	unorderedPayload, err := json.Marshal(encoded)
	if err != nil {
		t.Fatalf("json.Marshal(unordered): %v", err)
	}
	unordered := runtimeEvaluationBasisEnvelopeFixture(unorderedPayload)
	if _, err := DecodeRuntimeEvaluationBasisArtifact(unordered); err == nil || !strings.Contains(err.Error(), "not canonical") {
		t.Fatalf("noncanonical order error = %v", err)
	}
}

func TestRuntimeEvaluationBasisVerifyRejectsForgedIdentityAndStoredPins(t *testing.T) {
	first, err := SealRuntimeEvaluationBasis(runtimeEvaluationBasisFixturePins(t))
	if err != nil {
		t.Fatalf("SealRuntimeEvaluationBasis(first): %v", err)
	}
	otherPin := runtimeEvaluatorMechanismPin(
		t,
		"haft.rule.other/v1",
		"artifact:other",
		"1.0.0",
		0x7a,
	)
	second, err := SealRuntimeEvaluationBasis([]RuntimeEvaluationMechanismPin{otherPin})
	if err != nil {
		t.Fatalf("SealRuntimeEvaluationBasis(second): %v", err)
	}

	if _, err := VerifyRuntimeEvaluationBasisArtifact(second.Ref(), first.CanonicalBytes()); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("forged expected-ref error = %v", err)
	}

	forgedRef := first
	forgedRef.ref = second.Ref()
	if err := forgedRef.Verify(); err == nil || !strings.Contains(err.Error(), "reference is not derived") {
		t.Fatalf("forged stored-ref error = %v", err)
	}

	forgedPins := first
	forgedPins.pins = []RuntimeEvaluationBasisPin{otherPin}
	if err := forgedPins.Verify(); err == nil || !strings.Contains(err.Error(), "stored pins") {
		t.Fatalf("forged stored-pins error = %v", err)
	}

	forgedOrder := first
	forgedOrder.pins = []RuntimeEvaluationBasisPin{
		first.pins[2],
		first.pins[1],
		first.pins[0],
	}
	if err := forgedOrder.Verify(); err == nil || !strings.Contains(err.Error(), "canonical pin order") {
		t.Fatalf("forged stored-order error = %v", err)
	}
}

func TestRuntimeEvaluationBasisOwnsInputsAndReturnsClones(t *testing.T) {
	pins := runtimeEvaluationBasisFixturePins(t)
	artifact, err := SealRuntimeEvaluationBasis(pins)
	if err != nil {
		t.Fatalf("SealRuntimeEvaluationBasis(): %v", err)
	}
	originalRef := artifact.Ref()
	originalCanonical := artifact.CanonicalBytes()
	originalPins := artifact.Pins()

	pins[0] = runtimeEvaluatorMechanismPin(
		t,
		"haft.rule.input-mutation/v1",
		"artifact:input-mutation",
		"1.0.0",
		0x70,
	)
	returnedBytes := artifact.CanonicalBytes()
	returnedBytes[0] ^= 0xff
	returnedPins := artifact.Pins()
	returnedPins[0] = pins[0]

	if artifact.Ref() != originalRef {
		t.Fatal("mutating caller-owned values changed X-ref")
	}
	if !bytes.Equal(artifact.CanonicalBytes(), originalCanonical) {
		t.Fatal("mutating returned bytes changed stored canonical bytes")
	}
	if runtimeMechanismPinSortKey(artifact.Pins()[0]) != runtimeMechanismPinSortKey(originalPins[0]) {
		t.Fatal("mutating returned pins changed stored pins")
	}
	if err := artifact.Verify(); err != nil {
		t.Fatalf("artifact failed after external mutation: %v", err)
	}
}

func TestRuntimeEvaluationBasisEnforcesResourceBounds(t *testing.T) {
	pins := make([]RuntimeEvaluationMechanismPin, 0, maximumRuntimeEvaluationBasisPins+1)
	mechanism := runtimeMechanismArtifactPin(t, "artifact:evaluator", "1.0.0", 0x31)
	for index := 0; index <= maximumRuntimeEvaluationBasisPins; index++ {
		rule, err := typedmemory.NewRuleRef(fmt.Sprintf("haft.rule.%d/v1", index))
		if err != nil {
			t.Fatalf("NewRuleRef(%d): %v", index, err)
		}
		pin, err := NewEvaluatorRuntimeMechanismPin(EvaluatorRuntimeMechanismPinInput{
			Rule:      rule,
			Contract:  RuntimeMechanismContractMemberOf,
			Mechanism: mechanism,
		})
		if err != nil {
			t.Fatalf("NewEvaluatorRuntimeMechanismPin(%d): %v", index, err)
		}
		pins = append(pins, pin)
	}
	if _, err := SealRuntimeEvaluationBasis(pins); err == nil || !strings.Contains(err.Error(), "limit") {
		t.Fatalf("pin-limit error = %v", err)
	}

	oversized := make([]byte, maximumRuntimeEvaluationBasisBytes+1)
	if _, err := DecodeRuntimeEvaluationBasisArtifact(oversized); err == nil || !strings.Contains(err.Error(), "limit") {
		t.Fatalf("byte-limit error = %v", err)
	}

	longRef, err := typedmemory.NewCarrierRef(strings.Repeat("x", maximumRuntimeMechanismTextBytes+1))
	if err != nil {
		t.Fatalf("NewCarrierRef(long): %v", err)
	}
	edition, err := typedmemory.NewCarrierEdition("1.0.0")
	if err != nil {
		t.Fatalf("NewCarrierEdition(): %v", err)
	}
	_, err = NewRuntimeMechanismArtifactPin(RuntimeMechanismArtifactPinInput{
		Artifact: longRef,
		Edition:  edition,
		Digest:   runtimeEvaluationBasisDigestFixture(t, 0x21),
	})
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("text-limit error = %v", err)
	}
}

func TestRuntimeEvaluationBasisEmptyBasisIsCanonical(t *testing.T) {
	first, err := SealRuntimeEvaluationBasis(nil)
	if err != nil {
		t.Fatalf("SealRuntimeEvaluationBasis(nil): %v", err)
	}
	second, err := SealRuntimeEvaluationBasis([]RuntimeEvaluationMechanismPin{})
	if err != nil {
		t.Fatalf("SealRuntimeEvaluationBasis(empty): %v", err)
	}
	if first.Ref() != second.Ref() || !bytes.Equal(first.CanonicalBytes(), second.CanonicalBytes()) {
		t.Fatal("nil and empty exact bases did not canonicalize identically")
	}
	if len(first.Pins()) != 0 {
		t.Fatalf("empty basis pins = %d", len(first.Pins()))
	}
}

type unknownRuntimeEvaluationMechanismPin struct{}

func (unknownRuntimeEvaluationMechanismPin) Role() RuntimeMechanismRole {
	return RuntimeMechanismRoleEvaluator
}

func (unknownRuntimeEvaluationMechanismPin) InvocationContract() RuntimeMechanismInvocationContract {
	return RuntimeMechanismContractMemberOf
}

func (unknownRuntimeEvaluationMechanismPin) resolvedRuntimeMechanismCanonical() []byte {
	return nil
}

func (unknownRuntimeEvaluationMechanismPin) resolvedRuntimeBasisCanonical() []byte {
	return nil
}

func (unknownRuntimeEvaluationMechanismPin) runtimeEvaluationBasisPinVariant() {}

func (unknownRuntimeEvaluationMechanismPin) runtimeEvaluationMechanismPinVariant() {}

func TestRuntimeEvaluationBasisRejectsUnknownClosedAlgebraVariant(t *testing.T) {
	_, err := SealRuntimeEvaluationBasis(
		[]RuntimeEvaluationMechanismPin{unknownRuntimeEvaluationMechanismPin{}},
	)
	if err == nil || !strings.Contains(err.Error(), "closed runtime mechanism algebra") {
		t.Fatalf("unknown-variant error = %v", err)
	}
}

func runtimeEvaluationBasisFixturePins(t *testing.T) []RuntimeEvaluationMechanismPin {
	t.Helper()
	return []RuntimeEvaluationMechanismPin{
		runtimeCodecMechanismPin(
			t,
			"haft.Codec.Text",
			"v1",
			0x11,
			"artifact:codec",
			"1.0.0",
			0x21,
		),
		runtimeEvaluatorMechanismPin(
			t,
			"haft.rule.constraint/v1",
			"artifact:evaluator",
			"1.0.0",
			0x31,
		),
		runtimeCarrierMembershipMechanismPin(
			t,
			"haft.member-of.project-record-carrier/v1",
			"artifact:carrier-membership",
			"1.0.0",
			0x41,
		),
	}
}

func runtimeCodecMechanismPin(
	t *testing.T,
	codecID string,
	version string,
	specDigest byte,
	artifact string,
	edition string,
	mechanismDigest byte,
) CodecRuntimeMechanismPin {
	t.Helper()
	id, err := typedmemory.NewCodecID(codecID)
	if err != nil {
		t.Fatalf("NewCodecID(): %v", err)
	}
	canonicalization, err := typedmemory.NewCanonicalizationVersion(version)
	if err != nil {
		t.Fatalf("NewCanonicalizationVersion(): %v", err)
	}
	codec, err := typedmemory.NewCodecRef(
		id,
		canonicalization,
		runtimeEvaluationBasisDigestFixture(t, specDigest),
	)
	if err != nil {
		t.Fatalf("NewCodecRef(): %v", err)
	}
	return runtimeCodecMechanismPinForRef(
		t,
		codec,
		artifact,
		edition,
		mechanismDigest,
	)
}

func runtimeCodecMechanismPinForRef(
	t *testing.T,
	codec typedmemory.CodecRef,
	artifact string,
	edition string,
	mechanismDigest byte,
) CodecRuntimeMechanismPin {
	t.Helper()
	entry, err := runtimemechanism.NewCodecCanonicalizationEntry(codec)
	if err != nil {
		t.Fatalf("NewCodecCanonicalizationEntry(): %v", err)
	}
	resolved := runtimeMechanismArtifactFixture(
		t,
		artifact,
		edition,
		mechanismDigest,
		[]runtimemechanism.RuntimeMechanismEntryV1{entry},
	)
	mechanism, err := NewRuntimeMechanismArtifactPinFromArtifact(resolved)
	if err != nil {
		t.Fatalf("NewRuntimeMechanismArtifactPinFromArtifact(): %v", err)
	}
	pin, err := NewCodecRuntimeMechanismPin(CodecRuntimeMechanismPinInput{
		Codec:            codec,
		Mechanism:        mechanism,
		ResolvedArtifact: &resolved,
	})
	if err != nil {
		t.Fatalf("NewCodecRuntimeMechanismPin(): %v", err)
	}
	return pin
}

func runtimeEvaluatorMechanismPin(
	t *testing.T,
	ruleRaw string,
	artifact string,
	edition string,
	mechanismDigest byte,
) EvaluatorRuntimeMechanismPin {
	t.Helper()
	return runtimeEvaluatorMechanismPinWithContract(
		t,
		ruleRaw,
		RuntimeMechanismContractMemberOf,
		artifact,
		edition,
		mechanismDigest,
	)
}

func runtimeEvaluatorMechanismPinWithContract(
	t *testing.T,
	ruleRaw string,
	contract RuntimeMechanismInvocationContract,
	artifact string,
	edition string,
	mechanismDigest byte,
) EvaluatorRuntimeMechanismPin {
	t.Helper()
	rule, err := typedmemory.NewRuleRef(ruleRaw)
	if err != nil {
		t.Fatalf("NewRuleRef(): %v", err)
	}
	entry := runtimeEvaluatorEntryFixture(t, contract, rule)
	resolved := runtimeMechanismArtifactFixture(
		t,
		artifact,
		edition,
		mechanismDigest,
		[]runtimemechanism.RuntimeMechanismEntryV1{entry},
	)
	mechanism, err := NewRuntimeMechanismArtifactPinFromArtifact(resolved)
	if err != nil {
		t.Fatalf("NewRuntimeMechanismArtifactPinFromArtifact(): %v", err)
	}
	pin, err := NewEvaluatorRuntimeMechanismPin(EvaluatorRuntimeMechanismPinInput{
		Rule:             rule,
		Contract:         contract,
		Mechanism:        mechanism,
		ResolvedArtifact: &resolved,
	})
	if err != nil {
		t.Fatalf("NewEvaluatorRuntimeMechanismPin(): %v", err)
	}
	return pin
}

func runtimeCarrierMembershipMechanismPin(
	t *testing.T,
	ruleRaw string,
	artifact string,
	edition string,
	mechanismDigest byte,
) CarrierMembershipRuntimeMechanismPin {
	t.Helper()
	rule, err := typedmemory.NewRuleRef(ruleRaw)
	if err != nil {
		t.Fatalf("NewRuleRef(): %v", err)
	}
	entry, err := runtimemechanism.NewCarrierMembershipDeliveryEntry(rule)
	if err != nil {
		t.Fatalf("NewCarrierMembershipDeliveryEntry(): %v", err)
	}
	resolved := runtimeMechanismArtifactFixture(
		t,
		artifact,
		edition,
		mechanismDigest,
		[]runtimemechanism.RuntimeMechanismEntryV1{entry},
	)
	mechanism, err := NewRuntimeMechanismArtifactPinFromArtifact(resolved)
	if err != nil {
		t.Fatalf("NewRuntimeMechanismArtifactPinFromArtifact(): %v", err)
	}
	pin, err := NewCarrierMembershipRuntimeMechanismPin(
		CarrierMembershipRuntimeMechanismPinInput{
			Rule:             rule,
			Mechanism:        mechanism,
			ResolvedArtifact: &resolved,
		},
	)
	if err != nil {
		t.Fatalf("NewCarrierMembershipRuntimeMechanismPin(): %v", err)
	}
	return pin
}

func runtimeEvaluatorEntryFixture(
	t *testing.T,
	contract RuntimeMechanismInvocationContract,
	rule typedmemory.RuleRef,
) runtimemechanism.RuntimeMechanismEntryV1 {
	t.Helper()
	constructors := map[RuntimeMechanismInvocationContract]func(
		typedmemory.RuleRef,
	) (runtimemechanism.RuntimeMechanismEntryV1, error){
		RuntimeMechanismContractEntitySetEnumeration:           runtimemechanism.NewEntitySetEnumerationEntry,
		RuntimeMechanismContractCandidateVisibility:            runtimemechanism.NewCandidateVisibilityEntry,
		RuntimeMechanismContractKindDefinedness:                runtimemechanism.NewKindDefinednessEntry,
		RuntimeMechanismContractMemberOf:                       runtimemechanism.NewMemberOfEntry,
		RuntimeMechanismContractReferenceDesignationResolution: runtimemechanism.NewReferenceDesignationResolutionEntry,
		RuntimeMechanismContractClaimInterpretation:            runtimemechanism.NewClaimInterpretationEntry,
		RuntimeMechanismContractClaimMeasurement:               runtimemechanism.NewClaimMeasurementEntry,
		RuntimeMechanismContractClaimEvaluation:                runtimemechanism.NewClaimEvaluationEntry,
		RuntimeMechanismContractEpistemeConstitutionEvaluation: runtimemechanism.NewEpistemeConstitutionEvaluationEntry,
	}
	constructor, found := constructors[contract]
	if !found {
		t.Fatalf("unsupported evaluator contract %q", contract)
	}
	entry, err := constructor(rule)
	if err != nil {
		t.Fatalf("construct evaluator runtime mechanism entry: %v", err)
	}
	return entry
}

func runtimeMechanismArtifactFixture(
	t *testing.T,
	artifactRaw string,
	editionRaw string,
	seed byte,
	entries []runtimemechanism.RuntimeMechanismEntryV1,
) runtimemechanism.RuntimeMechanismArtifactV1 {
	t.Helper()
	artifact, err := typedmemory.NewCarrierRef(artifactRaw)
	if err != nil {
		t.Fatalf("NewCarrierRef(): %v", err)
	}
	edition, err := typedmemory.NewCarrierEdition(editionRaw)
	if err != nil {
		t.Fatalf("NewCarrierEdition(): %v", err)
	}
	fillerRule, err := typedmemory.NewRuleRef(fmt.Sprintf("haft.runtime.seed.%02x", seed))
	if err != nil {
		t.Fatalf("NewRuleRef(filler): %v", err)
	}
	filler, err := runtimemechanism.NewCarrierMembershipDeliveryEntry(fillerRule)
	if err != nil {
		t.Fatalf("NewCarrierMembershipDeliveryEntry(filler): %v", err)
	}
	allEntries := append([]runtimemechanism.RuntimeMechanismEntryV1(nil), entries...)
	allEntries = append(allEntries, filler)
	resolved, err := runtimemechanism.SealRuntimeMechanismArtifactV1(
		artifact,
		edition,
		allEntries,
	)
	if err != nil {
		t.Fatalf("SealRuntimeMechanismArtifactV1(): %v", err)
	}
	return resolved
}

func runtimeMechanismArtifactPin(
	t *testing.T,
	artifactRaw string,
	editionRaw string,
	digestByte byte,
) RuntimeMechanismArtifactPin {
	t.Helper()
	artifact, err := typedmemory.NewCarrierRef(artifactRaw)
	if err != nil {
		t.Fatalf("NewCarrierRef(): %v", err)
	}
	edition, err := typedmemory.NewCarrierEdition(editionRaw)
	if err != nil {
		t.Fatalf("NewCarrierEdition(): %v", err)
	}
	pin, err := NewRuntimeMechanismArtifactPin(RuntimeMechanismArtifactPinInput{
		Artifact: artifact,
		Edition:  edition,
		Digest:   runtimeEvaluationBasisDigestFixture(t, digestByte),
	})
	if err != nil {
		t.Fatalf("NewRuntimeMechanismArtifactPin(): %v", err)
	}
	return pin
}

func runtimeEvaluationBasisDigestFixture(
	t *testing.T,
	value byte,
) typedmemory.SHA256Digest {
	t.Helper()
	raw := "sha256:" + strings.Repeat(fmt.Sprintf("%02x", value), 32)
	digest, err := typedmemory.NewSHA256Digest(raw)
	if err != nil {
		t.Fatalf("NewSHA256Digest(): %v", err)
	}
	return digest
}

func runtimeEvaluationBasisEnvelopeFixture(payload []byte) []byte {
	writer := newRuntimeEvaluationBasisWriter(runtimeEvaluationBasisArtifactDomain)
	writer.addBytes(payload)
	return writer.bytes()
}
