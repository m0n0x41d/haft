use std::collections::{HashMap, HashSet};
use std::time::{SystemTime, UNIX_EPOCH};

use rusqlite::{Connection, OpenFlags, params};
use serde::Deserialize;

use crate::models::*;

fn now_unix_ts() -> String {
    let secs = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|d| d.as_secs())
        .unwrap_or(0);
    format!("t{secs}")
}

// ─── Artifact kind constants (match Go's artifact.Kind values) ───

const KIND_PROBLEM_CARD: &str = "ProblemCard";
const KIND_DECISION_RECORD: &str = "DecisionRecord";
const KIND_SOLUTION_PORTFOLIO: &str = "SolutionPortfolio";
const KIND_NOTE: &str = "Note";

// ─── Internal types for structured_data JSON deserialization ───

#[derive(Debug, Deserialize, Default)]
struct ProblemFields {
    #[serde(default)]
    signal: String,
    #[serde(default)]
    constraints: Vec<String>,
    #[serde(default)]
    optimization_targets: Vec<String>,
    #[serde(default)]
    observation_indicators: Vec<String>,
    #[serde(default)]
    acceptance: String,
    #[serde(default)]
    blast_radius: String,
    #[serde(default)]
    reversibility: String,
    #[serde(default)]
    characterizations: Vec<CharacterizationData>,
}

#[derive(Debug, Deserialize, Default)]
struct CharacterizationData {
    #[serde(default)]
    version: i64,
    #[serde(default)]
    dimensions: Vec<DimensionData>,
    #[serde(default)]
    parity_plan: Option<ParityPlanData>,
}

#[derive(Debug, Deserialize, Default)]
struct DimensionData {
    #[serde(default)]
    name: String,
    #[serde(default)]
    scale_type: String,
    #[serde(default)]
    unit: String,
    #[serde(default)]
    polarity: String,
    #[serde(default)]
    role: String,
    #[serde(default)]
    how_to_measure: String,
    #[serde(default)]
    valid_until: String,
}

#[derive(Debug, Deserialize, Default)]
struct ParityPlanData {
    #[serde(default)]
    baseline_set: Vec<String>,
    #[serde(default)]
    window: String,
    #[serde(default)]
    budget: String,
    #[serde(default)]
    normalization: Vec<NormRuleData>,
    #[serde(default)]
    missing_data_policy: String,
    #[serde(default)]
    pinned_conditions: Vec<String>,
}

#[derive(Debug, Deserialize, Default)]
struct NormRuleData {
    #[serde(default)]
    dimension: String,
    #[serde(default)]
    method: String,
}

#[derive(Debug, Deserialize, Default)]
struct DecisionFields {
    #[serde(default)]
    problem_refs: Vec<String>,
    #[serde(default)]
    selected_title: String,
    #[serde(default)]
    why_selected: String,
    #[serde(default)]
    selection_policy: String,
    #[serde(default)]
    counterargument: String,
    #[serde(default)]
    weakest_link: String,
    #[serde(default)]
    why_not_others: Vec<RejectionData>,
    #[serde(default)]
    invariants: Vec<String>,
    #[serde(default)]
    pre_conditions: Vec<String>,
    #[serde(default)]
    post_conditions: Vec<String>,
    #[serde(default)]
    admissibility: Vec<String>,
    #[serde(default)]
    evidence_requirements: Vec<String>,
    #[serde(default)]
    refresh_triggers: Vec<String>,
    #[serde(default)]
    claims: Vec<ClaimData>,
    #[serde(default)]
    first_module_coverage: bool,
    #[serde(default)]
    rollback_triggers: Vec<String>,
    #[serde(default)]
    rollback_steps: Vec<String>,
    #[serde(default)]
    rollback_blast_radius: String,
}

#[derive(Debug, Deserialize, Default)]
struct RejectionData {
    #[serde(default)]
    variant: String,
    #[serde(default)]
    reason: String,
}

#[derive(Debug, Deserialize, Default)]
struct ClaimData {
    #[serde(default)]
    id: String,
    #[serde(default)]
    claim: String,
    #[serde(default)]
    observable: String,
    #[serde(default)]
    threshold: String,
    #[serde(default)]
    status: String,
    #[serde(default)]
    verify_after: String,
}

#[derive(Debug, Deserialize, Default)]
struct PortfolioFields {
    #[serde(default)]
    problem_ref: String,
    #[serde(default)]
    variants: Vec<VariantData>,
    #[serde(default)]
    comparison: Option<ComparisonData>,
}

#[derive(Debug, Deserialize, Default)]
struct VariantData {
    #[serde(default)]
    id: String,
    #[serde(default)]
    title: String,
    #[serde(default)]
    description: String,
    #[serde(default)]
    weakest_link: String,
    #[serde(default)]
    novelty_marker: String,
    #[serde(default)]
    stepping_stone: bool,
    #[serde(default)]
    strengths: Vec<String>,
    #[serde(default)]
    risks: Vec<String>,
}

#[derive(Debug, Deserialize, Default)]
struct ComparisonData {
    #[serde(default)]
    dimensions: Vec<String>,
    #[serde(default)]
    scores: HashMap<String, HashMap<String, String>>,
    #[serde(default)]
    non_dominated_set: Vec<String>,
    #[serde(default)]
    dominated_variants: Vec<DominatedVariantData>,
    #[serde(default)]
    pareto_tradeoffs: Vec<TradeoffData>,
    #[serde(default)]
    policy_applied: String,
    #[serde(default)]
    selected_ref: String,
    // Go stores "recommendation_rationale", view exposes "recommendation"
    #[serde(default)]
    recommendation_rationale: String,
}

#[derive(Debug, Deserialize, Default)]
struct DominatedVariantData {
    #[serde(default)]
    variant: String,
    #[serde(default)]
    dominated_by: Vec<String>,
    #[serde(default)]
    summary: String,
}

#[derive(Debug, Deserialize, Default)]
struct TradeoffData {
    #[serde(default)]
    variant: String,
    #[serde(default)]
    summary: String,
}

// ─── HaftDb ───

pub struct HaftDb {
    conn: Connection,
}

/// Error type wrapping rusqlite::Error.
pub type Result<T> = std::result::Result<T, rusqlite::Error>;

impl HaftDb {
    /// Open haft.db read-write with WAL mode and busy_timeout=5000ms.
    /// The Go writer still runs concurrently; SQLite's WAL + busy_timeout
    /// coordinates the two. A handful of desktop-side Tauri commands
    /// (`archive_task`, `set_task_auto_run`, flow persistence) issue UPDATEs
    /// directly against this handle, so read-only would reject them.
    pub fn open(path: &str) -> Result<Self> {
        let flags = OpenFlags::SQLITE_OPEN_READ_WRITE | OpenFlags::SQLITE_OPEN_NO_MUTEX;
        let conn = Connection::open_with_flags(path, flags)?;
        conn.pragma_update(None, "busy_timeout", 5000)?;
        let _: std::result::Result<String, _> =
            conn.query_row("PRAGMA journal_mode=wal", [], |row| row.get(0));
        Ok(Self { conn })
    }

    // ─── Dashboard ───

    pub fn get_dashboard(&self, project_name: &str) -> Result<DashboardView> {
        let problems = self.list_problems()?;
        let decisions = self.list_decisions()?;
        let portfolios = self.list_portfolios()?;
        let note_count = self.count_active_by_kind(KIND_NOTE)?;
        let stale_items = self.find_stale_artifacts()?;

        // Categorize decisions by health (approximation of Go's DeriveDecisionHealth).
        let evidence_map = self.evidence_summary_by_decision()?;

        let mut healthy = Vec::new();
        let mut pending = Vec::new();
        let mut unassessed = Vec::new();

        for d in &decisions {
            if d.status != "active" {
                continue;
            }
            match evidence_map.get(&d.id) {
                None => unassessed.push(d.clone()),
                Some((active, measurement)) if *active > 0 && *measurement > 0 => {
                    healthy.push(d.clone());
                }
                Some((active, _)) if *active > 0 => pending.push(d.clone()),
                _ => unassessed.push(d.clone()),
            }
        }

        Ok(DashboardView {
            project_name: project_name.to_string(),
            problem_count: problems.len() as i64,
            decision_count: decisions.len() as i64,
            portfolio_count: portfolios.len() as i64,
            note_count,
            stale_count: stale_items.len() as i64,
            recent_problems: problems.into_iter().take(8).collect(),
            recent_decisions: decisions.into_iter().take(8).collect(),
            healthy_decisions: truncate_vec(healthy, 8),
            pending_decisions: truncate_vec(pending, 8),
            unassessed_decisions: truncate_vec(unassessed, 8),
            stale_items,
        })
    }

    // ─── Problems ───

    pub fn list_problems(&self) -> Result<Vec<ProblemView>> {
        let mut stmt = self.conn.prepare(
            "SELECT id, kind, version, status, context, mode, title, content,
                    valid_until, created_at, updated_at, COALESCE(structured_data, '')
             FROM artifacts
             WHERE kind = ?1 AND status = 'active'
             ORDER BY created_at DESC LIMIT 200",
        )?;

        let rows = stmt.query_map(params![KIND_PROBLEM_CARD], |row| {
            let sd: String = row.get(11)?;
            let f: ProblemFields = parse_json(&sd);
            Ok(ProblemView {
                id: row.get(0)?,
                title: row.get(6)?,
                status: row.get(3)?,
                mode: col_str(row, 5),
                signal: f.signal,
                reversibility: f.reversibility,
                constraints: f.constraints,
                created_at: truncate_date(&col_str(row, 9)),
            })
        })?;

        rows.collect()
    }

    pub fn get_problem(&self, id: &str) -> Result<ProblemDetailView> {
        let mut stmt = self.conn.prepare(
            "SELECT id, kind, version, status, context, mode, title, content,
                    valid_until, created_at, updated_at, COALESCE(structured_data, '')
             FROM artifacts WHERE id = ?1",
        )?;

        let (art_id, status, mode, title, body, created_at, updated_at, sd) =
            stmt.query_row(params![id], |row| {
                Ok((
                    row.get::<_, String>(0)?,
                    row.get::<_, String>(3)?,
                    col_str(row, 5),
                    row.get::<_, String>(6)?,
                    row.get::<_, String>(7)?,
                    col_str(row, 9),
                    col_str(row, 10),
                    row.get::<_, String>(11)?,
                ))
            })?;

        let f: ProblemFields = parse_json(&sd);

        // Backlinks: artifacts that reference this problem.
        let (linked_portfolios, linked_decisions) = self.get_problem_links(&art_id)?;

        let characterizations = f
            .characterizations
            .iter()
            .map(to_characterization_view)
            .collect::<Vec<_>>();
        let latest_characterization = f.characterizations.last().map(to_characterization_view);

        Ok(ProblemDetailView {
            id: art_id,
            title,
            status,
            mode,
            signal: f.signal,
            constraints: f.constraints,
            optimization_targets: f.optimization_targets,
            observation_indicators: f.observation_indicators,
            acceptance: f.acceptance,
            blast_radius: f.blast_radius,
            reversibility: f.reversibility,
            characterizations,
            latest_characterization,
            linked_portfolios,
            linked_decisions,
            body,
            created_at,
            updated_at,
        })
    }

    // ─── Decisions ───

    pub fn list_decisions(&self) -> Result<Vec<DecisionView>> {
        let mut stmt = self.conn.prepare(
            "SELECT id, kind, version, status, context, mode, title, content,
                    valid_until, created_at, updated_at, COALESCE(structured_data, '')
             FROM artifacts
             WHERE kind = ?1 AND status = 'active'
             ORDER BY created_at DESC LIMIT 200",
        )?;

        let rows = stmt.query_map(params![KIND_DECISION_RECORD], |row| {
            let sd: String = row.get(11)?;
            let f: DecisionFields = parse_json(&sd);
            Ok(DecisionView {
                id: row.get(0)?,
                title: row.get(6)?,
                status: row.get(3)?,
                mode: col_str(row, 5),
                selected_title: f.selected_title,
                weakest_link: f.weakest_link,
                valid_until: col_str(row, 8),
                created_at: truncate_date(&col_str(row, 9)),
                implement_guard: DecisionImplementGuardView::default(),
            })
        })?;

        rows.collect()
    }

    pub fn get_decision(&self, id: &str) -> Result<DecisionDetailView> {
        let mut stmt = self.conn.prepare(
            "SELECT id, kind, version, status, context, mode, title, content,
                    valid_until, created_at, updated_at, COALESCE(structured_data, '')
             FROM artifacts WHERE id = ?1",
        )?;

        let (art_id, status, mode, title, body, valid_until, created_at, updated_at, sd) = stmt
            .query_row(params![id], |row| {
                Ok((
                    row.get::<_, String>(0)?,
                    row.get::<_, String>(3)?,
                    col_str(row, 5),
                    row.get::<_, String>(6)?,
                    row.get::<_, String>(7)?,
                    col_str(row, 8),
                    col_str(row, 9),
                    col_str(row, 10),
                    row.get::<_, String>(11)?,
                ))
            })?;

        let f: DecisionFields = parse_json(&sd);

        let affected_files = self.get_affected_files(&art_id)?;
        let evidence = self.build_evidence_summary(&art_id, &f.claims)?;

        Ok(DecisionDetailView {
            id: art_id,
            title,
            status,
            mode,
            problem_refs: f.problem_refs,
            selected_title: f.selected_title,
            why_selected: f.why_selected,
            selection_policy: f.selection_policy,
            counterargument: f.counterargument,
            weakest_link: f.weakest_link,
            why_not_others: f
                .why_not_others
                .into_iter()
                .map(|r| RejectionView {
                    variant: r.variant,
                    reason: r.reason,
                })
                .collect(),
            invariants: f.invariants,
            pre_conditions: f.pre_conditions,
            post_conditions: f.post_conditions,
            admissibility: f.admissibility,
            evidence_requirements: f.evidence_requirements,
            refresh_triggers: f.refresh_triggers,
            claims: f
                .claims
                .into_iter()
                .map(|c| ClaimView {
                    id: c.id,
                    claim: c.claim,
                    observable: c.observable,
                    threshold: c.threshold,
                    status: c.status,
                    verify_after: c.verify_after,
                })
                .collect(),
            first_module_coverage: f.first_module_coverage,
            affected_files,
            coverage_modules: Vec::new(),
            coverage_warnings: Vec::new(),
            rollback_triggers: f.rollback_triggers,
            rollback_steps: f.rollback_steps,
            rollback_blast_radius: f.rollback_blast_radius,
            evidence,
            valid_until,
            body,
            created_at,
            updated_at,
        })
    }

    // ─── Portfolios ───

    pub fn list_portfolios(&self) -> Result<Vec<PortfolioSummaryView>> {
        let mut stmt = self.conn.prepare(
            "SELECT id, kind, version, status, context, mode, title, content,
                    valid_until, created_at, updated_at, COALESCE(structured_data, '')
             FROM artifacts
             WHERE kind = ?1 AND status = 'active'
             ORDER BY created_at DESC LIMIT 200",
        )?;

        let rows = stmt.query_map(params![KIND_SOLUTION_PORTFOLIO], |row| {
            let sd: String = row.get(11)?;
            let f: PortfolioFields = parse_json(&sd);
            Ok(PortfolioSummaryView {
                id: row.get(0)?,
                title: row.get(6)?,
                status: row.get(3)?,
                mode: col_str(row, 5),
                problem_ref: f.problem_ref,
                has_comparison: f.comparison.is_some(),
                created_at: truncate_date(&col_str(row, 9)),
            })
        })?;

        rows.collect()
    }

    pub fn get_portfolio(&self, id: &str) -> Result<PortfolioDetailView> {
        let mut stmt = self.conn.prepare(
            "SELECT id, kind, version, status, context, mode, title, content,
                    valid_until, created_at, updated_at, COALESCE(structured_data, '')
             FROM artifacts WHERE id = ?1",
        )?;

        let (art_id, status, title, body, created_at, updated_at, sd) =
            stmt.query_row(params![id], |row| {
                Ok((
                    row.get::<_, String>(0)?,
                    row.get::<_, String>(3)?,
                    row.get::<_, String>(6)?,
                    row.get::<_, String>(7)?,
                    col_str(row, 9),
                    col_str(row, 10),
                    row.get::<_, String>(11)?,
                ))
            })?;

        let f: PortfolioFields = parse_json(&sd);

        let variants = f
            .variants
            .into_iter()
            .map(|v| VariantView {
                id: v.id,
                title: v.title,
                description: v.description,
                weakest_link: v.weakest_link,
                novelty_marker: v.novelty_marker,
                stepping_stone: v.stepping_stone,
                strengths: v.strengths,
                risks: v.risks,
            })
            .collect();

        let comparison = f.comparison.map(|c| ComparisonView {
            dimensions: c.dimensions,
            scores: c.scores,
            non_dominated_set: c.non_dominated_set,
            dominated_notes: c
                .dominated_variants
                .into_iter()
                .map(|d| DominatedNote {
                    variant: d.variant,
                    dominated_by: d.dominated_by,
                    summary: d.summary,
                })
                .collect(),
            pareto_tradeoffs: c
                .pareto_tradeoffs
                .into_iter()
                .map(|t| TradeoffNote {
                    variant: t.variant,
                    summary: t.summary,
                })
                .collect(),
            policy_applied: c.policy_applied,
            selected_ref: c.selected_ref,
            recommendation: c.recommendation_rationale,
        });

        Ok(PortfolioDetailView {
            id: art_id,
            title,
            status,
            problem_ref: f.problem_ref,
            variants,
            comparison,
            body,
            created_at,
            updated_at,
        })
    }

    // ─── Tasks ───

    pub fn list_tasks(&self, project_path: &str) -> Result<Vec<TaskState>> {
        let mut stmt = self.conn.prepare(
            "SELECT id, title, agent, project_name, project_path, status, prompt,
                    branch, worktree, worktree_path, reused_worktree, error_message,
                    output_tail, started_at, COALESCE(completed_at, ''),
                    COALESCE(auto_run, 0), COALESCE(chat_blocks_json, '[]'),
                    COALESCE(raw_output, '')
             FROM desktop_tasks
             WHERE archived_at IS NULL AND project_path = ?1
             ORDER BY started_at DESC, id DESC",
        )?;

        let rows = stmt.query_map(params![project_path], |row| Ok(task_from_row(row)))?;
        rows.collect()
    }

    pub fn get_task(&self, id: &str) -> Result<TaskState> {
        let mut stmt = self.conn.prepare(
            "SELECT id, title, agent, project_name, project_path, status, prompt,
                    branch, worktree, worktree_path, reused_worktree, error_message,
                    output_tail, started_at, COALESCE(completed_at, ''),
                    COALESCE(auto_run, 0), COALESCE(chat_blocks_json, '[]'),
                    COALESCE(raw_output, '')
             FROM desktop_tasks
             WHERE id = ?1 AND archived_at IS NULL",
        )?;

        stmt.query_row(params![id], |row| Ok(task_from_row(row)))
    }

    // ─── Search (FTS5) ───

    pub fn search_artifacts(&self, query: &str) -> Result<Vec<ArtifactView>> {
        let fts_query = query
            .split_whitespace()
            .map(|w| format!("\"{}\"", w.replace('"', "")))
            .collect::<Vec<_>>()
            .join(" ");

        if fts_query.is_empty() {
            return Ok(Vec::new());
        }

        let mut stmt = self.conn.prepare(
            "SELECT a.id, a.kind, a.version, a.status, a.context, a.mode,
                    a.title, a.content, a.valid_until, a.created_at, a.updated_at
             FROM artifacts a
             JOIN artifacts_fts f ON a.id = f.id
             WHERE artifacts_fts MATCH ?1
             ORDER BY bm25(artifacts_fts, 0.0, 10.0, 1.0, 5.0, 3.0)
             LIMIT 50",
        )?;

        let rows = stmt.query_map(params![fts_query], |row| {
            Ok(ArtifactView {
                id: row.get(0)?,
                kind: row.get(1)?,
                title: row.get(6)?,
                status: row.get(3)?,
                mode: col_str(row, 5),
                created_at: truncate_date(&col_str(row, 9)),
                updated_at: truncate_date(&col_str(row, 10)),
            })
        })?;

        rows.collect()
    }

    // ─── Projects ───

    pub fn list_projects(&self) -> Result<Vec<String>> {
        let mut stmt = self.conn.prepare(
            "SELECT DISTINCT project_path FROM desktop_tasks
             WHERE archived_at IS NULL
             ORDER BY project_path",
        )?;
        let rows = stmt.query_map([], |row| row.get(0))?;
        rows.collect()
    }

    // ─── Config (governance state key-value) ───

    pub fn get_config(&self, key: &str) -> Result<String> {
        self.conn.query_row(
            "SELECT COALESCE(state_value, '') FROM desktop_governance_state WHERE state_key = ?1",
            params![key],
            |row| row.get(0),
        )
    }

    pub fn get_all_config(&self) -> Result<String> {
        // Return empty JSON config — desktop config is file-based (~/.haft/config.json),
        // not in SQLite. This prevents the error while we wire up file-based config.
        Ok("{}".to_string())
    }

    pub fn list_all_tasks(&self) -> Result<Vec<TaskState>> {
        let mut stmt = self.conn.prepare(
            "SELECT id, title, agent, project_name, project_path, status, prompt,
                    branch, worktree, worktree_path, reused_worktree, error_message,
                    output_tail, started_at, COALESCE(completed_at, ''),
                    COALESCE(auto_run, 0), COALESCE(chat_blocks_json, '[]'),
                    COALESCE(raw_output, '')
             FROM desktop_tasks
             WHERE archived_at IS NULL
             ORDER BY started_at DESC, id DESC",
        )?;

        let rows = stmt.query_map([], |row| Ok(task_from_row(row)))?;
        rows.collect()
    }

    /// Soft-delete: sets `archived_at` timestamp so `list_tasks` hides the
    /// row. Row stays on disk for audit — use an explicit purge if needed.
    pub fn archive_task(&self, id: &str) -> Result<()> {
        let now = now_unix_ts();
        self.conn.execute(
            "UPDATE desktop_tasks SET archived_at = ?1, updated_at = ?1 WHERE id = ?2",
            params![now, id],
        )?;
        Ok(())
    }

    /// Toggle the `auto_run` flag on a task row. `list_tasks` exposes the
    /// flag as `auto_run: bool` in `TaskState`; the UI reads it per task.
    pub fn set_task_auto_run(&self, id: &str, auto_run: bool) -> Result<()> {
        let now = now_unix_ts();
        self.conn.execute(
            "UPDATE desktop_tasks SET auto_run = ?1, updated_at = ?2 WHERE id = ?3",
            params![auto_run as i64, now, id],
        )?;
        Ok(())
    }

    // ─── Flows ───

    /// List flows for a specific project — or across every project when
    /// `project_path` is empty. The frontend's Jobs page calls `list_flows`
    /// without arguments to show all flows in one pane; restricting to the
    /// matching row via `WHERE project_path = ?1` would return nothing in
    /// that case.
    pub fn list_flows(&self, project_path: &str) -> Result<Vec<FlowView>> {
        let sql = if project_path.is_empty() {
            "SELECT id, project_name, project_path, title, description, template_id,
                    agent, prompt, schedule, branch, use_worktree, enabled,
                    last_task_id, COALESCE(last_run_at, ''), COALESCE(next_run_at, ''),
                    last_error, created_at, updated_at
             FROM desktop_flows
             ORDER BY updated_at DESC, title ASC"
        } else {
            "SELECT id, project_name, project_path, title, description, template_id,
                    agent, prompt, schedule, branch, use_worktree, enabled,
                    last_task_id, COALESCE(last_run_at, ''), COALESCE(next_run_at, ''),
                    last_error, created_at, updated_at
             FROM desktop_flows
             WHERE project_path = ?1
             ORDER BY updated_at DESC, title ASC"
        };
        let mut stmt = self.conn.prepare(sql)?;

        let rows = if project_path.is_empty() {
            stmt.query_map([], |row| Ok(flow_from_row(row)))?
                .collect::<Result<Vec<_>>>()?
        } else {
            stmt.query_map(params![project_path], |row| Ok(flow_from_row(row)))?
                .collect::<Result<Vec<_>>>()?
        };
        Ok(rows)
    }

    pub fn list_flow_templates(&self) -> Vec<FlowTemplateView> {
        default_flow_templates()
    }

    /// Load a single flow by id. Used by `run_flow_now` on the Tauri side
    /// to translate a flow record into a spawn request.
    pub fn get_flow(&self, id: &str) -> Result<FlowView> {
        self.conn.query_row(
            "SELECT id, project_name, project_path, title, description, template_id,
                    agent, prompt, schedule, branch, use_worktree, enabled,
                    last_task_id, COALESCE(last_run_at, ''), COALESCE(next_run_at, ''),
                    last_error, created_at, updated_at
             FROM desktop_flows
             WHERE id = ?1",
            params![id],
            |row| Ok(flow_from_row(row)),
        )
    }

    /// Update `last_run_at` / `updated_at` when a flow is manually triggered.
    /// Separate from flow mutation so the Tauri-side run path can persist
    /// "I launched this" without waiting on a full spawn round-trip.
    pub fn mark_flow_run(&self, id: &str) -> Result<()> {
        let now = now_unix_ts();
        self.conn.execute(
            "UPDATE desktop_flows SET last_run_at = ?1, updated_at = ?1 WHERE id = ?2",
            params![now, id],
        )?;
        Ok(())
    }

    // ─── Private helpers ───

    fn count_active_by_kind(&self, kind: &str) -> Result<i64> {
        self.conn.query_row(
            "SELECT COUNT(*) FROM artifacts WHERE kind = ?1 AND status = 'active'",
            params![kind],
            |row| row.get(0),
        )
    }

    fn find_stale_artifacts(&self) -> Result<Vec<ArtifactView>> {
        let mut stmt = self.conn.prepare(
            "SELECT id, kind, version, status, context, mode, title, content,
                    valid_until, created_at, updated_at
             FROM artifacts
             WHERE status NOT IN ('superseded', 'deprecated')
               AND valid_until != ''
               AND valid_until < datetime('now')
             ORDER BY valid_until ASC
             LIMIT 50",
        )?;

        let rows = stmt.query_map([], |row| {
            Ok(ArtifactView {
                id: row.get(0)?,
                kind: row.get(1)?,
                title: row.get(6)?,
                status: row.get(3)?,
                mode: col_str(row, 5),
                created_at: truncate_date(&col_str(row, 9)),
                updated_at: truncate_date(&col_str(row, 10)),
            })
        })?;

        rows.collect()
    }

    /// Returns (active_evidence_count, measurement_supports_count) per decision artifact_ref.
    fn evidence_summary_by_decision(&self) -> Result<HashMap<String, (i64, i64)>> {
        let mut stmt = self.conn.prepare(
            "SELECT artifact_ref,
                    SUM(CASE WHEN verdict != 'superseded' THEN 1 ELSE 0 END),
                    SUM(CASE WHEN type = 'measurement' AND verdict = 'supports' THEN 1 ELSE 0 END)
             FROM evidence_items
             GROUP BY artifact_ref",
        )?;

        let mut map = HashMap::new();
        let rows = stmt.query_map([], |row| {
            Ok((
                row.get::<_, String>(0)?,
                row.get::<_, i64>(1)?,
                row.get::<_, i64>(2)?,
            ))
        })?;

        for r in rows {
            let (ref_id, active, measurement) = r?;
            map.insert(ref_id, (active, measurement));
        }

        Ok(map)
    }

    /// Backlinks: artifacts that reference a problem via artifact_links.
    fn get_problem_links(
        &self,
        problem_id: &str,
    ) -> Result<(Vec<ArtifactView>, Vec<ArtifactView>)> {
        let mut stmt = self.conn.prepare(
            "SELECT a.id, a.kind, a.title, a.status, a.mode, a.created_at, a.updated_at
             FROM artifact_links l
             JOIN artifacts a ON l.source_id = a.id
             WHERE l.target_id = ?1",
        )?;

        let mut portfolios = Vec::new();
        let mut decisions = Vec::new();

        let rows = stmt.query_map(params![problem_id], |row| {
            Ok(ArtifactView {
                id: row.get(0)?,
                kind: row.get(1)?,
                title: row.get(2)?,
                status: row.get(3)?,
                mode: col_str(row, 4),
                created_at: truncate_date(&col_str(row, 5)),
                updated_at: truncate_date(&col_str(row, 6)),
            })
        })?;

        for r in rows {
            let view = r?;
            match view.kind.as_str() {
                KIND_SOLUTION_PORTFOLIO => portfolios.push(view),
                KIND_DECISION_RECORD => decisions.push(view),
                _ => {}
            }
        }

        Ok((portfolios, decisions))
    }

    fn get_affected_files(&self, artifact_id: &str) -> Result<Vec<String>> {
        let mut stmt = self
            .conn
            .prepare("SELECT file_path FROM affected_files WHERE artifact_id = ?1")?;
        let rows = stmt.query_map(params![artifact_id], |row| row.get(0))?;
        rows.collect()
    }

    fn build_evidence_summary(
        &self,
        artifact_id: &str,
        claims: &[ClaimData],
    ) -> Result<EvidenceSummaryView> {
        let mut stmt = self.conn.prepare(
            "SELECT id, type, content, COALESCE(verdict, ''), congruence_level,
                    formality_level, COALESCE(claim_refs, '[]'), COALESCE(valid_until, ''),
                    CASE WHEN valid_until != '' AND valid_until < datetime('now') THEN 1 ELSE 0 END
             FROM evidence_items
             WHERE artifact_ref = ?1
             ORDER BY created_at DESC",
        )?;

        let items: Vec<EvidenceItemView> = stmt
            .query_map(params![artifact_id], |row| {
                let claim_refs_json: String = row.get(6)?;
                let claim_refs: Vec<String> =
                    serde_json::from_str(&claim_refs_json).unwrap_or_default();
                Ok(EvidenceItemView {
                    id: row.get(0)?,
                    evidence_type: row.get(1)?,
                    content: row.get(2)?,
                    verdict: row.get(3)?,
                    congruence_level: row.get(4)?,
                    formality_level: row.get(5)?,
                    claim_refs,
                    valid_until: row.get(7)?,
                    is_expired: row.get::<_, i64>(8)? != 0,
                })
            })?
            .collect::<Result<Vec<_>>>()?;

        // Coverage gaps: claim IDs with no supporting evidence.
        let covered_ids: HashSet<&str> = items
            .iter()
            .filter(|e| e.verdict != "superseded")
            .flat_map(|e| e.claim_refs.iter().map(|s| s.as_str()))
            .collect();

        let total_claims = claims.len() as i64;
        let covered_claims = claims
            .iter()
            .filter(|c| covered_ids.contains(c.id.as_str()))
            .count() as i64;
        let coverage_gaps: Vec<String> = claims
            .iter()
            .filter(|c| !c.id.is_empty() && !covered_ids.contains(c.id.as_str()))
            .map(|c| c.id.clone())
            .collect();

        Ok(EvidenceSummaryView {
            items,
            total_claims,
            covered_claims,
            coverage_gaps,
        })
    }
}

// ─── Free functions ───

fn parse_json<T: for<'de> Deserialize<'de> + Default>(s: &str) -> T {
    if s.is_empty() {
        return T::default();
    }
    serde_json::from_str(s).unwrap_or_default()
}

/// Read a potentially-null TEXT column as String, defaulting to "".
fn col_str(row: &rusqlite::Row<'_>, idx: usize) -> String {
    row.get::<_, Option<String>>(idx)
        .unwrap_or(None)
        .unwrap_or_default()
}

/// Truncate an RFC3339 timestamp to YYYY-MM-DD for list views.
fn truncate_date(s: &str) -> String {
    if s.len() >= 10 {
        s[..10].to_string()
    } else {
        s.to_string()
    }
}

fn truncate_vec<T>(mut v: Vec<T>, n: usize) -> Vec<T> {
    v.truncate(n);
    v
}

fn task_from_row(row: &rusqlite::Row<'_>) -> TaskState {
    let chat_json: String = row.get::<_, String>(16).unwrap_or_default();
    let chat_blocks: Vec<ChatBlock> = serde_json::from_str(&chat_json).unwrap_or_default();

    TaskState {
        id: row.get::<_, String>(0).unwrap_or_default(),
        title: row.get::<_, String>(1).unwrap_or_default(),
        agent: row.get::<_, String>(2).unwrap_or_default(),
        project: row.get::<_, String>(3).unwrap_or_default(),
        project_path: row.get::<_, String>(4).unwrap_or_default(),
        status: row.get::<_, String>(5).unwrap_or_default(),
        prompt: row.get::<_, String>(6).unwrap_or_default(),
        branch: row.get::<_, String>(7).unwrap_or_default(),
        worktree: row.get::<_, i64>(8).unwrap_or(0) != 0,
        worktree_path: row.get::<_, String>(9).unwrap_or_default(),
        reused_worktree: row.get::<_, i64>(10).unwrap_or(0) != 0,
        error_message: row.get::<_, String>(11).unwrap_or_default(),
        output: row.get::<_, String>(12).unwrap_or_default(),
        chat_blocks,
        raw_output: row.get::<_, String>(17).unwrap_or_default(),
        started_at: row.get::<_, String>(13).unwrap_or_default(),
        completed_at: row.get::<_, String>(14).unwrap_or_default(),
        auto_run: row.get::<_, i64>(15).unwrap_or(0) != 0,
    }
}

fn flow_from_row(row: &rusqlite::Row<'_>) -> FlowView {
    FlowView {
        id: row.get::<_, String>(0).unwrap_or_default(),
        project_name: row.get::<_, String>(1).unwrap_or_default(),
        project_path: row.get::<_, String>(2).unwrap_or_default(),
        title: row.get::<_, String>(3).unwrap_or_default(),
        description: row.get::<_, String>(4).unwrap_or_default(),
        template_id: row.get::<_, String>(5).unwrap_or_default(),
        agent: row.get::<_, String>(6).unwrap_or_default(),
        prompt: row.get::<_, String>(7).unwrap_or_default(),
        schedule: row.get::<_, String>(8).unwrap_or_default(),
        branch: row.get::<_, String>(9).unwrap_or_default(),
        use_worktree: row.get::<_, i64>(10).unwrap_or(0) != 0,
        enabled: row.get::<_, i64>(11).unwrap_or(0) != 0,
        last_task_id: row.get::<_, String>(12).unwrap_or_default(),
        last_run_at: row.get::<_, String>(13).unwrap_or_default(),
        next_run_at: row.get::<_, String>(14).unwrap_or_default(),
        last_error: row.get::<_, String>(15).unwrap_or_default(),
        created_at: row.get::<_, String>(16).unwrap_or_default(),
        updated_at: row.get::<_, String>(17).unwrap_or_default(),
    }
}

fn default_flow_templates() -> Vec<FlowTemplateView> {
    vec![
        FlowTemplateView {
            id: "decision-refresh".into(),
            name: "Decision Refresh".into(),
            description: "Verify due decisions and turn stale reasoning into scheduled operator work.".into(),
            agent: "claude".into(),
            schedule: "0 9 * * 1".into(),
            branch: "flows/decision-refresh".into(),
            use_worktree: true,
            prompt: "Review active decisions with expired or near-expired validity windows.\n\nInstructions:\n- List decisions that need refresh or measurement.\n- Spawn or update the appropriate verification follow-up.\n- Record clear next actions for any decision that remains stale after inspection.".into(),
        },
        FlowTemplateView {
            id: "drift-scan".into(),
            name: "Drift Detection".into(),
            description: "Run a recurring drift-focused review against baselined files and decisions.".into(),
            agent: "codex".into(),
            schedule: "0 10 * * 1-5".into(),
            branch: "flows/drift-scan".into(),
            use_worktree: true,
            prompt: "Scan the current project for drift against decision baselines and recently affected files.\n\nInstructions:\n- Review recorded baselines and affected files.\n- Surface files or modules that have drifted since the last baseline.\n- Summarize the highest-priority follow-up problems or verification tasks.".into(),
        },
        FlowTemplateView {
            id: "dependency-audit".into(),
            name: "Dependency Audit".into(),
            description: "Check the project for outdated dependencies and risky upgrade pressure.".into(),
            agent: "codex".into(),
            schedule: "0 11 * * 1".into(),
            branch: "flows/dependency-audit".into(),
            use_worktree: true,
            prompt: "Audit dependencies for outdated or risky versions.\n\nInstructions:\n- Inspect the project's package and module manifests.\n- Highlight outdated or vulnerable dependencies.\n- Recommend the smallest safe remediation steps and note anything that should become a tracked problem.".into(),
        },
        FlowTemplateView {
            id: "evidence-health".into(),
            name: "Evidence Health".into(),
            description: "Look for weak evidence and decisions that should be refreshed before they decay further.".into(),
            agent: "claude".into(),
            schedule: "0 14 * * 5".into(),
            branch: "flows/evidence-health".into(),
            use_worktree: true,
            prompt: "Review decision evidence health across the active project.\n\nInstructions:\n- List decisions with expired, weak, or missing evidence.\n- Identify coverage gaps and stale claims.\n- Recommend specific verification tasks for the most critical gaps.".into(),
        },
    ]
}

fn to_characterization_view(c: &CharacterizationData) -> CharacterizationView {
    CharacterizationView {
        version: c.version,
        dimensions: c
            .dimensions
            .iter()
            .map(|d| DimensionView {
                name: d.name.clone(),
                scale_type: d.scale_type.clone(),
                unit: d.unit.clone(),
                polarity: d.polarity.clone(),
                role: d.role.clone(),
                how_to_measure: d.how_to_measure.clone(),
                valid_until: d.valid_until.clone(),
            })
            .collect(),
        parity_plan: c.parity_plan.as_ref().map(|p| ParityPlanView {
            baseline_set: p.baseline_set.clone(),
            window: p.window.clone(),
            budget: p.budget.clone(),
            normalization: p
                .normalization
                .iter()
                .map(|n| NormRuleView {
                    dimension: n.dimension.clone(),
                    method: n.method.clone(),
                })
                .collect(),
            missing_data_policy: p.missing_data_policy.clone(),
            pinned_conditions: p.pinned_conditions.clone(),
        }),
    }
}

// ─── Tests ───

#[cfg(test)]
mod tests {
    use super::*;

    /// Create a temp database with schema + fixtures, return the path.
    /// The returned TempDir must be kept alive for the duration of the test.
    fn setup_test_db() -> (tempfile::TempDir, String) {
        let dir = tempfile::tempdir().expect("create temp dir");
        let path = dir.path().join("haft.db").to_string_lossy().to_string();

        let conn = Connection::open(&path).expect("open rw db");
        conn.execute_batch(SCHEMA_SQL).expect("create schema");
        insert_fixtures(&conn);

        drop(conn);
        (dir, path)
    }

    const SCHEMA_SQL: &str = "
        CREATE TABLE artifacts (
            id TEXT PRIMARY KEY,
            kind TEXT NOT NULL,
            version INTEGER DEFAULT 1,
            status TEXT DEFAULT 'active',
            context TEXT,
            mode TEXT,
            title TEXT NOT NULL,
            content TEXT NOT NULL,
            structured_data TEXT DEFAULT '',
            search_keywords TEXT DEFAULT '',
            valid_until TEXT,
            created_at TEXT NOT NULL,
            updated_at TEXT NOT NULL
        );

        CREATE VIRTUAL TABLE artifacts_fts USING fts5(
            id, title, content, kind, search_keywords,
            tokenize='porter unicode61'
        );

        CREATE TABLE artifact_links (
            source_id TEXT NOT NULL,
            target_id TEXT NOT NULL,
            link_type TEXT NOT NULL,
            created_at TEXT NOT NULL,
            PRIMARY KEY (source_id, target_id, link_type)
        );

        CREATE TABLE evidence_items (
            id TEXT PRIMARY KEY,
            artifact_ref TEXT NOT NULL,
            type TEXT NOT NULL,
            content TEXT NOT NULL,
            verdict TEXT,
            carrier_ref TEXT,
            congruence_level INTEGER DEFAULT 3,
            formality_level INTEGER DEFAULT 5,
            claim_refs TEXT DEFAULT '[]',
            claim_scope TEXT DEFAULT '[]',
            valid_until TEXT,
            created_at TEXT NOT NULL
        );

        CREATE TABLE affected_files (
            artifact_id TEXT NOT NULL,
            file_path TEXT NOT NULL,
            file_hash TEXT,
            PRIMARY KEY (artifact_id, file_path)
        );

        CREATE TABLE desktop_tasks (
            id TEXT PRIMARY KEY,
            project_name TEXT NOT NULL,
            project_path TEXT NOT NULL,
            title TEXT NOT NULL,
            agent TEXT NOT NULL,
            status TEXT NOT NULL,
            prompt TEXT NOT NULL,
            branch TEXT DEFAULT '',
            worktree INTEGER DEFAULT 0,
            worktree_path TEXT DEFAULT '',
            reused_worktree INTEGER DEFAULT 0,
            error_message TEXT DEFAULT '',
            output_tail TEXT DEFAULT '',
            auto_run INTEGER DEFAULT 0,
            chat_blocks_json TEXT DEFAULT '[]',
            raw_output TEXT DEFAULT '',
            started_at TEXT NOT NULL,
            completed_at TEXT,
            updated_at TEXT NOT NULL,
            archived_at TEXT
        );

        CREATE TABLE desktop_governance_state (
            state_key TEXT PRIMARY KEY,
            state_value TEXT DEFAULT '',
            updated_at TEXT NOT NULL
        );

        CREATE TABLE desktop_flows (
            id TEXT PRIMARY KEY,
            project_name TEXT NOT NULL,
            project_path TEXT NOT NULL,
            title TEXT NOT NULL,
            description TEXT NOT NULL DEFAULT '',
            template_id TEXT NOT NULL DEFAULT '',
            agent TEXT NOT NULL,
            prompt TEXT NOT NULL,
            schedule TEXT NOT NULL,
            branch TEXT NOT NULL DEFAULT '',
            use_worktree INTEGER NOT NULL DEFAULT 0,
            enabled INTEGER NOT NULL DEFAULT 1,
            last_task_id TEXT NOT NULL DEFAULT '',
            last_run_at TEXT,
            next_run_at TEXT,
            last_error TEXT NOT NULL DEFAULT '',
            created_at TEXT NOT NULL,
            updated_at TEXT NOT NULL
        );
    ";

    fn insert_fixtures(conn: &Connection) {
        // Problem
        let prob_sd = serde_json::json!({
            "signal": "Wails IPC is unreliable",
            "constraints": ["Go core stays Go", "React frontend preserved"],
            "optimization_targets": ["startup speed"],
            "observation_indicators": ["crash rate"],
            "acceptance": "No IPC bugs in 30 days",
            "blast_radius": "desktop only",
            "reversibility": "medium",
            "characterizations": [{
                "version": 1,
                "dimensions": [{
                    "name": "reliability",
                    "scale_type": "ratio",
                    "unit": "crashes/day",
                    "polarity": "lower_better",
                    "role": "target",
                    "how_to_measure": "count crash logs",
                    "valid_until": "2026-06-01"
                }],
                "parity_plan": {
                    "baseline_set": ["wails-v2"],
                    "window": "30d",
                    "budget": "8h",
                    "normalization": [{"dimension": "reliability", "method": "z-score"}],
                    "missing_data_policy": "explicit_abstain",
                    "pinned_conditions": ["same hardware"]
                }
            }]
        });

        conn.execute(
            "INSERT INTO artifacts (id, kind, status, mode, title, content, structured_data, created_at, updated_at)
             VALUES (?1, ?2, 'active', 'standard', ?3, ?4, ?5, ?6, ?7)",
            params![
                "prob-20260416-001",
                KIND_PROBLEM_CARD,
                "Wails IPC unreliable",
                "# Problem\nWails IPC layer is buggy.",
                prob_sd.to_string(),
                "2026-04-16T10:00:00Z",
                "2026-04-16T10:00:00Z"
            ],
        ).expect("insert problem");

        // Decision (with evidence + claims)
        let dec_sd = serde_json::json!({
            "problem_refs": ["prob-20260416-001"],
            "selected_title": "Tauri v2 native GUI",
            "why_selected": "Eliminates IPC entirely",
            "selection_policy": "Eliminate fragile IPC",
            "counterargument": "Schema coupling with Go",
            "weakest_link": "Schema coupling",
            "why_not_others": [{"variant": "Go HTTP sidecar", "reason": "Reintroduces IPC"}],
            "invariants": ["Go core unchanged", "GUI reads SQLite directly"],
            "pre_conditions": ["Tauri v2 stable"],
            "post_conditions": ["Launches from Spotlight", "All projects in sidebar"],
            "admissibility": ["NOT: Go sidecar process"],
            "evidence_requirements": ["Zero IPC bugs in 30 days"],
            "refresh_triggers": ["Major Tauri update"],
            "claims": [{
                "id": "claim-001",
                "claim": "Zero IPC bugs in 30 days",
                "observable": "IPC bug reports",
                "threshold": "0",
                "status": "unverified",
                "verify_after": "2026-05-16"
            }],
            "first_module_coverage": true,
            "rollback_triggers": ["rusqlite schema read failure"],
            "rollback_steps": ["Revert to Wails v2 from git"],
            "rollback_blast_radius": "desktop-tauri/"
        });

        conn.execute(
            "INSERT INTO artifacts (id, kind, status, mode, title, content, structured_data, valid_until, created_at, updated_at)
             VALUES (?1, ?2, 'active', 'deep', ?3, ?4, ?5, ?6, ?7, ?8)",
            params![
                "dec-20260416-001",
                KIND_DECISION_RECORD,
                "Tauri v2 native GUI",
                "# Decision\nTauri v2 native GUI.",
                dec_sd.to_string(),
                "2026-07-16",
                "2026-04-16T11:00:00Z",
                "2026-04-16T11:00:00Z"
            ],
        ).expect("insert decision");

        // Evidence for the decision
        conn.execute(
            "INSERT INTO evidence_items (id, artifact_ref, type, content, verdict, congruence_level, formality_level, claim_refs, valid_until, created_at)
             VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10)",
            params![
                "ev-001",
                "dec-20260416-001",
                "measurement",
                "No IPC bugs reported after 7 days",
                "supports",
                3,
                1,
                r#"["claim-001"]"#,
                "2026-05-16",
                "2026-04-16T12:00:00Z"
            ],
        ).expect("insert evidence");

        // Affected files
        conn.execute(
            "INSERT INTO affected_files (artifact_id, file_path, file_hash) VALUES (?1, ?2, ?3)",
            params!["dec-20260416-001", "desktop-tauri/src/lib.rs", "abc123"],
        )
        .expect("insert affected file");

        // Portfolio
        let sol_sd = serde_json::json!({
            "problem_ref": "prob-20260416-001",
            "variants": [{
                "id": "v1",
                "title": "Tauri v2",
                "description": "Native Rust + WebView",
                "weakest_link": "Schema coupling",
                "novelty_marker": "Direct SQLite reads",
                "stepping_stone": true,
                "strengths": ["No IPC", "Native performance"],
                "risks": ["Schema coupling"]
            }],
            "comparison": {
                "dimensions": ["reliability", "complexity"],
                "scores": {"v1": {"reliability": "high", "complexity": "medium"}},
                "non_dominated_set": ["v1"],
                "dominated_variants": [{"variant": "v2", "dominated_by": ["v1"], "summary": "Strictly worse"}],
                "pareto_tradeoffs": [{"variant": "v1", "summary": "Best on reliability"}],
                "policy_applied": "eliminate fragile IPC",
                "selected_ref": "v1",
                "recommendation_rationale": "Tauri v2 dominates"
            }
        });

        conn.execute(
            "INSERT INTO artifacts (id, kind, status, mode, title, content, structured_data, created_at, updated_at)
             VALUES (?1, ?2, 'active', 'standard', ?3, ?4, ?5, ?6, ?7)",
            params![
                "sol-20260416-001",
                KIND_SOLUTION_PORTFOLIO,
                "Tauri migration portfolio",
                "# Portfolio\nSolution variants.",
                sol_sd.to_string(),
                "2026-04-16T09:00:00Z",
                "2026-04-16T09:00:00Z"
            ],
        ).expect("insert portfolio");

        // Note (for count)
        conn.execute(
            "INSERT INTO artifacts (id, kind, status, mode, title, content, created_at, updated_at)
             VALUES ('note-001', 'Note', 'active', 'note', 'Quick note', 'Some note body.', '2026-04-16T08:00:00Z', '2026-04-16T08:00:00Z')",
            [],
        ).expect("insert note");

        // Artifact links: portfolio → problem, decision → problem
        conn.execute(
            "INSERT INTO artifact_links (source_id, target_id, link_type, created_at)
             VALUES ('sol-20260416-001', 'prob-20260416-001', 'based_on', '2026-04-16T09:00:00Z')",
            [],
        )
        .expect("insert link sol->prob");
        conn.execute(
            "INSERT INTO artifact_links (source_id, target_id, link_type, created_at)
             VALUES ('dec-20260416-001', 'prob-20260416-001', 'based_on', '2026-04-16T11:00:00Z')",
            [],
        )
        .expect("insert link dec->prob");

        // FTS5 entries (must be manually synced)
        conn.execute(
            "INSERT INTO artifacts_fts (id, title, content, kind, search_keywords)
             VALUES ('prob-20260416-001', 'Wails IPC unreliable', 'Wails IPC layer is buggy.', 'ProblemCard', 'wails ipc desktop')",
            [],
        ).expect("insert fts problem");
        conn.execute(
            "INSERT INTO artifacts_fts (id, title, content, kind, search_keywords)
             VALUES ('dec-20260416-001', 'Tauri v2 native GUI', 'Tauri v2 native GUI.', 'DecisionRecord', 'tauri rust gui')",
            [],
        ).expect("insert fts decision");

        // Task
        conn.execute(
            "INSERT INTO desktop_tasks (id, project_name, project_path, title, agent, status, prompt, branch, started_at, completed_at, updated_at)
             VALUES ('task-001', 'haft', '/tmp/haft', 'Implement read layer', 'claude', 'completed', 'Write db.rs', 'dev', '2026-04-16T13:00:00Z', '2026-04-16T14:00:00Z', '2026-04-16T14:00:00Z')",
            [],
        ).expect("insert task");

        // Governance state
        conn.execute(
            "INSERT INTO desktop_governance_state (state_key, state_value, updated_at)
             VALUES ('last_scan', '2026-04-16T15:00:00Z', '2026-04-16T15:00:00Z')",
            [],
        )
        .expect("insert governance state");

        // Flow
        conn.execute(
            "INSERT INTO desktop_flows (id, project_name, project_path, title, description, template_id, agent, prompt, schedule, branch, use_worktree, enabled, last_task_id, last_run_at, next_run_at, last_error, created_at, updated_at)
             VALUES ('flow-001', 'haft', '/tmp/haft', 'Decision Refresh', 'Weekly decision refresh', 'decision-refresh', 'claude', 'Review decisions', '0 9 * * 1', 'flows/decision-refresh', 1, 1, '', '', '', '', '2026-04-16T16:00:00Z', '2026-04-16T16:00:00Z')",
            [],
        ).expect("insert flow");
    }

    #[test]
    fn test_open_readwrite_allows_desktop_updates() {
        let (_dir, path) = setup_test_db();
        let db = HaftDb::open(&path).expect("open db");

        // Desktop owns a small write surface: task archival, task autorun,
        // and flow state updates. The connection must be writable while the
        // artifact graph remains owned by the Go core.
        let result = db.conn.execute(
            "INSERT INTO artifacts (id, kind, title, content, created_at, updated_at)
             VALUES ('x', 'Note', 'x', 'x', 'x', 'x')",
            [],
        );
        assert!(result.is_ok(), "desktop connection must allow writes");
    }

    #[test]
    fn test_list_problems() {
        let (_dir, path) = setup_test_db();
        let db = HaftDb::open(&path).unwrap();

        let problems = db.list_problems().unwrap();
        assert_eq!(problems.len(), 1);

        let p = &problems[0];
        assert_eq!(p.id, "prob-20260416-001");
        assert_eq!(p.signal, "Wails IPC is unreliable");
        assert_eq!(
            p.constraints,
            vec!["Go core stays Go", "React frontend preserved"]
        );
        assert_eq!(p.created_at, "2026-04-16");

        // Verify JSON shape
        let json = serde_json::to_value(p).unwrap();
        assert!(json.get("id").unwrap().is_string());
        assert!(json.get("constraints").unwrap().is_array());
    }

    #[test]
    fn test_get_problem_detail() {
        let (_dir, path) = setup_test_db();
        let db = HaftDb::open(&path).unwrap();

        let detail = db.get_problem("prob-20260416-001").unwrap();
        assert_eq!(detail.signal, "Wails IPC is unreliable");
        assert_eq!(detail.acceptance, "No IPC bugs in 30 days");
        assert_eq!(detail.blast_radius, "desktop only");
        assert!(!detail.characterizations.is_empty());

        let char_view = &detail.characterizations[0];
        assert_eq!(char_view.version, 1);
        assert_eq!(char_view.dimensions[0].name, "reliability");
        assert!(char_view.parity_plan.is_some());

        assert!(detail.latest_characterization.is_some());

        // Backlinks
        assert_eq!(detail.linked_portfolios.len(), 1);
        assert_eq!(detail.linked_portfolios[0].id, "sol-20260416-001");
        assert_eq!(detail.linked_decisions.len(), 1);
        assert_eq!(detail.linked_decisions[0].id, "dec-20260416-001");

        assert_eq!(detail.body, "# Problem\nWails IPC layer is buggy.");
    }

    #[test]
    fn test_list_decisions() {
        let (_dir, path) = setup_test_db();
        let db = HaftDb::open(&path).unwrap();

        let decisions = db.list_decisions().unwrap();
        assert_eq!(decisions.len(), 1);
        assert_eq!(decisions[0].selected_title, "Tauri v2 native GUI");
        assert_eq!(decisions[0].weakest_link, "Schema coupling");
        assert_eq!(decisions[0].valid_until, "2026-07-16");

        let json = serde_json::to_value(&decisions[0]).unwrap();
        assert!(json.get("implement_guard").unwrap().is_object());
    }

    #[test]
    fn test_get_decision_detail() {
        let (_dir, path) = setup_test_db();
        let db = HaftDb::open(&path).unwrap();

        let detail = db.get_decision("dec-20260416-001").unwrap();
        assert_eq!(detail.selected_title, "Tauri v2 native GUI");
        assert_eq!(detail.problem_refs, vec!["prob-20260416-001"]);
        assert_eq!(detail.why_not_others.len(), 1);
        assert_eq!(detail.why_not_others[0].variant, "Go HTTP sidecar");
        assert_eq!(
            detail.invariants,
            vec!["Go core unchanged", "GUI reads SQLite directly"]
        );
        assert_eq!(detail.claims.len(), 1);
        assert_eq!(detail.claims[0].id, "claim-001");
        assert_eq!(detail.affected_files, vec!["desktop-tauri/src/lib.rs"]);
        assert!(detail.first_module_coverage);

        // Evidence
        assert_eq!(detail.evidence.items.len(), 1);
        assert_eq!(detail.evidence.items[0].evidence_type, "measurement");
        assert_eq!(detail.evidence.total_claims, 1);
        assert_eq!(detail.evidence.covered_claims, 1);
        assert!(detail.evidence.coverage_gaps.is_empty());

        let json = serde_json::to_value(&detail).unwrap();
        assert!(json.get("rollback_triggers").unwrap().is_array());
    }

    #[test]
    fn test_list_portfolios() {
        let (_dir, path) = setup_test_db();
        let db = HaftDb::open(&path).unwrap();

        let portfolios = db.list_portfolios().unwrap();
        assert_eq!(portfolios.len(), 1);
        assert_eq!(portfolios[0].problem_ref, "prob-20260416-001");
        assert!(portfolios[0].has_comparison);
    }

    #[test]
    fn test_get_portfolio_detail() {
        let (_dir, path) = setup_test_db();
        let db = HaftDb::open(&path).unwrap();

        let detail = db.get_portfolio("sol-20260416-001").unwrap();
        assert_eq!(detail.variants.len(), 1);
        assert_eq!(detail.variants[0].title, "Tauri v2");
        assert!(detail.variants[0].stepping_stone);
        assert_eq!(
            detail.variants[0].strengths,
            vec!["No IPC", "Native performance"]
        );

        let comp = detail.comparison.as_ref().unwrap();
        assert_eq!(comp.dimensions, vec!["reliability", "complexity"]);
        assert_eq!(comp.recommendation, "Tauri v2 dominates");
        assert_eq!(comp.dominated_notes.len(), 1);
        assert_eq!(comp.pareto_tradeoffs.len(), 1);

        let json = serde_json::to_value(&detail).unwrap();
        assert!(json.get("comparison").unwrap().is_object());
    }

    #[test]
    fn test_search_artifacts() {
        let (_dir, path) = setup_test_db();
        let db = HaftDb::open(&path).unwrap();

        let results = db.search_artifacts("wails IPC").unwrap();
        assert!(!results.is_empty());
        assert_eq!(results[0].id, "prob-20260416-001");

        let results = db.search_artifacts("tauri").unwrap();
        assert!(!results.is_empty());

        let results = db.search_artifacts("").unwrap();
        assert!(results.is_empty());
    }

    #[test]
    fn test_list_tasks() {
        let (_dir, path) = setup_test_db();
        let db = HaftDb::open(&path).unwrap();

        let tasks = db.list_tasks("/tmp/haft").unwrap();
        assert_eq!(tasks.len(), 1);
        assert_eq!(tasks[0].id, "task-001");
        assert_eq!(tasks[0].agent, "claude");
        assert_eq!(tasks[0].status, "completed");
        assert_eq!(tasks[0].project, "haft");

        let json = serde_json::to_value(&tasks[0]).unwrap();
        assert!(json.get("chat_blocks").unwrap().is_array());
        assert_eq!(json.get("worktree").unwrap(), false);
    }

    #[test]
    fn test_get_task() {
        let (_dir, path) = setup_test_db();
        let db = HaftDb::open(&path).unwrap();

        let task = db.get_task("task-001").unwrap();
        assert_eq!(task.title, "Implement read layer");
        assert_eq!(task.prompt, "Write db.rs");
    }

    #[test]
    fn test_list_projects() {
        let (_dir, path) = setup_test_db();
        let db = HaftDb::open(&path).unwrap();

        let projects = db.list_projects().unwrap();
        assert_eq!(projects, vec!["/tmp/haft"]);
    }

    #[test]
    fn test_get_config() {
        let (_dir, path) = setup_test_db();
        let db = HaftDb::open(&path).unwrap();

        let val = db.get_config("last_scan").unwrap();
        assert_eq!(val, "2026-04-16T15:00:00Z");

        let err = db.get_config("nonexistent");
        assert!(err.is_err());
    }

    #[test]
    fn test_dashboard() {
        let (_dir, path) = setup_test_db();
        let db = HaftDb::open(&path).unwrap();

        let dash = db.get_dashboard("haft").unwrap();
        assert_eq!(dash.project_name, "haft");
        assert_eq!(dash.problem_count, 1);
        assert_eq!(dash.decision_count, 1);
        assert_eq!(dash.portfolio_count, 1);
        assert_eq!(dash.note_count, 1);

        // Decision has measurement+supports evidence → healthy
        assert_eq!(dash.healthy_decisions.len(), 1);
        assert!(dash.pending_decisions.is_empty());
        assert!(dash.unassessed_decisions.is_empty());

        let json = serde_json::to_value(&dash).unwrap();
        assert!(json.get("recent_problems").unwrap().is_array());
        assert!(json.get("stale_items").unwrap().is_array());
    }

    #[test]
    fn test_list_flows() {
        let (_dir, path) = setup_test_db();
        let db = HaftDb::open(&path).unwrap();

        let flows = db.list_flows("/tmp/haft").unwrap();
        assert_eq!(flows.len(), 1);
        assert_eq!(flows[0].id, "flow-001");
        assert_eq!(flows[0].title, "Decision Refresh");
        assert_eq!(flows[0].template_id, "decision-refresh");
        assert!(flows[0].use_worktree);
        assert!(flows[0].enabled);

        let json = serde_json::to_value(&flows[0]).unwrap();
        assert!(json.get("use_worktree").unwrap().is_boolean());
        assert!(json.get("enabled").unwrap().is_boolean());

        // No flows for different project
        let empty = db.list_flows("/other/project").unwrap();
        assert!(empty.is_empty());
    }

    #[test]
    fn test_list_flow_templates() {
        let (_dir, path) = setup_test_db();
        let db = HaftDb::open(&path).unwrap();

        let templates = db.list_flow_templates();
        assert_eq!(templates.len(), 4);
        assert_eq!(templates[0].id, "decision-refresh");
        assert_eq!(templates[1].id, "drift-scan");
        assert_eq!(templates[2].id, "dependency-audit");
        assert_eq!(templates[3].id, "evidence-health");
        assert!(templates[0].use_worktree);
    }

    #[test]
    fn test_empty_structured_data() {
        let dir = tempfile::tempdir().expect("create temp dir");
        let path = dir.path().join("haft.db").to_string_lossy().to_string();

        let conn = Connection::open(&path).expect("open rw db");
        conn.execute_batch(SCHEMA_SQL).expect("create schema");

        // Insert artifact with empty structured_data
        conn.execute(
            "INSERT INTO artifacts (id, kind, status, mode, title, content, structured_data, created_at, updated_at)
             VALUES ('prob-empty', 'ProblemCard', 'active', 'standard', 'Empty problem', 'body', '', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')",
            [],
        ).unwrap();

        drop(conn);

        let db = HaftDb::open(&path).unwrap();
        let problems = db.list_problems().unwrap();
        assert_eq!(problems.len(), 1);
        assert_eq!(problems[0].signal, "");
        assert!(problems[0].constraints.is_empty());
    }
}
