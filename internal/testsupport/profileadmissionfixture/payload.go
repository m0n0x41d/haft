package profileadmissionfixture

import (
	"testing"

	"github.com/m0n0x41d/haft/internal/projectprofile"
)

func newIntegrationPayload(
	t testing.TB,
	suffix string,
) projectprofile.ProfileDeclarationPayload {
	t.Helper()
	return newIntegrationPayloadWithEntity(
		t,
		suffix,
		projectprofile.NoEntityReference{},
	)
}

func newNonSoftwareIntegrationPayload(
	t testing.TB,
	suffix string,
) projectprofile.ProfileDeclarationPayload {
	t.Helper()
	scopeID := mustValue(t, "non-software-"+suffix, projectprofile.NewScopeID)
	scope, err := projectprofile.NewNonSoftwareRealization(
		scopeID,
		projectprofile.NoEntityReference{},
		projectprofile.UnspecifiedKindOrientation{},
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("NewNonSoftwareRealization: %v", err)
	}
	scopes, err := projectprofile.NewScopeSet(
		[]projectprofile.RealizationScope{scope},
	)
	if err != nil {
		t.Fatalf("NewScopeSet: %v", err)
	}
	payload, err := projectprofile.NewProfileDeclarationPayload(scopes)
	if err != nil {
		t.Fatalf("NewProfileDeclarationPayload: %v", err)
	}
	return payload
}

func newMixedIntegrationPayload(
	t testing.TB,
) projectprofile.ProfileDeclarationPayload {
	t.Helper()
	softwareID := mustValue(t, "software", projectprofile.NewScopeID)
	software, err := projectprofile.NewSoftwareRealization(
		softwareID,
		projectprofile.NoEntityReference{},
	)
	if err != nil {
		t.Fatalf("NewSoftwareRealization: %v", err)
	}
	documentsID := mustValue(t, "documents", projectprofile.NewScopeID)
	documents, err := projectprofile.NewNonSoftwareRealization(
		documentsID,
		projectprofile.NoEntityReference{},
		projectprofile.UnspecifiedKindOrientation{},
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("NewNonSoftwareRealization: %v", err)
	}
	scopes, err := projectprofile.NewScopeSet(
		[]projectprofile.RealizationScope{software, documents},
	)
	if err != nil {
		t.Fatalf("NewScopeSet: %v", err)
	}
	payload, err := projectprofile.NewProfileDeclarationPayload(scopes)
	if err != nil {
		t.Fatalf("NewProfileDeclarationPayload: %v", err)
	}
	return payload
}

func newIntegrationPayloadWithEntity(
	t testing.TB,
	suffix string,
	entityRef projectprofile.EntityReference,
) projectprofile.ProfileDeclarationPayload {
	t.Helper()
	scopeID := mustValue(t, "software-"+suffix, projectprofile.NewScopeID)
	scope, err := projectprofile.NewSoftwareRealization(scopeID, entityRef)
	if err != nil {
		t.Fatalf("NewSoftwareRealization: %v", err)
	}
	scopes, err := projectprofile.NewScopeSet(
		[]projectprofile.RealizationScope{scope},
	)
	if err != nil {
		t.Fatalf("NewScopeSet: %v", err)
	}
	payload, err := projectprofile.NewProfileDeclarationPayload(scopes)
	if err != nil {
		t.Fatalf("NewProfileDeclarationPayload: %v", err)
	}
	return payload
}

func mustValue[T any](
	t testing.TB,
	raw string,
	parse func(string) (T, error),
) T {
	t.Helper()
	value, err := parse(raw)
	if err != nil {
		t.Fatalf("parse %q: %v", raw, err)
	}
	return value
}
