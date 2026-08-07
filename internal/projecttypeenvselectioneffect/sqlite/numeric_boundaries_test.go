package sqlite

import (
	"math"
	"strings"
	"testing"
)

func TestExactSQLiteIntegerAcceptsSignedMaximum(t *testing.T) {
	t.Parallel()

	got, err := exactSQLiteInteger("graph revision", uint64(math.MaxInt64))
	if err != nil {
		t.Fatalf("exactSQLiteInteger(MaxInt64): %v", err)
	}
	if got != math.MaxInt64 {
		t.Fatalf("exactSQLiteInteger(MaxInt64) = %d, want %d", got, int64(math.MaxInt64))
	}
}

func TestExactSQLiteIntegerRejectsOverflow(t *testing.T) {
	t.Parallel()

	got, err := exactSQLiteInteger("graph revision", uint64(math.MaxInt64)+1)
	if err == nil {
		t.Fatal("exactSQLiteInteger(MaxInt64+1) succeeded")
	}
	if got != 0 {
		t.Fatalf("exactSQLiteInteger(MaxInt64+1) = %d, want zero", got)
	}
	if !strings.Contains(err.Error(), "graph revision exceeds SQLite INTEGER range") {
		t.Fatalf("exactSQLiteInteger(MaxInt64+1) error = %q", err)
	}
}
