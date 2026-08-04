package fpf

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestSQLiteQueryIndex_SourceNativeTiersAndExactHydration(t *testing.T) {
	readmePath := filepath.Join("..", "..", "data", "FPF", "Readme.md")
	specPath := filepath.Join("..", "..", "data", "FPF", "FPF-Spec.md")
	units, err := LoadSourceUnits(readmePath, specPath, "query-test-revision")
	if err != nil {
		t.Fatalf("LoadSourceUnits() error: %v", err)
	}

	dbPath := filepath.Join(t.TempDir(), "fpf.db")
	if err := StoreSourceUnits(dbPath, units); err != nil {
		t.Fatalf("StoreSourceUnits() error: %v", err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open query db: %v", err)
	}
	defer func() { _ = db.Close() }()
	index := NewSQLiteQueryIndex(db)

	exactResult, err := Query(index, LookupQuery{Identifier: "A.7"})
	if err != nil {
		t.Fatalf("exact A.7 query error: %v", err)
	}
	exact, ok := exactResult.(ExactHit)
	if !ok {
		t.Fatalf("exact A.7 result = %T, want ExactHit", exactResult)
	}
	if exact.Unit.Role != SourceUnitRolePatternBody {
		t.Fatalf("exact A.7 role = %s, want pattern_body", exact.Unit.Role)
	}
	if !strings.Contains(exact.Unit.Body, "Solution") {
		t.Fatal("exact A.7 hydration must contain the full pattern Solution source")
	}

	relationResultExact, err := Query(index, LookupQuery{Identifier: "A.6.REL"})
	if err != nil {
		t.Fatalf("exact A.6.REL query error: %v", err)
	}
	relationExact, ok := relationResultExact.(ExactHit)
	if !ok {
		t.Fatalf("exact A.6.REL result = %T, want ExactHit", relationResultExact)
	}
	if relationExact.Unit.Role != SourceUnitRolePatternBody ||
		relationExact.Unit.Provenance.SourceRevision != "query-test-revision" {
		t.Fatalf("exact A.6.REL hydration lost role or revision: %#v", relationExact.Unit)
	}
	if !strings.Contains(relationExact.Unit.Body, "assertion") ||
		!strings.Contains(relationExact.Unit.Body, "occurrence") {
		t.Fatal("exact A.6.REL hydration lost the source-owned assertion/occurrence distinction")
	}

	titleResult, err := Query(index, LookupQuery{Identifier: "Abductive Loop"})
	if err != nil {
		t.Fatalf("exact title query error: %v", err)
	}
	titleExact, ok := titleResult.(ExactHit)
	if !ok {
		t.Fatalf("exact title result = %T, want ExactHit", titleResult)
	}
	if titleExact.Unit.Role != SourceUnitRolePatternBody || titleExact.Unit.PatternID != "B.5.2" {
		t.Fatalf("exact title hydrated %#v, want B.5.2 pattern_body", titleExact.Unit)
	}

	tocResult, err := Query(index, LookupQuery{
		Identifier: "A.7",
		Roles:      []SourceUnitRole{SourceUnitRoleTOCRow},
	})
	if err != nil {
		t.Fatalf("role-local A.7 query error: %v", err)
	}
	tocExact, ok := tocResult.(ExactHit)
	if !ok || tocExact.Unit.Role != SourceUnitRoleTOCRow {
		t.Fatalf("role-local A.7 result = %#v, want exact toc_row", tocResult)
	}

	phrase := strictDistinctionAuthoredPhrase(t, units)
	phraseResult, err := Query(index, ConcernQuery{
		Text: phrase,
	})
	if err != nil {
		t.Fatalf("authored phrase query error: %v", err)
	}
	phraseCandidates, ok := phraseResult.(CandidateSet)
	if !ok {
		t.Fatalf("authored phrase result = %T, want CandidateSet", phraseResult)
	}
	if !candidateSetHasTier(phraseCandidates, RetrievalTierAuthoredPhrase) {
		t.Fatalf("authored phrase tier not observable: %#v", phraseCandidates.Groups)
	}
	assertDefaultConcernRoles(t, phraseCandidates)

	relationResult, err := Query(index, ConcernQuery{
		Text: "How to introduce a new mechanism in FPF?",
		ResponseBudget: ResponseBudget{
			MaxRelationsPerCandidate: 100,
		},
	})
	if err != nil {
		t.Fatalf("E.20 relation projection query error: %v", err)
	}
	relationCandidates, ok := relationResult.(CandidateSet)
	if !ok {
		t.Fatalf("E.20 relation projection result = %T, want CandidateSet", relationResult)
	}
	e20Candidate := findCandidateSourceByPatternID(t, relationCandidates, "E.20")
	e20Body := findSourceUnitByRoleAndPatternID(t, units, SourceUnitRolePatternBody, "E.20")
	e20TOC := findSourceUnitByRoleAndPatternID(t, units, SourceUnitRoleTOCRow, "E.20")
	projection := e20Candidate.RelationProjection
	if projection == nil || projection.SubjectPatternID != "E.20" || projection.CanonicalUnitID != e20Body.UnitID {
		t.Fatalf("E.20 candidate relation projection lacks canonical body ownership: %#v", projection)
	}
	externalRelation := findSourceRelation(t, projection.Relations, SourceRelationKindCoordinatesWith, "G.X:EXT")
	if externalRelation.TargetClass != SourceRelationTargetClassAuthoredNonlocal || externalRelation.Provenance != e20TOC.Provenance {
		t.Fatalf("E.20 external relation lost target class or exact ToC provenance: %#v", externalRelation)
	}
	boundedRelationResult, err := Query(index, ConcernQuery{
		Text: "How to introduce a new mechanism in FPF?",
		ResponseBudget: ResponseBudget{
			MaxRelationsPerCandidate: 1,
		},
	})
	if err != nil {
		t.Fatalf("bounded E.20 relation projection query error: %v", err)
	}
	boundedProjection := findCandidateSourceByPatternID(t, boundedRelationResult.(CandidateSet), "E.20").RelationProjection
	if boundedProjection == nil || !boundedProjection.Truncated || len(boundedProjection.Relations) != 1 || boundedProjection.OmittedAtLeast < 1 {
		t.Fatalf("bounded relation projection lacks observable truncation: %#v", boundedProjection)
	}

	russianResult, err := Query(index, ConcernQuery{
		Text:            "Как выбрать подходящий способ работы с архитектурой?",
		EntityOfConcern: "architecture",
		KnownContext:    []string{"problem pressure", "candidate structures"},
		IntendedUse:     "compare structures",
	})
	if err != nil {
		t.Fatalf("Russian contextual query error: %v", err)
	}
	russianCandidates, ok := russianResult.(CandidateSet)
	if !ok {
		t.Fatalf("Russian contextual result = %T, want CandidateSet", russianResult)
	}
	if candidateSetSize(russianCandidates) < 2 {
		t.Fatalf("Russian contextual query returned %d candidates, want multiple", candidateSetSize(russianCandidates))
	}
	if !candidateSetHasContextGround(russianCandidates) {
		t.Fatalf("context expansion basis is not observable: %#v", russianCandidates.Groups)
	}

	practicalUseCases := []struct {
		name           string
		query          ConcernQuery
		cardUnitID     string
		directPatterns []string
	}{
		{
			name: "Russian problem-shaping entry",
			query: ConcernQuery{
				Text:            "Как превратить ранний сигнал в честную постановку проблемы, не протаскивая решение?",
				EntityOfConcern: "project concern",
				KnownContext: []string{
					"PROBLEM-SHAPING C.22.2 ProblemCard EntityOfConcern acceptance basis",
				},
				IntendedUse: "select the earliest truthful problem-side result",
			},
			cardUnitID:     "readme:practical_use_card:problem-shaping",
			directPatterns: []string{"C.22.2"},
		},
		{
			name: "English option-comparison entry",
			query: ConcernQuery{
				Text:            "How should we compare several alternatives without collapsing protected trade-offs or selecting a hidden winner?",
				EntityOfConcern: "OptionSet",
				KnownContext: []string{
					"OPTION-COMPARISON A.19.ECS C.18 C.11 evaluation characteristic space frontier comparison basis",
				},
				IntendedUse: "select the exact comparison result kind",
			},
			cardUnitID:     "readme:practical_use_card:option-comparison",
			directPatterns: []string{"A.19.ECS", "C.18", "C.11"},
		},
	}
	for _, useCase := range practicalUseCases {
		t.Run(useCase.name, func(t *testing.T) {
			result, queryErr := Query(index, useCase.query)
			if queryErr != nil {
				t.Fatalf("structured concern query error: %v", queryErr)
			}
			candidates, ok := result.(CandidateSet)
			if !ok {
				t.Fatalf("structured concern result = %T, want CandidateSet", result)
			}
			assertDefaultConcernRoles(t, candidates)
			if !candidateSetHasUnitID(candidates, useCase.cardUnitID) {
				t.Fatalf("structured concern omits practical-use card %s: %#v", useCase.cardUnitID, candidates.Groups)
			}
			for _, patternID := range useCase.directPatterns {
				if !candidateSetHasPatternID(candidates, patternID) {
					t.Fatalf("structured concern omits direct ToC pattern %s: %#v", patternID, candidates.Groups)
				}
			}
		})
	}

	mechanicalQuestions := []string{
		"rename file",
		"How should I rename this file?",
		"How should I format this code?",
		"How should I rotate this jpg?",
		"How should I use this file?",
		"How do I use this code?",
		"What should I do with this action?",
	}
	for _, question := range mechanicalQuestions {
		mechanicalResult, queryErr := Query(index, ConcernQuery{Text: question})
		if queryErr != nil {
			t.Fatalf("generic mechanical concern %q query error: %v", question, queryErr)
		}
		mechanicalAbstention, abstained := mechanicalResult.(Abstained)
		if !abstained {
			t.Fatalf("generic source overlap %q returned %#v, want Abstained", question, mechanicalResult)
		}
		if mechanicalAbstention.Reason != "insufficient_source_grounded_match" {
			t.Fatalf("mechanical abstention %q reason = %q, want explainable insufficient source ground", question, mechanicalAbstention.Reason)
		}
		if !stringSliceContainsText(mechanicalAbstention.MissingBasis, "source-corpus IDF weight") ||
			!stringSliceContainsText(mechanicalAbstention.MissingBasis, "document frequencies") ||
			!stringSliceContainsText(mechanicalAbstention.MissingBasis, "canonical pattern-body witnesses") {
			t.Fatalf("mechanical abstention %q missing basis is not explainable: %#v", question, mechanicalAbstention.MissingBasis)
		}
	}
	groundedResult, err := Query(index, ConcernQuery{Text: "architecture distinctions"})
	if err != nil {
		t.Fatalf("grounded two-lexeme concern query error: %v", err)
	}
	if _, ok := groundedResult.(CandidateSet); !ok {
		t.Fatalf("grounded two-lexeme concern result = %T, want CandidateSet", groundedResult)
	}
	descriptionResult, err := Query(index, ConcernQuery{
		Text: "How do I distinguish an object from its description and carrier?",
	})
	if err != nil {
		t.Fatalf("description/carrier concern query error: %v", err)
	}
	descriptionSet := descriptionResult.(CandidateSet)
	assertDefaultConcernRoles(t, descriptionSet)
	if !candidateSetHasUnitID(descriptionSet, "readme:practical_use_card:description-use") ||
		!candidateSetHasRole(descriptionSet, SourceUnitRoleTOCRow) {
		t.Fatalf("description/carrier navigation omits DESCRIPTION-USE card or ToC: %#v", descriptionSet.Groups)
	}

	targetResult, err := Query(index, ConcernQuery{Text: "What is the target system here?"})
	if err != nil {
		t.Fatalf("target-system concern query error: %v", err)
	}
	targetSet := targetResult.(CandidateSet)
	assertDefaultConcernRoles(t, targetSet)
	recognitionCard := findSourceCandidateByUnitID(t, targetSet, "readme:practical_use_card:system-recognition")
	assertNavigationExpansionGround(t, recognitionCard, "system")
	assertCandidateDirectRefs(t, recognitionCard, []string{
		"A.1.SCR",
		"A.1",
	})
	delimitationCard := findSourceCandidateByUnitID(t, targetSet, "readme:practical_use_card:system-delimitation")
	assertNavigationExpansionGround(t, delimitationCard, "system")
	assertCandidateDirectRefs(t, delimitationCard, []string{
		"B.1.2",
		"A.14",
		"C.13",
		"A.1",
		"C.11",
		"C.32.PAD",
		"C.2.1",
		"A.22",
	})
	targetWitness := findSourceCandidateByPatternID(t, targetSet, "C.26")
	// The current source has one exact "target system" pattern-body span in
	// C.26. C.32.PAD remains a direct ref on SYSTEM-DELIMITATION above, not an
	// exact phrase witness after the source revision changed its wording.
	assertProjectedPatternBodyPhraseGround(
		t,
		targetWitness,
		"target system",
		"C.26",
		SourcePhraseKindExactProbeSpan,
	)

	vignetteResult, err := Query(index, ConcernQuery{
		Text: "What is the system vignette here?",
	})
	if err != nil {
		t.Fatalf("system-vignette concern query error: %v", err)
	}
	vignetteSet := vignetteResult.(CandidateSet)
	assertDefaultConcernRoles(t, vignetteSet)
	vignetteCard := findSourceCandidateByUnitID(t, vignetteSet, "readme:practical_use_card:system-recognition")
	assertNavigationExpansionGround(t, vignetteCard, "system")
	vignetteWitness := findSourceCandidateByPatternID(t, vignetteSet, "A.21")
	assertProjectedPatternBodyPhraseGround(
		t,
		vignetteWitness,
		"system vignette",
		"A.21",
		SourcePhraseKindExactProbeSpan,
	)

	changeResult, err := Query(index, ConcernQuery{
		Text: "Which exact entities are parts of this system and which relations only cross its boundary?",
	})
	if err != nil {
		t.Fatalf("system-delimitation concern query error: %v", err)
	}
	delimitationSet, ok := changeResult.(CandidateSet)
	if !ok {
		t.Fatalf("system-delimitation concern result = %T, want CandidateSet", changeResult)
	}
	assertDefaultConcernRoles(t, delimitationSet)
	if !candidateSetHasUnitID(delimitationSet, "readme:practical_use_card:system-delimitation") {
		t.Fatalf("system-delimitation navigation omits SYSTEM-DELIMITATION: %#v", delimitationSet.Groups)
	}

	assertConcernIncludesPattern(t, index, "causality ladder", "C.28")
	assertConcernIncludesPattern(t, index, "work plan schedule intent", "A.15.2")

	budgetedResult, err := Query(index, ConcernQuery{
		Text: "architecture",
		ResponseBudget: ResponseBudget{
			MaxCandidatesPerRole: 1,
			MaxTotalCandidates:   1,
			MaxExcerptCharacters: 32,
		},
	})
	if err != nil {
		t.Fatalf("budgeted query error: %v", err)
	}
	budgeted := budgetedResult.(CandidateSet)
	if !budgeted.Truncation.Applied || budgeted.Truncation.IncludedCandidates != 1 {
		t.Fatalf("budget truncation not explicit: %#v", budgeted.Truncation)
	}
	if textLength := candidateSourceTextRuneCount(budgeted.Groups[0].Candidates[0].Source); textLength > 32 {
		t.Fatalf("candidate source-text length = %d, want strict total budget 32", textLength)
	}
}

func TestSQLiteQueryIndex_ConcernPinsExactSourceAnchorsBeforeTightResponseBudget(t *testing.T) {
	readmePath := filepath.Join("..", "..", "data", "FPF", "Readme.md")
	specPath := filepath.Join("..", "..", "data", "FPF", "FPF-Spec.md")
	units, err := LoadSourceUnits(readmePath, specPath, "exact-anchor-test-revision")
	if err != nil {
		t.Fatalf("LoadSourceUnits() error: %v", err)
	}

	dbPath := filepath.Join(t.TempDir(), "fpf.db")
	if err := StoreSourceUnits(dbPath, units); err != nil {
		t.Fatalf("StoreSourceUnits() error: %v", err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open query db: %v", err)
	}
	defer func() { _ = db.Close() }()
	index := NewSQLiteQueryIndex(db)
	budget := ResponseBudget{
		MaxCandidatesPerRole: 1,
		MaxTotalCandidates:   1,
	}

	tests := []struct {
		name         string
		text         string
		knownContext string
		patternID    string
		matchedValue string
	}{
		{
			name:         "WorkPlan identifier alongside its kind",
			text:         "How to model a plan or schedule?",
			knownContext: "U.WorkPlan/A.15.2",
			patternID:    "A.15.2",
			matchedValue: "A.15.2",
		},
		{
			name:         "CGUS exact pattern id",
			text:         "How do I preserve branches joins cycles and alternatives behind one walkthrough?",
			knownContext: "A.22.CGUS",
			patternID:    "A.22.CGUS",
			matchedValue: "A.22.CGUS",
		},
		{
			name:         "CGUS exact source title",
			text:         "Как удержать ветвления и возвраты?",
			knownContext: "Constraint-Governed Unfolding Structure",
			patternID:    "A.22.CGUS",
			matchedValue: "Constraint-Governed Unfolding Structure",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, queryErr := Query(index, ConcernQuery{
				Text:           test.text,
				KnownContext:   []string{test.knownContext},
				ResponseBudget: budget,
			})
			if queryErr != nil {
				t.Fatalf("Query() error: %v", queryErr)
			}
			set, ok := result.(CandidateSet)
			if !ok {
				t.Fatalf("Query() result = %T, want CandidateSet", result)
			}
			if candidateSetSize(set) != 1 || !candidateSetHasPatternID(set, test.patternID) {
				t.Fatalf("tight-budget candidates omit exact %s anchor: %#v", test.patternID, set.Groups)
			}
			candidate := findSourceCandidateByPatternID(t, set, test.patternID)
			if !candidateHasExactSourceGround(candidate, "known_context", test.matchedValue) {
				t.Fatalf("exact %s match ground is not observable: %#v", test.patternID, candidate.MatchGrounds)
			}
		})
	}

	rawResult, err := Query(index, ConcernQuery{Text: "ывапролджэюнаносюжет"})
	if err != nil {
		t.Fatalf("unrelated raw-language Query() error: %v", err)
	}
	if _, ok := rawResult.(Abstained); !ok {
		t.Fatalf("unrelated raw-language result = %T, want honest Abstained", rawResult)
	}
}

func TestSQLiteQueryIndex_PlannedPatternIDReturnsTransparentTOCExactHit(t *testing.T) {
	units := minimalValidSourceUnits()
	plannedBody := "| Z.99 | Planned Pattern | Planned |"
	units = append(units, SourceUnit{
		UnitID:            "spec:toc_row:z-99",
		SourceID:          "Z.99",
		Role:              SourceUnitRoleTOCRow,
		Title:             "Planned Pattern",
		Body:              plannedBody,
		PatternID:         "Z.99",
		PublicationStatus: "Planned",
		Provenance: SourceProvenance{
			SourcePath:     "FPF-Spec.md",
			StartLine:      99,
			EndLine:        99,
			ContentHash:    sourceContentHash(plannedBody),
			SourceRevision: "test-revision",
		},
	})

	dbPath := filepath.Join(t.TempDir(), "fpf.db")
	if err := StoreSourceUnits(dbPath, units); err != nil {
		t.Fatalf("StoreSourceUnits() rejected planned ToC-only pattern: %v", err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open query db: %v", err)
	}
	defer func() { _ = db.Close() }()

	result, err := Query(NewSQLiteQueryIndex(db), LookupQuery{Identifier: "Z.99"})
	if err != nil {
		t.Fatalf("planned exact lookup error: %v", err)
	}
	exact, ok := result.(ExactHit)
	if !ok {
		t.Fatalf("planned exact lookup = %T, want ExactHit", result)
	}
	if exact.Unit.Role != SourceUnitRoleTOCRow || exact.Unit.PublicationStatus != "Planned" {
		t.Fatalf("planned exact lookup hid publication state: %#v", exact.Unit)
	}
	if exact.Unit.Body != plannedBody || exact.Unit.Provenance.ContentHash == "" {
		t.Fatalf("planned exact lookup must return the transparent authored ToC unit: %#v", exact.Unit)
	}
}

func TestSQLiteQueryIndex_SourceRelationsRoundTripThroughNormalizedRows(t *testing.T) {
	units, expected := minimalValidSourceUnitsWithRelation()
	dbPath := filepath.Join(t.TempDir(), "fpf.db")
	if err := StoreSourceUnits(dbPath, units); err != nil {
		t.Fatalf("StoreSourceUnits() error: %v", err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open query db: %v", err)
	}
	defer func() { _ = db.Close() }()

	var relationRows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM source_unit_relations`).Scan(&relationRows); err != nil {
		t.Fatalf("count source relation rows: %v", err)
	}
	if relationRows != 1 {
		t.Fatalf("source relation rows = %d, want one normalized row", relationRows)
	}

	var foreignKeyTarget string
	foreignKeys, err := db.Query(`PRAGMA foreign_key_list(source_unit_relations)`)
	if err != nil {
		t.Fatalf("inspect source relation foreign key: %v", err)
	}
	defer func() { _ = foreignKeys.Close() }()
	for foreignKeys.Next() {
		var id int
		var sequence int
		var table string
		var from string
		var to string
		var onUpdate string
		var onDelete string
		var match string
		if err := foreignKeys.Scan(&id, &sequence, &table, &from, &to, &onUpdate, &onDelete, &match); err != nil {
			t.Fatalf("scan source relation foreign key: %v", err)
		}
		if from == "unit_id" && to == "unit_id" {
			foreignKeyTarget = table
		}
	}
	if foreignKeyTarget != "source_units" {
		t.Fatalf("source relation unit FK target = %q, want source_units", foreignKeyTarget)
	}

	result, err := Query(NewSQLiteQueryIndex(db), LookupQuery{Identifier: "Z.99"})
	if err != nil {
		t.Fatalf("relation-bearing exact lookup error: %v", err)
	}
	exact, ok := result.(ExactHit)
	if !ok {
		t.Fatalf("relation-bearing exact lookup = %T, want ExactHit", result)
	}
	if len(exact.Unit.Relations) != 1 || exact.Unit.Relations[0] != expected {
		t.Fatalf("hydrated source relations = %#v, want %#v", exact.Unit.Relations, expected)
	}
}

func TestSQLiteQueryIndex_UnresolvedAuthoredDirectRefNeedsMatchingCanonicalRelation(t *testing.T) {
	units, expected := minimalValidSourceUnitsWithUnresolvedAuthoredReference()
	dbPath := filepath.Join(t.TempDir(), "fpf.db")
	if err := StoreSourceUnits(dbPath, units); err != nil {
		t.Fatalf("StoreSourceUnits() rejected corroborated unresolved authored reference: %v", err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open query db: %v", err)
	}
	defer func() { _ = db.Close() }()

	result, err := Query(NewSQLiteQueryIndex(db), LookupQuery{Identifier: "Z.99"})
	if err != nil {
		t.Fatalf("unresolved-reference exact lookup error: %v", err)
	}
	exact, ok := result.(ExactHit)
	if !ok || len(exact.Unit.Relations) != 1 || exact.Unit.Relations[0] != expected {
		t.Fatalf("unresolved authored relation round trip = %#v, want %#v", result, expected)
	}

	unbacked := make([]SourceUnit, len(units))
	copy(unbacked, units)
	for index := range unbacked {
		if unbacked[index].Role == SourceUnitRolePatternBody && unbacked[index].PatternID == "Z.99" {
			unbacked[index].Relations = nil
		}
	}
	err = StoreSourceUnits(filepath.Join(t.TempDir(), "fpf.db"), unbacked)
	if err == nil || !strings.Contains(err.Error(), "has no addressable target") {
		t.Fatalf("StoreSourceUnits() error = %v, want unbacked missing-reference rejection", err)
	}
}

func TestVerifySourceQueryIndexDBRejectsSourceRelationRowDrift(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*testing.T, *sql.DB)
		wantError string
	}{
		{
			name: "missing row",
			mutate: func(t *testing.T, db *sql.DB) {
				t.Helper()
				if _, err := db.Exec(`DELETE FROM source_unit_relations WHERE unit_id = 'spec:toc_row:z-99'`); err != nil {
					t.Fatalf("delete source relation: %v", err)
				}
			},
			wantError: "source relation count mismatch",
		},
		{
			name: "extra row",
			mutate: func(t *testing.T, db *sql.DB) {
				t.Helper()
				_, err := db.Exec(`
					INSERT INTO source_unit_relations (
						unit_id, relation_ordinal, relation_kind, target_pattern_id,
						target_class, origin, source_path, start_line, end_line,
						content_hash, source_revision
					)
					SELECT unit_id, 1, 'informs', target_pattern_id,
						target_class, origin, source_path, start_line, end_line,
						content_hash, source_revision
					FROM source_unit_relations
					WHERE unit_id = 'spec:toc_row:z-99' AND relation_ordinal = 0`)
				if err != nil {
					t.Fatalf("insert extra source relation: %v", err)
				}
			},
			wantError: "source relation count mismatch",
		},
		{
			name: "valid but changed relation",
			mutate: func(t *testing.T, db *sql.DB) {
				t.Helper()
				_, err := db.Exec(`
					UPDATE source_unit_relations
					SET relation_kind = 'informs'
					WHERE unit_id = 'spec:toc_row:z-99' AND relation_ordinal = 0`)
				if err != nil {
					t.Fatalf("change source relation: %v", err)
				}
			},
			wantError: "source relation integrity mismatch",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			units, _ := minimalValidSourceUnitsWithRelation()
			dbPath := filepath.Join(t.TempDir(), "fpf.db")
			if err := StoreSourceUnits(dbPath, units); err != nil {
				t.Fatalf("StoreSourceUnits() error: %v", err)
			}
			db, err := sql.Open("sqlite", dbPath)
			if err != nil {
				t.Fatalf("open query db: %v", err)
			}
			defer func() { _ = db.Close() }()

			test.mutate(t, db)
			err = VerifySourceQueryIndexDB(db)
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("VerifySourceQueryIndexDB() error = %v, want %q", err, test.wantError)
			}
		})
	}
}

func TestVerifySourceQueryIndexReadOnlyDBRejectsDerivedProjectionDriftWithoutMutation(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*testing.T, *sql.DB)
		wantError string
	}{
		{
			name: "same-count FTS title update",
			mutate: func(t *testing.T, db *sql.DB) {
				execSourceIndexMutation(t, db, `
					UPDATE source_units_fts
					SET title = 'forged title'
					WHERE unit_id = 'unit:practical_use_card'`)
			},
			wantError: "source FTS projection mismatch",
		},
		{
			name: "same-count FTS body update",
			mutate: func(t *testing.T, db *sql.DB) {
				execSourceIndexMutation(t, db, `
					UPDATE source_units_fts
					SET body = 'forged body'
					WHERE unit_id = 'unit:practical_use_card'`)
			},
			wantError: "source FTS projection mismatch",
		},
		{
			name: "same-count FTS authored-phrases update",
			mutate: func(t *testing.T, db *sql.DB) {
				execSourceIndexMutation(t, db, `
					UPDATE source_units_fts
					SET authored_phrases = 'forged phrase'
					WHERE unit_id = 'unit:practical_use_card'`)
			},
			wantError: "source FTS projection mismatch",
		},
		{
			name: "same-count FTS keywords update",
			mutate: func(t *testing.T, db *sql.DB) {
				execSourceIndexMutation(t, db, `
					UPDATE source_units_fts
					SET keywords = 'forged-keyword'
					WHERE unit_id = 'unit:practical_use_card'`)
			},
			wantError: "source FTS projection mismatch",
		},
		{
			name: "same-count keyword replacement",
			mutate: func(t *testing.T, db *sql.DB) {
				execSourceIndexMutation(t, db, `
					DELETE FROM source_keywords
					WHERE unit_id = 'unit:practical_use_card' AND keyword = 'canonical-keyword'`)
				execSourceIndexMutation(t, db, `
					INSERT INTO source_keywords (unit_id, keyword)
					VALUES ('unit:practical_use_card', 'forged-keyword')`)
			},
			wantError: "source keyword projection mismatch",
		},
		{
			name: "same-count authored-phrase replacement",
			mutate: func(t *testing.T, db *sql.DB) {
				execSourceIndexMutation(t, db, `
					DELETE FROM source_authored_phrases
					WHERE unit_id = 'unit:practical_use_card' AND phrase = 'canonical phrase'`)
				execSourceIndexMutation(t, db, `
					INSERT INTO source_authored_phrases (unit_id, phrase)
					VALUES ('unit:practical_use_card', 'forged phrase')`)
			},
			wantError: "source authored-phrase projection mismatch",
		},
		{
			name: "same-count direct-reference replacement with valid target",
			mutate: func(t *testing.T, db *sql.DB) {
				execSourceIndexMutation(t, db, `
					DELETE FROM source_unit_refs
					WHERE unit_id = 'unit:practical_use_card' AND ref_id = 'A.1'`)
				execSourceIndexMutation(t, db, `
					INSERT INTO source_unit_refs (unit_id, ref_id)
					VALUES ('unit:practical_use_card', 'A.1:1')`)
			},
			wantError: "source direct-reference projection mismatch",
		},
		{
			name: "ordinary table substituted for FTS5 virtual table",
			mutate: func(t *testing.T, db *sql.DB) {
				execSourceIndexMutation(t, db, `
					CREATE TABLE source_units_fts_replacement AS
					SELECT unit_id, source_role, title, body, authored_phrases, keywords
					FROM source_units_fts`)
				execSourceIndexMutation(t, db, `DROP TABLE source_units_fts`)
				execSourceIndexMutation(t, db, `
					ALTER TABLE source_units_fts_replacement
					RENAME TO source_units_fts`)
			},
			wantError: "source FTS schema mismatch",
		},
		{
			name: "FTS5 shadow data deletion",
			mutate: func(t *testing.T, db *sql.DB) {
				result := execSourceIndexMutation(t, db, `
					DELETE FROM source_units_fts_data
					WHERE id = (SELECT MAX(id) FROM source_units_fts_data)`)
				rows, err := result.RowsAffected()
				if err != nil {
					t.Fatalf("read FTS5 shadow deletion count: %v", err)
				}
				if rows != 1 {
					t.Fatalf("FTS5 shadow deletion affected %d rows, want one", rows)
				}
			},
			wantError: "SQLite integrity check failed",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			units := minimalValidSourceUnits()
			for index := range units {
				if units[index].Role != SourceUnitRolePracticalUseCard {
					continue
				}
				units[index].AuthoredPhrases = []string{"canonical phrase"}
				units[index].Keywords = []string{"canonical-keyword"}
			}

			dbPath := filepath.Join(t.TempDir(), "fpf.db")
			if err := StoreSourceUnits(dbPath, units); err != nil {
				t.Fatalf("StoreSourceUnits() error: %v", err)
			}
			writable, err := sql.Open("sqlite", dbPath)
			if err != nil {
				t.Fatalf("open writable query db: %v", err)
			}
			test.mutate(t, writable)
			if err := writable.Close(); err != nil {
				t.Fatalf("close writable query db: %v", err)
			}

			assertReadOnlySourceIndexFailure(t, dbPath, test.wantError)
		})
	}
}

func execSourceIndexMutation(t *testing.T, db *sql.DB, statement string) sql.Result {
	t.Helper()
	result, err := db.Exec(statement)
	if err != nil {
		t.Fatalf("mutate source index: %v", err)
	}
	return result
}

func assertReadOnlySourceIndexFailure(t *testing.T, dbPath, wantError string) {
	t.Helper()
	beforeBytes := sourceIndexBytes(t, dbPath)
	beforeObjects := sourceIndexObjectCount(t, dbPath)
	readOnly := openSourceIndexReadOnlyTestDB(t, dbPath)
	err := VerifySourceQueryIndexReadOnlyDB(readOnly)
	if closeErr := readOnly.Close(); closeErr != nil {
		t.Fatalf("close read-only source index: %v", closeErr)
	}
	if err == nil || !strings.Contains(err.Error(), wantError) {
		t.Fatalf("VerifySourceQueryIndexReadOnlyDB() error = %v, want %q", err, wantError)
	}
	if strings.Contains(strings.ToLower(err.Error()), "readonly") {
		t.Fatalf("read-only verifier attempted a write: %v", err)
	}
	afterBytes := sourceIndexBytes(t, dbPath)
	if string(afterBytes) != string(beforeBytes) {
		t.Fatal("failed read-only verification changed source index bytes")
	}
	afterObjects := sourceIndexObjectCount(t, dbPath)
	if afterObjects != beforeObjects {
		t.Fatalf(
			"failed read-only verification changed SQLite object count from %d to %d",
			beforeObjects,
			afterObjects,
		)
	}
}

func openSourceIndexReadOnlyTestDB(t *testing.T, dbPath string) *sql.DB {
	t.Helper()
	absolutePath, err := filepath.Abs(filepath.Clean(dbPath))
	if err != nil {
		t.Fatalf("resolve source index path: %v", err)
	}
	readOnlyURI := url.URL{Scheme: "file", Path: absolutePath}
	query := readOnlyURI.Query()
	query.Set("mode", "ro")
	readOnlyURI.RawQuery = query.Encode()
	db, err := sql.Open("sqlite", readOnlyURI.String())
	if err != nil {
		t.Fatalf("open source index read-only: %v", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		t.Fatalf("ping source index read-only: %v", err)
	}
	return db
}

func sourceIndexBytes(t *testing.T, dbPath string) []byte {
	t.Helper()
	content, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("read source index: %v", err)
	}
	return content
}

func sourceIndexObjectCount(t *testing.T, dbPath string) int {
	t.Helper()
	readOnly := openSourceIndexReadOnlyTestDB(t, dbPath)
	defer func() { _ = readOnly.Close() }()
	var count int
	if err := readOnly.QueryRow(`SELECT COUNT(*) FROM sqlite_master`).Scan(&count); err != nil {
		t.Fatalf("count source index objects: %v", err)
	}
	return count
}

func strictDistinctionAuthoredPhrase(t *testing.T, units []SourceUnit) string {
	t.Helper()
	for _, unit := range units {
		if unit.Role == SourceUnitRoleTOCRow && unit.PatternID == "A.7" && len(unit.AuthoredPhrases) > 0 {
			return unit.AuthoredPhrases[0]
		}
	}
	t.Fatal("A.7 ToC authored phrase not found")
	return ""
}

func TestRebuildSourceQueryIndexAtomic_PreservesPreviousFileOnFailure(t *testing.T) {
	target := filepath.Join(t.TempDir(), "fpf.db")
	units := minimalValidSourceUnits()

	err := RebuildSourceQueryIndexAtomic(target, func(temporary string) error {
		return StoreSourceUnits(temporary, units)
	})
	if err != nil {
		t.Fatalf("initial atomic build error: %v", err)
	}
	before, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read initial target: %v", err)
	}

	err = RebuildSourceQueryIndexAtomic(target, func(temporary string) error {
		if err := StoreSourceUnits(temporary, units); err != nil {
			return err
		}
		return errors.New("late build failure")
	})
	if err == nil || !strings.Contains(err.Error(), "late build failure") {
		t.Fatalf("expected late build failure, got %v", err)
	}
	after, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read preserved target: %v", err)
	}
	if string(after) != string(before) {
		t.Fatal("failed atomic rebuild changed the previous target")
	}

	err = RebuildSourceQueryIndexAtomic(target, func(temporary string) error {
		db, openErr := sql.Open("sqlite", temporary)
		if openErr != nil {
			return openErr
		}
		defer func() { _ = db.Close() }()
		return EnsureSourceQuerySchemaDB(db)
	})
	if err == nil || !strings.Contains(err.Error(), "produced no") {
		t.Fatalf("expected fail-loud empty grammar error, got %v", err)
	}
	afterGrammarFailure, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target after grammar failure: %v", err)
	}
	if string(afterGrammarFailure) != string(before) {
		t.Fatal("grammar-invalid rebuild changed the previous target")
	}

	brokenCues := minimalValidSourceUnits()
	for index := range brokenCues {
		if brokenCues[index].Role == SourceUnitRolePracticalUseCard {
			brokenCues[index].UseCues.FirstResultText = ""
		}
	}
	err = RebuildSourceQueryIndexAtomic(target, func(temporary string) error {
		return StoreSourceUnits(temporary, brokenCues)
	})
	if err != nil {
		t.Fatalf("degraded practical-use cue rebuild error: %v", err)
	}
	readOnly := openSourceIndexReadOnlyTestDB(t, target)
	defer func() { _ = readOnly.Close() }()
	if err := VerifySourceQueryIndexReadOnlyDB(readOnly); err != nil {
		t.Fatalf("degraded practical-use source runtime is invalid: %v", err)
	}
}

func TestRebuildSourceIndexAtomic_CustomFinalVerifierPreservesPreviousFile(t *testing.T) {
	target := filepath.Join(t.TempDir(), "fpf.db")
	units := minimalValidSourceUnits()
	build := func(temporary string) error {
		return StoreSourceUnits(temporary, units)
	}
	if err := RebuildSourceQueryIndexAtomic(target, build); err != nil {
		t.Fatalf("initial query-only build error: %v", err)
	}
	before, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read initial target: %v", err)
	}

	verify := func(db *sql.DB) error {
		if err := VerifySourceQueryIndexDB(db); err != nil {
			return err
		}
		return errors.New("injected combined artifact failure")
	}
	err = RebuildSourceIndexAtomic(target, build, verify)
	if err == nil || !strings.Contains(err.Error(), "injected combined artifact failure") {
		t.Fatalf("custom verifier error = %v", err)
	}
	after, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read preserved target: %v", err)
	}
	if string(after) != string(before) {
		t.Fatal("custom-verifier failure changed the previous target")
	}

	err = RebuildSourceIndexAtomic(target, build, nil)
	if err == nil || !strings.Contains(err.Error(), "final verifier is required") {
		t.Fatalf("nil verifier error = %v", err)
	}
}

func TestSQLiteQueryIndex_RoleLocalFTSOmissionCountsDistinctCandidates(t *testing.T) {
	units := minimalValidSourceUnits()
	for index := range sourceCandidateProducerLimit + 1 {
		body := "overlap signal"
		units = append(units, SourceUnit{
			UnitID:     fmt.Sprintf("unit:overlap:%03d", index),
			SourceID:   fmt.Sprintf("OVERLAP-%03d", index),
			Role:       SourceUnitRolePracticalUseCard,
			Title:      fmt.Sprintf("Overlap %03d", index),
			Body:       body,
			DirectRefs: []string{"A.1"},
			UseCues: SourceUseCues{
				ConditionText:   "A source-owned situation",
				FirstResultText: "A source-owned first result",
				StopReturnText:  "A source-owned stop or return condition",
			},
			Provenance: SourceProvenance{
				SourcePath:     "Readme.md",
				StartLine:      index + 100,
				EndLine:        index + 100,
				ContentHash:    sourceContentHash(body),
				SourceRevision: "test-revision",
			},
		})
	}

	dbPath := filepath.Join(t.TempDir(), "fpf.db")
	if err := StoreSourceUnits(dbPath, units); err != nil {
		t.Fatalf("StoreSourceUnits() error: %v", err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open query db: %v", err)
	}
	defer func() { _ = db.Close() }()

	index := SQLiteQueryIndex{db: db}
	batch, err := index.SearchRoleLocalFTS(
		CandidateProbe{Text: "overlap signal", IntendedUse: "overlap signal"},
		[]SourceUnitRole{SourceUnitRolePracticalUseCard},
	)
	if err != nil {
		t.Fatalf("SearchRoleLocalFTS() error: %v", err)
	}
	if !batch.Truncated {
		t.Fatal("SearchRoleLocalFTS() did not report producer truncation")
	}
	if batch.OmittedAtLeast != 1 {
		t.Fatalf("OmittedAtLeast = %d, want one distinct source candidate", batch.OmittedAtLeast)
	}
}

func TestQueryCandidateTextBudgetIncludesUseCuesAndInspectHydratesFullSource(t *testing.T) {
	units := minimalValidSourceUnits()
	var full SourceUnit
	for index := range units {
		if units[index].Role != SourceUnitRolePracticalUseCard {
			continue
		}
		units[index].Body = "budgetprobe " + strings.Repeat("body-source ", 80)
		units[index].UseCues = SourceUseCues{
			ConditionText:   strings.Repeat("condition-source ", 80),
			FirstResultText: strings.Repeat("first-result-source ", 80),
			StopReturnText:  strings.Repeat("stop-return-source ", 80),
		}
		units[index].Provenance.ContentHash = sourceContentHash(units[index].Body)
		full = units[index]
	}

	dbPath := filepath.Join(t.TempDir(), "fpf.db")
	if err := StoreSourceUnits(dbPath, units); err != nil {
		t.Fatalf("StoreSourceUnits() error: %v", err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open query db: %v", err)
	}
	defer func() { _ = db.Close() }()
	index := NewSQLiteQueryIndex(db)

	result, err := Query(index, ConcernQuery{
		Text: "budgetprobe body-source",
		ResponseBudget: ResponseBudget{
			MaxCandidatesPerRole: 1,
			MaxTotalCandidates:   1,
			MaxExcerptCharacters: 37,
		},
	})
	if err != nil {
		t.Fatalf("budgeted Query() error: %v", err)
	}
	set, ok := result.(CandidateSet)
	if !ok || len(set.Groups) != 1 || len(set.Groups[0].Candidates) != 1 {
		t.Fatalf("budgeted Query() result = %#v, want one candidate", result)
	}
	candidate := set.Groups[0].Candidates[0].Source
	if candidate.UseCues == nil {
		t.Fatal("practical-use candidate omitted structured use cues")
	}
	if !candidate.ExcerptTruncated || !candidate.UseCuesTruncated {
		t.Fatalf("candidate truncation is not observable: %#v", candidate)
	}
	if length := candidateSourceTextRuneCount(candidate); length > 37 {
		t.Fatalf("candidate source-text length = %d, want strict total budget 37", length)
	}

	inspectResult, err := Query(index, InspectQuery{
		Identifier: "TEST",
		Roles:      []SourceUnitRole{SourceUnitRolePracticalUseCard},
	})
	if err != nil {
		t.Fatalf("InspectQuery error: %v", err)
	}
	exact, ok := inspectResult.(ExactHit)
	if !ok {
		t.Fatalf("InspectQuery result = %T, want ExactHit", inspectResult)
	}
	if exact.Unit.Body != full.Body || exact.Unit.UseCues != full.UseCues {
		t.Fatal("InspectQuery did not hydrate the full unbudgeted source unit")
	}
}

func TestSQLiteQueryIndex_ConcernAbstainsWithoutSourceBasis(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "fpf.db")
	if err := StoreSourceUnits(dbPath, minimalValidSourceUnits()); err != nil {
		t.Fatalf("StoreSourceUnits() error: %v", err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open query db: %v", err)
	}
	defer func() { _ = db.Close() }()
	index := NewSQLiteQueryIndex(db)
	tinyGrounded, err := Query(index, ConcernQuery{Text: "source body"})
	if err != nil {
		t.Fatalf("tiny source-grounded query error: %v", err)
	}
	if _, ok := tinyGrounded.(CandidateSet); !ok {
		t.Fatalf("tiny source-grounded query result = %T, want CandidateSet", tinyGrounded)
	}
	_, err = Query(index, ConcernQuery{
		Text: "source",
		ResponseBudget: ResponseBudget{
			MaxTotalCandidates: -1,
		},
	})
	if err == nil || !strings.Contains(err.Error(), "non-negative") {
		t.Fatalf("negative response budget error = %v, want fail-closed validation", err)
	}

	result, err := Query(
		index,
		ConcernQuery{Text: "qxjzvprl9271"},
	)
	if err != nil {
		t.Fatalf("Query() error: %v", err)
	}
	abstained, ok := result.(Abstained)
	if !ok {
		t.Fatalf("Query() result = %T, want Abstained", result)
	}
	if abstained.Reason != "no_source_derived_candidates" || len(abstained.MissingBasis) == 0 {
		t.Fatalf("Abstained carrier lacks explicit missing source basis: %#v", abstained)
	}
}

func TestQueryOptionalCandidateProducerIsExplicitAndCandidateOnly(t *testing.T) {
	units := minimalValidSourceUnits()
	dbPath := filepath.Join(t.TempDir(), "fpf.db")
	if err := StoreSourceUnits(dbPath, units); err != nil {
		t.Fatalf("StoreSourceUnits() error: %v", err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open query db: %v", err)
	}
	defer func() { _ = db.Close() }()
	index := NewSQLiteQueryIndex(db)
	request := ConcernQuery{Text: "qxjzvprl9271"}

	defaultResult, err := Query(index, request)
	if err != nil {
		t.Fatalf("default Query() error: %v", err)
	}
	if _, ok := defaultResult.(Abstained); !ok {
		t.Fatalf("default Query() result = %T, want Abstained without optional recall", defaultResult)
	}

	var practical SourceUnit
	for _, unit := range units {
		if unit.Role == SourceUnitRolePracticalUseCard {
			practical = unit
		}
	}
	producer := testOptionalCandidateProducer{candidate: RetrievedCandidate{
		Unit: SourceUnit{
			UnitID: practical.UnitID,
			Title:  "producer-forged title",
			Body:   "producer-forged source content",
		},
		MatchGrounds: []MatchGround{{
			Tier:         RetrievalTier("optional_test_recall"),
			ProbeField:   "text",
			SourceField:  "body",
			MatchedValue: "test-only candidate",
			Evidence: &MatchGroundEvidence{
				UnitID:     practical.UnitID,
				PatternID:  "FORGED.PATTERN",
				SourceRole: SourceUnitRolePatternBody,
				Provenance: SourceProvenance{
					SourcePath:     "forged/evidence.md",
					StartLine:      999,
					EndLine:        1000,
					ContentHash:    strings.Repeat("f", 64),
					SourceRevision: "forged-revision",
				},
				ProjectionRelation: "optional_source_witness",
			},
		}},
	}}
	optionalEvaluation, err := EvaluateQueryWithCandidateProducers(
		index,
		request,
		[]CandidateProducer{producer},
	)
	if err != nil {
		t.Fatalf("EvaluateQueryWithCandidateProducers() error: %v", err)
	}
	optionalResult := optionalEvaluation.Result()
	if got := strings.Join(optionalEvaluation.ProducerIDs(), ","); got !=
		"exact_source,source_phrase,authored_phrase,heading_keyword,role_local_fts,test_optional_candidate" {
		t.Fatalf("optional query producer ids = %q", got)
	}
	set, ok := optionalResult.(CandidateSet)
	if !ok || !candidateSetHasTier(set, RetrievalTier("optional_test_recall")) {
		t.Fatalf("optional producer result = %#v, want grounded CandidateSet", optionalResult)
	}
	optionalSource := set.Groups[0].Candidates[0].Source
	if optionalSource.Title != practical.Title || !strings.Contains(practical.Body, optionalSource.Excerpt) {
		t.Fatalf("optional producer source was not re-hydrated from QueryIndex: %#v", optionalSource)
	}
	evidence := set.Groups[0].Candidates[0].MatchGrounds[0].Evidence
	if evidence == nil ||
		evidence.UnitID != practical.UnitID ||
		evidence.PatternID != practical.PatternID ||
		evidence.SourceRole != practical.Role ||
		evidence.Provenance != practical.Provenance {
		t.Fatalf("optional producer evidence was not re-hydrated from QueryIndex: %#v", evidence)
	}
	unknownEvidenceProducer := testOptionalCandidateProducer{candidate: RetrievedCandidate{
		Unit: SourceUnit{UnitID: practical.UnitID},
		MatchGrounds: []MatchGround{{
			Tier:         RetrievalTier("optional_test_recall"),
			ProbeField:   "text",
			SourceField:  "body",
			MatchedValue: "test-only candidate",
			Evidence: &MatchGroundEvidence{
				UnitID: "unknown:evidence:unit",
			},
		}},
	}}
	_, err = QueryWithCandidateProducers(
		index,
		request,
		[]CandidateProducer{unknownEvidenceProducer},
	)
	if err == nil || !strings.Contains(err.Error(), "unknown evidence unit") {
		t.Fatalf("optional producer unknown evidence error = %v, want fail-closed rejection", err)
	}
	reservedTierProducer := testOptionalCandidateProducer{candidate: RetrievedCandidate{
		Unit: SourceUnit{UnitID: practical.UnitID},
		MatchGrounds: []MatchGround{{
			Tier:         RetrievalTierExactSource,
			ProbeField:   "text",
			SourceField:  sourceFieldExactIdentifierOrTitle,
			MatchedValue: practical.SourceID,
		}},
	}}
	_, err = QueryWithCandidateProducers(index, request, []CandidateProducer{reservedTierProducer})
	if err == nil || !strings.Contains(err.Error(), "reserved retrieval tier") {
		t.Fatalf("optional producer reserved exact-source tier error = %v, want fail-closed rejection", err)
	}

	inspectEvaluation, err := EvaluateQueryWithCandidateProducers(
		index,
		InspectQuery{Identifier: "TEST", Roles: []SourceUnitRole{SourceUnitRolePracticalUseCard}},
		[]CandidateProducer{failingTestCandidateProducer{}},
	)
	if err != nil {
		t.Fatalf("exact inspect invoked optional candidate producer: %v", err)
	}
	if _, ok := inspectEvaluation.Result().(ExactHit); !ok {
		t.Fatalf("exact inspect result = %T, want ExactHit", inspectEvaluation.Result())
	}
	if got := strings.Join(inspectEvaluation.ProducerIDs(), ","); got != queryProducerExactSource {
		t.Fatalf("exact inspect producer ids = %q", got)
	}
}

func TestSQLiteQueryIndex_RoleLocalFTSDoesNotMislabelKeywordOnlyMatch(t *testing.T) {
	units := minimalValidSourceUnits()
	for index := range units {
		if units[index].Role == SourceUnitRolePracticalUseCard {
			units[index].Keywords = []string{"keyword-only-probe"}
		}
	}

	dbPath := filepath.Join(t.TempDir(), "fpf.db")
	if err := StoreSourceUnits(dbPath, units); err != nil {
		t.Fatalf("StoreSourceUnits() error: %v", err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open query db: %v", err)
	}
	defer func() { _ = db.Close() }()
	index := SQLiteQueryIndex{db: db}
	probe := CandidateProbe{Text: "keyword-only-probe"}
	roles := []SourceUnitRole{SourceUnitRolePracticalUseCard}

	headingBatch, err := index.SearchHeadingsAndKeywords(probe, roles)
	if err != nil {
		t.Fatalf("SearchHeadingsAndKeywords() error: %v", err)
	}
	if len(headingBatch.Candidates) != 1 {
		t.Fatalf("heading/keyword candidates = %d, want keyword-grounded candidate", len(headingBatch.Candidates))
	}
	ftsBatch, err := index.SearchRoleLocalFTS(probe, roles)
	if err != nil {
		t.Fatalf("SearchRoleLocalFTS() error: %v", err)
	}
	if len(ftsBatch.Candidates) != 0 {
		t.Fatalf("role-local FTS mislabeled keyword-only match as title/body: %#v", ftsBatch.Candidates)
	}
}

func TestQueryCandidateSetIsPermutationInvariantAndCarriesNoExecutionOrder(t *testing.T) {
	units := minimalValidSourceUnits()
	for index := range units {
		if units[index].Role == SourceUnitRolePracticalUseCard || units[index].Role == SourceUnitRoleTOCRow {
			units[index].AuthoredPhrases = []string{"source body"}
		}
	}
	reversed := append([]SourceUnit(nil), units...)
	for left, right := 0, len(reversed)-1; left < right; left, right = left+1, right-1 {
		reversed[left], reversed[right] = reversed[right], reversed[left]
	}

	query := func(dbPath string, sourceUnits []SourceUnit) CandidateSet {
		t.Helper()
		if err := StoreSourceUnits(dbPath, sourceUnits); err != nil {
			t.Fatalf("StoreSourceUnits() error: %v", err)
		}
		db, err := sql.Open("sqlite", dbPath)
		if err != nil {
			t.Fatalf("open query db: %v", err)
		}
		defer func() { _ = db.Close() }()
		result, err := Query(NewSQLiteQueryIndex(db), ConcernQuery{Text: "source body"})
		if err != nil {
			t.Fatalf("Query() error: %v", err)
		}
		set, ok := result.(CandidateSet)
		if !ok {
			t.Fatalf("Query() result = %T, want CandidateSet", result)
		}
		return set
	}

	directory := t.TempDir()
	forward := query(filepath.Join(directory, "forward.db"), units)
	backward := query(filepath.Join(directory, "backward.db"), reversed)
	forwardIDs := candidateSetUnitIDs(forward)
	backwardIDs := candidateSetUnitIDs(backward)
	if len(forwardIDs) != len(backwardIDs) {
		t.Fatalf("candidate set size changed under source permutation: %v != %v", forwardIDs, backwardIDs)
	}
	for unitID := range forwardIDs {
		if _, exists := backwardIDs[unitID]; !exists {
			t.Fatalf("candidate %s disappeared under source permutation", unitID)
		}
	}
	for _, group := range forward.Groups {
		for _, candidate := range group.Candidates {
			if candidate.Source.SourceRole != SourceUnitRolePracticalUseCard && candidate.Source.UseCues != nil {
				t.Fatalf("non-practical candidate %s projected use cues", candidate.Source.UnitID)
			}
		}
	}

	payload, err := json.Marshal(forward)
	if err != nil {
		t.Fatalf("marshal CandidateSet: %v", err)
	}
	for _, forbidden := range []string{"causal_order", "work_order", "selected", "recommended", "required_next_action"} {
		if strings.Contains(string(payload), `"`+forbidden+`"`) {
			t.Fatalf("CandidateSet carries forbidden execution/selection field %q: %s", forbidden, payload)
		}
	}
}

func candidateSetHasTier(set CandidateSet, tier RetrievalTier) bool {
	for _, group := range set.Groups {
		for _, candidate := range group.Candidates {
			for _, ground := range candidate.MatchGrounds {
				if ground.Tier == tier {
					return true
				}
			}
		}
	}
	return false
}

func candidateHasExactSourceGround(candidate SourceCandidate, probeField, matchedValue string) bool {
	for _, ground := range candidate.MatchGrounds {
		if ground.Tier == RetrievalTierExactSource &&
			ground.ProbeField == probeField &&
			ground.SourceField == sourceFieldExactIdentifierOrTitle &&
			ground.MatchedValue == matchedValue {
			return true
		}
	}
	return false
}

func assertConcernIncludesPattern(t *testing.T, index QueryIndex, concern, patternID string) {
	t.Helper()
	result, err := Query(index, ConcernQuery{
		Text: concern,
		ResponseBudget: ResponseBudget{
			MaxCandidatesPerRole: 50,
			MaxTotalCandidates:   50,
		},
	})
	if err != nil {
		t.Fatalf("concern %q query error: %v", concern, err)
	}
	set, ok := result.(CandidateSet)
	if !ok {
		t.Fatalf("concern %q result = %T, want CandidateSet", concern, result)
	}
	if !candidateSetHasPatternID(set, patternID) {
		t.Fatalf("concern %q candidates omit %s: %#v", concern, patternID, set.Groups)
	}
}

func candidateSetHasPatternID(set CandidateSet, patternID string) bool {
	for _, group := range set.Groups {
		for _, candidate := range group.Candidates {
			if candidate.Source.PatternID == patternID {
				return true
			}
		}
	}
	return false
}

func candidateSetHasUnitID(set CandidateSet, unitID string) bool {
	for _, group := range set.Groups {
		for _, candidate := range group.Candidates {
			if candidate.Source.UnitID == unitID {
				return true
			}
		}
	}
	return false
}

func candidateSetHasRole(set CandidateSet, role SourceUnitRole) bool {
	for _, group := range set.Groups {
		if group.Role == role && len(group.Candidates) > 0 {
			return true
		}
	}
	return false
}

func assertCandidateDirectRefs(t *testing.T, candidate SourceCandidate, expected []string) {
	t.Helper()
	if !reflect.DeepEqual(candidate.Source.DirectRefs, expected) {
		t.Fatalf(
			"candidate %s direct refs = %#v, want current authored refs %#v",
			candidate.Source.UnitID,
			candidate.Source.DirectRefs,
			expected,
		)
	}
}

func findSourceCandidateByPatternID(t *testing.T, set CandidateSet, patternID string) SourceCandidate {
	t.Helper()
	for _, group := range set.Groups {
		for _, candidate := range group.Candidates {
			if candidate.Source.PatternID == patternID {
				return candidate
			}
		}
	}
	t.Fatalf("candidate set omits %s: %#v", patternID, set.Groups)
	return SourceCandidate{}
}

func findSourceCandidateByUnitID(t *testing.T, set CandidateSet, unitID string) SourceCandidate {
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

func assertNavigationExpansionGround(t *testing.T, candidate SourceCandidate, matchedValue string) {
	t.Helper()
	for _, ground := range candidate.MatchGrounds {
		if ground.SourceField != "expansion_after_source_admission" || ground.MatchedValue != matchedValue {
			continue
		}
		if ground.Evidence == nil ||
			ground.Evidence.UnitID != candidate.Source.UnitID ||
			ground.Evidence.SourceRole != candidate.Source.SourceRole ||
			ground.Evidence.ProjectionRelation != "source_field_partial_match" ||
			ground.Evidence.Provenance != candidate.Source.Provenance {
			t.Fatalf("navigation expansion ground lacks exact source provenance: %#v", ground)
		}
		return
	}
	t.Fatalf("candidate %s lacks admitted navigation expansion ground %q: %#v", candidate.Source.UnitID, matchedValue, candidate.MatchGrounds)
}

func assertProjectedPatternBodyPhraseGround(
	t *testing.T,
	candidate SourceCandidate,
	phrase string,
	patternID string,
	phraseKind SourcePhraseKind,
) {
	t.Helper()
	for _, ground := range candidate.MatchGrounds {
		if ground.SourceField != sourceFieldPatternBodyDerivedPhrase || ground.MatchedValue != phrase {
			continue
		}
		evidence := ground.Evidence
		if evidence == nil ||
			ground.PhraseKind != phraseKind ||
			evidence.PatternID != patternID ||
			evidence.SourceRole != SourceUnitRolePatternBody ||
			evidence.ProjectionRelation != "same_pattern_id" ||
			evidence.Provenance.SourcePath == "" ||
			evidence.Provenance.StartLine <= 0 ||
			evidence.Provenance.EndLine < evidence.Provenance.StartLine ||
			evidence.Provenance.ContentHash == "" ||
			evidence.Provenance.SourceRevision == "" {
			t.Fatalf("projected phrase ground lacks exact body provenance: %#v", ground)
		}
		return
	}
	t.Fatalf("candidate %s lacks projected pattern-body phrase %q: %#v", patternID, phrase, candidate.MatchGrounds)
}

func stringSliceContainsText(values []string, expected string) bool {
	for _, value := range values {
		if strings.Contains(value, expected) {
			return true
		}
	}
	return false
}

func findCandidateSourceByPatternID(t *testing.T, set CandidateSet, patternID string) CandidateSourceUnit {
	t.Helper()
	for _, group := range set.Groups {
		for _, candidate := range group.Candidates {
			if candidate.Source.PatternID == patternID {
				return candidate.Source
			}
		}
	}
	t.Fatalf("candidate set omits %s: %#v", patternID, set.Groups)
	return CandidateSourceUnit{}
}

func candidateSetUnitIDs(set CandidateSet) map[string]struct{} {
	ids := make(map[string]struct{})
	for _, group := range set.Groups {
		for _, candidate := range group.Candidates {
			ids[candidate.Source.UnitID] = struct{}{}
		}
	}
	return ids
}

func candidateSourceTextRuneCount(source CandidateSourceUnit) int {
	total := len([]rune(source.Excerpt))
	if source.UseCues == nil {
		return total
	}
	total += len([]rune(source.UseCues.ConditionText))
	total += len([]rune(source.UseCues.FirstResultText))
	total += len([]rune(source.UseCues.StopReturnText))
	return total
}

func candidateSetHasContextGround(set CandidateSet) bool {
	for _, group := range set.Groups {
		for _, candidate := range group.Candidates {
			for _, ground := range candidate.MatchGrounds {
				if ground.ProbeField != "text" {
					return true
				}
			}
		}
	}
	return false
}

func candidateSetSize(set CandidateSet) int {
	total := 0
	for _, group := range set.Groups {
		total += len(group.Candidates)
	}
	return total
}

func assertDefaultConcernRoles(t *testing.T, set CandidateSet) {
	t.Helper()
	for _, group := range set.Groups {
		if group.Role != SourceUnitRolePracticalUseCard && group.Role != SourceUnitRoleTOCRow {
			t.Fatalf("default concern leaked progressive-disclosure role %s", group.Role)
		}
	}
}

func minimalValidSourceUnits() []SourceUnit {
	roles := []SourceUnitRole{
		SourceUnitRolePracticalUseCard,
		SourceUnitRolePreface,
		SourceUnitRoleTOCRow,
		SourceUnitRolePatternBody,
		SourceUnitRolePatternSection,
	}
	units := make([]SourceUnit, 0, len(roles))
	for index, role := range roles {
		body := "source body for " + string(role)
		unit := SourceUnit{
			UnitID: "unit:" + string(role),
			Role:   role,
			Title:  "Source " + string(role),
			Body:   body,
			Provenance: SourceProvenance{
				SourcePath:     "FPF-Spec.md",
				StartLine:      index + 1,
				EndLine:        index + 1,
				ContentHash:    sourceContentHash(body),
				SourceRevision: "test-revision",
			},
		}
		if role == SourceUnitRolePatternBody {
			unit.SourceID = "A.1"
			unit.PatternID = "A.1"
		}
		if role == SourceUnitRoleTOCRow {
			unit.PatternID = "A.1"
		}
		if role == SourceUnitRolePatternSection {
			unit.SourceID = "A.1:1"
			unit.PatternID = "A.1:1"
			unit.ParentPatternID = "A.1"
		}
		if role == SourceUnitRolePracticalUseCard {
			unit.SourceID = "TEST"
			unit.DirectRefs = []string{"A.1"}
			unit.UseCues = SourceUseCues{
				ConditionText:   "A source-owned situation",
				FirstResultText: "A source-owned first result",
				StopReturnText:  "A source-owned stop or return condition",
			}
		}
		units = append(units, unit)
	}
	return units
}

func minimalValidSourceUnitsWithRelation() ([]SourceUnit, SourceRelation) {
	units := minimalValidSourceUnits()
	body := "| Z.99 | Planned Pattern | Planned |"
	provenance := SourceProvenance{
		SourcePath:     "FPF-Spec.md",
		StartLine:      99,
		EndLine:        99,
		ContentHash:    sourceContentHash(body),
		SourceRevision: "test-revision",
	}
	relation := SourceRelation{
		Kind:            SourceRelationKindBuildsOn,
		TargetPatternID: "A.1",
		TargetClass:     SourceRelationTargetClassLocalPattern,
		Origin:          SourceRelationOriginTOCExplicit,
		Provenance:      provenance,
	}
	units = append(units, SourceUnit{
		UnitID:            "spec:toc_row:z-99",
		SourceID:          "Z.99",
		Role:              SourceUnitRoleTOCRow,
		Title:             "Planned Pattern",
		Body:              body,
		PatternID:         "Z.99",
		PublicationStatus: "Planned",
		Relations:         []SourceRelation{relation},
		Provenance:        provenance,
	})
	return units, relation
}

func minimalValidSourceUnitsWithUnresolvedAuthoredReference() ([]SourceUnit, SourceRelation) {
	units := minimalValidSourceUnits()
	tocBody := "| Z.99 | Published Pattern With Source Gap | Stable | Coordinates with: A.404. |"
	tocProvenance := SourceProvenance{
		SourcePath:     "FPF-Spec.md",
		StartLine:      99,
		EndLine:        99,
		ContentHash:    sourceContentHash(tocBody),
		SourceRevision: "test-revision",
	}
	relation := SourceRelation{
		Kind:            SourceRelationKindCoordinatesWith,
		TargetPatternID: "A.404",
		TargetClass:     SourceRelationTargetClassUnresolvedAuthored,
		Origin:          SourceRelationOriginTOCExplicit,
		Provenance:      tocProvenance,
	}
	patternBody := "# Z.99 - Published Pattern With Source Gap\n\nCoordinates with A.404, whose authored publication unit is absent."
	units = append(units,
		SourceUnit{
			UnitID:            "spec:toc_row:z-99",
			Role:              SourceUnitRoleTOCRow,
			Title:             "Published Pattern With Source Gap",
			Body:              tocBody,
			PatternID:         "Z.99",
			PublicationStatus: "Stable",
			DirectRefs:        []string{"A.404"},
			Provenance:        tocProvenance,
		},
		SourceUnit{
			UnitID:    "spec:pattern:z-99",
			SourceID:  "Z.99",
			Role:      SourceUnitRolePatternBody,
			Title:     "Published Pattern With Source Gap",
			Body:      patternBody,
			PatternID: "Z.99",
			Relations: []SourceRelation{relation},
			Provenance: SourceProvenance{
				SourcePath:     "FPF-Spec.md",
				StartLine:      100,
				EndLine:        102,
				ContentHash:    sourceContentHash(patternBody),
				SourceRevision: "test-revision",
			},
		},
	)
	return units, relation
}

type testOptionalCandidateProducer struct {
	candidate RetrievedCandidate
}

func (testOptionalCandidateProducer) ProducerID() string {
	return "test_optional_candidate"
}

func (producer testOptionalCandidateProducer) ProduceCandidates(CandidateProbe, []SourceUnitRole) (CandidateBatch, error) {
	return CandidateBatch{Candidates: []RetrievedCandidate{producer.candidate}}, nil
}

type failingTestCandidateProducer struct{}

func (failingTestCandidateProducer) ProducerID() string {
	return "failing_test_candidate"
}

func (failingTestCandidateProducer) ProduceCandidates(CandidateProbe, []SourceUnitRole) (CandidateBatch, error) {
	return CandidateBatch{}, errors.New("must not run for exact inspect")
}
