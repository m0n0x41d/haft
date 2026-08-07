package typedmemory

import "testing"

func TestMemoryChangeSetRejectsEmpty(t *testing.T) {
	if _, err := NewMemoryChangeSet(nil); err == nil {
		t.Fatal("MemoryChangeSet accepted an empty list")
	}
}

func TestMemoryChangeSetRejectsDuplicateAssertionEffects(t *testing.T) {
	assertion, err := NewAssertionID("assertion:duplicate-effect")
	if err != nil {
		t.Fatalf("NewAssertionID(): %v", err)
	}
	firstReason, err := NewRetractionReason("first reason")
	if err != nil {
		t.Fatalf("NewRetractionReason(first): %v", err)
	}
	secondReason, err := NewRetractionReason("second reason")
	if err != nil {
		t.Fatalf("NewRetractionReason(second): %v", err)
	}
	provenance := typeEnvTestProvenanceRef(t, "memory:duplicate-retraction")
	first, err := NewRetractAssertion(assertion, firstReason, provenance)
	if err != nil {
		t.Fatalf("NewRetractAssertion(first): %v", err)
	}
	second, err := NewRetractAssertion(assertion, secondReason, provenance)
	if err != nil {
		t.Fatalf("NewRetractAssertion(second): %v", err)
	}
	if _, err := NewMemoryChangeSet([]MemoryChange{first, second}); err == nil {
		t.Fatal("MemoryChangeSet accepted two effects over one assertion")
	}
}

func TestMemoryChangeSetRejectsConflictingIdentitySubjects(t *testing.T) {
	entity := mustEntityID(t, "entity:alias-owner")
	contextRef := mustBoundedContextRef(t, "ctx:identity-conflict")
	alias := mustEntityAlias(t, "shared alias")
	provenance := typeEnvTestProvenanceRef(t, "memory:identity-conflict")
	first, err := NewAdmitAlias(entity, alias, contextRef, provenance)
	if err != nil {
		t.Fatalf("NewAdmitAlias(first): %v", err)
	}
	second, err := NewAdmitAlias(entity, alias, contextRef, provenance)
	if err != nil {
		t.Fatalf("NewAdmitAlias(second): %v", err)
	}
	firstEffect, err := NewApplyIdentityChange(first)
	if err != nil {
		t.Fatalf("NewApplyIdentityChange(first): %v", err)
	}
	secondEffect, err := NewApplyIdentityChange(second)
	if err != nil {
		t.Fatalf("NewApplyIdentityChange(second): %v", err)
	}
	if _, err := NewMemoryChangeSet([]MemoryChange{firstEffect, secondEffect}); err == nil {
		t.Fatal("MemoryChangeSet accepted repeated alias subject")
	}
}

func TestIdentitySubjectKeysAreInjectiveAcrossDelimiterCharacters(t *testing.T) {
	provenance := typeEnvTestProvenanceRef(t, "memory:identity-key-injectivity")
	first, err := NewAdmitAlias(
		mustEntityID(t, "entity:first"),
		mustEntityAlias(t, "c"),
		mustBoundedContextRef(t, "ctx:a:b"),
		provenance,
	)
	if err != nil {
		t.Fatalf("NewAdmitAlias(first): %v", err)
	}
	second, err := NewAdmitAlias(
		mustEntityID(t, "entity:second"),
		mustEntityAlias(t, "b:c"),
		mustBoundedContextRef(t, "ctx:a"),
		provenance,
	)
	if err != nil {
		t.Fatalf("NewAdmitAlias(second): %v", err)
	}
	firstEffect, err := NewApplyIdentityChange(first)
	if err != nil {
		t.Fatalf("NewApplyIdentityChange(first): %v", err)
	}
	secondEffect, err := NewApplyIdentityChange(second)
	if err != nil {
		t.Fatalf("NewApplyIdentityChange(second): %v", err)
	}
	if _, err := NewMemoryChangeSet([]MemoryChange{firstEffect, secondEffect}); err != nil {
		t.Fatalf("MemoryChangeSet collapsed distinct alias subjects: %v", err)
	}
}
