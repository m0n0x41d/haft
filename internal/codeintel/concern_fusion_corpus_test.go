package codeintel

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/codebase"
	"github.com/m0n0x41d/haft/internal/testsupport/kerneldbfixture"
)

type hg6CorpusManifest struct {
	Concerns []hg6CorpusConcern `json:"concerns"`
}

type hg6CorpusConcern struct {
	ID                    string            `json:"id"`
	Class                 string            `json:"class"`
	OperatorQuery         string            `json:"operator_query"`
	Filters               map[string]string `json:"filters"`
	AcceptableSymbolIDs   []string          `json:"acceptable_symbol_ids"`
	ExpectedReasoningRefs []string          `json:"expected_reasoning_refs"`
}

func TestConcernFusionMeetsFrozenReasoningToCodeCorpus(t *testing.T) {
	projectRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	manifest := readHG6CorpusManifest(t, projectRoot)
	fixtureRoot := copyHG6SourceCorpus(t, projectRoot)
	database, err := kerneldbfixture.OpenCurrentStore(
		filepath.Join(t.TempDir(), "hg6-corpus.db"),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	store := artifact.NewStore(database.GetRawDB())
	service := NewService(store)
	if _, err := service.EnsureIndex(
		context.Background(),
		fixtureRoot,
	); err != nil {
		t.Fatal(err)
	}
	importHG6ReasoningCarriers(
		t,
		projectRoot,
		store,
		service,
	)

	count := 0
	for _, concern := range manifest.Concerns {
		if concern.Class != "reasoning_to_code" {
			continue
		}
		query := hg6ConcernQueryWithFilters(concern)
		first, err := service.DiscoverConcern(
			context.Background(),
			fixtureRoot,
			query,
			12,
		)
		if err != nil {
			t.Fatalf("%s first discovery: %v", concern.ID, err)
		}
		rank, candidate := hg6AcceptableCandidate(
			first,
			concern.AcceptableSymbolIDs,
		)
		if rank < 1 || rank > 5 {
			t.Fatalf(
				"%s acceptable rank=%d candidates=%s",
				concern.ID,
				rank,
				hg6CandidateSummary(first),
			)
		}
		for _, reasoningRef := range concern.ExpectedReasoningRefs {
			if !candidateHasReasoningRef(candidate, reasoningRef) {
				t.Fatalf(
					"%s candidate %s lacks exact reasoning support %s: %+v",
					concern.ID,
					candidate.Symbol().AnchorID,
					reasoningRef,
					candidate.Artifacts(),
				)
			}
		}
		if candidate.Graph().Reasoning() <= 0 {
			t.Fatalf(
				"%s candidate has no reasoning graph component: %+v",
				concern.ID,
				candidate.Graph(),
			)
		}
		if first.Basis.GraphNodes > first.Basis.GraphInductionMaxNodes {
			t.Fatalf(
				"%s induced graph nodes=%d exceeds cap=%d",
				concern.ID,
				first.Basis.GraphNodes,
				first.Basis.GraphInductionMaxNodes,
			)
		}
		second, err := service.DiscoverConcern(
			context.Background(),
			fixtureRoot,
			query,
			12,
		)
		if err != nil {
			t.Fatalf("%s replay discovery: %v", concern.ID, err)
		}
		if first.Basis.ReplayRef != second.Basis.ReplayRef ||
			hg6CandidateSummary(first) != hg6CandidateSummary(second) {
			t.Fatalf(
				"%s replay drift:\nfirst=%s %s\nsecond=%s %s",
				concern.ID,
				first.Basis.ReplayRef,
				hg6CandidateSummary(first),
				second.Basis.ReplayRef,
				hg6CandidateSummary(second),
			)
		}
		t.Logf(
			"%s acceptable rank=%d reasoning=%v graph=%d/%d "+
				"node_cap_reached=%v replay=%s",
			concern.ID,
			rank,
			concern.ExpectedReasoningRefs,
			first.Basis.GraphNodes,
			first.Basis.FullGraphNodes,
			first.Basis.GraphNodeCapReached,
			first.Basis.ReplayRef,
		)
		count++
	}
	if count != 3 {
		t.Fatalf("reasoning-to-code corpus count=%d, want 3", count)
	}
	negativeCount := 0
	for _, concern := range manifest.Concerns {
		if concern.Class != "ambiguous_negative" {
			continue
		}
		result, err := service.DiscoverConcern(
			context.Background(),
			fixtureRoot,
			hg6ConcernQueryWithFilters(concern),
			12,
		)
		if err != nil {
			t.Fatalf("%s negative discovery: %v", concern.ID, err)
		}
		if result.Outcome.String() == ConcernResolvedExactIdentity {
			t.Fatalf(
				"%s ambiguous/negative concern resolved identity: %+v",
				concern.ID,
				result,
			)
		}
		wire, err := result.MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{
			`"selected"`,
			`"winner"`,
			`"resolved_symbol"`,
		} {
			if strings.Contains(string(wire), forbidden) {
				t.Fatalf(
					"%s selected through %s: %s",
					concern.ID,
					forbidden,
					wire,
				)
			}
		}
		negativeCount++
	}
	if negativeCount != 3 {
		t.Fatalf(
			"ambiguous/negative corpus count=%d, want 3",
			negativeCount,
		)
	}
}

func BenchmarkConcernFusionFrozenReasoningCorpus(b *testing.B) {
	projectRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		b.Fatal(err)
	}
	manifest := readHG6CorpusManifest(b, projectRoot)
	fixtureRoot := copyHG6SourceCorpus(b, projectRoot)
	database, err := kerneldbfixture.OpenCurrentStore(
		filepath.Join(b.TempDir(), "hg6-benchmark.db"),
	)
	if err != nil {
		b.Fatal(err)
	}
	defer database.Close()
	store := artifact.NewStore(database.GetRawDB())
	service := NewService(store)
	if _, err := service.EnsureIndex(
		context.Background(),
		fixtureRoot,
	); err != nil {
		b.Fatal(err)
	}
	importHG6ReasoningCarriers(
		b,
		projectRoot,
		store,
		service,
	)
	query := ""
	for _, concern := range manifest.Concerns {
		if concern.ID == "RC1" {
			query = hg6ConcernQueryWithFilters(concern)
			break
		}
	}
	if query == "" {
		b.Fatal("RC1 query missing")
	}
	b.ResetTimer()
	for range b.N {
		result, err := service.DiscoverConcern(
			context.Background(),
			fixtureRoot,
			query,
			12,
		)
		if err != nil || len(result.Candidates()) == 0 {
			b.Fatalf(
				"fusion candidates=%d err=%v",
				len(result.Candidates()),
				err,
			)
		}
	}
}

func readHG6CorpusManifest(
	t testing.TB,
	projectRoot string,
) hg6CorpusManifest {
	t.Helper()
	path := filepath.Join(
		projectRoot,
		".context",
		"v9-remaining-evidence",
		"r2-graph-baseline",
		"corpus-manifest.json",
	)
	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		// .context не отслеживается git, поэтому в свежем чекауте корпуса нет.
		// Пропускаем по образцу internal/recall/liveeval_test.go: отсутствие
		// носителя — не регрессия, и падение сообщало бы не о ней.
		t.Skipf("frozen HG6 corpus not found at %s — skipping", path)
	}
	if err != nil {
		t.Fatal(err)
	}
	var manifest hg6CorpusManifest
	if err := json.Unmarshal(content, &manifest); err != nil {
		t.Fatal(err)
	}
	return manifest
}

func copyHG6SourceCorpus(
	t testing.TB,
	projectRoot string,
) string {
	t.Helper()
	fixtureRoot := t.TempDir()
	for _, directory := range []string{
		"internal/codebase",
		"internal/codeintel",
		"internal/contextgraph",
	} {
		sourceDirectory := filepath.Join(projectRoot, directory)
		targetDirectory := filepath.Join(fixtureRoot, directory)
		if err := os.MkdirAll(targetDirectory, 0o755); err != nil {
			t.Fatal(err)
		}
		entries, err := os.ReadDir(sourceDirectory)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			if entry.IsDir() ||
				filepath.Ext(entry.Name()) != ".go" {
				continue
			}
			content, err := os.ReadFile(
				filepath.Join(sourceDirectory, entry.Name()),
			)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(
				filepath.Join(targetDirectory, entry.Name()),
				content,
				0o644,
			); err != nil {
				t.Fatal(err)
			}
		}
	}
	return fixtureRoot
}

func importHG6ReasoningCarriers(
	t testing.TB,
	projectRoot string,
	store *artifact.Store,
	service *Service,
) {
	t.Helper()
	ctx := context.Background()
	for _, decisionRef := range []string{
		"dec-20260603-5825abc6",
		"dec-20260604-ef966a11",
	} {
		content, err := os.ReadFile(filepath.Join(
			projectRoot,
			".haft",
			"decisions",
			decisionRef+".md",
		))
		if err != nil {
			t.Fatal(err)
		}
		item, err := artifact.ParseFile(string(content))
		if err != nil {
			t.Fatalf("parse %s: %v", decisionRef, err)
		}
		// The corpus needs the two real decision carriers and their exact
		// code bridges; their based_on targets are outside this bounded fixture.
		// Keep those absent rather than inventing placeholder artifacts.
		item.Meta.Links = nil
		if err := store.Create(ctx, item); err != nil {
			t.Fatalf("create %s: %v", decisionRef, err)
		}
		files := hg6DecisionAffectedFiles(item)
		affectedFiles := make([]artifact.AffectedFile, 0, len(files))
		for _, file := range files {
			affectedFiles = append(
				affectedFiles,
				artifact.AffectedFile{Path: file},
			)
		}
		if err := store.SetAffectedFiles(
			ctx,
			item.Meta.ID,
			affectedFiles,
		); err != nil {
			t.Fatalf("affected files %s: %v", decisionRef, err)
		}
		targets := hg6DecisionSymbolTargets(item)
		symbols, err := service.symbols.AllSymbols(ctx)
		if err != nil {
			t.Fatal(err)
		}
		bindings := hg6ResolveAffectedSymbols(symbols, targets)
		if len(bindings) == 0 {
			continue
		}
		if err := store.SetAffectedSymbols(
			ctx,
			item.Meta.ID,
			bindings,
		); err != nil {
			t.Fatalf("affected symbols %s: %v", decisionRef, err)
		}
	}
}

type hg6SymbolTarget struct {
	file     string
	name     string
	receiver string
}

func hg6DecisionAffectedFiles(
	item *artifact.Artifact,
) []string {
	fields := item.UnmarshalDecisionFields()
	files := append([]string{}, fields.ImplementationFootprint.Files...)
	for _, target := range fields.BindingTargets {
		files = append(files, target.FilePath)
	}
	for _, target := range fields.GovernanceTargets {
		parsed, ok := hg6ParseSymbolTarget(target.Ref)
		if ok {
			files = append(files, parsed.file)
		}
	}
	for _, line := range strings.Split(item.Body, "\n") {
		if !strings.HasPrefix(line, "**Affected files:**") {
			continue
		}
		raw := strings.TrimSpace(
			strings.TrimPrefix(line, "**Affected files:**"),
		)
		for _, part := range strings.Split(raw, ",") {
			file := strings.Trim(strings.TrimSpace(part), "`")
			files = append(files, file)
		}
	}
	return stableUniqueStrings(files)
}

func hg6DecisionSymbolTargets(
	item *artifact.Artifact,
) []hg6SymbolTarget {
	fields := item.UnmarshalDecisionFields()
	targets := make([]hg6SymbolTarget, 0)
	for _, target := range fields.BindingTargets {
		if target.FilePath == "" || target.SymbolName == "" {
			continue
		}
		targets = append(targets, hg6SymbolTarget{
			file:     target.FilePath,
			name:     target.SymbolName,
			receiver: target.Receiver,
		})
	}
	for _, target := range fields.GovernanceTargets {
		parsed, ok := hg6ParseSymbolTarget(target.Ref)
		if ok {
			targets = append(targets, parsed)
		}
	}
	return targets
}

func hg6ParseSymbolTarget(raw string) (hg6SymbolTarget, bool) {
	if !strings.HasPrefix(raw, "symbol:") {
		return hg6SymbolTarget{}, false
	}
	remainder := strings.TrimPrefix(raw, "symbol:")
	markers := []string{":func:", ":method:", ":type:"}
	for _, marker := range markers {
		position := strings.Index(remainder, marker)
		if position < 1 {
			continue
		}
		file := remainder[:position]
		identity := strings.TrimPrefix(remainder[position:], marker)
		parts := strings.Split(identity, ":")
		target := hg6SymbolTarget{file: file}
		if marker == ":method:" && len(parts) >= 2 {
			target.receiver = parts[0]
			target.name = parts[len(parts)-1]
			return target, true
		}
		target.name = parts[len(parts)-1]
		return target, target.name != ""
	}
	return hg6SymbolTarget{}, false
}

func hg6ResolveAffectedSymbols(
	symbols []codebase.CodeSymbol,
	targets []hg6SymbolTarget,
) []artifact.AffectedSymbol {
	byAnchor := make(map[string]artifact.AffectedSymbol)
	for _, target := range targets {
		for _, symbol := range symbols {
			if symbol.FilePath != target.file ||
				symbol.Name != target.name {
				continue
			}
			if target.receiver != "" &&
				symbol.Receiver != target.receiver {
				continue
			}
			byAnchor[symbol.AnchorID] =
				affectedSymbolFromCodeSymbol(symbol)
		}
	}
	anchors := make([]string, 0, len(byAnchor))
	for anchor := range byAnchor {
		anchors = append(anchors, anchor)
	}
	sort.Strings(anchors)
	out := make([]artifact.AffectedSymbol, 0, len(anchors))
	for _, anchor := range anchors {
		out = append(out, byAnchor[anchor])
	}
	return out
}

func hg6ConcernQueryWithFilters(concern hg6CorpusConcern) string {
	keys := make([]string, 0, len(concern.Filters))
	for key := range concern.Filters {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := []string{concern.OperatorQuery}
	for _, key := range keys {
		value := concern.Filters[key]
		if key == "language" {
			key = "lang"
		}
		parts = append(parts, key+":"+value)
	}
	return strings.Join(parts, " ")
}

func hg6AcceptableCandidate(
	result ConcernDiscoveryResult,
	acceptable []string,
) (int, ConcernCandidate) {
	wanted := make(map[string]bool, len(acceptable))
	for _, anchor := range acceptable {
		wanted[anchor] = true
	}
	for index, candidate := range result.Candidates() {
		if wanted[candidate.Symbol().AnchorID] {
			return index + 1, candidate
		}
	}
	return 0, ConcernCandidate{}
}

func candidateHasReasoningRef(
	candidate ConcernCandidate,
	reasoningRef string,
) bool {
	for _, support := range candidate.Artifacts() {
		if support.ArtifactRef == reasoningRef &&
			(support.Relation == ConcernBridgeExactSymbol ||
				support.Relation == ConcernBridgeAffectedFile) {
			return true
		}
	}
	return false
}

func hg6CandidateSummary(result ConcernDiscoveryResult) string {
	parts := make([]string, 0, len(result.Candidates()))
	for index, candidate := range result.Candidates() {
		lexical, present := candidate.Lexical().Candidate()
		tier := "none"
		coverage := "0/0"
		fieldCoverage := "0/0"
		if present {
			tier = lexical.Tier().String()
			coverage = fmt.Sprintf(
				"%d/%d",
				lexical.Coverage().Covered(),
				lexical.Coverage().Total(),
			)
			fieldCoverage = fmt.Sprintf(
				"%d/%d",
				lexical.FieldCoverage().Covered(),
				lexical.FieldCoverage().Total(),
			)
		}
		parts = append(parts,
			fmt.Sprintf(
				"%d:%s[%s]@%s/c=%s/f=%s/g=%.12f",
				index+1,
				candidate.Symbol().AnchorID,
				candidate.Symbol().QualifiedName,
				tier,
				coverage,
				fieldCoverage,
				candidate.Graph().Combined(),
			),
		)
	}
	return strings.Join(parts, ",")
}
