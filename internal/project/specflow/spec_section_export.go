package specflow

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/m0n0x41d/haft/internal/project"
	"gopkg.in/yaml.v3"
)

const SpecSectionPublicationProjectionAuthorityBoundary = "publication_projection_only_not_approval_rebaseline_evidence_gate_claim_truth_or_global_truth"

type SpecSectionEditionPublication struct {
	Markdown              string `json:"markdown"`
	SourceEditionHash     string `json:"source_edition_hash"`
	PublicationHash       string `json:"publication_hash"`
	PublicationProjection string `json:"publication_projection"`
	CarrierPath           string `json:"carrier_path"`
	AuthorityBoundary     string `json:"authority_boundary"`
}

func RenderSpecSectionEditionMarkdown(edition SpecSectionEdition) (SpecSectionEditionPublication, error) {
	normalized, err := normalizeSpecSectionEdition(edition)
	if err != nil {
		return SpecSectionEditionPublication{}, err
	}

	documentKind := specSectionPublicationDocumentKind(normalized.Section)
	if documentKind == "" {
		return SpecSectionEditionPublication{}, fmt.Errorf("spec section %q cannot be published without document_kind or spec", normalized.SectionID)
	}

	body, err := yaml.Marshal(specSectionYAMLProjectionFromSection(normalized.Section))
	if err != nil {
		return SpecSectionEditionPublication{}, fmt.Errorf("marshal spec section publication projection: %w", err)
	}

	markdown := specSectionPublicationMarkdown(normalized.SectionID, string(body))
	sections := project.SpecSectionsFromDocuments([]project.SpecDocumentInput{{
		Path:    specSectionPublicationCarrierPath(normalized, documentKind),
		Kind:    documentKind,
		Content: markdown,
	}})
	if len(sections) != 1 {
		return SpecSectionEditionPublication{}, fmt.Errorf("spec section %q publication parsed as %d sections", normalized.SectionID, len(sections))
	}
	if sections[0].ID != normalized.SectionID {
		return SpecSectionEditionPublication{}, fmt.Errorf("spec section publication id %q does not match source %q", sections[0].ID, normalized.SectionID)
	}

	roundTripHash := HashSection(sections[0])
	if roundTripHash != normalized.SemanticHash {
		return SpecSectionEditionPublication{}, fmt.Errorf("spec section %q publication loses semantic identity: got %s, want %s", normalized.SectionID, roundTripHash, normalized.SemanticHash)
	}

	return SpecSectionEditionPublication{
		Markdown:              markdown,
		SourceEditionHash:     normalized.SemanticHash,
		PublicationHash:       hashSpecSectionPublication(markdown),
		PublicationProjection: "typed_yaml_spec_section",
		CarrierPath:           specSectionPublicationCarrierPath(normalized, documentKind),
		AuthorityBoundary:     SpecSectionPublicationProjectionAuthorityBoundary,
	}, nil
}

type specSectionYAMLProjection struct {
	ID               string                      `yaml:"id"`
	Spec             string                      `yaml:"spec,omitempty"`
	SystemFrame      *specSectionSystemFrameYAML `yaml:"system_frame,omitempty"`
	Kind             string                      `yaml:"kind"`
	Title            string                      `yaml:"title,omitempty"`
	StatementType    string                      `yaml:"statement_type"`
	ClaimLayer       string                      `yaml:"claim_layer"`
	Owner            string                      `yaml:"owner"`
	Status           string                      `yaml:"status"`
	ValidUntil       string                      `yaml:"valid_until,omitempty"`
	Terms            []string                    `yaml:"terms,omitempty"`
	DependsOn        []string                    `yaml:"depends_on,omitempty"`
	TargetRefs       []string                    `yaml:"target_refs,omitempty"`
	EvidenceRequired []specSectionEvidenceYAML   `yaml:"evidence_required,omitempty"`
	Claims           []specSectionClaimYAML      `yaml:"claims,omitempty"`
}

type specSectionSystemFrameYAML struct {
	ID     string `yaml:"id,omitempty"`
	Kind   string `yaml:"kind,omitempty"`
	Source string `yaml:"source,omitempty"`
}

type specSectionEvidenceYAML struct {
	Kind        string `yaml:"kind"`
	Description string `yaml:"description"`
}

type specSectionClaimYAML struct {
	ID                   string   `yaml:"id"`
	Class                string   `yaml:"class"`
	Statement            string   `yaml:"statement,omitempty"`
	Scope                []string `yaml:"scope,omitempty"`
	SupportRefs          []string `yaml:"support_refs,omitempty"`
	EvidenceRefs         []string `yaml:"evidence_refs,omitempty"`
	ValidUntil           string   `yaml:"valid_until,omitempty"`
	GoverningPatternRefs []string `yaml:"governing_pattern_refs,omitempty"`
}

func specSectionYAMLProjectionFromSection(section project.SpecSection) specSectionYAMLProjection {
	return specSectionYAMLProjection{
		ID:               section.ID,
		Spec:             section.Spec,
		SystemFrame:      specSectionSystemFrameYAMLFromSection(section.SystemFrame),
		Kind:             section.Kind,
		Title:            section.Title,
		StatementType:    section.StatementType,
		ClaimLayer:       section.ClaimLayer,
		Owner:            section.Owner,
		Status:           section.Status,
		ValidUntil:       section.ValidUntil,
		Terms:            section.Terms,
		DependsOn:        section.DependsOn,
		TargetRefs:       section.TargetRefs,
		EvidenceRequired: specSectionEvidenceYAMLFromSection(section.EvidenceRequired),
		Claims:           specSectionClaimYAMLFromSection(section.Claims),
	}
}

func specSectionSystemFrameYAMLFromSection(frame project.SystemReferenceFrame) *specSectionSystemFrameYAML {
	if strings.TrimSpace(frame.ID) == "" && strings.TrimSpace(frame.Kind) == "" && strings.TrimSpace(frame.Source) == "" {
		return nil
	}

	return &specSectionSystemFrameYAML{
		ID:     frame.ID,
		Kind:   frame.Kind,
		Source: frame.Source,
	}
}

func specSectionEvidenceYAMLFromSection(requirements []project.SpecEvidenceRequirement) []specSectionEvidenceYAML {
	if len(requirements) == 0 {
		return nil
	}

	projected := make([]specSectionEvidenceYAML, 0, len(requirements))
	for _, requirement := range requirements {
		projected = append(projected, specSectionEvidenceYAML{
			Kind:        requirement.Kind,
			Description: requirement.Description,
		})
	}

	return projected
}

func specSectionClaimYAMLFromSection(claims []project.SpecClaim) []specSectionClaimYAML {
	if len(claims) == 0 {
		return nil
	}

	projected := make([]specSectionClaimYAML, 0, len(claims))
	for _, claim := range claims {
		projected = append(projected, specSectionClaimYAML{
			ID:                   claim.ID,
			Class:                claim.Class,
			Statement:            claim.Statement,
			Scope:                claim.Scope,
			SupportRefs:          claim.SupportRefs,
			EvidenceRefs:         claim.EvidenceRefs,
			ValidUntil:           claim.ValidUntil,
			GoverningPatternRefs: claim.GoverningPatternRefs,
		})
	}

	return projected
}

func specSectionPublicationMarkdown(sectionID string, yamlBody string) string {
	var builder strings.Builder
	builder.WriteString("## ")
	builder.WriteString(sectionID)
	builder.WriteString("\n\n```yaml spec-section\n")
	builder.WriteString(strings.TrimRight(yamlBody, "\n"))
	builder.WriteString("\n```\n")
	return builder.String()
}

func specSectionPublicationDocumentKind(section project.SpecSection) string {
	if strings.TrimSpace(section.DocumentKind) != "" {
		return strings.TrimSpace(section.DocumentKind)
	}
	return strings.TrimSpace(section.Spec)
}

func specSectionPublicationCarrierPath(edition SpecSectionEdition, documentKind string) string {
	if strings.TrimSpace(edition.CarrierPath) != "" {
		return filepath.ToSlash(edition.CarrierPath)
	}
	if strings.TrimSpace(edition.Section.Path) != "" {
		return filepath.ToSlash(edition.Section.Path)
	}
	return filepath.ToSlash(filepath.Join(".haft", "specs", documentKind+".md"))
}

func hashSpecSectionPublication(markdown string) string {
	sum := sha256.Sum256([]byte(markdown))
	return hex.EncodeToString(sum[:])
}
