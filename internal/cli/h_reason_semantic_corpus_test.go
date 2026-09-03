package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/artifact"
)

const hReasonSemanticCorpusPath = "internal/cli/testdata/h-reason-semantic-corpus.v1.json"

var hReasonFitAspectNames = []string{
	"problemFrame",
	"forces",
	"solutionConditions",
	"ordinaryBoundary",
	"resultAndReceivingUse",
}

var hReasonSemanticSHA256Token = regexp.MustCompile(`sha256:([0-9A-Za-z]+)`)
var hReasonSemanticSHA256Value = regexp.MustCompile(`^[0-9a-f]{64}$`)

type hReasonSemanticCorpus struct {
	ContractVersion string                     `json:"contract_version"`
	SourceBasis     hReasonSemanticSourceBasis `json:"source_basis"`
	CarrierBasis    []hReasonSemanticCarrier   `json:"carrier_basis"`
	HostProjections []string                   `json:"host_projections"`
	Scenarios       []hReasonSemanticScenario  `json:"scenarios"`
}

type hReasonSemanticSourceBasis struct {
	FPFRevision   string `json:"fpf_revision"`
	FPFSpecDigest string `json:"fpf_spec_digest"`
}

type hReasonSemanticCarrier struct {
	Role   string `json:"role"`
	Path   string `json:"path"`
	Digest string `json:"digest"`
}

type hReasonSemanticFitAspect struct {
	Result    string `json:"result"`
	Rationale string `json:"rationale"`
}

type hReasonSemanticScenario struct {
	ID              string                  `json:"id"`
	Requirement     string                  `json:"requirement"`
	RelianceProfile string                  `json:"reliance_profile"`
	Input           map[string]any          `json:"input"`
	Events          []hReasonSemanticEvent  `json:"events"`
	Expected        hReasonSemanticExpected `json:"expected"`
}

type hReasonSemanticInputBasis struct {
	Subject           string                            `json:"subject"`
	Candidates        []hReasonSemanticInputCandidate   `json:"candidates"`
	CurrentFacts      []string                          `json:"current_facts"`
	ReceivingUse      string                            `json:"receiving_use"`
	A10SourceBasis    *hReasonSemanticA10SourceBasis    `json:"a10_source_basis,omitempty"`
	LocalClaimContext *hReasonSemanticLocalClaimContext `json:"local_claim_context,omitempty"`
}

type hReasonSemanticInputCandidate struct {
	PatternID           string `json:"pattern_id"`
	CurrentCondition    string `json:"current_condition"`
	ExpectedFirstResult string `json:"expected_first_result"`
	ReceivingUse        string `json:"receiving_use"`
	OrdinaryBoundary    string `json:"ordinary_boundary"`
}

type hReasonSemanticA10Path struct {
	PathProfile                   string                            `json:"path_profile"`
	PathRef                       string                            `json:"path_ref"`
	ReliedOnClaimRef              string                            `json:"relied_on_claim_ref"`
	ClaimGraph                    string                            `json:"claim_graph"`
	EntityOfConcernRef            string                            `json:"entity_of_concern_ref"`
	ReferenceSchemeRef            string                            `json:"reference_scheme_ref"`
	LocalResult                   string                            `json:"local_result"`
	ResultRuleOwner               string                            `json:"result_rule_owner"`
	ResultRule                    string                            `json:"result_rule"`
	AttemptedUseRef               string                            `json:"attempted_use_ref"`
	ObservedCarrierFacts          []string                          `json:"observed_carrier_facts"`
	EstablishedDirectRelationRefs []string                          `json:"established_direct_relation_refs"`
	NarrowedReversibleUse         string                            `json:"narrowed_reversible_use"`
	TimeWindow                    string                            `json:"time_window"`
	RivalExplanation              string                            `json:"rival_explanation"`
	UnsupportedAttemptedUse       string                            `json:"unsupported_attempted_use"`
	RelianceDisposition           string                            `json:"reliance_disposition"`
	ReopenTrigger                 string                            `json:"reopen_trigger"`
	Contest                       hReasonSemanticA10Contest         `json:"contest"`
	UnresolvedGaps                []string                          `json:"unresolved_gaps"`
	ResultIdentity                hReasonSemanticC21Identity        `json:"result_identity"`
	ResultConstitution            hReasonSemanticGroundingBasisPair `json:"result_constitution"`
}

type hReasonSemanticA10SourceBasis struct {
	ReliedOnClaimRef              string                    `json:"relied_on_claim_ref"`
	ClaimGraph                    string                    `json:"claim_graph"`
	EntityOfConcernRef            string                    `json:"entity_of_concern_ref"`
	ReferenceSchemeRef            string                    `json:"reference_scheme_ref"`
	LocalResultRuleOwner          string                    `json:"local_result_rule_owner"`
	LocalResultRule               string                    `json:"local_result_rule"`
	AttemptedUseRef               string                    `json:"attempted_use_ref"`
	ObservedCarrierFacts          []string                  `json:"observed_carrier_facts"`
	EstablishedDirectRelationRefs []string                  `json:"established_direct_relation_refs"`
	NarrowedReversibleUse         string                    `json:"narrowed_reversible_use"`
	TimeWindow                    string                    `json:"time_window"`
	RivalExplanation              string                    `json:"rival_explanation"`
	UnsupportedAttemptedUse       string                    `json:"unsupported_attempted_use"`
	Contest                       hReasonSemanticA10Contest `json:"contest"`
	UnresolvedGaps                []string                  `json:"unresolved_gaps"`
}

type hReasonSemanticA10Contest struct {
	AffectedPartyView         string `json:"affected_party_view"`
	AccountableReviewRoute    string `json:"accountable_review_route"`
	AllowedChallengeEvidence  string `json:"allowed_challenge_evidence"`
	PossibleDispositionChange string `json:"possible_disposition_change"`
	OutcomeRecordCondition    string `json:"outcome_record_condition"`
	ReopenTrigger             string `json:"reopen_trigger"`
}

type hReasonSemanticC21Identity struct {
	EpistemeRef        string `json:"episteme_ref"`
	ClaimGraph         string `json:"claim_graph"`
	EntityOfConcernRef string `json:"entity_of_concern_ref"`
	ReferenceSchemeRef string `json:"reference_scheme_ref"`
}

type hReasonSemanticGroundingBasisPair struct {
	OccurrenceRef                      string   `json:"occurrence_ref"`
	Predicate                          string   `json:"predicate"`
	Participants                       []string `json:"participants"`
	Obtains                            bool     `json:"obtains"`
	PatternLocator                     string   `json:"pattern_locator"`
	DirectDeclarationClaimGraphLocator string   `json:"direct_declaration_claim_graph_locator"`
}

type hReasonSemanticLocalClaimSubstrate struct {
	DefinitionIdentity     hReasonSemanticC21Identity        `json:"definition_identity"`
	DefinitionConstitution hReasonSemanticGroundingBasisPair `json:"definition_constitution"`
	ConstructorRef         string                            `json:"constructor_ref"`
	ConstructorSemantics   string                            `json:"constructor_semantics"`
	Applicability          string                            `json:"applicability"`
}

type hReasonSemanticLocalClaimParticipant struct {
	Meaning string `json:"meaning"`
	Ref     string `json:"ref"`
}

type hReasonSemanticLocalClaimBasePredicate struct {
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

type hReasonSemanticLocalClaimContext struct {
	BlockedReceivingUse           string                             `json:"blocked_receiving_use"`
	ResultRef                     string                             `json:"result_ref"`
	RelativeObjectRef             string                             `json:"relative_object_ref"`
	RelativeObjectKindRef         string                             `json:"relative_object_kind_ref"`
	RelativeObjectIdentity        hReasonSemanticC21Identity         `json:"relative_object_identity"`
	RelativeObjectConstitution    hReasonSemanticGroundingBasisPair  `json:"relative_object_constitution"`
	ResultExpectationIdentity     hReasonSemanticC21Identity         `json:"result_expectation_identity"`
	ResultExpectationConstitution hReasonSemanticGroundingBasisPair  `json:"result_expectation_constitution"`
	SelectedSubstrate             hReasonSemanticLocalClaimSubstrate `json:"selected_substrate"`
	PositiveCase                  string                             `json:"positive_case"`
	DiscriminatingFailureCase     string                             `json:"discriminating_failure_case"`
	ReceivingUseReplay            string                             `json:"receiving_use_replay"`
	StopOrReturn                  string                             `json:"stop_or_return"`
}

type hReasonSemanticLocalClaimDerivationBasis struct {
	DerivationPatternLocator string                                   `json:"derivation_pattern_locator"`
	Disposition              string                                   `json:"disposition"`
	Context                  hReasonSemanticLocalClaimContext         `json:"context"`
	ResultIdentity           hReasonSemanticC21Identity               `json:"result_identity"`
	ResultConstitution       hReasonSemanticGroundingBasisPair        `json:"result_constitution"`
	Participants             []hReasonSemanticLocalClaimParticipant   `json:"participants"`
	BasePredicates           []hReasonSemanticLocalClaimBasePredicate `json:"base_predicates"`
	CaseFacts                []string                                 `json:"case_facts"`
	SupportOrWarrant         []string                                 `json:"support_or_warrant"`
}

type hReasonSemanticLocalClaimAssertion struct {
	ClaimIdentity     hReasonSemanticC21Identity        `json:"claim_identity"`
	ClaimConstitution hReasonSemanticGroundingBasisPair `json:"claim_constitution"`
	Polarity          string                            `json:"polarity"`
}

type hReasonSemanticEvent struct {
	Kind                         string                                    `json:"kind"`
	Tool                         string                                    `json:"tool,omitempty"`
	CallID                       string                                    `json:"call_id,omitempty"`
	Payload                      json.RawMessage                           `json:"payload,omitempty"`
	Result                       string                                    `json:"result,omitempty"`
	Candidate                    string                                    `json:"candidate,omitempty"`
	Positions                    []string                                  `json:"positions,omitempty"`
	Fit                          map[string]hReasonSemanticFitAspect       `json:"fit,omitempty"`
	Aggregate                    string                                    `json:"aggregate,omitempty"`
	Coordination                 string                                    `json:"coordination,omitempty"`
	CoordinationQuestion         string                                    `json:"coordination_question,omitempty"`
	CandidateUses                []string                                  `json:"candidate_uses,omitempty"`
	Basis                        string                                    `json:"basis,omitempty"`
	From                         string                                    `json:"from,omitempty"`
	To                           string                                    `json:"to,omitempty"`
	Disposition                  string                                    `json:"disposition,omitempty"`
	DirectBasis                  string                                    `json:"direct_basis,omitempty"`
	ExpectationRef               string                                    `json:"expectation_ref,omitempty"`
	ActualResultAssertion        string                                    `json:"actual_result_assertion,omitempty"`
	SubjectIdentity              *hReasonSemanticC21Identity               `json:"subject_identity,omitempty"`
	SubjectGrounding             *hReasonSemanticGroundingBasisPair        `json:"subject_grounding,omitempty"`
	GroundingFinding             *hReasonSemanticC21Identity               `json:"grounding_finding_identity,omitempty"`
	GroundingFindingConstitution *hReasonSemanticGroundingBasisPair        `json:"grounding_finding_constitution,omitempty"`
	ClosureFinding               *hReasonSemanticC21Identity               `json:"closure_finding_identity,omitempty"`
	ClosureFindingConstitution   *hReasonSemanticGroundingBasisPair        `json:"closure_finding_constitution,omitempty"`
	LocalClaimDerivation         *hReasonSemanticLocalClaimDerivationBasis `json:"local_claim_derivation_basis,omitempty"`
	LocalClaimAssertion          *hReasonSemanticLocalClaimAssertion       `json:"local_claim_assertion,omitempty"`
	RelativeObjectRef            string                                    `json:"relative_object_ref,omitempty"`
	BasisKind                    string                                    `json:"basis_kind,omitempty"`
	InterimResult                string                                    `json:"interim_result,omitempty"`
	ReturnCondition              string                                    `json:"return_condition,omitempty"`
	ExpectedResultKind           string                                    `json:"expected_result_kind,omitempty"`
	PatternLocator               string                                    `json:"pattern_locator,omitempty"`
	ClaimClasses                 []string                                  `json:"claim_classes,omitempty"`
	SelectionBasis               string                                    `json:"selection_basis,omitempty"`
	Writes                       *int                                      `json:"writes,omitempty"`
	Effects                      []string                                  `json:"effects,omitempty"`
	SupportRecords               []string                                  `json:"support_records,omitempty"`
	TraceFields                  map[string]string                         `json:"trace_fields,omitempty"`
	ObservationStatus            string                                    `json:"observation_status,omitempty"`
	PlanningLabel                string                                    `json:"planning_label,omitempty"`
	MembershipFields             map[string]string                         `json:"membership_fields,omitempty"`
	ClaimGraph                   []string                                  `json:"claim_graph,omitempty"`
	BoundaryKind                 string                                    `json:"boundary_kind,omitempty"`
	UsesLADE                     *bool                                     `json:"uses_lade,omitempty"`
	AuthorityOwner               string                                    `json:"authority_owner,omitempty"`
	A10Path                      *hReasonSemanticA10Path                   `json:"a10_path,omitempty"`
}

type hReasonSemanticExpected struct {
	FPFCalls         int      `json:"fpf_calls"`
	RequiredInspects []string `json:"required_inspects"`
	Writes           int      `json:"writes"`
	Effects          []string `json:"effects"`
	Recommendation   string   `json:"recommendation"`
	Coordination     string   `json:"coordination"`
	Closure          string   `json:"closure"`
	PlanningLabel    string   `json:"planning_label"`
}

// TestHReasonSemanticCorpusContract is the stable P13 anchor for the
// deterministic source-side corpus. P14 replays its machine-readable subset
// against installed hosts; this test validates the sealed observation grammar,
// not live provider behavior.
func TestHReasonSemanticCorpusContract(t *testing.T) {
	t.Parallel()

	root, err := findRepoRoot()
	if err != nil {
		t.Fatalf("find repository root: %v", err)
	}
	path := filepath.Join(root, filepath.FromSlash(hReasonSemanticCorpusPath))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read semantic corpus: %v", err)
	}
	var corpus hReasonSemanticCorpus
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&corpus); err != nil {
		t.Fatalf("decode semantic corpus: %v", err)
	}
	if corpus.ContractVersion != "haft.h-reason-semantic-corpus.v1" {
		t.Fatalf("contract_version = %q", corpus.ContractVersion)
	}
	assertHReasonSemanticDigestTokens(t, data)
	assertHReasonSemanticSourceBasis(t, root, data, corpus)
	assertHReasonSemanticCarrierBasis(t, root, corpus.CarrierBasis)
	for _, requiredHost := range []string{"canonical", "pi", "codex", "claude"} {
		if !slices.Contains(corpus.HostProjections, requiredHost) {
			t.Errorf("host_projections missing %q", requiredHost)
		}
	}

	requiredIDs := []string{
		"mechanical_exact_lookup",
		"single_candidate_pua_new_result",
		"multi_candidate_pur_applicable",
		"solution_conditions_misfit",
		"missing_fit_basis",
		"complementary_candidates_unordered",
		"prerequisite_result_partial_order",
		"stronger_neighbor_return",
		"pua_preexisting_with_grounding",
		"pua_expected_result_absent",
		"ordinary_bounded_zero_writes",
		"known_absent_without_receiving_use",
		"reliance_bearing_minimal_support",
		"recommendation_no_authority_effects",
		"planning_draft_without_membership",
		"workplan_positive_membership",
		"russian_concern_preserved",
		"a6_mixed_boundary_package",
		"ordinary_human_brief_not_a6_authority",
		"exact_identifier_recovery_executes",
		"profile_change_prepare_no_apply",
	}
	byID := make(map[string]hReasonSemanticScenario, len(corpus.Scenarios))
	for _, scenario := range corpus.Scenarios {
		if _, duplicate := byID[scenario.ID]; duplicate {
			t.Errorf("duplicate scenario id %q", scenario.ID)
			continue
		}
		byID[scenario.ID] = scenario
		t.Run(scenario.ID, func(t *testing.T) {
			assertHReasonSemanticScenario(t, scenario)
		})
	}
	for _, id := range requiredIDs {
		if _, ok := byID[id]; !ok {
			t.Errorf("semantic corpus missing required scenario %q", id)
		}
	}
	assertHReasonCrossScenarioInvariants(t, byID)
}

func TestHReasonSemanticCorpusRelianceBearingMaterializesOneAddressableSupportNote(t *testing.T) {
	t.Parallel()

	root, err := findRepoRoot()
	if err != nil {
		t.Fatalf("find repository root: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(hReasonSemanticCorpusPath)))
	if err != nil {
		t.Fatalf("read semantic corpus: %v", err)
	}
	var corpus hReasonSemanticCorpus
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&corpus); err != nil {
		t.Fatalf("decode semantic corpus: %v", err)
	}
	var reliance hReasonSemanticScenario
	for _, scenario := range corpus.Scenarios {
		if scenario.ID == "reliance_bearing_minimal_support" {
			reliance = scenario
			break
		}
	}
	if reliance.ID == "" {
		t.Fatal("semantic corpus lacks reliance-bearing materialization case")
	}
	var noteCall hReasonSemanticEvent
	var boundary hReasonSemanticEvent
	for _, event := range reliance.Events {
		if event.Kind == "tool_call" && event.Tool == "haft_note" {
			noteCall = event
		}
		if event.Kind == "support_set_boundary" {
			boundary = event
		}
	}
	if len(noteCall.Payload) == 0 || boundary.ObservationStatus != "materialized_via_non_binding_note" {
		t.Fatalf("reliance case does not declare one executable support-note materialization")
	}
	var args map[string]any
	if err := json.Unmarshal(noteCall.Payload, &args); err != nil {
		t.Fatalf("decode reliance note call: %v", err)
	}
	store := setupCLIArtifactStore(t)
	haftDir := t.TempDir()
	result, ref, err := dispatchToolWithCodeIntelAndIdentifierResolver(
		context.Background(), store, nil, haftDir, "haft_note", args, nil, nil,
	)
	if err != nil {
		t.Fatalf("execute reliance note call: %v", err)
	}
	if strings.TrimSpace(ref) == "" || strings.TrimSpace(result) == "" {
		t.Fatalf("reliance note call returned no addressable result: ref=%q result=%q", ref, result)
	}
	all, err := store.ListByKind(context.Background(), "", 0)
	if err != nil {
		t.Fatalf("list materialized support carriers: %v", err)
	}
	if len(all) != 1 || all[0].Meta.Kind != artifact.KindNote || all[0].Meta.ID != ref {
		t.Fatalf("materialized carriers = %#v, want one exact non-binding note %q", all, ref)
	}
	for key, value := range boundary.TraceFields {
		if !strings.Contains(all[0].Body, key+"="+value) {
			t.Errorf("materialized support note lacks exact trace field %s=%s", key, value)
		}
	}
	files, err := filepath.Glob(filepath.Join(haftDir, artifact.KindNote.Dir(), "*.md"))
	if err != nil {
		t.Fatalf("glob materialized support note: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("materialized support-note files = %v, want exactly one", files)
	}
}

func assertHReasonSemanticDigestTokens(t *testing.T, data []byte) {
	t.Helper()
	for _, match := range hReasonSemanticSHA256Token.FindAllSubmatch(data, -1) {
		value := string(match[1])
		if !hReasonSemanticSHA256Value.MatchString(value) {
			t.Errorf("semantic corpus contains malformed sha256 token %q", "sha256:"+value)
		}
	}
}

func assertHReasonSemanticSourceBasis(t *testing.T, root string, data []byte, corpus hReasonSemanticCorpus) {
	t.Helper()
	lockBytes, err := os.ReadFile(filepath.Join(root, "data", "haft", "fpf-integration.lock.json"))
	if err != nil {
		t.Fatalf("read integration lock: %v", err)
	}
	var lock struct {
		Coordinates struct {
			SourceRevision     string `json:"source_revision"`
			SpecDocumentDigest string `json:"spec_document_digest"`
		} `json:"coordinates"`
	}
	if err := json.Unmarshal(lockBytes, &lock); err != nil {
		t.Fatalf("decode integration lock: %v", err)
	}
	if corpus.SourceBasis.FPFRevision != lock.Coordinates.SourceRevision {
		t.Errorf("corpus FPF revision %q != integration lock %q", corpus.SourceBasis.FPFRevision, lock.Coordinates.SourceRevision)
	}
	if corpus.SourceBasis.FPFSpecDigest != lock.Coordinates.SpecDocumentDigest {
		t.Errorf("corpus FPF digest %q != integration lock %q", corpus.SourceBasis.FPFSpecDigest, lock.Coordinates.SpecDocumentDigest)
	}
	lockSum := sha256.Sum256(lockBytes)
	lockDigest := "sha256:" + hex.EncodeToString(lockSum[:])
	if !strings.Contains(string(data), lockDigest) {
		t.Errorf("semantic corpus does not pin current integration-lock bytes %q", lockDigest)
	}
}

func assertHReasonSemanticCarrierBasis(t *testing.T, root string, carriers []hReasonSemanticCarrier) {
	t.Helper()
	wantRoles := map[string]bool{"canonical": false, "pi_skill": false, "pi_prompt": false}
	for _, carrier := range carriers {
		if _, ok := wantRoles[carrier.Role]; !ok {
			t.Errorf("unexpected carrier role %q", carrier.Role)
			continue
		}
		if wantRoles[carrier.Role] {
			t.Errorf("duplicate carrier role %q", carrier.Role)
		}
		wantRoles[carrier.Role] = true
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(carrier.Path)))
		if err != nil {
			t.Errorf("read carrier %s: %v", carrier.Path, err)
			continue
		}
		sum := sha256.Sum256(data)
		got := "sha256:" + hex.EncodeToString(sum[:])
		if carrier.Digest != got {
			t.Errorf("carrier %s digest %q != current %q", carrier.Path, carrier.Digest, got)
		}
	}
	for role, found := range wantRoles {
		if !found {
			t.Errorf("carrier_basis missing role %q", role)
		}
	}
}

func assertHReasonSemanticScenario(t *testing.T, scenario hReasonSemanticScenario) {
	t.Helper()
	if scenario.ID == "" || scenario.Requirement == "" || len(scenario.Input) == 0 || len(scenario.Events) == 0 {
		t.Fatalf("scenario identity/input/events incomplete: %#v", scenario)
	}
	if scenario.RelianceProfile != "ordinaryBounded" && scenario.RelianceProfile != "relianceBearing" {
		t.Fatalf("invalid reliance_profile %q", scenario.RelianceProfile)
	}
	toolCalls := 0
	fpfCalls := 0
	inspects := make([]string, 0)
	toolResponses := 0
	outstandingCalls := 0
	mutationLedgers := 0
	finalClaims := 0
	observedRecommendation := "absent"
	observedCoordination := "none"
	observedClosure := "not_applicable"
	observedPlanningLabel := "none"
	judgements := make(map[string]string)
	lastJudgement := -1
	recommendationIndex := -1
	for eventIndex, event := range scenario.Events {
		switch event.Kind {
		case "tool_call":
			toolCalls++
			outstandingCalls++
			var payload map[string]any
			if err := json.Unmarshal(event.Payload, &payload); err != nil {
				t.Fatalf("tool_call payload: %v", err)
			}
			if payload["action"] == "fpf" {
				fpfCalls++
				if payload["mode"] == "inspect" {
					identifier, _ := payload["identifier"].(string)
					inspects = append(inspects, identifier)
				}
			}
		case "candidate_judgement":
			aspectResults := make([]string, 0, len(hReasonFitAspectNames))
			for _, aspect := range hReasonFitAspectNames {
				finding, present := event.Fit[aspect]
				if !present || strings.TrimSpace(finding.Rationale) == "" {
					t.Errorf("candidate %q missing fit aspect %q", event.Candidate, aspect)
				}
				if !slices.Contains([]string{"fit", "misfit", "insufficientBasis"}, finding.Result) {
					t.Errorf("candidate %q fit aspect %q has invalid result %q", event.Candidate, aspect, finding.Result)
				}
				aspectResults = append(aspectResults, finding.Result)
			}
			if !slices.Contains([]string{"applicable", "inapplicable", "insufficientBasis"}, event.Aggregate) {
				t.Errorf("candidate %q has invalid aggregate %q", event.Candidate, event.Aggregate)
			}
			wantAggregate := "applicable"
			if slices.Contains(aspectResults, "misfit") {
				wantAggregate = "inapplicable"
			} else if slices.Contains(aspectResults, "insufficientBasis") {
				wantAggregate = "insufficientBasis"
			}
			if event.Aggregate != wantAggregate {
				t.Errorf("candidate %q aggregate %q disagrees with aspect results %v; want %q", event.Candidate, event.Aggregate, aspectResults, wantAggregate)
			}
			judgements[event.Candidate] = event.Aggregate
			lastJudgement = eventIndex
		case "recommendation":
			observedRecommendation = "present"
			recommendationIndex = eventIndex
			if !strings.HasPrefix(event.SelectionBasis, "expected_first_result") {
				t.Errorf("recommendation selected by %q, not expected first result", event.SelectionBasis)
			}
			if judgements[event.Candidate] != "applicable" {
				t.Errorf("recommended candidate %q aggregate = %q, want applicable", event.Candidate, judgements[event.Candidate])
			}
		case "coordination":
			observedCoordination = event.Coordination
			if !slices.Contains([]string{"unordered", "partialOrder", "totalOrder"}, event.Coordination) {
				t.Errorf("invalid coordination %q", event.Coordination)
			}
			if event.Coordination != "unordered" && event.Basis == "" {
				t.Error("ordered coordination lacks explicit basis")
			}
			if event.Coordination == "unordered" && (event.From != "" || event.To != "") {
				t.Error("unordered coordination carries a pairwise relation")
			}
			if strings.TrimSpace(event.CoordinationQuestion) == "" {
				t.Error("coordination lacks one bounded coordination question")
			}
			if len(event.CandidateUses) < 2 {
				t.Error("coordination needs at least two distinct candidate uses")
			} else {
				seen := make(map[string]struct{}, len(event.CandidateUses))
				for _, use := range event.CandidateUses {
					if strings.TrimSpace(use) == "" {
						t.Error("coordination contains an empty candidate use")
					}
					if _, duplicate := seen[use]; duplicate {
						t.Errorf("coordination repeats candidate use %q", use)
					}
					seen[use] = struct{}{}
				}
			}
		case "closure":
			observedClosure = event.Disposition
			if !slices.Contains([]string{"newlyCurrentSubjectResult", "preExistingWithGrounding", "expectedSubjectResultAbsent"}, event.Disposition) {
				t.Errorf("invalid closure disposition %q", event.Disposition)
			}
			if event.Disposition == "expectedSubjectResultAbsent" && (event.InterimResult == "" || event.ReturnCondition == "") {
				t.Error("absent result needs interim result and return condition")
			}
			if event.Disposition != "expectedSubjectResultAbsent" && event.DirectBasis == "" {
				t.Errorf("closure disposition %q lacks a direct basis", event.Disposition)
			}
			if event.Disposition != "expectedSubjectResultAbsent" &&
				(event.ActualResultAssertion == "" || event.RelativeObjectRef == "" || event.BasisKind == "") {
				t.Errorf("closure disposition %q lacks exact actual result, relative object, or category-correct basis", event.Disposition)
			}
			if event.BasisKind == "localRelationBearingClaim" {
				assertHReasonLocalClaimClosure(t, scenario, event)
			}
			if event.Disposition == "preExistingWithGrounding" {
				assertHReasonPreExistingGroundingClosure(t, scenario, event)
			}
		case "mutation_ledger":
			mutationLedgers++
			if event.Writes == nil {
				t.Error("mutation ledger lacks writes count")
			} else if *event.Writes != scenario.Expected.Writes {
				t.Errorf("mutation writes %d != expected %d", *event.Writes, scenario.Expected.Writes)
			}
			if !slices.Equal(event.Effects, scenario.Expected.Effects) {
				t.Errorf("mutation effects %v != expected %v", event.Effects, scenario.Expected.Effects)
			}
		case "final_claim":
			finalClaims++
			if finalClaims > 1 {
				t.Error("scenario emits several final claims")
			}
			if len(event.ClaimClasses) == 0 || strings.TrimSpace(event.SelectionBasis) == "" {
				t.Error("final claim lacks claim classes or selection basis")
			}
			seenClasses := make(map[string]struct{}, len(event.ClaimClasses))
			for _, class := range event.ClaimClasses {
				if strings.TrimSpace(class) == "" {
					t.Error("final claim contains an empty claim class")
				}
				if _, duplicate := seenClasses[class]; duplicate {
					t.Errorf("final claim repeats claim class %q", class)
				}
				seenClasses[class] = struct{}{}
			}
		case "tool_response":
			toolResponses++
			outstandingCalls--
			if strings.TrimSpace(event.Result) == "" {
				t.Error("tool response lacks an observable result")
			}
			if outstandingCalls < 0 {
				t.Error("tool response appears without a preceding call")
			}
		case "planning_result":
			observedPlanningLabel = event.PlanningLabel
			if event.PlanningLabel == "U.WorkPlan" {
				for _, required := range []string{"claimGraph", "entityOfConcern", "referenceScheme", "horizon", "planItem", "intendedPerformance", "method", "window", "performerRole"} {
					if event.MembershipFields[required] == "" {
						t.Errorf("U.WorkPlan claim lacks membership field %q", required)
					}
				}
				coordinationBasis := false
				for _, field := range []string{"constraints", "resources", "dependencies", "commitments", "targets", "baseline"} {
					coordinationBasis = coordinationBasis || strings.TrimSpace(event.MembershipFields[field]) != ""
				}
				if !coordinationBasis {
					t.Error("U.WorkPlan claim lacks constraints, resources, dependencies, commitments, targets, or baseline for one coordination decision")
				}
				planItem := event.MembershipFields["planItem"]
				intendedPerformance := event.MembershipFields["intendedPerformance"]
				if len(event.ClaimGraph) < 2 ||
					!semanticClaimGraphContains(event.ClaimGraph, "has PlanItem "+planItem) ||
					!semanticClaimGraphContains(event.ClaimGraph, "PlanItem "+planItem+" designates intended performance "+intendedPerformance) ||
					!semanticClaimGraphContains(event.ClaimGraph, "concerns present entity "+event.MembershipFields["entityOfConcern"]) ||
					!semanticClaimGraphContains(event.ClaimGraph, "method "+event.MembershipFields["method"]) ||
					!semanticClaimGraphContains(event.ClaimGraph, "window "+event.MembershipFields["window"]) ||
					!semanticClaimGraphContains(event.ClaimGraph, "performer role "+event.MembershipFields["performerRole"]) ||
					!semanticClaimGraphContains(event.ClaimGraph, "target "+event.MembershipFields["targets"]) {
					t.Errorf("U.WorkPlan lacks an exact ClaimGraph identifying the plan episteme and declaring its PlanItem content: %v", event.ClaimGraph)
				}
				for _, claim := range event.ClaimGraph {
					if strings.Contains(claim, planItem+" MemberOf "+event.PlanningLabel) {
						t.Errorf("U.WorkPlan ClaimGraph incorrectly classifies declaration-local PlanItem as the WorkPlan: %q", claim)
					}
				}
			}
		case "pua_inspection":
			for _, required := range []string{"problemFrame", "problem", "forces", "solution", "consequences", "ordinaryBoundary", "strongerNeighbor", "workedSlices", "checklist"} {
				if !slices.Contains(event.Positions, required) {
					t.Errorf("PUA inspection for %q missing %q", event.Candidate, required)
				}
			}
		case "a10_result":
			assertHReasonSemanticA10Path(t, scenario, event.A10Path)
		case "boundary_analysis":
			if event.BoundaryKind == "" || event.UsesLADE == nil || event.AuthorityOwner == "" {
				t.Error("boundary analysis lacks kind, LADE posture, or direct authority owner")
			}
		case "result_expectation":
			if event.Candidate == "" || event.ExpectationRef == "" || event.ExpectedResultKind == "" ||
				event.PatternLocator == "" || event.RelativeObjectRef == "" {
				t.Error("result expectation lacks exact reference, candidate, result kind, relative object, or pattern locator")
			}
		case "return":
			if event.Candidate == "" || event.To == "" || event.Basis == "" {
				t.Error("return lacks current candidate, target, or basis")
			}
		case "recovery_call", "support_set_boundary":
			// Validated by cross-scenario invariants below.
		default:
			t.Errorf("unknown semantic event kind %q", event.Kind)
		}
	}
	if toolCalls < fpfCalls {
		t.Fatalf("impossible call count: tool=%d fpf=%d", toolCalls, fpfCalls)
	}
	if toolResponses != toolCalls {
		t.Errorf("tool responses %d != tool calls %d", toolResponses, toolCalls)
	}
	if outstandingCalls != 0 {
		t.Errorf("unclosed tool calls = %d", outstandingCalls)
	}
	if recommendationIndex >= 0 && recommendationIndex <= lastJudgement {
		t.Errorf("recommendation at event %d must follow every candidate aggregate; last judgement=%d", recommendationIndex, lastJudgement)
	}
	if fpfCalls != scenario.Expected.FPFCalls {
		t.Errorf("FPF calls %d != expected %d", fpfCalls, scenario.Expected.FPFCalls)
	}
	for _, required := range scenario.Expected.RequiredInspects {
		if !slices.Contains(inspects, required) {
			t.Errorf("missing required exact inspect %q in %v", required, inspects)
		}
	}
	if mutationLedgers != 1 {
		t.Errorf("mutation ledger count = %d, want 1", mutationLedgers)
	}
	if finalClaims != 1 {
		t.Errorf("final claim count = %d, want 1", finalClaims)
	}
	if observedRecommendation != scenario.Expected.Recommendation {
		t.Errorf("recommendation %q != expected %q", observedRecommendation, scenario.Expected.Recommendation)
	}
	if observedCoordination != scenario.Expected.Coordination {
		t.Errorf("coordination %q != expected %q", observedCoordination, scenario.Expected.Coordination)
	}
	if observedClosure != scenario.Expected.Closure {
		t.Errorf("closure %q != expected %q", observedClosure, scenario.Expected.Closure)
	}
	if observedPlanningLabel != scenario.Expected.PlanningLabel {
		t.Errorf("planning label %q != expected %q", observedPlanningLabel, scenario.Expected.PlanningLabel)
	}
}

func assertHReasonCrossScenarioInvariants(t *testing.T, scenarios map[string]hReasonSemanticScenario) {
	t.Helper()
	mechanical := scenarios["mechanical_exact_lookup"]
	if mechanical.Expected.FPFCalls != 0 || mechanical.Expected.Writes != 0 {
		t.Error("mechanical control must call no FPF surface and write nothing")
	}
	if semanticEventKinds(mechanical.Events)["tool_call"] != 0 ||
		!semanticScenarioHasFinalClaim(mechanical, "exact_project_lookup", "caller_abstention") {
		t.Error("mechanical control must abstain at the caller without fabricating a query result")
	}
	single := scenarios["single_candidate_pua_new_result"]
	if !semanticScenarioHasEvent(single, "closure", func(event hReasonSemanticEvent) bool {
		return event.Disposition == "newlyCurrentSubjectResult" &&
			event.DirectBasis != "" && event.ReturnCondition != ""
	}) {
		t.Error("single-candidate PUA use lacks honest new-result closure and return")
	}
	multi := scenarios["multi_candidate_pur_applicable"]
	events := semanticEventKinds(multi.Events)
	if events["candidate_judgement"] != 2 || events["recommendation"] != 1 {
		t.Errorf("multi-candidate scenario needs two judgements then one recommendation: %v", events)
	}
	firstJudgement := semanticEventIndex(multi.Events, "candidate_judgement")
	recommendation := semanticEventIndex(multi.Events, "recommendation")
	if firstJudgement < 0 || recommendation <= firstJudgement {
		t.Error("recommendation precedes candidate aggregate")
	}
	for _, event := range multi.Events {
		if event.SelectionBasis == "rank" || event.SelectionBasis == "display_order" {
			t.Error("multi-candidate scenario selects by retrieval order")
		}
	}
	if semanticScenarioHasEvent(scenarios["solution_conditions_misfit"], "recommendation", nil) {
		t.Error("solution-condition misfit scenario recommends a candidate")
	}
	if semanticScenarioHasEvent(scenarios["missing_fit_basis"], "recommendation", nil) {
		t.Error("missing-basis scenario recommends a candidate")
	}
	unordered := scenarios["complementary_candidates_unordered"]
	if !semanticScenarioHasEvent(unordered, "coordination", func(event hReasonSemanticEvent) bool {
		return event.Coordination == "unordered" && event.From == "" && event.To == ""
	}) {
		t.Error("complementary scenario does not preserve unordered coordination")
	}
	assertHReasonP14InputBasis(t, unordered)
	prerequisite := scenarios["prerequisite_result_partial_order"]
	var expectation hReasonSemanticEvent
	var prerequisiteClosure hReasonSemanticEvent
	var ordering hReasonSemanticEvent
	for _, event := range prerequisite.Events {
		switch event.Kind {
		case "result_expectation":
			expectation = event
		case "closure":
			prerequisiteClosure = event
		case "coordination":
			ordering = event
		}
	}
	if expectation.ExpectationRef == "" || expectation.Candidate == "" ||
		ordering.Basis != "prerequisiteResult" ||
		ordering.From != expectation.Candidate || ordering.To == "" ||
		prerequisiteClosure.ExpectationRef != expectation.ExpectationRef ||
		prerequisiteClosure.Candidate != expectation.Candidate ||
		prerequisiteClosure.ActualResultAssertion == "" ||
		prerequisiteClosure.RelativeObjectRef != expectation.RelativeObjectRef ||
		prerequisiteClosure.BasisKind != "localRelationBearingClaim" ||
		prerequisiteClosure.DirectBasis == "" {
		t.Error("prerequisiteResult ordering is not tied to one exact expectation and current PUA closure")
	}
	assertHReasonClosureBasisSupplied(t, prerequisite, prerequisiteClosure)
	if !semanticScenarioHasEvent(scenarios["stronger_neighbor_return"], "return", func(event hReasonSemanticEvent) bool {
		return event.Candidate != "" && event.To != "" && event.Basis == "stronger_neighbor_better_first_result"
	}) {
		t.Error("stronger-neighbor scenario lacks a named return")
	}
	for _, id := range []string{
		"multi_candidate_pur_applicable",
		"prerequisite_result_partial_order",
		"stronger_neighbor_return",
	} {
		input, err := json.Marshal(scenarios[id].Input)
		if err != nil {
			t.Errorf("marshal %q input: %v", id, err)
			continue
		}
		if !strings.Contains(string(input), "assurance-claim:") {
			t.Errorf("scenario %q reaches B.3 without an actual named assurance claim", id)
		}
	}
	for id, disposition := range map[string]string{
		"single_candidate_pua_new_result": "newlyCurrentSubjectResult",
		"pua_preexisting_with_grounding":  "preExistingWithGrounding",
		"pua_expected_result_absent":      "expectedSubjectResultAbsent",
	} {
		if scenarios[id].Expected.Closure != disposition {
			t.Errorf("PUA closure fixture %q = %q, want %q", id, scenarios[id].Expected.Closure, disposition)
		}
	}
	for _, id := range []string{"single_candidate_pua_new_result", "pua_preexisting_with_grounding"} {
		for _, event := range scenarios[id].Events {
			if event.Kind == "closure" {
				assertHReasonClosureBasisSupplied(t, scenarios[id], event)
			}
		}
	}
	for _, id := range []string{
		"mechanical_exact_lookup",
		"single_candidate_pua_new_result",
		"multi_candidate_pur_applicable",
		"recommendation_no_authority_effects",
	} {
		scenario := scenarios[id]
		if scenario.Expected.Writes != 0 || len(scenario.Expected.Effects) != 0 {
			t.Errorf("P14 core scenario %q has unauthorized effect expectation: writes=%d effects=%v", id, scenario.Expected.Writes, scenario.Expected.Effects)
		}
		assertHReasonP14InputBasis(t, scenario)
	}
	reliance := scenarios["reliance_bearing_minimal_support"]
	currentFacts, factsPresent := reliance.Input["current_facts"].([]any)
	if !factsPresent || len(currentFacts) < 2 {
		t.Error("reliance support fixture lacks supplied observed result and direct-basis facts")
	}
	for _, event := range reliance.Events {
		if event.Kind != "support_set_boundary" {
			continue
		}
		if event.ObservationStatus != "materialized_via_non_binding_note" {
			t.Errorf("reliance support boundary observation_status = %q", event.ObservationStatus)
		}
		if !slices.Equal(event.SupportRecords, []string{"Haft.Note"}) {
			t.Errorf("reliance support records = %v, want one Haft.Note carrier", event.SupportRecords)
		}
		for _, forbidden := range []string{"ProblemCard", "SolutionPortfolio", "DecisionRecord", "WorkPlan", "WorkCommission"} {
			if slices.Contains(event.SupportRecords, forbidden) {
				t.Errorf("reliance support set contains automatic %s", forbidden)
			}
		}
		for _, required := range []string{"entityOfConcernRef", "referenceSchemeRef", "practicalQuestion", "sourceEdition", "solutionLocator", "expectedResultKind", "expectedResultPatternLocator", "actualResultAssertion", "relativeObjectRef", "directBasis", "localClaimDerivationBasis", "localClaimAssertion", "closureDisposition", "receivingUse", "stopReturn"} {
			if event.TraceFields[required] == "" {
				t.Errorf("reliance trace missing %q", required)
			}
		}
		if got := event.TraceFields["expectedResultKind"]; got != "partialGapExposingA10CarrierAccountWithNarrowedRelianceDisposition" {
			t.Errorf("reliance trace expectedResultKind = %q, want the honest partial gap-exposing profile", got)
		}
		var closure hReasonSemanticEvent
		for _, candidateEvent := range reliance.Events {
			if candidateEvent.Kind == "closure" {
				closure = candidateEvent
				break
			}
		}
		if event.TraceFields["actualResultAssertion"] != closure.ActualResultAssertion ||
			event.TraceFields["relativeObjectRef"] != closure.RelativeObjectRef ||
			event.TraceFields["directBasis"] != closure.DirectBasis ||
			event.TraceFields["closureDisposition"] != closure.Disposition ||
			event.TraceFields["stopReturn"] != closure.ReturnCondition {
			t.Error("reliance support trace differs from the derived A.10 result closure")
		}
		derivationJSON, err := json.Marshal(closure.LocalClaimDerivation)
		if err != nil {
			t.Fatalf("marshal reliance local-claim derivation: %v", err)
		}
		assertionJSON, err := json.Marshal(closure.LocalClaimAssertion)
		if err != nil {
			t.Fatalf("marshal reliance local-claim assertion: %v", err)
		}
		if event.TraceFields["localClaimDerivationBasis"] != string(derivationJSON) ||
			event.TraceFields["localClaimAssertion"] != string(assertionJSON) {
			t.Error("reliance support trace does not preserve the exact A.6.RCD derivation and local C.2.1 claim")
		}
	}
	if reliance.Expected.Writes != 1 || !slices.Equal(reliance.Expected.Effects, []string{"non_binding_support_note"}) ||
		!semanticScenarioHasFinalClaim(reliance, "addressable_support_trace", "named_receiving_use") {
		t.Errorf("reliance support fixture does not bind its one observed non-binding carrier: writes=%d effects=%v", reliance.Expected.Writes, reliance.Expected.Effects)
	}
	recommendationScenario := scenarios["recommendation_no_authority_effects"]
	if !strings.Contains(fmt.Sprint(recommendationScenario.Input["basis"]), "closed live-alternative set") {
		t.Error("recommendation fixture does not supply a closed live-alternative comparison basis")
	}
	recovery := scenarios["exact_identifier_recovery_executes"]
	var emitted, executed json.RawMessage
	for _, event := range recovery.Events {
		switch event.Kind {
		case "recovery_call":
			emitted = event.Payload
		case "tool_call":
			if event.CallID == "memory-resolve-1" {
				executed = event.Payload
			}
		}
	}
	if string(emitted) != string(executed) {
		t.Errorf("recovery replay changed payload:\nemitted=%s\nexecuted=%s", emitted, executed)
	}
	russian := scenarios["russian_concern_preserved"]
	original, _ := russian.Input["query"].(string)
	var payload map[string]any
	if err := json.Unmarshal(russian.Events[0].Payload, &payload); err != nil {
		t.Fatalf("decode Russian query event: %v", err)
	}
	if payload["query"] != original {
		t.Errorf("original non-English query changed: input=%q payload=%q", original, payload["query"])
	}
	if payload["entity_of_concern"] == "" || payload["known_context"] == nil || payload["intended_use"] == "" {
		t.Error("non-English query lacks explicit English/FPF context")
	}
	if scenarios["planning_draft_without_membership"].Expected.PlanningLabel != "planning draft" ||
		scenarios["workplan_positive_membership"].Expected.PlanningLabel != "U.WorkPlan" {
		t.Error("planning fixtures do not distinguish cue from WorkPlan membership")
	}
	mixed := scenarios["a6_mixed_boundary_package"]
	if !semanticScenarioHasEvent(mixed, "boundary_analysis", func(event hReasonSemanticEvent) bool {
		return event.BoundaryKind == "mixed_normativity" && event.UsesLADE != nil && *event.UsesLADE && event.AuthorityOwner == "direct_claim_contracts"
	}) {
		t.Error("mixed boundary fixture does not use LADE under direct claim contracts")
	}
	ordinaryBrief := scenarios["ordinary_human_brief_not_a6_authority"]
	if ordinaryBrief.Expected.Writes != 0 || !semanticScenarioHasEvent(ordinaryBrief, "boundary_analysis", func(event hReasonSemanticEvent) bool {
		return event.BoundaryKind == "haft_local_human_gate_brief" && event.UsesLADE != nil && !*event.UsesLADE && event.AuthorityOwner == "spec_lifecycle_contract"
	}) {
		t.Error("ordinary Human Gate Brief must be explanatory only")
	}
	profilePrepare := scenarios["profile_change_prepare_no_apply"]
	if len(profilePrepare.Expected.Effects) != 1 || profilePrepare.Expected.Effects[0] != "non_binding_review_carrier" ||
		profilePrepare.Expected.Writes != 1 {
		t.Error("profile_change_prepare fixture crosses apply/authority boundary")
	}
}

func assertHReasonSemanticA10Path(
	t *testing.T,
	scenario hReasonSemanticScenario,
	path *hReasonSemanticA10Path,
) {
	t.Helper()
	if path == nil {
		t.Error("A.10 result lacks its descriptive evidence-provenance path")
		return
	}
	if path.PathProfile != "a10_partial_gap_exposing_reversible" {
		t.Errorf("A.10 path profile = %q, want an honest partial gap-exposing account", path.PathProfile)
	}
	for label, value := range map[string]string{
		"path ref":                  path.PathRef,
		"relied-on claim":           path.ReliedOnClaimRef,
		"claim graph":               path.ClaimGraph,
		"EntityOfConcern":           path.EntityOfConcernRef,
		"reference scheme":          path.ReferenceSchemeRef,
		"local result":              path.LocalResult,
		"direct result-rule owner":  path.ResultRuleOwner,
		"result rule":               path.ResultRule,
		"attempted use":             path.AttemptedUseRef,
		"narrowed reversible use":   path.NarrowedReversibleUse,
		"time window":               path.TimeWindow,
		"rival explanation":         path.RivalExplanation,
		"unsupported attempted use": path.UnsupportedAttemptedUse,
		"reopen trigger":            path.ReopenTrigger,
	} {
		if strings.TrimSpace(value) == "" {
			t.Errorf("A.10 path lacks %s", label)
		}
	}
	if strings.Contains(path.ResultRuleOwner, "A.10") ||
		strings.Contains(path.ResultRuleOwner, "G.11") {
		t.Errorf("A.10/G.11 is cited as the owner of the local result: %q", path.ResultRuleOwner)
	}
	if len(path.ObservedCarrierFacts) < 2 {
		t.Errorf("A.10 partial account lacks the two supplied carrier observations: %#v", path.ObservedCarrierFacts)
	}
	if len(path.EstablishedDirectRelationRefs) != 0 {
		t.Errorf("A.10 partial account promotes unproved path edges: %#v", path.EstablishedDirectRelationRefs)
	}
	for label, value := range map[string]string{
		"affected-party view":         path.Contest.AffectedPartyView,
		"accountable review route":    path.Contest.AccountableReviewRoute,
		"allowed challenge evidence":  path.Contest.AllowedChallengeEvidence,
		"possible disposition change": path.Contest.PossibleDispositionChange,
		"contest outcome condition":   path.Contest.OutcomeRecordCondition,
		"contest reopen trigger":      path.Contest.ReopenTrigger,
	} {
		if strings.TrimSpace(value) == "" {
			t.Errorf("A.10 path lacks %s", label)
		}
	}
	if path.Contest.ReopenTrigger != path.ReopenTrigger {
		t.Error("A.10 contest and path reopen triggers differ")
	}
	if len(path.UnresolvedGaps) == 0 {
		t.Error("partial A.10 account hides its missing direct governors")
	}
	assertHReasonA10MissingGovernors(t, scenario.ID, path.UnresolvedGaps)
	if !slices.Contains(
		[]string{"pass", "degrade", "abstain", "reopen", "evidence-needed", "assurance-needed", "blocked-current-use"},
		path.RelianceDisposition,
	) {
		t.Errorf("A.10 path has invalid RelianceDisposition %q", path.RelianceDisposition)
	}
	if path.RelianceDisposition != "degrade" {
		t.Errorf("partial A.10 carrier account disposition = %q, want degrade for only its named narrower reversible use", path.RelianceDisposition)
	}
	if strings.Contains(strings.ToLower(path.LocalResult), "evidencerelation") ||
		strings.Contains(strings.ToLower(path.LocalResult), " obtains") {
		t.Errorf("A.10 path invents a subject relation instead of describing independently established facts: %q", path.LocalResult)
	}
	assertHReasonC21Constitution(t, scenario.ID+" A.10 path result", path.ResultIdentity, path.ResultConstitution)
	if path.ResultIdentity.EpistemeRef != path.PathRef ||
		path.ResultIdentity.EntityOfConcernRef != path.ReliedOnClaimRef ||
		!strings.Contains(path.ResultIdentity.ClaimGraph, path.PathRef) ||
		!strings.Contains(path.ResultIdentity.ClaimGraph, path.LocalResult) ||
		!strings.Contains(path.ResultIdentity.ClaimGraph, path.AttemptedUseRef) ||
		!strings.Contains(path.ResultIdentity.ClaimGraph, "RelianceDisposition="+path.RelianceDisposition) {
		t.Errorf("A.10 path result is not independently identified by its derived content and exact relied-on claim: %#v", path.ResultIdentity)
	}

	raw, present := scenario.Input["a10_source_basis"]
	if !present {
		if basis, ok := scenario.Input["basis"].(map[string]any); ok {
			raw, present = basis["a10_source_basis"]
		}
	}
	if !present {
		t.Error("A.10 result has no independently supplied source/use basis")
		return
	}
	data, err := json.Marshal(raw)
	if err != nil {
		t.Errorf("marshal supplied A.10 path basis: %v", err)
		return
	}
	var supplied hReasonSemanticA10SourceBasis
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&supplied); err != nil {
		t.Errorf("decode supplied A.10 source/use basis: %v", err)
		return
	}
	observedBasis := hReasonSemanticA10SourceBasis{
		ReliedOnClaimRef:              path.ReliedOnClaimRef,
		ClaimGraph:                    path.ClaimGraph,
		EntityOfConcernRef:            path.EntityOfConcernRef,
		ReferenceSchemeRef:            path.ReferenceSchemeRef,
		LocalResultRuleOwner:          path.ResultRuleOwner,
		LocalResultRule:               path.ResultRule,
		AttemptedUseRef:               path.AttemptedUseRef,
		ObservedCarrierFacts:          path.ObservedCarrierFacts,
		EstablishedDirectRelationRefs: path.EstablishedDirectRelationRefs,
		NarrowedReversibleUse:         path.NarrowedReversibleUse,
		TimeWindow:                    path.TimeWindow,
		RivalExplanation:              path.RivalExplanation,
		UnsupportedAttemptedUse:       path.UnsupportedAttemptedUse,
		Contest:                       path.Contest,
		UnresolvedGaps:                path.UnresolvedGaps,
	}
	if !reflect.DeepEqual(supplied, observedBasis) {
		t.Errorf("A.10 result retrofits or changes its supplied source/use facts:\nsupplied=%#v\nobserved=%#v", supplied, observedBasis)
	}
}

func assertHReasonPreExistingGroundingClosure(
	t *testing.T,
	scenario hReasonSemanticScenario,
	closure hReasonSemanticEvent,
) {
	t.Helper()
	if closure.SubjectIdentity == nil || closure.SubjectGrounding == nil ||
		closure.GroundingFinding == nil || closure.GroundingFindingConstitution == nil ||
		closure.ClosureFinding == nil || closure.ClosureFindingConstitution == nil {
		t.Error("pre-existing closure lacks the subject identity, exact grounding pair, independently identified grounding finding, or separate closure-finding identity")
		return
	}
	decodeInput := func(name string, target any) bool {
		t.Helper()
		raw, present := scenario.Input[name]
		if !present {
			t.Errorf("pre-existing closure input lacks %q", name)
			return false
		}
		data, err := json.Marshal(raw)
		if err != nil {
			t.Errorf("marshal pre-existing input %q: %v", name, err)
			return false
		}
		decoder := json.NewDecoder(strings.NewReader(string(data)))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(target); err != nil {
			t.Errorf("decode pre-existing input %q: %v", name, err)
			return false
		}
		return true
	}
	var suppliedSubject hReasonSemanticC21Identity
	var suppliedGrounding hReasonSemanticGroundingBasisPair
	if !decodeInput("subject_identity", &suppliedSubject) ||
		!decodeInput("subject_grounding", &suppliedGrounding) {
		return
	}
	if !reflect.DeepEqual(suppliedSubject, *closure.SubjectIdentity) ||
		!reflect.DeepEqual(suppliedGrounding, *closure.SubjectGrounding) {
		t.Error("pre-existing closure retrofits or changes the supplied subject identity or grounding occurrence")
	}
	assertHReasonC21Constitution(t, scenario.ID+" subject", *closure.SubjectIdentity, *closure.SubjectGrounding)
	assertHReasonC21Constitution(t, scenario.ID+" grounding finding", *closure.GroundingFinding, *closure.GroundingFindingConstitution)
	assertHReasonC21Constitution(t, scenario.ID+" closure finding", *closure.ClosureFinding, *closure.ClosureFindingConstitution)
	if closure.GroundingFinding.EntityOfConcernRef != closure.SubjectIdentity.EpistemeRef ||
		closure.ClosureFinding.EntityOfConcernRef != closure.GroundingFinding.EpistemeRef ||
		closure.GroundingFinding.EpistemeRef == closure.SubjectIdentity.EpistemeRef ||
		closure.ClosureFinding.EpistemeRef == closure.GroundingFinding.EpistemeRef {
		t.Error("pre-existing subject, grounding finding, and result-closure finding are not three distinct, correctly nested C.2.1 epistemes")
	}
	joined := strings.Join([]string{
		closure.DirectBasis,
		closure.ClosureFinding.ClaimGraph,
		closure.ActualResultAssertion,
	}, "\n")
	if closure.LocalClaimAssertion == nil ||
		!strings.Contains(closure.DirectBasis, closure.LocalClaimAssertion.ClaimIdentity.EpistemeRef) ||
		!strings.Contains(closure.ClosureFinding.ClaimGraph, closure.LocalClaimAssertion.ClaimIdentity.EpistemeRef) ||
		strings.Contains(joined, "PatternUseResultClosureFinding@Context") ||
		strings.Contains(joined, "made result of this pattern use") ||
		strings.Contains(joined, "connects the grounding finding") ||
		strings.Contains(joined, "relates the grounding finding") ||
		strings.Contains(closure.DirectBasis, closure.SubjectGrounding.OccurrenceRef) {
		t.Error("pre-existing grounding-finding closure is circular or fails to report its separate category-correct local-claim basis")
	}
}

func assertHReasonLocalClaimClosure(
	t *testing.T,
	scenario hReasonSemanticScenario,
	closure hReasonSemanticEvent,
) {
	t.Helper()
	if _, stale := scenario.Input["local_claim_derivation_basis"]; stale {
		t.Errorf("scenario %q carries a second authored derivation beside the predeclared local-claim context", scenario.ID)
	}
	if nested, ok := scenario.Input["basis"].(map[string]any); ok {
		if _, stale := nested["local_claim_derivation_basis"]; stale {
			t.Errorf("scenario %q carries a second authored derivation beside the predeclared local-claim context", scenario.ID)
		}
	}
	if closure.LocalClaimDerivation == nil || closure.LocalClaimAssertion == nil {
		t.Errorf("scenario %q local-claim closure lacks structured A.6.RCD derivation or assertion", scenario.ID)
		return
	}
	raw, present := scenario.Input["local_claim_context"]
	if !present {
		if basis, ok := scenario.Input["basis"].(map[string]any); ok {
			raw, present = basis["local_claim_context"]
		}
	}
	if !present {
		t.Errorf("scenario %q local-claim closure lacks its predeclared context", scenario.ID)
		return
	}
	data, err := json.Marshal(raw)
	if err != nil {
		t.Errorf("scenario %q marshal supplied local-claim basis: %v", scenario.ID, err)
		return
	}
	var supplied hReasonSemanticLocalClaimContext
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&supplied); err != nil {
		t.Errorf("scenario %q decode supplied local-claim basis: %v", scenario.ID, err)
		return
	}
	if !reflect.DeepEqual(supplied, closure.LocalClaimDerivation.Context) {
		t.Errorf("scenario %q local-claim derivation changed its predeclared context:\nsupplied=%#v\nclosure=%#v", scenario.ID, supplied, closure.LocalClaimDerivation.Context)
	}
	assertHReasonLocalClaimContext(t, scenario.ID, supplied)
	assertHReasonLocalClaimDerivationBasis(t, scenario, *closure.LocalClaimDerivation)
	assertion := closure.LocalClaimAssertion
	if assertion.Polarity != "affirmative" ||
		strings.TrimSpace(assertion.ClaimIdentity.EpistemeRef) == "" ||
		strings.TrimSpace(assertion.ClaimIdentity.ClaimGraph) == "" ||
		assertion.ClaimIdentity.EntityOfConcernRef != supplied.ResultRef ||
		strings.TrimSpace(assertion.ClaimIdentity.ReferenceSchemeRef) == "" {
		t.Errorf("scenario %q local-claim assertion lacks an affirmative exact C.2.1 identity: %#v", scenario.ID, assertion)
	}
	assertHReasonC21Constitution(t, scenario.ID+" local claim", assertion.ClaimIdentity, assertion.ClaimConstitution)
	claimGraph := assertion.ClaimIdentity.ClaimGraph
	for label, value := range map[string]string{
		"actual result assertion": closure.ActualResultAssertion,
		"result ref":              supplied.ResultRef,
		"relative object":         supplied.RelativeObjectRef,
		"constructor":             supplied.SelectedSubstrate.ConstructorRef,
	} {
		if strings.TrimSpace(value) == "" || !strings.Contains(claimGraph, value) {
			t.Errorf("scenario %q local-claim ClaimGraph omits %s %q", scenario.ID, label, value)
		}
	}
	for _, base := range closure.LocalClaimDerivation.BasePredicates {
		if !strings.Contains(claimGraph, base.Predicate) {
			t.Errorf("scenario %q local-claim ClaimGraph omits base predicate %q", scenario.ID, base.Predicate)
		}
	}
	if closure.RelativeObjectRef != supplied.RelativeObjectRef ||
		closure.ReturnCondition != supplied.StopOrReturn ||
		!strings.Contains(closure.DirectBasis, assertion.ClaimIdentity.EpistemeRef) ||
		!strings.Contains(closure.DirectBasis, "A.6.RCD") {
		t.Errorf("scenario %q local-claim closure does not bind its derived assertion, relative object, and stop/return", scenario.ID)
	}
}

func assertHReasonLocalClaimDerivationBasis(
	t *testing.T,
	scenario hReasonSemanticScenario,
	basis hReasonSemanticLocalClaimDerivationBasis,
) {
	t.Helper()
	scenarioID := scenario.ID
	if basis.DerivationPatternLocator != "A.6.RCD" ||
		basis.Disposition != "localCompoundRelationBearingClaim" {
		t.Errorf("scenario %q local-claim derivation is not A.6.RCD disposition 2: %#v", scenarioID, basis)
	}
	for _, requiredInspect := range []string{"E.11.PUA", "A.6.RCD"} {
		if !semanticScenarioHasEvent(scenario, "tool_call", func(event hReasonSemanticEvent) bool {
			if event.Tool != "haft_query" {
				return false
			}
			var payload struct {
				Action     string `json:"action"`
				Mode       string `json:"mode"`
				Identifier string `json:"identifier"`
			}
			return json.Unmarshal(event.Payload, &payload) == nil &&
				payload.Action == "fpf" && payload.Mode == "inspect" &&
				payload.Identifier == requiredInspect
		}) {
			t.Errorf("scenario %q derives a local PUA closure without exact inspect %q", scenarioID, requiredInspect)
		}
	}
	assertHReasonC21Constitution(t, scenarioID+" derived result", basis.ResultIdentity, basis.ResultConstitution)
	if basis.ResultIdentity.EpistemeRef != basis.Context.ResultRef {
		t.Errorf("scenario %q derived result identity %q differs from predeclared ref %q", scenarioID, basis.ResultIdentity.EpistemeRef, basis.Context.ResultRef)
	}
	var sourceResultIdentity *hReasonSemanticC21Identity
	var sourceResultConstitution *hReasonSemanticGroundingBasisPair
	for index := range scenario.Events {
		event := &scenario.Events[index]
		if event.Kind == "a10_result" && event.A10Path != nil {
			sourceResultIdentity = &event.A10Path.ResultIdentity
			sourceResultConstitution = &event.A10Path.ResultConstitution
		}
	}
	if scenario.ID == "pua_preexisting_with_grounding" {
		for index := range scenario.Events {
			event := &scenario.Events[index]
			if event.Kind == "closure" && event.GroundingFinding != nil && event.GroundingFindingConstitution != nil {
				sourceResultIdentity = event.GroundingFinding
				sourceResultConstitution = event.GroundingFindingConstitution
			}
		}
	}
	if sourceResultIdentity == nil || sourceResultConstitution == nil ||
		!reflect.DeepEqual(*sourceResultIdentity, basis.ResultIdentity) ||
		!reflect.DeepEqual(*sourceResultConstitution, basis.ResultConstitution) {
		t.Errorf("scenario %q local-claim result basis is not the independently observed result identity", scenarioID)
	}
	if len(basis.Participants) < 2 || len(basis.BasePredicates) < 2 ||
		len(basis.CaseFacts) == 0 || len(basis.SupportOrWarrant) == 0 {
		t.Errorf("scenario %q local-claim basis lacks participants, governed bases, case facts, or support/warrant", scenarioID)
	}
	participantRefs := make(map[string]struct{}, len(basis.Participants))
	for _, participant := range basis.Participants {
		if strings.TrimSpace(participant.Meaning) == "" || strings.TrimSpace(participant.Ref) == "" {
			t.Errorf("scenario %q local-claim basis contains an incomplete participant: %#v", scenarioID, participant)
		}
		if _, duplicate := participantRefs[participant.Ref]; duplicate {
			t.Errorf("scenario %q local-claim basis repeats participant %q", scenarioID, participant.Ref)
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
			len(base.Participants) == 0 || len(base.CaseFacts) == 0 {
			t.Errorf("scenario %q local-claim base predicate is incomplete: %#v", scenarioID, base)
		}
		if !strings.HasPrefix(base.ClaimGraphLocator, base.PatternLocator+":") {
			t.Errorf("scenario %q base predicate %q has non-exact ClaimGraph locator %q for %q", scenarioID, base.Predicate, base.ClaimGraphLocator, base.PatternLocator)
		}
		for _, participant := range base.Participants {
			if _, present := participantRefs[participant]; !present {
				t.Errorf("scenario %q base predicate %q uses undeclared participant %q", scenarioID, base.Predicate, participant)
			}
		}
		if base.Predicate == "EpistemeConstitutionRelation" &&
			slices.Equal(base.Participants, []string{
				basis.ResultIdentity.ClaimGraph,
				basis.ResultIdentity.EntityOfConcernRef,
				basis.ResultIdentity.ReferenceSchemeRef,
			}) {
			resultConstitutionBound = true
		}
	}
	if !resultConstitutionBound {
		t.Errorf("scenario %q local-claim basis omits the exact independently identified result constitution", scenarioID)
	}
	wantLocators := []string{
		"C.2.1:4.2.1-4.2.3",
		"A.10:4.4",
		"A.10:4.6b",
		"E.11.PUA:4.3",
	}
	if scenario.ID == "pua_preexisting_with_grounding" {
		wantLocators = []string{
			"C.2.1:4.2.1-4.2.3",
			"C.2.1:4.2.1-4.2.3",
			"E.11.PUA:4.3",
			"E.11.PUA:4.6",
		}
	}
	slices.Sort(locators)
	slices.Sort(wantLocators)
	if !slices.Equal(locators, wantLocators) {
		t.Errorf("scenario %q local-claim bases have locators %v, want exact %v", scenarioID, locators, wantLocators)
	}
	forbidden := strings.ToLower(fmt.Sprint(basis))
	for _, phrase := range []string{
		"e.11.pua:4.5",
		"patternuseresultclosurefinding@context",
		"made result of this pattern use",
		"connects the grounding finding",
		"relates the grounding finding",
	} {
		if strings.Contains(forbidden, phrase) {
			t.Errorf("scenario %q local-claim basis is circular through %q", scenarioID, phrase)
		}
	}
}

func assertHReasonLocalClaimContext(t *testing.T, scenarioID string, context hReasonSemanticLocalClaimContext) {
	t.Helper()
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
			t.Errorf("scenario %q local-claim context lacks %s", scenarioID, label)
		}
	}
	assertHReasonC21Constitution(t, scenarioID+" relative object", context.RelativeObjectIdentity, context.RelativeObjectConstitution)
	assertHReasonC21Constitution(t, scenarioID+" result expectation", context.ResultExpectationIdentity, context.ResultExpectationConstitution)
	assertHReasonC21Constitution(t, scenarioID+" constructor", context.SelectedSubstrate.DefinitionIdentity, context.SelectedSubstrate.DefinitionConstitution)
	if context.RelativeObjectIdentity.EpistemeRef != context.RelativeObjectRef ||
		context.SelectedSubstrate.DefinitionIdentity.EntityOfConcernRef != context.SelectedSubstrate.ConstructorRef ||
		!strings.Contains(context.SelectedSubstrate.DefinitionIdentity.ClaimGraph, context.SelectedSubstrate.ConstructorRef) ||
		!strings.Contains(context.SelectedSubstrate.ConstructorSemantics, "exact expected-result-content match") {
		t.Errorf("scenario %q local-claim context does not identify its relative object or exact-match constructor", scenarioID)
	}
	if strings.Contains(context.SelectedSubstrate.DefinitionIdentity.ClaimGraph, context.ResultRef) ||
		strings.Contains(context.SelectedSubstrate.DefinitionIdentity.ClaimGraph, context.RelativeObjectRef) {
		t.Errorf("scenario %q fixture result leaked into its reusable constructor definition", scenarioID)
	}
	expectation := strings.ToLower(context.ResultExpectationIdentity.ClaimGraph)
	for _, phrase := range []string{
		strings.ToLower(context.RelativeObjectRef),
		"localrelationbearingclaim",
		"asserts neither result existence nor closure",
	} {
		if !strings.Contains(expectation, phrase) {
			t.Errorf("scenario %q expected-result content omits %q", scenarioID, phrase)
		}
	}
}

func assertHReasonC21Constitution(
	t *testing.T,
	label string,
	identity hReasonSemanticC21Identity,
	constitution hReasonSemanticGroundingBasisPair,
) {
	t.Helper()
	if strings.TrimSpace(identity.EpistemeRef) == "" ||
		strings.TrimSpace(identity.ClaimGraph) == "" ||
		strings.TrimSpace(identity.EntityOfConcernRef) == "" ||
		strings.TrimSpace(identity.ReferenceSchemeRef) == "" {
		t.Errorf("%s lacks an exact C.2.1 identity: %#v", label, identity)
		return
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
		t.Errorf("%s is not an obtaining exact C.2.1 constitution: %#v", label, constitution)
	}
}

func assertHReasonClosureBasisSupplied(
	t *testing.T,
	scenario hReasonSemanticScenario,
	closure hReasonSemanticEvent,
) {
	t.Helper()
	raw, present := scenario.Input["current_facts"]
	if !present {
		if basisRaw, basisPresent := scenario.Input["basis"]; basisPresent {
			raw = basisRaw
			present = true
		}
	}
	if !present {
		t.Errorf("scenario %q closure has no supplied current basis", scenario.ID)
		return
	}
	facts := strings.ToLower(fmt.Sprint(raw))
	if strings.Contains(facts, strings.ToLower(closure.ActualResultAssertion)) {
		t.Errorf("scenario %q supplies its exact closure assertion as an input fact instead of deriving the result", scenario.ID)
	}
	for _, leakedConclusion := range []string{
		"is the category-correct pua closure branch",
		"makes the resulting path this use's result",
		"made result of this pattern use",
		"relates the grounding finding to",
		"connects the grounding finding to",
	} {
		if strings.Contains(facts, leakedConclusion) {
			t.Errorf("scenario %q supplies a paraphrased closure conclusion %q instead of deriving it", scenario.ID, leakedConclusion)
		}
	}
	for label, value := range map[string]string{
		"relative object": closure.RelativeObjectRef,
		"basis kind":      closure.BasisKind,
	} {
		if strings.TrimSpace(value) == "" ||
			!strings.Contains(facts, strings.ToLower(value)) {
			t.Errorf("scenario %q closure %s %q is not supplied by current basis %s", scenario.ID, label, value, facts)
		}
	}
}

func assertHReasonP14InputBasis(t *testing.T, scenario hReasonSemanticScenario) {
	t.Helper()
	raw, present := scenario.Input["basis"]
	if !present {
		t.Errorf("P14 scenario %q has no decision-complete input basis", scenario.ID)
		return
	}
	data, err := json.Marshal(raw)
	if err != nil {
		t.Errorf("marshal P14 scenario %q input basis: %v", scenario.ID, err)
		return
	}
	var basis hReasonSemanticInputBasis
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&basis); err != nil {
		t.Errorf("decode P14 scenario %q input basis: %v", scenario.ID, err)
		return
	}
	if strings.TrimSpace(basis.Subject) == "" ||
		strings.TrimSpace(basis.ReceivingUse) == "" || len(basis.CurrentFacts) == 0 {
		t.Errorf("P14 scenario %q input basis is incomplete: %#v", scenario.ID, basis)
	}
	for index, fact := range basis.CurrentFacts {
		if strings.TrimSpace(fact) == "" {
			t.Errorf("P14 scenario %q current fact %d is empty", scenario.ID, index)
		}
	}
	if scenario.Expected.FPFCalls > 0 && len(basis.Candidates) == 0 {
		t.Errorf("P14 scenario %q calls FPF without a named input candidate", scenario.ID)
	}
	hasA10Candidate := false
	for index, candidate := range basis.Candidates {
		hasA10Candidate = hasA10Candidate || candidate.PatternID == "A.10"
		if strings.TrimSpace(candidate.PatternID) == "" ||
			strings.TrimSpace(candidate.CurrentCondition) == "" ||
			strings.TrimSpace(candidate.ExpectedFirstResult) == "" ||
			strings.TrimSpace(candidate.ReceivingUse) == "" ||
			strings.TrimSpace(candidate.OrdinaryBoundary) == "" {
			t.Errorf("P14 scenario %q candidate %d is incomplete: %#v", scenario.ID, index, candidate)
		}
	}
	if hasA10Candidate {
		if basis.A10SourceBasis == nil {
			t.Errorf("P14 scenario %q asks a host to assess A.10 without a structured source/use basis", scenario.ID)
			return
		}
		assertHReasonA10SourceBasis(t, scenario.ID, *basis.A10SourceBasis)
	}
}

func assertHReasonA10SourceBasis(t *testing.T, scenarioID string, basis hReasonSemanticA10SourceBasis) {
	t.Helper()
	for label, value := range map[string]string{
		"relied-on claim":             basis.ReliedOnClaimRef,
		"claim graph":                 basis.ClaimGraph,
		"EntityOfConcern":             basis.EntityOfConcernRef,
		"reference scheme":            basis.ReferenceSchemeRef,
		"local result-rule owner":     basis.LocalResultRuleOwner,
		"local result rule":           basis.LocalResultRule,
		"attempted use":               basis.AttemptedUseRef,
		"narrowed reversible use":     basis.NarrowedReversibleUse,
		"time window":                 basis.TimeWindow,
		"rival explanation":           basis.RivalExplanation,
		"unsupported attempted use":   basis.UnsupportedAttemptedUse,
		"affected-party view":         basis.Contest.AffectedPartyView,
		"accountable review route":    basis.Contest.AccountableReviewRoute,
		"challenge evidence":          basis.Contest.AllowedChallengeEvidence,
		"possible disposition change": basis.Contest.PossibleDispositionChange,
		"contest outcome condition":   basis.Contest.OutcomeRecordCondition,
		"contest reopen trigger":      basis.Contest.ReopenTrigger,
	} {
		if strings.TrimSpace(value) == "" {
			t.Errorf("P14 scenario %q A.10 source basis lacks %s", scenarioID, label)
		}
	}
	if strings.Contains(basis.LocalResultRuleOwner, "A.10") || strings.Contains(basis.LocalResultRuleOwner, "G.11") {
		t.Errorf("P14 scenario %q makes A.10/G.11 the local-result owner: %q", scenarioID, basis.LocalResultRuleOwner)
	}
	if !strings.HasPrefix(basis.ReliedOnClaimRef, "episteme:") {
		t.Errorf("P14 scenario %q A.10 source basis loses the C.2.1 result-episteme reference", scenarioID)
	}
	if len(basis.ObservedCarrierFacts) < 2 || len(basis.EstablishedDirectRelationRefs) != 0 {
		t.Errorf("P14 scenario %q A.10 partial basis must carry observed bytes but no unproved direct edges: %#v", scenarioID, basis)
	}
	if len(basis.UnresolvedGaps) == 0 {
		t.Errorf("P14 scenario %q A.10 source basis hides its missing direct governors", scenarioID)
	}
	assertHReasonA10MissingGovernors(t, scenarioID, basis.UnresolvedGaps)
	if strings.Contains(strings.ToLower(basis.Contest.PossibleDispositionChange), "degrade") {
		t.Errorf("P14 scenario %q leaks its derived RelianceDisposition in the supplied contest basis", scenarioID)
	}
}

func assertHReasonA10MissingGovernors(t *testing.T, scenarioID string, gaps []string) {
	t.Helper()
	joined := strings.Join(gaps, "\n")
	for _, owner := range []string{"C.2.1", "E.17", "E.24.PUB", "A.13", "A.15.1", "A.6.1", "A.10:4.6", "G.11"} {
		if !strings.Contains(joined, owner) {
			t.Errorf("scenario %q A.10 partial account omits missing direct governor %q", scenarioID, owner)
		}
	}
}

func semanticScenarioHasEvent(
	scenario hReasonSemanticScenario,
	kind string,
	predicate func(hReasonSemanticEvent) bool,
) bool {
	for _, event := range scenario.Events {
		if event.Kind == kind && (predicate == nil || predicate(event)) {
			return true
		}
	}
	return false
}

func semanticScenarioHasFinalClaim(
	scenario hReasonSemanticScenario,
	claimClass string,
	selectionBasis string,
) bool {
	return semanticScenarioHasEvent(scenario, "final_claim", func(event hReasonSemanticEvent) bool {
		return slices.Contains(event.ClaimClasses, claimClass) && event.SelectionBasis == selectionBasis
	})
}

func semanticEventKinds(events []hReasonSemanticEvent) map[string]int {
	counts := make(map[string]int)
	for _, event := range events {
		counts[event.Kind]++
	}
	return counts
}

func semanticEventIndex(events []hReasonSemanticEvent, kind string) int {
	for i, event := range events {
		if event.Kind == kind {
			return i
		}
	}
	return -1
}

func semanticClaimGraphContains(claims []string, fragment string) bool {
	for _, claim := range claims {
		if strings.Contains(claim, fragment) {
			return true
		}
	}
	return false
}

func (s hReasonSemanticScenario) String() string {
	return fmt.Sprintf("%s (%s)", s.ID, s.Requirement)
}
