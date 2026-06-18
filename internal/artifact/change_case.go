package artifact

import (
	"fmt"
	"strings"
	"time"
)

const (
	EngineeringChangeCaseSchemaVersion = 1
	EngineeringChangeCaseRecordKind    = "engineering_change_case"
	EngineeringChangeCaseAuthority     = "derived_aggregate_projection"

	EngineeringChangeCaseNextReviewEvidence = "review_evidence_and_drift_before_stronger_use"
	EngineeringChangeCaseNextDeclareUse     = "declare_attempted_use_to_materialize_evidence_paths"

	EngineeringChangeCaseBoundaryNotProof          = "not_proof"
	EngineeringChangeCaseBoundaryNotApproval       = "not_approval"
	EngineeringChangeCaseBoundaryNotGateDecision   = "not_gate_decision"
	EngineeringChangeCaseBoundaryNotWorkOccurrence = "not_work_occurrence"
	EngineeringChangeCaseBoundaryNotGlobalTruth    = "not_global_truth"
)

type EngineeringChangeCaseInput struct {
	DecisionRef  string
	AttemptedUse string
	ProducerRef  string
	MethodRef    string
	WorkRef      string
}

type EngineeringChangeCase struct {
	SchemaVersion        int                                   `json:"schema_version"`
	RecordKind           string                                `json:"record_kind"`
	Authority            string                                `json:"authority"`
	CaseRef              string                                `json:"case_ref"`
	DecisionSpeechActRef string                                `json:"decision_speech_act_ref"`
	ProblemCardRefs      []string                              `json:"problem_card_refs,omitempty"`
	Problems             []EngineeringChangeCaseProblem        `json:"problems,omitempty"`
	TransformationRefs   []string                              `json:"transformation_refs,omitempty"`
	Transformations      []EngineeringChangeCaseTransformation `json:"transformations,omitempty"`
	ChoiceResultRef      string                                `json:"choice_result_ref,omitempty"`
	ChoiceResult         *ChoiceResult                         `json:"choice_result,omitempty"`
	CandidateSetRef      string                                `json:"candidate_set_ref,omitempty"`
	ComparisonResultRef  string                                `json:"comparison_result_ref,omitempty"`
	EvidenceItemRefs     []string                              `json:"evidence_item_refs,omitempty"`
	EvidencePathRefs     []string                              `json:"evidence_path_refs,omitempty"`
	EvidencePaths        []EvidencePathRecord                  `json:"evidence_paths,omitempty"`
	OpenDriftFindingRefs []string                              `json:"open_drift_finding_refs,omitempty"`
	NextAdmissibleMove   string                                `json:"next_admissible_move"`
	AuthorityBoundary    EngineeringChangeCaseBoundary         `json:"authority_boundary"`
	DerivedAt            string                                `json:"derived_at"`
}

type EngineeringChangeCaseProblem struct {
	Ref       string `json:"ref"`
	Title     string `json:"title,omitempty"`
	Status    string `json:"status,omitempty"`
	Profile   string `json:"profile,omitempty"`
	Readiness string `json:"readiness,omitempty"`
}

type EngineeringChangeCaseTransformation struct {
	Ref    string               `json:"ref"`
	Record TransformationRecord `json:"record"`
}

type EngineeringChangeCaseBoundary struct {
	Proof          string `json:"proof"`
	Approval       string `json:"approval"`
	GateDecision   string `json:"gate_decision"`
	WorkOccurrence string `json:"work_occurrence"`
	GlobalTruth    string `json:"global_truth"`
}

func BuildEngineeringChangeCase(
	input EngineeringChangeCaseInput,
	decision *Artifact,
	problems []*Artifact,
	evidence []EvidenceItem,
	now time.Time,
) (EngineeringChangeCase, error) {
	normalized := normalizeEngineeringChangeCaseInput(input)
	if decision == nil {
		return EngineeringChangeCase{}, fmt.Errorf("decision artifact is required")
	}
	if decision.Meta.Kind != KindDecisionRecord {
		return EngineeringChangeCase{}, fmt.Errorf("%s is %s, not DecisionRecord", decision.Meta.ID, decision.Meta.Kind)
	}
	if normalized.DecisionRef == "" {
		normalized.DecisionRef = decision.Meta.ID
	}
	if normalized.DecisionRef != decision.Meta.ID {
		return EngineeringChangeCase{}, fmt.Errorf("decision_ref %q does not match artifact %q", normalized.DecisionRef, decision.Meta.ID)
	}

	fields := decision.UnmarshalDecisionFields()
	choice := NormalizeChoiceResult(fields.ChoiceResult)
	problemRefs := engineeringChangeCaseProblemRefs(fields, choice)
	transformation := NormalizeTransformationRecord(fields.TransformationRecord)
	evidenceRefs := engineeringChangeCaseEvidenceRefs(evidence)
	evidencePaths := engineeringChangeCaseEvidencePaths(normalized, evidence, now)

	return EngineeringChangeCase{
		SchemaVersion:        EngineeringChangeCaseSchemaVersion,
		RecordKind:           EngineeringChangeCaseRecordKind,
		Authority:            EngineeringChangeCaseAuthority,
		CaseRef:              "change-case:" + normalized.DecisionRef,
		DecisionSpeechActRef: normalized.DecisionRef,
		ProblemCardRefs:      problemRefs,
		Problems:             engineeringChangeCaseProblems(problemRefs, problems),
		TransformationRefs:   engineeringChangeCaseTransformationRefs(normalized.DecisionRef, transformation),
		Transformations:      engineeringChangeCaseTransformations(normalized.DecisionRef, transformation),
		ChoiceResultRef:      engineeringChangeCaseChoiceResultRef(normalized.DecisionRef, choice),
		ChoiceResult:         choice,
		CandidateSetRef:      engineeringChangeCaseCandidateSetRef(choice),
		ComparisonResultRef:  engineeringChangeCaseComparisonResultRef(choice),
		EvidenceItemRefs:     evidenceRefs,
		EvidencePathRefs:     engineeringChangeCaseEvidencePathRefs(normalized.DecisionRef, evidencePaths),
		EvidencePaths:        evidencePaths,
		NextAdmissibleMove:   engineeringChangeCaseNextMove(normalized, evidence),
		AuthorityBoundary: EngineeringChangeCaseBoundary{
			Proof:          EngineeringChangeCaseBoundaryNotProof,
			Approval:       EngineeringChangeCaseBoundaryNotApproval,
			GateDecision:   EngineeringChangeCaseBoundaryNotGateDecision,
			WorkOccurrence: EngineeringChangeCaseBoundaryNotWorkOccurrence,
			GlobalTruth:    EngineeringChangeCaseBoundaryNotGlobalTruth,
		},
		DerivedAt: engineeringChangeCaseDerivedAt(now),
	}, nil
}

func normalizeEngineeringChangeCaseInput(input EngineeringChangeCaseInput) EngineeringChangeCaseInput {
	return EngineeringChangeCaseInput{
		DecisionRef:  strings.TrimSpace(input.DecisionRef),
		AttemptedUse: strings.TrimSpace(input.AttemptedUse),
		ProducerRef:  strings.TrimSpace(input.ProducerRef),
		MethodRef:    strings.TrimSpace(input.MethodRef),
		WorkRef:      strings.TrimSpace(input.WorkRef),
	}
}

func engineeringChangeCaseProblemRefs(fields DecisionFields, choice *ChoiceResult) []string {
	refs := []string{}
	for _, ref := range compactStrings(fields.ProblemRefs) {
		refs = appendUniqueString(refs, ref)
	}
	if choice != nil {
		for _, ref := range compactStrings(choice.ProblemRefs) {
			refs = appendUniqueString(refs, ref)
		}
	}

	return refs
}

func engineeringChangeCaseProblems(refs []string, problems []*Artifact) []EngineeringChangeCaseProblem {
	problemByRef := make(map[string]*Artifact, len(problems))
	for _, problem := range problems {
		if problem == nil {
			continue
		}
		problemByRef[problem.Meta.ID] = problem
	}

	cases := make([]EngineeringChangeCaseProblem, 0, len(refs))
	for _, ref := range refs {
		item := EngineeringChangeCaseProblem{Ref: ref}
		if problem, ok := problemByRef[ref]; ok {
			fields := problem.UnmarshalProblemFields()
			item.Title = problem.Meta.Title
			item.Status = string(problem.Meta.Status)
			if fields.Profile != nil {
				item.Profile = fields.Profile.Level
				item.Readiness = fields.Profile.Readiness
			}
		}
		cases = append(cases, item)
	}

	return cases
}

func engineeringChangeCaseTransformationRefs(decisionRef string, record *TransformationRecord) []string {
	if record == nil {
		return nil
	}

	return []string{decisionRef + "#transformation_record"}
}

func engineeringChangeCaseTransformations(
	decisionRef string,
	record *TransformationRecord,
) []EngineeringChangeCaseTransformation {
	if record == nil {
		return nil
	}

	return []EngineeringChangeCaseTransformation{{
		Ref:    decisionRef + "#transformation_record",
		Record: *record,
	}}
}

func engineeringChangeCaseChoiceResultRef(decisionRef string, choice *ChoiceResult) string {
	if choice == nil {
		return ""
	}

	return decisionRef + "#choice_result"
}

func engineeringChangeCaseCandidateSetRef(choice *ChoiceResult) string {
	if choice == nil {
		return ""
	}

	return choice.PortfolioRef
}

func engineeringChangeCaseComparisonResultRef(choice *ChoiceResult) string {
	if choice == nil || choice.PortfolioRef == "" {
		return ""
	}

	return choice.PortfolioRef + "#comparison_result"
}

func engineeringChangeCaseEvidenceRefs(evidence []EvidenceItem) []string {
	refs := make([]string, 0, len(evidence))
	for _, item := range evidence {
		for _, ref := range compactStrings([]string{item.ID}) {
			refs = appendUniqueString(refs, ref)
		}
	}

	return refs
}

func engineeringChangeCaseEvidencePaths(
	input EngineeringChangeCaseInput,
	evidence []EvidenceItem,
	now time.Time,
) []EvidencePathRecord {
	if input.AttemptedUse == "" {
		return nil
	}

	paths := make([]EvidencePathRecord, 0, len(evidence))
	for _, item := range evidence {
		paths = append(paths, BuildEvidencePathRecord(
			EvidencePathInput{
				ArtifactRef:  input.DecisionRef,
				EvidenceRef:  item.ID,
				AttemptedUse: input.AttemptedUse,
				ProducerRef:  input.ProducerRef,
				MethodRef:    input.MethodRef,
				WorkRef:      input.WorkRef,
			},
			item,
			now,
		))
	}

	return paths
}

func engineeringChangeCaseEvidencePathRefs(
	decisionRef string,
	paths []EvidencePathRecord,
) []string {
	refs := make([]string, 0, len(paths))
	for _, path := range paths {
		for _, ref := range compactStrings([]string{decisionRef + "#evidence_path:" + path.Evidence.ID}) {
			refs = appendUniqueString(refs, ref)
		}
	}

	return refs
}

func engineeringChangeCaseNextMove(input EngineeringChangeCaseInput, evidence []EvidenceItem) string {
	if len(evidence) > 0 && input.AttemptedUse == "" {
		return EngineeringChangeCaseNextDeclareUse
	}

	return EngineeringChangeCaseNextReviewEvidence
}

func engineeringChangeCaseDerivedAt(now time.Time) string {
	if now.IsZero() {
		now = time.Now().UTC()
	}

	return now.UTC().Format(time.RFC3339)
}
