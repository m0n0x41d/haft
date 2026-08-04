package fpfrefresh

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/fpf"
)

type QuerySmokeResult struct {
	CaseID     string              `json:"case_id"`
	ResultKind fpf.QueryResultKind `json:"result_kind"`
	UnitIDs    []string            `json:"unit_ids,omitempty"`
}

// VerifyCandidateQueryContract checks the current source-specific retrieval
// expectations against one already-built candidate index. A failure is quality
// review evidence after VerifySourceQueryRuntime succeeds; it is not proof that
// the fresh source/index publication is structurally invalid. It makes no
// applicability, pattern-selection, approval, or release claim.
func VerifyCandidateQueryContract(databasePath string) ([]QuerySmokeResult, error) {
	database, err := openIntegrationDatabaseReadOnly(databasePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = database.Close() }()
	if err := fpf.VerifySourceQueryIndexReadOnlyDB(database); err != nil {
		return nil, fmt.Errorf("verify candidate source query index: %w", err)
	}
	index := fpf.NewSQLiteQueryIndex(database)
	results := make([]QuerySmokeResult, 0, 12)

	exactCases := []struct {
		caseID     string
		request    fpf.QueryRequest
		wantUnitID string
	}{
		{
			caseID:     "lookup-added-pattern",
			request:    fpf.LookupQuery{Identifier: "A.1.SCR"},
			wantUnitID: "spec:pattern_body:a-1-scr",
		},
		{
			caseID:     "inspect-added-pattern",
			request:    fpf.InspectQuery{Identifier: "A.1.SCR"},
			wantUnitID: "spec:pattern_body:a-1-scr",
		},
		{
			caseID:     "lookup-system-recognition",
			request:    fpf.LookupQuery{Identifier: "SYSTEM-RECOGNITION"},
			wantUnitID: "readme:practical_use_card:system-recognition",
		},
		{
			caseID:     "lookup-system-delimitation",
			request:    fpf.LookupQuery{Identifier: "SYSTEM-DELIMITATION"},
			wantUnitID: "readme:practical_use_card:system-delimitation",
		},
		{
			caseID:     "lookup-unchanged-architecture",
			request:    fpf.LookupQuery{Identifier: "ARCHITECTURE"},
			wantUnitID: "readme:practical_use_card:architecture",
		},
		{
			caseID:     "lookup-unchanged-working-documents",
			request:    fpf.LookupQuery{Identifier: "WORKING-DOCUMENTS"},
			wantUnitID: "readme:practical_use_card:working-documents",
		},
	}
	for _, testCase := range exactCases {
		result, queryErr := fpf.Query(index, testCase.request)
		if queryErr != nil {
			return nil, fmt.Errorf("%s: %w", testCase.caseID, queryErr)
		}
		exact, ok := result.(fpf.ExactHit)
		if !ok {
			return nil, fmt.Errorf(
				"query_contract_regression[%s]: result kind %s, want exact_hit",
				testCase.caseID,
				result.ResultKind(),
			)
		}
		if exact.Unit.UnitID != testCase.wantUnitID {
			return nil, fmt.Errorf(
				"query_contract_regression[%s]: exact unit %q, want %q",
				testCase.caseID,
				exact.Unit.UnitID,
				testCase.wantUnitID,
			)
		}
		results = append(results, QuerySmokeResult{
			CaseID:     testCase.caseID,
			ResultKind: result.ResultKind(),
			UnitIDs:    []string{exact.Unit.UnitID},
		})
	}

	removedCases := []string{"SYSTEM-IN-CONTEXT", "A.6.8"}
	for _, identifier := range removedCases {
		caseID := "inspect-removed-" + sourceResultSlug(identifier)
		result, queryErr := fpf.Query(index, fpf.InspectQuery{Identifier: identifier})
		if queryErr != nil {
			return nil, fmt.Errorf("%s: %w", caseID, queryErr)
		}
		abstained, ok := result.(fpf.Abstained)
		if !ok || abstained.Reason != fpf.ReasonExactSourceUnitNotFound {
			return nil, fmt.Errorf(
				"query_contract_regression[%s]: result %#v, want %s abstention",
				caseID,
				result,
				fpf.ReasonExactSourceUnitNotFound,
			)
		}
		results = append(results, QuerySmokeResult{
			CaseID:     caseID,
			ResultKind: result.ResultKind(),
		})
	}

	concernCases := []struct {
		caseID     string
		request    fpf.ConcernQuery
		wantUnitID string
	}{
		{
			caseID: "system-recognition-concern",
			request: fpf.ConcernQuery{
				Text: "Does this exact entity satisfy the complete system criterion for the named decision?",
			},
			wantUnitID: "readme:practical_use_card:system-recognition",
		},
		{
			caseID: "system-delimitation-concern",
			request: fpf.ConcernQuery{
				Text: "Which exact entities are parts of this system and which relations only cross its boundary?",
			},
			wantUnitID: "readme:practical_use_card:system-delimitation",
		},
		{
			caseID: "russian-system-recognition-with-context",
			request: fpf.ConcernQuery{
				Text:            "Является ли эта точная сущность системой для данного решения?",
				EntityOfConcern: "точная сущность, проверяемая как U.System",
				KnownContext:    []string{"system recognition exact entity named decision A.1 criterion"},
				IntendedUse:     "recover source candidates before selecting any applicable pattern",
			},
			wantUnitID: "readme:practical_use_card:system-recognition",
		},
		{
			caseID: "tight-budget-exact-source-anchor",
			request: fpf.ConcernQuery{
				Text: "SYSTEM-RECOGNITION exact entity system decision",
				ResponseBudget: fpf.ResponseBudget{
					MaxCandidatesPerRole:     1,
					MaxTotalCandidates:       1,
					MaxExcerptCharacters:     96,
					MaxRelationsPerCandidate: 1,
				},
			},
			wantUnitID: "readme:practical_use_card:system-recognition",
		},
	}
	for _, testCase := range concernCases {
		result, queryErr := fpf.Query(index, testCase.request)
		if queryErr != nil {
			return nil, fmt.Errorf("%s: %w", testCase.caseID, queryErr)
		}
		candidates, ok := result.(fpf.CandidateSet)
		if !ok {
			return nil, fmt.Errorf(
				"query_contract_regression[%s]: result kind %s, want candidate_set",
				testCase.caseID,
				result.ResultKind(),
			)
		}
		unitIDs := candidateUnitIDs(candidates)
		if !containsString(unitIDs, testCase.wantUnitID) {
			return nil, fmt.Errorf(
				"query_contract_regression[%s]: candidates %v omit %s",
				testCase.caseID,
				unitIDs,
				testCase.wantUnitID,
			)
		}
		results = append(results, QuerySmokeResult{
			CaseID:     testCase.caseID,
			ResultKind: result.ResultKind(),
			UnitIDs:    unitIDs,
		})
	}
	return results, nil
}

// VerifySourceQueryRuntime verifies the source-query storage contract without
// imposing the current candidate publication's exact PatternIDs or practical
// card set. Recovery uses this for a last-known-good predecessor whose source
// vocabulary may legitimately predate the candidate-specific smoke suite.
func VerifySourceQueryRuntime(databasePath string) error {
	database, err := openIntegrationDatabaseReadOnly(databasePath)
	if err != nil {
		return err
	}
	defer func() { _ = database.Close() }()
	if err := fpf.VerifySourceQueryIndexReadOnlyDB(database); err != nil {
		return fmt.Errorf("verify source query index: %w", err)
	}
	return nil
}

func candidateUnitIDs(candidates fpf.CandidateSet) []string {
	unitIDs := make([]string, 0)
	for _, group := range candidates.Groups {
		for _, candidate := range group.Candidates {
			unitIDs = append(unitIDs, candidate.Source.UnitID)
		}
	}
	return unitIDs
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func sourceResultSlug(value string) string {
	output := make([]rune, 0, len(value))
	separator := false
	for _, character := range value {
		if character >= 'A' && character <= 'Z' {
			character += 'a' - 'A'
		}
		if (character >= 'a' && character <= 'z') ||
			(character >= '0' && character <= '9') {
			output = append(output, character)
			separator = false
			continue
		}
		if !separator && len(output) > 0 {
			output = append(output, '-')
			separator = true
		}
	}
	for len(output) > 0 && output[len(output)-1] == '-' {
		output = output[:len(output)-1]
	}
	return string(output)
}
