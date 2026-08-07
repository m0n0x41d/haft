package sqlite

import (
	"errors"
	"math"
	"testing"
)

func TestCommittedPayloadSelectionNeverUsesLatestWins(t *testing.T) {
	tests := []struct {
		name       string
		evidence   committedPayloadEvidence
		wantRef    string
		wantDigest string
		wantError  error
	}{
		{
			name:      "absent",
			evidence:  committedPayloadEvidence{},
			wantError: errNoCommittedAuthorityBasis,
		},
		{
			name: "one historical",
			evidence: committedPayloadEvidence{
				matchCount:  1,
				firstRef:    "profile-admission.one",
				firstDigest: "sha256:one",
			},
			wantRef:    "profile-admission.one",
			wantDigest: "sha256:one",
		},
		{
			name: "explicit current wins",
			evidence: committedPayloadEvidence{
				matchCount:    3,
				currentCount:  1,
				currentRef:    "profile-admission.current",
				currentDigest: "sha256:current",
				firstRef:      "profile-admission.old",
				firstDigest:   "sha256:old",
			},
			wantRef:    "profile-admission.current",
			wantDigest: "sha256:current",
		},
		{
			name: "historical ambiguity",
			evidence: committedPayloadEvidence{
				matchCount:  2,
				firstRef:    "profile-admission.lexicographic-first",
				firstDigest: "sha256:first",
			},
			wantError: errAmbiguousCommittedPayload,
		},
	}
	runCommittedPayloadSelectionCases(t, tests, 0)
}

func TestProfileLedgerRevisionSQLiteValueFailsClosedOutsideIntegerRange(t *testing.T) {
	maximum, err := profileLedgerRevisionSQLiteValue(
		"test revision",
		uint64(math.MaxInt64),
	)
	if err != nil {
		t.Fatalf("maximum SQLite revision: %v", err)
	}
	if maximum != math.MaxInt64 {
		t.Fatalf("maximum SQLite revision = %d, want %d", maximum, int64(math.MaxInt64))
	}

	_, err = profileLedgerRevisionSQLiteValue(
		"test revision",
		uint64(math.MaxInt64)+1,
	)
	if err == nil {
		t.Fatal("revision outside SQLite integer range was accepted")
	}
}

func runCommittedPayloadSelectionCases(
	t *testing.T,
	tests []struct {
		name       string
		evidence   committedPayloadEvidence
		wantRef    string
		wantDigest string
		wantError  error
	},
	index int,
) {
	t.Helper()
	if index >= len(tests) {
		return
	}
	test := tests[index]
	t.Run(test.name, func(t *testing.T) {
		ref, digest, err := selectCommittedPayloadIdentity(test.evidence)
		if !errors.Is(err, test.wantError) {
			t.Fatalf("error = %v, want %v", err, test.wantError)
		}
		if ref != test.wantRef || digest != test.wantDigest {
			t.Fatalf("identity = (%q, %q), want (%q, %q)", ref, digest, test.wantRef, test.wantDigest)
		}
	})
	runCommittedPayloadSelectionCases(t, tests, index+1)
}
