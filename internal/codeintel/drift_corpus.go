package codeintel

import (
	"context"
	"fmt"

	"github.com/m0n0x41d/haft/internal/codebase"
)

// DriftSymbolCorpus is one immutable, transactionally observed publication of
// the derived code index. Drift consumers use it only as a lookup surface; it
// carries no decision, baseline, or binding authority.
type DriftSymbolCorpus struct {
	Index   codebase.IndexState
	Symbols []codebase.SymbolSnapshot
}

// DriftSymbolCorpusResult preserves the exact freshness attempt separately
// from corpus availability. A retained or absent epoch is useful diagnostic
// information but cannot support known-absence or moved-symbol claims.
type DriftSymbolCorpusResult struct {
	Refresh   IndexCoordinationResult
	Corpus    DriftSymbolCorpus
	Available bool
	Current   bool
	Reason    string
}

// CurrentDriftSymbolCorpus establishes freshness once, then reads every symbol
// under the coordinator's publication read lock. It replaces per-target source
// parsing in drift detection without moving drift classification policy into
// the code-index adapter.
func (service *Service) CurrentDriftSymbolCorpus(
	ctx context.Context,
	projectRoot string,
) (DriftSymbolCorpusResult, error) {
	if ctx == nil {
		return DriftSymbolCorpusResult{}, fmt.Errorf(
			"drift symbol corpus context is required",
		)
	}
	refresh, err := service.EnsureIndex(ctx, projectRoot)
	if err != nil {
		return DriftSymbolCorpusResult{}, err
	}
	result := DriftSymbolCorpusResult{Refresh: refresh}
	switch refresh.Outcome {
	case IndexAlreadyFresh, IndexRebuiltPublished, IndexFreshAfterWait:
		// These outcomes establish a current publication for this request.
	default:
		result.Reason = refresh.Reason
		if result.Reason == "" {
			result.Reason = "fresh complete code-index publication unavailable"
		}
		return result, nil
	}

	release, err := service.acquireIndexRead(projectRoot)
	if err != nil {
		return DriftSymbolCorpusResult{}, err
	}
	defer release()

	before, err := service.scanner.CurrentIndexState(ctx)
	if err != nil {
		return DriftSymbolCorpusResult{}, err
	}
	if !before.SupportsKnownAbsence() {
		result.Reason = before.DegradedReason
		if result.Reason == "" {
			result.Reason = "published code-index basis is incomplete"
		}
		return result, nil
	}
	rows, err := service.symbols.AllSymbols(ctx)
	if err != nil {
		return DriftSymbolCorpusResult{}, err
	}
	after, err := service.scanner.CurrentIndexState(ctx)
	if err != nil {
		return DriftSymbolCorpusResult{}, err
	}
	if !before.SameCurrentBasis(after) {
		result.Reason = "code-index publication changed while reading drift corpus"
		return result, nil
	}

	snapshots := make([]codebase.SymbolSnapshot, 0, len(rows))
	for _, row := range rows {
		if err := ctx.Err(); err != nil {
			return DriftSymbolCorpusResult{}, err
		}
		snapshots = append(snapshots, codebase.SymbolSnapshot{
			FilePath:      row.FilePath,
			SymbolName:    row.Name,
			SymbolKind:    row.Kind,
			QualifiedName: row.QualifiedName,
			SignatureHash: row.SignatureHash,
			Line:          row.StartLine,
			EndLine:       row.EndLine,
			Hash:          row.Hash,
			StartByte:     row.StartByte,
			EndByte:       row.EndByte,
			Receiver:      row.Receiver,
			Exported:      row.Exported,
		})
	}
	result.Corpus = DriftSymbolCorpus{Index: before, Symbols: snapshots}
	result.Available = true
	return result, nil
}

// PublishedDriftSymbolCorpus reads the last atomically published index without
// becoming a rebuild owner or waiting on the process rebuild lock. This is the
// status-safe path: startup owns refresh, while status returns a bounded
// partial projection until that refresh has established currentness.
func (service *Service) PublishedDriftSymbolCorpus(
	ctx context.Context,
	projectRoot string,
) (DriftSymbolCorpusResult, error) {
	if ctx == nil {
		return DriftSymbolCorpusResult{}, fmt.Errorf(
			"drift symbol corpus context is required",
		)
	}
	coordinator, err := service.indexCoordinator(projectRoot)
	if err != nil {
		return DriftSymbolCorpusResult{}, err
	}
	result := DriftSymbolCorpusResult{}
	if latest, ok := coordinator.latestResult(); ok {
		result.Refresh = latest
	}

	before, err := service.scanner.CurrentIndexState(ctx)
	if err != nil {
		result.Reason = err.Error()
		return result, nil
	}
	if !before.SupportsKnownAbsence() {
		result.Reason = before.DegradedReason
		if result.Reason == "" {
			result.Reason = "no complete published code-index epoch"
		}
		return result, nil
	}
	rows, err := service.symbols.AllSymbols(ctx)
	if err != nil {
		return DriftSymbolCorpusResult{}, err
	}
	after, err := service.scanner.CurrentIndexState(ctx)
	if err != nil {
		return DriftSymbolCorpusResult{}, err
	}
	if !before.SameCurrentBasis(after) {
		result.Reason = "code-index publication changed while reading drift corpus"
		return result, nil
	}

	snapshots := make([]codebase.SymbolSnapshot, 0, len(rows))
	for _, row := range rows {
		if err := ctx.Err(); err != nil {
			return DriftSymbolCorpusResult{}, err
		}
		snapshots = append(snapshots, codebase.SymbolSnapshot{
			FilePath:      row.FilePath,
			SymbolName:    row.Name,
			SymbolKind:    row.Kind,
			QualifiedName: row.QualifiedName,
			SignatureHash: row.SignatureHash,
			Line:          row.StartLine,
			EndLine:       row.EndLine,
			Hash:          row.Hash,
			StartByte:     row.StartByte,
			EndByte:       row.EndByte,
			Receiver:      row.Receiver,
			Exported:      row.Exported,
		})
	}
	result.Corpus = DriftSymbolCorpus{Index: before, Symbols: snapshots}
	result.Available = true
	result.Current = indexResultEstablishesCurrentPublication(
		result.Refresh,
		before.Epoch,
	)
	if !result.Current {
		result.Reason = "published code-index epoch is complete but source currentness has not been established in this server process"
	}
	return result, nil
}

func indexResultEstablishesCurrentPublication(
	result IndexCoordinationResult,
	epoch int64,
) bool {
	if result.PublishedEpoch != epoch || result.SourceFingerprint == "" {
		return false
	}
	switch result.Outcome {
	case IndexAlreadyFresh, IndexRebuiltPublished, IndexFreshAfterWait:
		return true
	default:
		return false
	}
}
