package projecttypeenvselectioneffect

import (
	"fmt"
	"strings"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	authorityUseDomain = "haft.project-typeenv.head-selection-authority-use-record.v1"
	verifierRefPrefix  = "project-typeenv-head-selection-verifier:"
)

// ProjectTypeEnvHeadSelectionVerifierRef identifies the exact verifier
// implementation family. VerifierEdition pins one immutable edition of it.
type ProjectTypeEnvHeadSelectionVerifierRef struct {
	value string
}

func NewProjectTypeEnvHeadSelectionVerifierRef(
	raw string,
) (ProjectTypeEnvHeadSelectionVerifierRef, error) {
	if raw != strings.TrimSpace(raw) ||
		!strings.HasPrefix(raw, verifierRefPrefix) ||
		len(strings.TrimPrefix(raw, verifierRefPrefix)) == 0 {
		return ProjectTypeEnvHeadSelectionVerifierRef{},
			fmt.Errorf("head-selection verifier ref is malformed")
	}
	return ProjectTypeEnvHeadSelectionVerifierRef{value: raw}, nil
}

func (ref ProjectTypeEnvHeadSelectionVerifierRef) String() string {
	return ref.value
}

type ProjectTypeEnvHeadSelectionVerifierEdition struct {
	value uint64
}

func NewProjectTypeEnvHeadSelectionVerifierEdition(
	value uint64,
) (ProjectTypeEnvHeadSelectionVerifierEdition, error) {
	if value == 0 {
		return ProjectTypeEnvHeadSelectionVerifierEdition{},
			fmt.Errorf("head-selection verifier edition must be greater than zero")
	}
	return ProjectTypeEnvHeadSelectionVerifierEdition{value: value}, nil
}

func (edition ProjectTypeEnvHeadSelectionVerifierEdition) Value() uint64 {
	return edition.value
}

// ProjectTypeEnvHeadSelectionAuthorityUseRecord is a stable-identity,
// content-digested record of one authority consumption. Its ref is derived
// from the transaction identity and is deliberately not its content digest.
type ProjectTypeEnvHeadSelectionAuthorityUseRecord struct {
	ref                    ProjectTypeEnvHeadSelectionAuthorityUseRecordRef
	digest                 typedmemory.SHA256Digest
	transactionRef         ProjectTypeEnvHeadSelectionTransactionRef
	transactionDigest      typedmemory.SHA256Digest
	project                projectidentity.ProjectID
	idempotencyKey         projecttypeenvselection.ProjectTypeEnvHeadSelectionIdempotencyKey
	requestRef             projecttypeenvselection.ProjectTypeEnvHeadSelectionRequestRef
	requestDigest          typedmemory.SHA256Digest
	authority              ProjectTypeEnvHeadSelectionAuthorityCoordinates
	workRef                authority.WorkRef
	receiptRef             ProjectTypeEnvHeadSelectionReceiptRef
	receiptDigest          typedmemory.SHA256Digest
	predecessor            projecttypeenvselection.ProjectTypeEnvHeadSelectionPredecessor
	target                 ProjectTypeEnvHeadSelectionTarget
	expectedGraphRevision  typedmemory.GraphRevision
	committedHeadRevision  projecttypeenvselection.HeadRevision
	committedGraphRevision typedmemory.GraphRevision
	committedResultRef     ProjectTypeEnvHeadSelectionCommittedResultRef
	committedResultDigest  typedmemory.SHA256Digest
	verifier               ProjectTypeEnvHeadSelectionVerifierRef
	verifierEdition        ProjectTypeEnvHeadSelectionVerifierEdition
	canonicalBytes         []byte
}

type ProjectTypeEnvHeadSelectionAuthorityUseRecordInput struct {
	Identity        ProjectTypeEnvHeadSelectionTransactionIdentity
	ReferenceDAG    ProjectTypeEnvHeadSelectionReferenceDAG
	Receipt         ProjectTypeEnvHeadSelectionReceiptV1
	Result          ProjectTypeEnvHeadSelectionCommittedResult
	Verifier        ProjectTypeEnvHeadSelectionVerifierRef
	VerifierEdition ProjectTypeEnvHeadSelectionVerifierEdition
}

func SealProjectTypeEnvHeadSelectionAuthorityUseRecord(
	input ProjectTypeEnvHeadSelectionAuthorityUseRecordInput,
) (ProjectTypeEnvHeadSelectionAuthorityUseRecord, error) {
	if err := input.Identity.Verify(); err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityUseRecord{}, err
	}
	if err := input.ReferenceDAG.Verify(input.Identity); err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityUseRecord{}, err
	}
	if err := input.Receipt.Verify(); err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityUseRecord{}, err
	}
	if err := input.Result.Verify(); err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityUseRecord{}, err
	}
	verifier, err := NewProjectTypeEnvHeadSelectionVerifierRef(
		input.Verifier.String(),
	)
	if err != nil || verifier != input.Verifier {
		return ProjectTypeEnvHeadSelectionAuthorityUseRecord{},
			fmt.Errorf("head-selection verifier is required")
	}
	edition, err := NewProjectTypeEnvHeadSelectionVerifierEdition(
		input.VerifierEdition.Value(),
	)
	if err != nil || edition != input.VerifierEdition {
		return ProjectTypeEnvHeadSelectionAuthorityUseRecord{},
			fmt.Errorf("head-selection verifier edition is required")
	}
	if input.Receipt.TransactionRef() != input.Identity.Ref() ||
		input.Receipt.TransactionDigest() != input.Identity.Digest() ||
		input.Receipt.AuthorityUseRecordRef() !=
			input.ReferenceDAG.AuthorityUseRecordRef() ||
		input.Receipt.WorkRef() != input.ReferenceDAG.WorkRef() ||
		input.Receipt.CommittedResultRef() != input.Result.Ref() ||
		input.Receipt.CommittedResultDigest() != input.Result.Digest() {
		return ProjectTypeEnvHeadSelectionAuthorityUseRecord{},
			fmt.Errorf("authority-use members do not describe one exact transaction")
	}
	writer := newCanonicalWriter(authorityUseDomain)
	writer.writeString(input.ReferenceDAG.AuthorityUseRecordRef().String())
	writer.writeString(input.Identity.Ref().String())
	writer.writeString(input.Identity.Digest().String())
	writer.writeString(input.Identity.Project().String())
	writer.writeString(input.Identity.IdempotencyKey().String())
	writer.writeString(input.Identity.RequestRef().String())
	writer.writeString(input.Identity.RequestDigest().String())
	encodeAuthorityCoordinates(&writer, input.Receipt.AuthorityCoordinates())
	writer.writeString(input.ReferenceDAG.WorkRef().String())
	writer.writeString(input.Receipt.Ref().String())
	writer.writeString(input.Receipt.Digest().String())
	encodePredecessor(&writer, input.Receipt.Predecessor())
	encodeTarget(&writer, input.Receipt.Target())
	writer.writeUint64(input.Receipt.ExpectedGraphRevision().Value())
	writer.writeUint64(input.Receipt.SuccessorHead().Revision().Value())
	writer.writeUint64(input.Receipt.CommittedGraphRevision().Value())
	writer.writeString(input.Result.Ref().String())
	writer.writeString(input.Result.Digest().String())
	writer.writeString(verifier.String())
	writer.writeUint64(edition.Value())
	return DecodeProjectTypeEnvHeadSelectionAuthorityUseRecord(writer.bytes())
}

func DecodeProjectTypeEnvHeadSelectionAuthorityUseRecord(
	canonical []byte,
) (ProjectTypeEnvHeadSelectionAuthorityUseRecord, error) {
	reader, err := newCanonicalReader(canonical, authorityUseDomain)
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityUseRecord{}, err
	}
	refText, err := reader.readString("authority-use ref")
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityUseRecord{}, err
	}
	ref, err := ParseProjectTypeEnvHeadSelectionAuthorityUseRecordRef(refText)
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityUseRecord{}, err
	}
	transactionText, err := reader.readString("authority-use transaction ref")
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityUseRecord{}, err
	}
	transactionRef, err := ParseProjectTypeEnvHeadSelectionTransactionRef(
		transactionText,
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityUseRecord{}, err
	}
	transactionDigest, err := readTypedDigest(
		reader,
		"authority-use transaction digest",
	)
	if err != nil || transactionRef.Digest() != transactionDigest {
		return ProjectTypeEnvHeadSelectionAuthorityUseRecord{},
			fmt.Errorf("authority-use transaction ref/digest mismatch")
	}
	projectText, err := reader.readString("authority-use project")
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityUseRecord{}, err
	}
	project, err := projectidentity.ParseProjectID(projectText)
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityUseRecord{}, err
	}
	keyText, err := reader.readString("authority-use idempotency key")
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityUseRecord{}, err
	}
	key, err := projecttypeenvselection.NewProjectTypeEnvHeadSelectionIdempotencyKey(
		keyText,
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityUseRecord{}, err
	}
	requestText, err := reader.readString("authority-use request ref")
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityUseRecord{}, err
	}
	requestRef, err := projecttypeenvselection.ParseProjectTypeEnvHeadSelectionRequestRef(
		requestText,
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityUseRecord{}, err
	}
	requestDigest, err := readTypedDigest(reader, "authority-use request digest")
	if err != nil || requestRef.Digest() != requestDigest {
		return ProjectTypeEnvHeadSelectionAuthorityUseRecord{},
			fmt.Errorf("authority-use request ref/digest mismatch")
	}
	coordinates, err := decodeAuthorityCoordinates(reader)
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityUseRecord{}, err
	}
	workText, err := reader.readString("authority-use Work ref")
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityUseRecord{}, err
	}
	workRef, err := authority.NewWorkRef(workText)
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityUseRecord{}, err
	}
	receiptText, err := reader.readString("authority-use receipt ref")
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityUseRecord{}, err
	}
	receiptRef, err := ParseProjectTypeEnvHeadSelectionReceiptRef(receiptText)
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityUseRecord{}, err
	}
	receiptDigest, err := readTypedDigest(reader, "authority-use receipt digest")
	if err != nil || receiptRef.Digest() != receiptDigest {
		return ProjectTypeEnvHeadSelectionAuthorityUseRecord{},
			fmt.Errorf("authority-use receipt ref/digest mismatch")
	}
	predecessor, err := decodePredecessor(reader, project)
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityUseRecord{}, err
	}
	target, err := decodeTarget(reader)
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityUseRecord{}, err
	}
	expected, err := reader.readUint64("authority-use expected GraphRevision")
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityUseRecord{}, err
	}
	headRevisionValue, err := reader.readUint64("authority-use HeadRevision")
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityUseRecord{}, err
	}
	headRevision, err := projecttypeenvselection.NewHeadRevision(
		headRevisionValue,
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityUseRecord{}, err
	}
	committed, err := reader.readUint64("authority-use committed GraphRevision")
	if err != nil || committed == 0 || committed-1 != expected {
		return ProjectTypeEnvHeadSelectionAuthorityUseRecord{},
			fmt.Errorf("authority-use GraphRevision pair is not contiguous")
	}
	resultText, err := reader.readString("authority-use committed-result ref")
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityUseRecord{}, err
	}
	resultRef, err := ParseProjectTypeEnvHeadSelectionCommittedResultRef(
		resultText,
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityUseRecord{}, err
	}
	resultDigest, err := readTypedDigest(
		reader,
		"authority-use committed-result digest",
	)
	if err != nil || resultRef.Digest() != resultDigest {
		return ProjectTypeEnvHeadSelectionAuthorityUseRecord{},
			fmt.Errorf("authority-use committed-result ref/digest mismatch")
	}
	verifierText, err := reader.readString("authority-use verifier ref")
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityUseRecord{}, err
	}
	verifier, err := NewProjectTypeEnvHeadSelectionVerifierRef(verifierText)
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityUseRecord{}, err
	}
	editionValue, err := reader.readUint64("authority-use verifier edition")
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityUseRecord{}, err
	}
	edition, err := NewProjectTypeEnvHeadSelectionVerifierEdition(editionValue)
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityUseRecord{}, err
	}
	if err := reader.requireEnd("head-selection authority-use record"); err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityUseRecord{}, err
	}
	digest, err := digestCanonical(authorityUseDomain, canonical)
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityUseRecord{}, err
	}
	return ProjectTypeEnvHeadSelectionAuthorityUseRecord{
		ref:                    ref,
		digest:                 digest,
		transactionRef:         transactionRef,
		transactionDigest:      transactionDigest,
		project:                project,
		idempotencyKey:         key,
		requestRef:             requestRef,
		requestDigest:          requestDigest,
		authority:              coordinates,
		workRef:                workRef,
		receiptRef:             receiptRef,
		receiptDigest:          receiptDigest,
		predecessor:            predecessor,
		target:                 target,
		expectedGraphRevision:  typedmemory.NewGraphRevision(expected),
		committedHeadRevision:  headRevision,
		committedGraphRevision: typedmemory.NewGraphRevision(committed),
		committedResultRef:     resultRef,
		committedResultDigest:  resultDigest,
		verifier:               verifier,
		verifierEdition:        edition,
		canonicalBytes:         append([]byte(nil), canonical...),
	}, nil
}

func (record ProjectTypeEnvHeadSelectionAuthorityUseRecord) Ref() ProjectTypeEnvHeadSelectionAuthorityUseRecordRef {
	return record.ref
}

func (record ProjectTypeEnvHeadSelectionAuthorityUseRecord) Digest() typedmemory.SHA256Digest {
	return record.digest
}

func (record ProjectTypeEnvHeadSelectionAuthorityUseRecord) TransactionRef() ProjectTypeEnvHeadSelectionTransactionRef {
	return record.transactionRef
}

func (record ProjectTypeEnvHeadSelectionAuthorityUseRecord) TransactionDigest() typedmemory.SHA256Digest {
	return record.transactionDigest
}

func (record ProjectTypeEnvHeadSelectionAuthorityUseRecord) Project() projectidentity.ProjectID {
	return record.project
}

func (record ProjectTypeEnvHeadSelectionAuthorityUseRecord) IdempotencyKey() projecttypeenvselection.ProjectTypeEnvHeadSelectionIdempotencyKey {
	return record.idempotencyKey
}

func (record ProjectTypeEnvHeadSelectionAuthorityUseRecord) RequestRef() projecttypeenvselection.ProjectTypeEnvHeadSelectionRequestRef {
	return record.requestRef
}

func (record ProjectTypeEnvHeadSelectionAuthorityUseRecord) RequestDigest() typedmemory.SHA256Digest {
	return record.requestDigest
}

func (record ProjectTypeEnvHeadSelectionAuthorityUseRecord) AuthorityCoordinates() ProjectTypeEnvHeadSelectionAuthorityCoordinates {
	return record.authority
}

func (record ProjectTypeEnvHeadSelectionAuthorityUseRecord) WorkRef() authority.WorkRef {
	return record.workRef
}

func (record ProjectTypeEnvHeadSelectionAuthorityUseRecord) ReceiptRef() ProjectTypeEnvHeadSelectionReceiptRef {
	return record.receiptRef
}

func (record ProjectTypeEnvHeadSelectionAuthorityUseRecord) ReceiptDigest() typedmemory.SHA256Digest {
	return record.receiptDigest
}

func (record ProjectTypeEnvHeadSelectionAuthorityUseRecord) Predecessor() projecttypeenvselection.ProjectTypeEnvHeadSelectionPredecessor {
	return record.predecessor
}

func (record ProjectTypeEnvHeadSelectionAuthorityUseRecord) Target() ProjectTypeEnvHeadSelectionTarget {
	return record.target
}

func (record ProjectTypeEnvHeadSelectionAuthorityUseRecord) ExpectedGraphRevision() typedmemory.GraphRevision {
	return record.expectedGraphRevision
}

func (record ProjectTypeEnvHeadSelectionAuthorityUseRecord) CommittedHeadRevision() projecttypeenvselection.HeadRevision {
	return record.committedHeadRevision
}

func (record ProjectTypeEnvHeadSelectionAuthorityUseRecord) CommittedGraphRevision() typedmemory.GraphRevision {
	return record.committedGraphRevision
}

func (record ProjectTypeEnvHeadSelectionAuthorityUseRecord) CommittedResultRef() ProjectTypeEnvHeadSelectionCommittedResultRef {
	return record.committedResultRef
}

func (record ProjectTypeEnvHeadSelectionAuthorityUseRecord) CommittedResultDigest() typedmemory.SHA256Digest {
	return record.committedResultDigest
}

func (record ProjectTypeEnvHeadSelectionAuthorityUseRecord) Verifier() ProjectTypeEnvHeadSelectionVerifierRef {
	return record.verifier
}

func (record ProjectTypeEnvHeadSelectionAuthorityUseRecord) VerifierEdition() ProjectTypeEnvHeadSelectionVerifierEdition {
	return record.verifierEdition
}

func (record ProjectTypeEnvHeadSelectionAuthorityUseRecord) CanonicalBytes() []byte {
	return append([]byte(nil), record.canonicalBytes...)
}

func (record ProjectTypeEnvHeadSelectionAuthorityUseRecord) Verify() error {
	decoded, err := DecodeProjectTypeEnvHeadSelectionAuthorityUseRecord(
		record.canonicalBytes,
	)
	if err != nil {
		return err
	}
	if decoded.ref != record.ref || decoded.digest != record.digest {
		return fmt.Errorf("authority-use record differs from canonical bytes")
	}
	return nil
}

func readTypedDigest(
	reader *canonicalReader,
	name string,
) (typedmemory.SHA256Digest, error) {
	text, err := reader.readString(name)
	if err != nil {
		return typedmemory.SHA256Digest{}, err
	}
	return typedmemory.NewSHA256Digest(text)
}
