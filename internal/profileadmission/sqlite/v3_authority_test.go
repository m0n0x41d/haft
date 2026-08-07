package sqlite

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/projectprofile"
)

func TestValidateV3WorkInputBindsExactCanonicalPayload(t *testing.T) {
	row := canonicalV3WorkInputTestRow(t, "software")
	if err := validateV3WorkInput(row); err != nil {
		t.Fatalf("validate canonical v3 WorkInput: %v", err)
	}

	tampered := row
	tamperedPayload := canonicalV3WorkInputTestPayload(t, "another-scope")
	tamperedJSON, err := projectprofile.EncodeProfileDeclarationPayloadCanonicalJSON(
		tamperedPayload,
	)
	if err != nil {
		t.Fatal(err)
	}
	tamperedDigest, err := projectprofile.DigestProfileDeclarationPayload(tamperedPayload)
	if err != nil {
		t.Fatal(err)
	}
	tampered.payloadJSON = string(tamperedJSON)
	tampered.payloadDigest = tamperedDigest.String()
	err = validateV3WorkInput(tampered)
	if err == nil || !strings.Contains(err.Error(), "absent from its profile payload") {
		t.Fatalf("tampered v3 WorkInput error = %v", err)
	}
}

func canonicalV3WorkInputTestRow(t testing.TB, scopeID string) v3WorkInputRow {
	t.Helper()
	payload := canonicalV3WorkInputTestPayload(t, scopeID)
	payloadJSON, err := projectprofile.EncodeProfileDeclarationPayloadCanonicalJSON(payload)
	if err != nil {
		t.Fatal(err)
	}
	payloadDigest, err := projectprofile.DigestProfileDeclarationPayload(payload)
	if err != nil {
		t.Fatal(err)
	}
	observationDigest := v3TestDigest("observation")
	dto := v3WorkInputJSON{
		Schema:            v3WorkInputSchema,
		ProjectRoot:       "/tmp/haft-v3-work-input-test",
		SuggestionRef:     "profile-suggestion:test",
		DetectorVersion:   "profile-detector/v1",
		PolicyVersion:     "profile-policy/v1",
		ObservationDigest: observationDigest,
		Scopes: []v3WorkInputScopeJSON{{
			ComponentCandidateRef: "component-candidate:software",
			ScopeID:               scopeID,
			RealizationKind:       "software",
		}},
	}
	canonical, err := json.Marshal(dto)
	if err != nil {
		t.Fatal(err)
	}
	digest := canonicalV3Digest(v3WorkInputSchema, canonical)
	return v3WorkInputRow{
		ref:               "profile-onboarding-work-input:" + strings.TrimPrefix(digest, "sha256:"),
		digest:            digest,
		projectRoot:       dto.ProjectRoot,
		suggestionRef:     dto.SuggestionRef,
		detectorVersion:   dto.DetectorVersion,
		policyVersion:     dto.PolicyVersion,
		observationDigest: observationDigest,
		payloadJSON:       string(payloadJSON),
		payloadDigest:     payloadDigest.String(),
		canonicalJSON:     string(canonical),
		recordedAt:        "2026-07-19T08:00:00Z",
	}
}

func canonicalV3WorkInputTestPayload(
	t testing.TB,
	scopeID string,
) projectprofile.ProfileDeclarationPayload {
	t.Helper()
	id, err := projectprofile.NewScopeID(scopeID)
	if err != nil {
		t.Fatal(err)
	}
	scope, err := projectprofile.NewSoftwareRealization(
		id,
		projectprofile.NoEntityReference{},
	)
	if err != nil {
		t.Fatal(err)
	}
	scopes, err := projectprofile.NewScopeSet([]projectprofile.RealizationScope{scope})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := projectprofile.NewProfileDeclarationPayload(scopes)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func v3TestDigest(seed string) string {
	digest := sha256.Sum256([]byte(seed))
	return "sha256:" + hex.EncodeToString(digest[:])
}
