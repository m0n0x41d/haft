package graph

import (
	"context"
	"database/sql"
	"strings"
)

// InvariantStatus represents the result of checking one invariant.
type InvariantStatus string

const (
	InvariantHolds    InvariantStatus = "holds"
	InvariantViolated InvariantStatus = "violated"
	InvariantUnknown  InvariantStatus = "unknown"
)

// InvariantResult is the outcome of verifying one invariant against code state.
type InvariantResult struct {
	Invariant Invariant       `json:"invariant"`
	Status    InvariantStatus `json:"status"`
	Reason    string          `json:"reason"`
}

// VerifyInvariants checks all invariants for a decision against the current
// module dependency graph. Returns per-invariant results.
//
// Currently supports pattern-based checks:
//   - "no import of X from Y" / "no direct ... access from Z" → check module_dependencies
//   - "no dependency from X to Y" → check dependency edges
//   - Unrecognized patterns → status "unknown" (needs manual verification)
func VerifyInvariants(ctx context.Context, s *Store, db *sql.DB, decisionID string) ([]InvariantResult, error) {
	invariants, err := s.extractInvariants(ctx, decisionID, "")
	if err != nil {
		return nil, err
	}

	results := make([]InvariantResult, 0, len(invariants))
	for _, inv := range invariants {
		result := checkInvariant(ctx, db, inv)
		results = append(results, result)
	}

	return results, nil
}

// checkInvariant attempts to verify a single invariant by parsing its text
// for recognizable patterns and checking them against the dependency graph.
func checkInvariant(ctx context.Context, db *sql.DB, inv Invariant) InvariantResult {
	text := strings.ToLower(inv.Text)

	// Pattern: "no import/dependency/access from X to/of Y"
	if from, to, ok := parseNoDependencyRule(text); ok {
		violated, reason := checkNoDependency(ctx, db, from, to)
		status := InvariantHolds
		if violated {
			status = InvariantViolated
		}
		return InvariantResult{
			Invariant: inv,
			Status:    status,
			Reason:    reason,
		}
	}

	// Pattern: "no circular dependencies" / "no circular imports"
	if strings.Contains(text, "no circular") && (strings.Contains(text, "depend") || strings.Contains(text, "import")) {
		violated, reason := checkNoCycles(ctx, db)
		status := InvariantHolds
		if violated {
			status = InvariantViolated
		}
		return InvariantResult{
			Invariant: inv,
			Status:    status,
			Reason:    reason,
		}
	}

	return InvariantResult{
		Invariant: inv,
		Status:    InvariantUnknown,
		Reason:    "Invariant text does not match a verifiable pattern. Manual check required.",
	}
}

// parseNoDependencyRule extracts source and target from patterns like:
//   - "no import of database from api"
//   - "no direct db access from handler layer"
//   - "no dependency from presentation to persistence"
func parseNoDependencyRule(text string) (from, to string, ok bool) {
	// "no ... from X to Y"
	if idx := strings.Index(text, " from "); idx >= 0 {
		rest := text[idx+6:]
		if toIdx := strings.Index(rest, " to "); toIdx >= 0 {
			from = strings.TrimSpace(rest[:toIdx])
			to = strings.TrimSpace(rest[toIdx+4:])
			// Clean trailing punctuation
			to = strings.TrimRight(to, ".,;:!? ")
			from = strings.TrimRight(from, ".,;:!? ")
			if from != "" && to != "" {
				return from, to, true
			}
		}
	}

	return "", "", false
}

// checkNoDependency verifies that no module matching `from` depends on
// any module matching `to` in the dependency graph.
func checkNoDependency(ctx context.Context, db *sql.DB, from, to string) (violated bool, reason string) {
	rows, err := db.QueryContext(ctx, `
		SELECT md.source_module, md.target_module, sm.path, tm.path
		FROM module_dependencies md
		JOIN codebase_modules sm ON sm.module_id = md.source_module
		JOIN codebase_modules tm ON tm.module_id = md.target_module
		WHERE (sm.path LIKE '%' || ? || '%' OR sm.name LIKE '%' || ? || '%')
		  AND (tm.path LIKE '%' || ? || '%' OR tm.name LIKE '%' || ? || '%')
	`, from, from, to, to)
	if err != nil {
		return false, "Could not query dependency graph: " + err.Error()
	}
	defer rows.Close()

	var violations []string
	for rows.Next() {
		var srcMod, tgtMod, srcPath, tgtPath string
		if err := rows.Scan(&srcMod, &tgtMod, &srcPath, &tgtPath); err != nil {
			continue
		}
		violations = append(violations, srcPath+" → "+tgtPath)
	}

	if len(violations) == 0 {
		return false, "No forbidden dependencies found."
	}

	return true, "Forbidden dependency detected: " + strings.Join(violations, ", ")
}

// checkNoCycles detects circular dependencies in the module graph.
func checkNoCycles(ctx context.Context, db *sql.DB) (violated bool, reason string) {
	// Use recursive CTE to find cycles.
	// The trick: DON'T filter out revisited nodes in the recursion —
	// instead, detect when current_mod equals start_mod (cycle found).
	rows, err := db.QueryContext(ctx, `
		WITH RECURSIVE chain(start_mod, current_mod, depth, path) AS (
			SELECT source_module, target_module, 1,
			       source_module || ' → ' || target_module
			FROM module_dependencies
			UNION
			SELECT c.start_mod, md.target_module, c.depth + 1,
			       c.path || ' → ' || md.target_module
			FROM chain c
			JOIN module_dependencies md ON md.source_module = c.current_mod
			WHERE c.depth < 20
			  AND md.target_module != c.start_mod
		)
		SELECT c.path || ' → ' || c.start_mod
		FROM chain c
		JOIN module_dependencies md ON md.source_module = c.current_mod AND md.target_module = c.start_mod
		LIMIT 3
	`)
	if err != nil {
		return false, "Could not check for cycles: " + err.Error()
	}
	defer rows.Close()

	var cycles []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			continue
		}
		cycles = append(cycles, path)
	}

	if len(cycles) == 0 {
		return false, "No circular dependencies detected."
	}

	return true, "Circular dependencies found: " + strings.Join(cycles, "; ")
}
