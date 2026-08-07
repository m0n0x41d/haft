package decisionbinding

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/authority"
)

const decisionBindingContentSchema = "haft.decision-binding-content/v1"

// DecisionBindingContent is the exact domain object reviewed by a person and
// later named by a manual decision SpeechAct. It can only be constructed from
// a fully resolved PreparedDecision; raw DecideInput is intentionally not an
// authorization boundary.
type DecisionBindingContent struct {
	state *decisionBindingContentState
}

type decisionBindingContentState struct {
	prepared       artifact.PreparedDecision
	canonicalBytes []byte
	digest         authority.Digest
}

type decisionBindingContentJSONV1 struct {
	Schema                 string          `json:"schema"`
	PreparedDecisionDigest string          `json:"prepared_decision_digest"`
	PreparedDecision       json.RawMessage `json:"prepared_decision"`
}

// NewDecisionBindingContent closes the decision-domain adapter over one exact
// PreparedDecision. Generic SpeechAct machinery sees only the resulting
// immutable content digest; preparation and review remain domain-owned.
func NewDecisionBindingContent(
	prepared artifact.PreparedDecision,
) (DecisionBindingContent, error) {
	rootValue, rootOK := prepared.ProjectRoot()
	decisionRef, decisionRefOK := prepared.DecisionRef()
	input, inputOK := prepared.ResolvedInput()
	preparedBytes, bytesOK := prepared.CanonicalBytes()
	preparedDigest, digestOK := prepared.Digest()
	if !rootOK || !decisionRefOK || !inputOK || !bytesOK || !digestOK {
		return DecisionBindingContent{}, fmt.Errorf("prepared decision is invalid")
	}
	root, err := authority.NewProjectRoot(rootValue)
	if err != nil {
		return DecisionBindingContent{}, fmt.Errorf(
			"decision binding requires a canonical project root: %w",
			err,
		)
	}
	if root.String() != rootValue || strings.TrimSpace(decisionRef) == "" {
		return DecisionBindingContent{}, fmt.Errorf("prepared decision identity is invalid")
	}
	if err := validateDecisionBindingBasis(input); err != nil {
		return DecisionBindingContent{}, err
	}
	projection := decisionBindingContentJSONV1{
		Schema:                 decisionBindingContentSchema,
		PreparedDecisionDigest: preparedDigest.String(),
		PreparedDecision:       slices.Clone(preparedBytes),
	}
	contentBytes, err := json.Marshal(projection)
	if err != nil {
		return DecisionBindingContent{}, fmt.Errorf("encode decision-binding content: %w", err)
	}
	contentDigest, err := digestBytes(contentBytes)
	if err != nil {
		return DecisionBindingContent{}, err
	}
	state := decisionBindingContentState{
		prepared:       prepared,
		canonicalBytes: slices.Clone(contentBytes),
		digest:         contentDigest,
	}
	content := DecisionBindingContent{state: &state}
	if !content.valid() {
		return DecisionBindingContent{}, fmt.Errorf("decision-binding content is inconsistent")
	}
	return content, nil
}

func (content DecisionBindingContent) PreparedDecision() (
	artifact.PreparedDecision,
	bool,
) {
	if !content.valid() {
		return artifact.PreparedDecision{}, false
	}
	return content.state.prepared, true
}

func (content DecisionBindingContent) ProjectRoot() (authority.ProjectRoot, bool) {
	if !content.valid() {
		return authority.ProjectRoot{}, false
	}
	value, ok := content.state.prepared.ProjectRoot()
	if !ok {
		return authority.ProjectRoot{}, false
	}
	root, err := authority.NewProjectRoot(value)
	return root, err == nil
}

// ContentRef is the stable address reviewed by the manual SpeechAct. It is
// derived only from the exact content digest; recording this content does not
// itself imply that a person performed any act.
func (content DecisionBindingContent) ContentRef() (
	authority.SpeechActReviewSubjectRef,
	bool,
) {
	if !content.valid() {
		return authority.SpeechActReviewSubjectRef{}, false
	}
	digest, ok := content.Digest()
	if !ok {
		return authority.SpeechActReviewSubjectRef{}, false
	}
	identity := strings.TrimPrefix(digest.String(), "sha256:")
	ref, err := authority.NewSpeechActReviewSubjectRef(
		"review-subject:decision-binding:" + identity,
	)
	return ref, err == nil
}

func (content DecisionBindingContent) DecisionRef() (string, bool) {
	if !content.valid() {
		return "", false
	}
	return content.state.prepared.DecisionRef()
}

// ResolvedInput returns the post-enrichment input captured inside the exact
// PreparedDecision. It is a defensive review view, not a binding constructor.
func (content DecisionBindingContent) ResolvedInput() (artifact.DecideInput, bool) {
	if !content.valid() {
		return artifact.DecideInput{}, false
	}
	return content.state.prepared.ResolvedInput()
}

func (content DecisionBindingContent) ReviewSnapshot() (
	artifact.PreparedDecisionReview,
	bool,
) {
	if !content.valid() {
		return artifact.PreparedDecisionReview{}, false
	}
	return content.state.prepared.ReviewSnapshot()
}

func (content DecisionBindingContent) CanonicalBytes() ([]byte, bool) {
	if !content.valid() {
		return nil, false
	}
	return slices.Clone(content.state.canonicalBytes), true
}

func (content DecisionBindingContent) Digest() (authority.Digest, bool) {
	if !content.valid() {
		return authority.Digest{}, false
	}
	return content.state.digest, true
}

func (content DecisionBindingContent) valid() bool {
	if content.state == nil {
		return false
	}
	prepared := content.state.prepared
	rootValue, rootOK := prepared.ProjectRoot()
	decisionRef, decisionRefOK := prepared.DecisionRef()
	input, inputOK := prepared.ResolvedInput()
	preparedBytes, bytesOK := prepared.CanonicalBytes()
	preparedDigest, digestOK := prepared.Digest()
	if !rootOK || !decisionRefOK || !inputOK || !bytesOK || !digestOK {
		return false
	}
	root, err := authority.NewProjectRoot(rootValue)
	if err != nil || root.String() != rootValue || strings.TrimSpace(decisionRef) == "" {
		return false
	}
	if err := validateDecisionBindingBasis(input); err != nil {
		return false
	}
	projection := decisionBindingContentJSONV1{
		Schema:                 decisionBindingContentSchema,
		PreparedDecisionDigest: preparedDigest.String(),
		PreparedDecision:       slices.Clone(preparedBytes),
	}
	contentBytes, err := json.Marshal(projection)
	if err != nil || !slices.Equal(contentBytes, content.state.canonicalBytes) {
		return false
	}
	contentDigest, err := digestBytes(contentBytes)
	return err == nil && contentDigest == content.state.digest
}

func validateDecisionBindingBasis(input artifact.DecideInput) error {
	selectedTitle := strings.TrimSpace(input.SelectedTitle)
	if selectedTitle == "" || selectedTitle != input.SelectedTitle {
		return fmt.Errorf("decision binding requires a canonical non-empty selected_title")
	}
	if err := artifact.ValidateChoiceResult(input.ChoiceResult); err != nil {
		return err
	}
	if err := validateReviewedChoice(input); err != nil {
		return err
	}
	if err := artifact.ValidateTransformationRecord(input.TransformationRecord); err != nil {
		return err
	}
	return nil
}

func validateReviewedChoice(input artifact.DecideInput) error {
	choice := input.ChoiceResult
	if choice == nil {
		return nil
	}
	if choice.NextMove != artifact.ChoiceNextMoveChooseNow {
		return fmt.Errorf("decision binding requires choice_result.next_move=choose_now")
	}
	if strings.TrimSpace(choice.VariantRef) != input.SelectedTitle {
		return fmt.Errorf(
			"choice_result.variant_ref must exactly match the human-reviewed selected_title",
		)
	}
	return nil
}

func digestBytes(value []byte) (authority.Digest, error) {
	hash := sha256.Sum256(value)
	hexValue := hex.EncodeToString(hash[:])
	return authority.NewDigest("sha256:" + hexValue)
}
