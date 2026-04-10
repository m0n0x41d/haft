package graph

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
)

// Store provides graph queries over existing Haft tables.
// It does NOT own the database connection — the caller manages lifecycle.
type Store struct {
	db *sql.DB
}

// NewStore creates a graph query store from an existing DB connection.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// FindDecisionsForFile returns all active decisions that govern a file,
// either directly (via affected_files) or transitively (via module membership).
func (s *Store) FindDecisionsForFile(ctx context.Context, filePath string) ([]Node, error) {
	// Direct: file is in a decision's affected_files
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT a.id, a.title
		FROM affected_files af
		JOIN artifacts a ON a.id = af.artifact_id
		WHERE af.file_path = ?
		  AND a.kind = 'DecisionRecord'
		  AND a.status = 'active'
	`, filePath)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	seen := make(map[string]bool)
	var result []Node

	for rows.Next() {
		var id, title string
		if err := rows.Scan(&id, &title); err != nil {
			return nil, err
		}
		if !seen[id] {
			seen[id] = true
			result = append(result, Node{ID: id, Kind: KindDecision, Name: title})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Transitive: file belongs to a module that has decisions via affected_files
	moduleDecisions, err := s.findDecisionsThroughModule(ctx, filePath)
	if err != nil {
		return nil, err
	}
	for _, node := range moduleDecisions {
		if !seen[node.ID] {
			seen[node.ID] = true
			result = append(result, node)
		}
	}

	return result, nil
}

// findDecisionsThroughModule finds decisions where ANY affected file
// belongs to the same module as the given file.
func (s *Store) findDecisionsThroughModule(ctx context.Context, filePath string) ([]Node, error) {
	// Find the module this file belongs to
	module, err := s.FindModuleForFile(ctx, filePath)
	if err != nil || module == nil {
		return nil, err
	}

	// Find all decisions that have affected files in this module
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT a.id, a.title
		FROM affected_files af
		JOIN artifacts a ON a.id = af.artifact_id
		JOIN codebase_modules m ON af.file_path LIKE m.path || '/%'
		WHERE m.module_id = ?
		  AND a.kind = 'DecisionRecord'
		  AND a.status = 'active'
	`, module.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Node
	for rows.Next() {
		var id, title string
		if err := rows.Scan(&id, &title); err != nil {
			return nil, err
		}
		result = append(result, Node{ID: id, Kind: KindDecision, Name: title})
	}

	return result, rows.Err()
}

// FindInvariantsForFile returns all invariants from decisions governing a file.
func (s *Store) FindInvariantsForFile(ctx context.Context, filePath string) ([]Invariant, error) {
	decisions, err := s.FindDecisionsForFile(ctx, filePath)
	if err != nil {
		return nil, err
	}

	var result []Invariant
	for _, dec := range decisions {
		invariants, err := s.extractInvariants(ctx, dec.ID, dec.Name)
		if err != nil {
			continue // best-effort: skip decisions with corrupt structured_data
		}
		result = append(result, invariants...)
	}

	return result, nil
}

// extractInvariants reads the invariants array from a decision's structured_data JSON.
func (s *Store) extractInvariants(ctx context.Context, decisionID, decisionTitle string) ([]Invariant, error) {
	var raw string
	err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(structured_data, '{}')
		FROM artifacts
		WHERE id = ?
	`, decisionID).Scan(&raw)
	if err != nil {
		return nil, err
	}

	var fields struct {
		Invariants []string `json:"invariants"`
	}
	if err := json.Unmarshal([]byte(raw), &fields); err != nil {
		return nil, err
	}

	result := make([]Invariant, 0, len(fields.Invariants))
	for _, text := range fields.Invariants {
		if strings.TrimSpace(text) == "" {
			continue
		}
		result = append(result, Invariant{
			Text:          text,
			DecisionID:    decisionID,
			DecisionTitle: decisionTitle,
		})
	}
	return result, nil
}

// FindModuleForFile returns the codebase module that contains a given file path.
// Uses longest-prefix matching on the module path.
func (s *Store) FindModuleForFile(ctx context.Context, filePath string) (*Node, error) {
	// Find the module whose path is the longest prefix of the file path
	var moduleID, modulePath, moduleName string
	err := s.db.QueryRowContext(ctx, `
		SELECT module_id, path, name
		FROM codebase_modules
		WHERE ? LIKE path || '/%' OR ? = path
		ORDER BY LENGTH(path) DESC
		LIMIT 1
	`, filePath, filePath).Scan(&moduleID, &modulePath, &moduleName)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &Node{
		ID:   moduleID,
		Kind: KindModule,
		Name: moduleName,
		Path: modulePath,
	}, nil
}

// TransitiveDependents returns all modules that depend on the given module,
// directly or transitively, using a recursive CTE.
func (s *Store) TransitiveDependents(ctx context.Context, moduleID string) ([]Node, error) {
	rows, err := s.db.QueryContext(ctx, `
		WITH RECURSIVE deps(mid, depth, path) AS (
			SELECT target_module, 1, source_module || '/' || target_module
			FROM module_dependencies
			WHERE source_module = ?
			UNION
			SELECT md.target_module, d.depth + 1, d.path || '/' || md.target_module
			FROM deps d
			JOIN module_dependencies md ON md.source_module = d.mid
			WHERE d.depth < 10
			  AND d.path NOT LIKE '%' || md.target_module || '%'
		)
		SELECT DISTINCT m.module_id, m.path, m.name, d.depth
		FROM deps d
		JOIN codebase_modules m ON m.module_id = d.mid
		ORDER BY d.depth, m.path
	`, moduleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Node
	for rows.Next() {
		var id, path, name string
		var depth int
		if err := rows.Scan(&id, &path, &name, &depth); err != nil {
			return nil, err
		}
		_ = depth
		result = append(result, Node{ID: id, Kind: KindModule, Name: name, Path: path})
	}

	return result, rows.Err()
}

// FindDecisionsForModule returns all active decisions that have affected files
// within the given module.
func (s *Store) FindDecisionsForModule(ctx context.Context, moduleID string) ([]Node, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT a.id, a.title
		FROM affected_files af
		JOIN artifacts a ON a.id = af.artifact_id
		JOIN codebase_modules m ON af.file_path LIKE m.path || '/%'
		WHERE m.module_id = ?
		  AND a.kind = 'DecisionRecord'
		  AND a.status = 'active'
	`, moduleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Node
	for rows.Next() {
		var id, title string
		if err := rows.Scan(&id, &title); err != nil {
			return nil, err
		}
		result = append(result, Node{ID: id, Kind: KindDecision, Name: title})
	}

	return result, rows.Err()
}
