package profileadmissionfixture

import (
	"testing"

	"github.com/m0n0x41d/haft/internal/projectprofile"
)

func TestFixtureProfileWorkInputPreservesSoftwareNonSoftwareAndMixedPayloads(
	t *testing.T,
) {
	root := mustValue(t, t.TempDir(), projectprofile.NewProjectRootV1)
	software := newIntegrationPayload(t, "v3-input-software")
	nonSoftware := newNonSoftwareIntegrationPayload(t, "v3-input-documents")
	mixed := newMixedIntegrationPayload(t)
	cases := []struct {
		name    string
		payload projectprofile.ProfileDeclarationPayload
	}{
		{name: "software", payload: software},
		{name: "non-software", payload: nonSoftware},
		{name: "mixed", payload: mixed},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			input := newFixtureProfileWorkInput(t, root, test.payload)
			if !input.Valid() {
				t.Fatal("fixture WorkInput is invalid")
			}
			want, err := projectprofile.DigestProfileDeclarationPayload(test.payload)
			if err != nil {
				t.Fatal(err)
			}
			if input.PayloadDigest() != want {
				t.Fatalf("payload digest = %s, want %s", input.PayloadDigest().String(), want.String())
			}
		})
	}
}
