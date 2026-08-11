package artifact

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/reff"
)

const (
	EvidenceCarrierSchemaVersion = "haft.evidence-record/v1"
	EvidenceForLinkType          = "evidence_for"
)

// EvidenceCarrier is the lossless, git-facing representation of one stored
// EvidenceRecord. The carrier is not the EvidenceRecord, its supporting
// basis, the Evidence relation, or the parent artifact.
type EvidenceCarrier struct {
	SchemaVersion string       `json:"schema_version"`
	ArtifactRef   string       `json:"artifact_ref"`
	Evidence      EvidenceItem `json:"-"`
}

type evidenceCarrierEnvelope struct {
	SchemaVersion string                `json:"schema_version"`
	ArtifactRef   string                `json:"artifact_ref"`
	Evidence      evidenceCarrierRecord `json:"evidence"`
}

type evidenceCarrierRecord struct {
	ID                 string                     `json:"id"`
	Type               string                     `json:"type"`
	Content            string                     `json:"content"`
	Verdict            string                     `json:"verdict,omitempty"`
	CarrierRef         string                     `json:"carrier_ref,omitempty"`
	CongruenceLevel    int                        `json:"congruence_level"`
	FormalityLevel     int                        `json:"formality_level"`
	FormalityScale     *reff.FormalityScale       `json:"formality_scale,omitempty"`
	FormalityBridge    *reff.FormalityBridge      `json:"formality_bridge,omitempty"`
	ClaimRefs          []string                   `json:"claim_refs,omitempty"`
	ClaimScope         []string                   `json:"claim_scope,omitempty"`
	ValidUntil         string                     `json:"valid_until,omitempty"`
	CausalSupportBasis CausalEvidenceSupportBasis `json:"causal_support_basis,omitempty"`
	Provenance         string                     `json:"provenance,omitempty"`
	CreatedAt          string                     `json:"created_at"`
	UpdatedAt          string                     `json:"updated_at"`
}

func evidenceCarrierRecordFromItem(item EvidenceItem) evidenceCarrierRecord {
	return evidenceCarrierRecord{
		ID:                 item.ID,
		Type:               item.Type,
		Content:            item.Content,
		Verdict:            item.Verdict,
		CarrierRef:         item.CarrierRef,
		CongruenceLevel:    item.CongruenceLevel,
		FormalityLevel:     item.FormalityLevel,
		FormalityScale:     item.FormalityScale,
		FormalityBridge:    item.FormalityBridge,
		ClaimRefs:          append([]string(nil), item.ClaimRefs...),
		ClaimScope:         append([]string(nil), item.ClaimScope...),
		ValidUntil:         item.ValidUntil,
		CausalSupportBasis: item.CausalSupportBasis,
		Provenance:         item.Provenance,
		CreatedAt:          item.CreatedAt,
		UpdatedAt:          item.UpdatedAt,
	}
}

func (record evidenceCarrierRecord) item() EvidenceItem {
	return EvidenceItem{
		ID:                 record.ID,
		Type:               record.Type,
		Content:            record.Content,
		Verdict:            record.Verdict,
		CarrierRef:         record.CarrierRef,
		CongruenceLevel:    record.CongruenceLevel,
		FormalityLevel:     record.FormalityLevel,
		FormalityScale:     record.FormalityScale,
		FormalityBridge:    record.FormalityBridge,
		ClaimRefs:          append([]string(nil), record.ClaimRefs...),
		ClaimScope:         append([]string(nil), record.ClaimScope...),
		ValidUntil:         record.ValidUntil,
		CausalSupportBasis: record.CausalSupportBasis,
		Provenance:         record.Provenance,
		CreatedAt:          record.CreatedAt,
		UpdatedAt:          record.UpdatedAt,
	}
}

// NewEvidenceCarrierArtifact constructs the artifact-shaped Markdown carrier.
// KindEvidencePack is a carrier discriminator here; sync special-cases it and
// never creates a second semantic row in artifacts.
func NewEvidenceCarrierArtifact(artifactRef string, item EvidenceItem) (*Artifact, error) {
	artifactRef = strings.TrimSpace(artifactRef)
	if artifactRef == "" {
		return nil, fmt.Errorf("evidence carrier artifact_ref is required")
	}
	if strings.TrimSpace(item.ID) == "" {
		return nil, fmt.Errorf("evidence carrier id is required")
	}
	createdAt, err := time.Parse(time.RFC3339, strings.TrimSpace(item.CreatedAt))
	if err != nil {
		return nil, fmt.Errorf("evidence %s created_at must be RFC3339: %w", item.ID, err)
	}
	updatedAt, err := time.Parse(time.RFC3339, strings.TrimSpace(item.UpdatedAt))
	if err != nil {
		return nil, fmt.Errorf("evidence %s updated_at must be RFC3339: %w", item.ID, err)
	}
	causalBasis, err := ParseCausalSupportBasis(string(item.CausalSupportBasis))
	if err != nil {
		return nil, err
	}
	item.CausalSupportBasis = causalBasis
	item.ClaimRefs = normalizeClaimRefs(item.ClaimRefs)
	item.ClaimScope = normalizeClaimScope(item.ClaimScope)
	item.Provenance = normalizeEvidenceProvenance(item.Provenance)
	formalityScale := storedEvidenceFormalityScale(&item, item.FormalityLevel)
	item.FormalityLevel = formalityScale.Level
	item.FormalityScale = &formalityScale
	if item.FormalityBridge == nil {
		item.FormalityBridge = evidenceFormalityBridge(formalityScale)
	}

	envelope := evidenceCarrierEnvelope{
		SchemaVersion: EvidenceCarrierSchemaVersion,
		ArtifactRef:   artifactRef,
		Evidence:      evidenceCarrierRecordFromItem(item),
	}
	structuredData, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("marshal evidence carrier: %w", err)
	}
	status := StatusActive
	if item.Verdict == string(StatusSuperseded) {
		status = StatusSuperseded
	}
	body := fmt.Sprintf(
		"# Evidence %s\n\n**Parent:** `%s`\n\n**Type:** %s\n\n**Verdict:** %s\n\n## Recorded observation\n\n%s\n",
		item.ID,
		artifactRef,
		item.Type,
		item.Verdict,
		evidenceCarrierBodyContent(item.Content),
	)
	return &Artifact{
		Meta: Meta{
			ID:         item.ID,
			Kind:       KindEvidencePack,
			Version:    1,
			Status:     status,
			Title:      fmt.Sprintf("Evidence for %s", artifactRef),
			ValidUntil: item.ValidUntil,
			CreatedAt:  createdAt.UTC(),
			UpdatedAt:  updatedAt.UTC(),
			Links:      []Link{{Ref: artifactRef, Type: EvidenceForLinkType}},
		},
		Body:           body,
		StructuredData: string(structuredData),
	}, nil
}

func evidenceCarrierBodyContent(content string) string {
	content = strings.ReplaceAll(content, StructuredDataBlockStart, "&lt;!-- haft:structured_data")
	return strings.ReplaceAll(content, StructuredDataBlockEnd, "haft:end --&gt;")
}

// ParseEvidenceCarrier validates an EvidencePack Markdown carrier without
// creating its parent or a generic artifact row.
func ParseEvidenceCarrier(carrierArtifact *Artifact, filePath string) (EvidenceCarrier, error) {
	if carrierArtifact == nil {
		return EvidenceCarrier{}, fmt.Errorf("evidence carrier is required")
	}
	if carrierArtifact.Meta.Kind != KindEvidencePack {
		return EvidenceCarrier{}, fmt.Errorf("%s is %s, not EvidencePack", carrierArtifact.Meta.ID, carrierArtifact.Meta.Kind)
	}
	if carrierArtifact.Meta.Version != 1 {
		return EvidenceCarrier{}, fmt.Errorf("unsupported evidence carrier frontmatter version %d", carrierArtifact.Meta.Version)
	}
	if filePath != "" {
		filenameID := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
		if filenameID != carrierArtifact.Meta.ID {
			return EvidenceCarrier{}, fmt.Errorf("evidence carrier filename id %q does not match frontmatter id %q", filenameID, carrierArtifact.Meta.ID)
		}
	}

	var envelope evidenceCarrierEnvelope
	decoder := json.NewDecoder(strings.NewReader(carrierArtifact.StructuredData))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&envelope); err != nil {
		return EvidenceCarrier{}, fmt.Errorf("decode evidence carrier structured_data: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return EvidenceCarrier{}, fmt.Errorf("decode evidence carrier structured_data: multiple JSON values")
		}
		return EvidenceCarrier{}, fmt.Errorf("decode evidence carrier structured_data: %w", err)
	}
	if envelope.SchemaVersion != EvidenceCarrierSchemaVersion {
		return EvidenceCarrier{}, fmt.Errorf("unsupported evidence carrier schema_version %q", envelope.SchemaVersion)
	}
	if strings.TrimSpace(envelope.ArtifactRef) == "" {
		return EvidenceCarrier{}, fmt.Errorf("evidence carrier artifact_ref is required")
	}
	if envelope.ArtifactRef != strings.TrimSpace(envelope.ArtifactRef) {
		return EvidenceCarrier{}, fmt.Errorf("evidence carrier artifact_ref must not contain surrounding whitespace")
	}
	item := envelope.Evidence.item()
	if item.ID != carrierArtifact.Meta.ID {
		return EvidenceCarrier{}, fmt.Errorf("evidence payload id %q does not match frontmatter id %q", item.ID, carrierArtifact.Meta.ID)
	}
	if strings.TrimSpace(item.Type) == "" {
		return EvidenceCarrier{}, fmt.Errorf("evidence %s type is required", item.ID)
	}
	if item.Content == "" {
		return EvidenceCarrier{}, fmt.Errorf("evidence %s content is required", item.ID)
	}
	if len(carrierArtifact.Meta.Links) != 1 ||
		carrierArtifact.Meta.Links[0].Type != EvidenceForLinkType ||
		carrierArtifact.Meta.Links[0].Ref != envelope.ArtifactRef {
		return EvidenceCarrier{}, fmt.Errorf("evidence carrier parent link must be exactly %s -> %s", EvidenceForLinkType, envelope.ArtifactRef)
	}
	createdAt, err := time.Parse(time.RFC3339, strings.TrimSpace(item.CreatedAt))
	if err != nil {
		return EvidenceCarrier{}, fmt.Errorf("evidence %s created_at must be RFC3339: %w", item.ID, err)
	}
	if !carrierArtifact.Meta.CreatedAt.Equal(createdAt) {
		return EvidenceCarrier{}, fmt.Errorf("evidence %s created_at does not match frontmatter", item.ID)
	}
	updatedAt, err := time.Parse(time.RFC3339, strings.TrimSpace(item.UpdatedAt))
	if err != nil {
		return EvidenceCarrier{}, fmt.Errorf("evidence %s updated_at must be RFC3339: %w", item.ID, err)
	}
	if !carrierArtifact.Meta.UpdatedAt.Equal(updatedAt) {
		return EvidenceCarrier{}, fmt.Errorf("evidence %s updated_at does not match frontmatter", item.ID)
	}
	if carrierArtifact.Meta.ValidUntil != item.ValidUntil {
		return EvidenceCarrier{}, fmt.Errorf("evidence %s valid_until does not match frontmatter", item.ID)
	}
	wantStatus := StatusActive
	if item.Verdict == string(StatusSuperseded) {
		wantStatus = StatusSuperseded
	}
	if carrierArtifact.Meta.Status != wantStatus {
		return EvidenceCarrier{}, fmt.Errorf("evidence %s status %q does not match verdict %q", item.ID, carrierArtifact.Meta.Status, item.Verdict)
	}
	causalBasis, err := ParseCausalSupportBasis(string(item.CausalSupportBasis))
	if err != nil {
		return EvidenceCarrier{}, err
	}
	if causalBasis != item.CausalSupportBasis {
		return EvidenceCarrier{}, fmt.Errorf("evidence %s causal_support_basis is not canonical", item.ID)
	}
	item.CausalSupportBasis = causalBasis
	claimRefs := normalizeClaimRefs(item.ClaimRefs)
	if !slices.Equal(item.ClaimRefs, claimRefs) {
		return EvidenceCarrier{}, fmt.Errorf("evidence %s claim_refs are not canonical", item.ID)
	}
	item.ClaimRefs = claimRefs
	claimScope := normalizeClaimScope(item.ClaimScope)
	if !slices.Equal(item.ClaimScope, claimScope) {
		return EvidenceCarrier{}, fmt.Errorf("evidence %s claim_scope is not canonical", item.ID)
	}
	item.ClaimScope = claimScope
	provenance := normalizeEvidenceProvenance(item.Provenance)
	if provenance != item.Provenance {
		return EvidenceCarrier{}, fmt.Errorf("evidence %s provenance is not canonical", item.ID)
	}
	item.Provenance = provenance
	storedVerdict := canonicalStoredEvidenceVerdict(item.Type, item.Verdict)
	if storedVerdict != item.Verdict {
		return EvidenceCarrier{}, fmt.Errorf("evidence %s verdict is not in stored canonical form", item.ID)
	}
	if item.FormalityScale == nil {
		return EvidenceCarrier{}, fmt.Errorf("evidence %s formality_scale is required", item.ID)
	}
	originalFormalityScale := *item.FormalityScale
	formalityScale := storedEvidenceFormalityScale(&item, item.FormalityLevel)
	if originalFormalityScale != formalityScale || item.FormalityLevel != formalityScale.Level {
		return EvidenceCarrier{}, fmt.Errorf("evidence %s formality scale is not canonical", item.ID)
	}
	item.FormalityLevel = formalityScale.Level
	item.FormalityScale = &formalityScale
	if evidenceFormalityBridge(formalityScale) != nil && item.FormalityBridge == nil {
		return EvidenceCarrier{}, fmt.Errorf("evidence %s formality_bridge is required for scale %s", item.ID, formalityScale.ScaleID)
	}
	if err := validateEvidenceCongruenceAtIngest(item.Verdict, item.CongruenceLevel); err != nil {
		return EvidenceCarrier{}, err
	}
	expectedCarrier, err := NewEvidenceCarrierArtifact(envelope.ArtifactRef, item)
	if err != nil {
		return EvidenceCarrier{}, err
	}
	if carrierArtifact.Meta.Title != expectedCarrier.Meta.Title ||
		carrierArtifact.Meta.Context != expectedCarrier.Meta.Context ||
		carrierArtifact.Meta.Mode != expectedCarrier.Meta.Mode {
		return EvidenceCarrier{}, fmt.Errorf("evidence %s frontmatter presentation does not match its payload", item.ID)
	}
	if carrierArtifact.Body != expectedCarrier.Body {
		return EvidenceCarrier{}, fmt.Errorf("evidence %s visible body does not match its payload", item.ID)
	}

	return EvidenceCarrier{
		SchemaVersion: envelope.SchemaVersion,
		ArtifactRef:   envelope.ArtifactRef,
		Evidence:      item,
	}, nil
}

type evidenceCarrierProjection struct {
	Path   string
	Digest string
}

func renderEvidenceCarrier(haftDir, artifactRef string, item EvidenceItem) (*Artifact, []byte, evidenceCarrierProjection, error) {
	carrierArtifact, err := NewEvidenceCarrierArtifact(artifactRef, item)
	if err != nil {
		return nil, nil, evidenceCarrierProjection{}, err
	}
	content := []byte(RenderArtifactFile(carrierArtifact))
	projection := evidenceCarrierProjection{
		Path:   filepath.Join(haftDir, KindEvidencePack.Dir(), item.ID+".md"),
		Digest: evidenceCarrierContentDigest(content),
	}
	return carrierArtifact, content, projection, nil
}

func evidenceCarrierContentDigest(content []byte) string {
	sum := sha256.Sum256(content)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func writeEvidenceCarrier(haftDir, artifactRef string, item EvidenceItem) (evidenceCarrierProjection, error) {
	_, content, projection, err := renderEvidenceCarrier(haftDir, artifactRef, item)
	if err != nil {
		return projection, err
	}
	dir := filepath.Dir(projection.Path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return projection, fmt.Errorf("create evidence carrier directory %s: %w", dir, err)
	}
	temp, err := os.CreateTemp(dir, ".evidence-carrier-*.tmp")
	if err != nil {
		return projection, fmt.Errorf("create evidence carrier temp file: %w", err)
	}
	tempPath := temp.Name()
	cleanup := func() { _ = os.Remove(tempPath) }
	defer cleanup()
	if err := temp.Chmod(0o644); err != nil {
		_ = temp.Close()
		return projection, fmt.Errorf("chmod evidence carrier temp file: %w", err)
	}
	if _, err := temp.Write(content); err != nil {
		_ = temp.Close()
		return projection, fmt.Errorf("write evidence carrier temp file: %w", err)
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return projection, fmt.Errorf("sync evidence carrier temp file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return projection, fmt.Errorf("close evidence carrier temp file: %w", err)
	}
	if err := os.Rename(tempPath, projection.Path); err != nil {
		return projection, fmt.Errorf("publish evidence carrier %s: %w", projection.Path, err)
	}
	directory, err := os.Open(dir)
	if err != nil {
		return projection, fmt.Errorf("open evidence carrier directory for sync: %w", err)
	}
	if err := directory.Sync(); err != nil {
		_ = directory.Close()
		return projection, fmt.Errorf("sync evidence carrier directory: %w", err)
	}
	if err := directory.Close(); err != nil {
		return projection, fmt.Errorf("close evidence carrier directory: %w", err)
	}
	return projection, nil
}

type evidenceCarrierDebtStore interface {
	RecordEvidenceCarrierProjectionDebt(context.Context, EvidenceCarrierProjectionDebt) error
	ResolveEvidenceCarrierProjectionDebt(context.Context, string) error
}

func projectEvidenceCarrier(ctx context.Context, store ArtifactStore, haftDir, artifactRef string, item EvidenceItem) error {
	projection, err := writeEvidenceCarrier(haftDir, artifactRef, item)
	debtStore, hasDebtStore := store.(evidenceCarrierDebtStore)
	if err == nil {
		if hasDebtStore {
			if resolveErr := debtStore.ResolveEvidenceCarrierProjectionDebt(ctx, item.ID); resolveErr != nil {
				return fmt.Errorf("evidence carrier written but projection debt resolution failed: %w", resolveErr)
			}
		}
		return nil
	}
	if !hasDebtStore {
		return fmt.Errorf("project evidence carrier: %w; durable projection debt store unavailable", err)
	}
	debt := EvidenceCarrierProjectionDebt{
		EvidenceID:    item.ID,
		ArtifactRef:   artifactRef,
		CarrierPath:   projection.Path,
		DesiredDigest: projection.Digest,
		LastError:     err.Error(),
	}
	if debtErr := debtStore.RecordEvidenceCarrierProjectionDebt(ctx, debt); debtErr != nil {
		return fmt.Errorf("project evidence carrier: %w; record projection debt: %v", err, debtErr)
	}
	return fmt.Errorf("project evidence carrier: %w; durable projection debt recorded", err)
}

func projectEvidenceCarriers(ctx context.Context, store ArtifactStore, haftDir, artifactRef string, items []EvidenceItem) []string {
	warnings := make([]string, 0)
	for _, item := range items {
		if err := projectEvidenceCarrier(ctx, store, haftDir, artifactRef, item); err != nil {
			warnings = append(warnings, fmt.Sprintf("Evidence carrier %s: %v", item.ID, err))
		}
	}
	return warnings
}

// RepairEvidenceCarrierProjectionDebt retries the current DB-to-carrier
// projection for every open debt row. It never creates or changes a parent.
func RepairEvidenceCarrierProjectionDebt(ctx context.Context, store *Store, haftDir string) (int, error) {
	debts, err := store.ListEvidenceCarrierProjectionDebt(ctx)
	if err != nil {
		return 0, err
	}
	repaired := 0
	failures := make([]string, 0)
	for _, debt := range debts {
		item, artifactRef, err := store.GetEvidenceItemByID(ctx, debt.EvidenceID)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: load evidence: %v", debt.EvidenceID, err))
			continue
		}
		if artifactRef != debt.ArtifactRef {
			failures = append(failures, fmt.Sprintf("%s: debt parent %s does not match stored parent %s", debt.EvidenceID, debt.ArtifactRef, artifactRef))
			continue
		}
		_, _, projection, err := renderEvidenceCarrier(haftDir, artifactRef, *item)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: render current carrier: %v", debt.EvidenceID, err))
			continue
		}
		existing, readErr := os.ReadFile(projection.Path)
		switch {
		case readErr == nil && evidenceCarrierContentDigest(existing) == projection.Digest:
			if err := store.ResolveEvidenceCarrierProjectionDebt(ctx, debt.EvidenceID); err != nil {
				failures = append(failures, fmt.Sprintf("%s: resolve matching projection debt: %v", debt.EvidenceID, err))
				continue
			}
			repaired++
			continue
		case readErr == nil:
			failures = append(failures, fmt.Sprintf(
				"%s: projection conflict at %s; existing carrier differs from the committed SQLite projection",
				debt.EvidenceID,
				projection.Path,
			))
			continue
		case !os.IsNotExist(readErr):
			failures = append(failures, fmt.Sprintf("%s: inspect existing carrier %s: %v", debt.EvidenceID, projection.Path, readErr))
			continue
		}
		if err := projectEvidenceCarrier(ctx, store, haftDir, artifactRef, *item); err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", debt.EvidenceID, err))
			continue
		}
		repaired++
	}
	if len(failures) > 0 {
		return repaired, fmt.Errorf("repair evidence carrier projection debt: %s", strings.Join(failures, "; "))
	}
	return repaired, nil
}
