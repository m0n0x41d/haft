package projectprofile_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/projectprofile"
)

const testProjectRoot = "/tmp/haft-project"

func TestPublicProfileApplicabilityFailsClosedWithoutSealedAdmission(t *testing.T) {
	auto := projectprofile.ResolveSoftwareSystemSpecMigration(projectprofile.Auto{})
	assertMissingBasis(t, auto, projectprofile.MissingAuthoritativeProfile)

	softwareProfile := mustDeclaredProfile(
		t,
		[]projectprofile.RealizationScope{mustSoftwareScope(t, "software-cli")},
	)
	software := projectprofile.ResolveSoftwareSystemSpecMigration(softwareProfile)
	assertMissingBasis(
		t,
		software,
		projectprofile.MissingCanonicalDurableProfileAdmission,
	)

	nonSoftwareProfile := mustDeclaredProfile(
		t,
		[]projectprofile.RealizationScope{mustNonSoftwareScope(t, "knowledge-model")},
	)
	nonSoftware := projectprofile.ResolveSoftwareSystemSpecMigration(nonSoftwareProfile)
	assertMissingBasis(
		t,
		nonSoftware,
		projectprofile.MissingCanonicalDurableProfileAdmission,
	)
}

func TestLegacyDeclaredCannotImplementFinalV1ConfiguredProfile(t *testing.T) {
	legacy := mustDeclaredProfile(
		t,
		[]projectprofile.RealizationScope{mustSoftwareScope(t, "software-cli")},
	)
	if _, ok := any(legacy).(projectprofile.ConfiguredProjectProfileV1); ok {
		t.Fatal("legacy Declared unexpectedly implements ConfiguredProjectProfileV1")
	}
}

func TestStrongIdentifiersRejectPathsAndSilentWhitespaceNormalization(t *testing.T) {
	for _, raw := range []string{"./cmd/haft", "specs/model", `docs\\model`, " scope "} {
		if _, err := projectprofile.NewScopeID(raw); err == nil {
			t.Fatalf("NewScopeID(%q) succeeded", raw)
		}
	}
	if _, err := projectprofile.NewEntityRef(" entity:haft "); err == nil {
		t.Fatal("NewEntityRef silently trimmed identity")
	}
	if _, err := projectprofile.NewContentDigest(" " + digestOf(t, "x").String()); err == nil {
		t.Fatal("NewContentDigest silently trimmed digest")
	}
}

func TestScopeSetAndTypedReferencesAreImmutableAndDuplicateSafe(t *testing.T) {
	patterns := []projectprofile.SourceUnitRef{mustSourceRef(t, "A.7")}
	scope, err := projectprofile.NewNonSoftwareRealization(
		mustScopeID(t, "knowledge-model"),
		projectprofile.NoEntityReference{},
		projectprofile.UnspecifiedKindOrientation{},
		patterns,
		[]projectprofile.SpecSectionRef{},
	)
	if err != nil {
		t.Fatalf("NewNonSoftwareRealization: %v", err)
	}
	patterns[0] = mustSourceRef(t, "C.28")
	returned := scope.GoverningPatternRefs()
	returned[0] = mustSourceRef(t, "A.1")
	if scope.GoverningPatternRefs()[0].String() != "A.7" {
		t.Fatal("non-software scope exposed mutable references")
	}

	_, err = projectprofile.NewScopeSet([]projectprofile.RealizationScope{
		mustSoftwareScope(t, "same-scope"),
		mustNonSoftwareScope(t, "same-scope"),
	})
	if err == nil || !strings.Contains(err.Error(), "duplicate scope_id") {
		t.Fatalf("duplicate error = %v", err)
	}
}

func mustDeclaredProfile(
	t *testing.T,
	values []projectprofile.RealizationScope,
) projectprofile.Declared {
	t.Helper()
	scopes := mustScopeSet(t, values)
	scopeDigest, err := projectprofile.DigestScopePayload(scopes)
	if err != nil {
		t.Fatalf("DigestScopePayload: %v", err)
	}
	record, err := projectprofile.NewOnboardingAgentDeclaredRecordBuilder(
		"authority-basis:profile-onboarding:1",
		digestOf(t, "candidate-provenance"),
		"admission-event:profile:1",
	).
		ForProject(testProjectRoot).
		ForScopePayload(scopeDigest).
		ForObservedBasis(mustObservedBasisDigest(t)).
		ObservedWithin(historicalWindow()).
		AtCarrierRevision(mustRevision(t, 1)).
		Build()
	if err != nil {
		t.Fatalf("Build OnboardingAgentDeclaredRecord: %v", err)
	}
	profile, err := projectprofile.NewDeclared(scopes, record)
	if err != nil {
		t.Fatalf("NewDeclared: %v", err)
	}
	return profile
}

func mustScopeSet(
	t *testing.T,
	values []projectprofile.RealizationScope,
) projectprofile.ScopeSet {
	t.Helper()
	set, err := projectprofile.NewScopeSet(values)
	if err != nil {
		t.Fatalf("NewScopeSet: %v", err)
	}
	return set
}

func mustObservedBasisDigest(t *testing.T) projectprofile.ContentDigest {
	t.Helper()
	basis, err := projectprofile.NewObservedBasis("repository:go.mod", "Go module is present")
	if err != nil {
		t.Fatalf("NewObservedBasis: %v", err)
	}
	digest, err := projectprofile.DigestObservedBasis([]projectprofile.ObservedBasis{basis})
	if err != nil {
		t.Fatalf("DigestObservedBasis: %v", err)
	}
	return digest
}

func digestOf(t *testing.T, seed string) projectprofile.ContentDigest {
	t.Helper()
	hex := fmt.Sprintf("%064x", []byte(seed))
	if len(hex) > 64 {
		hex = hex[:64]
	}
	digest, err := projectprofile.NewContentDigest("sha256:" + hex)
	if err != nil {
		t.Fatalf("NewContentDigest: %v", err)
	}
	return digest
}

func mustRevision(t *testing.T, value uint64) projectprofile.CarrierRevision {
	t.Helper()
	revision, err := projectprofile.NewCarrierRevision(value)
	if err != nil {
		t.Fatalf("NewCarrierRevision: %v", err)
	}
	return revision
}

func mustSoftwareScope(t *testing.T, raw string) projectprofile.SoftwareRealization {
	t.Helper()
	scope, err := projectprofile.NewSoftwareRealization(
		mustScopeID(t, raw),
		projectprofile.NoEntityReference{},
	)
	if err != nil {
		t.Fatalf("NewSoftwareRealization: %v", err)
	}
	return scope
}

func mustNonSoftwareScope(t *testing.T, raw string) projectprofile.NonSoftwareRealization {
	t.Helper()
	scope, err := projectprofile.NewNonSoftwareRealization(
		mustScopeID(t, raw),
		projectprofile.NoEntityReference{},
		projectprofile.UnspecifiedKindOrientation{},
		[]projectprofile.SourceUnitRef{},
		[]projectprofile.SpecSectionRef{},
	)
	if err != nil {
		t.Fatalf("NewNonSoftwareRealization: %v", err)
	}
	return scope
}

func mustScopeID(t *testing.T, raw string) projectprofile.ScopeID {
	t.Helper()
	value, err := projectprofile.NewScopeID(raw)
	if err != nil {
		t.Fatalf("NewScopeID: %v", err)
	}
	return value
}

func mustSourceRef(t *testing.T, raw string) projectprofile.SourceUnitRef {
	t.Helper()
	value, err := projectprofile.NewSourceUnitRef(raw)
	if err != nil {
		t.Fatalf("NewSourceUnitRef: %v", err)
	}
	return value
}

func historicalWindow() projectprofile.ObservationWindow {
	from := time.Date(2026, 7, 14, 8, 0, 0, 0, time.UTC)
	window, err := projectprofile.NewObservationWindow("onboarding:session:1", from, from.Add(time.Hour))
	if err != nil {
		panic(err)
	}
	return window
}

func assertMissingBasis(
	t *testing.T,
	result projectprofile.Applicability,
	want projectprofile.MissingBasis,
) {
	t.Helper()
	underdetermined, ok := result.(projectprofile.Underdetermined)
	if !ok {
		t.Fatalf("result = %T, want Underdetermined", result)
	}
	values := underdetermined.MissingBasis().Values()
	if len(values) != 1 || values[0] != want {
		t.Fatalf("missing basis = %#v, want %q", values, want)
	}
}
