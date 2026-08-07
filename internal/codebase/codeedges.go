package codebase

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

// EdgeKind enumerates the code→code relationships walked by traversal. Reference
// edges are deliberately NOT here — they are far more numerous and resolve far
// weaker, so they never enter the traversal set (per the parity decision).
type EdgeKind string

const (
	EdgeCall              EdgeKind = "call"
	EdgeInterfaceDispatch EdgeKind = "interface_dispatch"
	EdgeImplements        EdgeKind = "implements"
	EdgeExtends           EdgeKind = "extends"
	EdgeEmbeds            EdgeKind = "embeds"
	EdgeInstantiates      EdgeKind = "instantiates"
	EdgeValueReference    EdgeKind = "value_reference"
	EdgeTypeReference     EdgeKind = "type_reference"
	EdgeTemplateUse       EdgeKind = "template_use"
	// EdgeCallback is a synthesized indirect-call edge: a named function passed
	// as a callback argument (`register(handler)`, `emitter.on("x", handler)`) is
	// invoked later through dynamic dispatch the AST cannot follow directly. The
	// edge wires the registration site to the handler so callback-only functions
	// are not falsely shown as having zero callers. Always heuristic provenance.
	EdgeCallback EdgeKind = "callback"
)

// Provenance records how an edge was established. static = resolved directly
// from the AST + symbol table; heuristic = synthesized (e.g. structural
// interface→impl matching), which the agent can treat with appropriate caution.
type Provenance string

const (
	ProvenanceStatic    Provenance = "static"
	ProvenanceHeuristic Provenance = "heuristic"
)

// CodeEdge is a resolved directed relationship between two symbol nodes. It
// exists ONLY when both endpoints resolved — an unresolved call is an absent
// edge, never a nullable one ("partial coverage worse than none", in the type).
type CodeEdge struct {
	SrcID              string
	DstID              string
	Kind               EdgeKind
	FilePath           string
	Line               int
	Provenance         Provenance
	Origin             EdgeOrigin
	ResolutionMethod   ResolutionMethod
	Confidence         ConfidenceClass
	ResolverVersion    string
	SourceSnapshotHash string
	IndexEpoch         int64
}

type ResolutionCounts struct {
	Resolved   int
	Ambiguous  int
	Unresolved int
}

const codeEdgesSchema = `
CREATE TABLE IF NOT EXISTS code_edges (
  src_id               TEXT NOT NULL,
  dst_id               TEXT NOT NULL,
  kind                 TEXT NOT NULL,
  file_path            TEXT NOT NULL,
  line                 INTEGER NOT NULL DEFAULT 0,
  provenance           TEXT NOT NULL,
  origin               TEXT NOT NULL DEFAULT '',
  resolution_method    TEXT NOT NULL DEFAULT '',
  confidence           TEXT NOT NULL DEFAULT '',
  resolver_version     TEXT NOT NULL DEFAULT '',
  source_snapshot_hash TEXT NOT NULL DEFAULT '',
  index_epoch          INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (src_id, dst_id, kind, file_path, line)
);
CREATE INDEX IF NOT EXISTS idx_code_edges_src ON code_edges(src_id);
CREATE INDEX IF NOT EXISTS idx_code_edges_dst ON code_edges(dst_id);
CREATE TABLE IF NOT EXISTS code_resolution_diagnostics (
  source_id            TEXT NOT NULL,
  kind                 TEXT NOT NULL,
  file_path            TEXT NOT NULL,
  line                 INTEGER NOT NULL DEFAULT 0,
  status               TEXT NOT NULL,
  reason               TEXT NOT NULL,
  candidate_ids        TEXT NOT NULL DEFAULT '[]',
  origin               TEXT NOT NULL DEFAULT '',
  resolver_version     TEXT NOT NULL DEFAULT '',
  source_snapshot_hash TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (source_id, kind, file_path, line, status, reason)
);
CREATE INDEX IF NOT EXISTS idx_code_resolution_diagnostics_file ON code_resolution_diagnostics(file_path);`

// EdgeStore persists code edges. Shell over *sql.DB; the caller owns lifecycle.
type EdgeStore struct {
	db *sql.DB
}

// NewEdgeStore creates an edge store over an existing DB connection.
func NewEdgeStore(db *sql.DB) *EdgeStore { return &EdgeStore{db: db} }

// EnsureSchema creates the code_edges table + indexes if absent (idempotent).
func (e *EdgeStore) EnsureSchema(ctx context.Context) error {
	if _, err := e.db.ExecContext(ctx, codeEdgesSchema); err != nil {
		return fmt.Errorf("ensure code_edges schema: %w", err)
	}
	columns := []struct {
		name       string
		definition string
	}{
		{name: "origin", definition: "TEXT NOT NULL DEFAULT ''"},
		{name: "resolution_method", definition: "TEXT NOT NULL DEFAULT ''"},
		{name: "confidence", definition: "TEXT NOT NULL DEFAULT ''"},
		{name: "resolver_version", definition: "TEXT NOT NULL DEFAULT ''"},
		{name: "source_snapshot_hash", definition: "TEXT NOT NULL DEFAULT ''"},
		{name: "index_epoch", definition: "INTEGER NOT NULL DEFAULT 0"},
	}
	for _, column := range columns {
		if err := e.ensureColumn(ctx, column.name, column.definition); err != nil {
			return err
		}
	}
	return nil
}

// ReplaceFileEdges idempotently rebuilds the edges originating in one file
// (delete-by-file then insert) so re-indexing a file is exact, not additive.
func (e *EdgeStore) ReplaceFileEdges(ctx context.Context, filePath string, edges []CodeEdge) error {
	tx, err := e.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM code_edges WHERE file_path = ?`, filePath); err != nil {
		return err
	}
	for _, ed := range edges {
		ed = normalizeCodeEdge(ed)
		if _, err := tx.ExecContext(ctx,
			`INSERT OR REPLACE INTO code_edges
			 (src_id, dst_id, kind, file_path, line, provenance, origin, resolution_method, confidence, resolver_version, source_snapshot_hash, index_epoch)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			ed.SrcID,
			ed.DstID,
			string(ed.Kind),
			ed.FilePath,
			ed.Line,
			string(ed.Provenance),
			string(ed.Origin),
			string(ed.ResolutionMethod),
			string(ed.Confidence),
			ed.ResolverVersion,
			ed.SourceSnapshotHash,
			ed.IndexEpoch,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

const (
	outEdgesQuery = `SELECT src_id, dst_id, kind, file_path, line, provenance, origin, resolution_method, confidence, resolver_version, source_snapshot_hash, index_epoch FROM code_edges WHERE src_id = ? ORDER BY dst_id, src_id`
	inEdgesQuery  = `SELECT src_id, dst_id, kind, file_path, line, provenance, origin, resolution_method, confidence, resolver_version, source_snapshot_hash, index_epoch FROM code_edges WHERE dst_id = ? ORDER BY dst_id, src_id`
	allEdgesQuery = `SELECT src_id, dst_id, kind, file_path, line, provenance, origin, resolution_method, confidence, resolver_version, source_snapshot_hash, index_epoch FROM code_edges ORDER BY src_id, dst_id, kind, line`
)

// AllEdges returns every code edge in one pass — the whole-graph enumeration the
// fused-graph ranker (graphrank, dec-20260604-3aaad199 phase 2) needs to build
// adjacency once instead of N per-node queries. Stable order = deterministic build.
func (e *EdgeStore) AllEdges(ctx context.Context) ([]CodeEdge, error) {
	rows, err := e.db.QueryContext(ctx, allEdgesQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEdges(rows)
}

// OutEdges returns edges where srcID is the source (its callees / dispatch targets).
func (e *EdgeStore) OutEdges(ctx context.Context, srcID string) ([]CodeEdge, error) {
	rows, err := e.db.QueryContext(ctx, outEdgesQuery, srcID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEdges(rows)
}

// InEdges returns edges where dstID is the target (its callers / dispatchers).
func (e *EdgeStore) InEdges(ctx context.Context, dstID string) ([]CodeEdge, error) {
	rows, err := e.db.QueryContext(ctx, inEdgesQuery, dstID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEdges(rows)
}

// edgePairs builds the set of "src->dst" pairs present in edges (kind-agnostic),
// used to suppress a synthesized edge a direct edge already covers.
func edgePairs(edges []CodeEdge) map[string]bool {
	out := make(map[string]bool, len(edges))
	for _, e := range edges {
		out[e.SrcID+"->"+e.DstID] = true
	}
	return out
}

func scanEdges(rows *sql.Rows) ([]CodeEdge, error) {
	var out []CodeEdge
	for rows.Next() {
		var ed CodeEdge
		var kind, prov, origin, method, confidence string
		if err := rows.Scan(
			&ed.SrcID,
			&ed.DstID,
			&kind,
			&ed.FilePath,
			&ed.Line,
			&prov,
			&origin,
			&method,
			&confidence,
			&ed.ResolverVersion,
			&ed.SourceSnapshotHash,
			&ed.IndexEpoch,
		); err != nil {
			return nil, err
		}
		ed.Kind = EdgeKind(kind)
		ed.Provenance = Provenance(prov)
		ed.Origin = EdgeOrigin(origin)
		ed.ResolutionMethod = ResolutionMethod(method)
		ed.Confidence = ConfidenceClass(confidence)
		out = append(out, ed)
	}
	return out, rows.Err()
}

// ReplaceFileResolutions publishes admitted edges and non-admitted diagnostics
// for one source file in one transaction. A query can never observe an
// ambiguous/unresolved relation as a traversal edge.
func (e *EdgeStore) ReplaceFileResolutions(ctx context.Context, filePath string, outcomes []EdgeResolution) error {
	edges, diagnostics := PartitionEdgeResolutions(outcomes)
	tx, err := e.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM code_edges WHERE file_path = ?`, filePath); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM code_resolution_diagnostics WHERE file_path = ?`, filePath); err != nil {
		return err
	}
	for _, edge := range edges {
		if err := insertCodeEdge(ctx, tx, edge); err != nil {
			return err
		}
	}
	for _, diagnostic := range diagnostics {
		if err := insertResolutionDiagnostic(ctx, tx, diagnostic); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (e *EdgeStore) DiagnosticsByFile(ctx context.Context, filePath string) ([]ResolutionDiagnostic, error) {
	rows, err := e.db.QueryContext(ctx,
		`SELECT source_id, kind, file_path, line, status, reason, candidate_ids, origin, resolver_version, source_snapshot_hash
		 FROM code_resolution_diagnostics WHERE file_path = ? ORDER BY line, source_id, kind`,
		filePath,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	diagnostics := make([]ResolutionDiagnostic, 0)
	for rows.Next() {
		var diagnostic ResolutionDiagnostic
		var kind, status, reason, candidates, origin string
		if err := rows.Scan(
			&diagnostic.SourceID,
			&kind,
			&diagnostic.FilePath,
			&diagnostic.Line,
			&status,
			&reason,
			&candidates,
			&origin,
			&diagnostic.ResolverVersion,
			&diagnostic.SourceSnapshotHash,
		); err != nil {
			return nil, err
		}
		diagnostic.Kind = EdgeKind(kind)
		diagnostic.Status = ResolutionStatus(status)
		diagnostic.Reason = ResolutionReason(reason)
		diagnostic.Origin = EdgeOrigin(origin)
		if err := json.Unmarshal([]byte(candidates), &diagnostic.CandidateIDs); err != nil {
			return nil, fmt.Errorf("decode resolution candidates: %w", err)
		}
		diagnostics = append(diagnostics, diagnostic)
	}
	return diagnostics, rows.Err()
}

// ResolutionCountsForSource reports admitted and non-admitted relation counts
// originating at one symbol. It makes graph incompleteness queryable instead of
// hiding it behind an empty traversal result.
func (e *EdgeStore) ResolutionCountsForSource(ctx context.Context, sourceID string) (ResolutionCounts, error) {
	counts := ResolutionCounts{}
	if err := e.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM code_edges WHERE src_id = ?`, sourceID).Scan(&counts.Resolved); err != nil {
		return ResolutionCounts{}, err
	}
	rows, err := e.db.QueryContext(ctx, `
		SELECT status, COUNT(*)
		FROM code_resolution_diagnostics
		WHERE source_id = ?
		GROUP BY status`,
		sourceID,
	)
	if err != nil {
		return ResolutionCounts{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return ResolutionCounts{}, err
		}
		switch ResolutionStatus(status) {
		case ResolutionAmbiguous:
			counts.Ambiguous = count
		case ResolutionUnresolved:
			counts.Unresolved = count
		}
	}
	return counts, rows.Err()
}

func insertCodeEdge(ctx context.Context, tx *sql.Tx, edge CodeEdge) error {
	edge = normalizeCodeEdge(edge)
	_, err := tx.ExecContext(ctx,
		`INSERT OR REPLACE INTO code_edges
		 (src_id, dst_id, kind, file_path, line, provenance, origin, resolution_method, confidence, resolver_version, source_snapshot_hash, index_epoch)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		edge.SrcID,
		edge.DstID,
		string(edge.Kind),
		edge.FilePath,
		edge.Line,
		string(edge.Provenance),
		string(edge.Origin),
		string(edge.ResolutionMethod),
		string(edge.Confidence),
		edge.ResolverVersion,
		edge.SourceSnapshotHash,
		edge.IndexEpoch,
	)
	return err
}

func insertResolutionDiagnostic(ctx context.Context, tx *sql.Tx, diagnostic ResolutionDiagnostic) error {
	candidates, err := json.Marshal(diagnostic.CandidateIDs)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx,
		`INSERT OR REPLACE INTO code_resolution_diagnostics
		 (source_id, kind, file_path, line, status, reason, candidate_ids, origin, resolver_version, source_snapshot_hash)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		diagnostic.SourceID,
		string(diagnostic.Kind),
		diagnostic.FilePath,
		diagnostic.Line,
		string(diagnostic.Status),
		string(diagnostic.Reason),
		string(candidates),
		string(diagnostic.Origin),
		diagnostic.ResolverVersion,
		diagnostic.SourceSnapshotHash,
	)
	return err
}

func (e *EdgeStore) ensureColumn(ctx context.Context, name, definition string) error {
	rows, err := e.db.QueryContext(ctx, `PRAGMA table_info(code_edges)`)
	if err != nil {
		return fmt.Errorf("inspect code_edges columns: %w", err)
	}
	defer rows.Close()
	found := false
	for rows.Next() {
		var cid, notNull, primaryKey int
		var columnName, columnType string
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &columnName, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return fmt.Errorf("scan code_edges column: %w", err)
		}
		if columnName == name {
			found = true
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate code_edges columns: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close code_edges column scan: %w", err)
	}
	if found {
		return nil
	}
	statement := fmt.Sprintf("ALTER TABLE code_edges ADD COLUMN %s %s", name, definition)
	if _, err := e.db.ExecContext(ctx, statement); err != nil {
		return fmt.Errorf("add code_edges column %s: %w", name, err)
	}
	return nil
}
