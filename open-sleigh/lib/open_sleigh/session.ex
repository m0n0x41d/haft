defmodule OpenSleigh.Session do
  @moduledoc """
  The runtime unit of work: one `(Ticket × Phase × ConfigHash ×
  AdapterSession)` owned by exactly one `AgentWorker`.

  Per `specs/target-system/TARGET_SYSTEM_MODEL.md §Session` (v0.6 with
  run-attempt sub-state + turn / token fields).

  L1 stores the session state; mutation flows through L5
  `Orchestrator` via message passing (SE1 single-writer). This module
  exposes pure functional transformations — every update returns a
  new `Session.t()`.

  **Workspace-path validation is NOT done here.** Per v0.6.1 L4/L5
  ownership seam, the L4 `PathGuard.canonical/1` validates the path
  before L5 `Session.new/1` is called. L1 receives an already-
  validated path and stores it.
  """

  alias OpenSleigh.{AdapterSession, ConfigHash, Phase, RunAttemptSubState, SessionId, Ticket}

  @enforce_keys [
    :id,
    :ticket,
    :phase,
    :config_hash,
    :scoped_tools,
    :workspace_path,
    :claimed_at,
    :adapter_session,
    :sub_state
  ]
  defstruct [
    :id,
    :ticket,
    :phase,
    :config_hash,
    :scoped_tools,
    :workspace_path,
    :claimed_at,
    :adapter_session,
    :sub_state,
    :thread_id,
    :last_event_at,
    turn_count: 0,
    codex_input_tokens: 0,
    codex_output_tokens: 0,
    codex_total_tokens: 0,
    last_reported_total_tokens: 0
  ]

  @type t :: %__MODULE__{
          id: SessionId.t(),
          ticket: Ticket.t(),
          phase: Phase.t(),
          config_hash: ConfigHash.t(),
          scoped_tools: MapSet.t(atom()),
          workspace_path: Path.t(),
          claimed_at: DateTime.t(),
          adapter_session: AdapterSession.t(),
          sub_state: RunAttemptSubState.t(),
          thread_id: String.t() | nil,
          turn_count: non_neg_integer(),
          last_event_at: DateTime.t() | nil,
          codex_input_tokens: non_neg_integer(),
          codex_output_tokens: non_neg_integer(),
          codex_total_tokens: non_neg_integer(),
          last_reported_total_tokens: non_neg_integer()
        }

  @type new_error ::
          :invalid_id
          | :invalid_ticket
          | :invalid_phase
          | :invalid_config_hash
          | :invalid_scoped_tools
          | :invalid_workspace_path
          | :invalid_claimed_at
          | :invalid_adapter_session
          | :invalid_sub_state

  @doc """
  Construct a fresh `Session` in the `:preparing_workspace` sub-state.

  Required attrs: `:id`, `:ticket`, `:phase`, `:config_hash`,
  `:scoped_tools`, `:workspace_path`, `:claimed_at`,
  `:adapter_session`. `:sub_state` defaults to `:preparing_workspace`.
  """
  @spec new(keyword() | map()) :: {:ok, t()} | {:error, new_error()}
  def new(attrs) when is_list(attrs), do: new(Map.new(attrs))

  def new(%{} = attrs) do
    attrs = Map.put_new(attrs, :sub_state, :preparing_workspace)

    with :ok <- validate_id(attrs[:id]),
         :ok <- validate_ticket(attrs[:ticket]),
         :ok <- validate_phase(attrs[:phase]),
         :ok <- validate_config_hash(attrs[:config_hash]),
         :ok <- validate_scoped_tools(attrs[:scoped_tools]),
         :ok <- validate_workspace_path(attrs[:workspace_path]),
         :ok <- validate_claimed_at(attrs[:claimed_at]),
         :ok <- validate_adapter_session(attrs[:adapter_session]),
         :ok <- validate_sub_state(attrs[:sub_state]) do
      {:ok,
       %__MODULE__{
         id: attrs.id,
         ticket: attrs.ticket,
         phase: attrs.phase,
         config_hash: attrs.config_hash,
         scoped_tools: attrs.scoped_tools,
         workspace_path: attrs.workspace_path,
         claimed_at: attrs.claimed_at,
         adapter_session: attrs.adapter_session,
         sub_state: attrs.sub_state
       }}
    end
  end

  @doc """
  Transition to a new sub-state. Returns a new `Session.t()`.
  """
  @spec transition(t(), RunAttemptSubState.t()) :: t()
  def transition(%__MODULE__{} = session, new_sub_state) do
    true = RunAttemptSubState.valid?(new_sub_state)
    %{session | sub_state: new_sub_state}
  end

  @doc """
  Increment `turn_count` and update `last_event_at`.
  """
  @spec record_turn_completed(t(), DateTime.t()) :: t()
  def record_turn_completed(%__MODULE__{} = session, %DateTime{} = at) do
    %{session | turn_count: session.turn_count + 1, last_event_at: at}
  end

  @doc """
  Ingest an absolute-thread token-count update (per
  `specs/target-system/HAFT_CONTRACT.md §4` +
  `specs/target-system/AGENT_PROTOCOL.md §4` + ILLEGAL_STATES TA2).
  Computes the delta against `last_reported_total_tokens` to avoid
  double-counting.
  """
  @spec ingest_token_totals(t(), non_neg_integer(), non_neg_integer(), non_neg_integer()) :: t()
  def ingest_token_totals(
        %__MODULE__{} = session,
        absolute_input,
        absolute_output,
        absolute_total
      )
      when is_integer(absolute_input) and is_integer(absolute_output) and
             is_integer(absolute_total) do
    delta_total = max(absolute_total - session.last_reported_total_tokens, 0)
    delta_input = max(absolute_input - session.codex_input_tokens, 0)
    delta_output = max(absolute_output - session.codex_output_tokens, 0)

    %{
      session
      | codex_input_tokens: session.codex_input_tokens + delta_input,
        codex_output_tokens: session.codex_output_tokens + delta_output,
        codex_total_tokens: session.codex_total_tokens + delta_total,
        last_reported_total_tokens: absolute_total
    }
  end

  @doc """
  Set the `thread_id` once `thread/start` returns. One-time set;
  subsequent calls no-op (the thread_id is fixed for the session's
  lifetime; phase transitions open a new session, not a new thread).
  """
  @spec set_thread_id(t(), String.t()) :: t()
  def set_thread_id(%__MODULE__{thread_id: nil} = session, thread_id)
      when is_binary(thread_id) and byte_size(thread_id) > 0 do
    %{session | thread_id: thread_id}
  end

  def set_thread_id(%__MODULE__{} = session, _thread_id), do: session

  # ——— validators ———

  @spec validate_id(term()) :: :ok | {:error, :invalid_id}
  defp validate_id(id) do
    if SessionId.valid?(id), do: :ok, else: {:error, :invalid_id}
  end

  @spec validate_ticket(term()) :: :ok | {:error, :invalid_ticket}
  defp validate_ticket(%Ticket{}), do: :ok
  defp validate_ticket(_), do: {:error, :invalid_ticket}

  @spec validate_phase(term()) :: :ok | {:error, :invalid_phase}
  defp validate_phase(phase) do
    if Phase.valid?(phase), do: :ok, else: {:error, :invalid_phase}
  end

  @spec validate_config_hash(term()) :: :ok | {:error, :invalid_config_hash}
  defp validate_config_hash(hash) do
    if ConfigHash.valid?(hash), do: :ok, else: {:error, :invalid_config_hash}
  end

  @spec validate_scoped_tools(term()) :: :ok | {:error, :invalid_scoped_tools}
  defp validate_scoped_tools(%MapSet{}), do: :ok
  defp validate_scoped_tools(_), do: {:error, :invalid_scoped_tools}

  @spec validate_workspace_path(term()) :: :ok | {:error, :invalid_workspace_path}
  defp validate_workspace_path(p) when is_binary(p) and byte_size(p) > 0, do: :ok
  defp validate_workspace_path(_), do: {:error, :invalid_workspace_path}

  @spec validate_claimed_at(term()) :: :ok | {:error, :invalid_claimed_at}
  defp validate_claimed_at(%DateTime{}), do: :ok
  defp validate_claimed_at(_), do: {:error, :invalid_claimed_at}

  @spec validate_adapter_session(term()) :: :ok | {:error, :invalid_adapter_session}
  defp validate_adapter_session(%AdapterSession{}), do: :ok
  defp validate_adapter_session(_), do: {:error, :invalid_adapter_session}

  @spec validate_sub_state(term()) :: :ok | {:error, :invalid_sub_state}
  defp validate_sub_state(sub_state) do
    if RunAttemptSubState.valid?(sub_state), do: :ok, else: {:error, :invalid_sub_state}
  end
end
