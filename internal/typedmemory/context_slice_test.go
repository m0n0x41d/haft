package typedmemory

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"testing"
	"time"
)

func TestContextSliceRequiresExplicitGammaTime(t *testing.T) {
	context := mustContextSliceContext(t, "context:payments")
	_, err := NewContextSlice(ContextSliceInput{Context: context})
	if err == nil {
		t.Fatal("NewContextSlice() accepted a missing GammaTime selector")
	}
}

func TestContextSliceCanonicalizationIsOrderingInvariantAndDeduplicatesExactInputs(t *testing.T) {
	context := mustContextSliceContext(t, "context:payments")
	point := mustContextSlicePoint(t, "2026-07-16T08:00:00Z")
	standardA := mustContextSliceStandardPin(t, "standard:a", "2026.1", "standard-a")
	standardB := mustContextSliceStandardPin(t, "standard:b", "v3", "standard-b")
	environmentA := mustContextSliceEnvironment(t, "jurisdiction", "AM", "env-am")
	environmentB := mustContextSliceEnvironment(t, "platform", "linux-arm64", "env-linux")
	vocabularyA := mustContextSliceVocabularyPin(t, "vocabulary:a", "v2", "vocab-a")
	vocabularyB := mustContextSliceVocabularyPin(t, "vocabulary:b", "v1", "vocab-b")
	roleA := mustContextSliceRoleSetPin(t, "roles:a", "v4", "roles-a")
	roleB := mustContextSliceRoleSetPin(t, "roles:b", "v1", "roles-b")

	forward := mustContextSliceBuild(t, ContextSliceInput{
		Context:              context,
		StandardPins:         []StandardPin{standardA, standardB, standardA},
		EnvironmentSelectors: []EnvironmentSelector{environmentA, environmentB, environmentA},
		VocabularyPins:       []VocabularyPin{vocabularyA, vocabularyB, vocabularyA},
		RoleSetPins:          []RoleSetPin{roleA, roleB, roleA},
		GammaTime:            point,
	})
	reverse := mustContextSliceBuild(t, ContextSliceInput{
		Context:              context,
		StandardPins:         []StandardPin{standardB, standardA},
		EnvironmentSelectors: []EnvironmentSelector{environmentB, environmentA},
		VocabularyPins:       []VocabularyPin{vocabularyB, vocabularyA},
		RoleSetPins:          []RoleSetPin{roleB, roleA},
		GammaTime:            point,
	})

	if forward.Digest() != reverse.Digest() {
		t.Fatalf("reordering changed digest: %s != %s", forward.Digest().String(), reverse.Digest().String())
	}
	if !bytes.Equal(forward.CanonicalBytes(), reverse.CanonicalBytes()) {
		t.Fatal("reordering changed canonical bytes")
	}
	if len(forward.StandardPins()) != 2 || len(forward.EnvironmentSelectors()) != 2 {
		t.Fatal("exact duplicate set members were not deduplicated")
	}
	if forward.StandardPins()[0].VersionedPin().Reference().String() != "standard:a" {
		t.Fatal("Standard pins were not returned in canonical order")
	}
	if forward.EnvironmentSelectors()[0].Key().String() != "jurisdiction" {
		t.Fatal("environment selectors were not returned in canonical order")
	}
}

func TestContextSliceDigestChangesWithVersionSelectorAndTime(t *testing.T) {
	context := mustContextSliceContext(t, "context:payments")
	basePoint := mustContextSlicePoint(t, "2026-07-16T08:00:00Z")
	base := mustContextSliceBuild(t, ContextSliceInput{
		Context:              context,
		StandardPins:         []StandardPin{mustContextSliceStandardPin(t, "standard:api", "v1", "api-v1")},
		EnvironmentSelectors: []EnvironmentSelector{mustContextSliceEnvironment(t, "platform", "linux", "selector-linux")},
		GammaTime:            basePoint,
	})
	versionChanged := mustContextSliceBuild(t, ContextSliceInput{
		Context:              context,
		StandardPins:         []StandardPin{mustContextSliceStandardPin(t, "standard:api", "v2", "api-v2")},
		EnvironmentSelectors: []EnvironmentSelector{mustContextSliceEnvironment(t, "platform", "linux", "selector-linux")},
		GammaTime:            basePoint,
	})
	selectorChanged := mustContextSliceBuild(t, ContextSliceInput{
		Context:              context,
		StandardPins:         []StandardPin{mustContextSliceStandardPin(t, "standard:api", "v1", "api-v1")},
		EnvironmentSelectors: []EnvironmentSelector{mustContextSliceEnvironment(t, "platform", "darwin", "selector-darwin")},
		GammaTime:            basePoint,
	})
	timeChanged := mustContextSliceBuild(t, ContextSliceInput{
		Context:              context,
		StandardPins:         []StandardPin{mustContextSliceStandardPin(t, "standard:api", "v1", "api-v1")},
		EnvironmentSelectors: []EnvironmentSelector{mustContextSliceEnvironment(t, "platform", "linux", "selector-linux")},
		GammaTime:            mustContextSlicePoint(t, "2026-07-16T08:00:01Z"),
	})

	for label, candidate := range map[string]ContextSlice{
		"version":  versionChanged,
		"selector": selectorChanged,
		"time":     timeChanged,
	} {
		if base.Digest() == candidate.Digest() {
			t.Fatalf("%s change did not change ContextSlice digest", label)
		}
	}
}

func TestContextSliceRejectsConflictingPinsAndSelectors(t *testing.T) {
	context := mustContextSliceContext(t, "context:payments")
	point := mustContextSlicePoint(t, "2026-07-16T08:00:00Z")
	_, pinErr := NewContextSlice(ContextSliceInput{
		Context: context,
		StandardPins: []StandardPin{
			mustContextSliceStandardPin(t, "standard:api", "v1", "api-v1"),
			mustContextSliceStandardPin(t, "standard:api", "v2", "api-v2"),
		},
		GammaTime: point,
	})
	if pinErr == nil {
		t.Fatal("NewContextSlice() accepted conflicting versions for one Standard ref")
	}
	_, selectorErr := NewContextSlice(ContextSliceInput{
		Context: context,
		EnvironmentSelectors: []EnvironmentSelector{
			mustContextSliceEnvironment(t, "platform", "linux", "selector-linux"),
			mustContextSliceEnvironment(t, "platform", "darwin", "selector-darwin"),
		},
		GammaTime: point,
	})
	if selectorErr == nil {
		t.Fatal("NewContextSlice() accepted conflicting values for one environment key")
	}
}

func TestContextSliceValidityRejectsNonCanonicalInternalRepresentation(t *testing.T) {
	point := mustContextSlicePoint(t, "2026-07-16T08:00:00Z")
	slice := mustContextSliceBuild(t, ContextSliceInput{
		Context: mustContextSliceContext(t, "context:payments"),
		StandardPins: []StandardPin{
			mustContextSliceStandardPin(t, "standard:a", "v1", "standard-a"),
			mustContextSliceStandardPin(t, "standard:b", "v1", "standard-b"),
		},
		GammaTime: point,
	})
	if !slice.valid() {
		t.Fatal("constructor-produced ContextSlice is not valid")
	}

	forged := slice
	forged.standardPins = []StandardPin{slice.standardPins[1], slice.standardPins[0]}
	writer := canonicalContextSlice(
		forged.context,
		forged.standardPins,
		forged.environmentSelectors,
		forged.vocabularyPins,
		forged.roleSetPins,
		forged.gammaTime,
	)
	forged.canonicalBytes = writer.bytes()
	forged.reference = ContextSliceRef{digest: writer.digest()}
	if forged.valid() {
		t.Fatal("ContextSlice validity accepted a self-digested but non-canonical pin order")
	}
}

func TestContextSliceCanonicalDigestGolden(t *testing.T) {
	anchor := mustContextSlicePoint(t, "2026-07-16T08:00:00Z")
	windowStart := mustContextSlicePoint(t, "2026-07-15T08:00:00Z")
	windowEnd := mustContextSlicePoint(t, "2026-07-16T08:00:00Z")
	window, err := NewGammaWindow(
		windowStart.At(),
		windowEnd.At(),
		GammaBoundaryInclusive,
		GammaBoundaryExclusive,
	)
	if err != nil {
		t.Fatalf("NewGammaWindow: %v", err)
	}
	policy, err := NewGammaPolicyApplication(
		mustContextSliceCarrierRef(t, "policy:rolling-window"),
		mustContextSliceEdition(t, "2026-07"),
		mustContextSliceDigest(t, "rolling-window-policy"),
		anchor,
		window,
	)
	if err != nil {
		t.Fatalf("NewGammaPolicyApplication: %v", err)
	}
	slice := mustContextSliceBuild(t, ContextSliceInput{
		Context:              mustContextSliceContext(t, "context:payments"),
		StandardPins:         []StandardPin{mustContextSliceStandardPin(t, "standard:api", "v1", "api-v1")},
		EnvironmentSelectors: []EnvironmentSelector{mustContextSliceEnvironment(t, "platform", "linux", "selector-linux")},
		VocabularyPins:       []VocabularyPin{mustContextSliceVocabularyPin(t, "vocabulary:domain", "v2", "vocab-domain")},
		RoleSetPins:          []RoleSetPin{mustContextSliceRoleSetPin(t, "roles:operator", "v1", "roles-operator")},
		GammaTime:            policy,
	})

	const expected = "sha256:0291048e4f2398642775cf7ff34f84f0f9a5a0b6b596b217fc48ceec68f38d47"
	if slice.Digest().String() != expected {
		t.Fatalf("ContextSlice canonical digest = %q, want %q", slice.Digest().String(), expected)
	}
}

func TestContextSliceSeparatesPinPositionsAndPolicyInputs(t *testing.T) {
	contextRef := mustContextSliceContext(t, "context:payments")
	anchor := mustContextSlicePoint(t, "2026-07-16T08:00:00Z")
	resolved := mustContextSlicePoint(t, "2026-07-15T08:00:00Z")
	policyRef := mustContextSliceCarrierRef(t, "policy:membership-time")
	policyEdition := mustContextSliceEdition(t, "2026-07")
	policyDigest := mustContextSliceDigest(t, "membership-time-policy")
	policy, err := NewGammaPolicyApplication(
		policyRef,
		policyEdition,
		policyDigest,
		anchor,
		resolved,
	)
	if err != nil {
		t.Fatalf("NewGammaPolicyApplication: %v", err)
	}
	sharedRef := mustContextSliceCarrierRef(t, "carrier:shared")
	sharedEdition := mustContextSliceEdition(t, "v1")
	sharedDigest := mustContextSliceDigest(t, "shared-carrier")
	standard, err := NewStandardPin(sharedRef, sharedEdition, sharedDigest)
	if err != nil {
		t.Fatalf("NewStandardPin: %v", err)
	}
	vocabulary, err := NewVocabularyPin(sharedRef, sharedEdition, sharedDigest)
	if err != nil {
		t.Fatalf("NewVocabularyPin: %v", err)
	}
	roleSet, err := NewRoleSetPin(sharedRef, sharedEdition, sharedDigest)
	if err != nil {
		t.Fatalf("NewRoleSetPin: %v", err)
	}
	standardSlice := mustContextSliceBuild(t, ContextSliceInput{
		Context:      contextRef,
		StandardPins: []StandardPin{standard},
		GammaTime:    policy,
	})
	vocabularySlice := mustContextSliceBuild(t, ContextSliceInput{
		Context:        contextRef,
		VocabularyPins: []VocabularyPin{vocabulary},
		GammaTime:      policy,
	})
	roleSlice := mustContextSliceBuild(t, ContextSliceInput{
		Context:     contextRef,
		RoleSetPins: []RoleSetPin{roleSet},
		GammaTime:   policy,
	})
	if standardSlice.Digest() == vocabularySlice.Digest() ||
		standardSlice.Digest() == roleSlice.Digest() ||
		vocabularySlice.Digest() == roleSlice.Digest() {
		t.Fatal("equal carrier pins in distinct ContextSlice positions collided")
	}

	changedDigestPolicy, err := NewGammaPolicyApplication(
		policyRef,
		policyEdition,
		mustContextSliceDigest(t, "membership-time-policy-v2"),
		anchor,
		resolved,
	)
	if err != nil {
		t.Fatalf("NewGammaPolicyApplication(changed digest): %v", err)
	}
	changedAnchorPolicy, err := NewGammaPolicyApplication(
		policyRef,
		policyEdition,
		policyDigest,
		mustContextSlicePoint(t, "2026-07-16T08:00:01Z"),
		resolved,
	)
	if err != nil {
		t.Fatalf("NewGammaPolicyApplication(changed anchor): %v", err)
	}
	changedResolutionPolicy, err := NewGammaPolicyApplication(
		policyRef,
		policyEdition,
		policyDigest,
		anchor,
		mustContextSlicePoint(t, "2026-07-15T08:00:01Z"),
	)
	if err != nil {
		t.Fatalf("NewGammaPolicyApplication(changed resolution): %v", err)
	}
	for label, changedPolicy := range map[string]GammaPolicyApplication{
		"policy digest": changedDigestPolicy,
		"anchor":        changedAnchorPolicy,
		"resolution":    changedResolutionPolicy,
	} {
		changed := mustContextSliceBuild(t, ContextSliceInput{
			Context:      contextRef,
			StandardPins: []StandardPin{standard},
			GammaTime:    changedPolicy,
		})
		if changed.Digest() == standardSlice.Digest() {
			t.Fatalf("%s change did not change ContextSlice digest", label)
		}
	}
}

func TestGammaPolicyApplicationRequiresExactCompleteBasis(t *testing.T) {
	policyRef := mustContextSliceCarrierRef(t, "policy:rolling-window")
	edition := mustContextSliceEdition(t, "2026-07")
	digest := mustContextSliceDigest(t, "rolling-window-policy")
	anchor := mustContextSlicePoint(t, "2026-07-16T08:00:00Z")
	resolved := mustContextSlicePoint(t, "2026-07-15T08:00:00Z")

	tests := []struct {
		name     string
		ref      CarrierRef
		edition  CarrierEdition
		digest   SHA256Digest
		anchor   GammaPoint
		resolved ResolvedGammaTimeSelector
	}{
		{name: "missing ref", edition: edition, digest: digest, anchor: anchor, resolved: resolved},
		{name: "missing edition", ref: policyRef, digest: digest, anchor: anchor, resolved: resolved},
		{name: "missing digest", ref: policyRef, edition: edition, anchor: anchor, resolved: resolved},
		{name: "missing anchor", ref: policyRef, edition: edition, digest: digest, resolved: resolved},
		{name: "missing resolution", ref: policyRef, edition: edition, digest: digest, anchor: anchor},
		{name: "implicit edition", ref: policyRef, edition: mustContextSliceEdition(t, "latest"), digest: digest, anchor: anchor, resolved: resolved},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewGammaPolicyApplication(
				test.ref,
				test.edition,
				test.digest,
				test.anchor,
				test.resolved,
			)
			if err == nil {
				t.Fatal("NewGammaPolicyApplication() accepted incomplete or implicit policy basis")
			}
		})
	}

	application, err := NewGammaPolicyApplication(policyRef, edition, digest, anchor, resolved)
	if err != nil {
		t.Fatalf("NewGammaPolicyApplication() error = %v", err)
	}
	if len(application.CanonicalBytes()) == 0 {
		t.Fatal("GammaPolicyApplication has empty canonical bytes")
	}
}

func TestGammaUTCNormalizationAndExplicitWindowBoundaries(t *testing.T) {
	utc := mustContextSlicePoint(t, "2026-07-16T08:00:00Z")
	offsetTime := time.Date(2026, 7, 16, 12, 0, 0, 0, time.FixedZone("UTC+4", 4*60*60))
	offset, err := NewGammaPoint(offsetTime)
	if err != nil {
		t.Fatalf("NewGammaPoint(offset) error = %v", err)
	}
	if offset.At().Location() != time.UTC {
		t.Fatal("NewGammaPoint() did not normalize to UTC")
	}
	if !bytes.Equal(utc.CanonicalBytes(), offset.CanonicalBytes()) {
		t.Fatal("equal instants in different zones produced different canonical bytes")
	}

	start := utc.At()
	end := start.Add(time.Hour)
	seen := make(map[string]struct{})
	for _, startBoundary := range []GammaBoundary{GammaBoundaryInclusive, GammaBoundaryExclusive} {
		for _, endBoundary := range []GammaBoundary{GammaBoundaryInclusive, GammaBoundaryExclusive} {
			window, windowErr := NewGammaWindow(start, end, startBoundary, endBoundary)
			if windowErr != nil {
				t.Fatalf("NewGammaWindow(%s,%s) error = %v", startBoundary.String(), endBoundary.String(), windowErr)
			}
			key := string(window.CanonicalBytes())
			if _, exists := seen[key]; exists {
				t.Fatal("distinct boundary semantics produced equal canonical bytes")
			}
			seen[key] = struct{}{}
		}
	}
	if len(seen) != 4 {
		t.Fatalf("boundary canonical variants = %d, want 4", len(seen))
	}
	_, missingBoundaryErr := NewGammaWindow(start, end, 0, GammaBoundaryExclusive)
	if missingBoundaryErr == nil {
		t.Fatal("NewGammaWindow() accepted an implicit start boundary")
	}
	_, reversedErr := NewGammaWindow(end, start, GammaBoundaryInclusive, GammaBoundaryExclusive)
	if reversedErr == nil {
		t.Fatal("NewGammaWindow() accepted a reversed interval")
	}
	_, degenerateErr := NewGammaWindow(start, start, GammaBoundaryInclusive, GammaBoundaryInclusive)
	if degenerateErr == nil {
		t.Fatal("NewGammaWindow() accepted a point encoded as a window")
	}
}

func mustContextSliceBuild(t *testing.T, input ContextSliceInput) ContextSlice {
	t.Helper()
	slice, err := NewContextSlice(input)
	if err != nil {
		t.Fatalf("NewContextSlice() error = %v", err)
	}
	return slice
}

func mustContextSliceContext(t *testing.T, raw string) BoundedContextRef {
	t.Helper()
	value, err := NewBoundedContextRef(raw)
	if err != nil {
		t.Fatalf("NewBoundedContextRef(%q) error = %v", raw, err)
	}
	return value
}

func mustContextSlicePoint(t *testing.T, raw string) GammaPoint {
	t.Helper()
	instant, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		t.Fatalf("time.Parse(%q) error = %v", raw, err)
	}
	point, err := NewGammaPoint(instant)
	if err != nil {
		t.Fatalf("NewGammaPoint(%q) error = %v", raw, err)
	}
	return point
}

func mustContextSliceCarrierRef(t *testing.T, raw string) CarrierRef {
	t.Helper()
	value, err := NewCarrierRef(raw)
	if err != nil {
		t.Fatalf("NewCarrierRef(%q) error = %v", raw, err)
	}
	return value
}

func mustContextSliceEdition(t *testing.T, raw string) CarrierEdition {
	t.Helper()
	value, err := NewCarrierEdition(raw)
	if err != nil {
		t.Fatalf("NewCarrierEdition(%q) error = %v", raw, err)
	}
	return value
}

func mustContextSliceDigest(t *testing.T, seed string) SHA256Digest {
	t.Helper()
	sum := sha256.Sum256([]byte(seed))
	raw := fmt.Sprintf("sha256:%x", sum[:])
	value, err := NewSHA256Digest(raw)
	if err != nil {
		t.Fatalf("NewSHA256Digest(%q) error = %v", raw, err)
	}
	return value
}

func mustContextSliceStandardPin(t *testing.T, ref, edition, digestSeed string) StandardPin {
	t.Helper()
	pin, err := NewStandardPin(
		mustContextSliceCarrierRef(t, ref),
		mustContextSliceEdition(t, edition),
		mustContextSliceDigest(t, digestSeed),
	)
	if err != nil {
		t.Fatalf("NewStandardPin(%q) error = %v", ref, err)
	}
	return pin
}

func mustContextSliceVocabularyPin(t *testing.T, ref, edition, digestSeed string) VocabularyPin {
	t.Helper()
	pin, err := NewVocabularyPin(
		mustContextSliceCarrierRef(t, ref),
		mustContextSliceEdition(t, edition),
		mustContextSliceDigest(t, digestSeed),
	)
	if err != nil {
		t.Fatalf("NewVocabularyPin(%q) error = %v", ref, err)
	}
	return pin
}

func mustContextSliceRoleSetPin(t *testing.T, ref, edition, digestSeed string) RoleSetPin {
	t.Helper()
	pin, err := NewRoleSetPin(
		mustContextSliceCarrierRef(t, ref),
		mustContextSliceEdition(t, edition),
		mustContextSliceDigest(t, digestSeed),
	)
	if err != nil {
		t.Fatalf("NewRoleSetPin(%q) error = %v", ref, err)
	}
	return pin
}

func mustContextSliceEnvironment(t *testing.T, key, value, digestSeed string) EnvironmentSelector {
	t.Helper()
	selectorKey, err := NewEnvironmentSelectorKey(key)
	if err != nil {
		t.Fatalf("NewEnvironmentSelectorKey(%q) error = %v", key, err)
	}
	selectorValue, err := NewEnvironmentSelectorValue(value)
	if err != nil {
		t.Fatalf("NewEnvironmentSelectorValue(%q) error = %v", value, err)
	}
	selector, err := NewEnvironmentSelector(
		selectorKey,
		selectorValue,
		mustContextSliceDigest(t, digestSeed),
	)
	if err != nil {
		t.Fatalf("NewEnvironmentSelector(%q) error = %v", key, err)
	}
	return selector
}
