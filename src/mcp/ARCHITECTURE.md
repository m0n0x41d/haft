# Quint Code — Functional Architecture

## Layer Hierarchy

```
LAYER 4: Transport       internal/fpf/server.go
LAYER 3: Orchestration   cmd/serve.go
LAYER 2: Presentation    internal/present/
LAYER 1: Domain Logic    internal/artifact/
LAYER 0: Core Types      internal/artifact/types.go, iface.go
INFRA:   Persistence     internal/artifact/store.go (SQLiteStore)
```

## Layer Specifications

### LAYER 0: Core Types (`types.go`, `iface.go`)

**Concepts:** Kind, Status, Mode, Artifact, Meta, Link, AffectedFile, EvidenceItem,
ProblemFields, DecisionFields, ProblemListItem, LineageSource, DecideContext, ExploreContext

**Inexpressible:**
- Invalid Kind/Status/Mode at validated boundaries (ParseKind/ParseStatus/ParseMode reject unknown values)
- Artifact without ID or Kind (Build* functions require these)

**Functions:**
- `ParseKind(string) → (Kind, error)` — validates at boundary
- `ParseStatus(string) → (Status, error)`
- `ParseMode(string) → (Mode, error)`
- `Kind.IsValid() → bool`
- `Kind.IDPrefix() → string`
- `Kind.Dir() → string`
- `GenerateID(Kind, int) → string`

**Interface:**
- `ArtifactStore` — 26 methods defining the persistence contract
- Compile-time checked: `var _ ArtifactStore = (*Store)(nil)`

### LAYER 1: Domain Logic (`problem.go`, `solution.go`, `decision.go`, `note.go`, `refresh.go`)

**Concepts:** Problem framing, variant exploration, comparison, decision, measurement,
evidence attachment, staleness detection, drift, lineage, reconciliation

**Pure Build functions (no side effects):**
- `BuildProblemArtifact(id, now, input, recall) → *Artifact`
- `BuildNoteArtifact(id, now, input) → *Artifact`
- `BuildDecisionArtifact(DecideContext, DecideInput) → *Artifact`
- `BuildPortfolioArtifact(ExploreContext, ExploreInput, warnings, recall) → *Artifact`
- `BuildComparisonBody(existingBody, ComparisonResult, warnings) → string`
- `BuildLineageNotes(LineageSource) → string`
- `BuildWaiverSection(now, validUntil, reason, evidence) → string`
- `BuildSupersedeSection(now, newRef, reason) → string`
- `BuildDeprecateSection(now, reason) → string`
- `BuildRefreshReportArtifact(id, now, ref, action, reason, outcome) → *Artifact`

**Pure validation/analysis:**
- `ValidateExploreInput(ExploreInput) → error`
- `ValidateNote(ctx, store, NoteInput) → NoteValidation`
- `CheckVariantDiversity([]Variant) → []string`
- `extractSection(body, heading) → string`
- `extractGoldilocksSignals(*Artifact) → string`
- `ComputeNavState(ctx, store, context) → NavState`
- `ComputeWLNKSummary(ctx, store, id) → WLNKSummary`

**Orchestrators (thin: fetch → pure build → persist):**
- `FrameProblem(ctx, store, dir, input) → (*Artifact, path, error)`
- `CreateNote(ctx, store, dir, input) → (*Artifact, path, error)`
- `Decide(ctx, store, dir, input) → (*Artifact, path, error)`
- `ExploreSolutions(ctx, store, dir, input) → (*Artifact, path, error)`
- `CompareSolutions(ctx, store, dir, input) → (*Artifact, path, error)`
- `Measure(ctx, store, dir, input) → (*Artifact, error)`
- `AttachEvidence(ctx, store, input) → (*EvidenceItem, error)`
- `WaiveArtifact(ctx, store, dir, ref, reason, until, evidence) → (*Artifact, error)`
- `ReopenDecision(ctx, store, dir, ref, reason) → (*Artifact, *Artifact, error)`
- `SupersedeArtifact(ctx, store, dir, ref, newRef, reason) → (*Artifact, error)`
- `DeprecateArtifact(ctx, store, dir, ref, reason) → (*Artifact, error)`

**Inexpressible:**
- Decision without rationale (BuildDecisionArtifact rejects empty why_selected)
- Problem without signal (BuildProblemArtifact rejects empty signal)
- Variants without weakest link (ValidateExploreInput rejects)
- Fewer than 2 variants (ValidateExploreInput rejects)
- Invalid mode in Problem/Decision (ParseMode rejects)

**Depends on:** LAYER 0 (types, interface)

### LAYER 2: Presentation (`internal/present/`)

**Concepts:** MCP tool response formatting, nav strip rendering

**Functions (all pure: types → string):**
- `NavStrip(NavState) → string`
- `NoteResponse(*Artifact, path, NoteValidation, navStrip) → string`
- `NoteRejection(NoteValidation, navStrip) → string`
- `ProblemResponse(action, *Artifact, path, navStrip) → string`
- `ProblemsListResponse([]ProblemListItem, navStrip) → string`
- `SolutionResponse(action, *Artifact, path, navStrip) → string`
- `MissingProblemResponse(navStrip) → string`
- `DecisionResponse(action, *Artifact, path, extra, navStrip) → string`
- `BaselineResponse(ref, []AffectedFile, navStrip) → string`
- `DriftResponse([]DriftReport, navStrip) → string`
- `ScanResponse([]StaleItem, navStrip) → string`
- `RefreshActionResponse(RefreshAction, *Artifact, *Artifact, navStrip) → string`
- `ReconcileResponse([]ReconcileOverlap, navStrip) → string`

**Inexpressible:**
- Store access (no ArtifactStore in any signature)
- Side effects (no context.Context in any signature)

**Depends on:** LAYER 0 (types only, one-way, no cycles)

### LAYER 3: Orchestration (`cmd/serve.go`)

**Concepts:** Tool dispatch, cross-project recall, cross-project indexing, audit logging, refresh reminders

**Functions:**
- `makeV5Handler(store, dir, cfg, index) → V5ToolHandler` — ~15 lines of orchestration
- `dispatchTool(ctx, store, dir, name, args) → (string, error)` — pure switch
- `logToolEntry(name, action, args)` — isolated logging
- `applyCrossProjectRecall(ctx, result, ...) → string` — post-frame hook
- `applyCrossProjectIndex(ctx, ...) → void` — post-decide hook
- `applyRefreshReminder(ctx, result, ...) → string` — periodic prompt
- `handleQuintNote/Problem/Solution/Decision/Refresh/Query(...)` — per-tool handlers

**Inexpressible:**
- Direct DB access except through ArtifactStore interface
- Formatting (delegated to LAYER 2)

**Depends on:** LAYER 1 (domain), LAYER 2 (present), LAYER 0 (types)

### LAYER 4: Transport (`internal/fpf/server.go`)

**Concepts:** JSON-RPC 2.0 protocol, MCP tool schemas, stdio communication

**Inexpressible:**
- Domain logic (delegated to V5ToolHandler callback)
- Formatting (never touches response content)

**Depends on:** LAYER 3 (via V5ToolHandler callback)

## Compilation Chain

How a top-level MCP tool call compiles down to Go data structures:

```
MCP JSON-RPC request (stdin)
  │
  ▼ LAYER 4: server.go parses JSON-RPC, extracts tool name + arguments
  │
  ▼ LAYER 3: makeV5Handler dispatches to handler
  │           logToolEntry records the call
  │
  ▼ LAYER 3: handleQuintProblem extracts typed args from map[string]any
  │           calls dispatchTool → LAYER 1
  │
  ▼ LAYER 1: FrameProblem orchestrator
  │           ├─ store.NextSequence (INFRA: DB read)
  │           ├─ recallRelated (INFRA: FTS5 search)
  │           ├─ BuildProblemArtifact (PURE: input → Artifact)
  │           │    ├─ ParseMode(input.Mode) → Mode (validates)
  │           │    ├─ builds markdown body (string concatenation)
  │           │    ├─ json.Marshal(ProblemFields{...}) → structured_data
  │           │    └─ returns *Artifact (no side effects)
  │           ├─ store.Create (INFRA: DB write)
  │           └─ WriteFile (INFRA: filesystem write)
  │
  ▼ LAYER 3: applyCrossProjectRecall (appends history if applicable)
  │           applyCrossProjectIndex (writes to global index if decide)
  │           logAudit (fire-and-forget DB write)
  │           applyRefreshReminder (appends if stale)
  │
  ▼ LAYER 2: present.ProblemResponse (PURE: Artifact → string)
  │           present.NavStrip(ComputeNavState(...)) (PURE: NavState → string)
  │
  ▼ LAYER 4: server.go wraps in JSON-RPC response, writes to stdout

Result: Artifact persisted in DB + .quint/*.md + formatted MCP response
```

Every transformation is traceable. Pure functions are marked. Effects are at named boundaries.

## Data Flow: Structured Fields

```
Input (MCP JSON args)
  │
  ▼ BuildProblemArtifact
  │   ├─ markdown body (human-readable, .quint/*.md)
  │   └─ structured_data JSON (machine-readable, DB column)
  │       {"signal":"...", "constraints":[...], "acceptance":"..."}
  │
  ▼ store.Create → artifacts table
  │   ├─ content = markdown body
  │   └─ structured_data = JSON
  │
  ▼ BuildDecisionArtifact reads structured_data from problem
  │   (via store.Get → json.Unmarshal → ProblemFields)
  │   instead of re-parsing markdown with string.Index

One canonical representation (structured JSON). Markdown is the projection.
```
