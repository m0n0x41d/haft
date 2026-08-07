import assert from "node:assert/strict";
import { mkdirSync, mkdtempSync, readFileSync, readdirSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

import { parseResponse, toolNames, toolText } from "../extensions/haft/bridge.ts";
import { clipText, withGovernorHeader } from "../extensions/haft/governor.ts";
import { findHaftProjectRoot } from "../extensions/haft/project.ts";
import {
  HAFT_MEMORY_READ_OUTPUT_CONTRACT,
  HAFT_MEMORY_READ_OUTPUT_CONTRACT_JSON,
  HAFT_TOOLS
} from "../extensions/haft/tools.ts";

test("clipText passes short text through and clips long text with a marker", () => {
  assert.equal(clipText("short", 100), "short");

  const clipped = clipText("x".repeat(300), 100);
  assert.ok(clipped.length < 300);
  assert.match(clipped, /\[truncated by @haft\/pi prompt governor\]$/);
});

test("withGovernorHeader keeps kernel headers and wraps raw text", () => {
  const governor = "## Haft Project State (governor)\nbody";
  assert.equal(withGovernorHeader(governor), governor);

  assert.equal(withGovernorHeader("raw status"), "## Haft Project State\nraw status");
});

test("parseResponse tolerates non-JSON lines", () => {
  assert.equal(parseResponse("not json"), undefined);
  assert.deepEqual(parseResponse('{"jsonrpc":"2.0","id":1,"result":{}}'), {
    jsonrpc: "2.0",
    id: 1,
    result: {}
  });
});

test("toolNames extracts names defensively", () => {
  assert.deepEqual(toolNames({ tools: [{ name: "a" }, {}, { name: "b" }] }), ["a", "b"]);
  assert.deepEqual(toolNames({}), []);
});

test("toolText joins text content and throws on isError", () => {
  const ok = { content: [{ type: "text", text: "one" }, { type: "image" }, { type: "text", text: "two" }] };
  assert.equal(toolText(ok), "one\ntwo");

  assert.throws(() => toolText({ content: [{ type: "text", text: "bad" }], isError: true }), /bad/);
  assert.throws(() => toolText({ isError: true }), /returned an error/);
});

test("findHaftProjectRoot walks up to the nearest .haft", () => {
  const root = mkdtempSync(join(tmpdir(), "haft-pi-root-"));
  const project = join(root, "repo");
  const nested = join(project, "a", "b");
  mkdirSync(join(project, ".haft"), { recursive: true });
  mkdirSync(nested, { recursive: true });

  assert.equal(findHaftProjectRoot(nested), project);
  assert.equal(findHaftProjectRoot(root), undefined);
});

test("Pi tool schemas mirror maintenance, prediction, and problem dimension fields", () => {
  const refresh = toolSpec("haft_refresh");
  const decision = toolSpec("haft_decision");
  const problem = toolSpec("haft_problem");

  assert.match(JSON.stringify(refresh.parameters), /"plan"/);
  assert.match(JSON.stringify(decision.parameters), /"command"/);
  assert.match(JSON.stringify(problem.parameters), /"proxy_for"/);
});

test("Pi haft_query schema mirrors current read-only drill-down actions", () => {
  const query = toolSpec("haft_query");
  const schema = JSON.stringify(query.parameters);
  const properties = (query.parameters as {
    properties?: Record<string, { description?: string }>;
  }).properties ?? {};

  [
    "carrier_manifest",
    "carrier_check",
    "contract_audit",
    "contract_generation",
    "spec_review",
    "spec_use",
    "change_case",
    "correspondence_graph",
    "drift_route",
    "drift_events",
    "decision_reconcile",
    "governing_set",
    "blocked_use",
    "value_space",
    "evidence_path"
  ].forEach((action) => assert.match(schema, new RegExp(`"${action}"`)));

  [
    "concern",
    "lookup",
    "inspect",
    "identifier",
    "entity_of_concern",
    "known_context",
    "intended_use",
    "roles",
    "max_candidates_per_role",
    "max_total_candidates",
    "max_excerpt_characters",
    "scope_id",
    "view",
    "trace_ref"
  ].forEach((field) => assert.match(schema, new RegExp(`"${field}"`)));

  ["identifier", "artifact_ref", "symbol"].forEach((field) => {
    assert.match(properties[field]?.description ?? "", /wrong_identifier_namespace/);
  });
  assert.match(properties.identifier?.description ?? "", /action=fpf/);
  assert.match(properties.artifact_ref?.description ?? "", /action=related/);
  assert.match(properties.symbol?.description ?? "", /action=node/);
  assert.match(properties.scope_id?.description ?? "", /action=status or coverage/);
  assert.match(properties.scope_id?.description ?? "", /mixed-profile response/);
  assert.match(properties.scope_id?.description ?? "", /retry the same read-only call/);
  assert.match(properties.scope_id?.description ?? "", /never select by display order/);

  const view = properties.view as {
    type?: string;
    anyOf?: unknown;
    enum?: unknown;
    const?: unknown;
    description?: string;
  };
  assert.equal(view.type, "string");
  assert.equal(view.anyOf, undefined);
  assert.equal(view.enum, undefined);
  assert.equal(view.const, undefined);
  assert.match(view.description ?? "", /action=fpf/);
  assert.match(view.description ?? "", /working \(default\), trace, or diagnostic/);
  assert.match(view.description ?? "", /rather than becoming a global enum/);

  const traceRef = properties.trace_ref as { type?: string; description?: string };
  assert.equal(traceRef.type, "string");
  assert.match(traceRef.description ?? "", /action=fpf/);
  assert.match(traceRef.description ?? "", /view=trace or view=diagnostic/);
  assert.match(traceRef.description ?? "", /opaque replay reference/);
  assert.match(traceRef.description ?? "", /replay_mismatch before retrieval/);
  assert.match(traceRef.description ?? "", /working view rejects trace_ref/);

  assert.doesNotMatch(schema, /"pattern_use"|"pattern_recall"/);

  [
    "section_id",
    "operational_gate",
    "source_refs",
    "requires_current_formality",
    "bearer_ref",
    "method_ref",
    "lane"
  ].forEach((field) => assert.match(schema, new RegExp(`"${field}"`)));
});

test("Pi haft_query keeps memory modes inside a closed nested envelope", () => {
  const query = toolSpec("haft_query");
  const parameters = query.parameters as {
    additionalProperties?: boolean;
    properties: {
      mode: { anyOf?: Array<{ const?: string }> };
      memory_request: {
        anyOf?: Array<{
          additionalProperties?: boolean;
          properties?: Record<string, {
            const?: string;
            items?: { type?: string };
          }>;
          required?: string[];
        }>;
      };
    };
  };
  assert.equal(parameters.additionalProperties, false);
  const mode = parameters.properties.mode;
  const values = (mode.anyOf ?? []).map((entry) => entry.const).filter(Boolean).sort();

  assert.deepEqual(values, [
    "concern",
    "deep",
    "inspect",
    "lookup",
    "standard",
    "tactical"
  ]);

  const memory = parameters.properties.memory_request;
  const branches = memory.anyOf ?? [];
  assert.equal(branches.length, 3);
  const byMode = new Map(
    branches.map((branch) => [
      branch.properties?.mode?.const,
      branch
    ])
  );
  assert.deepEqual([...byMode.keys()].sort(), [
    "neighborhood",
    "recall",
    "resolve"
  ]);
  assert.deepEqual(byMode.get("resolve")?.required, [
    "mode",
    "contract_version",
    "basis",
    "query",
    "max_candidates"
  ]);
  assert.deepEqual(byMode.get("neighborhood")?.required, [
    "mode",
    "contract_version",
    "basis",
    "entity_ref",
    "bounded_context_ref",
    "view",
    "read_budget"
  ]);
  assert.deepEqual(byMode.get("recall")?.required, [
    "mode",
    "contract_version",
    "basis",
    "entity_ref",
    "bounded_context_ref",
    "view",
    "read_budget",
    "query",
    "candidate_budget"
  ]);
  branches.forEach((branch) => assert.equal(branch.additionalProperties, false));
  const neighborhoodView = byMode.get("neighborhood")
    ?.properties?.view as {
      properties?: {
        requested_facets?: {
          type?: string;
          items?: {
            anyOf?: Array<{ type?: string; const?: string }>;
          };
        };
      };
    };
  const requestedFacets = neighborhoodView.properties?.requested_facets;
  assert.equal(requestedFacets?.type, "array");
  assert.ok((requestedFacets?.items?.anyOf?.length ?? 0) > 0);
  requestedFacets?.items?.anyOf?.forEach((item) => {
    assert.equal(item.type, "string");
    assert.equal(typeof item.const, "string");
  });

  ["contract_version", "basis", "candidate_budget", "entity_ref", "read_budget"]
    .forEach((field) => assert.equal(field in parameters.properties, false));
});

test("Pi haft_query exposes the exact closed memory-read output contract", () => {
  const query = toolSpec("haft_query");
  const parsed = JSON.parse(HAFT_MEMORY_READ_OUTPUT_CONTRACT_JSON) as {
    schema: string;
    projection_profile_refs: string[];
    result_families: Array<{
      action: string;
      variants: Array<{ kind: string }>;
    }>;
    named_shapes: Array<{
      name: string;
      required_fields?: string[];
      allowed_values?: string[];
      variants?: Array<{ kind: string; required_fields: string[] }>;
    }>;
  };

  assert.deepEqual(query.outputContract, HAFT_MEMORY_READ_OUTPUT_CONTRACT);
  assert.equal(parsed.schema, "haft.memory-read-output-contract/v1");
  assert.deepEqual(parsed.result_families.map((family) => family.action), [
    "resolve",
    "neighborhood",
    "recall"
  ]);
  assert.deepEqual(
    parsed.result_families.map((family) => (
      [family.action, family.variants.map((variant) => variant.kind)]
    )),
    [
      ["resolve", ["exact_entity", "known_absent", "entity_candidates", "resolution_unsettled", "retry_required"]],
      ["neighborhood", ["exact_neighborhood", "retry_required", "abstained"]],
      ["recall", ["scoped_memory_candidate_set", "retry_required", "abstained"]]
    ]
  );

  const namedShapes = new Map(
    parsed.named_shapes.map((shape) => [shape.name, shape])
  );
  [
    "ProjectionBasis",
    "FacetBasisIssue",
    "WholeReadRetryCause",
    "ReadAbstentionBasis",
    "InterpretationContract",
    "RelationalRecordsInterpretation",
    "RelationalRecordItemPosture",
    "FacetCoverage",
    "AppliedReadBudget",
    "RetryRequired"
  ].forEach((name) => assert.ok(namedShapes.has(name), `missing ${name}`));
  assert.deepEqual(parsed.projection_profile_refs, [
    "agent_orientation.v1",
    "agent_orientation.v2",
    "decision_rationale.v1",
    "evidence_currentness.v1",
    "implementation_trace.v1",
    "spec_impact.v1"
  ]);
  assert.deepEqual(namedShapes.get("InterpretationContract")?.required_fields, [
    "structure",
    "identity",
    "relational_records",
    "ranking",
    "truth",
    "applicability",
    "authority",
    "work_order",
    "completeness",
    "hydrate_before_reliance"
  ]);
  assert.deepEqual(namedShapes.get("RelationalRecordsInterpretation")?.allowed_values, [
    "assertions_exact_at_snapshot",
    "occurrences_exact_at_snapshot",
    "legacy_unqualified_assertions",
    "candidate_assertions",
    "heterogeneous_relational_records",
    "unavailable"
  ]);
  assert.deepEqual(namedShapes.get("RelationalRecordItemPosture")?.allowed_values, [
    "assertion_exact",
    "occurrence_exact",
    "legacy_unqualified_assertion",
    "candidate_assertion"
  ]);
});

test("Pi haft_memory mirrors the kernel-owned validate and admit meta-schema", () => {
  const memory = toolSpec("haft_memory");
  const parameters = memory.parameters as any;

  assert.equal(parameters.additionalProperties, false);
  assert.deepEqual(parameters.required, ["request"]);
  assert.deepEqual(Object.keys(parameters.properties), ["request"]);

  const requestVariants = parameters.properties.request.anyOf ?? [];
  assert.equal(requestVariants.length, 2);
  requestVariants.forEach((variant: any) => assert.equal(variant.additionalProperties, false));
  const validate = requestVariants.find(
    (variant: any) => enumValues(variant.properties.action).includes("validate")
  );
  const admit = requestVariants.find(
    (variant: any) => enumValues(variant.properties.action).includes("admit")
  );
  assert.ok(validate);
  assert.ok(admit);
  assert.deepEqual(validate.required.sort(), [
    "action",
    "basis",
    "change_set",
    "contract_version"
  ]);
  assert.deepEqual(admit.required.sort(), [
    "action",
    "authority_class",
    "basis",
    "change_set",
    "contract_version",
    "idempotency_key",
    "request_provenance_ref"
  ]);
  assert.deepEqual(enumValues(validate.properties.contract_version), ["haft.memory.v2"]);
  assert.deepEqual(enumValues(admit.properties.contract_version), ["haft.memory.v2"]);

  const basisVariants = validate.properties.basis.anyOf ?? [];
  assert.equal(basisVariants.length, 3);
  basisVariants.forEach((variant: any) => assert.equal(variant.additionalProperties, false));
  assert.deepEqual(basisVariants.map((variant: any) => variant.properties.kind.const).sort(), [
    "bundled_candidate_open_world",
    "exact_project",
    "project_current"
  ]);
  const exactProject = basisVariants.find(
    (variant: any) => variant.properties.kind.const === "exact_project"
  );
  assert.deepEqual(exactProject?.required, ["kind", "type_env_digest", "graph_revision"]);
  assert.equal(exactProject?.properties.type_env_digest?.pattern, "^sha256:[0-9a-f]{64}$");
  assert.equal(exactProject?.properties.graph_revision?.minimum, 0);

  assert.deepEqual(admit.properties.basis.required, [
    "kind",
    "type_env_digest",
    "graph_revision"
  ]);
  assert.equal(admit.properties.basis.properties.kind.const, "exact_project");
  assert.equal(admit.properties.basis.properties.graph_revision.minimum, 0);

  const changeSet = validate.properties.change_set;
  assert.equal(changeSet.additionalProperties, false);
  assert.deepEqual(changeSet.required, ["changes"]);
  const changes = changeSet.properties.changes;
  assert.equal(changes.minItems, 1);
  assert.equal(changes.maxItems, 64);
  const changeVariants = changes.items.anyOf ?? [];
  assert.deepEqual(changeVariants.map((variant: any) => variant.properties.kind.const), [
    "declare_entity",
    "identity_change",
    "assert_relation",
    "retract_assertion"
  ]);
  changeVariants.forEach((variant: any) => assert.equal(variant.additionalProperties, false));

  const identity = changeVariants.find(
    (variant: any) => variant.properties.kind.const === "identity_change"
  );
  assert.deepEqual(
    (identity?.properties.change?.anyOf ?? []).map(
      (variant: any) => variant.properties.kind.const
    ),
    ["admit_alias", "supersede_alias"]
  );

  const assertion = changeVariants.find(
    (variant: any) => variant.properties.kind.const === "assert_relation"
  );
  assert.deepEqual(assertion?.required, [
    "kind",
    "assertion_id",
    "signature_id",
    "context_slice",
    "modality",
    "bindings",
    "provenance"
  ]);
  assert.deepEqual(
    (assertion?.properties.modality?.anyOf ?? []).map(
      (variant: any) => variant.properties.kind.const
    ),
    ["affirms_obtaining", "denies_obtaining", "obtaining_unknown"]
  );
  assert.doesNotMatch(JSON.stringify(parameters), /instantiate_relation/);

  assert.deepEqual(enumValues(admit.properties.authority_class), [
    "non_binding_semantic_assertion"
  ]);
  assert.equal(admit.properties.idempotency_key.minLength, 1);
  assert.equal(admit.properties.idempotency_key.maxLength, 512);
  assert.equal(admit.properties.request_provenance_ref.minLength, 1);
  assert.equal(admit.properties.request_provenance_ref.maxLength, 4096);
});

test("Pi haft_onboard mirrors the task-level setup contract", () => {
  const onboard = toolSpec("haft_onboard");
  const parameters = onboard.parameters as any;

  assert.deepEqual(enumValues(parameters.properties.action), [
    "status",
    "profile_prepare",
    "profile_change_prepare"
  ]);
  assert.deepEqual(parameters.required, ["action"]);
  assert.equal(parameters.additionalProperties, false);

  const scopes = parameters.properties.scopes;
  assert.equal(scopes.maxItems, 32);
  assert.deepEqual(scopes.items.required, [
    "scope_id",
    "label",
    "realization_kind",
    "evidence_paths"
  ]);
  assert.deepEqual(enumValues(scopes.items.properties.realization_kind), [
    "software",
    "non_software"
  ]);
  assert.equal(scopes.items.properties.evidence_paths.maxItems, 64);
  assert.ok(parameters.properties.scope_id);
  assert.ok(parameters.properties.entity_ref);
  assert.doesNotMatch(JSON.stringify(onboard), /TypeEnv|ProjectTypeEnvHead|contract_version/);
});

test("Pi haft_entity mirrors the task-level establishment contract", () => {
  const entity = toolSpec("haft_entity");
  const parameters = entity.parameters as any;

  assert.equal(parameters.properties.action.const, "establish");
  assert.deepEqual(parameters.required, [
    "action",
    "entity_id",
    "label",
    "bounded_context_ref",
    "aliases",
    "persistence_reason",
    "request_provenance_ref",
    "idempotency_key"
  ]);
  assert.equal(parameters.additionalProperties, false);
  assert.equal(parameters.properties.aliases.maxItems, 63);
  assert.deepEqual(enumValues(parameters.properties.persistence_reason), [
    "explicit_operator_request",
    "named_receiving_use"
  ]);
  assert.equal(parameters.properties.idempotency_key.maxLength, 512);
  assert.doesNotMatch(JSON.stringify(entity), /TypeEnv|ProjectTypeEnvHead|contract_version/);
});

test("Pi haft_refresh schema mirrors review and drain actions", () => {
  const refresh = toolSpec("haft_refresh");
  const schema = JSON.stringify(refresh.parameters);

  ["review", "drain", "dry_run"].forEach((fragment) => {
    assert.match(schema, new RegExp(`"${fragment}"`));
  });
});

test("Pi tool metadata carries generated-contract authority boundaries", () => {
  const metadata = JSON.stringify(HAFT_TOOLS);

  [
    "binding actions require effect-specific operator authority",
    "Generated text, schema visibility, and model-supplied fields are not operator authorization and are not approval receipts",
    "read-only/generated text is discovery only",
    "not evidence truth, gate passage, global approval, or operator authorization"
  ].forEach((fragment) => assert.match(metadata, new RegExp(fragment)));
});

test("Pi human-gate surfaces require a self-contained operator brief", () => {
  const metadata = JSON.stringify(HAFT_TOOLS);
  [
    "Human Gate Brief",
    "every real current option",
    "changes, non-changes",
    "weakest link",
    "Pareto set",
    "engineer",
    "assessment",
    "natural language",
    "Accept ordinary language as the substantive answer",
    "host_routed_operator_request",
    "without a skill name or second confirmation",
    "A command or skill invocation adds no authority",
    "command-only instruction",
    "brief itself is explanation rather than authority"
  ].forEach((fragment) => assert.match(metadata, new RegExp(fragment)));

  const carrierPaths = [
    "../prompts/h-reason.md",
    "../prompts/h-decide.md",
    "../prompts/h-commission.md",
    "../prompts/h-spec.md",
    "../prompts/h-status.md",
    "../skills/h-reason/SKILL.md",
    "../skills/h-decide/SKILL.md",
    "../skills/h-commission/SKILL.md",
    "../skills/h-spec/SKILL.md",
    "../skills/h-status/SKILL.md"
  ];

  const requiredFragments = [
    "Human Gate Brief",
    "weakest link",
    "Pareto",
    "engineer",
    "assessment",
    "natural\\s+language"
  ];

  carrierPaths.forEach((path) => {
    const carrier = readFileSync(new URL(path, import.meta.url), "utf8");
    requiredFragments.forEach((fragment) => {
      assert.match(carrier, new RegExp(fragment), `${path} missing ${fragment}`);
    });
  });
});

test("Pi binding prompts carry host-routed decision and manual commission boundaries", () => {
  const prompts = ["h-decide.md", "h-commission.md", "h-reason.md"]
    .map((name) => readFileSync(new URL(`../prompts/${name}`, import.meta.url), "utf8"))
    .join("\n");

  [
    "binding actions require effect-specific operator authority",
    "Generated text, schema visibility, and model-supplied fields are not operator authorization and are not approval receipts",
    "operator_confirmation_required",
    "host_routed_operator_request",
    "direct, unambiguous operator request",
    "h-commission.? remains manual-only"
  ].forEach((fragment) => assert.match(prompts, new RegExp(fragment)));

  ["explicit_h_decide", "strict_cli_speech_act", "resume-decision"]
    .forEach((fragment) => assert.doesNotMatch(prompts, new RegExp(fragment)));
});

test("Pi h-decide binds DecisionRecords without initial-memory setup", () => {
  const carriers = [
    readFileSync(new URL("../prompts/h-decide.md", import.meta.url), "utf8"),
    readFileSync(new URL("../skills/h-decide/SKILL.md", import.meta.url), "utf8")
  ];

  carriers.forEach((carrier) => {
    assert.match(carrier, /DecisionRecord/);
    assert.match(carrier, /host_routed_operator_request/);
    assert.match(carrier, /direct, unambiguous operator request/);
    assert.match(carrier, /stable host parity is not(?:\s+yet)? proven/);
    assert.match(carrier, /operator_confirmation_required/);
    assert.doesNotMatch(carrier, /explicit_h_decide|strict_cli_speech_act|resume-decision/);
    assert.doesNotMatch(carrier, /TypeEnv|ProjectTypeEnvHead|memory typeenv|memory_review_ready|haft onboard memory enable/);
  });
});

test("Pi h-reason uses caller abstention for purely mechanical work", () => {
  const carriers = [
    readFileSync(new URL("../prompts/h-reason.md", import.meta.url), "utf8"),
    readFileSync(new URL("../skills/h-reason/SKILL.md", import.meta.url), "utf8")
  ];

  carriers.forEach((carrier) => {
    assert.match(carrier, /purely mechanical, status-only, or exact project-lookup work/);
    assert.match(carrier, /caller abstention is the (correct )?result: skip FPF Query/);
    assert.match(carrier, /Do not fabricate\s+`QueryResult\(kind="abstained"\)`; no query ran/);
  });
});

test("Pi h-reason carriers preserve exact identifier namespaces", () => {
  const carriers = [
    readFileSync(new URL("../prompts/h-reason.md", import.meta.url), "utf8"),
    readFileSync(new URL("../skills/h-reason/SKILL.md", import.meta.url), "utf8")
  ];

  carriers.forEach((carrier) => {
    assert.match(carrier, /wrong_identifier_namespace/);
    assert.match(carrier, /same_call_retryable=false/);
    assert.match(carrier, /artifact_ref/);
    assert.match(carrier, /memory_request/);
    assert.match(carrier, /"mode":"resolve"/);
    assert.match(carrier, /haft_onboard\(action="status"\)/);
    assert.match(carrier, /haft_entity/);
    assert.match(carrier, /known_absent/);
    assert.match(carrier, /operator-named or agent-inferred/);
    assert.match(carrier, /establish the minimum EntityOfConcern without asking for separate permission/);
    assert.match(carrier, /recovery_call/);
    assert.doesNotMatch(carrier, /memory typeenv/);
  });
});

test("Pi h-onboard carriers make default project memory automatic", () => {
  const carriers = [
    readFileSync(new URL("../prompts/h-onboard.md", import.meta.url), "utf8"),
    readFileSync(new URL("../skills/h-onboard/SKILL.md", import.meta.url), "utf8")
  ];

  carriers.forEach((carrier) => {
    assert.match(carrier, /haft init.*installs default project memory/s);
    assert.match(carrier, /Never\s+ask the operator to enable, defer, select, or understand a memory schema/);
    assert.match(carrier, /direct, unambiguous\s+operator selection/s);
    assert.match(carrier, /Do not require a skill name/);
    assert.match(carrier, /host_routed_operator_request/);
    assert.match(carrier, /detector_default/);
    assert.doesNotMatch(carrier, /explicit h-onboard/i);
    assert.doesNotMatch(carrier, /TypeEnv|ProjectTypeEnvHead|memory typeenv|memory_prepare|memory_review_ready|memory_deferred|haft onboard memory enable|haft onboard memory defer/);
  });
});

test("Pi h-reason keeps routine working identity separate from trace provenance", () => {
  const carriers = [
    readFileSync(new URL("../prompts/h-reason.md", import.meta.url), "utf8"),
    readFileSync(new URL("../skills/h-reason/SKILL.md", import.meta.url), "utf8")
  ];

  carriers.forEach((carrier) => {
    assert.match(carrier, /ordinary working use, identify the selected direct pattern by\s*`PatternID`,\s+title, and stable source reference/i);
    assert.match(carrier, /Do not routinely reproduce\s+source spans,\s+repository-local paths, line ranges, hashes, revisions, or\s+other provenance/);
    assert.match(carrier, /Request trace or audit provenance only when the current\s+use requires it/);
    assert.doesNotMatch(carrier, /selected direct pattern and source span/);
  });
});

test("Pi h-status carriers preserve exact mixed-scope retry semantics", () => {
  const carriers = [
    readFileSync(new URL("../prompts/h-status.md", import.meta.url), "utf8"),
    readFileSync(new URL("../skills/h-status/SKILL.md", import.meta.url), "utf8")
  ];

  carriers.forEach((carrier) => {
    assert.match(carrier, /retry the same read-only/);
    assert.match(carrier, /"scope_id": "<exact emitted ScopeID>"/);
    assert.match(carrier, /Never select the first value/);
    assert.match(carrier, /unrelated already-authorized Work continues/);
    assert.match(carrier, /agent_orientation\.v2/);
    assert.doesNotMatch(carrier, /agent_orientation\.v1/);
  });
});

test("Pi tool guidance preserves source-first independent capability semantics", () => {
  const source = JSON.stringify(HAFT_TOOLS);

  [
    "retrieval rank is not applicability or authority",
    "status is not a universal first project step",
    "Do not create a ProblemCard merely to precede exploration",
    "Exploration and comparison are independent capabilities",
    "MCP haft_decision",
    "direct, unambiguous operator request",
    "skill token itself is not authorization",
    "Default MCP WorkCommission creation fails closed"
  ].forEach((fragment) => assert.match(source, new RegExp(fragment)));

  [
    "Frame the problem BEFORE exploring solutions",
    "Persist 3\\+ alternatives",
    "before open-ended Haft, FPF"
  ].forEach((fragment) => assert.doesNotMatch(source, new RegExp(fragment)));
});

test("Pi h-spec carriers expose profile-independent draft validation", () => {
  const carriers = ["../prompts/h-spec.md", "../skills/h-spec/SKILL.md"]
    .map((path) => readFileSync(new URL(path, import.meta.url), "utf8"));

  carriers.forEach((carrier) => {
    assert.match(carrier, /haft_spec_section\(action="draft_contract"\)/);
    assert.match(carrier, /haft_query\(action="spec_validate"\)/);
    assert.match(carrier, /haft spec validate --json/);
    assert.match(carrier, /does not determine applicability/);
    assert.match(carrier, /Keep `haft spec check`/);
  });
});

test("Pi exposes exactly the twelve public Haft capabilities as prompts and skills", () => {
  const expected = [
    "h-commission",
    "h-compare",
    "h-decide",
    "h-diagnose",
    "h-explore",
    "h-frame",
    "h-note",
    "h-onboard",
    "h-reason",
    "h-spec",
    "h-status",
    "h-verify"
  ];
  const packageRoot = fileURLToPath(new URL("..", import.meta.url));
  const prompts = readdirSync(join(packageRoot, "prompts"))
    .filter((name) => name.endsWith(".md"))
    .map((name) => name.slice(0, -3))
    .sort();
  const skills = readdirSync(join(packageRoot, "skills"))
    .filter((name) => readFileSync(join(packageRoot, "skills", name, "SKILL.md"), "utf8").length > 0)
    .sort();

  assert.deepEqual(prompts, expected);
  assert.deepEqual(skills, expected);
});

function toolSpec(name: string) {
  const found = HAFT_TOOLS.find((tool) => tool.name === name);
  assert.ok(found, `missing tool spec ${name}`);
  return found;
}

function enumValues(schema: { anyOf?: Array<{ const?: string }> }): string[] {
  return (schema.anyOf ?? [])
    .map((entry) => entry.const)
    .filter((value): value is string => value !== undefined);
}
