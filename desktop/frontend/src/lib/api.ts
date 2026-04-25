import { invoke } from "@tauri-apps/api/core";
import {
  commissionIpcArgs,
  listCommissionsIpcArgs,
  type CommissionSelector,
} from "./harnessIpc.ts";
import { isControlPromptText } from "./controlPrompt.ts";

// API layer — wraps Tauri invoke() with mock fallback for standalone dev.
// When running inside Tauri, calls invoke() from @tauri-apps/api/core.
// When running standalone (npm run dev), it uses mock data.

export interface DashboardData {
  project_name: string;
  problem_count: number;
  decision_count: number;
  portfolio_count: number;
  note_count: number;
  stale_count: number;
  recent_problems: ProblemSummary[];
  recent_decisions: DecisionSummary[];
  healthy_decisions: DecisionSummary[];
  pending_decisions: DecisionSummary[];
  unassessed_decisions: DecisionSummary[];
  stale_items: ArtifactSummary[];
}

export interface ProblemSummary {
  id: string;
  title: string;
  status: string;
  mode: string;
  signal: string;
  reversibility: string;
  constraints: string[];
  created_at: string;
}

export interface DecisionSummary {
  id: string;
  title: string;
  status: string;
  mode: string;
  selected_title: string;
  weakest_link: string;
  valid_until: string;
  created_at: string;
  implement_guard: DecisionImplementGuard;
}

export interface DecisionImplementGuard {
  blocked_reason: string;
  confirmation_messages: string[];
  warning_messages: string[];
}

export interface ArtifactSummary {
  id: string;
  kind: string;
  title: string;
  status: string;
}

export interface ProblemDetail {
  id: string;
  title: string;
  status: string;
  mode: string;
  signal: string;
  constraints: string[];
  optimization_targets: string[];
  observation_indicators: string[];
  acceptance: string;
  blast_radius: string;
  reversibility: string;
  characterizations: CharacterizationView[];
  latest_characterization: CharacterizationView | null;
  linked_portfolios: ArtifactSummary[];
  linked_decisions: ArtifactSummary[];
  body: string;
  created_at: string;
  updated_at: string;
}

export interface DecisionDetail {
  id: string;
  title: string;
  status: string;
  mode: string;
  problem_refs: string[];
  selected_title: string;
  why_selected: string;
  selection_policy: string;
  counterargument: string;
  weakest_link: string;
  why_not_others: { variant: string; reason: string }[];
  invariants: string[];
  pre_conditions: string[];
  post_conditions: string[];
  admissibility: string[];
  evidence_requirements: string[];
  refresh_triggers: string[];
  claims: ClaimView[];
  first_module_coverage: boolean;
  affected_files: string[];
  coverage_modules: CoverageModule[];
  coverage_warnings: string[];
  rollback_triggers: string[];
  rollback_steps: string[];
  rollback_blast_radius: string;
  evidence: {
    items: {
      id: string;
      type: string;
      content: string;
      verdict: string;
      formality_level: number;
      congruence_level: number;
      claim_refs: string[];
      valid_until: string;
      is_expired: boolean;
    }[];
    total_claims: number;
    covered_claims: number;
    coverage_gaps: string[];
  };
  valid_until: string;
  created_at: string;
  updated_at: string;
}

const EMPTY_DECISION_IMPLEMENT_GUARD: DecisionImplementGuard = {
  blocked_reason: "",
  confirmation_messages: [],
  warning_messages: [],
};

export interface CoverageData {
  total_modules: number;
  covered_count: number;
  partial_count: number;
  blind_count: number;
  governed_percent: number;
  last_scanned: string;
  modules: CoverageModule[];
}

export interface CoverageModule {
  id: string;
  path: string;
  name: string;
  lang: string;
  status: string;
  decision_count: number;
  decision_ids: string[];
  impacted: boolean;
  files: string[];
}

export interface GovernanceFinding {
  id: string;
  artifact_ref: string;
  title: string;
  kind: string;
  category: string;
  reason: string;
  valid_until: string;
  days_stale: number;
  r_eff: number;
  drift_count: number;
}

export interface ProblemCandidate {
  id: string;
  status: string;
  title: string;
  signal: string;
  acceptance: string;
  context: string;
  category: string;
  source_artifact_ref: string;
  source_title: string;
  problem_ref: string;
}

export interface GovernanceOverview {
  last_scan_at: string;
  coverage: CoverageData;
  findings: GovernanceFinding[];
  problem_candidates: ProblemCandidate[];
}

export interface ClaimView {
  id: string;
  claim: string;
  observable: string;
  threshold: string;
  status: string;
  verify_after: string;
}

export interface PortfolioDetail {
  id: string;
  title: string;
  status: string;
  problem_ref: string;
  variants: VariantView[];
  comparison: ComparisonView | null;
  body: string;
  created_at: string;
  updated_at: string;
}

export interface PortfolioSummary {
  id: string;
  title: string;
  status: string;
  mode: string;
  problem_ref: string;
  has_comparison: boolean;
  created_at: string;
}

export interface VariantView {
  id: string;
  title: string;
  description: string;
  weakest_link: string;
  novelty_marker: string;
  stepping_stone: boolean;
  strengths: string[];
  risks: string[];
}

export interface ComparisonView {
  dimensions: string[];
  scores: Record<string, Record<string, string>>;
  non_dominated_set: string[];
  dominated_notes: { variant: string; dominated_by: string[]; summary: string }[];
  pareto_tradeoffs: { variant: string; summary: string }[];
  policy_applied: string;
  selected_ref: string;
  recommendation: string;
}

export interface CharacterizationView {
  version: number;
  dimensions: DimensionView[];
  parity_plan: ParityPlanView | null;
}

export interface DimensionView {
  name: string;
  scale_type: string;
  unit: string;
  polarity: string;
  role: string;
  how_to_measure: string;
  valid_until: string;
}

export interface ParityPlanView {
  baseline_set: string[];
  window: string;
  budget: string;
  normalization: NormalizationRule[];
  missing_data_policy: string;
  pinned_conditions: string[];
}

export interface NormalizationRule {
  dimension: string;
  method: string;
}

export interface ProblemCreateInput {
  title: string;
  signal: string;
  acceptance: string;
  blast_radius: string;
  reversibility: string;
  context: string;
  mode: string;
  constraints: string[];
  optimization_targets: string[];
  observation_indicators: string[];
}

export interface ProblemCharacterizationInput {
  problem_ref: string;
  dimensions: DimensionInput[];
  parity_rules: string;
  parity_plan: ParityPlanInput | null;
}

export interface DimensionInput {
  name: string;
  scale_type: string;
  unit: string;
  polarity: string;
  role: string;
  how_to_measure: string;
  valid_until: string;
}

export interface ParityPlanInput {
  baseline_set: string[];
  window: string;
  budget: string;
  normalization: NormalizationRule[];
  missing_data_policy: string;
  pinned_conditions: string[];
}

export interface PortfolioCreateInput {
  problem_ref: string;
  context: string;
  mode: string;
  no_stepping_stone_rationale: string;
  variants: PortfolioVariantInput[];
}

export interface PortfolioVariantInput {
  id: string;
  title: string;
  description: string;
  strengths: string[];
  weakest_link: string;
  novelty_marker: string;
  risks: string[];
  stepping_stone: boolean;
  stepping_stone_basis: string;
  diversity_role: string;
  assumption_notes: string;
  rollback_notes: string;
  evidence_refs: string[];
}

export interface PortfolioCompareInput {
  portfolio_ref: string;
  dimensions: string[];
  scores: Record<string, Record<string, string>>;
  non_dominated_set: string[];
  incomparable: string[][];
  dominated_notes: DominatedNoteInput[];
  pareto_tradeoffs: TradeoffNoteInput[];
  policy_applied: string;
  selected_ref: string;
  recommendation: string;
  parity_plan: ParityPlanInput | null;
}

export interface DominatedNoteInput {
  variant: string;
  dominated_by: string[];
  summary: string;
}

export interface TradeoffNoteInput {
  variant: string;
  summary: string;
}

export interface DecisionCreateInput {
  problem_ref: string;
  problem_refs: string[];
  portfolio_ref: string;
  selected_ref: string;
  selected_title: string;
  why_selected: string;
  selection_policy: string;
  counterargument: string;
  why_not_others: DecisionRejectionInput[];
  invariants: string[];
  pre_conditions: string[];
  post_conditions: string[];
  admissibility: string[];
  evidence_requirements: string[];
  rollback: DecisionRollbackInput | null;
  refresh_triggers: string[];
  weakest_link: string;
  valid_until: string;
  context: string;
  mode: string;
  affected_files: string[];
  predictions: DecisionPredictionInput[];
  search_keywords: string;
  first_module_coverage: boolean;
}

export interface DecisionRejectionInput {
  variant: string;
  reason: string;
}

export interface DecisionRollbackInput {
  triggers: string[];
  steps: string[];
  blast_radius: string;
}

export interface DecisionPredictionInput {
  claim: string;
  observable: string;
  threshold: string;
  verify_after: string;
}

// --- Binding resolution ---

// Project types
export interface ProjectInfo {
  path: string;
  name: string;
  id: string;
  status?: "ready" | "needs_init" | "needs_onboard" | "missing";
  exists?: boolean;
  has_haft?: boolean;
  has_specs?: boolean;
  readiness_source?: string;
  readiness_error?: string;
  is_active: boolean;
  problem_count: number;
  decision_count: number;
  stale_count: number;
}

export interface SpecCheckReport {
  level: string;
  documents: SpecCheckDocument[];
  findings: SpecCheckFinding[];
  summary: SpecCheckSummary;
}

export interface SpecCheckDocument {
  path: string;
  kind: string;
  spec_sections: number;
  active_spec_sections: number;
  term_map_entries: number;
}

export interface SpecCheckFinding {
  level: string;
  code: string;
  path: string;
  field_path?: string;
  line?: number;
  section_id?: string;
  message: string;
}

export interface SpecCheckSummary {
  total_findings: number;
  spec_sections: number;
  active_spec_sections: number;
  term_map_entries: number;
}

export interface AgentPreset {
  name: string;
  agent_kind: string;
  model: string;
  role: string;
}

export interface InstalledAgent {
  kind: string;
  name: string;
  path: string;
  version: string;
}

export interface DesktopConfig {
  default_agent: string;
  review_agent: string;
  verify_agent: string;
  agent_presets: AgentPreset[];
  task_timeout_minutes: number;
  sound_enabled: boolean;
  notify_enabled: boolean;
  default_ide: string;
  default_worktree: boolean;
  auto_wire_mcp: boolean;
}

export interface ChatBlock {
  id: string;
  type: string;
  role?: string;
  text?: string;
  name?: string;
  call_id?: string;
  parent_id?: string;
  input?: string;
  output?: string;
  is_error?: boolean;
}

export interface ChatEntry {
  block: ChatBlock;
  toolResults: ChatBlock[];
  groupedTools?: ChatEntry[];
  toolCount?: number;
  thinkingCount?: number;
}

export interface TaskState {
  id: string;
  title: string;
  agent: string;
  project: string;
  project_path: string;
  status: string;
  prompt: string;
  branch: string;
  worktree: boolean;
  worktree_path: string;
  reused_worktree: boolean;
  started_at: string;
  completed_at: string;
  error_message: string;
  output: string;
  chat_blocks: ChatBlock[];
  raw_output: string;
  auto_run: boolean;
}

export type ChatTranscriptState = Pick<
  TaskState,
  "chat_blocks" | "raw_output" | "output" | "status" | "error_message" | "agent"
>;

export interface TaskOutputEvent {
  id: string;
  chunk: string;
  output: string;
}

export function hasStructuredChatBlocks(task: Pick<TaskState, "chat_blocks">): boolean {
  const entries = buildChatEntries(task.chat_blocks);

  return entries.some((entry) => isStructuredEntry(entry.block));
}

export function taskTranscriptText(task: Pick<TaskState, "raw_output" | "output">): string {
  const rawOutput = task.raw_output.trim();
  if (rawOutput !== "") {
    return rawOutput;
  }

  return task.output;
}

export function buildChatEntries(blocks: ChatBlock[]): ChatEntry[] {
  const filteredBlocks = blocks.filter((block) => !isNoiseBlock(block));
  const mergedBlocks = coalesceNarrativeBlocks(filteredBlocks);
  const toolParentByCallID = new Map<string, string>();
  const toolResultsByParentID = new Map<string, ChatBlock[]>();

  mergedBlocks.forEach((block) => {
    if (block.type !== "tool_use") {
      return;
    }

    if (block.call_id && block.id) {
      toolParentByCallID.set(block.call_id, block.id);
    }
  });

  mergedBlocks.forEach((block) => {
    if (block.type !== "tool_result") {
      return;
    }

    const parentID = resolveToolParentID(block, toolParentByCallID);
    if (!parentID) {
      return;
    }

    const normalizedResult = parentID === block.parent_id
      ? block
      : { ...block, parent_id: parentID };
    const parentResults = toolResultsByParentID.get(parentID) ?? [];
    toolResultsByParentID.set(parentID, [...parentResults, normalizedResult]);
  });

  const ungrouped = mergedBlocks.reduce<ChatEntry[]>((entries, block) => {
    if (block.type === "tool_result" && resolveToolParentID(block, toolParentByCallID)) {
      return entries;
    }

    entries.push({
      block,
      toolResults: block.type === "tool_use"
        ? toolResultsByParentID.get(block.id) ?? []
        : [],
    });

    return entries;
  }, []);

  return groupConsecutiveToolUse(ungrouped);
}

function isNoiseBlock(block: ChatBlock): boolean {
  const text = (block.text ?? "").trim();

  if (isControlPromptText(text)) {
    return true;
  }

  if (isAuditOnlyProviderEnvelope(text)) {
    return true;
  }

  return false;
}

type ProviderEnvelopeVisibility = "visible" | "audit_only";

const AUDIT_ONLY_PROVIDER_ENVELOPE_TYPES = new Set([
  "result",
  "system",
  "rate_limit_event",
  "thread.started",
  "turn.started",
  "turn.completed",
]);

function isAuditOnlyProviderEnvelope(text: string): boolean {
  const trimmed = text.trim();
  const envelope = parseProviderEnvelope(trimmed);

  if (envelope) {
    const visibility = providerEnvelopeVisibility(envelope);

    return visibility === "audit_only";
  }

  const lines = trimmed
    .split("\n")
    .map((line) => line.trim())
    .filter(Boolean);

  if (lines.length <= 1) {
    return false;
  }

  return lines.every(isAuditOnlyProviderEnvelopeLine);
}

function isAuditOnlyProviderEnvelopeLine(text: string): boolean {
  const envelope = parseProviderEnvelope(text);

  if (!envelope) {
    return false;
  }

  const visibility = providerEnvelopeVisibility(envelope);

  return visibility === "audit_only";
}

function parseProviderEnvelope(text: string): Record<string, unknown> | null {
  if (!looksLikeJsonContainer(text)) {
    return null;
  }

  try {
    const value: unknown = JSON.parse(text);

    if (!isPlainRecord(value)) {
      return null;
    }

    return value;
  } catch {
    return null;
  }
}

function providerEnvelopeVisibility(envelope: Record<string, unknown>): ProviderEnvelopeVisibility {
  const envelopeType = envelope.type;

  if (typeof envelopeType !== "string") {
    return "visible";
  }

  if (AUDIT_ONLY_PROVIDER_ENVELOPE_TYPES.has(envelopeType)) {
    return "audit_only";
  }

  return "visible";
}

function isPlainRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function looksLikeJsonContainer(value: string): boolean {
  const trimmed = value.trim();

  if (trimmed === "") {
    return false;
  }

  return (
    (trimmed.startsWith("{") && trimmed.endsWith("}")) ||
    (trimmed.startsWith("[") && trimmed.endsWith("]"))
  );
}

function isGroupableEntry(entry: ChatEntry): boolean {
  return entry.block.type === "tool_use" || entry.block.type === "thinking";
}

function groupConsecutiveToolUse(entries: ChatEntry[]): ChatEntry[] {
  const result: ChatEntry[] = [];
  let group: ChatEntry[] = [];

  const flushGroup = () => {
    if (group.length === 0) {
      return;
    }

    // Single tool_use stays ungrouped; single thinking also stays ungrouped
    if (group.length === 1) {
      result.push(group[0]);
      group = [];
      return;
    }

    const toolCount = group.filter((e) => e.block.type === "tool_use").length;
    const thinkingCount = group.filter((e) => e.block.type === "thinking").length;

    result.push({
      block: {
        id: `group-${group[0].block.id}`,
        type: "tool_group",
      },
      toolResults: [],
      groupedTools: group,
      toolCount,
      thinkingCount,
    });

    group = [];
  };

  entries.forEach((entry) => {
    if (isGroupableEntry(entry)) {
      group.push(entry);
      return;
    }

    flushGroup();
    result.push(entry);
  });

  flushGroup();
  return result;
}

function isStructuredEntry(block: ChatBlock): boolean {
  if (block.type === "tool_use" || block.type === "tool_result" || block.type === "thinking") {
    return hasRenderableBlockValue(block);
  }

  if (block.type !== "text") {
    return hasRenderableBlockValue(block);
  }

  return (block.role ?? "").trim() !== "user" && hasRenderableBlockValue(block);
}

function hasRenderableBlockValue(block: ChatBlock): boolean {
  return firstNonEmptyChatBlockValue(
    block.text,
    block.output,
    block.input,
    block.name,
  ).trim() !== "";
}

function coalesceNarrativeBlocks(blocks: ChatBlock[]): ChatBlock[] {
  return blocks.reduce<ChatBlock[]>((mergedBlocks, block) => {
    const previousBlock = mergedBlocks[mergedBlocks.length - 1];

    if (!canMergeNarrativeBlock(previousBlock, block)) {
      mergedBlocks.push(block);
      return mergedBlocks;
    }

    const previousText = previousBlock.text ?? "";
    const nextText = block.text ?? "";
    mergedBlocks[mergedBlocks.length - 1] = {
      ...previousBlock,
      text: mergeNarrativeText(previousText, nextText),
    };

    return mergedBlocks;
  }, []);
}

function canMergeNarrativeBlock(
  previousBlock: ChatBlock | undefined,
  nextBlock: ChatBlock,
): boolean {
  if (!previousBlock) {
    return false;
  }

  if (previousBlock.type !== nextBlock.type) {
    return false;
  }

  if (previousBlock.type !== "text" && previousBlock.type !== "thinking") {
    return false;
  }

  if ((previousBlock.role ?? "") !== (nextBlock.role ?? "")) {
    return false;
  }

  return true;
}

function mergeNarrativeText(previousText: string, nextText: string): string {
  if (previousText === "") {
    return nextText;
  }

  if (nextText === "") {
    return previousText;
  }

  if (previousText === nextText) {
    return previousText;
  }

  if (nextText.startsWith(previousText)) {
    return nextText;
  }

  if (previousText.endsWith(nextText) || previousText.includes(nextText)) {
    return previousText;
  }

  if (nextText.includes(previousText)) {
    return nextText;
  }

  if (previousText.endsWith("\n") || nextText.startsWith("\n")) {
    return `${previousText}${nextText}`;
  }

  if (previousText.endsWith(" ") || nextText.startsWith(" ")) {
    return `${previousText}${nextText}`;
  }

  if (startsMarkdownBlock(nextText) || endsSentence(previousText)) {
    return `${previousText}\n\n${nextText}`;
  }

  return `${previousText}${nextText}`;
}

function startsMarkdownBlock(value: string): boolean {
  return /^(#{1,6}\s|>\s?|[-*+]\s|\d+\.\s|```)/.test(value.trimStart());
}

function endsSentence(value: string): boolean {
  return /[.!?:]$/.test(value.trimEnd());
}

function resolveToolParentID(
  block: ChatBlock,
  toolParentByCallID: Map<string, string>,
): string {
  const parentID = block.parent_id?.trim();

  if (parentID) {
    return parentID;
  }

  if (!block.call_id) {
    return "";
  }

  return toolParentByCallID.get(block.call_id) ?? "";
}

function firstNonEmptyChatBlockValue(...values: Array<string | undefined>): string {
  for (const value of values) {
    if (!value) {
      continue;
    }

    if (value.trim() !== "") {
      return value;
    }
  }

  return "";
}

export interface PullRequestResult {
  task_id: string;
  decision_ref: string;
  branch: string;
  title: string;
  body: string;
  url: string;
  pushed: boolean;
  draft_created: boolean;
  copied_to_clipboard: boolean;
  warnings: string[];
}

export interface DesktopFlow {
  id: string;
  project_name: string;
  project_path: string;
  title: string;
  description: string;
  template_id: string;
  agent: string;
  prompt: string;
  schedule: string;
  branch: string;
  use_worktree: boolean;
  enabled: boolean;
  last_task_id: string;
  last_run_at: string;
  next_run_at: string;
  last_error: string;
  created_at: string;
  updated_at: string;
}

export interface WorkCommissionScope {
  allowed_paths?: string[];
  affected_files?: string[];
  lockset?: string[];
  forbidden_paths?: string[];
  allowed_actions?: string[];
  target_branch?: string;
  base_sha?: string;
  repo_ref?: string;
}

export interface WorkCommissionOperator {
  terminal?: boolean;
  expired?: boolean;
  attention?: boolean;
  attention_reason?: string;
  suggested_actions?: string[];
}

export interface WorkCommission {
  id: string;
  state: string;
  decision_ref: string;
  problem_card_ref: string;
  implementation_plan_ref?: string;
  projection_policy?: string;
  delivery_policy?: string;
  valid_until?: string;
  fetched_at?: string;
  lockset?: string[];
  scope?: WorkCommissionScope;
  operator?: WorkCommissionOperator;
  events?: Array<Record<string, unknown>>;
}

export interface HarnessRunResult {
  commission: WorkCommission;
  workspace: string;
  raw: string;
  lines: string[];
  changed_files: string[];
  can_apply: boolean;
  runtime?: Record<string, unknown>;
  status_updated_at?: string;
  latest_turn?: Record<string, unknown>;
}

export interface HarnessApplyResult {
  commission_id: string;
  workspace: string;
  project_root: string;
  files: string[];
  raw: string;
  lines: string[];
}

export interface FlowInput {
  id: string;
  title: string;
  description: string;
  template_id: string;
  agent: string;
  prompt: string;
  schedule: string;
  branch: string;
  use_worktree: boolean;
  enabled: boolean;
}

export interface FlowTemplate {
  id: string;
  name: string;
  description: string;
  agent: string;
  schedule: string;
  prompt: string;
  branch: string;
  use_worktree: boolean;
}

export interface TerminalSession {
  id: string;
  title: string;
  project_path: string;
  cwd: string;
  shell: string;
  status: string;
  created_at: string;
  updated_at: string;
}

// --- Transport layer ---

function isTauri(): boolean {
  return typeof window !== "undefined" && "__TAURI_INTERNALS__" in window;
}

async function tauriInvoke<T>(command: string, args?: Record<string, unknown>): Promise<T | null> {
  if (!isTauri()) return null;
  return invoke<T>(command, args);
}

/** Toggle window maximize — no-op outside Tauri. */
export async function toggleMaximize(): Promise<void> {
  if (!isTauri()) return;
  const { getCurrentWindow } = await import("@tauri-apps/api/window");
  await getCurrentWindow().toggleMaximize();
}

// --- Mock data for standalone development ---

const INITIAL_PROBLEM_DETAIL: ProblemDetail = {
  id: "prob-20260409-001",
  title: "AIEE product shape",
  status: "active",
  mode: "standard",
  signal: "Haft is CLI-only. Market moving to visual agent governance surfaces.",
  constraints: [
    "Solo developer + AI agents",
    "Go backend (63K LOC) must stay",
    "Local-first, privacy-first",
    "Must not become another coding agent IDE",
    "Must ship incrementally",
  ],
  optimization_targets: [
    "Time-to-first-visual-experience",
    "Differentiation from existing agent orchestrators",
    "Reuse of existing Go backend",
  ],
  observation_indicators: [
    "Feature count",
    "Technology novelty",
    "Scope creep into coding agent territory",
  ],
  acceptance: "",
  blast_radius: "Product identity, distribution model, frontend tech stack",
  reversibility: "medium",
  characterizations: [],
  latest_characterization: null,
  linked_portfolios: [
    { id: "sol-20260409-001", kind: "SolutionPortfolio", title: "AIEE options", status: "active" },
  ],
  linked_decisions: [
    {
      id: "dec-20260409-001",
      kind: "DecisionRecord",
      title: "Wails native workspace",
      status: "active",
    },
  ],
  body: "",
  created_at: "2026-04-09T00:00:00Z",
  updated_at: "2026-04-09T00:00:00Z",
};

const INITIAL_PORTFOLIO_DETAIL: PortfolioDetail = {
  id: "sol-20260409-001",
  title: "Solutions for AIEE product shape",
  status: "active",
  problem_ref: "prob-20260409-001",
  variants: [],
  comparison: null,
  body: "",
  created_at: "2026-04-09T00:00:00Z",
  updated_at: "2026-04-09T00:00:00Z",
};

const INITIAL_DECISION_DETAIL: DecisionDetail = {
  id: "dec-20260409-001",
  title: "Reasoning Workspace — Wails native, interactive",
  status: "active",
  mode: "standard",
  problem_refs: ["prob-20260409-001"],
  selected_title: "Reasoning Workspace — Wails native",
  why_selected: "Desktop-native from day 1. Single binary distribution, real product identity.",
  selection_policy: "Minimize regret under solo-dev constraints.",
  counterargument:
    "Wails v2 is a bet on a mid-sized OSS project when Tauri and Electron have larger ecosystems.",
  weakest_link: "Wails v2 WebView maturity",
  why_not_others: [
    {
      variant: "Progressive AIEE (web-first)",
      reason: "Theoretical optionality that doesn't pay for itself.",
    },
    {
      variant: "Full AIEE (Electron)",
      reason: "Violates solo-dev-sustainability constraint. 8-12 weeks scope.",
    },
  ],
  invariants: [
    "Go backend remains single source of truth for domain logic",
    "Reasoning-first identity: primary navigation is the decision/problem graph",
    "MCP plugin mode and CLI continue to work",
    "React frontend must be extractable",
  ],
  pre_conditions: [
    "Wails v2 CLI installed and builds hello-world on macOS",
    "React + shadcn/ui project scaffolded with TypeScript",
  ],
  post_conditions: [
    "haft desktop launches native window",
    "Problem board shows active problems",
    "Comparison table renders Pareto front",
  ],
  admissibility: [
    "No terminal emulator in the desktop app",
    "No file editor or diff viewer",
    "No Electron",
    "No FPF jargon in the UI",
    "No cloud backend",
  ],
  evidence_requirements: [
    "Wails v2 hello-world builds on macOS",
    "Binding round-trip latency <50ms",
    "shadcn/ui renders 50+ row table without jank",
  ],
  refresh_triggers: [
    "Wails v3 reaches stable",
    "WebView blocks feature for >3 days",
    "Zero desktop users after 3 months",
  ],
  claims: [
    {
      id: "claim-1",
      claim: "Wails scaffolding + first view <2 weeks",
      observable: "Working native window with problem board",
      threshold: "14 calendar days",
      status: "unverified",
      verify_after: "",
    },
    {
      id: "claim-2",
      claim: "Binary size <30MB",
      observable: "Built binary size on macOS arm64",
      threshold: "<30MB",
      status: "unverified",
      verify_after: "",
    },
  ],
  first_module_coverage: false,
  affected_files: [
    "desktop/app.go",
    "desktop/frontend/src/pages/Dashboard.tsx",
  ],
  coverage_modules: [
    {
      id: "mod-desktop",
      path: "desktop",
      name: "desktop",
      lang: "go",
      status: "covered",
      decision_count: 2,
      decision_ids: ["dec-20260409-001", "dec-20260410-001"],
      impacted: true,
      files: ["desktop/app.go"],
    },
    {
      id: "mod-desktop-frontend-src-pages",
      path: "desktop/frontend/src/pages",
      name: "pages",
      lang: "jsts",
      status: "partial",
      decision_count: 1,
      decision_ids: ["dec-20260409-001"],
      impacted: true,
      files: ["desktop/frontend/src/pages/Dashboard.tsx"],
    },
  ],
  coverage_warnings: [
    "The frontend page module is only partially governed because evidence is still thin.",
  ],
  rollback_triggers: [
    "Wails WebView blocks critical feature after 1 week",
    "Binary size exceeds 100MB",
  ],
  rollback_steps: [
    "Extract React frontend into standalone web app",
    "Add HTTP + WebSocket to Go backend",
    "Remove Wails binding layer",
  ],
  rollback_blast_radius: "Only Wails binding (~200 LOC) is throwaway. 1-2 days to extract.",
  evidence: { items: [], total_claims: 2, covered_claims: 0, coverage_gaps: ["claim-1: Wails scaffolding <2 weeks", "claim-2: Binary size <30MB"] },
  valid_until: "2026-07-09",
  created_at: "2026-04-09T00:00:00Z",
  updated_at: "2026-04-09T00:00:00Z",
};

let mockProblems: ProblemSummary[] = [
  {
    id: INITIAL_PROBLEM_DETAIL.id,
    title: INITIAL_PROBLEM_DETAIL.title,
    status: INITIAL_PROBLEM_DETAIL.status,
    mode: INITIAL_PROBLEM_DETAIL.mode,
    signal: INITIAL_PROBLEM_DETAIL.signal,
    reversibility: INITIAL_PROBLEM_DETAIL.reversibility,
    constraints: INITIAL_PROBLEM_DETAIL.constraints,
    created_at: "2026-04-09",
  },
];

let mockPortfolios: PortfolioSummary[] = [
  {
    id: INITIAL_PORTFOLIO_DETAIL.id,
    title: INITIAL_PORTFOLIO_DETAIL.title,
    status: INITIAL_PORTFOLIO_DETAIL.status,
    mode: "standard",
    problem_ref: INITIAL_PORTFOLIO_DETAIL.problem_ref,
    has_comparison: false,
    created_at: "2026-04-09",
  },
];

let mockDecisions: DecisionSummary[] = [
  {
    id: INITIAL_DECISION_DETAIL.id,
    title: INITIAL_DECISION_DETAIL.title,
    status: INITIAL_DECISION_DETAIL.status,
    mode: INITIAL_DECISION_DETAIL.mode,
    selected_title: INITIAL_DECISION_DETAIL.selected_title,
    weakest_link: INITIAL_DECISION_DETAIL.weakest_link,
    valid_until: INITIAL_DECISION_DETAIL.valid_until,
    created_at: "2026-04-09",
    implement_guard: EMPTY_DECISION_IMPLEMENT_GUARD,
  },
];

const mockProblemDetails = new Map<string, ProblemDetail>([
  [INITIAL_PROBLEM_DETAIL.id, INITIAL_PROBLEM_DETAIL],
]);

const mockPortfolioDetails = new Map<string, PortfolioDetail>([
  [INITIAL_PORTFOLIO_DETAIL.id, INITIAL_PORTFOLIO_DETAIL],
]);

const mockDecisionDetails = new Map<string, DecisionDetail>([
  [INITIAL_DECISION_DETAIL.id, INITIAL_DECISION_DETAIL],
]);

let mockGovernanceOverview: GovernanceOverview = {
  last_scan_at: nowString(),
  coverage: {
    total_modules: 6,
    covered_count: 3,
    partial_count: 1,
    blind_count: 2,
    governed_percent: 66,
    last_scanned: nowString(),
    modules: INITIAL_DECISION_DETAIL.coverage_modules,
  },
  findings: [
    {
      id: "finding-mock-1",
      artifact_ref: INITIAL_DECISION_DETAIL.id,
      title: INITIAL_DECISION_DETAIL.selected_title,
      kind: "DecisionRecord",
      category: "pending_verification",
      reason: "claim claim-2 is ready for verification. Observable: Built binary size on macOS arm64.",
      valid_until: INITIAL_DECISION_DETAIL.valid_until,
      days_stale: 0,
      r_eff: 0,
      drift_count: 0,
    },
  ],
  problem_candidates: [
    {
      id: "cand-mock-1",
      status: "active",
      title: "Verify due claims for Reasoning Workspace — Wails native",
      signal: "claim claim-2 is ready for verification. Observable: Built binary size on macOS arm64.",
      acceptance: "Due claims have evidence attached and the decision measurement reflects the latest verdict.",
      context: "desktop-governance",
      category: "pending_verification",
      source_artifact_ref: INITIAL_DECISION_DETAIL.id,
      source_title: INITIAL_DECISION_DETAIL.selected_title,
      problem_ref: "",
    },
  ],
};

let mockTasks: TaskState[] = [
  {
    id: "task-mock-1",
    title: "Implement desktop automation loop",
    agent: "claude",
    project: "haft",
    project_path: "/Users/demo/projects/haft",
    status: "running",
    prompt: "Implement the operator tooling slice and keep the project-local runtime intact.",
    branch: "feat/operator-tooling",
    worktree: true,
    worktree_path: "/Users/demo/projects/haft/.haft/worktrees/feat/operator-tooling",
    reused_worktree: false, auto_run: false,
    started_at: nowString(),
    completed_at: "",
    error_message: "",
    output: [
      "Inspect the current desktop runtime and map the task lifecycle.",
      "[tool] exec_command",
      "rg -n \"chat_blocks|raw_output\" desktop",
      "desktop/agents.go:79: ChatBlocks []ChatBlock",
      "desktop/task_store.go:152: COALESCE(chat_blocks_json, '[]')",
      "## Extraction plan",
      "",
      "- Move transcript rendering into reusable components.",
      "- Preserve raw fallback for legacy tasks.",
    ].join("\n"),
    chat_blocks: [
      {
        id: "block-mock-1",
        type: "thinking",
        role: "assistant",
        text: "Inspect the current desktop runtime and map the task lifecycle.",
      },
      {
        id: "block-mock-2",
        type: "tool_use",
        role: "assistant",
        name: "exec_command",
        call_id: "call-mock-1",
        input: "rg -n \"chat_blocks|raw_output\" desktop",
      },
      {
        id: "block-mock-3",
        type: "tool_result",
        role: "assistant",
        call_id: "call-mock-1",
        parent_id: "block-mock-2",
        output: [
          "desktop/agents.go:79: ChatBlocks []ChatBlock",
          "desktop/task_store.go:152: COALESCE(chat_blocks_json, '[]')",
        ].join("\n"),
      },
      {
        id: "block-mock-4",
        type: "text",
        role: "assistant",
        text: [
          "## Extraction plan",
          "",
          "- Move transcript rendering into reusable components.",
          "- Preserve raw fallback for legacy tasks.",
        ].join("\n"),
      },
    ],
    raw_output: [
      "Inspect the current desktop runtime and map the task lifecycle.",
      "[tool] exec_command",
      "rg -n \"chat_blocks|raw_output\" desktop",
      "desktop/agents.go:79: ChatBlocks []ChatBlock",
      "desktop/task_store.go:152: COALESCE(chat_blocks_json, '[]')",
      "## Extraction plan",
      "",
      "- Move transcript rendering into reusable components.",
      "- Preserve raw fallback for legacy tasks.",
    ].join("\n"),
  },
  {
    id: "task-mock-2",
    title: "Verify stale decisions",
    agent: "codex",
    project: "repo-b",
    project_path: "/Users/demo/projects/repo-b",
    status: "completed",
    prompt: "Verify stale decisions and summarize evidence gaps.",
    branch: "",
    worktree: false,
    worktree_path: "",
    reused_worktree: false, auto_run: false,
    started_at: nowString(),
    completed_at: nowString(),
    error_message: "",
    output: "Decision coverage report complete.",
    chat_blocks: [],
    raw_output: "Decision coverage report complete.",
  },
];

let mockFlows: DesktopFlow[] = [
  {
    id: "flow-mock-1",
    project_name: "haft",
    project_path: "/Users/demo/projects/haft",
    title: "Decision Refresh",
    description: "Verify due decisions every Monday morning.",
    template_id: "decision-refresh",
    agent: "claude",
    prompt: "Review active decisions with expired or near-expired validity windows.",
    schedule: "0 9 * * 1",
    branch: "flows/decision-refresh",
    use_worktree: true,
    enabled: true,
    last_task_id: "task-mock-2",
    last_run_at: nowString(),
    next_run_at: nowString(),
    last_error: "",
    created_at: nowString(),
    updated_at: nowString(),
  },
];

let mockCommissions: WorkCommission[] = [
  {
    id: "wc-mock-1",
    state: "queued",
    decision_ref: INITIAL_DECISION_DETAIL.id,
    problem_card_ref: INITIAL_PROBLEM_DETAIL.id,
    projection_policy: "local_only",
    delivery_policy: "workspace_patch_manual",
    valid_until: nowString(),
    fetched_at: nowString(),
    lockset: ["desktop/frontend/src/pages/Harness.tsx"],
    scope: {
      allowed_paths: ["desktop/frontend/src/pages/Harness.tsx"],
      affected_files: ["desktop/frontend/src/pages/Harness.tsx"],
      lockset: ["desktop/frontend/src/pages/Harness.tsx"],
      allowed_actions: ["edit_files", "run_tests"],
      target_branch: "dev",
      repo_ref: "local:haft",
    },
    operator: {
      terminal: false,
      expired: false,
      attention: false,
      attention_reason: "",
      suggested_actions: [],
    },
    events: [],
  },
];

let mockTerminalSessions: TerminalSession[] = [];

function nextMockID(prefix: "block" | "prob" | "sol" | "dec" | "flow" | "task" | "term"): string {
  return `${prefix}-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`;
}

function todayString(): string {
  return new Date().toISOString().slice(0, 10);
}

function nowString(): string {
  return new Date().toISOString();
}

function compactList(values: string[]): string[] {
  return values.map((value) => value.trim()).filter(Boolean);
}

// --- Public API ---

export async function getDashboard(): Promise<DashboardData> {
  const result = await tauriInvoke<DashboardData>("get_dashboard");
  if (result) return result;

  return {
    project_name: "",
    problem_count: mockProblems.length,
    decision_count: mockDecisions.length,
    portfolio_count: mockPortfolios.length,
    note_count: 5,
    stale_count: mockGovernanceOverview.findings.length,
    recent_problems: mockProblems,
    recent_decisions: mockDecisions,
    healthy_decisions: mockDecisions,
    pending_decisions: [],
    unassessed_decisions: [],
    stale_items: mockGovernanceOverview.findings.map((finding) => ({
      id: finding.artifact_ref,
      kind: finding.kind,
      title: finding.title,
      status: finding.category,
    })),
  };
}

export async function listProblems(): Promise<ProblemSummary[]> {
  const result = await tauriInvoke<ProblemSummary[]>("list_problems");
  if (result) return result;
  return mockProblems;
}

export async function listDecisions(): Promise<DecisionSummary[]> {
  const result = await tauriInvoke<DecisionSummary[]>("list_decisions");
  if (result) return result;
  return mockDecisions;
}

export async function getProblem(id: string): Promise<ProblemDetail> {
  const result = await tauriInvoke<ProblemDetail>("get_problem", { id });
  if (result) return result;
  return mockProblemDetails.get(id) ?? INITIAL_PROBLEM_DETAIL;
}

export async function getDecision(id: string): Promise<DecisionDetail> {
  const result = await tauriInvoke<DecisionDetail>("get_decision", { id });
  if (result) return result;
  return mockDecisionDetails.get(id) ?? INITIAL_DECISION_DETAIL;
}

export async function listPortfolios(): Promise<PortfolioSummary[]> {
  const result = await tauriInvoke<PortfolioSummary[]>("list_portfolios");
  if (result) return result;
  return mockPortfolios;
}

export async function getPortfolio(id: string): Promise<PortfolioDetail> {
  const result = await tauriInvoke<PortfolioDetail>("get_portfolio", { id });
  if (result) return result;
  return mockPortfolioDetails.get(id) ?? INITIAL_PORTFOLIO_DETAIL;
}

// Probe-or-commit gate
export interface ReadinessReport {
  portfolio_id: string;
  variant_count: number;
  dimension_count: number;
  score_coverage: number;
  constraint_count: number;
  missing_scores: string[];
  has_parity: boolean;
  recommendation: string; // commit, probe, widen, reroute
  recommendation_why: string;
  warnings: string[];
}

export async function assessComparisonReadiness(portfolioID: string): Promise<ReadinessReport> {
  const report = await tauriInvoke<ReadinessReport>("assess_comparison_readiness", { portfolio_id: portfolioID });
  if (report) return { ...report, missing_scores: report.missing_scores ?? [], warnings: report.warnings ?? [] };
  return {
    portfolio_id: portfolioID,
    variant_count: 0,
    dimension_count: 0,
    score_coverage: 0,
    constraint_count: 0,
    missing_scores: [],
    has_parity: false,
    recommendation: "reroute",
    recommendation_why: "Cannot assess readiness without backend connection.",
    warnings: [],
  };
}

export async function getCoverage(): Promise<CoverageData> {
  const coverage = await tauriInvoke<CoverageData>("get_coverage");
  if (coverage) return coverage;
  return mockGovernanceOverview.coverage;
}

export async function getGovernanceOverview(): Promise<GovernanceOverview> {
  const overview = await tauriInvoke<GovernanceOverview>("get_governance_overview");
  if (overview) return normalizeOverview(overview);
  return mockGovernanceOverview;
}

function normalizeOverview(o: GovernanceOverview): GovernanceOverview {
  return {
    ...o,
    findings: o.findings ?? [],
    problem_candidates: o.problem_candidates ?? [],
    coverage: {
      ...o.coverage,
      modules: (o.coverage?.modules ?? []).map(m => ({
        ...m,
        files: m.files ?? [],
      })),
      governed_percent: o.coverage?.governed_percent ?? 0,
      covered_count: o.coverage?.covered_count ?? 0,
      partial_count: o.coverage?.partial_count ?? 0,
      blind_count: o.coverage?.blind_count ?? 0,
    },
  };
}

export async function refreshGovernance(): Promise<GovernanceOverview> {
  const overview = await tauriInvoke<GovernanceOverview>("refresh_governance");
  if (overview) return overview;
  mockGovernanceOverview = {
    ...mockGovernanceOverview,
    last_scan_at: nowString(),
    coverage: {
      ...mockGovernanceOverview.coverage,
      last_scanned: nowString(),
    },
  };
  return mockGovernanceOverview;
}

export async function listProblemCandidates(): Promise<ProblemCandidate[]> {
  const candidates = await tauriInvoke<ProblemCandidate[]>("list_problem_candidates");
  if (candidates) return candidates;
  return mockGovernanceOverview.problem_candidates;
}

export async function dismissProblemCandidate(id: string): Promise<void> {
  await tauriInvoke<void>("dismiss_problem_candidate", { id });
  mockGovernanceOverview = {
    ...mockGovernanceOverview,
    problem_candidates: mockGovernanceOverview.problem_candidates.filter((candidate) => candidate.id !== id),
  };
}

export async function adoptProblemCandidate(id: string): Promise<ProblemDetail> {
  const adopted = await tauriInvoke<ProblemDetail>("adopt_problem_candidate", { id });
  if (adopted) return adopted;

  const candidate = mockGovernanceOverview.problem_candidates.find((item) => item.id === id);
  if (!candidate) {
    throw new Error(`Problem candidate ${id} not found`);
  }

  const detail: ProblemDetail = {
    id: nextMockID("prob"),
    title: candidate.title,
    status: "active",
    mode: "tactical",
    signal: candidate.signal,
    constraints: [],
    optimization_targets: ["Close the surfaced governance gap quickly"],
    observation_indicators: [],
    acceptance: candidate.acceptance,
    blast_radius: "Governance follow-up from the desktop decision loop",
    reversibility: "high",
    characterizations: [],
    latest_characterization: null,
    linked_portfolios: [],
    linked_decisions: candidate.source_artifact_ref
      ? [
          {
            id: candidate.source_artifact_ref,
            kind: "DecisionRecord",
            title: candidate.source_title,
            status: "active",
          },
        ]
      : [],
    body: "",
    created_at: nowString(),
    updated_at: nowString(),
  };

  mockProblemDetails.set(detail.id, detail);
  mockProblems = [
    {
      id: detail.id,
      title: detail.title,
      status: detail.status,
      mode: detail.mode,
      signal: detail.signal,
      reversibility: detail.reversibility,
      constraints: detail.constraints,
      created_at: todayString(),
    },
    ...mockProblems,
  ];
  mockGovernanceOverview = {
    ...mockGovernanceOverview,
    problem_candidates: mockGovernanceOverview.problem_candidates.filter((candidateItem) => candidateItem.id !== id),
  };

  return detail;
}

export async function waiveDecision(decisionID: string, reason: string): Promise<DecisionDetail> {
  const decision = await tauriInvoke<DecisionDetail>("waive_decision", { decision_id: decisionID, reason });
  if (decision) return decision;

  const currentDecision = mockDecisionDetails.get(decisionID) ?? INITIAL_DECISION_DETAIL;
  const validUntil = new Date(Date.now() + 90 * 24 * 60 * 60 * 1000).toISOString();
  const nextDecision = {
    ...currentDecision,
    status: "active",
    valid_until: validUntil,
    updated_at: nowString(),
  };

  mockDecisionDetails.set(decisionID, nextDecision);
  mockDecisions = mockDecisions.map((decisionSummary) =>
    decisionSummary.id === decisionID
      ? {
          ...decisionSummary,
          status: "active",
          valid_until: validUntil,
        }
      : decisionSummary,
  );
  mockGovernanceOverview = {
    ...mockGovernanceOverview,
    findings: mockGovernanceOverview.findings.filter(
      (finding) =>
        finding.artifact_ref !== decisionID || finding.category !== "evidence_expired",
    ),
  };

  return nextDecision;
}

export async function baselineDecision(decisionID: string): Promise<DecisionDetail> {
  const decision = await tauriInvoke<DecisionDetail>("baseline_decision", { decision_id: decisionID });
  if (decision) return decision;

  const currentDecision = mockDecisionDetails.get(decisionID) ?? INITIAL_DECISION_DETAIL;
  const nextDecision = {
    ...currentDecision,
    updated_at: nowString(),
  };

  mockDecisionDetails.set(decisionID, nextDecision);
  mockGovernanceOverview = {
    ...mockGovernanceOverview,
    findings: mockGovernanceOverview.findings.filter(
      (finding) =>
        finding.artifact_ref !== decisionID || finding.category !== "decision_stale",
    ),
  };

  return nextDecision;
}

export async function resolveAdoptBaseline(
  _findingID: string,
  decisionID: string,
): Promise<DecisionDetail> {
  // No backend `resolve_adopt_baseline` command exists; calling tauriInvoke
  // would throw an unknown-command error before the baseline path could run.
  // Delegate directly to baselineDecision — findings auto-resolve when the
  // governance scan next observes the fresh baseline.
  return baselineDecision(decisionID);
}

export async function measureDecision(
  decisionID: string,
  findings: string,
  verdict: string,
): Promise<DecisionDetail> {
  const decision = await tauriInvoke<DecisionDetail>("measure_decision", { decision_id: decisionID, findings, verdict });
  if (decision) return decision;

  const currentDecision = mockDecisionDetails.get(decisionID) ?? INITIAL_DECISION_DETAIL;
  const canonicalVerdict =
    verdict.trim().toLowerCase() === "accepted"
      ? "supports"
      : verdict.trim().toLowerCase() === "partial"
        ? "weakens"
        : "refutes";
  const nextDecision = {
    ...currentDecision,
    evidence: {
      ...currentDecision.evidence,
      items: [
        {
          id: `ev-${Date.now()}`,
          type: "measurement",
          content: findings.trim(),
          verdict: canonicalVerdict,
          formality_level: 2,
          congruence_level: 3,
          claim_refs: [],
          valid_until: currentDecision.valid_until,
          is_expired: false,
        },
        ...currentDecision.evidence.items,
      ],
      covered_claims: Math.max(currentDecision.evidence.covered_claims, 1),
      coverage_gaps: [],
    },
    updated_at: nowString(),
  };

  mockDecisionDetails.set(decisionID, nextDecision);
  mockGovernanceOverview = {
    ...mockGovernanceOverview,
    findings: mockGovernanceOverview.findings.filter(
      (finding) =>
        finding.artifact_ref !== decisionID ||
        (finding.category !== "evidence_expired" && finding.category !== "reff_degraded"),
    ),
  };

  return nextDecision;
}

export async function resolveAdoptMeasure(
  _findingID: string,
  decisionID: string,
  findings: string,
  verdict: string,
): Promise<DecisionDetail> {
  // See resolveAdoptBaseline — no backend command, delegate directly.
  return measureDecision(decisionID, findings, verdict);
}

export async function deprecateDecision(decisionID: string, reason: string): Promise<DecisionDetail> {
  const decision = await tauriInvoke<DecisionDetail>("deprecate_decision", { decision_id: decisionID, reason });
  if (decision) return decision;

  const currentDecision = mockDecisionDetails.get(decisionID) ?? INITIAL_DECISION_DETAIL;
  const nextDecision = {
    ...currentDecision,
    coverage_warnings: [
      ...currentDecision.coverage_warnings,
      `Deprecated: ${reason.trim()}`,
    ],
    status: "deprecated",
    updated_at: nowString(),
  };

  mockDecisionDetails.set(decisionID, nextDecision);
  mockDecisions = mockDecisions.map((decisionSummary) =>
    decisionSummary.id === decisionID
      ? {
          ...decisionSummary,
          status: "deprecated",
        }
      : decisionSummary,
  );
  mockGovernanceOverview = {
    ...mockGovernanceOverview,
    findings: mockGovernanceOverview.findings.filter((finding) => finding.artifact_ref !== decisionID),
  };

  return nextDecision;
}

export async function resolveAdoptDeprecate(
  _findingID: string,
  decisionID: string,
  reason: string,
): Promise<DecisionDetail> {
  // See resolveAdoptBaseline — no backend command, delegate directly.
  return deprecateDecision(decisionID, reason);
}

export async function reopenDecision(decisionID: string, reason: string): Promise<ProblemDetail> {
  // Backend returns { decision_id, decision_status, new_problem_id } — not
  // a ProblemDetail. Callers use `problem.id` for navigation, so we need
  // to hydrate by fetching the freshly-created problem with new_problem_id.
  const result = await tauriInvoke<{ new_problem_id?: string }>(
    "reopen_decision",
    { decision_id: decisionID, reason },
  );
  if (result && result.new_problem_id) {
    const hydrated = await tauriInvoke<ProblemDetail>("get_problem", { id: result.new_problem_id });
    if (hydrated) return hydrated;
  }

  const currentDecision = mockDecisionDetails.get(decisionID) ?? INITIAL_DECISION_DETAIL;
  const nextDecision = {
    ...currentDecision,
    status: "refresh_due",
    updated_at: nowString(),
  };
  const problemID = nextMockID("prob");
  const nextProblem = {
    id: problemID,
    title: `Revisit: ${currentDecision.selected_title}`,
    status: "active",
    mode: "tactical",
    signal: `Decision ${decisionID} needs re-evaluation: ${reason}`,
    constraints: [],
    optimization_targets: [],
    observation_indicators: [],
    acceptance: "",
    blast_radius: "Governance follow-up from the desktop decision loop",
    reversibility: "high",
    characterizations: [],
    latest_characterization: null,
    linked_portfolios: [],
    linked_decisions: [
      {
        id: decisionID,
        kind: "DecisionRecord",
        title: currentDecision.selected_title,
        status: "refresh_due",
      },
    ],
    body: "",
    created_at: nowString(),
    updated_at: nowString(),
  };

  mockDecisionDetails.set(decisionID, nextDecision);
  mockDecisions = mockDecisions.map((decisionSummary) =>
    decisionSummary.id === decisionID
      ? {
          ...decisionSummary,
          status: "refresh_due",
        }
      : decisionSummary,
  );
  mockProblemDetails.set(problemID, nextProblem);
  mockProblems = [
    {
      id: nextProblem.id,
      title: nextProblem.title,
      status: nextProblem.status,
      mode: nextProblem.mode,
      signal: nextProblem.signal,
      reversibility: nextProblem.reversibility,
      constraints: nextProblem.constraints,
      created_at: todayString(),
    },
    ...mockProblems,
  ];

  return nextProblem;
}

export async function resolveAdoptReopen(
  _findingID: string,
  decisionID: string,
  reason: string,
): Promise<ProblemDetail> {
  // See resolveAdoptBaseline — no backend command, delegate directly.
  return reopenDecision(decisionID, reason);
}

export async function resolveAdoptWaive(
  _findingID: string,
  decisionID: string,
  reason: string,
): Promise<DecisionDetail> {
  // See resolveAdoptBaseline — no backend command, delegate directly.
  return waiveDecision(decisionID, reason);
}

export async function createProblem(input: ProblemCreateInput): Promise<ProblemDetail> {
  // Backend's create_problem handler returns only {id, title, kind, status,
  // md_path, created_at} — not a full ProblemDetail. If we render that
  // partial record directly, pages like Problems.tsx crash accessing
  // arrays like `linked_portfolios`. Re-hydrate via getProblem(id) so
  // callers get the canonical ProblemDetail shape with every field present.
  const created = await tauriInvoke<{ id?: string }>("create_problem", { input });
  if (created && created.id) {
    const hydrated = await tauriInvoke<ProblemDetail>("get_problem", { id: created.id });
    if (hydrated) return hydrated;
  }

  const id = nextMockID("prob");
  const detail: ProblemDetail = {
    id,
    title: input.title.trim(),
    status: "active",
    mode: input.mode.trim() || "standard",
    signal: input.signal.trim(),
    constraints: compactList(input.constraints),
    optimization_targets: compactList(input.optimization_targets),
    observation_indicators: compactList(input.observation_indicators),
    acceptance: input.acceptance.trim(),
    blast_radius: input.blast_radius.trim(),
    reversibility: input.reversibility.trim(),
    characterizations: [],
    latest_characterization: null,
    linked_portfolios: [],
    linked_decisions: [],
    body: "",
    created_at: nowString(),
    updated_at: nowString(),
  };

  mockProblemDetails.set(id, detail);
  mockProblems = [
    {
      id,
      title: detail.title,
      status: detail.status,
      mode: detail.mode,
      signal: detail.signal,
      reversibility: detail.reversibility,
      constraints: detail.constraints,
      created_at: todayString(),
    },
    ...mockProblems,
  ];

  return detail;
}

export async function characterizeProblem(input: ProblemCharacterizationInput): Promise<ProblemDetail> {
  // Same hydration dance as createProblem — the characterize RPC returns
  // only artifact metadata, not a full ProblemDetail. Without the follow-up
  // get_problem call the UI dereferences missing arrays (`linked_portfolios`,
  // `constraints`) and crashes right after a successful characterize.
  const characterized = await tauriInvoke<{ id?: string }>("characterize_problem", { input });
  if (characterized && characterized.id) {
    const hydrated = await tauriInvoke<ProblemDetail>("get_problem", { id: characterized.id });
    if (hydrated) return hydrated;
  }

  const current = mockProblemDetails.get(input.problem_ref);
  if (!current) {
    throw new Error(`Problem ${input.problem_ref} not found`);
  }

  const characterization: CharacterizationView = {
    version: current.characterizations.length + 1,
    dimensions: input.dimensions.map((dimension) => ({
      ...dimension,
    })),
    parity_plan: input.parity_plan
      ? {
          baseline_set: compactList(input.parity_plan.baseline_set),
          window: input.parity_plan.window.trim(),
          budget: input.parity_plan.budget.trim(),
          normalization: input.parity_plan.normalization,
          missing_data_policy: input.parity_plan.missing_data_policy.trim(),
          pinned_conditions: compactList(input.parity_plan.pinned_conditions),
        }
      : null,
  };

  const next: ProblemDetail = {
    ...current,
    characterizations: [...current.characterizations, characterization],
    latest_characterization: characterization,
    updated_at: nowString(),
  };

  mockProblemDetails.set(next.id, next);

  return next;
}

export async function createPortfolio(input: PortfolioCreateInput): Promise<PortfolioDetail> {
  // handleCreatePortfolio returns only artifact metadata; UI expects full
  // variants / comparison. Hydrate via get_portfolio after create.
  const created = await tauriInvoke<{ id?: string }>("create_portfolio", { input });
  if (created && created.id) {
    const hydrated = await tauriInvoke<PortfolioDetail>("get_portfolio", { id: created.id });
    if (hydrated) return hydrated;
  }

  const id = nextMockID("sol");
  const variants = input.variants.map((variant, index) => ({
    id: variant.id.trim() || `var-${index + 1}`,
    title: variant.title.trim(),
    description: variant.description.trim(),
    weakest_link: variant.weakest_link.trim(),
    novelty_marker: variant.novelty_marker.trim(),
    stepping_stone: variant.stepping_stone,
    strengths: compactList(variant.strengths),
    risks: compactList(variant.risks),
  }));
  const detail: PortfolioDetail = {
    id,
    title: `Solutions for ${input.problem_ref || "problem"}`,
    status: "active",
    problem_ref: input.problem_ref.trim(),
    variants,
    comparison: null,
    body: "",
    created_at: nowString(),
    updated_at: nowString(),
  };

  mockPortfolioDetails.set(id, detail);
  mockPortfolios = [
    {
      id,
      title: detail.title,
      status: detail.status,
      mode: input.mode.trim() || "standard",
      problem_ref: detail.problem_ref,
      has_comparison: false,
      created_at: todayString(),
    },
    ...mockPortfolios,
  ];

  const problem = mockProblemDetails.get(detail.problem_ref);
  if (problem) {
    const nextProblem: ProblemDetail = {
      ...problem,
      linked_portfolios: [
        { id, kind: "SolutionPortfolio", title: detail.title, status: "active" },
        ...problem.linked_portfolios,
      ],
      updated_at: nowString(),
    };
    mockProblemDetails.set(problem.id, nextProblem);
  }

  return detail;
}

export async function comparePortfolio(input: PortfolioCompareInput): Promise<PortfolioDetail> {
  // handleComparePortfolio returns only artifact metadata; the UI expects
  // full variants/comparison. Hydrate via get_portfolio after compare.
  const compared = await tauriInvoke<{ id?: string }>("compare_portfolio", { input });
  if (compared && compared.id) {
    const hydrated = await tauriInvoke<PortfolioDetail>("get_portfolio", { id: compared.id });
    if (hydrated) return hydrated;
  }

  const current = mockPortfolioDetails.get(input.portfolio_ref);
  if (!current) {
    throw new Error(`Portfolio ${input.portfolio_ref} not found`);
  }

  const nonDominatedSet = input.non_dominated_set.length > 0
    ? compactList(input.non_dominated_set)
    : [input.selected_ref].filter(Boolean);
  const comparison: ComparisonView = {
    dimensions: compactList(input.dimensions),
    scores: input.scores,
    non_dominated_set: nonDominatedSet,
    dominated_notes: input.dominated_notes,
    pareto_tradeoffs: input.pareto_tradeoffs,
    policy_applied: input.policy_applied.trim(),
    selected_ref: input.selected_ref.trim(),
    recommendation: input.recommendation.trim(),
  };

  const next: PortfolioDetail = {
    ...current,
    comparison,
    updated_at: nowString(),
  };

  mockPortfolioDetails.set(next.id, next);
  mockPortfolios = mockPortfolios.map((portfolioSummary) =>
    portfolioSummary.id === next.id
      ? { ...portfolioSummary, has_comparison: true }
      : portfolioSummary,
  );

  return next;
}

export async function createDecision(input: DecisionCreateInput): Promise<DecisionDetail> {
  // handleCreateDecision returns only artifact metadata; Decisions.tsx reads
  // problem_refs / invariants / claims. Hydrate via get_decision after create.
  const created = await tauriInvoke<{ id?: string }>("create_decision", { input });
  if (created && created.id) {
    const hydrated = await tauriInvoke<DecisionDetail>("get_decision", { id: created.id });
    if (hydrated) return hydrated;
  }

  const id = nextMockID("dec");
  const portfolio = input.portfolio_ref ? mockPortfolioDetails.get(input.portfolio_ref) : null;
  const selectedVariant = portfolio?.variants.find(
    (variant) => variant.id === input.selected_ref || variant.title === input.selected_title,
  );
  const selectedTitle = input.selected_title.trim() || selectedVariant?.title || input.selected_ref.trim();
  const whyNotOthers = input.why_not_others.length > 0
    ? input.why_not_others
    : (portfolio?.variants
        .filter((variant) => variant.title !== selectedTitle)
        .map((variant) => ({
          variant: variant.title,
          reason: `Did not beat ${selectedTitle} under the current comparison policy.`,
        })) ?? []);
  const detail: DecisionDetail = {
    id,
    title: selectedTitle,
    status: "active",
    mode: input.mode.trim() || (portfolio?.comparison ? "standard" : "tactical"),
    problem_refs: compactList([input.problem_ref.trim(), ...(portfolio?.problem_ref ? [portfolio.problem_ref] : [])]),
    selected_title: selectedTitle,
    why_selected: input.why_selected.trim(),
    selection_policy: input.selection_policy.trim(),
    counterargument: input.counterargument.trim(),
    weakest_link: input.weakest_link.trim() || selectedVariant?.weakest_link || "",
    why_not_others: whyNotOthers,
    invariants: compactList(input.invariants),
    pre_conditions: compactList(input.pre_conditions),
    post_conditions: compactList(input.post_conditions),
    admissibility: compactList(input.admissibility),
    evidence_requirements: compactList(input.evidence_requirements),
    refresh_triggers: compactList(input.refresh_triggers),
    claims: input.predictions.map((prediction, index) => ({
      id: `${id}-claim-${index + 1}`,
      claim: prediction.claim.trim(),
      observable: prediction.observable.trim(),
      threshold: prediction.threshold.trim(),
      status: "unverified",
      verify_after: prediction.verify_after.trim(),
    })),
    first_module_coverage: input.first_module_coverage,
    affected_files: compactList(input.affected_files),
    coverage_modules: [],
    coverage_warnings: [],
    rollback_triggers: compactList(input.rollback?.triggers ?? []),
    rollback_steps: compactList(input.rollback?.steps ?? []),
    rollback_blast_radius: input.rollback?.blast_radius.trim() ?? "",
    evidence: { items: [], total_claims: 0, covered_claims: 0, coverage_gaps: [] },
    valid_until: input.valid_until.trim(),
    created_at: nowString(),
    updated_at: nowString(),
  };

  mockDecisionDetails.set(id, detail);
  mockDecisions = [
    {
      id,
      title: detail.title,
      status: detail.status,
      mode: detail.mode,
      selected_title: detail.selected_title,
      weakest_link: detail.weakest_link,
      valid_until: detail.valid_until,
      created_at: todayString(),
      implement_guard: EMPTY_DECISION_IMPLEMENT_GUARD,
    },
    ...mockDecisions,
  ];

  const problemRef = input.problem_ref.trim() || portfolio?.problem_ref || "";
  const problem = problemRef ? mockProblemDetails.get(problemRef) : null;
  if (problem) {
    const nextProblem: ProblemDetail = {
      ...problem,
      linked_decisions: [
        { id, kind: "DecisionRecord", title: detail.title, status: "active" },
        ...problem.linked_decisions,
      ],
      updated_at: nowString(),
    };
    mockProblemDetails.set(problem.id, nextProblem);
  }

  return detail;
}

export async function searchArtifacts(query: string): Promise<ArtifactSummary[]> {
  const result = await tauriInvoke<ArtifactSummary[]>("search_artifacts", { query });
  if (result) return result;
  return [];
}

// --- Project management ---

export async function listProjects(): Promise<ProjectInfo[]> {
  const result = await tauriInvoke<ProjectInfo[]>("list_projects");
  if (result) return result;
  return [
    {
      path: "/Users/demo/projects/haft",
      name: "haft",
      id: "qnt_demo1",
      status: "ready",
      exists: true,
      has_haft: true,
      has_specs: true,
      readiness_source: "core",
      readiness_error: "",
      is_active: true,
      problem_count: 12,
      decision_count: 8,
      stale_count: 2,
    },
  ];
}

export async function addProject(path: string): Promise<ProjectInfo> {
  const result = await tauriInvoke<ProjectInfo>("add_project", { path });
  if (result) return result;
  return { path, name: path.split("/").pop() || path, id: "", status: "needs_onboard", exists: true, has_haft: true, has_specs: false, readiness_source: "degraded_core_unavailable", readiness_error: "backend connection unavailable", is_active: false, problem_count: 0, decision_count: 0, stale_count: 0 };
}

export async function addProjectSmart(path: string): Promise<ProjectInfo> {
  const result = await tauriInvoke<ProjectInfo>("add_project_smart", { path });
  if (result) return result;
  return { path, name: path.split("/").pop() || path, id: "", status: "needs_onboard", exists: true, has_haft: true, has_specs: false, readiness_source: "degraded_core_unavailable", readiness_error: "backend connection unavailable", is_active: false, problem_count: 0, decision_count: 0, stale_count: 0 };
}

export async function switchProject(path: string): Promise<void> {
  await tauriInvoke<void>("switch_project", { path });
}

export async function removeProject(path: string): Promise<void> {
  await tauriInvoke<void>("remove_project", { path });
}

export async function scanForProjects(): Promise<ProjectInfo[]> {
  const result = await tauriInvoke<ProjectInfo[]>("scan_for_projects");
  if (result) return result;
  return [];
}

export async function openDirectoryPicker(): Promise<string> {
  const path = await tauriInvoke<string>("open_directory_picker");
  return path ?? "";
}

export async function initProject(path: string): Promise<ProjectInfo> {
  const project = await tauriInvoke<ProjectInfo>("init_project", { path });
  if (project) return project;
  return {
    path,
    name: path.split("/").pop() || path,
    id: "",
    status: "needs_onboard",
    exists: true,
    has_haft: true,
    has_specs: false,
    readiness_source: "degraded_core_unavailable",
    readiness_error: "backend connection unavailable",
    is_active: false,
    problem_count: 0,
    decision_count: 0,
    stale_count: 0,
  };
}

export async function runSpecCheck(projectRoot: string): Promise<SpecCheckReport> {
  const report = await tauriInvoke<SpecCheckReport>(
    "run_spec_check",
    projectRootIpcArgs(projectRoot),
  );
  if (report) return normalizeSpecCheckReport(report);

  return normalizeSpecCheckReport(mockSpecCheckReport(projectRoot));
}

function projectRootIpcArgs(projectRoot: string): { projectRoot: string } {
  return { projectRoot };
}

function normalizeSpecCheckReport(report: SpecCheckReport): SpecCheckReport {
  return {
    level: report.level || "L0/L1/L1.5",
    documents: report.documents ?? [],
    findings: report.findings ?? [],
    summary: {
      total_findings: report.summary?.total_findings ?? 0,
      spec_sections: report.summary?.spec_sections ?? 0,
      active_spec_sections: report.summary?.active_spec_sections ?? 0,
      term_map_entries: report.summary?.term_map_entries ?? 0,
    },
  };
}

function mockSpecCheckReport(projectRoot: string): SpecCheckReport {
  const root = projectRoot.replace(/\/+$/, "");

  return {
    level: "L0/L1/L1.5",
    documents: [
      specCheckDocument(`${root}/.haft/specs/target-system.md`, "target-system"),
      specCheckDocument(`${root}/.haft/specs/enabling-system.md`, "enabling-system"),
      specCheckDocument(`${root}/.haft/specs/term-map.md`, "term-map"),
    ],
    findings: [
      {
        level: "L0",
        code: "desktop_spec_check_unavailable",
        path: "",
        message: "Run inside Haft Desktop to execute the core spec check.",
      },
    ],
    summary: {
      total_findings: 1,
      spec_sections: 0,
      active_spec_sections: 0,
      term_map_entries: 0,
    },
  };
}

function specCheckDocument(path: string, kind: string): SpecCheckDocument {
  return {
    path,
    kind,
    spec_sections: 0,
    active_spec_sections: 0,
    term_map_entries: 0,
  };
}

// --- Task management ---

export async function listTasks(): Promise<TaskState[]> {
  const tasks = await tauriInvoke<TaskState[]>("list_tasks");
  if (tasks) return tasks;
  return mockTasks;
}

export async function listAllTasks(): Promise<TaskState[]> {
  const tasks = await tauriInvoke<TaskState[]>("list_all_tasks");
  if (tasks) return tasks;
  return mockTasks;
}

export async function detectAgents(): Promise<InstalledAgent[]> {
  const agents = await tauriInvoke<InstalledAgent[]>("detect_agents");
  return agents ?? [];
}

export async function spawnTask(agent: string, prompt: string, worktree: boolean, branch: string): Promise<TaskState> {
  const task = await tauriInvoke<TaskState>("spawn_task", { agent, prompt, worktree, branch });
  if (task) return task;

  const initialTranscript = `[user] ${prompt}`;
  const createdTask: TaskState = {
    id: nextMockID("task"),
    title: prompt.slice(0, 60),
    agent,
    project: "haft",
    project_path: "/Users/demo/projects/haft",
    status: "running",
    prompt,
    branch,
    worktree,
    worktree_path: worktree ? `/Users/demo/projects/haft/.haft/worktrees/${branch}` : "",
    reused_worktree: false, auto_run: false,
    started_at: new Date().toISOString(),
    completed_at: "",
    error_message: "",
    output: initialTranscript,
    chat_blocks: [
      {
        id: nextMockID("block"),
        type: "text",
        role: "user",
        text: prompt,
      },
    ],
    raw_output: initialTranscript,
  };

  mockTasks = [createdTask, ...mockTasks];
  return createdTask;
}

export async function cancelTask(id: string): Promise<void> {
  await tauriInvoke<void>("cancel_task", { id });
  mockTasks = mockTasks.map((task) =>
    task.id === id
      ? { ...task, status: "cancelled", completed_at: nowString() }
      : task,
  );
}

export async function writeTaskInput(id: string, data: string): Promise<void> {
  const trimmed = data.trim();
  if (!trimmed) {
    return;
  }

  await tauriInvoke<void>("write_task_input", { id, data: trimmed });

  mockTasks = mockTasks.map((task) => {
    if (task.id !== id || task.status !== "running") {
      return task;
    }

    const nextOutput = [task.output, `[user] ${trimmed}`]
      .filter(Boolean)
      .join("\n");

    return {
      ...task,
      output: nextOutput,
      raw_output: nextOutput,
      chat_blocks: [
        ...task.chat_blocks,
        {
          id: nextMockID("block"),
          type: "text",
          role: "user",
          text: trimmed,
        },
      ],
    };
  });
}

export async function archiveTask(id: string): Promise<void> {
  await tauriInvoke<void>("archive_task", { id });
  mockTasks = mockTasks.filter((task) => task.id !== id);
}

export async function setTaskAutoRun(id: string, autoRun: boolean): Promise<void> {
  await tauriInvoke<void>("set_task_auto_run", { id, auto_run: autoRun });
}

export async function getTaskOutput(id: string): Promise<string> {
  const output = await tauriInvoke<string>("get_task_output", { id });
  if (output != null) return output; // null check, not falsy — empty string "" is valid
  return mockTasks.find((task) => task.id === id)?.output ?? "";
}

export async function getTaskTranscriptState(id: string): Promise<ChatTranscriptState> {
  const task = await listTasks()
    .then((tasks) => tasks.find((item) => item.id === id) ?? null);

  if (task) {
    return pickTaskTranscriptState(task);
  }

  const output = await getTaskOutput(id);

  return {
    chat_blocks: [],
    raw_output: output,
    output,
    status: "",
    error_message: "",
    agent: "",
  };
}

export async function handoffTask(id: string, agent: string): Promise<TaskState> {
  const task = await tauriInvoke<TaskState>("handoff_task", { id, agent });
  if (task) return task;

  const source = mockTasks.find((item) => item.id === id);
  if (!source) {
    throw new Error(`Task ${id} not found`);
  }

  return spawnTask(agent, `Handoff for ${source.title}\n\n${source.prompt}`, source.worktree, source.branch);
}

export async function continueTask(id: string, message: string): Promise<TaskState> {
  const trimmed = message.trim();
  if (!trimmed) {
    throw new Error("Continuation message is required");
  }

  const task = await tauriInvoke<TaskState>("continue_task", { id, message: trimmed });
  if (task) return task;

  const source = mockTasks.find((item) => item.id === id);
  if (!source) {
    throw new Error(`Task ${id} not found`);
  }

  const continuationPrompt = [
    "Continue the existing desktop task.",
    "",
    `Task title:\n${source.title}`,
    "",
    `Original prompt:\n${source.prompt}`,
    "",
    `Prior transcript tail:\n${source.raw_output || source.output}`,
    "",
    `Operator follow-up:\n${trimmed}`,
  ].join("\n");

  return spawnTask(source.agent, continuationPrompt, source.worktree, source.branch);
}

function pickTaskTranscriptState(task: ChatTranscriptState): ChatTranscriptState {
  return {
    chat_blocks: task.chat_blocks ?? [],
    raw_output: task.raw_output ?? "",
    output: task.output ?? "",
    status: task.status ?? "",
    error_message: task.error_message ?? "",
    agent: task.agent ?? "",
  };
}

export async function listCommissions(
  selector: CommissionSelector = "open",
): Promise<WorkCommission[]> {
  const result = await tauriInvoke<{ commissions?: WorkCommission[] }>(
    "list_commissions",
    listCommissionsIpcArgs(selector),
  );
  if (result) return result.commissions ?? [];

  if (selector === "all") return mockCommissions;
  if (selector === "terminal") {
    return mockCommissions.filter((commission) => commission.operator?.terminal);
  }
  if (selector === "stale") {
    return mockCommissions.filter((commission) => commission.operator?.attention);
  }
  if (selector === "runnable") {
    return mockCommissions.filter((commission) => commission.state === "queued" || commission.state === "ready");
  }
  return mockCommissions.filter((commission) => !commission.operator?.terminal);
}

export async function showCommission(commissionID: string): Promise<WorkCommission> {
  const result = await tauriInvoke<{ commission?: WorkCommission }>(
    "show_commission",
    commissionIpcArgs(commissionID),
  );
  if (result?.commission) return result.commission;

  const commission = mockCommissions.find((item) => item.id === commissionID);
  if (!commission) {
    throw new Error(`WorkCommission ${commissionID} not found`);
  }
  return commission;
}

export async function requeueCommission(commissionID: string, reason: string): Promise<WorkCommission> {
  const result = await tauriInvoke<{ commission?: WorkCommission }>("requeue_commission", {
    ...commissionIpcArgs(commissionID),
    reason,
  });
  if (result?.commission) return result.commission;

  return updateMockCommission(commissionID, {
    state: "queued",
    operator: {
      terminal: false,
      expired: false,
      attention: false,
      attention_reason: "",
      suggested_actions: [],
    },
  });
}

export async function cancelCommission(commissionID: string, reason: string): Promise<WorkCommission> {
  const result = await tauriInvoke<{ commission?: WorkCommission }>("cancel_commission", {
    ...commissionIpcArgs(commissionID),
    reason,
  });
  if (result?.commission) return result.commission;

  return updateMockCommission(commissionID, {
    state: "cancelled",
    operator: {
      terminal: true,
      expired: false,
      attention: false,
      attention_reason: "",
      suggested_actions: [],
    },
  });
}

export async function getHarnessResult(commissionID: string): Promise<HarnessRunResult> {
  const result = await tauriInvoke<HarnessRunResult>(
    "harness_result",
    commissionIpcArgs(commissionID),
  );
  if (result) return normalizeHarnessResult(result);

  const commission = await showCommission(commissionID);
  return {
    commission,
    workspace: `/Users/demo/.open-sleigh/workspaces/${commissionID}`,
    raw: [
      "Open-Sleigh harness result",
      `commission: ${commission.id}`,
      `state: ${commission.state}`,
      `decision: ${commission.decision_ref}`,
      "git_status:",
      "- clean",
      "diff_stat:",
      "- empty",
    ].join("\n"),
    lines: [],
    changed_files: [],
    can_apply: false,
  };
}

export async function applyHarnessResult(commissionID: string): Promise<HarnessApplyResult> {
  const result = await tauriInvoke<HarnessApplyResult>(
    "harness_apply",
    commissionIpcArgs(commissionID),
  );
  if (result) return normalizeHarnessApplyResult(result);

  return {
    commission_id: commissionID,
    workspace: `/Users/demo/.open-sleigh/workspaces/${commissionID}`,
    project_root: "/Users/demo/projects/haft",
    files: [],
    raw: "No diff available in mock mode.",
    lines: ["No diff available in mock mode."],
  };
}

async function updateMockCommission(
  commissionID: string,
  patch: Partial<WorkCommission>,
): Promise<WorkCommission> {
  const current = mockCommissions.find((commission) => commission.id === commissionID);
  if (!current) {
    throw new Error(`WorkCommission ${commissionID} not found`);
  }

  const next = {
    ...current,
    ...patch,
  };
  mockCommissions = mockCommissions.map((commission) =>
    commission.id === commissionID ? next : commission,
  );
  return next;
}

function normalizeHarnessResult(result: HarnessRunResult): HarnessRunResult {
  return {
    ...result,
    lines: result.lines ?? [],
    changed_files: result.changed_files ?? [],
    raw: result.raw ?? "",
    can_apply: Boolean(result.can_apply),
  };
}

function normalizeHarnessApplyResult(result: HarnessApplyResult): HarnessApplyResult {
  return {
    ...result,
    files: result.files ?? [],
    lines: result.lines ?? [],
    raw: result.raw ?? "",
  };
}

export async function listFlows(): Promise<DesktopFlow[]> {
  const flows = await tauriInvoke<DesktopFlow[]>("list_flows");
  if (flows) return flows;
  return mockFlows;
}

export async function listFlowTemplates(): Promise<FlowTemplate[]> {
  const templates = await tauriInvoke<FlowTemplate[]>("list_flow_templates");
  if (templates) return templates;

  return [
    {
      id: "decision-refresh",
      name: "Decision Refresh",
      description: "Verify stale decisions every Monday morning.",
      agent: "claude",
      schedule: "0 9 * * 1",
      prompt: "Review active decisions with expired or near-expired validity windows.",
      branch: "flows/decision-refresh",
      use_worktree: true,
    },
    {
      id: "drift-scan",
      name: "Drift Detection",
      description: "Scan for drift across governed files on workdays.",
      agent: "codex",
      schedule: "0 10 * * 1-5",
      prompt: "Scan the current project for drift against decision baselines and recently affected files.",
      branch: "flows/drift-scan",
      use_worktree: true,
    },
    {
      id: "coverage-report",
      name: "Coverage Report",
      description: "Generate a weekly governance coverage summary.",
      agent: "claude",
      schedule: "0 15 * * 1",
      prompt: "Summarize module governance coverage for the current project.",
      branch: "flows/coverage-report",
      use_worktree: false,
    },
  ];
}

export async function createFlow(input: FlowInput): Promise<DesktopFlow> {
  const flow = await tauriInvoke<DesktopFlow>("create_flow", { input });
  if (flow) return flow;

  const createdFlow: DesktopFlow = {
    ...input,
    id: nextMockID("flow"),
    project_name: "haft",
    project_path: "/Users/demo/projects/haft",
    last_task_id: "",
    last_run_at: "",
    next_run_at: nowString(),
    last_error: "",
    created_at: nowString(),
    updated_at: nowString(),
  };

  mockFlows = [createdFlow, ...mockFlows];
  return createdFlow;
}

export async function updateFlow(input: FlowInput): Promise<DesktopFlow> {
  const flow = await tauriInvoke<DesktopFlow>("update_flow", { input });
  if (flow) return flow;

  const current = mockFlows.find((item) => item.id === input.id);
  if (!current) {
    throw new Error(`Flow ${input.id} not found`);
  }

  const nextFlow: DesktopFlow = {
    ...current,
    ...input,
    updated_at: nowString(),
  };

  mockFlows = mockFlows.map((item) => (item.id === input.id ? nextFlow : item));
  return nextFlow;
}

export async function toggleFlow(id: string, enabled: boolean): Promise<DesktopFlow> {
  const flow = await tauriInvoke<DesktopFlow>("toggle_flow", { id, enabled });
  if (flow) return flow;

  const current = mockFlows.find((item) => item.id === id);
  if (!current) {
    throw new Error(`Flow ${id} not found`);
  }

  const nextFlow: DesktopFlow = {
    ...current,
    enabled,
    next_run_at: enabled ? nowString() : "",
    updated_at: nowString(),
  };

  mockFlows = mockFlows.map((item) => (item.id === id ? nextFlow : item));
  return nextFlow;
}

export async function deleteFlow(id: string): Promise<void> {
  await tauriInvoke<void>("delete_flow", { id });
  mockFlows = mockFlows.filter((flow) => flow.id !== id);
}

export async function runFlowNow(id: string): Promise<TaskState> {
  const task = await tauriInvoke<TaskState>("run_flow_now", { id });
  if (task) return task;

  const flow = mockFlows.find((item) => item.id === id);
  if (!flow) {
    throw new Error(`Flow ${id} not found`);
  }

  const createdTask = await spawnTask(flow.agent, flow.prompt, flow.use_worktree, flow.branch);
  mockFlows = mockFlows.map((item) =>
    item.id === id
      ? {
          ...item,
          last_task_id: createdTask.id,
          last_run_at: nowString(),
          updated_at: nowString(),
        }
      : item,
  );

  return createdTask;
}

export async function implementDecision(
  decisionID: string,
  agent: string,
  worktree: boolean,
  branch: string,
): Promise<TaskState> {
  // `implement_decision` RPC returns {decision_ref, brief} — not a task.
  // We fetch the brief (optional), then spawn an actual agent task whose
  // prompt includes the brief. If the RPC fails, fall back to a simple
  // implement prompt so the user can still kick off the task.
  let prompt = `Implement ${decisionID}`;
  const briefResult = await tauriInvoke<{ brief?: string }>("implement_decision", {
    decision_id: decisionID,
    agent,
    worktree,
    branch,
  }).catch(() => null);
  if (briefResult && briefResult.brief) {
    prompt = `Implement ${decisionID}\n\n${briefResult.brief}`;
  }
  return spawnTask(agent, prompt, worktree, branch);
}

export async function createPullRequest(
  taskID: string,
  decisionRef: string,
  branch: string,
): Promise<PullRequestResult> {
  const result = await tauriInvoke<PullRequestResult>("create_pull_request", {
    decision_ref: decisionRef,
    branch,
  });
  if (result) return result;

  const task = mockTasks.find((item) => item.id === taskID);
  if (!task) {
    throw new Error(`Task ${taskID} not found`);
  }

  return {
    task_id: task.id,
    decision_ref: decisionRef,
    branch,
    title: task.title,
    body: `## Summary\n\n- Decision: ${decisionRef || "unknown"}\n- Task: ${task.id}`,
    url: "",
    pushed: false,
    draft_created: false,
    copied_to_clipboard: true,
    warnings: ["Automatic draft PR creation is unavailable in mock mode."],
  };
}

export async function verifyDecision(decisionID: string, agent: string): Promise<TaskState> {
  // `verify_decision` RPC returns {decision_ref, invariants} — not a task.
  // Fetch the invariants list (optional) and spawn an agent task whose
  // prompt asks it to verify those specific invariants.
  let prompt = `Verify ${decisionID}`;
  const verifyResult = await tauriInvoke<{ invariants?: unknown }>("verify_decision", {
    decision_id: decisionID,
    agent,
  }).catch(() => null);
  if (verifyResult && verifyResult.invariants) {
    prompt = `Verify ${decisionID}\n\nInvariants to check:\n${JSON.stringify(verifyResult.invariants, null, 2)}`;
  }
  return spawnTask(agent, prompt, false, "");
}

export async function openPathInIDE(path: string): Promise<void> {
  await tauriInvoke<void>("open_path_in_ide", { path });
}

export async function listTerminalSessions(): Promise<TerminalSession[]> {
  const sessions = await tauriInvoke<TerminalSession[]>("list_terminal_sessions");
  if (sessions) return sessions;
  return mockTerminalSessions;
}

export async function createTerminalSession(cwd: string): Promise<TerminalSession> {
  const session = await tauriInvoke<TerminalSession>("create_terminal_session", { cwd });
  if (session) return session;

  const createdSession: TerminalSession = {
    id: nextMockID("term"),
    title: cwd.split("/").filter(Boolean).pop() || "terminal",
    project_path: "/Users/demo/projects/haft",
    cwd,
    shell: "/bin/zsh",
    status: "running",
    created_at: nowString(),
    updated_at: nowString(),
  };

  mockTerminalSessions = [...mockTerminalSessions, createdSession];
  return createdSession;
}

export async function writeTerminalInput(id: string, data: string): Promise<void> {
  await tauriInvoke<void>("write_terminal_input", { id, data });
  void data;
}

export async function resizeTerminalSession(id: string, cols: number, rows: number): Promise<void> {
  await tauriInvoke<void>("resize_terminal_session", { id, cols, rows });
}

export async function closeTerminalSession(id: string): Promise<void> {
  await tauriInvoke<void>("close_terminal_session", { id });
  mockTerminalSessions = mockTerminalSessions.filter((session) => session.id !== id);
}

export async function getConfig(): Promise<DesktopConfig> {
  const config = await tauriInvoke<DesktopConfig>("get_config");
  if (config) return config;

  return {
    default_agent: "claude",
    review_agent: "codex",
    verify_agent: "claude",
    agent_presets: [
      { name: "Implementation", agent_kind: "claude", model: "", role: "implementation" },
      { name: "Review", agent_kind: "codex", model: "", role: "review" },
      { name: "Verify", agent_kind: "claude", model: "", role: "verify" },
    ],
    task_timeout_minutes: 300,
    sound_enabled: true,
    notify_enabled: true,
    default_ide: "code",
    default_worktree: true,
    auto_wire_mcp: true,
  };
}

export async function saveConfig(config: DesktopConfig): Promise<DesktopConfig> {
  const saved = await tauriInvoke<DesktopConfig>("save_config", { config });
  if (saved) return saved;
  return config;
}
