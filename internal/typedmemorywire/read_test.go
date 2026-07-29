package typedmemorywire

import (
	"fmt"
	"strings"
	"testing"
)

func TestDecodeRequestKeepsMemoryReadActionsDisjoint(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
		assert  func(*testing.T, Request)
	}{
		{
			name:    "resolve",
			payload: resolveReadPayload(`{"kind":"project_current"}`, ""),
			assert: func(t *testing.T, request Request) {
				t.Helper()
				resolved, ok := request.(ResolveReadRequest)
				if !ok || !IsDecodedResolveReadRequest(resolved) {
					t.Fatalf("resolve request = %T", request)
				}
				if resolved.Query() != "authentication service" ||
					resolved.MaxCandidates() != 7 {
					t.Fatalf("resolve coordinates = %#v", resolved)
				}
				if _, hasContext := resolved.Context(); hasContext {
					t.Fatal("omitted context did not remain any-context")
				}
			},
		},
		{
			name:    "neighborhood",
			payload: neighborhoodReadPayload(`{"kind":"project_current"}`),
			assert: func(t *testing.T, request Request) {
				t.Helper()
				read, ok := request.(NeighborhoodReadRequest)
				if !ok || !IsDecodedNeighborhoodReadRequest(read) {
					t.Fatalf("neighborhood request = %T", request)
				}
				if read.Entity().RefKindID().String() != "U.EntityRef" ||
					read.Entity().ReferenceID().String() != "service:auth" ||
					read.Context().String() != "context:project" {
					t.Fatalf("neighborhood identity = %#v", read)
				}
				if read.View().ProjectionProfileRef() !=
					"agent_orientation.v1" {
					t.Fatalf("view = %#v", read.View())
				}
				if read.ReadBudget().MaxItemsPerFacet() != 20 {
					t.Fatalf("budget = %#v", read.ReadBudget())
				}
			},
		},
		{
			name:    "recall",
			payload: recallReadPayload(`{"kind":"project_current"}`),
			assert: func(t *testing.T, request Request) {
				t.Helper()
				read, ok := request.(RecallReadRequest)
				if !ok || !IsDecodedRecallReadRequest(read) {
					t.Fatalf("recall request = %T", request)
				}
				if read.Query() != "CAS invariant" ||
					read.CandidateBudget() != 8 {
					t.Fatalf("recall coordinates = %#v", read)
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request, err := DecodeRequest(test.payload)
			if err != nil {
				t.Fatal(err)
			}
			if request.ContractVersion() != ContractVersion {
				t.Fatalf(
					"contract version = %q",
					request.ContractVersion(),
				)
			}
			test.assert(t, request)
		})
	}
}

func TestDecodeResolveReadRequestRetainsExactOptionalContext(t *testing.T) {
	payload := resolveReadPayload(
		exactReadBasisJSON(11),
		`,"bounded_context_ref":"context:project"`,
	)
	request, err := DecodeResolveReadRequest(payload)
	if err != nil {
		t.Fatal(err)
	}
	context, found := request.Context()
	if !found || context.String() != "context:project" {
		t.Fatalf("exact context = %#v, found=%t", context, found)
	}
	exact, ok := request.Basis().(ExactProjectSelector)
	if !ok ||
		exact.RequestedGraphRevision().Value() != 11 ||
		exact.RequestedTypeEnvDigest().String() != testDigest {
		t.Fatalf("exact basis = %#v", request.Basis())
	}
}

func TestMemoryReadDecoderRejectsCrossVariantAndWeakBasis(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
		path    string
	}{
		{
			name: "bundled basis",
			payload: resolveReadPayload(
				`{"kind":"bundled_candidate_open_world"}`,
				"",
			),
			path: "$.basis.kind",
		},
		{
			name:    "zero exact graph revision",
			payload: resolveReadPayload(exactReadBasisJSON(0), ""),
			path:    "$.basis.graph_revision",
		},
		{
			name: "validate field in resolve",
			payload: []byte(strings.Replace(
				string(resolveReadPayload(`{"kind":"project_current"}`, "")),
				`"query":"authentication service"`,
				`"query":"authentication service","change_set":{"changes":[]}`,
				1,
			)),
			path: "$",
		},
		{
			name: "MCP mode in dedicated resolve request",
			payload: []byte(strings.Replace(
				string(resolveReadPayload(`{"kind":"project_current"}`, "")),
				`"action":"resolve"`,
				`"action":"resolve","mode":"resolve"`,
				1,
			)),
			path: "$",
		},
		{
			name: "resolve field in neighborhood",
			payload: []byte(strings.Replace(
				string(neighborhoodReadPayload(`{"kind":"project_current"}`)),
				`"entity_ref":`,
				`"query":"authentication service","entity_ref":`,
				1,
			)),
			path: "$",
		},
		{
			name: "trimmed query required",
			payload: []byte(strings.Replace(
				string(resolveReadPayload(`{"kind":"project_current"}`, "")),
				`"authentication service"`,
				`" authentication service "`,
				1,
			)),
			path: "$.query",
		},
		{
			name: "missing explicit history posture",
			payload: []byte(strings.Replace(
				string(neighborhoodReadPayload(`{"kind":"project_current"}`)),
				"\"detail\":\"standard\",\n    \"include_history\":false",
				"\"detail\":\"standard\"",
				1,
			)),
			path: "$.view.include_history",
		},
		{
			name: "zero dimension",
			payload: []byte(strings.Replace(
				string(neighborhoodReadPayload(`{"kind":"project_current"}`)),
				`"max_items_per_facet":20`,
				`"max_items_per_facet":0`,
				1,
			)),
			path: "$.read_budget.max_items_per_facet",
		},
		{
			name: "duplicate query",
			payload: []byte(strings.Replace(
				string(recallReadPayload(`{"kind":"project_current"}`)),
				`"query":"CAS invariant"`,
				`"query":"CAS invariant","query":"other"`,
				1,
			)),
			path: "$.query",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := DecodeRequest(test.payload)
			if err == nil {
				t.Fatal("read decoder accepted invalid request")
			}
			var decoded *DecodeError
			if !errorAs(err, &decoded) {
				t.Fatalf("error = %T %v", err, err)
			}
			if decoded.Path() != test.path {
				t.Fatalf(
					"error path = %q, want %q: %v",
					decoded.Path(),
					test.path,
					err,
				)
			}
		})
	}
}

func TestMemoryReadRequestZeroValuesAreNotDecoded(t *testing.T) {
	if IsDecodedResolveReadRequest(ResolveReadRequest{}) {
		t.Fatal("zero resolve request passed decoder proof")
	}
	if IsDecodedNeighborhoodReadRequest(NeighborhoodReadRequest{}) {
		t.Fatal("zero neighborhood request passed decoder proof")
	}
	if IsDecodedRecallReadRequest(RecallReadRequest{}) {
		t.Fatal("zero recall request passed decoder proof")
	}
}

func resolveReadPayload(basis string, optionalContext string) []byte {
	return []byte(fmt.Sprintf(`{
  "contract_version":%q,
  "action":"resolve",
  "basis":%s,
  "query":"authentication service"%s,
  "max_candidates":7
}`, ContractVersion, basis, optionalContext))
}

func neighborhoodReadPayload(basis string) []byte {
	return []byte(fmt.Sprintf(`{
  "contract_version":%q,
  "action":"neighborhood",
  "basis":%s,
  "entity_ref":{"ref_kind_id":"U.EntityRef","reference_id":"service:auth"},
  "bounded_context_ref":"context:project",
  "view":{
    "projection_profile_ref":"agent_orientation.v1",
    "requested_facets":["problems","decisions","specifications"],
    "detail":"standard",
    "include_history":false
  },
  "read_budget":%s
}`, ContractVersion, basis, readBudgetJSON))
}

func recallReadPayload(basis string) []byte {
	return []byte(fmt.Sprintf(`{
  "contract_version":%q,
  "action":"recall",
  "basis":%s,
  "entity_ref":{"ref_kind_id":"U.EntityRef","reference_id":"service:auth"},
  "bounded_context_ref":"context:project",
  "view":{
    "projection_profile_ref":"agent_orientation.v1",
    "requested_facets":["problems","decisions","specifications"],
    "detail":"standard",
    "include_history":false
  },
  "read_budget":%s,
  "query":"CAS invariant",
  "candidate_budget":{"max_candidates":8}
}`, ContractVersion, basis, readBudgetJSON))
}

func exactReadBasisJSON(graphRevision uint64) string {
	return fmt.Sprintf(
		`{"kind":"exact_project","type_env_digest":%q,"graph_revision":%d}`,
		testDigest,
		graphRevision,
	)
}

const readBudgetJSON = `{
  "max_facets":5,
  "max_items_per_facet":20,
  "max_relation_paths_per_item":4,
  "max_carrier_excerpt_characters":4096,
  "max_provenance_depth":8
}`

func errorAs(err error, target **DecodeError) bool {
	current, ok := err.(*DecodeError)
	if !ok {
		return false
	}
	*target = current
	return true
}
