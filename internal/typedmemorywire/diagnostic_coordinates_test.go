package typedmemorywire

import (
	"fmt"
	"testing"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func TestDiagnosticCoordinatesPreserveOriginalBindingOrder(t *testing.T) {
	payload := []byte(fmt.Sprintf(`{
  "contract_version": %q,
  "action": "validate",
  "basis": {"kind": "project_current"},
  "change_set": {"changes": [
    {
      "kind": "instantiate_relation",
      "assertion_id": "assertion-1",
      "signature_id": "Local.Relation",
      "context_slice": %s,
      "bindings": [
        {
          "slot_kind": "Local.ZSlot",
          "fillers": [{
            "kind": "by_reference",
            "reference": {
              "kind": "persisted",
              "ref_kind": "U.EntityRef",
              "id": "entity-z"
            }
          }]
        },
        {
          "slot_kind": "Local.ASlot",
          "fillers": [{
            "kind": "by_reference",
            "reference": {
              "kind": "persisted",
              "ref_kind": "U.EntityRef",
              "id": "entity-a"
            }
          }]
        }
      ],
      "provenance": "fixture:diagnostic-coordinate"
    }
  ]}
}`, ContractVersion, testContextSliceJSON("project")))
	request, err := DecodeValidateRequest(payload)
	if err != nil {
		t.Fatalf("DecodeValidateRequest() error = %v", err)
	}

	coordinates := request.DiagnosticCoordinates()
	if coordinates.ChangeCount() != 1 {
		t.Fatalf("ChangeCount() = %d", coordinates.ChangeCount())
	}
	kind, found := coordinates.ChangeKind(0)
	if !found || kind != DiagnosticChangeInstantiateRelation {
		t.Fatalf("ChangeKind(0) = %d, %v", kind, found)
	}
	zSlot := mustDiagnosticSlotKind(t, "Local.ZSlot")
	aSlot := mustDiagnosticSlotKind(t, "Local.ASlot")
	zOrdinal, found := coordinates.BindingOrdinal(0, zSlot)
	if !found || zOrdinal != 0 {
		t.Fatalf("Z binding ordinal = %d, %v", zOrdinal, found)
	}
	aOrdinal, found := coordinates.BindingOrdinal(0, aSlot)
	if !found || aOrdinal != 1 {
		t.Fatalf("A binding ordinal = %d, %v", aOrdinal, found)
	}
	fillerCount, found := coordinates.FillerCount(0, aSlot)
	if !found || fillerCount != 1 {
		t.Fatalf("A filler count = %d, %v", fillerCount, found)
	}

	changeSet, err := request.BindChangeSet(testTypeEnvRef(t))
	if err != nil {
		t.Fatalf("BindChangeSet() error = %v", err)
	}
	relation := changeSet.Changes()[0].(typedmemory.InstantiateRelation).Relation()
	bindings := relation.Bindings()
	if bindings[0].Name() != aSlot || bindings[1].Name() != zSlot {
		t.Fatalf(
			"semantic bindings are not normalized independently: %s, %s",
			bindings[0].Name().String(),
			bindings[1].Name().String(),
		)
	}
	second := request.DiagnosticCoordinates()
	secondZOrdinal, secondFound := second.BindingOrdinal(0, zSlot)
	if !secondFound || secondZOrdinal != 0 {
		t.Fatalf("second coordinate copy = %d, %v", secondZOrdinal, secondFound)
	}
}

func TestDiagnosticCoordinatesCaptureNestedIdentityVariant(t *testing.T) {
	payload := requestWithIdentityChange(
		`{"kind":"merge_entities","survivor":"e1","merged":["e2"],"context":"project","basis":"review:1"}`,
	)
	request, err := DecodeValidateRequest(payload)
	if err != nil {
		t.Fatalf("DecodeValidateRequest() error = %v", err)
	}

	coordinates := request.DiagnosticCoordinates()
	kind, found := coordinates.ChangeKind(0)
	if !found || kind != DiagnosticChangeIdentity {
		t.Fatalf("ChangeKind(0) = %d, %v", kind, found)
	}
	identityKind, found := coordinates.IdentityKind(0)
	if !found || identityKind != DiagnosticIdentityMergeEntities {
		t.Fatalf("IdentityKind(0) = %d, %v", identityKind, found)
	}
}

func TestZeroValidateRequestHasNoDiagnosticCoordinates(t *testing.T) {
	coordinates := (ValidateRequest{}).DiagnosticCoordinates()
	if coordinates.ChangeCount() != 0 {
		t.Fatalf("zero request coordinate count = %d", coordinates.ChangeCount())
	}
}

func mustDiagnosticSlotKind(t *testing.T, raw string) typedmemory.SlotKindID {
	t.Helper()
	value, err := typedmemory.NewSlotKindID(raw)
	if err != nil {
		t.Fatalf("NewSlotKindID(%q) error = %v", raw, err)
	}
	return value
}
