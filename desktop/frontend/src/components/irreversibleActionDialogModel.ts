export type IrreversibleActionKind =
  | "implement"
  | "create_pr"
  | "reopen"
  | "supersede"
  | "deprecate";

export interface IrreversibleActionArtifact {
  kind: string;
  ref: string;
  title: string;
}

export interface IrreversibleActionDialogInput {
  action: IrreversibleActionKind;
  currentArtifact?: IrreversibleActionArtifact | null;
  relatedArtifact?: IrreversibleActionArtifact | null;
  agent?: string;
  branch?: string;
  usesWorktree?: boolean;
  warnings?: string[];
}

export interface IrreversibleActionDialogModel {
  action: IrreversibleActionKind;
  heading: string;
  confirmLabel: string;
  busyLabel: string;
  tone: "accent" | "warning" | "danger";
  description: string;
  whatWillHappen: string[];
  irreversibleWarning: string;
  affectedArtifacts: IrreversibleActionArtifact[];
  warnings: string[];
  requiresReason: boolean;
  reasonLabel: string;
  reasonPlaceholder: string;
}

type NormalizedDialogInput = Omit<IrreversibleActionDialogInput, "warnings"> & {
  warnings: string[];
};

type ModelBuilder = (input: NormalizedDialogInput) => IrreversibleActionDialogModel;

const MODEL_BUILDERS: Record<IrreversibleActionKind, ModelBuilder> = {
  implement: (input) => ({
    action: "implement",
    heading: "Confirm Implement",
    confirmLabel: "Implement",
    busyLabel: "Starting...",
    tone: "accent",
    description: `Start implementation for ${describeArtifact(input.currentArtifact)}.`,
    whatWillHappen: buildImplementSteps(input),
    irreversibleWarning:
      "This creates persistent task, branch, and verification state that is not rolled back automatically.",
    affectedArtifacts: compactArtifacts([input.currentArtifact]),
    warnings: input.warnings,
    requiresReason: false,
    reasonLabel: "",
    reasonPlaceholder: "",
  }),
  create_pr: (input) => ({
    action: "create_pr",
    heading: "Confirm Create PR",
    confirmLabel: "Create PR",
    busyLabel: "Publishing...",
    tone: "accent",
    description: `Publish the verified implementation for ${describeArtifact(input.currentArtifact)}.`,
    whatWillHappen: buildCreatePullRequestSteps(input),
    irreversibleWarning:
      "Pushing a branch and opening a pull request creates external history that is not undone automatically.",
    affectedArtifacts: compactArtifacts([input.currentArtifact]),
    warnings: input.warnings,
    requiresReason: false,
    reasonLabel: "",
    reasonPlaceholder: "",
  }),
  reopen: (input) => ({
    action: "reopen",
    heading: "Confirm Reopen",
    confirmLabel: "Reopen",
    busyLabel: "Reopening...",
    tone: "warning",
    description: `Start a new problem cycle for ${describeArtifact(input.currentArtifact)}.`,
    whatWillHappen: [
      `Mark ${describeArtifactRef(input.currentArtifact)} as refresh due.`,
      "Create a new ProblemCard linked to the current DecisionRecord.",
      "Record the lifecycle action in a RefreshReport.",
    ],
    irreversibleWarning:
      "This changes artifact lineage and the active governance path. Undo requires another explicit lifecycle action.",
    affectedArtifacts: compactArtifacts([input.currentArtifact]),
    warnings: input.warnings,
    requiresReason: true,
    reasonLabel: "Reason",
    reasonPlaceholder: "Why should this decision be reopened?",
  }),
  supersede: (input) => ({
    action: "supersede",
    heading: "Confirm Supersede",
    confirmLabel: "Supersede",
    busyLabel: "Superseding...",
    tone: "warning",
    description: buildSupersedeDescription(input),
    whatWillHappen: [
      `Mark ${describeArtifactRef(input.currentArtifact)} as superseded.`,
      `Link ${describeArtifactRef(input.relatedArtifact)} as the replacement artifact.`,
      "Record the lifecycle action in a RefreshReport.",
    ],
    irreversibleWarning:
      "Supersession changes which artifact governs the problem space and is not rolled back automatically.",
    affectedArtifacts: compactArtifacts([input.currentArtifact, input.relatedArtifact]),
    warnings: input.warnings,
    requiresReason: true,
    reasonLabel: "Reason",
    reasonPlaceholder: "Why is the replacement artifact taking over?",
  }),
  deprecate: (input) => ({
    action: "deprecate",
    heading: "Confirm Deprecate",
    confirmLabel: "Deprecate",
    busyLabel: "Deprecating...",
    tone: "danger",
    description: `Archive ${describeArtifact(input.currentArtifact)} as no longer relevant.`,
    whatWillHappen: [
      `Mark ${describeArtifactRef(input.currentArtifact)} as deprecated.`,
      "Remove it from the active working set while keeping it available for audit.",
      "Record the lifecycle action in a RefreshReport.",
    ],
    irreversibleWarning:
      "Deprecation removes the artifact from active governance. Reversal requires a new explicit action.",
    affectedArtifacts: compactArtifacts([input.currentArtifact]),
    warnings: input.warnings,
    requiresReason: true,
    reasonLabel: "Reason",
    reasonPlaceholder: "Why is this artifact no longer relevant?",
  }),
};

export function buildIrreversibleActionDialogModel(
  input: IrreversibleActionDialogInput,
): IrreversibleActionDialogModel {
  const normalizedInput = {
    ...input,
    warnings: compactNonEmptyStrings(input.warnings ?? []),
  };

  return MODEL_BUILDERS[input.action](normalizedInput);
}

function buildImplementSteps(input: NormalizedDialogInput): string[] {
  const workspaceStep = input.usesWorktree
    ? "Create a new worktree and feature branch for the implementation task."
    : "Run the implementation task in the active project workspace.";
  const agentName = input.agent?.trim() || "the configured";

  return [
    workspaceStep,
    `Spawn ${agentName} with the DecisionRecord invariants and reasoning context.`,
    "Run post-execution verification before the task can move to Create PR.",
  ];
}

function buildCreatePullRequestSteps(input: NormalizedDialogInput): string[] {
  const pushStep = input.branch?.trim()
    ? `Push branch ${input.branch.trim()} to origin.`
    : "Push the verified implementation branch to origin.";

  return [
    pushStep,
    "Generate a draft pull request from the DecisionRecord rationale and verification result.",
    "Copy the PR body to the clipboard if automatic draft PR creation is unavailable.",
  ];
}

function buildSupersedeDescription(input: NormalizedDialogInput): string {
  const currentArtifact = describeArtifact(input.currentArtifact);
  const relatedArtifact = describeArtifact(input.relatedArtifact);

  return `Replace ${currentArtifact} with ${relatedArtifact}.`;
}

function compactArtifacts(
  artifacts: Array<IrreversibleActionArtifact | null | undefined>,
): IrreversibleActionArtifact[] {
  return artifacts.filter((artifact): artifact is IrreversibleActionArtifact => Boolean(artifact));
}

function compactNonEmptyStrings(values: string[]): string[] {
  return values.map((value) => value.trim()).filter(Boolean);
}

function describeArtifact(artifact?: IrreversibleActionArtifact | null): string {
  const ref = artifact?.ref?.trim() ?? "";
  const kind = artifact?.kind?.trim() ?? "artifact";

  if (ref === "") {
    return kind;
  }

  return `${kind} ${ref}`;
}

function describeArtifactRef(artifact?: IrreversibleActionArtifact | null): string {
  const ref = artifact?.ref?.trim() ?? "";

  if (ref !== "") {
    return ref;
  }

  return describeArtifact(artifact);
}
