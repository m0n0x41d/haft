package neighborhood_test

import (
	"slices"
	"testing"

	"github.com/m0n0x41d/haft/internal/projectmemory/neighborhood"
)

func TestBuiltinProjectionProfilesAreClosedCanonicalAndImmutable(t *testing.T) {
	profiles := neighborhood.BuiltinProjectionProfiles()
	if len(profiles) != 6 {
		t.Fatalf("profile count = %d, want 6", len(profiles))
	}
	want := []string{
		"agent_orientation.v1",
		"agent_orientation.v2",
		"decision_rationale.v1",
		"evidence_currentness.v1",
		"implementation_trace.v1",
		"spec_impact.v1",
	}
	wantDigest := map[string]string{
		"agent_orientation.v1":    "sha256:1abb9b2c0b37a1c09b26f06bea257c9ce82cb14e1e0a1c7f8741e4b3889f867b",
		"agent_orientation.v2":    "sha256:fe4c1c013b423e5fe16fcfd454d375696e5424d9c305074097fce80879e4a06c",
		"decision_rationale.v1":   "sha256:aa3ae9f4c1f6645bb91e3cd5120133fab0f876622655b5399c4dca349bc3aff7",
		"evidence_currentness.v1": "sha256:6fcf246991f9af6370324672d52f89ff51c1c1265f42cdc418f37f85f66cab7c",
		"implementation_trace.v1": "sha256:fc58a9e8c22e9895868791e295b66d09c96c7757616e34822e43c8846acd8926",
		"spec_impact.v1":          "sha256:7059e102821b431ea26d4bd9026e797a46ddcbb7c03d0cbb17d784e60dace2f1",
	}
	digests := make(map[string]struct{}, len(profiles))
	for index, profile := range profiles {
		if !profile.Valid() {
			t.Fatalf("profile %d is invalid", index)
		}
		if profile.Ref().String() != want[index] {
			t.Fatalf(
				"profile %d = %q, want %q",
				index,
				profile.Ref().String(),
				want[index],
			)
		}
		digest := profile.Digest().String()
		if digest != wantDigest[profile.Ref().String()] {
			t.Fatalf(
				"profile %q digest = %q, want %q",
				profile.Ref().String(),
				digest,
				wantDigest[profile.Ref().String()],
			)
		}
		if _, found := digests[digest]; found {
			t.Fatalf("profile digest %q is duplicated", digest)
		}
		digests[digest] = struct{}{}
	}

	firstFacets := profiles[0].Facets()
	slices.Reverse(firstFacets)
	ref, err := neighborhood.ParseProjectionProfileRef("agent_orientation.v1")
	if err != nil {
		t.Fatal(err)
	}
	reloaded, found := neighborhood.LookupProjectionProfile(ref)
	if !found || !reloaded.Valid() {
		t.Fatal("mutating returned facets changed the registry")
	}
	if reloaded.Facets()[0] != neighborhood.FacetEpistemes {
		t.Fatalf("first facet = %q", reloaded.Facets()[0])
	}
}

func TestProjectionProfileItemMappingIsExplicitAndProfileLocal(t *testing.T) {
	legacy := mustProfile(t, "agent_orientation.v1")
	facet, found := legacy.FacetForItem(neighborhood.ItemDecisionRecord)
	if !found || facet != neighborhood.FacetDecisions {
		t.Fatalf("decision mapping = %q, found=%t", facet, found)
	}
	if _, found := legacy.FacetForItem(neighborhood.ItemNoteRecord); found {
		t.Fatal("agent_orientation.v1 silently changed its item/facet mapping")
	}

	current := mustProfile(t, "agent_orientation.v2")
	noteFacet, noteFound := current.FacetForItem(
		neighborhood.ItemNoteRecord,
	)
	if !noteFound || noteFacet != neighborhood.FacetEpistemes {
		t.Fatalf("note mapping = %q, found=%t", noteFacet, noteFound)
	}

	rationale := mustProfile(t, "decision_rationale.v1")
	if _, found := rationale.FacetForItem(neighborhood.ItemCodeAnchor); found {
		t.Fatal("decision-rationale profile silently admitted implementation")
	}
	if !rationale.AllowsFacet(neighborhood.FacetAlternatives) {
		t.Fatal("decision-rationale profile omitted alternatives")
	}
}

func TestProjectionProfileReferencesRejectUnknownOrNonCanonicalInput(t *testing.T) {
	invalid := []string{
		"",
		" agent_orientation.v1",
		"agent-orientation.v1",
		"agent_orientation",
		"agent_orientation.v0",
	}
	for _, raw := range invalid {
		if _, err := neighborhood.ParseProjectionProfileRef(raw); err == nil {
			t.Fatalf("profile ref %q was accepted", raw)
		}
	}
	unknown, err := neighborhood.ParseProjectionProfileRef("unknown.v1")
	if err != nil {
		t.Fatal(err)
	}
	if _, found := neighborhood.LookupProjectionProfile(unknown); found {
		t.Fatal("syntactically valid unknown profile was resolved")
	}
}

func mustProfile(
	t *testing.T,
	raw string,
) neighborhood.ProjectionProfileDefinition {
	t.Helper()
	ref, err := neighborhood.ParseProjectionProfileRef(raw)
	if err != nil {
		t.Fatal(err)
	}
	profile, found := neighborhood.LookupProjectionProfile(ref)
	if !found {
		t.Fatalf("profile %q is absent", raw)
	}
	return profile
}
