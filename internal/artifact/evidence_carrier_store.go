package artifact

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"
)

// EvidenceCarrierProjectionDebt is current, durable repair state for a
// committed EvidenceRecord whose Markdown carrier could not be published.
type EvidenceCarrierProjectionDebt struct {
	EvidenceID    string `json:"evidence_id"`
	ArtifactRef   string `json:"artifact_ref"`
	CarrierPath   string `json:"carrier_path"`
	DesiredDigest string `json:"desired_digest"`
	LastError     string `json:"last_error"`
	OpenedAt      string `json:"opened_at"`
	UpdatedAt     string `json:"updated_at"`
}

func (s *Store) RecordEvidenceCarrierProjectionDebt(ctx context.Context, debt EvidenceCarrierProjectionDebt) error {
	if strings.TrimSpace(debt.EvidenceID) == "" || strings.TrimSpace(debt.ArtifactRef) == "" {
		return fmt.Errorf("evidence projection debt requires evidence_id and artifact_ref")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO evidence_carrier_projection_debt (
			evidence_id, artifact_ref, carrier_path, desired_digest,
			last_error, opened_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(evidence_id) DO UPDATE SET
			artifact_ref = excluded.artifact_ref,
			carrier_path = excluded.carrier_path,
			desired_digest = excluded.desired_digest,
			last_error = excluded.last_error,
			updated_at = excluded.updated_at`,
		debt.EvidenceID,
		debt.ArtifactRef,
		debt.CarrierPath,
		debt.DesiredDigest,
		debt.LastError,
		now,
		now,
	)
	if err != nil {
		return fmt.Errorf("record evidence carrier projection debt: %w", err)
	}
	return nil
}

func (s *Store) ResolveEvidenceCarrierProjectionDebt(ctx context.Context, evidenceID string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM evidence_carrier_projection_debt WHERE evidence_id = ?`,
		evidenceID,
	)
	if err != nil {
		return fmt.Errorf("resolve evidence carrier projection debt: %w", err)
	}
	return nil
}

func (s *Store) ListEvidenceCarrierProjectionDebt(ctx context.Context) ([]EvidenceCarrierProjectionDebt, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT evidence_id, artifact_ref, carrier_path, desired_digest,
		       last_error, opened_at, updated_at
		FROM evidence_carrier_projection_debt
		ORDER BY opened_at, evidence_id`)
	if err != nil {
		return nil, fmt.Errorf("list evidence carrier projection debt: %w", err)
	}
	defer rows.Close()
	debts := make([]EvidenceCarrierProjectionDebt, 0)
	for rows.Next() {
		var debt EvidenceCarrierProjectionDebt
		if err := rows.Scan(
			&debt.EvidenceID,
			&debt.ArtifactRef,
			&debt.CarrierPath,
			&debt.DesiredDigest,
			&debt.LastError,
			&debt.OpenedAt,
			&debt.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan evidence carrier projection debt: %w", err)
		}
		debts = append(debts, debt)
	}
	return debts, rows.Err()
}

// CheckEvidenceCarrierImportProjectionDebt rejects an ambiguous git-vs-SQLite
// overwrite when a prior publication failure recorded an exact desired
// carrier digest. Legacy migration debt has no desired digest and is resolved
// by importing the valid carrier before repair.
func (s *Store) CheckEvidenceCarrierImportProjectionDebt(
	ctx context.Context,
	evidenceID string,
	content []byte,
) error {
	var desiredDigest string
	err := s.db.QueryRowContext(ctx,
		`SELECT desired_digest FROM evidence_carrier_projection_debt WHERE evidence_id = ?`,
		evidenceID,
	).Scan(&desiredDigest)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect evidence carrier projection debt for %s: %w", evidenceID, err)
	}
	if strings.TrimSpace(desiredDigest) == "" {
		return nil
	}
	observedDigest := evidenceCarrierContentDigest(content)
	if observedDigest == desiredDigest {
		return nil
	}
	return fmt.Errorf(
		"evidence carrier projection conflict for %s: pulled carrier digest %s differs from pending SQLite projection %s; reconcile the carrier explicitly before retrying",
		evidenceID,
		observedDigest,
		desiredDigest,
	)
}

// GetEvidenceItemByID resolves one stored EvidenceRecord and its exact parent.
func (s *Store) GetEvidenceItemByID(ctx context.Context, evidenceID string) (*EvidenceItem, string, error) {
	var artifactRef string
	if err := s.db.QueryRowContext(ctx,
		`SELECT artifact_ref FROM evidence_items WHERE id = ?`,
		evidenceID,
	).Scan(&artifactRef); err != nil {
		return nil, "", fmt.Errorf("get evidence %s parent: %w", evidenceID, err)
	}
	items, err := s.GetEvidenceItems(ctx, artifactRef)
	if err != nil {
		return nil, "", err
	}
	for index := range items {
		if items[index].ID == evidenceID {
			return &items[index], artifactRef, nil
		}
	}
	return nil, "", fmt.Errorf("get evidence %s: %w", evidenceID, sql.ErrNoRows)
}

// ImportEvidenceCarrier upserts the EvidenceRecord projection from one
// validated Markdown carrier. It requires an existing parent and never creates
// a WorkCommission, MethodRun, or any other parent artifact.
func (s *Store) ImportEvidenceCarrier(
	ctx context.Context,
	carrierArtifact *Artifact,
	carrier EvidenceCarrier,
) (string, error) {
	if carrierArtifact == nil {
		return "", fmt.Errorf("evidence carrier artifact is required")
	}
	if _, err := s.Get(ctx, carrier.ArtifactRef); err != nil {
		return "", fmt.Errorf("evidence parent %s must already exist: %w", carrier.ArtifactRef, err)
	}

	var existingItem *EvidenceItem
	existingParent := ""
	item, parent, err := s.GetEvidenceItemByID(ctx, carrier.Evidence.ID)
	switch {
	case err == nil:
		existingItem = item
		existingParent = parent
	case errors.Is(err, sql.ErrNoRows):
	case err != nil:
		return "", err
	}
	if existingParent != "" && existingParent != carrier.ArtifactRef {
		return "", fmt.Errorf(
			"evidence %s is already bound to parent %s, carrier requests %s",
			carrier.Evidence.ID,
			existingParent,
			carrier.ArtifactRef,
		)
	}

	legacyGhost, err := s.legacyEvidenceArtifact(ctx, carrierArtifact)
	if err != nil {
		return "", err
	}
	unchanged := existingItem != nil && evidenceItemsEqual(*existingItem, carrier.Evidence)
	if unchanged && !legacyGhost {
		return "unchanged", nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback() }()
	if legacyGhost {
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM artifact_links WHERE source_id = ? OR target_id = ?`,
			carrier.Evidence.ID,
			carrier.Evidence.ID,
		); err != nil {
			return "", fmt.Errorf("remove legacy evidence artifact links: %w", err)
		}
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM artifacts WHERE id = ? AND kind = ?`,
			carrier.Evidence.ID,
			string(KindEvidencePack),
		); err != nil {
			return "", fmt.Errorf("remove legacy evidence artifact row: %w", err)
		}
	}
	result := "created"
	if existingItem != nil {
		result = "updated"
		if !unchanged {
			if _, err := tx.ExecContext(ctx,
				`DELETE FROM evidence_items WHERE id = ?`,
				carrier.Evidence.ID,
			); err != nil {
				return "", fmt.Errorf("replace evidence %s: %w", carrier.Evidence.ID, err)
			}
		}
	}
	if !unchanged {
		item := carrier.Evidence
		if err := s.addEvidenceItemWithExec(ctx, tx, &item, carrier.ArtifactRef); err != nil {
			return "", fmt.Errorf("import evidence %s: %w", carrier.Evidence.ID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	if unchanged && legacyGhost {
		return "updated", nil
	}
	return result, nil
}

func (s *Store) legacyEvidenceArtifact(ctx context.Context, carrierArtifact *Artifact) (bool, error) {
	existing, err := s.Get(ctx, carrierArtifact.Meta.ID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if existing.Meta.Kind != KindEvidencePack {
		return false, fmt.Errorf(
			"evidence carrier id %s collides with %s artifact",
			carrierArtifact.Meta.ID,
			existing.Meta.Kind,
		)
	}
	if !canonicalJSONEqual(existing.StructuredData, carrierArtifact.StructuredData) {
		return false, fmt.Errorf(
			"legacy EvidencePack artifact %s does not match the carrier envelope",
			carrierArtifact.Meta.ID,
		)
	}
	return true, nil
}

func canonicalJSONEqual(left, right string) bool {
	var leftValue, rightValue any
	if json.Unmarshal([]byte(left), &leftValue) != nil ||
		json.Unmarshal([]byte(right), &rightValue) != nil {
		return strings.TrimSpace(left) == strings.TrimSpace(right)
	}
	return reflect.DeepEqual(leftValue, rightValue)
}

func evidenceItemsEqual(left, right EvidenceItem) bool {
	leftRecord := evidenceCarrierRecordFromItem(left)
	rightRecord := evidenceCarrierRecordFromItem(right)
	return reflect.DeepEqual(leftRecord, rightRecord)
}
