package profiledeclarationpreparation

import (
	"path/filepath"
	"testing"

	"github.com/m0n0x41d/haft/internal/profiledetector"
)

func TestInspectGeneratedProfileReviewDistinguishesUneditedAndEnriched(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := profiledetector.NewSnapshot(
		root,
		[]string{"go.mod", "internal/core.go"},
		2,
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	suggestion := profiledetector.Detect(snapshot)
	content, err := ProposeProfileOnboardingWorkInput(suggestion)
	if err != nil {
		t.Fatal(err)
	}
	review, ok := InspectGeneratedProfileReview(content)
	if !ok {
		t.Fatal("generated review was not recognized")
	}
	if review.ObservationDigest() != snapshot.ObservationDigest() {
		t.Fatalf("observation digest = %q", review.ObservationDigest())
	}
	enriched := append([]byte{}, content...)
	enriched = []byte(string(enriched[:len(enriched)-3]) + ",\n      \"entity_ref\": \"entity:software\"\n    }\n  ]\n}\n")
	if _, recognized := InspectGeneratedProfileReview(enriched); recognized {
		t.Fatal("semantically enriched review was recognized as generated and unedited")
	}
}

func TestInspectGeneratedProfileReviewRecognizesLegacyPartialSuggestion(t *testing.T) {
	t.Parallel()

	content := []byte(`{
  "schema": "haft.profile-onboarding.work-input/v1",
  "project_root": "/tmp/project",
  "suggestion_ref": "profile-suggestion:sha256:7708e5ba64e30def62ebd6fa62b3a1681331b003bcdd4cc8ef8e674f41f1ce76",
  "detector_version": "haft.project-profile-detector/file-metadata-v3",
  "policy_version": "haft.project-profile-detector-policy/supported-singleton-v3",
  "observation_digest": "sha256:3e02d0df4a830f8c15046813b04f476bc247fdfae78946ec882a179a0b09993d",
  "scopes": [
    {
      "component_candidate_ref": "profile-component-suggestion:sha256:2b528f6915d3e93e2b726e0742c384325e8dd131b9ca2e39c096a916b174aff5",
      "scope_id": "documents",
      "realization_kind": "non_software"
    }
  ]
}
`)

	if _, recognized := InspectGeneratedProfileReview(content); !recognized {
		t.Fatal("legacy generated partial review was not recognized")
	}
}
