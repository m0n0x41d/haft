/// <reference types="node" />

import assert from "node:assert/strict";
import test from "node:test";

import type {
  ProjectInfo,
  SpecCheckReport,
} from "./api.ts";
import {
  buildOnboardingCockpit,
  executeOnboardingAction,
  specCarrierPath,
  type OnboardingActionHandlers,
} from "./onboardingCockpit.ts";

test("desktop readiness smoke keeps only ready projects runnable", () => {
  const cases = [
    {
      name: "missing",
      project: projectFixture({
        exists: false,
        has_haft: true,
        has_specs: true,
        status: "ready",
      }),
    },
    {
      name: "needs_init",
      project: projectFixture({
        exists: true,
        has_haft: false,
        has_specs: false,
      }),
    },
    {
      name: "needs_onboard",
      project: projectFixture({
        exists: true,
        has_haft: true,
        has_specs: false,
      }),
    },
    {
      name: "ready",
      project: projectFixture({
        exists: true,
        has_haft: true,
        has_specs: true,
      }),
    },
  ];

  const smoke = cases.map((entry) => {
    const cockpit = buildOnboardingCockpit(entry.project, null);
    const actionKinds = cockpit.visible
      ? cockpit.actions.map((action) => action.kind)
      : [];

    return {
      name: entry.name,
      readiness: cockpit.readiness,
      visible: cockpit.visible,
      specState: cockpit.specState,
      primaryAction: cockpit.primaryAction?.kind ?? "",
      genericTaskPrimaryAllowed: cockpit.genericTaskPrimaryAllowed,
      actionKinds,
    };
  });

  assert.deepEqual(smoke, [
    {
      name: "missing",
      readiness: "missing",
      visible: false,
      specState: "not_applicable",
      primaryAction: "",
      genericTaskPrimaryAllowed: false,
      actionKinds: [],
    },
    {
      name: "needs_init",
      readiness: "needs_init",
      visible: false,
      specState: "not_applicable",
      primaryAction: "",
      genericTaskPrimaryAllowed: false,
      actionKinds: [],
    },
    {
      name: "needs_onboard",
      readiness: "needs_onboard",
      visible: true,
      specState: "not_checked",
      primaryAction: "run_spec_check",
      genericTaskPrimaryAllowed: false,
      actionKinds: [
        "open_target_spec",
        "open_enabling_spec",
        "open_term_map",
        "run_spec_check",
        "refresh_readiness",
      ],
    },
    {
      name: "ready",
      readiness: "ready",
      visible: false,
      specState: "not_applicable",
      primaryAction: "",
      genericTaskPrimaryAllowed: true,
      actionKinds: [],
    },
  ]);
});

test("needs_onboard project exposes typed onboarding actions as the primary surface", () => {
  const project = needsOnboardProject();
  const cockpit = buildOnboardingCockpit(project, null);
  const actionLabels = cockpit.actions.map((action) => action.label);
  const workflowIntents = cockpit.actions.map((action) => action.workflowIntent);

  assert.equal(cockpit.visible, true);
  assert.equal(cockpit.specState, "not_checked");
  assert.equal(cockpit.genericTaskPrimaryAllowed, false);
  assert.equal(cockpit.primaryAction?.kind, "run_spec_check");
  assert.deepEqual(actionLabels, [
    "Open Target Spec",
    "Open Enabling Spec",
    "Open Term Map",
    "Run Spec Check",
    "Refresh Readiness",
  ]);
  assert.equal(workflowIntents.some((intent) => intent.includes("spawn")), false);
});

test("spec-check findings map to carrier rows and next typed actions", () => {
  const project = needsOnboardProject();
  const report = blockedSpecCheckReport();
  const cockpit = buildOnboardingCockpit(project, report);
  const targetRow = cockpit.carrierRows.find((row) => row.kind === "target-system");
  const enablingRow = cockpit.carrierRows.find((row) => row.kind === "enabling-system");
  const termMapRow = cockpit.carrierRows.find((row) => row.kind === "term-map");

  assert.equal(cockpit.specState, "blocked");
  assert.equal(cockpit.primaryAction?.kind, "open_target_spec");
  assert.equal(targetRow?.state, "blocked");
  assert.equal(targetRow?.findingCount, 1);
  assert.equal(enablingRow?.state, "active");
  assert.equal(termMapRow?.state, "active");
  assert.equal(cockpit.findings[0].actionKind, "open_target_spec");
  assert.equal(cockpit.findings[0].actionLabel, "Open Target Spec");
  assert.equal(cockpit.findings[0].location, ".haft/specs/target-system.md:12");
});

test("clean spec-check report is shown honestly without enabling generic tasks", () => {
  const project = needsOnboardProject();
  const report = cleanSpecCheckReport();
  const cockpit = buildOnboardingCockpit(project, report);

  assert.equal(cockpit.specState, "clean");
  assert.equal(cockpit.statusLabel, "Spec check clean");
  assert.equal(cockpit.genericTaskPrimaryAllowed, false);
  assert.equal(cockpit.primaryAction?.kind, "run_spec_check");
});

test("no findings still blocks readiness when active spec shape is incomplete", () => {
  const project = needsOnboardProject();
  const report = draftOnlySpecCheckReport();
  const cockpit = buildOnboardingCockpit(project, report);
  const targetRow = cockpit.carrierRows.find((row) => row.kind === "target-system");

  assert.equal(cockpit.specState, "blocked");
  assert.equal(targetRow?.state, "draft");
  assert.equal(cockpit.summary.includes("active target/enabling sections"), true);
});

test("onboarding action execution dispatches typed effects", async () => {
  const project = needsOnboardProject();
  const cockpit = buildOnboardingCockpit(project, null);
  const openedPaths: string[] = [];
  const checkedPaths: string[] = [];
  const refreshedPaths: string[] = [];
  const handlers: OnboardingActionHandlers = {
    openPath: async (path) => {
      openedPaths.push(path);
    },
    runSpecCheck: async (projectPath) => {
      checkedPaths.push(projectPath);
      return cleanSpecCheckReport();
    },
    refreshReadiness: async (projectPath) => {
      refreshedPaths.push(projectPath);
    },
  };
  const targetAction = cockpit.actions.find((action) => action.kind === "open_target_spec");
  const checkAction = cockpit.actions.find((action) => action.kind === "run_spec_check");
  const refreshAction = cockpit.actions.find((action) => action.kind === "refresh_readiness");

  assert.ok(targetAction);
  assert.ok(checkAction);
  assert.ok(refreshAction);

  const openResult = await executeOnboardingAction(targetAction, project.path, handlers);
  const checkResult = await executeOnboardingAction(checkAction, project.path, handlers);
  const refreshResult = await executeOnboardingAction(refreshAction, project.path, handlers);

  assert.deepEqual(openedPaths, [specCarrierPath(project.path, "target-system")]);
  assert.deepEqual(checkedPaths, [project.path]);
  assert.deepEqual(refreshedPaths, [project.path]);
  assert.equal(openResult.kind, "opened");
  assert.equal(checkResult.kind, "checked");
  assert.equal(refreshResult.kind, "refreshed");
});

interface ProjectFixtureInput {
  exists: boolean;
  has_haft: boolean;
  has_specs: boolean;
  status?: ProjectInfo["status"];
}

function needsOnboardProject(): ProjectInfo {
  return projectFixture({
    exists: true,
    has_haft: true,
    has_specs: false,
    status: "needs_onboard",
  });
}

function projectFixture(input: ProjectFixtureInput): ProjectInfo {
  return {
    path: "/tmp/haft-product",
    name: "haft-product",
    id: "qnt_test",
    status: input.status,
    exists: input.exists,
    has_haft: input.has_haft,
    has_specs: input.has_specs,
    readiness_source: "core",
    readiness_error: "",
    is_active: false,
    problem_count: 0,
    decision_count: 0,
    stale_count: 0,
  };
}

function blockedSpecCheckReport(): SpecCheckReport {
  return {
    level: "L0/L1/L1.5",
    documents: [
      {
        path: ".haft/specs/target-system.md",
        kind: "target-system",
        spec_sections: 1,
        active_spec_sections: 0,
        term_map_entries: 0,
      },
      {
        path: ".haft/specs/enabling-system.md",
        kind: "enabling-system",
        spec_sections: 1,
        active_spec_sections: 1,
        term_map_entries: 0,
      },
      {
        path: ".haft/specs/term-map.md",
        kind: "term-map",
        spec_sections: 0,
        active_spec_sections: 0,
        term_map_entries: 3,
      },
    ],
    findings: [
      {
        level: "L0",
        code: "spec_section_missing_field",
        path: ".haft/specs/target-system.md",
        line: 12,
        section_id: "TS.use.001",
        field_path: "$.owner",
        message: "spec-section missing required field \"owner\"",
      },
    ],
    summary: {
      total_findings: 1,
      spec_sections: 2,
      active_spec_sections: 1,
      term_map_entries: 3,
    },
  };
}

function draftOnlySpecCheckReport(): SpecCheckReport {
  return {
    level: "L0/L1/L1.5",
    documents: [
      {
        path: ".haft/specs/target-system.md",
        kind: "target-system",
        spec_sections: 1,
        active_spec_sections: 0,
        term_map_entries: 0,
      },
      {
        path: ".haft/specs/enabling-system.md",
        kind: "enabling-system",
        spec_sections: 1,
        active_spec_sections: 0,
        term_map_entries: 0,
      },
      {
        path: ".haft/specs/term-map.md",
        kind: "term-map",
        spec_sections: 0,
        active_spec_sections: 0,
        term_map_entries: 1,
      },
    ],
    findings: [],
    summary: {
      total_findings: 0,
      spec_sections: 2,
      active_spec_sections: 0,
      term_map_entries: 1,
    },
  };
}

function cleanSpecCheckReport(): SpecCheckReport {
  return {
    level: "L0/L1/L1.5",
    documents: [
      {
        path: ".haft/specs/target-system.md",
        kind: "target-system",
        spec_sections: 2,
        active_spec_sections: 2,
        term_map_entries: 0,
      },
      {
        path: ".haft/specs/enabling-system.md",
        kind: "enabling-system",
        spec_sections: 2,
        active_spec_sections: 2,
        term_map_entries: 0,
      },
      {
        path: ".haft/specs/term-map.md",
        kind: "term-map",
        spec_sections: 0,
        active_spec_sections: 0,
        term_map_entries: 4,
      },
    ],
    findings: [],
    summary: {
      total_findings: 0,
      spec_sections: 4,
      active_spec_sections: 4,
      term_map_entries: 4,
    },
  };
}
