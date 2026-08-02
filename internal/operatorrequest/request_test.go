package operatorrequest

import "testing"

func TestRequestCarriesHonestHostRoutingProvenance(t *testing.T) {
	request, err := New(DecisionBinding, "subject:decision", []byte("payload"))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if request.Provenance() != HostRoutedOperatorRequest {
		t.Fatalf("provenance = %q", request.Provenance())
	}
	if request.Effect() != DecisionBinding || request.SubjectRef() != "subject:decision" {
		t.Fatalf("request coordinates = %#v", request)
	}
	if request.PayloadDigest() == "" {
		t.Fatal("payload digest is empty")
	}
	if request.Digest() == "" || request.Ref() != "operator-request:"+request.Digest() {
		t.Fatalf("request identity = %q / %q", request.Ref(), request.Digest())
	}
}

func TestRequestRejectsUnrepresentableCoordinates(t *testing.T) {
	tests := []struct {
		name    string
		effect  Effect
		subject string
		payload []byte
	}{
		{name: "unknown effect", effect: "unknown", subject: "subject:x", payload: []byte("x")},
		{name: "missing subject", effect: DecisionBinding, payload: []byte("x")},
		{name: "missing payload", effect: DecisionBinding, subject: "subject:x"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := New(test.effect, test.subject, test.payload); err == nil {
				t.Fatal("invalid request accepted")
			}
		})
	}
}

func TestRequestRoundTripsDurableCoordinates(t *testing.T) {
	original, err := New(
		ProjectTypeEnvHeadSelect,
		"project-typeenv-head-selection-request:example",
		[]byte("exact payload"),
	)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := FromCoordinates(
		original.Effect(),
		original.SubjectRef(),
		original.PayloadDigest(),
		original.Digest(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if restored != original {
		t.Fatalf("restored request = %#v; want %#v", restored, original)
	}
	if _, err := FromCoordinates(
		original.Effect(),
		"another-subject",
		original.PayloadDigest(),
		original.Digest(),
	); err == nil {
		t.Fatal("mismatched durable request coordinates accepted")
	}
}
