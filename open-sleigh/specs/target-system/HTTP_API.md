---
title: "15. Optional HTTP Observability API (MVP-1 minimum)"
description: The operator-facing read-only HTTP surface. Observability only — never mutates state, never exposes Haft artifacts. MVP-2 replaces with LiveView.
reading_order: 15
---

# Open-Sleigh: Optional HTTP Observability API

> **FPF note.** This surface is an **operator-facing `Carrier`** for the
> `Orchestrator` state and `ObservationsBus` reads. It is NOT a `Carrier`
> for Haft artifacts — reading Haft through HTTP would reintroduce the
> very proxy pattern `ILLEGAL_STATES.md` OB5 forbids. The engine runs
> correctly whether this server is enabled or not; operator convenience
> must not become a correctness dependency.
>
> Inherited from Symphony §13.7 as an operator surface. Optional in
> MVP-1 but recommended — a terminal-only status dashboard covers the
> same need. MVP-2 replaces this with LiveView.

---

## 1. Enablement

- `engine.status_http.enabled: true` in `sleigh.md` enables the server.
- `engine.status_http.host` defaults to `127.0.0.1` (loopback).
- `engine.status_http.port` defaults to `4767`.
- Changes to `engine.status_http.*` require restart.

## 2. Hard boundary

The HTTP API is **observability / control surface only**. It never
exposes or mutates Haft artifacts. It reads from the runtime status
snapshot at `engine.status_path`, then redacts secret-like keys and Haft
artifact body carriers before returning JSON or HTML.

It does NOT read from `Haft.Client`. This preserves OB isolation (see
`ILLEGAL_STATES.md` OB1–OB5 and `HAFT_CONTRACT.md §4`).

## 3. Minimum endpoints

| Endpoint | Returns | Notes |
|---|---|---|
| `GET /api/v1/state` | Redacted runtime status snapshot | Same data as `mix open_sleigh.status --json` after redaction |
| `GET /dashboard` | Minimal browser dashboard | HTML summary plus redacted JSON snapshot |
| `GET /` | Same dashboard | Convenience route |
| `GET /healthz` | `{"ok": true}` | Local liveness check |

JSON endpoints return `application/json`; the dashboard returns
`text/html`. Unknown routes return `404`. Missing or malformed status
snapshots return `503`.

## 4. Observability-only guarantee

`Orchestrator` correctness does NOT depend on the HTTP server running.
If it crashes, the engine keeps working. No phase transition, gate
evaluation, or Haft write is ever bounded by HTTP-server liveness.

The tracker is the only external I/O path that is correctness-
relevant; HTTP is convenience.

---

## See also

- [HAFT_CONTRACT.md](HAFT_CONTRACT.md) — §4 (token-accounting isolation — why HTTP never reads Haft artifacts)
- [ILLEGAL_STATES.md](ILLEGAL_STATES.md) — OB1–OB5 observation isolation invariants that HTTP must respect
- [../enabling-system/STACK_DECISION.md](../enabling-system/STACK_DECISION.md) — "No web framework; no load-bearing HTTP" rule that this endpoint conforms to (read-only, observability-only)
- [SLEIGH_CONFIG.md](SLEIGH_CONFIG.md) — `engine.status_http` configuration surface
