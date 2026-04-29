defmodule OpenSleigh.RuntimeStatusWriter do
  @moduledoc """
  Periodically writes a runtime status snapshot to disk.

  This is the cross-process carrier for the CLI operator surface:
  `mix open_sleigh.start` owns the live GenServers, while a later
  `mix open_sleigh.status` invocation runs in a separate BEAM VM and
  can only read durable state.
  """

  use GenServer

  alias OpenSleigh.{ObservationsBus, Orchestrator}

  @default_interval_ms 5_000
  @failure_display_limit 20
  @failure_metrics [
    :dispatch_failed,
    :hook_failed,
    :session_errored,
    :human_gate_resume_failed,
    :tracker_transition_failed
  ]

  @type opts :: [
          orchestrator: GenServer.server(),
          path: Path.t(),
          metadata: map(),
          interval_ms: non_neg_integer(),
          name: atom()
        ]

  @type runtime_identity ::
          {:commission, String.t(), legacy_ticket_id :: String.t() | nil}
          | {:ticket, String.t()}
          | :none

  @spec start_link(opts()) :: GenServer.on_start()
  def start_link(opts) do
    case Keyword.fetch(opts, :name) do
      {:ok, name} -> GenServer.start_link(__MODULE__, opts, name: name)
      :error -> GenServer.start_link(__MODULE__, opts)
    end
  end

  @doc "Force an immediate status write."
  @spec write(GenServer.server()) :: :ok
  def write(server), do: GenServer.call(server, :write)

  @impl true
  def init(opts) do
    metadata = Keyword.get(opts, :metadata, %{})

    state = %{
      orchestrator: Keyword.fetch!(opts, :orchestrator),
      path:
        opts
        |> Keyword.fetch!(:path)
        |> runtime_path(metadata, ".json"),
      metadata: metadata,
      interval_ms: Keyword.get(opts, :interval_ms, @default_interval_ms)
    }

    :ok = write_snapshot(state)
    {:ok, schedule_tick(state)}
  end

  @impl true
  def handle_call(:write, _from, state) do
    :ok = write_snapshot(state)
    {:reply, :ok, state}
  end

  @impl true
  def handle_info(:tick, state) do
    :ok = write_snapshot(state)
    {:noreply, schedule_tick(state)}
  end

  @spec schedule_tick(map()) :: map()
  defp schedule_tick(%{interval_ms: interval_ms} = state) when interval_ms > 0 do
    ref = Process.send_after(self(), :tick, interval_ms)
    Map.put(state, :timer_ref, ref)
  end

  defp schedule_tick(state), do: state

  @spec write_snapshot(map()) :: :ok
  defp write_snapshot(state) do
    state
    |> snapshot()
    |> Jason.encode!(pretty: true)
    |> write_file(state.path)
  end

  @spec snapshot(map()) :: map()
  defp snapshot(state) do
    observations = ObservationsBus.snapshot()
    identity = runtime_identity(state.metadata)

    %{
      updated_at: DateTime.utc_now(),
      metadata: state.metadata,
      orchestrator: Orchestrator.status(state.orchestrator),
      human_gates:
        state.orchestrator
        |> Orchestrator.pending_human_gates()
        |> Enum.map(&normalize_identity_payload/1),
      observations: observations,
      failures: recent_failures(observations)
    }
    |> put_identity_fields(identity)
    |> serialise()
  end

  @spec recent_failures([map()]) :: [map()]
  defp recent_failures(observations) do
    observations
    |> Enum.filter(&failure_observation?/1)
    |> Enum.sort_by(&Map.get(&1, :at, 0), :desc)
    |> Enum.map(&failure_view/1)
    |> Enum.take(@failure_display_limit)
  end

  @spec failure_observation?(map()) :: boolean()
  defp failure_observation?(%{metric: metric}) do
    metric in @failure_metrics
  end

  defp failure_observation?(_observation), do: false

  @spec failure_view(map()) :: map()
  defp failure_view(observation) do
    identity = observation_identity(observation)

    %{
      metric: Map.get(observation, :metric),
      reason: Map.get(observation, :value),
      ticket: failure_tag(observation, :ticket),
      session_id: failure_tag(observation, :session_id),
      phase: failure_tag(observation, :phase),
      hook: failure_tag(observation, :hook),
      policy: failure_tag(observation, :policy),
      target: failure_tag(observation, :target),
      at_ms: Map.get(observation, :at)
    }
    |> put_identity_fields(identity)
    |> Enum.reject(fn {_key, value} -> is_nil(value) end)
    |> Map.new()
  end

  @spec failure_tag(map(), atom()) :: term()
  defp failure_tag(%{tags: tags}, key) when is_map(tags) do
    tags
    |> Map.get(key, Map.get(tags, Atom.to_string(key)))
  end

  defp failure_tag(_observation, _key), do: nil

  @spec observation_identity(map()) :: runtime_identity()
  defp observation_identity(%{tags: tags}) when is_map(tags) do
    runtime_identity(tags)
  end

  defp observation_identity(_observation), do: :none

  @spec write_file(binary(), Path.t()) :: :ok
  defp write_file(encoded, path) do
    path
    |> Path.dirname()
    |> File.mkdir_p!()

    File.write!(path, encoded <> "\n")
  end

  @spec runtime_path(Path.t(), map(), String.t()) :: Path.t()
  defp runtime_path(path, metadata, extension) do
    metadata
    |> runtime_identity()
    |> identity_path(path, extension)
  end

  @spec identity_path(runtime_identity(), Path.t(), String.t()) :: Path.t()
  defp identity_path({:commission, commission_id, _legacy_ticket_id}, path, extension) do
    scoped_path(path, commission_id, extension)
  end

  defp identity_path({:ticket, ticket_id}, path, extension) do
    scoped_path(path, ticket_id, extension)
  end

  defp identity_path(:none, path, _extension), do: path

  @spec scoped_path(Path.t(), String.t(), String.t()) :: Path.t()
  defp scoped_path(path, identity, extension) do
    if Path.extname(path) == "" do
      path
      |> Path.join(safe_identity(identity) <> extension)
    else
      path
    end
  end

  @spec safe_identity(String.t()) :: String.t()
  defp safe_identity(identity) do
    String.replace(identity, ~r/[^A-Za-z0-9._-]/, "_")
  end

  @spec normalize_identity_payload(map()) :: map()
  defp normalize_identity_payload(%{} = payload) do
    payload
    |> put_identity_fields(runtime_identity(payload))
  end

  @spec runtime_identity(map()) :: runtime_identity()
  defp runtime_identity(%{} = attrs) do
    attrs
    |> identity_pair()
    |> runtime_identity_result()
  end

  @spec identity_pair(map()) :: {String.t() | nil, String.t() | nil}
  defp identity_pair(attrs) do
    {
      string_value(attrs, :commission_id),
      string_value(attrs, :ticket_id) || string_value(attrs, :ticket)
    }
  end

  @spec runtime_identity_result({String.t() | nil, String.t() | nil}) :: runtime_identity()
  defp runtime_identity_result({commission_id, ticket_id})
       when is_binary(commission_id) and commission_id != "" do
    {:commission, commission_id, ticket_id}
  end

  defp runtime_identity_result({_commission_id, ticket_id})
       when is_binary(ticket_id) and ticket_id != "" do
    {:ticket, ticket_id}
  end

  defp runtime_identity_result(_pair), do: :none

  @spec string_value(map(), atom()) :: String.t() | nil
  defp string_value(attrs, key) do
    attrs
    |> Map.get(key, Map.get(attrs, Atom.to_string(key)))
    |> non_empty_string()
  end

  @spec non_empty_string(term()) :: String.t() | nil
  defp non_empty_string(value) when is_binary(value) and value != "", do: value
  defp non_empty_string(_value), do: nil

  @spec put_identity_fields(map(), runtime_identity()) :: map()
  defp put_identity_fields(payload, {:commission, commission_id, legacy_ticket_id}) do
    payload
    |> Map.put(:commission_id, commission_id)
    |> maybe_put_legacy_ticket_id(legacy_ticket_id, commission_id)
  end

  defp put_identity_fields(payload, {:ticket, ticket_id}) do
    Map.put(payload, :ticket_id, ticket_id)
  end

  defp put_identity_fields(payload, :none), do: payload

  @spec maybe_put_legacy_ticket_id(map(), String.t() | nil, String.t()) :: map()
  defp maybe_put_legacy_ticket_id(payload, legacy_ticket_id, commission_id)
       when is_binary(legacy_ticket_id) and legacy_ticket_id != commission_id do
    Map.put(payload, :legacy_ticket_id, legacy_ticket_id)
  end

  defp maybe_put_legacy_ticket_id(payload, _legacy_ticket_id, _commission_id), do: payload

  @spec serialise(term()) :: term()
  defp serialise(nil), do: nil
  defp serialise(boolean) when is_boolean(boolean), do: boolean
  defp serialise(%DateTime{} = datetime), do: DateTime.to_iso8601(datetime)
  defp serialise(%MapSet{} = set), do: set |> MapSet.to_list() |> serialise()

  defp serialise(%{} = map) do
    map
    |> Enum.map(fn {key, value} -> {serialise_key(key), serialise(value)} end)
    |> Map.new()
  end

  defp serialise(list) when is_list(list), do: Enum.map(list, &serialise/1)
  defp serialise(tuple) when is_tuple(tuple), do: inspect(tuple)
  defp serialise(pid) when is_pid(pid), do: inspect(pid)
  defp serialise(ref) when is_reference(ref), do: inspect(ref)
  defp serialise(atom) when is_atom(atom), do: Atom.to_string(atom)
  defp serialise(value), do: value

  @spec serialise_key(term()) :: String.t()
  defp serialise_key(key) when is_atom(key), do: Atom.to_string(key)
  defp serialise_key(key) when is_binary(key), do: key
  defp serialise_key(key), do: to_string(key)
end
