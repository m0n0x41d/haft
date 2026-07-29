package typedmemorywire

import (
	"strings"
	"testing"
)

func TestDecodeQueryReadRequestReusesSealedReadUnion(t *testing.T) {
	tests := []struct {
		mode    string
		payload []byte
		assert  func(*testing.T, Request)
	}{
		{
			mode:    ActionResolve,
			payload: resolveReadPayload(`{"kind":"project_current"}`, ""),
			assert: func(t *testing.T, request Request) {
				t.Helper()
				exact, ok := request.(ResolveReadRequest)
				if !ok || !IsDecodedResolveReadRequest(exact) {
					t.Fatalf("resolve query decoded as %T", request)
				}
			},
		},
		{
			mode: ActionNeighborhood,
			payload: neighborhoodReadPayload(
				`{"kind":"project_current"}`,
			),
			assert: func(t *testing.T, request Request) {
				t.Helper()
				exact, ok := request.(NeighborhoodReadRequest)
				if !ok || !IsDecodedNeighborhoodReadRequest(exact) {
					t.Fatalf("neighborhood query decoded as %T", request)
				}
			},
		},
		{
			mode:    ActionRecall,
			payload: recallReadPayload(`{"kind":"project_current"}`),
			assert: func(t *testing.T, request Request) {
				t.Helper()
				exact, ok := request.(RecallReadRequest)
				if !ok || !IsDecodedRecallReadRequest(exact) {
					t.Fatalf("recall query decoded as %T", request)
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.mode, func(t *testing.T) {
			payload := queryReadPayload(test.payload, test.mode)
			request, err := DecodeQueryReadRequest(payload)
			if err != nil {
				t.Fatal(err)
			}
			if request.Action() != test.mode {
				t.Fatalf(
					"translated action = %q, want %q",
					request.Action(),
					test.mode,
				)
			}
			test.assert(t, request)
		})
	}
}

func TestDecodeQueryReadRequestPreservesStrictRawBoundary(t *testing.T) {
	base := queryReadPayload(
		resolveReadPayload(`{"kind":"project_current"}`, ""),
		ActionResolve,
	)
	tests := []struct {
		name    string
		payload []byte
	}{
		{
			name: "duplicate action",
			payload: []byte(strings.Replace(
				string(base),
				`"action":"memory"`,
				`"action":"memory","action":"memory"`,
				1,
			)),
		},
		{
			name: "duplicate mode",
			payload: []byte(strings.Replace(
				string(base),
				`"mode":"resolve"`,
				`"mode":"resolve","mode":"resolve"`,
				1,
			)),
		},
		{
			name: "duplicate memory request",
			payload: []byte(strings.Replace(
				string(base),
				`"memory_request":`,
				`"memory_request":{},"memory_request":`,
				1,
			)),
		},
		{
			name: "unknown outer field",
			payload: []byte(strings.Replace(
				string(base),
				`"memory_request":`,
				`"unexpected":true,"memory_request":`,
				1,
			)),
		},
		{
			name: "cross mode field",
			payload: []byte(strings.Replace(
				string(base),
				`"max_candidates":7`,
				`"max_candidates":7,"candidate_budget":{"max_candidates":2}`,
				1,
			)),
		},
		{
			name: "dedicated action bypass",
			payload: []byte(strings.Replace(
				string(base),
				`"action":"memory"`,
				`"action":"resolve"`,
				1,
			)),
		},
		{
			name: "legacy flat public request",
			payload: resolveReadPayload(
				`{"kind":"project_current"}`,
				"",
			),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := DecodeQueryReadRequest(test.payload); err == nil {
				t.Fatal("strict query decoder accepted invalid payload")
			}
		})
	}
}

func queryReadPayload(payload []byte, mode string) []byte {
	dedicatedAction := `"action":"` + mode + `",`
	nested := strings.Replace(
		string(payload),
		dedicatedAction,
		`"mode":"`+mode+`",`,
		1,
	)
	return []byte(
		`{"action":"memory","memory_request":` + nested + `}`,
	)
}
