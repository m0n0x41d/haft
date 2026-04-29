defmodule OpenSleigh.ObservationsBus do
  @moduledoc """
  ETS-backed telemetry sink for anti-Goodhart indicators
  (`specs/target-system/GATES.md §5` observations list).

  **Thai-disaster guardrail core (OB1, OB5).** This module has
  **zero** `alias`, `import`, or `use` reference to
  `OpenSleigh.Haft.Client`, `OpenSleigh.Haft.Protocol`, or
  `OpenSleigh.Haft.Server`. Token counts, gate-bypass rates,
  labeller-agreement kappa, and every other observation-indicator
  value stay on this bus and can only reach the operator surface
  (HTTP API, terminal dashboard), never the Haft artifact graph.

  **OB3 type-narrowing.** `emit/3` restricts `value` to primitive
  scalars (`number | String.t | atom`). `Haft.ArtifactRef.t()` is an
  opaque struct, so it cannot be coerced into these types; any
  caller tempted to smuggle one through fails the `@spec` contract
  and Dialyzer.

  State model: one named ETS table keyed by `{metric, tag_map}`
  storing the last numeric value, count, and monotonic timestamp.
  """

  use GenServer

  @table __MODULE__

  # ——— public API ———

  @spec start_link(keyword()) :: GenServer.on_start()
  def start_link(opts \\ []) do
    GenServer.start_link(__MODULE__, :ok, name: Keyword.get(opts, :name, __MODULE__))
  end

  @typedoc """
  Allowed observation value. OB3 — no maps, no structs, no tuples.
  Primitive scalar only.
  """
  @type value :: number() | String.t() | atom()

  @doc """
  Emit an observation. `metric` is an atom (e.g. `:gate_bypass_rate`,
  `:codex_total_tokens_per_ticket`); `value` is a primitive;
  `tags` is an optional map of atom-keyed string/atom/number
  labels.

  Returns `:ok` synchronously (ETS insert is fast + bus-local).
  """
  @spec emit(atom(), value(), map()) :: :ok
  def emit(metric, value, tags \\ %{})

  def emit(metric, value, tags)
      when is_atom(metric) and (is_number(value) or is_binary(value) or is_atom(value)) and
             is_map(tags) do
    key = {metric, tags}
    now = System.monotonic_time(:millisecond)

    @table
    |> table_id()
    |> insert_observation({key, value, now})

    :ok
  end

  @doc """
  Snapshot of all observations. Returns a list of
  `%{metric, value, tags, at}` maps. Used by the HTTP API
  observability endpoint (see `specs/target-system/HTTP_API.md`).
  """
  @spec snapshot() :: [map()]
  def snapshot do
    @table
    |> table_id()
    |> tab2list()
    |> Enum.map(fn {{metric, tags}, value, at} ->
      %{metric: metric, value: value, tags: tags, at: at}
    end)
  end

  @doc "Reset all observations (test helper)."
  @spec reset() :: :ok
  def reset do
    @table
    |> table_id()
    |> delete_all_objects()

    :ok
  end

  # ——— GenServer ———

  @impl true
  def init(:ok) do
    :ets.new(@table, [:named_table, :public, :set, read_concurrency: true])
    {:ok, %{}}
  end

  @spec table_id(atom()) :: :ets.tid() | :undefined
  defp table_id(table), do: :ets.whereis(table)

  @spec insert_observation(:ets.tid() | :undefined, tuple()) :: :ok
  defp insert_observation(:undefined, _observation), do: :ok

  defp insert_observation(table, observation) do
    :ets.insert(table, observation)
    :ok
  end

  @spec tab2list(:ets.tid() | :undefined) :: [tuple()]
  defp tab2list(:undefined), do: []
  defp tab2list(table), do: :ets.tab2list(table)

  @spec delete_all_objects(:ets.tid() | :undefined) :: :ok
  defp delete_all_objects(:undefined), do: :ok

  defp delete_all_objects(table) do
    :ets.delete_all_objects(table)
    :ok
  end
end
