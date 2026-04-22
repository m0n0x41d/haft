defmodule OpenSleigh.Ticket do
  @moduledoc """
  A unit of work claimed from the tracker. Immutable within Open-
  Sleigh; the tracker owns mutable ticket state.

  Per `specs/target-system/TARGET_SYSTEM_MODEL.md §Ticket` and
  `ILLEGAL_STATES.md` UP1 (v0.5 hardened).

  **`problem_card_ref` is required** in MVP-1 — a ticket without one
  fail-fasts at Frame entry with `:no_upstream_frame`. Open-Sleigh
  never authors a ProblemCard; framing is the upstream human's role
  via Haft + `/h-reason`.
  """

  alias OpenSleigh.ProblemCardRef

  @typedoc "Tracker-origin discriminator."
  @type source ::
          {:linear, workspace :: String.t()}
          | {:github, repo :: String.t()}
          | {:jira, project :: String.t()}

  @enforce_keys [:id, :source, :title, :body, :state, :problem_card_ref, :fetched_at]
  defstruct [
    :id,
    :source,
    :title,
    :body,
    :state,
    :problem_card_ref,
    :target_branch,
    :fetched_at,
    metadata: %{}
  ]

  @type t :: %__MODULE__{
          id: String.t(),
          source: source(),
          title: String.t(),
          body: String.t(),
          state: atom(),
          problem_card_ref: ProblemCardRef.t(),
          target_branch: String.t() | nil,
          fetched_at: DateTime.t(),
          metadata: map()
        }

  @type new_error ::
          :invalid_id
          | :invalid_source
          | :invalid_title
          | :invalid_body
          | :invalid_state
          | :missing_problem_card_ref
          | :invalid_target_branch
          | :invalid_fetched_at
          | :invalid_metadata

  @doc """
  Construct a `Ticket`. `problem_card_ref` is required — passing
  `nil` returns `{:error, :missing_problem_card_ref}` (UP1).

  This is the **L1 shape check** only. The L5 `Orchestrator.claim/1`
  additionally verifies that the ProblemCardRef resolves to a live,
  non-self-authored Haft artifact via the Frame-entry gate.
  """
  @spec new(keyword() | map()) :: {:ok, t()} | {:error, new_error()}
  def new(attrs) when is_list(attrs), do: new(Map.new(attrs))

  def new(%{} = attrs) do
    with :ok <- validate_id(attrs[:id]),
         :ok <- validate_source(attrs[:source]),
         :ok <- validate_title(attrs[:title]),
         :ok <- validate_body(attrs[:body]),
         :ok <- validate_state(attrs[:state]),
         :ok <- validate_problem_card_ref(attrs[:problem_card_ref]),
         :ok <- validate_target_branch(Map.get(attrs, :target_branch)),
         :ok <- validate_fetched_at(attrs[:fetched_at]),
         :ok <- validate_metadata(Map.get(attrs, :metadata, %{})) do
      {:ok,
       %__MODULE__{
         id: attrs.id,
         source: attrs.source,
         title: attrs.title,
         body: attrs.body,
         state: attrs.state,
         problem_card_ref: attrs.problem_card_ref,
         target_branch: Map.get(attrs, :target_branch),
         fetched_at: attrs.fetched_at,
         metadata: Map.get(attrs, :metadata, %{})
       }}
    end
  end

  @spec validate_id(term()) :: :ok | {:error, :invalid_id}
  defp validate_id(id) when is_binary(id) and byte_size(id) > 0, do: :ok
  defp validate_id(_), do: {:error, :invalid_id}

  @spec validate_source(term()) :: :ok | {:error, :invalid_source}
  defp validate_source({kind, scope})
       when kind in [:linear, :github, :jira] and is_binary(scope) and byte_size(scope) > 0,
       do: :ok

  defp validate_source(_), do: {:error, :invalid_source}

  @spec validate_title(term()) :: :ok | {:error, :invalid_title}
  defp validate_title(title) when is_binary(title) and byte_size(title) > 0, do: :ok
  defp validate_title(_), do: {:error, :invalid_title}

  @spec validate_body(term()) :: :ok | {:error, :invalid_body}
  # Body may be empty (just a title-only ticket is legal).
  defp validate_body(body) when is_binary(body), do: :ok
  defp validate_body(_), do: {:error, :invalid_body}

  @spec validate_state(term()) :: :ok | {:error, :invalid_state}
  defp validate_state(state) when is_atom(state) and not is_nil(state), do: :ok
  defp validate_state(_), do: {:error, :invalid_state}

  @spec validate_problem_card_ref(term()) :: :ok | {:error, :missing_problem_card_ref}
  defp validate_problem_card_ref(nil), do: {:error, :missing_problem_card_ref}

  defp validate_problem_card_ref(ref) do
    if OpenSleigh.ProblemCardRef.valid?(ref) do
      :ok
    else
      {:error, :missing_problem_card_ref}
    end
  end

  @spec validate_target_branch(term()) :: :ok | {:error, :invalid_target_branch}
  defp validate_target_branch(nil), do: :ok

  defp validate_target_branch(branch) when is_binary(branch) and byte_size(branch) > 0,
    do: :ok

  defp validate_target_branch(_), do: {:error, :invalid_target_branch}

  @spec validate_fetched_at(term()) :: :ok | {:error, :invalid_fetched_at}
  defp validate_fetched_at(%DateTime{}), do: :ok
  defp validate_fetched_at(_), do: {:error, :invalid_fetched_at}

  @spec validate_metadata(term()) :: :ok | {:error, :invalid_metadata}
  defp validate_metadata(m) when is_map(m), do: :ok
  defp validate_metadata(_), do: {:error, :invalid_metadata}
end
