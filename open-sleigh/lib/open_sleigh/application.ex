defmodule OpenSleigh.Application do
  @moduledoc """
  Root OTP application.

  For MVP-1 the supervision tree is deliberately minimal — only the
  observation-and-workspace pieces that every engine instance needs
  regardless of which adapters are wired. The session-specific
  pieces (Orchestrator, TrackerPoller, HaftSupervisor, WorkflowStore)
  are started by the operator `mix open_sleigh.start` task or by
  `test_helper.exs` for tests, so the tree stays decoupled from
  adapter choice.

  Default child here is just a `Task.Supervisor` for `AgentWorker`
  tasks plus the `ObservationsBus`.

  Per `specs/enabling-system/FUNCTIONAL_ARCHITECTURE.md` Thai-disaster
  guardrail: `ObservationsBus` is present, `Haft.Client` is NOT
  started at the application level — it is introduced only when
  `HaftSupervisor` is wired by the operator / test harness. This
  preserves OB1 (zero compile-time path from ObservationsBus to
  Haft.Client even at application-start time).
  """

  use Application

  alias OpenSleigh.ObservationsBus

  @impl true
  def start(_type, _args) do
    children = [
      {Task.Supervisor, name: OpenSleigh.AgentSupervisor},
      ObservationsBus
    ]

    opts = [strategy: :one_for_one, name: OpenSleigh.Supervisor]
    Supervisor.start_link(children, opts)
  end
end
