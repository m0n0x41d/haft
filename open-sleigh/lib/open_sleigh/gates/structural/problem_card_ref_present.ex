defmodule OpenSleigh.Gates.Structural.ProblemCardRefPresent do
  @moduledoc """
  Structural gate — fires at **Frame entry** (not exit — this is a
  prerequisite check before the agent even runs).

  Per `specs/target-system/GATES.md §1` + `ILLEGAL_STATES.md` UP1 (v0.5
  hardened from ⚠️ to ❌):
  every MVP-1 Ticket MUST carry a valid `problem_card_ref` that
  resolves to a live Haft artifact NOT authored by Open-Sleigh itself.

  L1 `Ticket.new/1` already rejects nil / empty refs. This gate adds
  the **resolvability + authorship** check that requires L4 Haft
  access:

  1. `ctx.ticket.problem_card_ref` is non-nil → L1 already ensured
  2. `ctx.upstream_problem_card` is non-nil → L5 dispatcher fetched it
  3. `ctx.upstream_problem_card.authoring_source != :open_sleigh_self`
     → UP3 backstop

  If (2) fails, the L5 dispatcher should set
  `upstream_problem_card: nil` after a failed `haft_query`.
  """

  @behaviour OpenSleigh.Gates.Structural

  alias OpenSleigh.GateContext

  @gate_name :problem_card_ref_present

  @impl true
  @spec gate_name() :: atom()
  def gate_name, do: @gate_name

  @impl true
  @spec apply(GateContext.t()) ::
          :ok
          | {:error, :no_upstream_frame}
          | {:error, :upstream_self_authored}
  def apply(%GateContext{upstream_problem_card: nil}),
    do: {:error, :no_upstream_frame}

  def apply(%GateContext{upstream_problem_card: %{authoring_source: :open_sleigh_self}}),
    do: {:error, :upstream_self_authored}

  def apply(%GateContext{}), do: :ok

  @impl true
  @spec description() :: String.t()
  def description,
    do:
      "Ticket has a valid upstream ProblemCardRef that resolves to a non-self-authored Haft artifact (UP1/UP3)."
end
