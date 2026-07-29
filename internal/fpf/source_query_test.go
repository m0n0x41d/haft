package fpf

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestSourceProbePhrases_RemoveOnlyQueryScaffoldAndPreserveOrder(t *testing.T) {
	phrases := derivedSourcePhrases(CandidateProbe{
		Text: "What is the target system here?",
	})
	if len(phrases) != 2 ||
		phrases[0].Kind != SourcePhraseKindExactProbeSpan ||
		phrases[1].ProbeField != "text" ||
		phrases[1].Value != "target system" ||
		phrases[1].Kind != SourcePhraseKindScaffoldCompressed {
		t.Fatalf("derived source phrases = %#v, want exact span plus ordered scaffold-compressed target system", phrases)
	}

	exact := sourceUnitExactGroundingValues(SourceUnit{
		UnitID:   "readme:practical_use_card:system-in-context",
		SourceID: "SYSTEM-IN-CONTEXT",
		Role:     SourceUnitRolePracticalUseCard,
		Title:    "Make the current system question explicit",
	})
	if _, splitSourceID := exact["system"]; splitSourceID {
		t.Fatal("single SourceID component system must not become exact identity evidence")
	}
}

func TestCandidateSetExactOrPhraseGroundPolicyRejectsTokenUnion(t *testing.T) {
	tests := []struct {
		name    string
		grounds []MatchGround
		want    bool
	}{
		{
			name: "separate keyword tokens remain weak",
			grounds: []MatchGround{
				{
					Tier:         RetrievalTierHeadingKeyword,
					SourceField:  "keywords",
					MatchedValue: "institutional target and effect",
				},
				{
					Tier:         RetrievalTierHeadingKeyword,
					SourceField:  "keywords",
					MatchedValue: "performing U.System",
				},
				{
					Tier:         RetrievalTierRoleLocalFTS,
					SourceField:  sourceFieldTitleBodyToken,
					MatchedValue: "target",
				},
				{
					Tier:         RetrievalTierRoleLocalFTS,
					SourceField:  sourceFieldTitleBodyToken,
					MatchedValue: "system",
				},
			},
			want: false,
		},
		{
			name: "exact source identity is strong",
			grounds: []MatchGround{{
				Tier:        RetrievalTierExactSource,
				SourceField: sourceFieldExactIdentifierOrTitle,
			}},
			want: true,
		},
		{
			name: "authored phrase is strong",
			grounds: []MatchGround{{
				Tier:        RetrievalTierAuthoredPhrase,
				SourceField: "authored_phrases",
			}},
			want: true,
		},
		{
			name: "contiguous source phrase is strong",
			grounds: []MatchGround{{
				Tier:        RetrievalTierRoleLocalFTS,
				SourceField: sourceFieldDerivedPhrase,
			}},
			want: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidates := CandidateSet{
				Groups: []SourceCandidateGroup{{
					Candidates: []SourceCandidate{{
						MatchGrounds: test.grounds,
					}},
				}},
			}
			if got := candidateSetHasExactOrPhraseGround(candidates); got != test.want {
				t.Fatalf(
					"candidateSetHasExactOrPhraseGround() = %t, want %t",
					got,
					test.want,
				)
			}
		})
	}
}

func TestExactSourceAnchorsPreserveProbeFieldAndStructuredTokens(t *testing.T) {
	anchors := exactSourceAnchors(CandidateProbe{
		Text:         "Как составить план?",
		KnownContext: []string{"U.WorkPlan/A.15.2", "A.22.CGUS"},
	})

	want := map[string]bool{
		"known_context\x00U.WorkPlan": false,
		"known_context\x00A.15.2":     false,
		"known_context\x00A.22.CGUS":  false,
	}
	for _, anchor := range anchors {
		key := anchor.ProbeField + "\x00" + anchor.Value
		if _, expected := want[key]; expected {
			want[key] = true
		}
	}
	for key, found := range want {
		if !found {
			t.Fatalf("exact source anchors omit %q: %#v", key, anchors)
		}
	}
}

func TestSQLiteQueryIndex_DerivedPhraseDoesNotCrossSourceFieldBoundary(t *testing.T) {
	units := minimalValidSourceUnits()
	for index := range units {
		if units[index].Role != SourceUnitRolePatternBody {
			continue
		}
		units[index].Title = "target"
		units[index].Body = "system"
		units[index].Provenance.ContentHash = sourceContentHash(units[index].Body)
	}

	dbPath := filepath.Join(t.TempDir(), "fpf.db")
	if err := StoreSourceUnits(dbPath, units); err != nil {
		t.Fatalf("store source units: %v", err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open source query db: %v", err)
	}
	defer func() { _ = db.Close() }()

	batch, err := NewSQLiteQueryIndex(db).SearchSourceProbePhrases(
		[]SourceProbePhrase{{ProbeField: "text", Value: "target system", Kind: SourcePhraseKindExactProbeSpan}},
		[]SourceUnitRole{SourceUnitRolePatternBody},
	)
	if err != nil {
		t.Fatalf("search derived source phrase: %v", err)
	}
	if len(batch.Candidates) != 0 {
		t.Fatalf("phrase crossed title/body boundary: %#v", batch.Candidates)
	}
}

func TestQueryOptionalSourceSpoofFallsBackAndMergesBoundedBatch(t *testing.T) {
	units := sourceQueryP2FixtureUnits()
	dbPath := filepath.Join(t.TempDir(), "fpf.db")
	if err := StoreSourceUnits(dbPath, units); err != nil {
		t.Fatalf("store source units: %v", err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open source query db: %v", err)
	}
	defer func() { _ = db.Close() }()
	index := NewSQLiteQueryIndex(db)

	request := ConcernQuery{Text: "target system"}
	defaultResult, err := Query(index, request)
	if err != nil {
		t.Fatalf("default Query() error: %v", err)
	}
	defaultSet, ok := defaultResult.(CandidateSet)
	if !ok {
		t.Fatalf("default Query() result = %T, want CandidateSet", defaultResult)
	}
	if countSourceQueryP2Candidate(defaultSet, "unit:practical_use_card") != 1 ||
		countSourceQueryP2Candidate(defaultSet, "unit:toc_row") != 1 ||
		countSourceQueryP2Candidate(defaultSet, "spec:toc_row:a-2") != 0 ||
		countSourceQueryP2Candidate(defaultSet, "spec:toc_row:a-3") != 0 {
		t.Fatalf("default no-optional fallback changed: %#v", defaultSet.Groups)
	}
	if defaultSet.Truncation.Applied ||
		defaultSet.Truncation.OmittedAtLeast != 0 ||
		defaultSet.Truncation.IncludedCandidates != 2 {
		t.Fatalf("default no-optional truncation changed: %#v", defaultSet.Truncation)
	}

	producer := maliciousOptionalSourceProducer{batch: CandidateBatch{
		Candidates: []RetrievedCandidate{
			{
				Unit: SourceUnit{UnitID: "unit:toc_row"},
				MatchGrounds: []MatchGround{{
					Tier:         RetrievalTierAuthoredPhrase,
					ProbeField:   "text",
					SourceField:  sourceFieldDerivedPhrase,
					MatchedValue: "target system",
				}},
			},
			{
				Unit: SourceUnit{UnitID: "spec:toc_row:a-2"},
				MatchGrounds: []MatchGround{
					{
						Tier:         RetrievalTierAuthoredPhrase,
						ProbeField:   "text",
						SourceField:  "semantic_similarity",
						MatchedValue: "target system",
					},
					{
						Tier:         RetrievalTier("optional_vector"),
						ProbeField:   "text",
						SourceField:  "vector_similarity",
						MatchedValue: "0.91",
					},
				},
			},
			{
				Unit: SourceUnit{UnitID: "spec:toc_row:a-3"},
				MatchGrounds: []MatchGround{{
					Tier:         RetrievalTier("optional_vector"),
					ProbeField:   "text",
					SourceField:  sourceFieldDerivedPhrase,
					MatchedValue: "target system",
				}},
			},
		},
		Truncated:      true,
		OmittedAtLeast: 2,
		OmittedBasis:   []string{"malicious_optional_limit"},
	}}

	result, err := QueryWithCandidateProducers(
		index,
		request,
		[]CandidateProducer{producer},
	)
	if err != nil {
		t.Fatalf("QueryWithCandidateProducers() error: %v", err)
	}
	set, ok := result.(CandidateSet)
	if !ok {
		t.Fatalf("QueryWithCandidateProducers() result = %T, want CandidateSet", result)
	}
	if got := sourceQueryP2CandidateIDsForRole(set, SourceUnitRoleTOCRow); !equalSourceQueryP2Strings(
		got,
		[]string{"unit:toc_row", "spec:toc_row:a-2", "spec:toc_row:a-3"},
	) {
		t.Fatalf("merged ToC order = %v, want fallback witness then optional producer order", got)
	}
	if countSourceQueryP2Candidate(set, "unit:toc_row") != 1 {
		t.Fatalf("fallback/optional duplicate was not eliminated: %#v", set.Groups)
	}
	if !set.Truncation.Applied ||
		set.Truncation.OmittedAtLeast != 2 ||
		set.Truncation.IncludedCandidates != 4 ||
		!sourceQueryP2Contains(set.Truncation.Basis, "malicious_optional_limit") {
		t.Fatalf("optional truncation metadata was not preserved: %#v", set.Truncation)
	}

	fallbackWitness := findSourceQueryP2Candidate(t, set, "unit:toc_row")
	if !sourceQueryP2HasGround(fallbackWitness, func(ground MatchGround) bool {
		return ground.SourceField == sourceFieldPatternBodyDerivedPhrase &&
			ground.MatchedValue == "target system" &&
			ground.Evidence != nil &&
			ground.Evidence.UnitID == "unit:pattern_body" &&
			ground.Evidence.SourceRole == SourceUnitRolePatternBody &&
			ground.Evidence.ProjectionRelation == "same_pattern_id"
	}) {
		t.Fatalf("fallback candidate lacks exact pattern-body witness: %#v", fallbackWitness.MatchGrounds)
	}
	if !sourceQueryP2HasGround(fallbackWitness, func(ground MatchGround) bool {
		return ground.Tier == retrievalTierOptionalCandidate &&
			ground.SourceField == sourceFieldOptionalCandidateMatch &&
			ground.MatchedValue == "target system"
	}) {
		t.Fatalf("duplicate optional diagnostic was not retained safely: %#v", fallbackWitness.MatchGrounds)
	}

	optionalA2 := findSourceQueryP2Candidate(t, set, "spec:toc_row:a-2")
	if sourceQueryP2HasGround(optionalA2, matchGroundHasExactOrPhraseSourceGround) {
		t.Fatalf("optional A.2 candidate retained strong source discriminator: %#v", optionalA2.MatchGrounds)
	}
	if !sourceQueryP2HasGround(optionalA2, func(ground MatchGround) bool {
		return ground.Tier == retrievalTierOptionalCandidate &&
			ground.SourceField == "semantic_similarity" &&
			ground.MatchedValue == "target system"
	}) || !sourceQueryP2HasGround(optionalA2, func(ground MatchGround) bool {
		return ground.Tier == RetrievalTier("optional_vector") &&
			ground.SourceField == "vector_similarity" &&
			ground.MatchedValue == "0.91"
	}) {
		t.Fatalf("optional A.2 diagnostics were not preserved: %#v", optionalA2.MatchGrounds)
	}

	optionalA3 := findSourceQueryP2Candidate(t, set, "spec:toc_row:a-3")
	if sourceQueryP2HasGround(optionalA3, matchGroundHasExactOrPhraseSourceGround) ||
		!sourceQueryP2HasGround(optionalA3, func(ground MatchGround) bool {
			return ground.Tier == RetrievalTier("optional_vector") &&
				ground.SourceField == sourceFieldOptionalCandidateMatch &&
				ground.MatchedValue == "target system"
		}) {
		t.Fatalf("optional A.3 source-field spoof was not safely canonicalized: %#v", optionalA3.MatchGrounds)
	}

	cappedResult, err := QueryWithCandidateProducers(
		index,
		ConcernQuery{
			Text: "target system",
			ResponseBudget: ResponseBudget{
				MaxCandidatesPerRole: 2,
				MaxTotalCandidates:   3,
			},
		},
		[]CandidateProducer{producer},
	)
	if err != nil {
		t.Fatalf("capped optional fallback query error: %v", err)
	}
	cappedSet, ok := cappedResult.(CandidateSet)
	if !ok {
		t.Fatalf("capped optional fallback result = %T, want CandidateSet", cappedResult)
	}
	if got := sourceQueryP2CandidateIDsForRole(cappedSet, SourceUnitRoleTOCRow); !equalSourceQueryP2Strings(
		got,
		[]string{"unit:toc_row", "spec:toc_row:a-2"},
	) {
		t.Fatalf("capped ToC order = %v, want deterministic fallback then optional order", got)
	}
	if cappedSet.Truncation.IncludedCandidates != 3 ||
		cappedSet.Truncation.OmittedAtLeast != 3 ||
		!sourceQueryP2Contains(cappedSet.Truncation.Basis, "malicious_optional_limit") ||
		!sourceQueryP2Contains(cappedSet.Truncation.Basis, "response_budget") {
		t.Fatalf("combined producer/response cap metadata is inaccurate: %#v", cappedSet.Truncation)
	}

	strongResult, err := QueryWithCandidateProducers(
		index,
		ConcernQuery{Text: "TEST"},
		[]CandidateProducer{producer},
	)
	if err != nil {
		t.Fatalf("source-native exact query error: %v", err)
	}
	strongSet, ok := strongResult.(CandidateSet)
	if !ok {
		t.Fatalf("source-native exact query result = %T, want CandidateSet", strongResult)
	}
	sourceNative := findSourceQueryP2Candidate(t, strongSet, "unit:practical_use_card")
	if !sourceQueryP2HasGround(sourceNative, func(ground MatchGround) bool {
		return ground.Tier == RetrievalTierExactSource &&
			ground.SourceField == sourceFieldExactIdentifierOrTitle &&
			ground.MatchedValue == "TEST"
	}) {
		t.Fatalf("source-native exact grounding was changed: %#v", sourceNative.MatchGrounds)
	}
}

type maliciousOptionalSourceProducer struct {
	batch CandidateBatch
}

func (maliciousOptionalSourceProducer) ProducerID() string {
	return "malicious_optional_source"
}

func (producer maliciousOptionalSourceProducer) ProduceCandidates(
	CandidateProbe,
	[]SourceUnitRole,
) (CandidateBatch, error) {
	return producer.batch, nil
}

func sourceQueryP2FixtureUnits() []SourceUnit {
	units := minimalValidSourceUnits()
	for index := range units {
		if units[index].Role != SourceUnitRolePatternBody {
			continue
		}
		units[index].Body = "The authored pattern body names the target system exactly."
		units[index].Provenance.ContentHash = sourceContentHash(units[index].Body)
	}
	units = append(units, sourceQueryP2PatternUnits(
		"A.2",
		"spec:toc_row:a-2",
		"spec:pattern_body:a-2",
		20,
	)...)
	units = append(units, sourceQueryP2PatternUnits(
		"A.3",
		"spec:toc_row:a-3",
		"spec:pattern_body:a-3",
		30,
	)...)
	return units
}

func sourceQueryP2PatternUnits(
	patternID string,
	tocUnitID string,
	bodyUnitID string,
	startLine int,
) []SourceUnit {
	tocBody := "Source navigation row for " + patternID
	patternBody := "An unrelated authored pattern body for " + patternID
	return []SourceUnit{
		{
			UnitID:    tocUnitID,
			Role:      SourceUnitRoleTOCRow,
			Title:     "Optional navigation " + patternID,
			Body:      tocBody,
			PatternID: patternID,
			Provenance: SourceProvenance{
				SourcePath:     "FPF-Spec.md",
				StartLine:      startLine,
				EndLine:        startLine,
				ContentHash:    sourceContentHash(tocBody),
				SourceRevision: "test-revision",
			},
		},
		{
			UnitID:    bodyUnitID,
			SourceID:  patternID,
			Role:      SourceUnitRolePatternBody,
			Title:     "Optional pattern " + patternID,
			Body:      patternBody,
			PatternID: patternID,
			Provenance: SourceProvenance{
				SourcePath:     "FPF-Spec.md",
				StartLine:      startLine + 1,
				EndLine:        startLine + 1,
				ContentHash:    sourceContentHash(patternBody),
				SourceRevision: "test-revision",
			},
		},
	}
}

func countSourceQueryP2Candidate(set CandidateSet, unitID string) int {
	count := 0
	for _, group := range set.Groups {
		for _, candidate := range group.Candidates {
			if candidate.Source.UnitID == unitID {
				count++
			}
		}
	}
	return count
}

func sourceQueryP2CandidateIDsForRole(set CandidateSet, role SourceUnitRole) []string {
	for _, group := range set.Groups {
		if group.Role != role {
			continue
		}
		ids := make([]string, 0, len(group.Candidates))
		for _, candidate := range group.Candidates {
			ids = append(ids, candidate.Source.UnitID)
		}
		return ids
	}
	return nil
}

func findSourceQueryP2Candidate(t *testing.T, set CandidateSet, unitID string) SourceCandidate {
	t.Helper()
	for _, group := range set.Groups {
		for _, candidate := range group.Candidates {
			if candidate.Source.UnitID == unitID {
				return candidate
			}
		}
	}
	t.Fatalf("candidate set omits %s: %#v", unitID, set.Groups)
	return SourceCandidate{}
}

func sourceQueryP2HasGround(candidate SourceCandidate, predicate func(MatchGround) bool) bool {
	for _, ground := range candidate.MatchGrounds {
		if predicate(ground) {
			return true
		}
	}
	return false
}

func sourceQueryP2Contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func equalSourceQueryP2Strings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
