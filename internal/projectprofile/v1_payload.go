package projectprofile

import (
	"cmp"
	"fmt"
	"slices"
)

const profileDeclarationPayloadDigestDomainV1 = "haft.project-profile.declaration-payload/v1"

// ProfileDeclarationPayload contains the semantic proposal only. Provenance,
// Work, authority, observed basis, receipts, revisions, and projections are
// deliberately inexpressible here.
//
// In final-v1, structural non-contradiction means: a non-empty ScopeSet,
// exactly one sealed realization variant per unique ScopeID, duplicate-free
// canonical reference sets, and internally valid optional entity/kind
// bindings. The same EntityRef may intentionally occur in distinct contextual
// scopes; this algebra does not invent a cross-scope conflict relation.
type ProfileDeclarationPayload struct {
	scopes ScopeSet
}

func NewProfileDeclarationPayload(scopes ScopeSet) (ProfileDeclarationPayload, error) {
	if !scopes.valid() {
		return ProfileDeclarationPayload{}, fmt.Errorf("profile declaration payload scopes are invalid")
	}
	canonical, err := canonicalScopeSetV1(scopes)
	if err != nil {
		return ProfileDeclarationPayload{}, err
	}
	return ProfileDeclarationPayload{scopes: canonical}, nil
}

// ValidateProfileDeclarationPayloadStructuralConsistencyV1 checks the exact
// final-v1 non-contradiction predicate without admitting or persisting the
// payload.
func ValidateProfileDeclarationPayloadStructuralConsistencyV1(
	payload ProfileDeclarationPayload,
) error {
	_, err := canonicalScopeSetV1(payload.scopes)
	return err
}

func (payload ProfileDeclarationPayload) Scopes() ScopeSet {
	return payload.scopes
}

func (payload ProfileDeclarationPayload) valid() bool {
	_, err := NewProfileDeclarationPayload(payload.scopes)
	return err == nil
}

func DigestProfileDeclarationPayload(payload ProfileDeclarationPayload) (ContentDigest, error) {
	validated, err := NewProfileDeclarationPayload(payload.scopes)
	if err != nil {
		return ContentDigest{}, err
	}
	writer := newCanonicalDigestWriter(profileDeclarationPayloadDigestDomainV1)
	values := validated.scopes.Values()
	err = addScopes(writer, values)
	if err != nil {
		return ContentDigest{}, err
	}
	return writer.digest(), nil
}

func canonicalScopeSetV1(scopes ScopeSet) (ScopeSet, error) {
	values := scopes.Values()
	canonical, err := mapSliceV1(values, func(_ int, value RealizationScope) (RealizationScope, error) {
		return canonicalRealizationScopeV1(value)
	})
	if err != nil {
		return ScopeSet{}, err
	}
	slices.SortFunc(canonical, compareRealizationScopeV1)
	return NewScopeSet(canonical)
}

func canonicalRealizationScopeV1(scope RealizationScope) (RealizationScope, error) {
	switch value := scope.(type) {
	case SoftwareRealization:
		scopeID := value.ScopeID()
		entityReference := value.EntityReference()
		return NewSoftwareRealization(scopeID, entityReference)
	case NonSoftwareRealization:
		patterns := value.GoverningPatternRefs()
		contracts := value.ContractRefs()
		slices.SortFunc(patterns, compareSourceUnitRefV1)
		slices.SortFunc(contracts, compareSpecSectionRefV1)
		err := rejectDuplicateSourceUnitRefsV1(patterns)
		if err != nil {
			return nil, err
		}
		err = rejectDuplicateSpecSectionRefsV1(contracts)
		if err != nil {
			return nil, err
		}
		scopeID := value.ScopeID()
		entityReference := value.EntityReference()
		kindOrientation := value.KindOrientation()
		return NewNonSoftwareRealization(
			scopeID,
			entityReference,
			kindOrientation,
			patterns,
			contracts,
		)
	default:
		return nil, fmt.Errorf("unknown realization scope variant")
	}
}

func compareRealizationScopeV1(left RealizationScope, right RealizationScope) int {
	leftID := left.ScopeID().String()
	rightID := right.ScopeID().String()
	return cmp.Compare(leftID, rightID)
}

func compareSourceUnitRefV1(left SourceUnitRef, right SourceUnitRef) int {
	leftText := left.String()
	rightText := right.String()
	return cmp.Compare(leftText, rightText)
}

func compareSpecSectionRefV1(left SpecSectionRef, right SpecSectionRef) int {
	leftText := left.String()
	rightText := right.String()
	return cmp.Compare(leftText, rightText)
}

func rejectDuplicateSourceUnitRefsV1(values []SourceUnitRef) error {
	return visitAdjacentV1(values, func(previous SourceUnitRef, current SourceUnitRef) error {
		previousText := previous.String()
		currentText := current.String()
		if previousText == currentText {
			return fmt.Errorf("duplicate governing pattern ref %q", currentText)
		}
		return nil
	})
}

func rejectDuplicateSpecSectionRefsV1(values []SpecSectionRef) error {
	return visitAdjacentV1(values, func(previous SpecSectionRef, current SpecSectionRef) error {
		previousText := previous.String()
		currentText := current.String()
		if previousText == currentText {
			return fmt.Errorf("duplicate contract ref %q", currentText)
		}
		return nil
	})
}
