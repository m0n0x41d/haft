package cli

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/codeintel"
)

type blockingStatusCorpusSource struct{}

func (blockingStatusCorpusSource) PublishedDriftSymbolCorpus(
	ctx context.Context,
	_ string,
) (codeintel.DriftSymbolCorpusResult, error) {
	<-ctx.Done()
	return codeintel.DriftSymbolCorpusResult{}, ctx.Err()
}

func TestFetchBoundedProjectStatusDataReturnsUnavailableOnCancellation(t *testing.T) {
	store := setupCLIArtifactStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	started := time.Now()
	data, err := fetchBoundedProjectStatusData(
		ctx,
		store,
		blockingStatusCorpusSource{},
		"",
		t.TempDir(),
	)
	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("fetch status: %v", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("cancelled status took %s", elapsed)
	}
	if data.DriftProjection.State != artifact.DriftProjectionUnavailable {
		t.Fatalf("drift projection = %+v", data.DriftProjection)
	}
}

func TestStatusDeadlineFitsObservedMCPHostLimit(t *testing.T) {
	if statusRequestDeadline >= 60*time.Second {
		t.Fatalf("status deadline %s must fit within 60s host limit", statusRequestDeadline)
	}
	if statusCorpusDeadline >= statusRequestDeadline {
		t.Fatalf("corpus deadline %s must reserve response time inside %s", statusCorpusDeadline, statusRequestDeadline)
	}
}
