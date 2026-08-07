package typedmemory

import "fmt"

const (
	relationalAssertionCandidateCanonicalDomain = "relational-assertion-candidate.v3"
	validatedRelationalAssertionCanonicalDomain = "validated-relational-assertion.v3"
)

// CanonicalBytes returns the exact request-local DeclareEntity candidate,
// including its BatchLocalRef. This is the same byte identity bound by
// ProspectiveBatchView and SameBatchDeclarationResolution.
func (change DeclareEntity) CanonicalBytes() ([]byte, error) {
	return canonicalMemoryChange(change)
}

// Digest returns the domain-separated candidate-declaration identity bound by
// ProspectiveBatchView and SameBatchDeclarationResolution.
func (change DeclareEntity) Digest() (SHA256Digest, error) {
	canonical, err := change.CanonicalBytes()
	if err != nil {
		return SHA256Digest{}, err
	}
	writer := newCanonicalWriter("prospective-declare-entity-candidate.v1")
	writer.addBytes(canonical)
	return writer.digest(), nil
}

// CanonicalBytes returns the exact canonical row material already embedded in
// ValidatedMemoryChangeSet.CanonicalBytes for this relation instance.
func (relation RelationInstance) CanonicalBytes() ([]byte, error) {
	return canonicalRelationInstance(relation)
}

// Digest hashes the exact bytes returned by CanonicalBytes.
func (relation RelationInstance) Digest() (SHA256Digest, error) {
	canonical, err := relation.CanonicalBytes()
	if err != nil {
		return SHA256Digest{}, err
	}
	return digestCanonicalBytes(canonical), nil
}

// CanonicalBytes returns the exact disjoint v3 candidate representation. It
// never reuses the legacy relation-instantiation.v2 domain.
func (assertion RelationalAssertionCandidate) CanonicalBytes() ([]byte, error) {
	return canonicalRelationalAssertionCandidate(assertion)
}

// Digest hashes the exact v3 candidate bytes returned by CanonicalBytes.
func (assertion RelationalAssertionCandidate) Digest() (SHA256Digest, error) {
	canonical, err := assertion.CanonicalBytes()
	if err != nil {
		return SHA256Digest{}, err
	}
	return digestCanonicalBytes(canonical), nil
}

// CanonicalBytes returns the exact v3 request change representation.
func (change AssertRelation) CanonicalBytes() ([]byte, error) {
	return canonicalMemoryChange(change)
}

// Digest hashes the exact v3 request change representation.
func (change AssertRelation) Digest() (SHA256Digest, error) {
	canonical, err := change.CanonicalBytes()
	if err != nil {
		return SHA256Digest{}, err
	}
	return digestCanonicalBytes(canonical), nil
}

// CanonicalBytes returns the exact strong v3 assertion representation. A
// positive modality remains claim content and does not encode an occurrence.
func (assertion RelationalAssertion) CanonicalBytes() ([]byte, error) {
	return canonicalRelationalAssertion(assertion)
}

// Digest hashes the exact strong v3 assertion bytes returned by CanonicalBytes.
func (assertion RelationalAssertion) Digest() (SHA256Digest, error) {
	canonical, err := assertion.CanonicalBytes()
	if err != nil {
		return SHA256Digest{}, err
	}
	return digestCanonicalBytes(canonical), nil
}

// CanonicalBytes returns the exact canonical row material already embedded in
// its enclosing RelationInstance.
func (binding SlotBinding) CanonicalBytes() []byte {
	return canonicalSlotBinding(binding)
}

// Digest hashes the exact bytes returned by CanonicalBytes.
func (binding SlotBinding) Digest() SHA256Digest {
	return digestCanonicalBytes(binding.CanonicalBytes())
}

// CanonicalBytes returns the exact canonical row material already embedded in
// its enclosing SlotBinding.
func (filler ReferenceFiller) CanonicalBytes() []byte {
	return canonicalSlotFiller(filler)
}

// Digest hashes the exact bytes returned by CanonicalBytes.
func (filler ReferenceFiller) Digest() SHA256Digest {
	return digestCanonicalBytes(filler.CanonicalBytes())
}

// CanonicalBytes returns the exact canonical row material already embedded in
// its enclosing SlotBinding.
func (filler ValueFiller) CanonicalBytes() []byte {
	return canonicalSlotFiller(filler)
}

// Digest hashes the exact bytes returned by CanonicalBytes.
func (filler ValueFiller) Digest() SHA256Digest {
	return digestCanonicalBytes(filler.CanonicalBytes())
}

// CanonicalBytes returns the exact canonical row material already embedded in
// ValidatedMemoryChangeSet.CanonicalBytes for this identity change.
func (change AdmitAlias) CanonicalBytes() ([]byte, error) {
	return canonicalIdentityChange(change)
}

// Digest hashes the exact bytes returned by CanonicalBytes.
func (change AdmitAlias) Digest() (SHA256Digest, error) {
	canonical, err := change.CanonicalBytes()
	if err != nil {
		return SHA256Digest{}, err
	}
	return digestCanonicalBytes(canonical), nil
}

// CanonicalBytes returns the exact canonical row material already embedded in
// ValidatedMemoryChangeSet.CanonicalBytes for this identity change.
func (change SupersedeAlias) CanonicalBytes() ([]byte, error) {
	return canonicalIdentityChange(change)
}

// Digest hashes the exact bytes returned by CanonicalBytes.
func (change SupersedeAlias) Digest() (SHA256Digest, error) {
	canonical, err := change.CanonicalBytes()
	if err != nil {
		return SHA256Digest{}, err
	}
	return digestCanonicalBytes(canonical), nil
}

// CanonicalBytes returns the exact canonical row material already embedded in
// ValidatedMemoryChangeSet.CanonicalBytes for this retraction.
func (change RetractAssertion) CanonicalBytes() ([]byte, error) {
	return canonicalMemoryChange(change)
}

// Digest hashes the exact bytes returned by CanonicalBytes.
func (change RetractAssertion) Digest() (SHA256Digest, error) {
	canonical, err := change.CanonicalBytes()
	if err != nil {
		return SHA256Digest{}, err
	}
	return digestCanonicalBytes(canonical), nil
}

func (set MemoryChangeSet) Digest() (SHA256Digest, error) {
	writer, err := canonicalCandidateMemoryChangeSet(set)
	if err != nil {
		return SHA256Digest{}, err
	}
	return writer.digest(), nil
}

// CanonicalBytes returns the exact request-local candidate envelope. Unlike
// ValidatedMemoryChangeSet.CanonicalBytes, these bytes retain local references
// and other boundary evidence needed for idempotency and admission replay.
func (set MemoryChangeSet) CanonicalBytes() ([]byte, error) {
	writer, err := canonicalCandidateMemoryChangeSet(set)
	if err != nil {
		return nil, err
	}
	return writer.bytes(), nil
}

func canonicalCandidateMemoryChangeSet(
	set MemoryChangeSet,
) (canonicalWriter, error) {
	if !set.valid() {
		return canonicalWriter{}, fmt.Errorf("cannot canonicalize an invalid MemoryChangeSet")
	}

	// MemoryChangeSet is deliberately a NonEmptyList: its order is the local
	// atomic effect order for this one admission request. It is not a project,
	// causal, graph-traversal, or WorkPlan order. Named relation slots and
	// unordered value shapes normalize independently below.
	writer := newCanonicalWriter("memory-change-set.v1")
	for _, change := range set.changes {
		encoded, err := canonicalMemoryChange(change)
		if err != nil {
			return canonicalWriter{}, err
		}
		writer.addBytes(encoded)
	}
	return writer, nil
}

// CanonicalDigest commits to the admitted semantic representation, after
// reference resolution and codec normalization. It intentionally does not
// reuse MemoryChangeSet.Digest: candidate input bytes and asserted digests are
// boundary evidence, not part of the admitted relation value.
func (set ValidatedMemoryChangeSet) CanonicalDigest() (SHA256Digest, error) {
	writer, err := canonicalValidatedMemoryChangeSet(set)
	if err != nil {
		return SHA256Digest{}, err
	}
	return writer.digest(), nil
}

// CanonicalBytes returns the exact admitted semantic envelope whose digest is
// CanonicalDigest. Persistence stores these bytes so a restart can verify the
// event without reconstructing request-local candidate labels.
func (set ValidatedMemoryChangeSet) CanonicalBytes() ([]byte, error) {
	writer, err := canonicalValidatedMemoryChangeSet(set)
	if err != nil {
		return nil, err
	}
	return writer.bytes(), nil
}

func canonicalValidatedMemoryChangeSet(
	set ValidatedMemoryChangeSet,
) (canonicalWriter, error) {
	if !set.valid() {
		return canonicalWriter{}, fmt.Errorf("cannot canonicalize an invalid ValidatedMemoryChangeSet")
	}

	// The validated change list retains the request-local atomic effect order.
	// Named relation slots and unordered fillers are already canonicalized by
	// their closed constructors and are encoded again below from strong values.
	writer := newCanonicalWriter("validated-memory-change-set.v1")
	for _, change := range set.changes {
		encoded, err := canonicalValidatedMemoryChange(change)
		if err != nil {
			return canonicalWriter{}, err
		}
		writer.addBytes(encoded)
	}
	return writer, nil
}

func canonicalValidatedMemoryChange(change ValidatedMemoryChange) ([]byte, error) {
	switch value := change.(type) {
	case ValidatedDeclareEntity:
		return canonicalAdmittedEntityDeclaration(value.declaration)
	case ValidatedIdentityChange:
		return canonicalIdentityChange(value.change)
	case ValidatedRelationInstance:
		return canonicalRelationInstance(value.relation)
	case ValidatedRelationalAssertion:
		return canonicalRelationalAssertion(value.assertion)
	case ValidatedRetraction:
		return canonicalMemoryChange(value.change)
	default:
		return nil, fmt.Errorf("unsupported ValidatedMemoryChange variant %T", change)
	}
}

func canonicalAdmittedEntityDeclaration(
	declaration AdmittedEntityDeclaration,
) ([]byte, error) {
	if !declaration.valid() {
		return nil, fmt.Errorf("cannot canonicalize an invalid admitted entity declaration")
	}

	writer := newCanonicalWriter("admitted-declare-entity.v1")
	writer.addString(declaration.entity.String())
	writer.addString(declaration.context.String())
	writer.addString(declaration.label.String())
	writer.addString(declaration.provenance.String())
	return writer.bytes(), nil
}

func canonicalRelationInstance(relation RelationInstance) ([]byte, error) {
	if !relation.valid() {
		return nil, fmt.Errorf("cannot canonicalize an invalid relation instance")
	}

	writer := newCanonicalWriter("validated-relation-instance.v2")
	writer.addString(relation.assertion.String())
	writer.addString(relation.signature.String())
	writer.addString(relation.slice.Ref().String())
	writer.addBytes(relation.slice.CanonicalBytes())
	for _, binding := range relation.bindings {
		writer.addBytes(canonicalSlotBinding(binding))
	}
	writer.addString(relation.provenance.String())
	return writer.bytes(), nil
}

func canonicalRelationalAssertion(assertion RelationalAssertion) ([]byte, error) {
	if !assertion.valid() {
		return nil, fmt.Errorf("cannot canonicalize an invalid relational assertion")
	}

	writer := newCanonicalWriter(validatedRelationalAssertionCanonicalDomain)
	writer.addString(assertion.assertion.String())
	writer.addString(assertion.signature.String())
	writer.addString(assertion.slice.Ref().String())
	writer.addBytes(assertion.slice.CanonicalBytes())
	writer.addString(assertion.modality.Kind().String())
	for _, binding := range assertion.bindings {
		writer.addBytes(canonicalSlotBinding(binding))
	}
	writer.addString(assertion.provenance.String())
	return writer.bytes(), nil
}

func canonicalSlotBinding(binding SlotBinding) []byte {
	writer := newCanonicalWriter("validated-slot-binding.v1")
	writer.addString(binding.name.String())
	for _, filler := range binding.fillers {
		writer.addBytes(canonicalSlotFiller(filler))
	}
	return writer.bytes()
}

func canonicalMemoryChange(change MemoryChange) ([]byte, error) {
	switch value := change.(type) {
	case DeclareEntity:
		writer := newCanonicalWriter("declare-entity.v1")
		writer.addString(value.entity.String())
		writer.addString(value.localRef.String())
		writer.addString(value.context.String())
		writer.addString(value.label.String())
		writer.addString(value.provenance.String())
		return writer.bytes(), nil
	case ApplyIdentityChange:
		return canonicalIdentityChange(value.change)
	case InstantiateRelation:
		return canonicalRelationInstantiation(value.relation)
	case AssertRelation:
		return canonicalRelationalAssertionCandidate(value.assertion)
	case RetractAssertion:
		writer := newCanonicalWriter("retract-assertion.v1")
		writer.addString(value.assertion.String())
		writer.addString(value.reason.String())
		writer.addString(value.provenance.String())
		return writer.bytes(), nil
	default:
		return nil, fmt.Errorf("unsupported MemoryChange variant %T", change)
	}
}

func canonicalIdentityChange(change IdentityChange) ([]byte, error) {
	switch value := change.(type) {
	case AdmitAlias:
		writer := newCanonicalWriter("identity-admit-alias.v1")
		writer.addString(value.entity.String())
		writer.addString(value.alias.String())
		writer.addString(value.context.String())
		writer.addString(value.provenance.String())
		return writer.bytes(), nil
	case SupersedeAlias:
		writer := newCanonicalWriter("identity-supersede-alias.v1")
		writer.addString(value.entity.String())
		writer.addString(value.oldAlias.String())
		writer.addString(value.replacement.String())
		writer.addString(value.context.String())
		writer.addString(value.provenance.String())
		return writer.bytes(), nil
	case MergeEntities:
		writer := newCanonicalWriter("identity-merge-entities.v1")
		writer.addString(value.survivor.String())
		for _, entity := range value.merged {
			writer.addString(entity.String())
		}
		writer.addString(value.context.String())
		writer.addString(value.basis.String())
		return writer.bytes(), nil
	case SplitEntity:
		writer := newCanonicalWriter("identity-split-entity.v1")
		writer.addString(value.source.String())
		for _, entity := range value.targets {
			writer.addString(entity.String())
		}
		writer.addString(value.context.String())
		writer.addString(value.basis.String())
		return writer.bytes(), nil
	default:
		return nil, fmt.Errorf("unsupported IdentityChange variant %T", change)
	}
}

func canonicalRelationInstantiation(relation RelationInstantiation) ([]byte, error) {
	if !relation.valid() {
		return nil, fmt.Errorf("cannot canonicalize an invalid relation instantiation")
	}

	writer := newCanonicalWriter("relation-instantiation.v2")
	writer.addString(relation.assertion.String())
	writer.addString(relation.signature.String())
	writer.addString(relation.slice.Ref().String())
	writer.addBytes(relation.slice.CanonicalBytes())
	for _, binding := range relation.bindings {
		writer.addBytes(canonicalCandidateSlotBinding(binding))
	}
	writer.addString(relation.provenance.String())
	return writer.bytes(), nil
}

func canonicalRelationalAssertionCandidate(
	assertion RelationalAssertionCandidate,
) ([]byte, error) {
	if !assertion.valid() {
		return nil, fmt.Errorf("cannot canonicalize an invalid relational assertion candidate")
	}

	writer := newCanonicalWriter(relationalAssertionCandidateCanonicalDomain)
	writer.addString(assertion.assertion.String())
	writer.addString(assertion.signature.String())
	writer.addString(assertion.slice.Ref().String())
	writer.addBytes(assertion.slice.CanonicalBytes())
	writer.addString(assertion.modality.Kind().String())
	for _, binding := range assertion.bindings {
		writer.addBytes(canonicalCandidateSlotBinding(binding))
	}
	writer.addString(assertion.provenance.String())
	return writer.bytes(), nil
}

func canonicalCandidateSlotBinding(binding CandidateSlotBinding) []byte {
	writer := newCanonicalWriter("candidate-slot-binding.v1")
	writer.addString(binding.name.String())

	fillers := make([][]byte, 0, len(binding.fillers))
	for _, filler := range binding.fillers {
		fillers = append(fillers, canonicalCandidateSlotFiller(filler))
	}
	for _, filler := range sortedCanonicalBytes(fillers) {
		writer.addBytes(filler)
	}
	return writer.bytes()
}

func canonicalCandidateSlotFiller(filler CandidateSlotFiller) []byte {
	switch value := filler.(type) {
	case ByReferenceCandidate:
		writer := newCanonicalWriter("candidate-by-reference.v1")
		writer.addString(value.reference.RefKind().String())
		writer.addString(value.reference.ReferenceKey())
		return writer.bytes()
	case ByValueCandidate:
		writer := newCanonicalWriter("candidate-by-value.v1")
		writer.addString(value.value.ValueKind().String())
		writer.addString(value.value.ValueShape().String())
		writer.addString(value.value.Codec().String())
		writer.addBytes(value.value.InputBytes())
		digest, present := value.value.AssertedDigest().Digest()
		if present {
			writer.addString(digest.String())
		}
		return writer.bytes()
	default:
		return nil
	}
}

func canonicalSlotFiller(filler SlotFiller) []byte {
	switch value := filler.(type) {
	case ReferenceFiller:
		writer := newCanonicalWriter("validated-by-reference.v2")
		writer.addString(value.reference.RefKind().String())
		writer.addString(value.reference.ReferenceKey())
		writer.addString(value.entity.String())
		return writer.bytes()
	case ValueFiller:
		writer := newCanonicalWriter("validated-by-value.v1")
		writer.addString(value.value.ValueKind().String())
		writer.addString(value.value.ValueShape().String())
		writer.addString(value.value.Codec().String())
		writer.addBytes(value.value.CanonicalBytes())
		writer.addString(value.value.Digest().String())
		return writer.bytes()
	default:
		return nil
	}
}
