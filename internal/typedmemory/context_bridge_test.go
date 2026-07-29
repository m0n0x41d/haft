package typedmemory

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func TestContextBridgePreservesFullRuntimeContractAndIdentity(t *testing.T) {
	fixture := newTypeEnvFixture(t)
	bridge := fixture.bridge
	input := contextBridgeTestInput(bridge)

	if bridge.ID() != input.ID || bridge.Source() != input.Source || bridge.Target() != input.Target {
		t.Fatal("ContextBridge lost its identity or exact endpoint editions")
	}
	if bridge.Mapping() != input.Mapping || bridge.Direction() != input.Direction {
		t.Fatal("ContextBridge lost its named mapping or direction")
	}
	if bridge.OrderCoverage() != input.OrderCoverage ||
		bridge.KindCongruence() != input.KindCongruence {
		t.Fatal("ContextBridge lost its order-coverage or CL^k declaration")
	}
	if !reflect.DeepEqual(bridge.LossNotes().Values(), input.LossNotes.Values()) ||
		!reflect.DeepEqual(bridge.DefinednessArea().Values(), input.DefinednessArea.Values()) {
		t.Fatal("ContextBridge lost its loss notes or definedness area")
	}
	if !bytes.Equal(bridge.Provenance().CanonicalBytes(), input.Provenance.CanonicalBytes()) {
		t.Fatal("ContextBridge lost its exact declaration provenance")
	}
	if !bytes.Contains(bridge.CanonicalBytes(), []byte(NoOrderLinksCovered.String())) {
		t.Fatal("ContextBridge canonical identity omitted closed order coverage")
	}

	alternateSource := mustTypedMemoryValue(NewContextBridgeEndpoint(
		typeEnvTestContextRef(t, "ctx:alternate-source"),
		input.Source.Edition(),
	))
	alternateTarget := mustTypedMemoryValue(NewContextBridgeEndpoint(
		typeEnvTestContextRef(t, "ctx:alternate-target"),
		input.Target.Edition(),
	))
	alternateSourceEdition := mustTypedMemoryValue(NewContextBridgeEndpoint(
		input.Source.Context(),
		mustTypedMemoryValue(NewContextEdition("2.0.0-source")),
	))
	alternateTargetEdition := mustTypedMemoryValue(NewContextBridgeEndpoint(
		input.Target.Context(),
		mustTypedMemoryValue(NewContextEdition("2.0.0-target")),
	))
	alternateMapping := mustTypedMemoryValue(NewNamedTargetKindMapping(
		typeEnvTestKindID(t, "U.AlternateSource"),
		typeEnvTestKindID(t, "U.AlternateTarget"),
	))
	alternateCongruence := mustTypedMemoryValue(NewKindCongruenceLevel(3))
	alternateLossNotes := mustTypedMemoryValue(NewKindBridgeLossNotes([]string{
		"A second exact representation loss applies.",
	}))
	alternateDefinedness := mustTypedMemoryValue(NewKindBridgeDefinednessArea([]string{
		"A second exact applicability condition holds.",
	}))
	alternateProvenance := typeEnvTestFPFProvenance(t, "prov:fpf:alternate-bridge", 0xb7)
	variants := []struct {
		name   string
		mutate func(*ContextBridgeInput)
	}{
		{"id", func(value *ContextBridgeInput) {
			value.ID = typeEnvTestBridgeID(t, "bridge:alternate")
		}},
		{"source context", func(value *ContextBridgeInput) { value.Source = alternateSource }},
		{"source edition", func(value *ContextBridgeInput) { value.Source = alternateSourceEdition }},
		{"target context", func(value *ContextBridgeInput) { value.Target = alternateTarget }},
		{"target edition", func(value *ContextBridgeInput) { value.Target = alternateTargetEdition }},
		{"mapping", func(value *ContextBridgeInput) { value.Mapping = alternateMapping }},
		{"direction", func(value *ContextBridgeInput) { value.Direction = OneWayBridge }},
		{"kind congruence", func(value *ContextBridgeInput) { value.KindCongruence = alternateCongruence }},
		{"loss notes", func(value *ContextBridgeInput) { value.LossNotes = alternateLossNotes }},
		{"definedness area", func(value *ContextBridgeInput) { value.DefinednessArea = alternateDefinedness }},
		{"provenance", func(value *ContextBridgeInput) { value.Provenance = alternateProvenance }},
	}
	for _, variant := range variants {
		t.Run(variant.name, func(t *testing.T) {
			candidateInput := input
			variant.mutate(&candidateInput)
			candidate, err := NewContextBridge(candidateInput)
			if err != nil {
				t.Fatalf("NewContextBridge(%s): %v", variant.name, err)
			}
			if candidate.Digest() == bridge.Digest() {
				t.Fatalf("changing %s did not change ContextBridge identity", variant.name)
			}
		})
	}
}

func TestContextBridgeRejectsIncompleteOrUnsupportedContract(t *testing.T) {
	fixture := newTypeEnvFixture(t)
	input := contextBridgeTestInput(fixture.bridge)
	tests := []struct {
		name   string
		mutate func(*ContextBridgeInput)
	}{
		{"missing ID", func(value *ContextBridgeInput) { value.ID = ContextBridgeID{} }},
		{"missing source", func(value *ContextBridgeInput) { value.Source = ContextBridgeEndpoint{} }},
		{"missing target", func(value *ContextBridgeInput) { value.Target = ContextBridgeEndpoint{} }},
		{"same endpoint context", func(value *ContextBridgeInput) {
			value.Target = mustTypedMemoryValue(NewContextBridgeEndpoint(
				value.Source.Context(),
				value.Target.Edition(),
			))
		}},
		{"missing mapping", func(value *ContextBridgeInput) { value.Mapping = NamedTargetKindMapping{} }},
		{"missing direction", func(value *ContextBridgeInput) { value.Direction = 0 }},
		{"unsupported order coverage", func(value *ContextBridgeInput) {
			value.OrderCoverage = KindBridgeOrderCoverage(2)
		}},
		{"missing CL", func(value *ContextBridgeInput) { value.KindCongruence = 0 }},
		{"missing loss notes", func(value *ContextBridgeInput) { value.LossNotes = KindBridgeLossNotes{} }},
		{"missing definedness", func(value *ContextBridgeInput) {
			value.DefinednessArea = KindBridgeDefinednessArea{}
		}},
		{"missing provenance", func(value *ContextBridgeInput) { value.Provenance = nil }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := input
			test.mutate(&candidate)
			if _, err := NewContextBridge(candidate); err == nil {
				t.Fatalf("ContextBridge accepted %s", test.name)
			}
		})
	}
}

func TestContextBridgeExactEditionsAndClosedCongruenceLadder(t *testing.T) {
	moving := []string{"latest", "LATEST", "current", "head", "*"}
	for _, raw := range moving {
		t.Run("moving_"+raw, func(t *testing.T) {
			if _, err := NewContextEdition(raw); err == nil {
				t.Fatalf("moving context edition %q was accepted", raw)
			}
		})
	}
	for _, raw := range []string{" exact-edition", "exact-edition ", "line\nbreak"} {
		if _, err := NewContextEdition(raw); err == nil {
			t.Fatalf("noncanonical context edition %q was accepted", raw)
		}
	}
	if edition, err := NewContextEdition("AuthStandard-v2.3"); err != nil ||
		edition.String() != "AuthStandard-v2.3" {
		t.Fatalf("exact source-owned edition rejected or changed: %#v, %v", edition, err)
	}

	for value := uint8(0); value <= 3; value++ {
		level, err := NewKindCongruenceLevel(value)
		if err != nil || level.Value() != value || !level.valid() {
			t.Fatalf("CL^k %d rejected or changed: %#v, %v", value, level, err)
		}
	}
	if _, err := NewKindCongruenceLevel(4); err == nil {
		t.Fatal("CL^k outside the closed 0..3 ladder was accepted")
	}
	if KindCongruenceLevel(0).valid() {
		t.Fatal("zero Go representation became an implicit semantic CL^k 0")
	}
	if _, err := parseKindBridgeOrderCoverage("preserved"); err == nil {
		t.Fatal("unsupported order-preservation claim was accepted")
	}
}

func TestContextBridgeCanonicalTextSetsAreNonemptyUniqueAndImmutable(t *testing.T) {
	forward := mustTypedMemoryValue(NewKindBridgeLossNotes([]string{
		"Loss B is disclosed.",
		"Loss A is disclosed.",
	}))
	reverse := mustTypedMemoryValue(NewKindBridgeLossNotes([]string{
		"Loss A is disclosed.",
		"Loss B is disclosed.",
	}))
	if !bytes.Equal(forward.canonicalBytes(), reverse.canonicalBytes()) ||
		!reflect.DeepEqual(forward.Values(), reverse.Values()) {
		t.Fatal("loss-note set permutation changed canonical representation")
	}

	for _, values := range [][]string{
		nil,
		{},
		{"duplicate", "duplicate"},
		{" whitespace"},
		{"line\nbreak"},
	} {
		if _, err := NewKindBridgeLossNotes(values); err == nil {
			t.Fatalf("invalid loss-note set accepted: %#v", values)
		}
		if _, err := NewKindBridgeDefinednessArea(values); err == nil {
			t.Fatalf("invalid definedness set accepted: %#v", values)
		}
	}

	values := forward.Values()
	values[0] = "mutated"
	canonical := forward.canonicalBytes()
	canonical[0] ^= 0xff
	if strings.Contains(strings.Join(forward.Values(), " "), "mutated") || !forward.valid() {
		t.Fatal("loss-note accessors leaked mutable canonical storage")
	}

	drifted := cloneKindBridgeLossNotes(forward)
	drifted.set.values[0], drifted.set.values[1] = drifted.set.values[1], drifted.set.values[0]
	if drifted.valid() {
		t.Fatal("noncanonical internal text order was accepted as valid")
	}
}

func TestContextBridgeDirectionAndCopiesFailClosed(t *testing.T) {
	fixture := newTypeEnvFixture(t)
	input := contextBridgeTestInput(fixture.bridge)
	input.Mapping = mustTypedMemoryValue(NewNamedTargetKindMapping(
		fixture.entityKind.ID(),
		fixture.systemKind.ID(),
	))
	bridge := mustTypedMemoryValue(NewContextBridge(input))
	sourceContext := input.Source.Context()
	targetContext := input.Target.Context()
	sourceKind := input.Mapping.SourceKind()
	targetKind := input.Mapping.TargetKind()

	if !bridge.AllowsMapping(sourceContext, sourceKind, targetContext, targetKind) {
		t.Fatal("two-way bridge rejected its declared forward mapping")
	}
	if !bridge.AllowsMapping(targetContext, targetKind, sourceContext, sourceKind) {
		t.Fatal("two-way bridge rejected its exact inverse")
	}
	if bridge.AllowsMapping(targetContext, sourceKind, sourceContext, targetKind) {
		t.Fatal("two-way bridge admitted a correspondence other than its exact inverse")
	}
	oneWayInput := input
	oneWayInput.Direction = OneWayBridge
	oneWay := mustTypedMemoryValue(NewContextBridge(oneWayInput))
	if oneWay.AllowsMapping(targetContext, targetKind, sourceContext, sourceKind) {
		t.Fatal("one-way bridge silently admitted its reverse")
	}

	inputLossNotes := input.LossNotes
	built := mustTypedMemoryValue(NewContextBridge(input))
	inputLossNotes.set.values[0] = "mutated after construction"
	returnedNotes := built.LossNotes()
	returnedNotes.set.values[0] = "mutated through accessor"
	canonical := built.CanonicalBytes()
	canonical[0] ^= 0xff
	if !built.valid() || strings.Contains(strings.Join(built.LossNotes().Values(), " "), "mutated") {
		t.Fatal("ContextBridge construction or accessors leaked mutable storage")
	}

	builder := fixture.builder()
	fixture.bridge.lossNotes.set.values[0] = "mutated after AddContextBridge"
	environment, err := builder.Build()
	if err != nil {
		t.Fatalf("TypeEnvBuilder did not own its ContextBridge input: %v", err)
	}
	bridges := environment.ContextBridges()
	bridges[0].lossNotes.set.values[0] = "mutated through TypeEnv accessor"
	if !environment.ContextBridges()[0].valid() {
		t.Fatal("TypeEnv ContextBridge accessor leaked mutable storage")
	}

	forged := cloneContextBridge(built)
	forged.lossNotes.set.values[0] = "forged without canonical update"
	if forged.valid() || forged.AllowsMapping(sourceContext, sourceKind, targetContext, targetKind) {
		t.Fatal("drifted ContextBridge did not fail closed")
	}
	if _, err := NewAddContextBridgeSchemaChange(forged); err == nil {
		t.Fatal("SchemaChange admitted a drifted ContextBridge")
	}

	change := mustTypedMemoryValue(NewAddContextBridgeSchemaChange(built))
	changeCopy := change.Bridge()
	changeCopy.lossNotes.set.values[0] = "mutated through SchemaChange accessor"
	if !change.Bridge().valid() {
		t.Fatal("AddContextBridgeSchemaChange leaked mutable bridge storage")
	}
	changeSet := mustTypedMemoryValue(NewSchemaChangeSet([]SchemaChange{change}))
	setChanges := changeSet.Changes()
	setBridge := setChanges[0].(AddContextBridgeSchemaChange)
	setBridge.bridge.definednessArea.set.values[0] = "mutated through SchemaChangeSet accessor"
	retainedChange := changeSet.Changes()[0].(AddContextBridgeSchemaChange)
	if !retainedChange.bridge.valid() {
		t.Fatal("SchemaChangeSet leaked mutable ContextBridge storage")
	}
}

func TestContextBridgeRequiresCanonicalCompilerDerivedProvenance(t *testing.T) {
	fixture := newTypeEnvFixture(t)
	input := contextBridgeTestInput(fixture.bridge)
	compilerProvenance := mustTypedMemoryValue(NewCompilerDerivedProvenance(
		typeEnvTestProvenanceRef(t, "prov:compiler:context-bridge"),
		[]SourceLocation{
			typeEnvTestSourceLocation(t, 0xd1),
			typeEnvTestSourceLocation(t, 0xd2),
		},
		typeEnvTestCompilerRuleID(t, "compiler.context-bridge.v1"),
	))
	input.Provenance = compilerProvenance
	built, err := NewContextBridge(input)
	if err != nil {
		t.Fatalf("canonical compiler-derived provenance rejected: %v", err)
	}

	invalidInputs := []struct {
		name   string
		mutate func(*CompilerDerivedProvenance)
	}{
		{"invalid source", func(value *CompilerDerivedProvenance) {
			value.inputs[0] = SourceLocation{}
		}},
		{"duplicate source", func(value *CompilerDerivedProvenance) {
			value.inputs[1] = value.inputs[0]
		}},
		{"noncanonical order", func(value *CompilerDerivedProvenance) {
			value.inputs[0], value.inputs[1] = value.inputs[1], value.inputs[0]
		}},
	}
	for _, test := range invalidInputs {
		t.Run(test.name, func(t *testing.T) {
			forged := compilerProvenance
			forged.inputs = append([]SourceLocation(nil), compilerProvenance.inputs...)
			test.mutate(&forged)
			candidate := input
			candidate.Provenance = forged
			if _, err := NewContextBridge(candidate); err == nil {
				t.Fatalf("ContextBridge accepted %s compiler provenance", test.name)
			}
		})
	}

	compilerProvenance.inputs[0] = SourceLocation{}
	returned := built.Provenance().(CompilerDerivedProvenance)
	returned.inputs[0] = SourceLocation{}
	retained := built.Provenance().(CompilerDerivedProvenance)
	if !built.valid() || !retained.inputs[0].valid() {
		t.Fatal("ContextBridge leaked compiler-derived provenance input or accessor storage")
	}
}

func contextBridgeTestInput(bridge ContextBridge) ContextBridgeInput {
	return ContextBridgeInput{
		ID:              bridge.ID(),
		Source:          bridge.Source(),
		Target:          bridge.Target(),
		Mapping:         bridge.Mapping(),
		Direction:       bridge.Direction(),
		OrderCoverage:   bridge.OrderCoverage(),
		KindCongruence:  bridge.KindCongruence(),
		LossNotes:       bridge.LossNotes(),
		DefinednessArea: bridge.DefinednessArea(),
		Provenance:      bridge.Provenance(),
	}
}
