/// <reference types="node" />

import assert from "node:assert/strict";
import test from "node:test";

import {
  getHarnessResult,
  listCommissions,
  listProjects,
  type ProjectInfo,
} from "./api.ts";
import {
  buildHarnessCockpitDetail,
  type HarnessActionKind,
  type HarnessActionView,
} from "./harnessCockpit.ts";
import { projectIsRunnable } from "./projectReadiness.ts";
import { getPageTitle, resolveNavigation } from "../navigation.ts";

test("desktop smoke contract covers boot readiness and Harness view model", async () => {
  const navigation = resolveNavigation("harness");
  const pageTitle = getPageTitle(navigation.page);

  assert.equal(navigation.page, "harness");
  assert.equal(pageTitle, "Runtime");

  const projects = await listProjects();
  const activeProject = readyActiveProject(projects);
  const activeProjectRunnable = projectIsRunnable(activeProject);

  assert.equal(activeProjectRunnable, true);
  assert.equal(activeProject.status, "ready");
  assert.equal(activeProject.readiness_source, "core");

  const commissions = await listCommissions("open");
  const selectedCommission = commissions[0];
  assert.ok(selectedCommission, "expected mock desktop boot to expose an open commission");

  const result = await getHarnessResult(selectedCommission.id);
  const detail = buildHarnessCockpitDetail(selectedCommission, result, null);
  const resultEnabled = actionEnabled(detail.actions, "result");
  const tailEnabled = actionEnabled(detail.actions, "tail");
  const applyEnabled = actionEnabled(detail.actions, "apply");

  assert.equal(detail.id, selectedCommission.id);
  assert.equal(detail.state.kind, "runnable");
  assert.equal(detail.workspace.diffState, "clean");
  assert.equal(detail.workspace.canApply, false);
  assert.equal(resultEnabled, true);
  assert.equal(tailEnabled, true);
  assert.equal(applyEnabled, false);
  assert.equal(detail.tail.followCommand, `haft harness tail ${selectedCommission.id} --follow`);
});

function readyActiveProject(projects: ProjectInfo[]): ProjectInfo {
  const readyProjects = projects
    .filter(projectIsRunnable)
    .filter((project) => project.is_active);
  const project = readyProjects[0];

  assert.ok(project, "expected mock desktop boot to expose an active ready project");

  return project;
}

function actionEnabled(
  actions: HarnessActionView[],
  kind: HarnessActionKind,
): boolean {
  const action = actions.find((item) => item.kind === kind);

  assert.ok(action, `missing ${kind} action`);

  return action.enabled;
}
