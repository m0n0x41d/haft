# Quint Code v5 — Known Issues

Date: 2026-03-17
Source: self-review after Cycles A-D

## SHOULD_FIX (next cycle)

### S1. NextSequence race condition
**File:** `internal/artifact/store.go:269` — `NextSequence`
**Issue:** SELECT COUNT then return count+1 is a TOCTOU race. Two concurrent calls on same day produce duplicate IDs.
**Fix:** Use INSERT...RETURNING or atomic sequence table.

### S2. ReopenDecision lineage scan inefficiency + duplicate append
**File:** `internal/artifact/refresh.go:171-199` — `ReopenDecision`
**Issue:** Loops 100→1 doing strings.Index per iteration. Also appends both versioned AND old-style characterization without `else`, producing duplicate lineage content.
**Fix:** Count up from 1, break on first miss. Add `else` between versioned and old-style paths. Fix forward scan offset to use `idx + len(marker)`.

### S3. FindStaleArtifacts error silently discarded
**File:** `internal/artifact/refresh.go:48` — `ScanStale`
**Issue:** `staleOther, _ := store.FindStaleArtifacts(ctx)` discards error. Partial results shown silently.
**Fix:** Log error or return as warning.

### S4. AttachEvidence CL=0 silently upgraded to CL=3
**File:** `internal/artifact/decision.go:410-415` — `AttachEvidence`
**Issue:** `CongruenceLevel == 0` means "opposed context" but is treated as "not provided" and defaulted to 3. Same for FormalityLevel=0 → 5.
**Fix:** Use pointer types or sentinel value -1 for "not set".

### S5. ReopenDecision ignores store.Update and AddLink errors
**File:** `internal/artifact/refresh.go:230-234` — `ReopenDecision`
**Issue:** After appending lineage body, `store.Update(ctx, newProb)` and `store.AddLink(...)` errors are discarded.
**Fix:** Check errors, collect as warnings.

### S6. SelectProblems ignores limit when contextFilter is set
**File:** `internal/artifact/problem.go:201-221` — `SelectProblems`
**Issue:** Context-filtered path returns all matching problems without applying `limit`.
**Fix:** `if len(problems) > limit { problems = problems[:limit] }`

### S7. FindActiveProblem returns non-active problems when no context
**File:** `internal/artifact/problem.go:224-249` — `FindActiveProblem`
**Issue:** No-context path uses `ListByKind` which has no status filter. Context path correctly filters `StatusActive`.
**Fix:** Add status filter or use `ListActive` + kind filter.

### S8. ComputeWLNKSummary missing default in CL label map
**File:** `internal/artifact/decision.go:515` — `ComputeWLNKSummary`
**Issue:** CL values outside 0-2 produce empty label string.
**Fix:** Add default case or `ok` check on map access.

### S9. server.go toolName parameter ignored
**File:** `internal/fpf/server.go:543 + cmd/serve.go:73`
**Issue:** `handleToolsCall` passes `req.Params` (full envelope) to v5Handler. The handler re-extracts `params.Name` from it, ignoring the `toolName` argument. Works by coincidence — both extract the same `name` field.
**Fix:** Either pass `params.Arguments` (the actual args) and use `toolName`, or document the contract.

## NITPICK (low priority)

- `truncate` works on byte length, not rune length — breaks multi-byte UTF-8
- `writeFileQuiet` uses stderr instead of logger package
- No validation of verdict enum values in Measure/AttachEvidence
- QueryStatus calls ListByContext twice when contextFilter is set
- `FormatDecisionResponse` accepts nil artifact for "apply" action without documenting it
- ScanStale overwrites reason for refresh_due + expired combo
- parseFrontmatter accepts bare `ref:` without dash in links section
