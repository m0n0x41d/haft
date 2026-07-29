package projecttypeenvprofilecompatibility

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"testing"

	"github.com/m0n0x41d/haft/internal/projectmemory/projectionprofile"
	typeenvcompatibility "github.com/m0n0x41d/haft/internal/projecttypeenvcompatibility"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestAssessProjectionProfileClosedOutcomes(t *testing.T) {
	profile := testProjectionProfile(t)
	baseSlots := profileSlotConfigs(profile, 1)
	base := buildProfileTypeEnv(t, "profile-base", baseSlots)

	compatibleTarget := buildProfileTypeEnv(
		t,
		"profile-compatible",
		append([]profileSlotConfig(nil), baseSlots...),
	)
	compatible := assessProfile(t, profile, base, compatibleTarget)
	if _, ok := compatible.(CompatibleProjectionProfile); !ok {
		t.Fatalf("compatible outcome = %T", compatible)
	}
	if compatible.Kind() != ProfileCompatible || len(compatible.AffectedFacets()) != 0 {
		t.Fatal("compatible profile has non-compatible posture or affected facets")
	}
	if compatible.ProfileEdition() != profile.Edition() || len(compatible.FacetIssues()) != 0 {
		t.Fatal("compatible profile lost its exact edition or invented facet issues")
	}

	widenedSlots := append([]profileSlotConfig(nil), baseSlots...)
	widenedSlots[0].maximum = 2
	widenedTarget := buildProfileTypeEnv(t, "profile-widened", widenedSlots)
	degraded := assessProfile(t, profile, base, widenedTarget)
	if _, ok := degraded.(DegradedProjectionProfileFacets); !ok {
		t.Fatalf("degraded outcome = %T", degraded)
	}
	if degraded.Kind() != ProfileDegradedFacets || len(degraded.AffectedFacets()) != len(profile.Facets()) {
		t.Fatal("widened declared slot did not degrade every conservatively affected facet")
	}
	assertFacetIssuesCoverAffectedFacets(t, degraded)

	removedSlots := append([]profileSlotConfig(nil), baseSlots[1:]...)
	removedTarget := buildProfileTypeEnv(t, "profile-removed", removedSlots)
	blocked := assessProfile(t, profile, base, removedTarget)
	if _, ok := blocked.(BlockedProjectionProfile); !ok {
		t.Fatalf("blocked outcome = %T", blocked)
	}
	if blocked.Kind() != ProfileBlocked || !hasProfileGroundKind(blocked, ProfileGroundDeclaredSlotRemoved) {
		t.Fatal("removed declared slot did not block the exact profile edition")
	}
	assertFacetIssuesCoverAffectedFacets(t, blocked)
}

func TestAssessProjectionProfileBlocksCompilerGap(t *testing.T) {
	profile := testProjectionProfile(t)
	baseSlots := profileSlotConfigs(profile, 1)
	base := buildProfileTypeEnv(t, "profile-gap-base", baseSlots)

	targetChanged := append([]profileSlotConfig(nil), baseSlots...)
	targetChanged[0].targetKind = "U.ProfileAlternate"
	target := buildProfileTypeEnv(t, "profile-gap-target", targetChanged)
	compilerGap := assessProfile(t, profile, base, target)
	if compilerGap.Kind() != ProfileBlocked || !hasProfileGroundKind(compilerGap, ProfileGroundDeclaredSlotCompilerGap) {
		t.Fatal("declared-slot compiler gap did not block profile compatibility")
	}
}

func TestAssessProjectionProfileTreatsSlotAbsentFromBothAsCompatible(t *testing.T) {
	profile := testProjectionProfile(t)
	unknownProfileSlots := []profileSlotConfig{{name: "UnrelatedSlot", maximum: 1}}
	unchanged := buildProfileTypeEnv(t, "profile-absent-both", unknownProfileSlots)
	result := assessProfile(t, profile, unchanged, unchanged)
	if result.Kind() != ProfileCompatible ||
		!hasProfileGroundKind(result, ProfileGroundDeclaredSlotAbsentBoth) {
		t.Fatal("slot absent from both exact successor surfaces was not explicitly compatible")
	}
	if hasProfileGroundKind(result, ProfileGroundDeclaredSlotMissing) {
		t.Fatal("slot absent from both was mislabeled missing from target")
	}
	for _, ground := range result.Grounds() {
		if ground.Kind() != ProfileGroundDeclaredSlotAbsentBoth ||
			ground.Posture() != ProfileGroundSatisfied ||
			ground.RuleKey() != "" {
			t.Fatalf("absent-from-both ground = %#v", ground)
		}
	}
}

func TestAssessProjectionProfileMatchesQualifiedCanonicalSlotsByDeclaredRole(
	t *testing.T,
) {
	profile := testProjectionProfile(t)
	baseSlots := profileSlotConfigs(profile, 1)
	qualifiedSlots := qualifyProfileSlotConfigs(
		baseSlots,
		"Haft.NoteAtConcern.",
	)
	base := buildProfileTypeEnv(t, "profile-qualified-base", qualifiedSlots)
	target := buildProfileTypeEnv(t, "profile-qualified-target", qualifiedSlots)

	result := assessProfile(t, profile, base, target)
	if result.Kind() != ProfileCompatible {
		t.Fatalf("qualified canonical slots produced %s, want compatible", result.Kind())
	}
	for _, ground := range result.Grounds() {
		if ground.Posture() != ProfileGroundSatisfied || ground.RuleKey() == "" {
			t.Fatalf("qualified slot ground = %#v, want exact satisfied rule", ground)
		}
	}
}

func TestAssessProjectionProfileBlocksPredecessorPresentTargetRemovedSlot(
	t *testing.T,
) {
	profile := testProjectionProfile(t)
	baseSlots := profileSlotConfigs(profile, 1)
	qualifiedSlots := qualifyProfileSlotConfigs(
		baseSlots,
		"Haft.NoteAtConcern.",
	)
	additionalClaimGraphSlot := profileSlotConfig{
		name:       "Haft.ProblemCardAtConcern.ClaimGraphSlot",
		maximum:    1,
		targetKind: "U.ProfileItem",
	}
	baseSlots = append(qualifiedSlots, additionalClaimGraphSlot)
	targetSlots := append([]profileSlotConfig(nil), qualifiedSlots...)
	base := buildProfileTypeEnv(t, "profile-qualified-removal-base", baseSlots)
	target := buildProfileTypeEnv(t, "profile-qualified-removal-target", targetSlots)

	result := assessProfile(t, profile, base, target)
	if result.Kind() != ProfileBlocked ||
		!hasProfileGroundKind(result, ProfileGroundDeclaredSlotRemoved) {
		t.Fatal("removing one qualified role occurrence did not block the profile")
	}
}

func TestAssessProjectionProfileCanonicalizesMixedMatchedAndAbsentGrounds(
	t *testing.T,
) {
	profile := testProjectionProfile(t)
	matchedSlots := qualifyProfileSlotConfigs(
		profileSlotConfigs(profile, 1)[:1],
		"Haft.NoteAtConcern.",
	)
	base := buildProfileTypeEnv(t, "profile-mixed-base", matchedSlots)
	target := buildProfileTypeEnv(t, "profile-mixed-target", matchedSlots)
	result := assessProfile(t, profile, base, target)
	if result.Kind() != ProfileCompatible {
		t.Fatalf("mixed unchanged/absent grounds produced %s", result.Kind())
	}
	unchanged := 0
	absent := 0
	for index, ground := range result.Grounds() {
		if index > 0 &&
			profileCompatibilityGroundKey(result.Grounds()[index-1]) >=
				profileCompatibilityGroundKey(ground) {
			t.Fatal("mixed compatibility grounds are not canonical")
		}
		switch ground.Kind() {
		case ProfileGroundDeclaredSlotUnchanged:
			unchanged++
		case ProfileGroundDeclaredSlotAbsentBoth:
			absent++
		default:
			t.Fatalf("mixed compatibility ground kind = %s", ground.Kind())
		}
	}
	if unchanged != 1 || absent != len(profile.SlotReads())-1 {
		t.Fatalf("mixed grounds unchanged/absent = %d/%d", unchanged, absent)
	}
	decoded, err := DecodeProjectionProfileCompatibility(result.CanonicalBytes())
	if err != nil {
		t.Fatalf("DecodeProjectionProfileCompatibility(): %v", err)
	}
	if decoded.Digest() != result.Digest() ||
		!bytes.Equal(decoded.CanonicalBytes(), result.CanonicalBytes()) {
		t.Fatal("mixed compatibility canonical round-trip changed identity")
	}
}

func TestSlotKindMatchesDeclaredReadDoesNotAliasQualifiedOrSimilarNames(
	t *testing.T,
) {
	tests := []struct {
		name     string
		declared string
		actual   string
		want     bool
	}{
		{
			name:     "unqualified exact",
			declared: "ClaimGraphSlot",
			actual:   "ClaimGraphSlot",
			want:     true,
		},
		{
			name:     "qualified canonical role",
			declared: "ClaimGraphSlot",
			actual:   "Haft.NoteAtConcern.ClaimGraphSlot",
			want:     true,
		},
		{
			name:     "similar terminal name",
			declared: "ClaimGraphSlot",
			actual:   "Haft.NoteAtConcern.NotClaimGraphSlot",
			want:     false,
		},
		{
			name:     "role appears before terminal component",
			declared: "ClaimGraphSlot",
			actual:   "Haft.ClaimGraphSlot.OtherSlot",
			want:     false,
		},
		{
			name:     "qualified declaration remains exact",
			declared: "Haft.NoteAtConcern.ClaimGraphSlot",
			actual:   "Haft.ProblemCardAtConcern.ClaimGraphSlot",
			want:     false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			declared := mustProfileTest(typedmemory.NewSlotKindID(test.declared))
			actual := mustProfileTest(typedmemory.NewSlotKindID(test.actual))
			got := slotKindMatchesDeclaredRead(declared, actual)
			if got != test.want {
				t.Fatalf("slotKindMatchesDeclaredRead() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestProjectionProfileCompatibilityCanonicalRoundTripAndDeterminism(t *testing.T) {
	profile := testProjectionProfile(t)
	baseSlots := profileSlotConfigs(profile, 1)
	widenedSlots := append([]profileSlotConfig(nil), baseSlots...)
	widenedSlots[0].maximum = 2
	base := buildProfileTypeEnv(t, "profile-roundtrip-base", baseSlots)
	target := buildProfileTypeEnv(t, "profile-roundtrip-target", widenedSlots)
	first := assessProfile(t, profile, base, target)

	reversedBase := append([]profileSlotConfig(nil), baseSlots...)
	reversedTarget := append([]profileSlotConfig(nil), widenedSlots...)
	slices.Reverse(reversedBase)
	slices.Reverse(reversedTarget)
	permutedBase := buildProfileTypeEnv(t, "profile-roundtrip-base", reversedBase)
	permutedTarget := buildProfileTypeEnv(t, "profile-roundtrip-target", reversedTarget)
	second := assessProfile(t, profile, permutedBase, permutedTarget)
	if first.Digest() != second.Digest() || !bytes.Equal(first.CanonicalBytes(), second.CanonicalBytes()) {
		t.Fatal("builder permutation changed profile compatibility identity")
	}

	decoded, err := DecodeProjectionProfileCompatibility(first.CanonicalBytes())
	if err != nil {
		t.Fatalf("DecodeProjectionProfileCompatibility(): %v", err)
	}
	if decoded.Digest() != first.Digest() || !bytes.Equal(decoded.CanonicalBytes(), first.CanonicalBytes()) {
		t.Fatal("profile compatibility canonical round-trip changed identity")
	}
	trailing := append(first.CanonicalBytes(), 0x01)
	if _, err := DecodeProjectionProfileCompatibility(trailing); err == nil {
		t.Fatal("profile compatibility decoder accepted trailing bytes")
	}
}

func TestDecodeProjectionProfileCompatibilityPreservesHistoricalMissingGround(
	t *testing.T,
) {
	profile := testProjectionProfile(t)
	unknownSlots := []profileSlotConfig{{name: "UnrelatedSlot", maximum: 1}}
	environment := buildProfileTypeEnv(t, "profile-historical-missing", unknownSlots)
	diff := mustProfileTest(typeenvcompatibility.CompareSuccessor(
		environment,
		environment,
	))
	grounds := make([]ProjectionProfileCompatibilityGround, 0, len(profile.SlotReads()))
	for _, slot := range profile.SlotReads() {
		grounds = append(grounds, ProjectionProfileCompatibilityGround{
			slot:    slot,
			kind:    ProfileGroundDeclaredSlotMissing,
			posture: ProfileGroundBlocking,
		})
	}
	issues := projectionProfileFacetIssues(profile, grounds)
	state := projectionProfileCompatibilityState{
		kind:        ProfileBlocked,
		profileRef:  profile.Ref(),
		profileEdit: profile.Edition(),
		profileHash: profile.Digest(),
		base:        diff.Base(),
		target:      diff.Target(),
		diffDigest:  diff.Digest(),
		grounds:     grounds,
		issues:      issues,
		facets:      affectedProjectionProfileFacets(issues),
	}
	state.digest = mustProfileTest(digestBytes(
		profileCompatibilityCanonicalBytes(state),
	))
	historical := mustProfileTest(projectionProfileCompatibilityFromState(state))
	decoded, err := DecodeProjectionProfileCompatibility(historical.CanonicalBytes())
	if err != nil {
		t.Fatalf("decode historical missing-ground artifact: %v", err)
	}
	if decoded.Kind() != ProfileBlocked ||
		!hasProfileGroundKind(decoded, ProfileGroundDeclaredSlotMissing) ||
		decoded.Digest() != historical.Digest() {
		t.Fatal("historical missing-ground artifact changed during decode")
	}
}

func TestInstalledProjectionProfileCompatibilitySetCoversEveryExactEdition(t *testing.T) {
	profile := testProjectionProfile(t)
	baseSlots := profileSlotConfigs(profile, 1)
	widenedSlots := append([]profileSlotConfig(nil), baseSlots...)
	widenedSlots[0].maximum = 2
	base := buildProfileTypeEnv(t, "profile-set-base", baseSlots)
	target := buildProfileTypeEnv(t, "profile-set-target", widenedSlots)
	diff := mustProfileTest(typeenvcompatibility.CompareSuccessor(base, target))
	set := mustProfileTest(AssessInstalledProjectionProfiles(diff))
	installed := projectionprofile.Installed()
	if len(set.Profiles()) != len(installed) {
		t.Fatalf("profile set size = %d, want %d", len(set.Profiles()), len(installed))
	}
	for index, result := range set.Profiles() {
		expected := installed[index]
		if result.ProfileRef() != expected.Ref() ||
			result.ProfileEdition() != expected.Edition() ||
			result.ProfileDigest() != expected.Digest() {
			t.Fatalf("profile result %d lost exact installed identity", index)
		}
	}
	decoded, err := DecodeProjectionProfileCompatibilitySet(set.CanonicalBytes())
	if err != nil {
		t.Fatalf("DecodeProjectionProfileCompatibilitySet(): %v", err)
	}
	if decoded.Digest() != set.Digest() || !bytes.Equal(decoded.CanonicalBytes(), set.CanonicalBytes()) {
		t.Fatal("profile compatibility set round-trip changed identity")
	}
	trailing := append(set.CanonicalBytes(), 0x01)
	if _, err := DecodeProjectionProfileCompatibilitySet(trailing); err == nil {
		t.Fatal("profile compatibility set decoder accepted trailing bytes")
	}
}

func TestInstalledProjectionProfileCompatibilitySetIsPermutationInvariant(t *testing.T) {
	profile := testProjectionProfile(t)
	baseSlots := profileSlotConfigs(profile, 1)
	widenedSlots := append([]profileSlotConfig(nil), baseSlots...)
	widenedSlots[0].maximum = 2
	reversedBase := append([]profileSlotConfig(nil), baseSlots...)
	reversedTarget := append([]profileSlotConfig(nil), widenedSlots...)
	slices.Reverse(reversedBase)
	slices.Reverse(reversedTarget)

	firstBase := buildProfileTypeEnv(t, "profile-set-permutation-base", baseSlots)
	firstTarget := buildProfileTypeEnv(t, "profile-set-permutation-target", widenedSlots)
	secondBase := buildProfileTypeEnv(t, "profile-set-permutation-base", reversedBase)
	secondTarget := buildProfileTypeEnv(t, "profile-set-permutation-target", reversedTarget)
	firstDiff := mustProfileTest(typeenvcompatibility.CompareSuccessor(firstBase, firstTarget))
	secondDiff := mustProfileTest(typeenvcompatibility.CompareSuccessor(secondBase, secondTarget))
	first := mustProfileTest(AssessInstalledProjectionProfiles(firstDiff))
	second := mustProfileTest(AssessInstalledProjectionProfiles(secondDiff))
	if first.Digest() != second.Digest() || !bytes.Equal(first.CanonicalBytes(), second.CanonicalBytes()) {
		t.Fatal("TypeEnv builder permutation changed installed profile compatibility set")
	}
}

func TestTransitionProjectionProfileCompatibilitySetBindsDiffAndInstalledProfiles(
	t *testing.T,
) {
	profile := testProjectionProfile(t)
	baseSlots := profileSlotConfigs(profile, 1)
	targetSlots := append([]profileSlotConfig(nil), baseSlots...)
	targetSlots[0].maximum = 2
	base := buildProfileTypeEnv(t, "transition-profile-base", baseSlots)
	target := buildProfileTypeEnv(t, "transition-profile-target", targetSlots)
	diff := mustProfileTest(typeenvcompatibility.CompareSuccessor(base, target))
	artifact := mustProfileTest(AssessTransitionProjectionProfiles(diff))

	profiles := mustProfileTest(DecodeTransitionProjectionProfiles(artifact))
	if artifact.SuccessorDiff().Digest() != diff.Digest() ||
		profiles.SuccessorDiffDigest() != diff.Digest() {
		t.Fatal("transition profile artifact lost its exact successor diff")
	}
	degradedFound := false
	for _, result := range profiles.Profiles() {
		if result.ProfileRef() != profile.Ref() {
			continue
		}
		if result.Kind() != ProfileDegradedFacets ||
			len(result.AffectedFacets()) == 0 {
			t.Fatal("transition artifact did not keep the widened profile explicitly degraded")
		}
		degradedFound = true
	}
	if !degradedFound {
		t.Fatal("transition artifact omitted the exact degraded profile edition")
	}
	if artifact.Ref().Digest() != artifact.Digest() {
		t.Fatal("transition profile artifact ref differs from its content digest")
	}
	parsedRef, err := ParseTransitionProjectionProfileCompatibilitySetRef(
		artifact.Ref().String(),
	)
	if err != nil || parsedRef != artifact.Ref() {
		t.Fatalf("parse transition profile artifact ref: %v", err)
	}
	decoded, err := DecodeTransitionProjectionProfileCompatibilitySet(
		artifact.CanonicalBytes(),
	)
	if err != nil {
		t.Fatalf("DecodeTransitionProjectionProfileCompatibilitySet(): %v", err)
	}
	if decoded.Ref() != artifact.Ref() ||
		!bytes.Equal(decoded.CanonicalBytes(), artifact.CanonicalBytes()) {
		t.Fatal("transition profile artifact round-trip changed identity")
	}
	trailing := append(artifact.CanonicalBytes(), 0x01)
	if _, err := DecodeTransitionProjectionProfileCompatibilitySet(trailing); err == nil {
		t.Fatal("transition profile artifact decoder accepted trailing bytes")
	}
}

func TestTransitionProjectionProfileCompatibilitySetPreservesBlockedPosture(
	t *testing.T,
) {
	profile := testProjectionProfile(t)
	baseSlots := profileSlotConfigs(profile, 1)
	targetSlots := append([]profileSlotConfig(nil), baseSlots[1:]...)
	base := buildProfileTypeEnv(t, "transition-profile-blocked-base", baseSlots)
	target := buildProfileTypeEnv(t, "transition-profile-blocked-target", targetSlots)
	diff := mustProfileTest(typeenvcompatibility.CompareSuccessor(base, target))
	artifact := mustProfileTest(AssessTransitionProjectionProfiles(diff))

	blocked := mustProfileTest(TransitionProjectionProfilesHaveBlockedProfile(artifact))
	if !blocked {
		t.Fatal("transition profile artifact hid a blocked installed profile")
	}
}

type profileSlotConfig struct {
	name       string
	maximum    uint64
	targetKind string
}

func profileSlotConfigs(
	profile projectionprofile.Descriptor,
	maximum uint64,
) []profileSlotConfig {
	values := make([]profileSlotConfig, 0, len(profile.SlotReads()))
	for _, slot := range profile.SlotReads() {
		values = append(values, profileSlotConfig{
			name:       slot.String(),
			maximum:    maximum,
			targetKind: "U.ProfileItem",
		})
	}
	return values
}

func qualifyProfileSlotConfigs(
	values []profileSlotConfig,
	prefix string,
) []profileSlotConfig {
	qualified := make([]profileSlotConfig, 0, len(values))
	for _, value := range values {
		value.name = prefix + value.name
		qualified = append(qualified, value)
	}
	return qualified
}

func buildProfileTypeEnv(
	t *testing.T,
	seed string,
	slotConfigs []profileSlotConfig,
) typedmemory.TypeEnv {
	t.Helper()
	ref := mustProfileTest(typedmemory.NewTypeEnvRef(profileTestDigest(seed)))
	provenance := profileTestProvenance(t)
	contextRef := mustProfileTest(typedmemory.NewBoundedContextRef("profile.context"))
	context := mustProfileTest(typedmemory.NewBoundedContext(contextRef, provenance))
	itemKindID := mustProfileTest(typedmemory.NewKindID("U.ProfileItem"))
	alternateKindID := mustProfileTest(typedmemory.NewKindID("U.ProfileAlternate"))
	itemKind := mustProfileTest(typedmemory.NewKindDefinition(itemKindID, provenance))
	alternateKind := mustProfileTest(typedmemory.NewKindDefinition(alternateKindID, provenance))

	slots := make([]typedmemory.SlotSpec, 0, len(slotConfigs))
	for _, config := range slotConfigs {
		kindID := itemKindID
		if config.targetKind == "U.ProfileAlternate" {
			kindID = alternateKindID
		}
		valueKind := mustProfileTest(typedmemory.NewValueKindRef(ref, kindID))
		target := mustProfileTest(typedmemory.NewValueSlotTarget(valueKind))
		cardinality := mustProfileTest(typedmemory.NewBoundedCardinality(0, config.maximum))
		slot := mustProfileTest(
			typedmemory.NewSlotSpec(
				mustProfileTest(typedmemory.NewSlotKindID(config.name)),
				target,
				cardinality,
				provenance,
			),
		)
		slots = append(slots, slot)
	}
	signatureRef := mustProfileTest(
		typedmemory.NewRelationSignatureRef(
			ref,
			mustProfileTest(typedmemory.NewSignatureID("Relation.ProfileProjection")),
		),
	)
	signature := mustProfileTest(
		typedmemory.NewRelationSignature(
			signatureRef,
			[]typedmemory.BoundedContextRef{contextRef},
			slots,
			provenance,
		),
	)
	location := provenance.Location()
	coverageSubject := mustProfileTest(typedmemory.SourceUnitCoverage(location.UnitID()))
	coverageEntry := mustProfileTest(typedmemory.NewCompiledCoverageEntry(coverageSubject, location))
	coverage := mustProfileTest(typedmemory.NewCoverageManifest([]typedmemory.CoverageEntry{coverageEntry}))
	builder := typedmemory.NewTypeEnvBuilder(ref)
	builder.SetSourceRevision(mustProfileTest(typedmemory.NewSourceRevision("profile-source-v1")))
	builder.SetCompilerSchemaVersion(mustProfileTest(typedmemory.NewCompilerSchemaVersion("profile-compiler-v1")))
	builder.SetCoverageManifest(coverage)
	builder.AddBoundedContext(context)
	builder.AddKindDefinition(itemKind)
	builder.AddKindDefinition(alternateKind)
	builder.AddRelationSignature(signature)
	return mustProfileTest(builder.Build())
}

func profileTestProvenance(t *testing.T) typedmemory.FPFSourceProvenance {
	t.Helper()
	location := mustProfileTest(
		typedmemory.NewUnpatternedSourceLocation(
			mustProfileTest(typedmemory.NewSourceUnitID("profile.fixture")),
			mustProfileTest(typedmemory.NewSourceRevision("profile-source-v1")),
			profileTestDigest("profile-source"),
			mustProfileTest(typedmemory.NewSourceLineRange(1, 1)),
		),
	)
	return mustProfileTest(
		typedmemory.NewFPFSourceProvenance(
			mustProfileTest(typedmemory.NewProvenanceRef("prov:profile:fixture")),
			location,
			mustProfileTest(typedmemory.NewCompilerRuleID("profile.fixture.v1")),
		),
	)
}

func profileTestDigest(seed string) typedmemory.SHA256Digest {
	sum := sha256.Sum256([]byte(seed))
	encoded := hex.EncodeToString(sum[:])
	return mustProfileTest(typedmemory.NewSHA256Digest("sha256:" + encoded))
}

func testProjectionProfile(
	t *testing.T,
) projectionprofile.Descriptor {
	t.Helper()
	ref := mustProfileTest(
		projectionprofile.ParseRef("decision_rationale.v1"),
	)
	profile, found := projectionprofile.Lookup(ref)
	if !found {
		t.Fatal("builtin decision_rationale.v1 profile is missing")
	}
	return profile
}

func assessProfile(
	t *testing.T,
	profile projectionprofile.Descriptor,
	base typedmemory.TypeEnv,
	target typedmemory.TypeEnv,
) ProjectionProfileCompatibility {
	t.Helper()
	diff := mustProfileTest(typeenvcompatibility.CompareSuccessor(base, target))
	return mustProfileTest(AssessProjectionProfile(profile, diff))
}

func hasProfileGroundKind(
	result ProjectionProfileCompatibility,
	kind ProfileCompatibilityGroundKind,
) bool {
	for _, ground := range result.Grounds() {
		if ground.Kind() == kind {
			return true
		}
	}
	return false
}

func assertFacetIssuesCoverAffectedFacets(
	t *testing.T,
	result ProjectionProfileCompatibility,
) {
	t.Helper()
	covered := make(map[projectionprofile.FacetKind]struct{})
	for _, issue := range result.FacetIssues() {
		if issue.Posture() == ProfileGroundSatisfied {
			t.Fatal("facet issue carries a satisfied posture")
		}
		covered[issue.Facet()] = struct{}{}
	}
	for _, facet := range result.AffectedFacets() {
		if _, found := covered[facet]; !found {
			t.Fatalf("affected facet %s has no typed issue", facet)
		}
	}
}

func mustProfileTest[T any](value T, err error) T {
	if err != nil {
		panic("profile compatibility fixture: " + err.Error())
	}
	return value
}
