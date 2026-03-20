package codebase

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/m0n0x41d/quint-code/logger"
)

// Scanner detects modules and builds the dependency graph for a project.
type Scanner struct {
	db       *sql.DB
	registry *Registry
}

// NewScanner creates a new codebase scanner.
func NewScanner(db *sql.DB) *Scanner {
	return &Scanner{
		db:       db,
		registry: NewRegistry(),
	}
}

// ScanModules detects all modules in the project and stores them in the DB.
// Respects .gitignore, global git ignore, and .quintignore.
func (s *Scanner) ScanModules(ctx context.Context, projectRoot string) ([]Module, error) {
	scanStart := time.Now()
	ignoreChecker := NewIgnoreChecker(projectRoot)

	var allModules []Module

	for _, detector := range s.registry.Detectors() {
		modules, err := detector.DetectModules(projectRoot)
		if err != nil {
			continue // skip languages that fail
		}
		// Filter out ignored modules
		for _, m := range modules {
			if m.Path != "" && ignoreChecker.IsIgnored(m.Path) {
				continue
			}
			allModules = append(allModules, m)
		}
	}

	if len(allModules) == 0 {
		return nil, nil
	}

	// Store in DB
	now := time.Now().UTC().Format(time.RFC3339)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Clear existing modules and rebuild
	if _, err := tx.ExecContext(ctx, `DELETE FROM codebase_modules`); err != nil {
		return nil, fmt.Errorf("clear modules: %w", err)
	}

	for _, m := range allModules {
		_, err := tx.ExecContext(ctx,
			`INSERT OR REPLACE INTO codebase_modules (module_id, path, name, lang, file_count, last_scanned)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			m.ID, m.Path, m.Name, m.Lang, m.FileCount, now)
		if err != nil {
			return nil, fmt.Errorf("insert module %s: %w", m.ID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit modules: %w", err)
	}

	logger.CodebaseOp("scan_modules", len(allModules), time.Since(scanStart).Milliseconds())

	return allModules, nil
}

// ScanDependencies parses imports across all modules and builds the dependency graph.
func (s *Scanner) ScanDependencies(ctx context.Context, projectRoot string) ([]ImportEdge, error) {
	scanStart := time.Now()
	ignoreChecker := NewIgnoreChecker(projectRoot)

	// Get all known modules for import resolution
	modules, err := s.GetModules(ctx)
	if err != nil {
		return nil, err
	}
	moduleIDs := make(map[string]bool)
	for _, m := range modules {
		moduleIDs[m.ID] = true
	}

	var allEdges []ImportEdge

	// Walk all source files and parse imports
	err = filepath.WalkDir(projectRoot, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() {
			if IsExcludedDir(d.Name()) {
				return filepath.SkipDir
			}
			// Check .gitignore
			if relDir, err := filepath.Rel(projectRoot, path); err == nil && ignoreChecker.IsIgnored(relDir) {
				return filepath.SkipDir
			}
			return nil
		}

		parser := s.registry.ParserForFile(path)
		if parser == nil {
			return nil
		}

		edges, err := parser.ParseImports(path, projectRoot)
		if err != nil {
			return nil
		}

		// Filter: only keep edges where target is a known local module
		for _, e := range edges {
			if moduleIDs[e.TargetModule] {
				allEdges = append(allEdges, e)
			}
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk for imports: %w", err)
	}

	// Deduplicate edges
	allEdges = deduplicateEdges(allEdges)

	// Store in DB
	now := time.Now().UTC().Format(time.RFC3339)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM module_dependencies`); err != nil {
		return nil, fmt.Errorf("clear deps: %w", err)
	}

	for _, e := range allEdges {
		_, err := tx.ExecContext(ctx,
			`INSERT OR REPLACE INTO module_dependencies (source_module, target_module, dep_type, file_path, last_scanned)
			 VALUES (?, ?, 'import', ?, ?)`,
			e.SourceModule, e.TargetModule, e.SourceFile, now)
		if err != nil {
			continue // skip constraint violations
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit deps: %w", err)
	}

	logger.CodebaseOp("scan_dependencies", len(allEdges), time.Since(scanStart).Milliseconds())

	return allEdges, nil
}

// GetModules returns all stored modules.
func (s *Scanner) GetModules(ctx context.Context) ([]Module, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT module_id, path, name, lang, file_count FROM codebase_modules ORDER BY path`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var modules []Module
	for rows.Next() {
		var m Module
		if err := rows.Scan(&m.ID, &m.Path, &m.Name, &m.Lang, &m.FileCount); err != nil {
			return nil, err
		}
		modules = append(modules, m)
	}
	return modules, rows.Err()
}

// GetDependents returns modules that depend on the given module (1-hop).
func (s *Scanner) GetDependents(ctx context.Context, moduleID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT source_module FROM module_dependencies WHERE target_module = ?`, moduleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deps []string
	for rows.Next() {
		var dep string
		if err := rows.Scan(&dep); err != nil {
			return nil, err
		}
		deps = append(deps, dep)
	}
	return deps, rows.Err()
}

// ResolveFileToModule finds the most specific module for a file path (longest prefix match).
func (s *Scanner) ResolveFileToModule(ctx context.Context, filePath string) (string, error) {
	modules, err := s.GetModules(ctx)
	if err != nil {
		return "", err
	}

	bestMatch := ""
	bestLen := -1
	for _, m := range modules {
		prefix := m.Path
		if prefix == "" {
			// Root module matches everything
			if bestLen < 0 {
				bestMatch = m.ID
				bestLen = 0
			}
			continue
		}
		if strings.HasPrefix(filePath, prefix+"/") || strings.HasPrefix(filePath, prefix+string(filepath.Separator)) {
			if len(prefix) > bestLen {
				bestMatch = m.ID
				bestLen = len(prefix)
			}
		}
	}

	return bestMatch, nil
}

// ModulesLastScanned returns the time of the last module scan, or zero if never scanned.
func (s *Scanner) ModulesLastScanned(ctx context.Context) time.Time {
	var ts string
	err := s.db.QueryRowContext(ctx,
		`SELECT MAX(last_scanned) FROM codebase_modules`).Scan(&ts)
	if err != nil || ts == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return time.Time{}
	}
	return t
}

func deduplicateEdges(edges []ImportEdge) []ImportEdge {
	seen := make(map[string]bool)
	var result []ImportEdge
	for _, e := range edges {
		key := e.SourceModule + "->" + e.TargetModule
		if !seen[key] {
			seen[key] = true
			result = append(result, e)
		}
	}
	return result
}
