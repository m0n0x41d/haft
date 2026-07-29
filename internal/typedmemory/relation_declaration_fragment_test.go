package typedmemory

import (
	"reflect"
	"strings"
	"testing"
)

func TestTypedRelationDeclarationFragmentMakesFullFPFSemanticsInexpressible(t *testing.T) {
	fragmentType := reflect.TypeOf(TypedRelationDeclarationFragment{})
	allowedFields := map[string]struct{}{
		"ref":        {},
		"contexts":   {},
		"slots":      {},
		"provenance": {},
	}
	for index := 0; index < fragmentType.NumField(); index++ {
		field := fragmentType.Field(index)
		if _, allowed := allowedFields[field.Name]; !allowed {
			t.Fatalf("fragment exposes undeclared semantic field %q", field.Name)
		}
	}

	forbidden := []string{
		"predicate",
		"laws",
		"applicability",
		"occurrenceidentity",
		"referencescheme",
		"declarationepisteme",
	}
	for index := 0; index < fragmentType.NumMethod(); index++ {
		method := fragmentType.Method(index)
		name := strings.ToLower(method.Name)
		for _, token := range forbidden {
			if strings.Contains(name, token) {
				t.Fatalf("fragment method %q fabricates unavailable semantics", method.Name)
			}
		}
	}
}

func TestHistoricalRelationSignatureConstructorsAliasTypedFragmentPosture(t *testing.T) {
	fixture := newTypeEnvFixture(t)
	ref := fixture.relationFragment.Ref()
	legacy, err := NewRelationSignature(
		ref,
		fixture.relationFragment.Contexts(),
		fixture.relationFragment.Slots(),
		fixture.relationFragment.Provenance(),
	)
	if err != nil {
		t.Fatalf("NewRelationSignature() error = %v", err)
	}
	currentRef, err := NewTypedRelationDeclarationFragmentRef(ref.TypeEnv(), ref.ID())
	if err != nil {
		t.Fatalf("NewTypedRelationDeclarationFragmentRef() error = %v", err)
	}
	if currentRef != ref {
		t.Fatal("current fragment ref changed the historical edition coordinate")
	}
	if !reflect.DeepEqual(legacy, fixture.relationFragment) {
		t.Fatal("historical RelationSignature constructor changed fragment content")
	}
	if legacy.Posture() != RelationDeclarationTypedFragment {
		t.Fatalf("legacy alias posture = %q", legacy.Posture())
	}

	environment := fixture.build(t)
	fromCurrent, currentExists := environment.TypedRelationDeclarationFragment(ref)
	fromLegacy, legacyExists := environment.RelationSignature(ref)
	if !currentExists || !legacyExists {
		t.Fatal("current or historical TypeEnv accessor lost the fragment")
	}
	if !reflect.DeepEqual(fromCurrent, fromLegacy) {
		t.Fatal("historical TypeEnv accessor reinterpreted the fragment")
	}

	currentChange, err := NewDefineTypedRelationDeclarationFragmentSchemaChange(
		fixture.relationFragment,
	)
	if err != nil {
		t.Fatalf("NewDefineTypedRelationDeclarationFragmentSchemaChange() error = %v", err)
	}
	legacyChange, err := NewDefineRelationSignatureSchemaChange(legacy)
	if err != nil {
		t.Fatalf("NewDefineRelationSignatureSchemaChange() error = %v", err)
	}
	if currentChange.Posture() != RelationDeclarationTypedFragment ||
		!reflect.DeepEqual(currentChange, legacyChange) ||
		currentChange.ChangeKey() != legacyChange.ChangeKey() {
		t.Fatal("historical schema-change alias changed fragment posture or edition key")
	}
	if _, writable := any(fixture.relationFragment).(MemoryChange); writable {
		t.Fatal("typed relation declaration fragment became a generic memory write")
	}
}
