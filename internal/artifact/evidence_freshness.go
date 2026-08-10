package artifact

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/reff"
)

type EvidenceFreshnessClass string

// EvidenceFreshnessInventoryPosture states whether every reported count was
// collected from one complete scan. Unavailable inventories never expose
// partial counts as if they were a complete observation.
type EvidenceFreshnessInventoryPosture string

const (
	EvidenceFreshnessDated                       EvidenceFreshnessClass = "dated"
	EvidenceFreshnessExpired                     EvidenceFreshnessClass = "expired"
	EvidenceFreshnessExplicitPerpetualWithReason EvidenceFreshnessClass = "explicit_perpetual_with_rationale"
	EvidenceFreshnessLegacyBlankUnknown          EvidenceFreshnessClass = "legacy_blank_unknown"
	EvidenceFreshnessNotAssuranceApplicable      EvidenceFreshnessClass = "not_assurance_applicable"

	EvidenceFreshnessInventoryPostureAvailable   EvidenceFreshnessInventoryPosture = "available"
	EvidenceFreshnessInventoryPostureUnavailable EvidenceFreshnessInventoryPosture = "unavailable"

	EvidenceFreshnessDiagnosticAuthority = "read_only_classification_not_finding_or_policy"
)

type EvidenceFreshnessClassificationInput struct {
	ValidUntil          string
	PerpetualRationale  string
	AssuranceApplicable bool
}

type EvidenceFreshnessClassification struct {
	Class           EvidenceFreshnessClass `json:"class"`
	Authority       string                 `json:"authority"`
	ValidUntil      string                 `json:"valid_until,omitempty"`
	CheckedAt       string                 `json:"checked_at"`
	Diagnostic      string                 `json:"diagnostic,omitempty"`
	ScoringEffect   string                 `json:"scoring_effect"`
	AdmissionEffect string                 `json:"admission_effect"`
}

type EvidenceFreshnessInventory struct {
	Posture                            EvidenceFreshnessInventoryPosture `json:"posture"`
	Authority                          string                            `json:"authority"`
	CheckedAt                          string                            `json:"checked_at"`
	Diagnostic                         string                            `json:"diagnostic,omitempty"`
	TotalItems                         int                               `json:"total_items"`
	Dated                              int                               `json:"dated"`
	Expired                            int                               `json:"expired"`
	ExplicitPerpetualWithRationale     int                               `json:"explicit_perpetual_with_rationale"`
	LegacyBlankUnknown                 int                               `json:"legacy_blank_unknown"`
	NotAssuranceApplicable             int                               `json:"not_assurance_applicable"`
	UnparseableValidUntil              int                               `json:"unparseable_valid_until"`
	LegacyBlankUnknownIsCCED1Violation bool                              `json:"legacy_blank_unknown_is_cc_ed_1_violation"`
	FindingsAdded                      int                               `json:"findings_added"`
	ScoringChanged                     bool                              `json:"scoring_changed"`
	AdmissionChanged                   bool                              `json:"admission_changed"`
	MutationsPerformed                 bool                              `json:"mutations_performed"`
}

func ClassifyEvidenceFreshness(
	input EvidenceFreshnessClassificationInput,
	now time.Time,
) EvidenceFreshnessClassification {
	checkedAt := now
	if checkedAt.IsZero() {
		checkedAt = time.Now().UTC()
	}
	classification := EvidenceFreshnessClassification{
		Authority:       EvidenceFreshnessDiagnosticAuthority,
		ValidUntil:      strings.TrimSpace(input.ValidUntil),
		CheckedAt:       checkedAt.UTC().Format(time.RFC3339),
		ScoringEffect:   "none",
		AdmissionEffect: "none",
	}
	if !input.AssuranceApplicable {
		classification.Class = EvidenceFreshnessNotAssuranceApplicable
		return classification
	}
	if classification.ValidUntil == "" {
		if strings.TrimSpace(input.PerpetualRationale) != "" {
			classification.Class = EvidenceFreshnessExplicitPerpetualWithReason
			return classification
		}
		classification.Class = EvidenceFreshnessLegacyBlankUnknown
		classification.Diagnostic = "blank_valid_until_has_no_structured_perpetual_rationale"
		return classification
	}
	expiresAt, parsed := reff.ParseValidUntil(classification.ValidUntil)
	if !parsed {
		classification.Class = EvidenceFreshnessLegacyBlankUnknown
		classification.Diagnostic = "valid_until_unparseable_under_current_date_contract"
		return classification
	}
	if expiresAt.Before(checkedAt) {
		classification.Class = EvidenceFreshnessExpired
		return classification
	}
	classification.Class = EvidenceFreshnessDated
	return classification
}

// BuildEvidenceFreshnessInventory classifies the existing evidence rows
// without rewriting them or changing admission/scoring. The current schema has
// no perpetual-rationale column, so a blank stored valid_until is necessarily
// legacy_blank_unknown in this lens; it is not promoted to a CC-ED.1 finding.
func BuildEvidenceFreshnessInventory(
	ctx context.Context,
	store *Store,
	now time.Time,
) (EvidenceFreshnessInventory, error) {
	checkedAt := now
	if checkedAt.IsZero() {
		checkedAt = time.Now().UTC()
	}
	inventory := EvidenceFreshnessInventory{
		Posture:   EvidenceFreshnessInventoryPostureUnavailable,
		Authority: EvidenceFreshnessDiagnosticAuthority,
		CheckedAt: checkedAt.UTC().Format(time.RFC3339),
	}
	rows, err := store.db.QueryContext(
		ctx,
		"SELECT valid_until FROM evidence_items ORDER BY id",
	)
	if err != nil {
		return unavailableEvidenceFreshnessInventory(
			checkedAt,
			fmt.Errorf("read evidence freshness carriers: %w", err),
		)
	}
	defer rows.Close()
	for rows.Next() {
		var validUntil sql.NullString
		if err := rows.Scan(&validUntil); err != nil {
			return unavailableEvidenceFreshnessInventory(
				checkedAt,
				fmt.Errorf("scan evidence freshness carrier: %w", err),
			)
		}
		classification := ClassifyEvidenceFreshness(
			EvidenceFreshnessClassificationInput{
				ValidUntil:          validUntil.String,
				AssuranceApplicable: true,
			},
			checkedAt,
		)
		inventory.TotalItems++
		switch classification.Class {
		case EvidenceFreshnessDated:
			inventory.Dated++
		case EvidenceFreshnessExpired:
			inventory.Expired++
		case EvidenceFreshnessExplicitPerpetualWithReason:
			inventory.ExplicitPerpetualWithRationale++
		case EvidenceFreshnessLegacyBlankUnknown:
			inventory.LegacyBlankUnknown++
			if classification.Diagnostic ==
				"valid_until_unparseable_under_current_date_contract" {
				inventory.UnparseableValidUntil++
			}
		case EvidenceFreshnessNotAssuranceApplicable:
			inventory.NotAssuranceApplicable++
		}
	}
	if err := rows.Err(); err != nil {
		return unavailableEvidenceFreshnessInventory(
			checkedAt,
			fmt.Errorf("iterate evidence freshness carriers: %w", err),
		)
	}
	inventory.Posture = EvidenceFreshnessInventoryPostureAvailable
	return inventory, nil
}

func unavailableEvidenceFreshnessInventory(
	checkedAt time.Time,
	err error,
) (EvidenceFreshnessInventory, error) {
	return EvidenceFreshnessInventory{
		Posture:    EvidenceFreshnessInventoryPostureUnavailable,
		Authority:  EvidenceFreshnessDiagnosticAuthority,
		CheckedAt:  checkedAt.UTC().Format(time.RFC3339),
		Diagnostic: err.Error(),
	}, err
}
