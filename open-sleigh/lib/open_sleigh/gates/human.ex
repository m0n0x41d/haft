defmodule OpenSleigh.Gates.Human do
  @moduledoc """
  Behaviour contract for human gates (L2 metadata; dispatched by L5
  `HumanGateListener`).

  Per `specs/target-system/PHASE_ONTOLOGY.md §Human` +
  `specs/target-system/GATES.md §4`:

  * **Triggered, not computed.** A human gate does not return a
    `:ok`/`:error` from a function call at L2 — it is posted to the
    tracker (or GitHub), then an external signal satisfies it
    (`/approve`, `:rejected`, or `:timeout`).
  * The L2 module defines the gate's **metadata**: canonical name,
    what transition it guards, the tracker-comment template, the
    approval-timeout policy.
  * The L5 `HumanGateListener` resolves the metadata into a live
    pending gate and waits for the signal.

  Satisfied gates flow into `PhaseOutcome.gate_results` as
  `{:human, HumanGateApproval.t()}`. Per PR10 / Q-OS-3, `PhaseOutcome.new/2`
  rejects outcomes that are missing a human approval when
  `phase_config.gates.human` declared one.
  """

  alias OpenSleigh.{Phase, Ticket}

  @doc "The canonical atom name of this gate."
  @callback gate_name() :: atom()

  @doc """
  Does this gate fire for this `(phase, ticket)` pair? Per
  `specs/target-system/GATES.md §4`
  triggers, e.g. `commission_approved` fires when
  `phase == :execute` AND the ticket's target branch matches the
  `external_publication.branch_regex`.
  """
  @callback fires?(Phase.t(), Ticket.t(), external_publication_config :: map()) :: boolean()

  @doc """
  Render the tracker comment that will be posted to request approval.
  Returns a human-readable string.
  """
  @callback render_request(Phase.t(), Ticket.t()) :: String.t()

  @doc "Short human-readable description."
  @callback description() :: String.t()
end
