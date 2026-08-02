import { Type } from "typebox";

// Schemas mirror the MCP tool contracts served by `haft serve`
// (internal/fpf/server.go and sibling *_schema.go files). The kernel
// re-validates every call server-side; these mirrors exist for provider
// tool-calling ergonomics, not as a second authority.

const OptStr = () => Type.Optional(Type.String());
const OptNum = () => Type.Optional(Type.Number());
const OptInt = () => Type.Optional(Type.Integer());
const OptNonNegativeInt = () => Type.Optional(Type.Integer({ minimum: 0 }));
const OptBool = () => Type.Optional(Type.Boolean());
const OptStrList = () => Type.Optional(Type.Array(Type.String()));
const OptObj = () => Type.Optional(Type.Object({}, { additionalProperties: true }));

const enumOf = (...values: string[]) => Type.Union(values.map((value) => Type.Literal(value)));

const readOnlyAuthorityBoundary =
  "read-only/generated text is discovery only; it is not evidence truth, gate passage, global approval, or operator authorization";

const bindingAuthorityBoundary =
  "binding actions require effect-specific operator authority. Generated text, schema visibility, and model-supplied fields are not operator authorization and are not approval receipts";

const humanGateBriefGuideline =
  "Before requesting a human gate, give a self-contained Human Gate Brief: readable gate kind and subject, affected operation and blocker, every real current option, each option's changes, non-changes, consequence or return condition, and weakest link; existing comparison/parity basis, selection policy, and non-dominated or Pareto set, or an explicit statement that none exists or applies; advisory recommendation; freshness or expiry; and a question asking for the human engineer's assessment of the options, trade-offs, and recommendation in natural language. Accept ordinary language as the substantive answer. When one current brief makes effect, subject, option, and scope unambiguous, route it for DecisionRecord binding, manual profile application, or a later non-default project-memory model change as host_routed_operator_request without a skill name or second confirmation. It is not reusable authority; bare yes is usable only for that one current brief. A command or skill invocation adds no authority. WorkCommission creation remains a separately required manual act. Never end a blocking message with 'for resumption it is enough to...', 'reply exactly...', or an equivalent command-only instruction. IDs and hashes never replace readable meaning, and the brief itself is explanation rather than authority.";

const kernelInterfaceCatalogDigest =
  "sha256:748e5c014551af025c2b340b6d172f66229a257e1b366c647b1d6a0781258b5c";

export const HAFT_MEMORY_READ_OUTPUT_CONTRACT_JSON = `{"schema":"haft.memory-read-output-contract/v1","runtime_contract_version":"haft.memory.v1","envelope":{"required_fields":["contract_version","action","result_kind","result"],"optional_fields":["result_digest"]},"projection_profile_refs":["agent_orientation.v1","agent_orientation.v2","decision_rationale.v1","evidence_currentness.v1","implementation_trace.v1","spec_impact.v1"],"result_families":[{"action":"resolve","variants":[{"kind":"exact_entity","required_fields":["resolution_scope","snapshot_basis","interpretation_contract","entity","resolution_witnesses"]},{"kind":"known_absent","required_fields":["resolution_scope","snapshot_basis","interpretation_contract","inspected_index","completeness_basis_ref"]},{"kind":"entity_candidates","required_fields":["resolution_scope","snapshot_basis","interpretation_contract","candidates","candidate_set_coverage","applied_budget"]},{"kind":"resolution_unsettled","required_fields":["resolution_scope","snapshot_basis","interpretation_contract","issues"]},{"kind":"retry_required","required_fields":["resolution_scope","snapshot_basis","interpretation_contract","observed_snapshot","required_snapshot","cause","retry_operation"]}]},{"action":"neighborhood","variants":[{"kind":"exact_neighborhood","required_envelope_fields":["result_digest"],"required_fields":["schema","memory_view_context","snapshot_basis","projection_basis","projection_basis_digest","root","facets","boundaries","interpretation_contract","read_affordances","applied_budget"]},{"kind":"retry_required","required_fields":["cause","required_snapshot","retry_operation","interpretation_contract"]},{"kind":"abstained","required_fields":["basis","inspected_sources","interpretation_contract"]}]},{"action":"recall","variants":[{"kind":"scoped_memory_candidate_set","required_fields":["scope","snapshot_basis","interpretation_contract","candidates","candidate_set_coverage","applied_budget"]},{"kind":"retry_required","required_fields":["scope","snapshot_basis","interpretation_contract","cause","required_snapshot","retry_operation"]},{"kind":"abstained","required_fields":["scope","snapshot_basis","interpretation_contract","inspected_producers","basis"]}]}],"named_shapes":[{"name":"ProjectionBasis","required_fields":["schema","profile_ref","profile_edition","profile_digest","projection_schema_version","canonical_inputs","derived_projection_inputs","declared_input_families","declared_slot_kinds","correspondence_manifests","item_basis"]},{"name":"ProjectionItemBasis","discriminator":"kind","variants":[{"kind":"direct","required_fields":["kind","output","inputs","transform","intentional_loss"]},{"kind":"correspondence","required_fields":["kind","output","inputs","correspondence_manifest_ref","transform","intentional_loss"]}]},{"name":"FacetBasisIssue","discriminator":"kind","variants":[{"kind":"missing_type_basis","required_fields":["kind","facet","required_ref_or_kind"]},{"kind":"missing_correspondence_basis","required_fields":["kind","facet","required_correspondence"]},{"kind":"unresolved_legacy_identity","required_fields":["kind","facet","legacy_ref","resolution_ref"]},{"kind":"stale_derived_projection","required_fields":["kind","facet","projection_ref","observed_version","required_version"]},{"kind":"explicit_bridge_required","required_fields":["kind","facet","source_context_ref","target_context_ref","bridge"],"optional_fields":["known_bridge_ref"]}]},{"name":"WholeReadRetryCause","discriminator":"kind","variants":[{"kind":"stale_snapshot","required_fields":["kind","observed_snapshot","required_snapshot"]},{"kind":"stale_cursor","required_fields":["kind","cursor","required_snapshot"]},{"kind":"projection_rebuild_required","required_fields":["kind","projection_ref","observed_epoch","required_epoch"]}]},{"name":"ReadAbstentionBasis","discriminator":"kind","variants":[{"kind":"entity_or_context_not_found","required_fields":["kind","entity_ref","bounded_context_ref","snapshot_basis"]},{"kind":"no_admissible_facet","required_fields":["kind","issues"]}]},{"name":"InterpretationContract","required_fields":["structure","identity","relational_records","ranking","truth","applicability","authority","work_order","completeness","hydrate_before_reliance"]},{"name":"RelationalRecordsInterpretation","allowed_values":["assertions_exact_at_snapshot","occurrences_exact_at_snapshot","legacy_unqualified_assertions","candidate_assertions","heterogeneous_relational_records","unavailable"]},{"name":"RelationalRecordItemPosture","allowed_values":["assertion_exact","occurrence_exact","legacy_unqualified_assertion","candidate_assertion"]},{"name":"RelationDeclarationPosture","allowed_values":["typed_relation_declaration_fragment"]},{"name":"RelationPathWitness","required_fields":["assertion_id","relation_declaration_fragment_id","relation_declaration_posture","bounded_context_ref","slot_kind_id","target_ref","provenance_ref","admission_event_ref","relational_record_posture"],"optional_fields":["signature_id","explicit_modality"]},{"name":"FacetCoverage","discriminator":"kind","variants":[{"kind":"complete","required_fields":["kind","included"]},{"kind":"partial","required_fields":["kind","included","omitted_at_least","snapshot_cursor"]},{"kind":"not_applicable","required_fields":["kind","included","applicability_basis_ref"]},{"kind":"unavailable","required_fields":["kind","included","missing_basis_ref"]},{"kind":"stale","required_fields":["kind","included","retry_basis_ref"]}]},{"name":"AppliedReadBudget","required_fields":["requested_limits","applied_limits","per_facet","emitted_relation_path_count","omitted_relation_path_count","emitted_excerpt_character_count","emitted_provenance_depth","bounded_content_utf8_bytes","continuation_cursors"]},{"name":"RetryRequired","required_fields":["cause","required_snapshot","retry_operation","interpretation_contract"]},{"name":"ScopedRecallAbstentionBasis","discriminator":"kind","variants":[{"kind":"no_matching_memory","required_fields":["kind","complete_producer_refs"]},{"kind":"no_usable_producer","required_fields":["kind","unavailable_producer_refs","missing_basis_ref"]}]}]}`;

export const HAFT_MEMORY_READ_OUTPUT_CONTRACT: unknown =
  JSON.parse(HAFT_MEMORY_READ_OUTPUT_CONTRACT_JSON);

const parityPlanSchema = Type.Optional(Type.Object({
  baseline_set: OptStrList(),
  budget: OptStr(),
  missing_data_policy: Type.Optional(enumOf("explicit_abstain", "zero", "exclude")),
  normalization: Type.Optional(Type.Array(Type.Object({
    dimension: OptStr(),
    method: OptStr()
  }))),
  pinned_conditions: OptStrList(),
  window: OptStr()
}));

const carryThroughItemSchema = Type.Object({
  source_ref: Type.String(),
  source_item_ref: Type.String(),
  acceptance_ref: Type.String(),
  acceptance_ref_kind: Type.Optional(enumOf("operator_message", "review_disposition", "decision_record", "manual_cli_receipt", "external_unverified", "unknown")),
  acceptance_ref_status: Type.Optional(enumOf("verified", "externally_asserted", "missing", "malformed")),
  disposition: Type.Optional(enumOf("pending", "applied", "rejected", "deferred", "superseded")),
  target_refs: OptStrList(),
  evidence_refs: OptStrList(),
  reason: OptStr()
}, { additionalProperties: true });

const specFitProbeSchema = Type.Object({
  problem_signal: OptStr(),
  scope: OptStr(),
  mode: OptStr(),
  section_refs: OptStrList(),
  affected_files: OptStrList(),
  target_refs: OptStrList(),
  conflict_refs: OptStrList(),
  declared_relation: OptStr()
}, { additionalProperties: true });

const specFitVariantSchema = Type.Object({
  id: OptStr(),
  title: OptStr(),
  description: OptStr(),
  section_refs: OptStrList(),
  affected_files: OptStrList(),
  target_refs: OptStrList(),
  conflict_refs: OptStrList(),
  declared_relation: OptStr()
}, { additionalProperties: true });

const memoryProjectBasisSchema = Type.Union([
  Type.Object({
    kind: Type.Literal("project_current")
  }, { additionalProperties: false }),
  Type.Object({
    kind: Type.Literal("exact_project"),
    type_env_digest: Type.String({ pattern: "^sha256:[0-9a-f]{64}$" }),
    graph_revision: Type.Integer({ minimum: 1 })
  }, { additionalProperties: false })
]);

const memoryEntityReferenceSchema = Type.Object({
  ref_kind_id: Type.String(),
  reference_id: Type.String()
}, { additionalProperties: false });

const memoryProjectRecordReferenceSchema = Type.Object({
  ref_kind_id: Type.Literal("Haft.ProjectRecordRef"),
  reference_id: Type.String()
}, { additionalProperties: false });

const memoryIdentifierSchema = Type.String({ minLength: 1, maxLength: 4096 });
const memoryTextSchema = Type.String({ minLength: 1, maxLength: 16384 });
const memoryDigestSchema = Type.String({ pattern: "^sha256:[0-9a-f]{64}$" });

const memoryValidationBasisSchema = Type.Union([
  Type.Object({
    kind: Type.Literal("bundled_candidate_open_world")
  }, { additionalProperties: false }),
  Type.Object({
    kind: Type.Literal("project_current")
  }, { additionalProperties: false }),
  Type.Object({
    kind: Type.Literal("exact_project"),
    type_env_digest: memoryDigestSchema,
    graph_revision: Type.Integer({ minimum: 0 })
  }, { additionalProperties: false })
]);

const memoryVersionedPinSchema = Type.Object({
  reference: memoryIdentifierSchema,
  edition: memoryIdentifierSchema,
  digest: memoryDigestSchema
}, { additionalProperties: false });

const memoryEnvironmentSelectorSchema = Type.Object({
  key: memoryIdentifierSchema,
  value: memoryIdentifierSchema,
  source_digest: memoryDigestSchema
}, { additionalProperties: false });

const memoryGammaPointSchema = Type.Object({
  kind: Type.Literal("point"),
  at: memoryIdentifierSchema
}, { additionalProperties: false });

const memoryGammaWindowSchema = Type.Object({
  kind: Type.Literal("window"),
  start: memoryIdentifierSchema,
  end: memoryIdentifierSchema,
  start_boundary: enumOf("inclusive", "exclusive"),
  end_boundary: enumOf("inclusive", "exclusive")
}, { additionalProperties: false });

const memoryResolvedGammaTimeSchema = Type.Union([
  memoryGammaPointSchema,
  memoryGammaWindowSchema
]);

const memoryGammaPolicyApplicationSchema = Type.Object({
  kind: Type.Literal("policy_application"),
  policy_ref: memoryIdentifierSchema,
  policy_edition: memoryIdentifierSchema,
  policy_digest: memoryDigestSchema,
  evaluation_anchor: memoryGammaPointSchema,
  resolved: memoryResolvedGammaTimeSchema
}, { additionalProperties: false });

const memoryGammaTimeSchema = Type.Union([
  memoryGammaPointSchema,
  memoryGammaWindowSchema,
  memoryGammaPolicyApplicationSchema
]);

const memoryContextSliceSchema = Type.Object({
  context: memoryIdentifierSchema,
  standard_pins: Type.Array(memoryVersionedPinSchema, { maxItems: 64 }),
  environment_selectors: Type.Array(memoryEnvironmentSelectorSchema, { maxItems: 64 }),
  vocabulary_pins: Type.Array(memoryVersionedPinSchema, { maxItems: 64 }),
  role_set_pins: Type.Array(memoryVersionedPinSchema, { maxItems: 64 }),
  gamma_time: memoryGammaTimeSchema
}, { additionalProperties: false });

const memoryPersistedReferenceSchema = Type.Object({
  kind: Type.Literal("persisted"),
  ref_kind: memoryIdentifierSchema,
  id: memoryIdentifierSchema
}, { additionalProperties: false });

const memoryLocalReferenceSchema = Type.Object({
  kind: Type.Literal("local"),
  ref_kind: memoryIdentifierSchema,
  local_ref: memoryIdentifierSchema
}, { additionalProperties: false });

const memoryReferenceSchema = Type.Union([
  memoryPersistedReferenceSchema,
  memoryLocalReferenceSchema
]);

const memoryReferenceFillerSchema = Type.Object({
  kind: Type.Literal("by_reference"),
  reference: memoryReferenceSchema
}, { additionalProperties: false });

const memoryValueShapeSchema = Type.Object({
  id: memoryIdentifierSchema,
  digest: memoryDigestSchema
}, { additionalProperties: false });

const memoryCodecSchema = Type.Object({
  id: memoryIdentifierSchema,
  version: memoryIdentifierSchema,
  specification_digest: memoryDigestSchema
}, { additionalProperties: false });

const memoryAssertedDigestSchema = Type.Union([
  Type.Object({
    kind: Type.Literal("none")
  }, { additionalProperties: false }),
  Type.Object({
    kind: Type.Literal("exact"),
    digest: memoryDigestSchema
  }, { additionalProperties: false })
]);

const memoryTypedValueCandidateSchema = Type.Object({
  value_kind: memoryIdentifierSchema,
  value_shape: memoryValueShapeSchema,
  codec: memoryCodecSchema,
  input_base64: Type.String({ minLength: 1, maxLength: 349528 }),
  asserted_digest: memoryAssertedDigestSchema
}, { additionalProperties: false });

const memoryValueFillerSchema = Type.Object({
  kind: Type.Literal("by_value"),
  value: memoryTypedValueCandidateSchema
}, { additionalProperties: false });

const memoryFillerSchema = Type.Union([
  memoryReferenceFillerSchema,
  memoryValueFillerSchema
]);

const memorySlotBindingSchema = Type.Object({
  slot_kind: memoryIdentifierSchema,
  fillers: Type.Array(memoryFillerSchema, { minItems: 1, maxItems: 64 })
}, { additionalProperties: false });

const memoryDeclareEntityChangeSchema = Type.Object({
  kind: Type.Literal("declare_entity"),
  entity_id: memoryIdentifierSchema,
  local_ref: memoryIdentifierSchema,
  context: memoryIdentifierSchema,
  label: memoryTextSchema,
  provenance: memoryIdentifierSchema
}, { additionalProperties: false });

const memoryAdmitAliasSchema = Type.Object({
  kind: Type.Literal("admit_alias"),
  entity_id: memoryIdentifierSchema,
  alias: memoryIdentifierSchema,
  context: memoryIdentifierSchema,
  provenance: memoryIdentifierSchema
}, { additionalProperties: false });

const memorySupersedeAliasSchema = Type.Object({
  kind: Type.Literal("supersede_alias"),
  entity_id: memoryIdentifierSchema,
  old_alias: memoryIdentifierSchema,
  replacement: memoryIdentifierSchema,
  context: memoryIdentifierSchema,
  provenance: memoryIdentifierSchema
}, { additionalProperties: false });

const memoryIdentityOperationSchema = Type.Union([
  memoryAdmitAliasSchema,
  memorySupersedeAliasSchema
]);

const memoryIdentityChangeSchema = Type.Object({
  kind: Type.Literal("identity_change"),
  change: memoryIdentityOperationSchema
}, { additionalProperties: false });

const memoryAssertionModalitySchema = Type.Union([
  Type.Object({
    kind: Type.Literal("affirms_obtaining")
  }, { additionalProperties: false }),
  Type.Object({
    kind: Type.Literal("denies_obtaining")
  }, { additionalProperties: false }),
  Type.Object({
    kind: Type.Literal("obtaining_unknown")
  }, { additionalProperties: false })
]);

const memoryAssertRelationChangeSchema = Type.Object({
  kind: Type.Literal("assert_relation"),
  assertion_id: memoryIdentifierSchema,
  signature_id: memoryIdentifierSchema,
  context_slice: memoryContextSliceSchema,
  modality: memoryAssertionModalitySchema,
  bindings: Type.Array(memorySlotBindingSchema, { minItems: 1, maxItems: 64 }),
  provenance: memoryIdentifierSchema
}, { additionalProperties: false });

const memoryRetractAssertionChangeSchema = Type.Object({
  kind: Type.Literal("retract_assertion"),
  assertion_id: memoryIdentifierSchema,
  reason: memoryTextSchema,
  provenance: memoryIdentifierSchema
}, { additionalProperties: false });

const memoryChangeSchema = Type.Union([
  memoryDeclareEntityChangeSchema,
  memoryIdentityChangeSchema,
  memoryAssertRelationChangeSchema,
  memoryRetractAssertionChangeSchema
]);

const memoryChangeSetSchema = Type.Object({
  changes: Type.Array(memoryChangeSchema, { minItems: 1, maxItems: 64 })
}, { additionalProperties: false });

const memoryNeighborhoodViewSchema = Type.Object({
  projection_profile_ref: enumOf(
    "agent_orientation.v2",
    "agent_orientation.v1",
    "decision_rationale.v1",
    "spec_impact.v1",
    "evidence_currentness.v1",
    "implementation_trace.v1"
  ),
  requested_facets: Type.Array(enumOf(
    "epistemes",
    "problems",
    "alternatives",
    "decisions",
    "specifications",
    "evidence",
    "work",
    "implementation",
    "unresolved"
  )),
  detail: enumOf("overview", "standard", "evidence"),
  include_history: Type.Boolean()
}, { additionalProperties: false });

const memoryReadBudgetSchema = Type.Object({
  max_facets: Type.Integer({ minimum: 1 }),
  max_items_per_facet: Type.Integer({ minimum: 1 }),
  max_relation_paths_per_item: Type.Integer({ minimum: 1 }),
  max_carrier_excerpt_characters: Type.Integer({ minimum: 1 }),
  max_provenance_depth: Type.Integer({ minimum: 1 })
}, { additionalProperties: false });

const memoryResolveQuerySchema = Type.Object({
  mode: Type.Literal("resolve"),
  contract_version: Type.Literal("haft.memory.v1"),
  basis: memoryProjectBasisSchema,
  query: memoryTextSchema,
  bounded_context_ref: Type.Optional(memoryIdentifierSchema),
  max_candidates: Type.Integer({ minimum: 1, maximum: 4294967295 })
}, {
  additionalProperties: false,
  description: "Closed read-only resolve branch. known_absent performs no write and grants no persistence authority."
});

const memoryNeighborhoodQuerySchema = Type.Object({
  mode: Type.Literal("neighborhood"),
  contract_version: Type.Literal("haft.memory.v1"),
  basis: memoryProjectBasisSchema,
  entity_ref: memoryEntityReferenceSchema,
  bounded_context_ref: memoryIdentifierSchema,
  view: memoryNeighborhoodViewSchema,
  read_budget: memoryReadBudgetSchema
}, {
  additionalProperties: false,
  description: "Closed read-only neighborhood branch for one exact EntityOfConcern and bounded context."
});

const memoryRecallQuerySchema = Type.Object({
  mode: Type.Literal("recall"),
  contract_version: Type.Literal("haft.memory.v1"),
  basis: memoryProjectBasisSchema,
  entity_ref: memoryEntityReferenceSchema,
  bounded_context_ref: memoryIdentifierSchema,
  view: memoryNeighborhoodViewSchema,
  read_budget: memoryReadBudgetSchema,
  query: memoryTextSchema,
  candidate_budget: Type.Object({
    max_candidates: Type.Integer({ minimum: 1, maximum: 4294967295 })
  }, { additionalProperties: false })
}, {
  additionalProperties: false,
  description: "Closed read-only lexical recall branch inside one exact EntityOfConcern scope."
});

const memoryQueryEnvelopeSchema = Type.Union([
  memoryResolveQuerySchema,
  memoryNeighborhoodQuerySchema,
  memoryRecallQuerySchema
], {
  description: "Required when action=memory. Choose exactly one closed branch. Legacy flat memory fields are rejected."
});

const haftQueryParameters = Type.Object({
  action: enumOf(
    "search", "status", "board", "related", "code_context", "callees", "callers",
    "impact", "node", "explore", "ceremony", "projection", "list", "coverage",
    "fpf", "check", "carrier_manifest", "carrier_check", "contract_audit",
    "contract_generation", "spec_review", "spec_use", "spec_trace", "spec_binding_preflight", "spec_fit_probe", "change_case",
    "correspondence_graph", "drift_route", "drift_events", "decision_reconcile",
    "governing_set", "blocked_use", "value_space", "evidence_path", "resolve_term",
    "memory"
  ),
  artifact_ref: Type.Optional(Type.String({
    description: "Canonical Haft artifact ID for action=related exact recovery. Code symbols use symbol; FPF source IDs use identifier. A wrong_identifier_namespace response supplies the exact recovery_call."
  })),
  anchor_id: OptStr(),
  ref: OptStr(),
  artifact_id: OptStr(),
  attempted_use: OptStr(),
  bearer_ref: OptStr(),
  blocked_use: OptStr(),
  claim_ref: OptStr(),
  context: OptStr(),
  depth: OptNum(),
  decision_draft: OptObj(),
  drift_kind: OptStr(),
  explain: OptBool(),
  exact_record_needed: OptStr(),
  entity_of_concern: OptStr(),
  evidence_ref: OptStr(),
  file: OptStr(),
  files: OptStrList(),
  full: OptBool(),
  kind: OptStr(),
  identifier: Type.Optional(Type.String({
    description: "FPF source identifier only for action=fpf mode=lookup or mode=inspect. A Haft artifact ID or code symbol is a wrong_identifier_namespace; use artifact_ref with action=related or symbol with action=node."
  })),
  intended_use: OptStr(),
  known_context: OptStrList(),
  label: OptStr(),
  lane: Type.Optional(enumOf("index", "symbols", "decisions", "invariants", "notes", "problems", "portfolios", "all")),
  limit: OptNum(),
  line: OptNum(),
  method_ref: OptStr(),
  max_candidates_per_role: OptNonNegativeInt(),
  max_candidates: OptNonNegativeInt(),
  max_excerpt_characters: OptNonNegativeInt(),
  max_relations_per_candidate: OptNonNegativeInt(),
  max_total_candidates: OptNonNegativeInt(),
  mode: Type.Optional(enumOf(
    "concern",
    "lookup",
    "inspect",
    "tactical",
    "standard",
    "deep"
  )),
  memory_request: Type.Optional(memoryQueryEnvelopeSchema),
  operational_gate: OptObj(),
  policy: Type.Optional(enumOf("documentary_only", "stronger_use_requires_current_source", "temporary_waiver")),
  producer_ref: OptStr(),
  probe: Type.Optional(specFitProbeSchema),
  query: OptStr(),
  requires_current_formality: OptBool(),
  roles: Type.Optional(Type.Array(enumOf("practical_use_card", "preface", "toc_row", "pattern_body", "pattern_section"))),
  section_id: OptStr(),
  scope_id: Type.Optional(Type.String({
    description: "For action=status or coverage, one exact canonical project ScopeID. When a mixed-profile response reports available ScopeIDs, retry the same read-only call with one exact value; never select by display order."
  })),
  source_refs: OptStrList(),
  symbol: Type.Optional(Type.String({
    description: "For action=node, a code symbol only. A canonical Haft artifact ID returns wrong_identifier_namespace; recover it with haft_query(action=related, artifact_ref=<id>), not another node call."
  })),
  term: OptStr(),
  trace_ref: Type.Optional(Type.String({
    description: "For action=fpf with view=trace or view=diagnostic, the opaque replay reference returned by an earlier FPF response. Source-snapshot or typed-request drift returns replay_mismatch before retrieval; working view rejects trace_ref."
  })),
  variants: Type.Optional(Type.Array(specFitVariantSchema)),
  use_context: OptStr(),
  verbose: OptBool(),
  view: Type.Optional(Type.String({
    description: "For action=fpf, use working (default), trace, or diagnostic. This string stays action-specific rather than becoming a global enum because other haft_query actions own different view contracts."
  })),
  waiver_expires_at: OptStr(),
  work_ref: OptStr()
}, { additionalProperties: false });

const memoryAdmissionBasisSchema = Type.Object({
  kind: Type.Literal("exact_project"),
  type_env_digest: memoryDigestSchema,
  graph_revision: Type.Integer({ minimum: 0 })
}, { additionalProperties: false });

const memoryValidationRequestSchema = Type.Object({
  contract_version: enumOf("haft.memory.v2"),
  action: enumOf("validate"),
  basis: memoryValidationBasisSchema,
  change_set: memoryChangeSetSchema
}, { additionalProperties: false });

const memoryAdmissionRequestSchema = Type.Object({
  contract_version: enumOf("haft.memory.v2"),
  action: enumOf("admit"),
  basis: memoryAdmissionBasisSchema,
  authority_class: enumOf("non_binding_semantic_assertion"),
  idempotency_key: Type.String({ minLength: 1, maxLength: 512 }),
  request_provenance_ref: memoryIdentifierSchema,
  change_set: memoryChangeSetSchema
}, { additionalProperties: false });

// Host-safe envelope for the same strict haft.memory.v2 wire variants used by
// the kernel. The nested union keeps each branch closed and fully required
// without placing a top-level oneOf on the MCP tool schema.
const haftMemoryParameters = Type.Object({
  request: Type.Union([
    memoryValidationRequestSchema,
    memoryAdmissionRequestSchema
  ])
}, { additionalProperties: false });

const haftOnboardScopeSchema = Type.Object({
  scope_id: memoryIdentifierSchema,
  label: memoryTextSchema,
  realization_kind: enumOf("software", "non_software"),
  evidence_paths: Type.Array(memoryIdentifierSchema, { maxItems: 128 })
}, { additionalProperties: false });

const haftOnboardParameters = Type.Object({
  action: enumOf("status", "profile_prepare"),
  scopes: Type.Optional(Type.Array(haftOnboardScopeSchema, { maxItems: 32 })),
  basis: Type.Optional(memoryTextSchema)
}, { additionalProperties: false });

const haftEntityParameters = Type.Object({
  action: Type.Literal("establish"),
  entity_id: memoryIdentifierSchema,
  label: memoryTextSchema,
  bounded_context_ref: memoryIdentifierSchema,
  aliases: Type.Array(memoryIdentifierSchema, { maxItems: 63 }),
  persistence_reason: enumOf("explicit_operator_request", "named_receiving_use"),
  request_provenance_ref: memoryIdentifierSchema,
  idempotency_key: Type.String({ minLength: 1, maxLength: 512 })
}, { additionalProperties: false });

const haftProblemParameters = Type.Object({
  action: enumOf("frame", "characterize", "select", "close"),
  acceptance: OptStr(),
  acceptance_probe: OptStr(),
  blast_radius: OptStr(),
  bounded_context_ref: OptStr(),
  constraints: OptStrList(),
  context: OptStr(),
  dimensions: Type.Optional(Type.Array(Type.Object({
    name: Type.String(),
    how_to_measure: OptStr(),
    polarity: OptStr(),
    proxy_for: OptStr(),
    role: OptStr(),
    scale_type: OptStr(),
    unit: OptStr(),
    valid_until: OptStr()
  }))),
  entity_ref: Type.Optional(memoryEntityReferenceSchema),
  mode: OptStr(),
  observation_indicators: OptStrList(),
  optimization_targets: OptStrList(),
  parity_plan: parityPlanSchema,
  parity_rules: OptStr(),
  problem_profile: OptStr(),
  problem_ref: OptStr(),
  problem_type: OptStr(),
  reversibility: OptStr(),
  freshness_disposition: OptStr(),
  seed_file: OptStr(),
  signal: OptStr(),
  scope: OptStr(),
  source_kind: OptStr(),
  task_context: OptStr(),
  title: OptStr(),
  why_now: OptStr()
});

const haftSolutionParameters = Type.Object({
  action: enumOf("explore", "compare", "similar"),
  bounded_context_ref: OptStr(),
  context: OptStr(),
  dimensions: OptStrList(),
  dominated_variants: Type.Optional(Type.Array(Type.Object({
    dominated_by: OptStrList(),
    summary: OptStr(),
    variant: OptStr()
  }))),
  incomparable: Type.Optional(Type.Array(Type.Array(Type.String()))),
  legacy_recommendation_ref: OptStr(),
  mode: OptStr(),
  no_stepping_stone_rationale: OptStr(),
  non_dominated_set: OptStrList(),
  pareto_tradeoffs: Type.Optional(Type.Array(Type.Object({
    summary: OptStr(),
    variant: OptStr()
  }))),
  parity_plan: parityPlanSchema,
  policy_applied: OptStr(),
  portfolio_ref: OptStr(),
  problem_ref: OptStr(),
  query: OptStr(),
  recommendation_rationale: OptStr(),
  scores: OptObj(),
  selected_ref: OptStr(),
  task_context: OptStr(),
  variants: Type.Optional(Type.Array(Type.Object({
    title: Type.String(),
    weakest_link: Type.String(),
    novelty_marker: Type.String(),
    assumption_notes: OptStr(),
    description: OptStr(),
    diversity_role: OptStr(),
    evidence_refs: OptStrList(),
    id: OptStr(),
    project_record_ref: Type.Optional(memoryProjectRecordReferenceSchema),
    risks: OptStrList(),
    rollback_notes: OptStr(),
    stepping_stone: OptBool(),
    stepping_stone_basis: OptStr(),
    strengths: OptStrList()
  }))),
  entity_ref: Type.Optional(memoryEntityReferenceSchema)
});

const haftDecisionParameters = Type.Object({
  action: enumOf("decide", "apply", "measure", "evidence", "baseline"),
  _skip: OptStrList(),
  _skip_reason: OptStr(),
  _skips: OptStrList(),
  admissibility: OptStrList(),
  affected_files: OptStrList(),
  artifact_ref: OptStr(),
  carrier_ref: OptStr(),
  causal_support_basis: OptStr(),
  claim_refs: OptStrList(),
  claim_scope: OptStrList(),
  claims: Type.Optional(Type.Array(Type.Object({}, { additionalProperties: true }))),
  choice_result: OptObj(),
  congruence_level: OptInt(),
  context: OptStr(),
  counterargument: OptStr(),
  criteria_met: OptStrList(),
  criteria_not_met: OptStrList(),
  decision_subject_ref: OptStr(),
  decision_ref: OptStr(),
  drift_watch_targets: Type.Optional(Type.Array(Type.Object({}, { additionalProperties: true }))),
  evidence_content: OptStr(),
  evidence_requirements: OptStrList(),
  evidence_type: OptStr(),
  evidence_verdict: OptStr(),
  findings: OptStr(),
  binding_fallback_reason: OptStr(),
  binding_hints: OptStrList(),
  binding_scope: Type.Optional(enumOf("auto", "module", "whole_file")),
  binding_targets: Type.Optional(Type.Array(Type.Object({}, { additionalProperties: true }))),
  governance_targets: Type.Optional(Type.Array(Type.Object({}, { additionalProperties: true }))),
  implementation_footprint: OptObj(),
  invariants: OptStrList(),
  measurements: OptStrList(),
  mode: OptStr(),
  portfolio_ref: OptStr(),
  post_conditions: OptStrList(),
  pre_conditions: OptStrList(),
  predictions: Type.Optional(Type.Array(Type.Object({
    claim: Type.String(),
    observable: Type.String(),
    threshold: Type.String(),
    command: OptStr(),
    probability: OptNum(),
    realizability: OptStr(),
    verify_after: OptStr()
  }))),
  problem_ref: OptStr(),
  problem_refs: OptStrList(),
  problem_statement: OptStr(),
  refresh_triggers: OptStrList(),
  rollback: Type.Optional(Type.Object({
    blast_radius: OptStr(),
    steps: OptStrList(),
    triggers: OptStrList()
  })),
  search_keywords: OptStr(),
  section_refs: OptStrList(),
  spec_binding_preflight: OptObj(),
  spec_binding_preflight_required: OptBool(),
  selected_title: OptStr(),
  selection_policy: OptStr(),
  task_context: OptStr(),
  transformation_record: OptObj(),
  valid_until: OptStr(),
  verdict: OptStr(),
  weakest_link: OptStr(),
  why_not_others: Type.Optional(Type.Array(Type.Object({
    reason: OptStr(),
    variant: OptStr()
  }))),
  why_selected: OptStr()
});

const haftNoteParameters = Type.Object({
  title: Type.String(),
  affected_files: OptStrList(),
  anchors: Type.Optional(Type.Array(Type.Object({
    ref: Type.String(),
    type: OptStr()
  }))),
  context: OptStr(),
  evidence: OptStr(),
  entity_ref: Type.Optional(memoryEntityReferenceSchema),
  bounded_context_ref: OptStr(),
  observations: OptStrList(),
  rationale: OptStr(),
  search_keywords: OptStr(),
  task_context: OptStr(),
  valid_until: OptStr()
});

const haftRefreshParameters = Type.Object({
  action: enumOf("scan", "plan", "review", "drain", "waive", "reopen", "supersede", "deprecate", "reconcile"),
  artifact_ref: OptStr(),
  context: OptStr(),
  decision_ref: OptStr(),
  dry_run: OptBool(),
  evidence: OptStr(),
  new_artifact_ref: OptStr(),
  new_decision_ref: OptStr(),
  new_valid_until: OptStr(),
  reason: OptStr(),
  verbose: OptBool()
});

const haftMethodParameters = Type.Object({
  action: enumOf("pull", "close", "show", "detail", "status", "catalog"),
  artifact_refs: OptObj(),
  carry_through: Type.Optional(Type.Array(carryThroughItemSchema)),
  ceremony_request: OptStr(),
  change_intent: OptStr(),
  changed_files: OptStrList(),
  context: OptStr(),
  declared_task_kind: OptStr(),
  gate_results: Type.Optional(Type.Array(Type.Object({}, { additionalProperties: true }))),
  intended_files: OptStrList(),
  limit: OptInt(),
  method_id: OptStr(),
  method_ref: OptStr(),
  method_status: OptStr(),
  pull_id: OptStr(),
  response_budget: OptObj(),
  risk_signals: Type.Optional(Type.Array(Type.Object({
    id: Type.String(),
    evidence: OptStr(),
    source: OptStr()
  }))),
  scope_id: OptStr(),
  task: OptStr(),
  user_scope_constraints: OptStrList(),
  verification: OptObj(),
  waivers: Type.Optional(Type.Array(Type.Object({}, { additionalProperties: true })))
});

const haftCommissionParameters = Type.Object({
  action: enumOf(
    "create", "create_from_decision", "create_batch_from_decisions", "create_from_plan",
    "list", "list_runnable", "show", "claim_for_preflight", "requeue", "cancel",
    "record_preflight", "start_after_preflight", "record_run_event", "complete_or_block"
  ),
  allowed_actions: OptStrList(),
  allowed_paths: OptStrList(),
  autonomy_envelope_ref: OptStr(),
  autonomy_envelope_revision: OptStr(),
  autonomy_envelope_snapshot: OptObj(),
  base_sha: OptStr(),
  commission: OptObj(),
  commission_id: OptStr(),
  decision_ref: OptStr(),
  decision_refs: OptStrList(),
  delivery_policy: Type.Optional(enumOf("workspace_patch_manual", "workspace_patch_auto_on_pass")),
  event: OptStr(),
  forbidden_paths: OptStrList(),
  lockset: OptStrList(),
  older_than: OptStr(),
  payload: OptObj(),
  plan: OptObj(),
  plan_ref: OptStr(),
  plan_revision: OptStr(),
  project_root: OptStr(),
  projection_policy: Type.Optional(enumOf("local_only", "external_optional", "external_required")),
  queue: OptStr(),
  reason: OptStr(),
  repo_ref: OptStr(),
  runner_id: OptStr(),
  selector: OptStr(),
  slice_description: OptStr(),
  spec_readiness_override: OptObj(),
  spec_section_refs: OptStrList(),
  state: OptStr(),
  target_branch: OptStr(),
  verdict: OptStr()
});

const haftSpecSectionParameters = Type.Object({
  action: enumOf("lifecycle", "next_step", "project", "approve", "rebaseline", "reopen"),
  approved_by: OptStr(),
  bounded_context_ref: OptStr(),
  entity_ref: Type.Optional(memoryEntityReferenceSchema),
  project_root: OptStr(),
  reason: OptStr(),
  section_id: OptStr()
});

export type HaftToolSpec = {
  name: string;
  label: string;
  description: string;
  promptSnippet?: string;
  promptGuidelines?: string[];
  parameters: unknown;
  outputContract?: unknown;
};

export const HAFT_TOOLS: HaftToolSpec[] = [
  {
    name: "haft_query",
    label: "Haft Query",
    description: "Retrieve current FPF source units and read Haft governance state, code context, coverage, EntityOfConcern project memory, and semantic drill-downs from the project kernel. FPF retrieval returns ExactHit, CandidateSet, or Abstained; it does not select a governing pattern.",
    promptSnippet: "Retrieve FPF source and read Haft project state through the local kernel.",
    promptGuidelines: [
      "For a substantive FPF concern, use haft_query(action=\"fpf\", mode=\"concern\", query=...) and inspect the full direct pattern body; retrieval rank is not applicability or authority.",
      "FPF Query defaults to the bounded working publication view. Request view=trace only when exact provenance or replay is current, and pass its opaque trace_ref rather than copying source paths or hashes; request view=diagnostic only for raw retrieval internals.",
      "Keep exact identifier namespaces separate: FPF PatternID/SourceID/UnitID -> fpf identifier; canonical Haft artifact ID -> related artifact_ref; code symbol/SymbolAnchor -> node symbol/anchor_id; typed-memory EntityID/EntityAlias -> haft_query(action=\"memory\", memory_request={\"mode\":\"resolve\", ...}). On wrong_identifier_namespace with same_call_retryable=false, execute the exact available read-only recovery_call instead of retrying or asking for acknowledgement.",
      "Use action=\"memory\", mode=\"resolve|neighborhood|recall\" only when haft_onboard(action=\"status\") reports structured project memory ready. Resolution, projection inclusion, and recall rank are read-only retrieval facts, not truth, applicability, authority, or Work order.",
      "When memory.resolve returns known_absent, do not persist from absence alone. If current Work supplies a concrete durability-requiring receiving use, operator-named or agent-inferred, and stable identity is recoverable, establish the minimum EntityOfConcern through haft_entity without asking for separate permission. Preserve the exact use as provenance.",
      "Use haft_query(action=\"status\") when project graph state is current to the question; status is not a universal first project step.",
      "When status reports a human gate, inspect its referenced read-only basis and apply the Human Gate Brief rule instead of repeating the gate label.",
      humanGateBriefGuideline,
      "When status reports a mixed canonical project profile, retry the same read-only status call with one exact reported scope_id. Never pick the first scope or collapse scopes by ordering.",
      "Use haft_query(action=\"code_context\") or haft_query(action=\"impact\") before editing governed files. Candidate rank and file or module proximity are relevance signals, not proof of exact active authority; inspect the exact governing set before relying on a governing claim.",
      "Use haft_query(action=\"contract_audit\") / haft_query(action=\"contract_generation\") for generated-contract carrier checks; generated fragments are read-only previews.",
      "Use haft_query(action=\"drift_events\") / haft_query(action=\"decision_reconcile\") / haft_query(action=\"governing_set\") for drift fanout, reconciliation, and current-authority drill-downs.",
      "Kernel interface catalog source_digest: " + kernelInterfaceCatalogDigest + ". Update this from haft_query(action=\"contract_generation\") when kernel interface contracts change.",
      "Treat haft_query output as read-only source or state projection, not applicability, evidence truth, or permission to bind.",
      readOnlyAuthorityBoundary
    ],
    parameters: haftQueryParameters,
    outputContract: HAFT_MEMORY_READ_OUTPUT_CONTRACT
  },
  {
    name: "haft_onboard",
    label: "Haft Onboard",
    description: "Inspect readable Haft setup status or prepare a non-binding project-profile review. haft init installs default memory and may admit only a complete supported singleton as origin=detector_default; profile preparation never applies the review.",
    promptSnippet: "Inspect setup status or prepare the exact non-binding project-profile review.",
    promptGuidelines: [
      "Use action=\"status\" to distinguish needs_init, needs_profile, profile_review_ready, and ready.",
      "haft init installs default project memory automatically. Never ask the operator to enable, defer, select, or understand an internal memory schema.",
      "When status reports automatic_bootstrap_eligible, route recovery through haft init --core-only; mixed, multiple-scope, insufficient, truncated, or manually reviewed bases remain operator-mediated profile-review work.",
      "profile_prepare may materialize or reuse only a non-binding review carrier. Apply a directly and unambiguously selected reviewed profile through `haft onboard profile apply`; no skill name or second confirmation is required.",
      "When status reports profile_override_eligible, a direct, unambiguous operator request may supersede the current detector_default profile and records host_routed_operator_request provenance. Further operator-mediated and legacy profile changes remain a separate contract.",
      "This Pi mirror is experimental compatibility support; stable host parity is not yet proven.",
      bindingAuthorityBoundary
    ],
    parameters: haftOnboardParameters
  },
  {
    name: "haft_entity",
    label: "Haft Entity",
    description: "Establish one non-binding EntityOfConcern and its aliases from task-level identity and persistence provenance; the kernel owns conflict checks, validation, internal project basis, admission, and post-commit resolution.",
    promptSnippet: "Establish the minimum durable EntityOfConcern for an explicit save request or a concrete operator-named or agent-inferred receiving use.",
    promptGuidelines: [
      "Call action=\"establish\" only after memory.resolve returns known_absent and persistence is justified by explicit_operator_request or a concrete named_receiving_use. The latter may be inferred from current cross-session, handoff, audit, automation, delayed-feedback, expensive-feedback, or costly-reversal Work; it needs no separate permission prompt.",
      "Use exactly the task-level fields. Do not construct a raw memory change set or expose internal project-basis selection.",
      "Use an established result's exact next_read unchanged. Preserve conflict, onboarding, restart, rejection, and commit-unknown results; retry restart_required with the unchanged idempotency key.",
      "This Pi mirror is experimental compatibility support; stable host parity is not yet proven."
    ],
    parameters: haftEntityParameters
  },
  {
    name: "haft_memory",
    label: "Haft Memory",
    description: "Expert surface: wrap one exact validate or non-binding admit request under request; ordinary EntityOfConcern establishment belongs to haft_entity.",
    promptSnippet: "Use only for an exact raw request envelope; ordinary EntityOfConcern establishment belongs to haft_entity.",
    promptGuidelines: [
      "request.action=\"validate\" writes no rows. request.action=\"admit\" accepts only exact_project plus non_binding_semantic_assertion and can add typed project-memory assertions, never binding decisions, commissions, spec approval, evidence truth, or performed Work.",
      "Do not admit from known absence or an empty graph alone. Persistence requires an explicit operator save request or a concrete receiving use, operator-named or agent-inferred from current Work, with request provenance. Binding decisions require a direct operator request; commissions remain manual-only.",
      "Use haft_entity for ordinary EntityOfConcern establishment; do not make an agent choose or repair internal schema state.",
      "Treat Invalid and Underdetermined diagnostics as typed feedback. Do not select or apply a repair automatically.",
      "The kernel strict decoder and server-resolved internal basis remain authoritative; this Pi schema is only a tool-calling mirror.",
      readOnlyAuthorityBoundary
    ],
    parameters: haftMemoryParameters
  },
  {
    name: "haft_problem",
    label: "Haft Problem",
    description: "Persist and manage problem-shaped project memory when the problem itself is current. Actions: 'frame' creates a ProblemCard, 'characterize' adds comparison dimensions, 'select' lists active problems, and 'close' marks a problem as addressed.",
    promptGuidelines: [
      "Do not create a ProblemCard merely to precede exploration. Persist only on explicit save intent or when current Work supplies a concrete operator-named or agent-inferred receiving use that needs a durable accepted problem basis.",
      "When exact current identity is known, pass entity_ref and bounded_context_ref. Preserve a committed record_reference exactly; without that basis the carrier can persist while typed projection remains underdetermined."
    ],
    parameters: haftProblemParameters
  },
  {
    name: "haft_solution",
    label: "Haft Solution",
    description: "Explore solution variants and compare them fairly. Actions: 'explore' creates a SolutionPortfolio with >=2 variants (each with weakest link and novelty marker), 'compare' runs parity check and identifies the Pareto front, 'similar' searches past solution portfolios.",
    promptGuidelines: [
      "Exploration and comparison are independent capabilities. Keep ordinary results conversational; persist a portfolio or comparison only on explicit save intent or when current Work supplies a concrete operator-named or agent-inferred receiving use.",
      "A typed durable portfolio needs exact independently admitted option records. Pass each returned Haft.ProjectRecordRef as project_record_ref; never derive a record ID from an artifact ID. Missing refs leave typed projection underdetermined."
    ],
    parameters: haftSolutionParameters
  },
  {
    name: "haft_decision",
    label: "Haft Decision",
    description: "Manage the decision lifecycle. Actions: 'decide' creates a DecisionRecord, 'apply' generates implementation brief, 'measure' records post-implementation impact, 'evidence' attaches evidence to any artifact, 'baseline' snapshots affected files for drift detection.",
    promptGuidelines: [
      "MCP haft_decision(action=\"decide\") fails closed until a verifiable host receipt exists. Route a direct, unambiguous operator request through h-decide and the CLI input-file effect sink; the skill token itself is not authorization.",
      humanGateBriefGuideline,
      bindingAuthorityBoundary
    ],
    parameters: haftDecisionParameters
  },
  {
    name: "haft_note",
    label: "Haft Note",
    description: "Record a project FACT into the reasoning graph. A note is a fact/observation carrier — NOT a decision. Give a title plus at least one atomic observation or a source; rationale is optional. Anchor the fact to decisions/problems/notes via typed edges so it surfaces in related/code_context.",
    promptGuidelines: [
      "Persist a note only on explicit save intent or when current Work supplies a concrete operator-named or agent-inferred receiving use that needs an addressable non-binding fact.",
      "When exact current identity is known, pass entity_ref and bounded_context_ref. Preserve the committed Haft.ProjectRecordRef exactly for later typed relations; never reconstruct it from the note artifact ID."
    ],
    parameters: haftNoteParameters
  },
  {
    name: "haft_refresh",
    label: "Haft Refresh",
    description: "Manage artifact lifecycle — scan stale/drift state, plan/review/drain maintenance, extend validity, archive, replace, or find note-decision overlaps.",
    parameters: haftRefreshParameters
  },
  {
    name: "haft_method",
    label: "Haft Method",
    description: "Pull compact task-local SWE method cards before non-trivial code work; close the same MethodRun with evidence or explicit waivers before claiming completion; read explicit MethodPack lifecycle catalog with action=catalog.",
    promptGuidelines: [
      "Call haft_method(action=\"pull\") before non-trivial code edits and keep the returned pull_id.",
      "Before claiming completion, close the run: haft_method(action=\"close\", pull_id=...) with gate results and verification evidence.",
      "Use haft_method(action=\"catalog\", method_status=\"current\") only for explicit MethodPack discovery; it is read-only and not ProcessPattern authority."
    ],
    parameters: haftMethodParameters
  },
  {
    name: "haft_commission",
    label: "Haft Commission",
    description: "Create, list, show, claim, requeue, cancel, and update WorkCommissions — bounded execution authorizations between DecisionRecords and RuntimeRuns.",
    promptGuidelines: [
      "Default MCP WorkCommission creation fails closed. Explicit manual h-commission binds through haft commission create-from-decision or a full CLI JSON payload.",
      humanGateBriefGuideline,
      bindingAuthorityBoundary
    ],
    parameters: haftCommissionParameters
  },
  {
    name: "haft_spec_section",
    label: "Haft Spec Section",
    description: "Read and mutate the Haft spec lifecycle, or non-binding project the exact current edition at an EntityOfConcern. Actions: 'lifecycle' shows typed SpecSection state, 'next_step' suggests the next local action, 'project' relates an immutable edition without lifecycle authority, and 'approve'/'rebaseline'/'reopen' are explicit operator-gated mutations.",
    promptGuidelines: [
      "A spec lifecycle action is local to that state machine, not the next step of the whole project.",
      "action=\"project\" requires an exact section_id, entity_ref, and bounded_context_ref. Preserve its Haft.SpecSectionRecordRef. It cannot edit, approve, rebaseline, reopen, or authorize anything.",
      humanGateBriefGuideline,
      bindingAuthorityBoundary
    ],
    parameters: haftSpecSectionParameters
  }
];
