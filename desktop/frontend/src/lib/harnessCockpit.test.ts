/// <reference types="node" />

import assert from "node:assert/strict";
import test from "node:test";

import {
  buildHarnessCockpitDetail,
  harnessCommissionActions,
  normalizeApplyReadiness,
  normalizeHarnessCommissionState,
} from "./harnessCockpit.ts";
import type {
  HarnessRunResult,
  HarnessTailResult,
  HarnessWorkspaceAuthorization,
  WorkCommission,
} from "./api.ts";

test("normalizes harness commission states into cockpit states", () => {
  const cases: Array<[string, string]> = [
    ["queued", "runnable"],
    ["running", "running"],
    ["blocked_policy", "blocked"],
    ["completed", "completed"],
    ["completed_with_projection_debt", "completed"],
    ["failed", "failed"],
  ];

  const kinds = cases.map(([state]) =>
    normalizeHarnessCommissionState(commissionFixture(state)).kind,
  );

  assert.deepEqual(
    kinds,
    cases.map(([, kind]) => kind),
  );
});

test("projection debt state is terminal and keeps warning tone", () => {
  const state = normalizeHarnessCommissionState(
    commissionFixture("completed_with_projection_debt"),
  );

  assert.equal(state.kind, "completed");
  assert.equal(state.label, "Projection debt");
  assert.equal(state.tone, "warning");
  assert.equal(state.terminal, true);
});

test("completed commission exposes result tail and apply without cancel or requeue", () => {
  const commission = commissionFixture("completed");
  const result = resultFixture(commission, true);
  const actions = harnessCommissionActions(commission, result);

  assert.equal(action(actions, "result").enabled, true);
  assert.equal(action(actions, "tail").enabled, true);
  assert.equal(action(actions, "apply").enabled, true);
  assert.equal(action(actions, "cancel").enabled, false);
  assert.equal(action(actions, "requeue").enabled, false);
});

test("failed commission exposes recovery actions but not apply without a diff", () => {
  const commission = commissionFixture("failed");
  const result = resultFixture(commission, false);
  const actions = harnessCommissionActions(commission, result);

  assert.equal(action(actions, "result").enabled, true);
  assert.equal(action(actions, "tail").enabled, true);
  assert.equal(action(actions, "apply").enabled, false);
  assert.equal(action(actions, "cancel").enabled, true);
  assert.equal(action(actions, "requeue").enabled, true);
});

test("disabled apply action surfaces typed scope authorization reasons", () => {
  const scenarios = [
    {
      verdict: "out_of_scope",
      paths: ["src/app.go"],
      expectedKind: "out_of_scope",
      expectedReason: "workspace diff contains paths outside commission scope: src/app.go",
    },
    {
      verdict: "forbidden",
      paths: ["secrets/key.txt"],
      expectedKind: "forbidden",
      expectedReason: "workspace diff contains paths forbidden by commission scope: secrets/key.txt",
    },
    {
      verdict: "unknown_scope",
      paths: ["../outside.txt"],
      expectedKind: "unknown_scope",
      expectedReason:
        "workspace diff cannot be applied because commission scope is unknown for paths: ../outside.txt",
    },
  ];

  const observed = scenarios.map((scenario) => {
    const commission = commissionFixture("completed");
    const result = resultFixture(commission, false);
    const authorization = authorizationFixture(scenario.verdict, scenario.paths);
    const facts = result.workspace_facts;
    assert.ok(facts);
    facts.authorization = authorization;
    result.operator_next = staleOperatorReasonFixture();

    const actions = harnessCommissionActions(commission, result);
    const readiness = normalizeApplyReadiness(result);

    return {
      reason: action(actions, "apply").reason,
      readiness,
    };
  });

  assert.deepEqual(
    observed.map((item) => item.reason),
    scenarios.map((scenario) => scenario.expectedReason),
  );
  assert.deepEqual(
    observed.map((item) => item.readiness.kind),
    scenarios.map(() => "disabled"),
  );
  assert.deepEqual(
    observed.map((item) => {
      assert.equal(item.readiness.kind, "disabled");
      return item.readiness.disabledKind;
    }),
    scenarios.map((scenario) => scenario.expectedKind),
  );
});

test("cockpit detail renders workspace diff and evidence facts instead of raw JSON", () => {
  const commission = commissionFixture("completed");
  const result = resultFixture(commission, true);
  const tail = tailFixture(commission.id);
  const detail = buildHarnessCockpitDetail(commission, result, tail);

  assert.equal(detail.workspace.path, "/tmp/workspaces/wc-1");
  assert.equal(detail.workspace.diffState, "changed");
  assert.deepEqual(detail.workspace.changedFiles, ["internal/cli/harness.go"]);
  assert.equal(detail.evidence.requiredCount, 1);
  assert.deepEqual(detail.evidence.requirements, ["kind=go_test command=go test ./..."]);
  assert.equal(detail.tail.hasEvents, true);
  assert.equal("raw" in detail, false);
});

function commissionFixture(state: string): WorkCommission {
  return {
    id: "wc-1",
    state,
    decision_ref: "dec-1",
    problem_card_ref: "prob-1",
    delivery_policy: "workspace_patch_manual",
    valid_until: "2099-01-01T00:00:00Z",
    operator: {
      terminal: state === "completed",
      expired: false,
      attention: state === "failed",
      attention_reason: "",
      suggested_actions: [],
    },
  };
}

function resultFixture(
  commission: WorkCommission,
  canApply: boolean,
): HarnessRunResult {
  return {
    commission,
    workspace: "/tmp/workspaces/wc-1",
    raw: `{"provider":"raw"}`,
    lines: [`{"event":"raw"}`],
    changed_files: ["internal/cli/harness.go"],
    can_apply: canApply,
    workspace_facts: {
      path: "/tmp/workspaces/wc-1",
      diff_state: "changed",
      git_status: ["M internal/cli/harness.go"],
      diff_stat: ["internal/cli/harness.go | 2 +-"],
      changed_files: ["internal/cli/harness.go"],
      error: "",
    },
    runtime_facts: {
      active: false,
      phase: "",
      sub_state: "",
      session_id: "",
      task_pid: "",
      workspace_path: "",
      status_updated_at: "",
      last_event: "",
      last_event_at: "",
      last_turn_status: "",
      last_turn_id: "",
      preview: "",
    },
    evidence_facts: {
      required_count: 1,
      requirements: [
        {
          kind: "go_test",
          command: "go test ./...",
          description: "",
          claim_ref: "",
          section_ref: "",
        },
      ],
      latest_measure: {
        phase: "measure",
        verdict: "pass",
        next: "terminal:pass",
        at: "2026-04-24T05:08:35Z",
      },
      terminal: null,
    },
  };
}

function authorizationFixture(
  verdict: string,
  paths: string[],
): HarnessWorkspaceAuthorization {
  const emptyPaths: string[] = [];

  return {
    verdict,
    can_apply: false,
    allowed_paths: [],
    out_of_scope_paths: verdict === "out_of_scope" ? paths : emptyPaths,
    forbidden_paths: verdict === "forbidden" ? paths : emptyPaths,
    unknown_scope_paths: verdict === "unknown_scope" ? paths : emptyPaths,
    operator_reason: {
      code: "",
      verdict,
      paths,
      message: "operator prose should not win over typed scope facts",
    },
  };
}

function staleOperatorReasonFixture() {
  return {
    kind: "inspect",
    reason: "stale operator prose should not win",
    lines: [],
    apply_disabled_reason: {
      code: "forbidden_paths",
      verdict: "forbidden",
      paths: ["stale.txt"],
      message: "stale operator reason should not win",
    },
  };
}

function tailFixture(commissionID: string): HarnessTailResult {
  return {
    commission_id: commissionID,
    log_path: "/tmp/runtime.jsonl",
    line_count: 20,
    lines: [
      "2026-04-24T05:01:00Z agent_completed: commission=wc-1 phase=execute status=completed",
    ],
    has_events: true,
    follow_command: `haft harness tail ${commissionID} --follow`,
  };
}

function action(
  actions: ReturnType<typeof harnessCommissionActions>,
  kind: string,
) {
  const found = actions.find((item) => item.kind === kind);
  assert.ok(found, `missing action ${kind}`);

  return found;
}
