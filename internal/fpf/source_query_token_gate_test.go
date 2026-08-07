//go:build query_token_gate

package fpf

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

const (
	queryTokenGateSchema             = "haft.fpf-query-token-gate/v1"
	queryTokenGateResultSchema       = "haft.fpf-query-token-gate-result/v1"
	queryTokenGateEncoding           = "o200k_base"
	queryTokenGateTokenizerVersion   = "0.9.0"
	queryTokenGateEncodingAssetHash  = "446a9538cb6c348e3516120d7c08b09f57c36495e2acfffe59a5bf8b0cfb1a2d"
	queryTokenGateCalibrationTokens  = 21
	queryTokenGateMinimumReduction   = 0.30
	queryTokenGateCorpusDigest       = "834f126c5ca9ea631ccbac467a29f9fa126a5b1d2b830710fd7a02a8034e5bd0"
	queryTokenGateStrengthLimit      = "synthetic frozen carriers prove projection/token arithmetic without data/FPF; they do not prove installed-index representativeness or live retrieval quality"
	queryTokenGateSyntheticRevision  = "sha256:9f3ecf8a5c2cbbb1babd645176848237c1ae44f35a91839d6661c618612ce064"
	queryTokenGateSyntheticReadmeSHA = "sha256:7fa645b8b8d4f70ab1833bab006e6e6af297aebf83730992dfd11541df5fb2b7"
	queryTokenGateSyntheticSpecSHA   = "sha256:750fd19874346ca2b3cc94ce99ff994355b77e2daa9cb8e75c322ae517ed65ff"
)

type queryTokenGateCase struct {
	CaseID  string
	Request ConcernQuery
	Result  CandidateSet
}

type queryTokenGateInput struct {
	Schema string                    `json:"schema"`
	Cases  []queryTokenGateInputCase `json:"cases"`
}

type queryTokenGateInputCase struct {
	CaseID        string `json:"case_id"`
	CanonicalJSON string `json:"canonical_json"`
	WorkingJSON   string `json:"working_json"`
}

type queryTokenGateOutput struct {
	Schema                string                     `json:"schema"`
	Encoding              string                     `json:"encoding"`
	TokenizerDistribution string                     `json:"tokenizer_distribution"`
	TokenizerVersion      string                     `json:"tokenizer_version"`
	EncodingAssetSHA256   string                     `json:"encoding_asset_sha256"`
	CalibrationTokens     int                        `json:"calibration_tokens"`
	Counts                []queryTokenGateOutputCase `json:"counts"`
}

type queryTokenGateOutputCase struct {
	CaseID          string `json:"case_id"`
	CanonicalTokens int    `json:"canonical_tokens"`
	WorkingTokens   int    `json:"working_tokens"`
}

type queryTokenGateCandidateSeed struct {
	PatternID string
	Title     string
	Excerpt   string
	Cues      SourceUseCues
	Refs      []string
}

func TestFPFQueryWorkingViewSyntheticTokenCalculus(t *testing.T) {
	corpus := buildQueryTokenGateCorpus()
	assertQueryTokenGateCorpusDigest(t, corpus)

	snapshot := queryTokenGateSnapshot(t)
	publicationRequest, err := NewQueryPublicationRequest("working", "")
	if err != nil {
		t.Fatal(err)
	}

	inputCases := make([]queryTokenGateInputCase, 0, len(corpus))
	for _, testCase := range corpus {
		evaluation := newQueryEvaluation(
			testCase.Result,
			[]string{
				queryProducerExactSource,
				queryProducerSourcePhrase,
				queryProducerAuthoredPhrase,
				queryProducerHeadingKeyword,
				queryProducerRoleLocalFTS,
			},
		)
		execution, err := NewCanonicalQueryExecution(
			testCase.Request,
			evaluation,
			snapshot,
		)
		if err != nil {
			t.Fatalf("%s canonical execution: %v", testCase.CaseID, err)
		}
		published, err := ProjectQueryResult(execution, publicationRequest)
		if err != nil {
			t.Fatalf("%s working projection: %v", testCase.CaseID, err)
		}
		working, ok := published.(workingCandidateSet)
		if !ok {
			t.Fatalf("%s working projection = %T", testCase.CaseID, published)
		}
		assertQueryTokenGateSemantics(t, testCase.Result, working)

		canonicalJSON, err := json.Marshal(testCase.Result)
		if err != nil {
			t.Fatalf("%s compact canonical JSON: %v", testCase.CaseID, err)
		}
		workingJSON, err := EncodePublishedQuery(working, PublishedQueryJSONCompact)
		if err != nil {
			t.Fatalf("%s compact working JSON: %v", testCase.CaseID, err)
		}
		inputCases = append(inputCases, queryTokenGateInputCase{
			CaseID:        testCase.CaseID,
			CanonicalJSON: string(canonicalJSON),
			WorkingJSON:   string(workingJSON),
		})
	}

	measurements := runQueryTokenGateCounter(t, inputCases)
	assertQueryTokenGateReduction(t, measurements)
	t.Logf("strength limit: %s", queryTokenGateStrengthLimit)
}

func buildQueryTokenGateCorpus() []queryTokenGateCase {
	concerns := []struct {
		caseID         string
		text           string
		entity         string
		knownContext   []string
		intendedUse    string
		candidateCount int
		groundsPerItem int
		omittedAtLeast int
	}{
		{
			caseID:         "architecture-boundary",
			text:           "Which direct FPF patterns distinguish the target system from its enabling engineering system while keeping a public interface change reversible?",
			entity:         "v9 typed-memory public query boundary",
			knownContext:   []string{"canonical provenance remains internal", "CLI and MCP must have semantic parity"},
			intendedUse:    "select direct patterns to inspect before implementing a reversible boundary",
			candidateCount: 10,
			groundsPerItem: 4,
			omittedAtLeast: 4,
		},
		{
			caseID:         "russian-ambiguous-concern",
			text:           "Какой прямой паттерн FPF нужен, если описание плана уже есть, но выполненную работу и свидетельство результата нельзя выдавать за этот план?",
			entity:         "различение плана, работы и свидетельства",
			knownContext:   []string{"план не является выполненной работой", "описание не доказывает результат"},
			intendedUse:    "найти паттерны для разборки неоднозначного инженерного утверждения",
			candidateCount: 8,
			groundsPerItem: 3,
			omittedAtLeast: 2,
		},
		{
			caseID:         "evidence-drift",
			text:           "How should a reliance-bearing decision be checked when the baseline is stable but runtime evidence may have decayed under a changed source snapshot?",
			entity:         "decision verification evidence loop",
			knownContext:   []string{"baseline and measurement are distinct", "replay must fail closed on snapshot drift"},
			intendedUse:    "identify the governing verification and evidence-decay patterns",
			candidateCount: 7,
			groundsPerItem: 3,
			omittedAtLeast: 1,
		},
		{
			caseID:         "authority-gate",
			text:           "Which source patterns separate a generated recommendation from an operator binding act and a later work commission without making the document an acting agent?",
			entity:         "human authority boundary",
			knownContext:   []string{"recommendation is non-binding", "binding and execution authority are distinct"},
			intendedUse:    "recover direct authority patterns before changing a manual gate",
			candidateCount: 6,
			groundsPerItem: 4,
			omittedAtLeast: 0,
		},
		{
			caseID:         "pattern-use-navigation",
			text:           "Given several plausible source candidates, what pattern governs choosing the smallest working carrier now while retaining a compact trace for a named reliance-bearing use?",
			entity:         "FPF pattern use in a working situation",
			knownContext:   []string{"retrieval rank is not applicability", "ordinary and reliance-bearing uses need different carriers"},
			intendedUse:    "choose which candidate to inspect without treating retrieval metadata as authority",
			candidateCount: 9,
			groundsPerItem: 2,
			omittedAtLeast: 3,
		},
		{
			caseID:         "type-identity-compatibility",
			text:           "What source distinctions keep an immutable TypeEnv identity, its selected project head, and an edition-aware runtime evaluator from collapsing into one mutable current configuration?",
			entity:         "typed-memory constitution evaluation",
			knownContext:   []string{"old and current runtime editions must remain callable", "head selection is separate from constitution identity"},
			intendedUse:    "locate direct identity and compatibility patterns for an exact-X runtime proof",
			candidateCount: 5,
			groundsPerItem: 3,
			omittedAtLeast: 0,
		},
		{
			caseID:         "abstraction-levels",
			text:           "Which FPF source units help review a layered functional core where each layer must expose one abstraction level and invalid transitions should be inexpressible?",
			entity:         "typed-memory computation core",
			knownContext:   []string{"pure transformations precede the effect shell", "the public DTO cannot accept canonical internal results"},
			intendedUse:    "inspect patterns relevant to a module-boundary review",
			candidateCount: 8,
			groundsPerItem: 3,
			omittedAtLeast: 2,
		},
	}

	seeds := queryTokenGateCandidateSeeds()
	result := make([]queryTokenGateCase, 0, len(concerns))
	for caseIndex, concern := range concerns {
		request := ConcernQuery{
			Text:            concern.text,
			EntityOfConcern: concern.entity,
			KnownContext:    append([]string(nil), concern.knownContext...),
			IntendedUse:     concern.intendedUse,
			ResponseBudget:  defaultResponseBudget,
		}
		candidateSet := buildQueryTokenGateCandidateSet(
			caseIndex,
			concern.text,
			seeds,
			concern.candidateCount,
			concern.groundsPerItem,
			concern.omittedAtLeast,
		)
		result = append(result, queryTokenGateCase{
			CaseID:  concern.caseID,
			Request: request,
			Result:  candidateSet,
		})
	}
	return result
}

func queryTokenGateCandidateSeeds() []queryTokenGateCandidateSeed {
	return []queryTokenGateCandidateSeed{
		{
			PatternID: "SYN.11.PUA",
			Title:     "Pattern use in a working situation",
			Excerpt:   "Select a direct source pattern by the recognizable situation and the exact result needed now. Retrieval candidates remain navigation material rather than an applicability verdict, authority claim, causal order, or recommendation.",
			Cues: SourceUseCues{
				ConditionText:   "Several source patterns are plausible and the current use must choose a direct next result.",
				FirstResultText: "A bounded working carrier that preserves the selected source identity and the smallest sufficient authored semantics.",
				StopReturnText:  "Return when the current condition changes or the selected source basis is no longer sufficient.",
			},
			Refs: []string{"SYN.11.PUR", "SYN.17.0"},
		},
		{
			PatternID: "SYN.11.PUR",
			Title:     "Pattern use result",
			Excerpt:   "Keep the result honest about whether it is ordinary bounded use, reliance-bearing use, or diagnostic investigation. A compact trace can support replay without making internal retrieval grounds routine model context.",
			Cues: SourceUseCues{
				ConditionText:   "A named receiving use determines which description of the same canonical result is sufficient.",
				FirstResultText: "A use-shaped result with visible ambiguity, truncation, and a stable continuation reference.",
				StopReturnText:  "Do not widen the carrier merely because more internal representation is available.",
			},
			Refs: []string{"SYN.11.PUA", "SYN.4.WORK"},
		},
		{
			PatternID: "SYN.4.WORK",
			Title:     "Work, work plans, and performed work",
			Excerpt:   "A plan coordinates intended work but does not perform that work. Evidence about an observed result must remain distinct from a method description, a promise, a commission, and the work occurrence itself.",
			Cues: SourceUseCues{
				ConditionText:   "Planning language is being used as though it proves delivery or runtime behavior.",
				FirstResultText: "Separate intended transformations, acting roles, performed occurrences, and resulting evidence.",
				StopReturnText:  "Return when each claim names the correct object and evidence relation.",
			},
			Refs: []string{"SYN.10.EVIDENCE", "SYN.21.GATE"},
		},
		{
			PatternID: "SYN.10.EVIDENCE",
			Title:     "Evidence and evidence decay",
			Excerpt:   "Evidence supports a bounded claim under an explicit source and observation basis. Freshness, applicability, and provenance are different dimensions; none can be replaced by confidence prose or a green status indicator.",
			Cues: SourceUseCues{
				ConditionText:   "A reliance-bearing claim may have drifted since its baseline or observation was captured.",
				FirstResultText: "A baseline-versus-measure comparison with explicit freshness and mismatch posture.",
				StopReturnText:  "Return or abstain when the current source snapshot cannot support replay.",
			},
			Refs: []string{"SYN.3.4.DECAY", "SYN.21.GATE"},
		},
		{
			PatternID: "SYN.21.GATE",
			Title:     "Semantic and authority gate",
			Excerpt:   "An attention signal is not an authority receipt. Interrupt only where the current operation would make a binding choice, broaden execution authority, or rely on contradictory controlling content.",
			Cues: SourceUseCues{
				ConditionText:   "The next operation may cross from reversible work into a binding or authority-changing act.",
				FirstResultText: "Name the exact choice, actor, authority source, and operation that cannot continue without it.",
				StopReturnText:  "Continue unrelated authorized work while the affected operation remains gated.",
			},
			Refs: []string{"SYN.2.ROLE", "SYN.4.WORK"},
		},
		{
			PatternID: "SYN.2.ROLE",
			Title:     "Roles, agents, and authority",
			Excerpt:   "Descriptions, records, and carriers do not act. Name the system or role that performs work and keep a recommendation, a human binding act, and delegated execution authority as separate relations.",
			Cues: SourceUseCues{
				ConditionText:   "A carrier is described as though it chose, approved, commissioned, or performed work.",
				FirstResultText: "An explicit acting role and a bounded authority relation for each claimed occurrence.",
				StopReturnText:  "Return when no document or graph node is treated as the acting agent.",
			},
			Refs: []string{"SYN.21.GATE", "SYN.15.IDENTITY"},
		},
		{
			PatternID: "SYN.15.IDENTITY",
			Title:     "Identity and immutable reference",
			Excerpt:   "A stable identity names one object across descriptions and observations without making the mutable selected head part of that identity. Exact references must fail closed when their edition or source basis changes.",
			Cues: SourceUseCues{
				ConditionText:   "Current configuration, immutable identity, and evaluator edition are being collapsed.",
				FirstResultText: "Separate identity, selected head, edition pin, and observable evaluation result.",
				StopReturnText:  "Return when old and current exact references resolve under their declared basis.",
			},
			Refs: []string{"SYN.5.COMPAT", "SYN.10.EVIDENCE"},
		},
		{
			PatternID: "SYN.5.COMPAT",
			Title:     "Edition-aware compatibility",
			Excerpt:   "Compatibility is evaluated between explicit typed editions under a callable mechanism. A current implementation does not silently reinterpret an installed historical constitution or an older exact reference.",
			Cues: SourceUseCues{
				ConditionText:   "The same semantic record may be evaluated by several installed runtime editions.",
				FirstResultText: "A closed evaluator selection and a typed compatible, incompatible, or underdetermined result.",
				StopReturnText:  "Fail closed when the exact evaluator edition is not installed or callable.",
			},
			Refs: []string{"SYN.15.IDENTITY", "SYN.3.4.DECAY"},
		},
		{
			PatternID: "SYN.3.4.DECAY",
			Title:     "Source snapshot and replay drift",
			Excerpt:   "Replay binds the typed request, source snapshot, and canonical result. Snapshot or request mismatch is checked before retrieval; result mismatch is reported after a valid preflight rather than silently using current source.",
			Cues: SourceUseCues{
				ConditionText:   "A prior source-derived result must be reproduced after time or environment changed.",
				FirstResultText: "A compact opaque replay coordinate and a typed mismatch when any bound dimension drifts.",
				StopReturnText:  "Do not ask the model to copy repository paths, hashes, or source revisions.",
			},
			Refs: []string{"SYN.10.EVIDENCE", "SYN.11.PUR"},
		},
		{
			PatternID: "SYN.17.0",
			Title:     "Description projection boundary",
			Excerpt:   "One canonical object may have several use-specific descriptions. The public working projection omits internal witnesses without claiming they do not exist, while trace and diagnostic projections remain explicit alternatives.",
			Cues: SourceUseCues{
				ConditionText:   "An internal canonical representation is being serialized directly as a routine public carrier.",
				FirstResultText: "A closed canonical-result to selected-view to shared-encoder pipeline.",
				StopReturnText:  "Return when ordinary, trace, and diagnostic uses cannot accidentally share an unrestricted DTO.",
			},
			Refs: []string{"SYN.11.PUA", "SYN.15.IDENTITY"},
		},
	}
}

func buildQueryTokenGateCandidateSet(
	caseIndex int,
	concern string,
	seeds []queryTokenGateCandidateSeed,
	candidateCount int,
	groundsPerItem int,
	omittedAtLeast int,
) CandidateSet {
	groups := []SourceCandidateGroup{
		{Role: SourceUnitRolePracticalUseCard, Candidates: []SourceCandidate{}},
		{Role: SourceUnitRoleTOCRow, Candidates: []SourceCandidate{}},
	}
	for candidateIndex := 0; candidateIndex < candidateCount; candidateIndex++ {
		seedIndex := (caseIndex*3 + candidateIndex) % len(seeds)
		seed := seeds[seedIndex]
		roleIndex := candidateIndex % len(groups)
		role := groups[roleIndex].Role
		candidate := buildQueryTokenGateCandidate(
			caseIndex,
			candidateIndex,
			concern,
			seed,
			role,
			groundsPerItem,
		)
		groups[roleIndex].Candidates = append(groups[roleIndex].Candidates, candidate)
	}

	truncationApplied := omittedAtLeast > 0
	return CandidateSet{
		Kind:    QueryResultKindCandidateSet,
		Concern: concern,
		Groups:  groups,
		Truncation: CandidateTruncation{
			Applied:            truncationApplied,
			Budget:             defaultResponseBudget,
			IncludedCandidates: candidateCount,
			OmittedAtLeast:     omittedAtLeast,
			Basis: []string{
				"response_budget",
				"role_local_fts_producer_limit",
				"source_grounding_sufficiency",
			},
		},
	}
}

func buildQueryTokenGateCandidate(
	caseIndex int,
	candidateIndex int,
	concern string,
	seed queryTokenGateCandidateSeed,
	role SourceUnitRole,
	groundsPerItem int,
) SourceCandidate {
	unitID := fmt.Sprintf("synthetic:%s:%02d:%02d", role, caseIndex, candidateIndex)
	canonicalUnitID := fmt.Sprintf("synthetic:pattern_body:%s", strings.ToLower(seed.PatternID))
	line := 100 + caseIndex*100 + candidateIndex*7
	unitSource := strings.Join([]string{seed.Title, seed.Excerpt, string(role)}, "\n")
	unitProvenance := queryTokenGateProvenance("data/FPF/FPF-Spec.md", line, unitSource)
	relationSource := fmt.Sprintf("%s builds_on %s and coordinates_with %s", seed.PatternID, seed.Refs[0], seed.Refs[1])
	relationProvenance := queryTokenGateProvenance("data/FPF/TableOfContents.md", line+2, relationSource)
	relations := []SourceRelation{
		{
			Kind:            SourceRelationKindBuildsOn,
			TargetPatternID: seed.Refs[0],
			TargetClass:     SourceRelationTargetClassLocalPattern,
			Origin:          SourceRelationOriginTOCExplicit,
			Provenance:      relationProvenance,
		},
		{
			Kind:            SourceRelationKindCoordinatesWith,
			TargetPatternID: seed.Refs[1],
			TargetClass:     SourceRelationTargetClassLocalPattern,
			Origin:          SourceRelationOriginTOCExplicit,
			Provenance:      relationProvenance,
		},
	}
	grounds := buildQueryTokenGateMatchGrounds(
		caseIndex,
		candidateIndex,
		concern,
		seed,
		groundsPerItem,
		line,
	)
	projectedText := projectCandidateSourceText(
		SourceUnit{
			Role:    role,
			Body:    seed.Excerpt,
			UseCues: seed.Cues,
		},
		defaultResponseBudget.MaxExcerptCharacters,
	)
	return SourceCandidate{
		Source: CandidateSourceUnit{
			UnitID:            unitID,
			SourceID:          seed.PatternID,
			SourceRole:        role,
			Title:             seed.Title,
			Excerpt:           projectedText.Excerpt,
			ExcerptTruncated:  projectedText.ExcerptTruncated,
			UseCues:           projectedText.UseCues,
			UseCuesTruncated:  projectedText.UseCuesTruncated,
			PatternID:         seed.PatternID,
			PublicationStatus: "synthetic_stable",
			DirectRefs:        append([]string(nil), seed.Refs...),
			RelationProjection: &CandidateRelationProjection{
				SubjectPatternID: seed.PatternID,
				CanonicalUnitID:  canonicalUnitID,
				Relations:        relations,
				Truncated:        candidateIndex%3 == 0,
				OmittedAtLeast:   candidateIndex % 2,
			},
			Provenance: unitProvenance,
		},
		MatchGrounds: grounds,
	}
}

func buildQueryTokenGateMatchGrounds(
	caseIndex int,
	candidateIndex int,
	concern string,
	seed queryTokenGateCandidateSeed,
	groundCount int,
	line int,
) []MatchGround {
	tiers := []RetrievalTier{
		RetrievalTierExactSource,
		RetrievalTierAuthoredPhrase,
		RetrievalTierHeadingKeyword,
		RetrievalTierRoleLocalFTS,
	}
	grounds := make([]MatchGround, 0, groundCount)
	for groundIndex := 0; groundIndex < groundCount; groundIndex++ {
		tier := tiers[groundIndex%len(tiers)]
		evidenceText := fmt.Sprintf(
			"synthetic retrieval witness case=%d candidate=%d ground=%d pattern=%s concern=%s",
			caseIndex,
			candidateIndex,
			groundIndex,
			seed.PatternID,
			concern,
		)
		evidenceProvenance := queryTokenGateProvenance(
			"data/FPF/Readme.md",
			line+groundIndex+3,
			evidenceText,
		)
		grounds = append(grounds, MatchGround{
			Tier:         tier,
			ProbeField:   "query",
			SourceField:  "authored_heading_or_practical_use_cue",
			MatchedValue: fmt.Sprintf("%s candidate %d retrieval witness %d", seed.Title, candidateIndex, groundIndex),
			PhraseKind:   SourcePhraseKindExactProbeSpan,
			Evidence: &MatchGroundEvidence{
				UnitID:             fmt.Sprintf("synthetic:evidence:%02d:%02d:%02d", caseIndex, candidateIndex, groundIndex),
				PatternID:          seed.PatternID,
				SourceRole:         SourceUnitRolePatternBody,
				Provenance:         evidenceProvenance,
				ProjectionRelation: "navigation_candidate_for_pattern_body_source_witness",
			},
		})
	}
	return grounds
}

func queryTokenGateProvenance(path string, line int, content string) SourceProvenance {
	return SourceProvenance{
		SourcePath:     path,
		StartLine:      line,
		EndLine:        line + 2,
		ContentHash:    sourceContentHash(content),
		SourceRevision: queryTokenGateSyntheticRevision,
	}
}

func queryTokenGateSnapshot(t *testing.T) QuerySourceSnapshot {
	t.Helper()
	snapshot, err := NewQuerySourceSnapshot(
		SpecIndexSchemaVersion,
		queryTokenGateSyntheticRevision,
		queryTokenGateSyntheticReadmeSHA,
		queryTokenGateSyntheticSpecSHA,
	)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func assertQueryTokenGateCorpusDigest(t *testing.T, corpus []queryTokenGateCase) {
	t.Helper()
	encoded, err := json.Marshal(corpus)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(encoded)
	got := hex.EncodeToString(digest[:])
	if got != queryTokenGateCorpusDigest {
		t.Fatalf(
			"representative concern corpus digest = %s, want %s; corpus/schema changes require an explicit quantitative rebaseline",
			got,
			queryTokenGateCorpusDigest,
		)
	}
}

func assertQueryTokenGateSemantics(
	t *testing.T,
	canonical CandidateSet,
	working workingCandidateSet,
) {
	t.Helper()
	if canonical.Kind != working.Kind {
		t.Fatalf("working kind = %q, want %q", working.Kind, canonical.Kind)
	}
	wantBudget := projectResponseBudget(canonical.Truncation.Budget)
	if !reflect.DeepEqual(wantBudget, working.Truncation.Budget) {
		t.Fatalf("working response budget = %#v, want %#v", working.Truncation.Budget, canonical.Truncation.Budget)
	}
	if canonical.Truncation.Applied != working.Truncation.Applied ||
		canonical.Truncation.IncludedCandidates != working.Truncation.IncludedCandidates ||
		canonical.Truncation.OmittedAtLeast != working.Truncation.OmittedAtLeast {
		t.Fatalf("working truncation posture changed: got %#v, canonical %#v", working.Truncation, canonical.Truncation)
	}
	if len(canonical.Groups) != len(working.Groups) {
		t.Fatalf("working group count = %d, want %d", len(working.Groups), len(canonical.Groups))
	}
	for groupIndex, canonicalGroup := range canonical.Groups {
		workingGroup := working.Groups[groupIndex]
		if canonicalGroup.Role != workingGroup.Role {
			t.Fatalf("group %d role = %q, want %q", groupIndex, workingGroup.Role, canonicalGroup.Role)
		}
		if len(canonicalGroup.Candidates) != len(workingGroup.Candidates) {
			t.Fatalf(
				"group %d candidate count = %d, want %d",
				groupIndex,
				len(workingGroup.Candidates),
				len(canonicalGroup.Candidates),
			)
		}
		for candidateIndex, canonicalCandidate := range canonicalGroup.Candidates {
			workingCandidate := workingGroup.Candidates[candidateIndex]
			assertQueryTokenGateCandidateTextBudget(
				t,
				canonicalCandidate.Source,
				canonical.Truncation.Budget.MaxExcerptCharacters,
			)
			assertQueryTokenGateCandidateSemantics(
				t,
				canonicalCandidate.Source,
				workingCandidate.Source,
			)
		}
	}
}

func assertQueryTokenGateCandidateTextBudget(
	t *testing.T,
	candidate CandidateSourceUnit,
	limit int,
) {
	t.Helper()
	texts := []string{candidate.Excerpt}
	if candidate.UseCues != nil {
		texts = append(
			texts,
			candidate.UseCues.ConditionText,
			candidate.UseCues.FirstResultText,
			candidate.UseCues.StopReturnText,
		)
	}
	total := 0
	for _, text := range texts {
		total += len([]rune(text))
	}
	if total > limit {
		t.Fatalf("representative candidate %q uses %d source-text runes, budget %d", candidate.UnitID, total, limit)
	}
}

func assertQueryTokenGateCandidateSemantics(
	t *testing.T,
	canonical CandidateSourceUnit,
	working PublishedCandidateSourceUnit,
) {
	t.Helper()
	canonicalScalar := []any{
		canonical.UnitID,
		canonical.SourceID,
		canonical.SourceRole,
		canonical.Title,
		canonical.Excerpt,
		canonical.ExcerptTruncated,
		canonical.UseCuesTruncated,
		canonical.PatternID,
		canonical.ParentPatternID,
		canonical.PublicationStatus,
	}
	workingScalar := []any{
		working.UnitID,
		working.SourceID,
		working.SourceRole,
		working.Title,
		working.Excerpt,
		working.ExcerptTruncated,
		working.UseCuesTruncated,
		working.PatternID,
		working.ParentPatternID,
		working.PublicationStatus,
	}
	if !reflect.DeepEqual(canonicalScalar, workingScalar) {
		t.Fatalf("working candidate scalar semantics changed: got %#v, canonical %#v", workingScalar, canonicalScalar)
	}
	wantUseCues := cloneOptionalUseCues(canonical.UseCues)
	if !reflect.DeepEqual(wantUseCues, working.UseCues) {
		t.Fatalf("working candidate cues = %#v, want %#v", working.UseCues, canonical.UseCues)
	}
	if !reflect.DeepEqual(canonical.DirectRefs, working.DirectRefs) {
		t.Fatalf("working candidate direct refs = %#v, want %#v", working.DirectRefs, canonical.DirectRefs)
	}
	if working.DirectRefsTruncated || working.DirectRefsOmittedAtLeast != 0 {
		t.Fatalf("representative direct refs were reduced: %#v", working)
	}
	canonicalProjection := canonical.RelationProjection
	workingProjection := working.RelationProjection
	if canonicalProjection == nil || workingProjection == nil {
		t.Fatalf("representative relation projection missing: canonical=%#v working=%#v", canonicalProjection, workingProjection)
	}
	if canonicalProjection.Truncated != workingProjection.Truncated ||
		canonicalProjection.OmittedAtLeast != workingProjection.OmittedAtLeast {
		t.Fatalf("working relation truncation posture changed: got %#v, canonical %#v", workingProjection, canonicalProjection)
	}
	if len(canonicalProjection.Relations) != len(workingProjection.Relations) {
		t.Fatalf("working relation count = %d, want %d", len(workingProjection.Relations), len(canonicalProjection.Relations))
	}
	for relationIndex, canonicalRelation := range canonicalProjection.Relations {
		workingRelation := workingProjection.Relations[relationIndex]
		if workingRelation.Kind != canonicalRelation.Kind ||
			workingRelation.TargetPatternID != canonicalRelation.TargetPatternID {
			t.Fatalf("working relation %d = %#v, want kind/target from %#v", relationIndex, workingRelation, canonicalRelation)
		}
	}
}

func runQueryTokenGateCounter(
	t *testing.T,
	cases []queryTokenGateInputCase,
) queryTokenGateOutput {
	t.Helper()
	request := queryTokenGateInput{Schema: queryTokenGateSchema, Cases: cases}
	encodedRequest, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	python := strings.TrimSpace(os.Getenv("HAFT_QUERY_TOKEN_GATE_PYTHON"))
	if python == "" {
		python = "python3"
	}
	helperPath := filepath.Join("..", "..", "scripts", "fpf_query_token_count.py")
	command := exec.Command(python, helperPath)
	command.Stdin = bytes.NewReader(encodedRequest)
	stderr := bytes.Buffer{}
	command.Stderr = &stderr
	encodedOutput, err := command.Output()
	if err != nil {
		t.Fatalf("pinned o200k_base counter failed: %v\n%s", err, stderr.String())
	}

	output := queryTokenGateOutput{}
	if err := json.Unmarshal(encodedOutput, &output); err != nil {
		t.Fatalf("decode pinned token counter output: %v\n%s", err, encodedOutput)
	}
	if output.Schema != queryTokenGateResultSchema ||
		output.Encoding != queryTokenGateEncoding ||
		output.TokenizerDistribution != "tiktoken" ||
		output.TokenizerVersion != queryTokenGateTokenizerVersion ||
		output.EncodingAssetSHA256 != queryTokenGateEncodingAssetHash ||
		output.CalibrationTokens != queryTokenGateCalibrationTokens {
		t.Fatalf("unexpected tokenizer identity: %#v", output)
	}
	if len(output.Counts) != len(cases) {
		t.Fatalf("token counter returned %d cases, want %d", len(output.Counts), len(cases))
	}
	for index, count := range output.Counts {
		if count.CaseID != cases[index].CaseID {
			t.Fatalf("token counter case %d = %q, want %q", index, count.CaseID, cases[index].CaseID)
		}
		if count.CanonicalTokens <= 0 || count.WorkingTokens <= 0 {
			t.Fatalf("token counter returned non-positive counts: %#v", count)
		}
	}
	return output
}

func assertQueryTokenGateReduction(t *testing.T, measurements queryTokenGateOutput) {
	t.Helper()
	canonicalCounts := make([]int, 0, len(measurements.Counts))
	workingCounts := make([]int, 0, len(measurements.Counts))
	perCaseReductions := make([]float64, 0, len(measurements.Counts))
	for _, measurement := range measurements.Counts {
		reduction := 1 - float64(measurement.WorkingTokens)/float64(measurement.CanonicalTokens)
		canonicalCounts = append(canonicalCounts, measurement.CanonicalTokens)
		workingCounts = append(workingCounts, measurement.WorkingTokens)
		perCaseReductions = append(perCaseReductions, reduction)
		t.Logf(
			"case=%s canonical=%d working=%d reduction=%.1f%%",
			measurement.CaseID,
			measurement.CanonicalTokens,
			measurement.WorkingTokens,
			reduction*100,
		)
	}

	medianPerCaseReduction := medianFloat64(perCaseReductions)
	medianCanonical := medianInt(canonicalCounts)
	medianWorking := medianInt(workingCounts)
	ratioOfMediansReduction := 1 - medianWorking/medianCanonical
	t.Logf(
		"median_per_case_reduction=%.1f%% median_canonical=%.1f median_working=%.1f ratio_of_medians_reduction=%.1f%%",
		medianPerCaseReduction*100,
		medianCanonical,
		medianWorking,
		ratioOfMediansReduction*100,
	)
	if medianPerCaseReduction < queryTokenGateMinimumReduction {
		t.Fatalf(
			"median per-case o200k_base reduction = %.2f%%, require at least %.2f%%",
			medianPerCaseReduction*100,
			queryTokenGateMinimumReduction*100,
		)
	}
	if ratioOfMediansReduction < queryTokenGateMinimumReduction {
		t.Fatalf(
			"o200k_base ratio-of-medians reduction = %.2f%%, require at least %.2f%%",
			ratioOfMediansReduction*100,
			queryTokenGateMinimumReduction*100,
		)
	}
}

func medianInt(values []int) float64 {
	ordered := append([]int(nil), values...)
	sort.Ints(ordered)
	middle := len(ordered) / 2
	if len(ordered)%2 == 1 {
		return float64(ordered[middle])
	}
	return float64(ordered[middle-1]+ordered[middle]) / 2
}

func medianFloat64(values []float64) float64 {
	ordered := append([]float64(nil), values...)
	sort.Float64s(ordered)
	middle := len(ordered) / 2
	if len(ordered)%2 == 1 {
		return ordered[middle]
	}
	return (ordered[middle-1] + ordered[middle]) / 2
}
