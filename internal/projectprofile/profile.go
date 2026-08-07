package projectprofile

import "fmt"

type EntityReference interface {
	entityReferenceVariant()
}

type NoEntityReference struct{}

func (NoEntityReference) entityReferenceVariant() {}

type ReferencedEntity struct {
	ref EntityRef
}

func NewReferencedEntity(ref EntityRef) ReferencedEntity {
	return ReferencedEntity{ref: ref}
}

func (ReferencedEntity) entityReferenceVariant() {}

func (reference ReferencedEntity) Ref() EntityRef {
	return reference.ref
}

type KindOrientation interface {
	kindOrientationVariant()
}

type UnspecifiedKindOrientation struct{}

func (UnspecifiedKindOrientation) kindOrientationVariant() {}

type ReferencedKindOrientation struct {
	ref KindRef
}

func NewReferencedKindOrientation(ref KindRef) ReferencedKindOrientation {
	return ReferencedKindOrientation{ref: ref}
}

func (ReferencedKindOrientation) kindOrientationVariant() {}

func (orientation ReferencedKindOrientation) Ref() KindRef {
	return orientation.ref
}

type RealizationScope interface {
	realizationScopeVariant()
	ScopeID() ScopeID
}

type SoftwareRealization struct {
	scopeID   ScopeID
	entityRef EntityReference
}

func NewSoftwareRealization(scopeID ScopeID, entityRef EntityReference) (SoftwareRealization, error) {
	if !scopeID.valid() {
		return SoftwareRealization{}, fmt.Errorf("software realization scope_id is invalid")
	}
	if err := validateEntityReference(entityRef); err != nil {
		return SoftwareRealization{}, err
	}
	return SoftwareRealization{scopeID: scopeID, entityRef: entityRef}, nil
}

func (SoftwareRealization) realizationScopeVariant() {}

func (scope SoftwareRealization) ScopeID() ScopeID {
	return scope.scopeID
}

func (scope SoftwareRealization) EntityReference() EntityReference {
	return scope.entityRef
}

type NonSoftwareRealization struct {
	scopeID              ScopeID
	entityRef            EntityReference
	kindOrientation      KindOrientation
	governingPatternRefs []SourceUnitRef
	contractRefs         []SpecSectionRef
}

func NewNonSoftwareRealization(
	scopeID ScopeID,
	entityRef EntityReference,
	kindOrientation KindOrientation,
	governingPatternRefs []SourceUnitRef,
	contractRefs []SpecSectionRef,
) (NonSoftwareRealization, error) {
	if !scopeID.valid() {
		return NonSoftwareRealization{}, fmt.Errorf("non-software realization scope_id is invalid")
	}
	if err := validateEntityReference(entityRef); err != nil {
		return NonSoftwareRealization{}, err
	}
	if err := validateKindOrientation(kindOrientation); err != nil {
		return NonSoftwareRealization{}, err
	}
	if err := validateSourceUnitRefs(governingPatternRefs); err != nil {
		return NonSoftwareRealization{}, err
	}
	if err := validateSpecSectionRefs(contractRefs); err != nil {
		return NonSoftwareRealization{}, err
	}
	return NonSoftwareRealization{
		scopeID:              scopeID,
		entityRef:            entityRef,
		kindOrientation:      kindOrientation,
		governingPatternRefs: append([]SourceUnitRef{}, governingPatternRefs...),
		contractRefs:         append([]SpecSectionRef{}, contractRefs...),
	}, nil
}

func (NonSoftwareRealization) realizationScopeVariant() {}

func (scope NonSoftwareRealization) ScopeID() ScopeID {
	return scope.scopeID
}

func (scope NonSoftwareRealization) EntityReference() EntityReference {
	return scope.entityRef
}

func (scope NonSoftwareRealization) KindOrientation() KindOrientation {
	return scope.kindOrientation
}

func (scope NonSoftwareRealization) GoverningPatternRefs() []SourceUnitRef {
	return append([]SourceUnitRef{}, scope.governingPatternRefs...)
}

func (scope NonSoftwareRealization) ContractRefs() []SpecSectionRef {
	return append([]SpecSectionRef{}, scope.contractRefs...)
}

type ScopeSet struct {
	values []RealizationScope
}

func NewScopeSet(values []RealizationScope) (ScopeSet, error) {
	if len(values) == 0 {
		return ScopeSet{}, fmt.Errorf("declared profile requires at least one realization scope")
	}
	seen := map[string]struct{}{}
	result := make([]RealizationScope, 0, len(values))
	for index, value := range values {
		if err := validateRealizationScope(value); err != nil {
			return ScopeSet{}, fmt.Errorf("realization scope %d: %w", index, err)
		}
		key := value.ScopeID().String()
		if _, exists := seen[key]; exists {
			return ScopeSet{}, fmt.Errorf("duplicate scope_id %q", key)
		}
		seen[key] = struct{}{}
		result = append(result, value)
	}
	return ScopeSet{values: result}, nil
}

func (scopes ScopeSet) Values() []RealizationScope {
	return append([]RealizationScope{}, scopes.values...)
}

func (scopes ScopeSet) Len() int {
	return len(scopes.values)
}

func (scopes ScopeSet) valid() bool {
	_, err := NewScopeSet(scopes.values)
	return err == nil
}

type ConfiguredProjectProfile interface {
	configuredProjectProfileVariant()
}

type Auto struct{}

func (Auto) configuredProjectProfileVariant() {}

type Declared struct {
	scopes      ScopeSet
	declaration ProfileDeclarationReceipt
}

// NewDeclared reconstructs or validates the description carried by a legacy
// declaration record. It does not perform admission, prove persistence, or
// make the result implement ConfiguredProjectProfileV1.
func NewDeclared(
	scopes ScopeSet,
	declaration ProfileDeclarationReceipt,
) (Declared, error) {
	if !scopes.valid() {
		return Declared{}, fmt.Errorf("declared profile scopes are invalid")
	}
	if err := validateProfileDeclarationReceipt(declaration); err != nil {
		return Declared{}, err
	}
	scopePayloadDigest, err := DigestScopePayload(scopes)
	if err != nil {
		return Declared{}, err
	}
	if scopePayloadDigest != declaration.ScopePayloadDigest() {
		return Declared{}, fmt.Errorf("declaration scope-payload digest does not match canonical scopes")
	}
	return Declared{
		scopes:      scopes,
		declaration: declaration,
	}, nil
}

func (Declared) configuredProjectProfileVariant() {}

func (profile Declared) Scopes() ScopeSet {
	return profile.scopes
}

func (profile Declared) Declaration() ProfileDeclarationReceipt {
	return profile.declaration
}

func (profile Declared) CarrierRevision() CarrierRevision {
	return profile.declaration.CarrierRevision()
}

func validateRealizationScope(scope RealizationScope) error {
	switch value := scope.(type) {
	case SoftwareRealization:
		_, err := NewSoftwareRealization(value.scopeID, value.entityRef)
		return err
	case NonSoftwareRealization:
		_, err := NewNonSoftwareRealization(
			value.scopeID,
			value.entityRef,
			value.kindOrientation,
			value.governingPatternRefs,
			value.contractRefs,
		)
		return err
	default:
		return fmt.Errorf("unknown realization scope variant")
	}
}

func validateEntityReference(reference EntityReference) error {
	switch value := reference.(type) {
	case NoEntityReference:
		return nil
	case ReferencedEntity:
		if value.ref.valid() {
			return nil
		}
	}
	return fmt.Errorf("entity reference must be absent or a valid EntityRef")
}

func validateKindOrientation(orientation KindOrientation) error {
	switch value := orientation.(type) {
	case UnspecifiedKindOrientation:
		return nil
	case ReferencedKindOrientation:
		if value.ref.valid() {
			return nil
		}
	}
	return fmt.Errorf("kind orientation must be unspecified or reference a valid KindRef")
}

func validateSourceUnitRefs(values []SourceUnitRef) error {
	seen := map[string]struct{}{}
	for index, value := range values {
		if !value.valid() {
			return fmt.Errorf("governing pattern ref %d is invalid", index)
		}
		if _, exists := seen[value.String()]; exists {
			return fmt.Errorf("duplicate governing pattern ref %q", value.String())
		}
		seen[value.String()] = struct{}{}
	}
	return nil
}

func validateSpecSectionRefs(values []SpecSectionRef) error {
	seen := map[string]struct{}{}
	for index, value := range values {
		if !value.valid() {
			return fmt.Errorf("contract ref %d is invalid", index)
		}
		if _, exists := seen[value.String()]; exists {
			return fmt.Errorf("duplicate contract ref %q", value.String())
		}
		seen[value.String()] = struct{}{}
	}
	return nil
}
