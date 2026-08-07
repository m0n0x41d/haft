package graph

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/m0n0x41d/haft/internal/governance"
	"github.com/m0n0x41d/haft/internal/projectpath"
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

// FindDecisionsForFile is the legacy union of exact affected-path context and
// module context. Callers that make authority claims must use the two explicit
// lanes instead.
func (s *Store) FindDecisionsForFile(ctx context.Context, filePath string) ([]Node, error) {
	affectedPathContext, err := s.FindAffectedPathContextDecisions(
		ctx,
		filePath,
	)
	if err != nil {
		return nil, err
	}
	moduleDecisions, err := s.FindModuleDecisionsForFile(ctx, filePath)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	result := make(
		[]Node,
		0,
		len(affectedPathContext)+len(moduleDecisions),
	)
	for _, node := range append(
		affectedPathContext,
		moduleDecisions...,
	) {
		if seen[node.ID] {
			continue
		}
		seen[node.ID] = true
		result = append(result, node)
	}
	return result, nil
}

// FindAffectedPathContextDecisions returns current DecisionRecords with an
// exact affected_files backlink. This is context, never proof of a binding.
func (s *Store) FindAffectedPathContextDecisions(
	ctx context.Context,
	filePath string,
) ([]Node, error) {
	canonical, err := projectpath.Parse(filePath)
	if err != nil {
		return nil, fmt.Errorf(
			"find affected-path decision context: %w",
			err,
		)
	}
	paths, err := s.currentDecisionPaths(ctx)
	if err != nil {
		return nil, err
	}

	var result []Node
	seen := make(map[string]bool)
	for _, candidate := range paths {
		path, err := projectpath.Parse(candidate.path)
		if err != nil || path.String() != canonical.String() {
			continue
		}
		if seen[candidate.decisionID] {
			continue
		}
		seen[candidate.decisionID] = true
		result = append(result, Node{
			ID:   candidate.decisionID,
			Kind: KindDecision,
			Name: candidate.decisionTitle,
		})
	}
	return result, nil
}

// FindExactDecisionsForFile is retained as a compatibility alias. Its result
// is exact affected-path context, not exact binding authority.
func (s *Store) FindExactDecisionsForFile(
	ctx context.Context,
	filePath string,
) ([]Node, error) {
	return s.FindAffectedPathContextDecisions(ctx, filePath)
}

// FindModuleDecisionsForFile returns current DecisionRecords whose module-mode
// path scope reaches the file's most-specific indexed module.
func (s *Store) FindModuleDecisionsForFile(
	ctx context.Context,
	filePath string,
) ([]Node, error) {
	canonical, err := projectpath.Parse(filePath)
	if err != nil {
		return nil, fmt.Errorf("find module decisions for file: %w", err)
	}
	return s.findDecisionsThroughModule(ctx, canonical.String())
}

// findDecisionsThroughModule finds decisions where ANY affected file
// belongs to the same module as the given file.
func (s *Store) findDecisionsThroughModule(ctx context.Context, filePath string) ([]Node, error) {
	module, err := s.FindModuleForFile(ctx, filePath)
	if err != nil || module == nil {
		return nil, err
	}
	return s.FindDecisionsForModule(ctx, module.ID)
}

// FindInvariantsForFile returns only module-context invariants. Exact binding
// invariants require a symbol/file binding target and are assembled by
// contextgraph with that target identity.
func (s *Store) FindInvariantsForFile(ctx context.Context, filePath string) ([]Invariant, error) {
	decisions, err := s.FindModuleDecisionsForFile(ctx, filePath)
	if err != nil {
		return nil, err
	}
	return s.FindInvariantsForDecisions(ctx, decisions)
}

// FindInvariantsForDecisions returns the invariant projection for the supplied
// already-classified decision lane.
func (s *Store) FindInvariantsForDecisions(
	ctx context.Context,
	decisions []Node,
) ([]Invariant, error) {
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
	canonical, err := projectpath.Parse(filePath)
	if err != nil {
		return nil, fmt.Errorf("find module for file: %w", err)
	}
	modules, err := s.indexedModules(ctx)
	if err != nil {
		return nil, err
	}
	module, ok, err := mostSpecificIndexedModule(modules, canonical)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	node := module.node
	return &node, nil
}

// TransitiveDependents returns all modules that depend on the given module,
// directly or transitively, using a recursive CTE.
func (s *Store) TransitiveDependents(ctx context.Context, moduleID string) ([]Node, error) {
	rows, err := s.db.QueryContext(ctx, `
		WITH RECURSIVE deps(mid, depth, path) AS (
			SELECT source_module, 1, '|' || target_module || '|' || source_module || '|'
			FROM module_dependencies
			WHERE target_module = ?
			UNION
			SELECT md.source_module, d.depth + 1, d.path || md.source_module || '|'
			FROM deps d
			JOIN module_dependencies md ON md.target_module = d.mid
			WHERE d.depth < 10
			  AND instr(d.path, '|' || md.source_module || '|') = 0
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

// FindDecisionsForModule returns current module-mode DecisionRecords whose
// affected path resolves to this exact most-specific indexed module.
func (s *Store) FindDecisionsForModule(ctx context.Context, moduleID string) ([]Node, error) {
	contexts, err := s.AllDecisionModuleContexts(ctx)
	if err != nil {
		return nil, err
	}

	var result []Node
	for _, item := range contexts {
		if item.ModuleID != moduleID {
			continue
		}
		result = append(result, Node{
			ID:   item.DecisionID,
			Kind: KindDecision,
			Name: item.DecisionTitle,
		})
	}
	return result, nil
}

// DecisionModuleContext is one explicit contextual relation between a current
// module-mode DecisionRecord and the exact most-specific indexed module that
// owns one of its affected paths.
type DecisionModuleContext struct {
	DecisionID    string
	DecisionTitle string
	ModuleID      string
	ModulePath    string
	Source        string
}

// AllDecisionModuleContexts returns deduplicated current module-context
// relations. It never emits exact-mode or footprint-only decisions.
func (s *Store) AllDecisionModuleContexts(
	ctx context.Context,
) ([]DecisionModuleContext, error) {
	modules, err := s.indexedModules(ctx)
	if err != nil {
		return nil, err
	}
	decisions, err := s.currentDecisions(ctx)
	if err != nil {
		return nil, err
	}
	paths, err := s.currentDecisionPaths(ctx)
	if err != nil {
		return nil, err
	}
	indexedFiles, _, err := s.indexedSourcePaths(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]DecisionModuleContext, 0)
	seen := make(map[string]bool)
	pathsByDecision := make(map[string][]currentDecisionPath)
	for _, candidate := range paths {
		pathsByDecision[candidate.decisionID] = append(
			pathsByDecision[candidate.decisionID],
			candidate,
		)
	}
	for _, decision := range decisions {
		policy, err := governance.ParseDecisionPathPolicy(
			decision.structuredData,
		)
		if err != nil {
			continue
		}
		for _, moduleTarget := range policy.ModuleTargets() {
			module, ok := exactIndexedModule(modules, moduleTarget)
			if !ok {
				continue
			}
			result = appendDecisionModuleContext(
				result,
				seen,
				decision,
				module,
				"explicit_module_binding",
			)
		}
		usesLegacyPathScope := policy.UsesLegacyModulePathScope()
		usesTypedModuleRootPathScope :=
			policy.UsesTypedModuleRootPathScope()
		if !usesLegacyPathScope && !usesTypedModuleRootPathScope {
			continue
		}
		for _, candidate := range pathsByDecision[decision.decisionID] {
			affectedPath, err := projectpath.Parse(candidate.path)
			if err != nil {
				continue
			}
			module, ok, err := mostSpecificIndexedModule(
				modules,
				affectedPath,
			)
			if err != nil {
				return nil, err
			}
			if !ok {
				continue
			}
			if !policy.AllowsAffectedPathModuleContext(
				candidate.path,
				affectedPath,
				module.path,
				indexedFiles[affectedPath.String()],
			) {
				continue
			}
			result = appendDecisionModuleContext(
				result,
				seen,
				decision,
				module,
				moduleContextSource(
					affectedPath,
					module.path,
					usesTypedModuleRootPathScope &&
						!usesLegacyPathScope,
				),
			)
		}
	}
	sortDecisionModuleContexts(result)
	return result, nil
}

func exactIndexedModule(
	modules []indexedModule,
	target projectpath.ModulePath,
) (indexedModule, bool) {
	for _, module := range modules {
		if module.path.String() == target.String() {
			return module, true
		}
	}
	return indexedModule{}, false
}

func appendDecisionModuleContext(
	result []DecisionModuleContext,
	seen map[string]bool,
	decision currentDecision,
	module indexedModule,
	source string,
) []DecisionModuleContext {
	key := decision.decisionID + "\x00" + module.node.ID
	if seen[key] {
		return result
	}
	seen[key] = true
	return append(result, DecisionModuleContext{
		DecisionID:    decision.decisionID,
		DecisionTitle: decision.decisionTitle,
		ModuleID:      module.node.ID,
		ModulePath:    module.node.Path,
		Source:        source,
	})
}

func moduleContextSource(
	affectedPath projectpath.Path,
	modulePath projectpath.ModulePath,
	typedModuleRoot bool,
) string {
	if typedModuleRoot {
		return "typed_affected_module_root"
	}
	if affectedPath.String() == modulePath.String() {
		return "legacy_module_root"
	}
	return "legacy_affected_file"
}

func sortDecisionModuleContexts(contexts []DecisionModuleContext) {
	sort.SliceStable(
		contexts,
		func(left int, right int) bool {
			leftRank := decisionModuleContextSourceRank(
				contexts[left].Source,
			)
			rightRank := decisionModuleContextSourceRank(
				contexts[right].Source,
			)
			if leftRank != rightRank {
				return leftRank < rightRank
			}
			if contexts[left].ModulePath != contexts[right].ModulePath {
				return contexts[left].ModulePath <
					contexts[right].ModulePath
			}
			return contexts[left].DecisionID <
				contexts[right].DecisionID
		},
	)
}

func decisionModuleContextSourceRank(source string) int {
	switch source {
	case "explicit_module_binding":
		return 0
	case "typed_affected_module_root":
		return 1
	case "legacy_module_root":
		return 2
	case "legacy_affected_file":
		return 3
	default:
		return 4
	}
}

func (s *Store) indexedSourcePaths(
	ctx context.Context,
) (map[string]bool, bool, error) {
	var tableCount int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM sqlite_master
		WHERE type = 'table' AND name = 'code_files'`).Scan(&tableCount)
	if err != nil {
		return nil, false, err
	}
	if tableCount == 0 {
		return nil, false, nil
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT file_path
		FROM code_files
		ORDER BY file_path`)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	result := make(map[string]bool)
	for rows.Next() {
		var rawPath string
		if err := rows.Scan(&rawPath); err != nil {
			return nil, false, err
		}
		filePath, err := projectpath.Parse(rawPath)
		if err != nil || rawPath != filePath.String() {
			continue
		}
		result[filePath.String()] = true
	}
	return result, len(result) > 0, rows.Err()
}

type currentDecisionPath struct {
	decisionID     string
	decisionTitle  string
	path           string
	structuredData string
}

type currentDecision struct {
	decisionID     string
	decisionTitle  string
	structuredData string
}

func (s *Store) currentDecisions(
	ctx context.Context,
) ([]currentDecision, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, title, COALESCE(structured_data, '{}')
		FROM artifacts
		WHERE kind = 'DecisionRecord'
		  AND status IN ('active', 'refresh_due')
		ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]currentDecision, 0)
	for rows.Next() {
		var item currentDecision
		err := rows.Scan(
			&item.decisionID,
			&item.decisionTitle,
			&item.structuredData,
		)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Store) currentDecisionPaths(
	ctx context.Context,
) ([]currentDecisionPath, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT a.id, a.title, af.file_path, COALESCE(a.structured_data, '{}')
		FROM affected_files af
		JOIN artifacts a ON a.id = af.artifact_id
		WHERE a.kind = 'DecisionRecord'
		  AND a.status IN ('active', 'refresh_due')
		ORDER BY a.id, af.file_path`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]currentDecisionPath, 0)
	for rows.Next() {
		var item currentDecisionPath
		err := rows.Scan(
			&item.decisionID,
			&item.decisionTitle,
			&item.path,
			&item.structuredData,
		)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

type indexedModule struct {
	node Node
	path projectpath.ModulePath
	ref  projectpath.ModuleRef
}

func (s *Store) indexedModules(ctx context.Context) ([]indexedModule, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT module_id, path, name
		FROM codebase_modules
		ORDER BY LENGTH(path) DESC, module_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]indexedModule, 0)
	for rows.Next() {
		var id, rawPath, name string
		if err := rows.Scan(&id, &rawPath, &name); err != nil {
			return nil, err
		}
		moduleRef, err := projectpath.NewModuleRef(id, rawPath)
		if err != nil {
			return nil, fmt.Errorf(
				"indexed module %s has invalid path %q: %w",
				id,
				rawPath,
				err,
			)
		}
		result = append(result, indexedModule{
			node: Node{
				ID:   id,
				Kind: KindModule,
				Name: name,
				Path: moduleRef.Path().String(),
			},
			path: moduleRef.Path(),
			ref:  moduleRef,
		})
	}
	return result, rows.Err()
}

func mostSpecificIndexedModule(
	modules []indexedModule,
	candidate projectpath.Path,
) (indexedModule, bool, error) {
	refs := make([]projectpath.ModuleRef, 0, len(modules))
	for _, module := range modules {
		refs = append(refs, module.ref)
	}
	resolved, ok, err := projectpath.ResolveMostSpecificModule(
		refs,
		candidate,
	)
	if err != nil || !ok {
		return indexedModule{}, false, err
	}
	for _, module := range modules {
		if module.ref.ID() == resolved.ID() {
			return module, true, nil
		}
	}
	return indexedModule{}, false, fmt.Errorf(
		"resolved module %q is missing from indexed module set",
		resolved.ID(),
	)
}
