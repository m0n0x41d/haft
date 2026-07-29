package authority

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAuthorityIdentifiersRejectNonCanonicalWeakStrings(t *testing.T) {
	tests := []struct {
		name  string
		parse func(string) error
		raw   string
	}{
		{
			name: "uppercase token",
			parse: func(raw string) error {
				_, err := NewPresentationID(raw)
				return err
			},
			raw: "Presentation.One",
		},
		{
			name: "trimmed reference",
			parse: func(raw string) error {
				_, err := NewSpeechActRef(raw)
				return err
			},
			raw: " work:speech ",
		},
		{
			name: "control character",
			parse: func(raw string) error {
				_, err := NewPermissionRef(raw)
				return err
			},
			raw: "permission:\ninvalid",
		},
		{
			name: "relative project root",
			parse: func(raw string) error {
				_, err := NewProjectRoot(raw)
				return err
			},
			raw: "relative/project",
		},
		{
			name: "unclean project root",
			parse: func(raw string) error {
				_, err := NewProjectRoot(raw)
				return err
			},
			raw: "/tmp/project/../project",
		},
		{
			name: "uppercase digest",
			parse: func(raw string) error {
				_, err := NewDigest(raw)
				return err
			},
			raw: "sha256:" + strings.Repeat("A", 64),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.parse(test.raw); err == nil {
				t.Fatalf("non-canonical value %q was accepted", test.raw)
			}
		})
	}
}

func TestAuthorizationEnvelopeRequiresEveryExactBinding(t *testing.T) {
	now := time.Now().UTC().Round(0)
	validity := mustWindow(t, now.Add(-time.Hour), now.Add(time.Hour))
	work := mustWindow(t, now.Add(-time.Minute), now.Add(time.Minute))
	basis := mustWindow(t, now.Add(-2*time.Minute), now.Add(2*time.Minute))
	builder := NewAuthorizationEnvelopeBuilder(
		mustParse(t, NewActionKind, "profile.declare.from-onboarding-candidate"),
		mustParse(t, NewProjectRoot, filepath.Join(t.TempDir(), "project")),
	).
		ForProfileAuthor(
			mustParse(t, NewRoleAssignmentRef, "role-assignment:profile-author"),
			mustParse(t, NewDigest, testDigestValue('6')),
		)

	_, err := builder.
		WithClassifier(
			mustParse(t, NewClassifierVersion, "classifier:v1"),
			mustParse(t, NewPolicyVersion, "policy:v1"),
		).
		InSession(mustParse(t, NewSessionRef, "session:onboarding")).
		AllowWorkWithin(work).
		AllowBasisObservationWithin(basis).
		ValidWithin(validity).
		SingleUse(mustParse(t, NewSingleUseKey, "use.one")).
		Build()
	if err == nil {
		t.Fatal("envelope without MethodDescription ref was accepted")
	}

	envelope, err := builder.
		ForMethodDescription(
			mustParse(t, NewMethodDescriptionRef, "method-description:profile-onboarding"),
			mustParse(t, NewDigest, testDigestValue('7')),
		).
		UnderMethodContract(
			mustParse(t, NewMethodContractRef, "method-contract:profile-onboarding:v1"),
			mustParse(t, NewDigest, testDigestValue('8')),
		).
		WithClassifier(
			mustParse(t, NewClassifierVersion, "classifier:v1"),
			mustParse(t, NewPolicyVersion, "policy:v1"),
		).
		InSession(mustParse(t, NewSessionRef, "session:onboarding")).
		AllowWorkWithin(work).
		AllowBasisObservationWithin(basis).
		ValidWithin(validity).
		SingleUse(mustParse(t, NewSingleUseKey, "use.one")).
		Build()
	if err != nil {
		t.Fatalf("complete envelope rejected: %v", err)
	}
	first, err := envelope.Digest()
	if err != nil {
		t.Fatalf("digest complete envelope: %v", err)
	}
	second, err := envelope.Digest()
	if err != nil {
		t.Fatalf("digest complete envelope again: %v", err)
	}
	if first != second {
		t.Fatalf("canonical envelope digest is unstable: %s != %s", first.String(), second.String())
	}

	missingExactBindings := []struct {
		name   string
		mutate func(AuthorizationEnvelope) AuthorizationEnvelope
	}{
		{
			name: "RoleAssignment digest",
			mutate: func(value AuthorizationEnvelope) AuthorizationEnvelope {
				value.profileAuthorDigest = Digest{}
				return value
			},
		},
		{
			name: "MethodDescription digest",
			mutate: func(value AuthorizationEnvelope) AuthorizationEnvelope {
				value.methodDescriptionDigest = Digest{}
				return value
			},
		},
		{
			name: "MethodContract ref",
			mutate: func(value AuthorizationEnvelope) AuthorizationEnvelope {
				value.methodContract = MethodContractRef{}
				return value
			},
		},
		{
			name: "MethodContract digest",
			mutate: func(value AuthorizationEnvelope) AuthorizationEnvelope {
				value.methodContractDigest = Digest{}
				return value
			},
		},
	}
	for _, test := range missingExactBindings {
		t.Run(test.name, func(t *testing.T) {
			mutated := test.mutate(envelope)
			if err := validateAuthorizationEnvelope(mutated); err == nil {
				t.Fatalf("envelope without %s was accepted", test.name)
			}
		})
	}
}

func TestAuthorityWindowsAreCanonicalAndEndExclusive(t *testing.T) {
	from := time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC)
	until := from.Add(time.Hour)
	window := mustWindow(t, from, until)
	if !window.Contains(from) {
		t.Fatal("window does not include its start")
	}
	if window.Contains(until) {
		t.Fatal("window includes its end")
	}
	if _, err := parseAuthorityTime("2026-07-14T10:00:00+00:00"); err == nil {
		t.Fatal("non-canonical +00:00 authority time was accepted instead of Z")
	}
	if _, err := NewTimeWindow(until, from); err == nil {
		t.Fatal("reversed authority window was accepted")
	}
}
