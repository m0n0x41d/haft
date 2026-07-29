package projectmemory

import (
	"github.com/m0n0x41d/haft/internal/projectmemory/memoryresolve"
	"github.com/m0n0x41d/haft/internal/projectmemory/neighborhood"
	"github.com/m0n0x41d/haft/internal/projectmemory/scopedrecall"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorywire"
)

const MemoryReadOutputContractSchemaV1 = "haft.memory-read-output-contract/v1"

// MemoryReadOutputContract describes the public wire representation already
// owned by the closed memory-read domain results. It is discovery metadata,
// not a second validator or a caller-extensible ontology.
type MemoryReadOutputContract struct {
	Schema                string                           `json:"schema"`
	RuntimeContract       string                           `json:"runtime_contract_version"`
	Envelope              MemoryReadOutputFieldSet         `json:"envelope"`
	ProjectionProfileRefs []string                         `json:"projection_profile_refs"`
	ResultFamilies        []MemoryReadResultFamilyContract `json:"result_families"`
	NamedShapes           []MemoryReadNamedShapeContract   `json:"named_shapes"`
}

type MemoryReadOutputFieldSet struct {
	RequiredFields []string `json:"required_fields"`
	OptionalFields []string `json:"optional_fields,omitempty"`
}

type MemoryReadResultFamilyContract struct {
	Action   string                            `json:"action"`
	Variants []MemoryReadOutputVariantContract `json:"variants"`
}

type MemoryReadNamedShapeContract struct {
	Name           string                            `json:"name"`
	Discriminator  string                            `json:"discriminator,omitempty"`
	RequiredFields []string                          `json:"required_fields,omitempty"`
	OptionalFields []string                          `json:"optional_fields,omitempty"`
	AllowedValues  []string                          `json:"allowed_values,omitempty"`
	Variants       []MemoryReadOutputVariantContract `json:"variants,omitempty"`
}

type MemoryReadOutputVariantContract struct {
	Kind                   string   `json:"kind"`
	RequiredEnvelopeFields []string `json:"required_envelope_fields,omitempty"`
	RequiredFields         []string `json:"required_fields"`
	OptionalFields         []string `json:"optional_fields,omitempty"`
}

// MemoryReadOutputContractV1 returns a fresh descriptor so callers cannot
// mutate a package-global slice and silently change interface discovery.
func MemoryReadOutputContractV1() MemoryReadOutputContract {
	profiles := neighborhood.BuiltinProjectionProfiles()
	profileRefs := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		profileRefs = append(profileRefs, profile.Ref().String())
	}

	contract := MemoryReadOutputContract{
		Schema:          MemoryReadOutputContractSchemaV1,
		RuntimeContract: typedmemorywire.ContractVersion,
		Envelope: MemoryReadOutputFieldSet{
			RequiredFields: []string{
				"contract_version",
				"action",
				"result_kind",
				"result",
			},
			OptionalFields: []string{"result_digest"},
		},
		ProjectionProfileRefs: profileRefs,
		ResultFamilies:        memoryReadResultFamiliesV1(),
		NamedShapes:           memoryReadNamedShapesV1(),
	}
	return contract
}

func memoryReadResultFamiliesV1() []MemoryReadResultFamilyContract {
	return []MemoryReadResultFamilyContract{
		{
			Action: typedmemorywire.ActionResolve,
			Variants: []MemoryReadOutputVariantContract{
				{
					Kind: string(memoryresolve.ResultExactEntity),
					RequiredFields: []string{
						"resolution_scope",
						"snapshot_basis",
						"interpretation_contract",
						"entity",
						"resolution_witnesses",
					},
				},
				{
					Kind: string(memoryresolve.ResultKnownAbsent),
					RequiredFields: []string{
						"resolution_scope",
						"snapshot_basis",
						"interpretation_contract",
						"inspected_index",
						"completeness_basis_ref",
					},
				},
				{
					Kind: string(memoryresolve.ResultEntityCandidates),
					RequiredFields: []string{
						"resolution_scope",
						"snapshot_basis",
						"interpretation_contract",
						"candidates",
						"candidate_set_coverage",
						"applied_budget",
					},
				},
				{
					Kind: string(memoryresolve.ResultResolutionUnsettled),
					RequiredFields: []string{
						"resolution_scope",
						"snapshot_basis",
						"interpretation_contract",
						"issues",
					},
				},
				{
					Kind: string(memoryresolve.ResultRetryRequired),
					RequiredFields: []string{
						"resolution_scope",
						"snapshot_basis",
						"interpretation_contract",
						"observed_snapshot",
						"required_snapshot",
						"cause",
						"retry_operation",
					},
				},
			},
		},
		{
			Action: typedmemorywire.ActionNeighborhood,
			Variants: []MemoryReadOutputVariantContract{
				{
					Kind:                   string(neighborhood.ResultExactNeighborhood),
					RequiredEnvelopeFields: []string{"result_digest"},
					RequiredFields: []string{
						"schema",
						"memory_view_context",
						"snapshot_basis",
						"projection_basis",
						"projection_basis_digest",
						"root",
						"facets",
						"boundaries",
						"interpretation_contract",
						"read_affordances",
						"applied_budget",
					},
				},
				{
					Kind: string(neighborhood.ResultRetryRequired),
					RequiredFields: []string{
						"cause",
						"required_snapshot",
						"retry_operation",
						"interpretation_contract",
					},
				},
				{
					Kind: string(neighborhood.ResultAbstained),
					RequiredFields: []string{
						"basis",
						"inspected_sources",
						"interpretation_contract",
					},
				},
			},
		},
		{
			Action: typedmemorywire.ActionRecall,
			Variants: []MemoryReadOutputVariantContract{
				{
					Kind: string(scopedrecall.ScopedResultCandidateSet),
					RequiredFields: []string{
						"scope",
						"snapshot_basis",
						"interpretation_contract",
						"candidates",
						"candidate_set_coverage",
						"applied_budget",
					},
				},
				{
					Kind: string(scopedrecall.ScopedResultRetryRequired),
					RequiredFields: []string{
						"scope",
						"snapshot_basis",
						"interpretation_contract",
						"cause",
						"required_snapshot",
						"retry_operation",
					},
				},
				{
					Kind: string(scopedrecall.ScopedResultAbstained),
					RequiredFields: []string{
						"scope",
						"snapshot_basis",
						"interpretation_contract",
						"inspected_producers",
						"basis",
					},
				},
			},
		},
	}
}

func memoryReadNamedShapesV1() []MemoryReadNamedShapeContract {
	return []MemoryReadNamedShapeContract{
		{
			Name: "ProjectionBasis",
			RequiredFields: []string{
				"schema",
				"profile_ref",
				"profile_edition",
				"profile_digest",
				"projection_schema_version",
				"canonical_inputs",
				"derived_projection_inputs",
				"declared_input_families",
				"declared_slot_kinds",
				"correspondence_manifests",
				"item_basis",
			},
		},
		{
			Name:          "ProjectionItemBasis",
			Discriminator: "kind",
			Variants: []MemoryReadOutputVariantContract{
				{
					Kind: string(neighborhood.ItemBasisDirect),
					RequiredFields: []string{
						"kind",
						"output",
						"inputs",
						"transform",
						"intentional_loss",
					},
				},
				{
					Kind: string(neighborhood.ItemBasisCorrespondence),
					RequiredFields: []string{
						"kind",
						"output",
						"inputs",
						"correspondence_manifest_ref",
						"transform",
						"intentional_loss",
					},
				},
			},
		},
		{
			Name:          "FacetBasisIssue",
			Discriminator: "kind",
			Variants: []MemoryReadOutputVariantContract{
				{
					Kind: string(neighborhood.IssueMissingTypeBasis),
					RequiredFields: []string{
						"kind",
						"facet",
						"required_ref_or_kind",
					},
				},
				{
					Kind: string(neighborhood.IssueMissingCorrespondenceBasis),
					RequiredFields: []string{
						"kind",
						"facet",
						"required_correspondence",
					},
				},
				{
					Kind: string(neighborhood.IssueUnresolvedLegacyIdentity),
					RequiredFields: []string{
						"kind",
						"facet",
						"legacy_ref",
						"resolution_ref",
					},
				},
				{
					Kind: string(neighborhood.IssueStaleDerivedProjection),
					RequiredFields: []string{
						"kind",
						"facet",
						"projection_ref",
						"observed_version",
						"required_version",
					},
				},
				{
					Kind: string(neighborhood.IssueExplicitBridgeRequired),
					RequiredFields: []string{
						"kind",
						"facet",
						"source_context_ref",
						"target_context_ref",
						"bridge",
					},
					OptionalFields: []string{"known_bridge_ref"},
				},
			},
		},
		{
			Name:          "WholeReadRetryCause",
			Discriminator: "kind",
			Variants: []MemoryReadOutputVariantContract{
				{
					Kind: string(neighborhood.RetryStaleSnapshot),
					RequiredFields: []string{
						"kind",
						"observed_snapshot",
						"required_snapshot",
					},
				},
				{
					Kind: string(neighborhood.RetryStaleCursor),
					RequiredFields: []string{
						"kind",
						"cursor",
						"required_snapshot",
					},
				},
				{
					Kind: string(neighborhood.RetryProjectionRebuildRequired),
					RequiredFields: []string{
						"kind",
						"projection_ref",
						"observed_epoch",
						"required_epoch",
					},
				},
			},
		},
		{
			Name:          "ReadAbstentionBasis",
			Discriminator: "kind",
			Variants: []MemoryReadOutputVariantContract{
				{
					Kind: string(neighborhood.AbstainEntityOrContextNotFound),
					RequiredFields: []string{
						"kind",
						"entity_ref",
						"bounded_context_ref",
						"snapshot_basis",
					},
				},
				{
					Kind:           string(neighborhood.AbstainNoAdmissibleFacet),
					RequiredFields: []string{"kind", "issues"},
				},
			},
		},
		{
			Name: "InterpretationContract",
			RequiredFields: []string{
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
		},
		{
			Name: "RelationalRecordsInterpretation",
			AllowedValues: []string{
				string(neighborhood.RelationalRecordsAssertionsExactAtSnapshot),
				string(neighborhood.RelationalRecordsOccurrencesExactAtSnapshot),
				string(neighborhood.RelationalRecordsLegacyUnqualifiedAssertions),
				string(neighborhood.RelationalRecordsCandidateAssertions),
				string(neighborhood.RelationalRecordsHeterogeneous),
				string(neighborhood.RelationalRecordsUnavailable),
			},
		},
		{
			Name: "RelationalRecordItemPosture",
			AllowedValues: []string{
				string(neighborhood.RelationalRecordItemAssertionExact),
				string(neighborhood.RelationalRecordItemOccurrenceExact),
				string(neighborhood.RelationalRecordItemLegacyUnqualifiedAssertion),
				string(neighborhood.RelationalRecordItemCandidateAssertion),
			},
		},
		{
			Name: "RelationDeclarationPosture",
			AllowedValues: []string{
				string(typedmemory.RelationDeclarationTypedFragment),
			},
		},
		{
			Name: "RelationPathWitness",
			RequiredFields: []string{
				"assertion_id",
				"relation_declaration_fragment_id",
				"relation_declaration_posture",
				"bounded_context_ref",
				"slot_kind_id",
				"target_ref",
				"provenance_ref",
				"admission_event_ref",
				"relational_record_posture",
			},
			OptionalFields: []string{"signature_id", "explicit_modality"},
		},
		{
			Name:          "FacetCoverage",
			Discriminator: "kind",
			Variants: []MemoryReadOutputVariantContract{
				{
					Kind:           string(neighborhood.CoverageComplete),
					RequiredFields: []string{"kind", "included"},
				},
				{
					Kind: string(neighborhood.CoveragePartial),
					RequiredFields: []string{
						"kind",
						"included",
						"omitted_at_least",
						"snapshot_cursor",
					},
				},
				{
					Kind: string(neighborhood.CoverageNotApplicable),
					RequiredFields: []string{
						"kind",
						"included",
						"applicability_basis_ref",
					},
				},
				{
					Kind: string(neighborhood.CoverageUnavailable),
					RequiredFields: []string{
						"kind",
						"included",
						"missing_basis_ref",
					},
				},
				{
					Kind: string(neighborhood.CoverageStale),
					RequiredFields: []string{
						"kind",
						"included",
						"retry_basis_ref",
					},
				},
			},
		},
		{
			Name: "AppliedReadBudget",
			RequiredFields: []string{
				"requested_limits",
				"applied_limits",
				"per_facet",
				"emitted_relation_path_count",
				"omitted_relation_path_count",
				"emitted_excerpt_character_count",
				"emitted_provenance_depth",
				"bounded_content_utf8_bytes",
				"continuation_cursors",
			},
		},
		{
			Name: "RetryRequired",
			RequiredFields: []string{
				"cause",
				"required_snapshot",
				"retry_operation",
				"interpretation_contract",
			},
		},
		{
			Name:          "ScopedRecallAbstentionBasis",
			Discriminator: "kind",
			Variants: []MemoryReadOutputVariantContract{
				{
					Kind: string(scopedrecall.AbstentionNoMatchingMemory),
					RequiredFields: []string{
						"kind",
						"complete_producer_refs",
					},
				},
				{
					Kind: string(scopedrecall.AbstentionNoUsableProducer),
					RequiredFields: []string{
						"kind",
						"unavailable_producer_refs",
						"missing_basis_ref",
					},
				},
			},
		},
	}
}
