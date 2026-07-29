package projectprofile

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"time"
	"unicode/utf8"
)

const profileDeclarationPayloadJSONSchemaV1 = "haft.project-profile.declaration-payload/v1"

type entityReferenceJSONV1 struct {
	Kind string `json:"kind"`
	Ref  string `json:"ref,omitempty"`
}

// kindOrientationJSONV1 preserves the published v1 kind_admission wire shape.
// The legacy spelling is a compatibility concern, not the domain concept name.
type kindOrientationJSONV1 struct {
	Kind string `json:"kind"`
	Ref  string `json:"ref,omitempty"`
}

type realizationScopeJSONV1 struct {
	Kind                 string                 `json:"kind"`
	ScopeID              string                 `json:"scope_id"`
	EntityReference      entityReferenceJSONV1  `json:"entity_reference"`
	KindOrientation      *kindOrientationJSONV1 `json:"kind_admission,omitempty"`
	GoverningPatternRefs *[]string              `json:"governing_pattern_refs,omitempty"`
	ContractRefs         *[]string              `json:"contract_refs,omitempty"`
}

type profileDeclarationPayloadJSONV1 struct {
	Schema string                   `json:"schema"`
	Scopes []realizationScopeJSONV1 `json:"scopes"`
}

type closedIntervalJSONV1 struct {
	From  string `json:"from"`
	Until string `json:"until"`
}

func EncodeProfileDeclarationPayloadCanonicalJSON(
	payload ProfileDeclarationPayload,
) ([]byte, error) {
	dto, err := profileDeclarationPayloadToJSONV1(payload)
	if err != nil {
		return nil, err
	}
	return marshalCanonicalJSONV1(dto)
}

func DecodeProfileDeclarationPayloadCanonicalJSON(
	data []byte,
) (ProfileDeclarationPayload, error) {
	var dto profileDeclarationPayloadJSONV1
	err := decodeJSONV1(data, &dto)
	if err != nil {
		return ProfileDeclarationPayload{}, err
	}
	payload, err := profileDeclarationPayloadFromJSONV1(dto)
	if err != nil {
		return ProfileDeclarationPayload{}, err
	}
	canonical, err := EncodeProfileDeclarationPayloadCanonicalJSON(payload)
	if err != nil {
		return ProfileDeclarationPayload{}, err
	}
	if !bytes.Equal(data, canonical) {
		return ProfileDeclarationPayload{}, fmt.Errorf("profile payload JSON is not canonical")
	}
	return payload, nil
}

func profileDeclarationPayloadToJSONV1(
	payload ProfileDeclarationPayload,
) (profileDeclarationPayloadJSONV1, error) {
	validated, err := NewProfileDeclarationPayload(payload.scopes)
	if err != nil {
		return profileDeclarationPayloadJSONV1{}, err
	}
	values := validated.scopes.Values()
	scopes, err := mapSliceV1(values, func(_ int, value RealizationScope) (realizationScopeJSONV1, error) {
		return realizationScopeToJSONV1(value)
	})
	if err != nil {
		return profileDeclarationPayloadJSONV1{}, err
	}
	return profileDeclarationPayloadJSONV1{
		Schema: profileDeclarationPayloadJSONSchemaV1,
		Scopes: scopes,
	}, nil
}

func profileDeclarationPayloadFromJSONV1(
	dto profileDeclarationPayloadJSONV1,
) (ProfileDeclarationPayload, error) {
	if dto.Schema != profileDeclarationPayloadJSONSchemaV1 {
		return ProfileDeclarationPayload{}, fmt.Errorf("unsupported profile payload JSON schema %q", dto.Schema)
	}
	if len(dto.Scopes) == 0 {
		return ProfileDeclarationPayload{}, fmt.Errorf("profile payload JSON scopes must not be empty")
	}
	values, err := mapSliceV1(dto.Scopes, func(index int, value realizationScopeJSONV1) (RealizationScope, error) {
		scope, err := realizationScopeFromJSONV1(value)
		if err != nil {
			return nil, fmt.Errorf("scope %d: %w", index, err)
		}
		return scope, nil
	})
	if err != nil {
		return ProfileDeclarationPayload{}, err
	}
	scopes, err := NewScopeSet(values)
	if err != nil {
		return ProfileDeclarationPayload{}, err
	}
	return NewProfileDeclarationPayload(scopes)
}

func realizationScopeToJSONV1(scope RealizationScope) (realizationScopeJSONV1, error) {
	switch value := scope.(type) {
	case SoftwareRealization:
		entityReference := value.EntityReference()
		entity, err := entityReferenceToJSONV1(entityReference)
		if err != nil {
			return realizationScopeJSONV1{}, err
		}
		scopeID := value.ScopeID().String()
		return realizationScopeJSONV1{
			Kind:            "software",
			ScopeID:         scopeID,
			EntityReference: entity,
		}, nil
	case NonSoftwareRealization:
		entityReference := value.EntityReference()
		entity, err := entityReferenceToJSONV1(entityReference)
		if err != nil {
			return realizationScopeJSONV1{}, err
		}
		kindOrientation := value.KindOrientation()
		kind, err := kindOrientationToJSONV1(kindOrientation)
		if err != nil {
			return realizationScopeJSONV1{}, err
		}
		patternRefs := value.GoverningPatternRefs()
		patterns := sourceUnitRefStringsV1(patternRefs)
		contractRefs := value.ContractRefs()
		contracts := specSectionRefStringsV1(contractRefs)
		scopeID := value.ScopeID().String()
		return realizationScopeJSONV1{
			Kind:                 "non_software",
			ScopeID:              scopeID,
			EntityReference:      entity,
			KindOrientation:      &kind,
			GoverningPatternRefs: &patterns,
			ContractRefs:         &contracts,
		}, nil
	default:
		return realizationScopeJSONV1{}, fmt.Errorf("unknown realization scope variant")
	}
}

func realizationScopeFromJSONV1(dto realizationScopeJSONV1) (RealizationScope, error) {
	scopeID, err := NewScopeID(dto.ScopeID)
	if err != nil {
		return nil, err
	}
	entity, err := entityReferenceFromJSONV1(dto.EntityReference)
	if err != nil {
		return nil, err
	}
	switch dto.Kind {
	case "software":
		if dto.KindOrientation != nil || dto.GoverningPatternRefs != nil || dto.ContractRefs != nil {
			return nil, fmt.Errorf("software scope contains non-software fields")
		}
		return NewSoftwareRealization(scopeID, entity)
	case "non_software":
		if dto.KindOrientation == nil {
			return nil, fmt.Errorf("non-software scope kind_admission is required")
		}
		if dto.GoverningPatternRefs == nil || dto.ContractRefs == nil {
			return nil, fmt.Errorf("non-software scope reference lists must be explicit")
		}
		kind, kindErr := kindOrientationFromJSONV1(*dto.KindOrientation)
		if kindErr != nil {
			return nil, kindErr
		}
		patterns, patternErr := sourceUnitRefsFromStringsV1(*dto.GoverningPatternRefs)
		if patternErr != nil {
			return nil, patternErr
		}
		contracts, contractErr := specSectionRefsFromStringsV1(*dto.ContractRefs)
		if contractErr != nil {
			return nil, contractErr
		}
		return NewNonSoftwareRealization(scopeID, entity, kind, patterns, contracts)
	default:
		return nil, fmt.Errorf("unknown realization scope kind %q", dto.Kind)
	}
}

func entityReferenceToJSONV1(reference EntityReference) (entityReferenceJSONV1, error) {
	switch value := reference.(type) {
	case NoEntityReference:
		return entityReferenceJSONV1{Kind: "none"}, nil
	case ReferencedEntity:
		return entityReferenceJSONV1{Kind: "referenced", Ref: value.Ref().String()}, nil
	default:
		return entityReferenceJSONV1{}, fmt.Errorf("unknown entity-reference variant")
	}
}

func entityReferenceFromJSONV1(dto entityReferenceJSONV1) (EntityReference, error) {
	switch dto.Kind {
	case "none":
		if dto.Ref != "" {
			return nil, fmt.Errorf("absent entity reference contains ref")
		}
		return NoEntityReference{}, nil
	case "referenced":
		ref, err := NewEntityRef(dto.Ref)
		if err != nil {
			return nil, err
		}
		return NewReferencedEntity(ref), nil
	default:
		return nil, fmt.Errorf("unknown entity-reference kind %q", dto.Kind)
	}
}

func kindOrientationToJSONV1(orientation KindOrientation) (kindOrientationJSONV1, error) {
	switch value := orientation.(type) {
	case UnspecifiedKindOrientation:
		return kindOrientationJSONV1{Kind: "none"}, nil
	case ReferencedKindOrientation:
		return kindOrientationJSONV1{Kind: "admitted", Ref: value.Ref().String()}, nil
	default:
		return kindOrientationJSONV1{}, fmt.Errorf("unknown kind-orientation variant")
	}
}

func kindOrientationFromJSONV1(dto kindOrientationJSONV1) (KindOrientation, error) {
	switch dto.Kind {
	case "none":
		if dto.Ref != "" {
			return nil, fmt.Errorf("unspecified kind orientation contains ref")
		}
		return UnspecifiedKindOrientation{}, nil
	case "admitted":
		ref, err := NewKindRef(dto.Ref)
		if err != nil {
			return nil, err
		}
		return NewReferencedKindOrientation(ref), nil
	default:
		return nil, fmt.Errorf("unknown kind-orientation compatibility kind %q", dto.Kind)
	}
}

func sourceUnitRefStringsV1(values []SourceUnitRef) []string {
	return mapSliceV1Pure(values, func(value SourceUnitRef) string {
		return value.String()
	})
}

func specSectionRefStringsV1(values []SpecSectionRef) []string {
	return mapSliceV1Pure(values, func(value SpecSectionRef) string {
		return value.String()
	})
}

func sourceUnitRefsFromStringsV1(values []string) ([]SourceUnitRef, error) {
	return mapSliceV1(values, func(index int, value string) (SourceUnitRef, error) {
		ref, err := NewSourceUnitRef(value)
		if err != nil {
			return SourceUnitRef{}, fmt.Errorf("governing pattern ref %d: %w", index, err)
		}
		return ref, nil
	})
}

func specSectionRefsFromStringsV1(values []string) ([]SpecSectionRef, error) {
	return mapSliceV1(values, func(index int, value string) (SpecSectionRef, error) {
		ref, err := NewSpecSectionRef(value)
		if err != nil {
			return SpecSectionRef{}, fmt.Errorf("contract ref %d: %w", index, err)
		}
		return ref, nil
	})
}

func marshalCanonicalJSONV1(value any) ([]byte, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode canonical project-profile JSON: %w", err)
	}
	return data, nil
}

func decodeJSONV1(data []byte, target any) error {
	if len(data) == 0 {
		return fmt.Errorf("canonical project-profile JSON is empty")
	}
	if !utf8.Valid(data) {
		return fmt.Errorf("canonical project-profile JSON is not valid UTF-8")
	}
	reader := bytes.NewReader(data)
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	err := decoder.Decode(target)
	if err != nil {
		return fmt.Errorf("decode canonical project-profile JSON: %w", err)
	}
	var trailing any
	err = decoder.Decode(&trailing)
	if err != io.EOF {
		return fmt.Errorf("canonical project-profile JSON contains trailing data")
	}
	return nil
}

func closedIntervalToJSONV1(interval closedIntervalV1) closedIntervalJSONV1 {
	return closedIntervalJSONV1{
		From:  canonicalTime(interval.from),
		Until: canonicalTime(interval.until),
	}
}

func closedIntervalFromJSONV1(
	name string,
	dto closedIntervalJSONV1,
) (closedIntervalV1, error) {
	from, err := parseCanonicalTimeV1(name+" from", dto.From)
	if err != nil {
		return closedIntervalV1{}, err
	}
	until, err := parseCanonicalTimeV1(name+" until", dto.Until)
	if err != nil {
		return closedIntervalV1{}, err
	}
	return newClosedIntervalV1(name, from, until)
}

func parseCanonicalTimeV1(name string, raw string) (time.Time, error) {
	value, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s must be RFC3339Nano: %w", name, err)
	}
	if canonicalTime(value) != raw {
		return time.Time{}, fmt.Errorf("%s must use canonical UTC RFC3339Nano form", name)
	}
	return value.UTC(), nil
}
