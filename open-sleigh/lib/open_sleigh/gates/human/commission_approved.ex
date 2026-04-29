defmodule OpenSleigh.Gates.Human.CommissionApproved do
  @moduledoc """
  Human gate — the MVP-1 Transformer-Mandate enforcement point.
  Fires on:

  * `phase == :execute` AND `ticket.target_branch` matches the
    `external_publication.branch_regex` (default `^(main|master|release/.*)$`)
  * Any tracker-transition to a terminal state

  Per `specs/target-system/GATES.md §4`: "the agent can verify framing,
  can implement, can
  request approval — but it cannot author framing, cannot
  unilaterally publish to `main`, and cannot close a ticket."

  This module is metadata only. The L5 `HumanGateListener` resolves
  this binding into a live pending gate and waits for `/approve` on
  the tracker (or a `:rejected` / `:timeout`).
  """

  @behaviour OpenSleigh.Gates.Human

  alias OpenSleigh.{Phase, Ticket}

  @gate_name :commission_approved

  @impl true
  @spec gate_name() :: atom()
  def gate_name, do: @gate_name

  @impl true
  @spec fires?(Phase.t(), Ticket.t(), map()) :: boolean()
  def fires?(:execute, %Ticket{target_branch: branch}, config) when is_binary(branch) do
    regex = Map.get(config, :branch_regex, "^(main|master|release/.*)$")
    String.match?(branch, Regex.compile!(regex))
  end

  def fires?(_phase, _ticket, _config), do: false

  @impl true
  @spec render_request(Phase.t(), Ticket.t()) :: String.t()
  def render_request(phase, %Ticket{id: ticket_id, target_branch: branch}) do
    """
    **Open-Sleigh HumanGate — `commission_approved`**

    Phase #{inspect(phase)} on ticket `#{ticket_id}` produced a PR
    targeting `#{branch || "(unknown branch)"}`. Because the branch
    matches the `external_publication` regex, the Transformer
    Mandate requires a human approval before the session advances
    to Measure.

    Reply with `/approve` (approve and continue) or `/reject <reason>`
    (send back to Execute) to this comment.

    Default timeout: 24h. No reply after 24h → escalation comment;
    after 72h → session cancelled.
    """
  end

  @impl true
  @spec description() :: String.t()
  def description,
    do: "Human must approve external-publication transitions (PR→main, ticket→terminal)."
end
