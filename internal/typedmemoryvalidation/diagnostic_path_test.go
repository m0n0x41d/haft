package typedmemoryvalidation

import (
	"fmt"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorywire"
)

func TestDiagnosticPathProjectorUsesExactStrictWireCoordinates(t *testing.T) {
	request := decodeDiagnosticPathRequest(t)
	projector, err := newDiagnosticPathProjector(request)
	if err != nil {
		t.Fatalf("newDiagnosticPathProjector() error = %v", err)
	}
	tests := []struct {
		name string
		code typedmemory.DiagnosticCode
		path string
		want string
	}{
		{
			name: "declaration context",
			path: "changes[0].bounded_context",
			want: "$.change_set.changes[0].context",
		},
		{
			name: "nested identity context",
			path: "changes[1].bounded_context",
			want: "$.change_set.changes[1].change.context",
		},
		{
			name: "nested reconciliation basis",
			path: "changes[1].reconciliation_basis_ref",
			want: "$.change_set.changes[1].change.basis",
		},
		{
			name: "relation signature",
			path: "changes[2].signature",
			want: "$.change_set.changes[2].signature_id",
		},
		{
			name: "relation context slice",
			path: "changes[2].bounded_context",
			want: "$.change_set.changes[2].context_slice.context",
		},
		{
			name: "context slice gamma",
			path: "changes[2].context_slice.gamma_time",
			want: "$.change_set.changes[2].context_slice.gamma_time",
		},
		{
			name: "unknown slot uses original binding zero",
			code: typedmemory.DiagnosticUnknownSlot,
			path: "changes[2].slots.Local.ZSlot",
			want: "$.change_set.changes[2].bindings[0].slot_kind",
		},
		{
			name: "cardinality uses original binding one",
			code: typedmemory.DiagnosticCardinalityMismatch,
			path: "changes[2].slots.Local.ASlot",
			want: "$.change_set.changes[2].bindings[1].fillers",
		},
		{
			name: "indexed reference filler",
			path: "changes[2].slots.Local.ASlot.fillers[0].ref_kind",
			want: "$.change_set.changes[2].bindings[1].fillers[0].reference.ref_kind",
		},
		{
			name: "indexed value kind",
			path: "changes[2].slots.Local.MSlot.fillers[0].value_kind_ref",
			want: "$.change_set.changes[2].bindings[2].fillers[0].value.value_kind",
		},
		{
			name: "indexed typed value bytes",
			path: "changes[2].slots.Local.MSlot.fillers[0].value.canonical_bytes",
			want: "$.change_set.changes[2].bindings[2].fillers[0].value.input_base64",
		},
		{
			name: "retraction assertion",
			path: "changes[3].assertion_id",
			want: "$.change_set.changes[3].assertion_id",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := mustCoreDiagnosticPath(t, test.path)
			projected, projectionErr := projector.project(test.code, "", path)
			if projectionErr != nil {
				t.Fatalf("project() error = %v", projectionErr)
			}
			if projected.kind != DiagnosticPathRequestJSON {
				t.Fatalf("kind = %q", projected.kind)
			}
			if projected.value != test.want {
				t.Fatalf("value = %q, want %q", projected.value, test.want)
			}
		})
	}
}

func TestDiagnosticPathProjectorLabelsNonWireSemanticCoordinates(t *testing.T) {
	request := decodeDiagnosticPathRequest(t)
	projector, err := newDiagnosticPathProjector(request)
	if err != nil {
		t.Fatalf("newDiagnosticPathProjector() error = %v", err)
	}
	tests := []string{
		"validation.core",
		"changes[1].merged[0]",
		"changes[2].slots.Local.RequiredButMissing",
		"changes[2].slots.Local.ASlot.fillers[0].value_kind",
		"changes[2].slots.Local.MSlot.fillers[0].value.binding",
	}
	for _, raw := range tests {
		t.Run(raw, func(t *testing.T) {
			path := mustCoreDiagnosticPath(t, raw)
			projected, projectionErr := projector.project("fixture", "", path)
			if projectionErr != nil {
				t.Fatalf("project() error = %v", projectionErr)
			}
			if projected.kind != DiagnosticPathTypedMemorySemantic {
				t.Fatalf("kind = %q", projected.kind)
			}
			want := typedMemorySemanticPathPrefix + raw
			if projected.value != want {
				t.Fatalf("value = %q, want %q", projected.value, want)
			}
		})
	}
}

func TestDiagnosticPathProjectorDoesNotMislabelNormalizedFillerOrdinalAsWire(t *testing.T) {
	request := decodeDiagnosticPathRequest(t)
	projector, err := newDiagnosticPathProjector(request)
	if err != nil {
		t.Fatalf("newDiagnosticPathProjector() error = %v", err)
	}
	path := mustCoreDiagnosticPath(
		t,
		"changes[2].slots.Local.ASlot.fillers[0]",
	)
	projected, err := projector.project(
		typedmemory.DiagnosticTypeRuleUnavailable,
		"admitted reference filler has no exact validation evidence",
		path,
	)
	if err != nil {
		t.Fatalf("project() error = %v", err)
	}
	if projected.kind != DiagnosticPathTypedMemorySemantic {
		t.Fatalf("kind = %q", projected.kind)
	}
}

func TestDiagnosticProjectionKeepsStringPathWireCompatibility(t *testing.T) {
	projection := newUnderDiagnostic(
		"fixture",
		"fixture projection",
		"$.change_set",
		"expected",
		"actual",
		"inspect-fixture",
	)
	projection.path = typedMemorySemanticPathPrefix + "validation.core"
	projection.pathKind = DiagnosticPathTypedMemorySemantic
	payload, err := projection.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON() error = %v", err)
	}
	want := `"path":"typed-memory-semantic:validation.core"`
	if !strings.Contains(string(payload), want) {
		t.Fatalf("payload = %s, want fragment %s", payload, want)
	}
	if strings.Contains(string(payload), `"path":{`) {
		t.Fatalf("path changed from its v1 string shape: %s", payload)
	}
}

func decodeDiagnosticPathRequest(t *testing.T) typedmemorywire.ValidateRequest {
	t.Helper()
	payload := fmt.Sprintf(`{
  "contract_version": %q,
  "action": "validate",
  "basis": {"kind": "project_current"},
  "change_set": {"changes": [
    {
      "kind": "declare_entity",
      "entity_id": "entity:declared",
      "local_ref": "local:declared",
      "context": "context:project",
      "label": "Declared entity",
      "provenance": "fixture:declaration"
    },
    {
      "kind": "identity_change",
      "change": {
        "kind": "merge_entities",
        "survivor": "entity:survivor",
        "merged": ["entity:merged"],
        "context": "context:project",
        "basis": "review:merge"
      }
    },
    {
      "kind": "instantiate_relation",
      "assertion_id": "assertion:relation",
      "signature_id": "Local.Relation",
      "context_slice": {
        "context": "context:project",
        "standard_pins": [],
        "environment_selectors": [],
        "vocabulary_pins": [],
        "role_set_pins": [],
        "gamma_time": {
          "kind": "point",
          "at": "2026-07-16T08:00:00Z"
        }
      },
      "bindings": [
        {
          "slot_kind": "Local.ZSlot",
          "fillers": [{
            "kind": "by_reference",
            "reference": {
              "kind": "persisted",
              "ref_kind": "U.EntityRef",
              "id": "entity:z"
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
              "id": "entity:a"
            }
          }]
        },
        {
          "slot_kind": "Local.MSlot",
          "fillers": [{
            "kind": "by_value",
            "value": {
              "value_kind": "U.ClaimGraph",
              "value_shape": {"id": "Haft.ClaimGraph", "digest": %q},
              "codec": {
                "id": "Haft.ClaimGraphCodec",
                "version": "v1",
                "specification_digest": %q
              },
              "input_base64": "e30=",
              "asserted_digest": {"kind": "none"}
            }
          }]
        }
      ],
      "provenance": "fixture:relation"
    },
    {
      "kind": "retract_assertion",
      "assertion_id": "assertion:old",
      "reason": "superseded",
      "provenance": "fixture:retraction"
    }
  ]}
}`, typedmemorywire.ContractVersion, diagnosticPathDigest, diagnosticPathDigest)
	request, err := typedmemorywire.DecodeValidateRequest([]byte(payload))
	if err != nil {
		t.Fatalf("DecodeValidateRequest() error = %v\npayload=%s", err, payload)
	}
	return request
}

const diagnosticPathDigest = "sha256:1111111111111111111111111111111111111111111111111111111111111111"

func mustCoreDiagnosticPath(t *testing.T, raw string) typedmemory.DiagnosticPath {
	t.Helper()
	path, err := typedmemory.NewDiagnosticPath(raw)
	if err != nil {
		t.Fatalf("NewDiagnosticPath(%q) error = %v", raw, err)
	}
	return path
}
