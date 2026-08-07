package artifact

import (
	"strings"
	"testing"
	"time"
)

func TestProblemPublicationUnitSeparatesSourcePublicationAndCarrier(t *testing.T) {
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	fields := ProblemFields{
		Signal:     "Carrier edits must not invent a new semantic edition.",
		Acceptance: "Source hash stays stable while carrier hash changes.",
	}

	first := NewProblemSemanticEnvelopeForProblem("prob-publication", now, fields, "# Title\n\n## Signal\n\nCarrier A\n")
	carrierEdit := NewProblemSemanticEnvelopeForProblem("prob-publication", now, fields, "# Title\n\n## Signal\n\nCarrier B\n")
	semanticEditFields := fields
	semanticEditFields.Signal = "Semantic edits create a new semantic edition."
	semanticEdit := NewProblemSemanticEnvelopeForProblem("prob-publication", now, semanticEditFields, "# Title\n\n## Signal\n\nCarrier B\n")

	if first.SemanticEdition.Hash == "" {
		t.Fatal("source edition hash must be explicit")
	}
	if first.SemanticEdition.Hash != first.PublicationUnit.SourceEditionPin.Hash {
		t.Fatalf("source edition pin hash = %q, want semantic edition hash %q", first.PublicationUnit.SourceEditionPin.Hash, first.SemanticEdition.Hash)
	}
	if first.SemanticEdition.Hash != carrierEdit.SemanticEdition.Hash {
		t.Fatalf("carrier-only edit changed semantic edition: %q != %q", first.SemanticEdition.Hash, carrierEdit.SemanticEdition.Hash)
	}
	if first.PublicationUnit.PublicationHash != carrierEdit.PublicationUnit.PublicationHash {
		t.Fatalf("carrier-only edit changed publication hash: %q != %q", first.PublicationUnit.PublicationHash, carrierEdit.PublicationUnit.PublicationHash)
	}
	if first.PublicationUnit.CarrierHash == carrierEdit.PublicationUnit.CarrierHash {
		t.Fatalf("carrier hash did not change on carrier edit: %q", first.PublicationUnit.CarrierHash)
	}
	if first.SemanticEdition.Hash == semanticEdit.SemanticEdition.Hash {
		t.Fatalf("semantic edit preserved source edition hash: %q", first.SemanticEdition.Hash)
	}
	if first.PublicationUnit.PublicationHash == semanticEdit.PublicationUnit.PublicationHash {
		t.Fatalf("semantic edit preserved publication hash: %q", first.PublicationUnit.PublicationHash)
	}
}

func TestNormalizeProblemStructuredDataRejectsUnknownSemanticSchema(t *testing.T) {
	artifact := &Artifact{
		Meta: Meta{
			ID:        "prob-unknown-schema",
			Kind:      KindProblemCard,
			Version:   1,
			Status:    StatusActive,
			Title:     "Unknown schema",
			CreatedAt: time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC),
		},
		Body: "# Unknown schema\n\n## Signal\n\nDo not import future semantics as exact.\n",
		StructuredData: `{
			"signal": "Do not import future semantics as exact.",
			"semantic": {
				"schema_version": 999,
				"status": "exact"
			}
		}`,
	}

	err := NormalizeProblemStructuredDataForImport(artifact)
	if err == nil {
		t.Fatal("expected unknown semantic schema to fail closed")
	}
	if !strings.Contains(err.Error(), "unsupported problem semantic schema_version 999") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNormalizeProblemStructuredDataRejectsUnknownPublicationUnitSchema(t *testing.T) {
	artifact := &Artifact{
		Meta: Meta{
			ID:        "prob-unknown-publication-unit",
			Kind:      KindProblemCard,
			Version:   1,
			Status:    StatusActive,
			Title:     "Unknown publication unit",
			CreatedAt: time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC),
		},
		Body: "# Unknown publication unit\n\n## Signal\n\nDo not import future publication semantics as exact.\n",
		StructuredData: `{
			"signal": "Do not import future publication semantics as exact.",
			"semantic": {
				"schema_version": 1,
				"status": "exact",
				"publication_unit": {
					"schema_version": 999
				}
			}
		}`,
	}

	err := NormalizeProblemStructuredDataForImport(artifact)
	if err == nil {
		t.Fatal("expected unknown publication_unit schema to fail closed")
	}
	if !strings.Contains(err.Error(), "unsupported problem publication_unit schema_version 999") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNormalizeProblemStructuredDataDoesNotPromoteLegacySemanticStatus(t *testing.T) {
	artifact := &Artifact{
		Meta: Meta{
			ID:        "prob-legacy-status",
			Kind:      KindProblemCard,
			Version:   1,
			Status:    StatusActive,
			Title:     "Legacy status",
			CreatedAt: time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC),
		},
		Body: "# Legacy status\n\n## Signal\n\nLegacy semantic status must stay visible.\n",
		StructuredData: `{
			"signal": "Legacy semantic status must stay visible.",
			"semantic": {
				"schema_version": 1,
				"status": "legacy"
			}
		}`,
	}

	if err := NormalizeProblemStructuredDataForImport(artifact); err != nil {
		t.Fatal(err)
	}

	fields := artifact.UnmarshalProblemFields()
	if fields.Semantic == nil {
		t.Fatal("semantic envelope missing")
	}
	if fields.Semantic.Status != SemanticStatusLegacy {
		t.Fatalf("semantic status = %q, want legacy", fields.Semantic.Status)
	}
	if fields.Semantic.PublicationProjection.ProjectionKind != "legacy_problem_card_markdown" {
		t.Fatalf("projection kind = %q, want legacy_problem_card_markdown", fields.Semantic.PublicationProjection.ProjectionKind)
	}
}

func TestProblemSemanticEnvelopeForArtifactDerivesPublicationUnitForOldExactEnvelope(t *testing.T) {
	artifact := &Artifact{
		Meta: Meta{
			ID:        "prob-old-exact",
			Kind:      KindProblemCard,
			Version:   1,
			Status:    StatusActive,
			Title:     "Old exact",
			CreatedAt: time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC),
		},
		Body: "# Old exact\n\n## Signal\n\nOld exact envelope has no publication unit.\n",
		StructuredData: `{
			"signal": "Old exact envelope has no publication unit.",
			"semantic": {
				"schema_version": 1,
				"status": "exact"
			}
		}`,
	}

	semantic := ProblemSemanticEnvelopeForArtifact(artifact)
	if semantic.Status != SemanticStatusExact {
		t.Fatalf("semantic status = %q, want exact", semantic.Status)
	}
	if semantic.PublicationUnit.SchemaVersion != PublicationUnitSchemaVersion {
		t.Fatalf("publication unit = %+v, want v1", semantic.PublicationUnit)
	}
	if semantic.PublicationUnit.SourceEditionPin.Hash != semantic.SemanticEdition.Hash {
		t.Fatalf("source edition pin = %q, want %q", semantic.PublicationUnit.SourceEditionPin.Hash, semantic.SemanticEdition.Hash)
	}
}
