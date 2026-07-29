package projecttypeenvselectioneffect

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	headSelectionClosureDomain    = "haft.project-typeenv.head-selection-closure.v1"
	headSelectionClosureRefPrefix = "project-typeenv-head-selection-closure:"
)

type ProjectTypeEnvHeadSelectionClosureRef struct {
	digest typedmemory.SHA256Digest
}

func ParseProjectTypeEnvHeadSelectionClosureRef(
	raw string,
) (ProjectTypeEnvHeadSelectionClosureRef, error) {
	digest, err := parseTypedDigestRef(
		"head-selection closure",
		headSelectionClosureRefPrefix,
		raw,
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionClosureRef{}, err
	}
	return ProjectTypeEnvHeadSelectionClosureRef{digest: digest}, nil
}

func (ref ProjectTypeEnvHeadSelectionClosureRef) Digest() typedmemory.SHA256Digest {
	return ref.digest
}

func (ref ProjectTypeEnvHeadSelectionClosureRef) String() string {
	return headSelectionClosureRefPrefix + ref.digest.String()
}

// ProjectTypeEnvHeadSelectionClosureV1 is the final content-addressed root.
// No member contains ClosureRef, so its member graph is cycle-free.
type ProjectTypeEnvHeadSelectionClosureV1 struct {
	ref                    ProjectTypeEnvHeadSelectionClosureRef
	digest                 typedmemory.SHA256Digest
	transactionRef         ProjectTypeEnvHeadSelectionTransactionRef
	transactionDigest      typedmemory.SHA256Digest
	project                projectidentity.ProjectID
	idempotencyKey         projecttypeenvselection.ProjectTypeEnvHeadSelectionIdempotencyKey
	requestRef             projecttypeenvselection.ProjectTypeEnvHeadSelectionRequestRef
	requestDigest          typedmemory.SHA256Digest
	authority              ProjectTypeEnvHeadSelectionAuthorityCoordinates
	authorityUseRef        ProjectTypeEnvHeadSelectionAuthorityUseRecordRef
	authorityUseDigest     typedmemory.SHA256Digest
	workRef                authority.WorkRef
	workRecordRef          ProjectTypeEnvHeadCASWorkRecordRef
	workRecordDigest       typedmemory.SHA256Digest
	predecessor            projecttypeenvselection.ProjectTypeEnvHeadSelectionPredecessor
	target                 ProjectTypeEnvHeadSelectionTarget
	expectedGraphRevision  typedmemory.GraphRevision
	committedGraphRevision typedmemory.GraphRevision
	successorHead          projecttypeenvselection.ProjectTypeEnvHeadState
	successorHeadDigest    typedmemory.SHA256Digest
	deltaRef               ProjectTypeEnvActivationDeltaRef
	deltaDigest            typedmemory.SHA256Digest
	envelopeRef            ProjectTypeEnvActivationAdmissionEnvelopeRef
	envelopeDigest         typedmemory.SHA256Digest
	basisRef               ProjectTypeEnvActivationAdmissionBasisRef
	basisDigest            typedmemory.SHA256Digest
	manifestRef            ProjectTypeEnvActivationMaterializationManifestRef
	manifestDigest         typedmemory.SHA256Digest
	event                  projecttypeenvselection.GraphEventRef
	commit                 projecttypeenvselection.GraphCommitRef
	materializationDigest  typedmemory.SHA256Digest
	receiptRef             ProjectTypeEnvHeadSelectionReceiptRef
	receiptDigest          typedmemory.SHA256Digest
	resultRef              ProjectTypeEnvHeadSelectionCommittedResultRef
	resultDigest           typedmemory.SHA256Digest
	canonicalBytes         []byte
}

type ProjectTypeEnvHeadSelectionClosureInput struct {
	Identity     ProjectTypeEnvHeadSelectionTransactionIdentity
	ReferenceDAG ProjectTypeEnvHeadSelectionReferenceDAG
	Activation   CommittedProjectTypeEnvActivation
	Result       ProjectTypeEnvHeadSelectionCommittedResult
	Receipt      ProjectTypeEnvHeadSelectionReceiptV1
	AuthorityUse ProjectTypeEnvHeadSelectionAuthorityUseRecord
	CASWork      ProjectTypeEnvHeadCASWorkRecord
}

func SealProjectTypeEnvHeadSelectionClosureV1(
	input ProjectTypeEnvHeadSelectionClosureInput,
) (ProjectTypeEnvHeadSelectionClosureV1, error) {
	if err := input.Identity.Verify(); err != nil {
		return ProjectTypeEnvHeadSelectionClosureV1{}, err
	}
	if err := input.ReferenceDAG.Verify(input.Identity); err != nil {
		return ProjectTypeEnvHeadSelectionClosureV1{}, err
	}
	if err := input.Activation.Verify(); err != nil {
		return ProjectTypeEnvHeadSelectionClosureV1{}, err
	}
	if err := input.Result.Verify(); err != nil {
		return ProjectTypeEnvHeadSelectionClosureV1{}, err
	}
	if err := input.Receipt.Verify(); err != nil {
		return ProjectTypeEnvHeadSelectionClosureV1{}, err
	}
	if err := input.AuthorityUse.Verify(); err != nil {
		return ProjectTypeEnvHeadSelectionClosureV1{}, err
	}
	if err := input.CASWork.Verify(); err != nil {
		return ProjectTypeEnvHeadSelectionClosureV1{}, err
	}
	if input.Activation.Identity().Ref() != input.Identity.Ref() ||
		input.Result.TransactionRef() != input.Identity.Ref() ||
		input.Receipt.TransactionRef() != input.Identity.Ref() ||
		input.Receipt.AuthorityUseRecordRef() != input.AuthorityUse.Ref() ||
		input.Receipt.WorkRef() != input.CASWork.WorkRef() ||
		input.Receipt.CASWorkRecordRef() != input.CASWork.Ref() ||
		input.AuthorityUse.WorkRef() != input.CASWork.WorkRef() ||
		input.AuthorityUse.ReceiptRef() != input.Receipt.Ref() ||
		input.CASWork.AuthorityUseRecordRef() != input.AuthorityUse.Ref() ||
		input.CASWork.ReceiptRef() != input.Receipt.Ref() ||
		input.Result.Ref() != input.Receipt.CommittedResultRef() ||
		input.Activation.Delta().Ref() != input.Result.ActivationRef() ||
		input.Activation.Manifest().Ref() != input.Result.ManifestRef() {
		return ProjectTypeEnvHeadSelectionClosureV1{},
			fmt.Errorf("closure members do not describe one exact transaction")
	}
	writer := newCanonicalWriter(headSelectionClosureDomain)
	writer.writeString(input.Identity.Ref().String())
	writer.writeString(input.Identity.Digest().String())
	writer.writeString(input.Identity.Project().String())
	writer.writeString(input.Identity.IdempotencyKey().String())
	writer.writeString(input.Identity.RequestRef().String())
	writer.writeString(input.Identity.RequestDigest().String())
	encodeAuthorityCoordinates(&writer, input.Receipt.AuthorityCoordinates())
	writer.writeString(input.AuthorityUse.Ref().String())
	writer.writeString(input.AuthorityUse.Digest().String())
	writer.writeString(input.CASWork.WorkRef().String())
	writer.writeString(input.CASWork.Ref().String())
	writer.writeString(input.CASWork.Digest().String())
	encodePredecessor(&writer, input.Receipt.Predecessor())
	encodeTarget(&writer, input.Receipt.Target())
	writer.writeUint64(input.Receipt.ExpectedGraphRevision().Value())
	writer.writeUint64(input.Receipt.CommittedGraphRevision().Value())
	writer.writeBytes(input.Receipt.SuccessorHead().CanonicalBytes())
	writer.writeString(input.Receipt.SuccessorHeadDigest().String())
	writer.writeString(input.Activation.Delta().Ref().String())
	writer.writeString(input.Activation.Delta().Digest().String())
	writer.writeString(input.Activation.Envelope().Ref().String())
	writer.writeString(input.Activation.Envelope().Digest().String())
	writer.writeString(input.Activation.Basis().Ref().String())
	writer.writeString(input.Activation.Basis().Digest().String())
	writer.writeString(input.Activation.Manifest().Ref().String())
	writer.writeString(input.Activation.Manifest().Digest().String())
	writer.writeString(input.Activation.EventRef().String())
	writer.writeString(input.Activation.CommitRef().String())
	writer.writeString(input.Activation.MaterializationDigest().String())
	writer.writeString(input.Receipt.Ref().String())
	writer.writeString(input.Receipt.Digest().String())
	writer.writeString(input.Result.Ref().String())
	writer.writeString(input.Result.Digest().String())
	return DecodeProjectTypeEnvHeadSelectionClosureV1(writer.bytes())
}

func DecodeProjectTypeEnvHeadSelectionClosureV1(
	canonical []byte,
) (ProjectTypeEnvHeadSelectionClosureV1, error) {
	reader, err := newCanonicalReader(canonical, headSelectionClosureDomain)
	if err != nil {
		return ProjectTypeEnvHeadSelectionClosureV1{}, err
	}
	transactionText, err := reader.readString("closure transaction ref")
	if err != nil {
		return ProjectTypeEnvHeadSelectionClosureV1{}, err
	}
	transactionRef, err := ParseProjectTypeEnvHeadSelectionTransactionRef(
		transactionText,
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionClosureV1{}, err
	}
	transactionDigest, err := readTypedDigest(reader, "closure transaction digest")
	if err != nil || transactionRef.Digest() != transactionDigest {
		return ProjectTypeEnvHeadSelectionClosureV1{},
			fmt.Errorf("closure transaction ref/digest mismatch")
	}
	projectText, err := reader.readString("closure project")
	if err != nil {
		return ProjectTypeEnvHeadSelectionClosureV1{}, err
	}
	project, err := projectidentity.ParseProjectID(projectText)
	if err != nil {
		return ProjectTypeEnvHeadSelectionClosureV1{}, err
	}
	keyText, err := reader.readString("closure idempotency key")
	if err != nil {
		return ProjectTypeEnvHeadSelectionClosureV1{}, err
	}
	key, err := projecttypeenvselection.NewProjectTypeEnvHeadSelectionIdempotencyKey(
		keyText,
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionClosureV1{}, err
	}
	requestText, err := reader.readString("closure request ref")
	if err != nil {
		return ProjectTypeEnvHeadSelectionClosureV1{}, err
	}
	requestRef, err := projecttypeenvselection.ParseProjectTypeEnvHeadSelectionRequestRef(
		requestText,
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionClosureV1{}, err
	}
	requestDigest, err := readTypedDigest(reader, "closure request digest")
	if err != nil || requestRef.Digest() != requestDigest {
		return ProjectTypeEnvHeadSelectionClosureV1{},
			fmt.Errorf("closure request ref/digest mismatch")
	}
	coordinates, err := decodeAuthorityCoordinates(reader)
	if err != nil {
		return ProjectTypeEnvHeadSelectionClosureV1{}, err
	}
	useText, err := reader.readString("closure authority-use ref")
	if err != nil {
		return ProjectTypeEnvHeadSelectionClosureV1{}, err
	}
	useRef, err := ParseProjectTypeEnvHeadSelectionAuthorityUseRecordRef(useText)
	if err != nil {
		return ProjectTypeEnvHeadSelectionClosureV1{}, err
	}
	useDigest, err := readTypedDigest(reader, "closure authority-use digest")
	if err != nil {
		return ProjectTypeEnvHeadSelectionClosureV1{}, err
	}
	workText, err := reader.readString("closure Work ref")
	if err != nil {
		return ProjectTypeEnvHeadSelectionClosureV1{}, err
	}
	workRef, err := authority.NewWorkRef(workText)
	if err != nil {
		return ProjectTypeEnvHeadSelectionClosureV1{}, err
	}
	workRecordText, err := reader.readString("closure Work-record ref")
	if err != nil {
		return ProjectTypeEnvHeadSelectionClosureV1{}, err
	}
	workRecordRef, err := ParseProjectTypeEnvHeadCASWorkRecordRef(workRecordText)
	if err != nil {
		return ProjectTypeEnvHeadSelectionClosureV1{}, err
	}
	workRecordDigest, err := readTypedDigest(reader, "closure Work-record digest")
	if err != nil {
		return ProjectTypeEnvHeadSelectionClosureV1{}, err
	}
	predecessor, err := decodePredecessor(reader, project)
	if err != nil {
		return ProjectTypeEnvHeadSelectionClosureV1{}, err
	}
	target, err := decodeTarget(reader)
	if err != nil {
		return ProjectTypeEnvHeadSelectionClosureV1{}, err
	}
	expected, err := reader.readUint64("closure expected GraphRevision")
	if err != nil {
		return ProjectTypeEnvHeadSelectionClosureV1{}, err
	}
	committed, err := reader.readUint64("closure committed GraphRevision")
	if err != nil || committed == 0 || committed-1 != expected {
		return ProjectTypeEnvHeadSelectionClosureV1{},
			fmt.Errorf("closure GraphRevision pair is not contiguous")
	}
	headBytes, err := reader.readBytes("closure successor head")
	if err != nil {
		return ProjectTypeEnvHeadSelectionClosureV1{}, err
	}
	head, err := projecttypeenvselection.DecodeProjectTypeEnvHeadState(headBytes)
	if err != nil || head.Project() != project ||
		head.SelectedComposite() != target.Composite() {
		return ProjectTypeEnvHeadSelectionClosureV1{},
			fmt.Errorf("closure successor head is invalid")
	}
	headDigest, err := readTypedDigest(reader, "closure successor head digest")
	if err != nil {
		return ProjectTypeEnvHeadSelectionClosureV1{}, err
	}
	recomputedHeadDigest, err := digestRaw(headBytes)
	if err != nil || recomputedHeadDigest != headDigest {
		return ProjectTypeEnvHeadSelectionClosureV1{},
			fmt.Errorf("closure successor head digest mismatch")
	}
	deltaRef, deltaDigest, err := decodeContentAddressedPair(
		reader,
		"closure activation delta",
		ParseProjectTypeEnvActivationDeltaRef,
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionClosureV1{}, err
	}
	envelopeRef, envelopeDigest, err := decodeContentAddressedPair(
		reader,
		"closure activation envelope",
		ParseProjectTypeEnvActivationAdmissionEnvelopeRef,
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionClosureV1{}, err
	}
	basisRef, basisDigest, err := decodeContentAddressedPair(
		reader,
		"closure activation basis",
		ParseProjectTypeEnvActivationAdmissionBasisRef,
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionClosureV1{}, err
	}
	manifestRef, manifestDigest, err := decodeContentAddressedPair(
		reader,
		"closure activation manifest",
		ParseProjectTypeEnvActivationMaterializationManifestRef,
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionClosureV1{}, err
	}
	eventText, err := reader.readString("closure event ref")
	if err != nil {
		return ProjectTypeEnvHeadSelectionClosureV1{}, err
	}
	event, err := projecttypeenvselection.ParseGraphEventRef(eventText)
	if err != nil {
		return ProjectTypeEnvHeadSelectionClosureV1{}, err
	}
	commitText, err := reader.readString("closure commit ref")
	if err != nil {
		return ProjectTypeEnvHeadSelectionClosureV1{}, err
	}
	commit, err := projecttypeenvselection.ParseGraphCommitRef(commitText)
	if err != nil {
		return ProjectTypeEnvHeadSelectionClosureV1{}, err
	}
	materializationDigest, err := readTypedDigest(
		reader,
		"closure materialization digest",
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionClosureV1{}, err
	}
	receiptRef, receiptDigest, err := decodeContentAddressedPair(
		reader,
		"closure receipt",
		ParseProjectTypeEnvHeadSelectionReceiptRef,
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionClosureV1{}, err
	}
	resultRef, resultDigest, err := decodeContentAddressedPair(
		reader,
		"closure committed result",
		ParseProjectTypeEnvHeadSelectionCommittedResultRef,
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionClosureV1{}, err
	}
	if err := reader.requireEnd("head-selection closure"); err != nil {
		return ProjectTypeEnvHeadSelectionClosureV1{}, err
	}
	digest, err := digestCanonical(headSelectionClosureDomain, canonical)
	if err != nil {
		return ProjectTypeEnvHeadSelectionClosureV1{}, err
	}
	return ProjectTypeEnvHeadSelectionClosureV1{
		ref:                    ProjectTypeEnvHeadSelectionClosureRef{digest: digest},
		digest:                 digest,
		transactionRef:         transactionRef,
		transactionDigest:      transactionDigest,
		project:                project,
		idempotencyKey:         key,
		requestRef:             requestRef,
		requestDigest:          requestDigest,
		authority:              coordinates,
		authorityUseRef:        useRef,
		authorityUseDigest:     useDigest,
		workRef:                workRef,
		workRecordRef:          workRecordRef,
		workRecordDigest:       workRecordDigest,
		predecessor:            predecessor,
		target:                 target,
		expectedGraphRevision:  typedmemory.NewGraphRevision(expected),
		committedGraphRevision: typedmemory.NewGraphRevision(committed),
		successorHead:          head,
		successorHeadDigest:    headDigest,
		deltaRef:               deltaRef,
		deltaDigest:            deltaDigest,
		envelopeRef:            envelopeRef,
		envelopeDigest:         envelopeDigest,
		basisRef:               basisRef,
		basisDigest:            basisDigest,
		manifestRef:            manifestRef,
		manifestDigest:         manifestDigest,
		event:                  event,
		commit:                 commit,
		materializationDigest:  materializationDigest,
		receiptRef:             receiptRef,
		receiptDigest:          receiptDigest,
		resultRef:              resultRef,
		resultDigest:           resultDigest,
		canonicalBytes:         append([]byte(nil), canonical...),
	}, nil
}

func (closure ProjectTypeEnvHeadSelectionClosureV1) Ref() ProjectTypeEnvHeadSelectionClosureRef {
	return closure.ref
}

func (closure ProjectTypeEnvHeadSelectionClosureV1) Digest() typedmemory.SHA256Digest {
	return closure.digest
}

func (closure ProjectTypeEnvHeadSelectionClosureV1) TransactionRef() ProjectTypeEnvHeadSelectionTransactionRef {
	return closure.transactionRef
}

func (closure ProjectTypeEnvHeadSelectionClosureV1) TransactionDigest() typedmemory.SHA256Digest {
	return closure.transactionDigest
}

func (closure ProjectTypeEnvHeadSelectionClosureV1) Project() projectidentity.ProjectID {
	return closure.project
}

func (closure ProjectTypeEnvHeadSelectionClosureV1) IdempotencyKey() projecttypeenvselection.ProjectTypeEnvHeadSelectionIdempotencyKey {
	return closure.idempotencyKey
}

func (closure ProjectTypeEnvHeadSelectionClosureV1) RequestRef() projecttypeenvselection.ProjectTypeEnvHeadSelectionRequestRef {
	return closure.requestRef
}

func (closure ProjectTypeEnvHeadSelectionClosureV1) RequestDigest() typedmemory.SHA256Digest {
	return closure.requestDigest
}

func (closure ProjectTypeEnvHeadSelectionClosureV1) AuthorityCoordinates() ProjectTypeEnvHeadSelectionAuthorityCoordinates {
	return closure.authority
}

func (closure ProjectTypeEnvHeadSelectionClosureV1) AuthorityUseRecordRef() ProjectTypeEnvHeadSelectionAuthorityUseRecordRef {
	return closure.authorityUseRef
}

func (closure ProjectTypeEnvHeadSelectionClosureV1) AuthorityUseRecordDigest() typedmemory.SHA256Digest {
	return closure.authorityUseDigest
}

func (closure ProjectTypeEnvHeadSelectionClosureV1) WorkRef() authority.WorkRef {
	return closure.workRef
}

func (closure ProjectTypeEnvHeadSelectionClosureV1) CASWorkRecordRef() ProjectTypeEnvHeadCASWorkRecordRef {
	return closure.workRecordRef
}

func (closure ProjectTypeEnvHeadSelectionClosureV1) CASWorkRecordDigest() typedmemory.SHA256Digest {
	return closure.workRecordDigest
}

func (closure ProjectTypeEnvHeadSelectionClosureV1) Predecessor() projecttypeenvselection.ProjectTypeEnvHeadSelectionPredecessor {
	return closure.predecessor
}

func (closure ProjectTypeEnvHeadSelectionClosureV1) Target() ProjectTypeEnvHeadSelectionTarget {
	return closure.target
}

func (closure ProjectTypeEnvHeadSelectionClosureV1) ExpectedGraphRevision() typedmemory.GraphRevision {
	return closure.expectedGraphRevision
}

func (closure ProjectTypeEnvHeadSelectionClosureV1) CommittedGraphRevision() typedmemory.GraphRevision {
	return closure.committedGraphRevision
}

func (closure ProjectTypeEnvHeadSelectionClosureV1) SuccessorHead() projecttypeenvselection.ProjectTypeEnvHeadState {
	return closure.successorHead
}

func (closure ProjectTypeEnvHeadSelectionClosureV1) SuccessorHeadDigest() typedmemory.SHA256Digest {
	return closure.successorHeadDigest
}

func (closure ProjectTypeEnvHeadSelectionClosureV1) ActivationDeltaRef() ProjectTypeEnvActivationDeltaRef {
	return closure.deltaRef
}

func (closure ProjectTypeEnvHeadSelectionClosureV1) ActivationDeltaDigest() typedmemory.SHA256Digest {
	return closure.deltaDigest
}

func (closure ProjectTypeEnvHeadSelectionClosureV1) ActivationEnvelopeRef() ProjectTypeEnvActivationAdmissionEnvelopeRef {
	return closure.envelopeRef
}

func (closure ProjectTypeEnvHeadSelectionClosureV1) ActivationEnvelopeDigest() typedmemory.SHA256Digest {
	return closure.envelopeDigest
}

func (closure ProjectTypeEnvHeadSelectionClosureV1) ActivationBasisRef() ProjectTypeEnvActivationAdmissionBasisRef {
	return closure.basisRef
}

func (closure ProjectTypeEnvHeadSelectionClosureV1) ActivationBasisDigest() typedmemory.SHA256Digest {
	return closure.basisDigest
}

func (closure ProjectTypeEnvHeadSelectionClosureV1) ActivationManifestRef() ProjectTypeEnvActivationMaterializationManifestRef {
	return closure.manifestRef
}

func (closure ProjectTypeEnvHeadSelectionClosureV1) ActivationManifestDigest() typedmemory.SHA256Digest {
	return closure.manifestDigest
}

func (closure ProjectTypeEnvHeadSelectionClosureV1) EventRef() projecttypeenvselection.GraphEventRef {
	return closure.event
}

func (closure ProjectTypeEnvHeadSelectionClosureV1) CommitRef() projecttypeenvselection.GraphCommitRef {
	return closure.commit
}

func (closure ProjectTypeEnvHeadSelectionClosureV1) MaterializationDigest() typedmemory.SHA256Digest {
	return closure.materializationDigest
}

func (closure ProjectTypeEnvHeadSelectionClosureV1) ReceiptRef() ProjectTypeEnvHeadSelectionReceiptRef {
	return closure.receiptRef
}

func (closure ProjectTypeEnvHeadSelectionClosureV1) ReceiptDigest() typedmemory.SHA256Digest {
	return closure.receiptDigest
}

func (closure ProjectTypeEnvHeadSelectionClosureV1) CommittedResultRef() ProjectTypeEnvHeadSelectionCommittedResultRef {
	return closure.resultRef
}

func (closure ProjectTypeEnvHeadSelectionClosureV1) CommittedResultDigest() typedmemory.SHA256Digest {
	return closure.resultDigest
}

func (closure ProjectTypeEnvHeadSelectionClosureV1) CanonicalBytes() []byte {
	return append([]byte(nil), closure.canonicalBytes...)
}

func (closure ProjectTypeEnvHeadSelectionClosureV1) Verify() error {
	decoded, err := DecodeProjectTypeEnvHeadSelectionClosureV1(
		closure.canonicalBytes,
	)
	if err != nil {
		return err
	}
	if decoded.ref != closure.ref || decoded.digest != closure.digest {
		return fmt.Errorf("head-selection closure differs from canonical bytes")
	}
	return nil
}

type contentAddressedRef interface {
	Digest() typedmemory.SHA256Digest
}

func decodeContentAddressedPair[T contentAddressedRef](
	reader *canonicalReader,
	name string,
	parse func(string) (T, error),
) (T, typedmemory.SHA256Digest, error) {
	var zero T
	refText, err := reader.readString(name + " ref")
	if err != nil {
		return zero, typedmemory.SHA256Digest{}, err
	}
	ref, err := parse(refText)
	if err != nil {
		return zero, typedmemory.SHA256Digest{}, err
	}
	digest, err := readTypedDigest(reader, name+" digest")
	if err != nil || ref.Digest() != digest {
		return zero, typedmemory.SHA256Digest{},
			fmt.Errorf("%s ref/digest mismatch", name)
	}
	return ref, digest, nil
}
