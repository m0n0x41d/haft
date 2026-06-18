package artifact

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	StructuredDataBlockStart = "<!-- haft:structured_data"
	StructuredDataBlockEnd   = "haft:end -->"

	problemSemanticProfileID       = "haft-semantic-spine-v3.problem-card.v1"
	problemSemanticProfileSource   = "embedded:haft-semantic-spine-v3/problem-card-v1"
	problemSemanticProfileValidTil = "2026-09-18T00:00:00Z"
)

const problemSemanticProfileBody = `profile_id: haft-semantic-spine-v3.problem-card.v1
family: ProblemCard
schema_version: 1
semantic_object: EpistemeEdition
carrier_policy: markdown is a publication carrier; sqlite structured_data is runtime source
views: working, exact, audit
legacy_policy: missing semantic envelope is legacy/degraded, never exact
`

func NewProblemSemanticEnvelope(id string, now time.Time) SemanticEnvelope {
	return SemanticEnvelope{
		SchemaVersion: 1,
		Status:        SemanticStatusExact,
		Profile: FPFProfileRef{
			ID:         problemSemanticProfileID,
			SourceKind: "embedded-profile",
			SourceRef:  problemSemanticProfileSource,
			Hash:       problemSemanticProfileHash(),
			ValidUntil: problemSemanticProfileValidTil,
		},
		SemanticEdition: SemanticEditionRef{
			ID:        fmt.Sprintf("episteme://haft/problem-card/%s/v1", id),
			Family:    string(KindProblemCard),
			Version:   1,
			CreatedAt: now.Format(time.RFC3339),
		},
		ReferenceScheme: ReferenceScheme{
			Primary: "artifact_id",
			Anchors: []string{
				"frontmatter.id",
				"structured_data.semantic.semantic_edition.id",
			},
		},
		CarrierBinding: CarrierBinding{
			CarrierKind:   "markdown",
			CarrierRef:    id,
			StorageKind:   string(KindProblemCard),
			SourceOfTruth: "sqlite",
		},
		PublicationProjection: PublicationProjection{
			ProjectionKind: "problem_card_markdown",
			Views:          []string{"working", "exact", "audit"},
			SyncPolicy:     "explicit_sync_validated_import",
		},
	}
}

func LegacyProblemSemanticEnvelope(id string, reason string) SemanticEnvelope {
	warnings := []string{"legacy carrier has no exact semantic envelope"}
	if trimmed := strings.TrimSpace(reason); trimmed != "" {
		warnings = append(warnings, trimmed)
	}

	return SemanticEnvelope{
		SchemaVersion: 1,
		Status:        SemanticStatusLegacy,
		Profile: FPFProfileRef{
			ID:         problemSemanticProfileID,
			SourceKind: "embedded-profile",
			SourceRef:  problemSemanticProfileSource,
			Hash:       problemSemanticProfileHash(),
			ValidUntil: problemSemanticProfileValidTil,
		},
		SemanticEdition: SemanticEditionRef{
			ID:      fmt.Sprintf("legacy://haft/problem-card/%s", id),
			Family:  string(KindProblemCard),
			Version: 0,
		},
		ReferenceScheme: ReferenceScheme{
			Primary: "artifact_id",
			Anchors: []string{
				"frontmatter.id",
			},
		},
		CarrierBinding: CarrierBinding{
			CarrierKind:   "markdown",
			CarrierRef:    id,
			StorageKind:   string(KindProblemCard),
			SourceOfTruth: "sqlite",
		},
		PublicationProjection: PublicationProjection{
			ProjectionKind: "legacy_problem_card_markdown",
			Views:          []string{"working", "exact", "audit"},
			SyncPolicy:     "explicit_sync_validated_import",
		},
		Warnings: warnings,
	}
}

func ProblemSemanticEnvelopeForArtifact(a *Artifact) SemanticEnvelope {
	if a == nil {
		return LegacyProblemSemanticEnvelope("", "artifact is nil")
	}

	fields := a.UnmarshalProblemFields()
	if fields.Semantic != nil {
		return *fields.Semantic
	}

	return LegacyProblemSemanticEnvelope(a.Meta.ID, "structured_data.semantic missing")
}

func NormalizeProblemStructuredDataForImport(a *Artifact) error {
	if a == nil || a.Meta.Kind != KindProblemCard {
		return nil
	}

	fields := a.UnmarshalProblemFields()
	if strings.TrimSpace(fields.Signal) == "" {
		fields.Signal = ExtractMarkdownSection(a.Body, "Signal")
	}
	if fields.Semantic == nil {
		legacy := LegacyProblemSemanticEnvelope(a.Meta.ID, "imported carrier had no semantic envelope")
		fields.Semantic = &legacy
	}

	data, err := json.Marshal(fields)
	if err != nil {
		return fmt.Errorf("marshal problem structured_data: %w", err)
	}

	a.StructuredData = string(data)
	return nil
}

func ExtractMarkdownSection(body string, heading string) string {
	marker := "## " + heading
	idx := strings.Index(body, marker)
	if idx == -1 {
		return ""
	}

	start := idx + len(marker)
	end := strings.Index(body[start:], "\n## ")
	if end > 0 {
		return strings.TrimSpace(body[start : start+end])
	}

	return strings.TrimSpace(body[start:])
}

func problemSemanticProfileHash() string {
	sum := sha256.Sum256([]byte(problemSemanticProfileBody))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func ExtractStructuredDataBlock(body string) (string, string) {
	start := strings.Index(body, StructuredDataBlockStart)
	if start == -1 {
		return body, ""
	}

	afterStart := body[start+len(StructuredDataBlockStart):]
	afterStart = strings.TrimLeft(afterStart, "\r\n")
	end := strings.Index(afterStart, StructuredDataBlockEnd)
	if end == -1 {
		return body, ""
	}

	structuredData := strings.TrimSpace(afterStart[:end])
	afterEnd := afterStart[end+len(StructuredDataBlockEnd):]

	before := strings.TrimRight(body[:start], "\r\n ")
	tail := strings.TrimLeft(afterEnd, "\r\n ")
	if tail == "" {
		if before == "" {
			return "", structuredData
		}
		return before + "\n", structuredData
	}
	if before == "" {
		return tail, structuredData
	}
	return before + "\n\n" + tail, structuredData
}

func RenderStructuredDataBlock(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	var decoded any
	rendered := raw
	if err := json.Unmarshal([]byte(raw), &decoded); err == nil {
		if data, marshalErr := json.MarshalIndent(decoded, "", "  "); marshalErr == nil {
			rendered = string(data)
		}
	}

	return "\n<!-- haft:structured_data\n" + rendered + "\nhaft:end -->\n"
}
