package fpf

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/m0n0x41d/quint-code/db"
	"github.com/m0n0x41d/quint-code/logger"
)

func formatAge(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	d := time.Since(t)
	if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(d.Hours()/24))
}

func (t *Tools) Search(query, scope, layerFilter, statusFilter, affectedScopeFilter string, limit int) (string, error) {
	defer t.RecordWork("Search", time.Now())

	logger.Info().
		Str("query", query).
		Str("scope", scope).
		Str("layer_filter", layerFilter).
		Str("status_filter", statusFilter).
		Int("limit", limit).
		Msg("Search called")

	if t.DB == nil {
		logger.Error().Msg("Search: database not initialized")
		return "", fmt.Errorf("database not initialized - run quint_internalize first")
	}

	if query == "" {
		return "", fmt.Errorf("query is required")
	}

	ctx := context.Background()
	results, err := t.DB.Search(ctx, query, scope, layerFilter, statusFilter, limit)
	if err != nil {
		logger.Error().Err(err).Str("query", query).Msg("Search: query failed")
		return "", fmt.Errorf("search failed: %w", err)
	}

	logger.Debug().Int("result_count", len(results)).Msg("Search: query executed")

	if affectedScopeFilter != "" {
		results = filterByAffectedScope(results, affectedScopeFilter)
	}

	if len(results) == 0 {
		return fmt.Sprintf("No results found for: %s", query), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Search Results for: %s\n\n", query))
	sb.WriteString(fmt.Sprintf("Found %d results\n\n", len(results)))

	for i, r := range results {
		sb.WriteString(fmt.Sprintf("### %d. %s\n", i+1, r.Title))
		sb.WriteString(fmt.Sprintf("- **ID:** %s\n", r.ID))
		sb.WriteString(fmt.Sprintf("- **Type:** %s\n", r.Type))
		if r.Layer != "" {
			sb.WriteString(fmt.Sprintf("- **Layer:** %s\n", r.Layer))
		}
		if r.RScore > 0 {
			sb.WriteString(fmt.Sprintf("- **R_eff:** %.2f\n", r.RScore))
		}
		if !r.UpdatedAt.IsZero() {
			sb.WriteString(fmt.Sprintf("- **Updated:** %s\n", formatAge(r.UpdatedAt)))
		}
		if r.Snippet != "" {
			sb.WriteString(fmt.Sprintf("- **Snippet:** %s\n", r.Snippet))
		}
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

func filterByAffectedScope(results []db.SearchResult, affectedScopeFilter string) []db.SearchResult {
	var filtered []db.SearchResult

	for _, r := range results {
		if r.Scope == "" {
			continue
		}

		var patterns []string
		if err := json.Unmarshal([]byte(r.Scope), &patterns); err != nil {
			patterns = []string{r.Scope}
		}

		for _, pattern := range patterns {
			matched, err := filepath.Match(pattern, affectedScopeFilter)
			if err == nil && matched {
				filtered = append(filtered, r)
				break
			}

			if strings.HasSuffix(pattern, "/*") || strings.HasSuffix(pattern, "/**") {
				prefix := strings.TrimSuffix(strings.TrimSuffix(pattern, "/*"), "/**")
				if strings.HasPrefix(affectedScopeFilter, prefix) {
					filtered = append(filtered, r)
					break
				}
			}

			if strings.Contains(affectedScopeFilter, pattern) || strings.Contains(pattern, affectedScopeFilter) {
				filtered = append(filtered, r)
				break
			}
		}
	}

	return filtered
}

func (t *Tools) GetOpenDecisions(ctx context.Context) ([]DecisionSummary, error) {
	if t.DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	query := `
		SELECT h.id, h.title, h.created_at
		FROM holons h
		WHERE (h.type = 'DRR' OR h.layer = 'DRR')
		AND NOT EXISTS (
			SELECT 1 FROM evidence e
			WHERE e.holon_id = h.id
			AND e.type IN ('implementation', 'abandonment', 'supersession')
		)
		ORDER BY h.created_at DESC
	`
	rows, err := t.DB.GetRawDB().QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []DecisionSummary
	for rows.Next() {
		var d DecisionSummary
		var createdAt sql.NullTime
		if err := rows.Scan(&d.ID, &d.Title, &createdAt); err != nil {
			continue
		}
		if createdAt.Valid {
			d.CreatedAt = createdAt.Time
		}
		d.Resolution = "open"
		results = append(results, d)
	}
	return results, nil
}

func (t *Tools) GetResolvedDecisions(ctx context.Context, resolution string, limit int) ([]DecisionSummary, error) {
	if t.DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	evidenceType := map[string]string{
		"implemented": "implementation",
		"abandoned":   "abandonment",
		"superseded":  "supersession",
	}[resolution]

	if evidenceType == "" {
		return nil, fmt.Errorf("invalid resolution filter: %s", resolution)
	}

	if limit <= 0 {
		limit = 10
	}

	query := `
		SELECT h.id, h.title, h.created_at, e.type, e.created_at as resolved_at, e.content, e.carrier_ref
		FROM holons h
		JOIN evidence e ON e.holon_id = h.id
		WHERE (h.type = 'DRR' OR h.layer = 'DRR')
		AND e.type = ?
		ORDER BY e.created_at DESC
		LIMIT ?
	`
	rows, err := t.DB.GetRawDB().QueryContext(ctx, query, evidenceType, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []DecisionSummary
	for rows.Next() {
		var d DecisionSummary
		var createdAt, resolvedAt sql.NullTime
		var evidenceType string
		var carrierRef sql.NullString
		if err := rows.Scan(&d.ID, &d.Title, &createdAt, &evidenceType, &resolvedAt, &d.Notes, &carrierRef); err != nil {
			continue
		}
		if createdAt.Valid {
			d.CreatedAt = createdAt.Time
		}
		if resolvedAt.Valid {
			d.ResolvedAt = resolvedAt.Time
		}
		if carrierRef.Valid {
			d.Reference = carrierRef.String
		}
		d.Resolution = resolution
		results = append(results, d)
	}
	return results, nil
}

func (t *Tools) GetRecentResolvedDecisions(ctx context.Context, limit int) ([]DecisionSummary, error) {
	if t.DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	if limit <= 0 {
		limit = 5
	}

	query := `
		SELECT h.id, h.title, h.created_at, e.type, e.created_at as resolved_at, e.content, e.carrier_ref
		FROM holons h
		JOIN evidence e ON e.holon_id = h.id
		WHERE (h.type = 'DRR' OR h.layer = 'DRR')
		AND e.type IN ('implementation', 'abandonment', 'supersession')
		ORDER BY e.created_at DESC
		LIMIT ?
	`
	rows, err := t.DB.GetRawDB().QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	evidenceToResolution := map[string]string{
		"implementation": "implemented",
		"abandonment":    "abandoned",
		"supersession":   "superseded",
	}

	var results []DecisionSummary
	for rows.Next() {
		var d DecisionSummary
		var createdAt, resolvedAt sql.NullTime
		var evidenceType string
		var carrierRef sql.NullString
		if err := rows.Scan(&d.ID, &d.Title, &createdAt, &evidenceType, &resolvedAt, &d.Notes, &carrierRef); err != nil {
			continue
		}
		if createdAt.Valid {
			d.CreatedAt = createdAt.Time
		}
		if resolvedAt.Valid {
			d.ResolvedAt = resolvedAt.Time
		}
		if carrierRef.Valid {
			d.Reference = carrierRef.String
		}
		d.Resolution = evidenceToResolution[evidenceType]
		results = append(results, d)
	}
	return results, nil
}
