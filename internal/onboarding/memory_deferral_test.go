package onboarding

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestMemoryDeferralCanonicalRoundTripCarriesOnlyNonBindingProvenance(
	t *testing.T,
) {
	recordedAt := time.Date(
		2026,
		time.July,
		28,
		12,
		30,
		0,
		123,
		time.UTC,
	)
	deferral, err := NewMemoryDeferral(
		MemoryDeferralInput{
			ProjectID:    "qnt_a11ce001",
			ReviewRef:    MemoryReviewRef,
			ReviewDigest: "sha256:" + strings.Repeat("a", 64),
			Choice:       DeferStructuredMemoryChoice,
			RecordedAt:   recordedAt,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if deferral.Provenance() != MemoryDeferralProvenance ||
		deferral.InterpretationLimit() !=
			MemoryDeferralInterpretationLimit {
		t.Fatalf(
			"deferral provenance = %q/%q",
			deferral.Provenance(),
			deferral.InterpretationLimit(),
		)
	}
	content, err := deferral.CanonicalJSON()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeMemoryDeferral(content)
	if err != nil {
		t.Fatal(err)
	}
	reencoded, err := decoded.CanonicalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(reencoded, content) {
		t.Fatalf(
			"canonical round trip changed bytes:\n%s\n%s",
			content,
			reencoded,
		)
	}
}

func TestMemoryDeferralRejectsAuthorityLikeOrNonCanonicalCarriers(
	t *testing.T,
) {
	recordedAt := time.Date(
		2026,
		time.July,
		28,
		12,
		30,
		0,
		0,
		time.UTC,
	)
	valid := MemoryDeferralInput{
		ProjectID:    "qnt_a11ce001",
		ReviewRef:    MemoryReviewRef,
		ReviewDigest: "sha256:" + strings.Repeat("b", 64),
		Choice:       DeferStructuredMemoryChoice,
		RecordedAt:   recordedAt,
	}
	mutations := []struct {
		name   string
		mutate func(MemoryDeferralInput) MemoryDeferralInput
	}{
		{
			name: "different review",
			mutate: func(input MemoryDeferralInput) MemoryDeferralInput {
				input.ReviewRef = "review:other"
				return input
			},
		},
		{
			name: "binding choice",
			mutate: func(input MemoryDeferralInput) MemoryDeferralInput {
				input.Choice = EnableStructuredMemoryChoice
				return input
			},
		},
		{
			name: "uppercase digest",
			mutate: func(input MemoryDeferralInput) MemoryDeferralInput {
				input.ReviewDigest =
					"sha256:" + strings.Repeat("A", 64)
				return input
			},
		},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			if _, err := NewMemoryDeferral(
				mutation.mutate(valid),
			); err == nil {
				t.Fatal("invalid deferral was accepted")
			}
		})
	}
	deferral, err := NewMemoryDeferral(valid)
	if err != nil {
		t.Fatal(err)
	}
	content, err := deferral.CanonicalJSON()
	if err != nil {
		t.Fatal(err)
	}
	nonCanonical := bytes.Replace(
		content,
		[]byte(`"non_binding_disposition_only"`),
		[]byte(`"authority_receipt"`),
		1,
	)
	if _, err := DecodeMemoryDeferral(nonCanonical); err == nil {
		t.Fatal("authority-like interpretation was accepted")
	}
	trailing := append(
		append([]byte{}, content...),
		[]byte(`{}`)...,
	)
	if _, err := DecodeMemoryDeferral(trailing); err == nil {
		t.Fatal("trailing material was accepted")
	}
}
