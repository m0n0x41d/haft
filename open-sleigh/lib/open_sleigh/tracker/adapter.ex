defmodule OpenSleigh.Tracker.Adapter do
  @moduledoc """
  Behaviour contract for tracker adapters (Linear first; GitHub,
  Jira later). Per `specs/target-system/SYSTEM_CONTEXT.md` +
  `SPEC §3`.

  **Tracker-mutation boundary (AGENT_PROTOCOL.md §7):** the agent
  has NO direct tracker-mutation tool. All writes (state
  transitions, comments) flow through this behaviour, invoked by
  the L5 `Orchestrator`. The agent proposes transitions as part of
  its `PhaseOutcome`; the Orchestrator effects them, gated by
  HumanGate where required.

  L4 modules are **stateless**. HTTP clients, auth tokens, and
  connection pools are owned by L5 wrappers; L4 functions take an
  auth handle / endpoint and perform the call through it.
  """

  alias OpenSleigh.{EffectError, Ticket}

  @typedoc "Per-impl handle (typically {endpoint, api_key} or a mock state ref)."
  @type handle :: term()

  @typedoc "A ticket normalised to the Open-Sleigh `Ticket.t()` shape."
  @type normalised_ticket :: Ticket.t()

  @typedoc "Normalised tracker comment used by HumanGateListener."
  @type normalised_comment :: %{
          required(:id) => String.t(),
          required(:body) => String.t(),
          required(:author) => String.t(),
          optional(:created_at) => DateTime.t(),
          optional(:url) => String.t()
        }

  @doc """
  Fetch the current list of tickets in the declared `active_states`
  for this tracker. Returns normalised `Ticket.t()` values.
  """
  @callback list_active(handle()) ::
              {:ok, [normalised_ticket()]} | {:error, EffectError.t()}

  @doc """
  Fetch a single ticket by tracker-native id (used for
  reconciliation per SPEC §10.3).
  """
  @callback get(handle(), ticket_id :: String.t()) ::
              {:ok, normalised_ticket()} | {:error, EffectError.t()}

  @doc "Request a state transition (e.g. move to `Done`)."
  @callback transition(handle(), ticket_id :: String.t(), new_state :: atom()) ::
              :ok | {:error, EffectError.t()}

  @doc "Post a comment to the ticket (for HumanGate requests, structured error messages, etc.)."
  @callback post_comment(handle(), ticket_id :: String.t(), body :: String.t()) ::
              :ok | {:error, EffectError.t()}

  @doc "List ticket comments, oldest-first."
  @callback list_comments(handle(), ticket_id :: String.t()) ::
              {:ok, [normalised_comment()]} | {:error, EffectError.t()}

  @doc "Adapter kind atom (`:linear` / `:github` / `:mock`)."
  @callback adapter_kind() :: atom()
end
