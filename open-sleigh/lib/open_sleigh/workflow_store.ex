defmodule OpenSleigh.WorkflowStore do
  @moduledoc """
  L5 GenServer holding the current compiled `sleigh.md` config.

  For MVP-1 skeleton (pre-L6), callers pass phase configs and
  prompts directly on `start_link/1`. When L6 `Sleigh.Compiler`
  lands, the file-watcher will drop a fresh compiled bundle here
  via `put_compiled/1`.

  Hot-reload per SPEC §8.1: in-flight sessions keep their pinned
  `config_hash`; only **new** sessions read the freshest bundle
  from this store.
  """

  use GenServer

  alias OpenSleigh.{ConfigHash, Phase, PhaseConfig}

  @typedoc "Compiled bundle shape (L6 will produce matching values)."
  @type bundle :: %{
          phase_configs: %{Phase.t() => PhaseConfig.t()},
          prompts: %{Phase.t() => String.t()},
          config_hashes: %{Phase.t() => ConfigHash.t()},
          external_publication: map(),
          engine: map(),
          tracker: map(),
          agent: map(),
          codex: map(),
          judge: map(),
          hooks: map(),
          haft: map(),
          workspace: map(),
          workflow: atom()
        }

  # ——— public API ———

  @spec start_link(keyword()) :: GenServer.on_start()
  def start_link(opts) do
    GenServer.start_link(__MODULE__, opts, name: Keyword.get(opts, :name, __MODULE__))
  end

  @doc "Fetch the current `PhaseConfig` for `phase` (or `:error`)."
  @spec phase_config(GenServer.server(), Phase.t()) ::
          {:ok, PhaseConfig.t()} | {:error, :unknown_phase}
  def phase_config(server \\ __MODULE__, phase) do
    GenServer.call(server, {:phase_config, phase})
  end

  @doc "Fetch the current prompt template for `phase`."
  @spec prompt_for(GenServer.server(), Phase.t()) ::
          {:ok, String.t()} | {:error, :unknown_phase}
  def prompt_for(server \\ __MODULE__, phase) do
    GenServer.call(server, {:prompt_for, phase})
  end

  @doc "Fetch the current compiled `config_hash` for `phase`."
  @spec config_hash_for(GenServer.server(), Phase.t()) ::
          {:ok, ConfigHash.t()} | {:error, :unknown_phase}
  def config_hash_for(server \\ __MODULE__, phase) do
    GenServer.call(server, {:config_hash_for, phase})
  end

  @doc "Fetch the `external_publication` config section."
  @spec external_publication(GenServer.server()) :: map()
  def external_publication(server \\ __MODULE__) do
    GenServer.call(server, :external_publication)
  end

  @doc "Replace the compiled bundle atomically (hot-reload)."
  @spec put_compiled(GenServer.server(), bundle()) :: :ok
  def put_compiled(server \\ __MODULE__, bundle) when is_map(bundle) do
    GenServer.call(server, {:put_compiled, bundle})
  end

  # ——— GenServer ———

  @impl true
  def init(opts) do
    state = %{
      phase_configs: Keyword.get(opts, :phase_configs, %{}),
      prompts: Keyword.get(opts, :prompts, %{}),
      config_hashes: Keyword.get(opts, :config_hashes, %{}),
      external_publication: Keyword.get(opts, :external_publication, %{}),
      workflow: Keyword.get(opts, :workflow, :mvp1)
    }

    {:ok, state}
  end

  @impl true
  def handle_call({:phase_config, phase}, _from, state) do
    {:reply, Map.fetch(state.phase_configs, phase) |> wrap_unknown(), state}
  end

  def handle_call({:prompt_for, phase}, _from, state) do
    {:reply, Map.fetch(state.prompts, phase) |> wrap_unknown(), state}
  end

  def handle_call({:config_hash_for, phase}, _from, state) do
    {:reply, Map.fetch(state.config_hashes, phase) |> wrap_unknown(), state}
  end

  def handle_call(:external_publication, _from, state) do
    {:reply, state.external_publication, state}
  end

  def handle_call({:put_compiled, bundle}, _from, _state) do
    new_state = %{
      phase_configs: Map.get(bundle, :phase_configs, %{}),
      prompts: Map.get(bundle, :prompts, %{}),
      config_hashes: Map.get(bundle, :config_hashes, %{}),
      external_publication: Map.get(bundle, :external_publication, %{}),
      workflow: Map.get(bundle, :workflow, :mvp1)
    }

    {:reply, :ok, new_state}
  end

  @spec wrap_unknown({:ok, term()} | :error) ::
          {:ok, term()} | {:error, :unknown_phase}
  defp wrap_unknown({:ok, _} = ok), do: ok
  defp wrap_unknown(:error), do: {:error, :unknown_phase}
end
