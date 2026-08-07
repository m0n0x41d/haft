package projecttypeenvselectionauthority

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
)

const (
	speechActRecordSchema       = "haft.project-typeenv.head-selection-speech-act-record/v1"
	speechActRecordDomain       = "haft.project-typeenv.head-selection-speech-act-record/v1"
	maximumSpeechActRecordBytes = 1024 * 1024
)

// ProjectTypeEnvHeadSelectionSpeechActRecord is the durable description of
// one exact communicative Work occurrence. It embeds the canonical generic v2
// source, then binds that source to one exact reviewed content and request. It
// is not the SpeechAct occurrence or an authority resolution.
type ProjectTypeEnvHeadSelectionSpeechActRecord struct {
	ref              ProjectTypeEnvHeadSelectionSpeechActRecordRef
	digest           authority.Digest
	source           authority.VerifiedSpeechActSourceV2
	sourceContract   ProjectTypeEnvHeadSelectionSpeechActSourceContract
	projectBinding   ProjectAuthorityContextBinding
	content          ProjectTypeEnvHeadSelectionAuthorizationContent
	permissionRecord ProjectTypeEnvHeadSelectionPermissionRecord
	requiredAffected []authority.AffectedRef
	canonicalJSON    []byte
}

type ProjectTypeEnvHeadSelectionSpeechActRecordInput struct {
	Source         authority.VerifiedSpeechActSourceV2
	SourceContract ProjectTypeEnvHeadSelectionSpeechActSourceContract
	ProjectBinding ProjectAuthorityContextBinding
	Content        ProjectTypeEnvHeadSelectionAuthorizationContent
	Request        projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest
	Stage          projecttypeenvselection.ProjectTypeEnvStage
}

type speechActRecordProjection struct {
	Schema                 string                   `json:"schema"`
	SourceDigest           string                   `json:"source_digest"`
	Source                 json.RawMessage          `json:"source"`
	SourceContractDigest   string                   `json:"source_contract_digest"`
	SourceContract         json.RawMessage          `json:"source_contract"`
	ProjectBindingDigest   string                   `json:"project_context_binding_digest"`
	ProjectBinding         json.RawMessage          `json:"project_context_binding"`
	SpeechActRef           string                   `json:"speech_act_ref"`
	WorkRef                string                   `json:"work_ref"`
	ContentDescriptionKind string                   `json:"content_description_ref_kind"`
	ContentDescriptionRef  string                   `json:"content_description_ref"`
	ContentDigest          string                   `json:"content_digest"`
	PermissionRef          string                   `json:"permission_ref"`
	PermissionRecordDigest string                   `json:"permission_record_digest"`
	RequestRef             string                   `json:"request_ref"`
	RequestDigest          string                   `json:"request_digest"`
	Project                string                   `json:"project_id"`
	JudgementContext       string                   `json:"judgement_context_ref"`
	Action                 string                   `json:"action"`
	IntendedUpdate         intendedUpdateProjection `json:"intended_head_update"`
	RequiredAffectedRefs   []string                 `json:"required_affected_refs"`
	Stage                  string                   `json:"stage_ref"`
	VerifiedComposite      string                   `json:"verified_composite_ref"`
}

func SealProjectTypeEnvHeadSelectionSpeechActRecord(
	input ProjectTypeEnvHeadSelectionSpeechActRecordInput,
) (ProjectTypeEnvHeadSelectionSpeechActRecord, error) {
	if !input.Source.Valid() {
		return ProjectTypeEnvHeadSelectionSpeechActRecord{}, fmt.Errorf(
			"head-selection SpeechAct record requires a verified v2 source",
		)
	}
	if err := input.Content.ExactAgainst(input.Request); err != nil {
		return ProjectTypeEnvHeadSelectionSpeechActRecord{}, fmt.Errorf(
			"head-selection SpeechAct content binding: %w",
			err,
		)
	}
	if !sameStage(input.Content.Stage(), input.Stage) {
		return ProjectTypeEnvHeadSelectionSpeechActRecord{}, fmt.Errorf(
			"head-selection SpeechAct Stage mismatch",
		)
	}
	request := input.Request
	if !sameRequest(input.Content.Request(), request) {
		return ProjectTypeEnvHeadSelectionSpeechActRecord{}, fmt.Errorf(
			"head-selection SpeechAct request mismatch",
		)
	}
	if err := verifySourceContentBinding(input.Source, input.Content); err != nil {
		return ProjectTypeEnvHeadSelectionSpeechActRecord{}, err
	}
	if err := input.SourceContract.ExactAgainst(input.Content.JudgementContext()); err != nil {
		return ProjectTypeEnvHeadSelectionSpeechActRecord{}, err
	}
	if err := input.SourceContract.VerifySource(input.Source, input.Content); err != nil {
		return ProjectTypeEnvHeadSelectionSpeechActRecord{}, err
	}
	permissionRecord, err := SealProjectTypeEnvHeadSelectionPermissionRecord(
		input.Content,
		input.Source,
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionSpeechActRecord{}, err
	}
	projectRoot, rootOK := input.Source.ProjectRoot()
	if !rootOK || !input.ProjectBinding.ExactFor(
		request.Project(),
		projectRoot,
		input.Content.JudgementContext(),
	) {
		return ProjectTypeEnvHeadSelectionSpeechActRecord{}, fmt.Errorf(
			"head-selection SpeechAct ProjectRoot/ProjectID binding mismatch",
		)
	}
	required, err := requiredAffectedReferents(request)
	if err != nil {
		return ProjectTypeEnvHeadSelectionSpeechActRecord{}, err
	}
	if err := verifyRequiredAffectedReferents(input.Source, required); err != nil {
		return ProjectTypeEnvHeadSelectionSpeechActRecord{}, err
	}
	record := ProjectTypeEnvHeadSelectionSpeechActRecord{
		source:           input.Source,
		sourceContract:   input.SourceContract,
		projectBinding:   input.ProjectBinding,
		content:          input.Content,
		permissionRecord: permissionRecord,
		requiredAffected: required,
	}
	projection, err := projectSpeechActRecord(record)
	if err != nil {
		return ProjectTypeEnvHeadSelectionSpeechActRecord{}, err
	}
	canonical, err := json.Marshal(projection)
	if err != nil {
		return ProjectTypeEnvHeadSelectionSpeechActRecord{}, fmt.Errorf(
			"encode head-selection SpeechAct record: %w",
			err,
		)
	}
	if len(canonical) > maximumSpeechActRecordBytes {
		return ProjectTypeEnvHeadSelectionSpeechActRecord{}, fmt.Errorf(
			"head-selection SpeechAct record exceeds %d bytes",
			maximumSpeechActRecordBytes,
		)
	}
	digest, err := digestCanonical(speechActRecordDomain, canonical)
	if err != nil {
		return ProjectTypeEnvHeadSelectionSpeechActRecord{}, err
	}
	record.digest = digest
	record.ref = ProjectTypeEnvHeadSelectionSpeechActRecordRef{digest: digest}
	record.canonicalJSON = canonical
	return record, nil
}

func verifySourceContentBinding(
	source authority.VerifiedSpeechActSourceV2,
	content ProjectTypeEnvHeadSelectionAuthorizationContent,
) error {
	description, descriptionOK := source.DescriptionRef()
	descriptionDigest, digestOK := source.DescriptionDigest()
	context, contextOK := source.BoundedContext()
	policy, policyOK := source.ContextPolicy()
	policyContext, policyContextOK := policy.BoundedContext()
	window, windowOK := source.WorkWindow()
	if !descriptionOK || !digestOK || !contextOK || !policyOK ||
		!policyContextOK || !windowOK {
		return fmt.Errorf("head-selection SpeechAct source lacks exact v2 coordinates")
	}
	if description != content.DescriptionRef() || descriptionDigest != content.Digest() {
		return fmt.Errorf("head-selection SpeechAct reviewed content mismatch")
	}
	if context != content.JudgementContext() || policyContext != content.JudgementContext() {
		return fmt.Errorf("head-selection SpeechAct judgement context mismatch")
	}
	validity := content.ValidityWindow()
	windowCovered := !window.From().Before(validity.From()) &&
		!window.Until().After(validity.Until())
	if !windowCovered {
		return fmt.Errorf("head-selection SpeechAct Work is outside content validity window")
	}
	return nil
}

func requiredAffectedReferents(
	request projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest,
) ([]authority.AffectedRef, error) {
	head, err := request.Head()
	if err != nil {
		return nil, err
	}
	target := request.Target()
	raw := []string{
		request.Project().String(),
		head.String(),
		target.VerifiedComposite().String(),
		target.Stage().String(),
	}
	switch predecessor := request.Predecessor().(type) {
	case projecttypeenvselection.GenesisStagePredecessor:
	case projecttypeenvselection.TransitionStagePredecessor:
		raw = append(
			raw,
			predecessor.Head().String(),
			predecessor.SelectedComposite().String(),
		)
	default:
		return nil, fmt.Errorf("head-selection predecessor is invalid")
	}
	sort.Strings(raw)
	result := make([]authority.AffectedRef, 0, len(raw))
	for index, value := range raw {
		if index > 0 && value == raw[index-1] {
			continue
		}
		ref, parseErr := authority.NewAffectedRef(value)
		if parseErr != nil {
			return nil, parseErr
		}
		result = append(result, ref)
	}
	return result, nil
}

func verifyRequiredAffectedReferents(
	source authority.VerifiedSpeechActSourceV2,
	required []authority.AffectedRef,
) error {
	affected, ok := source.AffectedRefs()
	if !ok {
		return fmt.Errorf("head-selection SpeechAct affected referents are unavailable")
	}
	available := make(map[string]struct{}, len(affected))
	for _, ref := range affected {
		available[ref.String()] = struct{}{}
	}
	for _, ref := range required {
		if _, exists := available[ref.String()]; !exists {
			return fmt.Errorf(
				"head-selection SpeechAct is missing required affected referent %q",
				ref.String(),
			)
		}
	}
	return nil
}

func projectSpeechActRecord(
	record ProjectTypeEnvHeadSelectionSpeechActRecord,
) (speechActRecordProjection, error) {
	sourceCanonical, canonicalOK := record.source.CanonicalJSON()
	contractCanonical := record.sourceContract.CanonicalJSON()
	bindingCanonical := record.projectBinding.CanonicalJSON()
	sourceDigest, digestOK := record.source.Digest()
	speechAct, speechActOK := record.source.SpeechActRef()
	work, workOK := record.source.WorkRef()
	if !canonicalOK || !digestOK || !speechActOK || !workOK {
		return speechActRecordProjection{}, fmt.Errorf("head-selection SpeechAct v2 source is incomplete")
	}
	affected := make([]string, 0, len(record.requiredAffected))
	for _, ref := range record.requiredAffected {
		affected = append(affected, ref.String())
	}
	content := record.content
	target := content.Request().Target()
	return speechActRecordProjection{
		Schema:                 speechActRecordSchema,
		SourceDigest:           sourceDigest.String(),
		Source:                 json.RawMessage(sourceCanonical),
		SourceContractDigest:   record.sourceContract.Digest().String(),
		SourceContract:         json.RawMessage(contractCanonical),
		ProjectBindingDigest:   record.projectBinding.Digest().String(),
		ProjectBinding:         json.RawMessage(bindingCanonical),
		SpeechActRef:           speechAct.String(),
		WorkRef:                work.String(),
		ContentDescriptionKind: string(content.DescriptionRef().Kind()),
		ContentDescriptionRef:  content.DescriptionRef().String(),
		ContentDigest:          content.Digest().String(),
		PermissionRef:          record.permissionRecord.Ref().String(),
		PermissionRecordDigest: record.permissionRecord.Digest().String(),
		RequestRef:             content.Request().Ref().String(),
		RequestDigest:          requestDigest(content.Request()),
		Project:                content.Project().String(),
		JudgementContext:       content.JudgementContext().String(),
		Action:                 content.Action().String(),
		IntendedUpdate:         projectIntendedUpdate(content.IntendedUpdate()),
		RequiredAffectedRefs:   affected,
		Stage:                  target.Stage().String(),
		VerifiedComposite:      target.VerifiedComposite().String(),
	}, nil
}

func DecodeProjectTypeEnvHeadSelectionSpeechActRecord(
	input ProjectTypeEnvHeadSelectionSpeechActRecordInput,
	canonical []byte,
	digest authority.Digest,
) (ProjectTypeEnvHeadSelectionSpeechActRecord, error) {
	if len(canonical) == 0 || len(canonical) > maximumSpeechActRecordBytes {
		return ProjectTypeEnvHeadSelectionSpeechActRecord{}, fmt.Errorf(
			"head-selection SpeechAct record has invalid canonical size",
		)
	}
	projection := speechActRecordProjection{}
	if err := decodeStrictJSON(canonical, &projection); err != nil {
		return ProjectTypeEnvHeadSelectionSpeechActRecord{}, err
	}
	rebuilt, err := SealProjectTypeEnvHeadSelectionSpeechActRecord(input)
	if err != nil {
		return ProjectTypeEnvHeadSelectionSpeechActRecord{}, err
	}
	if rebuilt.digest != digest || !bytes.Equal(rebuilt.canonicalJSON, canonical) {
		return ProjectTypeEnvHeadSelectionSpeechActRecord{}, fmt.Errorf(
			"head-selection SpeechAct record is not exact canonical material",
		)
	}
	return rebuilt, nil
}

func (record ProjectTypeEnvHeadSelectionSpeechActRecord) Ref() ProjectTypeEnvHeadSelectionSpeechActRecordRef {
	return record.ref
}

func (record ProjectTypeEnvHeadSelectionSpeechActRecord) Digest() authority.Digest {
	return record.digest
}

func (record ProjectTypeEnvHeadSelectionSpeechActRecord) Source() authority.VerifiedSpeechActSourceV2 {
	return record.source
}

func (record ProjectTypeEnvHeadSelectionSpeechActRecord) Content() ProjectTypeEnvHeadSelectionAuthorizationContent {
	return record.content
}

func (record ProjectTypeEnvHeadSelectionSpeechActRecord) PermissionRecord() ProjectTypeEnvHeadSelectionPermissionRecord {
	return record.permissionRecord
}

func (record ProjectTypeEnvHeadSelectionSpeechActRecord) RequiredAffectedRefs() []authority.AffectedRef {
	return append([]authority.AffectedRef(nil), record.requiredAffected...)
}

func (record ProjectTypeEnvHeadSelectionSpeechActRecord) CanonicalJSON() []byte {
	return append([]byte(nil), record.canonicalJSON...)
}

func (record ProjectTypeEnvHeadSelectionSpeechActRecord) Verify(
	request projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest,
) error {
	rebuilt, err := SealProjectTypeEnvHeadSelectionSpeechActRecord(
		ProjectTypeEnvHeadSelectionSpeechActRecordInput{
			Source:         record.source,
			SourceContract: record.sourceContract,
			ProjectBinding: record.projectBinding,
			Content:        record.content,
			Request:        request,
			Stage:          record.content.Stage(),
		},
	)
	if err != nil {
		return err
	}
	if rebuilt.ref != record.ref || rebuilt.digest != record.digest ||
		!bytes.Equal(rebuilt.canonicalJSON, record.canonicalJSON) {
		return fmt.Errorf("head-selection SpeechAct record stored state differs from canonical bytes")
	}
	return nil
}

func (record ProjectTypeEnvHeadSelectionSpeechActRecord) SourceContract() ProjectTypeEnvHeadSelectionSpeechActSourceContract {
	return record.sourceContract
}

func (record ProjectTypeEnvHeadSelectionSpeechActRecord) ProjectBinding() ProjectAuthorityContextBinding {
	return record.projectBinding
}
