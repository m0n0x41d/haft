package projecttypeenvselectioneffect

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	committedResultDomain    = "haft.project-typeenv.head-selection-committed-result.v1"
	committedResultRefPrefix = "project-typeenv-head-selection-result:"

	headSelectionReceiptDomain    = "haft.project-typeenv.head-selection-receipt.v1"
	headSelectionReceiptRefPrefix = "project-typeenv-head-selection-receipt:"
)

type ProjectTypeEnvHeadSelectionCommittedResultRef struct {
	digest typedmemory.SHA256Digest
}

func ParseProjectTypeEnvHeadSelectionCommittedResultRef(
	raw string,
) (ProjectTypeEnvHeadSelectionCommittedResultRef, error) {
	digest, err := parseTypedDigestRef(
		"head-selection committed result",
		committedResultRefPrefix,
		raw,
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionCommittedResultRef{}, err
	}
	return ProjectTypeEnvHeadSelectionCommittedResultRef{digest: digest}, nil
}

func (ref ProjectTypeEnvHeadSelectionCommittedResultRef) Digest() typedmemory.SHA256Digest {
	return ref.digest
}

func (ref ProjectTypeEnvHeadSelectionCommittedResultRef) String() string {
	return committedResultRefPrefix + ref.digest.String()
}

type ProjectTypeEnvHeadSelectionCommittedResult struct {
	ref                    ProjectTypeEnvHeadSelectionCommittedResultRef
	digest                 typedmemory.SHA256Digest
	transactionRef         ProjectTypeEnvHeadSelectionTransactionRef
	transactionDigest      typedmemory.SHA256Digest
	activationRef          ProjectTypeEnvActivationDeltaRef
	activationDigest       typedmemory.SHA256Digest
	manifestRef            ProjectTypeEnvActivationMaterializationManifestRef
	manifestDigest         typedmemory.SHA256Digest
	event                  projecttypeenvselection.GraphEventRef
	commit                 projecttypeenvselection.GraphCommitRef
	materializationDigest  typedmemory.SHA256Digest
	head                   projecttypeenvselection.ProjectTypeEnvHeadState
	headDigest             typedmemory.SHA256Digest
	committedGraphRevision typedmemory.GraphRevision
	canonicalBytes         []byte
}

func SealProjectTypeEnvHeadSelectionCommittedResult(
	activation CommittedProjectTypeEnvActivation,
) (ProjectTypeEnvHeadSelectionCommittedResult, error) {
	if err := activation.Verify(); err != nil {
		return ProjectTypeEnvHeadSelectionCommittedResult{}, err
	}
	writer := newCanonicalWriter(committedResultDomain)
	writer.writeString(activation.Identity().Ref().String())
	writer.writeString(activation.Identity().Digest().String())
	writer.writeString(activation.Delta().Ref().String())
	writer.writeString(activation.Delta().Digest().String())
	writer.writeString(activation.Manifest().Ref().String())
	writer.writeString(activation.Manifest().Digest().String())
	writer.writeString(activation.EventRef().String())
	writer.writeString(activation.CommitRef().String())
	writer.writeString(activation.MaterializationDigest().String())
	writer.writeBytes(activation.SuccessorHead().CanonicalBytes())
	writer.writeString(activation.SuccessorHeadDigest().String())
	writer.writeUint64(activation.Identity().CommittedGraphRevision().Value())
	return DecodeProjectTypeEnvHeadSelectionCommittedResult(writer.bytes())
}

func DecodeProjectTypeEnvHeadSelectionCommittedResult(
	canonical []byte,
) (ProjectTypeEnvHeadSelectionCommittedResult, error) {
	reader, err := newCanonicalReader(canonical, committedResultDomain)
	if err != nil {
		return ProjectTypeEnvHeadSelectionCommittedResult{}, err
	}
	transactionText, err := reader.readString("committed result transaction ref")
	if err != nil {
		return ProjectTypeEnvHeadSelectionCommittedResult{}, err
	}
	transactionRef, err := ParseProjectTypeEnvHeadSelectionTransactionRef(transactionText)
	if err != nil {
		return ProjectTypeEnvHeadSelectionCommittedResult{}, err
	}
	transactionDigestText, err := reader.readString("committed result transaction digest")
	if err != nil {
		return ProjectTypeEnvHeadSelectionCommittedResult{}, err
	}
	transactionDigest, err := typedmemory.NewSHA256Digest(transactionDigestText)
	if err != nil || transactionRef.Digest() != transactionDigest {
		return ProjectTypeEnvHeadSelectionCommittedResult{},
			fmt.Errorf("committed result transaction ref/digest mismatch")
	}
	activationText, err := reader.readString("committed result activation ref")
	if err != nil {
		return ProjectTypeEnvHeadSelectionCommittedResult{}, err
	}
	activationRef, err := ParseProjectTypeEnvActivationDeltaRef(activationText)
	if err != nil {
		return ProjectTypeEnvHeadSelectionCommittedResult{}, err
	}
	activationDigestText, err := reader.readString("committed result activation digest")
	if err != nil {
		return ProjectTypeEnvHeadSelectionCommittedResult{}, err
	}
	activationDigest, err := typedmemory.NewSHA256Digest(activationDigestText)
	if err != nil || activationRef.Digest() != activationDigest {
		return ProjectTypeEnvHeadSelectionCommittedResult{},
			fmt.Errorf("committed result activation ref/digest mismatch")
	}
	manifestText, err := reader.readString("committed result manifest ref")
	if err != nil {
		return ProjectTypeEnvHeadSelectionCommittedResult{}, err
	}
	manifestRef, err := ParseProjectTypeEnvActivationMaterializationManifestRef(
		manifestText,
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionCommittedResult{}, err
	}
	manifestDigestText, err := reader.readString("committed result manifest digest")
	if err != nil {
		return ProjectTypeEnvHeadSelectionCommittedResult{}, err
	}
	manifestDigest, err := typedmemory.NewSHA256Digest(manifestDigestText)
	if err != nil || manifestRef.Digest() != manifestDigest {
		return ProjectTypeEnvHeadSelectionCommittedResult{},
			fmt.Errorf("committed result manifest ref/digest mismatch")
	}
	eventText, err := reader.readString("committed result event ref")
	if err != nil {
		return ProjectTypeEnvHeadSelectionCommittedResult{}, err
	}
	event, err := projecttypeenvselection.ParseGraphEventRef(eventText)
	if err != nil {
		return ProjectTypeEnvHeadSelectionCommittedResult{}, err
	}
	commitText, err := reader.readString("committed result commit ref")
	if err != nil {
		return ProjectTypeEnvHeadSelectionCommittedResult{}, err
	}
	commit, err := projecttypeenvselection.ParseGraphCommitRef(commitText)
	if err != nil {
		return ProjectTypeEnvHeadSelectionCommittedResult{}, err
	}
	materializationText, err := reader.readString("committed result materialization digest")
	if err != nil {
		return ProjectTypeEnvHeadSelectionCommittedResult{}, err
	}
	materializationDigest, err := typedmemory.NewSHA256Digest(materializationText)
	if err != nil {
		return ProjectTypeEnvHeadSelectionCommittedResult{}, err
	}
	headBytes, err := reader.readBytes("committed result head state")
	if err != nil {
		return ProjectTypeEnvHeadSelectionCommittedResult{}, err
	}
	head, err := projecttypeenvselection.DecodeProjectTypeEnvHeadState(headBytes)
	if err != nil {
		return ProjectTypeEnvHeadSelectionCommittedResult{}, err
	}
	headDigestText, err := reader.readString("committed result head digest")
	if err != nil {
		return ProjectTypeEnvHeadSelectionCommittedResult{}, err
	}
	headDigest, err := typedmemory.NewSHA256Digest(headDigestText)
	if err != nil {
		return ProjectTypeEnvHeadSelectionCommittedResult{}, err
	}
	recomputedHeadDigest, err := digestRaw(headBytes)
	if err != nil || recomputedHeadDigest != headDigest {
		return ProjectTypeEnvHeadSelectionCommittedResult{},
			fmt.Errorf("committed result head digest mismatch")
	}
	graphRevisionValue, err := reader.readUint64("committed result GraphRevision")
	if err != nil || graphRevisionValue == 0 {
		return ProjectTypeEnvHeadSelectionCommittedResult{},
			fmt.Errorf("committed result GraphRevision is invalid")
	}
	if err := reader.requireEnd("head-selection committed result"); err != nil {
		return ProjectTypeEnvHeadSelectionCommittedResult{}, err
	}
	digest, err := digestCanonical(committedResultDomain, canonical)
	if err != nil {
		return ProjectTypeEnvHeadSelectionCommittedResult{}, err
	}
	return ProjectTypeEnvHeadSelectionCommittedResult{
		ref:                    ProjectTypeEnvHeadSelectionCommittedResultRef{digest: digest},
		digest:                 digest,
		transactionRef:         transactionRef,
		transactionDigest:      transactionDigest,
		activationRef:          activationRef,
		activationDigest:       activationDigest,
		manifestRef:            manifestRef,
		manifestDigest:         manifestDigest,
		event:                  event,
		commit:                 commit,
		materializationDigest:  materializationDigest,
		head:                   head,
		headDigest:             headDigest,
		committedGraphRevision: typedmemory.NewGraphRevision(graphRevisionValue),
		canonicalBytes:         append([]byte(nil), canonical...),
	}, nil
}

func (value ProjectTypeEnvHeadSelectionCommittedResult) Ref() ProjectTypeEnvHeadSelectionCommittedResultRef {
	return value.ref
}

func (value ProjectTypeEnvHeadSelectionCommittedResult) Digest() typedmemory.SHA256Digest {
	return value.digest
}

func (value ProjectTypeEnvHeadSelectionCommittedResult) TransactionRef() ProjectTypeEnvHeadSelectionTransactionRef {
	return value.transactionRef
}

func (value ProjectTypeEnvHeadSelectionCommittedResult) TransactionDigest() typedmemory.SHA256Digest {
	return value.transactionDigest
}

func (value ProjectTypeEnvHeadSelectionCommittedResult) ActivationRef() ProjectTypeEnvActivationDeltaRef {
	return value.activationRef
}

func (value ProjectTypeEnvHeadSelectionCommittedResult) ActivationDigest() typedmemory.SHA256Digest {
	return value.activationDigest
}

func (value ProjectTypeEnvHeadSelectionCommittedResult) ManifestRef() ProjectTypeEnvActivationMaterializationManifestRef {
	return value.manifestRef
}

func (value ProjectTypeEnvHeadSelectionCommittedResult) ManifestDigest() typedmemory.SHA256Digest {
	return value.manifestDigest
}

func (value ProjectTypeEnvHeadSelectionCommittedResult) EventRef() projecttypeenvselection.GraphEventRef {
	return value.event
}

func (value ProjectTypeEnvHeadSelectionCommittedResult) CommitRef() projecttypeenvselection.GraphCommitRef {
	return value.commit
}

func (value ProjectTypeEnvHeadSelectionCommittedResult) MaterializationDigest() typedmemory.SHA256Digest {
	return value.materializationDigest
}

func (value ProjectTypeEnvHeadSelectionCommittedResult) Head() projecttypeenvselection.ProjectTypeEnvHeadState {
	return value.head
}

func (value ProjectTypeEnvHeadSelectionCommittedResult) HeadDigest() typedmemory.SHA256Digest {
	return value.headDigest
}

func (value ProjectTypeEnvHeadSelectionCommittedResult) CommittedGraphRevision() typedmemory.GraphRevision {
	return value.committedGraphRevision
}

func (value ProjectTypeEnvHeadSelectionCommittedResult) CanonicalBytes() []byte {
	return append([]byte(nil), value.canonicalBytes...)
}

func (value ProjectTypeEnvHeadSelectionCommittedResult) Verify() error {
	decoded, err := DecodeProjectTypeEnvHeadSelectionCommittedResult(
		value.canonicalBytes,
	)
	if err != nil {
		return err
	}
	if decoded.ref != value.ref || decoded.digest != value.digest {
		return fmt.Errorf("committed result differs from canonical bytes")
	}
	return nil
}

type ProjectTypeEnvHeadSelectionReceiptRef struct {
	digest typedmemory.SHA256Digest
}

func ParseProjectTypeEnvHeadSelectionReceiptRef(
	raw string,
) (ProjectTypeEnvHeadSelectionReceiptRef, error) {
	digest, err := parseTypedDigestRef(
		"head-selection receipt",
		headSelectionReceiptRefPrefix,
		raw,
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionReceiptRef{}, err
	}
	return ProjectTypeEnvHeadSelectionReceiptRef{digest: digest}, nil
}

func (ref ProjectTypeEnvHeadSelectionReceiptRef) Digest() typedmemory.SHA256Digest {
	return ref.digest
}

func (ref ProjectTypeEnvHeadSelectionReceiptRef) String() string {
	return headSelectionReceiptRefPrefix + ref.digest.String()
}

// ProjectTypeEnvHeadSelectionReceiptV1 contains stable authority-use and Work
// record refs, but not their later content digests. The final closure supplies
// those member digests and therefore breaks the otherwise cyclic address graph.
type ProjectTypeEnvHeadSelectionReceiptV1 struct {
	ref                    ProjectTypeEnvHeadSelectionReceiptRef
	digest                 typedmemory.SHA256Digest
	transactionRef         ProjectTypeEnvHeadSelectionTransactionRef
	transactionDigest      typedmemory.SHA256Digest
	project                projectidentity.ProjectID
	idempotencyKey         projecttypeenvselection.ProjectTypeEnvHeadSelectionIdempotencyKey
	requestRef             projecttypeenvselection.ProjectTypeEnvHeadSelectionRequestRef
	requestDigest          typedmemory.SHA256Digest
	authority              ProjectTypeEnvHeadSelectionAuthorityCoordinates
	authorityUseRef        ProjectTypeEnvHeadSelectionAuthorityUseRecordRef
	workRef                authority.WorkRef
	workRecordRef          ProjectTypeEnvHeadCASWorkRecordRef
	predecessor            projecttypeenvselection.ProjectTypeEnvHeadSelectionPredecessor
	target                 ProjectTypeEnvHeadSelectionTarget
	expectedGraphRevision  typedmemory.GraphRevision
	committedGraphRevision typedmemory.GraphRevision
	successorHead          projecttypeenvselection.ProjectTypeEnvHeadState
	successorHeadDigest    typedmemory.SHA256Digest
	event                  projecttypeenvselection.GraphEventRef
	commit                 projecttypeenvselection.GraphCommitRef
	materializationDigest  typedmemory.SHA256Digest
	committedResultRef     ProjectTypeEnvHeadSelectionCommittedResultRef
	committedResultDigest  typedmemory.SHA256Digest
	canonicalBytes         []byte
}

type ProjectTypeEnvHeadSelectionReceiptInput struct {
	Identity     ProjectTypeEnvHeadSelectionTransactionIdentity
	ReferenceDAG ProjectTypeEnvHeadSelectionReferenceDAG
	Authority    ProjectTypeEnvHeadSelectionAuthorityCoordinates
	Activation   CommittedProjectTypeEnvActivation
	Result       ProjectTypeEnvHeadSelectionCommittedResult
}

func SealProjectTypeEnvHeadSelectionReceiptV1(
	input ProjectTypeEnvHeadSelectionReceiptInput,
) (ProjectTypeEnvHeadSelectionReceiptV1, error) {
	if err := input.Identity.Verify(); err != nil {
		return ProjectTypeEnvHeadSelectionReceiptV1{}, err
	}
	if err := input.ReferenceDAG.Verify(input.Identity); err != nil {
		return ProjectTypeEnvHeadSelectionReceiptV1{}, err
	}
	if err := input.Activation.Verify(); err != nil {
		return ProjectTypeEnvHeadSelectionReceiptV1{}, err
	}
	if err := input.Result.Verify(); err != nil {
		return ProjectTypeEnvHeadSelectionReceiptV1{}, err
	}
	if input.Authority.ContentDigest() != input.Identity.ContentDigest() ||
		input.Activation.Identity().Ref() != input.Identity.Ref() ||
		input.Activation.Identity().Digest() != input.Identity.Digest() ||
		input.Result.TransactionRef() != input.Identity.Ref() ||
		input.Result.TransactionDigest() != input.Identity.Digest() ||
		input.Result.ActivationRef() != input.Activation.Delta().Ref() ||
		input.Result.ActivationDigest() != input.Activation.Delta().Digest() ||
		input.Result.ManifestRef() != input.Activation.Manifest().Ref() ||
		input.Result.ManifestDigest() != input.Activation.Manifest().Digest() ||
		input.Result.HeadDigest() != input.Activation.SuccessorHeadDigest() {
		return ProjectTypeEnvHeadSelectionReceiptV1{},
			fmt.Errorf("receipt members do not describe one exact transaction")
	}
	writer := newCanonicalWriter(headSelectionReceiptDomain)
	writer.writeString(input.Identity.Ref().String())
	writer.writeString(input.Identity.Digest().String())
	writer.writeString(input.Identity.Project().String())
	writer.writeString(input.Identity.IdempotencyKey().String())
	writer.writeString(input.Identity.RequestRef().String())
	writer.writeString(input.Identity.RequestDigest().String())
	encodeAuthorityCoordinates(&writer, input.Authority)
	writer.writeString(input.ReferenceDAG.AuthorityUseRecordRef().String())
	writer.writeString(input.ReferenceDAG.WorkRef().String())
	writer.writeString(input.ReferenceDAG.CASWorkRecordRef().String())
	encodePredecessor(&writer, input.Activation.Delta().Predecessor())
	encodeTarget(&writer, input.Activation.Delta().Target())
	writer.writeUint64(input.Activation.Delta().ExpectedGraphRevision().Value())
	writer.writeUint64(input.Identity.CommittedGraphRevision().Value())
	writer.writeBytes(input.Activation.SuccessorHead().CanonicalBytes())
	writer.writeString(input.Activation.SuccessorHeadDigest().String())
	writer.writeString(input.Activation.EventRef().String())
	writer.writeString(input.Activation.CommitRef().String())
	writer.writeString(input.Activation.MaterializationDigest().String())
	writer.writeString(input.Result.Ref().String())
	writer.writeString(input.Result.Digest().String())
	return DecodeProjectTypeEnvHeadSelectionReceiptV1(writer.bytes())
}

func DecodeProjectTypeEnvHeadSelectionReceiptV1(
	canonical []byte,
) (ProjectTypeEnvHeadSelectionReceiptV1, error) {
	reader, err := newCanonicalReader(canonical, headSelectionReceiptDomain)
	if err != nil {
		return ProjectTypeEnvHeadSelectionReceiptV1{}, err
	}
	transactionText, err := reader.readString("receipt transaction ref")
	if err != nil {
		return ProjectTypeEnvHeadSelectionReceiptV1{}, err
	}
	transactionRef, err := ParseProjectTypeEnvHeadSelectionTransactionRef(transactionText)
	if err != nil {
		return ProjectTypeEnvHeadSelectionReceiptV1{}, err
	}
	transactionDigestText, err := reader.readString("receipt transaction digest")
	if err != nil {
		return ProjectTypeEnvHeadSelectionReceiptV1{}, err
	}
	transactionDigest, err := typedmemory.NewSHA256Digest(transactionDigestText)
	if err != nil || transactionRef.Digest() != transactionDigest {
		return ProjectTypeEnvHeadSelectionReceiptV1{},
			fmt.Errorf("receipt transaction ref/digest mismatch")
	}
	projectText, err := reader.readString("receipt project")
	if err != nil {
		return ProjectTypeEnvHeadSelectionReceiptV1{}, err
	}
	project, err := projectidentity.ParseProjectID(projectText)
	if err != nil {
		return ProjectTypeEnvHeadSelectionReceiptV1{}, err
	}
	keyText, err := reader.readString("receipt idempotency key")
	if err != nil {
		return ProjectTypeEnvHeadSelectionReceiptV1{}, err
	}
	key, err := projecttypeenvselection.NewProjectTypeEnvHeadSelectionIdempotencyKey(
		keyText,
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionReceiptV1{}, err
	}
	requestText, err := reader.readString("receipt request ref")
	if err != nil {
		return ProjectTypeEnvHeadSelectionReceiptV1{}, err
	}
	requestRef, err := projecttypeenvselection.ParseProjectTypeEnvHeadSelectionRequestRef(
		requestText,
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionReceiptV1{}, err
	}
	requestDigestText, err := reader.readString("receipt request digest")
	if err != nil {
		return ProjectTypeEnvHeadSelectionReceiptV1{}, err
	}
	requestDigest, err := typedmemory.NewSHA256Digest(requestDigestText)
	if err != nil || requestRef.Digest() != requestDigest {
		return ProjectTypeEnvHeadSelectionReceiptV1{},
			fmt.Errorf("receipt request ref/digest mismatch")
	}
	authorityCoordinates, err := decodeAuthorityCoordinates(reader)
	if err != nil {
		return ProjectTypeEnvHeadSelectionReceiptV1{}, err
	}
	useText, err := reader.readString("receipt authority-use ref")
	if err != nil {
		return ProjectTypeEnvHeadSelectionReceiptV1{}, err
	}
	useRef, err := ParseProjectTypeEnvHeadSelectionAuthorityUseRecordRef(useText)
	if err != nil {
		return ProjectTypeEnvHeadSelectionReceiptV1{}, err
	}
	workText, err := reader.readString("receipt Work ref")
	if err != nil {
		return ProjectTypeEnvHeadSelectionReceiptV1{}, err
	}
	workRef, err := authority.NewWorkRef(workText)
	if err != nil {
		return ProjectTypeEnvHeadSelectionReceiptV1{}, err
	}
	workRecordText, err := reader.readString("receipt Work-record ref")
	if err != nil {
		return ProjectTypeEnvHeadSelectionReceiptV1{}, err
	}
	workRecordRef, err := ParseProjectTypeEnvHeadCASWorkRecordRef(workRecordText)
	if err != nil {
		return ProjectTypeEnvHeadSelectionReceiptV1{}, err
	}
	predecessor, err := decodePredecessor(reader, project)
	if err != nil {
		return ProjectTypeEnvHeadSelectionReceiptV1{}, err
	}
	target, err := decodeTarget(reader)
	if err != nil {
		return ProjectTypeEnvHeadSelectionReceiptV1{}, err
	}
	expectedValue, err := reader.readUint64("receipt expected GraphRevision")
	if err != nil {
		return ProjectTypeEnvHeadSelectionReceiptV1{}, err
	}
	committedValue, err := reader.readUint64("receipt committed GraphRevision")
	if err != nil || committedValue == 0 || committedValue-1 != expectedValue {
		return ProjectTypeEnvHeadSelectionReceiptV1{},
			fmt.Errorf("receipt GraphRevision pair is not contiguous")
	}
	headBytes, err := reader.readBytes("receipt successor head")
	if err != nil {
		return ProjectTypeEnvHeadSelectionReceiptV1{}, err
	}
	head, err := projecttypeenvselection.DecodeProjectTypeEnvHeadState(headBytes)
	if err != nil || head.Project() != project || head.SelectedComposite() != target.Composite() {
		return ProjectTypeEnvHeadSelectionReceiptV1{},
			fmt.Errorf("receipt successor head is invalid")
	}
	headDigestText, err := reader.readString("receipt successor head digest")
	if err != nil {
		return ProjectTypeEnvHeadSelectionReceiptV1{}, err
	}
	headDigest, err := typedmemory.NewSHA256Digest(headDigestText)
	if err != nil {
		return ProjectTypeEnvHeadSelectionReceiptV1{}, err
	}
	recomputedHeadDigest, err := digestRaw(headBytes)
	if err != nil || recomputedHeadDigest != headDigest {
		return ProjectTypeEnvHeadSelectionReceiptV1{},
			fmt.Errorf("receipt successor head digest mismatch")
	}
	eventText, err := reader.readString("receipt event ref")
	if err != nil {
		return ProjectTypeEnvHeadSelectionReceiptV1{}, err
	}
	event, err := projecttypeenvselection.ParseGraphEventRef(eventText)
	if err != nil {
		return ProjectTypeEnvHeadSelectionReceiptV1{}, err
	}
	commitText, err := reader.readString("receipt commit ref")
	if err != nil {
		return ProjectTypeEnvHeadSelectionReceiptV1{}, err
	}
	commit, err := projecttypeenvselection.ParseGraphCommitRef(commitText)
	if err != nil {
		return ProjectTypeEnvHeadSelectionReceiptV1{}, err
	}
	materializationText, err := reader.readString("receipt materialization digest")
	if err != nil {
		return ProjectTypeEnvHeadSelectionReceiptV1{}, err
	}
	materializationDigest, err := typedmemory.NewSHA256Digest(materializationText)
	if err != nil {
		return ProjectTypeEnvHeadSelectionReceiptV1{}, err
	}
	resultText, err := reader.readString("receipt committed-result ref")
	if err != nil {
		return ProjectTypeEnvHeadSelectionReceiptV1{}, err
	}
	resultRef, err := ParseProjectTypeEnvHeadSelectionCommittedResultRef(resultText)
	if err != nil {
		return ProjectTypeEnvHeadSelectionReceiptV1{}, err
	}
	resultDigestText, err := reader.readString("receipt committed-result digest")
	if err != nil {
		return ProjectTypeEnvHeadSelectionReceiptV1{}, err
	}
	resultDigest, err := typedmemory.NewSHA256Digest(resultDigestText)
	if err != nil || resultRef.Digest() != resultDigest {
		return ProjectTypeEnvHeadSelectionReceiptV1{},
			fmt.Errorf("receipt committed-result ref/digest mismatch")
	}
	if err := reader.requireEnd("head-selection receipt"); err != nil {
		return ProjectTypeEnvHeadSelectionReceiptV1{}, err
	}
	digest, err := digestCanonical(headSelectionReceiptDomain, canonical)
	if err != nil {
		return ProjectTypeEnvHeadSelectionReceiptV1{}, err
	}
	return ProjectTypeEnvHeadSelectionReceiptV1{
		ref:                    ProjectTypeEnvHeadSelectionReceiptRef{digest: digest},
		digest:                 digest,
		transactionRef:         transactionRef,
		transactionDigest:      transactionDigest,
		project:                project,
		idempotencyKey:         key,
		requestRef:             requestRef,
		requestDigest:          requestDigest,
		authority:              authorityCoordinates,
		authorityUseRef:        useRef,
		workRef:                workRef,
		workRecordRef:          workRecordRef,
		predecessor:            predecessor,
		target:                 target,
		expectedGraphRevision:  typedmemory.NewGraphRevision(expectedValue),
		committedGraphRevision: typedmemory.NewGraphRevision(committedValue),
		successorHead:          head,
		successorHeadDigest:    headDigest,
		event:                  event,
		commit:                 commit,
		materializationDigest:  materializationDigest,
		committedResultRef:     resultRef,
		committedResultDigest:  resultDigest,
		canonicalBytes:         append([]byte(nil), canonical...),
	}, nil
}

func (value ProjectTypeEnvHeadSelectionReceiptV1) Ref() ProjectTypeEnvHeadSelectionReceiptRef {
	return value.ref
}

func (value ProjectTypeEnvHeadSelectionReceiptV1) Digest() typedmemory.SHA256Digest {
	return value.digest
}

func (value ProjectTypeEnvHeadSelectionReceiptV1) TransactionRef() ProjectTypeEnvHeadSelectionTransactionRef {
	return value.transactionRef
}

func (value ProjectTypeEnvHeadSelectionReceiptV1) TransactionDigest() typedmemory.SHA256Digest {
	return value.transactionDigest
}

func (value ProjectTypeEnvHeadSelectionReceiptV1) Project() projectidentity.ProjectID {
	return value.project
}

func (value ProjectTypeEnvHeadSelectionReceiptV1) IdempotencyKey() projecttypeenvselection.ProjectTypeEnvHeadSelectionIdempotencyKey {
	return value.idempotencyKey
}

func (value ProjectTypeEnvHeadSelectionReceiptV1) RequestRef() projecttypeenvselection.ProjectTypeEnvHeadSelectionRequestRef {
	return value.requestRef
}

func (value ProjectTypeEnvHeadSelectionReceiptV1) RequestDigest() typedmemory.SHA256Digest {
	return value.requestDigest
}

func (value ProjectTypeEnvHeadSelectionReceiptV1) AuthorityCoordinates() ProjectTypeEnvHeadSelectionAuthorityCoordinates {
	return value.authority
}

func (value ProjectTypeEnvHeadSelectionReceiptV1) AuthorityUseRecordRef() ProjectTypeEnvHeadSelectionAuthorityUseRecordRef {
	return value.authorityUseRef
}

func (value ProjectTypeEnvHeadSelectionReceiptV1) WorkRef() authority.WorkRef {
	return value.workRef
}

func (value ProjectTypeEnvHeadSelectionReceiptV1) CASWorkRecordRef() ProjectTypeEnvHeadCASWorkRecordRef {
	return value.workRecordRef
}

func (value ProjectTypeEnvHeadSelectionReceiptV1) Predecessor() projecttypeenvselection.ProjectTypeEnvHeadSelectionPredecessor {
	return value.predecessor
}

func (value ProjectTypeEnvHeadSelectionReceiptV1) Target() ProjectTypeEnvHeadSelectionTarget {
	return value.target
}

func (value ProjectTypeEnvHeadSelectionReceiptV1) ExpectedGraphRevision() typedmemory.GraphRevision {
	return value.expectedGraphRevision
}

func (value ProjectTypeEnvHeadSelectionReceiptV1) CommittedGraphRevision() typedmemory.GraphRevision {
	return value.committedGraphRevision
}

func (value ProjectTypeEnvHeadSelectionReceiptV1) SuccessorHead() projecttypeenvselection.ProjectTypeEnvHeadState {
	return value.successorHead
}

func (value ProjectTypeEnvHeadSelectionReceiptV1) SuccessorHeadDigest() typedmemory.SHA256Digest {
	return value.successorHeadDigest
}

func (value ProjectTypeEnvHeadSelectionReceiptV1) EventRef() projecttypeenvselection.GraphEventRef {
	return value.event
}

func (value ProjectTypeEnvHeadSelectionReceiptV1) CommitRef() projecttypeenvselection.GraphCommitRef {
	return value.commit
}

func (value ProjectTypeEnvHeadSelectionReceiptV1) MaterializationDigest() typedmemory.SHA256Digest {
	return value.materializationDigest
}

func (value ProjectTypeEnvHeadSelectionReceiptV1) CommittedResultRef() ProjectTypeEnvHeadSelectionCommittedResultRef {
	return value.committedResultRef
}

func (value ProjectTypeEnvHeadSelectionReceiptV1) CommittedResultDigest() typedmemory.SHA256Digest {
	return value.committedResultDigest
}

func (value ProjectTypeEnvHeadSelectionReceiptV1) CanonicalBytes() []byte {
	return append([]byte(nil), value.canonicalBytes...)
}

func (value ProjectTypeEnvHeadSelectionReceiptV1) Verify() error {
	decoded, err := DecodeProjectTypeEnvHeadSelectionReceiptV1(value.canonicalBytes)
	if err != nil {
		return err
	}
	if decoded.ref != value.ref || decoded.digest != value.digest {
		return fmt.Errorf("head-selection receipt differs from canonical bytes")
	}
	return nil
}
