// API layer — wraps Wails Go bindings with mock fallback for standalone dev.
// After `wails dev` generates bindings at wailsjs/go/main/App, this module
// imports them. When running standalone (npm run dev), it uses mock data.

export interface DashboardData {
  project_name: string;
  problem_count: number;
  decision_count: number;
  portfolio_count: number;
  note_count: number;
  stale_count: number;
  recent_problems: ProblemSummary[];
  recent_decisions: DecisionSummary[];
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
  is_active: boolean;
  problem_count: number;
  decision_count: number;
  stale_count: number;
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
  auto_run: boolean;
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

type WailsBindings = {
  GetDashboard: () => Promise<DashboardData>;
  GetCoverage?: () => Promise<CoverageData>;
  GetGovernanceOverview?: () => Promise<GovernanceOverview>;
  RefreshGovernance?: () => Promise<GovernanceOverview>;
  ListProblemCandidates?: () => Promise<ProblemCandidate[]>;
  DismissProblemCandidate?: (id: string) => Promise<void>;
  AdoptProblemCandidate?: (id: string) => Promise<ProblemDetail>;
  ListProblems: () => Promise<ProblemSummary[]>;
  ListDecisions: () => Promise<DecisionSummary[]>;
  GetProblem: (id: string) => Promise<ProblemDetail>;
  GetDecision: (id: string) => Promise<DecisionDetail>;
  ListPortfolios: () => Promise<PortfolioSummary[]>;
  GetPortfolio: (id: string) => Promise<PortfolioDetail>;
  CreateProblem?: (input: ProblemCreateInput) => Promise<ProblemDetail>;
  CharacterizeProblem?: (input: ProblemCharacterizationInput) => Promise<ProblemDetail>;
  CreatePortfolio?: (input: PortfolioCreateInput) => Promise<PortfolioDetail>;
  ComparePortfolio?: (input: PortfolioCompareInput) => Promise<PortfolioDetail>;
  CreateDecision?: (input: DecisionCreateInput) => Promise<DecisionDetail>;
  SearchArtifacts: (query: string) => Promise<ArtifactSummary[]>;
  ListProjects: () => Promise<ProjectInfo[]>;
  AddProject: (path: string) => Promise<ProjectInfo>;
  SwitchProject: (path: string) => Promise<void>;
  ScanForProjects: () => Promise<ProjectInfo[]>;
  OpenDirectoryPicker?: () => Promise<string>;
  InitProject?: (path: string) => Promise<ProjectInfo>;
  DetectAgents?: () => Promise<InstalledAgent[]>;
  ListTasks?: () => Promise<TaskState[]>;
  SpawnTask?: (agent: string, prompt: string, worktree: boolean, branch: string) => Promise<TaskState>;
  CancelTask?: (id: string) => Promise<void>;
  ArchiveTask?: (id: string) => Promise<void>;
  GetTaskOutput?: (id: string) => Promise<string>;
  ListAllTasks?: () => Promise<TaskState[]>;
  HandoffTask?: (id: string, agent: string) => Promise<TaskState>;
  ListFlows?: () => Promise<DesktopFlow[]>;
  ListFlowTemplates?: () => Promise<FlowTemplate[]>;
  CreateFlow?: (input: FlowInput) => Promise<DesktopFlow>;
  UpdateFlow?: (input: FlowInput) => Promise<DesktopFlow>;
  ToggleFlow?: (id: string, enabled: boolean) => Promise<DesktopFlow>;
  DeleteFlow?: (id: string) => Promise<void>;
  RunFlowNow?: (id: string) => Promise<TaskState>;
  ListTerminalSessions?: () => Promise<TerminalSession[]>;
  CreateTerminalSession?: (cwd: string) => Promise<TerminalSession>;
  WriteTerminalInput?: (id: string, data: string) => Promise<void>;
  ResizeTerminalSession?: (id: string, cols: number, rows: number) => Promise<void>;
  CloseTerminalSession?: (id: string) => Promise<void>;
  ImplementDecision?: (
    decisionID: string,
    agent: string,
    worktree: boolean,
    branch: string,
  ) => Promise<TaskState>;
  VerifyDecision?: (decisionID: string, agent: string) => Promise<TaskState>;
  OpenPathInIDE?: (path: string) => Promise<void>;
  GetConfig?: () => Promise<DesktopConfig>;
  SaveConfig?: (config: DesktopConfig) => Promise<DesktopConfig>;
};

let bindings: WailsBindings | null = null;

async function loadBindings(): Promise<WailsBindings | null> {
  if (bindings) return bindings;
  try {
    // Use a variable path so TypeScript doesn't try to resolve the module at build time.
    // Wails generates these bindings at runtime via `wails dev`.
    const bindingPath = "../../wailsjs/go/main/App";
    const mod = await import(/* @vite-ignore */ bindingPath);
    bindings = mod as unknown as WailsBindings;
    return bindings;
  } catch {
    return null;
  }
}

async function callBinding<T>(name: string, ...args: unknown[]): Promise<T | null> {
  const b = await loadBindings();
  const fn = b ? (b as unknown as Record<string, (...params: unknown[]) => Promise<T>>)[name] : null;

  if (typeof fn !== "function") {
    return null;
  }

  return fn(...args);
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
    "Differentiation from Zenflow/Cursor/Codex",
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
    output: "Inspecting the current desktop runtime...\nAdding flow scheduler bindings...",
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

let mockTerminalSessions: TerminalSession[] = [];

function nextMockID(prefix: "prob" | "sol" | "dec" | "flow" | "task" | "term"): string {
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
  const b = await loadBindings();
  if (b) return b.GetDashboard();

  return {
    project_name: "haft",
    problem_count: mockProblems.length,
    decision_count: mockDecisions.length,
    portfolio_count: mockPortfolios.length,
    note_count: 5,
    stale_count: mockGovernanceOverview.findings.length,
    recent_problems: mockProblems,
    recent_decisions: mockDecisions,
    stale_items: mockGovernanceOverview.findings.map((finding) => ({
      id: finding.artifact_ref,
      kind: finding.kind,
      title: finding.title,
      status: finding.category,
    })),
  };
}

export async function listProblems(): Promise<ProblemSummary[]> {
  const b = await loadBindings();
  if (b) return b.ListProblems();
  return mockProblems;
}

export async function listDecisions(): Promise<DecisionSummary[]> {
  const b = await loadBindings();
  if (b) return b.ListDecisions();
  return mockDecisions;
}

export async function getProblem(id: string): Promise<ProblemDetail> {
  const b = await loadBindings();
  if (b) return b.GetProblem(id);
  return mockProblemDetails.get(id) ?? INITIAL_PROBLEM_DETAIL;
}

export async function getDecision(id: string): Promise<DecisionDetail> {
  const b = await loadBindings();
  if (b) return b.GetDecision(id);
  return mockDecisionDetails.get(id) ?? INITIAL_DECISION_DETAIL;
}

export async function listPortfolios(): Promise<PortfolioSummary[]> {
  const b = await loadBindings();
  if (b) return b.ListPortfolios();
  return mockPortfolios;
}

export async function getPortfolio(id: string): Promise<PortfolioDetail> {
  const b = await loadBindings();
  if (b) return b.GetPortfolio(id);
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
  const report = await callBinding<ReadinessReport>("AssessComparisonReadiness", portfolioID);
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
  const coverage = await callBinding<CoverageData>("GetCoverage");
  if (coverage) return coverage;
  return mockGovernanceOverview.coverage;
}

export async function getGovernanceOverview(): Promise<GovernanceOverview> {
  const overview = await callBinding<GovernanceOverview>("GetGovernanceOverview");
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
  const overview = await callBinding<GovernanceOverview>("RefreshGovernance");
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
  const candidates = await callBinding<ProblemCandidate[]>("ListProblemCandidates");
  if (candidates) return candidates;
  return mockGovernanceOverview.problem_candidates;
}

export async function dismissProblemCandidate(id: string): Promise<void> {
  await callBinding<void>("DismissProblemCandidate", id);
  mockGovernanceOverview = {
    ...mockGovernanceOverview,
    problem_candidates: mockGovernanceOverview.problem_candidates.filter((candidate) => candidate.id !== id),
  };
}

export async function adoptProblemCandidate(id: string): Promise<ProblemDetail> {
  const adopted = await callBinding<ProblemDetail>("AdoptProblemCandidate", id);
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

export async function createProblem(input: ProblemCreateInput): Promise<ProblemDetail> {
  const problem = await callBinding<ProblemDetail>("CreateProblem", input);
  if (problem) return problem;

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
  const problem = await callBinding<ProblemDetail>("CharacterizeProblem", input);
  if (problem) return problem;

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
  const portfolio = await callBinding<PortfolioDetail>("CreatePortfolio", input);
  if (portfolio) return portfolio;

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
  const portfolio = await callBinding<PortfolioDetail>("ComparePortfolio", input);
  if (portfolio) return portfolio;

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
  const decision = await callBinding<DecisionDetail>("CreateDecision", input);
  if (decision) return decision;

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
  const b = await loadBindings();
  if (b) return b.SearchArtifacts(query);
  return [];
}

// --- Project management ---

export async function listProjects(): Promise<ProjectInfo[]> {
  const b = await loadBindings();
  if (b) return b.ListProjects();
  return [
    {
      path: "/Users/demo/projects/haft",
      name: "haft",
      id: "qnt_demo1",
      is_active: true,
      problem_count: 12,
      decision_count: 8,
      stale_count: 2,
    },
  ];
}

export async function addProject(path: string): Promise<ProjectInfo> {
  const b = await loadBindings();
  if (b) return b.AddProject(path);
  return { path, name: path.split("/").pop() || path, id: "", is_active: false, problem_count: 0, decision_count: 0, stale_count: 0 };
}

export async function switchProject(path: string): Promise<void> {
  const b = await loadBindings();
  if (b) return b.SwitchProject(path);
}

export async function scanForProjects(): Promise<ProjectInfo[]> {
  const b = await loadBindings();
  if (b) return b.ScanForProjects();
  return [];
}

export async function openDirectoryPicker(): Promise<string> {
  const path = await callBinding<string>("OpenDirectoryPicker");
  return path ?? "";
}

export async function initProject(path: string): Promise<ProjectInfo> {
  const project = await callBinding<ProjectInfo>("InitProject", path);
  if (project) return project;
  return {
    path,
    name: path.split("/").pop() || path,
    id: "",
    is_active: false,
    problem_count: 0,
    decision_count: 0,
    stale_count: 0,
  };
}

// --- Task management ---

export async function listTasks(): Promise<TaskState[]> {
  const tasks = await callBinding<TaskState[]>("ListTasks");
  if (tasks) return tasks;
  return mockTasks;
}

export async function listAllTasks(): Promise<TaskState[]> {
  const tasks = await callBinding<TaskState[]>("ListAllTasks");
  if (tasks) return tasks;
  return mockTasks;
}

export async function detectAgents(): Promise<InstalledAgent[]> {
  const agents = await callBinding<InstalledAgent[]>("DetectAgents");
  return agents ?? [];
}

export async function spawnTask(agent: string, prompt: string, worktree: boolean, branch: string): Promise<TaskState> {
  const task = await callBinding<TaskState>("SpawnTask", agent, prompt, worktree, branch);
  if (task) return task;

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
    output: "",
  };

  mockTasks = [createdTask, ...mockTasks];
  return createdTask;
}

export async function cancelTask(id: string): Promise<void> {
  await callBinding<void>("CancelTask", id);
  mockTasks = mockTasks.map((task) =>
    task.id === id
      ? { ...task, status: "cancelled", completed_at: nowString() }
      : task,
  );
}

export async function archiveTask(id: string): Promise<void> {
  await callBinding<void>("ArchiveTask", id);
  mockTasks = mockTasks.filter((task) => task.id !== id);
}

export async function setTaskAutoRun(id: string, autoRun: boolean): Promise<void> {
  await callBinding<void>("SetTaskAutoRun", id, autoRun);
}

export async function getTaskOutput(id: string): Promise<string> {
  const output = await callBinding<string>("GetTaskOutput", id);
  if (output != null) return output; // null check, not falsy — empty string "" is valid
  return mockTasks.find((task) => task.id === id)?.output ?? "";
}

export async function handoffTask(id: string, agent: string): Promise<TaskState> {
  const task = await callBinding<TaskState>("HandoffTask", id, agent);
  if (task) return task;

  const source = mockTasks.find((item) => item.id === id);
  if (!source) {
    throw new Error(`Task ${id} not found`);
  }

  return spawnTask(agent, `Handoff for ${source.title}\n\n${source.prompt}`, source.worktree, source.branch);
}

export async function listFlows(): Promise<DesktopFlow[]> {
  const flows = await callBinding<DesktopFlow[]>("ListFlows");
  if (flows) return flows;
  return mockFlows;
}

export async function listFlowTemplates(): Promise<FlowTemplate[]> {
  const templates = await callBinding<FlowTemplate[]>("ListFlowTemplates");
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
      agent: "haft",
      schedule: "0 15 * * 1",
      prompt: "Summarize module governance coverage for the current project.",
      branch: "flows/coverage-report",
      use_worktree: false,
    },
  ];
}

export async function createFlow(input: FlowInput): Promise<DesktopFlow> {
  const flow = await callBinding<DesktopFlow>("CreateFlow", input);
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
  const flow = await callBinding<DesktopFlow>("UpdateFlow", input);
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
  const flow = await callBinding<DesktopFlow>("ToggleFlow", id, enabled);
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
  await callBinding<void>("DeleteFlow", id);
  mockFlows = mockFlows.filter((flow) => flow.id !== id);
}

export async function runFlowNow(id: string): Promise<TaskState> {
  const task = await callBinding<TaskState>("RunFlowNow", id);
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
  const task = await callBinding<TaskState>(
    "ImplementDecision",
    decisionID,
    agent,
    worktree,
    branch,
  );
  if (task) return task;
  return spawnTask(agent, `Implement ${decisionID}`, worktree, branch);
}

export async function verifyDecision(decisionID: string, agent: string): Promise<TaskState> {
  const task = await callBinding<TaskState>("VerifyDecision", decisionID, agent);
  if (task) return task;
  return spawnTask(agent, `Verify ${decisionID}`, false, "");
}

export async function openPathInIDE(path: string): Promise<void> {
  await callBinding<void>("OpenPathInIDE", path);
}

export async function listTerminalSessions(): Promise<TerminalSession[]> {
  const sessions = await callBinding<TerminalSession[]>("ListTerminalSessions");
  if (sessions) return sessions;
  return mockTerminalSessions;
}

export async function createTerminalSession(cwd: string): Promise<TerminalSession> {
  const session = await callBinding<TerminalSession>("CreateTerminalSession", cwd);
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
  await callBinding<void>("WriteTerminalInput", id, data);
  void data;
}

export async function resizeTerminalSession(id: string, cols: number, rows: number): Promise<void> {
  await callBinding<void>("ResizeTerminalSession", id, cols, rows);
}

export async function closeTerminalSession(id: string): Promise<void> {
  await callBinding<void>("CloseTerminalSession", id);
  mockTerminalSessions = mockTerminalSessions.filter((session) => session.id !== id);
}

export async function getConfig(): Promise<DesktopConfig> {
  const config = await callBinding<DesktopConfig>("GetConfig");
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
  const saved = await callBinding<DesktopConfig>("SaveConfig", config);
  if (saved) return saved;
  return config;
}
