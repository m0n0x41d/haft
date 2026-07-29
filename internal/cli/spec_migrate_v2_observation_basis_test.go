package cli

import (
	"context"
	"encoding/json"
	"testing"
)

func TestSpecMigrationV2ObservationBasisRejectsEveryBoundDimension(t *testing.T) {
	fixture := newCLISpecMigrationV2Fixture(t, true)
	observation, closeLedger, err := observeSpecMigrationV2ForTest(
		context.Background(),
		fixture.root,
		fixture.packetPath,
	)
	if err != nil {
		t.Fatalf("observeSpecMigrationV2: %v", err)
	}
	defer closeLedger()
	canonical, err := encodeSpecMigrationV2ObservationBasis(observation)
	if err != nil {
		t.Fatalf("encode observation basis: %v", err)
	}
	if err := compareSpecMigrationV2CanonicalObservationBasis(canonical, canonical); err != nil {
		t.Fatalf("exact observation replay rejected: %v", err)
	}

	cases := map[string]func(*specMigrationV2ObservationBasis){
		"packet carrier": func(basis *specMigrationV2ObservationBasis) {
			basis.PacketCarrier = append(basis.PacketCarrier, byte('x'))
		},
		"project root": func(basis *specMigrationV2ObservationBasis) {
			basis.ProjectRoot += ".replacement"
		},
		"Git witness": func(basis *specMigrationV2ObservationBasis) {
			basis.GitWitness = append(basis.GitWitness, byte('x'))
		},
		"profile binding": func(basis *specMigrationV2ObservationBasis) {
			basis.Profile.LedgerRevision++
		},
		"source snapshot": func(basis *specMigrationV2ObservationBasis) {
			basis.Source.Bytes = append(basis.Source.Bytes, byte('x'))
		},
		"target snapshot": func(basis *specMigrationV2ObservationBasis) {
			basis.Target.Bytes = append(basis.Target.Bytes, byte('x'))
		},
		"target claims": func(basis *specMigrationV2ObservationBasis) {
			basis.TargetClaims.Claims = append(basis.TargetClaims.Claims, "SS.drift.L1")
		},
		"outside carrier": func(basis *specMigrationV2ObservationBasis) {
			basis.Outside = append(basis.Outside, specMigrationV2OutsideCarrierBasis{
				ID:      "outside:drift",
				Carrier: ".context/drift.md",
				Bytes:   []byte("drift"),
			})
		},
		"partition audit": func(basis *specMigrationV2ObservationBasis) {
			basis.PartitionAudit = append(basis.PartitionAudit, byte('x'))
		},
		"review carrier": func(basis *specMigrationV2ObservationBasis) {
			basis.ReviewSoftwareCarrier += ".replacement"
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			changed := cloneSpecMigrationV2ObservationBasis(t, canonical)
			mutate(&changed)
			changedCanonical, err := json.Marshal(changed)
			if err != nil {
				t.Fatal(err)
			}
			if err := compareSpecMigrationV2CanonicalObservationBasis(canonical, changedCanonical); err == nil {
				t.Fatal("changed observation basis was accepted")
			}
		})
	}
}

func cloneSpecMigrationV2ObservationBasis(
	t *testing.T,
	canonical []byte,
) specMigrationV2ObservationBasis {
	t.Helper()
	value := specMigrationV2ObservationBasis{}
	if err := json.Unmarshal(canonical, &value); err != nil {
		t.Fatal(err)
	}
	return value
}
