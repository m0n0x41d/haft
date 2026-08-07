package memberofc32

import (
	"errors"
	"testing"

	"github.com/m0n0x41d/haft/internal/typedmemoryevaluation"
)

func TestRuntimeRequiresExactEnumerationAndDefinedness(t *testing.T) {
	_, err := NewRuntime(
		typedmemoryevaluation.EntitySetEnumerationRegistry{},
		typedmemoryevaluation.CandidateVisibilityRegistry{},
		typedmemoryevaluation.KindDefinednessRegistry{},
	)
	if !errors.Is(err, ErrRuntimeMissing) {
		t.Fatalf("NewRuntime() error = %v, want %v", err, ErrRuntimeMissing)
	}
}
