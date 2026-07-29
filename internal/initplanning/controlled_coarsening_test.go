package initplanning

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func TestControlledCoarseningDeclarationIsCanonicalAndContentAddressed(
	t *testing.T,
) {
	left, err := NewControlledCoarseningDeclaration(
		ControlledCoarseningInput{
			SourceRef:             "skill-source-bundle:source",
			SourceDigest:          digestBytes([]byte("source")),
			RenderingRef:          "host-rendering:pi-package",
			RenderingDigest:       digestBytes([]byte("rendering")),
			NarrowerAdmissibleUse: "pi_host_orientation_and_kernel_routed_invocation",
			SourceLossModes: []SourceLossMode{
				SourceLossRepresentationFactor,
				SourceLossOmittedDetail,
			},
			NonAdmissibleUses: []string{
				"semantic_authority",
				"canonical_skill_source",
			},
			ReopenTriggers: []string{
				"skill_semantics_or_invocation_policy_review",
				"kernel_contract_or_public_skill_set_change",
			},
		},
	)
	if err != nil {
		t.Fatalf("NewControlledCoarseningDeclaration: %v", err)
	}
	right, err := NewControlledCoarseningDeclaration(
		ControlledCoarseningInput{
			SourceRef:             "skill-source-bundle:source",
			SourceDigest:          digestBytes([]byte("source")),
			RenderingRef:          "host-rendering:pi-package",
			RenderingDigest:       digestBytes([]byte("rendering")),
			NarrowerAdmissibleUse: "pi_host_orientation_and_kernel_routed_invocation",
			SourceLossModes: []SourceLossMode{
				SourceLossOmittedDetail,
				SourceLossRepresentationFactor,
			},
			NonAdmissibleUses: []string{
				"canonical_skill_source",
				"semantic_authority",
			},
			ReopenTriggers: []string{
				"kernel_contract_or_public_skill_set_change",
				"skill_semantics_or_invocation_policy_review",
			},
		},
	)
	if err != nil {
		t.Fatalf("NewControlledCoarseningDeclaration reversed: %v", err)
	}
	if left.Digest() != right.Digest() ||
		!bytes.Equal(left.CanonicalBytes(), right.CanonicalBytes()) {
		t.Fatal("input order changed controlled-coarsening identity")
	}
	if !left.Valid() ||
		left.Ref() != "controlled-coarsening:"+
			strings.TrimPrefix(left.Digest(), "sha256:") {
		t.Fatalf("declaration identity is invalid: %s", left.CanonicalBytes())
	}
}

func TestControlledCoarseningDeclarationRejectsMissingBoundaryRows(
	t *testing.T,
) {
	base := ControlledCoarseningInput{
		SourceRef:             "skill-source-bundle:source",
		SourceDigest:          digestBytes([]byte("source")),
		RenderingRef:          "host-rendering:pi-package",
		RenderingDigest:       digestBytes([]byte("rendering")),
		NarrowerAdmissibleUse: "pi_host_orientation_and_kernel_routed_invocation",
		SourceLossModes:       []SourceLossMode{SourceLossOmittedDetail},
		NonAdmissibleUses:     []string{"canonical_skill_source"},
		ReopenTriggers:        []string{"skill_semantics_review"},
	}
	tests := map[string]func(ControlledCoarseningInput) ControlledCoarseningInput{
		"source": func(input ControlledCoarseningInput) ControlledCoarseningInput {
			input.SourceRef = ""
			return input
		},
		"rendering": func(input ControlledCoarseningInput) ControlledCoarseningInput {
			input.RenderingRef = ""
			return input
		},
		"admissible use": func(input ControlledCoarseningInput) ControlledCoarseningInput {
			input.NarrowerAdmissibleUse = ""
			return input
		},
		"loss": func(input ControlledCoarseningInput) ControlledCoarseningInput {
			input.SourceLossModes = nil
			return input
		},
		"non-admissible use": func(input ControlledCoarseningInput) ControlledCoarseningInput {
			input.NonAdmissibleUses = nil
			return input
		},
		"reopen": func(input ControlledCoarseningInput) ControlledCoarseningInput {
			input.ReopenTriggers = nil
			return input
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := NewControlledCoarseningDeclaration(mutate(base))
			if err == nil {
				t.Fatal("incomplete controlled-coarsening declaration accepted")
			}
		})
	}
}

func TestControlledCoarseningDeclarationRejectsUnknownOrDuplicateLoss(
	t *testing.T,
) {
	base := ControlledCoarseningInput{
		SourceRef:             "skill-source-bundle:source",
		SourceDigest:          digestBytes([]byte("source")),
		RenderingRef:          "host-rendering:pi-package",
		RenderingDigest:       digestBytes([]byte("rendering")),
		NarrowerAdmissibleUse: "pi_host_orientation_and_kernel_routed_invocation",
		NonAdmissibleUses:     []string{"canonical_skill_source"},
		ReopenTriggers:        []string{"skill_semantics_review"},
	}
	for name, losses := range map[string][]SourceLossMode{
		"unknown":   {"invented-loss"},
		"duplicate": {SourceLossOmittedDetail, SourceLossOmittedDetail},
	} {
		t.Run(name, func(t *testing.T) {
			input := base
			input.SourceLossModes = losses
			_, err := NewControlledCoarseningDeclaration(input)
			if err == nil {
				t.Fatal("invalid loss modes accepted")
			}
		})
	}
}

func TestControlledCoarseningDeclarationGettersCopySlices(t *testing.T) {
	declaration, err := NewControlledCoarseningDeclaration(
		ControlledCoarseningInput{
			SourceRef:             "skill-source-bundle:source",
			SourceDigest:          digestBytes([]byte("source")),
			RenderingRef:          "host-rendering:pi-package",
			RenderingDigest:       digestBytes([]byte("rendering")),
			NarrowerAdmissibleUse: "orientation",
			SourceLossModes:       []SourceLossMode{SourceLossOmittedDetail},
			NonAdmissibleUses:     []string{"authority"},
			ReopenTriggers:        []string{"reliance"},
		},
	)
	if err != nil {
		t.Fatalf("NewControlledCoarseningDeclaration: %v", err)
	}
	before := declaration.NonAdmissibleUses()
	changed := declaration.NonAdmissibleUses()
	changed[0] = "tampered"
	if !reflect.DeepEqual(declaration.NonAdmissibleUses(), before) {
		t.Fatal("controlled-coarsening declaration exposed mutable slices")
	}
}
