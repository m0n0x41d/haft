# Desktop Layer Contract

Status: target contract with current audit notes.

## Clarifying Answers

- Primary domain entity: an operator-controlled project runtime surface.
- Top-level code writer: Haft desktop developers and coding agents working through the repository.
- Current inexpressible states that must become inexpressible: not enough. The current code can still express stale projects as active, terminal tasks as accepting PTY input, partial task status events as full task records, and missing harness workspaces as normal runnable state.
- Abstraction levels in the domain: five.
- Messiest current code: `desktop/frontend/src/lib/api.ts`, `desktop/frontend/src/pages/Tasks.tsx`, `desktop/frontend/src/App.tsx`, `desktop-tauri/src/agents.rs`, and `desktop-tauri/src/db.rs`.

## Current Contract Gaps

- `api.ts` mixes domain types, mock data, Tauri transport, and fallback behavior.
- `Tasks.tsx` mixes task selection, transcript sync, PTY input, handoff, governance resolution, and presentation.
- `App.tsx` mixes project registry state, shell navigation, task registry, toasts, terminal visibility, and project mutation UX.
- `agents.rs` mixes command handlers, PTY lifecycle, transcript parsing, persistence projection, status transition logic, and frontend JSON contracts.
- `db.rs` is a read model adapter and a SQL mapping layer in the same module.

## Current IA Decision

The desktop shell is no longer modeled as a generic dashboard plus a separate primary Harness page.

- `Core` is the project cockpit: identity, operator triage, active runtime, and quiet jumps to details.
- `Onboarding` is the spec construction workspace: target spec, enabling spec, term map, and coverage gaps.
- `Conversations` are long-lived operator threads; runtime turns are embedded cards, not terminal endings.
- `Runtime` remains a detail/operator surface for WorkCommissions and apply/requeue/cancel actions.
- `Artifacts` remain the governance graph: problems, decisions, comparison, evidence, commissions.
- `Settings` owns project registry, agent presets, runtime defaults, and keys.

Desktop is one surface over Haft Core, not the owner of semantic authority. The
same workflows must remain available through MCP and CLI where those surfaces
are the better operator fit:

```text
Desktop Cockpit = primary human navigation and approval surface
MCP Plugin      = embedded agent reasoning/authoring/commissioning surface
CLI Harness     = runtime/operator automation surface
Haft Core       = semantic authority and artifact graph
Open-Sleigh     = execution mechanics
```

The key distinction:

```text
Core = what needs the operator in this project now.
Onboarding = how a repository becomes harnessable.
Runtime = how the harness engine is executing WorkCommissions.
Conversation = human-agent interaction over time.
RuntimeRun = one execution attempt inside or referenced by a conversation.
```

Design rule:

- The default Core surface must be cognitively small: identity -> Needs You -> Active Runtime -> quiet jumps.
- A project that is `needs_init` or `needs_onboard` must make that the primary operator action before generic task running.
- Spec readiness, not task count, is the first-class readiness state for serious harness work.
- Counts, coverage, and raw tables are Pro/detail mode only.
- No hidden control prompt, JSON carrier, or runtime envelope may render as a user chat message.
- Every workflow button compiles to a typed artifact transition. It may inject
  a prompt stage into a host agent, but the result must be an explicit
  artifact proposal/mutation followed by deterministic validation.

## LAYER 0: Desktop Domain Core

Concepts:
- `ProjectReadiness`
- `TaskInputCapability`
- `TaskRunState`
- `TaskTranscript`
- `HarnessCommissionState`
- `CoreAttentionItem`
- `CoreRuntimeItem`
- `SpecReadiness`
- `SpecCoverageSummary`
- `OperatorAction`
- `WorkflowIntent`
- `ArtifactTransition`

Functions:
- `project_readiness(path, has_config) -> ProjectReadiness`
- `task_input_capability(status) -> TaskInputCapability`
- `task_run_state(raw_status) -> TaskRunState`
- `build_core_attention(governance, tasks, commissions) -> CoreAttentionItem[]`
- `build_core_runtime_items(commissions) -> CoreRuntimeItem[]`
- `spec_readiness(project, spec_check) -> SpecReadiness`
- `spec_coverage_summary(edges, evidence) -> SpecCoverageSummary`
- `commission_operator_actions(state, delivery_policy) -> OperatorAction[]`
- `workflow_intent(surface_action) -> WorkflowIntent`
- `artifact_transition(workflow_result) -> ArtifactTransition`

Inexpressible:
- A missing project that is also active/runnable.
- A terminal task that accepts PTY input.
- A terminal task that cannot be continued from the chat surface.
- A harness commission with no next operator action.
- A status string used directly by UI logic.
- A generic dashboard item shown in Core without an operator-facing reason.
- A project marked runnable for broad harness work while target/enabling specs are missing.
- A spec coverage gap shown without a next operator action.
- A Desktop button that directly sends an opaque prompt and marks semantic state ready.

Depends on: none.

Canonical normal form:
- Every raw status compiles to an algebraic state before UI or command logic can branch on it.
- Every Core row is either `CoreAttentionItem` or `CoreRuntimeItem`; JSX does not construct these ad hoc.
- Every project has exactly one readiness state: `ready`, `needs_init`, `needs_onboard`, or `missing`.
- Every operator button has exactly one typed workflow intent.

## LAYER 1: Contract Normalization

Concepts:
- `ProjectRegistryView`
- `TaskStateView`
- `HarnessResultView`
- `SpecCheckView`
- `DesktopCommandRequest`

Functions:
- `normalize_project_registry(raw_registry, filesystem_facts) -> ProjectRegistryView`
- `normalize_task_event(previous_task, status_event) -> TaskStateView`
- `normalize_spawn_response(running_task) -> TaskStateView`
- `normalize_harness_result(raw_result, workspace_facts) -> HarnessResultView`
- `normalize_spec_check(raw_spec_check) -> SpecCheckView`
- `normalize_harness_ipc_args(frontend_action) -> TauriCamelCaseArgs`

Inexpressible:
- Partial Tauri event payload treated as a complete frontend task.
- Project identity derived from stale registry name when repo-local config says otherwise.
- Harness result shown without workspace/diff/apply policy.
- Spec check result shown as raw JSON instead of section-id grouped operator work.
- Tauri IPC called with Go/Rust snake_case keys from frontend code.

Depends on: LAYER 0.

Canonical normal form:
- Frontend sees one `TaskStateView` shape.
- Project list entries always include `status`, `exists`, and `has_haft`.

## LAYER 2: Effect Adapters

Concepts:
- `FilesystemAdapter`
- `DesktopRpcAdapter`
- `PtyAdapter`
- `TauriEventAdapter`
- `BrowserTimerAdapter`

Functions:
- `read_project_registry() -> Result<RawRegistry>`
- `call_desktop_rpc(command, payload, project_root) -> Result<Json>`
- `spawn_pty(command, cwd, env) -> Result<PtySession>`
- `emit_task_status(event) -> Result<()>`

Inexpressible:
- Domain decisions hidden inside filesystem, PTY, RPC, or timer code.
- Raw OS errors exposed as operator UX without domain translation.

Depends on: LAYER 1.

Canonical normal form:
- Every effect returns `Result<ContractValue, DomainError>`.

## LAYER 3: Application Orchestration

Concepts:
- `DesktopAppModel`
- `TaskRuntimeController`
- `ProjectRegistryController`
- `HarnessOperatorController`
- `OnboardingController`

Functions:
- `apply_task_event(model, event) -> DesktopAppModel`
- `apply_project_action(model, action_result) -> DesktopAppModel`
- `start_task(request) -> Result<TaskStateView>`
- `submit_task_input(task_id, text) -> Result<TaskStateView>`
- `start_onboarding(project_id, stage) -> Result<SpecCheckView>`
- `apply_spec_section_update(project_id, section_patch) -> Result<SpecCheckView>`

Inexpressible:
- UI components directly deciding effect sequencing.
- A task input submission bypassing `TaskInputCapability`.
- Project switch to a non-ready project.
- Broad harness run from a project whose relevant spec readiness is not admissible.

Depends on: LAYER 2.

Canonical normal form:
- A user action is compiled to one controller operation, which returns one normalized view update.

## LAYER 4: Presentation

Concepts:
- `DashboardPage`
- `TasksPage`
- `HarnessPage`
- `SettingsPage`
- `TerminalPanel`

Functions:
- `render(model) -> ReactElement`
- `user_event -> OperatorAction`

Inexpressible:
- Calling Tauri directly from presentational components.
- Branching on raw status strings in JSX.
- Showing an enabled button when the corresponding `OperatorAction` is absent.

Depends on: LAYER 3.

Canonical normal form:
- Components receive view models and callbacks only.

## Compilation Example

Top-level operator event:

```text
Send follow-up text
```

Compilation:

```text
TaskState.status
  -> task_input_capability(status)
  -> OperatorAction.WriteLiveInput | OperatorAction.ContinueTask
  -> live task: write_task_input command -> PtyAdapter.write
  -> terminal task: continue_task command -> spawn new runtime turn
  -> TaskStateView update
```

Python-shaped data:

```python
{
  "task_id": "task-001",
  "input_capability": {"kind": "continuation"},
  "operator_action": {"kind": "continue_task"},
}
```

The invalid state `{"status": "completed", "write_to_pty": True}` must not be constructible.

## Required Test Matrix

- LAYER 0 pure tests:
  - task input capability from each terminal/runnable status.
  - terminal task follow-up compiles to continuation, not closed input.
  - project readiness from `(exists, has_haft)`.
  - spec readiness from target/enabling/spec-check states.
  - spec coverage summary groups uncovered/reasoned/commissioned/verified/stale deterministically.
  - Core attention treats blocked commissions as operator work.
  - Core runtime hides terminal commissions by default.
  - commission phase is normalized from runtime state names.
  - harness operator actions from commission/result states.
- LAYER 1 contract tests:
  - spawn response includes the full frontend `TaskStateView`.
  - partial task status event does not erase existing task fields.
  - duplicate project registry identities collapse to one canonical entry.
  - harness Tauri IPC uses camelCase keys and Go RPC payload uses snake_case keys.
  - spec check raw output normalizes into section-id grouped operator work.
- LAYER 2 adapter tests:
  - PTY closed writer maps to domain error, not raw `os error`.
  - desktop-rpc project registration repairs stale local project names.
- LAYER 3 orchestration tests:
  - completed task follow-up is rejected before PTY write.
  - completed task chat follow-up starts a continuation task.
  - missing project cannot be selected as runnable.
  - harness result exposes apply/requeue/cancel next actions.
  - onboarding can be started only from `needs_onboard` or explicit re-onboard action.
  - broad harness run is blocked when spec readiness is missing for spec-required work.
- LAYER 4 smoke tests:
  - Tasks page does not enable follow-up input for completed tasks.
  - Settings project list can remove missing projects.
  - Harness page shows current, terminal, and blocked commissions without raw JSON flood.
  - Onboarding/spec surfaces never render raw spec-check JSON as chat content.

## Implementation Order

1. Extract LAYER 0 pure contracts currently embedded in pages and command handlers.
2. Make Tauri command payloads total and normalized at LAYER 1.
3. Move PTY/RPC/filesystem details behind small LAYER 2 adapters.
4. Shrink `App.tsx`, `Tasks.tsx`, `api.ts`, and `agents.rs` by moving decisions down to the proper layer.
5. Add contract tests before moving each behavior.
