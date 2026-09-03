package p14acceptance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
)

const p14HReasonSemanticCorpusPath = "internal/cli/testdata/h-reason-semantic-corpus.v1.json"

var fullP14GitRevision = regexp.MustCompile(`^[0-9a-f]{40}$`)
var p14AgentFPFCamelBoundary = regexp.MustCompile(`([a-z0-9])([A-Z])`)

var p14AgentFPFPatternUseCaseIDs = []string{
	"mechanical_exact_lookup",
	"single_candidate_pua_new_result",
	"multi_candidate_pur_applicable",
	"recommendation_no_authority_effects",
}

type p14AgentFPFPatternUseCase struct {
	ID                        string                                `json:"id"`
	Query                     string                                `json:"query"`
	Basis                     p14AgentFPFInputBasis                 `json:"basis"`
	ScenarioDigest            string                                `json:"scenario_digest"`
	ExpectedClaimClasses      []string                              `json:"expected_claim_classes"`
	ExpectedSelectionBasis    string                                `json:"expected_selection_basis"`
	ExpectedFPFCalls          int                                   `json:"expected_fpf_calls"`
	RequiredInspects          []string                              `json:"required_inspects"`
	ExpectedPUAPositions      []string                              `json:"expected_pua_positions"`
	ExpectedJudgements        []p14AgentFPFJudgement                `json:"expected_judgements"`
	ExpectedWrites            int                                   `json:"expected_writes"`
	ExpectedEffects           []string                              `json:"expected_effects"`
	ExpectedRecommendation    string                                `json:"expected_recommendation"`
	RecommendationCandidate   string                                `json:"recommendation_candidate,omitempty"`
	ExpectedClosure           string                                `json:"expected_closure"`
	ExpectedResultAssertion   string                                `json:"expected_result_assertion,omitempty"`
	ExpectedA10Path           *p14AgentFPFA10Path                   `json:"expected_a10_path,omitempty"`
	ExpectedLocalClaimBasis   *p14AgentFPFLocalClaimDerivationBasis `json:"expected_local_claim_derivation_basis,omitempty"`
	ExpectedLocalClaim        *p14AgentFPFLocalClaimAssertion       `json:"expected_local_claim_assertion,omitempty"`
	ExpectedDirectBasis       string                                `json:"expected_direct_basis"`
	ExpectedRelativeObjectRef string                                `json:"expected_relative_object_ref,omitempty"`
	ExpectedBasisKind         string                                `json:"expected_basis_kind,omitempty"`
	ExpectedStopOrReturn      string                                `json:"expected_stop_or_return"`
	FiveAspectAggregateCount  int                                   `json:"five_aspect_aggregate_count"`
	RankSelectionForbidden    bool                                  `json:"rank_selection_forbidden"`
	UnauthorizedEffectsAbsent bool                                  `json:"unauthorized_effects_absent"`
	Calls                     []p14LiveProtocolProbe                `json:"calls"`
}

type p14AgentFPFCorpusEnvelope struct {
	ContractVersion string                       `json:"contract_version"`
	SourceBasis     p14AgentFPFCorpusSourceBasis `json:"source_basis"`
	Scenarios       []p14AgentFPFCorpusScenario  `json:"scenarios"`
}

type p14AgentFPFCorpusSourceBasis struct {
	FPFRevision string `json:"fpf_revision"`
}

type p14AgentFPFCorpusScenario struct {
	ID       string                       `json:"id"`
	Input    map[string]any               `json:"input"`
	Events   []map[string]json.RawMessage `json:"events"`
	Expected p14AgentFPFCorpusExpected    `json:"expected"`
}

type p14AgentFPFInputBasis struct {
	Subject           string                        `json:"subject"`
	Candidates        []p14AgentFPFInputCandidate   `json:"candidates"`
	CurrentFacts      []string                      `json:"current_facts"`
	ReceivingUse      string                        `json:"receiving_use"`
	A10SourceBasis    *p14AgentFPFA10SourceBasis    `json:"a10_source_basis,omitempty"`
	LocalClaimContext *p14AgentFPFLocalClaimContext `json:"local_claim_context,omitempty"`
}

type p14AgentFPFA10Path struct {
	PathProfile                   string                        `json:"path_profile"`
	PathRef                       string                        `json:"path_ref"`
	ReliedOnClaimRef              string                        `json:"relied_on_claim_ref"`
	ClaimGraph                    string                        `json:"claim_graph"`
	EntityOfConcernRef            string                        `json:"entity_of_concern_ref"`
	ReferenceSchemeRef            string                        `json:"reference_scheme_ref"`
	LocalResult                   string                        `json:"local_result"`
	ResultRuleOwner               string                        `json:"result_rule_owner"`
	ResultRule                    string                        `json:"result_rule"`
	AttemptedUseRef               string                        `json:"attempted_use_ref"`
	ObservedCarrierFacts          []string                      `json:"observed_carrier_facts"`
	EstablishedDirectRelationRefs []string                      `json:"established_direct_relation_refs"`
	NarrowedReversibleUse         string                        `json:"narrowed_reversible_use"`
	TimeWindow                    string                        `json:"time_window"`
	RivalExplanation              string                        `json:"rival_explanation"`
	UnsupportedAttemptedUse       string                        `json:"unsupported_attempted_use"`
	RelianceDisposition           string                        `json:"reliance_disposition"`
	ReopenTrigger                 string                        `json:"reopen_trigger"`
	Contest                       p14AgentFPFA10Contest         `json:"contest"`
	UnresolvedGaps                []string                      `json:"unresolved_gaps"`
	ResultIdentity                p14AgentFPFC21Identity        `json:"result_identity"`
	ResultConstitution            p14AgentFPFGroundingBasisPair `json:"result_constitution"`
}

type p14AgentFPFA10SourceBasis struct {
	ReliedOnClaimRef              string                `json:"relied_on_claim_ref"`
	ClaimGraph                    string                `json:"claim_graph"`
	EntityOfConcernRef            string                `json:"entity_of_concern_ref"`
	ReferenceSchemeRef            string                `json:"reference_scheme_ref"`
	LocalResultRuleOwner          string                `json:"local_result_rule_owner"`
	LocalResultRule               string                `json:"local_result_rule"`
	AttemptedUseRef               string                `json:"attempted_use_ref"`
	ObservedCarrierFacts          []string              `json:"observed_carrier_facts"`
	EstablishedDirectRelationRefs []string              `json:"established_direct_relation_refs"`
	NarrowedReversibleUse         string                `json:"narrowed_reversible_use"`
	TimeWindow                    string                `json:"time_window"`
	RivalExplanation              string                `json:"rival_explanation"`
	UnsupportedAttemptedUse       string                `json:"unsupported_attempted_use"`
	Contest                       p14AgentFPFA10Contest `json:"contest"`
	UnresolvedGaps                []string              `json:"unresolved_gaps"`
}

type p14AgentFPFA10Contest struct {
	AffectedPartyView         string `json:"affected_party_view"`
	AccountableReviewRoute    string `json:"accountable_review_route"`
	AllowedChallengeEvidence  string `json:"allowed_challenge_evidence"`
	PossibleDispositionChange string `json:"possible_disposition_change"`
	OutcomeRecordCondition    string `json:"outcome_record_condition"`
	ReopenTrigger             string `json:"reopen_trigger"`
}

type p14AgentFPFC21Identity struct {
	EpistemeRef        string `json:"episteme_ref"`
	ClaimGraph         string `json:"claim_graph"`
	EntityOfConcernRef string `json:"entity_of_concern_ref"`
	ReferenceSchemeRef string `json:"reference_scheme_ref"`
}

type p14AgentFPFGroundingBasisPair struct {
	OccurrenceRef                      string   `json:"occurrence_ref"`
	Predicate                          string   `json:"predicate"`
	Participants                       []string `json:"participants"`
	Obtains                            bool     `json:"obtains"`
	PatternLocator                     string   `json:"pattern_locator"`
	DirectDeclarationClaimGraphLocator string   `json:"direct_declaration_claim_graph_locator"`
}

type p14AgentFPFLocalClaimSubstrate struct {
	DefinitionIdentity     p14AgentFPFC21Identity        `json:"definition_identity"`
	DefinitionConstitution p14AgentFPFGroundingBasisPair `json:"definition_constitution"`
	ConstructorRef         string                        `json:"constructor_ref"`
	ConstructorSemantics   string                        `json:"constructor_semantics"`
	Applicability          string                        `json:"applicability"`
}

type p14AgentFPFLocalClaimParticipant struct {
	Meaning string `json:"meaning"`
	Ref     string `json:"ref"`
}

type p14AgentFPFLocalClaimBasePredicate struct {
	Predicate         string   `json:"predicate"`
	PatternLocator    string   `json:"pattern_locator"`
	ClaimGraphLocator string   `json:"claim_graph_locator"`
	Polarity          string   `json:"polarity"`
	Applicability     string   `json:"applicability"`
	ObtainingLaw      string   `json:"obtaining_law"`
	EditionOrWindow   string   `json:"edition_or_window"`
	Participants      []string `json:"participants"`
	CaseFacts         []string `json:"case_facts"`
}

type p14AgentFPFLocalClaimContext struct {
	BlockedReceivingUse           string                         `json:"blocked_receiving_use"`
	ResultRef                     string                         `json:"result_ref"`
	RelativeObjectRef             string                         `json:"relative_object_ref"`
	RelativeObjectKindRef         string                         `json:"relative_object_kind_ref"`
	RelativeObjectIdentity        p14AgentFPFC21Identity         `json:"relative_object_identity"`
	RelativeObjectConstitution    p14AgentFPFGroundingBasisPair  `json:"relative_object_constitution"`
	ResultExpectationIdentity     p14AgentFPFC21Identity         `json:"result_expectation_identity"`
	ResultExpectationConstitution p14AgentFPFGroundingBasisPair  `json:"result_expectation_constitution"`
	SelectedSubstrate             p14AgentFPFLocalClaimSubstrate `json:"selected_substrate"`
	PositiveCase                  string                         `json:"positive_case"`
	DiscriminatingFailureCase     string                         `json:"discriminating_failure_case"`
	ReceivingUseReplay            string                         `json:"receiving_use_replay"`
	StopOrReturn                  string                         `json:"stop_or_return"`
}

type p14AgentFPFLocalClaimDerivationBasis struct {
	DerivationPatternLocator string                               `json:"derivation_pattern_locator"`
	Disposition              string                               `json:"disposition"`
	Context                  p14AgentFPFLocalClaimContext         `json:"context"`
	ResultIdentity           p14AgentFPFC21Identity               `json:"result_identity"`
	ResultConstitution       p14AgentFPFGroundingBasisPair        `json:"result_constitution"`
	Participants             []p14AgentFPFLocalClaimParticipant   `json:"participants"`
	BasePredicates           []p14AgentFPFLocalClaimBasePredicate `json:"base_predicates"`
	CaseFacts                []string                             `json:"case_facts"`
	SupportOrWarrant         []string                             `json:"support_or_warrant"`
}

type p14AgentFPFLocalClaimAssertion struct {
	ClaimIdentity     p14AgentFPFC21Identity        `json:"claim_identity"`
	ClaimConstitution p14AgentFPFGroundingBasisPair `json:"claim_constitution"`
	Polarity          string                        `json:"polarity"`
}

type p14AgentFPFInputCandidate struct {
	PatternID           string `json:"pattern_id"`
	CurrentCondition    string `json:"current_condition"`
	ExpectedFirstResult string `json:"expected_first_result"`
	ReceivingUse        string `json:"receiving_use"`
	OrdinaryBoundary    string `json:"ordinary_boundary"`
}

type p14AgentFPFCorpusExpected struct {
	FPFCalls         int      `json:"fpf_calls"`
	RequiredInspects []string `json:"required_inspects"`
	Writes           int      `json:"writes"`
	Effects          []string `json:"effects"`
	Recommendation   string   `json:"recommendation"`
	Closure          string   `json:"closure"`
}

type p14AgentFPFFitAssessment struct {
	Result    string `json:"result"`
	Rationale string `json:"rationale"`
}

type p14AgentFPFJudgement struct {
	Candidate string                              `json:"candidate"`
	Fit       map[string]p14AgentFPFFitAssessment `json:"fit"`
	Aggregate string                              `json:"aggregate"`
}

type p14AgentFPFMutationLedger struct {
	Writes  int      `json:"writes"`
	Effects []string `json:"effects"`
}

type p14AgentFPFCaseObservation struct {
	Schema                  string                                `json:"schema"`
	CaseID                  string                                `json:"case_id"`
	ClaimClasses            []string                              `json:"claim_classes"`
	SelectionBasis          string                                `json:"selection_basis"`
	Closure                 string                                `json:"closure"`
	ResultAssertion         string                                `json:"result_assertion"`
	A10Path                 *p14AgentFPFA10Path                   `json:"a10_path,omitempty"`
	LocalClaimDerivation    *p14AgentFPFLocalClaimDerivationBasis `json:"local_claim_derivation_basis,omitempty"`
	LocalClaimAssertion     *p14AgentFPFLocalClaimAssertion       `json:"local_claim_assertion,omitempty"`
	DirectBasis             string                                `json:"direct_basis"`
	RelativeObjectRef       string                                `json:"relative_object_ref,omitempty"`
	BasisKind               string                                `json:"basis_kind,omitempty"`
	StopOrReturn            string                                `json:"stop_or_return"`
	PUAInspectionPositions  []string                              `json:"pua_inspection_positions"`
	CandidateJudgements     []p14AgentFPFJudgement                `json:"candidate_judgements"`
	Recommendation          string                                `json:"recommendation"`
	RecommendationCandidate string                                `json:"recommendation_candidate,omitempty"`
	MutationLedger          p14AgentFPFMutationLedger             `json:"mutation_ledger"`
}

const (
	p14AgentFPFCaseObservationSchema = "haft.p14.agent-fpf-case-observation/v2"
	p14AgentFPFObservationPrefix     = "HAFT_P14_OBSERVATION="
)

func loadP14AgentFPFPatternUseCases() (
	[]p14AgentFPFPatternUseCase,
	string,
	string,
	error,
) {
	root, err := p14RepositoryRoot()
	if err != nil {
		return nil, "", "", err
	}
	raw, err := os.ReadFile(filepath.Join(
		root,
		filepath.FromSlash(p14HReasonSemanticCorpusPath),
	))
	if err != nil {
		return nil, "", "", fmt.Errorf("read P14 h-reason semantic corpus: %w", err)
	}
	var corpus p14AgentFPFCorpusEnvelope
	if err := json.Unmarshal(raw, &corpus); err != nil {
		return nil, "", "", fmt.Errorf("decode P14 h-reason semantic corpus: %w", err)
	}
	if corpus.ContractVersion != "haft.h-reason-semantic-corpus.v1" ||
		!fullP14GitRevision.MatchString(corpus.SourceBasis.FPFRevision) {
		return nil, "", "", fmt.Errorf("P14 h-reason semantic corpus basis is invalid")
	}
	byID := make(map[string]p14AgentFPFCorpusScenario, len(corpus.Scenarios))
	for _, scenario := range corpus.Scenarios {
		if _, duplicate := byID[scenario.ID]; duplicate {
			return nil, "", "", fmt.Errorf(
				"P14 h-reason semantic corpus repeats %q",
				scenario.ID,
			)
		}
		byID[scenario.ID] = scenario
	}
	cases := make([]p14AgentFPFPatternUseCase, 0, len(p14AgentFPFPatternUseCaseIDs))
	for _, id := range p14AgentFPFPatternUseCaseIDs {
		scenario, present := byID[id]
		if !present {
			return nil, "", "", fmt.Errorf(
				"P14 h-reason semantic corpus omits %q",
				id,
			)
		}
		built, buildErr := buildP14AgentFPFPatternUseCase(scenario)
		if buildErr != nil {
			return nil, "", "", buildErr
		}
		cases = append(cases, built)
	}
	return cases, p14Digest(raw), corpus.SourceBasis.FPFRevision, nil
}

func buildP14AgentFPFPatternUseCase(
	scenario p14AgentFPFCorpusScenario,
) (p14AgentFPFPatternUseCase, error) {
	query, _ := scenario.Input["query"].(string)
	if strings.TrimSpace(query) == "" {
		return p14AgentFPFPatternUseCase{}, fmt.Errorf(
			"P14 h-reason case %q has no exact query",
			scenario.ID,
		)
	}
	scenarioBytes, err := marshalP14CanonicalJSON(scenario)
	if err != nil {
		return p14AgentFPFPatternUseCase{}, err
	}
	result := p14AgentFPFPatternUseCase{
		ID:                        scenario.ID,
		Query:                     query,
		ScenarioDigest:            p14Digest(scenarioBytes),
		ExpectedFPFCalls:          scenario.Expected.FPFCalls,
		RequiredInspects:          slices.Clone(scenario.Expected.RequiredInspects),
		ExpectedWrites:            scenario.Expected.Writes,
		ExpectedEffects:           slices.Clone(scenario.Expected.Effects),
		ExpectedRecommendation:    scenario.Expected.Recommendation,
		ExpectedClosure:           scenario.Expected.Closure,
		RankSelectionForbidden:    true,
		UnauthorizedEffectsAbsent: scenario.Expected.Writes == 0 && len(scenario.Expected.Effects) == 0,
	}
	basisRaw, err := marshalP14CanonicalJSON(scenario.Input["basis"])
	if err != nil {
		return p14AgentFPFPatternUseCase{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(basisRaw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result.Basis); err != nil {
		return p14AgentFPFPatternUseCase{}, fmt.Errorf(
			"P14 h-reason case %q basis: %w",
			scenario.ID,
			err,
		)
	}
	if err := validateP14AgentFPFInputBasis(
		result.Basis,
		scenario.Expected.FPFCalls,
	); err != nil {
		return p14AgentFPFPatternUseCase{}, fmt.Errorf(
			"P14 h-reason case %q basis: %w",
			scenario.ID,
			err,
		)
	}
	fpfCalls := 0
	inspects := make([]string, 0)
	closureSeen := false
	for _, event := range scenario.Events {
		kind := p14AgentFPFJSONText(event["kind"])
		switch kind {
		case "final_claim":
			if err := json.Unmarshal(event["claim_classes"], &result.ExpectedClaimClasses); err != nil {
				return p14AgentFPFPatternUseCase{}, fmt.Errorf(
					"P14 h-reason case %q final claim classes: %w",
					scenario.ID,
					err,
				)
			}
			result.ExpectedSelectionBasis = p14AgentFPFJSONText(event["selection_basis"])
		case "pua_inspection":
			if err := json.Unmarshal(event["positions"], &result.ExpectedPUAPositions); err != nil {
				return p14AgentFPFPatternUseCase{}, fmt.Errorf(
					"P14 h-reason case %q PUA positions: %w",
					scenario.ID,
					err,
				)
			}
		case "recommendation":
			result.RecommendationCandidate = p14AgentFPFJSONText(event["candidate"])
			result.ExpectedResultAssertion = "recommendation_candidate:" +
				result.RecommendationCandidate
			result.ExpectedDirectBasis = p14AgentFPFJSONText(event["selection_basis"])
			for _, candidate := range result.Basis.Candidates {
				if candidate.PatternID == result.RecommendationCandidate {
					result.ExpectedStopOrReturn = candidate.OrdinaryBoundary
					break
				}
			}
		case "closure":
			if closureSeen {
				return p14AgentFPFPatternUseCase{}, fmt.Errorf(
					"P14 h-reason case %q repeats its closure",
					scenario.ID,
				)
			}
			closureSeen = true
			result.ExpectedResultAssertion = p14AgentFPFJSONText(
				event["actual_result_assertion"],
			)
			result.ExpectedDirectBasis = p14AgentFPFJSONText(event["direct_basis"])
			result.ExpectedRelativeObjectRef = p14AgentFPFJSONText(
				event["relative_object_ref"],
			)
			result.ExpectedBasisKind = p14AgentFPFJSONText(event["basis_kind"])
			result.ExpectedStopOrReturn = p14AgentFPFJSONText(event["return_condition"])
			if raw := event["local_claim_derivation_basis"]; len(raw) != 0 {
				var basis p14AgentFPFLocalClaimDerivationBasis
				if err := decodeP14AgentFPFStrict(raw, &basis); err != nil {
					return p14AgentFPFPatternUseCase{}, fmt.Errorf(
						"P14 h-reason case %q local-claim derivation: %w",
						scenario.ID,
						err,
					)
				}
				result.ExpectedLocalClaimBasis = p14AgentFPFClone(&basis)
			}
			if raw := event["local_claim_assertion"]; len(raw) != 0 {
				var assertion p14AgentFPFLocalClaimAssertion
				if err := decodeP14AgentFPFStrict(raw, &assertion); err != nil {
					return p14AgentFPFPatternUseCase{}, fmt.Errorf(
						"P14 h-reason case %q local-claim assertion: %w",
						scenario.ID,
						err,
					)
				}
				result.ExpectedLocalClaim = p14AgentFPFClone(&assertion)
			}
		case "a10_result":
			var observedPath p14AgentFPFA10Path
			if err := decodeP14AgentFPFStrict(
				event["a10_path"],
				&observedPath,
			); err != nil {
				return p14AgentFPFPatternUseCase{}, fmt.Errorf(
					"P14 h-reason case %q A.10 path: %w",
					scenario.ID,
					err,
				)
			}
			if result.ExpectedA10Path != nil ||
				result.Basis.A10SourceBasis == nil ||
				validateP14AgentFPFA10DerivedPath(
					*result.Basis.A10SourceBasis,
					observedPath,
				) != nil {
				return p14AgentFPFPatternUseCase{}, fmt.Errorf(
					"P14 h-reason case %q A.10 result differs from its supplied basis",
					scenario.ID,
				)
			}
			result.ExpectedA10Path = p14AgentFPFCloneA10Path(&observedPath)
		}
		if kind == "candidate_judgement" {
			fit := map[string]p14AgentFPFFitAssessment{}
			if err := json.Unmarshal(event["fit"], &fit); err != nil {
				return p14AgentFPFPatternUseCase{}, fmt.Errorf(
					"P14 h-reason case %q fit: %w",
					scenario.ID,
					err,
				)
			}
			for _, aspect := range []string{
				"problemFrame",
				"forces",
				"solutionConditions",
				"ordinaryBoundary",
				"resultAndReceivingUse",
			} {
				assessment := fit[aspect]
				if !slices.Contains(
					[]string{"fit", "misfit", "insufficientBasis"},
					assessment.Result,
				) || strings.TrimSpace(assessment.Rationale) == "" {
					return p14AgentFPFPatternUseCase{}, fmt.Errorf(
						"P14 h-reason case %q has invalid fit aspect %q",
						scenario.ID,
						aspect,
					)
				}
			}
			aggregate := p14AgentFPFJSONText(event["aggregate"])
			if !p14AgentFPFAggregateMatchesFit(fit, aggregate) {
				return p14AgentFPFPatternUseCase{}, fmt.Errorf(
					"P14 h-reason case %q aggregate differs from five aspects",
					scenario.ID,
				)
			}
			result.FiveAspectAggregateCount++
			result.ExpectedJudgements = append(
				result.ExpectedJudgements,
				p14AgentFPFJudgement{
					Candidate: p14AgentFPFJSONText(event["candidate"]),
					Fit:       fit,
					Aggregate: aggregate,
				},
			)
		}
		if selection := p14AgentFPFJSONText(event["selection_basis"]); strings.Contains(strings.ToLower(selection), "rank") {
			return p14AgentFPFPatternUseCase{}, fmt.Errorf(
				"P14 h-reason case %q selects by rank",
				scenario.ID,
			)
		}
		if kind != "tool_call" {
			continue
		}
		tool := p14AgentFPFJSONText(event["tool"])
		args := map[string]any{}
		if err := json.Unmarshal(event["payload"], &args); err != nil {
			return p14AgentFPFPatternUseCase{}, fmt.Errorf(
				"P14 h-reason case %q tool payload: %w",
				scenario.ID,
				err,
			)
		}
		result.Calls = append(result.Calls, p14LiveProtocolProbe{
			Tool: tool,
			Args: args,
		})
		if args["action"] == "fpf" {
			fpfCalls++
			if args["mode"] == "inspect" {
				identifier, _ := args["identifier"].(string)
				inspects = append(inspects, identifier)
			}
		}
	}
	if scenario.ID == "mechanical_exact_lookup" {
		if len(result.Calls) != 0 {
			return p14AgentFPFPatternUseCase{}, fmt.Errorf(
				"P14 mechanical control already carries a tool recipe",
			)
		}
		result.Calls = []p14LiveProtocolProbe{{
			Tool: "haft_query",
			Args: map[string]any{"action": "status", "full": false},
		}}
		result.ExpectedDirectBasis = "installed_candidate_version_command_output"
		result.ExpectedStopOrReturn = result.Basis.ReceivingUse
	}
	if fpfCalls != scenario.Expected.FPFCalls ||
		!slices.Equal(inspects, scenario.Expected.RequiredInspects) {
		return p14AgentFPFPatternUseCase{}, fmt.Errorf(
			"P14 h-reason case %q call trace differs",
			scenario.ID,
		)
	}
	if err := validateP14AgentFPFPatternUseCase(result); err != nil {
		return p14AgentFPFPatternUseCase{}, err
	}
	return result, nil
}

func validateP14AgentFPFInputBasis(
	basis p14AgentFPFInputBasis,
	expectedFPFCalls int,
) error {
	if strings.TrimSpace(basis.Subject) == "" ||
		strings.TrimSpace(basis.ReceivingUse) == "" ||
		len(basis.CurrentFacts) == 0 {
		return fmt.Errorf("decision-complete subject, facts, or receiving use is absent")
	}
	for _, fact := range basis.CurrentFacts {
		if strings.TrimSpace(fact) == "" {
			return fmt.Errorf("current fact is empty")
		}
	}
	if (expectedFPFCalls == 0) != (len(basis.Candidates) == 0) {
		return fmt.Errorf("candidate set differs from FPF-call posture")
	}
	seen := make(map[string]struct{}, len(basis.Candidates))
	for _, candidate := range basis.Candidates {
		if candidate.PatternID == "" ||
			strings.TrimSpace(candidate.CurrentCondition) == "" ||
			strings.TrimSpace(candidate.ExpectedFirstResult) == "" ||
			strings.TrimSpace(candidate.ReceivingUse) == "" ||
			strings.TrimSpace(candidate.OrdinaryBoundary) == "" {
			return fmt.Errorf("candidate basis is incomplete")
		}
		if _, duplicate := seen[candidate.PatternID]; duplicate {
			return fmt.Errorf("candidate %q is duplicated", candidate.PatternID)
		}
		seen[candidate.PatternID] = struct{}{}
	}
	hasA10Candidate := false
	for _, candidate := range basis.Candidates {
		if candidate.PatternID == "A.10" {
			hasA10Candidate = true
		}
	}
	if (basis.A10SourceBasis != nil) != hasA10Candidate {
		return fmt.Errorf("A.10 source basis differs from the candidate set")
	}
	if basis.A10SourceBasis != nil {
		if err := validateP14AgentFPFA10SourceBasis(*basis.A10SourceBasis); err != nil {
			return err
		}
	}
	if basis.LocalClaimContext != nil {
		if err := validateP14AgentFPFLocalClaimContext(*basis.LocalClaimContext); err != nil {
			return err
		}
	}
	return nil
}

func validateP14AgentFPFA10SourceBasis(basis p14AgentFPFA10SourceBasis) error {
	raw, err := marshalP14CanonicalJSON(basis)
	if err != nil || strings.Contains(string(raw), `:""`) ||
		len(basis.ObservedCarrierFacts) < 2 ||
		len(basis.EstablishedDirectRelationRefs) != 0 ||
		len(basis.UnresolvedGaps) == 0 ||
		strings.Contains(basis.LocalResultRuleOwner, "A.10") ||
		strings.Contains(basis.LocalResultRuleOwner, "G.11") ||
		strings.Contains(
			strings.ToLower(basis.Contest.PossibleDispositionChange),
			"degrade",
		) {
		return fmt.Errorf("A.10 source/use basis is incomplete")
	}
	for _, fact := range basis.ObservedCarrierFacts {
		if strings.TrimSpace(fact) == "" {
			return fmt.Errorf("A.10 observed carrier fact is empty")
		}
	}
	for _, gap := range basis.UnresolvedGaps {
		if strings.TrimSpace(gap) == "" {
			return fmt.Errorf("A.10 unresolved gap is empty")
		}
	}
	return nil
}

func validateP14AgentFPFA10DerivedPath(
	supplied p14AgentFPFA10SourceBasis,
	path p14AgentFPFA10Path,
) error {
	if err := validateP14AgentFPFA10SourceBasis(supplied); err != nil {
		return err
	}
	if path.PathProfile != "a10_partial_gap_exposing_reversible" ||
		!strings.HasPrefix(path.PathRef, "path:") ||
		strings.TrimSpace(path.LocalResult) == "" ||
		strings.Contains(strings.ToLower(path.LocalResult), "evidencerelation") ||
		strings.Contains(strings.ToLower(path.LocalResult), " obtains") ||
		path.RelianceDisposition != "degrade" ||
		strings.TrimSpace(path.ReopenTrigger) == "" ||
		path.Contest.ReopenTrigger != path.ReopenTrigger ||
		!reflect.DeepEqual(supplied, p14AgentFPFA10SourceFromPath(path)) {
		return fmt.Errorf("A.10 derived path differs from its supplied source/use basis")
	}
	if err := validateP14AgentFPFC21Constitution(
		path.ResultIdentity,
		path.ResultConstitution,
	); err != nil {
		return fmt.Errorf("A.10 derived result identity: %w", err)
	}
	if path.ResultIdentity.EpistemeRef != path.PathRef ||
		path.ResultIdentity.EntityOfConcernRef != path.ReliedOnClaimRef ||
		!strings.Contains(path.ResultIdentity.ClaimGraph, path.PathRef) ||
		!strings.Contains(path.ResultIdentity.ClaimGraph, path.LocalResult) ||
		!strings.Contains(path.ResultIdentity.ClaimGraph, path.AttemptedUseRef) ||
		!strings.Contains(
			path.ResultIdentity.ClaimGraph,
			"RelianceDisposition="+path.RelianceDisposition,
		) {
		return fmt.Errorf("A.10 path does not independently identify its result")
	}
	return nil
}

func validateP14AgentFPFC21Constitution(
	identity p14AgentFPFC21Identity,
	constitution p14AgentFPFGroundingBasisPair,
) error {
	if strings.TrimSpace(identity.EpistemeRef) == "" ||
		strings.TrimSpace(identity.ClaimGraph) == "" ||
		strings.TrimSpace(identity.EntityOfConcernRef) == "" ||
		strings.TrimSpace(identity.ReferenceSchemeRef) == "" ||
		strings.TrimSpace(constitution.OccurrenceRef) == "" {
		return fmt.Errorf("exact C.2.1 identity or constitution occurrence is absent")
	}
	wantParticipants := []string{
		identity.ClaimGraph,
		identity.EntityOfConcernRef,
		identity.ReferenceSchemeRef,
	}
	if constitution.Predicate != "EpistemeConstitutionRelation" ||
		!constitution.Obtains ||
		!slices.Equal(constitution.Participants, wantParticipants) ||
		constitution.PatternLocator != "C.2.1" ||
		constitution.DirectDeclarationClaimGraphLocator != "C.2.1:4.2.1-4.2.3" {
		return fmt.Errorf("identity is not bound by an obtaining exact C.2.1 constitution")
	}
	return nil
}

func validateP14AgentFPFLocalClaimContext(
	context p14AgentFPFLocalClaimContext,
) error {
	for label, value := range map[string]string{
		"blocked receiving use":       context.BlockedReceivingUse,
		"result ref":                  context.ResultRef,
		"relative object":             context.RelativeObjectRef,
		"relative object kind":        context.RelativeObjectKindRef,
		"constructor ref":             context.SelectedSubstrate.ConstructorRef,
		"constructor semantics":       context.SelectedSubstrate.ConstructorSemantics,
		"constructor applicability":   context.SelectedSubstrate.Applicability,
		"positive case":               context.PositiveCase,
		"discriminating failure case": context.DiscriminatingFailureCase,
		"receiving-use replay":        context.ReceivingUseReplay,
		"stop or return":              context.StopOrReturn,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("local-claim context lacks %s", label)
		}
	}
	for _, pair := range []struct {
		label        string
		identity     p14AgentFPFC21Identity
		constitution p14AgentFPFGroundingBasisPair
	}{
		{"relative object", context.RelativeObjectIdentity, context.RelativeObjectConstitution},
		{"result expectation", context.ResultExpectationIdentity, context.ResultExpectationConstitution},
		{"constructor", context.SelectedSubstrate.DefinitionIdentity, context.SelectedSubstrate.DefinitionConstitution},
	} {
		if err := validateP14AgentFPFC21Constitution(
			pair.identity,
			pair.constitution,
		); err != nil {
			return fmt.Errorf("local-claim %s: %w", pair.label, err)
		}
	}
	if context.RelativeObjectKindRef != "U.Episteme" ||
		context.RelativeObjectIdentity.EpistemeRef != context.RelativeObjectRef ||
		context.SelectedSubstrate.DefinitionIdentity.EntityOfConcernRef !=
			context.SelectedSubstrate.ConstructorRef ||
		!strings.Contains(
			context.SelectedSubstrate.DefinitionIdentity.ClaimGraph,
			context.SelectedSubstrate.ConstructorRef,
		) ||
		!strings.Contains(
			context.SelectedSubstrate.ConstructorSemantics,
			"exact expected-result-content match",
		) {
		return fmt.Errorf("local-claim context does not identify its relative object or exact-match constructor")
	}
	if strings.Contains(
		context.SelectedSubstrate.DefinitionIdentity.ClaimGraph,
		context.ResultRef,
	) || strings.Contains(
		context.SelectedSubstrate.DefinitionIdentity.ClaimGraph,
		context.RelativeObjectRef,
	) {
		return fmt.Errorf("local-claim result leaked into its reusable constructor")
	}
	expectation := strings.ToLower(context.ResultExpectationIdentity.ClaimGraph)
	for _, phrase := range []string{
		strings.ToLower(context.RelativeObjectRef),
		"localrelationbearingclaim",
		"asserts neither result existence nor closure",
	} {
		if !strings.Contains(expectation, phrase) {
			return fmt.Errorf("local-claim expectation omits %q", phrase)
		}
	}
	return nil
}

func validateP14AgentFPFLocalClaimClosure(
	testCase p14AgentFPFPatternUseCase,
) error {
	context := testCase.Basis.LocalClaimContext
	path := testCase.ExpectedA10Path
	basis := testCase.ExpectedLocalClaimBasis
	assertion := testCase.ExpectedLocalClaim
	if context == nil || path == nil || basis == nil || assertion == nil {
		return fmt.Errorf("P14 single-candidate closure lacks its local-claim structures")
	}
	if err := validateP14AgentFPFLocalClaimContext(*context); err != nil {
		return err
	}
	if basis.DerivationPatternLocator != "A.6.RCD" ||
		basis.Disposition != "localCompoundRelationBearingClaim" ||
		!reflect.DeepEqual(basis.Context, *context) ||
		!reflect.DeepEqual(basis.ResultIdentity, path.ResultIdentity) ||
		!reflect.DeepEqual(basis.ResultConstitution, path.ResultConstitution) ||
		basis.ResultIdentity.EpistemeRef != context.ResultRef {
		return fmt.Errorf("local-claim derivation is not bound to its predeclared context and independently observed result")
	}
	if err := validateP14AgentFPFC21Constitution(
		basis.ResultIdentity,
		basis.ResultConstitution,
	); err != nil {
		return fmt.Errorf("local-claim result basis: %w", err)
	}
	if len(basis.Participants) < 2 || len(basis.BasePredicates) < 2 ||
		len(basis.CaseFacts) == 0 || len(basis.SupportOrWarrant) == 0 {
		return fmt.Errorf("local-claim derivation lacks participants, bases, facts, or warrant")
	}
	participantRefs := make(map[string]struct{}, len(basis.Participants))
	for _, participant := range basis.Participants {
		if strings.TrimSpace(participant.Meaning) == "" ||
			strings.TrimSpace(participant.Ref) == "" {
			return fmt.Errorf("local-claim derivation has an incomplete participant")
		}
		if _, duplicate := participantRefs[participant.Ref]; duplicate {
			return fmt.Errorf("local-claim derivation repeats participant %q", participant.Ref)
		}
		participantRefs[participant.Ref] = struct{}{}
	}
	locators := make([]string, 0, len(basis.BasePredicates))
	resultConstitutionBound := false
	for _, base := range basis.BasePredicates {
		locators = append(locators, base.ClaimGraphLocator)
		if strings.TrimSpace(base.Predicate) == "" ||
			strings.TrimSpace(base.PatternLocator) == "" ||
			strings.TrimSpace(base.ClaimGraphLocator) == "" ||
			base.Polarity != "affirmative" ||
			strings.TrimSpace(base.Applicability) == "" ||
			strings.TrimSpace(base.ObtainingLaw) == "" ||
			strings.TrimSpace(base.EditionOrWindow) == "" ||
			len(base.Participants) == 0 || len(base.CaseFacts) == 0 ||
			!strings.HasPrefix(base.ClaimGraphLocator, base.PatternLocator+":") {
			return fmt.Errorf("local-claim base predicate is incomplete or has no exact locator")
		}
		for _, participant := range base.Participants {
			if _, present := participantRefs[participant]; !present {
				return fmt.Errorf(
					"local-claim base %q uses undeclared participant %q",
					base.Predicate,
					participant,
				)
			}
		}
		for _, fact := range base.CaseFacts {
			if strings.TrimSpace(fact) == "" {
				return fmt.Errorf("local-claim base %q has an empty case fact", base.Predicate)
			}
		}
		if base.Predicate == basis.ResultConstitution.Predicate &&
			base.ClaimGraphLocator == basis.ResultConstitution.DirectDeclarationClaimGraphLocator &&
			slices.Equal(base.Participants, basis.ResultConstitution.Participants) {
			resultConstitutionBound = true
		}
	}
	if !resultConstitutionBound {
		return fmt.Errorf("local-claim basis omits the independently observed result constitution")
	}
	wantLocators := []string{
		"C.2.1:4.2.1-4.2.3",
		"A.10:4.4",
		"A.10:4.6b",
		"E.11.PUA:4.3",
	}
	slices.Sort(locators)
	slices.Sort(wantLocators)
	if !slices.Equal(locators, wantLocators) {
		return fmt.Errorf("local-claim basis does not use the exact independent locators")
	}
	for _, item := range append(slices.Clone(basis.CaseFacts), basis.SupportOrWarrant...) {
		if strings.TrimSpace(item) == "" {
			return fmt.Errorf("local-claim derivation has an empty fact or warrant")
		}
	}
	if err := rejectP14AgentFPFCircularClosureBasis(*basis); err != nil {
		return err
	}
	if assertion.Polarity != "affirmative" ||
		assertion.ClaimIdentity.EntityOfConcernRef != context.ResultRef {
		return fmt.Errorf("local-claim assertion lacks its affirmative exact result subject")
	}
	if err := validateP14AgentFPFC21Constitution(
		assertion.ClaimIdentity,
		assertion.ClaimConstitution,
	); err != nil {
		return fmt.Errorf("local-claim assertion: %w", err)
	}
	claimGraph := assertion.ClaimIdentity.ClaimGraph
	for label, value := range map[string]string{
		"actual result assertion": testCase.ExpectedResultAssertion,
		"result ref":              context.ResultRef,
		"relative object":         context.RelativeObjectRef,
		"constructor":             context.SelectedSubstrate.ConstructorRef,
	} {
		if strings.TrimSpace(value) == "" || !strings.Contains(claimGraph, value) {
			return fmt.Errorf("local-claim assertion omits %s", label)
		}
	}
	for _, base := range basis.BasePredicates {
		if !strings.Contains(claimGraph, base.Predicate) {
			return fmt.Errorf("local-claim assertion omits base predicate %q", base.Predicate)
		}
	}
	if testCase.ExpectedRelativeObjectRef != context.RelativeObjectRef ||
		testCase.ExpectedBasisKind != "localRelationBearingClaim" ||
		testCase.ExpectedStopOrReturn != context.StopOrReturn ||
		!strings.Contains(
			testCase.ExpectedDirectBasis,
			assertion.ClaimIdentity.EpistemeRef,
		) || !strings.Contains(testCase.ExpectedDirectBasis, "A.6.RCD") {
		return fmt.Errorf("local-claim closure does not bind its assertion, relative object, basis kind, and return condition")
	}
	return nil
}

func rejectP14AgentFPFCircularClosureBasis(
	basis p14AgentFPFLocalClaimDerivationBasis,
) error {
	raw, err := json.Marshal(basis)
	if err != nil {
		return fmt.Errorf("marshal local-claim derivation: %w", err)
	}
	joined := strings.ToLower(string(raw))
	for _, phrase := range []string{
		"e.11.pua:4.5",
		"patternuseresultclosurefinding@context",
		"pattern_use_result_closure_finding",
		"closure-schema",
		"closure_schema",
		"made result of this pattern use",
		"connects the grounding finding",
		"relates the grounding finding",
	} {
		if strings.Contains(joined, phrase) {
			return fmt.Errorf("local-claim basis is circular through %q", phrase)
		}
	}
	return nil
}

func p14AgentFPFA10SourceFromPath(path p14AgentFPFA10Path) p14AgentFPFA10SourceBasis {
	return p14AgentFPFA10SourceBasis{
		ReliedOnClaimRef:              path.ReliedOnClaimRef,
		ClaimGraph:                    path.ClaimGraph,
		EntityOfConcernRef:            path.EntityOfConcernRef,
		ReferenceSchemeRef:            path.ReferenceSchemeRef,
		LocalResultRuleOwner:          path.ResultRuleOwner,
		LocalResultRule:               path.ResultRule,
		AttemptedUseRef:               path.AttemptedUseRef,
		ObservedCarrierFacts:          slices.Clone(path.ObservedCarrierFacts),
		EstablishedDirectRelationRefs: slices.Clone(path.EstablishedDirectRelationRefs),
		NarrowedReversibleUse:         path.NarrowedReversibleUse,
		TimeWindow:                    path.TimeWindow,
		RivalExplanation:              path.RivalExplanation,
		UnsupportedAttemptedUse:       path.UnsupportedAttemptedUse,
		Contest:                       path.Contest,
		UnresolvedGaps:                slices.Clone(path.UnresolvedGaps),
	}
}

func p14AgentFPFAggregateMatchesFit(
	fit map[string]p14AgentFPFFitAssessment,
	aggregate string,
) bool {
	want := "applicable"
	for _, assessment := range fit {
		switch assessment.Result {
		case "misfit":
			want = "inapplicable"
		case "insufficientBasis":
			if want != "inapplicable" {
				want = "insufficientBasis"
			}
		}
	}
	return aggregate == want
}

func validateP14AgentFPFPatternUseCase(
	testCase p14AgentFPFPatternUseCase,
) error {
	if testCase.ID == "" || testCase.Query == "" ||
		!validP14Digest(testCase.ScenarioDigest) || len(testCase.Calls) == 0 ||
		len(testCase.ExpectedClaimClasses) == 0 ||
		strings.TrimSpace(testCase.ExpectedSelectionBasis) == "" ||
		strings.TrimSpace(testCase.ExpectedDirectBasis) == "" ||
		strings.TrimSpace(testCase.ExpectedStopOrReturn) == "" ||
		!testCase.RankSelectionForbidden || !testCase.UnauthorizedEffectsAbsent {
		return fmt.Errorf("P14 h-reason case %q is open", testCase.ID)
	}
	wantInspects := map[string][]string{
		"mechanical_exact_lookup":             {},
		"single_candidate_pua_new_result":     {"E.11.PUA", "A.10", "A.6.RCD"},
		"multi_candidate_pur_applicable":      {"E.11.PUR", "A.10", "B.3"},
		"recommendation_no_authority_effects": {"E.11.PUR", "A.10"},
	}
	want, present := wantInspects[testCase.ID]
	if !present || !slices.Equal(testCase.RequiredInspects, want) {
		return fmt.Errorf("P14 h-reason case %q inspect closure differs", testCase.ID)
	}
	if testCase.ID == "single_candidate_pua_new_result" &&
		(testCase.ExpectedClosure != "newlyCurrentSubjectResult" ||
			testCase.ExpectedA10Path == nil ||
			testCase.Basis.LocalClaimContext == nil ||
			testCase.ExpectedLocalClaimBasis == nil ||
			testCase.ExpectedLocalClaim == nil) {
		return fmt.Errorf("P14 single-candidate PUA closure differs")
	}
	if testCase.ID == "single_candidate_pua_new_result" {
		if err := validateP14AgentFPFLocalClaimClosure(testCase); err != nil {
			return err
		}
	} else if testCase.ExpectedA10Path != nil ||
		testCase.Basis.LocalClaimContext != nil ||
		testCase.ExpectedLocalClaimBasis != nil ||
		testCase.ExpectedLocalClaim != nil ||
		testCase.ExpectedRelativeObjectRef != "" ||
		testCase.ExpectedBasisKind != "" {
		return fmt.Errorf("P14 non-PUA-result case carries a local-claim closure")
	}
	if testCase.ID == "multi_candidate_pur_applicable" &&
		testCase.FiveAspectAggregateCount != 2 {
		return fmt.Errorf("P14 multi-candidate PUR aggregate count differs")
	}
	if testCase.ID == "recommendation_no_authority_effects" &&
		testCase.ExpectedRecommendation != "present" {
		return fmt.Errorf("P14 advisory recommendation case differs")
	}
	return nil
}

func sealP14AgentFPFCandidateVersion(
	testCases []p14AgentFPFPatternUseCase,
	version string,
) ([]p14AgentFPFPatternUseCase, error) {
	version = strings.TrimSpace(version)
	if version == "" || strings.ContainsAny(version, "\r\n\t ") {
		return nil, fmt.Errorf("P14 installed candidate version is invalid")
	}
	sealed := slices.Clone(testCases)
	found := false
	for index := range sealed {
		if sealed[index].ID != "mechanical_exact_lookup" {
			continue
		}
		sealed[index].ExpectedResultAssertion = version
		found = true
	}
	if !found {
		return nil, fmt.Errorf("P14 mechanical version case is absent")
	}
	return sealed, nil
}

func validateP14SealedAgentFPFPatternUseCase(
	testCase p14AgentFPFPatternUseCase,
) error {
	if err := validateP14AgentFPFPatternUseCase(testCase); err != nil {
		return err
	}
	if strings.TrimSpace(testCase.ExpectedResultAssertion) == "" {
		return fmt.Errorf("P14 h-reason case %q has no result assertion", testCase.ID)
	}
	return nil
}

func validateP14AgentFPFCandidateProtocolVersion(
	testCases []p14AgentFPFPatternUseCase,
	protocol p14MCPProtocolDiscovery,
) error {
	version, err := p14MCPProtocolServerVersion(protocol)
	if err != nil {
		return err
	}
	found := false
	for _, testCase := range testCases {
		if err := validateP14SealedAgentFPFPatternUseCase(testCase); err != nil {
			return err
		}
		if testCase.ID != "mechanical_exact_lookup" {
			continue
		}
		if testCase.ExpectedResultAssertion != version {
			return fmt.Errorf("P14 mechanical version differs from MCP initialize")
		}
		found = true
	}
	if !found {
		return fmt.Errorf("P14 mechanical version case is absent")
	}
	return nil
}

func p14AgentFPFPromptText(
	base string,
	testCase p14AgentFPFPatternUseCase,
) string {
	basisRaw, err := marshalP14CanonicalJSON(testCase.Basis)
	if err != nil {
		panic(err)
	}
	versionBasis := ""
	if testCase.ID == "mechanical_exact_lookup" &&
		strings.TrimSpace(testCase.ExpectedResultAssertion) != "" {
		versionBasis = " Exact installed candidate version observed before this turn: " +
			testCase.ExpectedResultAssertion + "."
	}
	return base + " Current case: " + testCase.Query + versionBasis +
		" Sealed current basis: " + string(basisRaw) + "." +
		" End the final answer with exactly one line beginning " +
		p14AgentFPFObservationPrefix +
		" followed by one JSON object using schema " +
		p14AgentFPFCaseObservationSchema +
		". Report only the useful result assertion, optional structured A.10 path when supplied, its independently observed result_identity and result_constitution, the full local_claim_derivation_basis and local_claim_assertion when the sealed context requires them, direct basis, relative object, basis kind, stop-or-return condition, claim classes, selection basis, closure, PUA inspection positions, candidate five-aspect judgements and aggregates, recommendation, and mutation ledger actually produced in this turn. A local-claim derivation must use the exact independent base locators and must not use E.11.PUA:4.5 or any result-closure schema as a derivation base."
}

func parseP14AgentFPFCaseObservation(
	text string,
) (p14AgentFPFCaseObservation, error) {
	matches := make([]string, 0, 1)
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.HasPrefix(line, p14AgentFPFObservationPrefix) {
			matches = append(matches, strings.TrimPrefix(line, p14AgentFPFObservationPrefix))
			continue
		}
		return p14AgentFPFCaseObservation{}, fmt.Errorf(
			"P14 agent FPF final response contains non-observation text",
		)
	}
	if len(matches) != 1 || strings.TrimSpace(matches[0]) == "" {
		return p14AgentFPFCaseObservation{}, fmt.Errorf(
			"P14 agent FPF final response has %d observation payloads",
			len(matches),
		)
	}
	var observation p14AgentFPFCaseObservation
	decoder := json.NewDecoder(strings.NewReader(matches[0]))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&observation); err != nil {
		return p14AgentFPFCaseObservation{}, fmt.Errorf(
			"decode P14 agent FPF final observation: %w",
			err,
		)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return p14AgentFPFCaseObservation{}, fmt.Errorf(
			"P14 agent FPF final observation has trailing JSON",
		)
	}
	return observation, nil
}

func validateP14AgentFPFCaseObservation(
	testCase p14AgentFPFPatternUseCase,
	observation p14AgentFPFCaseObservation,
) error {
	if observation.Schema != p14AgentFPFCaseObservationSchema ||
		observation.CaseID != testCase.ID ||
		!slices.Equal(observation.ClaimClasses, testCase.ExpectedClaimClasses) ||
		observation.SelectionBasis != testCase.ExpectedSelectionBasis ||
		observation.Closure != testCase.ExpectedClosure ||
		observation.ResultAssertion != testCase.ExpectedResultAssertion ||
		!p14AgentFPFObservedA10PathMatches(testCase, observation.A10Path) ||
		!reflect.DeepEqual(
			observation.LocalClaimDerivation,
			testCase.ExpectedLocalClaimBasis,
		) ||
		!reflect.DeepEqual(
			observation.LocalClaimAssertion,
			testCase.ExpectedLocalClaim,
		) ||
		observation.DirectBasis != testCase.ExpectedDirectBasis ||
		observation.RelativeObjectRef != testCase.ExpectedRelativeObjectRef ||
		observation.BasisKind != testCase.ExpectedBasisKind ||
		observation.StopOrReturn != testCase.ExpectedStopOrReturn ||
		!slices.Equal(observation.PUAInspectionPositions, testCase.ExpectedPUAPositions) ||
		!p14AgentFPFObservedJudgementsMatch(testCase, observation.CandidateJudgements) ||
		observation.Recommendation != testCase.ExpectedRecommendation ||
		observation.RecommendationCandidate != testCase.RecommendationCandidate ||
		observation.MutationLedger.Writes != testCase.ExpectedWrites ||
		!slices.Equal(observation.MutationLedger.Effects, testCase.ExpectedEffects) {
		return fmt.Errorf(
			"P14 agent FPF final observation differs for %q",
			testCase.ID,
		)
	}
	for _, judgement := range observation.CandidateJudgements {
		if !p14AgentFPFAggregateMatchesFit(judgement.Fit, judgement.Aggregate) {
			return fmt.Errorf(
				"P14 agent FPF observed aggregate differs for %q/%q",
				testCase.ID,
				judgement.Candidate,
			)
		}
	}
	if strings.Contains(strings.ToLower(observation.SelectionBasis), "rank") {
		return fmt.Errorf("P14 agent FPF observed rank selection for %q", testCase.ID)
	}
	observedCase := testCase
	observedCase.ExpectedA10Path = observation.A10Path
	observedCase.ExpectedLocalClaimBasis = observation.LocalClaimDerivation
	observedCase.ExpectedLocalClaim = observation.LocalClaimAssertion
	observedCase.ExpectedRelativeObjectRef = observation.RelativeObjectRef
	observedCase.ExpectedBasisKind = observation.BasisKind
	if testCase.ID == "single_candidate_pua_new_result" {
		if err := validateP14AgentFPFLocalClaimClosure(observedCase); err != nil {
			return fmt.Errorf(
				"P14 agent FPF observed local-claim closure differs for %q: %w",
				testCase.ID,
				err,
			)
		}
	}
	return nil
}

func p14AgentFPFObservedA10PathMatches(
	testCase p14AgentFPFPatternUseCase,
	observed *p14AgentFPFA10Path,
) bool {
	expected := testCase.ExpectedA10Path
	if expected == nil || observed == nil {
		return expected == nil && observed == nil
	}
	return reflect.DeepEqual(*expected, *observed) &&
		testCase.Basis.A10SourceBasis != nil &&
		validateP14AgentFPFA10DerivedPath(
			*testCase.Basis.A10SourceBasis,
			*observed,
		) == nil
}

func p14AgentFPFObservedJudgementsMatch(
	testCase p14AgentFPFPatternUseCase,
	observed []p14AgentFPFJudgement,
) bool {
	if len(observed) != len(testCase.ExpectedJudgements) {
		return false
	}
	for index, judgement := range observed {
		expected := testCase.ExpectedJudgements[index]
		if judgement.Candidate != expected.Candidate ||
			judgement.Aggregate != expected.Aggregate ||
			len(judgement.Fit) != 5 ||
			len(expected.Fit) != 5 {
			return false
		}
		candidate, present := p14AgentFPFCandidateBasis(
			testCase.Basis,
			judgement.Candidate,
		)
		if !present {
			return false
		}
		for _, aspect := range []string{
			"problemFrame",
			"forces",
			"solutionConditions",
			"ordinaryBoundary",
			"resultAndReceivingUse",
		} {
			assessment, present := judgement.Fit[aspect]
			expectedAssessment, expectedPresent := expected.Fit[aspect]
			if !present || !expectedPresent ||
				assessment.Result != expectedAssessment.Result ||
				!p14AgentFPFRationaleLinked(
					assessment.Rationale,
					p14AgentFPFAspectBasisText(
						testCase.Basis,
						candidate,
						aspect,
					),
				) {
				return false
			}
		}
	}
	return true
}

func p14AgentFPFCandidateBasis(
	basis p14AgentFPFInputBasis,
	patternID string,
) (p14AgentFPFInputCandidate, bool) {
	for _, candidate := range basis.Candidates {
		if candidate.PatternID != patternID {
			continue
		}
		return candidate, true
	}
	return p14AgentFPFInputCandidate{}, false
}

func p14AgentFPFAspectBasisText(
	basis p14AgentFPFInputBasis,
	candidate p14AgentFPFInputCandidate,
	aspect string,
) string {
	switch aspect {
	case "problemFrame":
		return basis.Subject + " " + candidate.CurrentCondition
	case "forces", "solutionConditions":
		return candidate.CurrentCondition + " " + strings.Join(basis.CurrentFacts, " ")
	case "ordinaryBoundary":
		return candidate.OrdinaryBoundary
	case "resultAndReceivingUse":
		return strings.Join([]string{
			candidate.ExpectedFirstResult,
			candidate.ReceivingUse,
			basis.ReceivingUse,
		}, " ")
	default:
		return ""
	}
}

func p14AgentFPFRationaleLinked(rationale string, basis string) bool {
	rationaleTokens := p14AgentFPFSemanticTokens(rationale)
	if len(rationaleTokens) == 0 {
		return false
	}
	basisTokens := p14AgentFPFSemanticTokens(basis)
	for token := range rationaleTokens {
		if _, present := basisTokens[token]; present {
			return true
		}
	}
	return false
}

func p14AgentFPFSemanticTokens(text string) map[string]struct{} {
	generic := map[string]struct{}{
		"basis": {}, "candidate": {}, "current": {}, "exact": {},
		"pattern": {}, "result": {},
	}
	result := make(map[string]struct{})
	text = p14AgentFPFCamelBoundary.ReplaceAllString(text, `$1 $2`)
	for _, token := range strings.FieldsFunc(
		strings.ToLower(text),
		func(r rune) bool {
			return (r < 'a' || r > 'z') && (r < '0' || r > '9')
		},
	) {
		if _, excluded := generic[token]; len(token) >= 5 && !excluded {
			result[token] = struct{}{}
		}
	}
	return result
}

func p14ExpectedAgentFPFCaseObservation(
	testCase p14AgentFPFPatternUseCase,
) p14AgentFPFCaseObservation {
	return p14AgentFPFCaseObservation{
		Schema:                  p14AgentFPFCaseObservationSchema,
		CaseID:                  testCase.ID,
		ClaimClasses:            slices.Clone(testCase.ExpectedClaimClasses),
		SelectionBasis:          testCase.ExpectedSelectionBasis,
		Closure:                 testCase.ExpectedClosure,
		ResultAssertion:         testCase.ExpectedResultAssertion,
		A10Path:                 p14AgentFPFCloneA10Path(testCase.ExpectedA10Path),
		LocalClaimDerivation:    p14AgentFPFClone(testCase.ExpectedLocalClaimBasis),
		LocalClaimAssertion:     p14AgentFPFClone(testCase.ExpectedLocalClaim),
		DirectBasis:             testCase.ExpectedDirectBasis,
		RelativeObjectRef:       testCase.ExpectedRelativeObjectRef,
		BasisKind:               testCase.ExpectedBasisKind,
		StopOrReturn:            testCase.ExpectedStopOrReturn,
		PUAInspectionPositions:  slices.Clone(testCase.ExpectedPUAPositions),
		CandidateJudgements:     slices.Clone(testCase.ExpectedJudgements),
		Recommendation:          testCase.ExpectedRecommendation,
		RecommendationCandidate: testCase.RecommendationCandidate,
		MutationLedger: p14AgentFPFMutationLedger{
			Writes:  testCase.ExpectedWrites,
			Effects: slices.Clone(testCase.ExpectedEffects),
		},
	}
}

func p14AgentFPFCloneA10Path(
	input *p14AgentFPFA10Path,
) *p14AgentFPFA10Path {
	return p14AgentFPFClone(input)
}

func p14AgentFPFClone[T any](input *T) *T {
	if input == nil {
		return nil
	}
	raw, err := json.Marshal(input)
	if err != nil {
		panic(err)
	}
	var cloned T
	if err := json.Unmarshal(raw, &cloned); err != nil {
		panic(err)
	}
	return &cloned
}

func p14AgentFPFCaseByID(
	id string,
) (p14AgentFPFPatternUseCase, error) {
	cases, _, _, err := loadP14AgentFPFPatternUseCases()
	if err != nil {
		return p14AgentFPFPatternUseCase{}, err
	}
	for _, testCase := range cases {
		if testCase.ID == id {
			return testCase, nil
		}
	}
	return p14AgentFPFPatternUseCase{}, fmt.Errorf(
		"P14 agent FPF case %q is absent",
		id,
	)
}

func p14AgentFPFJSONText(raw json.RawMessage) string {
	value := ""
	_ = json.Unmarshal(raw, &value)
	return value
}

func decodeP14AgentFPFStrict(raw json.RawMessage, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("structured value has trailing JSON")
	}
	return nil
}

type p14AgentFPFPatternUseNormalizedOutput struct {
	Schema               string   `json:"schema"`
	SemanticCorpusDigest string   `json:"semantic_corpus_digest"`
	CaseDigests          []string `json:"case_digests"`
	ObservedCaseDigests  []string `json:"observed_case_digests"`
	FPFCallCounts        []int    `json:"fpf_call_counts"`
	UnauthorizedEffects  int      `json:"unauthorized_effects"`
}

func p14CodexMCPAgentFPFCalls(
	_ preparedP14Scenario,
	request preparedP14Request,
) ([]p14CodexMCPCallDefinition, error) {
	var surface p14LiveProtocolSurface
	if err := decodeP14StrictCompactJSON(
		request.CanonicalPayload,
		&surface,
		"actual Codex MCP agent FPF protocol",
	); err != nil {
		return nil, err
	}
	if surface.Surface != "live_mcp" ||
		surface.AgentPrompt == nil ||
		!validP14CandidateVersion(surface.CandidateVersion) ||
		len(surface.AgentFPFCases) != len(p14AgentFPFPatternUseCaseIDs) ||
		!validP14Digest(surface.SemanticCorpusDigest) ||
		!fullP14GitRevision.MatchString(surface.SemanticCorpusRevision) {
		return nil, fmt.Errorf("P14 agent FPF protocol is not sealed")
	}
	definitions := make([]p14CodexMCPCallDefinition, 0)
	for _, testCase := range surface.AgentFPFCases {
		if err := validateP14SealedAgentFPFPatternUseCase(testCase); err != nil {
			return nil, err
		}
		promptText := p14AgentFPFPromptText(surface.AgentPrompt.Text, testCase)
		prompt := p14CodexMCPPlannedAgentPrompt{
			ID:                    "agent_fpf_pattern_use_" + testCase.ID + "_prompt",
			TextCanonical:         promptText,
			TextDigest:            p14Digest([]byte(promptText)),
			ExpectedToolCallCount: len(testCase.Calls),
		}
		for index, call := range testCase.Calls {
			args, err := cloneP14JSONMap(call.Args)
			if err != nil {
				return nil, err
			}
			definition := p14CodexMCPCallDefinition{
				CaseID: fmt.Sprintf("%s_%02d", testCase.ID, index+1),
				Tool:   call.Tool,
				Args:   args,
			}
			if index == 0 {
				definition.AgentPrompt = &prompt
			}
			definitions = append(definitions, definition)
		}
	}
	return definitions, nil
}

func normalizeP14CodexMCPAgentFPFPatternUse(
	_ preparedRequestOracleCarrier,
	_ preparedP14Scenario,
	request preparedP14Request,
	evidence []p14CodexMCPCallEvidence,
) (p14CodexMCPFamilyResult, error) {
	var surface p14LiveProtocolSurface
	if err := decodeP14StrictCompactJSON(
		request.CanonicalPayload,
		&surface,
		"actual Codex agent FPF pattern-use surface",
	); err != nil {
		return p14CodexMCPFamilyResult{}, err
	}
	wantCount := 0
	for _, testCase := range surface.AgentFPFCases {
		wantCount += len(testCase.Calls)
	}
	if len(evidence) != wantCount {
		return p14CodexMCPNormalizedFailure(
			"agent_fpf_pattern_use_mismatch",
			"agent FPF call count differs from the sealed corpus subset",
			"closed_agent_fpf_pattern_use",
		), nil
	}
	caseDigests := make([]string, 0, len(surface.AgentFPFCases))
	observedCaseDigests := make([]string, 0, len(surface.AgentFPFCases))
	fpfCallCounts := make([]int, 0, len(surface.AgentFPFCases))
	evidenceIndex := 0
	for _, testCase := range surface.AgentFPFCases {
		caseDigests = append(caseDigests, testCase.ScenarioDigest)
		fpfCalls := 0
		var observedFinal p14AgentFPFCaseObservation
		for callIndex, call := range testCase.Calls {
			observed := evidence[evidenceIndex]
			evidenceIndex++
			wantCaseID := fmt.Sprintf("%s_%02d", testCase.ID, callIndex+1)
			if observed.CaseID != wantCaseID || observed.Response.IsError ||
				(callIndex == 0 &&
					(observed.AgentPrompt == nil || observed.AgentResponse == nil)) {
				return p14CodexMCPNormalizedFailure(
					"agent_fpf_pattern_use_mismatch",
					"agent FPF prompt or response differs for "+wantCaseID,
					"agent_prompt_transcript_bound",
					"closed_agent_fpf_pattern_use",
				), nil
			}
			if callIndex == 0 {
				parsed, parseErr := parseP14AgentFPFCaseObservation(
					observed.AgentResponse.TextCanonical,
				)
				if parseErr != nil ||
					validateP14AgentFPFCaseObservation(testCase, parsed) != nil {
					return p14CodexMCPNormalizedFailure(
						"agent_fpf_pattern_use_mismatch",
						"agent FPF final semantic observation differs for "+testCase.ID,
						"assistant_final_response_bound",
						"closed_agent_fpf_pattern_use",
					), nil
				}
				observedFinal = parsed
			}
			body, err := p14CodexMCPResponseBody(observed)
			if err != nil {
				return p14CodexMCPFamilyResult{}, err
			}
			if len(bytes.TrimSpace(body)) == 0 {
				return p14CodexMCPNormalizedFailure(
					"agent_fpf_pattern_use_mismatch",
					"agent FPF response body is empty",
					"captured_response_digest_bound",
				), nil
			}
			if call.Args["action"] == "fpf" {
				fpfCalls++
				identifier, _ := call.Args["identifier"].(string)
				if identifier == "" ||
					!bytes.Contains(body, []byte(`"identifier":"`+identifier+`"`)) ||
					!bytes.Contains(body, []byte(`"pattern_id":"`+identifier+`"`)) {
					return p14CodexMCPNormalizedFailure(
						"agent_fpf_pattern_use_mismatch",
						"exact inspected pattern body differs for "+identifier,
						"closed_agent_fpf_pattern_use",
					), nil
				}
			}
		}
		observedRaw, err := marshalP14CanonicalJSON(observedFinal)
		if err != nil {
			return p14CodexMCPFamilyResult{}, err
		}
		observedCaseDigests = append(
			observedCaseDigests,
			p14Digest(observedRaw),
		)
		if fpfCalls != testCase.ExpectedFPFCalls {
			return p14CodexMCPNormalizedFailure(
				"agent_fpf_pattern_use_mismatch",
				"agent FPF case call count differs",
				"mechanical_control_made_no_fpf_call",
				"closed_agent_fpf_pattern_use",
			), nil
		}
		fpfCallCounts = append(fpfCallCounts, fpfCalls)
	}
	normalized := p14AgentFPFPatternUseNormalizedOutput{
		Schema:               "haft.p14.agent-fpf-pattern-use-output/v1",
		SemanticCorpusDigest: surface.SemanticCorpusDigest,
		CaseDigests:          caseDigests,
		ObservedCaseDigests:  observedCaseDigests,
		FPFCallCounts:        fpfCallCounts,
		UnauthorizedEffects:  0,
	}
	return p14CodexMCPNormalizedResult(
		normalized,
		nil,
		"agent_prompt_transcript_bound",
		"assistant_final_response_bound",
		"mechanical_control_made_no_fpf_call",
		"single_candidate_inspected_pua_and_direct_pattern",
		"multi_candidate_inspected_pur_and_every_candidate",
		"five_aspect_aggregate_and_no_rank_selection_bound",
		"zero_unauthorized_effects_bound",
		"closed_agent_fpf_pattern_use",
	)
}

func p14AgentFPFPromptCase(caseID string) bool {
	for _, id := range p14AgentFPFPatternUseCaseIDs {
		if caseID == id+"_01" {
			return true
		}
	}
	return false
}

func p14AgentFPFCaseIDFromPromptID(promptID string) (string, bool) {
	const prefix = "agent_fpf_pattern_use_"
	const suffix = "_prompt"
	if !strings.HasPrefix(promptID, prefix) ||
		!strings.HasSuffix(promptID, suffix) {
		return "", false
	}
	id := strings.TrimSuffix(strings.TrimPrefix(promptID, prefix), suffix)
	for _, candidate := range p14AgentFPFPatternUseCaseIDs {
		if id == candidate {
			return id, true
		}
	}
	return "", false
}

func TestP14AgentFPFPatternUseBuilderConsumesSealedCorpusSubset(t *testing.T) {
	root, err := p14RepositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	contract, _, err := loadRequestOracleContract(root)
	if err != nil {
		t.Fatal(err)
	}
	declared, err := findP14ScenarioContract(contract, "agent_fpf_pattern_use")
	if err != nil {
		t.Fatal(err)
	}
	scenario, err := buildP14LiveProtocolScenario(declared)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateP14LiveProtocolPreparedScenario(declared, scenario); err != nil {
		t.Fatal(err)
	}
	request, present := p14PreparedSurfaceRequest(scenario, "live_mcp")
	if !present {
		t.Fatal("P14 agent FPF pattern use has no live MCP request")
	}
	var surface p14LiveProtocolSurface
	if err := decodeP14StrictCompactJSON(
		request.CanonicalPayload,
		&surface,
		"P14 agent FPF pattern-use surface",
	); err != nil {
		t.Fatal(err)
	}
	ids := make([]string, 0, len(surface.AgentFPFCases))
	for _, testCase := range surface.AgentFPFCases {
		ids = append(ids, testCase.ID)
	}
	if !slices.Equal(ids, p14AgentFPFPatternUseCaseIDs) ||
		!validP14Digest(surface.SemanticCorpusDigest) ||
		!fullP14GitRevision.MatchString(surface.SemanticCorpusRevision) {
		t.Fatalf("P14 agent FPF sealed subset = %#v", surface)
	}
	tampered := surface
	tampered.AgentFPFCases = slices.Clone(surface.AgentFPFCases)
	tampered.AgentFPFCases[2].RankSelectionForbidden = false
	raw, err := marshalP14CanonicalJSON(tampered)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(raw, []byte(request.CanonicalPayload)) {
		t.Fatal("P14 agent FPF tamper did not change sealed bytes")
	}
}

func TestP14AgentFPFFinalObservationFailsClosed(t *testing.T) {
	cases, _, _, err := loadP14AgentFPFPatternUseCases()
	if err != nil {
		t.Fatal(err)
	}
	cases, err = sealP14AgentFPFCandidateVersion(cases, "9.2.0")
	if err != nil {
		t.Fatal(err)
	}
	byID := make(map[string]p14AgentFPFPatternUseCase, len(cases))
	for _, testCase := range cases {
		byID[testCase.ID] = testCase
		observation := p14ExpectedAgentFPFCaseObservation(testCase)
		raw, err := marshalP14CanonicalJSON(observation)
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := parseP14AgentFPFCaseObservation(
			p14AgentFPFObservationPrefix + string(raw),
		)
		if err != nil {
			t.Fatal(err)
		}
		if err := validateP14AgentFPFCaseObservation(testCase, parsed); err != nil {
			t.Fatal(err)
		}
	}

	for name, response := range map[string]string{
		"prose": "answer\n" + p14AgentFPFObservationPrefix + `{}`,
		"duplicate": p14AgentFPFObservationPrefix + `{}` + "\n" +
			p14AgentFPFObservationPrefix + `{}`,
		"trailing_json": p14AgentFPFObservationPrefix + `{} {}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseP14AgentFPFCaseObservation(response); err == nil {
				t.Fatal("P14 final observation parser accepted an open response")
			}
		})
	}

	t.Run("a10_path_absent", func(t *testing.T) {
		testCase := byID["single_candidate_pua_new_result"]
		observation := p14ExpectedAgentFPFCaseObservation(testCase)
		observation.A10Path = nil
		if err := validateP14AgentFPFCaseObservation(testCase, observation); err == nil {
			t.Fatal("P14 single-candidate result accepted an absent A.10 path")
		}
	})
	t.Run("a10_result_identity_forged", func(t *testing.T) {
		testCase := byID["single_candidate_pua_new_result"]
		observation := p14ExpectedAgentFPFCaseObservation(testCase)
		observation.A10Path.ResultIdentity.EpistemeRef = "path:forged"
		if err := validateP14AgentFPFCaseObservation(testCase, observation); err == nil {
			t.Fatal("P14 single-candidate result accepted a forged A.10 identity")
		}
	})
	t.Run("local_claim_derivation_absent", func(t *testing.T) {
		testCase := byID["single_candidate_pua_new_result"]
		observation := p14ExpectedAgentFPFCaseObservation(testCase)
		observation.LocalClaimDerivation = nil
		if err := validateP14AgentFPFCaseObservation(testCase, observation); err == nil {
			t.Fatal("P14 single-candidate result accepted an absent local-claim derivation")
		}
	})
	t.Run("local_claim_result_not_independently_observed", func(t *testing.T) {
		testCase := byID["single_candidate_pua_new_result"]
		observation := p14ExpectedAgentFPFCaseObservation(testCase)
		observation.LocalClaimDerivation.ResultIdentity.EpistemeRef = "path:forged"
		if err := validateP14AgentFPFCaseObservation(testCase, observation); err == nil {
			t.Fatal("P14 local claim accepted a result other than the observed A.10 result")
		}
	})
	t.Run("local_claim_assertion_constitution_forged", func(t *testing.T) {
		testCase := byID["single_candidate_pua_new_result"]
		observation := p14ExpectedAgentFPFCaseObservation(testCase)
		observation.LocalClaimAssertion.ClaimConstitution.Obtains = false
		if err := validateP14AgentFPFCaseObservation(testCase, observation); err == nil {
			t.Fatal("P14 local claim accepted a non-obtaining claim constitution")
		}
	})
	t.Run("circular_local_claim_bases", func(t *testing.T) {
		testCase := byID["single_candidate_pua_new_result"]
		for name, mutate := range map[string]func(*p14AgentFPFLocalClaimDerivationBasis){
			"pua_closure_locator": func(basis *p14AgentFPFLocalClaimDerivationBasis) {
				basis.BasePredicates[3].ClaimGraphLocator = "E.11.PUA:4.5"
			},
			"closure_schema": func(basis *p14AgentFPFLocalClaimDerivationBasis) {
				basis.BasePredicates[0].ObtainingLaw = "PatternUseResultClosureFinding@Context"
			},
		} {
			t.Run(name, func(t *testing.T) {
				tampered := testCase
				tampered.ExpectedLocalClaimBasis = p14AgentFPFClone(
					testCase.ExpectedLocalClaimBasis,
				)
				mutate(tampered.ExpectedLocalClaimBasis)
				if err := validateP14AgentFPFPatternUseCase(tampered); err == nil {
					t.Fatal("P14 case accepted a circular local-claim derivation base")
				}
			})
		}
	})
	t.Run("a10_path_on_other_case", func(t *testing.T) {
		testCase := byID["multi_candidate_pur_applicable"]
		observation := p14ExpectedAgentFPFCaseObservation(testCase)
		observation.A10Path = p14AgentFPFCloneA10Path(
			byID["single_candidate_pua_new_result"].ExpectedA10Path,
		)
		if err := validateP14AgentFPFCaseObservation(testCase, observation); err == nil {
			t.Fatal("P14 PUR result accepted an invented A.10 path")
		}
	})
	t.Run("generic_rationales", func(t *testing.T) {
		testCase := byID["multi_candidate_pur_applicable"]
		observation := p14ExpectedAgentFPFCaseObservation(testCase)
		observation.CandidateJudgements = slices.Clone(
			observation.CandidateJudgements,
		)
		for index := range observation.CandidateJudgements {
			fit := make(map[string]p14AgentFPFFitAssessment, 5)
			for aspect, assessment := range observation.CandidateJudgements[index].Fit {
				assessment.Rationale = "current"
				fit[aspect] = assessment
			}
			observation.CandidateJudgements[index].Fit = fit
		}
		if err := validateP14AgentFPFCaseObservation(testCase, observation); err == nil {
			t.Fatal("P14 five-aspect result accepted generic rationales")
		}
	})
	t.Run("useful_result_absent", func(t *testing.T) {
		testCase := byID["mechanical_exact_lookup"]
		observation := p14ExpectedAgentFPFCaseObservation(testCase)
		observation.ResultAssertion = ""
		if err := validateP14AgentFPFCaseObservation(testCase, observation); err == nil {
			t.Fatal("P14 mechanical result accepted an absent exact version")
		}
	})
}
