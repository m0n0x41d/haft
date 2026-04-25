import type {
  ProjectInfo,
  SpecCheckDocument,
  SpecCheckFinding,
  SpecCheckReport,
} from "./api";
import {
  projectNeedsOnboarding,
  projectReadiness,
  type ProjectReadiness,
} from "./projectReadiness";

export type SpecCarrierKind = "target-system" | "enabling-system" | "term-map";

export type OnboardingOpenActionKind =
  | "open_target_spec"
  | "open_enabling_spec"
  | "open_term_map";

export type OnboardingCommandActionKind =
  | "run_spec_check"
  | "refresh_readiness";

export type OnboardingActionKind =
  | OnboardingOpenActionKind
  | OnboardingCommandActionKind;

export type OnboardingSpecState =
  | "not_checked"
  | "blocked"
  | "clean"
  | "not_applicable";

export type SpecCarrierState =
  | "not_checked"
  | "missing"
  | "blocked"
  | "empty"
  | "draft"
  | "active";

export interface OnboardingOpenAction {
  kind: OnboardingOpenActionKind;
  label: string;
  workflowIntent: string;
  carrierKind: SpecCarrierKind;
  targetPath: string;
}

export interface OnboardingCommandAction {
  kind: OnboardingCommandActionKind;
  label: string;
  workflowIntent: string;
}

export type OnboardingAction =
  | OnboardingOpenAction
  | OnboardingCommandAction;

export interface OnboardingCarrierRow {
  kind: SpecCarrierKind;
  label: string;
  path: string;
  state: SpecCarrierState;
  totalSections: number;
  activeSections: number;
  termMapEntries: number;
  findingCount: number;
  actionKind: OnboardingOpenActionKind;
}

export interface OnboardingFindingView {
  id: string;
  level: string;
  code: string;
  title: string;
  location: string;
  message: string;
  sectionID: string;
  actionKind: OnboardingOpenActionKind | null;
  actionLabel: string;
}

export interface OnboardingCockpitView {
  visible: boolean;
  projectName: string;
  projectPath: string;
  readiness: ProjectReadiness;
  specState: OnboardingSpecState;
  statusLabel: string;
  summary: string;
  carrierRows: OnboardingCarrierRow[];
  findings: OnboardingFindingView[];
  actions: OnboardingAction[];
  primaryAction: OnboardingAction | null;
  genericTaskPrimaryAllowed: boolean;
}

export interface OnboardingActionHandlers {
  openPath: (path: string) => Promise<void>;
  runSpecCheck: (projectPath: string) => Promise<SpecCheckReport>;
  refreshReadiness: (projectPath: string) => Promise<void>;
}

export type OnboardingActionResult =
  | { kind: "opened"; path: string }
  | { kind: "checked"; report: SpecCheckReport }
  | { kind: "refreshed"; projectPath: string };

interface SpecCarrierDefinition {
  kind: SpecCarrierKind;
  label: string;
  fileName: string;
  openActionKind: OnboardingOpenActionKind;
  openActionLabel: string;
  openWorkflowIntent: string;
}

const SPEC_CARRIERS: SpecCarrierDefinition[] = [
  {
    kind: "target-system",
    label: "Target spec",
    fileName: "target-system.md",
    openActionKind: "open_target_spec",
    openActionLabel: "Open Target Spec",
    openWorkflowIntent: "onboarding.target_spec.open",
  },
  {
    kind: "enabling-system",
    label: "Enabling spec",
    fileName: "enabling-system.md",
    openActionKind: "open_enabling_spec",
    openActionLabel: "Open Enabling Spec",
    openWorkflowIntent: "onboarding.enabling_spec.open",
  },
  {
    kind: "term-map",
    label: "Term map",
    fileName: "term-map.md",
    openActionKind: "open_term_map",
    openActionLabel: "Open Term Map",
    openWorkflowIntent: "onboarding.term_map.open",
  },
];

const SPEC_CARRIER_BY_KIND: Record<SpecCarrierKind, SpecCarrierDefinition> = {
  "target-system": SPEC_CARRIERS[0],
  "enabling-system": SPEC_CARRIERS[1],
  "term-map": SPEC_CARRIERS[2],
};

const COMMAND_ACTIONS: OnboardingCommandAction[] = [
  {
    kind: "run_spec_check",
    label: "Run Spec Check",
    workflowIntent: "onboarding.spec_check.run",
  },
  {
    kind: "refresh_readiness",
    label: "Refresh Readiness",
    workflowIntent: "onboarding.readiness.refresh",
  },
];

const SPEC_STATE_LABELS: Record<OnboardingSpecState, string> = {
  not_checked: "Spec check not run",
  blocked: "Spec check blocked",
  clean: "Spec check clean",
  not_applicable: "No onboarding required",
};

export function buildOnboardingCockpit(
  project: ProjectInfo,
  report: SpecCheckReport | null,
): OnboardingCockpitView {
  const readiness = projectReadiness(project);
  const actions = onboardingActions(project.path);
  const visible = projectNeedsOnboarding(project);
  const specState = onboardingSpecState(readiness, report);
  const carrierRows = onboardingCarrierRows(project.path, report);
  const findings = onboardingFindings(report);
  const primaryAction = visible
    ? primaryOnboardingAction(actions, findings, report)
    : null;

  return {
    visible,
    projectName: project.name,
    projectPath: project.path,
    readiness,
    specState,
    statusLabel: SPEC_STATE_LABELS[specState],
    summary: onboardingSummary(project, report, specState),
    carrierRows,
    findings,
    actions,
    primaryAction,
    genericTaskPrimaryAllowed: readiness === "ready",
  };
}

export function onboardingActions(projectPath: string): OnboardingAction[] {
  const openActions = SPEC_CARRIERS.map((carrier) => specCarrierAction(projectPath, carrier));

  return [
    ...openActions,
    ...COMMAND_ACTIONS,
  ];
}

export async function executeOnboardingAction(
  action: OnboardingAction,
  projectPath: string,
  handlers: OnboardingActionHandlers,
): Promise<OnboardingActionResult> {
  switch (action.kind) {
    case "open_target_spec":
    case "open_enabling_spec":
    case "open_term_map":
      await handlers.openPath(action.targetPath);
      return { kind: "opened", path: action.targetPath };
    case "run_spec_check": {
      const report = await handlers.runSpecCheck(projectPath);
      return { kind: "checked", report };
    }
    case "refresh_readiness":
      await handlers.refreshReadiness(projectPath);
      return { kind: "refreshed", projectPath };
  }
}

export function specCarrierPath(projectPath: string, kind: SpecCarrierKind): string {
  const carrier = carrierDefinition(kind);
  const root = projectPath.replace(/\/+$/, "");

  return `${root}/.haft/specs/${carrier.fileName}`;
}

function specCarrierAction(
  projectPath: string,
  carrier: SpecCarrierDefinition,
): OnboardingOpenAction {
  return {
    kind: carrier.openActionKind,
    label: carrier.openActionLabel,
    workflowIntent: carrier.openWorkflowIntent,
    carrierKind: carrier.kind,
    targetPath: specCarrierPath(projectPath, carrier.kind),
  };
}

function primaryOnboardingAction(
  actions: OnboardingAction[],
  findings: OnboardingFindingView[],
  report: SpecCheckReport | null,
): OnboardingAction | null {
  if (!report) {
    return findAction(actions, "run_spec_check");
  }

  const findingActionKind = findings
    .map((finding) => finding.actionKind)
    .find((kind) => kind !== null);
  if (findingActionKind) {
    return findAction(actions, findingActionKind);
  }

  return findAction(actions, "run_spec_check");
}

function findAction(
  actions: OnboardingAction[],
  kind: OnboardingActionKind,
): OnboardingAction | null {
  return actions.find((action) => action.kind === kind) ?? null;
}

function onboardingSpecState(
  readiness: ProjectReadiness,
  report: SpecCheckReport | null,
): OnboardingSpecState {
  if (readiness !== "needs_onboard") {
    return "not_applicable";
  }

  if (!report) {
    return "not_checked";
  }

  if (report.summary.total_findings > 0) {
    return "blocked";
  }

  if (reportHasMinimumSpecShape(report)) {
    return "clean";
  }

  return "blocked";
}

function onboardingCarrierRows(
  projectPath: string,
  report: SpecCheckReport | null,
): OnboardingCarrierRow[] {
  return SPEC_CARRIERS.map((carrier) => {
    const document = findDocument(report, carrier.kind);
    const findingCount = countCarrierFindings(report, carrier.kind);

    return {
      kind: carrier.kind,
      label: carrier.label,
      path: specCarrierPath(projectPath, carrier.kind),
      state: carrierState(report, document, carrier.kind, findingCount),
      totalSections: document?.spec_sections ?? 0,
      activeSections: document?.active_spec_sections ?? 0,
      termMapEntries: document?.term_map_entries ?? 0,
      findingCount,
      actionKind: carrier.openActionKind,
    };
  });
}

function onboardingFindings(report: SpecCheckReport | null): OnboardingFindingView[] {
  if (!report) {
    return [];
  }

  return report.findings.map((finding, index) => findingView(finding, index));
}

function findingView(
  finding: SpecCheckFinding,
  index: number,
): OnboardingFindingView {
  const carrier = carrierKindFromPath(finding.path);
  const sectionID = finding.section_id ?? "";

  return {
    id: `${finding.code}:${finding.path}:${finding.line ?? 0}:${sectionID}:${index}`,
    level: finding.level,
    code: finding.code,
    title: findingTitle(finding, carrier),
    location: findingLocation(finding),
    message: finding.message,
    sectionID,
    actionKind: carrier ? carrierDefinition(carrier).openActionKind : null,
    actionLabel: carrier ? carrierDefinition(carrier).openActionLabel : "",
  };
}

function findingTitle(
  finding: SpecCheckFinding,
  carrier: SpecCarrierKind | null,
): string {
  if (finding.section_id) {
    return finding.section_id;
  }

  if (carrier) {
    return carrierDefinition(carrier).label;
  }

  return finding.code;
}

function findingLocation(finding: SpecCheckFinding): string {
  if (!finding.path) {
    return "";
  }

  if (!finding.line || finding.line <= 0) {
    return finding.path;
  }

  return `${finding.path}:${finding.line}`;
}

function onboardingSummary(
  project: ProjectInfo,
  report: SpecCheckReport | null,
  specState: OnboardingSpecState,
): string {
  if (project.readiness_error) {
    return project.readiness_error;
  }

  if (!report) {
    return "Open the spec carriers or run the deterministic spec check to see concrete gaps.";
  }

  if (specState === "blocked") {
    if (report.summary.total_findings === 0) {
      return "No deterministic findings were returned, but active target/enabling sections and term-map entries are still incomplete.";
    }

    return `${report.summary.total_findings} deterministic finding(s) must be resolved before broad harness work.`;
  }

  if (specState === "clean") {
    return "The deterministic spec check is clean for the current carriers.";
  }

  return "This project is not in onboarding mode.";
}

function findDocument(
  report: SpecCheckReport | null,
  kind: SpecCarrierKind,
): SpecCheckDocument | null {
  if (!report) {
    return null;
  }

  return report.documents.find((document) => document.kind === kind) ?? null;
}

function reportHasMinimumSpecShape(report: SpecCheckReport): boolean {
  const target = findDocument(report, "target-system");
  const enabling = findDocument(report, "enabling-system");
  const termMap = findDocument(report, "term-map");

  if (!target || target.active_spec_sections === 0) {
    return false;
  }

  if (!enabling || enabling.active_spec_sections === 0) {
    return false;
  }

  if (!termMap || termMap.term_map_entries === 0) {
    return false;
  }

  return true;
}

function countCarrierFindings(
  report: SpecCheckReport | null,
  kind: SpecCarrierKind,
): number {
  if (!report) {
    return 0;
  }

  return report.findings
    .filter((finding) => carrierKindFromPath(finding.path) === kind)
    .length;
}

function carrierState(
  report: SpecCheckReport | null,
  document: SpecCheckDocument | null,
  kind: SpecCarrierKind,
  findingCount: number,
): SpecCarrierState {
  if (!report) {
    return "not_checked";
  }

  if (!document) {
    return "missing";
  }

  if (findingCount > 0) {
    return "blocked";
  }

  if (kind === "term-map" && document.term_map_entries > 0) {
    return "active";
  }

  if (kind === "term-map") {
    return "empty";
  }

  if (document.active_spec_sections > 0) {
    return "active";
  }

  if (document.spec_sections > 0) {
    return "draft";
  }

  return "empty";
}

function carrierKindFromPath(path: string): SpecCarrierKind | null {
  if (path.includes("target-system.md")) {
    return "target-system";
  }

  if (path.includes("enabling-system.md")) {
    return "enabling-system";
  }

  if (path.includes("term-map.md")) {
    return "term-map";
  }

  return null;
}

function carrierDefinition(kind: SpecCarrierKind): SpecCarrierDefinition {
  return SPEC_CARRIER_BY_KIND[kind];
}
