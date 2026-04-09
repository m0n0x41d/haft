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
  rollback_triggers: string[];
  rollback_steps: string[];
  rollback_blast_radius: string;
  valid_until: string;
  created_at: string;
  updated_at: string;
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
}

type WailsBindings = {
  GetDashboard: () => Promise<DashboardData>;
  ListProblems: () => Promise<ProblemSummary[]>;
  ListDecisions: () => Promise<DecisionSummary[]>;
  GetProblem: (id: string) => Promise<ProblemDetail>;
  GetDecision: (id: string) => Promise<DecisionDetail>;
  GetPortfolio: (id: string) => Promise<PortfolioDetail>;
  SearchArtifacts: (query: string) => Promise<ArtifactSummary[]>;
  ListProjects: () => Promise<ProjectInfo[]>;
  AddProject: (path: string) => Promise<ProjectInfo>;
  SwitchProject: (path: string) => Promise<void>;
  ScanForProjects: () => Promise<ProjectInfo[]>;
  DetectAgents?: () => Promise<InstalledAgent[]>;
  ListTasks?: () => Promise<TaskState[]>;
  SpawnTask?: (agent: string, prompt: string, worktree: boolean, branch: string) => Promise<TaskState>;
  CancelTask?: (id: string) => Promise<void>;
  ArchiveTask?: (id: string) => Promise<void>;
  GetTaskOutput?: (id: string) => Promise<string>;
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

const MOCK_PROBLEMS: ProblemSummary[] = [
  {
    id: "prob-20260409-001",
    title: "AIEE product shape",
    status: "active",
    mode: "standard",
    signal: "Haft is CLI-only. Market moving to visual agent governance surfaces.",
    reversibility: "medium",
    constraints: ["Solo developer", "Go backend must stay", "Local-first"],
    created_at: "2026-04-09",
  },
];

const MOCK_DECISIONS: DecisionSummary[] = [
  {
    id: "dec-20260409-001",
    title: "Reasoning Workspace — Wails native, interactive",
    status: "active",
    mode: "standard",
    selected_title: "Reasoning Workspace — Wails native",
    weakest_link: "Wails v2 WebView maturity",
    valid_until: "2026-07-09",
    created_at: "2026-04-09",
  },
];

// --- Public API ---

export async function getDashboard(): Promise<DashboardData> {
  const b = await loadBindings();
  if (b) return b.GetDashboard();

  return {
    project_name: "haft",
    problem_count: MOCK_PROBLEMS.length,
    decision_count: MOCK_DECISIONS.length,
    portfolio_count: 1,
    note_count: 5,
    stale_count: 0,
    recent_problems: MOCK_PROBLEMS,
    recent_decisions: MOCK_DECISIONS,
    stale_items: [],
  };
}

export async function listProblems(): Promise<ProblemSummary[]> {
  const b = await loadBindings();
  if (b) return b.ListProblems();
  return MOCK_PROBLEMS;
}

export async function listDecisions(): Promise<DecisionSummary[]> {
  const b = await loadBindings();
  if (b) return b.ListDecisions();
  return MOCK_DECISIONS;
}

export async function getProblem(id: string): Promise<ProblemDetail> {
  const b = await loadBindings();
  if (b) return b.GetProblem(id);

  return {
    id,
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
}

export async function getDecision(id: string): Promise<DecisionDetail> {
  const b = await loadBindings();
  if (b) return b.GetDecision(id);

  return {
    id,
    title: "Reasoning Workspace — Wails native, interactive",
    status: "active",
    mode: "standard",
    selected_title: "Reasoning Workspace — Wails native",
    why_selected:
      "Desktop-native from day 1. Single binary distribution, real product identity.",
    selection_policy:
      "Minimize regret under solo-dev constraints.",
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
    valid_until: "2026-07-09",
    created_at: "2026-04-09T00:00:00Z",
    updated_at: "2026-04-09T00:00:00Z",
  };
}

export async function getPortfolio(id: string): Promise<PortfolioDetail> {
  const b = await loadBindings();
  if (b) return b.GetPortfolio(id);

  return {
    id,
    title: "Solutions for AIEE product shape",
    status: "active",
    problem_ref: "prob-20260409-001",
    variants: [],
    comparison: null,
    body: "",
    created_at: "2026-04-09T00:00:00Z",
    updated_at: "2026-04-09T00:00:00Z",
  };
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

// --- Task management ---

export async function listTasks(): Promise<TaskState[]> {
  const tasks = await callBinding<TaskState[]>("ListTasks");
  if (tasks) return tasks;
  return [];
}

export async function detectAgents(): Promise<InstalledAgent[]> {
  const agents = await callBinding<InstalledAgent[]>("DetectAgents");
  return agents ?? [];
}

export async function spawnTask(agent: string, prompt: string, worktree: boolean, branch: string): Promise<TaskState> {
  const task = await callBinding<TaskState>("SpawnTask", agent, prompt, worktree, branch);
  if (task) return task;

  return {
    id: `mock-${Date.now()}`,
    title: prompt.slice(0, 60),
    agent,
    project: "haft",
    project_path: "/Users/demo/projects/haft",
    status: "running",
    prompt,
    branch,
    worktree,
    worktree_path: worktree ? `/Users/demo/projects/haft/.haft/worktrees/${branch}` : "",
    reused_worktree: false,
    started_at: new Date().toISOString(),
    completed_at: "",
    error_message: "",
    output: "",
  };
}

export async function cancelTask(id: string): Promise<void> {
  await callBinding<void>("CancelTask", id);
}

export async function archiveTask(id: string): Promise<void> {
  await callBinding<void>("ArchiveTask", id);
}

export async function getTaskOutput(id: string): Promise<string> {
  const output = await callBinding<string>("GetTaskOutput", id);
  return output ?? "";
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
