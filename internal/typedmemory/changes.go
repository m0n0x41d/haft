package typedmemory

import (
	"fmt"
	"strings"
)

type EntityLabel struct {
	value string
}

func NewEntityLabel(raw string) (EntityLabel, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return EntityLabel{}, fmt.Errorf("entity label is required")
	}
	return EntityLabel{value: value}, nil
}

func (label EntityLabel) String() string { return label.value }

func (label EntityLabel) valid() bool { return label.value != "" }

type RetractionReason struct {
	value string
}

func NewRetractionReason(raw string) (RetractionReason, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return RetractionReason{}, fmt.Errorf("retraction reason is required")
	}
	return RetractionReason{value: value}, nil
}

func (reason RetractionReason) String() string { return reason.value }

func (reason RetractionReason) valid() bool { return reason.value != "" }

type MemoryChange interface {
	memoryChangeVariant()
	validMemoryChange() bool
}

type DeclareEntity struct {
	entity     EntityID
	localRef   BatchLocalRef
	context    BoundedContextRef
	label      EntityLabel
	provenance ProvenanceRef
}

func NewDeclareEntity(
	entity EntityID,
	localRef BatchLocalRef,
	context BoundedContextRef,
	label EntityLabel,
	provenance ProvenanceRef,
) (DeclareEntity, error) {
	if !entity.valid() || !localRef.valid() || !context.valid() || !label.valid() || !provenance.valid() {
		return DeclareEntity{}, fmt.Errorf("entity declaration requires entity, batch-local ref, context, label, and provenance")
	}
	return DeclareEntity{
		entity:     entity,
		localRef:   localRef,
		context:    context,
		label:      label,
		provenance: provenance,
	}, nil
}

func (change DeclareEntity) Entity() EntityID { return change.entity }

func (change DeclareEntity) LocalRef() BatchLocalRef { return change.localRef }

func (change DeclareEntity) Context() BoundedContextRef { return change.context }

func (change DeclareEntity) Label() EntityLabel { return change.label }

func (change DeclareEntity) Provenance() ProvenanceRef { return change.provenance }

func (DeclareEntity) memoryChangeVariant() {}

func (change DeclareEntity) validMemoryChange() bool {
	return change.entity.valid() &&
		change.localRef.valid() &&
		change.context.valid() &&
		change.label.valid() &&
		change.provenance.valid()
}

type ApplyIdentityChange struct {
	change IdentityChange
}

func NewApplyIdentityChange(change IdentityChange) (ApplyIdentityChange, error) {
	if !validIdentityChangeVariant(change) {
		return ApplyIdentityChange{}, fmt.Errorf("identity MemoryChange requires a valid closed IdentityChange")
	}
	return ApplyIdentityChange{change: change}, nil
}

func (change ApplyIdentityChange) Change() IdentityChange { return change.change }

func (ApplyIdentityChange) memoryChangeVariant() {}

func (change ApplyIdentityChange) validMemoryChange() bool {
	return validIdentityChangeVariant(change.change)
}

type InstantiateRelation struct {
	relation RelationInstantiation
}

func NewInstantiateRelation(relation RelationInstantiation) (InstantiateRelation, error) {
	if !relation.valid() {
		return InstantiateRelation{}, fmt.Errorf("relation MemoryChange requires a valid relation instantiation")
	}
	return InstantiateRelation{relation: relation}, nil
}

func (change InstantiateRelation) Relation() RelationInstantiation { return change.relation }

func (InstantiateRelation) memoryChangeVariant() {}

func (change InstantiateRelation) validMemoryChange() bool { return change.relation.valid() }

// AssertRelation is the canonical v3 relation-assertion change. It remains
// disjoint from the legacy InstantiateRelation variant so v2 request bytes and
// their unqualified meaning cannot be silently rewritten.
type AssertRelation struct {
	assertion RelationalAssertionCandidate
}

func NewAssertRelation(assertion RelationalAssertionCandidate) (AssertRelation, error) {
	if !assertion.valid() {
		return AssertRelation{}, fmt.Errorf("AssertRelation requires a valid explicit relational assertion")
	}
	return AssertRelation{assertion: assertion}, nil
}

func (change AssertRelation) Assertion() RelationalAssertionCandidate {
	return change.assertion
}

func (AssertRelation) memoryChangeVariant() {}

func (change AssertRelation) validMemoryChange() bool { return change.assertion.valid() }

type RetractAssertion struct {
	assertion  AssertionID
	reason     RetractionReason
	provenance ProvenanceRef
}

func NewRetractAssertion(
	assertion AssertionID,
	reason RetractionReason,
	provenance ProvenanceRef,
) (RetractAssertion, error) {
	if !assertion.valid() || !reason.valid() || !provenance.valid() {
		return RetractAssertion{}, fmt.Errorf("assertion retraction requires assertion, reason, and provenance")
	}
	return RetractAssertion{assertion: assertion, reason: reason, provenance: provenance}, nil
}

func (change RetractAssertion) Assertion() AssertionID { return change.assertion }

func (change RetractAssertion) Reason() RetractionReason { return change.reason }

func (change RetractAssertion) Provenance() ProvenanceRef { return change.provenance }

func (RetractAssertion) memoryChangeVariant() {}

func (change RetractAssertion) validMemoryChange() bool {
	return change.assertion.valid() && change.reason.valid() && change.provenance.valid()
}

type MemoryChangeSet struct {
	changes []MemoryChange
}

func NewMemoryChangeSet(changes []MemoryChange) (MemoryChangeSet, error) {
	if len(changes) == 0 {
		return MemoryChangeSet{}, fmt.Errorf("MemoryChangeSet must be non-empty")
	}

	owned := append([]MemoryChange(nil), changes...)
	if err := validateMemoryChangeSetIdentity(owned); err != nil {
		return MemoryChangeSet{}, err
	}
	return MemoryChangeSet{changes: owned}, nil
}

func (set MemoryChangeSet) Changes() []MemoryChange {
	return append([]MemoryChange(nil), set.changes...)
}

func (set MemoryChangeSet) valid() bool {
	return len(set.changes) > 0 && validateMemoryChangeSetIdentity(set.changes) == nil
}

type ValidatedMemoryChange interface {
	validatedMemoryChangeVariant()
}

// AdmittedEntityDeclaration is the stable effect form of DeclareEntity.
// BatchLocalRef exists only while decoding and validating one candidate batch;
// it is deliberately absent here so storage cannot persist a request-local
// label as project identity.
type AdmittedEntityDeclaration struct {
	entity     EntityID
	context    BoundedContextRef
	label      EntityLabel
	provenance ProvenanceRef
}

func newAdmittedEntityDeclaration(change DeclareEntity) AdmittedEntityDeclaration {
	return AdmittedEntityDeclaration{
		entity:     change.entity,
		context:    change.context,
		label:      change.label,
		provenance: change.provenance,
	}
}

func (declaration AdmittedEntityDeclaration) Entity() EntityID { return declaration.entity }

func (declaration AdmittedEntityDeclaration) Context() BoundedContextRef {
	return declaration.context
}

func (declaration AdmittedEntityDeclaration) Label() EntityLabel { return declaration.label }

func (declaration AdmittedEntityDeclaration) Provenance() ProvenanceRef {
	return declaration.provenance
}

func (declaration AdmittedEntityDeclaration) valid() bool {
	return declaration.entity.valid() &&
		declaration.context.valid() &&
		declaration.label.valid() &&
		declaration.provenance.valid()
}

type ValidatedDeclareEntity struct {
	declaration AdmittedEntityDeclaration
}

func (change ValidatedDeclareEntity) Change() AdmittedEntityDeclaration {
	return change.declaration
}

func (ValidatedDeclareEntity) validatedMemoryChangeVariant() {}

type ValidatedIdentityChange struct {
	change IdentityChange
}

func (change ValidatedIdentityChange) Change() IdentityChange { return change.change }

func (ValidatedIdentityChange) validatedMemoryChangeVariant() {}

type ValidatedRelationInstance struct {
	relation RelationInstance
}

func (change ValidatedRelationInstance) Relation() RelationInstance { return change.relation }

func (ValidatedRelationInstance) validatedMemoryChangeVariant() {}

// ValidatedRelationalAssertion is the strong v3 structural-validation result.
// Storage admission for this variant is intentionally outside the first P12R
// assertion slice.
type ValidatedRelationalAssertion struct {
	assertion RelationalAssertion
}

func (change ValidatedRelationalAssertion) Assertion() RelationalAssertion {
	return change.assertion
}

func (ValidatedRelationalAssertion) validatedMemoryChangeVariant() {}

type ValidatedRetraction struct {
	change RetractAssertion
}

func (change ValidatedRetraction) Change() RetractAssertion { return change.change }

func (ValidatedRetraction) validatedMemoryChangeVariant() {}

type ValidatedMemoryChangeSet struct {
	changes []ValidatedMemoryChange
}

func newValidatedMemoryChangeSet(changes []ValidatedMemoryChange) ValidatedMemoryChangeSet {
	return ValidatedMemoryChangeSet{changes: append([]ValidatedMemoryChange(nil), changes...)}
}

func (set ValidatedMemoryChangeSet) Changes() []ValidatedMemoryChange {
	return append([]ValidatedMemoryChange(nil), set.changes...)
}

func (set ValidatedMemoryChangeSet) valid() bool {
	if len(set.changes) == 0 {
		return false
	}
	for _, change := range set.changes {
		switch value := change.(type) {
		case ValidatedDeclareEntity:
			if !value.declaration.valid() {
				return false
			}
		case ValidatedIdentityChange:
			if !validIdentityChangeVariant(value.change) {
				return false
			}
		case ValidatedRelationInstance:
			if !value.relation.valid() {
				return false
			}
		case ValidatedRelationalAssertion:
			if !value.assertion.valid() {
				return false
			}
		case ValidatedRetraction:
			if !value.change.validMemoryChange() {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func validateMemoryChangeSetIdentity(changes []MemoryChange) error {
	entityIDs := make(map[string]struct{})
	localRefs := make(map[string]struct{})
	assertionIDs := make(map[string]struct{})
	aliasSubjects := make(map[string]struct{})
	reconciliationSubjects := make(map[string]struct{})

	for index, change := range changes {
		if !validMemoryChangeVariant(change) {
			return fmt.Errorf("MemoryChangeSet contains invalid change at index %d", index)
		}

		switch value := change.(type) {
		case DeclareEntity:
			if _, exists := entityIDs[value.entity.String()]; exists {
				return fmt.Errorf("MemoryChangeSet declares entity %q more than once", value.entity.String())
			}
			if _, exists := localRefs[value.localRef.String()]; exists {
				return fmt.Errorf("MemoryChangeSet declares batch-local ref %q more than once", value.localRef.String())
			}
			entityIDs[value.entity.String()] = struct{}{}
			localRefs[value.localRef.String()] = struct{}{}
		case InstantiateRelation:
			assertion := value.relation.Assertion().String()
			if _, exists := assertionIDs[assertion]; exists {
				return fmt.Errorf("MemoryChangeSet changes assertion %q more than once", assertion)
			}
			assertionIDs[assertion] = struct{}{}
		case AssertRelation:
			assertion := value.assertion.Assertion().String()
			if _, exists := assertionIDs[assertion]; exists {
				return fmt.Errorf("MemoryChangeSet changes assertion %q more than once", assertion)
			}
			assertionIDs[assertion] = struct{}{}
		case RetractAssertion:
			assertion := value.assertion.String()
			if _, exists := assertionIDs[assertion]; exists {
				return fmt.Errorf("MemoryChangeSet changes assertion %q more than once", assertion)
			}
			assertionIDs[assertion] = struct{}{}
		case ApplyIdentityChange:
			if err := reserveIdentityChangeSubjects(
				value.change,
				aliasSubjects,
				reconciliationSubjects,
			); err != nil {
				return err
			}
		}
	}
	return nil
}

func validMemoryChangeVariant(change MemoryChange) bool {
	switch value := change.(type) {
	case DeclareEntity:
		return value.validMemoryChange()
	case ApplyIdentityChange:
		return value.validMemoryChange()
	case InstantiateRelation:
		return value.validMemoryChange()
	case AssertRelation:
		return value.validMemoryChange()
	case RetractAssertion:
		return value.validMemoryChange()
	default:
		return false
	}
}

func reserveIdentityChangeSubjects(
	change IdentityChange,
	aliases map[string]struct{},
	reconciliations map[string]struct{},
) error {
	switch value := change.(type) {
	case AdmitAlias:
		return reserveIdentitySubject(
			aliases,
			exactTupleKey("alias-subject", value.context.String(), value.alias.String()),
		)
	case SupersedeAlias:
		keys := []string{
			exactTupleKey("alias-subject", value.context.String(), value.oldAlias.String()),
			exactTupleKey("alias-subject", value.context.String(), value.replacement.String()),
		}
		return reserveIdentitySubjects(aliases, keys)
	case MergeEntities:
		keys := []string{exactTupleKey("entity-subject", value.context.String(), value.survivor.String())}
		for _, entity := range value.merged {
			keys = append(keys, exactTupleKey("entity-subject", value.context.String(), entity.String()))
		}
		return reserveIdentitySubjects(reconciliations, keys)
	case SplitEntity:
		keys := []string{exactTupleKey("entity-subject", value.context.String(), value.source.String())}
		for _, entity := range value.targets {
			keys = append(keys, exactTupleKey("entity-subject", value.context.String(), entity.String()))
		}
		return reserveIdentitySubjects(reconciliations, keys)
	default:
		return fmt.Errorf("MemoryChangeSet contains an unknown IdentityChange variant")
	}
}

func reserveIdentitySubjects(subjects map[string]struct{}, keys []string) error {
	for _, key := range keys {
		if err := reserveIdentitySubject(subjects, key); err != nil {
			return err
		}
	}
	return nil
}

func reserveIdentitySubject(subjects map[string]struct{}, key string) error {
	if _, exists := subjects[key]; exists {
		return fmt.Errorf("MemoryChangeSet repeats identity subject %q", key)
	}
	subjects[key] = struct{}{}
	return nil
}
