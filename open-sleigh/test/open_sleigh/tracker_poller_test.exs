defmodule OpenSleigh.TrackerPollerTest do
  use ExUnit.Case, async: false

  alias OpenSleigh.{ObservationsBus, Tracker.Mock, TrackerPoller}

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
end
