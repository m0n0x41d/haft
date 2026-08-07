package projecttypeenvselectioneffect

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	transactionIdentityDomain = "haft.project-typeenv.head-selection-transaction-identity.v1"
	transactionIdentityPrefix = "project-typeenv-head-selection-transaction:"
	referenceDAGDomain        = "haft.project-typeenv.head-selection-reference-dag.v1"

	authorityUseRecordRefPrefix = "project-typeenv-head-selection-authority-use:"
	casWorkRecordRefPrefix      = "project-typeenv-head-cas-work-record:"
	casWorkRefPrefix            = "project-typeenv-head-cas-work:"
	graphActivationKeyPrefix    = "project-typeenv-head-activation:"
)

var stableHexRefPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

type ProjectTypeEnvHeadSelectionTransactionRef struct {
	digest typedmemory.SHA256Digest
}

func ParseProjectTypeEnvHeadSelectionTransactionRef(
	raw string,
) (ProjectTypeEnvHeadSelectionTransactionRef, error) {
	digest, err := parseTypedDigestRef(
		"head-selection transaction",
		transactionIdentityPrefix,
		raw,
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionTransactionRef{}, err
	}
	return ProjectTypeEnvHeadSelectionTransactionRef{digest: digest}, nil
}

func (ref ProjectTypeEnvHeadSelectionTransactionRef) Digest() typedmemory.SHA256Digest {
	return ref.digest
}

func (ref ProjectTypeEnvHeadSelectionTransactionRef) String() string {
	return transactionIdentityPrefix + ref.digest.String()
}

// ProjectTypeEnvHeadSelectionTransactionIdentity is the cycle-free occurrence
// root for one original selection. It is sensitive to both revision spaces.
// It is not proof that COMMIT occurred.
type ProjectTypeEnvHeadSelectionTransactionIdentity struct {
	ref                    ProjectTypeEnvHeadSelectionTransactionRef
	digest                 typedmemory.SHA256Digest
	project                projectidentity.ProjectID
	idempotencyKey         projecttypeenvselection.ProjectTypeEnvHeadSelectionIdempotencyKey
	requestRef             projecttypeenvselection.ProjectTypeEnvHeadSelectionRequestRef
	requestDigest          typedmemory.SHA256Digest
	contentDigest          authority.Digest
	successorHeadRevision  projecttypeenvselection.HeadRevision
	committedGraphRevision typedmemory.GraphRevision
	canonicalBytes         []byte
}

type ProjectTypeEnvHeadSelectionTransactionIdentityInput struct {
	Project                projectidentity.ProjectID
	IdempotencyKey         projecttypeenvselection.ProjectTypeEnvHeadSelectionIdempotencyKey
	RequestRef             projecttypeenvselection.ProjectTypeEnvHeadSelectionRequestRef
	RequestDigest          typedmemory.SHA256Digest
	ContentDigest          authority.Digest
	SuccessorHeadRevision  projecttypeenvselection.HeadRevision
	CommittedGraphRevision typedmemory.GraphRevision
}

func SealProjectTypeEnvHeadSelectionTransactionIdentity(
	input ProjectTypeEnvHeadSelectionTransactionIdentityInput,
) (ProjectTypeEnvHeadSelectionTransactionIdentity, error) {
	state, err := normalizeTransactionIdentityState(transactionIdentityState{
		project:                input.Project,
		idempotencyKey:         input.IdempotencyKey,
		requestRef:             input.RequestRef,
		requestDigest:          input.RequestDigest,
		contentDigest:          input.ContentDigest,
		successorHeadRevision:  input.SuccessorHeadRevision,
		committedGraphRevision: input.CommittedGraphRevision,
	})
	if err != nil {
		return ProjectTypeEnvHeadSelectionTransactionIdentity{}, err
	}
	return DecodeProjectTypeEnvHeadSelectionTransactionIdentity(
		encodeTransactionIdentityState(state),
	)
}

func DecodeProjectTypeEnvHeadSelectionTransactionIdentity(
	canonical []byte,
) (ProjectTypeEnvHeadSelectionTransactionIdentity, error) {
	reader, err := newCanonicalReader(canonical, transactionIdentityDomain)
	if err != nil {
		return ProjectTypeEnvHeadSelectionTransactionIdentity{}, err
	}
	state, err := decodeTransactionIdentityState(reader)
	if err != nil {
		return ProjectTypeEnvHeadSelectionTransactionIdentity{}, err
	}
	if err := reader.requireEnd("head-selection transaction identity"); err != nil {
		return ProjectTypeEnvHeadSelectionTransactionIdentity{}, err
	}
	normalized, err := normalizeTransactionIdentityState(state)
	if err != nil {
		return ProjectTypeEnvHeadSelectionTransactionIdentity{}, err
	}
	reencoded := encodeTransactionIdentityState(normalized)
	if !bytes.Equal(reencoded, canonical) {
		return ProjectTypeEnvHeadSelectionTransactionIdentity{},
			fmt.Errorf("head-selection transaction identity is not canonical")
	}
	digest, err := digestCanonical(transactionIdentityDomain, canonical)
	if err != nil {
		return ProjectTypeEnvHeadSelectionTransactionIdentity{}, err
	}
	ref := ProjectTypeEnvHeadSelectionTransactionRef{digest: digest}
	return ProjectTypeEnvHeadSelectionTransactionIdentity{
		ref:                    ref,
		digest:                 digest,
		project:                normalized.project,
		idempotencyKey:         normalized.idempotencyKey,
		requestRef:             normalized.requestRef,
		requestDigest:          normalized.requestDigest,
		contentDigest:          normalized.contentDigest,
		successorHeadRevision:  normalized.successorHeadRevision,
		committedGraphRevision: normalized.committedGraphRevision,
		canonicalBytes:         append([]byte(nil), canonical...),
	}, nil
}

func (identity ProjectTypeEnvHeadSelectionTransactionIdentity) Ref() ProjectTypeEnvHeadSelectionTransactionRef {
	return identity.ref
}

func (identity ProjectTypeEnvHeadSelectionTransactionIdentity) Digest() typedmemory.SHA256Digest {
	return identity.digest
}

func (identity ProjectTypeEnvHeadSelectionTransactionIdentity) Project() projectidentity.ProjectID {
	return identity.project
}

func (identity ProjectTypeEnvHeadSelectionTransactionIdentity) IdempotencyKey() projecttypeenvselection.ProjectTypeEnvHeadSelectionIdempotencyKey {
	return identity.idempotencyKey
}

func (identity ProjectTypeEnvHeadSelectionTransactionIdentity) RequestRef() projecttypeenvselection.ProjectTypeEnvHeadSelectionRequestRef {
	return identity.requestRef
}

func (identity ProjectTypeEnvHeadSelectionTransactionIdentity) RequestDigest() typedmemory.SHA256Digest {
	return identity.requestDigest
}

func (identity ProjectTypeEnvHeadSelectionTransactionIdentity) ContentDigest() authority.Digest {
	return identity.contentDigest
}

func (identity ProjectTypeEnvHeadSelectionTransactionIdentity) SuccessorHeadRevision() projecttypeenvselection.HeadRevision {
	return identity.successorHeadRevision
}

func (identity ProjectTypeEnvHeadSelectionTransactionIdentity) CommittedGraphRevision() typedmemory.GraphRevision {
	return identity.committedGraphRevision
}

func (identity ProjectTypeEnvHeadSelectionTransactionIdentity) CanonicalBytes() []byte {
	return append([]byte(nil), identity.canonicalBytes...)
}

func (identity ProjectTypeEnvHeadSelectionTransactionIdentity) Verify() error {
	decoded, err := DecodeProjectTypeEnvHeadSelectionTransactionIdentity(
		identity.canonicalBytes,
	)
	if err != nil {
		return err
	}
	if decoded.ref != identity.ref ||
		decoded.digest != identity.digest ||
		decoded.project != identity.project ||
		decoded.idempotencyKey != identity.idempotencyKey ||
		decoded.requestRef != identity.requestRef ||
		decoded.requestDigest != identity.requestDigest ||
		decoded.contentDigest != identity.contentDigest ||
		decoded.successorHeadRevision != identity.successorHeadRevision ||
		decoded.committedGraphRevision != identity.committedGraphRevision {
		return fmt.Errorf("head-selection transaction identity differs from canonical bytes")
	}
	return nil
}

type transactionIdentityState struct {
	project                projectidentity.ProjectID
	idempotencyKey         projecttypeenvselection.ProjectTypeEnvHeadSelectionIdempotencyKey
	requestRef             projecttypeenvselection.ProjectTypeEnvHeadSelectionRequestRef
	requestDigest          typedmemory.SHA256Digest
	contentDigest          authority.Digest
	successorHeadRevision  projecttypeenvselection.HeadRevision
	committedGraphRevision typedmemory.GraphRevision
}

func normalizeTransactionIdentityState(
	state transactionIdentityState,
) (transactionIdentityState, error) {
	project, err := normalizeProject(state.project)
	if err != nil {
		return transactionIdentityState{}, err
	}
	key, err := projecttypeenvselection.NewProjectTypeEnvHeadSelectionIdempotencyKey(
		state.idempotencyKey.String(),
	)
	if err != nil || key != state.idempotencyKey {
		return transactionIdentityState{}, fmt.Errorf("head-selection idempotency key is required")
	}
	requestRef, err := normalizeRequestRef(state.requestRef, state.requestDigest)
	if err != nil {
		return transactionIdentityState{}, err
	}
	contentDigest, err := authority.NewDigest(state.contentDigest.String())
	if err != nil || contentDigest != state.contentDigest {
		return transactionIdentityState{}, fmt.Errorf("authorization-content digest is required")
	}
	headRevision, err := projecttypeenvselection.NewHeadRevision(
		state.successorHeadRevision.Value(),
	)
	if err != nil || headRevision != state.successorHeadRevision {
		return transactionIdentityState{}, fmt.Errorf("successor HeadRevision is required")
	}
	if state.committedGraphRevision.Value() == 0 {
		return transactionIdentityState{}, fmt.Errorf(
			"committed GraphRevision must be greater than zero",
		)
	}
	graphRevision := typedmemory.NewGraphRevision(
		state.committedGraphRevision.Value(),
	)
	return transactionIdentityState{
		project:                project,
		idempotencyKey:         key,
		requestRef:             requestRef,
		requestDigest:          state.requestDigest,
		contentDigest:          contentDigest,
		successorHeadRevision:  headRevision,
		committedGraphRevision: graphRevision,
	}, nil
}

func encodeTransactionIdentityState(state transactionIdentityState) []byte {
	writer := newCanonicalWriter(transactionIdentityDomain)
	writer.writeString(state.project.String())
	writer.writeString(state.idempotencyKey.String())
	writer.writeString(state.requestRef.String())
	writer.writeString(state.requestDigest.String())
	writer.writeString(state.contentDigest.String())
	writer.writeUint64(state.successorHeadRevision.Value())
	writer.writeUint64(state.committedGraphRevision.Value())
	return writer.bytes()
}

func decodeTransactionIdentityState(
	reader *canonicalReader,
) (transactionIdentityState, error) {
	projectText, err := reader.readString("transaction project")
	if err != nil {
		return transactionIdentityState{}, err
	}
	project, err := projectidentity.ParseProjectID(projectText)
	if err != nil {
		return transactionIdentityState{}, err
	}
	keyText, err := reader.readString("transaction idempotency key")
	if err != nil {
		return transactionIdentityState{}, err
	}
	key, err := projecttypeenvselection.NewProjectTypeEnvHeadSelectionIdempotencyKey(keyText)
	if err != nil {
		return transactionIdentityState{}, err
	}
	requestText, err := reader.readString("transaction request ref")
	if err != nil {
		return transactionIdentityState{}, err
	}
	requestRef, err := projecttypeenvselection.ParseProjectTypeEnvHeadSelectionRequestRef(
		requestText,
	)
	if err != nil {
		return transactionIdentityState{}, err
	}
	requestDigestText, err := reader.readString("transaction request digest")
	if err != nil {
		return transactionIdentityState{}, err
	}
	requestDigest, err := typedmemory.NewSHA256Digest(requestDigestText)
	if err != nil {
		return transactionIdentityState{}, err
	}
	contentDigestText, err := reader.readString("transaction content digest")
	if err != nil {
		return transactionIdentityState{}, err
	}
	contentDigest, err := authority.NewDigest(contentDigestText)
	if err != nil {
		return transactionIdentityState{}, err
	}
	headValue, err := reader.readUint64("transaction successor HeadRevision")
	if err != nil {
		return transactionIdentityState{}, err
	}
	headRevision, err := projecttypeenvselection.NewHeadRevision(headValue)
	if err != nil {
		return transactionIdentityState{}, err
	}
	graphValue, err := reader.readUint64("transaction committed GraphRevision")
	if err != nil {
		return transactionIdentityState{}, err
	}
	return transactionIdentityState{
		project:                project,
		idempotencyKey:         key,
		requestRef:             requestRef,
		requestDigest:          requestDigest,
		contentDigest:          contentDigest,
		successorHeadRevision:  headRevision,
		committedGraphRevision: typedmemory.NewGraphRevision(graphValue),
	}, nil
}

type ProjectTypeEnvHeadSelectionAuthorityUseRecordRef struct {
	digest typedmemory.SHA256Digest
}

func ParseProjectTypeEnvHeadSelectionAuthorityUseRecordRef(
	raw string,
) (ProjectTypeEnvHeadSelectionAuthorityUseRecordRef, error) {
	digest, err := parseTypedDigestRef(
		"head-selection authority-use record",
		authorityUseRecordRefPrefix,
		raw,
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityUseRecordRef{}, err
	}
	return ProjectTypeEnvHeadSelectionAuthorityUseRecordRef{digest: digest}, nil
}

func (ref ProjectTypeEnvHeadSelectionAuthorityUseRecordRef) Digest() typedmemory.SHA256Digest {
	return ref.digest
}

func (ref ProjectTypeEnvHeadSelectionAuthorityUseRecordRef) String() string {
	return authorityUseRecordRefPrefix + ref.digest.String()
}

type ProjectTypeEnvHeadCASWorkRecordRef struct {
	digest typedmemory.SHA256Digest
}

func ParseProjectTypeEnvHeadCASWorkRecordRef(
	raw string,
) (ProjectTypeEnvHeadCASWorkRecordRef, error) {
	digest, err := parseTypedDigestRef(
		"head CAS Work record",
		casWorkRecordRefPrefix,
		raw,
	)
	if err != nil {
		return ProjectTypeEnvHeadCASWorkRecordRef{}, err
	}
	return ProjectTypeEnvHeadCASWorkRecordRef{digest: digest}, nil
}

func (ref ProjectTypeEnvHeadCASWorkRecordRef) Digest() typedmemory.SHA256Digest {
	return ref.digest
}

func (ref ProjectTypeEnvHeadCASWorkRecordRef) String() string {
	return casWorkRecordRefPrefix + ref.digest.String()
}

type GraphActivationIdempotencyKey struct {
	value string
}

func ParseGraphActivationIdempotencyKey(
	raw string,
) (GraphActivationIdempotencyKey, error) {
	value, found := strings.CutPrefix(raw, graphActivationKeyPrefix)
	if !found || !stableHexRefPattern.MatchString(value) {
		return GraphActivationIdempotencyKey{},
			fmt.Errorf("graph activation idempotency key is malformed")
	}
	return GraphActivationIdempotencyKey{value: raw}, nil
}

func (key GraphActivationIdempotencyKey) String() string { return key.value }

// ProjectTypeEnvHeadSelectionReferenceDAG contains only cycle-free stable
// occurrence identities. Receipt and closure refs are deliberately absent.
type ProjectTypeEnvHeadSelectionReferenceDAG struct {
	transactionRef ProjectTypeEnvHeadSelectionTransactionRef
	transactionDig typedmemory.SHA256Digest
	authorityUse   ProjectTypeEnvHeadSelectionAuthorityUseRecordRef
	work           authority.WorkRef
	workRecord     ProjectTypeEnvHeadCASWorkRecordRef
	graphKey       GraphActivationIdempotencyKey
	canonicalBytes []byte
}

func DeriveProjectTypeEnvHeadSelectionReferenceDAG(
	identity ProjectTypeEnvHeadSelectionTransactionIdentity,
) (ProjectTypeEnvHeadSelectionReferenceDAG, error) {
	if err := identity.Verify(); err != nil {
		return ProjectTypeEnvHeadSelectionReferenceDAG{}, err
	}
	useDigest, err := digestFields(
		"haft.project-typeenv.head-selection-authority-use-ref.v1",
		identity.Ref().String(),
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionReferenceDAG{}, err
	}
	workDigest, err := digestFields(
		"haft.project-typeenv.head-cas-work-ref.v1",
		identity.Ref().String(),
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionReferenceDAG{}, err
	}
	workRecordDigest, err := digestFields(
		"haft.project-typeenv.head-cas-work-record-ref.v1",
		identity.Ref().String(),
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionReferenceDAG{}, err
	}
	graphKeyDigest, err := digestFields(
		"haft.project-typeenv.head-activation-idempotency.v1",
		identity.Ref().String(),
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionReferenceDAG{}, err
	}
	work, err := authority.NewWorkRef(casWorkRefPrefix + canonicalHex(workDigest))
	if err != nil {
		return ProjectTypeEnvHeadSelectionReferenceDAG{}, err
	}
	graphKey, err := ParseGraphActivationIdempotencyKey(
		graphActivationKeyPrefix + canonicalHex(graphKeyDigest),
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionReferenceDAG{}, err
	}
	dag := ProjectTypeEnvHeadSelectionReferenceDAG{
		transactionRef: identity.Ref(),
		transactionDig: identity.Digest(),
		authorityUse: ProjectTypeEnvHeadSelectionAuthorityUseRecordRef{
			digest: useDigest,
		},
		work: work,
		workRecord: ProjectTypeEnvHeadCASWorkRecordRef{
			digest: workRecordDigest,
		},
		graphKey: graphKey,
	}
	writer := newCanonicalWriter(referenceDAGDomain)
	encodeReferenceDAG(&writer, dag)
	dag.canonicalBytes = writer.bytes()
	return dag, nil
}

func DecodeProjectTypeEnvHeadSelectionReferenceDAG(
	identity ProjectTypeEnvHeadSelectionTransactionIdentity,
	canonical []byte,
) (ProjectTypeEnvHeadSelectionReferenceDAG, error) {
	reader, err := newCanonicalReader(canonical, referenceDAGDomain)
	if err != nil {
		return ProjectTypeEnvHeadSelectionReferenceDAG{}, err
	}
	stored, err := decodeReferenceDAG(reader)
	if err != nil {
		return ProjectTypeEnvHeadSelectionReferenceDAG{}, err
	}
	if err := reader.requireEnd("head-selection reference DAG"); err != nil {
		return ProjectTypeEnvHeadSelectionReferenceDAG{}, err
	}
	derived, err := DeriveProjectTypeEnvHeadSelectionReferenceDAG(identity)
	if err != nil {
		return ProjectTypeEnvHeadSelectionReferenceDAG{}, err
	}
	if !sameReferenceDAG(stored, derived) ||
		!bytes.Equal(derived.canonicalBytes, canonical) {
		return ProjectTypeEnvHeadSelectionReferenceDAG{},
			fmt.Errorf("head-selection reference DAG is not exact for transaction identity")
	}
	return derived, nil
}

func (dag ProjectTypeEnvHeadSelectionReferenceDAG) TransactionRef() ProjectTypeEnvHeadSelectionTransactionRef {
	return dag.transactionRef
}

func (dag ProjectTypeEnvHeadSelectionReferenceDAG) TransactionDigest() typedmemory.SHA256Digest {
	return dag.transactionDig
}

func (dag ProjectTypeEnvHeadSelectionReferenceDAG) AuthorityUseRecordRef() ProjectTypeEnvHeadSelectionAuthorityUseRecordRef {
	return dag.authorityUse
}

func (dag ProjectTypeEnvHeadSelectionReferenceDAG) WorkRef() authority.WorkRef {
	return dag.work
}

func (dag ProjectTypeEnvHeadSelectionReferenceDAG) CASWorkRecordRef() ProjectTypeEnvHeadCASWorkRecordRef {
	return dag.workRecord
}

func (dag ProjectTypeEnvHeadSelectionReferenceDAG) GraphIdempotencyKey() GraphActivationIdempotencyKey {
	return dag.graphKey
}

func (dag ProjectTypeEnvHeadSelectionReferenceDAG) CanonicalBytes() []byte {
	return append([]byte(nil), dag.canonicalBytes...)
}

func (dag ProjectTypeEnvHeadSelectionReferenceDAG) Verify(
	identity ProjectTypeEnvHeadSelectionTransactionIdentity,
) error {
	_, err := DecodeProjectTypeEnvHeadSelectionReferenceDAG(identity, dag.canonicalBytes)
	return err
}

func encodeReferenceDAG(
	writer *canonicalWriter,
	dag ProjectTypeEnvHeadSelectionReferenceDAG,
) {
	writer.writeString(dag.transactionRef.String())
	writer.writeString(dag.transactionDig.String())
	writer.writeString(dag.authorityUse.String())
	writer.writeString(dag.work.String())
	writer.writeString(dag.workRecord.String())
	writer.writeString(dag.graphKey.String())
}

func decodeReferenceDAG(
	reader *canonicalReader,
) (ProjectTypeEnvHeadSelectionReferenceDAG, error) {
	transactionText, err := reader.readString("DAG transaction ref")
	if err != nil {
		return ProjectTypeEnvHeadSelectionReferenceDAG{}, err
	}
	transactionRef, err := ParseProjectTypeEnvHeadSelectionTransactionRef(transactionText)
	if err != nil {
		return ProjectTypeEnvHeadSelectionReferenceDAG{}, err
	}
	transactionDigestText, err := reader.readString("DAG transaction digest")
	if err != nil {
		return ProjectTypeEnvHeadSelectionReferenceDAG{}, err
	}
	transactionDigest, err := typedmemory.NewSHA256Digest(transactionDigestText)
	if err != nil {
		return ProjectTypeEnvHeadSelectionReferenceDAG{}, err
	}
	useText, err := reader.readString("DAG authority-use ref")
	if err != nil {
		return ProjectTypeEnvHeadSelectionReferenceDAG{}, err
	}
	useRef, err := ParseProjectTypeEnvHeadSelectionAuthorityUseRecordRef(useText)
	if err != nil {
		return ProjectTypeEnvHeadSelectionReferenceDAG{}, err
	}
	workText, err := reader.readString("DAG Work ref")
	if err != nil {
		return ProjectTypeEnvHeadSelectionReferenceDAG{}, err
	}
	workRef, err := authority.NewWorkRef(workText)
	if err != nil {
		return ProjectTypeEnvHeadSelectionReferenceDAG{}, err
	}
	workRecordText, err := reader.readString("DAG Work-record ref")
	if err != nil {
		return ProjectTypeEnvHeadSelectionReferenceDAG{}, err
	}
	workRecordRef, err := ParseProjectTypeEnvHeadCASWorkRecordRef(workRecordText)
	if err != nil {
		return ProjectTypeEnvHeadSelectionReferenceDAG{}, err
	}
	graphKeyText, err := reader.readString("DAG graph idempotency key")
	if err != nil {
		return ProjectTypeEnvHeadSelectionReferenceDAG{}, err
	}
	graphKey, err := ParseGraphActivationIdempotencyKey(graphKeyText)
	if err != nil {
		return ProjectTypeEnvHeadSelectionReferenceDAG{}, err
	}
	return ProjectTypeEnvHeadSelectionReferenceDAG{
		transactionRef: transactionRef,
		transactionDig: transactionDigest,
		authorityUse:   useRef,
		work:           workRef,
		workRecord:     workRecordRef,
		graphKey:       graphKey,
	}, nil
}

func sameReferenceDAG(
	left ProjectTypeEnvHeadSelectionReferenceDAG,
	right ProjectTypeEnvHeadSelectionReferenceDAG,
) bool {
	return left.transactionRef == right.transactionRef &&
		left.transactionDig == right.transactionDig &&
		left.authorityUse == right.authorityUse &&
		left.work == right.work &&
		left.workRecord == right.workRecord &&
		left.graphKey == right.graphKey
}

func parseTypedDigestRef(
	name string,
	prefix string,
	raw string,
) (typedmemory.SHA256Digest, error) {
	if raw != strings.TrimSpace(raw) || !strings.HasPrefix(raw, prefix) {
		return typedmemory.SHA256Digest{}, fmt.Errorf(
			"%s ref must start with %q",
			name,
			prefix,
		)
	}
	digest, err := typedmemory.NewSHA256Digest(strings.TrimPrefix(raw, prefix))
	if err != nil {
		return typedmemory.SHA256Digest{}, fmt.Errorf("%s ref digest: %w", name, err)
	}
	return digest, nil
}
