package db

import (
	"context"
	"database/sql"
	"time"
)

type DecisionContextRow struct {
	ID    string
	Title string
	Scope string
}

type DecisionSummaryRow struct {
	ID           string
	Title        string
	CreatedAt    sql.NullTime
	EvidenceType sql.NullString
	ResolvedAt   sql.NullTime
	Content      sql.NullString
	CarrierRef   sql.NullString
}

type OrphanedHypothesisRow struct {
	ID string
}

type RelationRow struct {
	TargetID     string
	RelationType string
}

type FreshnessRow struct {
	EvidenceID   string
	HolonID      string
	Title        string
	Layer        string
	EvidenceType string
	DaysOverdue  int
}

type WaiverRow struct {
	EvidenceID      string
	HolonID         string
	HolonTitle      string
	WaivedUntil     string
	WaivedBy        string
	Rationale       string
	DaysUntilExpiry int
}

func (s *Store) GetActiveDecisionContexts(ctx context.Context) ([]DecisionContextRow, error) {
	rows, err := s.conn.QueryContext(ctx, `
		SELECT h.id, h.title, COALESCE(h.scope, '') as scope
		FROM holons h
		WHERE h.type = 'decision_context'
		AND (h.context_status IS NULL OR h.context_status = 'open')
		AND h.id NOT IN (
		    SELECT target_id FROM relations WHERE relation_type = 'closes'
		)
		ORDER BY h.created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []DecisionContextRow
	for rows.Next() {
		var row DecisionContextRow
		if err := rows.Scan(&row.ID, &row.Title, &row.Scope); err != nil {
			continue
		}
		results = append(results, row)
	}
	return results, nil
}

func (s *Store) GetHypothesisCountForContext(ctx context.Context, dcID string) int64 {
	var count int64
	_ = s.conn.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM relations r
		JOIN holons h ON h.id = r.source_id
		WHERE r.target_id = ? AND r.relation_type = 'memberOf'
		AND h.type = 'hypothesis'
	`, dcID).Scan(&count)
	return count
}

type ApproachTypeStat struct {
	ApproachType string
	Count        int64
}

func (s *Store) GetApproachTypeDistribution(ctx context.Context, dcID string) []ApproachTypeStat {
	rows, err := s.conn.QueryContext(ctx, `
		SELECT COALESCE(h.approach_type, '') as approach_type, COUNT(*) as count
		FROM relations r
		JOIN holons h ON h.id = r.source_id
		WHERE r.target_id = ? AND r.relation_type = 'memberOf'
		AND h.type = 'hypothesis' AND h.layer NOT IN ('invalid')
		GROUP BY h.approach_type
	`, dcID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var stats []ApproachTypeStat
	for rows.Next() {
		var stat ApproachTypeStat
		if err := rows.Scan(&stat.ApproachType, &stat.Count); err != nil {
			continue
		}
		stats = append(stats, stat)
	}
	return stats
}

func (s *Store) GetDecisionContextForHolon(ctx context.Context, holonID string) string {
	var targetID string
	err := s.conn.QueryRowContext(ctx,
		`SELECT target_id FROM relations WHERE source_id = ? AND relation_type = 'memberOf' LIMIT 1`,
		holonID).Scan(&targetID)
	if err != nil {
		return ""
	}
	return targetID
}

func (s *Store) GetClosingDRRForContext(ctx context.Context, dcID string) string {
	var drrID string
	err := s.conn.QueryRowContext(ctx,
		`SELECT source_id FROM relations WHERE target_id = ? AND relation_type = 'closes' LIMIT 1`,
		dcID).Scan(&drrID)
	if err != nil {
		return ""
	}
	return drrID
}

func (s *Store) GetOpenDRRForHypothesis(ctx context.Context, hypID string) string {
	var drrID string
	err := s.conn.QueryRowContext(ctx, `
		SELECT r.source_id FROM relations r
		JOIN holons h ON h.id = r.source_id
		WHERE r.target_id = ?
		  AND r.relation_type IN ('selects', 'rejects')
		  AND h.type = 'DRR'
		  AND NOT EXISTS (
		      SELECT 1 FROM evidence e
		      WHERE e.holon_id = h.id
		      AND e.type IN ('implementation', 'abandonment', 'supersession')
		  )
		LIMIT 1`,
		hypID).Scan(&drrID)
	if err != nil {
		return ""
	}
	return drrID
}

func (s *Store) GetOrphanedHypotheses(ctx context.Context, dcID string) ([]string, error) {
	rows, err := s.conn.QueryContext(ctx, `
		SELECT r.source_id FROM relations r
		JOIN holons h ON h.id = r.source_id
		WHERE r.target_id = ?
		  AND r.relation_type = 'memberOf'
		  AND h.layer IN ('L0', 'L1', 'L2')
	`, dcID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (s *Store) GetOpenDecisions(ctx context.Context) ([]DecisionSummaryRow, error) {
	rows, err := s.conn.QueryContext(ctx, `
		SELECT h.id, h.title, h.created_at
		FROM holons h
		WHERE (h.type = 'DRR' OR h.layer = 'DRR')
		AND NOT EXISTS (
			SELECT 1 FROM evidence e
			WHERE e.holon_id = h.id
			AND e.type IN ('implementation', 'abandonment', 'supersession')
		)
		ORDER BY h.created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []DecisionSummaryRow
	for rows.Next() {
		var row DecisionSummaryRow
		if err := rows.Scan(&row.ID, &row.Title, &row.CreatedAt); err != nil {
			continue
		}
		results = append(results, row)
	}
	return results, nil
}

func (s *Store) GetResolvedDecisions(ctx context.Context, evidenceType string, limit int) ([]DecisionSummaryRow, error) {
	rows, err := s.conn.QueryContext(ctx, `
		SELECT h.id, h.title, h.created_at, e.type, e.created_at as resolved_at, e.content, e.carrier_ref
		FROM holons h
		JOIN evidence e ON e.holon_id = h.id
		WHERE (h.type = 'DRR' OR h.layer = 'DRR')
		AND e.type = ?
		ORDER BY e.created_at DESC
		LIMIT ?
	`, evidenceType, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []DecisionSummaryRow
	for rows.Next() {
		var row DecisionSummaryRow
		if err := rows.Scan(&row.ID, &row.Title, &row.CreatedAt, &row.EvidenceType, &row.ResolvedAt, &row.Content, &row.CarrierRef); err != nil {
			continue
		}
		results = append(results, row)
	}
	return results, nil
}

func (s *Store) GetRecentResolvedDecisions(ctx context.Context, limit int) ([]DecisionSummaryRow, error) {
	rows, err := s.conn.QueryContext(ctx, `
		SELECT h.id, h.title, h.created_at, e.type, e.created_at as resolved_at, e.content, e.carrier_ref
		FROM holons h
		JOIN evidence e ON e.holon_id = h.id
		WHERE (h.type = 'DRR' OR h.layer = 'DRR')
		AND e.type IN ('implementation', 'abandonment', 'supersession')
		ORDER BY e.created_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []DecisionSummaryRow
	for rows.Next() {
		var row DecisionSummaryRow
		if err := rows.Scan(&row.ID, &row.Title, &row.CreatedAt, &row.EvidenceType, &row.ResolvedAt, &row.Content, &row.CarrierRef); err != nil {
			continue
		}
		results = append(results, row)
	}
	return results, nil
}

func (s *Store) GetInvalidHolonsWithTitle(ctx context.Context, title, excludeID string) ([]string, error) {
	rows, err := s.conn.QueryContext(ctx, `
		SELECT id FROM holons
		WHERE layer = 'invalid'
		AND title = ?
		AND id != ?
	`, title, excludeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (s *Store) GetRelationsForHolon(ctx context.Context, holonID string) ([]RelationRow, error) {
	rows, err := s.conn.QueryContext(ctx,
		`SELECT target_id, relation_type FROM relations WHERE source_id = ?`, holonID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []RelationRow
	for rows.Next() {
		var row RelationRow
		if err := rows.Scan(&row.TargetID, &row.RelationType); err != nil {
			continue
		}
		results = append(results, row)
	}
	return results, nil
}

func (s *Store) GetStaleEvidence(ctx context.Context) ([]FreshnessRow, error) {
	rows, err := s.conn.QueryContext(ctx, `
		SELECT
			e.id as evidence_id,
			e.holon_id,
			h.title,
			h.layer,
			e.type as evidence_type,
			CAST(JULIANDAY('now') - JULIANDAY(substr(e.valid_until, 1, 10)) AS INTEGER) as days_overdue
		FROM evidence e
		JOIN active_holons h ON e.holon_id = h.id
		LEFT JOIN (
			SELECT evidence_id, MAX(waived_until) as latest_waiver
			FROM waivers
			GROUP BY evidence_id
		) w ON e.id = w.evidence_id
		WHERE e.valid_until IS NOT NULL
		  AND substr(e.valid_until, 1, 10) < date('now')
		  AND (w.latest_waiver IS NULL OR w.latest_waiver < datetime('now'))
		ORDER BY h.id, days_overdue DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []FreshnessRow
	for rows.Next() {
		var row FreshnessRow
		if err := rows.Scan(&row.EvidenceID, &row.HolonID, &row.Title, &row.Layer, &row.EvidenceType, &row.DaysOverdue); err != nil {
			continue
		}
		results = append(results, row)
	}
	return results, nil
}

func (s *Store) GetActiveWaivers(ctx context.Context) ([]WaiverRow, error) {
	rows, err := s.conn.QueryContext(ctx, `
		SELECT w.evidence_id, e.holon_id, h.title, w.waived_until, w.waived_by, w.rationale,
		       CAST(JULIANDAY(w.waived_until) - JULIANDAY('now') AS INTEGER) as days_until_expiry
		FROM waivers w
		JOIN evidence e ON w.evidence_id = e.id
		JOIN active_holons h ON e.holon_id = h.id
		WHERE w.waived_until > datetime('now')
		ORDER BY w.waived_until ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []WaiverRow
	for rows.Next() {
		var row WaiverRow
		if err := rows.Scan(&row.EvidenceID, &row.HolonID, &row.HolonTitle, &row.WaivedUntil, &row.WaivedBy, &row.Rationale, &row.DaysUntilExpiry); err != nil {
			continue
		}
		results = append(results, row)
	}
	return results, nil
}

func (s *Store) CountAllHolonsByLayer(ctx context.Context, layer string) int64 {
	var count int64
	_ = s.conn.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM holons WHERE layer = ?`, layer).Scan(&count)
	return count
}

func (s *Store) CountHypothesesByLayer(ctx context.Context, layer string) int64 {
	var count int64
	_ = s.conn.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM holons WHERE layer = ? AND type = 'hypothesis'`, layer).Scan(&count)
	return count
}

func (s *Store) CountDRRs(ctx context.Context) int64 {
	var count int64
	_ = s.conn.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM holons WHERE type = 'DRR' OR layer = 'DRR'`).Scan(&count)
	return count
}

type ContextLayerCounts struct {
	L0 int64
	L1 int64
	L2 int64
}

func (s *Store) GetContextLayerCounts(ctx context.Context, dcID string) ContextLayerCounts {
	var counts ContextLayerCounts
	rows, err := s.conn.QueryContext(ctx, `
		SELECT h.layer, COUNT(*) as count
		FROM holons h
		JOIN relations r ON h.id = r.source_id
		WHERE r.target_id = ? AND r.relation_type = 'memberOf'
		  AND h.layer NOT IN ('invalid')
		GROUP BY h.layer`, dcID)
	if err != nil {
		return counts
	}
	defer rows.Close()

	for rows.Next() {
		var layer string
		var count int64
		if err := rows.Scan(&layer, &count); err != nil {
			continue
		}
		switch layer {
		case "L0":
			counts.L0 = count
		case "L1":
			counts.L1 = count
		case "L2":
			counts.L2 = count
		}
	}
	return counts
}

func (s *Store) HasUnauditedL2InContext(ctx context.Context, dcID string) bool {
	var exists bool
	err := s.conn.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM holons h
			JOIN relations r ON h.id = r.source_id
			WHERE r.target_id = ? AND r.relation_type = 'memberOf'
			  AND h.layer = 'L2'
			  AND NOT EXISTS (
			      SELECT 1 FROM evidence e
			      WHERE e.holon_id = h.id AND e.type = 'audit_report'
			  )
		)`, dcID).Scan(&exists)
	if err != nil {
		return true
	}
	return exists
}

type HolonAgeRow struct {
	ID        string
	CreatedAt time.Time
}

func (s *Store) GetContextHolonAge(ctx context.Context, dcID string) *HolonAgeRow {
	var row HolonAgeRow
	err := s.conn.QueryRowContext(ctx, `
		SELECT h.id, h.created_at
		FROM holons h
		JOIN relations r ON h.id = r.source_id
		WHERE r.target_id = ? AND r.relation_type = 'memberOf'
		ORDER BY h.created_at ASC
		LIMIT 1`, dcID).Scan(&row.ID, &row.CreatedAt)
	if err != nil {
		return nil
	}
	return &row
}
