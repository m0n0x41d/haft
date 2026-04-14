import assert from "node:assert/strict";
import test from "node:test";

import { buildIrreversibleActionDialogModel } from "../src/components/irreversibleActionDialogModel.ts";

test("implement confirmation includes execution effects and warnings", () => {
  const model = buildIrreversibleActionDialogModel({
    action: "implement",
    agent: "codex",
    usesWorktree: true,
    currentArtifact: {
      kind: "DecisionRecord",
      ref: "dec-101",
      title: "Create draft PR from verified task",
    },
    warnings: [
      "No invariants defined — post-execution verification will be skipped",
      "No parity plan recorded — comparison may not be fair — proceed?",
    ],
  });

  assert.equal(model.heading, "Confirm Implement");
  assert.equal(model.requiresReason, false);
  assert.equal(model.affectedArtifacts.length, 1);
  assert.equal(model.affectedArtifacts[0]?.ref, "dec-101");
  assert.match(model.whatWillHappen.join(" "), /worktree/i);
  assert.match(model.whatWillHappen.join(" "), /codex/i);
  assert.equal(model.warnings.length, 2);
});

test("create PR confirmation describes publication and draft fallback", () => {
  const model = buildIrreversibleActionDialogModel({
    action: "create_pr",
    branch: "feat/decision-loop",
    currentArtifact: {
      kind: "DecisionRecord",
      ref: "dec-102",
      title: "Publish verified task",
    },
  });

  assert.equal(model.heading, "Confirm Create PR");
  assert.equal(model.requiresReason, false);
  assert.match(model.whatWillHappen[0] ?? "", /feat\/decision-loop/);
  assert.match(model.whatWillHappen.join(" "), /clipboard/i);
});

test("reopen confirmation requires a reason and records lifecycle effects", () => {
  const model = buildIrreversibleActionDialogModel({
    action: "reopen",
    currentArtifact: {
      kind: "DecisionRecord",
      ref: "dec-103",
      title: "Revisit decision",
    },
  });

  assert.equal(model.requiresReason, true);
  assert.equal(model.reasonLabel, "Reason");
  assert.match(model.whatWillHappen.join(" "), /ProblemCard/);
  assert.match(model.whatWillHappen.join(" "), /RefreshReport/);
});

test("deprecate confirmation shows audit retention and active-set removal", () => {
  const model = buildIrreversibleActionDialogModel({
    action: "deprecate",
    currentArtifact: {
      kind: "DecisionRecord",
      ref: "dec-104",
      title: "Retire obsolete decision",
    },
  });

  assert.equal(model.tone, "danger");
  assert.equal(model.requiresReason, true);
  assert.match(model.whatWillHappen.join(" "), /audit/i);
  assert.match(model.irreversibleWarning, /active governance/i);
});

test("supersede confirmation tracks both the current and replacement artifacts", () => {
  const model = buildIrreversibleActionDialogModel({
    action: "supersede",
    currentArtifact: {
      kind: "DecisionRecord",
      ref: "dec-105",
      title: "Original decision",
    },
    relatedArtifact: {
      kind: "DecisionRecord",
      ref: "dec-106",
      title: "Replacement decision",
    },
  });

  assert.equal(model.requiresReason, true);
  assert.equal(model.affectedArtifacts.length, 2);
  assert.deepEqual(
    model.affectedArtifacts.map((artifact) => artifact.ref),
    ["dec-105", "dec-106"],
  );
  assert.match(model.description, /dec-106/);
});
