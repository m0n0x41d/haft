package codeintel

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/codebase"
)

func TestDiscoverConcernReturnsCandidatesWithoutSelectingIdentity(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeCurrentnessSource(
		t,
		root,
		"internal/codebase/incremental.go",
		`package codebase
type Scanner struct{}
func (s *Scanner) publishIndexEpoch() {}
func (s *Scanner) RefreshIncremental() {}
`,
	)
	store, closeStore := newCurrentnessArtifactStore(t, root)
	defer closeStore()

	result, err := NewService(store).DiscoverConcern(
		ctx,
		root,
		"Where is the code index epoch published atomically?",
		8,
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome.String() != ConcernCandidates {
		t.Fatalf("outcome = %+v", result)
	}
	if len(result.Candidates()) == 0 {
		t.Fatal("expected an evidence-bearing candidate set")
	}
	candidate := result.Candidates()[0]
	lexical, present := candidate.Lexical().Candidate()
	if candidate.Symbol().Name != "publishIndexEpoch" ||
		candidate.Symbol().AnchorID == "" ||
		candidate.Epoch() != result.Index.Epoch ||
		!present ||
		len(lexical.Matches()) == 0 {
		t.Fatalf("candidate = %#v", candidate)
	}
	wire, err := result.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{
		`"selected"`,
		`"selected_symbol"`,
		`"winner"`,
	} {
		if strings.Contains(string(wire), forbidden) {
			t.Fatalf("result selected identity through %s: %s", forbidden, wire)
		}
	}
}

func TestDiscoverConcernRetriesAcrossConcurrentPublication(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeCurrentnessSource(
		t,
		root,
		"stable.go",
		"package sample\nfunc StableSymbol() {}\n",
	)
	store, closeStore := newCurrentnessArtifactStore(t, root)
	defer closeStore()
	service := NewService(store)

	hookCalls := 0
	service.beforeBasisConfirm = func(ctx context.Context) error {
		hookCalls++
		if hookCalls > 1 {
			return nil
		}
		writeCurrentnessSource(
			t,
			root,
			"added.go",
			"package sample\nfunc AddedSymbol() {}\n",
		)
		_, err := service.scanner.RefreshIncremental(ctx, root)
		return err
	}
	result, err := service.DiscoverConcern(
		ctx,
		root,
		"AddedSymbol",
		5,
	)
	if err != nil {
		t.Fatal(err)
	}
	if hookCalls != 2 ||
		result.Index.Epoch != 2 ||
		len(result.Candidates()) != 1 ||
		result.Candidates()[0].Symbol().Name != "AddedSymbol" {
		t.Fatalf(
			"retried discovery: hooks=%d result=%+v candidates=%+v",
			hookCalls,
			result,
			result.Candidates(),
		)
	}
}

func TestDiscoverConcernDoesNotClaimAbsenceUnderBoundedCoverage(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeCurrentnessSource(
		t,
		root,
		"stable.go",
		"package sample\nfunc StableSymbol() {}\n",
	)
	writeCurrentnessSource(
		t,
		root,
		"oversized.go",
		"package sample\n"+strings.Repeat(" ", 500_001),
	)
	store, closeStore := newCurrentnessArtifactStore(t, root)
	defer closeStore()

	result, err := NewService(store).DiscoverConcern(
		ctx,
		root,
		"QuantumBanana",
		5,
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome.String() != ConcernIncompleteBasis ||
		len(result.Candidates()) != 0 {
		t.Fatalf("bounded missing concern = %+v", result)
	}
}

func TestConcernSeedSetNormalizesEachActiveOriginLane(t *testing.T) {
	set := newConcernSeedSet([]unweightedConcernSeed{
		{
			nodeID: "code-a",
			origin: ConcernSeedOrigin{
				code: ConcernSeedCodeLexical,
			},
			sourceRef: "sym:a",
		},
		{
			nodeID: "code-b",
			origin: ConcernSeedOrigin{
				code: ConcernSeedCodeLexical,
			},
			sourceRef: "sym:b",
		},
		{
			nodeID: "dec-a",
			origin: ConcernSeedOrigin{
				code: ConcernSeedReasoningArtifact,
			},
			sourceRef: "dec-a",
		},
	})
	weights := map[string]float64{}
	total := 0.0
	for _, seed := range set.Items() {
		weights[seed.NodeID()] = seed.Weight()
		total += seed.Weight()
	}
	if total != 1 ||
		weights["code-a"] != 0.25 ||
		weights["code-b"] != 0.25 ||
		weights["dec-a"] != 0.5 {
		t.Fatalf("normalized typed seeds = %+v total=%v", weights, total)
	}
}

func TestDiscoverConcernFusesReasoningArtifactThroughAffectedFile(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeCurrentnessSource(
		t,
		root,
		"internal/service/assembler.go",
		"package service\nfunc AssembleGateway() {}\n",
	)
	store, closeStore := newCurrentnessArtifactStore(t, root)
	defer closeStore()
	decision := createConcernArtifact(
		t,
		ctx,
		store,
		"dec-20260726-filebridge",
		artifact.KindDecisionRecord,
		"Cobalt umbrella boundary policy",
		"The cobalt umbrella boundary policy is assembled in the service.",
	)
	if err := store.SetAffectedFiles(
		ctx,
		decision.Meta.ID,
		[]artifact.AffectedFile{
			{Path: "internal/service/assembler.go"},
		},
	); err != nil {
		t.Fatal(err)
	}

	result, err := NewService(store).DiscoverConcern(
		ctx,
		root,
		"Cobalt umbrella boundary policy lang:go path:internal/service",
		5,
	)
	if err != nil {
		t.Fatal(err)
	}
	candidate := concernCandidateByName(
		t,
		result,
		"AssembleGateway",
	)
	if _, present := candidate.Lexical().Candidate(); present {
		t.Fatalf("reasoning-only candidate gained lexical evidence: %+v", candidate)
	}
	if candidate.Graph().Reasoning() <= 0 {
		t.Fatalf("reasoning graph support = %+v", candidate.Graph())
	}
	assertConcernArtifactSupport(
		t,
		candidate,
		decision.Meta.ID,
		ConcernBridgeAffectedFile,
		true,
	)
	if result.Basis.AffectedFileBridges != 1 ||
		result.Basis.ReplayRef == "" {
		t.Fatalf("fusion basis = %+v", result.Basis)
	}
}

func TestDiscoverConcernUsesCurrentExactSymbolBinding(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeCurrentnessSource(
		t,
		root,
		"internal/service/exact.go",
		"package service\nfunc ApplyInvariant() {}\n",
	)
	store, closeStore := newCurrentnessArtifactStore(t, root)
	defer closeStore()
	service := NewService(store)
	if _, err := service.EnsureIndex(ctx, root); err != nil {
		t.Fatal(err)
	}
	symbols, err := service.symbols.GetByName(ctx, "ApplyInvariant")
	if err != nil || len(symbols) != 1 {
		t.Fatalf("resolve exact fixture symbol: %+v err=%v", symbols, err)
	}
	symbol := symbols[0]
	decision := createConcernArtifact(
		t,
		ctx,
		store,
		"dec-20260726-exactbridge",
		artifact.KindDecisionRecord,
		"Quartz exact invariant",
		"The quartz exact invariant has one durable implementation binding.",
	)
	if err := store.SetAffectedSymbols(
		ctx,
		decision.Meta.ID,
		[]artifact.AffectedSymbol{
			affectedSymbolFromCodeSymbol(symbol),
		},
	); err != nil {
		t.Fatal(err)
	}

	result, err := service.DiscoverConcern(
		ctx,
		root,
		"Quartz exact invariant lang:go path:internal/service",
		5,
	)
	if err != nil {
		t.Fatal(err)
	}
	candidate := concernCandidateByName(t, result, "ApplyInvariant")
	assertConcernArtifactSupport(
		t,
		candidate,
		decision.Meta.ID,
		ConcernBridgeExactSymbol,
		true,
	)
	if result.Basis.ExactSymbolBridges != 1 ||
		result.Basis.StaleSymbolBindings != 0 {
		t.Fatalf("exact binding basis = %+v", result.Basis)
	}
}

func TestExactBindingTieBreakUsesExplicitConcernSymbolName(t *testing.T) {
	query, err := codebase.NewConcernQuery(
		"Where is the selected Explore architecture assembled?",
	)
	if err != nil {
		t.Fatal(err)
	}
	explore := ConcernCandidate{
		symbol: codebase.CodeSymbol{
			AnchorID:      "sym:v2:explore",
			Name:          "Explore",
			QualifiedName: "Service.Explore",
		},
		directBridge: ConcernBridgeExactSymbol,
		directNameTermMatch: concernSymbolNameMatchesQuery(
			"Explore",
			query,
		),
		graph: ConcernGraphSupport{
			kind:     ConcernGraphPresent,
			combined: 0.1,
		},
	}
	ensureIndex := ConcernCandidate{
		symbol: codebase.CodeSymbol{
			AnchorID:      "sym:v2:ensure-index",
			Name:          "EnsureIndex",
			QualifiedName: "Service.EnsureIndex",
		},
		directBridge: ConcernBridgeExactSymbol,
		directNameTermMatch: concernSymbolNameMatchesQuery(
			"EnsureIndex",
			query,
		),
		graph: ConcernGraphSupport{
			kind:     ConcernGraphPresent,
			combined: 0.9,
		},
	}
	candidates := []ConcernCandidate{ensureIndex, explore}
	sortConcernCandidates(candidates)
	if candidates[0].symbol.Name != "Explore" {
		t.Fatalf(
			"exact binding tie-break selected %s",
			candidates[0].symbol.Name,
		)
	}
}

func TestDiscoverConcernDropsStaleSymbolBindingInsteadOfInventingEdge(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeCurrentnessSource(
		t,
		root,
		"internal/service/stable.go",
		"package service\nfunc StableBoundary() {}\n",
	)
	store, closeStore := newCurrentnessArtifactStore(t, root)
	defer closeStore()
	decision := createConcernArtifact(
		t,
		ctx,
		store,
		"dec-20260726-stalebridge",
		artifact.KindDecisionRecord,
		"Magenta stale anchor policy",
		"The magenta stale anchor policy has no current code declaration.",
	)
	_, err := store.DB().ExecContext(ctx, `
		INSERT INTO artifact_symbol_bindings (
		  artifact_id, anchor_id, anchor_version, file_path, language,
		  symbol_name, symbol_kind, qualified_name, signature_hash,
		  binding_status
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		decision.Meta.ID,
		"sym:v2:stale",
		2,
		"internal/service/missing.go",
		"go",
		"MissingBoundary",
		"func",
		"MissingBoundary",
		"sha256:missing",
		artifact.SymbolBindingActive,
	)
	if err != nil {
		t.Fatal(err)
	}
	result, err := NewService(store).DiscoverConcern(
		ctx,
		root,
		"Magenta stale anchor policy lang:go path:internal/service",
		5,
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome.String() != ConcernNoCandidates ||
		len(result.Candidates()) != 0 ||
		result.Basis.StaleSymbolBindings != 1 ||
		result.Basis.ExactSymbolBridges != 0 {
		t.Fatalf("stale binding result = %+v", result)
	}
}

type fixedConcernExactSeedSource struct {
	batch ConcernExactSeedBatch
}

func (s fixedConcernExactSeedSource) ExactConcernSeeds(
	context.Context,
	ConcernSeedRequest,
) (ConcernExactSeedBatch, error) {
	return s.batch, nil
}

func TestConcernExactSeedAdapterIsOptionalAndExact(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeCurrentnessSource(
		t,
		root,
		"internal/service/adapter.go",
		"package service\nfunc AdapterTarget() {}\n",
	)
	store, closeStore := newCurrentnessArtifactStore(t, root)
	defer closeStore()
	defaultService := NewService(store)
	if _, err := defaultService.EnsureIndex(ctx, root); err != nil {
		t.Fatal(err)
	}
	symbols, err := defaultService.symbols.GetByName(
		ctx,
		"AdapterTarget",
	)
	if err != nil || len(symbols) != 1 {
		t.Fatalf("adapter fixture symbol = %+v err=%v", symbols, err)
	}
	query := "Obsidian remembered entity lang:go path:internal/service"
	withoutAdapter, err := defaultService.DiscoverConcern(
		ctx,
		root,
		query,
		5,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(withoutAdapter.Candidates()) != 0 {
		t.Fatalf(
			"null adapter invented candidates: %+v",
			withoutAdapter.Candidates(),
		)
	}
	external, err := NewConcernSymbolAnchorSeed(
		symbols[0].AnchorID,
		"entity:adapter-target",
		"typed_memory_entity",
	)
	if err != nil {
		t.Fatal(err)
	}
	service := NewServiceWithConcernExactSeeds(
		store,
		fixedConcernExactSeedSource{
			batch: NewConcernExactSeedBatch(
				[]ConcernExternalSeed{external},
			),
		},
	)
	withAdapter, err := service.DiscoverConcern(
		ctx,
		root,
		query,
		5,
	)
	if err != nil {
		t.Fatal(err)
	}
	candidate := concernCandidateByName(
		t,
		withAdapter,
		"AdapterTarget",
	)
	if candidate.Graph().TypedMemory() <= 0 {
		t.Fatalf("typed-memory exact support = %+v", candidate.Graph())
	}
}

func TestConcernExactIdentityCannotBeDisplacedByGraphPopularity(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeCurrentnessSource(
		t,
		root,
		"internal/service/exact_identity.go",
		"package service\nfunc ExactIdentity() {}\nfunc PopularHub() {}\n",
	)
	store, closeStore := newCurrentnessArtifactStore(t, root)
	defer closeStore()
	service := NewService(store)
	if _, err := service.EnsureIndex(ctx, root); err != nil {
		t.Fatal(err)
	}
	symbols, err := service.symbols.GetByName(ctx, "ExactIdentity")
	if err != nil || len(symbols) != 1 {
		t.Fatalf("exact identity fixture = %+v err=%v", symbols, err)
	}
	for index := 0; index < 24; index++ {
		item := createConcernArtifact(
			t,
			ctx,
			store,
			fmt.Sprintf("note-20260726-hub-%02d", index),
			artifact.KindNote,
			fmt.Sprintf("Popular graph hub %02d", index),
			"Popular graph hub.",
		)
		if err := store.SetAffectedFiles(
			ctx,
			item.Meta.ID,
			[]artifact.AffectedFile{
				{Path: "internal/service/exact_identity.go"},
			},
		); err != nil {
			t.Fatal(err)
		}
	}
	result, err := service.DiscoverConcern(
		ctx,
		root,
		symbols[0].AnchorID,
		5,
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome.String() != ConcernResolvedExactIdentity ||
		len(result.Candidates()) != 1 ||
		result.Candidates()[0].Symbol().AnchorID != symbols[0].AnchorID {
		t.Fatalf("exact identity displaced = %+v", result)
	}
}

func TestDiscoverConcernRetriesWhenReasoningBasisChanges(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeCurrentnessSource(
		t,
		root,
		"internal/service/replay.go",
		"package service\nfunc ReplayBoundary() {}\n",
	)
	store, closeStore := newCurrentnessArtifactStore(t, root)
	defer closeStore()
	decision := createConcernArtifact(
		t,
		ctx,
		store,
		"dec-20260726-replay",
		artifact.KindDecisionRecord,
		"Silver replay boundary",
		"Silver replay boundary revision one.",
	)
	if err := store.SetAffectedFiles(
		ctx,
		decision.Meta.ID,
		[]artifact.AffectedFile{
			{Path: "internal/service/replay.go"},
		},
	); err != nil {
		t.Fatal(err)
	}
	service := NewService(store)
	query := "Silver replay boundary lang:go path:internal/service"
	before, err := service.DiscoverConcern(ctx, root, query, 5)
	if err != nil {
		t.Fatal(err)
	}
	hookCalls := 0
	service.beforeBasisConfirm = func(ctx context.Context) error {
		hookCalls++
		if hookCalls > 1 {
			return nil
		}
		current, err := store.Get(ctx, decision.Meta.ID)
		if err != nil {
			return err
		}
		current.Body = "Silver replay boundary revision two."
		return store.Update(ctx, current)
	}
	after, err := service.DiscoverConcern(ctx, root, query, 5)
	if err != nil {
		t.Fatal(err)
	}
	if hookCalls != 2 {
		t.Fatalf(
			"reasoning basis confirmation calls=%d, want retry then success",
			hookCalls,
		)
	}
	if before.Basis.ReplayRef == after.Basis.ReplayRef {
		t.Fatalf(
			"reasoning revision kept replay identity %s",
			after.Basis.ReplayRef,
		)
	}
}

func createConcernArtifact(
	t *testing.T,
	ctx context.Context,
	store *artifact.Store,
	id string,
	kind artifact.Kind,
	title string,
	body string,
) *artifact.Artifact {
	t.Helper()
	now := time.Now().UTC()
	item := &artifact.Artifact{
		Meta: artifact.Meta{
			ID:        id,
			Kind:      kind,
			Version:   1,
			Status:    artifact.StatusActive,
			Title:     title,
			CreatedAt: now,
			UpdatedAt: now,
		},
		Body: body,
	}
	if err := store.Create(ctx, item); err != nil {
		t.Fatal(err)
	}
	return item
}

func affectedSymbolFromCodeSymbol(
	symbol codebase.CodeSymbol,
) artifact.AffectedSymbol {
	return artifact.AffectedSymbol{
		AnchorID:      symbol.AnchorID,
		AnchorVersion: symbol.AnchorVersion,
		FilePath:      symbol.FilePath,
		Language:      symbol.Lang,
		SymbolName:    symbol.Name,
		SymbolKind:    symbol.Kind,
		Receiver:      symbol.Receiver,
		QualifiedName: symbol.QualifiedName,
		SignatureHash: symbol.SignatureHash,
		Line:          symbol.StartLine,
		EndLine:       symbol.EndLine,
		Hash:          symbol.Hash,
	}
}

func concernCandidateByName(
	t *testing.T,
	result ConcernDiscoveryResult,
	name string,
) ConcernCandidate {
	t.Helper()
	for _, candidate := range result.Candidates() {
		if candidate.Symbol().Name == name {
			return candidate
		}
	}
	t.Fatalf(
		"candidate %q missing from %+v",
		name,
		result.Candidates(),
	)
	return ConcernCandidate{}
}

func assertConcernArtifactSupport(
	t *testing.T,
	candidate ConcernCandidate,
	artifactRef string,
	relation string,
	seeded bool,
) {
	t.Helper()
	for _, support := range candidate.Artifacts() {
		if support.ArtifactRef == artifactRef &&
			support.Relation == relation &&
			support.Seeded == seeded {
			return
		}
	}
	t.Fatalf(
		"support %s/%s/%t missing from %+v",
		artifactRef,
		relation,
		seeded,
		candidate.Artifacts(),
	)
}
