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
