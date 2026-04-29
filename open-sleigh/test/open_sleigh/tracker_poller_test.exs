defmodule OpenSleigh.TrackerPollerTest do
  use ExUnit.Case, async: false

  alias OpenSleigh.CommissionSource.Intake
  alias OpenSleigh.{ObservationsBus, Scope, Tracker.Mock, TrackerPoller, WorkCommission}

  defmodule FakeOrchestrator do
    @moduledoc false
    use GenServer

    @spec start_link(keyword()) :: GenServer.on_start()
    def start_link(_opts),
      do: GenServer.start(__MODULE__, :ok, name: __MODULE__)

    @impl true
    @spec init(:ok) :: {:ok, map()}
    def init(:ok), do: {:ok, %{received: []}}

    @impl true
    def handle_cast({:candidates, tickets}, state),
      do: {:noreply, %{state | received: [tickets | state.received]}}

    @spec received() :: [[OpenSleigh.Ticket.t()]]
    def received, do: GenServer.call(__MODULE__, :received)

    @impl true
    def handle_call(:received, _from, state),
      do: {:reply, Enum.reverse(state.received), state}
  end

  defmodule FakeCommissionSource do
    @moduledoc false

    def list_runnable(%{owner: owner, commission: commission}) do
      send(owner, :commission_source_listed)
      {:ok, [commission]}
    end

    def claim_for_preflight(%{owner: owner, commission: commission}, commission_id) do
      send(owner, {:commission_source_claimed, commission_id})

      commission
      |> Map.put(:state, :preflighting)
      |> then(&{:ok, &1})
    end
  end

  setup do
    ObservationsBus.reset()

    {:ok, tracker} = Mock.start()

    :ok =
      Mock.seed(tracker, [
        %{
          id: "OCT-9",
          source: {:linear, "oct"},
          title: "T",
          body: "",
          state: :in_progress,
          problem_card_ref: "haft-pc-9",
          fetched_at: ~U[2026-04-22 10:00:00Z]
        }
      ])

    {:ok, _} = FakeOrchestrator.start_link([])
    on_exit(fn -> stop_fake_orchestrator() end)

    %{tracker: tracker}
  end

  defp stop_fake_orchestrator do
    case Process.whereis(FakeOrchestrator) do
      nil -> :ok
      pid -> GenServer.stop(pid)
    end
  end

  test "poke/1 triggers an immediate tick", ctx do
    name = String.to_atom("TP_#{:erlang.unique_integer([:positive])}")

    {:ok, _} =
      TrackerPoller.start_link(
        tracker_handle: ctx.tracker,
        tracker_adapter: Mock,
        orchestrator: FakeOrchestrator,
        # Very long interval so we can't race the scheduled tick.
        interval_ms: 60_000,
        name: name
      )

    :ok = TrackerPoller.poke(name)
    # Give the cast+cast round-trip a moment.
    Process.sleep(50)

    assert [[%{id: "OCT-9"}]] = FakeOrchestrator.received()
  end

  test "emits :tracker_poll observation on success", ctx do
    name = String.to_atom("TP_#{:erlang.unique_integer([:positive])}")

    {:ok, _} =
      TrackerPoller.start_link(
        tracker_handle: ctx.tracker,
        tracker_adapter: Mock,
        orchestrator: FakeOrchestrator,
        interval_ms: 60_000,
        name: name
      )

    :ok = TrackerPoller.poke(name)
    Process.sleep(50)

    assert Enum.any?(ObservationsBus.snapshot(), &(&1.metric == :tracker_poll))
  end

  test "dynamic commission source is replenished before tracker candidates", ctx do
    name = String.to_atom("TP_#{:erlang.unique_integer([:positive])}")

    source =
      Intake.source_ref(
        FakeCommissionSource,
        %{owner: self(), commission: commission_fixture!("wc-poller-001")},
        1,
        true
      )

    {:ok, _} =
      TrackerPoller.start_link(
        tracker_handle: ctx.tracker,
        tracker_adapter: Mock,
        commission_source: source,
        orchestrator: FakeOrchestrator,
        interval_ms: 60_000,
        name: name
      )

    :ok = TrackerPoller.poke(name)
    Process.sleep(50)

    assert_receive :commission_source_listed
    assert_receive {:commission_source_claimed, "wc-poller-001"}

    received =
      FakeOrchestrator.received()
      |> List.last()
      |> Enum.map(& &1.id)

    assert "OCT-9" in received
    assert "wc-poller-001" in received
    assert Enum.any?(ObservationsBus.snapshot(), &(&1.metric == :commission_source_poll))
  end

  defp commission_fixture!(id) do
    scope = scope_fixture!()

    %{
      id: id,
      decision_ref: "dec-poller",
      decision_revision_hash: "decision-r1",
      problem_card_ref: "pc-poller",
      implementation_plan_ref: "plan-poller",
      implementation_plan_revision: "plan-r1",
      scope: scope,
      scope_hash: scope.hash,
      base_sha: scope.base_sha,
      lockset: scope.lockset,
      evidence_requirements: [],
      projection_policy: :local_only,
      state: :queued,
      valid_until: ~U[2099-01-01 00:00:00Z],
      fetched_at: ~U[2026-04-22 10:00:00Z]
    }
    |> WorkCommission.new()
    |> unwrap!()
  end

  defp scope_fixture! do
    attrs = %{
      repo_ref: "local:haft",
      base_sha: "base-r1",
      target_branch: "feature/poller",
      allowed_paths: ["**/*"],
      forbidden_paths: [],
      allowed_actions: MapSet.new([:edit_files, :run_tests]),
      affected_files: ["**/*"],
      allowed_modules: [],
      lockset: ["**/*"]
    }

    {:ok, hash} = Scope.canonical_hash(attrs)

    attrs
    |> Map.put(:hash, hash)
    |> Scope.new()
    |> unwrap!()
  end

  defp unwrap!({:ok, value}), do: value
end
