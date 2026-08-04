package codebase

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/m0n0x41d/haft/internal/projectpath"
	"github.com/m0n0x41d/haft/logger"
)

// Resolve concurrency is a runtime thermal policy, not part of the persisted
// index basis. Keeping it separate from IndexBudget avoids forcing a full
// rebuild merely because an interactive host lowers its active CPU fan-out.
const defaultMaxResolveWorkers = int64(2)

// Scanner detects modules and builds the dependency graph for a project.
type Scanner struct {
	db                     *sql.DB
	registry               *Registry
	indexBudget            IndexBudget
	maxResolveWorkers      WorkerCount
	projectSnapshotFactory func(map[string]AdmittedSource, tsProjectResolution) *projectIndexSnapshot
	projectSnapshot        *projectIndexSnapshot
	projectSnapshotRoot    string
	publicationCheckpoint  func(context.Context, *sql.Tx, indexPublicationStage) error
}

// NewScanner creates a new codebase scanner.
func NewScanner(db *sql.DB) *Scanner {
	maxResolveWorkers, _ := NewWorkerCount(defaultMaxResolveWorkers)
	return &Scanner{
		db:                     db,
		registry:               NewRegistry(),
		indexBudget:            DefaultIndexBudget(),
		maxResolveWorkers:      maxResolveWorkers,
		projectSnapshotFactory: newProjectIndexSnapshot,
	}
}

// ScanModules detects all modules in the project and stores them in the DB.
// Respects .gitignore, global git ignore, and .haftignore.
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
			modulePath, err := projectpath.ParseModule(m.Path)
			if err != nil {
				return nil, fmt.Errorf(
					"detector %s returned invalid module path %q: %w",
					detector.Language(),
					m.Path,
					err,
				)
			}
			m.Path = modulePath.String()
			if m.Path != "" && ignoreChecker.IsIgnored(m.Path) {
				continue
			}
			allModules = append(allModules, m)
		}
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
	usage := EmptyAdmissionUsage()

	// Walk all source files and parse imports
	err = walkProjectFiles(projectRoot, func(
		path string,
		relPath string,
		_ os.DirEntry,
	) error {
		parser := s.registry.ParserForFile(path)
		if parser == nil {
			return nil
		}
		admission, nextUsage, err := s.registry.ReadSourceAdmission(
			projectRoot,
			relPath,
			DefaultIndexBudget(),
			usage,
		)
		if err != nil {
			return err
		}
		usage = nextUsage
		if admission.Kind().String() == "source_skipped" {
			info, err := SkippedSourceInfo(admission)
			if err != nil {
				return err
			}
			if info.Reason == "read_failure" ||
				info.Reason == "source_changed" {
				return fmt.Errorf(
					"observe import source %s: %s",
					relPath,
					info.Detail,
				)
			}
			return nil
		}
		source, err := AdmittedSourceFrom(admission)
		if err != nil {
			return err
		}
		edges, err := parser.ParseImports(source, projectRoot)
		if err != nil {
			return fmt.Errorf("parse imports %s: %w", relPath, err)
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

// ScanSymbols extracts and stores code symbols for every supported source file,
// populating the code_symbols node layer of the code graph. Respects the same
// ignore/exclusion rules as ScanModules; idempotent per file (delete-then-insert).
// Files excluded by the admission policy are not indexed. Observation and parse
// failures remain visible to the caller rather than becoming false empty files.
// Per-file transactions here remain the cold path; RefreshIncremental owns
// atomic candidate-epoch publication.
func (s *Scanner) ScanSymbols(ctx context.Context, projectRoot string) (int, error) {
	scanStart := time.Now()
	store := NewSymbolStore(s.db)
	if err := store.EnsureSchema(ctx); err != nil {
		return 0, err
	}
	budget := DefaultIndexBudget()
	usage := EmptyAdmissionUsage()
	indexed := 0

	err := walkProjectFiles(projectRoot, func(
		path string,
		relPath string,
		_ os.DirEntry,
	) error {
		if !s.registry.SupportsSymbols(path) {
			return nil
		}
		admission, nextUsage, err := s.registry.ReadSourceAdmission(
			projectRoot,
			relPath,
			budget,
			usage,
		)
		if err != nil {
			return err
		}
		usage = nextUsage
		if admission.Kind().String() == "source_skipped" {
			info, err := SkippedSourceInfo(admission)
			if err != nil {
				return err
			}
			if info.RequiresRetry() {
				return fmt.Errorf(
					"observe symbol source %s: %s",
					relPath,
					info.Detail,
				)
			}
			return nil
		}
		source, err := AdmittedSourceFrom(admission)
		if err != nil {
			return err
		}
		if err := store.IndexAdmittedFileSymbolsWithRegistry(
			ctx,
			source,
			s.registry,
		); err != nil {
			return fmt.Errorf("index symbols for %s: %w", relPath, err)
		}
		indexed++
		return nil
	})
	if err != nil {
		return indexed, fmt.Errorf("walk for symbols: %w", err)
	}

	logger.CodebaseOp("scan_symbols", indexed, time.Since(scanStart).Milliseconds())
	return indexed, nil
}

// ScanEdges builds the code_edges layer for every file with a registered
// EdgeResolver, via the language-agnostic port (Go today; other languages add
// an adapter). Must run AFTER ScanSymbols — cross-file/dispatch resolution
// reads the full node store. Idempotent per file. Observation and resolver
// failures remain visible to the caller.
func (s *Scanner) ScanEdges(ctx context.Context, projectRoot string) (int, error) {
	scanStart := time.Now()
	edgeStore := NewEdgeStore(s.db)
	if err := edgeStore.EnsureSchema(ctx); err != nil {
		return 0, err
	}
	symbols := NewSymbolStore(s.db)
	budget := DefaultIndexBudget()
	usage := EmptyAdmissionUsage()
	total := 0

	err := walkProjectFiles(projectRoot, func(
		path string,
		relPath string,
		_ os.DirEntry,
	) error {
		resolver := s.registry.ResolverForFile(path)
		if resolver == nil {
			return nil // no edge resolver for this language yet
		}
		admission, nextUsage, err := s.registry.ReadSourceAdmission(
			projectRoot,
			relPath,
			budget,
			usage,
		)
		if err != nil {
			return err
		}
		usage = nextUsage
		if admission.Kind().String() == "source_skipped" {
			info, err := SkippedSourceInfo(admission)
			if err != nil {
				return err
			}
			if info.RequiresRetry() {
				return fmt.Errorf(
					"observe edge source %s: %s",
					relPath,
					info.Detail,
				)
			}
			return nil
		}
		source, err := AdmittedSourceFrom(admission)
		if err != nil {
			return err
		}
		outcomes, err := s.resolveIndexFile(
			ctx,
			projectRoot,
			source,
			symbols,
			nil,
		)
		if err != nil {
			return fmt.Errorf("resolve edges for %s: %w", relPath, err)
		}
		edges, _ := PartitionEdgeResolutions(outcomes)
		if err := edgeStore.ReplaceFileResolutions(
			ctx,
			relPath,
			outcomes,
		); err != nil {
			return err
		}
		total += len(edges)
		return nil
	})
	if err != nil {
		return total, fmt.Errorf("walk for edges: %w", err)
	}

	logger.CodebaseOp("scan_edges", total, time.Since(scanStart).Milliseconds())
	return total, nil
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
	modulePathOwners := make(map[string]string)
	for rows.Next() {
		var m Module
		if err := rows.Scan(&m.ID, &m.Path, &m.Name, &m.Lang, &m.FileCount); err != nil {
			return nil, err
		}
		modulePath, err := projectpath.ParseModule(m.Path)
		if err != nil {
			return nil, fmt.Errorf(
				"stored module %s has invalid path %q: %w",
				m.ID,
				m.Path,
				err,
			)
		}
		m.Path = modulePath.String()
		if owner, exists := modulePathOwners[m.Path]; exists &&
			owner != m.ID {
			return nil, fmt.Errorf(
				"stored module path %q has ambiguous identities %q and %q",
				m.Path,
				owner,
				m.ID,
			)
		}
		modulePathOwners[m.Path] = m.ID
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
	candidate, err := projectpath.Parse(filePath)
	if err != nil {
		return "", fmt.Errorf("resolve file to module: %w", err)
	}
	modules, err := s.GetModules(ctx)
	if err != nil {
		return "", err
	}

	refs := make([]projectpath.ModuleRef, 0, len(modules))
	for _, m := range modules {
		moduleRef, err := projectpath.NewModuleRef(m.ID, m.Path)
		if err != nil {
			return "", err
		}
		refs = append(refs, moduleRef)
	}
	resolved, ok, err := projectpath.ResolveMostSpecificModule(
		refs,
		candidate,
	)
	if err != nil || !ok {
		return "", err
	}
	return resolved.ID(), nil
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
