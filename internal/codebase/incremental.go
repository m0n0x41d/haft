package codebase

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
)

const (
	CodeFileIndexed  = "indexed"
	CodeFileEmpty    = "empty"
	CodeFileDegraded = "degraded"
	CodeFileSkipped  = "skipped"
)

type CodeFileState struct {
	FilePath    string
	ContentHash string
	Language    string
	ParseStatus string
	SymbolCount int64
	IndexEpoch  int64
}

type CodeImport struct {
	SourceFile string
	TargetBase string
	Kind       string
}

type IndexDelta struct {
	Added         []string
	Modified      []string
	Deleted       []string
	Reindex       []string
	FullRebuild   bool
	ConfigChanged bool
}

type IndexRefreshResult struct {
	Published    bool
	Degraded     bool
	FullRebuild  bool
	Epoch        int64
	ChangedFiles int
	Reason       string
}

type IndexState struct {
	Epoch          int64
	Degraded       bool
	DegradedReason string
	Basis          IndexBasisSnapshot
}

func (s IndexState) SupportsKnownAbsence() bool {
	return !s.Degraded && s.Basis.SupportsKnownAbsence()
}

// SameCurrentBasis reports whether two observations can support one public
// result without mixing published or degraded currentness states.
func (s IndexState) SameCurrentBasis(other IndexState) bool {
	return s.Epoch == other.Epoch &&
		s.Degraded == other.Degraded &&
		s.DegradedReason == other.DegradedReason &&
		s.Basis.BasisDigest == other.Basis.BasisDigest &&
		s.Basis.CorpusDigest == other.Basis.CorpusDigest &&
		s.Basis.Coverage.Posture == other.Basis.Coverage.Posture
}

const incrementalIndexSchema = `
CREATE TABLE IF NOT EXISTS code_files (
  file_path TEXT PRIMARY KEY,
  content_hash TEXT NOT NULL,
  language TEXT NOT NULL DEFAULT '',
  parse_status TEXT NOT NULL,
  symbol_count INTEGER NOT NULL DEFAULT 0,
  index_epoch INTEGER NOT NULL,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS code_imports (
  source_file TEXT NOT NULL,
  target_base TEXT NOT NULL,
  import_kind TEXT NOT NULL DEFAULT 'import',
  index_epoch INTEGER NOT NULL,
  PRIMARY KEY (source_file, target_base, import_kind)
);
CREATE INDEX IF NOT EXISTS idx_code_imports_target ON code_imports(target_base);
CREATE TABLE IF NOT EXISTS code_index_epochs (
  epoch INTEGER PRIMARY KEY,
  status TEXT NOT NULL,
  corpus_digest TEXT NOT NULL DEFAULT '',
  basis_digest TEXT NOT NULL DEFAULT '',
  coverage_posture TEXT NOT NULL DEFAULT 'legacy_unknown',
  discovered_files INTEGER NOT NULL DEFAULT 0,
  admitted_files INTEGER NOT NULL DEFAULT 0,
  indexed_files INTEGER NOT NULL DEFAULT 0,
  empty_files INTEGER NOT NULL DEFAULT 0,
  skipped_files INTEGER NOT NULL DEFAULT 0,
  full_rebuild INTEGER NOT NULL DEFAULT 0,
  changed_files INTEGER NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS code_index_exclusions (
  epoch INTEGER NOT NULL,
  file_path TEXT NOT NULL,
  reason TEXT NOT NULL,
  observed_bytes INTEGER NOT NULL DEFAULT 0,
  limit_bytes INTEGER NOT NULL DEFAULT 0,
  detail TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (epoch, file_path),
  FOREIGN KEY (epoch) REFERENCES code_index_epochs(epoch) ON DELETE CASCADE
);`

func (s *Scanner) EnsureIncrementalSchema(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, incrementalIndexSchema); err != nil {
		return fmt.Errorf("ensure incremental index schema: %w", err)
	}
	if err := s.EnsureIndexMetaSchema(ctx); err != nil {
		return err
	}
	columns := []struct {
		name       string
		definition string
	}{
		{name: "current_epoch", definition: "INTEGER NOT NULL DEFAULT 0"},
		{name: "config_hash", definition: "TEXT NOT NULL DEFAULT ''"},
		{name: "schema_version", definition: "INTEGER NOT NULL DEFAULT 0"},
		{name: "degraded", definition: "INTEGER NOT NULL DEFAULT 0"},
		{name: "degraded_reason", definition: "TEXT NOT NULL DEFAULT ''"},
	}
	for _, column := range columns {
		if err := ensureSQLiteColumn(ctx, s.db, "code_index_meta", column.name, column.definition); err != nil {
			return err
		}
	}
	if err := ensureSQLiteColumn(
		ctx,
		s.db,
		"code_files",
		"symbol_count",
		"INTEGER NOT NULL DEFAULT 0",
	); err != nil {
		return err
	}
	epochColumns := []struct {
		name       string
		definition string
	}{
		{name: "corpus_digest", definition: "TEXT NOT NULL DEFAULT ''"},
		{name: "basis_digest", definition: "TEXT NOT NULL DEFAULT ''"},
		{name: "coverage_posture", definition: "TEXT NOT NULL DEFAULT 'legacy_unknown'"},
		{name: "discovered_files", definition: "INTEGER NOT NULL DEFAULT 0"},
		{name: "admitted_files", definition: "INTEGER NOT NULL DEFAULT 0"},
		{name: "indexed_files", definition: "INTEGER NOT NULL DEFAULT 0"},
		{name: "empty_files", definition: "INTEGER NOT NULL DEFAULT 0"},
		{name: "skipped_files", definition: "INTEGER NOT NULL DEFAULT 0"},
	}
	for _, column := range epochColumns {
		if err := ensureSQLiteColumn(
			ctx,
			s.db,
			"code_index_epochs",
			column.name,
			column.definition,
		); err != nil {
			return err
		}
	}
	return nil
}

func (s *Scanner) RefreshIncremental(ctx context.Context, projectRoot string) (IndexRefreshResult, error) {
	if err := s.EnsureIncrementalSchema(ctx); err != nil {
		return IndexRefreshResult{}, err
	}
	if err := NewSymbolStore(s.db).EnsureSchema(ctx); err != nil {
		return IndexRefreshResult{}, err
	}
	if err := NewEdgeStore(s.db).EnsureSchema(ctx); err != nil {
		return IndexRefreshResult{}, err
	}
	meta, err := s.incrementalMeta(ctx)
	if err != nil {
		return IndexRefreshResult{}, err
	}
	corpus, err := s.scanCurrentCodeCorpus(projectRoot)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return IndexRefreshResult{}, err
		}
		reason := err.Error()
		if markErr := s.markIndexDegraded(ctx, reason); markErr != nil {
			return IndexRefreshResult{}, errors.Join(err, markErr)
		}
		return IndexRefreshResult{
			Degraded: true,
			Epoch:    meta.epoch,
			Reason:   reason,
		}, nil
	}
	current := corpus.states
	stored, err := s.storedCodeFiles(ctx)
	if err != nil {
		return IndexRefreshResult{}, err
	}
	publishedState, err := s.CurrentIndexState(ctx)
	if err != nil {
		return IndexRefreshResult{}, err
	}
	configHash, err := projectConfigHash(projectRoot)
	if err != nil {
		return IndexRefreshResult{}, err
	}
	configChanged := meta.configHash != configHash
	basisUnavailable := meta.epoch > 0 &&
		publishedState.Basis.BasisDigest == ""
	schemaChanged := meta.schemaVersion != CodeIndexSchemaVersion ||
		basisUnavailable
	preliminary := computeIndexDelta(stored, current, nil, configChanged, schemaChanged)
	if len(preliminary.Added) == 0 && len(preliminary.Modified) == 0 && len(preliminary.Deleted) == 0 && !preliminary.FullRebuild {
		if err := s.clearIndexDegraded(ctx); err != nil {
			return IndexRefreshResult{}, err
		}
		return IndexRefreshResult{Epoch: meta.epoch}, nil
	}
	projectResolution := buildTSProjectResolution(projectRoot)
	imports, err := s.refreshCodeImports(
		ctx,
		current,
		corpus.admissions,
		preliminary,
		projectResolution,
	)
	if err != nil {
		return IndexRefreshResult{}, err
	}
	delta := computeIndexDelta(stored, current, imports, configChanged, schemaChanged)
	if len(delta.Reindex) == 0 &&
		len(delta.Deleted) == 0 &&
		!delta.FullRebuild {
		if err := s.clearIndexDegraded(ctx); err != nil {
			return IndexRefreshResult{}, err
		}
		return IndexRefreshResult{Epoch: meta.epoch}, nil
	}
	epoch := meta.epoch + 1
	parsed, parseErr := s.parseIndexFiles(
		delta.Reindex,
		current,
		corpus.admissions,
		corpus.dispositions,
		epoch,
	)
	if parseErr != nil {
		reason := parseErr.Error()
		_ = s.markIndexDegraded(ctx, reason)
		return IndexRefreshResult{Degraded: true, Epoch: meta.epoch, Reason: reason}, nil
	}
	baseSymbols := []CodeSymbol{}
	if !delta.FullRebuild {
		baseSymbols, err = NewSymbolStore(s.db).AllSymbols(ctx)
		if err != nil {
			return IndexRefreshResult{}, err
		}
	}
	view := newMemorySymbolView(mergeIndexSymbols(baseSymbols, parsed.symbols, delta.Reindex, delta.Deleted))
	projectSnapshotFactory := s.projectSnapshotFactory
	if projectSnapshotFactory == nil {
		projectSnapshotFactory = newProjectIndexSnapshot
	}
	projectSnapshot := s.prepareProjectIndexSnapshot(
		projectRoot,
		corpus.admissions,
		delta,
		projectResolution,
		projectSnapshotFactory,
	)
	resolutionFiles := admittedIndexPaths(
		delta.Reindex,
		corpus.admissions,
	)
	resolved, resolveErr := s.resolveIndexFiles(
		ctx,
		projectRoot,
		resolutionFiles,
		corpus.admissions,
		view,
		projectSnapshot,
		epoch,
	)
	if resolveErr != nil {
		reason := resolveErr.Error()
		_ = s.markIndexDegraded(ctx, reason)
		return IndexRefreshResult{Degraded: true, Epoch: meta.epoch, Reason: reason}, nil
	}
	fingerprint, err := s.SourceFingerprint(projectRoot)
	if err != nil {
		return IndexRefreshResult{}, err
	}
	dispositions, err := candidateFileDispositions(
		current,
		stored,
		parsed.dispositions,
		corpus.dispositions,
	)
	if err != nil {
		return IndexRefreshResult{}, err
	}
	candidate, err := BuildIndexEpochCandidate(
		epoch,
		current,
		corpus.admissions,
		dispositions,
		corpus.exclusions,
		s.indexBudget,
	)
	if err != nil {
		return IndexRefreshResult{}, err
	}
	if err := s.publishIndexEpoch(
		ctx,
		candidate,
		delta,
		current,
		imports,
		parsed,
		resolved,
		fingerprint,
		configHash,
	); err != nil {
		_ = s.markIndexDegraded(ctx, err.Error())
		return IndexRefreshResult{}, err
	}
	s.projectSnapshot = projectSnapshot
	s.projectSnapshotRoot = projectRoot
	return IndexRefreshResult{
		Published:    true,
		FullRebuild:  delta.FullRebuild,
		Epoch:        epoch,
		ChangedFiles: len(delta.Reindex) + len(delta.Deleted),
	}, nil
}

type incrementalMetaState struct {
	epoch         int64
	configHash    string
	schemaVersion int
}

func (s *Scanner) incrementalMeta(ctx context.Context) (incrementalMetaState, error) {
	var state incrementalMetaState
	err := s.db.QueryRowContext(ctx, `
		SELECT current_epoch, config_hash, schema_version
		FROM code_index_meta WHERE id = 1`).Scan(&state.epoch, &state.configHash, &state.schemaVersion)
	if err == sql.ErrNoRows {
		return state, nil
	}
	return state, err
}

func (s *Scanner) CurrentIndexState(ctx context.Context) (IndexState, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return IndexState{}, err
	}
	defer func() { _ = tx.Rollback() }()
	var state IndexState
	var degraded int
	err = tx.QueryRowContext(ctx, `
		SELECT current_epoch, degraded, degraded_reason
		FROM code_index_meta WHERE id = 1`).Scan(&state.Epoch, &degraded, &state.DegradedReason)
	if err == sql.ErrNoRows {
		state.Basis = unavailableIndexBasis()
		if err := tx.Commit(); err != nil {
			return IndexState{}, err
		}
		return state, nil
	}
	if err != nil {
		return IndexState{}, err
	}
	state.Degraded = degraded != 0
	state.Basis, err = readIndexBasis(ctx, tx, state.Epoch)
	if err != nil {
		return IndexState{}, err
	}
	if err := tx.Commit(); err != nil {
		return IndexState{}, err
	}
	return state, nil
}

func readIndexBasis(
	ctx context.Context,
	tx *sql.Tx,
	epoch int64,
) (IndexBasisSnapshot, error) {
	if epoch == 0 {
		return unavailableIndexBasis(), nil
	}
	basis := IndexBasisSnapshot{Epoch: epoch}
	err := tx.QueryRowContext(ctx, `
		SELECT corpus_digest, basis_digest, coverage_posture,
		       discovered_files, admitted_files, indexed_files,
		       empty_files, skipped_files
		FROM code_index_epochs
		WHERE epoch = ? AND status = 'complete'`,
		epoch,
	).Scan(
		&basis.CorpusDigest,
		&basis.BasisDigest,
		&basis.Coverage.Posture,
		&basis.Coverage.DiscoveredFiles,
		&basis.Coverage.AdmittedFiles,
		&basis.Coverage.IndexedFiles,
		&basis.Coverage.EmptyFiles,
		&basis.Coverage.SkippedFiles,
	)
	if err == sql.ErrNoRows {
		return legacyIndexBasis(epoch), nil
	}
	if err != nil {
		return IndexBasisSnapshot{}, err
	}
	basis.Coverage.KnownAbsenceSupported =
		basis.Coverage.Posture == IndexCoverageComplete
	rows, err := tx.QueryContext(ctx, `
		SELECT file_path, reason, observed_bytes, limit_bytes, detail
		FROM code_index_exclusions
		WHERE epoch = ?
		ORDER BY file_path`,
		epoch,
	)
	if err != nil {
		return IndexBasisSnapshot{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var exclusion IndexExclusionSnapshot
		if err := rows.Scan(
			&exclusion.Path,
			&exclusion.Reason,
			&exclusion.ObservedBytes,
			&exclusion.LimitBytes,
			&exclusion.Detail,
		); err != nil {
			return IndexBasisSnapshot{}, err
		}
		basis.Exclusions = append(basis.Exclusions, exclusion)
	}
	if err := rows.Err(); err != nil {
		return IndexBasisSnapshot{}, err
	}
	if int64(len(basis.Exclusions)) !=
		basis.Coverage.SkippedFiles {
		return IndexBasisSnapshot{}, fmt.Errorf(
			"epoch %d exclusion ledger mismatch",
			epoch,
		)
	}
	return basis, nil
}

func unavailableIndexBasis() IndexBasisSnapshot {
	return IndexBasisSnapshot{
		Coverage: IndexCoverageSnapshot{
			Posture: IndexCoverageUnavailable,
		},
	}
}

func legacyIndexBasis(epoch int64) IndexBasisSnapshot {
	return IndexBasisSnapshot{
		Epoch: epoch,
		Coverage: IndexCoverageSnapshot{
			Posture: IndexCoverageLegacyUnknown,
		},
	}
}

type scannedCodeCorpus struct {
	states       map[string]CodeFileState
	admissions   map[string]AdmittedSource
	dispositions map[string]FileIndexDisposition
	exclusions   map[string]SourceSkipInfo
}

func (s *Scanner) scanCurrentCodeCorpus(
	projectRoot string,
) (scannedCodeCorpus, error) {
	corpus := scannedCodeCorpus{
		states:       map[string]CodeFileState{},
		admissions:   map[string]AdmittedSource{},
		dispositions: map[string]FileIndexDisposition{},
		exclusions:   map[string]SourceSkipInfo{},
	}
	usage := EmptyAdmissionUsage()
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
			s.indexBudget,
			usage,
		)
		if err != nil {
			return err
		}
		usage = nextUsage
		language, _ := s.registry.SymbolLanguageForFile(relPath)
		canonicalPath := filepath.ToSlash(relPath)
		if admission.Kind().String() == "source_skipped" {
			info, err := SkippedSourceInfo(admission)
			if err != nil {
				return err
			}
			if info.RequiresRetry() {
				return fmt.Errorf(
					"observe %s: %s",
					canonicalPath,
					info.Detail,
				)
			}
			reason, err := ParseSourceSkipReason(info.Reason)
			if err != nil {
				return err
			}
			disposition, err := NewSkippedFileDisposition(reason)
			if err != nil {
				return err
			}
			corpus.dispositions[canonicalPath] = disposition
			corpus.exclusions[canonicalPath] = info
			corpus.states[canonicalPath] = CodeFileState{
				FilePath:    canonicalPath,
				ContentHash: skippedSourceStateHash(info),
				Language:    language,
				ParseStatus: disposition.StatusCode(),
			}
			return nil
		}
		source, err := AdmittedSourceFrom(admission)
		if err != nil {
			return err
		}
		corpus.admissions[canonicalPath] = source
		corpus.states[canonicalPath] = CodeFileState{
			FilePath:    canonicalPath,
			ContentHash: source.Digest(),
			Language:    language,
		}
		return nil
	})
	return corpus, err
}

func skippedSourceStateHash(info SourceSkipInfo) string {
	payload := fmt.Sprintf(
		"%s\x00%s\x00%d\x00%d",
		info.Path,
		info.Reason,
		info.ObservedBytes,
		info.LimitBytes,
	)
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

func (s *Scanner) storedCodeFiles(ctx context.Context) (map[string]CodeFileState, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT file_path, content_hash, language, parse_status,
		       symbol_count, index_epoch
		FROM code_files`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	states := map[string]CodeFileState{}
	for rows.Next() {
		var state CodeFileState
		if err := rows.Scan(
			&state.FilePath,
			&state.ContentHash,
			&state.Language,
			&state.ParseStatus,
			&state.SymbolCount,
			&state.IndexEpoch,
		); err != nil {
			return nil, err
		}
		states[state.FilePath] = state
	}
	return states, rows.Err()
}

func (s *Scanner) storedCodeImports(ctx context.Context) ([]CodeImport, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT source_file, target_base, import_kind
		FROM code_imports
		ORDER BY source_file, target_base, import_kind`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	imports := make([]CodeImport, 0)
	for rows.Next() {
		var item CodeImport
		if err := rows.Scan(&item.SourceFile, &item.TargetBase, &item.Kind); err != nil {
			return nil, err
		}
		imports = append(imports, item)
	}
	return imports, rows.Err()
}

func (s *Scanner) refreshCodeImports(
	ctx context.Context,
	current map[string]CodeFileState,
	admissions map[string]AdmittedSource,
	delta IndexDelta,
	resolution tsProjectResolution,
) ([]CodeImport, error) {
	if delta.FullRebuild {
		return scanCodeImports(admissions, resolution), nil
	}
	stored, err := s.storedCodeImports(ctx)
	if err != nil {
		return nil, err
	}
	refreshPaths := append([]string{}, delta.Added...)
	refreshPaths = append(refreshPaths, delta.Modified...)
	refreshed := scanCodeImports(
		selectAdmittedSources(admissions, refreshPaths),
		resolution,
	)
	return mergeCodeImports(stored, refreshed, refreshPaths, delta.Deleted), nil
}

func selectAdmittedSources(
	sources map[string]AdmittedSource,
	paths []string,
) map[string]AdmittedSource {
	selected := make(map[string]AdmittedSource, len(paths))
	for _, path := range paths {
		source, exists := sources[path]
		if exists {
			selected[path] = source
		}
	}
	return selected
}

func admittedIndexPaths(
	paths []string,
	sources map[string]AdmittedSource,
) []string {
	admitted := make([]string, 0, len(paths))
	for _, path := range paths {
		if _, exists := sources[path]; exists {
			admitted = append(admitted, path)
		}
	}
	return admitted
}

func mergeCodeImports(stored, refreshed []CodeImport, refreshPaths, deleted []string) []CodeImport {
	drop := make(map[string]bool, len(refreshPaths)+len(deleted))
	for _, path := range refreshPaths {
		drop[path] = true
	}
	for _, path := range deleted {
		drop[path] = true
	}
	merged := make([]CodeImport, 0, len(stored)+len(refreshed))
	for _, item := range stored {
		if !drop[item.SourceFile] {
			merged = append(merged, item)
		}
	}
	merged = append(merged, refreshed...)
	sort.Slice(merged, func(i, j int) bool {
		left := merged[i].SourceFile + "\x00" + merged[i].TargetBase + "\x00" + merged[i].Kind
		right := merged[j].SourceFile + "\x00" + merged[j].TargetBase + "\x00" + merged[j].Kind
		return left < right
	})
	return merged
}

func computeIndexDelta(
	stored map[string]CodeFileState,
	current map[string]CodeFileState,
	imports []CodeImport,
	configChanged bool,
	schemaChanged bool,
) IndexDelta {
	delta := IndexDelta{ConfigChanged: configChanged, FullRebuild: configChanged || schemaChanged || len(stored) == 0}
	for path, state := range current {
		previous, exists := stored[path]
		switch {
		case !exists:
			delta.Added = append(delta.Added, path)
		case previous.ContentHash != state.ContentHash:
			delta.Modified = append(delta.Modified, path)
		}
	}
	for path := range stored {
		if _, exists := current[path]; !exists {
			delta.Deleted = append(delta.Deleted, path)
		}
	}
	if delta.FullRebuild {
		for path := range current {
			delta.Reindex = append(delta.Reindex, path)
		}
	} else {
		delta.Reindex = incrementalReindexClosure(current, imports, append(append([]string{}, delta.Added...), delta.Modified...), delta.Deleted)
	}
	sort.Strings(delta.Added)
	sort.Strings(delta.Modified)
	sort.Strings(delta.Deleted)
	sort.Strings(delta.Reindex)
	return delta
}

func (s *Scanner) prepareProjectIndexSnapshot(
	projectRoot string,
	admissions map[string]AdmittedSource,
	delta IndexDelta,
	resolution tsProjectResolution,
	factory func(map[string]AdmittedSource, tsProjectResolution) *projectIndexSnapshot,
) *projectIndexSnapshot {
	if delta.FullRebuild ||
		s.projectSnapshot == nil ||
		s.projectSnapshotRoot != projectRoot {
		return factory(admissions, resolution)
	}
	return updateProjectIndexSnapshot(
		s.projectSnapshot,
		admissions,
		delta.Reindex,
		delta.Deleted,
		resolution,
	)
}

func incrementalReindexClosure(current map[string]CodeFileState, imports []CodeImport, changed, deleted []string) []string {
	selected := map[string]bool{}
	queue := append(append([]string{}, changed...), deleted...)
	reverse := map[string][]string{}
	for _, item := range imports {
		reverse[moduleBase(item.TargetBase)] = append(reverse[moduleBase(item.TargetBase)], item.SourceFile)
	}
	for len(queue) > 0 {
		path := queue[0]
		queue = queue[1:]
		base := moduleBase(path)
		for _, importer := range append(reverse[base], reverse[strings.TrimSuffix(base, "/index")]...) {
			if !selected[importer] {
				selected[importer] = true
				queue = append(queue, importer)
			}
		}
		dir := filepath.Dir(path)
		for candidate := range current {
			if filepath.Dir(candidate) == dir && !selected[candidate] {
				selected[candidate] = true
			}
		}
	}
	for _, path := range changed {
		selected[path] = true
	}
	out := make([]string, 0, len(selected))
	for path := range selected {
		if _, exists := current[path]; exists {
			out = append(out, path)
		}
	}
	return out
}

type parsedIndexBatch struct {
	symbols      map[string][]CodeSymbol
	dispositions map[string]FileIndexDisposition
}

func candidateFileDispositions(
	current map[string]CodeFileState,
	stored map[string]CodeFileState,
	parsed map[string]FileIndexDisposition,
	preparse map[string]FileIndexDisposition,
) (map[string]FileIndexDisposition, error) {
	dispositions := make(
		map[string]FileIndexDisposition,
		len(current),
	)
	for path := range current {
		if disposition, found := preparse[path]; found {
			dispositions[path] = disposition
			continue
		}
		if disposition, found := parsed[path]; found {
			dispositions[path] = disposition
			continue
		}
		storedState, found := stored[path]
		if !found {
			return nil, fmt.Errorf(
				"candidate file %s has no new or stored disposition",
				path,
			)
		}
		disposition, err := ParsePersistedFileIndexDisposition(
			storedState.ParseStatus,
			storedState.SymbolCount,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"restore disposition for %s: %w",
				path,
				err,
			)
		}
		dispositions[path] = disposition
	}
	return dispositions, nil
}

func (s *Scanner) parseIndexFiles(
	files []string,
	current map[string]CodeFileState,
	admissions map[string]AdmittedSource,
	preparseDispositions map[string]FileIndexDisposition,
	epoch int64,
) (parsedIndexBatch, error) {
	batch := parsedIndexBatch{
		symbols:      map[string][]CodeSymbol{},
		dispositions: map[string]FileIndexDisposition{},
	}
	for _, path := range files {
		source, admitted := admissions[path]
		if !admitted {
			disposition, found := preparseDispositions[path]
			if !found {
				return parsedIndexBatch{}, fmt.Errorf(
					"file %s has neither admission nor skip disposition",
					path,
				)
			}
			batch.dispositions[path] = disposition
			continue
		}
		if filepath.Ext(path) == ".vue" {
			status := InspectVueAdmittedParse(source)
			if status.Status == VueParseDegraded {
				failure, err := NewFileIndexFailure(path, status.Reason)
				if err != nil {
					return parsedIndexBatch{}, err
				}
				return parsedIndexBatch{}, failure
			}
		}
		snapshots, err := s.registry.ExtractAdmittedSymbolSnapshots(source)
		if err != nil {
			failure, failureErr := NewFileIndexFailure(
				path,
				err.Error(),
			)
			if failureErr != nil {
				return parsedIndexBatch{}, failureErr
			}
			return parsedIndexBatch{}, failure
		}
		language := current[path].Language
		for _, snapshot := range snapshots {
			symbol := codeSymbolFromSnapshot(snapshot, language)
			symbol.IndexEpoch = epoch
			batch.symbols[path] = append(batch.symbols[path], symbol)
		}
		if len(snapshots) == 0 {
			batch.dispositions[path] = NewEmptyFileDisposition()
			continue
		}
		symbolCount, err := NewFileCount(int64(len(snapshots)))
		if err != nil {
			return parsedIndexBatch{}, err
		}
		disposition, err := NewIndexedFileDisposition(symbolCount)
		if err != nil {
			return parsedIndexBatch{}, err
		}
		batch.dispositions[path] = disposition
	}
	return batch, nil
}

type resolvedIndexBatch struct {
	outcomes map[string][]EdgeResolution
}

// indexPublicationStage is the closed failure-injection and audit boundary for
// one candidate transaction. No stage is a separately visible index state.
type indexPublicationStage struct {
	code string
}

var indexPublicationStages = map[string]indexPublicationStage{
	"after_symbols":         {code: "after_symbols"},
	"after_search_rows":     {code: "after_search_rows"},
	"after_edges":           {code: "after_edges"},
	"after_candidate_rows":  {code: "after_candidate_rows"},
	"after_basis_rows":      {code: "after_basis_rows"},
	"after_current_pointer": {code: "after_current_pointer"},
}

func (s indexPublicationStage) String() string {
	return s.code
}

type indexResolveJob struct {
	position int
	path     string
}

type indexResolveResult struct {
	position int
	path     string
	outcomes []EdgeResolution
	err      error
}

func (s *Scanner) resolveIndexFiles(
	ctx context.Context,
	projectRoot string,
	files []string,
	admissions map[string]AdmittedSource,
	view SymbolView,
	projectSnapshot *projectIndexSnapshot,
	epoch int64,
) (resolvedIndexBatch, error) {
	batch := resolvedIndexBatch{outcomes: map[string][]EdgeResolution{}}
	if len(files) == 0 {
		return batch, nil
	}
	jobs := make(chan indexResolveJob, len(files))
	results := make(chan indexResolveResult, len(files))
	workerCount := indexResolveWorkerCount(
		len(files),
		s.indexBudget.MaxParseWorkers(),
	)
	for worker := 0; worker < workerCount; worker++ {
		go s.resolveIndexWorker(
			ctx,
			projectRoot,
			admissions,
			view,
			projectSnapshot,
			jobs,
			results,
		)
	}
	for position, path := range files {
		jobs <- indexResolveJob{position: position, path: path}
	}
	close(jobs)
	ordered := make([]indexResolveResult, len(files))
	for range files {
		result := <-results
		ordered[result.position] = result
	}
	for _, result := range ordered {
		if result.err != nil {
			return resolvedIndexBatch{}, fmt.Errorf("resolve %s: %w", result.path, result.err)
		}
		batch.outcomes[result.path] = edgeOutcomesAtEpoch(result.outcomes, epoch)
	}
	return batch, nil
}

func indexResolveWorkerCount(
	fileCount int,
	maxWorkers WorkerCount,
) int {
	workers := runtime.GOMAXPROCS(0)
	if workers > int(maxWorkers.Value()) {
		workers = int(maxWorkers.Value())
	}
	if workers > fileCount {
		workers = fileCount
	}
	if workers < 1 {
		return 1
	}
	return workers
}

func (s *Scanner) resolveIndexWorker(
	ctx context.Context,
	projectRoot string,
	admissions map[string]AdmittedSource,
	view SymbolView,
	projectSnapshot *projectIndexSnapshot,
	jobs <-chan indexResolveJob,
	results chan<- indexResolveResult,
) {
	for job := range jobs {
		source, exists := admissions[job.path]
		if !exists {
			results <- indexResolveResult{
				position: job.position,
				path:     job.path,
				err: fmt.Errorf(
					"resolve source %s is not admitted",
					job.path,
				),
			}
			continue
		}
		outcomes, err := s.resolveIndexFile(
			ctx,
			projectRoot,
			source,
			view,
			projectSnapshot,
		)
		results <- indexResolveResult{
			position: job.position,
			path:     job.path,
			outcomes: outcomes,
			err:      err,
		}
	}
}

func (s *Scanner) resolveIndexFile(
	ctx context.Context,
	projectRoot string,
	source AdmittedSource,
	view SymbolView,
	projectSnapshot *projectIndexSnapshot,
) ([]EdgeResolution, error) {
	path := source.Path().String()
	resolver := s.registry.ResolverForFile(path)
	if resolver == nil {
		return nil, nil
	}
	prepared, supportsPrepared := resolver.(admittedProjectSnapshotEdgeOutcomeResolver)
	if supportsPrepared && projectSnapshot != nil {
		return prepared.ResolveAdmittedFileEdgeOutcomesWithProjectSnapshot(
			ctx,
			projectRoot,
			source,
			view,
			projectSnapshot,
		)
	}
	if truthful, ok := resolver.(admittedEdgeOutcomeResolver); ok {
		return truthful.ResolveAdmittedFileEdgeOutcomes(
			ctx,
			projectRoot,
			source,
			view,
		)
	}
	if admitted, ok := resolver.(admittedEdgeResolver); ok {
		edges, err := admitted.ResolveAdmittedFileEdges(
			ctx,
			projectRoot,
			source,
			view,
		)
		if err != nil {
			return nil, err
		}
		outcomes := make([]EdgeResolution, 0, len(edges))
		for _, edge := range edges {
			outcomes = append(outcomes, ResolvedEdge{Edge: edge})
		}
		return outcomes, nil
	}
	if prepared, ok := resolver.(projectSnapshotEdgeOutcomeResolver); ok {
		return prepared.ResolveFileEdgeOutcomesWithProjectSnapshot(ctx, projectRoot, path, view, projectSnapshot)
	}
	if truthful, ok := resolver.(EdgeOutcomeResolver); ok {
		return truthful.ResolveFileEdgeOutcomes(ctx, projectRoot, path, view)
	}
	edges, err := resolver.ResolveFileEdges(ctx, projectRoot, path, view)
	if err != nil {
		return nil, err
	}
	outcomes := make([]EdgeResolution, 0, len(edges))
	for _, edge := range edges {
		outcomes = append(outcomes, ResolvedEdge{Edge: edge})
	}
	return outcomes, nil
}

func edgeOutcomesAtEpoch(outcomes []EdgeResolution, epoch int64) []EdgeResolution {
	indexed := make([]EdgeResolution, 0, len(outcomes))
	for _, outcome := range outcomes {
		resolved, ok := outcome.(ResolvedEdge)
		if ok {
			resolved.Edge.IndexEpoch = epoch
			outcome = resolved
		}
		indexed = append(indexed, outcome)
	}
	return indexed
}

func (s *Scanner) publishIndexEpoch(
	ctx context.Context,
	candidate IndexEpochCandidate,
	delta IndexDelta,
	current map[string]CodeFileState,
	imports []CodeImport,
	parsed parsedIndexBatch,
	resolved resolvedIndexBatch,
	fingerprint string,
	configHash string,
) error {
	basis := candidate.Basis()
	epoch := basis.Epoch
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if delta.FullRebuild {
		for _, statement := range []string{
			`DELETE FROM code_edges`,
			`DELETE FROM code_resolution_diagnostics`,
			`DELETE FROM code_symbol_search`,
			`DELETE FROM code_symbols`,
			`DELETE FROM code_imports`,
			`DELETE FROM code_files`,
		} {
			if _, err := tx.ExecContext(ctx, statement); err != nil {
				return err
			}
		}
	}
	touched := append(append([]string{}, delta.Reindex...), delta.Deleted...)
	for _, path := range touched {
		if delta.FullRebuild {
			continue
		}
		for _, statement := range []string{
			`DELETE FROM code_edges WHERE file_path = ?`,
			`DELETE FROM code_resolution_diagnostics WHERE file_path = ?`,
			`DELETE FROM code_symbol_search
			 WHERE symbol_id IN (
			   SELECT id FROM code_symbols WHERE file_path = ?
			 )`,
			`DELETE FROM code_symbols WHERE file_path = ?`,
			`DELETE FROM code_imports WHERE source_file = ?`,
		} {
			if _, err := tx.ExecContext(ctx, statement, path); err != nil {
				return err
			}
		}
	}
	for _, path := range delta.Deleted {
		if _, err := tx.ExecContext(ctx, `DELETE FROM code_files WHERE file_path = ?`, path); err != nil {
			return err
		}
	}
	for _, path := range delta.Reindex {
		for _, symbol := range parsed.symbols[path] {
			if err := insertIndexedSymbol(ctx, tx, symbol); err != nil {
				return err
			}
			if err := insertSymbolSearchRow(
				ctx,
				tx,
				symbol,
				epoch,
			); err != nil {
				return err
			}
		}
		if err := s.reachPublicationCheckpoint(
			ctx,
			tx,
			indexPublicationStages["after_symbols"],
		); err != nil {
			return err
		}
		if err := s.reachPublicationCheckpoint(
			ctx,
			tx,
			indexPublicationStages["after_search_rows"],
		); err != nil {
			return err
		}
		for _, outcome := range resolved.outcomes[path] {
			switch value := outcome.(type) {
			case ResolvedEdge:
				if err := insertCodeEdge(ctx, tx, value.Edge); err != nil {
					return err
				}
			case AmbiguousEdge, UnresolvedEdge:
				_, diagnostics := PartitionEdgeResolutions([]EdgeResolution{outcome})
				for _, diagnostic := range diagnostics {
					if err := insertResolutionDiagnostic(ctx, tx, diagnostic); err != nil {
						return err
					}
				}
			}
		}
		if err := s.reachPublicationCheckpoint(
			ctx,
			tx,
			indexPublicationStages["after_edges"],
		); err != nil {
			return err
		}
		state := current[path]
		disposition, found := parsed.dispositions[path]
		if !found {
			return fmt.Errorf(
				"file %s has no post-parse disposition",
				path,
			)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO code_files (
			  file_path, content_hash, language, parse_status,
			  symbol_count, index_epoch, updated_at
			)
			VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
			ON CONFLICT(file_path) DO UPDATE SET
			 content_hash=excluded.content_hash, language=excluded.language,
			 parse_status=excluded.parse_status,
			 symbol_count=excluded.symbol_count,
			 index_epoch=excluded.index_epoch,
			 updated_at=CURRENT_TIMESTAMP`,
			path,
			state.ContentHash,
			state.Language,
			disposition.StatusCode(),
			len(parsed.symbols[path]),
			epoch,
		); err != nil {
			return err
		}
	}
	selected := map[string]bool{}
	for _, path := range delta.Reindex {
		selected[path] = true
	}
	for _, item := range imports {
		if !selected[item.SourceFile] {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT OR REPLACE INTO code_imports (source_file, target_base, import_kind, index_epoch)
			VALUES (?, ?, ?, ?)`, item.SourceFile, item.TargetBase, item.Kind, epoch); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(
		ctx,
		`UPDATE code_symbol_search SET published_epoch = ?`,
		epoch,
	); err != nil {
		return err
	}
	if err := s.reachPublicationCheckpoint(
		ctx,
		tx,
		indexPublicationStages["after_candidate_rows"],
	); err != nil {
		return err
	}
	changedFiles := len(delta.Reindex) + len(delta.Deleted)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO code_index_epochs (
		  epoch, status, corpus_digest, basis_digest, coverage_posture,
		  discovered_files, admitted_files, indexed_files, empty_files,
		  skipped_files, full_rebuild, changed_files
		)
		VALUES (?, 'complete', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		epoch,
		basis.CorpusDigest,
		basis.BasisDigest,
		basis.Coverage.Posture,
		basis.Coverage.DiscoveredFiles,
		basis.Coverage.AdmittedFiles,
		basis.Coverage.IndexedFiles,
		basis.Coverage.EmptyFiles,
		basis.Coverage.SkippedFiles,
		boolToInt(delta.FullRebuild),
		changedFiles,
	); err != nil {
		return err
	}
	for _, exclusion := range basis.Exclusions {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO code_index_exclusions (
			  epoch, file_path, reason, observed_bytes, limit_bytes, detail
			)
			VALUES (?, ?, ?, ?, ?, ?)`,
			epoch,
			exclusion.Path,
			exclusion.Reason,
			exclusion.ObservedBytes,
			exclusion.LimitBytes,
			exclusion.Detail,
		); err != nil {
			return err
		}
	}
	if err := s.reachPublicationCheckpoint(
		ctx,
		tx,
		indexPublicationStages["after_basis_rows"],
	); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO code_index_meta (id, fingerprint, current_epoch, config_hash, schema_version, degraded, degraded_reason)
		VALUES (1, ?, ?, ?, ?, 0, '')
		ON CONFLICT(id) DO UPDATE SET fingerprint=excluded.fingerprint,
		 current_epoch=excluded.current_epoch, config_hash=excluded.config_hash,
		 schema_version=excluded.schema_version, degraded=0, degraded_reason=''`,
		fingerprint, epoch, configHash, CodeIndexSchemaVersion); err != nil {
		return err
	}
	if err := s.reachPublicationCheckpoint(
		ctx,
		tx,
		indexPublicationStages["after_current_pointer"],
	); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Scanner) reachPublicationCheckpoint(
	ctx context.Context,
	transaction *sql.Tx,
	stage indexPublicationStage,
) error {
	if s.publicationCheckpoint == nil {
		return nil
	}
	return s.publicationCheckpoint(ctx, transaction, stage)
}

func (s *Scanner) markIndexDegraded(ctx context.Context, reason string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO code_index_meta (id, fingerprint, degraded, degraded_reason)
		VALUES (1, '', 1, ?)
		ON CONFLICT(id) DO UPDATE SET degraded = 1, degraded_reason = excluded.degraded_reason`, reason)
	return err
}

func (s *Scanner) clearIndexDegraded(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE code_index_meta
		SET degraded = 0, degraded_reason = ''
		WHERE id = 1`)
	return err
}

func insertIndexedSymbol(ctx context.Context, tx *sql.Tx, symbol CodeSymbol) error {
	symbol = normalizeCodeSymbolIdentity(symbol)
	_, err := tx.ExecContext(ctx, `
		INSERT OR REPLACE INTO code_symbols (
		 id, anchor_id, anchor_version, file_path, name, qualified_name, signature_hash,
		 kind, receiver, start_line, end_line, start_byte, end_byte, hash, exported, lang, index_epoch
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		symbol.ID, symbol.AnchorID, symbol.AnchorVersion, symbol.FilePath, symbol.Name,
		symbol.QualifiedName, symbol.SignatureHash, symbol.Kind, symbol.Receiver,
		symbol.StartLine, symbol.EndLine, symbol.StartByte, symbol.EndByte, symbol.Hash,
		boolToInt(symbol.Exported), symbol.Lang, symbol.IndexEpoch,
	)
	return err
}

func (s *SymbolStore) AllSymbols(ctx context.Context) ([]CodeSymbol, error) {
	rows, err := s.db.QueryContext(ctx, codeSymbolSelect+` ORDER BY file_path, start_line`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCodeSymbols(rows)
}

func mergeIndexSymbols(base []CodeSymbol, parsed map[string][]CodeSymbol, reindex, deleted []string) []CodeSymbol {
	drop := map[string]bool{}
	for _, path := range append(append([]string{}, reindex...), deleted...) {
		drop[path] = true
	}
	out := make([]CodeSymbol, 0, len(base))
	for _, symbol := range base {
		if !drop[symbol.FilePath] {
			out = append(out, symbol)
		}
	}
	for _, symbols := range parsed {
		out = append(out, symbols...)
	}
	return out
}

type memorySymbolView struct {
	byFile map[string][]CodeSymbol
	byDir  map[string][]CodeSymbol
	byName map[string][]CodeSymbol
}

func newMemorySymbolView(symbols []CodeSymbol) *memorySymbolView {
	view := &memorySymbolView{byFile: map[string][]CodeSymbol{}, byDir: map[string][]CodeSymbol{}, byName: map[string][]CodeSymbol{}}
	for _, symbol := range symbols {
		view.byFile[symbol.FilePath] = append(view.byFile[symbol.FilePath], symbol)
		view.byDir[filepath.Dir(symbol.FilePath)] = append(view.byDir[filepath.Dir(symbol.FilePath)], symbol)
		view.byName[symbol.Name] = append(view.byName[symbol.Name], symbol)
	}
	return view
}

func (view *memorySymbolView) GetByFile(_ context.Context, path string) ([]CodeSymbol, error) {
	return append([]CodeSymbol{}, view.byFile[path]...), nil
}

func (view *memorySymbolView) GetByDir(_ context.Context, dir string) ([]CodeSymbol, error) {
	return append([]CodeSymbol{}, view.byDir[filepath.Clean(dir)]...), nil
}

func (view *memorySymbolView) GetByName(_ context.Context, name string) ([]CodeSymbol, error) {
	return append([]CodeSymbol{}, view.byName[name]...), nil
}

func scanCodeImports(
	sources map[string]AdmittedSource,
	resolution tsProjectResolution,
) []CodeImport {
	imports := make([]CodeImport, 0)
	for path, source := range sources {
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".ts" && ext != ".tsx" && ext != ".mts" && ext != ".cts" && ext != ".js" && ext != ".jsx" && ext != ".mjs" && ext != ".cjs" && ext != ".vue" {
			continue
		}
		content := source.bytes()
		for _, pattern := range []*regexp.Regexp{jsImportRe, jsRequireRe, jsReExportRe} {
			for _, match := range pattern.FindAllSubmatch(content, -1) {
				bases, local := resolveTSModuleSpecifiers(string(match[1]), filepath.Dir(path), resolution)
				if !local {
					continue
				}
				for _, base := range bases {
					imports = append(imports, CodeImport{SourceFile: path, TargetBase: base, Kind: "import"})
				}
			}
		}
	}
	return imports
}

func projectConfigHash(projectRoot string) (string, error) {
	names := map[string]bool{
		"go.mod": true, "go.sum": true, "go.work": true, "go.work.sum": true,
		"Cargo.toml": true, "Cargo.lock": true, "package.json": true,
		"pnpm-workspace.yaml": true, "pnpm-lock.yaml": true,
	}
	lines := make([]string, 0)
	err := walkProjectFiles(projectRoot, func(
		path string,
		rel string,
		entry os.DirEntry,
	) error {
		if !names[entry.Name()] && !strings.HasPrefix(entry.Name(), "tsconfig") {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(content)
		lines = append(lines, rel+"\x00"+hex.EncodeToString(sum[:]))
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(lines)
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(sum[:]), nil
}

func ensureSQLiteColumn(ctx context.Context, db *sql.DB, table, column, definition string) error {
	rows, err := db.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return err
	}
	found := false
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, kind string
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &kind, &notNull, &defaultValue, &primaryKey); err != nil {
			_ = rows.Close()
			return err
		}
		found = found || name == column
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if found {
		return nil
	}
	_, err = db.ExecContext(ctx, fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, definition))
	return err
}
