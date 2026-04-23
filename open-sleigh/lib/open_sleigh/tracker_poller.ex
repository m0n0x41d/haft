defmodule OpenSleigh.TrackerPoller do
  @moduledoc """
  Periodic `list_active` poller. Calls the `Tracker.Adapter` every
  `poll_interval_ms` and forwards candidates to the `Orchestrator`.

  For MVP-1 skeleton this is a simple GenServer with a
  `Process.send_after/4` self-tick. When real metrics land, the
  interval can be driven from `WorkflowStore.engine_config` +
  `sleigh.md engine.poll_interval_ms`.
  """

  use GenServer

  alias OpenSleigh.{CommissionSource.Intake, ObservationsBus, Orchestrator}

  require Logger

  @default_interval_ms 30_000

  @typedoc "Startup options."
  @type opts :: [
          tracker_handle: term(),
          tracker_adapter: module(),
          commission_source: Intake.source_ref() | nil,
          orchestrator: GenServer.server(),
          interval_ms: pos_integer(),
          name: atom()
        ]

  @spec start_link(opts()) :: GenServer.on_start()
  def start_link(opts) do
    GenServer.start_link(__MODULE__, opts, name: Keyword.get(opts, :name, __MODULE__))
  end

  @doc "Trigger an immediate tick (test helper / HTTP /refresh)."
  @spec poke(GenServer.server()) :: :ok
  def poke(server \\ __MODULE__), do: GenServer.cast(server, :poke)

  @impl true
  def init(opts) do
    state = %{
      tracker_handle: Keyword.fetch!(opts, :tracker_handle),
      tracker_adapter: Keyword.fetch!(opts, :tracker_adapter),
      commission_source: Keyword.get(opts, :commission_source),
      orchestrator: Keyword.get(opts, :orchestrator, Orchestrator),
      interval_ms: Keyword.get(opts, :interval_ms, @default_interval_ms)
    }

    schedule_tick(state.interval_ms)
    {:ok, state}
  end

  @impl true
  def handle_info(:tick, state) do
    do_poll(state)
    schedule_tick(state.interval_ms)
    {:noreply, state}
  end

  @impl true
  def handle_cast(:poke, state) do
    do_poll(state)
    {:noreply, state}
  end

  @spec do_poll(map()) :: :ok
  defp do_poll(state) do
    :ok = replenish_commission_source(state)

    case state.tracker_adapter.list_active(state.tracker_handle) do
      {:ok, tickets} ->
        :ok = ObservationsBus.emit(:tracker_poll, length(tickets), %{})
        Orchestrator.submit_candidates(state.orchestrator, tickets)

      {:error, reason} ->
        :ok = ObservationsBus.emit(:tracker_poll_failed, Atom.to_string(reason), %{})
        :ok
    end
  end

  @spec replenish_commission_source(map()) :: :ok
  defp replenish_commission_source(%{commission_source: source} = state) do
    source
    |> Intake.dynamic?()
    |> maybe_replenish_commission_source(state)
  end

  @spec maybe_replenish_commission_source(boolean(), map()) :: :ok
  defp maybe_replenish_commission_source(false, _state), do: :ok

  defp maybe_replenish_commission_source(true, state) do
    case Intake.replenish(state.commission_source, state.tracker_adapter, state.tracker_handle) do
      {:ok, claimed_count} ->
        ObservationsBus.emit(:commission_source_poll, claimed_count, %{})

      {:error, reason} ->
        ObservationsBus.emit(:commission_source_poll_failed, reason_text(reason), %{})
    end
  end

  @spec reason_text(term()) :: String.t()
  defp reason_text(reason) when is_atom(reason), do: Atom.to_string(reason)
  defp reason_text(reason), do: inspect(reason)

  @spec schedule_tick(pos_integer()) :: reference()
  defp schedule_tick(ms), do: Process.send_after(self(), :tick, ms)
end
