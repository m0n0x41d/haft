package projecttypeenvcompatibility

import (
	"bytes"
	"testing"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestSuccessorDiffClosedTaxonomy(t *testing.T) {
	unchangedBase := buildCompatibilityFixture(t, newFixtureOptions())
	unchangedTarget := buildCompatibilityFixture(
		t,
		newFixtureOptions().withTypeEnvSeed("unchanged-successor"),
	)
	unchanged := mustCompareSuccessor(t, unchangedBase, unchangedTarget)
	assertEverySuccessorRuleClass(t, unchanged, SuccessorUnchanged)

	additiveTarget := buildCompatibilityFixture(
		t,
		newFixtureOptions().
			withFamily(KindDefinitionFamily, "v1").
			withTypeEnvSeed("additive-successor"),
	)
	additive := mustCompareSuccessor(t, unchangedBase, additiveTarget)
	assertSuccessorFamilyClass(t, additive, KindDefinitionFamily, SuccessorAdditive)

	removed := mustCompareSuccessor(t, additiveTarget, unchangedBase)
	assertSuccessorFamilyClass(t, removed, KindDefinitionFamily, SuccessorRemoved)

	narrowSlot := buildCompatibilityFixture(
		t,
		newFixtureOptions().
			withFamily(RelationSlotFamily, "v1").
			withTypeEnvSeed("narrow-slot"),
	)
	wideSlot := buildCompatibilityFixture(
		t,
		newFixtureOptions().
			withFamily(RelationSlotFamily, "v2").
			withTypeEnvSeed("wide-slot"),
	)
	widened := mustCompareSuccessor(t, narrowSlot, wideSlot)
	assertSuccessorFamilyClass(t, widened, RelationSlotFamily, SuccessorWidened)
	narrowed := mustCompareSuccessor(t, wideSlot, narrowSlot)
	assertSuccessorFamilyClass(t, narrowed, RelationSlotFamily, SuccessorNarrowed)

	codecV1 := buildCompatibilityFixture(
		t,
		newFixtureOptions().
			withFamily(ValueBindingFamily, "v1").
			withTypeEnvSeed("codec-v1"),
	)
	codecV2 := buildCompatibilityFixture(
		t,
		newFixtureOptions().
			withFamily(ValueBindingFamily, "v2").
			withTypeEnvSeed("codec-v2"),
	)
	compilerGap := mustCompareSuccessor(t, codecV1, codecV2)
	assertSuccessorFamilyClass(t, compilerGap, ValueBindingFamily, SuccessorCompilerGap)
	if !compilerGap.HasCompilerGap() {
		t.Fatal("compiler-gap successor did not retain its fail-closed posture")
	}
}

func TestSuccessorDiffClassifiesCandidateAndContextDomains(t *testing.T) {
	persisted := buildCompatibilityFixture(
		t,
		newFixtureOptions().
			withFamily(EntitySetDefinitionFamily, "v1").
			withTypeEnvSeed("persisted-entity-set"),
	)
	prospective := buildCompatibilityFixture(
		t,
		newFixtureOptions().
			withFamily(EntitySetDefinitionFamily, "policy-v2").
			withTypeEnvSeed("prospective-entity-set"),
	)
	widened := mustCompareSuccessor(t, persisted, prospective)
	assertSuccessorGround(
		t,
		widened,
		EntitySetDefinitionFamily,
		SuccessorWidened,
		GroundCandidateDomainExpanded,
	)
	narrowed := mustCompareSuccessor(t, prospective, persisted)
	assertSuccessorGround(
		t,
		narrowed,
		EntitySetDefinitionFamily,
		SuccessorNarrowed,
		GroundCandidateDomainRestricted,
	)

	leftContext := buildCompatibilityFixture(
		t,
		newFixtureOptions().
			withFamily(RelationSignatureFamily, "v1").
			withTypeEnvSeed("left-context"),
	)
	rightContext := buildCompatibilityFixture(
		t,
		newFixtureOptions().
			withFamily(RelationSignatureFamily, "v2").
			withTypeEnvSeed("right-context"),
	)
	incomparable := mustCompareSuccessor(t, leftContext, rightContext)
	assertSuccessorGround(
		t,
		incomparable,
		RelationSignatureFamily,
		SuccessorCompilerGap,
		GroundContextDomainsIncomparable,
	)
}

func TestRelationFragmentClassifierPreservesLegacySignatureSemantics(t *testing.T) {
	legacy := newCanonicalWriter("executable-typeenv.relation-signature.v1")
	legacy.addString("Haft.ConcernMemory")
	legacy.addUint64(1)
	legacy.addString("haft-project")

	current := newCanonicalWriter(
		"executable-typeenv.typed-relation-declaration-fragment.v1",
	)
	current.addString("Haft.ConcernMemory")
	current.addString(typedmemory.RelationDeclarationTypedFragment.String())
	current.addUint64(1)
	current.addString("haft-project")

	class, ground, err := classifyTypedRelationDeclarationFragmentChange(
		legacy.bytes(),
		current.bytes(),
	)
	if err != nil {
		t.Fatalf("classify legacy/current relation declaration: %v", err)
	}
	if class != SuccessorUnchanged || ground != GroundExactSemanticMatch {
		t.Fatalf("legacy/current classification = %s/%s", class.String(), ground)
	}
}

func TestSuccessorDiffCanonicalRoundTripAndPermutation(t *testing.T) {
	base := buildCompatibilityFixture(t, newFixtureOptions())
	targetOptions := newFixtureOptions().
		withAllOptionalFamilies("v1").
		withTypeEnvSeed("canonical-target")
	orderedTarget := buildCompatibilityFixture(t, targetOptions)
	permutedTarget := buildCompatibilityFixture(t, targetOptions.withReverseOrder())

	ordered := mustCompareSuccessor(t, base, orderedTarget)
	permuted := mustCompareSuccessor(t, base, permutedTarget)
	if !bytes.Equal(ordered.CanonicalBytes(), permuted.CanonicalBytes()) {
		t.Fatal("builder permutation changed successor canonical bytes")
	}
	if ordered.Digest() != permuted.Digest() {
		t.Fatal("builder permutation changed successor digest")
	}
	decoded, err := DecodeSuccessorDiff(ordered.CanonicalBytes())
	if err != nil {
		t.Fatalf("DecodeSuccessorDiff(): %v", err)
	}
	if decoded.Digest() != ordered.Digest() ||
		!bytes.Equal(decoded.CanonicalBytes(), ordered.CanonicalBytes()) {
		t.Fatal("successor canonical round-trip changed identity")
	}

	trailing := append(ordered.CanonicalBytes(), 0x01)
	if _, err := DecodeSuccessorDiff(trailing); err == nil {
		t.Fatal("DecodeSuccessorDiff accepted trailing bytes")
	}
}

func TestSuccessorDiffRejectsChangedSemanticsUnderOneTypeEnvRef(t *testing.T) {
	base := buildCompatibilityFixture(t, newFixtureOptions())
	changed := buildCompatibilityFixture(
		t,
		newFixtureOptions().withFamily(KindDefinitionFamily, "v1"),
	)
	if _, err := CompareSuccessor(base, changed); err == nil {
		t.Fatal("CompareSuccessor accepted two semantic surfaces under one TypeEnvRef")
	}
}

func mustCompareSuccessor(
	t *testing.T,
	before typedmemory.TypeEnv,
	after typedmemory.TypeEnv,
) SuccessorDiff {
	t.Helper()
	diff, err := CompareSuccessor(before, after)
	if err != nil {
		t.Fatalf("CompareSuccessor(): %v", err)
	}
	return diff
}

func assertEverySuccessorRuleClass(
	t *testing.T,
	diff SuccessorDiff,
	want SuccessorRuleClass,
) {
	t.Helper()
	for _, rule := range diff.Rules() {
		if rule.Class() != want {
			t.Fatalf("rule %s/%s = %s; want %s", rule.Family(), rule.Key(), rule.Class(), want)
		}
	}
}

func assertSuccessorFamilyClass(
	t *testing.T,
	diff SuccessorDiff,
	family Family,
	want SuccessorRuleClass,
) {
	t.Helper()
	for _, rule := range diff.Rules() {
		if rule.Family() == family && rule.Class() == want {
			return
		}
	}
	t.Fatalf("successor diff has no %s rule classified %s", family, want)
}

func assertSuccessorGround(
	t *testing.T,
	diff SuccessorDiff,
	family Family,
	class SuccessorRuleClass,
	ground SuccessorRuleGround,
) {
	t.Helper()
	for _, rule := range diff.Rules() {
		if rule.Family() == family && rule.Class() == class && rule.Ground() == ground {
			return
		}
	}
	t.Fatalf("successor diff has no %s %s rule grounded by %s", family, class, ground)
}
