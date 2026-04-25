use serde::{Deserialize, Serialize};
use std::collections::HashMap;

// View models — Rust equivalents of Go DTOs from desktop/views.go.
// All fields use the same JSON names as Go's json tags.
// Vecs serialize as [] (never null) — Rust's Vec<T> default is empty, matching Go safe* helpers.

// ─── Dashboard ───

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct DashboardView {
    pub project_name: String,
    pub problem_count: i64,
    pub decision_count: i64,
    pub portfolio_count: i64,
    pub note_count: i64,
    pub stale_count: i64,
    pub recent_problems: Vec<ProblemView>,
    pub recent_decisions: Vec<DecisionView>,
    pub healthy_decisions: Vec<DecisionView>,
    pub pending_decisions: Vec<DecisionView>,
    pub unassessed_decisions: Vec<DecisionView>,
    pub stale_items: Vec<ArtifactView>,
}

// ─── Artifact ───

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct ArtifactView {
    pub id: String,
    pub kind: String,
    pub title: String,
    pub status: String,
    pub mode: String,
    pub created_at: String,
    pub updated_at: String,
}

// ─── Problems ───

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct ProblemView {
    pub id: String,
    pub title: String,
    pub status: String,
    pub mode: String,
    pub signal: String,
    pub reversibility: String,
    pub constraints: Vec<String>,
    pub created_at: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct ProblemDetailView {
    pub id: String,
    pub title: String,
    pub status: String,
    pub mode: String,
    pub signal: String,
    pub constraints: Vec<String>,
    pub optimization_targets: Vec<String>,
    pub observation_indicators: Vec<String>,
    pub acceptance: String,
    pub blast_radius: String,
    pub reversibility: String,
    pub characterizations: Vec<CharacterizationView>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub latest_characterization: Option<CharacterizationView>,
    pub linked_portfolios: Vec<ArtifactView>,
    pub linked_decisions: Vec<ArtifactView>,
    pub body: String,
    pub created_at: String,
    pub updated_at: String,
}

// ─── Decisions ───

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct DecisionView {
    pub id: String,
    pub title: String,
    pub status: String,
    pub mode: String,
    pub selected_title: String,
    pub weakest_link: String,
    pub valid_until: String,
    pub created_at: String,
    pub implement_guard: DecisionImplementGuardView,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct DecisionImplementGuardView {
    pub blocked_reason: String,
    pub confirmation_messages: Vec<String>,
    pub warning_messages: Vec<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct DecisionDetailView {
    pub id: String,
    pub title: String,
    pub status: String,
    pub mode: String,
    pub problem_refs: Vec<String>,
    pub selected_title: String,
    pub why_selected: String,
    pub selection_policy: String,
    pub counterargument: String,
    pub weakest_link: String,
    pub why_not_others: Vec<RejectionView>,
    pub invariants: Vec<String>,
    pub pre_conditions: Vec<String>,
    pub post_conditions: Vec<String>,
    pub admissibility: Vec<String>,
    pub evidence_requirements: Vec<String>,
    pub refresh_triggers: Vec<String>,
    pub claims: Vec<ClaimView>,
    pub first_module_coverage: bool,
    pub affected_files: Vec<String>,
    pub coverage_modules: Vec<CoverageModuleView>,
    pub coverage_warnings: Vec<String>,
    pub rollback_triggers: Vec<String>,
    pub rollback_steps: Vec<String>,
    pub rollback_blast_radius: String,
    pub evidence: EvidenceSummaryView,
    pub valid_until: String,
    pub body: String,
    pub created_at: String,
    pub updated_at: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct RejectionView {
    pub variant: String,
    pub reason: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct ClaimView {
    pub id: String,
    pub claim: String,
    pub observable: String,
    pub threshold: String,
    pub status: String,
    pub verify_after: String,
}

// ─── Evidence ───

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct EvidenceItemView {
    pub id: String,
    #[serde(rename = "type")]
    pub evidence_type: String,
    pub content: String,
    pub verdict: String,
    pub formality_level: i64,
    pub congruence_level: i64,
    pub claim_refs: Vec<String>,
    pub valid_until: String,
    pub is_expired: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct EvidenceSummaryView {
    pub items: Vec<EvidenceItemView>,
    pub total_claims: i64,
    pub covered_claims: i64,
    pub coverage_gaps: Vec<String>,
}

// ─── Portfolios ───

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct PortfolioSummaryView {
    pub id: String,
    pub title: String,
    pub status: String,
    pub mode: String,
    pub problem_ref: String,
    pub has_comparison: bool,
    pub created_at: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct PortfolioDetailView {
    pub id: String,
    pub title: String,
    pub status: String,
    pub problem_ref: String,
    pub variants: Vec<VariantView>,
    pub comparison: Option<ComparisonView>,
    pub body: String,
    pub created_at: String,
    pub updated_at: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct VariantView {
    pub id: String,
    pub title: String,
    pub description: String,
    pub weakest_link: String,
    pub novelty_marker: String,
    pub stepping_stone: bool,
    pub strengths: Vec<String>,
    pub risks: Vec<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct ComparisonView {
    pub dimensions: Vec<String>,
    pub scores: HashMap<String, HashMap<String, String>>,
    pub non_dominated_set: Vec<String>,
    pub dominated_notes: Vec<DominatedNote>,
    pub pareto_tradeoffs: Vec<TradeoffNote>,
    pub policy_applied: String,
    pub selected_ref: String,
    pub recommendation: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct DominatedNote {
    pub variant: String,
    pub dominated_by: Vec<String>,
    pub summary: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct TradeoffNote {
    pub variant: String,
    pub summary: String,
}

// ─── Characterization ───

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct CharacterizationView {
    pub version: i64,
    pub dimensions: Vec<DimensionView>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub parity_plan: Option<ParityPlanView>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct DimensionView {
    pub name: String,
    pub scale_type: String,
    pub unit: String,
    pub polarity: String,
    pub role: String,
    pub how_to_measure: String,
    pub valid_until: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct ParityPlanView {
    pub baseline_set: Vec<String>,
    pub window: String,
    pub budget: String,
    pub normalization: Vec<NormRuleView>,
    pub missing_data_policy: String,
    pub pinned_conditions: Vec<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct NormRuleView {
    pub dimension: String,
    pub method: String,
}

// ─── Coverage ───

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct CoverageModuleView {
    pub id: String,
    pub path: String,
    pub name: String,
    pub lang: String,
    pub status: String,
    pub decision_count: i64,
    pub decision_ids: Vec<String>,
    pub impacted: bool,
    pub files: Vec<String>,
}

// ─── Tasks ───

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct TaskState {
    pub id: String,
    pub title: String,
    pub agent: String,
    pub project: String,
    pub project_path: String,
    pub status: String,
    pub prompt: String,
    pub branch: String,
    pub worktree: bool,
    pub worktree_path: String,
    pub reused_worktree: bool,
    pub started_at: String,
    pub completed_at: String,
    pub error_message: String,
    pub output: String,
    pub chat_blocks: Vec<ChatBlock>,
    pub raw_output: String,
    pub auto_run: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct ChatBlock {
    pub id: String,
    #[serde(rename = "type")]
    pub block_type: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub role: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub text: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub name: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub call_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub parent_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub input: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub output: String,
    #[serde(default, skip_serializing_if = "is_false")]
    pub is_error: bool,
}

fn is_false(v: &bool) -> bool {
    !v
}

// ─── Flows ───

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct FlowView {
    pub id: String,
    pub project_name: String,
    pub project_path: String,
    pub title: String,
    pub description: String,
    pub template_id: String,
    pub agent: String,
    pub prompt: String,
    pub schedule: String,
    pub branch: String,
    pub use_worktree: bool,
    pub enabled: bool,
    pub last_task_id: String,
    pub last_run_at: String,
    pub next_run_at: String,
    pub last_error: String,
    pub created_at: String,
    pub updated_at: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct FlowTemplateView {
    pub id: String,
    pub name: String,
    pub description: String,
    pub agent: String,
    pub schedule: String,
    pub prompt: String,
    pub branch: String,
    pub use_worktree: bool,
}

// ─── Projects / Config ───
//
// Mirror the TypeScript types in `desktop/frontend/src/lib/api.ts`. The
// frontend deserializes Tauri responses via these names; changing a field
// name here silently breaks the UI because Tauri sends JSON with serde's
// default snake_case encoding.

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct ProjectInfo {
    pub path: String,
    pub name: String,
    pub id: String,
    pub status: String,
    pub exists: bool,
    pub has_haft: bool,
    pub has_specs: bool,
    pub readiness_source: String,
    pub readiness_error: String,
    pub is_active: bool,
    pub problem_count: i64,
    pub decision_count: i64,
    pub stale_count: i64,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct AgentPreset {
    pub name: String,
    pub agent_kind: String,
    pub model: String,
    pub role: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DesktopConfig {
    pub default_agent: String,
    pub review_agent: String,
    pub verify_agent: String,
    pub agent_presets: Vec<AgentPreset>,
    pub task_timeout_minutes: i64,
    pub sound_enabled: bool,
    pub notify_enabled: bool,
    pub default_ide: String,
    pub default_worktree: bool,
    pub auto_wire_mcp: bool,
}

impl Default for DesktopConfig {
    fn default() -> Self {
        Self {
            default_agent: "claude".into(),
            review_agent: "claude".into(),
            verify_agent: "claude".into(),
            agent_presets: Vec::new(),
            task_timeout_minutes: 30,
            sound_enabled: true,
            notify_enabled: true,
            default_ide: "cursor".into(),
            default_worktree: true,
            auto_wire_mcp: true,
        }
    }
}
