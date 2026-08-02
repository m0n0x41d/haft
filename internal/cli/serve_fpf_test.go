package cli

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/fpf"
	_ "modernc.org/sqlite"
)

func TestHandleQuintQuery_FPFReturnsClosedSourceNativeUnion(t *testing.T) {
	dbPath := buildFPFSourceQueryTestDB(t)
	restoreOpen := stubSourceQueryDB(t, dbPath)
	defer restoreOpen()

	result, err := handleQuintQuery(context.Background(), setupCLIArtifactStore(t), nil, t.TempDir(), map[string]any{
		"action":               "fpf",
		"mode":                 "concern",
		"query":                "strict distinctions",
		"known_context":        []any{"system boundaries"},
		"max_total_candidates": float64(2),
	})
	if err != nil {
		t.Fatal(err)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(result), &payload); err != nil {
		t.Fatalf("FPF result is not JSON: %v\n%s", err, result)
	}
	if payload["kind"] != string(fpf.QueryResultKindCandidateSet) {
		t.Fatalf("kind = %#v, want candidate_set: %s", payload["kind"], result)
	}
	for _, forbidden := range []string{"── Haft", "recommended_pattern_use", "required_next_action"} {
		if strings.Contains(result, forbidden) {
			t.Fatalf("closed FPF result contaminated by %q: %s", forbidden, result)
		}
	}
}

func TestHandleQuintQuery_FPFLookupAndInspectAreExact(t *testing.T) {
	dbPath := buildFPFSourceQueryTestDB(t)
	restoreOpen := stubSourceQueryDB(t, dbPath)
	defer restoreOpen()
	store := setupCLIArtifactStore(t)

	lookup, err := handleQuintQuery(context.Background(), store, nil, t.TempDir(), map[string]any{
		"action":     "fpf",
		"mode":       "lookup",
		"identifier": "A.7",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(lookup, `"kind":"exact_hit"`) || !strings.Contains(lookup, `"source_role":"pattern_body"`) {
		t.Fatalf("unexpected exact lookup: %s", lookup)
	}

	missing, err := handleQuintQuery(context.Background(), store, nil, t.TempDir(), map[string]any{
		"action":     "fpf",
		"mode":       "inspect",
		"identifier": "definitely-missing",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(missing, `"kind":"abstained"`) || !strings.Contains(missing, "exact_source_unit_not_found") {
		t.Fatalf("unexpected inspect abstention: %s", missing)
	}
}

func TestHandleQuintQuery_FPFRequiresCanonicalMode(t *testing.T) {
	_, err := handleQuintQuery(context.Background(), setupCLIArtifactStore(t), nil, t.TempDir(), map[string]any{
		"action": "fpf",
		"query":  "legacy implicit search",
	})
	if err == nil || !strings.Contains(err.Error(), "mode is required") {
		t.Fatalf("missing mode error = %v", err)
	}

	_, err = fpfQueryRequestFromArgs(map[string]any{"mode": "tree", "query": "legacy mode"})
	if err == nil || !strings.Contains(err.Error(), "unsupported FPF query mode") {
		t.Fatalf("legacy mode error = %v", err)
	}
}

func TestFPFQueryRequestFromArgsPreservesTypedConcernFields(t *testing.T) {
	request, err := fpfQueryRequestFromArgs(map[string]any{
		"mode":                        "concern",
		"query":                       "How should this concern be framed?",
		"entity_of_concern":           "delivery system",
		"known_context":               []any{"time-boxed release", "source-first retrieval"},
		"intended_use":                "shape a bounded decision",
		"max_candidates_per_role":     float64(3),
		"max_total_candidates":        float64(7),
		"max_excerpt_characters":      float64(900),
		"max_relations_per_candidate": float64(4),
	})
	if err != nil {
		t.Fatal(err)
	}

	concern, ok := request.(fpf.ConcernQuery)
	if !ok {
		t.Fatalf("request = %T, want fpf.ConcernQuery", request)
	}
	if concern.EntityOfConcern != "delivery system" || concern.IntendedUse != "shape a bounded decision" {
		t.Fatalf("contextual fields lost: %#v", concern)
	}
	if len(concern.KnownContext) != 2 {
		t.Fatalf("array fields lost: %#v", concern)
	}
	if concern.ResponseBudget.MaxCandidatesPerRole != 3 || concern.ResponseBudget.MaxTotalCandidates != 7 || concern.ResponseBudget.MaxExcerptCharacters != 900 || concern.ResponseBudget.MaxRelationsPerCandidate != 4 {
		t.Fatalf("response budget lost: %#v", concern.ResponseBudget)
	}
}

func TestFPFQueryRequestFromArgsRejectsConcernRoles(t *testing.T) {
	_, err := fpfQueryRequestFromArgs(map[string]any{
		"mode":  "concern",
		"query": "How should this concern be framed?",
		"roles": []any{"pattern_body"},
	})
	if err == nil || !strings.Contains(err.Error(), "roles are not accepted for concern mode") {
		t.Fatalf("concern roles error = %v", err)
	}
}

func TestFPFQueryRequestFromArgsRejectsFractionalBudget(t *testing.T) {
	_, err := fpfQueryRequestFromArgs(map[string]any{
		"mode":                 "concern",
		"query":                "bounded concern",
		"max_total_candidates": 2.5,
	})
	if err == nil || !strings.Contains(err.Error(), "max_total_candidates must be a non-negative integer") {
		t.Fatalf("fractional budget error = %v", err)
	}
}

func TestHandleQuintQuery_FPFRejectsPresentNonStringPublicationFields(t *testing.T) {
	store := setupCLIArtifactStore(t)
	tests := []struct {
		name  string
		field string
		value any
	}{
		{name: "null view", field: "view", value: nil},
		{name: "numeric view", field: "view", value: float64(1)},
		{name: "null trace ref", field: "trace_ref", value: nil},
		{name: "numeric trace ref", field: "trace_ref", value: float64(1)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args := map[string]any{
				"action": "fpf",
				"mode":   "concern",
				"query":  "bounded concern",
				"view":   "trace",
			}
			args[test.field] = test.value
			_, err := handleQuintQuery(
				context.Background(),
				store,
				nil,
				t.TempDir(),
				args,
			)
			if err == nil || !strings.Contains(err.Error(), test.field+" must be a string") {
				t.Fatalf("%s error = %v", test.field, err)
			}
		})
	}
}

func TestHandleQuintQuery_RelatedArtifactIDReturnsProblemCardJSON(t *testing.T) {
	ctx := context.Background()
	store := setupCLIArtifactStore(t)

	problem := &artifact.Artifact{
		Meta: artifact.Meta{
			ID:         "prob-20260423-test",
			Kind:       artifact.KindProblemCard,
			Title:      "Portable project MCP config",
			ValidUntil: "2026-05-23",
		},
		Body:           "Problem body for Open-Sleigh frame verification.",
		StructuredData: `{"signal":"shared config embeds an absolute path"}`,
	}
	if err := store.Create(ctx, problem); err != nil {
		t.Fatalf("create problem: %v", err)
	}

	result, err := handleQuintQuery(ctx, store, nil, t.TempDir(), map[string]any{
		"action":      "related",
		"artifact_id": problem.Meta.ID,
	})
	if err != nil {
		t.Fatalf("handleQuintQuery returned error: %v", err)
	}

	var payload map[string]map[string]any
	if err := json.Unmarshal([]byte(result), &payload); err != nil {
		t.Fatalf("expected JSON response, got %v:\n%s", err, result)
	}

	card := payload["problem_card"]
	if card["id"] != problem.Meta.ID {
		t.Fatalf("problem_card.id = %#v, want %s", card["id"], problem.Meta.ID)
	}
	if card["kind"] != string(artifact.KindProblemCard) {
		t.Fatalf("problem_card.kind = %#v", card["kind"])
	}
	if card["body"] != problem.Body {
		t.Fatalf("problem_card.body = %#v, want %q", card["body"], problem.Body)
	}
	if card["structured_data"] != problem.StructuredData {
		t.Fatalf("problem_card.structured_data = %#v", card["structured_data"])
	}
	semantic, ok := card["semantic"].(map[string]any)
	if !ok {
		t.Fatalf("problem_card.semantic missing or wrong shape: %#v", card["semantic"])
	}
	if semantic["status"] != string(artifact.SemanticStatusLegacy) {
		t.Fatalf("problem_card.semantic.status = %#v, want legacy", semantic["status"])
	}
	views, ok := card["views"].(map[string]any)
	if !ok {
		t.Fatalf("problem_card.views missing or wrong shape: %#v", card["views"])
	}
	if _, ok := views["working"]; !ok {
		t.Fatalf("problem_card.views missing working view: %#v", views)
	}
	if _, ok := views["exact"]; !ok {
		t.Fatalf("problem_card.views missing exact view: %#v", views)
	}
	if _, ok := views["audit"]; !ok {
		t.Fatalf("problem_card.views missing audit view: %#v", views)
	}
}

func TestHandleQuintQuery_ExactSearchAndCanonicalRelatedContract(t *testing.T) {
	ctx := context.Background()
	store := setupCLIArtifactStore(t)
	target := &artifact.Artifact{
		Meta: artifact.Meta{
			ID:    "dec-20260711-exact",
			Kind:  artifact.KindDecisionRecord,
			Title: "Exact decision",
		},
		Body:           "Full decision body.",
		StructuredData: `{"claims":[{"id":"claim-1","observable":"go test ./...","threshold":"passes"}]}`,
	}
	neighbor := &artifact.Artifact{
		Meta: artifact.Meta{
			ID:    "dec-20260711-neighbor",
			Kind:  artifact.KindDecisionRecord,
			Title: "Neighbor decision",
		},
		Body: "Full decision body with similar terms.",
	}
	for _, item := range []*artifact.Artifact{target, neighbor} {
		if err := store.Create(ctx, item); err != nil {
			t.Fatal(err)
		}
	}

	compact, err := handleQuintQuery(ctx, store, nil, t.TempDir(), map[string]any{
		"action": "search",
		"query":  target.Meta.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(compact, target.Meta.ID) || strings.Contains(compact, neighbor.Meta.ID) {
		t.Fatalf("compact exact search returned wrong set:\n%s", compact)
	}
	if strings.Contains(compact, "── Haft") {
		t.Fatalf("compact exact search must be a closed result without project-state footer:\n%s", compact)
	}

	fullSearch, err := handleQuintQuery(ctx, store, nil, t.TempDir(), map[string]any{
		"action": "search",
		"query":  target.Meta.ID,
		"full":   true,
	})
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := handleQuintQuery(ctx, store, nil, t.TempDir(), map[string]any{
		"action":       "related",
		"artifact_ref": target.Meta.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if fullSearch != canonical {
		t.Fatalf("full exact search and canonical related differ:\nsearch=%s\nrelated=%s", fullSearch, canonical)
	}
	if !strings.Contains(canonical, target.Body) || !strings.Contains(canonical, "structured_data") || !strings.Contains(canonical, "claim-1") {
		t.Fatalf("full exact payload missing body or structured data: %s", canonical)
	}

	_, err = handleQuintQuery(ctx, store, nil, t.TempDir(), map[string]any{
		"action": "search",
		"query":  "decision body",
		"full":   true,
	})
	if err == nil || !strings.Contains(err.Error(), "run haft_query(action=\"search\"") {
		t.Fatalf("non-exact full search error = %v", err)
	}

	_, err = handleQuintQuery(ctx, store, nil, t.TempDir(), map[string]any{
		"action": "search",
		"query":  "dec-20260711-missing",
	})
	if err == nil || !strings.Contains(err.Error(), "semantic fallback was not used") {
		t.Fatalf("exact miss error = %v", err)
	}
}

func TestHandleQuintQuery_RelatedFileModeRemainsAvailable(t *testing.T) {
	ctx := context.Background()
	store := setupCLIArtifactStore(t)
	decision := &artifact.Artifact{
		Meta: artifact.Meta{ID: "dec-20260711-file", Kind: artifact.KindDecisionRecord, Title: "File decision"},
	}
	if err := store.Create(ctx, decision); err != nil {
		t.Fatal(err)
	}
	if err := store.SetAffectedFiles(ctx, decision.Meta.ID, []artifact.AffectedFile{{Path: "internal/example.go"}}); err != nil {
		t.Fatal(err)
	}

	result, err := handleQuintQuery(ctx, store, nil, t.TempDir(), map[string]any{
		"action": "related",
		"file":   "internal/example.go",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, decision.Meta.ID) {
		t.Fatalf("file-related result missing decision: %s", result)
	}
}

func TestHandleQuintQuery_RelatedProblemPayloadIncludesExactSemanticViews(t *testing.T) {
	ctx := context.Background()
	store := setupCLIArtifactStore(t)

	problem, _, err := artifact.FrameProblem(ctx, store, t.TempDir(), artifact.ProblemFrameInput{
		Title:                "Exact semantic payload",
		ProblemProfile:       artifact.ProblemProfileDeep,
		Signal:               "Fresh ProblemCards should expose exact semantic views.",
		WhyNow:               "PublicationUnit is now part of the semantic spine.",
		Scope:                "ProblemCard related payload views.",
		AcceptanceProbe:      "related payload includes working, exact, and audit views.",
		FreshnessDisposition: "Re-check when related view shape changes.",
		Acceptance:           "related payload includes working, exact, and audit views.",
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := handleQuintQuery(ctx, store, nil, t.TempDir(), map[string]any{
		"action": "related",
		"ref":    problem.Meta.ID,
	})
	if err != nil {
		t.Fatalf("handleQuintQuery returned error: %v", err)
	}

	var payload map[string]map[string]any
	if err := json.Unmarshal([]byte(result), &payload); err != nil {
		t.Fatalf("expected JSON response, got %v:\n%s", err, result)
	}

	card := payload["problem_card"]
	semantic := card["semantic"].(map[string]any)
	if semantic["status"] != string(artifact.SemanticStatusExact) {
		t.Fatalf("semantic.status = %#v, want exact", semantic["status"])
	}
	publicationUnit := semantic["publication_unit"].(map[string]any)
	if publicationUnit["publication_hash"] == "" || publicationUnit["carrier_hash"] == "" {
		t.Fatalf("semantic publication_unit missing hashes: %#v", publicationUnit)
	}

	views := card["views"].(map[string]any)
	working := views["working"].(map[string]any)
	if working["semantic_status"] != string(artifact.SemanticStatusExact) {
		t.Fatalf("working.semantic_status = %#v, want exact", working["semantic_status"])
	}
	if working["source_edition_hash"] == "" || working["publication_hash"] == "" {
		t.Fatalf("working view missing source/publication hashes: %#v", working)
	}
	if working["problem_profile"] != artifact.ProblemProfileDeep {
		t.Fatalf("working.problem_profile = %#v, want %q", working["problem_profile"], artifact.ProblemProfileDeep)
	}
	if working["p2w_readiness"] != artifact.ProblemReadinessReady {
		t.Fatalf("working.p2w_readiness = %#v, want %q", working["p2w_readiness"], artifact.ProblemReadinessReady)
	}
	exact := views["exact"].(map[string]any)
	if _, ok := exact["source_episteme"].(map[string]any); !ok {
		t.Fatalf("exact view missing source_episteme: %#v", exact)
	}
	if _, ok := exact["publication_projection"].(map[string]any); !ok {
		t.Fatalf("exact view missing publication_projection: %#v", exact)
	}
	if _, ok := exact["carrier_bytes"].(map[string]any); !ok {
		t.Fatalf("exact view missing carrier_bytes: %#v", exact)
	}
	audit := views["audit"].(map[string]any)
	if audit["semantic_status"] != string(artifact.SemanticStatusExact) {
		t.Fatalf("audit.semantic_status = %#v, want exact", audit["semantic_status"])
	}
	if _, ok := audit["publication_unit"].(map[string]any); !ok {
		t.Fatalf("audit view missing publication_unit: %#v", audit)
	}
}

func TestHandleQuintProblem_FramePersistsProblemProfile(t *testing.T) {
	ctx := context.Background()
	store := setupCLIArtifactStore(t)

	result, _, err := handleQuintProblemWithCreatedRef(ctx, store, t.TempDir(), map[string]any{
		"action":                "frame",
		"title":                 "Ticket cannot become work without boundary",
		"problem_profile":       artifact.ProblemProfileDeep,
		"source_kind":           artifact.ProblemSourceTicket,
		"signal":                "A ticket names a request but not an implementation boundary.",
		"why_now":               "The slice train is consuming problem frames.",
		"freshness_disposition": "Re-check before admission.",
	})
	if err != nil {
		t.Fatalf("handleQuintProblemWithCreatedRef returned error: %v", err)
	}
	if !strings.Contains(result, "Problem framed") {
		t.Fatalf("unexpected frame response:\n%s", result)
	}

	items, err := artifact.SelectProblems(ctx, store, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("created problem count = %d, want 1", len(items))
	}

	reloaded, err := store.Get(ctx, items[0].Meta.ID)
	if err != nil {
		t.Fatal(err)
	}

	profile := reloaded.UnmarshalProblemFields().Profile
	if profile == nil {
		t.Fatal("problem profile missing")
	}
	if profile.SourceKind != artifact.ProblemSourceTicket {
		t.Fatalf("profile.source_kind = %q, want %q", profile.SourceKind, artifact.ProblemSourceTicket)
	}
	if profile.Readiness != artifact.ProblemReadinessBlocked {
		t.Fatalf("profile.readiness = %q, want %q", profile.Readiness, artifact.ProblemReadinessBlocked)
	}
}

func setupCLIArtifactStore(t *testing.T) *artifact.Store {
	t.Helper()

	db, err := sql.Open("sqlite", t.TempDir()+"/cli-tools.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	stmts := []string{
		`CREATE TABLE artifacts (
			id TEXT PRIMARY KEY, kind TEXT NOT NULL, version INTEGER NOT NULL DEFAULT 1,
			status TEXT NOT NULL DEFAULT 'active', context TEXT, mode TEXT,
			title TEXT NOT NULL, content TEXT NOT NULL, file_path TEXT,
			valid_until TEXT, created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
			search_keywords TEXT DEFAULT '', structured_data TEXT DEFAULT '')`,
		`CREATE TABLE artifact_links (
			source_id TEXT NOT NULL, target_id TEXT NOT NULL, link_type TEXT NOT NULL,
			created_at TEXT NOT NULL, PRIMARY KEY (source_id, target_id, link_type))`,
		`CREATE TABLE evidence_items (
			id TEXT PRIMARY KEY, artifact_ref TEXT NOT NULL, type TEXT NOT NULL,
			content TEXT NOT NULL, verdict TEXT, carrier_ref TEXT,
			congruence_level INTEGER DEFAULT 3, formality_level INTEGER DEFAULT 5,
			claim_refs TEXT DEFAULT '[]', claim_scope TEXT DEFAULT '[]',
			valid_until TEXT, created_at TEXT NOT NULL)`,
		`CREATE TABLE affected_files (
			artifact_id TEXT NOT NULL, file_path TEXT NOT NULL, file_hash TEXT,
			PRIMARY KEY (artifact_id, file_path))`,
		`CREATE TABLE affected_symbols (
			artifact_id TEXT NOT NULL, file_path TEXT NOT NULL,
			symbol_name TEXT NOT NULL, symbol_kind TEXT NOT NULL,
			symbol_line INTEGER, symbol_end_line INTEGER, symbol_hash TEXT,
			PRIMARY KEY (artifact_id, file_path, symbol_name))`,
		`CREATE TABLE artifact_symbol_bindings (
			artifact_id TEXT NOT NULL, anchor_id TEXT NOT NULL,
			anchor_version INTEGER NOT NULL, file_path TEXT NOT NULL,
			language TEXT NOT NULL, symbol_name TEXT NOT NULL,
			symbol_kind TEXT NOT NULL, receiver TEXT NOT NULL DEFAULT '',
			qualified_name TEXT NOT NULL, signature_hash TEXT NOT NULL,
			symbol_line INTEGER NOT NULL DEFAULT 0,
			symbol_end_line INTEGER NOT NULL DEFAULT 0,
			body_hash TEXT NOT NULL DEFAULT '',
			binding_status TEXT NOT NULL DEFAULT 'active',
			resolution_source TEXT NOT NULL DEFAULT '',
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (artifact_id, anchor_id))`,
		`CREATE TABLE codebase_modules (
			module_id TEXT PRIMARY KEY, path TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL, lang TEXT, file_count INTEGER DEFAULT 0,
			last_scanned TEXT NOT NULL)`,
		`CREATE TABLE module_dependencies (
			source_module TEXT NOT NULL, target_module TEXT NOT NULL,
			dep_type TEXT NOT NULL DEFAULT 'import', file_path TEXT,
			last_scanned TEXT NOT NULL,
			PRIMARY KEY (source_module, target_module, dep_type))`,
		`CREATE VIRTUAL TABLE artifacts_fts USING fts5(id, title, content, kind, search_keywords, tokenize='porter unicode61')`,
		`CREATE TRIGGER artifacts_fts_insert AFTER INSERT ON artifacts BEGIN
			INSERT INTO artifacts_fts(id, title, content, kind, search_keywords) VALUES (new.id, new.title, new.content, new.kind, new.search_keywords);
		END`,
		`CREATE TRIGGER artifacts_fts_update AFTER UPDATE ON artifacts BEGIN
			DELETE FROM artifacts_fts WHERE id = old.id;
			INSERT INTO artifacts_fts(id, title, content, kind, search_keywords) VALUES (new.id, new.title, new.content, new.kind, new.search_keywords);
		END`,
		`CREATE TRIGGER artifacts_fts_delete AFTER DELETE ON artifacts BEGIN
			DELETE FROM artifacts_fts WHERE id = old.id;
		END`,
	}

	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("setup: %v\nSQL: %s", err, stmt)
		}
	}

	return artifact.NewStore(db)
}

func buildFPFSourceQueryTestDB(t *testing.T) string {
	t.Helper()
	return buildFPFSourceQueryTestDBAtRevision(t, "cli-query-test-revision")
}

func buildFPFSourceQueryTestDBAtRevision(t *testing.T, revisionLabel string) string {
	t.Helper()

	revision := syntheticFPFSourceRevision(revisionLabel)
	units := syntheticFPFSourceQueryUnits(revision)
	dbPath := filepath.Join(t.TempDir(), "fpf-source-query.db")
	if err := fpf.StoreSourceUnits(dbPath, units); err != nil {
		t.Fatalf("store synthetic source units: %v", err)
	}
	metadataDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open synthetic source metadata database: %v", err)
	}
	if _, err := metadataDB.Exec(`CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`); err != nil {
		_ = metadataDB.Close()
		t.Fatalf("create synthetic source metadata grammar: %v", err)
	}
	if err := metadataDB.Close(); err != nil {
		t.Fatalf("close synthetic source metadata database: %v", err)
	}
	entries := map[string]string{
		"schema_version":         fpf.SpecIndexSchemaVersion,
		"fpf_commit":             revision,
		"readme_document_digest": syntheticFPFDocumentDigest("synthetic-readme-" + revision),
		"spec_document_digest":   syntheticFPFDocumentDigest("synthetic-spec-" + revision),
	}
	if err := fpf.SetSpecMetaEntries(dbPath, entries); err != nil {
		t.Fatalf("store synthetic source metadata: %v", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open synthetic source index: %v", err)
	}
	defer func() { _ = db.Close() }()
	if err := fpf.VerifySourceQueryIndexReadOnlyDB(db); err != nil {
		t.Fatalf("synthetic source index does not satisfy canonical validation: %v", err)
	}
	snapshot, err := fpf.LoadQuerySourceSnapshot(db)
	if err != nil {
		t.Fatalf("load synthetic source metadata: %v", err)
	}
	if snapshot.Revision() != revision || snapshot.IndexSchemaVersion() != fpf.SpecIndexSchemaVersion {
		t.Fatalf("synthetic source snapshot = %#v", snapshot)
	}
	return dbPath
}

func syntheticFPFSourceQueryUnits(revision string) []fpf.SourceUnit {
	strictTOCBody := "| A.7 | Strict Distinctions | Stable | Builds on B.5 |"
	strictTOCProvenance := syntheticFPFProvenance(
		"FPF-Spec.md",
		10,
		strictTOCBody,
		revision,
	)
	strictRelation := fpf.SourceRelation{
		Kind:            fpf.SourceRelationKindBuildsOn,
		TargetPatternID: "B.5",
		TargetClass:     fpf.SourceRelationTargetClassLocalPattern,
		Origin:          fpf.SourceRelationOriginTOCExplicit,
		Provenance:      strictTOCProvenance,
	}

	return []fpf.SourceUnit{
		syntheticFPFSourceUnit(
			"readme:practical-use:strict",
			"README.STRICT",
			fpf.SourceUnitRolePracticalUseCard,
			"Strict distinctions working situation",
			"Architecture structural view needs strict distinctions before selecting a method.",
			"",
			"",
			"Readme.md",
			1,
			revision,
			func(unit *fpf.SourceUnit) {
				unit.DirectRefs = []string{"A.7"}
				unit.AuthoredPhrases = []string{"architecture structural view", "strict distinctions"}
				unit.Keywords = []string{"architecture", "structural", "distinctions"}
				unit.UseCues = fpf.SourceUseCues{
					ConditionText:   "When architecture terms collapse object, description, and carrier.",
					FirstResultText: "A bounded set of explicit distinctions.",
					StopReturnText:  "Return when the current concern is distinguishable.",
				}
			},
		),
		syntheticFPFSourceUnit(
			"readme:practical-use:alternatives",
			"README.ALTERNATIVES",
			fpf.SourceUnitRolePracticalUseCard,
			"Architecture alternatives working situation",
			"Architecture structural view can keep rival alternatives visible without a hidden winner.",
			"",
			"",
			"Readme.md",
			2,
			revision,
			func(unit *fpf.SourceUnit) {
				unit.DirectRefs = []string{"B.5"}
				unit.AuthoredPhrases = []string{"architecture structural view", "rival alternatives"}
				unit.Keywords = []string{"architecture", "structural", "alternatives"}
				unit.UseCues = fpf.SourceUseCues{
					ConditionText:   "When several architecture candidates remain live.",
					FirstResultText: "A bounded alternative set.",
					StopReturnText:  "Return when a selection policy is needed.",
				}
			},
		),
		syntheticFPFSourceUnit(
			"readme:preface",
			"",
			fpf.SourceUnitRolePreface,
			"Source-first preface",
			"Source retrieval is not applicability, authority, or performed work.",
			"",
			"",
			"Readme.md",
			3,
			revision,
			nil,
		),
		{
			UnitID:            "spec:toc-row:a-7",
			Role:              fpf.SourceUnitRoleTOCRow,
			Title:             "Strict Distinctions",
			Body:              strictTOCBody,
			PatternID:         "A.7",
			PublicationStatus: "Stable",
			DirectRefs:        []string{"B.5"},
			AuthoredPhrases:   []string{"strict distinctions", "architecture structural view"},
			Keywords:          []string{"architecture", "structural", "distinctions"},
			Provenance:        strictTOCProvenance,
		},
		syntheticFPFSourceUnit(
			"spec:toc-row:b-5",
			"",
			fpf.SourceUnitRoleTOCRow,
			"Rival Alternatives",
			"| B.5 | Rival Alternatives | Stable | Coordinates with A.7 |",
			"B.5",
			"",
			"FPF-Spec.md",
			11,
			revision,
			func(unit *fpf.SourceUnit) {
				unit.PublicationStatus = "Stable"
				unit.DirectRefs = []string{"A.7"}
				unit.AuthoredPhrases = []string{"architecture structural view", "rival alternatives"}
				unit.Keywords = []string{"architecture", "structural", "alternatives"}
			},
		),
		syntheticFPFSourceUnit(
			"spec:pattern-body:a-7",
			"A.7",
			fpf.SourceUnitRolePatternBody,
			"A.7 Strict Distinctions",
			"A.7 exact body sentinel\nProblem frame\nProblem\nForces\nSolution\nOrdinary boundary\nWorked slice\nChecklist",
			"A.7",
			"",
			"FPF-Spec.md",
			20,
			revision,
			func(unit *fpf.SourceUnit) {
				unit.AuthoredPhrases = []string{"strict distinctions"}
				unit.Keywords = []string{"architecture", "distinctions", "solution"}
				unit.Relations = []fpf.SourceRelation{strictRelation}
			},
		),
		syntheticFPFSourceUnit(
			"spec:pattern-body:b-5",
			"B.5",
			fpf.SourceUnitRolePatternBody,
			"B.5 Rival Alternatives",
			"B.5 exact body sentinel\nProblem frame\nProblem\nForces\nSolution\nOrdinary boundary\nWorked slice\nChecklist",
			"B.5",
			"",
			"FPF-Spec.md",
			30,
			revision,
			func(unit *fpf.SourceUnit) {
				unit.AuthoredPhrases = []string{"rival alternatives"}
				unit.Keywords = []string{"architecture", "alternatives", "solution"}
			},
		),
		syntheticFPFSourceUnit(
			"spec:pattern-section:a-7:solution",
			"A.7:solution",
			fpf.SourceUnitRolePatternSection,
			"A.7 Solution",
			"A.7 exact section solution content.",
			"A.7:solution",
			"A.7",
			"FPF-Spec.md",
			21,
			revision,
			func(unit *fpf.SourceUnit) {
				unit.Keywords = []string{"solution"}
			},
		),
		syntheticFPFSourceUnit(
			"spec:pattern-section:b-5:solution",
			"B.5:solution",
			fpf.SourceUnitRolePatternSection,
			"B.5 Solution",
			"B.5 exact section solution content.",
			"B.5:solution",
			"B.5",
			"FPF-Spec.md",
			31,
			revision,
			func(unit *fpf.SourceUnit) {
				unit.Keywords = []string{"solution"}
			},
		),
	}
}

func syntheticFPFSourceUnit(
	unitID string,
	sourceID string,
	role fpf.SourceUnitRole,
	title string,
	body string,
	patternID string,
	parentPatternID string,
	sourcePath string,
	line int,
	revision string,
	decorate func(*fpf.SourceUnit),
) fpf.SourceUnit {
	unit := fpf.SourceUnit{
		UnitID:          unitID,
		SourceID:        sourceID,
		Role:            role,
		Title:           title,
		Body:            body,
		PatternID:       patternID,
		ParentPatternID: parentPatternID,
		Provenance:      syntheticFPFProvenance(sourcePath, line, body, revision),
	}
	if decorate != nil {
		decorate(&unit)
	}
	return unit
}

func syntheticFPFProvenance(
	sourcePath string,
	line int,
	body string,
	revision string,
) fpf.SourceProvenance {
	digest := sha256.Sum256([]byte(body))
	return fpf.SourceProvenance{
		SourcePath:     sourcePath,
		StartLine:      line,
		EndLine:        line,
		ContentHash:    hex.EncodeToString(digest[:]),
		SourceRevision: revision,
	}
}

func syntheticFPFDocumentDigest(body string) string {
	digest := sha256.Sum256([]byte(body))
	return "sha256:" + hex.EncodeToString(digest[:])
}

func syntheticFPFSourceRevision(label string) string {
	digest := sha256.Sum256([]byte(label))
	return hex.EncodeToString(digest[:20])
}

func stubSourceQueryDB(t *testing.T, dbPath string) func() {
	t.Helper()
	originalOpen := openFPFDBFunc
	originalVerify := verifyFPFDBFunc
	openFPFDBFunc = func() (*sql.DB, func(), error) {
		db, err := sql.Open("sqlite", dbPath)
		if err != nil {
			return nil, nil, err
		}
		return db, func() { _ = db.Close() }, nil
	}
	verifyFPFDBFunc = fpf.VerifySourceQueryIndexReadOnlyDB
	return func() {
		openFPFDBFunc = originalOpen
		verifyFPFDBFunc = originalVerify
	}
}
