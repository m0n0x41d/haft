package specmigrationv2_test

import (
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/specmigrationv2"
)

func TestSHA256RequiresCanonicalLowercasePrefixedForm(t *testing.T) {
	canonical := "sha256:" + strings.Repeat("a", 64)
	value, err := specmigrationv2.NewSHA256(canonical)
	if err != nil {
		t.Fatalf("NewSHA256: %v", err)
	}
	if value.String() != canonical {
		t.Fatalf("digest = %q, want %q", value.String(), canonical)
	}

	invalid := []string{
		strings.Repeat("a", 64),
		"sha256:" + strings.Repeat("A", 64),
		"sha256:abc",
		" " + canonical,
		canonical + " ",
	}
	for _, raw := range invalid {
		_, parseErr := specmigrationv2.NewSHA256(raw)
		if parseErr == nil {
			t.Errorf("NewSHA256(%q) succeeded", raw)
		}
	}
}

func TestStrongIdentifiersRejectSilentWhitespaceNormalization(t *testing.T) {
	constructors := []struct {
		name string
		run  func() error
	}{
		{
			name: "migration packet",
			run: func() error {
				_, err := specmigrationv2.NewMigrationPacketID(" migration-v2")
				return err
			},
		},
		{
			name: "source section",
			run: func() error {
				_, err := specmigrationv2.NewSourceSectionID("ES.alpha.001 ")
				return err
			},
		},
		{
			name: "target section",
			run: func() error {
				_, err := specmigrationv2.NewTargetSectionID(" SS.alpha.001")
				return err
			},
		},
		{
			name: "target claim",
			run: func() error {
				_, err := specmigrationv2.NewTargetAtomicClaimID("SS.alpha.001.L1\n")
				return err
			},
		},
		{
			name: "carrier path",
			run: func() error {
				_, err := specmigrationv2.NewSourceCarrierID(" .haft/specs/source.md")
				return err
			},
		},
	}
	for _, constructor := range constructors {
		t.Run(constructor.name, func(t *testing.T) {
			if err := constructor.run(); err == nil {
				t.Fatal("constructor accepted a non-canonical identifier")
			}
		})
	}
}

func TestSourceAndTargetIdentifiersAreShapeSpecific(t *testing.T) {
	_, sourceErr := specmigrationv2.NewSourceSectionID("SS.functional.query.001")
	if sourceErr == nil {
		t.Error("source section constructor accepted target section ID")
	}
	_, targetErr := specmigrationv2.NewTargetSectionID("ES.runtime-policy.001")
	if targetErr == nil {
		t.Error("target section constructor accepted source section ID")
	}
	claim, claimErr := specmigrationv2.NewTargetAtomicClaimID("SS.functional.query.001.D2")
	if claimErr != nil {
		t.Fatalf("NewTargetAtomicClaimID: %v", claimErr)
	}
	if claim.Section().String() != "SS.functional.query.001" {
		t.Fatalf("claim section = %q", claim.Section().String())
	}
}

func TestCarrierIdentifiersRejectTraversalAbsoluteAndBackslashPaths(t *testing.T) {
	invalid := []string{"../secret", "/tmp/target", `docs\target.md`, "./target.md"}
	for _, raw := range invalid {
		_, err := specmigrationv2.NewSourceCarrierID(raw)
		if err == nil {
			t.Errorf("NewSourceCarrierID(%q) succeeded", raw)
		}
	}
}
