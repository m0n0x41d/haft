package localpractice

import (
	"os"
	"strings"
	"testing"
)

func TestParseKindBridgePreservesExactNamedMappingAndDirection(t *testing.T) {
	t.Parallel()

	parsed, err := Parse(readKindBridgeCarrier(t))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	carrier := parsed.Carrier()
	declarations := carrier.Signature().Vocabulary().Declarations()
	if len(declarations) != 1 {
		t.Fatalf("declarations = %d, want 1", len(declarations))
	}
	bridge, ok := declarations[0].(KindBridgeDeclaration)
	if !ok {
		t.Fatalf("declaration type = %T, want KindBridgeDeclaration", declarations[0])
	}
	assertDeclaration(
		t,
		bridge,
		DeclarationKindBridge,
		"Haft.Bridge.AuthenticatedRequestToFrontendRequest",
		23,
		43,
	)
	if bridge.Source().BoundedContextRef().Value() != "auth-service" ||
		bridge.Source().Edition().Value() != "AuthStandard-v2.3" {
		t.Fatalf("source endpoint = %#v", bridge.Source())
	}
	if bridge.Target().BoundedContextRef().Value() != "frontend" ||
		bridge.Target().Edition().Value() != "FrontendAuth-v1.4" {
		t.Fatalf("target endpoint = %#v", bridge.Target())
	}
	assertRange(t, "source endpoint", bridge.Source().Span(), 27, 28)
	assertRange(t, "target endpoint", bridge.Target().Span(), 30, 31)
	mapping := bridge.Mapping()
	if mapping.Kind() != KindBridgeNamedTarget ||
		mapping.SourceKind().Value() != "Auth.AuthenticatedRequest" ||
		mapping.TargetKind().Value() != "Frontend.VerifiedRequest" {
		t.Fatalf("mapping = %#v", mapping)
	}
	if bridge.Direction().Kind() != KindBridgeOneWay {
		t.Fatalf("direction = %q", bridge.Direction().Kind())
	}
	if bridge.OrderPreservation().Kind() != KindBridgeNoOrderLinksCovered {
		t.Fatalf("order preservation = %q", bridge.OrderPreservation().Kind())
	}
	if bridge.KindCongruence().Value() != 2 {
		t.Fatalf("kind congruence = %d", bridge.KindCongruence().Value())
	}
	if len(bridge.LossNotes()) != 1 || len(bridge.DefinednessArea()) != 2 {
		t.Fatalf(
			"loss/definedness sizes = %d/%d",
			len(bridge.LossNotes()),
			len(bridge.DefinednessArea()),
		)
	}
	if !bridge.AllowsMapping(
		"auth-service",
		"Auth.AuthenticatedRequest",
		"frontend",
		"Frontend.VerifiedRequest",
	) {
		t.Fatal("declared forward mapping was rejected")
	}
	if bridge.AllowsMapping(
		"frontend",
		"Frontend.VerifiedRequest",
		"auth-service",
		"Auth.AuthenticatedRequest",
	) {
		t.Fatal("one-way bridge silently admitted its reverse")
	}
	if bridge.AllowsMapping(
		"auth-service",
		"Auth.AuthenticatedRequest",
		"frontend",
		"Auth.AuthenticatedRequest",
	) {
		t.Fatal("non-identity mapping silently admitted an identity target")
	}
	if carrier.Signature().Applicability().BoundedContextRef().Value() != "frontend" {
		t.Fatal("bridge carrier lost its exact A.6.0 Applicability context")
	}
}

func TestParseKindBridgeTwoWayGrantsOnlyTheExactInverse(t *testing.T) {
	t.Parallel()

	source := string(readKindBridgeCarrier(t))
	source = strings.Replace(source, "direction: one_way", "direction: two_way", 1)
	parsed, err := Parse([]byte(source))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	declaration := parsed.Carrier().Signature().Vocabulary().Declarations()[0]
	bridge := declaration.(KindBridgeDeclaration)
	if !bridge.AllowsMapping(
		"frontend",
		"Frontend.VerifiedRequest",
		"auth-service",
		"Auth.AuthenticatedRequest",
	) {
		t.Fatal("two-way bridge rejected its exact inverse")
	}
	if bridge.AllowsMapping(
		"frontend",
		"Auth.AuthenticatedRequest",
		"auth-service",
		"Frontend.VerifiedRequest",
	) {
		t.Fatal("two-way bridge admitted a correspondence other than its exact inverse")
	}
}

func TestParseKindBridgeFailsClosedOnPartialOrUnknownSemantics(t *testing.T) {
	t.Parallel()

	valid := string(readKindBridgeCarrier(t))
	tests := []struct {
		name        string
		old         string
		replacement string
		message     string
	}{
		{
			name:        "missing target kind",
			old:         "          target_kind: Frontend.VerifiedRequest\n",
			replacement: "",
			message:     "missing required field \"target_kind\"",
		},
		{
			name:        "unknown signature translation",
			old:         "kind: named_target",
			replacement: "kind: signature_translation",
			message:     "requires an exact named_target mapping",
		},
		{
			name:        "undeclared reverse direction",
			old:         "direction: one_way",
			replacement: "direction: reverse",
			message:     "want one_way or two_way",
		},
		{
			name:        "unsupported order claim",
			old:         "order_preservation: no_links_covered",
			replacement: "order_preservation: preserved",
			message:     "can only declare no_links_covered",
		},
		{
			name:        "congruence outside CL ladder",
			old:         "kind_congruence: 2",
			replacement: "kind_congruence: 4",
			message:     "closed CL^k range 0..3",
		},
		{
			name:        "empty loss notes",
			old:         "loss_notes:\n          - The x-auth header representation is not preserved.",
			replacement: "loss_notes: []",
			message:     "must contain at least one item",
		},
		{
			name: "empty definedness area",
			old: "definedness_area:\n" +
				"          - AuthStandard v2.3 and FrontendAuth v1.4 are both active.\n" +
				"          - Target membership is re-evaluated in the frontend ContextSlice.",
			replacement: "definedness_area: []",
			message:     "must contain at least one item",
		},
		{
			name:        "implicit latest source edition",
			old:         "edition: AuthStandard-v2.3",
			replacement: "edition: latest",
			message:     "must pin an exact edition",
		},
		{
			name:        "same endpoint context",
			old:         "bounded_context_ref: auth-service",
			replacement: "bounded_context_ref: frontend",
			message:     "distinct bounded contexts",
		},
		{
			name:        "unknown scope field cannot blend scope bridge",
			old:         "        direction: one_way",
			replacement: "        scope: project-wide\n        direction: one_way",
			message:     "unknown field \"scope\"",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := strings.Replace(valid, test.old, test.replacement, 1)
			if source == valid {
				t.Fatal("test replacement did not mutate fixture")
			}
			_, err := Parse([]byte(source))
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("Parse() error = %v, want substring %q", err, test.message)
			}
		})
	}
}

func TestKindBridgeAccessorsDoNotLeakSlices(t *testing.T) {
	t.Parallel()

	parsed, err := Parse(readKindBridgeCarrier(t))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	bridge := parsed.Carrier().Signature().Vocabulary().Declarations()[0].(KindBridgeDeclaration)
	lossNotes := bridge.LossNotes()
	lossNotes[0] = SourceText{}
	definedness := bridge.DefinednessArea()
	definedness[0] = SourceText{}
	if bridge.LossNotes()[0].Value() == "" || bridge.DefinednessArea()[0].Value() == "" {
		t.Fatal("KindBridgeDeclaration leaked internal source slices")
	}
}

func readKindBridgeCarrier(t *testing.T) []byte {
	t.Helper()
	source, err := os.ReadFile("testdata/valid_kind_bridge.yaml")
	if err != nil {
		t.Fatalf("read KindBridge carrier: %v", err)
	}
	return source
}
