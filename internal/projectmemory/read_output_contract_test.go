package projectmemory

import (
	"reflect"
	"slices"
	"testing"

	"github.com/m0n0x41d/haft/internal/projectmemory/memoryresolve"
	"github.com/m0n0x41d/haft/internal/projectmemory/neighborhood"
	"github.com/m0n0x41d/haft/internal/projectmemory/scopedrecall"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorywire"
)

func TestMemoryReadOutputContractNamesClosedRuntimeFamilies(t *testing.T) {
	t.Parallel()

	contract := MemoryReadOutputContractV1()
	wantFamilies := map[string][]string{
		typedmemorywire.ActionResolve: {
			string(memoryresolve.ResultExactEntity),
			string(memoryresolve.ResultKnownAbsent),
			string(memoryresolve.ResultEntityCandidates),
			string(memoryresolve.ResultResolutionUnsettled),
			string(memoryresolve.ResultRetryRequired),
		},
		typedmemorywire.ActionNeighborhood: {
			string(neighborhood.ResultExactNeighborhood),
			string(neighborhood.ResultRetryRequired),
			string(neighborhood.ResultAbstained),
		},
		typedmemorywire.ActionRecall: {
			string(scopedrecall.ScopedResultCandidateSet),
			string(scopedrecall.ScopedResultRetryRequired),
			string(scopedrecall.ScopedResultAbstained),
		},
	}

	gotFamilies := make(map[string][]string, len(contract.ResultFamilies))
	for _, family := range contract.ResultFamilies {
		for _, variant := range family.Variants {
			gotFamilies[family.Action] = append(
				gotFamilies[family.Action],
				variant.Kind,
			)
			assertDistinctOutputFields(
				t,
				family.Action+"."+variant.Kind,
				variant.RequiredFields,
				variant.OptionalFields,
			)
			assertDistinctOutputFieldsWhenPresent(
				t,
				family.Action+"."+variant.Kind+" envelope",
				variant.RequiredEnvelopeFields,
			)
		}
	}
	if !reflect.DeepEqual(gotFamilies, wantFamilies) {
		t.Fatalf("memory-read result families = %#v, want %#v", gotFamilies, wantFamilies)
	}
	if contract.Schema != MemoryReadOutputContractSchemaV1 ||
		contract.RuntimeContract != typedmemorywire.ContractVersion {
		t.Fatalf("memory-read contract header = %#v", contract)
	}
}

func TestMemoryReadOutputContractNamesRequiredSharedShapes(t *testing.T) {
	t.Parallel()

	contract := MemoryReadOutputContractV1()
	wantShapes := []string{
		"ProjectionBasis",
		"ProjectionItemBasis",
		"FacetBasisIssue",
		"WholeReadRetryCause",
		"ReadAbstentionBasis",
		"InterpretationContract",
		"RelationalRecordsInterpretation",
		"RelationalRecordItemPosture",
		"RelationDeclarationPosture",
		"RelationPathWitness",
		"FacetCoverage",
		"AppliedReadBudget",
		"RetryRequired",
		"ScopedRecallAbstentionBasis",
	}
	gotShapes := make([]string, 0, len(contract.NamedShapes))
	for _, shape := range contract.NamedShapes {
		gotShapes = append(gotShapes, shape.Name)
		if len(shape.Variants) == 0 && len(shape.AllowedValues) == 0 {
			assertDistinctOutputFields(
				t,
				shape.Name,
				shape.RequiredFields,
				shape.OptionalFields,
			)
		}
		if len(shape.Variants) > 0 && shape.Discriminator == "" {
			t.Fatalf("%s union has no discriminator", shape.Name)
		}
		if len(shape.AllowedValues) > 0 {
			assertDistinctOutputFields(
				t,
				shape.Name+" enum",
				shape.AllowedValues,
				nil,
			)
		}
		for _, variant := range shape.Variants {
			assertDistinctOutputFields(
				t,
				shape.Name+"."+variant.Kind,
				variant.RequiredFields,
				variant.OptionalFields,
			)
		}
	}
	if !reflect.DeepEqual(gotShapes, wantShapes) {
		t.Fatalf("memory-read named shapes = %#v, want %#v", gotShapes, wantShapes)
	}
}

func TestMemoryReadOutputContractReturnsDefensiveSlices(t *testing.T) {
	t.Parallel()

	first := MemoryReadOutputContractV1()
	first.ProjectionProfileRefs[0] = "mutated.v1"
	first.ResultFamilies[0].Variants[0].RequiredFields[0] = "mutated"

	second := MemoryReadOutputContractV1()
	if second.ProjectionProfileRefs[0] == "mutated.v1" ||
		second.ResultFamilies[0].Variants[0].RequiredFields[0] == "mutated" {
		t.Fatal("memory-read output contract leaked mutable package state")
	}
}

func TestMemoryReadOutputContractClosesRelationalRecordPostures(t *testing.T) {
	t.Parallel()

	contract := MemoryReadOutputContractV1()
	shapes := make(map[string]MemoryReadNamedShapeContract, len(contract.NamedShapes))
	for _, shape := range contract.NamedShapes {
		shapes[shape.Name] = shape
	}

	interpretation := shapes["InterpretationContract"]
	if !reflect.DeepEqual(
		interpretation.RequiredFields,
		[]string{
			"structure",
			"identity",
			"relational_records",
			"ranking",
			"truth",
			"applicability",
			"authority",
			"work_order",
			"completeness",
			"hydrate_before_reliance",
		},
	) {
		t.Fatalf("InterpretationContract fields = %#v", interpretation.RequiredFields)
	}

	wantAggregate := []string{
		string(neighborhood.RelationalRecordsAssertionsExactAtSnapshot),
		string(neighborhood.RelationalRecordsOccurrencesExactAtSnapshot),
		string(neighborhood.RelationalRecordsLegacyUnqualifiedAssertions),
		string(neighborhood.RelationalRecordsCandidateAssertions),
		string(neighborhood.RelationalRecordsHeterogeneous),
		string(neighborhood.RelationalRecordsUnavailable),
	}
	if !reflect.DeepEqual(
		shapes["RelationalRecordsInterpretation"].AllowedValues,
		wantAggregate,
	) {
		t.Fatalf(
			"aggregate relational-record values = %#v",
			shapes["RelationalRecordsInterpretation"].AllowedValues,
		)
	}

	wantItems := []string{
		string(neighborhood.RelationalRecordItemAssertionExact),
		string(neighborhood.RelationalRecordItemOccurrenceExact),
		string(neighborhood.RelationalRecordItemLegacyUnqualifiedAssertion),
		string(neighborhood.RelationalRecordItemCandidateAssertion),
	}
	if !reflect.DeepEqual(
		shapes["RelationalRecordItemPosture"].AllowedValues,
		wantItems,
	) {
		t.Fatalf(
			"item relational-record values = %#v",
			shapes["RelationalRecordItemPosture"].AllowedValues,
		)
	}

	if !reflect.DeepEqual(
		shapes["RelationDeclarationPosture"].AllowedValues,
		[]string{string(typedmemory.RelationDeclarationTypedFragment)},
	) {
		t.Fatalf(
			"relation declaration values = %#v",
			shapes["RelationDeclarationPosture"].AllowedValues,
		)
	}
	witness := shapes["RelationPathWitness"]
	if !slices.Contains(witness.RequiredFields, "relation_declaration_fragment_id") {
		t.Fatalf("RelationPathWitness fields = %#v", witness.RequiredFields)
	}
	if !slices.Contains(witness.RequiredFields, "relation_declaration_posture") {
		t.Fatalf("RelationPathWitness fields = %#v", witness.RequiredFields)
	}
	if !slices.Contains(witness.OptionalFields, "signature_id") {
		t.Fatalf("RelationPathWitness compatibility fields = %#v", witness.OptionalFields)
	}
}

func assertDistinctOutputFields(
	t *testing.T,
	label string,
	required []string,
	optional []string,
) {
	t.Helper()
	if len(required) == 0 {
		t.Fatalf("%s has no required fields", label)
	}
	seen := make(map[string]bool, len(required)+len(optional))
	for _, field := range append(append([]string{}, required...), optional...) {
		if field == "" || seen[field] {
			t.Fatalf("%s has invalid or duplicate field %q", label, field)
		}
		seen[field] = true
	}
}

func assertDistinctOutputFieldsWhenPresent(
	t *testing.T,
	label string,
	fields []string,
) {
	t.Helper()
	if len(fields) == 0 {
		return
	}
	assertDistinctOutputFields(t, label, fields, nil)
}
