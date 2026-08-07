package typedmemory

import (
	"bytes"
	"fmt"
	"sort"
)

const (
	memberOfQueryDomain       = "member-of-query.v1"
	memberOfBasisDomain       = "member-of-basis.v2"
	memberOfBasisV3Domain     = "member-of-basis.v3"
	memberOfMemberDomain      = "member-of-member.v2"
	memberOfMemberV3Domain    = "member-of-member.v3"
	memberOfNotMemberDomain   = "member-of-not-member.v2"
	memberOfNotMemberV3Domain = "member-of-not-member.v3"
	memberOfUndefinedDomain   = "member-of-undefined.v2"
)

// MemberOfQuery carries the stable entity, kind, and base ContextSlice
// coordinates. It is not executable by itself: MemberOfEvaluationRequest adds
// the exact addressable EntitySet observation pin that completes Haft's
// context-local C.3.2 slice extension.
type MemberOfQuery struct {
	entity         EntityID
	valueKind      ValueKindRef
	contextSlice   ContextSlice
	canonicalBytes []byte
	digest         SHA256Digest
}

func NewMemberOfQuery(
	entity EntityID,
	valueKind ValueKindRef,
	contextSlice ContextSlice,
) (MemberOfQuery, error) {
	if !entity.valid() {
		return MemberOfQuery{}, fmt.Errorf("MemberOf entity ID is required")
	}
	if !valueKind.valid() {
		return MemberOfQuery{}, fmt.Errorf("MemberOf ValueKind is required")
	}
	if !validCompleteContextSlice(contextSlice) {
		return MemberOfQuery{}, fmt.Errorf("MemberOf requires a complete content-addressed ContextSlice")
	}
	writer := newCanonicalWriter(memberOfQueryDomain)
	writer.addString(entity.String())
	writer.addString(valueKind.String())
	writer.addBytes(contextSlice.CanonicalBytes())
	writer.addString(contextSlice.Ref().String())
	return MemberOfQuery{
		entity:         entity,
		valueKind:      valueKind,
		contextSlice:   contextSlice,
		canonicalBytes: writer.bytes(),
		digest:         writer.digest(),
	}, nil
}

func (query MemberOfQuery) EntityID() EntityID { return query.entity }

func (query MemberOfQuery) ValueKind() ValueKindRef { return query.valueKind }

func (query MemberOfQuery) ContextSlice() ContextSlice { return query.contextSlice }

func (query MemberOfQuery) CanonicalBytes() []byte {
	return append([]byte(nil), query.canonicalBytes...)
}

func (query MemberOfQuery) Digest() SHA256Digest { return query.digest }

func (query MemberOfQuery) valid() bool {
	if !query.entity.valid() ||
		!query.valueKind.valid() ||
		!validCompleteContextSlice(query.contextSlice) ||
		!query.digest.valid() ||
		len(query.canonicalBytes) == 0 {
		return false
	}
	rebuilt, err := NewMemberOfQuery(query.entity, query.valueKind, query.contextSlice)
	if err != nil {
		return false
	}
	return rebuilt.digest == query.digest &&
		bytes.Equal(rebuilt.canonicalBytes, query.canonicalBytes)
}

func validCompleteContextSlice(contextSlice ContextSlice) bool {
	return contextSlice.valid()
}

// MemberOfEvaluationView is the sealed EntitySet observation pin used to
// complete Haft's local extension of U.ContextSlice for C.3.2 evaluation. A
// persisted snapshot and an explicitly prospective batch prefix are different
// slice content and cannot be substituted for one another.
type MemberOfEvaluationView interface {
	Kind() MemberOfEvaluationViewKind
	TypeEnv() TypeEnvRef
	PreStateGraphRevision() GraphRevision
	CanonicalBytes() []byte
	Digest() SHA256Digest
	memberOfEvaluationViewVariant()
}

type MemberOfEvaluationViewKind uint8

const (
	PersistedSnapshotEvaluationView MemberOfEvaluationViewKind = iota + 1
	ProspectiveBatchEvaluationView
)

func (kind MemberOfEvaluationViewKind) String() string {
	switch kind {
	case PersistedSnapshotEvaluationView:
		return "persisted_snapshot"
	case ProspectiveBatchEvaluationView:
		return "prospective_batch"
	default:
		return ""
	}
}

type PersistedSnapshotView struct {
	typeEnv        TypeEnvRef
	graphRevision  GraphRevision
	canonicalBytes []byte
	digest         SHA256Digest
}

func NewPersistedSnapshotView(
	typeEnv TypeEnvRef,
	graphRevision GraphRevision,
) (PersistedSnapshotView, error) {
	if !typeEnv.valid() {
		return PersistedSnapshotView{}, fmt.Errorf("persisted MemberOf view requires an exact TypeEnv")
	}
	writer := canonicalPersistedSnapshotView(typeEnv, graphRevision)
	return PersistedSnapshotView{
		typeEnv:        typeEnv,
		graphRevision:  graphRevision,
		canonicalBytes: writer.bytes(),
		digest:         writer.digest(),
	}, nil
}

func (view PersistedSnapshotView) TypeEnv() TypeEnvRef { return view.typeEnv }

func (PersistedSnapshotView) Kind() MemberOfEvaluationViewKind {
	return PersistedSnapshotEvaluationView
}

func (view PersistedSnapshotView) PreStateGraphRevision() GraphRevision {
	return view.graphRevision
}

func (view PersistedSnapshotView) CanonicalBytes() []byte {
	return append([]byte(nil), view.canonicalBytes...)
}

func (view PersistedSnapshotView) Digest() SHA256Digest { return view.digest }

func (PersistedSnapshotView) memberOfEvaluationViewVariant() {}

func canonicalPersistedSnapshotView(
	typeEnv TypeEnvRef,
	graphRevision GraphRevision,
) canonicalWriter {
	writer := newCanonicalWriter("member-of-evaluation-view.persisted-snapshot.v1")
	writer.addString(typeEnv.String())
	writer.addUint64(graphRevision.Value())
	return writer
}

// OrderedCandidatePrefix carries the exact addressable candidate content as
// well as its digest. A digest alone cannot let an EntitySet evaluator inspect
// all prior declarations, so both are sealed together.
type OrderedCandidatePrefix struct {
	changes        []MemoryChange
	canonicalBytes []byte
	digest         SHA256Digest
}

func ComputeOrderedCandidatePrefix(
	changeSet MemoryChangeSet,
	endExclusive uint64,
) (OrderedCandidatePrefix, error) {
	if !changeSet.valid() {
		return OrderedCandidatePrefix{}, fmt.Errorf("candidate prefix requires a valid MemoryChangeSet")
	}
	if endExclusive > uint64(len(changeSet.changes)) {
		return OrderedCandidatePrefix{}, fmt.Errorf("candidate prefix end exceeds the MemoryChangeSet")
	}
	changes := append([]MemoryChange(nil), changeSet.changes[:endExclusive]...)
	writer, err := canonicalOrderedCandidatePrefix(changes)
	if err != nil {
		return OrderedCandidatePrefix{}, err
	}
	return OrderedCandidatePrefix{
		changes:        changes,
		canonicalBytes: writer.bytes(),
		digest:         writer.digest(),
	}, nil
}

func canonicalOrderedCandidatePrefix(
	changes []MemoryChange,
) (canonicalWriter, error) {
	writer := newCanonicalWriter("member-of-ordered-candidate-prefix.v1")
	writer.addUint64(uint64(len(changes)))
	for ordinal, change := range changes {
		if !validMemoryChangeVariant(change) {
			return canonicalWriter{}, fmt.Errorf("candidate prefix contains invalid change %d", ordinal)
		}
		encoded, err := canonicalMemoryChange(change)
		if err != nil {
			return canonicalWriter{}, err
		}
		ordinalValue, exact := exactUint64FromNonNegativeInt(ordinal)
		if !exact {
			return canonicalWriter{}, fmt.Errorf(
				"candidate prefix change ordinal %d exceeds the canonical uint64 range",
				ordinal,
			)
		}
		writer.addUint64(ordinalValue)
		writer.addBytes(encoded)
	}
	return writer, nil
}

func (prefix OrderedCandidatePrefix) Changes() []MemoryChange {
	return append([]MemoryChange(nil), prefix.changes...)
}

func (prefix OrderedCandidatePrefix) CanonicalBytes() []byte {
	return append([]byte(nil), prefix.canonicalBytes...)
}

func (prefix OrderedCandidatePrefix) Digest() SHA256Digest { return prefix.digest }

func (prefix OrderedCandidatePrefix) valid() bool {
	writer, err := canonicalOrderedCandidatePrefix(prefix.changes)
	return err == nil &&
		prefix.digest.valid() &&
		writer.digest() == prefix.digest &&
		bytes.Equal(writer.bytes(), prefix.canonicalBytes)
}

type ProspectiveBatchViewInput struct {
	TypeEnv                  TypeEnvRef
	PreStateGraphRevision    GraphRevision
	EvaluationChangeOrdinal  uint64
	DeclarationChangeOrdinal uint64
	Declaration              DeclareEntity
	LocalReference           LocalRef
	PersistedReference       PersistedRef
	OrderedCandidatePrefix   OrderedCandidatePrefix
}

// ProspectiveBatchView makes a prior DeclareEntity candidate observable to a
// MemberOf evaluator without pretending that it already exists in persisted
// state. The complete candidate bytes, provenance, lowering and ordered prefix
// are committed to the view.
type ProspectiveBatchView struct {
	typeEnv                  TypeEnvRef
	graphRevision            GraphRevision
	evaluationChangeOrdinal  uint64
	declarationChangeOrdinal uint64
	declaration              DeclareEntity
	declarationBytes         []byte
	declarationDigest        SHA256Digest
	localReference           LocalRef
	persistedReference       PersistedRef
	orderedCandidatePrefix   OrderedCandidatePrefix
	canonicalBytes           []byte
	digest                   SHA256Digest
}

func NewProspectiveBatchView(
	input ProspectiveBatchViewInput,
) (ProspectiveBatchView, error) {
	if !input.TypeEnv.valid() {
		return ProspectiveBatchView{}, fmt.Errorf("prospective MemberOf view requires an exact TypeEnv")
	}
	if !input.Declaration.validMemoryChange() ||
		!validStrongRef(input.LocalReference) ||
		!validStrongRef(input.PersistedReference) ||
		!input.OrderedCandidatePrefix.valid() {
		return ProspectiveBatchView{}, fmt.Errorf("prospective MemberOf view requires exact declaration, lowering, and prefix basis")
	}
	if input.DeclarationChangeOrdinal >= input.EvaluationChangeOrdinal {
		return ProspectiveBatchView{}, fmt.Errorf("prospective declaration must precede relation evaluation")
	}
	if uint64(len(input.OrderedCandidatePrefix.changes)) != input.EvaluationChangeOrdinal {
		return ProspectiveBatchView{}, fmt.Errorf("prospective candidate prefix does not end at the relation evaluation ordinal")
	}
	prefixDeclaration, declared := input.OrderedCandidatePrefix.changes[input.DeclarationChangeOrdinal].(DeclareEntity)
	if !declared {
		return ProspectiveBatchView{}, fmt.Errorf("prospective declaration ordinal does not contain DeclareEntity")
	}
	prefixDeclarationBytes, err := prefixDeclaration.CanonicalBytes()
	if err != nil {
		return ProspectiveBatchView{}, fmt.Errorf("canonicalize prospective prefix declaration: %w", err)
	}
	inputDeclarationBytes, err := input.Declaration.CanonicalBytes()
	if err != nil {
		return ProspectiveBatchView{}, fmt.Errorf("canonicalize prospective declaration: %w", err)
	}
	if !bytes.Equal(prefixDeclarationBytes, inputDeclarationBytes) {
		return ProspectiveBatchView{}, fmt.Errorf("prospective declaration does not match the exact ordered candidate prefix")
	}
	if input.LocalReference.BatchLocalRef() != input.Declaration.LocalRef() {
		return ProspectiveBatchView{}, fmt.Errorf("prospective local reference does not name the declaration")
	}
	if input.LocalReference.RefKind().TypeEnv() != input.TypeEnv ||
		input.PersistedReference.RefKind().TypeEnv() != input.TypeEnv {
		return ProspectiveBatchView{}, fmt.Errorf("prospective reference lowering belongs to another TypeEnv")
	}
	if input.LocalReference.RefKind() != input.PersistedReference.RefKind() {
		return ProspectiveBatchView{}, fmt.Errorf("prospective reference lowering changed RefKind")
	}
	if input.PersistedReference.ReferenceID().String() != input.Declaration.Entity().String() {
		return ProspectiveBatchView{}, fmt.Errorf("prospective persisted reference is not the declared stable EntityID")
	}
	declarationBytes := inputDeclarationBytes
	declarationDigest, err := input.Declaration.Digest()
	if err != nil {
		return ProspectiveBatchView{}, fmt.Errorf("digest prospective declaration: %w", err)
	}
	writer := canonicalProspectiveBatchView(
		input.TypeEnv,
		input.PreStateGraphRevision,
		input.EvaluationChangeOrdinal,
		input.DeclarationChangeOrdinal,
		declarationBytes,
		declarationDigest,
		input.Declaration.Provenance(),
		input.LocalReference,
		input.PersistedReference,
		input.OrderedCandidatePrefix,
	)
	return ProspectiveBatchView{
		typeEnv:                  input.TypeEnv,
		graphRevision:            input.PreStateGraphRevision,
		evaluationChangeOrdinal:  input.EvaluationChangeOrdinal,
		declarationChangeOrdinal: input.DeclarationChangeOrdinal,
		declaration:              input.Declaration,
		declarationBytes:         append([]byte(nil), declarationBytes...),
		declarationDigest:        declarationDigest,
		localReference:           input.LocalReference,
		persistedReference:       input.PersistedReference,
		orderedCandidatePrefix:   input.OrderedCandidatePrefix,
		canonicalBytes:           writer.bytes(),
		digest:                   writer.digest(),
	}, nil
}

func (view ProspectiveBatchView) TypeEnv() TypeEnvRef { return view.typeEnv }

func (ProspectiveBatchView) Kind() MemberOfEvaluationViewKind {
	return ProspectiveBatchEvaluationView
}

func (view ProspectiveBatchView) PreStateGraphRevision() GraphRevision {
	return view.graphRevision
}

func (view ProspectiveBatchView) EvaluationChangeOrdinal() uint64 {
	return view.evaluationChangeOrdinal
}

func (view ProspectiveBatchView) DeclarationChangeOrdinal() uint64 {
	return view.declarationChangeOrdinal
}

func (view ProspectiveBatchView) Declaration() DeclareEntity { return view.declaration }

func (view ProspectiveBatchView) DeclarationCanonicalBytes() []byte {
	return append([]byte(nil), view.declarationBytes...)
}

func (view ProspectiveBatchView) DeclarationDigest() SHA256Digest {
	return view.declarationDigest
}

func (view ProspectiveBatchView) LocalReference() LocalRef { return view.localReference }

func (view ProspectiveBatchView) PersistedReference() PersistedRef {
	return view.persistedReference
}

func (view ProspectiveBatchView) OrderedCandidatePrefix() OrderedCandidatePrefix {
	return view.orderedCandidatePrefix
}

func (view ProspectiveBatchView) CanonicalBytes() []byte {
	return append([]byte(nil), view.canonicalBytes...)
}

func (view ProspectiveBatchView) Digest() SHA256Digest { return view.digest }

func (ProspectiveBatchView) memberOfEvaluationViewVariant() {}

func canonicalProspectiveBatchView(
	typeEnv TypeEnvRef,
	graphRevision GraphRevision,
	evaluationChangeOrdinal uint64,
	declarationChangeOrdinal uint64,
	declarationBytes []byte,
	declarationDigest SHA256Digest,
	declarationProvenance ProvenanceRef,
	localReference LocalRef,
	persistedReference PersistedRef,
	orderedCandidatePrefix OrderedCandidatePrefix,
) canonicalWriter {
	writer := newCanonicalWriter("member-of-evaluation-view.prospective-batch.v1")
	writer.addString(typeEnv.String())
	writer.addUint64(graphRevision.Value())
	writer.addUint64(evaluationChangeOrdinal)
	writer.addUint64(declarationChangeOrdinal)
	writer.addBytes(declarationBytes)
	writer.addString(declarationDigest.String())
	writer.addString(declarationProvenance.String())
	writer.addString(localReference.RefKind().String())
	writer.addString(localReference.BatchLocalRef().String())
	writer.addString(persistedReference.RefKind().String())
	writer.addString(persistedReference.ReferenceID().String())
	writer.addUint64(uint64(len(orderedCandidatePrefix.changes)))
	writer.addString(orderedCandidatePrefix.Digest().String())
	return writer
}

func validMemberOfEvaluationView(view MemberOfEvaluationView) bool {
	switch value := view.(type) {
	case PersistedSnapshotView:
		if !value.typeEnv.valid() || !value.digest.valid() || len(value.canonicalBytes) == 0 {
			return false
		}
		writer := canonicalPersistedSnapshotView(value.typeEnv, value.graphRevision)
		return canonicalValueMatches(writer, value.canonicalBytes, value.digest)
	case ProspectiveBatchView:
		if !value.typeEnv.valid() ||
			!value.declaration.validMemoryChange() ||
			!validStrongRef(value.localReference) ||
			!validStrongRef(value.persistedReference) ||
			!value.orderedCandidatePrefix.valid() ||
			!value.declarationDigest.valid() ||
			value.declarationChangeOrdinal >= value.evaluationChangeOrdinal {
			return false
		}
		rebuilt, err := NewProspectiveBatchView(ProspectiveBatchViewInput{
			TypeEnv:                  value.typeEnv,
			PreStateGraphRevision:    value.graphRevision,
			EvaluationChangeOrdinal:  value.evaluationChangeOrdinal,
			DeclarationChangeOrdinal: value.declarationChangeOrdinal,
			Declaration:              value.declaration,
			LocalReference:           value.localReference,
			PersistedReference:       value.persistedReference,
			OrderedCandidatePrefix:   value.orderedCandidatePrefix,
		})
		return err == nil &&
			rebuilt.declarationDigest == value.declarationDigest &&
			bytes.Equal(rebuilt.declarationBytes, value.declarationBytes) &&
			rebuilt.digest == value.digest &&
			bytes.Equal(rebuilt.canonicalBytes, value.canonicalBytes)
	default:
		return false
	}
}

func sameMemberOfEvaluationView(left, right MemberOfEvaluationView) bool {
	return validMemberOfEvaluationView(left) &&
		validMemberOfEvaluationView(right) &&
		left.Digest() == right.Digest() &&
		bytes.Equal(left.CanonicalBytes(), right.CanonicalBytes())
}

// MemberOfEvaluationRequest is the executable C.3.2 predicate input. Its
// canonical tuple joins EntityID and ValueKindRef with Haft's complete local
// slice extension: the base U.ContextSlice plus one exact addressable
// EntitySet observation pin. The view is therefore slice content, not a hidden
// fourth coordinate.
type MemberOfEvaluationRequest struct {
	query          MemberOfQuery
	view           MemberOfEvaluationView
	canonicalBytes []byte
	digest         SHA256Digest
}

func NewMemberOfEvaluationRequest(
	query MemberOfQuery,
	view MemberOfEvaluationView,
) (MemberOfEvaluationRequest, error) {
	if !query.valid() || !validMemberOfEvaluationView(view) {
		return MemberOfEvaluationRequest{}, fmt.Errorf("MemberOf evaluation request requires an exact query and view")
	}
	if query.ValueKind().TypeEnv() != view.TypeEnv() {
		return MemberOfEvaluationRequest{}, fmt.Errorf("MemberOf query and evaluation view belong to different TypeEnvs")
	}
	if prospective, ok := view.(ProspectiveBatchView); ok {
		if query.EntityID() != prospective.Declaration().Entity() ||
			query.ContextSlice().Context() != prospective.Declaration().Context() {
			return MemberOfEvaluationRequest{}, fmt.Errorf("prospective MemberOf view does not declare the queried entity in the queried context")
		}
	}
	writer := newCanonicalWriter("member-of-evaluation-request.v1")
	writer.addBytes(query.CanonicalBytes())
	writer.addBytes(view.CanonicalBytes())
	return MemberOfEvaluationRequest{
		query:          query,
		view:           view,
		canonicalBytes: writer.bytes(),
		digest:         writer.digest(),
	}, nil
}

func (request MemberOfEvaluationRequest) Query() MemberOfQuery { return request.query }

func (request MemberOfEvaluationRequest) View() MemberOfEvaluationView { return request.view }

func (request MemberOfEvaluationRequest) CanonicalBytes() []byte {
	return append([]byte(nil), request.canonicalBytes...)
}

func (request MemberOfEvaluationRequest) Digest() SHA256Digest { return request.digest }

func (request MemberOfEvaluationRequest) valid() bool {
	rebuilt, err := NewMemberOfEvaluationRequest(request.query, request.view)
	return err == nil &&
		rebuilt.digest == request.digest &&
		bytes.Equal(rebuilt.canonicalBytes, request.canonicalBytes)
}

// ObservableInputRef names one observable consumed by a membership evaluator.
// Its bytes are fixed separately by MemberOfObservableInput.Digest.
type ObservableInputRef struct {
	value string
}

func NewObservableInputRef(raw string) (ObservableInputRef, error) {
	value, err := parseOpaqueIdentifier("MemberOf observable input reference", raw)
	if err != nil {
		return ObservableInputRef{}, err
	}
	return ObservableInputRef{value: value}, nil
}

func (ref ObservableInputRef) String() string { return ref.value }

func (ref ObservableInputRef) valid() bool { return ref.value != "" }

type MemberOfObservableInput struct {
	reference ObservableInputRef
	digest    SHA256Digest
}

func NewMemberOfObservableInput(
	reference ObservableInputRef,
	digest SHA256Digest,
) (MemberOfObservableInput, error) {
	if !reference.valid() {
		return MemberOfObservableInput{}, fmt.Errorf("MemberOf observable input reference is required")
	}
	if !digest.valid() {
		return MemberOfObservableInput{}, fmt.Errorf("MemberOf observable input digest is required")
	}
	return MemberOfObservableInput{reference: reference, digest: digest}, nil
}

func (input MemberOfObservableInput) Reference() ObservableInputRef {
	return input.reference
}

func (input MemberOfObservableInput) Digest() SHA256Digest { return input.digest }

func (input MemberOfObservableInput) CanonicalBytes() []byte {
	writer := newCanonicalWriter("member-of-observable-input.v1")
	writer.addString(input.reference.String())
	writer.addString(input.digest.String())
	return writer.bytes()
}

func (input MemberOfObservableInput) valid() bool {
	return input.reference.valid() && input.digest.valid()
}

type MemberOfEvaluationProvenanceInput struct {
	Reference         ProvenanceRef
	EvaluatorArtifact CarrierRef
	EvaluatorEdition  CarrierEdition
	EvaluatorDigest   SHA256Digest
}

// MemberOfEvaluationProvenance identifies the exact evaluator implementation
// that applied the declared RuleRef. It is semantic replay provenance, not
// run-time evidence or a confidence score.
type MemberOfEvaluationProvenance struct {
	reference         ProvenanceRef
	evaluatorArtifact CarrierRef
	evaluatorEdition  CarrierEdition
	evaluatorDigest   SHA256Digest
}

func NewMemberOfEvaluationProvenance(
	input MemberOfEvaluationProvenanceInput,
) (MemberOfEvaluationProvenance, error) {
	if !input.Reference.valid() {
		return MemberOfEvaluationProvenance{}, fmt.Errorf("MemberOf evaluation provenance reference is required")
	}
	if !input.EvaluatorArtifact.valid() {
		return MemberOfEvaluationProvenance{}, fmt.Errorf("MemberOf evaluator artifact is required")
	}
	if !input.EvaluatorEdition.valid() || implicitContextSelector(input.EvaluatorEdition.String()) {
		return MemberOfEvaluationProvenance{}, fmt.Errorf("MemberOf evaluator edition must be exact")
	}
	if !input.EvaluatorDigest.valid() {
		return MemberOfEvaluationProvenance{}, fmt.Errorf("MemberOf evaluator digest is required")
	}
	return MemberOfEvaluationProvenance{
		reference:         input.Reference,
		evaluatorArtifact: input.EvaluatorArtifact,
		evaluatorEdition:  input.EvaluatorEdition,
		evaluatorDigest:   input.EvaluatorDigest,
	}, nil
}

func (provenance MemberOfEvaluationProvenance) Reference() ProvenanceRef {
	return provenance.reference
}

func (provenance MemberOfEvaluationProvenance) EvaluatorArtifact() CarrierRef {
	return provenance.evaluatorArtifact
}

func (provenance MemberOfEvaluationProvenance) EvaluatorEdition() CarrierEdition {
	return provenance.evaluatorEdition
}

func (provenance MemberOfEvaluationProvenance) EvaluatorDigest() SHA256Digest {
	return provenance.evaluatorDigest
}

func (provenance MemberOfEvaluationProvenance) CanonicalBytes() []byte {
	writer := newCanonicalWriter("member-of-evaluation-provenance.v1")
	writer.addString(provenance.reference.String())
	writer.addString(provenance.evaluatorArtifact.String())
	writer.addString(provenance.evaluatorEdition.String())
	writer.addString(provenance.evaluatorDigest.String())
	return writer.bytes()
}

func (provenance MemberOfEvaluationProvenance) valid() bool {
	return provenance.reference.valid() &&
		provenance.evaluatorArtifact.valid() &&
		provenance.evaluatorEdition.valid() &&
		!implicitContextSelector(provenance.evaluatorEdition.String()) &&
		provenance.evaluatorDigest.valid()
}

type MemberOfBasisVersion uint8

const (
	LegacyMemberOfBasisVersionV2 MemberOfBasisVersion = iota + 1
	C32PrerequisiteMemberOfBasisVersionV3
)

func (version MemberOfBasisVersion) String() string {
	switch version {
	case LegacyMemberOfBasisVersionV2:
		return "legacy_v2"
	case C32PrerequisiteMemberOfBasisVersionV3:
		return "c32_prerequisite_v3"
	default:
		return ""
	}
}

// MemberOfBasisPosture keeps historical v2 replay and prerequisite-bound v3
// construction as distinct variants. A legacy basis cannot expose a zero or
// optional prerequisite certificate.
type MemberOfBasisPosture interface {
	Version() MemberOfBasisVersion
	memberOfBasisPostureVariant()
}

type LegacyMemberOfBasisV2 struct{}

func (LegacyMemberOfBasisV2) Version() MemberOfBasisVersion {
	return LegacyMemberOfBasisVersionV2
}

func (LegacyMemberOfBasisV2) memberOfBasisPostureVariant() {}

type C32PrerequisiteMemberOfBasisV3 struct {
	certificate C32PrerequisiteCertificate
}

func NewC32PrerequisiteMemberOfBasisV3(
	certificate C32PrerequisiteCertificate,
) (C32PrerequisiteMemberOfBasisV3, error) {
	if !certificate.valid() {
		return C32PrerequisiteMemberOfBasisV3{}, fmt.Errorf(
			"MemberOf v3 posture requires an exact C.3.2 prerequisite certificate",
		)
	}
	return C32PrerequisiteMemberOfBasisV3{certificate: certificate}, nil
}

func (C32PrerequisiteMemberOfBasisV3) Version() MemberOfBasisVersion {
	return C32PrerequisiteMemberOfBasisVersionV3
}

func (posture C32PrerequisiteMemberOfBasisV3) Certificate() C32PrerequisiteCertificate {
	return posture.certificate
}

func (C32PrerequisiteMemberOfBasisV3) memberOfBasisPostureVariant() {}

func validMemberOfBasisPosture(posture MemberOfBasisPosture) bool {
	switch value := posture.(type) {
	case LegacyMemberOfBasisV2:
		return value.Version() == LegacyMemberOfBasisVersionV2
	case C32PrerequisiteMemberOfBasisV3:
		return value.certificate.valid()
	default:
		return false
	}
}

type MemberOfBasisInput struct {
	Query                MemberOfQuery
	EvaluationView       MemberOfEvaluationView
	KindSignature        KindSignatureDefinition
	EntitySet            EntitySetDefinition
	ObservableInputs     []MemberOfObservableInput
	EvaluationProvenance MemberOfEvaluationProvenance
}

type MemberOfBasisV3Input struct {
	Basis         MemberOfBasisInput
	Prerequisites C32PrerequisiteCertificate
}

// MemberOfBasis is the complete semantic replay basis for a defined membership
// result. It binds the TypeEnv, KindSignature, EntitySet definition, evaluator
// rule, exact observable content references and digests, ContextSlice, and
// evaluator implementation. Durable byte availability belongs to the storage
// layer behind each observable reference; it is not duplicated inline here.
type MemberOfBasis struct {
	posture               MemberOfBasisPosture
	memberOfRequestDigest SHA256Digest
	typeEnv               TypeEnvRef
	kindSignature         KindSignatureRef
	entitySet             EntitySetDefinitionRef
	evaluator             RuleRef
	observableInputs      []MemberOfObservableInput
	contextSlice          ContextSliceRef
	evaluationView        MemberOfEvaluationView
	evaluationProvenance  MemberOfEvaluationProvenance
	canonicalBytes        []byte
	digest                SHA256Digest
}

func NewMemberOfBasis(input MemberOfBasisInput) (MemberOfBasis, error) {
	return NewLegacyMemberOfBasisV2(input)
}

// NewLegacyMemberOfBasisV2 preserves the historical v2 canonical contract.
// New selected-TypeEnv engines must use NewMemberOfBasisV3 instead.
func NewLegacyMemberOfBasisV2(input MemberOfBasisInput) (MemberOfBasis, error) {
	if !input.Query.valid() {
		return MemberOfBasis{}, fmt.Errorf("MemberOf basis query is required")
	}
	request, err := NewMemberOfEvaluationRequest(input.Query, input.EvaluationView)
	if err != nil {
		return MemberOfBasis{}, fmt.Errorf("MemberOf basis evaluation view: %w", err)
	}
	if !input.KindSignature.valid() {
		return MemberOfBasis{}, fmt.Errorf("MemberOf KindSignature basis is required")
	}
	if input.KindSignature.ValueKind() != input.Query.ValueKind() {
		return MemberOfBasis{}, fmt.Errorf("MemberOf KindSignature does not match the queried ValueKind")
	}
	if input.KindSignature.Ref().Context() != input.Query.ContextSlice().Context() {
		return MemberOfBasis{}, fmt.Errorf("MemberOf KindSignature context does not match the ContextSlice")
	}
	if !input.EntitySet.valid() {
		return MemberOfBasis{}, fmt.Errorf("MemberOf EntitySet definition basis is required")
	}
	if input.EntitySet.Ref().TypeEnv() != input.Query.ValueKind().TypeEnv() {
		return MemberOfBasis{}, fmt.Errorf("MemberOf EntitySet belongs to another TypeEnv")
	}
	if input.EntitySet.Ref().Context() != input.Query.ContextSlice().Context() {
		return MemberOfBasis{}, fmt.Errorf("MemberOf EntitySet context does not match the ContextSlice")
	}
	if input.KindSignature.EntitySet() != input.EntitySet.Ref() {
		return MemberOfBasis{}, fmt.Errorf("MemberOf KindSignature and EntitySet definitions do not match")
	}
	if _, prospective := request.View().(ProspectiveBatchView); prospective {
		if _, visible := input.EntitySet.CandidatePolicy().(PriorBatchDeclarationsVisible); !visible {
			return MemberOfBasis{}, fmt.Errorf("MemberOf prospective basis requires an EntitySet that explicitly admits prior batch declarations")
		}
	}
	if !input.EvaluationProvenance.valid() {
		return MemberOfBasis{}, fmt.Errorf("MemberOf evaluation provenance is required")
	}
	inputs, err := normalizeMemberOfObservableInputs(input.ObservableInputs)
	if err != nil {
		return MemberOfBasis{}, err
	}
	if len(inputs) == 0 {
		return MemberOfBasis{}, fmt.Errorf("MemberOf basis requires at least one observable input")
	}
	typeEnv := input.Query.ValueKind().TypeEnv()
	contextSlice := input.Query.ContextSlice().Ref()
	kindSignature := input.KindSignature.Ref()
	entitySet := input.EntitySet.Ref()
	evaluator := input.KindSignature.Evaluator()
	writer := canonicalMemberOfBasis(
		typeEnv,
		kindSignature,
		entitySet,
		evaluator,
		inputs,
		contextSlice,
		input.EvaluationView,
		input.EvaluationProvenance,
	)
	return MemberOfBasis{
		posture:              LegacyMemberOfBasisV2{},
		typeEnv:              typeEnv,
		kindSignature:        kindSignature,
		entitySet:            entitySet,
		evaluator:            evaluator,
		observableInputs:     inputs,
		contextSlice:         contextSlice,
		evaluationView:       input.EvaluationView,
		evaluationProvenance: input.EvaluationProvenance,
		canonicalBytes:       writer.bytes(),
		digest:               writer.digest(),
	}, nil
}

// NewMemberOfBasisV3 requires the exact content-addressed result of the
// EntitySetEnumeration -> KindDefinedness prerequisite chain. It never upgrades
// or reinterprets a v2 basis in place.
func NewMemberOfBasisV3(input MemberOfBasisV3Input) (MemberOfBasis, error) {
	basis, err := NewLegacyMemberOfBasisV2(input.Basis)
	if err != nil {
		return MemberOfBasis{}, err
	}
	request, err := NewMemberOfEvaluationRequest(
		input.Basis.Query,
		input.Basis.EvaluationView,
	)
	if err != nil {
		return MemberOfBasis{}, fmt.Errorf(
			"MemberOf v3 evaluation request: %w",
			err,
		)
	}
	if !c32PrerequisitesMatchMemberOfBasis(
		input.Prerequisites,
		request,
		input.Basis.KindSignature,
		input.Basis.EntitySet,
	) {
		return MemberOfBasis{}, fmt.Errorf(
			"MemberOf v3 prerequisite certificate does not match the exact query, signature, EntitySet, rules, and view",
		)
	}
	posture, err := NewC32PrerequisiteMemberOfBasisV3(input.Prerequisites)
	if err != nil {
		return MemberOfBasis{}, err
	}
	writer := canonicalMemberOfBasisV3(
		basis.typeEnv,
		basis.kindSignature,
		basis.entitySet,
		basis.evaluator,
		basis.observableInputs,
		basis.contextSlice,
		basis.evaluationView,
		basis.evaluationProvenance,
		input.Prerequisites,
	)
	basis.posture = posture
	basis.memberOfRequestDigest = request.Digest()
	basis.canonicalBytes = writer.bytes()
	basis.digest = writer.digest()
	return basis, nil
}

func c32PrerequisitesMatchMemberOfBasis(
	certificate C32PrerequisiteCertificate,
	request MemberOfEvaluationRequest,
	signature KindSignatureDefinition,
	entitySet EntitySetDefinition,
) bool {
	if !certificate.valid() || !request.valid() {
		return false
	}
	query := request.Query()
	if certificate.TypeEnv() != query.ValueKind().TypeEnv() ||
		certificate.KindSignature() != signature.Ref() ||
		certificate.EntitySet() != entitySet.Ref() ||
		certificate.ContextSlice() != query.ContextSlice().Ref() ||
		!sameMemberOfEvaluationView(
			certificate.EvaluationView(),
			request.View(),
		) ||
		certificate.MemberOfRequestDigest() != request.Digest() ||
		certificate.EnumerationRule() != entitySet.EnumerationRule() ||
		certificate.DefinednessRule() != signature.DefinednessRule() {
		return false
	}
	switch request.View().(type) {
	case PersistedSnapshotView:
		_, ok := certificate.CandidateVisibility().(C32PersistedVisibilityCoordinate)
		return ok
	case ProspectiveBatchView:
		coordinate, ok := certificate.CandidateVisibility().(C32ProspectiveVisibilityCoordinate)
		if !ok {
			return false
		}
		policy, ok := entitySet.CandidatePolicy().(PriorBatchDeclarationsVisible)
		return ok && coordinate.Rule() == policy.EvaluationRule()
	default:
		return false
	}
}

func (basis MemberOfBasis) Posture() MemberOfBasisPosture { return basis.posture }

func (basis MemberOfBasis) MemberOfRequestDigest() (SHA256Digest, bool) {
	if _, ok := basis.posture.(C32PrerequisiteMemberOfBasisV3); !ok {
		return SHA256Digest{}, false
	}
	return basis.memberOfRequestDigest, true
}

func (basis MemberOfBasis) TypeEnv() TypeEnvRef { return basis.typeEnv }

func (basis MemberOfBasis) KindSignature() KindSignatureRef {
	return basis.kindSignature
}

func (basis MemberOfBasis) EntitySet() EntitySetDefinitionRef { return basis.entitySet }

func (basis MemberOfBasis) Evaluator() RuleRef { return basis.evaluator }

func (basis MemberOfBasis) ObservableInputs() []MemberOfObservableInput {
	return append([]MemberOfObservableInput(nil), basis.observableInputs...)
}

func (basis MemberOfBasis) ContextSlice() ContextSliceRef { return basis.contextSlice }

func (basis MemberOfBasis) EvaluationView() MemberOfEvaluationView {
	return basis.evaluationView
}

func (basis MemberOfBasis) EvaluationProvenance() MemberOfEvaluationProvenance {
	return basis.evaluationProvenance
}

func (basis MemberOfBasis) CanonicalBytes() []byte {
	return append([]byte(nil), basis.canonicalBytes...)
}

func (basis MemberOfBasis) Digest() SHA256Digest { return basis.digest }

func (basis MemberOfBasis) valid() bool {
	if !validMemberOfBasisPosture(basis.posture) ||
		!basis.typeEnv.valid() ||
		!basis.kindSignature.valid() ||
		!basis.entitySet.valid() ||
		basis.kindSignature.TypeEnv() != basis.typeEnv ||
		basis.entitySet.TypeEnv() != basis.typeEnv ||
		basis.kindSignature.Context() != basis.entitySet.Context() ||
		!basis.evaluator.valid() ||
		!basis.contextSlice.valid() ||
		!validMemberOfEvaluationView(basis.evaluationView) ||
		basis.evaluationView.TypeEnv() != basis.typeEnv ||
		!basis.evaluationProvenance.valid() ||
		!basis.digest.valid() ||
		len(basis.canonicalBytes) == 0 {
		return false
	}
	inputs, err := normalizeMemberOfObservableInputs(basis.observableInputs)
	if err != nil || len(inputs) == 0 || len(inputs) != len(basis.observableInputs) {
		return false
	}
	switch posture := basis.posture.(type) {
	case LegacyMemberOfBasisV2:
		if basis.memberOfRequestDigest.valid() {
			return false
		}
		writer := canonicalMemberOfBasis(
			basis.typeEnv,
			basis.kindSignature,
			basis.entitySet,
			basis.evaluator,
			inputs,
			basis.contextSlice,
			basis.evaluationView,
			basis.evaluationProvenance,
		)
		return writer.digest() == basis.digest &&
			bytes.Equal(writer.bytes(), basis.canonicalBytes)
	case C32PrerequisiteMemberOfBasisV3:
		certificate := posture.Certificate()
		if !basis.memberOfRequestDigest.valid() ||
			certificate.MemberOfRequestDigest() != basis.memberOfRequestDigest ||
			certificate.TypeEnv() != basis.typeEnv ||
			certificate.KindSignature() != basis.kindSignature ||
			certificate.EntitySet() != basis.entitySet ||
			certificate.ContextSlice() != basis.contextSlice ||
			!sameMemberOfEvaluationView(
				certificate.EvaluationView(),
				basis.evaluationView,
			) {
			return false
		}
		writer := canonicalMemberOfBasisV3(
			basis.typeEnv,
			basis.kindSignature,
			basis.entitySet,
			basis.evaluator,
			inputs,
			basis.contextSlice,
			basis.evaluationView,
			basis.evaluationProvenance,
			certificate,
		)
		return writer.digest() == basis.digest &&
			bytes.Equal(writer.bytes(), basis.canonicalBytes)
	default:
		return false
	}
}

func canonicalMemberOfBasis(
	typeEnv TypeEnvRef,
	kindSignature KindSignatureRef,
	entitySet EntitySetDefinitionRef,
	evaluator RuleRef,
	observableInputs []MemberOfObservableInput,
	contextSlice ContextSliceRef,
	evaluationView MemberOfEvaluationView,
	evaluationProvenance MemberOfEvaluationProvenance,
) canonicalWriter {
	writer := newCanonicalWriter(memberOfBasisDomain)
	writer.addString(typeEnv.String())
	writer.addString(kindSignature.String())
	writer.addString(entitySet.String())
	writer.addString(evaluator.String())
	addCanonicalMemberOfObservableInputs(&writer, observableInputs)
	writer.addString(contextSlice.String())
	writer.addBytes(evaluationView.CanonicalBytes())
	writer.addBytes(evaluationProvenance.CanonicalBytes())
	return writer
}

func canonicalMemberOfBasisV3(
	typeEnv TypeEnvRef,
	kindSignature KindSignatureRef,
	entitySet EntitySetDefinitionRef,
	evaluator RuleRef,
	observableInputs []MemberOfObservableInput,
	contextSlice ContextSliceRef,
	evaluationView MemberOfEvaluationView,
	evaluationProvenance MemberOfEvaluationProvenance,
	prerequisites C32PrerequisiteCertificate,
) canonicalWriter {
	writer := newCanonicalWriter(memberOfBasisV3Domain)
	writer.addString(typeEnv.String())
	writer.addString(kindSignature.String())
	writer.addString(entitySet.String())
	writer.addString(evaluator.String())
	addCanonicalMemberOfObservableInputs(&writer, observableInputs)
	writer.addString(contextSlice.String())
	writer.addBytes(evaluationView.CanonicalBytes())
	writer.addBytes(evaluationProvenance.CanonicalBytes())
	writer.addBytes(prerequisites.CanonicalBytes())
	return writer
}

func addCanonicalMemberOfObservableInputs(
	writer *canonicalWriter,
	inputs []MemberOfObservableInput,
) {
	writer.addUint64(uint64(len(inputs)))
	for _, input := range inputs {
		writer.addBytes(input.CanonicalBytes())
	}
}

func normalizeMemberOfObservableInputs(
	values []MemberOfObservableInput,
) ([]MemberOfObservableInput, error) {
	result := append([]MemberOfObservableInput(nil), values...)
	sort.Slice(result, func(left, right int) bool {
		leftRef := result[left].Reference().String()
		rightRef := result[right].Reference().String()
		if leftRef != rightRef {
			return leftRef < rightRef
		}
		return result[left].Digest().String() < result[right].Digest().String()
	})
	normalized := make([]MemberOfObservableInput, 0, len(result))
	for _, input := range result {
		if !input.valid() {
			return nil, fmt.Errorf("MemberOf observable input is invalid")
		}
		if len(normalized) == 0 {
			normalized = append(normalized, input)
			continue
		}
		previous := normalized[len(normalized)-1]
		if previous.Reference() != input.Reference() {
			normalized = append(normalized, input)
			continue
		}
		if previous.Digest() == input.Digest() {
			continue
		}
		return nil, fmt.Errorf(
			"MemberOf observable input %q has conflicting digests",
			input.Reference().String(),
		)
	}
	return normalized, nil
}

// ComputeMemberOfObservableInputSetDigest projects the exact normalized input
// set consumed by MemberOfBasis. Ordering, identical-row deduplication,
// conflicting-digest rejection, and per-input canonical bytes are shared with
// the basis constructor rather than recreated by persistence adapters.
func ComputeMemberOfObservableInputSetDigest(
	values []MemberOfObservableInput,
) (SHA256Digest, error) {
	normalized, err := normalizeMemberOfObservableInputs(values)
	if err != nil {
		return SHA256Digest{}, err
	}
	if len(normalized) == 0 {
		return SHA256Digest{}, fmt.Errorf("MemberOf observable input set requires at least one input")
	}
	writer := newCanonicalWriter("member-of-observable-input-set.v1")
	addCanonicalMemberOfObservableInputs(&writer, normalized)
	return writer.digest(), nil
}

type MemberOfMissingBasisKind uint8

const (
	MissingMemberOfTypeEnv MemberOfMissingBasisKind = iota + 1
	MissingMemberOfKindSignature
	MissingMemberOfEntitySet
	MissingMemberOfEvaluator
	MissingMemberOfObservableInput
	MissingMemberOfEvaluationProvenance
	MissingMemberOfCandidateVisibility
	NoApplicableMemberOfObservableSource
	MissingMemberOfUniqueTrustedObservableSource
)

func (kind MemberOfMissingBasisKind) String() string {
	switch kind {
	case MissingMemberOfTypeEnv:
		return "typeenv"
	case MissingMemberOfKindSignature:
		return "kind_signature"
	case MissingMemberOfEntitySet:
		return "entity_set"
	case MissingMemberOfEvaluator:
		return "evaluator"
	case MissingMemberOfObservableInput:
		return "observable_input"
	case MissingMemberOfEvaluationProvenance:
		return "evaluation_provenance"
	case MissingMemberOfCandidateVisibility:
		return "candidate_visibility"
	case NoApplicableMemberOfObservableSource:
		return "no_applicable_observable_source"
	case MissingMemberOfUniqueTrustedObservableSource:
		return "unique_trusted_observable_source"
	default:
		return ""
	}
}

func (kind MemberOfMissingBasisKind) valid() bool { return kind.String() != "" }

// MemberOfMissingBasis is constructed only through strong, position-specific
// constructors below, so an undefined judgement cannot carry an untyped
// arbitrary reason string.
type MemberOfMissingBasis struct {
	kind    MemberOfMissingBasisKind
	subject string
}

func MissingTypeEnvForMemberOf(typeEnv TypeEnvRef) (MemberOfMissingBasis, error) {
	if !typeEnv.valid() {
		return MemberOfMissingBasis{}, fmt.Errorf("missing MemberOf TypeEnv reference is required")
	}
	return newMemberOfMissingBasis(MissingMemberOfTypeEnv, typeEnv.String())
}

func MissingKindSignatureForMemberOf(
	query MemberOfQuery,
) (MemberOfMissingBasis, error) {
	if !query.valid() {
		return MemberOfMissingBasis{}, fmt.Errorf("missing MemberOf KindSignature query is required")
	}
	subject := query.ValueKind().String() +
		"/context/" + query.ContextSlice().Context().String()
	return newMemberOfMissingBasis(MissingMemberOfKindSignature, subject)
}

func MissingEntitySetForMemberOf(
	entitySet EntitySetDefinitionRef,
) (MemberOfMissingBasis, error) {
	if !entitySet.valid() {
		return MemberOfMissingBasis{}, fmt.Errorf("missing MemberOf EntitySet definition reference is required")
	}
	return newMemberOfMissingBasis(MissingMemberOfEntitySet, entitySet.String())
}

func MissingEvaluatorForMemberOf(rule RuleRef) (MemberOfMissingBasis, error) {
	if !rule.valid() {
		return MemberOfMissingBasis{}, fmt.Errorf("missing MemberOf evaluator rule is required")
	}
	return newMemberOfMissingBasis(MissingMemberOfEvaluator, rule.String())
}

func MissingObservableInputForMemberOf(
	reference ObservableInputRef,
) (MemberOfMissingBasis, error) {
	if !reference.valid() {
		return MemberOfMissingBasis{}, fmt.Errorf("missing MemberOf observable input reference is required")
	}
	return newMemberOfMissingBasis(MissingMemberOfObservableInput, reference.String())
}

// MissingObservableSourceForMemberOf identifies the absence of one unique
// exact observable source for a complete query. Unlike
// MissingObservableInputForMemberOf, it does not invent a concrete source ref
// when the evaluator-specific catalog selection found none or found several.
func MissingObservableSourceForMemberOf(
	query MemberOfQuery,
) (MemberOfMissingBasis, error) {
	if !query.valid() {
		return MemberOfMissingBasis{}, fmt.Errorf(
			"missing MemberOf observable source query is required",
		)
	}
	subject := "query:" + query.Digest().String()
	return newMemberOfMissingBasis(MissingMemberOfObservableInput, subject)
}

// NoApplicableObservableSourceForMemberOf records the exact open-world
// posture in which an evaluator-specific catalog contains no source whose
// coordinates apply to the complete query. It remains Undefined; this basis
// is only a typed distinction from malformed, untrusted, or ambiguous source
// material and never means NotMember.
func NoApplicableObservableSourceForMemberOf(
	query MemberOfQuery,
) (MemberOfMissingBasis, error) {
	if !query.valid() {
		return MemberOfMissingBasis{}, fmt.Errorf(
			"no-applicable-source MemberOf query is required",
		)
	}
	subject := "query:" + query.Digest().String()
	return newMemberOfMissingBasis(
		NoApplicableMemberOfObservableSource,
		subject,
	)
}

// MissingUniqueTrustedObservableSourceForMemberOf records that source-like
// material was present for the query but could not establish one unique,
// policy-trusted observable source. It deliberately cannot be mistaken for
// the absence posture above.
func MissingUniqueTrustedObservableSourceForMemberOf(
	query MemberOfQuery,
) (MemberOfMissingBasis, error) {
	if !query.valid() {
		return MemberOfMissingBasis{}, fmt.Errorf(
			"unique-trusted-source MemberOf query is required",
		)
	}
	subject := "query:" + query.Digest().String()
	return newMemberOfMissingBasis(
		MissingMemberOfUniqueTrustedObservableSource,
		subject,
	)
}

func MissingEvaluationProvenanceForMemberOf(
	rule RuleRef,
) (MemberOfMissingBasis, error) {
	if !rule.valid() {
		return MemberOfMissingBasis{}, fmt.Errorf("missing MemberOf evaluation provenance rule is required")
	}
	return newMemberOfMissingBasis(MissingMemberOfEvaluationProvenance, rule.String())
}

func MissingCandidateVisibilityForMemberOf(
	entitySet EntitySetDefinitionRef,
) (MemberOfMissingBasis, error) {
	if !entitySet.valid() {
		return MemberOfMissingBasis{}, fmt.Errorf("missing MemberOf candidate-visibility EntitySet reference is required")
	}
	return newMemberOfMissingBasis(MissingMemberOfCandidateVisibility, entitySet.String())
}

func newMemberOfMissingBasis(
	kind MemberOfMissingBasisKind,
	subject string,
) (MemberOfMissingBasis, error) {
	if !kind.valid() {
		return MemberOfMissingBasis{}, fmt.Errorf("MemberOf missing-basis kind is required")
	}
	value, err := parseOpaqueIdentifier("MemberOf missing-basis subject", subject)
	if err != nil {
		return MemberOfMissingBasis{}, err
	}
	return MemberOfMissingBasis{kind: kind, subject: value}, nil
}

func (basis MemberOfMissingBasis) Kind() MemberOfMissingBasisKind { return basis.kind }

func (basis MemberOfMissingBasis) Subject() string { return basis.subject }

func (basis MemberOfMissingBasis) CanonicalBytes() []byte {
	writer := newCanonicalWriter("member-of-missing-basis.v1")
	writer.addString(basis.kind.String())
	writer.addString(basis.subject)
	return writer.bytes()
}

func (basis MemberOfMissingBasis) valid() bool {
	return basis.kind.valid() && basis.subject != ""
}

type MemberOfJudgementKind uint8

const (
	MemberJudgement MemberOfJudgementKind = iota + 1
	NotMemberJudgement
	UndefinedMemberJudgement
)

func (kind MemberOfJudgementKind) String() string {
	switch kind {
	case MemberJudgement:
		return "member"
	case NotMemberJudgement:
		return "not_member"
	case UndefinedMemberJudgement:
		return "undefined"
	default:
		return ""
	}
}

// MemberOfJudgement is a closed tri-state result. Undefined is not false and
// must map to fail-closed Underdetermined at a persistence guard.
type MemberOfJudgement interface {
	Kind() MemberOfJudgementKind
	Query() MemberOfQuery
	EvaluationRequest() MemberOfEvaluationRequest
	CanonicalBytes() []byte
	Digest() SHA256Digest
	memberOfJudgementVariant()
}

// DefinedMemberOfJudgement is the exact-basis-bearing subset of the result
// algebra. MemberOfUndefined cannot satisfy this interface, so a later sealed
// admission basis cannot accidentally persist missing basis as a false result.
// Whether Member or NotMember permits a particular write remains a separate
// validation policy.
type DefinedMemberOfJudgement interface {
	MemberOfJudgement
	Basis() MemberOfBasis
	EvaluationView() MemberOfEvaluationView
	definedMemberOfJudgementVariant()
}

type MemberOfMember struct {
	query          MemberOfQuery
	basis          MemberOfBasis
	canonicalBytes []byte
	digest         SHA256Digest
}

func NewMemberOfMember(
	query MemberOfQuery,
	basis MemberOfBasis,
) (MemberOfMember, error) {
	writer, err := canonicalDefinedMemberOfJudgement(
		memberOfMemberDomainForBasis(basis),
		query,
		basis,
	)
	if err != nil {
		return MemberOfMember{}, err
	}
	return MemberOfMember{
		query:          query,
		basis:          basis,
		canonicalBytes: writer.bytes(),
		digest:         writer.digest(),
	}, nil
}

func (MemberOfMember) Kind() MemberOfJudgementKind { return MemberJudgement }

func (judgement MemberOfMember) Query() MemberOfQuery { return judgement.query }

func (judgement MemberOfMember) EvaluationRequest() MemberOfEvaluationRequest {
	request, _ := NewMemberOfEvaluationRequest(
		judgement.query,
		judgement.basis.EvaluationView(),
	)
	return request
}

func (judgement MemberOfMember) Basis() MemberOfBasis { return judgement.basis }

func (judgement MemberOfMember) EvaluationView() MemberOfEvaluationView {
	return judgement.basis.EvaluationView()
}

func (judgement MemberOfMember) CanonicalBytes() []byte {
	return append([]byte(nil), judgement.canonicalBytes...)
}

func (judgement MemberOfMember) Digest() SHA256Digest { return judgement.digest }

func (MemberOfMember) memberOfJudgementVariant() {}

func (MemberOfMember) definedMemberOfJudgementVariant() {}

type MemberOfNotMember struct {
	query          MemberOfQuery
	basis          MemberOfBasis
	canonicalBytes []byte
	digest         SHA256Digest
}

func NewMemberOfNotMember(
	query MemberOfQuery,
	basis MemberOfBasis,
) (MemberOfNotMember, error) {
	writer, err := canonicalDefinedMemberOfJudgement(
		memberOfNotMemberDomainForBasis(basis),
		query,
		basis,
	)
	if err != nil {
		return MemberOfNotMember{}, err
	}
	return MemberOfNotMember{
		query:          query,
		basis:          basis,
		canonicalBytes: writer.bytes(),
		digest:         writer.digest(),
	}, nil
}

func (MemberOfNotMember) Kind() MemberOfJudgementKind { return NotMemberJudgement }

func (judgement MemberOfNotMember) Query() MemberOfQuery { return judgement.query }

func (judgement MemberOfNotMember) EvaluationRequest() MemberOfEvaluationRequest {
	request, _ := NewMemberOfEvaluationRequest(
		judgement.query,
		judgement.basis.EvaluationView(),
	)
	return request
}

func (judgement MemberOfNotMember) Basis() MemberOfBasis { return judgement.basis }

func (judgement MemberOfNotMember) EvaluationView() MemberOfEvaluationView {
	return judgement.basis.EvaluationView()
}

func (judgement MemberOfNotMember) CanonicalBytes() []byte {
	return append([]byte(nil), judgement.canonicalBytes...)
}

func (judgement MemberOfNotMember) Digest() SHA256Digest { return judgement.digest }

func (MemberOfNotMember) memberOfJudgementVariant() {}

func (MemberOfNotMember) definedMemberOfJudgementVariant() {}

func canonicalDefinedMemberOfJudgement(
	domain string,
	query MemberOfQuery,
	basis MemberOfBasis,
) (canonicalWriter, error) {
	if !query.valid() {
		return canonicalWriter{}, fmt.Errorf("MemberOf judgement query is required")
	}
	if !basis.valid() {
		return canonicalWriter{}, fmt.Errorf("MemberOf judgement basis is required")
	}
	mismatches := basis.Mismatches(query)
	if len(mismatches) > 0 {
		return canonicalWriter{}, fmt.Errorf(
			"MemberOf judgement basis does not match query: %s",
			mismatches[0].Kind().String(),
		)
	}
	if _, v3 := basis.posture.(C32PrerequisiteMemberOfBasisV3); v3 {
		request, requestErr := NewMemberOfEvaluationRequest(
			query,
			basis.evaluationView,
		)
		if requestErr != nil || request.Digest() != basis.memberOfRequestDigest {
			return canonicalWriter{}, fmt.Errorf(
				"MemberOf v3 prerequisite request does not match the judgement query and view",
			)
		}
	}
	writer := newCanonicalWriter(domain)
	writer.addBytes(query.CanonicalBytes())
	writer.addBytes(basis.CanonicalBytes())
	return writer, nil
}

func memberOfMemberDomainForBasis(basis MemberOfBasis) string {
	if _, ok := basis.posture.(C32PrerequisiteMemberOfBasisV3); ok {
		return memberOfMemberV3Domain
	}
	return memberOfMemberDomain
}

func memberOfNotMemberDomainForBasis(basis MemberOfBasis) string {
	if _, ok := basis.posture.(C32PrerequisiteMemberOfBasisV3); ok {
		return memberOfNotMemberV3Domain
	}
	return memberOfNotMemberDomain
}

type MemberOfUndefined struct {
	request        MemberOfEvaluationRequest
	missing        []MemberOfMissingBasis
	repair         RepairPointer
	canonicalBytes []byte
	digest         SHA256Digest
}

func NewMemberOfUndefined(
	request MemberOfEvaluationRequest,
	missing []MemberOfMissingBasis,
	repair RepairPointer,
) (MemberOfUndefined, error) {
	if !request.valid() {
		return MemberOfUndefined{}, fmt.Errorf("undefined MemberOf evaluation request is required")
	}
	if !repair.valid() {
		return MemberOfUndefined{}, fmt.Errorf("undefined MemberOf repair pointer is required")
	}
	missingSet, err := normalizeMemberOfMissingBasis(missing)
	if err != nil {
		return MemberOfUndefined{}, err
	}
	if len(missingSet) == 0 {
		return MemberOfUndefined{}, fmt.Errorf("undefined MemberOf requires at least one missing basis")
	}
	writer := newCanonicalWriter(memberOfUndefinedDomain)
	writer.addBytes(request.CanonicalBytes())
	writer.addUint64(uint64(len(missingSet)))
	for _, basis := range missingSet {
		writer.addBytes(basis.CanonicalBytes())
	}
	writer.addString(repair.String())
	return MemberOfUndefined{
		request:        request,
		missing:        missingSet,
		repair:         repair,
		canonicalBytes: writer.bytes(),
		digest:         writer.digest(),
	}, nil
}

func (MemberOfUndefined) Kind() MemberOfJudgementKind { return UndefinedMemberJudgement }

func (judgement MemberOfUndefined) Query() MemberOfQuery {
	return judgement.request.Query()
}

func (judgement MemberOfUndefined) EvaluationRequest() MemberOfEvaluationRequest {
	return judgement.request
}

func (judgement MemberOfUndefined) EvaluationView() MemberOfEvaluationView {
	return judgement.request.View()
}

func (judgement MemberOfUndefined) MissingBasis() []MemberOfMissingBasis {
	return append([]MemberOfMissingBasis(nil), judgement.missing...)
}

// IsNoApplicableObservableSource is intentionally exact: mixed or otherwise
// underdetermined missing-basis sets cannot authorize an absence-only
// continuation.
func (judgement MemberOfUndefined) IsNoApplicableObservableSource() bool {
	return len(judgement.missing) == 1 &&
		judgement.missing[0].Kind() == NoApplicableMemberOfObservableSource &&
		judgement.missing[0].Subject() == "query:"+judgement.Query().Digest().String()
}

func (judgement MemberOfUndefined) Repair() RepairPointer { return judgement.repair }

func (judgement MemberOfUndefined) CanonicalBytes() []byte {
	return append([]byte(nil), judgement.canonicalBytes...)
}

func (judgement MemberOfUndefined) Digest() SHA256Digest { return judgement.digest }

func (MemberOfUndefined) memberOfJudgementVariant() {}

func normalizeMemberOfMissingBasis(
	values []MemberOfMissingBasis,
) ([]MemberOfMissingBasis, error) {
	result := append([]MemberOfMissingBasis(nil), values...)
	sort.Slice(result, func(left, right int) bool {
		return bytes.Compare(result[left].CanonicalBytes(), result[right].CanonicalBytes()) < 0
	})
	normalized := make([]MemberOfMissingBasis, 0, len(result))
	for _, basis := range result {
		if !basis.valid() {
			return nil, fmt.Errorf("undefined MemberOf contains an invalid missing basis")
		}
		if len(normalized) > 0 &&
			bytes.Equal(normalized[len(normalized)-1].CanonicalBytes(), basis.CanonicalBytes()) {
			continue
		}
		normalized = append(normalized, basis)
	}
	return normalized, nil
}

type MemberOfMismatchKind uint8

const (
	MemberOfInvalidQueryMismatch MemberOfMismatchKind = iota + 1
	MemberOfInvalidBasisMismatch
	MemberOfTypeEnvMismatch
	MemberOfKindSignatureMismatch
	MemberOfEntitySetMismatch
	MemberOfContextSliceMismatch
	MemberOfEvaluationViewMismatch
	MemberOfJudgementQueryMismatch
	MemberOfInvalidJudgementMismatch
)

func (kind MemberOfMismatchKind) String() string {
	switch kind {
	case MemberOfInvalidQueryMismatch:
		return "invalid_query"
	case MemberOfInvalidBasisMismatch:
		return "invalid_basis"
	case MemberOfTypeEnvMismatch:
		return "typeenv_mismatch"
	case MemberOfKindSignatureMismatch:
		return "kind_signature_mismatch"
	case MemberOfEntitySetMismatch:
		return "entity_set_mismatch"
	case MemberOfContextSliceMismatch:
		return "context_slice_mismatch"
	case MemberOfEvaluationViewMismatch:
		return "evaluation_view_mismatch"
	case MemberOfJudgementQueryMismatch:
		return "judgement_query_mismatch"
	case MemberOfInvalidJudgementMismatch:
		return "invalid_judgement"
	default:
		return ""
	}
}

// MemberOfMismatch is a deterministic invalidity datum for the later
// snapshot/validation boundary. It never downgrades mismatch to Undefined.
type MemberOfMismatch struct {
	kind     MemberOfMismatchKind
	expected string
	actual   string
}

func (mismatch MemberOfMismatch) Kind() MemberOfMismatchKind { return mismatch.kind }

func (mismatch MemberOfMismatch) Expected() string { return mismatch.expected }

func (mismatch MemberOfMismatch) Actual() string { return mismatch.actual }

func (basis MemberOfBasis) Mismatches(query MemberOfQuery) []MemberOfMismatch {
	if !query.valid() {
		return []MemberOfMismatch{{kind: MemberOfInvalidQueryMismatch}}
	}
	if !basis.valid() {
		return []MemberOfMismatch{{kind: MemberOfInvalidBasisMismatch}}
	}
	mismatches := make([]MemberOfMismatch, 0, 4)
	if basis.typeEnv != query.ValueKind().TypeEnv() {
		mismatches = append(mismatches, MemberOfMismatch{
			kind:     MemberOfTypeEnvMismatch,
			expected: query.ValueKind().TypeEnv().String(),
			actual:   basis.typeEnv.String(),
		})
	}
	if basis.kindSignature.ValueKind() != query.ValueKind() ||
		basis.kindSignature.Context() != query.ContextSlice().Context() {
		mismatches = append(mismatches, MemberOfMismatch{
			kind:     MemberOfKindSignatureMismatch,
			expected: query.ValueKind().String() + "/context/" + query.ContextSlice().Context().String(),
			actual:   basis.kindSignature.ValueKind().String() + "/context/" + basis.kindSignature.Context().String(),
		})
	}
	if basis.entitySet.TypeEnv() != query.ValueKind().TypeEnv() ||
		basis.entitySet.Context() != query.ContextSlice().Context() {
		mismatches = append(mismatches, MemberOfMismatch{
			kind:     MemberOfEntitySetMismatch,
			expected: query.ValueKind().TypeEnv().String() + "/context/" + query.ContextSlice().Context().String(),
			actual:   basis.entitySet.TypeEnv().String() + "/context/" + basis.entitySet.Context().String(),
		})
	}
	if basis.contextSlice != query.ContextSlice().Ref() {
		mismatches = append(mismatches, MemberOfMismatch{
			kind:     MemberOfContextSliceMismatch,
			expected: query.ContextSlice().Ref().String(),
			actual:   basis.contextSlice.String(),
		})
	}
	return mismatches
}

func memberOfJudgementBaseQueryMismatches(
	query MemberOfQuery,
	judgement MemberOfJudgement,
) []MemberOfMismatch {
	if !query.valid() {
		return []MemberOfMismatch{{kind: MemberOfInvalidQueryMismatch}}
	}
	if !validMemberOfJudgement(judgement) {
		return []MemberOfMismatch{{kind: MemberOfInvalidJudgementMismatch}}
	}
	actualQuery := judgement.Query()
	if !sameMemberOfQuery(query, actualQuery) {
		return []MemberOfMismatch{{
			kind:     MemberOfJudgementQueryMismatch,
			expected: query.Digest().String(),
			actual:   actualQuery.Digest().String(),
		}}
	}
	switch value := judgement.(type) {
	case MemberOfMember:
		return value.Basis().Mismatches(query)
	case MemberOfNotMember:
		return value.Basis().Mismatches(query)
	case MemberOfUndefined:
		return nil
	default:
		return []MemberOfMismatch{{kind: MemberOfInvalidJudgementMismatch}}
	}
}

func MemberOfJudgementMismatchesRequest(
	request MemberOfEvaluationRequest,
	judgement MemberOfJudgement,
) []MemberOfMismatch {
	if !request.valid() {
		return []MemberOfMismatch{{kind: MemberOfInvalidQueryMismatch}}
	}
	mismatches := memberOfJudgementBaseQueryMismatches(request.Query(), judgement)
	if len(mismatches) > 0 {
		return mismatches
	}
	switch value := judgement.(type) {
	case MemberOfMember:
		if !sameMemberOfEvaluationView(request.View(), value.EvaluationView()) {
			return []MemberOfMismatch{{
				kind:     MemberOfEvaluationViewMismatch,
				expected: request.View().Digest().String(),
				actual:   value.EvaluationView().Digest().String(),
			}}
		}
	case MemberOfNotMember:
		if !sameMemberOfEvaluationView(request.View(), value.EvaluationView()) {
			return []MemberOfMismatch{{
				kind:     MemberOfEvaluationViewMismatch,
				expected: request.View().Digest().String(),
				actual:   value.EvaluationView().Digest().String(),
			}}
		}
	case MemberOfUndefined:
		if value.EvaluationRequest().Digest() != request.Digest() ||
			!bytes.Equal(
				value.EvaluationRequest().CanonicalBytes(),
				request.CanonicalBytes(),
			) {
			return []MemberOfMismatch{{
				kind:     MemberOfEvaluationViewMismatch,
				expected: request.Digest().String(),
				actual:   value.EvaluationRequest().Digest().String(),
			}}
		}
	default:
		return []MemberOfMismatch{{kind: MemberOfInvalidJudgementMismatch}}
	}
	return nil
}

func MemberOfJudgementMatchesRequest(
	request MemberOfEvaluationRequest,
	judgement MemberOfJudgement,
) bool {
	return len(MemberOfJudgementMismatchesRequest(request, judgement)) == 0
}

func validMemberOfJudgement(judgement MemberOfJudgement) bool {
	switch value := judgement.(type) {
	case MemberOfMember:
		writer, err := canonicalDefinedMemberOfJudgement(
			memberOfMemberDomainForBasis(value.basis),
			value.query,
			value.basis,
		)
		return err == nil &&
			writer.digest() == value.digest &&
			bytes.Equal(writer.bytes(), value.canonicalBytes)
	case MemberOfNotMember:
		writer, err := canonicalDefinedMemberOfJudgement(
			memberOfNotMemberDomainForBasis(value.basis),
			value.query,
			value.basis,
		)
		return err == nil &&
			writer.digest() == value.digest &&
			bytes.Equal(writer.bytes(), value.canonicalBytes)
	case MemberOfUndefined:
		if !value.request.valid() || !value.repair.valid() || !value.digest.valid() {
			return false
		}
		missing, err := normalizeMemberOfMissingBasis(value.missing)
		if err != nil || len(missing) == 0 || len(missing) != len(value.missing) {
			return false
		}
		writer := newCanonicalWriter(memberOfUndefinedDomain)
		writer.addBytes(value.request.CanonicalBytes())
		writer.addUint64(uint64(len(missing)))
		for _, basis := range missing {
			writer.addBytes(basis.CanonicalBytes())
		}
		writer.addString(value.repair.String())
		return writer.digest() == value.digest &&
			bytes.Equal(writer.bytes(), value.canonicalBytes)
	default:
		return false
	}
}

func sameMemberOfQuery(left, right MemberOfQuery) bool {
	return left.Digest() == right.Digest() &&
		bytes.Equal(left.CanonicalBytes(), right.CanonicalBytes())
}
