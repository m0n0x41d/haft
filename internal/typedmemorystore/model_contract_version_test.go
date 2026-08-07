package typedmemorystore

import (
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestAdmissionContractVersionIsClosedAndCanonical(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		raw  string
		want AdmissionContractVersion
	}{
		{raw: "haft.memory.v1", want: AdmissionContractV1()},
		{raw: "haft.memory.v2", want: AdmissionContractV2()},
	}
	for _, testCase := range testCases {
		version, err := ParseAdmissionContractVersion(testCase.raw)
		if err != nil {
			t.Fatalf("ParseAdmissionContractVersion(%q): %v", testCase.raw, err)
		}
		if version != testCase.want || version.String() != testCase.raw {
			t.Fatalf("parsed version = %#v; want %#v", version, testCase.want)
		}
	}

	for _, raw := range []string{"", " haft.memory.v2", "haft.memory.v3"} {
		if _, err := ParseAdmissionContractVersion(raw); err == nil {
			t.Fatalf("ParseAdmissionContractVersion(%q) succeeded", raw)
		}
	}
	if (AdmissionContractVersion{}).String() != "" ||
		(AdmissionContractVersion{}).IsV1() ||
		(AdmissionContractVersion{}).IsV2() {
		t.Fatal("zero AdmissionContractVersion became a supported version")
	}
}

func TestAdmissionRequestsRequireAndRetainExplicitContractVersion(t *testing.T) {
	fixture := newSQLiteStoreFixture(t)
	candidate := fixture.declaration(
		t,
		"authorization-service",
		"Authorization service",
	)
	key := mustIdempotencyKey(t, "declare:authorization:versioned")
	provenance := mustRequestProvenanceRef(t)

	_, err := NewReplayRequestBuilder().
		SetProject(fixture.project).
		SetExpectedRevision(typedmemory.NewGraphRevision(0)).
		SetExpectedTypeEnv(fixture.environment.Ref()).
		SetIdempotencyKey(key).
		SetRequestProvenance(provenance).
		SetCandidate(candidate).
		Build()
	if err == nil || !strings.Contains(err.Error(), "contract version must be explicit") {
		t.Fatalf("unversioned replay error = %v", err)
	}

	_, err = NewCommitRequestBuilder().
		SetProject(fixture.project).
		SetExpectedRevision(typedmemory.NewGraphRevision(0)).
		SetExpectedTypeEnv(fixture.environment.Ref()).
		SetIdempotencyKey(key).
		SetRequestProvenance(provenance).
		SetCandidate(candidate).
		Build()
	if err == nil || !strings.Contains(err.Error(), "contract version must be explicit") {
		t.Fatalf("unversioned commit error = %v", err)
	}

	for _, version := range []AdmissionContractVersion{
		AdmissionContractV1(),
		AdmissionContractV2(),
	} {
		replay, buildErr := NewReplayRequestBuilder().
			SetContractVersion(version).
			SetProject(fixture.project).
			SetExpectedRevision(typedmemory.NewGraphRevision(0)).
			SetExpectedTypeEnv(fixture.environment.Ref()).
			SetIdempotencyKey(key).
			SetRequestProvenance(provenance).
			SetCandidate(candidate).
			Build()
		if buildErr != nil {
			t.Fatalf("build replay %q: %v", version.String(), buildErr)
		}
		if replay.ContractVersion() != version {
			t.Fatalf("replay version = %q; want %q", replay.ContractVersion().String(), version.String())
		}

		commit, buildErr := NewCommitRequestBuilder().
			SetContractVersion(version).
			SetProject(fixture.project).
			SetExpectedRevision(typedmemory.NewGraphRevision(0)).
			SetExpectedTypeEnv(fixture.environment.Ref()).
			SetIdempotencyKey(key).
			SetRequestProvenance(provenance).
			SetCandidate(candidate).
			Build()
		if buildErr != nil {
			t.Fatalf("build commit %q: %v", version.String(), buildErr)
		}
		if commit.ContractVersion() != version {
			t.Fatalf("commit version = %q; want %q", commit.ContractVersion().String(), version.String())
		}
	}
}
