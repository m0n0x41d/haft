package localpractice

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestParseReferenceConstraintCarrierPreservesClosedVariantsAndSourceSpans(t *testing.T) {
	source := readReferenceConstraintCarrier(t)
	parsed, err := Parse(source)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	reparsed, err := Parse(source)
	if err != nil {
		t.Fatalf("Parse(repeated) error = %v", err)
	}
	if !reflect.DeepEqual(parsed.Carrier(), reparsed.Carrier()) {
		t.Fatal("repeated parse produced a different source AST")
	}

	declarations := parsed.Carrier().Signature().Vocabulary().Declarations()
	if len(declarations) != 3 {
		t.Fatalf("declarations = %d, want 3", len(declarations))
	}

	subsetDeclaration, ok := declarations[1].(ConstraintDeclaration)
	if !ok {
		t.Fatalf("subset declaration type = %T", declarations[1])
	}
	assertRange(t, "subset declaration", subsetDeclaration.Span(), 42, 48)
	subset, ok := subsetDeclaration.Rule().(ReferenceSlotSubsetConstraint)
	if !ok {
		t.Fatalf("subset rule type = %T", subsetDeclaration.Rule())
	}
	if subset.Kind() != ConstraintReferenceSlotSubset {
		t.Fatalf("subset kind = %q", subset.Kind())
	}
	if subset.Relation().Value() != "Haft.ReferencePartition" {
		t.Fatalf("subset relation = %q", subset.Relation().Value())
	}
	if subset.Subset().Value() != "SelectedSlot" {
		t.Fatalf("subset coordinate = %q", subset.Subset().Value())
	}
	if subset.Superset().Value() != "WholeSlot" {
		t.Fatalf("superset coordinate = %q", subset.Superset().Value())
	}
	assertRange(t, "subset rule", subset.Span(), 45, 48)
	assertRange(t, "subset coordinate", subset.Subset().Span(), 47, 47)
	assertRange(t, "superset coordinate", subset.Superset().Span(), 48, 48)

	partitionDeclaration, ok := declarations[2].(ConstraintDeclaration)
	if !ok {
		t.Fatalf("partition declaration type = %T", declarations[2])
	}
	assertRange(t, "partition declaration", partitionDeclaration.Span(), 49, 57)
	partition, ok := partitionDeclaration.Rule().(ReferenceSlotPartitionConstraint)
	if !ok {
		t.Fatalf("partition rule type = %T", partitionDeclaration.Rule())
	}
	if partition.Kind() != ConstraintReferenceSlotPartition {
		t.Fatalf("partition kind = %q", partition.Kind())
	}
	if partition.Relation().Value() != "Haft.ReferencePartition" {
		t.Fatalf("partition relation = %q", partition.Relation().Value())
	}
	if partition.Whole().Value() != "WholeSlot" {
		t.Fatalf("partition whole = %q", partition.Whole().Value())
	}
	parts := partition.Parts()
	if len(parts) != 2 || parts[0].Value() != "SelectedSlot" || parts[1].Value() != "RejectedSlot" {
		t.Fatalf("partition parts = %#v", parts)
	}
	assertRange(t, "partition rule", partition.Span(), 52, 57)
	assertRange(t, "partition whole", partition.Whole().Span(), 54, 54)
	assertRange(t, "first partition part", parts[0].Span(), 56, 56)
	assertRange(t, "second partition part", parts[1].Span(), 57, 57)

	parts[0] = SourceText{}
	if partition.Parts()[0].Value() != "SelectedSlot" {
		t.Fatal("ReferenceSlotPartitionConstraint leaked its parts slice")
	}
}

func TestParseRejectsMalformedReferenceSlotConstraints(t *testing.T) {
	valid := string(readReferenceConstraintCarrier(t))
	tests := []struct {
		name        string
		old         string
		replacement string
		want        string
	}{
		{
			name:        "subset unknown key",
			old:         "          superset: WholeSlot",
			replacement: "          superset: WholeSlot\n          recommendation: SelectedSlot",
			want:        "unknown field \"recommendation\"",
		},
		{
			name:        "subset duplicate key",
			old:         "          subset: SelectedSlot",
			replacement: "          subset: SelectedSlot\n          subset: RejectedSlot",
			want:        "duplicate field \"subset\"",
		},
		{
			name:        "subset missing key",
			old:         "          superset: WholeSlot\n",
			replacement: "",
			want:        "missing required field \"superset\"",
		},
		{
			name:        "subset coordinate lacks Slot suffix",
			old:         "          subset: SelectedSlot",
			replacement: "          subset: SelectedPosition",
			want:        "must end with Slot",
		},
		{
			name:        "subset equals superset",
			old:         "          subset: SelectedSlot",
			replacement: "          subset: WholeSlot",
			want:        "coordinates must be distinct",
		},
		{
			name:        "subset relation is not a qualified token",
			old:         "          relation: Haft.ReferencePartition\n          subset: SelectedSlot",
			replacement: "          relation: Haft ReferencePartition\n          subset: SelectedSlot",
			want:        "must not contain whitespace",
		},
		{
			name:        "partition unknown key",
			old:         "          whole: WholeSlot\n          parts:",
			replacement: "          whole: WholeSlot\n          overlap: forbidden\n          parts:",
			want:        "unknown field \"overlap\"",
		},
		{
			name:        "partition duplicate key",
			old:         "          whole: WholeSlot\n          parts:",
			replacement: "          whole: WholeSlot\n          whole: SelectedSlot\n          parts:",
			want:        "duplicate field \"whole\"",
		},
		{
			name: "partition missing key",
			old: "          parts:\n" +
				"            - SelectedSlot\n" +
				"            - RejectedSlot\n",
			replacement: "",
			want:        "missing required field \"parts\"",
		},
		{
			name:        "partition parts is not a sequence",
			old:         "          parts:\n            - SelectedSlot\n            - RejectedSlot",
			replacement: "          parts: SelectedSlot",
			want:        "must be a sequence",
		},
		{
			name:        "partition has fewer than two parts",
			old:         "            - SelectedSlot\n            - RejectedSlot",
			replacement: "            - SelectedSlot",
			want:        "at least two SlotKinds",
		},
		{
			name:        "partition contains duplicate part",
			old:         "            - RejectedSlot",
			replacement: "            - SelectedSlot",
			want:        "duplicate value \"SelectedSlot\"",
		},
		{
			name:        "partition part lacks Slot suffix",
			old:         "            - RejectedSlot",
			replacement: "            - RejectedPosition",
			want:        "must end with Slot",
		},
		{
			name:        "partition whole is also a part",
			old:         "            - RejectedSlot",
			replacement: "            - WholeSlot",
			want:        "whole cannot also be a part",
		},
		{
			name:        "nested alias and anchor remain forbidden",
			old:         "          subset: SelectedSlot\n          superset: WholeSlot",
			replacement: "          subset: &selected SelectedSlot\n          superset: *selected",
			want:        "aliases and anchors",
		},
		{
			name:        "nested folded scalar remains forbidden",
			old:         "          whole: WholeSlot",
			replacement: "          whole: >-\n            WholeSlot",
			want:        "block scalar styles",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := strings.Replace(valid, test.old, test.replacement, 1)
			if source == valid {
				t.Fatalf("fixture mutation did not replace %q", test.old)
			}
			_, err := Parse([]byte(source))
			if err == nil {
				t.Fatal("Parse() accepted malformed reference constraint")
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Parse() error = %q, want substring %q", err, test.want)
			}
		})
	}
}

func readReferenceConstraintCarrier(t *testing.T) []byte {
	t.Helper()
	source, err := os.ReadFile("testdata/valid_reference_constraints.yaml")
	if err != nil {
		t.Fatalf("read reference-constraint carrier: %v", err)
	}
	return source
}
