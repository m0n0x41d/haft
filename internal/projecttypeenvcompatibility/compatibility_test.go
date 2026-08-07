package projecttypeenvcompatibility

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestCompareEqualExecutableTypeEnvs(t *testing.T) {
	left := buildCompatibilityFixture(t, newFixtureOptions())
	right := buildCompatibilityFixture(
		t,
		newFixtureOptions().withTypeEnvSeed("rebased-typeenv"),
	)

	diff, err := Compare(left, right)
	if err != nil {
		t.Fatalf("Compare(): %v", err)
	}
	if !diff.Empty() {
		t.Fatalf("changes = %#v; want empty", diff.Changes())
	}
	if diff.Base() != left.Ref() || diff.Target() != right.Ref() {
		t.Fatal("diff lost exact base or target TypeEnv coordinate")
	}
	if diff.Digest().String() == "" {
		t.Fatal("empty diff has no canonical digest")
	}
}

func TestCompareClassifiesEveryExecutableFamily(t *testing.T) {
	families := []Family{
		BoundedContextFamily,
		KindDefinitionFamily,
		EntitySetDefinitionFamily,
		KindSignatureDefinitionFamily,
		RefKindDefinitionFamily,
		ContextKindAvailabilityFamily,
		SubkindRelationFamily,
		ContextBridgeFamily,
		RelationSlotFamily,
		ValueShapeFamily,
		ValueBindingFamily,
		ConstraintFamily,
	}

	for _, family := range families {
		family := family
		t.Run(family.String(), func(t *testing.T) {
			without := buildCompatibilityFixture(t, newFixtureOptions())
			withV1 := buildCompatibilityFixture(
				t,
				newFixtureOptions().withFamily(family, "v1"),
			)
			withV2 := buildCompatibilityFixture(
				t,
				newFixtureOptions().withFamily(family, "v2"),
			)

			assertSingleFamilyChange(
				t,
				without,
				withV1,
				family,
				typedmemory.CompatibilityAdded,
			)
			assertSingleFamilyChange(
				t,
				withV1,
				without,
				family,
				typedmemory.CompatibilityRemoved,
			)
			if familySupportsChanged(family) {
				assertSingleFamilyChange(
					t,
					withV1,
					withV2,
					family,
					typedmemory.CompatibilityChanged,
				)
				return
			}
			if diff := mustCompare(t, withV1, withV2); !diff.Empty() {
				t.Fatalf(
					"metadata-only %s change = %s",
					family,
					formatChanges(diff.Changes()),
				)
			}
		})
	}
}

func TestCompareRelationSignatureIncludesItsMandatorySlot(t *testing.T) {
	without := buildCompatibilityFixture(t, newFixtureOptions())
	with := buildCompatibilityFixture(
		t,
		newFixtureOptions().withFamily(RelationSignatureFamily, "v1"),
	)
	diff := mustCompare(t, without, with)
	assertContainsChange(
		t,
		diff,
		RelationSignatureFamily,
		typedmemory.CompatibilityAdded,
	)
	assertContainsChange(
		t,
		diff,
		RelationSlotFamily,
		typedmemory.CompatibilityAdded,
	)
	if len(diff.Changes()) != 2 {
		t.Fatalf("relation addition changes = %s; want signature and mandatory slot", formatChanges(diff.Changes()))
	}

	changed := buildCompatibilityFixture(
		t,
		newFixtureOptions().withFamily(RelationSignatureFamily, "v2"),
	)
	assertSingleFamilyChange(
		t,
		with,
		changed,
		RelationSignatureFamily,
		typedmemory.CompatibilityChanged,
	)
}

func TestCompareIgnoresDocumentaryAndDerivationMetadata(t *testing.T) {
	before := buildCompatibilityFixture(t, newFixtureOptions())
	metadataChanged := buildCompatibilityFixture(
		t,
		newFixtureOptions().
			withSourceRevision("v2").
			withCompilerVersion("v2").
			withCoverageMetadata("v2").
			withTypeEnvSeed("metadata-rebased-typeenv"),
	)
	if diff := mustCompare(t, before, metadataChanged); !diff.Empty() {
		t.Fatalf("metadata-only TypeEnv changes = %s; want compatible", formatChanges(diff.Changes()))
	}

	for _, family := range []Family{
		BoundedContextFamily,
		KindDefinitionFamily,
		EntitySetDefinitionFamily,
		KindSignatureDefinitionFamily,
		RefKindDefinitionFamily,
		ContextKindAvailabilityFamily,
		SubkindRelationFamily,
		ContextBridgeFamily,
		RelationSignatureFamily,
		RelationSlotFamily,
		ValueShapeFamily,
		ValueBindingFamily,
		ConstraintFamily,
	} {
		family := family
		t.Run(family.String(), func(t *testing.T) {
			left := buildCompatibilityFixture(
				t,
				newFixtureOptions().withFamily(family, "v1"),
			)
			right := buildCompatibilityFixture(
				t,
				newFixtureOptions().withFamily(family, "metadata-v2"),
			)
			if diff := mustCompare(t, left, right); !diff.Empty() {
				t.Fatalf("documentary %s change = %s", family, formatChanges(diff.Changes()))
			}
		})
	}
}

func TestCompareIsInvariantToBuilderPermutation(t *testing.T) {
	base := buildCompatibilityFixture(t, newFixtureOptions())
	targetOptions := newFixtureOptions().withAllOptionalFamilies("v1")
	permutedOptions := targetOptions.withReverseOrder()
	target := buildCompatibilityFixture(t, targetOptions)
	permuted := buildCompatibilityFixture(t, permutedOptions)

	left, err := Compare(base, target)
	if err != nil {
		t.Fatalf("Compare(base, target): %v", err)
	}
	right, err := Compare(base, permuted)
	if err != nil {
		t.Fatalf("Compare(base, permuted): %v", err)
	}
	if !bytes.Equal(left.CanonicalBytes(), right.CanonicalBytes()) {
		t.Fatal("builder permutation changed canonical compatibility bytes")
	}
	if left.Digest() != right.Digest() {
		t.Fatal("builder permutation changed compatibility digest")
	}
}

func TestDiffAndChangesOwnCanonicalStorage(t *testing.T) {
	before := buildCompatibilityFixture(t, newFixtureOptions())
	after := buildCompatibilityFixture(
		t,
		newFixtureOptions().withFamily(ValueBindingFamily, "v1"),
	)
	diff, err := Compare(before, after)
	if err != nil {
		t.Fatalf("Compare(): %v", err)
	}
	expectedCanonical := diff.CanonicalBytes()
	expectedDigest := diff.Digest()
	changes := diff.Changes()
	if len(changes) != 1 {
		t.Fatalf("changes = %d; want 1", len(changes))
	}

	changeCanonical := changes[0].CanonicalBytes()
	changeCanonical[0] ^= 0xff
	changes[0] = nil
	diffCanonical := diff.CanonicalBytes()
	diffCanonical[0] ^= 0xff

	if !bytes.Equal(diff.CanonicalBytes(), expectedCanonical) {
		t.Fatal("caller mutation changed retained diff canonical bytes")
	}
	if diff.Digest() != expectedDigest {
		t.Fatal("caller mutation changed retained diff digest")
	}
	retained := diff.Changes()
	if len(retained) != 1 || retained[0] == nil {
		t.Fatal("caller mutation changed retained changes")
	}
	if bytes.Equal(changeCanonical, retained[0].CanonicalBytes()) {
		t.Fatal("change CanonicalBytes exposed mutable storage")
	}
}

func TestDecodeDiffRoundTripsExactBaseTargetAndChanges(t *testing.T) {
	before := buildCompatibilityFixture(t, newFixtureOptions())
	after := buildCompatibilityFixture(
		t,
		newFixtureOptions().withAllOptionalFamilies("v1"),
	)
	diff := mustCompare(t, before, after)

	decoded, err := DecodeDiff(diff.CanonicalBytes())
	if err != nil {
		t.Fatalf("DecodeDiff(): %v", err)
	}
	if decoded.Base() != before.Ref() || decoded.Target() != after.Ref() {
		t.Fatal("decoded diff lost exact base or target TypeEnv")
	}
	if decoded.Digest() != diff.Digest() {
		t.Fatal("decoded diff digest changed")
	}
	if !bytes.Equal(decoded.CanonicalBytes(), diff.CanonicalBytes()) {
		t.Fatal("decoded diff canonical bytes changed")
	}
	if len(decoded.Changes()) != len(diff.Changes()) {
		t.Fatalf(
			"decoded changes = %d; want %d",
			len(decoded.Changes()),
			len(diff.Changes()),
		)
	}
}

func TestDecodeDiffRejectsTrailingBytesAndNonCanonicalChangeOrder(t *testing.T) {
	before := buildCompatibilityFixture(t, newFixtureOptions())
	after := buildCompatibilityFixture(
		t,
		newFixtureOptions().withAllOptionalFamilies("v1"),
	)
	diff := mustCompare(t, before, after)
	if len(diff.Changes()) < 2 {
		t.Fatal("fixture requires at least two compatibility changes")
	}

	trailing := append(diff.CanonicalBytes(), 0x01)
	if _, err := DecodeDiff(trailing); err == nil {
		t.Fatal("DecodeDiff accepted trailing bytes")
	}

	changes := diff.Changes()
	writer := newCanonicalWriter(diffCanonicalDomain)
	writer.addString(diff.Base().String())
	writer.addString(diff.Target().String())
	writer.addUint64(uint64(len(changes)))
	for index := len(changes) - 1; index >= 0; index-- {
		writer.addBytes(changes[index].CanonicalBytes())
	}
	if _, err := DecodeDiff(writer.bytes()); err == nil {
		t.Fatal("DecodeDiff accepted non-canonical change order")
	}
}

func TestDiffIdentityBindsExactTargetTypeEnv(t *testing.T) {
	base := buildCompatibilityFixture(t, newFixtureOptions())
	firstTarget := buildCompatibilityFixture(
		t,
		newFixtureOptions().
			withFamily(ValueBindingFamily, "v1").
			withTypeEnvSeed("target-a"),
	)
	secondTarget := buildCompatibilityFixture(
		t,
		newFixtureOptions().
			withFamily(ValueBindingFamily, "v1").
			withTypeEnvSeed("target-b"),
	)

	first := mustCompare(t, base, firstTarget)
	second := mustCompare(t, base, secondTarget)
	if first.Target() == second.Target() {
		t.Fatal("fixture targets unexpectedly share a TypeEnv ref")
	}
	if first.Digest() == second.Digest() {
		t.Fatal("different exact target TypeEnv refs share a diff digest")
	}
	if bytes.Equal(first.CanonicalBytes(), second.CanonicalBytes()) {
		t.Fatal("different exact target TypeEnv refs share canonical diff bytes")
	}
}

func TestDiffFingerprintsAreRefAndDigestSensitive(t *testing.T) {
	base := buildCompatibilityFixture(
		t,
		newFixtureOptions().withFamily(ValueBindingFamily, "v1"),
	)
	codecChanged := buildCompatibilityFixture(
		t,
		newFixtureOptions().withFamily(ValueBindingFamily, "v2"),
	)
	shapeV1 := buildCompatibilityFixture(
		t,
		newFixtureOptions().withFamily(ValueShapeFamily, "payload-v1"),
	)
	shapeV2 := buildCompatibilityFixture(
		t,
		newFixtureOptions().withFamily(ValueShapeFamily, "payload-v2"),
	)
	rebased := buildCompatibilityFixture(
		t,
		newFixtureOptions().
			withFamily(ValueBindingFamily, "v1").
			withTypeEnvSeed("other-typeenv"),
	)
	rebasedCodecChanged := buildCompatibilityFixture(
		t,
		newFixtureOptions().
			withFamily(ValueBindingFamily, "v2").
			withTypeEnvSeed("other-typeenv"),
	)

	codecDiff := mustCompare(t, base, codecChanged)
	shapeDiff := mustCompare(t, shapeV1, shapeV2)
	rebaseDiff := mustCompare(t, base, rebased)
	rebasedCodecDiff := mustCompare(t, rebased, rebasedCodecChanged)

	assertChangedDigestsDiffer(t, codecDiff, ValueBindingFamily)
	assertShapeCoordinateReplacement(t, shapeDiff)
	if !rebaseDiff.Empty() {
		t.Fatalf("owning TypeEnvRef rebase produced semantic changes: %s", formatChanges(rebaseDiff.Changes()))
	}
	if codecDiff.Digest() == rebasedCodecDiff.Digest() {
		t.Fatal("different exact base TypeEnv refs share a diff digest")
	}
	if _, err := typedmemory.NewSHA256Digest(codecDiff.Digest().String()); err != nil {
		t.Fatalf("diff digest is not canonical SHA-256: %v", err)
	}
}

func TestCompareRetainsProspectiveEntitySetPolicy(t *testing.T) {
	persistedOnly := buildCompatibilityFixture(
		t,
		newFixtureOptions().withFamily(EntitySetDefinitionFamily, "v1"),
	)
	prospective := buildCompatibilityFixture(
		t,
		newFixtureOptions().withFamily(EntitySetDefinitionFamily, "policy-v2"),
	)

	assertSingleFamilyChange(
		t,
		persistedOnly,
		prospective,
		EntitySetDefinitionFamily,
		typedmemory.CompatibilityChanged,
	)
}

func TestCanonicalRelationSlotRetainsTargetModeAndUnboundedCardinality(
	t *testing.T,
) {
	leftOwner := testTypeEnvRef(t, "relation-slot-left-owner")
	rightOwner := testTypeEnvRef(t, "relation-slot-right-owner")
	left := testRelationSlot(
		t,
		leftOwner,
		false,
		mustFixture(typedmemory.NewBoundedCardinality(0, 1)),
	)
	leftRebased := testRelationSlot(
		t,
		rightOwner,
		false,
		mustFixture(typedmemory.NewBoundedCardinality(0, 1)),
	)
	byReference := testRelationSlot(
		t,
		leftOwner,
		true,
		typedmemory.NewUnboundedCardinality(0),
	)
	signature := mustFixture(typedmemory.NewSignatureID("Relation.SlotProbe"))

	leftCanonical := mustFixture(
		canonicalRelationSlot(leftOwner, signature, left),
	)
	rebasedCanonical := mustFixture(
		canonicalRelationSlot(rightOwner, signature, leftRebased),
	)
	referenceCanonical := mustFixture(
		canonicalRelationSlot(leftOwner, signature, byReference),
	)

	if !bytes.Equal(leftCanonical, rebasedCanonical) {
		t.Fatal("local relation-slot TypeEnv owner was not normalized")
	}
	if bytes.Equal(leftCanonical, referenceCanonical) {
		t.Fatal("reference target and unbounded cardinality were erased")
	}
	if _, err := canonicalRelationSlot(rightOwner, signature, left); err == nil {
		t.Fatal("relation-slot projection accepted an unexpected local owner")
	}
}

func TestCanonicalConstraintCoversClosedVariantsAndNormalizesOwners(
	t *testing.T,
) {
	leftOwner := testTypeEnvRef(t, "constraint-left-owner")
	rightOwner := testTypeEnvRef(t, "constraint-right-owner")
	left := testConstraintVariants(t, leftOwner, "v1")
	rebased := testConstraintVariants(t, rightOwner, "v1")
	changed := testConstraintVariants(t, leftOwner, "v2")

	if len(left) != 5 || len(rebased) != len(left) || len(changed) != len(left) {
		t.Fatal("constraint fixture does not cover the closed five-variant algebra")
	}
	for index := range left {
		leftCanonical := mustFixture(canonicalConstraint(leftOwner, left[index]))
		rebasedCanonical := mustFixture(
			canonicalConstraint(rightOwner, rebased[index]),
		)
		changedCanonical := mustFixture(
			canonicalConstraint(leftOwner, changed[index]),
		)
		if !bytes.Equal(leftCanonical, rebasedCanonical) {
			t.Fatalf("constraint variant %d retained its local TypeEnv owner", index)
		}
		if bytes.Equal(leftCanonical, changedCanonical) {
			t.Fatalf("constraint variant %d erased behavior-bearing operands", index)
		}
	}
	if _, err := canonicalConstraint(rightOwner, left[1]); err == nil {
		t.Fatal("constraint projection accepted an unexpected relation owner")
	}
}

func TestCompareRejectsMissingExecutableTypeEnv(t *testing.T) {
	valid := buildCompatibilityFixture(t, newFixtureOptions())
	if _, err := Compare(typedmemory.TypeEnv{}, valid); err == nil {
		t.Fatal("Compare accepted a missing previous TypeEnv")
	}
	if _, err := Compare(valid, typedmemory.TypeEnv{}); err == nil {
		t.Fatal("Compare accepted a missing current TypeEnv")
	}
}

func assertSingleFamilyChange(
	t *testing.T,
	before typedmemory.TypeEnv,
	after typedmemory.TypeEnv,
	family Family,
	kind typedmemory.CompatibilityChangeKind,
) {
	t.Helper()
	diff := mustCompare(t, before, after)
	changes := diff.Changes()
	if len(changes) != 1 {
		t.Fatalf("changes = %d (%s); want one %s %s", len(changes), formatChanges(changes), kind, family)
	}
	change := changes[0]
	if change.Family() != family || change.Kind() != kind {
		t.Fatalf(
			"change = %s/%s; want %s/%s",
			change.Family(),
			change.Kind(),
			family,
			kind,
		)
	}
	beforeDigest, hasBefore := change.BeforeDigest()
	afterDigest, hasAfter := change.AfterDigest()
	if kind == typedmemory.CompatibilityAdded && (hasBefore || !hasAfter) {
		t.Fatal("added change has an illegal before/after fingerprint state")
	}
	if kind == typedmemory.CompatibilityRemoved && (!hasBefore || hasAfter) {
		t.Fatal("removed change has an illegal before/after fingerprint state")
	}
	if kind == typedmemory.CompatibilityChanged &&
		(!hasBefore || !hasAfter || beforeDigest == afterDigest) {
		t.Fatal("changed change does not retain two distinct fingerprints")
	}
}

func assertContainsChange(
	t *testing.T,
	diff Diff,
	family Family,
	kind typedmemory.CompatibilityChangeKind,
) {
	t.Helper()
	for _, change := range diff.Changes() {
		if change.Family() == family && change.Kind() == kind {
			return
		}
	}
	t.Fatalf(
		"changes = %s; want %s %s",
		formatChanges(diff.Changes()),
		kind,
		family,
	)
}

func assertChangedDigestsDiffer(t *testing.T, diff Diff, family Family) {
	t.Helper()
	for _, change := range diff.Changes() {
		if change.Family() != family {
			continue
		}
		before, hasBefore := change.BeforeDigest()
		after, hasAfter := change.AfterDigest()
		if !hasBefore || !hasAfter || before == after {
			t.Fatalf("%s change does not retain distinct exact digests", family)
		}
		return
	}
	t.Fatalf("diff has no %s change: %s", family, formatChanges(diff.Changes()))
}

func assertShapeCoordinateReplacement(t *testing.T, diff Diff) {
	t.Helper()
	changes := diff.Changes()
	if len(changes) != 2 {
		t.Fatalf("shape replacement changes = %s; want remove+add", formatChanges(changes))
	}
	assertContainsChange(
		t,
		diff,
		ValueShapeFamily,
		typedmemory.CompatibilityRemoved,
	)
	assertContainsChange(
		t,
		diff,
		ValueShapeFamily,
		typedmemory.CompatibilityAdded,
	)
}

func familySupportsChanged(family Family) bool {
	switch family {
	case EntitySetDefinitionFamily,
		KindSignatureDefinitionFamily,
		RefKindDefinitionFamily,
		ContextBridgeFamily,
		RelationSlotFamily,
		ValueBindingFamily,
		ConstraintFamily:
		return true
	default:
		return false
	}
}

func mustCompare(
	t *testing.T,
	before typedmemory.TypeEnv,
	after typedmemory.TypeEnv,
) Diff {
	t.Helper()
	diff, err := Compare(before, after)
	if err != nil {
		t.Fatalf("Compare(): %v", err)
	}
	return diff
}

func formatChanges(changes []Change) string {
	values := make([]string, 0, len(changes))
	for _, change := range changes {
		values = append(
			values,
			change.Family().String()+":"+change.Key()+":"+change.Kind().String(),
		)
	}
	return strings.Join(values, ", ")
}

func testRelationSlot(
	t *testing.T,
	owner typedmemory.TypeEnvRef,
	byReference bool,
	cardinality typedmemory.Cardinality,
) typedmemory.SlotSpec {
	t.Helper()
	valueKind := mustFixture(
		typedmemory.NewValueKindRef(
			owner,
			mustFixture(typedmemory.NewKindID("U.RelationSlotProbe")),
		),
	)
	target := typedmemory.SlotTarget(
		mustFixture(typedmemory.NewValueSlotTarget(valueKind)),
	)
	if byReference {
		target = mustFixture(
			typedmemory.NewReferenceSlotTarget(
				valueKind,
				mustFixture(
					typedmemory.NewRefKindRef(
						owner,
						mustFixture(typedmemory.NewRefKindID("Ref.RelationSlotProbe")),
					),
				),
			),
		)
	}
	return mustFixture(
		typedmemory.NewSlotSpec(
			mustFixture(typedmemory.NewSlotKindID("probe")),
			target,
			cardinality,
			testProvenance(t, "relation-slot-probe"),
		),
	)
}

func testConstraintVariants(
	t *testing.T,
	owner typedmemory.TypeEnvRef,
	variant string,
) []typedmemory.ConstraintRule {
	t.Helper()
	signature := mustFixture(
		typedmemory.NewRelationSignatureRef(
			owner,
			mustFixture(typedmemory.NewSignatureID("Relation.ConstraintProbe")),
		),
	)
	slotAlpha := mustFixture(typedmemory.NewSlotKindID("alpha"))
	slotBeta := mustFixture(typedmemory.NewSlotKindID("beta"))
	slotGamma := mustFixture(typedmemory.NewSlotKindID("gamma"))
	slotWhole := mustFixture(typedmemory.NewSlotKindID("whole"))
	kindA := mustFixture(typedmemory.NewKindID("U.A"))
	kindB := mustFixture(typedmemory.NewKindID("U.B"))
	kindC := mustFixture(typedmemory.NewKindID("U.C"))
	provenance := testProvenance(t, "constraint-variant-"+variant)

	disjointKinds := []typedmemory.KindID{kindA, kindB}
	groupMode := typedmemory.SlotGroupExactlyOne
	cardinality := mustFixture(typedmemory.NewBoundedCardinality(0, 1))
	subset := slotAlpha
	superset := slotBeta
	partitionParts := []typedmemory.SlotKindID{slotAlpha, slotBeta}
	if variant == "v2" {
		disjointKinds = []typedmemory.KindID{kindA, kindC}
		groupMode = typedmemory.SlotGroupAllOrNone
		cardinality = typedmemory.NewUnboundedCardinality(0)
		subset = slotBeta
		superset = slotAlpha
		partitionParts = []typedmemory.SlotKindID{slotAlpha, slotGamma}
	}

	return []typedmemory.ConstraintRule{
		mustFixture(
			typedmemory.NewKindDisjointConstraint(
				mustFixture(typedmemory.NewConstraintID("constraint.disjoint")),
				disjointKinds,
				provenance,
			),
		),
		mustFixture(
			typedmemory.NewSlotGroupConstraint(
				mustFixture(typedmemory.NewConstraintID("constraint.group")),
				signature,
				[]typedmemory.SlotKindID{slotAlpha, slotBeta},
				groupMode,
				provenance,
			),
		),
		mustFixture(
			typedmemory.NewSlotCardinalityConstraint(
				mustFixture(typedmemory.NewConstraintID("constraint.cardinality")),
				signature,
				slotAlpha,
				cardinality,
				provenance,
			),
		),
		mustFixture(
			typedmemory.NewReferenceSlotSubsetConstraint(
				mustFixture(typedmemory.NewConstraintID("constraint.subset")),
				signature,
				subset,
				superset,
				provenance,
			),
		),
		mustFixture(
			typedmemory.NewReferenceSlotPartitionConstraint(
				mustFixture(typedmemory.NewConstraintID("constraint.partition")),
				signature,
				slotWhole,
				partitionParts,
				provenance,
			),
		),
	}
}

type fixtureOptions struct {
	included             map[Family]bool
	variants             map[Family]string
	reverse              bool
	typeEnvSeed          string
	sourceRevision       string
	compilerVersion      string
	coverageMetadataSeed string
}

func newFixtureOptions() fixtureOptions {
	return fixtureOptions{
		included:             map[Family]bool{},
		variants:             map[Family]string{},
		typeEnvSeed:          "typeenv-v1",
		sourceRevision:       "v1",
		compilerVersion:      "v1",
		coverageMetadataSeed: "v1",
	}
}

func (options fixtureOptions) withFamily(family Family, variant string) fixtureOptions {
	result := options.clone()
	result.included[family] = true
	result.variants[family] = variant
	return result
}

func (options fixtureOptions) withSourceRevision(variant string) fixtureOptions {
	result := options.clone()
	result.sourceRevision = variant
	return result
}

func (options fixtureOptions) withCompilerVersion(variant string) fixtureOptions {
	result := options.clone()
	result.compilerVersion = variant
	return result
}

func (options fixtureOptions) withCoverageMetadata(variant string) fixtureOptions {
	result := options.clone()
	result.coverageMetadataSeed = variant
	return result
}

func (options fixtureOptions) withAllOptionalFamilies(variant string) fixtureOptions {
	result := options.clone()
	for _, family := range []Family{
		BoundedContextFamily,
		KindDefinitionFamily,
		EntitySetDefinitionFamily,
		KindSignatureDefinitionFamily,
		RefKindDefinitionFamily,
		ContextKindAvailabilityFamily,
		SubkindRelationFamily,
		ContextBridgeFamily,
		RelationSignatureFamily,
		RelationSlotFamily,
		ValueShapeFamily,
		ValueBindingFamily,
		ConstraintFamily,
	} {
		result.included[family] = true
		result.variants[family] = variant
	}
	return result
}

func (options fixtureOptions) withReverseOrder() fixtureOptions {
	result := options.clone()
	result.reverse = true
	return result
}

func (options fixtureOptions) withTypeEnvSeed(seed string) fixtureOptions {
	result := options.clone()
	result.typeEnvSeed = seed
	return result
}

func (options fixtureOptions) clone() fixtureOptions {
	included := make(map[Family]bool, len(options.included))
	for family, present := range options.included {
		included[family] = present
	}
	variants := make(map[Family]string, len(options.variants))
	for family, variant := range options.variants {
		variants[family] = variant
	}
	return fixtureOptions{
		included:             included,
		variants:             variants,
		reverse:              options.reverse,
		typeEnvSeed:          options.typeEnvSeed,
		sourceRevision:       options.sourceRevision,
		compilerVersion:      options.compilerVersion,
		coverageMetadataSeed: options.coverageMetadataSeed,
	}
}

func (options fixtureOptions) includes(family Family) bool {
	return options.included[family]
}

func (options fixtureOptions) variant(family Family) string {
	value := options.variants[family]
	if value == "" {
		return "v1"
	}
	return value
}

type compatibilityFixture struct {
	ref                typedmemory.TypeEnvRef
	coverage           []typedmemory.CoverageEntry
	contexts           []typedmemory.BoundedContext
	kinds              []typedmemory.KindDefinition
	entitySets         []typedmemory.EntitySetDefinition
	kindSignatures     []typedmemory.KindSignatureDefinition
	refKinds           []typedmemory.RefKindDefinition
	availabilities     []typedmemory.ContextKindAvailability
	subkinds           []typedmemory.SubkindRelation
	bridges            []typedmemory.ContextBridge
	relations          []typedmemory.RelationSignature
	shapes             []typedmemory.ValueShapeDeclaration
	bindings           []typedmemory.ValueBinding
	constraints        []typedmemory.ConstraintRule
	contextByName      map[string]typedmemory.BoundedContextRef
	kindByName         map[string]typedmemory.KindID
	entitySetByContext map[string]typedmemory.EntitySetDefinition
	shapeByName        map[string]typedmemory.ValueShapeDeclaration
}

func buildCompatibilityFixture(
	t *testing.T,
	options fixtureOptions,
) typedmemory.TypeEnv {
	t.Helper()
	fixture := newCompatibilityFixture(t, options)
	if options.reverse {
		fixture.reverse()
	}
	revision := mustFixture(
		typedmemory.NewSourceRevision("fixture-source-" + options.sourceRevision),
	)

	compiler := mustFixture(
		typedmemory.NewCompilerSchemaVersion(
			"fixture-compiler-" + options.compilerVersion,
		),
	)

	coverage := mustFixture(typedmemory.NewCoverageManifest(fixture.coverage))
	builder := typedmemory.NewTypeEnvBuilder(fixture.ref)
	builder.SetSourceRevision(revision)
	builder.SetCompilerSchemaVersion(compiler)
	builder.SetCoverageManifest(coverage)
	for _, value := range fixture.contexts {
		builder.AddBoundedContext(value)
	}
	for _, value := range fixture.kinds {
		builder.AddKindDefinition(value)
	}
	for _, value := range fixture.entitySets {
		builder.AddEntitySetDefinition(value)
	}
	for _, value := range fixture.kindSignatures {
		builder.AddKindSignatureDefinition(value)
	}
	for _, value := range fixture.refKinds {
		builder.AddRefKindDefinition(value)
	}
	for _, value := range fixture.availabilities {
		builder.AddContextKindAvailability(value)
	}
	for _, value := range fixture.subkinds {
		builder.AddSubkindRelation(value)
	}
	for _, value := range fixture.bridges {
		builder.AddContextBridge(value)
	}
	for _, value := range fixture.relations {
		builder.AddRelationSignature(value)
	}
	for _, value := range fixture.shapes {
		builder.AddValueShape(value)
	}
	for _, value := range fixture.bindings {
		builder.AddValueBinding(value)
	}
	for _, value := range fixture.constraints {
		builder.AddConstraint(value)
	}
	return mustFixture(builder.Build())
}

func newCompatibilityFixture(
	t *testing.T,
	options fixtureOptions,
) compatibilityFixture {
	t.Helper()
	fixture := compatibilityFixture{
		ref:                testTypeEnvRef(t, options.typeEnvSeed),
		contextByName:      map[string]typedmemory.BoundedContextRef{},
		kindByName:         map[string]typedmemory.KindID{},
		entitySetByContext: map[string]typedmemory.EntitySetDefinition{},
		shapeByName:        map[string]typedmemory.ValueShapeDeclaration{},
	}
	fixture.coverage = fixtureCoverage(t, options)
	fixture.addContexts(t, options)
	fixture.addKinds(t, options)
	fixture.addEntitySets(t, options)
	fixture.addAvailabilities(t, options)
	fixture.addKindSignatures(t, options)
	fixture.addRefKinds(t, options)
	fixture.addSubkinds(t, options)
	fixture.addBridges(t, options)
	fixture.addRelations(t, options)
	fixture.addShapes(t, options)
	fixture.addBindings(t, options)
	fixture.addConstraints(t, options)
	return fixture
}

func fixtureCoverage(
	t *testing.T,
	options fixtureOptions,
) []typedmemory.CoverageEntry {
	t.Helper()
	baseSubject := mustFixture(
		typedmemory.SourceUnitCoverage(mustFixture(
			typedmemory.NewSourceUnitID("fixture.coverage.base"),
		)),
	)
	base := mustFixture(
		typedmemory.NewCompiledCoverageEntry(
			baseSubject,
			testSourceLocation(t, "coverage-base"),
		),
	)
	entries := []typedmemory.CoverageEntry{base}
	subject := mustFixture(
		typedmemory.SourceUnitCoverage(mustFixture(
			typedmemory.NewSourceUnitID("fixture.coverage.optional"),
		)),
	)
	optional := mustFixture(
		typedmemory.NewCompiledCoverageEntry(
			subject,
			testSourceLocation(
				t,
				"coverage-optional-"+options.coverageMetadataSeed,
			),
		),
	)
	return append(entries, optional)
}

func (fixture *compatibilityFixture) addContexts(
	t *testing.T,
	options fixtureOptions,
) {
	t.Helper()
	for _, name := range []string{
		"ctx.left",
		"ctx.right",
		"ctx.signature",
		"ctx.entity",
		"ctx.availability",
	} {
		fixture.addContext(t, name, "base")
	}
	if options.includes(BoundedContextFamily) {
		fixture.addContext(
			t,
			"ctx.optional",
			options.variant(BoundedContextFamily),
		)
	}
}

func (fixture *compatibilityFixture) addContext(
	t *testing.T,
	name string,
	provenanceVariant string,
) {
	t.Helper()
	ref := mustFixture(typedmemory.NewBoundedContextRef(name))
	context := mustFixture(

		typedmemory.NewBoundedContext(
			ref,
			testProvenance(t, "context-"+name+"-"+provenanceVariant),
		))

	fixture.contexts = append(fixture.contexts, context)
	fixture.contextByName[name] = ref
}

func (fixture *compatibilityFixture) addKinds(
	t *testing.T,
	options fixtureOptions,
) {
	t.Helper()
	for _, name := range []string{
		"U.A",
		"U.B",
		"U.C",
		"U.D",
		"U.E",
	} {
		fixture.addKind(t, name, "base")
	}
	if options.includes(KindDefinitionFamily) {
		fixture.addKind(
			t,
			"U.Optional",
			options.variant(KindDefinitionFamily),
		)
	}
}

func (fixture *compatibilityFixture) addKind(
	t *testing.T,
	name string,
	provenanceVariant string,
) {
	t.Helper()
	id := mustFixture(typedmemory.NewKindID(name))
	definition := mustFixture(

		typedmemory.NewKindDefinition(
			id,
			testProvenance(t, "kind-"+name+"-"+provenanceVariant),
		))

	fixture.kinds = append(fixture.kinds, definition)
	fixture.kindByName[name] = id
}

func (fixture *compatibilityFixture) addEntitySets(
	t *testing.T,
	options fixtureOptions,
) {
	t.Helper()
	for _, contextName := range []string{
		"ctx.left",
		"ctx.right",
		"ctx.signature",
	} {
		fixture.addEntitySet(t, contextName, "base")
	}
	if options.includes(EntitySetDefinitionFamily) {
		fixture.addEntitySet(
			t,
			"ctx.entity",
			options.variant(EntitySetDefinitionFamily),
		)
	}
}

func (fixture *compatibilityFixture) addEntitySet(
	t *testing.T,
	contextName string,
	variant string,
) {
	t.Helper()
	context := fixture.contextByName[contextName]
	behaviorVariant := fixtureBehaviorVariant(variant)
	policy := typedmemory.EntitySetCandidatePolicy(
		typedmemory.PersistedEntitiesOnly{},
	)
	if variant == "policy-v2" {
		behaviorVariant = "v1"
		policy = mustFixture(
			typedmemory.NewPriorBatchDeclarationsVisible(
				mustFixture(
					typedmemory.NewRuleRef(
						"rule.entity-set.prospective.v2",
					),
				),
			),
		)
	}
	rule := mustFixture(
		typedmemory.NewRuleRef(
			"rule.entity-set." + strings.ReplaceAll(behaviorVariant, "-", "."),
		),
	)

	definition := mustFixture(

		typedmemory.NewEntitySetDefinition(typedmemory.EntitySetDefinitionInput{
			TypeEnv:         fixture.ref,
			Context:         context,
			EnumerationRule: rule,
			CandidatePolicy: policy,
			Provenance:      testProvenance(t, "entity-set-"+contextName+"-"+variant),
		}))

	fixture.entitySets = append(fixture.entitySets, definition)
	fixture.entitySetByContext[contextName] = definition
}

func (fixture *compatibilityFixture) addAvailabilities(
	t *testing.T,
	options fixtureOptions,
) {
	t.Helper()
	fixture.addAvailability(t, "ctx.left", "U.A", "left-base")
	fixture.addAvailability(t, "ctx.right", "U.B", "right-base")
	fixture.addAvailability(t, "ctx.signature", "U.C", "signature-base")
	if options.includes(ContextKindAvailabilityFamily) {
		fixture.addAvailability(
			t,
			"ctx.availability",
			"U.E",
			options.variant(ContextKindAvailabilityFamily),
		)
	}
}

func (fixture *compatibilityFixture) addAvailability(
	t *testing.T,
	contextName string,
	kindName string,
	variant string,
) {
	t.Helper()
	availability := testContextKindAvailability(
		t,
		fixture.ref,
		fixture.contextByName[contextName],
		fixture.kindByName[kindName],
		contextName+"-"+kindName+"-"+variant,
	)
	fixture.availabilities = append(fixture.availabilities, availability)
}

func (fixture *compatibilityFixture) addKindSignatures(
	t *testing.T,
	options fixtureOptions,
) {
	t.Helper()
	fixture.addKindSignature(t, "ctx.left", "U.A", "base")
	fixture.addKindSignature(t, "ctx.right", "U.B", "base")
	if options.includes(KindSignatureDefinitionFamily) {
		fixture.addKindSignature(
			t,
			"ctx.signature",
			"U.C",
			options.variant(KindSignatureDefinitionFamily),
		)
	}
}

func (fixture *compatibilityFixture) addKindSignature(
	t *testing.T,
	contextName string,
	kindName string,
	variant string,
) {
	t.Helper()
	behaviorVariant := fixtureBehaviorVariant(variant)
	kind := mustFixture(

		typedmemory.NewValueKindRef(fixture.ref, fixture.kindByName[kindName]))

	formality := mustFixture(typedmemory.NewSignatureFormality(3))
	definedness := mustFixture(
		typedmemory.NewRuleRef(
			"rule.kind-signature.defined." + behaviorVariant,
		),
	)

	evaluator := mustFixture(
		typedmemory.NewRuleRef(
			"rule.kind-signature.evaluate." + behaviorVariant,
		),
	)

	definition := mustFixture(

		typedmemory.NewKindSignatureDefinition(typedmemory.KindSignatureDefinitionInput{
			ValueKind:       kind,
			Formality:       formality,
			DefinednessRule: definedness,
			Evaluator:       evaluator,
			EntitySet:       fixture.entitySetByContext[contextName].Ref(),
			Provenance:      testProvenance(t, "kind-signature-"+contextName+"-"+kindName+"-"+variant),
		}))

	fixture.kindSignatures = append(fixture.kindSignatures, definition)
}

func (fixture *compatibilityFixture) addRefKinds(
	t *testing.T,
	options fixtureOptions,
) {
	t.Helper()
	if !options.includes(RefKindDefinitionFamily) {
		return
	}
	variant := options.variant(RefKindDefinitionFamily)
	valueKindName := "U.D"
	if variant == "v2" {
		valueKindName = "U.E"
	}
	ref := mustFixture(

		typedmemory.NewRefKindRef(
			fixture.ref,
			mustFixture(typedmemory.NewRefKindID("Ref.Optional")),
		))

	valueKind := mustFixture(

		typedmemory.NewValueKindRef(fixture.ref, fixture.kindByName[valueKindName]))

	definition := mustFixture(

		typedmemory.NewRefKindDefinition(
			ref,
			valueKind,
			testProvenance(t, "ref-kind-"+variant),
		))

	fixture.refKinds = append(fixture.refKinds, definition)
}

func (fixture *compatibilityFixture) addSubkinds(
	t *testing.T,
	options fixtureOptions,
) {
	t.Helper()
	if !options.includes(SubkindRelationFamily) {
		return
	}
	variant := options.variant(SubkindRelationFamily)
	relation := mustFixture(

		typedmemory.NewSubkindRelation(
			fixture.kindByName["U.D"],
			fixture.kindByName["U.A"],
			testProvenance(t, "subkind-"+variant),
		))

	fixture.subkinds = append(fixture.subkinds, relation)
}

func (fixture *compatibilityFixture) addBridges(
	t *testing.T,
	options fixtureOptions,
) {
	t.Helper()
	if !options.includes(ContextBridgeFamily) {
		return
	}
	variant := options.variant(ContextBridgeFamily)
	behaviorVariant := fixtureBehaviorVariant(variant)
	source := mustFixture(

		typedmemory.NewContextBridgeEndpoint(
			fixture.contextByName["ctx.left"],
			mustFixture(typedmemory.NewContextEdition("2026.1")),
		))

	target := mustFixture(

		typedmemory.NewContextBridgeEndpoint(
			fixture.contextByName["ctx.right"],
			mustFixture(typedmemory.NewContextEdition("2026.1")),
		))

	mapping := mustFixture(

		typedmemory.NewNamedTargetKindMapping(
			fixture.kindByName["U.A"],
			fixture.kindByName["U.B"],
		))

	congruence := mustFixture(typedmemory.NewKindCongruenceLevel(2))
	lossNotes := mustFixture(
		typedmemory.NewKindBridgeLossNotes([]string{"loss-" + behaviorVariant}),
	)

	definedness := mustFixture(
		typedmemory.NewKindBridgeDefinednessArea(
			[]string{"defined-" + behaviorVariant},
		),
	)

	bridge := mustFixture(

		typedmemory.NewContextBridge(typedmemory.ContextBridgeInput{
			ID:              mustFixture(typedmemory.NewContextBridgeID("bridge.optional")),
			Source:          source,
			Target:          target,
			Mapping:         mapping,
			Direction:       typedmemory.OneWayBridge,
			OrderCoverage:   typedmemory.NoOrderLinksCovered,
			KindCongruence:  congruence,
			LossNotes:       lossNotes,
			DefinednessArea: definedness,
			Provenance:      testProvenance(t, "bridge-"+variant),
		}))

	fixture.bridges = append(fixture.bridges, bridge)
}

func (fixture *compatibilityFixture) addRelations(
	t *testing.T,
	options fixtureOptions,
) {
	t.Helper()
	baseSlots := []typedmemory.SlotSpec{
		fixture.relationSlot(t, "subject", 1, "base"),
	}
	if options.includes(RelationSlotFamily) {
		variant := options.variant(RelationSlotFamily)
		maximum := uint64(1)
		if variant == "v2" {
			maximum = 2
		}
		baseSlots = append(
			baseSlots,
			fixture.relationSlot(t, "object", maximum, variant),
		)
	}
	fixture.addRelation(
		t,
		"Relation.Base",
		[]typedmemory.BoundedContextRef{fixture.contextByName["ctx.left"]},
		baseSlots,
		"base",
	)
	if !options.includes(RelationSignatureFamily) {
		return
	}
	variant := options.variant(RelationSignatureFamily)
	behaviorVariant := fixtureBehaviorVariant(variant)
	contextName := "ctx.left"
	if behaviorVariant == "v2" {
		contextName = "ctx.right"
	}
	fixture.addRelation(
		t,
		"Relation.Optional",
		[]typedmemory.BoundedContextRef{fixture.contextByName[contextName]},
		[]typedmemory.SlotSpec{
			fixture.relationSlot(t, "subject", 1, behaviorVariant),
		},
		variant,
	)
}

func (fixture *compatibilityFixture) relationSlot(
	t *testing.T,
	slotName string,
	maximum uint64,
	provenanceVariant string,
) typedmemory.SlotSpec {
	t.Helper()
	target := mustFixture(
		typedmemory.NewValueSlotTarget(mustFixture(
			typedmemory.NewValueKindRef(
				fixture.ref,
				fixture.kindByName["U.D"],
			),
		)),
	)
	cardinality := mustFixture(typedmemory.NewBoundedCardinality(0, maximum))
	return mustFixture(
		typedmemory.NewSlotSpec(
			mustFixture(typedmemory.NewSlotKindID(slotName)),
			target,
			cardinality,
			testProvenance(t, "relation-slot-"+slotName+"-"+provenanceVariant),
		),
	)
}

func (fixture *compatibilityFixture) addRelation(
	t *testing.T,
	signatureName string,
	contexts []typedmemory.BoundedContextRef,
	slots []typedmemory.SlotSpec,
	provenanceVariant string,
) {
	t.Helper()
	ref := mustFixture(
		typedmemory.NewRelationSignatureRef(
			fixture.ref,
			mustFixture(typedmemory.NewSignatureID(signatureName)),
		),
	)
	relation := mustFixture(
		typedmemory.NewRelationSignature(
			ref,
			contexts,
			slots,
			testProvenance(t, "relation-"+signatureName+"-"+provenanceVariant),
		),
	)
	fixture.relations = append(fixture.relations, relation)
}

func (fixture *compatibilityFixture) addShapes(
	t *testing.T,
	options fixtureOptions,
) {
	t.Helper()
	fixture.addShape(t, "shape.base", typedmemory.ScalarText, "base")
	if !options.includes(ValueShapeFamily) {
		return
	}
	variant := options.variant(ValueShapeFamily)
	scalarKind := typedmemory.ScalarBoolean
	if variant == "payload-v2" {
		scalarKind = typedmemory.ScalarBytes
	}
	fixture.addShape(t, "shape.optional", scalarKind, variant)
}

func (fixture *compatibilityFixture) addShape(
	t *testing.T,
	name string,
	scalarKind typedmemory.ScalarKind,
	provenanceVariant string,
) {
	t.Helper()
	shape := mustFixture(typedmemory.NewScalarShape(scalarKind))
	id := mustFixture(typedmemory.NewShapeID(name))
	ref := mustFixture(typedmemory.DeriveValueShapeRef(id, shape))
	declaration := mustFixture(

		typedmemory.NewValueShapeDeclaration(
			ref,
			shape,
			testProvenance(t, "shape-"+name+"-"+provenanceVariant),
		))

	fixture.shapes = append(fixture.shapes, declaration)
	fixture.shapeByName[name] = declaration
}

func (fixture *compatibilityFixture) addBindings(
	t *testing.T,
	options fixtureOptions,
) {
	t.Helper()
	if !options.includes(ValueBindingFamily) {
		return
	}
	variant := options.variant(ValueBindingFamily)
	behaviorVariant := fixtureBehaviorVariant(variant)
	codec := mustFixture(

		typedmemory.NewCodecRef(
			mustFixture(typedmemory.NewCodecID("codec.optional")),
			mustFixture(typedmemory.NewCanonicalizationVersion("1.0.0")),
			testDigest(t, "codec-"+behaviorVariant),
		))

	binding := mustFixture(

		typedmemory.NewValueBinding(
			mustFixture(

				typedmemory.NewValueKindRef(fixture.ref, fixture.kindByName["U.D"])),

			fixture.shapeByName["shape.base"].Ref(),
			codec,
			testProvenance(t, "binding-"+variant),
		))

	fixture.bindings = append(fixture.bindings, binding)
}

func (fixture *compatibilityFixture) addConstraints(
	t *testing.T,
	options fixtureOptions,
) {
	t.Helper()
	if !options.includes(ConstraintFamily) {
		return
	}
	variant := options.variant(ConstraintFamily)
	first := fixture.kindByName["U.D"]
	if variant == "v2" {
		first = fixture.kindByName["U.C"]
	}
	constraint := mustFixture(

		typedmemory.NewKindDisjointConstraint(
			mustFixture(typedmemory.NewConstraintID("constraint.optional")),
			[]typedmemory.KindID{first, fixture.kindByName["U.E"]},
			testProvenance(t, "constraint-"+variant),
		))

	fixture.constraints = append(fixture.constraints, constraint)
}

func (fixture *compatibilityFixture) reverse() {
	slices.Reverse(fixture.coverage)
	slices.Reverse(fixture.contexts)
	slices.Reverse(fixture.kinds)
	slices.Reverse(fixture.entitySets)
	slices.Reverse(fixture.kindSignatures)
	slices.Reverse(fixture.refKinds)
	slices.Reverse(fixture.availabilities)
	slices.Reverse(fixture.subkinds)
	slices.Reverse(fixture.bridges)
	slices.Reverse(fixture.relations)
	slices.Reverse(fixture.shapes)
	slices.Reverse(fixture.bindings)
	slices.Reverse(fixture.constraints)
}

func testContextKindAvailability(
	t *testing.T,
	base typedmemory.TypeEnvRef,
	context typedmemory.BoundedContextRef,
	kind typedmemory.KindID,
	seed string,
) typedmemory.ContextKindAvailability {
	t.Helper()
	symbol := mustFixture(typedmemory.KindSymbolRef(kind))
	manifest := mustFixture(

		typedmemory.NewSignatureManifestRef(
			"fixture.availability."+sanitizeFixtureID(seed),
			"1.0.0",
		))

	basis := mustFixture(

		typedmemory.NewManifestSymbolBasis(
			manifest,
			typedmemory.ManifestProvide,
			symbol,
		))

	digest := testDigest(t, "availability-"+seed)
	projectProvenance := mustFixture(

		typedmemory.NewProjectSourceProvenanceBuilder(
			mustFixture(

				typedmemory.NewProvenanceRef("prov:availability:"+sanitizeFixtureID(seed))),

			mustFixture(

				typedmemory.NewCarrierRef("carrier:availability:"+sanitizeFixtureID(seed))),

			mustFixture(typedmemory.NewCarrierEdition("1.0.0")),
			digest,
		).
			SetDeclarationRange(mustFixture(typedmemory.NewSourceLineRange(1, 1))).
			SetCompilerRule(mustFixture(

				typedmemory.NewCompilerRuleID("fixture.availability.v1"))).
			SetBoundedContext(context).
			SetBaseTypeEnv(base).
			SetSignatureBlockRow(typedmemory.VocabularyRow).
			SetManifestBasis(basis).
			Build())

	contextSource := mustFixture(

		typedmemory.NewContextKindAvailabilitySource(
			context.String(),
			projectProvenance,
		))

	declarationSource := mustFixture(

		typedmemory.NewContextKindAvailabilitySource(
			kind.String(),
			projectProvenance,
		))

	extensionRef := mustFixture(

		typedmemory.ParseTypeEnvExtensionRef(
			"typeenv-extension:" + manifest.ID() + "@" + digest.String(),
		))

	provider := mustFixture(

		typedmemory.NewExtensionKindAvailabilityProvider(
			typedmemory.ExtensionKindAvailabilityProviderInput{
				ExtensionRef:      extensionRef,
				Context:           context,
				ContextSource:     contextSource,
				Symbol:            symbol,
				DeclarationSource: declarationSource,
			},
		))

	ground := mustFixture(

		typedmemory.NewLocalContextKindAvailabilityGround(
			typedmemory.LocalContextKindAvailabilityGroundInput{
				Context:             context,
				KindID:              kind,
				ContextSource:       contextSource,
				ApplicabilitySource: contextSource,
				Provider:            provider,
			},
		))

	grounds := mustFixture(

		typedmemory.NewContextKindAvailabilityGroundSet(
			[]typedmemory.ContextKindAvailabilityGround{ground},
		))

	return mustFixture(

		typedmemory.NewContextKindAvailability(context, kind, grounds))

}

func testProvenance(
	t *testing.T,
	seed string,
) typedmemory.DeclarationProvenance {
	t.Helper()
	return mustFixture(

		typedmemory.NewFPFSourceProvenance(
			mustFixture(

				typedmemory.NewProvenanceRef("prov:"+sanitizeFixtureID(seed))),

			testSourceLocation(t, seed),
			mustFixture(

				typedmemory.NewCompilerRuleID("fixture.rule."+sanitizeFixtureID(seed))),
		))

}

func testSourceLocation(t *testing.T, seed string) typedmemory.SourceLocation {
	t.Helper()
	return mustFixture(

		typedmemory.NewUnpatternedSourceLocation(
			mustFixture(

				typedmemory.NewSourceUnitID("fixture.unit."+sanitizeFixtureID(seed))),

			mustFixture(typedmemory.NewSourceRevision("fixture-revision-v1")),
			testDigest(t, "source-"+seed),
			mustFixture(typedmemory.NewSourceLineRange(1, 1)),
		))

}

func testTypeEnvRef(t *testing.T, seed string) typedmemory.TypeEnvRef {
	t.Helper()
	return mustFixture(typedmemory.NewTypeEnvRef(testDigest(t, seed)))
}

func testDigest(t *testing.T, seed string) typedmemory.SHA256Digest {
	t.Helper()
	sum := sha256.Sum256([]byte(seed))
	encoded := hex.EncodeToString(sum[:])
	return mustFixture(typedmemory.NewSHA256Digest("sha256:" + encoded))
}

func sanitizeFixtureID(value string) string {
	replacer := strings.NewReplacer(
		"/", ".",
		" ", ".",
		":", ".",
		"_", ".",
	)
	return replacer.Replace(value)
}

func fixtureBehaviorVariant(value string) string {
	if strings.HasPrefix(value, "metadata-") {
		return "v1"
	}
	return value
}

func mustFixture[T any](value T, err error) T {
	if err != nil {
		panic("fixture constructor: " + err.Error())
	}
	return value
}
