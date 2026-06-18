package artifact

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildEngineeringValueSpaceHasNoSingleScore(t *testing.T) {
	space := BuildEngineeringValueSpace(EngineeringValueSpaceInput{
		BearerRef:    "release-1",
		Window:       "2026-Q3",
		MethodRef:    "method-1",
		EvidenceRefs: []string{"evid-1"},
	})

	if space.ScorePolicy.SingleScore != EngineeringValueNoSingleScore {
		t.Fatalf("single score policy = %q", space.ScorePolicy.SingleScore)
	}
	if space.ScorePolicy.Aggregation != EngineeringValueCharacteristicOnly {
		t.Fatalf("aggregation = %q", space.ScorePolicy.Aggregation)
	}
	payload, err := json.Marshal(space)
	if err != nil {
		t.Fatalf("marshal value space: %v", err)
	}
	if strings.Contains(string(payload), "total_score") {
		t.Fatalf("value space must not expose a total score: %s", payload)
	}
}

func TestBuildEngineeringValueSpaceEveryMeasureNamesRequiredInterpretationFields(t *testing.T) {
	space := BuildEngineeringValueSpace(EngineeringValueSpaceInput{
		BearerRef:    "release-1",
		Window:       "2026-Q3",
		MethodRef:    "method-1",
		EvidenceRefs: []string{"evid-1", " evid-2 "},
	})

	if len(space.Characteristics) == 0 {
		t.Fatalf("expected characteristics")
	}
	for _, characteristic := range space.Characteristics {
		if characteristic.BearerRef == "" {
			t.Fatalf("%s missing bearer_ref", characteristic.ID)
		}
		if characteristic.Method == "" {
			t.Fatalf("%s missing method", characteristic.ID)
		}
		if characteristic.Window == "" {
			t.Fatalf("%s missing window", characteristic.ID)
		}
		if characteristic.Denominator == "" {
			t.Fatalf("%s missing denominator", characteristic.ID)
		}
		if len(characteristic.EvidenceRefs) != 2 {
			t.Fatalf("%s evidence refs = %#v", characteristic.ID, characteristic.EvidenceRefs)
		}
		if characteristic.ReopenCondition == "" {
			t.Fatalf("%s missing reopen condition", characteristic.ID)
		}
	}
}

func TestBuildEngineeringValueSpaceMissingEvidenceBlocksValueClaim(t *testing.T) {
	space := BuildEngineeringValueSpace(EngineeringValueSpaceInput{
		BearerRef: "release-1",
	})

	for _, characteristic := range space.Characteristics {
		if characteristic.Missingness != "evidence_refs_missing_value_claim_blocked" {
			t.Fatalf("%s missingness = %q", characteristic.ID, characteristic.Missingness)
		}
	}
}

func TestBuildEngineeringValueSpaceTreatsHealthyReopeningSeparately(t *testing.T) {
	space := BuildEngineeringValueSpace(EngineeringValueSpaceInput{
		BearerRef: "release-1",
	})

	if space.InterpretationRules.HealthyReopening != EngineeringValueHealthyReopenTreatment {
		t.Fatalf("healthy reopening rule = %q", space.InterpretationRules.HealthyReopening)
	}
	if space.AuthorityBoundary.Score != EngineeringValueBoundaryNotScore {
		t.Fatalf("score boundary = %+v", space.AuthorityBoundary)
	}
}
