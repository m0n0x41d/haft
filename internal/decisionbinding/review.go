package decisionbinding

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/m0n0x41d/haft/internal/artifact"
)

const decisionReviewTextLimit = 64 * 1024

// DecisionReviewCard is the semantic, human-facing view of one exact binding
// content. Machine refs and digests remain in DecisionBindingContent and are
// deliberately absent from this normal review surface.
type DecisionReviewCard struct {
	state *decisionReviewCardState
}

type decisionReviewCardState struct {
	content        DecisionBindingContent
	selectedChoice string
	problem        string
	scope          []string
	consequences   []string
	nonEffects     []string
	reversibility  []string
	text           string
}

func NewDecisionReviewCard(
	content DecisionBindingContent,
) (DecisionReviewCard, error) {
	state, err := buildDecisionReviewCardState(content)
	if err != nil {
		return DecisionReviewCard{}, err
	}
	card := DecisionReviewCard{state: &state}
	if !card.valid() {
		return DecisionReviewCard{}, fmt.Errorf("decision review card is inconsistent")
	}
	return card, nil
}

func (content DecisionBindingContent) ReviewCard() (DecisionReviewCard, error) {
	return NewDecisionReviewCard(content)
}

func (card DecisionReviewCard) SelectedChoice() (string, bool) {
	if !card.valid() {
		return "", false
	}
	return card.state.selectedChoice, true
}

func (card DecisionReviewCard) Problem() (string, bool) {
	if !card.valid() {
		return "", false
	}
	return card.state.problem, true
}

func (card DecisionReviewCard) Scope() ([]string, bool) {
	if !card.valid() {
		return nil, false
	}
	return slices.Clone(card.state.scope), true
}

func (card DecisionReviewCard) Consequences() ([]string, bool) {
	if !card.valid() {
		return nil, false
	}
	return slices.Clone(card.state.consequences), true
}

func (card DecisionReviewCard) NonEffects() ([]string, bool) {
	if !card.valid() {
		return nil, false
	}
	return slices.Clone(card.state.nonEffects), true
}

func (card DecisionReviewCard) Reversibility() ([]string, bool) {
	if !card.valid() {
		return nil, false
	}
	return slices.Clone(card.state.reversibility), true
}

func (card DecisionReviewCard) Text() (string, bool) {
	if !card.valid() {
		return "", false
	}
	return card.state.text, true
}

func (card DecisionReviewCard) valid() bool {
	if card.state == nil {
		return false
	}
	rebuilt, err := buildDecisionReviewCardState(card.state.content)
	if err != nil {
		return false
	}
	state := card.state
	return rebuilt.selectedChoice == state.selectedChoice &&
		rebuilt.problem == state.problem &&
		slices.Equal(rebuilt.scope, state.scope) &&
		slices.Equal(rebuilt.consequences, state.consequences) &&
		slices.Equal(rebuilt.nonEffects, state.nonEffects) &&
		slices.Equal(rebuilt.reversibility, state.reversibility) &&
		rebuilt.text == state.text
}

func buildDecisionReviewCardState(
	content DecisionBindingContent,
) (decisionReviewCardState, error) {
	root, rootOK := content.ProjectRoot()
	input, inputOK := content.ResolvedInput()
	snapshot, snapshotOK := content.ReviewSnapshot()
	if !rootOK || !inputOK || !snapshotOK {
		return decisionReviewCardState{}, fmt.Errorf("decision-binding content is invalid")
	}
	title, titleOK := snapshot.Title()
	if !titleOK {
		return decisionReviewCardState{}, fmt.Errorf("decision review snapshot has no title")
	}
	problem := decisionReviewProblem(input, snapshot)
	scope := decisionReviewScope(root.String(), input, snapshot)
	consequences := decisionReviewConsequences(input)
	nonEffects := decisionReviewNonEffects()
	reversibility := decisionReviewReversibility(input)
	text := renderDecisionReviewText(
		title,
		problem,
		scope,
		consequences,
		nonEffects,
		reversibility,
	)
	if err := validateDecisionReviewText(text); err != nil {
		return decisionReviewCardState{}, err
	}
	return decisionReviewCardState{
		content:        content,
		selectedChoice: title,
		problem:        problem,
		scope:          scope,
		consequences:   consequences,
		nonEffects:     nonEffects,
		reversibility:  reversibility,
		text:           text,
	}, nil
}

func decisionReviewProblem(
	input artifact.DecideInput,
	snapshot artifact.PreparedDecisionReview,
) string {
	problem := strings.TrimSpace(input.ProblemStatement)
	if problem != "" {
		return problem
	}
	body, bodyOK := snapshot.Body()
	if !bodyOK {
		return "This choice addresses the prepared project problem basis."
	}
	problem = decisionRecordProblemFrame(body)
	if problem != "" {
		return problem
	}
	return "This choice addresses the linked project problem basis supplied with the decision draft."
}

func decisionRecordProblemFrame(body string) string {
	const frameHeading = "## 1. Problem Frame"
	const decisionHeading = "## 2. Decision"
	start := strings.Index(body, frameHeading)
	if start < 0 {
		return ""
	}
	frame := body[start+len(frameHeading):]
	end := strings.Index(frame, decisionHeading)
	if end >= 0 {
		frame = frame[:end]
	}
	return strings.TrimSpace(frame)
}

func decisionReviewScope(
	projectRoot string,
	input artifact.DecideInput,
	snapshot artifact.PreparedDecisionReview,
) []string {
	mode, _ := snapshot.Mode()
	contextValue, _ := snapshot.Context()
	result := []string{"Project: " + projectRoot}
	result = appendReviewItem(result, "Decision mode: ", string(mode))
	result = appendReviewItem(result, "Project context: ", contextValue)
	result = appendReviewItem(result, "Task context: ", input.TaskContext)
	if strings.TrimSpace(input.DecisionSubjectRef) != "" {
		result = appendUniqueReviewItem(
			result,
			"Decision subject: one explicitly linked project object",
		)
	}
	result = appendReviewFiles(result, input.AffectedFiles)
	result = appendReviewFiles(result, input.ImplementationFootprint.Files)
	result = appendReviewBindingTargets(result, input.BindingTargets)
	if len(input.SectionRefs) > 0 {
		result = appendUniqueReviewItem(
			result,
			"Specification scope: "+strconv.Itoa(len(input.SectionRefs))+" linked section(s)",
		)
	}
	if input.SpecBindingPreflight != nil && strings.TrimSpace(input.SpecBindingPreflight.State) != "" {
		result = appendUniqueReviewItem(
			result,
			"Specification binding state: "+strings.TrimSpace(input.SpecBindingPreflight.State),
		)
	}
	if len(input.GovernanceTargets) > 0 {
		result = appendUniqueReviewItem(
			result,
			"Governance scope: "+strconv.Itoa(len(input.GovernanceTargets))+" target(s)",
		)
	}
	if len(input.DriftWatchTargets) > 0 {
		result = appendUniqueReviewItem(
			result,
			"Drift watch scope: "+strconv.Itoa(len(input.DriftWatchTargets))+" target(s)",
		)
	}
	result = appendReviewItem(result, "Binding scope: ", input.BindingScope)
	result = appendReviewItem(result, "Governance mode: ", input.GovernanceMode)
	return result
}

func appendReviewFiles(result []string, values []string) []string {
	for _, value := range values {
		result = appendReviewItem(result, "Affected file: ", value)
	}
	return result
}

func appendReviewBindingTargets(
	result []string,
	values []artifact.BindingTarget,
) []string {
	for _, value := range values {
		item := bindingTargetReviewItem(value)
		result = appendUniqueReviewItem(result, item)
	}
	return result
}

func bindingTargetReviewItem(value artifact.BindingTarget) string {
	filePath := strings.TrimSpace(value.FilePath)
	symbolName := strings.TrimSpace(value.SymbolName)
	modulePath := strings.TrimSpace(value.ModulePath)
	if symbolName != "" && filePath != "" {
		return "Affected code object: " + symbolName + " in " + filePath
	}
	if modulePath != "" {
		return "Affected module: " + modulePath
	}
	if filePath != "" {
		return "Affected file: " + filePath
	}
	if strings.TrimSpace(value.Kind) != "" {
		return "Affected code scope: " + strings.TrimSpace(value.Kind)
	}
	return ""
}

func decisionReviewConsequences(input artifact.DecideInput) []string {
	result := []string{
		"Project memory will bind the selected choice as a DecisionRecord.",
	}
	result = appendReviewItem(result, "Selection rationale: ", input.WhySelected)
	result = appendReviewItem(result, "Selection policy: ", input.SelectionPolicy)
	result = appendReviewItem(result, "Strongest counterargument: ", input.CounterArgument)
	result = appendReviewItem(result, "Weakest link: ", input.WeakestLink)
	result = appendPrefixedReviewItems(result, "Precondition: ", input.PreConditions)
	result = appendPrefixedReviewItems(result, "Expected postcondition: ", input.PostConditions)
	result = appendPrefixedReviewItems(result, "Invariant to preserve: ", input.Invariants)
	result = appendPrefixedReviewItems(result, "Admissibility condition: ", input.Admissibility)
	result = appendPrefixedReviewItems(result, "Evidence obligation: ", input.EvidenceReqs)
	for _, reason := range input.WhyNotOthers {
		result = appendReviewItem(result, "Rejected-alternative rationale: ", reason.Reason)
	}
	for _, prediction := range input.Predictions {
		result = appendReviewItem(result, "Claim to check: ", prediction.Claim)
	}
	for _, claim := range input.Claims {
		result = appendReviewItem(result, "Decision claim: ", claim.Claim)
	}
	result = appendTransformationConsequence(result, input.TransformationRecord)
	if len(input.Skips) > 0 {
		result = appendReviewItem(result, "Acknowledged draft omission: ", input.SkipReason)
	}
	return result
}

func appendTransformationConsequence(
	result []string,
	record *artifact.TransformationRecord,
) []string {
	if record == nil {
		return result
	}
	parts := []string{
		strings.TrimSpace(record.TransformedEntity),
		"changes from",
		strings.TrimSpace(record.InitialState),
		"to",
		strings.TrimSpace(record.PostState),
	}
	consequence := strings.Join(parts, " ")
	if strings.TrimSpace(record.Relation) != "" {
		consequence += " under relation " + strings.TrimSpace(record.Relation)
	}
	return appendReviewItem(result, "Target-state consequence: ", consequence)
}

func decisionReviewNonEffects() []string {
	return []string{
		"It does not perform implementation Work or claim that planned Work already happened.",
		"It does not grant execution authority or create a WorkCommission.",
		"It does not edit code, specifications, or project-profile carriers.",
		"It does not approve, reopen, or rebaseline a specification lifecycle.",
		"It does not activate TypeEnv, commit, push, release, or deploy anything.",
	}
}

func decisionReviewReversibility(input artifact.DecideInput) []string {
	result := []string{
		"Canceling creates no DecisionRecord, decision authority, WorkCommission, code change, specification change, or project-profile change.",
		"An inert prepared review carrier may remain for audit or exact retry; its existence alone cannot satisfy a decision gate or institute any effect.",
	}
	choice := input.ChoiceResult
	if choice != nil {
		result = appendReviewItem(result, "Reversibility: ", choice.Reversibility)
		result = appendReviewItem(result, "Reopen when: ", choice.ReopenCondition)
	}
	rollback := input.Rollback
	if rollback != nil {
		result = appendPrefixedReviewItems(result, "Rollback trigger: ", rollback.Triggers)
		result = appendPrefixedReviewItems(result, "Rollback step: ", rollback.Steps)
		result = appendReviewItem(result, "Rollback blast radius: ", rollback.BlastRadius)
	}
	result = appendReviewItem(result, "Review validity boundary: ", input.ValidUntil)
	result = appendPrefixedReviewItems(result, "Reconsider when: ", input.RefreshTriggers)
	if len(result) == 2 {
		result = append(
			result,
			"No explicit reversal detail is present; effect-time DecisionRecord validation remains authoritative.",
		)
	}
	return result
}

func renderDecisionReviewText(
	selectedChoice string,
	problem string,
	scope []string,
	consequences []string,
	nonEffects []string,
	reversibility []string,
) string {
	sections := []string{
		"Bind this project decision",
		"Selected choice\n" + selectedChoice,
		"Problem\n" + problem,
		renderReviewList("Scope", scope),
		renderReviewList("Consequences", consequences),
		renderReviewList("This will not", nonEffects),
		renderReviewList("Cancel and reversibility", reversibility),
	}
	return strings.Join(sections, "\n\n")
}

func renderReviewList(heading string, values []string) string {
	lines := []string{heading}
	for _, value := range values {
		lines = append(lines, "- "+value)
	}
	return strings.Join(lines, "\n")
}

func appendPrefixedReviewItems(
	result []string,
	prefix string,
	values []string,
) []string {
	for _, value := range values {
		result = appendReviewItem(result, prefix, value)
	}
	return result
}

func appendReviewItem(result []string, prefix string, raw string) []string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return result
	}
	return appendUniqueReviewItem(result, prefix+value)
}

func appendUniqueReviewItem(result []string, value string) []string {
	if value == "" || slices.Contains(result, value) {
		return result
	}
	return append(result, value)
}

func validateDecisionReviewText(value string) error {
	if value == "" || strings.TrimSpace(value) != value {
		return fmt.Errorf("decision review text must be non-empty canonical text")
	}
	if len(value) > decisionReviewTextLimit {
		return fmt.Errorf("decision review text exceeds 64 KiB")
	}
	invalidControl := strings.ContainsFunc(value, func(item rune) bool {
		return unicode.IsControl(item) && item != '\n' && item != '\t'
	})
	if invalidControl {
		return fmt.Errorf("decision review text contains unsupported control characters")
	}
	return nil
}
