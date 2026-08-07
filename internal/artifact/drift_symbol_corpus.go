package artifact

import (
	"context"
	"fmt"
	"strings"

	"github.com/m0n0x41d/haft/internal/codebase"
)

type DriftProjectionState string

const (
	DriftProjectionSkipped     DriftProjectionState = "skipped"
	DriftProjectionComplete    DriftProjectionState = "complete"
	DriftProjectionPartial     DriftProjectionState = "partial"
	DriftProjectionUnavailable DriftProjectionState = "unavailable"
)

// DriftProjection reports whether the status response could establish the
// complete current drift basis. Partial and unavailable projections never
// support a known-absence claim.
type DriftProjection struct {
	State            DriftProjectionState `json:"state"`
	SourceIndexEpoch int64                `json:"source_index_epoch,omitempty"`
	BasisDigest      string               `json:"basis_digest,omitempty"`
	Reason           string               `json:"reason,omitempty"`
	OmittedDecisions int                  `json:"omitted_decisions,omitempty"`
}

// DriftSymbolCorpus is a request-local immutable lookup over one symbol-index
// publication. It owns lookup mechanics only; CheckDrift retains all
// materiality and binding-resolution policy.
type DriftSymbolCorpus struct {
	projection DriftProjection
	byName     map[string][]codebase.SymbolSnapshot
	byHash     map[string][]codebase.SymbolSnapshot
}

func NewCompleteDriftSymbolCorpus(
	epoch int64,
	basisDigest string,
	symbols []codebase.SymbolSnapshot,
) *DriftSymbolCorpus {
	return newDriftSymbolCorpus(DriftProjection{
		State:            DriftProjectionComplete,
		SourceIndexEpoch: epoch,
		BasisDigest:      strings.TrimSpace(basisDigest),
	}, symbols)
}

func NewPartialDriftSymbolCorpus(
	epoch int64,
	basisDigest string,
	reason string,
	symbols []codebase.SymbolSnapshot,
) *DriftSymbolCorpus {
	return newDriftSymbolCorpus(DriftProjection{
		State:            DriftProjectionPartial,
		SourceIndexEpoch: epoch,
		BasisDigest:      strings.TrimSpace(basisDigest),
		Reason:           strings.TrimSpace(reason),
	}, symbols)
}

func NewUnavailableDriftSymbolCorpus(reason string) *DriftSymbolCorpus {
	return newDriftSymbolCorpus(DriftProjection{
		State:  DriftProjectionUnavailable,
		Reason: strings.TrimSpace(reason),
	}, nil)
}

func newDriftSymbolCorpus(
	projection DriftProjection,
	symbols []codebase.SymbolSnapshot,
) *DriftSymbolCorpus {
	corpus := &DriftSymbolCorpus{
		projection: projection,
		byName:     make(map[string][]codebase.SymbolSnapshot),
		byHash:     make(map[string][]codebase.SymbolSnapshot),
	}
	for _, snapshot := range symbols {
		path := normalizeProjectPath(snapshot.FilePath)
		if generatedOrIgnoredPath(path) || carrierOnlyPath(path) {
			continue
		}
		snapshot.FilePath = path
		name := strings.TrimSpace(snapshot.SymbolName)
		if name != "" {
			corpus.byName[name] = append(corpus.byName[name], snapshot)
		}
		hash := strings.TrimSpace(snapshot.Hash)
		if hash != "" {
			corpus.byHash[hash] = append(corpus.byHash[hash], snapshot)
		}
	}
	return corpus
}

func (corpus *DriftSymbolCorpus) Projection() DriftProjection {
	if corpus == nil {
		return DriftProjection{State: DriftProjectionSkipped}
	}
	return corpus.projection
}

func (corpus *DriftSymbolCorpus) complete() bool {
	return corpus != nil && corpus.projection.State == DriftProjectionComplete
}

func (corpus *DriftSymbolCorpus) markPartial(reason string, omitted int) {
	if corpus == nil {
		return
	}
	if corpus.projection.State == DriftProjectionComplete {
		corpus.projection.State = DriftProjectionPartial
	}
	if corpus.projection.State == DriftProjectionUnavailable {
		return
	}
	reason = strings.TrimSpace(reason)
	if reason != "" && !strings.Contains(corpus.projection.Reason, reason) {
		if corpus.projection.Reason != "" {
			corpus.projection.Reason += "; "
		}
		corpus.projection.Reason += reason
	}
	corpus.projection.OmittedDecisions += omitted
}

func (corpus *DriftSymbolCorpus) exactMoved(
	oldRelPath string,
	target BindingTarget,
) (codebase.SymbolSnapshot, bool) {
	if corpus == nil {
		return codebase.SymbolSnapshot{}, false
	}
	oldRelPath = normalizeProjectPath(oldRelPath)
	for _, snapshot := range corpus.byHash[strings.TrimSpace(target.BodyHash)] {
		if snapshot.FilePath == oldRelPath {
			continue
		}
		if symbolSnapshotMatchesBindingTarget(snapshot, target) {
			return snapshot, true
		}
	}
	return codebase.SymbolSnapshot{}, false
}

func (corpus *DriftSymbolCorpus) editedMoved(
	oldRelPath string,
	target BindingTarget,
	match func(codebase.SymbolSnapshot, BindingTarget) bool,
) (codebase.SymbolSnapshot, bool) {
	if corpus == nil {
		return codebase.SymbolSnapshot{}, false
	}
	oldRelPath = normalizeProjectPath(oldRelPath)
	candidates := make([]codebase.SymbolSnapshot, 0, 2)
	for _, snapshot := range corpus.byName[strings.TrimSpace(target.SymbolName)] {
		if snapshot.FilePath == oldRelPath || !match(snapshot, target) {
			continue
		}
		if strings.TrimSpace(snapshot.Hash) == strings.TrimSpace(target.BodyHash) {
			continue
		}
		candidates = append(candidates, snapshot)
		if len(candidates) > 1 {
			return codebase.SymbolSnapshot{}, false
		}
	}
	if len(candidates) != 1 {
		return codebase.SymbolSnapshot{}, false
	}
	return candidates[0], true
}

// buildDriftSymbolCorpusFromSource is the compatibility path for direct
// artifact callers that do not own the shared code-index coordinator. It walks
// and parses the corpus once per CheckDrift call, never once per target.
func buildDriftSymbolCorpusFromSource(
	ctx context.Context,
	projectRoot string,
) (*DriftSymbolCorpus, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	files, err := listScopeFiles(projectRoot, ".")
	if err != nil {
		return nil, err
	}
	var snapshots []codebase.SymbolSnapshot
	parseFailures := 0
	for _, path := range files {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		path = normalizeProjectPath(path)
		if generatedOrIgnoredPath(path) || carrierOnlyPath(path) {
			continue
		}
		items, extractErr := codebase.ExtractSymbolSnapshots(projectRoot, path)
		if extractErr != nil {
			parseFailures++
			continue
		}
		snapshots = append(snapshots, items...)
	}
	state := DriftProjectionComplete
	reason := ""
	if parseFailures > 0 {
		state = DriftProjectionPartial
		reason = fmt.Sprintf("%d source file(s) could not be parsed", parseFailures)
	}
	return newDriftSymbolCorpus(DriftProjection{
		State:  state,
		Reason: reason,
	}, snapshots), nil
}
