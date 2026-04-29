defmodule OpenSleigh.AdapterSession do
  @moduledoc """
  L4 effect context passed to every adapter call. Carries the session-
  level metadata that every I/O call needs.

  Per `specs/target-system/TARGET_SYSTEM_MODEL.md §AdapterSession` +
  `specs/target-system/AGENT_PROTOCOL.md §1`.

  This struct is L1 pure data. The L5 `AgentWorker` constructs it at
  session spawn and passes it verbatim to every L4 adapter call. L4
  functions take an `AdapterSession.t()` — they never own session
  state themselves (v0.6.1 L4/L5 ownership seam).
  """

  alias OpenSleigh.{ConfigHash, SessionId}

  @enforce_keys [
    :session_id,
    :config_hash,
    :scoped_tools,
    :workspace_path,
    :adapter_kind,
    :adapter_version,
    :max_turns,
    :max_tokens_per_turn,
    :wall_clock_timeout_s
  ]
  defstruct [
    :session_id,
    :config_hash,
    :scoped_tools,
    :workspace_path,
    :adapter_kind,
    :adapter_version,
    :max_turns,
    :max_tokens_per_turn,
    :wall_clock_timeout_s
  ]

  @type t :: %__MODULE__{
          session_id: SessionId.t(),
          config_hash: ConfigHash.t(),
          scoped_tools: MapSet.t(atom()),
          workspace_path: Path.t(),
          adapter_kind: atom(),
          adapter_version: String.t(),
          max_turns: pos_integer(),
          max_tokens_per_turn: pos_integer(),
          wall_clock_timeout_s: pos_integer()
        }

  @type new_error ::
          :invalid_session_id
          | :invalid_config_hash
          | :invalid_scoped_tools
          | :invalid_workspace_path
          | :invalid_adapter_kind
          | :invalid_adapter_version
          | :invalid_max_turns
          | :invalid_max_tokens_per_turn
          | :invalid_wall_clock_timeout_s

  @doc """
  Construct an `AdapterSession`. L1 performs structural checks only.
  The `workspace_path` must already be a `PathGuard`-validated
  canonical path from L4 — L1 checks it is a non-empty binary; the
  PathGuard canonicalisation (symlink / hardlink / `.git` remote
  check) happens at L5 `Session.new/1` before this struct is built.
  """
  @spec new(keyword() | map()) :: {:ok, t()} | {:error, new_error()}
  def new(attrs) when is_list(attrs), do: new(Map.new(attrs))

  def new(%{} = attrs) do
    with :ok <- validate_session_id(attrs[:session_id]),
         :ok <- validate_config_hash(attrs[:config_hash]),
         :ok <- validate_scoped_tools(attrs[:scoped_tools]),
         :ok <- validate_workspace_path(attrs[:workspace_path]),
         :ok <- validate_adapter_kind(attrs[:adapter_kind]),
         :ok <- validate_adapter_version(attrs[:adapter_version]),
         :ok <- validate_max_turns(attrs[:max_turns]),
         :ok <- validate_max_tokens(attrs[:max_tokens_per_turn]),
         :ok <- validate_timeout(attrs[:wall_clock_timeout_s]) do
      {:ok,
       %__MODULE__{
         session_id: attrs.session_id,
         config_hash: attrs.config_hash,
         scoped_tools: attrs.scoped_tools,
         workspace_path: attrs.workspace_path,
         adapter_kind: attrs.adapter_kind,
         adapter_version: attrs.adapter_version,
         max_turns: attrs.max_turns,
         max_tokens_per_turn: attrs.max_tokens_per_turn,
         wall_clock_timeout_s: attrs.wall_clock_timeout_s
       }}
    end
  end

  @spec validate_session_id(term()) :: :ok | {:error, :invalid_session_id}
  defp validate_session_id(id) do
    if SessionId.valid?(id), do: :ok, else: {:error, :invalid_session_id}
  end

  @spec validate_config_hash(term()) :: :ok | {:error, :invalid_config_hash}
  defp validate_config_hash(hash) do
    if ConfigHash.valid?(hash), do: :ok, else: {:error, :invalid_config_hash}
  end

  @spec validate_scoped_tools(term()) :: :ok | {:error, :invalid_scoped_tools}
  defp validate_scoped_tools(%MapSet{} = set) do
    if Enum.all?(set, &(is_atom(&1) and not is_nil(&1))) do
      :ok
    else
      {:error, :invalid_scoped_tools}
    end
  end

  defp validate_scoped_tools(_), do: {:error, :invalid_scoped_tools}

  @spec validate_workspace_path(term()) :: :ok | {:error, :invalid_workspace_path}
  defp validate_workspace_path(path) when is_binary(path) and byte_size(path) > 0, do: :ok
  defp validate_workspace_path(_), do: {:error, :invalid_workspace_path}

  @spec validate_adapter_kind(term()) :: :ok | {:error, :invalid_adapter_kind}
  defp validate_adapter_kind(kind) when is_atom(kind) and not is_nil(kind), do: :ok
  defp validate_adapter_kind(_), do: {:error, :invalid_adapter_kind}

  @spec validate_adapter_version(term()) :: :ok | {:error, :invalid_adapter_version}
  defp validate_adapter_version(v) when is_binary(v) and byte_size(v) > 0, do: :ok
  defp validate_adapter_version(_), do: {:error, :invalid_adapter_version}

  @spec validate_max_turns(term()) :: :ok | {:error, :invalid_max_turns}
  defp validate_max_turns(n) when is_integer(n) and n >= 1, do: :ok
  defp validate_max_turns(_), do: {:error, :invalid_max_turns}

  @spec validate_max_tokens(term()) :: :ok | {:error, :invalid_max_tokens_per_turn}
  defp validate_max_tokens(n) when is_integer(n) and n >= 1, do: :ok
  defp validate_max_tokens(_), do: {:error, :invalid_max_tokens_per_turn}

  @spec validate_timeout(term()) :: :ok | {:error, :invalid_wall_clock_timeout_s}
  defp validate_timeout(n) when is_integer(n) and n >= 1, do: :ok
  defp validate_timeout(_), do: {:error, :invalid_wall_clock_timeout_s}
end
