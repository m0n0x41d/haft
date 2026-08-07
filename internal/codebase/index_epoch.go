package codebase

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

const (
	IndexCoverageComplete              = "complete"
	IndexCoverageBoundedWithExclusions = "bounded_with_exclusions"
	IndexCoverageLegacyUnknown         = "legacy_unknown"
	IndexCoverageUnavailable           = "unavailable"
)

// IndexExclusionSnapshot is the exact persisted/public projection of one known
// source exclusion in an epoch.
type IndexExclusionSnapshot struct {
	Path          string `json:"path"`
	Reason        string `json:"reason"`
	ObservedBytes int64  `json:"observed_bytes"`
	LimitBytes    int64  `json:"limit_bytes"`
	Detail        string `json:"detail"`
}

// IndexCoverageSnapshot is the public immutable projection of one candidate's
// full supported-source corpus.
type IndexCoverageSnapshot struct {
	Posture               string `json:"posture"`
	DiscoveredFiles       int64  `json:"discovered_files"`
	AdmittedFiles         int64  `json:"admitted_files"`
	IndexedFiles          int64  `json:"indexed_files"`
	EmptyFiles            int64  `json:"empty_files"`
	SkippedFiles          int64  `json:"skipped_files"`
	KnownAbsenceSupported bool   `json:"known_absence_supported"`
}

// IndexBasisSnapshot identifies the exact published graph basis used by a
// query. BasisDigest binds the epoch, exact corpus, coverage, and exclusions.
type IndexBasisSnapshot struct {
	Epoch        int64                    `json:"epoch"`
	CorpusDigest string                   `json:"corpus_digest"`
	BasisDigest  string                   `json:"basis_digest"`
	Coverage     IndexCoverageSnapshot    `json:"coverage"`
	Exclusions   []IndexExclusionSnapshot `json:"exclusions,omitempty"`
}

func (b IndexBasisSnapshot) CoverageRef() string {
	if b.Coverage.Posture == IndexCoverageUnavailable {
		return "code-index:v5:unavailable"
	}
	if b.BasisDigest == "" {
		return fmt.Sprintf(
			"code-index:v5:legacy-unaccounted-coverage:published-epoch:%d",
			b.Epoch,
		)
	}
	return "code-index:v5:basis:sha256:" + b.BasisDigest
}

func (b IndexBasisSnapshot) SupportsKnownAbsence() bool {
	return b.Coverage.KnownAbsenceSupported &&
		b.Coverage.Posture == IndexCoverageComplete &&
		b.Epoch > 0 &&
		b.CorpusDigest != "" &&
		b.BasisDigest != ""
}

// IndexEpochCandidate is a valid, unpublished epoch identity. It contains no
// database handles or mutable source maps; publication is an outer-shell
// transaction over this pure result and the already-built graph batches.
type IndexEpochCandidate struct {
	basis IndexBasisSnapshot
}

func (c IndexEpochCandidate) Basis() IndexBasisSnapshot {
	return cloneIndexBasis(c.basis)
}

func BuildIndexEpochCandidate(
	epoch int64,
	states map[string]CodeFileState,
	admissions map[string]AdmittedSource,
	dispositions map[string]FileIndexDisposition,
	exclusions map[string]SourceSkipInfo,
	budget IndexBudget,
) (IndexEpochCandidate, error) {
	if epoch < 1 || !budget.valid() {
		return IndexEpochCandidate{}, fmt.Errorf(
			"candidate epoch identity or budget is invalid",
		)
	}
	if len(states) != len(dispositions) {
		return IndexEpochCandidate{}, fmt.Errorf(
			"candidate corpus has %d states but %d dispositions",
			len(states),
			len(dispositions),
		)
	}
	paths := make([]string, 0, len(states))
	for path := range states {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	coverage := IndexCoverageSnapshot{
		Posture:         IndexCoverageComplete,
		DiscoveredFiles: int64(len(paths)),
		AdmittedFiles:   int64(len(admissions)),
	}
	exclusionSnapshots := make([]IndexExclusionSnapshot, 0, len(exclusions))
	corpusLines := []string{indexBudgetIdentityLine(budget)}
	for _, path := range paths {
		state := states[path]
		disposition, found := dispositions[path]
		if !found || disposition == nil {
			return IndexEpochCandidate{}, fmt.Errorf(
				"candidate file %s has no disposition",
				path,
			)
		}
		if state.FilePath != path ||
			state.ContentHash == "" ||
			state.Language == "" {
			return IndexEpochCandidate{}, fmt.Errorf(
				"candidate file %s has invalid state identity",
				path,
			)
		}
		admittedSource, admitted := admissions[path]
		switch disposition.Kind().String() {
		case CodeFileIndexed:
			if !admitted {
				return IndexEpochCandidate{}, fmt.Errorf(
					"indexed file %s lacks admitted source",
					path,
				)
			}
			if state.ContentHash != admittedSource.Digest() ||
				state.Language != admittedSource.Language().String() {
				return IndexEpochCandidate{}, fmt.Errorf(
					"indexed file %s state does not match admitted source",
					path,
				)
			}
			coverage.IndexedFiles++
		case CodeFileEmpty:
			if !admitted {
				return IndexEpochCandidate{}, fmt.Errorf(
					"empty file %s lacks admitted source",
					path,
				)
			}
			if state.ContentHash != admittedSource.Digest() ||
				state.Language != admittedSource.Language().String() {
				return IndexEpochCandidate{}, fmt.Errorf(
					"empty file %s state does not match admitted source",
					path,
				)
			}
			coverage.EmptyFiles++
		case CodeFileSkipped:
			if admitted {
				return IndexEpochCandidate{}, fmt.Errorf(
					"skipped file %s also has admitted source",
					path,
				)
			}
			info, found := exclusions[path]
			if !found || info.Reason != disposition.DetailCode() {
				return IndexEpochCandidate{}, fmt.Errorf(
					"skipped file %s lacks matching exclusion",
					path,
				)
			}
			if state.ContentHash != skippedSourceStateHash(info) {
				return IndexEpochCandidate{}, fmt.Errorf(
					"skipped file %s state does not match its exclusion",
					path,
				)
			}
			coverage.SkippedFiles++
			exclusionSnapshots = append(
				exclusionSnapshots,
				indexExclusionSnapshot(info),
			)
		case CodeFileDegraded:
			return IndexEpochCandidate{}, fmt.Errorf(
				"degraded file %s cannot enter a published epoch",
				path,
			)
		default:
			return IndexEpochCandidate{}, fmt.Errorf(
				"candidate file %s has unknown disposition",
				path,
			)
		}
		corpusLines = append(
			corpusLines,
			strings.Join(
				[]string{
					path,
					state.ContentHash,
					state.Language,
					disposition.StatusCode(),
					disposition.DetailCode(),
				},
				"\x00",
			),
		)
	}
	if coverage.IndexedFiles+
		coverage.EmptyFiles+
		coverage.SkippedFiles != coverage.DiscoveredFiles {
		return IndexEpochCandidate{}, fmt.Errorf(
			"candidate coverage does not partition the corpus",
		)
	}
	if int64(len(exclusionSnapshots)) != coverage.SkippedFiles {
		return IndexEpochCandidate{}, fmt.Errorf(
			"candidate exclusion ledger does not cover skipped files",
		)
	}
	if coverage.SkippedFiles > 0 {
		coverage.Posture = IndexCoverageBoundedWithExclusions
	}
	coverage.KnownAbsenceSupported =
		coverage.Posture == IndexCoverageComplete
	corpusDigest := digestIndexLines(corpusLines)
	basisLines := []string{
		fmt.Sprintf("epoch\x00%d", epoch),
		"corpus\x00" + corpusDigest,
		fmt.Sprintf(
			"coverage\x00%s\x00%d\x00%d\x00%d\x00%d\x00%d",
			coverage.Posture,
			coverage.DiscoveredFiles,
			coverage.AdmittedFiles,
			coverage.IndexedFiles,
			coverage.EmptyFiles,
			coverage.SkippedFiles,
		),
	}
	for _, exclusion := range exclusionSnapshots {
		basisLines = append(
			basisLines,
			strings.Join(
				[]string{
					exclusion.Path,
					exclusion.Reason,
					fmt.Sprintf("%d", exclusion.ObservedBytes),
					fmt.Sprintf("%d", exclusion.LimitBytes),
					exclusion.Detail,
				},
				"\x00",
			),
		)
	}
	basis := IndexBasisSnapshot{
		Epoch:        epoch,
		CorpusDigest: corpusDigest,
		BasisDigest:  digestIndexLines(basisLines),
		Coverage:     coverage,
		Exclusions:   exclusionSnapshots,
	}
	return IndexEpochCandidate{basis: basis}, nil
}

func indexBudgetIdentityLine(budget IndexBudget) string {
	return fmt.Sprintf(
		"budget\x00%d\x00%d\x00%d\x00%d\x00%s",
		budget.MaxFileBytes().Value(),
		budget.MaxFiles().Value(),
		budget.MaxObservedBytes().Value(),
		budget.MaxParseWorkers().Value(),
		budget.GeneratedSources().String(),
	)
}

func indexExclusionSnapshot(info SourceSkipInfo) IndexExclusionSnapshot {
	return IndexExclusionSnapshot(info)
}

func digestIndexLines(lines []string) string {
	content := strings.Join(lines, "\n")
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func cloneIndexBasis(
	source IndexBasisSnapshot,
) IndexBasisSnapshot {
	clone := source
	clone.Exclusions = append(
		[]IndexExclusionSnapshot(nil),
		source.Exclusions...,
	)
	return clone
}
