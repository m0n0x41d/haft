defmodule OpenSleigh.RunAttemptSubState do
  @moduledoc """
  Closed sum of run-attempt sub-states — the 5th axis of the phase
  ontology (v0.6, Symphony-inherited).

  Canonical in `specs/target-system/PHASE_ONTOLOGY.md` axis 5 (with the
  per-sub-state retry policy table). SPEC.md §5.2 carries an abstract +
  pointer only.

  Inside a single phase session, the `AgentWorker` passes through these
  sub-states. Each has a distinct failure-retry policy so logs, metrics,
  and reopen decisions can differentiate them.

  This axis is **runtime-only** — it lives on `Session.t()` (L1) and
  dies with the session. It does NOT appear in persisted `PhaseOutcome`;
  persisted artifacts record the terminal verdict, not the worker's
  internal state machine.
  """

  @typedoc "A run-attempt sub-state atom."
  @type t ::
          :preparing_workspace
          | :building_prompt
          | :launching_agent_process
          | :initializing_session
          | :streaming_turn
          | :finishing
          | :succeeded
          | :failed
          | :timed_out
          | :stalled
          | :canceled_by_reconciliation

  @all [
    :preparing_workspace,
    :building_prompt,
    :launching_agent_process,
    :initializing_session,
    :streaming_turn,
    :finishing,
    :succeeded,
    :failed,
    :timed_out,
    :stalled,
    :canceled_by_reconciliation
  ]

  @terminal [:succeeded, :failed, :timed_out, :stalled, :canceled_by_reconciliation]

  @retryable [:failed, :timed_out, :stalled]

  @doc "All admissible run-attempt sub-states."
  @spec all() :: [t(), ...]
  def all, do: @all

  @doc "Is `value` a valid sub-state?"
  @spec valid?(term()) :: boolean()
  def valid?(value) when value in @all, do: true
  def valid?(_), do: false

  @doc """
  Is `sub_state` terminal for the current run attempt? (The session may
  still retry if the sub-state is also retryable — see `retryable?/1`.)
  """
  @spec terminal?(t()) :: boolean()
  def terminal?(sub_state) when sub_state in @terminal, do: true
  def terminal?(_), do: false

  @doc """
  Should a terminal-`:failed`-class sub-state trigger a retry with
  exponential backoff? `:canceled_by_reconciliation` and `:succeeded`
  do NOT retry (per `specs/target-system/PHASE_ONTOLOGY.md §Axis 5 —
  per-sub-state retry policy`).
  """
  @spec retryable?(t()) :: boolean()
  def retryable?(sub_state) when sub_state in @retryable, do: true
  def retryable?(_), do: false
end
