package authority

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

const (
	speechActMethodDescriptionDigestDomain  = "haft.authority.speech-act-method-description/v1"
	manualAuthorityIssueMethodRefValue      = "method:manual-authority-issue"
	manualAuthorityIssueDescriptionRefValue = "method-description:manual-authority-issue:v1"
	manualAuthorityIssueProcedureRefValue   = "procedure:review-exact-intent-capture-controlling-terminal:v1"
	manualAuthorityIssueContextRefValue     = "bounded-context:haft-local-authority"
	manualAuthorityIssueProcedureSemantics  = "display exact pre-act review bindings; require the policy-owned canonical utterance on the controlling terminal; observe terminal session and capture time; derive capture, authorizer assignment, and SpeechAct in that order"
)

type SpeechActMethodDescription struct {
	state *speechActMethodDescriptionState
}

type speechActMethodDescriptionState struct {
	methodRef          MethodRef
	ref                MethodDescriptionRef
	procedureRef       MethodProcedureRef
	boundedContext     BoundedContextRef
	procedureSemantics string
	digest             Digest
	canonicalJSON      []byte
}

func NewSpeechActMethodDescription(
	methodRef MethodRef,
	ref MethodDescriptionRef,
	procedureRef MethodProcedureRef,
	boundedContext BoundedContextRef,
	procedureSemantics string,
) (SpeechActMethodDescription, error) {
	canonicalSemantics := procedureSemantics != "" &&
		len(procedureSemantics) <= 8*1024 &&
		strings.TrimSpace(procedureSemantics) == procedureSemantics &&
		!strings.ContainsAny(procedureSemantics, "\r\x00")
	if !methodRef.valid() || !ref.valid() || !procedureRef.valid() ||
		!boundedContext.valid() || !canonicalSemantics {
		return SpeechActMethodDescription{}, fmt.Errorf("SpeechAct MethodDescription requires canonical refs")
	}
	projection := struct {
		Schema             string `json:"schema"`
		MethodRef          string `json:"method_ref"`
		Ref                string `json:"method_description_ref"`
		ProcedureRef       string `json:"procedure_ref"`
		BoundedContext     string `json:"bounded_context_ref"`
		ProcedureSemantics string `json:"procedure_semantics"`
	}{
		Schema:             "haft.authority.speech-act-method-description/v1",
		MethodRef:          methodRef.String(),
		Ref:                ref.String(),
		ProcedureRef:       procedureRef.String(),
		BoundedContext:     boundedContext.String(),
		ProcedureSemantics: procedureSemantics,
	}
	canonicalJSON, err := json.Marshal(projection)
	if err != nil {
		return SpeechActMethodDescription{}, fmt.Errorf("encode SpeechAct MethodDescription: %w", err)
	}
	writer := newAuthorityDigestWriter(speechActMethodDescriptionDigestDomain)
	writer.add(string(canonicalJSON))
	state := speechActMethodDescriptionState{
		methodRef:          methodRef,
		ref:                ref,
		procedureRef:       procedureRef,
		boundedContext:     boundedContext,
		procedureSemantics: procedureSemantics,
		digest:             writer.digest(),
		canonicalJSON:      canonicalJSON,
	}
	return SpeechActMethodDescription{state: &state}, nil
}

// NewManualControllingTTYMethodDescription binds the sealed manual
// controlling-terminal procedure to caller-owned Method and context refs.
// The procedure is shared; the judgement context is not.
func NewManualControllingTTYMethodDescription(
	methodRef MethodRef,
	ref MethodDescriptionRef,
	procedureRef MethodProcedureRef,
	boundedContext BoundedContextRef,
) (SpeechActMethodDescription, error) {
	description, err := NewSpeechActMethodDescription(
		methodRef,
		ref,
		procedureRef,
		boundedContext,
		manualAuthorityIssueProcedureSemantics,
	)
	if err != nil {
		return SpeechActMethodDescription{}, fmt.Errorf("build manual controlling-terminal MethodDescription: %w", err)
	}
	return description, nil
}

// ManualAuthorityIssueMethodDescription is the sealed, recomputable source
// for the manual /dev/tty issue Method and MethodDescription pair.
func ManualAuthorityIssueMethodDescription() SpeechActMethodDescription {
	methodRef, _ := NewMethodRef(manualAuthorityIssueMethodRefValue)
	ref, _ := NewMethodDescriptionRef(manualAuthorityIssueDescriptionRefValue)
	procedureRef, _ := NewMethodProcedureRef(manualAuthorityIssueProcedureRefValue)
	boundedContext, _ := NewBoundedContextRef(manualAuthorityIssueContextRefValue)
	description, _ := NewManualControllingTTYMethodDescription(
		methodRef,
		ref,
		procedureRef,
		boundedContext,
	)
	return description
}

func (description SpeechActMethodDescription) MethodRef() (MethodRef, bool) {
	if !description.valid() {
		return MethodRef{}, false
	}
	return description.state.methodRef, true
}

func (description SpeechActMethodDescription) Ref() (MethodDescriptionRef, bool) {
	if !description.valid() {
		return MethodDescriptionRef{}, false
	}
	return description.state.ref, true
}

func (description SpeechActMethodDescription) Digest() (Digest, bool) {
	if !description.valid() {
		return Digest{}, false
	}
	return description.state.digest, true
}

func (description SpeechActMethodDescription) valid() bool {
	if description.state == nil {
		return false
	}
	rebuilt, err := NewSpeechActMethodDescription(
		description.state.methodRef,
		description.state.ref,
		description.state.procedureRef,
		description.state.boundedContext,
		description.state.procedureSemantics,
	)
	return err == nil &&
		rebuilt.state.digest == description.state.digest &&
		slices.Equal(rebuilt.state.canonicalJSON, description.state.canonicalJSON)
}
