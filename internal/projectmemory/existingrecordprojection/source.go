package existingrecordprojection

import (
	"fmt"
	"strings"

	"github.com/m0n0x41d/haft/internal/artifact"
)

// ConcernCoordinates are exact operator-supplied coordinates for one
// existing-record projection. They are never inferred from artifact identity,
// title, context text, or retrieval rank.
type ConcernCoordinates struct {
	RefKindID        string
	ReferenceID      string
	BoundedContextID string
}

// SourceArguments reconstructs only the source-owned task-adapter arguments
// that are already present in the exact persisted carrier edition.
func SourceArguments(
	route Route,
	record *artifact.Artifact,
	concern ConcernCoordinates,
) (map[string]any, error) {
	if err := verifyRouteRecord(route, record); err != nil {
		return nil, err
	}
	factory, present := sourceArgumentFactories[route.Projection()]
	if !present {
		return nil, fmt.Errorf(
			"existing-record projection %s has no source-owned argument factory",
			route.Projection(),
		)
	}
	return factory(record, concern)
}

type sourceArgumentFactory func(
	*artifact.Artifact,
	ConcernCoordinates,
) (map[string]any, error)

var sourceArgumentFactories = map[Projection]sourceArgumentFactory{
	ProjectionNoteAtConcern:                noteSourceArguments,
	ProjectionProblemCardAtConcern:         problemSourceArguments,
	ProjectionSolutionPortfolioAtConcern:   concernOnlySourceArguments,
	ProjectionPortfolioComparisonAtConcern: concernOnlySourceArguments,
	ProjectionDecisionChoiceAtConcern:      decisionSourceArguments,
}

func verifyRouteRecord(
	route Route,
	record *artifact.Artifact,
) error {
	if record == nil {
		return fmt.Errorf(
			"existing-record source %s is absent",
			route.ArtifactRef(),
		)
	}
	if record.Meta.ID != route.ArtifactRef() ||
		record.Meta.Kind != route.ArtifactKind() ||
		record.Meta.Version != route.ArtifactVersion() {
		return fmt.Errorf(
			"existing-record source %s no longer matches planned kind/version",
			route.ArtifactRef(),
		)
	}
	return nil
}

func noteSourceArguments(
	record *artifact.Artifact,
	concern ConcernCoordinates,
) (map[string]any, error) {
	observations := canonicalMarkdownListSection(
		record.Body,
		"Observations",
	)
	rationale := canonicalMarkdownScalarSection(
		record.Body,
		"Rationale",
	)
	evidence := canonicalMarkdownScalarSection(
		record.Body,
		"Source",
	)
	if len(observations) == 0 &&
		rationale == "" &&
		evidence == "" {
		return nil, fmt.Errorf(
			"note %s has no recoverable canonical Observations, Rationale, or Source section",
			record.Meta.ID,
		)
	}
	arguments, err := exactConcernArguments(concern)
	if err != nil {
		return nil, err
	}
	arguments["observations"] = observations
	arguments["rationale"] = rationale
	arguments["evidence"] = evidence
	return arguments, nil
}

func problemSourceArguments(
	record *artifact.Artifact,
	concern ConcernCoordinates,
) (map[string]any, error) {
	fields := record.UnmarshalProblemFields()
	if strings.TrimSpace(fields.Signal) == "" {
		return nil, fmt.Errorf(
			"problem %s has no recoverable structured signal",
			record.Meta.ID,
		)
	}
	arguments, err := exactConcernArguments(concern)
	if err != nil {
		return nil, err
	}
	arguments["problem_type"] = string(fields.ProblemType)
	arguments["signal"] = fields.Signal
	arguments["constraints"] = append([]string(nil), fields.Constraints...)
	arguments["optimization_targets"] =
		append([]string(nil), fields.OptimizationTargets...)
	arguments["observation_indicators"] =
		append([]string(nil), fields.ObservationIndicators...)
	arguments["acceptance"] = fields.Acceptance
	arguments["blast_radius"] = fields.BlastRadius
	arguments["reversibility"] = fields.Reversibility
	appendProblemProfileArguments(arguments, fields.Profile)
	return arguments, nil
}

func appendProblemProfileArguments(
	arguments map[string]any,
	profile *artifact.ProblemCardProfile,
) {
	if profile == nil {
		return
	}
	arguments["problem_profile"] = profile.Level
	arguments["source_kind"] = profile.SourceKind
	arguments["why_now"] = profile.WhyNow
	arguments["scope"] = profile.Scope
	arguments["acceptance_probe"] = profile.AcceptanceProbe
	arguments["freshness_disposition"] = profile.FreshnessDisposition
}

func concernOnlySourceArguments(
	_ *artifact.Artifact,
	concern ConcernCoordinates,
) (map[string]any, error) {
	return exactConcernArguments(concern)
}

func decisionSourceArguments(
	_ *artifact.Artifact,
	_ ConcernCoordinates,
) (map[string]any, error) {
	return map[string]any{}, nil
}

func exactConcernArguments(
	concern ConcernCoordinates,
) (map[string]any, error) {
	refKindID := strings.TrimSpace(concern.RefKindID)
	referenceID := strings.TrimSpace(concern.ReferenceID)
	boundedContextID := strings.TrimSpace(concern.BoundedContextID)
	if refKindID == "" ||
		referenceID == "" ||
		boundedContextID == "" {
		return nil, fmt.Errorf(
			"exact EntityOfConcern ref_kind_id, reference_id, and bounded_context_ref are required",
		)
	}
	return map[string]any{
		"entity_ref": map[string]any{
			"ref_kind_id":  refKindID,
			"reference_id": referenceID,
		},
		"bounded_context_ref": boundedContextID,
	}, nil
}

func canonicalMarkdownListSection(
	body string,
	heading string,
) []string {
	section := canonicalMarkdownSection(body, heading)
	lines := strings.Split(section, "\n")
	values := make([]string, 0, len(lines))
	for _, line := range lines {
		value := strings.TrimSpace(
			strings.TrimPrefix(strings.TrimSpace(line), "- "),
		)
		if !strings.HasPrefix(strings.TrimSpace(line), "- ") ||
			value == "" {
			continue
		}
		values = append(values, value)
	}
	return values
}

func canonicalMarkdownScalarSection(
	body string,
	heading string,
) string {
	return strings.TrimSpace(
		canonicalMarkdownSection(body, heading),
	)
}

func canonicalMarkdownSection(
	body string,
	heading string,
) string {
	marker := "## " + heading + "\n"
	start := strings.Index(body, marker)
	if start < 0 {
		return ""
	}
	content := body[start+len(marker):]
	content = strings.TrimPrefix(content, "\n")
	next := strings.Index(content, "\n## ")
	if next < 0 {
		return content
	}
	return content[:next]
}
