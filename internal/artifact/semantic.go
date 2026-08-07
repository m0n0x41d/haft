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

	SemanticEnvelopeSchemaVersion = 1
	PublicationUnitSchemaVersion  = 1

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
	return NewProblemSemanticEnvelopeForProblem(id, now, ProblemFields{}, "")
}

func NewProblemSemanticEnvelopeForProblem(id string, now time.Time, fields ProblemFields, body string) SemanticEnvelope {
	sourceHash := ProblemSourceEditionHash(fields)
	projection := problemPublicationProjection(sourceHash)
	semanticEdition := SemanticEditionRef{
		ID:        fmt.Sprintf("episteme://haft/problem-card/%s/v1", id),
		Family:    string(KindProblemCard),
		Version:   1,
		CreatedAt: now.Format(time.RFC3339),
		Hash:      sourceHash,
	}

	return SemanticEnvelope{
		SchemaVersion: SemanticEnvelopeSchemaVersion,
		Status:        SemanticStatusExact,
		Profile: FPFProfileRef{
			ID:         problemSemanticProfileID,
			SourceKind: "embedded-profile",
			SourceRef:  problemSemanticProfileSource,
			Hash:       problemSemanticProfileHash(),
			ValidUntil: problemSemanticProfileValidTil,
		},
		SemanticEdition: semanticEdition,
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
		PublicationProjection: projection,
		PublicationUnit: problemPublicationUnit(
			semanticEdition,
			projection,
			body,
			nil,
			nil,
			PublicationRecoverability{
				Status:    "exact",
				Mechanism: []string{"sqlite structured_data", "markdown structured_data block"},
			},
		),
	}
}

func LegacyProblemSemanticEnvelope(id string, reason string) SemanticEnvelope {
	return LegacyProblemSemanticEnvelopeForCarrier(id, reason, "")
}

func LegacyProblemSemanticEnvelopeForCarrier(id string, reason string, body string) SemanticEnvelope {
	warnings := []string{"legacy carrier has no exact semantic envelope"}
	if trimmed := strings.TrimSpace(reason); trimmed != "" {
		warnings = append(warnings, trimmed)
	}

	semanticEdition := SemanticEditionRef{
		ID:      fmt.Sprintf("legacy://haft/problem-card/%s", id),
		Family:  string(KindProblemCard),
		Version: 0,
	}
	projection := problemPublicationProjectionForKind("legacy_problem_card_markdown", "")

	return SemanticEnvelope{
		SchemaVersion: SemanticEnvelopeSchemaVersion,
		Status:        SemanticStatusLegacy,
		Profile: FPFProfileRef{
			ID:         problemSemanticProfileID,
			SourceKind: "embedded-profile",
			SourceRef:  problemSemanticProfileSource,
			Hash:       problemSemanticProfileHash(),
			ValidUntil: problemSemanticProfileValidTil,
		},
		SemanticEdition: semanticEdition,
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
		PublicationProjection: projection,
		PublicationUnit: problemPublicationUnit(
			semanticEdition,
			projection,
			body,
			[]string{"structured_data.semantic"},
			[]PublicationLoss{
				{
					Field:       "structured_data.semantic",
					Reason:      "legacy carrier did not bind an exact semantic envelope",
					Recoverable: false,
				},
			},
			PublicationRecoverability{
				Status:    "legacy_degraded",
				Mechanism: []string{"frontmatter", "markdown headings"},
			},
		),
		Warnings: warnings,
	}
}

func ProblemSemanticEnvelopeForArtifact(a *Artifact) SemanticEnvelope {
	if a == nil {
		return LegacyProblemSemanticEnvelope("", "artifact is nil")
	}

	fields := a.UnmarshalProblemFields()
	if fields.Semantic != nil {
		return problemSemanticEnvelopeForFields(a, fields)
	}

	return LegacyProblemSemanticEnvelopeForCarrier(a.Meta.ID, "structured_data.semantic missing", a.Body)
}

func problemSemanticEnvelopeForFields(a *Artifact, fields ProblemFields) SemanticEnvelope {
	if fields.Semantic.SchemaVersion == SemanticEnvelopeSchemaVersion && fields.Semantic.Status == SemanticStatusExact {
		if fields.Semantic.PublicationUnit.SchemaVersion == PublicationUnitSchemaVersion {
			return *fields.Semantic
		}

		return NewProblemSemanticEnvelopeForProblem(a.Meta.ID, problemSemanticEditionTime(a), fields, a.Body)
	}
	if fields.Semantic.Status != SemanticStatusExact && fields.Semantic.PublicationUnit.SchemaVersion == 0 {
		return LegacyProblemSemanticEnvelopeForCarrier(a.Meta.ID, "stored carrier semantic status is not exact", a.Body)
	}

	return *fields.Semantic
}

func NormalizeProblemStructuredDataForImport(a *Artifact) error {
	if a == nil || a.Meta.Kind != KindProblemCard {
		return nil
	}

	fields, err := decodeProblemFieldsForImport(a.StructuredData)
	if err != nil {
		return err
	}
	if strings.TrimSpace(fields.Signal) == "" {
		fields.Signal = ExtractMarkdownSection(a.Body, "Signal")
	}
	if fields.Semantic == nil || fields.Semantic.SchemaVersion == 0 {
		legacy := LegacyProblemSemanticEnvelopeForCarrier(a.Meta.ID, "imported carrier had no exact semantic envelope", a.Body)
		fields.Semantic = &legacy
	} else if fields.Semantic.Status != SemanticStatusExact {
		legacy := LegacyProblemSemanticEnvelopeForCarrier(a.Meta.ID, "imported carrier semantic status is not exact", a.Body)
		fields.Semantic = &legacy
	} else {
		semantic := NewProblemSemanticEnvelopeForProblem(a.Meta.ID, problemSemanticEditionTime(a), fields, a.Body)
		fields.Semantic = &semantic
	}

	data, err := json.Marshal(fields)
	if err != nil {
		return fmt.Errorf("marshal problem structured_data: %w", err)
	}

	a.StructuredData = string(data)
	return nil
}

func decodeProblemFieldsForImport(raw string) (ProblemFields, error) {
	if strings.TrimSpace(raw) == "" {
		return ProblemFields{}, nil
	}

	fields := ProblemFields{}
	if err := json.Unmarshal([]byte(raw), &fields); err != nil {
		return ProblemFields{}, fmt.Errorf("parse problem structured_data: %w", err)
	}
	if err := validateProblemSemanticSchema(fields.Semantic); err != nil {
		return ProblemFields{}, err
	}

	return fields, nil
}

func validateProblemSemanticSchema(semantic *SemanticEnvelope) error {
	if semantic == nil {
		return nil
	}
	if semantic.SchemaVersion > SemanticEnvelopeSchemaVersion {
		return fmt.Errorf("unsupported problem semantic schema_version %d (max %d)", semantic.SchemaVersion, SemanticEnvelopeSchemaVersion)
	}
	if semantic.PublicationUnit.SchemaVersion > PublicationUnitSchemaVersion {
		return fmt.Errorf("unsupported problem publication_unit schema_version %d (max %d)", semantic.PublicationUnit.SchemaVersion, PublicationUnitSchemaVersion)
	}

	return nil
}

func problemSemanticEditionTime(a *Artifact) time.Time {
	if a == nil || a.Meta.UpdatedAt.IsZero() {
		return time.Now().UTC()
	}

	return a.Meta.UpdatedAt.UTC()
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

func ProblemSourceEditionHash(fields ProblemFields) string {
	fields.Semantic = nil

	return hashCanonicalJSON("haft.problem.source_edition.v1", fields)
}

func ProblemCarrierHash(body string) string {
	return hashBytes("haft.problem.carrier_body.v1\n" + body)
}

func problemPublicationProjection(sourceHash string) PublicationProjection {
	return problemPublicationProjectionForKind("problem_card_markdown", sourceHash)
}

func problemPublicationProjectionForKind(kind string, sourceHash string) PublicationProjection {
	projection := PublicationProjection{
		ProjectionKind: kind,
		Views:          []string{"working", "exact", "audit"},
		SyncPolicy:     "explicit_sync_validated_import",
	}
	projection.Hash = hashCanonicalJSON("haft.problem.publication_projection.v1", struct {
		ProjectionKind    string   `json:"projection_kind"`
		SourceEditionHash string   `json:"source_edition_hash,omitempty"`
		Views             []string `json:"views,omitempty"`
		SyncPolicy        string   `json:"sync_policy"`
	}{
		ProjectionKind:    projection.ProjectionKind,
		SourceEditionHash: sourceHash,
		Views:             projection.Views,
		SyncPolicy:        projection.SyncPolicy,
	})

	return projection
}

func problemPublicationUnit(
	edition SemanticEditionRef,
	projection PublicationProjection,
	body string,
	omittedFields []string,
	losses []PublicationLoss,
	recoverability PublicationRecoverability,
) PublicationUnit {
	return PublicationUnit{
		SchemaVersion: PublicationUnitSchemaVersion,
		SourceEditionPin: SourceEditionPin{
			Ref:    edition.ID,
			Hash:   edition.Hash,
			Status: sourceEditionPinStatus(edition),
		},
		PublicationHash: projection.Hash,
		CarrierHash:     ProblemCarrierHash(body),
		OmittedFields:   append([]string(nil), omittedFields...),
		Losses:          append([]PublicationLoss(nil), losses...),
		Recoverability:  recoverability,
	}
}

func sourceEditionPinStatus(edition SemanticEditionRef) string {
	if strings.TrimSpace(edition.Hash) == "" {
		return "legacy_unpinned"
	}

	return "pinned_by_source_hash"
}

func hashCanonicalJSON(domain string, value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return hashBytes(domain + "\n<marshal-error>")
	}

	return hashBytes(domain + "\n" + string(data))
}

func hashBytes(value string) string {
	sum := sha256.Sum256([]byte(value))
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
