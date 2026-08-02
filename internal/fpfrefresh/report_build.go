package fpfrefresh

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// CompatibilityReportInput joins already-verified snapshot coordinates with
// the read-only databases needed for exact delta analysis.
type CompatibilityReportInput struct {
	ToolRevision                 string
	Predecessor                  IntegrationCoordinates
	Candidate                    IntegrationCoordinates
	PredecessorDatabasePath      string
	CandidateDatabasePath        string
	LatestLocalPracticeCandidate string
	AdditionalDeltas             []Delta
	AdditionalDiagnostics        []Diagnostic
	Timings                      []StageTiming
}

// BuildCompatibilityReport derives the closed refresh state from the
// predeclared report policy. The caller cannot supply an outcome.
func BuildCompatibilityReport(input CompatibilityReportInput) (Report, error) {
	if input.LatestLocalPracticeCandidate == "" {
		return Report{}, fmt.Errorf(
			"complete refresh report requires the latest repo-owned Local-Practice candidate",
		)
	}
	predecessor, candidate, err := ReportSnapshotsFromIntegrationCoordinates(
		input.Predecessor,
		input.Candidate,
	)
	if err != nil {
		return Report{}, err
	}
	analysis, err := AnalyzeSnapshotCompatibility(SnapshotAnalysisInput{
		PredecessorDatabasePath:      input.PredecessorDatabasePath,
		CandidateDatabasePath:        input.CandidateDatabasePath,
		LatestLocalPracticeCandidate: input.LatestLocalPracticeCandidate,
	})
	if err != nil {
		return Report{}, err
	}
	deltas := make([]Delta, 0, len(analysis.Deltas)+len(input.AdditionalDeltas))
	deltas = append(deltas, analysis.Deltas...)
	deltas = append(deltas, input.AdditionalDeltas...)
	diagnostics := make(
		[]Diagnostic,
		0,
		len(analysis.Diagnostics)+len(input.AdditionalDiagnostics),
	)
	diagnostics = append(diagnostics, analysis.Diagnostics...)
	diagnostics = append(diagnostics, input.AdditionalDiagnostics...)
	revision, err := typedmemory.NewSourceRevision(input.ToolRevision)
	if err != nil {
		return Report{}, fmt.Errorf("refresh report tool revision: %w", err)
	}
	return newReport(
		revision,
		predecessor,
		candidate,
		analysis.LocalPracticeCompatibility,
		deltas,
		diagnostics,
		input.Timings,
	)
}
