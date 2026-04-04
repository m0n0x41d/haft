package reff

import (
	"testing"
	"time"
)

func TestComputeED_FreshEvidence(t *testing.T) {
	now := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	validUntil := now.Add(24 * time.Hour)

	if got := ComputeED(validUntil, now, 1.0); got != 0 {
		t.Fatalf("ComputeED(fresh) = %v, want 0", got)
	}
}

func TestComputeED_ExpiredEvidence(t *testing.T) {
	now := time.Date(2026, 4, 11, 0, 0, 0, 0, time.UTC)
	validUntil := now.Add(-10 * 24 * time.Hour)

	if got := ComputeED(validUntil, now, 0); got != 10.0 {
		t.Fatalf("ComputeED(expired) = %v, want 10.0", got)
	}
}

func TestAggregateED_SumsItems(t *testing.T) {
	now := time.Date(2026, 4, 11, 0, 0, 0, 0, time.UTC)
	items := []EDItem{
		{ValidUntil: now.Add(-10 * 24 * time.Hour), Now: now, K: 1.0},
		{ValidUntil: now.Add(-5 * 24 * time.Hour), Now: now, K: 0.5},
		{ValidUntil: now.Add(24 * time.Hour), Now: now, K: 1.0},
	}

	if got := AggregateED(items); got != 12.5 {
		t.Fatalf("AggregateED = %v, want 12.5", got)
	}
}

func TestCheckEDBudget(t *testing.T) {
	if alert := CheckEDBudget(10, 30); alert != nil {
		t.Fatalf("expected nil alert within budget, got %+v", alert)
	}

	alert := CheckEDBudget(31, 30)
	if alert == nil {
		t.Fatal("expected alert when debt exceeds budget")
	}
	if alert.Excess != 1 {
		t.Fatalf("excess = %v, want 1", alert.Excess)
	}
}

func TestParseValidUntil_AcceptsDateOnly(t *testing.T) {
	got, ok := ParseValidUntil("2026-04-11")
	if !ok {
		t.Fatal("expected date-only parse to succeed")
	}

	want := time.Date(2026, 4, 11, 23, 59, 59, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("ParseValidUntil(date) = %v, want %v", got, want)
	}
}

func TestScoreEvidence_DateOnlyExpiry(t *testing.T) {
	now := time.Date(2026, 4, 12, 0, 0, 0, 0, time.UTC)
	got := ScoreEvidence("supports", 3, "2026-04-11", now)

	if got != 0.1 {
		t.Fatalf("ScoreEvidence(date-only expiry) = %v, want 0.1", got)
	}
}
