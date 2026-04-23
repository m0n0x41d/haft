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

  alias OpenSleigh.{ConfigHash, ProblemCardRef, Scope, WorkCommission}

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

  @type commission_error ::
          Scope.new_error()
          | WorkCommission.new_error()

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

  @doc """
  Convert a legacy tracker Ticket into the WorkCommission shape the
  commission-first runtime expects.

  Tracker-first intake remains a compatibility bridge only: the
  synthetic commission is scoped from `ticket.metadata` when present
  and otherwise receives a broad legacy local scope. A caller-supplied
  `:scope_hash` / `"scope_hash"` in metadata is treated as the
  commission snapshot hash, so drift between that snapshot and the
  embedded Scope fails closed with `:scope_hash_mismatch`.
  """
  @spec to_work_commission(t()) :: {:ok, WorkCommission.t()} | {:error, commission_error()}
  def to_work_commission(%__MODULE__{} = ticket) do
    case metadata(ticket, :commission, nil) do
      %WorkCommission{} = commission ->
        commission
        |> Map.from_struct()
        |> WorkCommission.new()

      _other ->
        synthetic_commission(ticket)
    end
  end

  @spec synthetic_scope(t()) :: {:ok, Scope.t()} | {:error, Scope.new_error()}
  defp synthetic_scope(%__MODULE__{} = ticket) do
    ticket
    |> synthetic_scope_attrs()
    |> put_scope_hash()
    |> Scope.new()
  end

  @spec synthetic_scope_attrs(t()) :: map()
  defp synthetic_scope_attrs(%__MODULE__{} = ticket) do
    scope_metadata = metadata(ticket, :scope, %{})

    %{
      repo_ref: scope_metadata_value(scope_metadata, :repo_ref, source_ref(ticket.source)),
      base_sha:
        scope_metadata_value(
          scope_metadata,
          :base_sha,
          metadata(ticket, :base_sha, legacy_hash(ticket, "base"))
        ),
      target_branch:
        scope_metadata_value(
          scope_metadata,
          :target_branch,
          ticket.target_branch || "legacy-tracker"
        ),
      allowed_paths:
        scope_metadata_value(
          scope_metadata,
          :allowed_paths,
          metadata(ticket, :allowed_paths, ["**/*"])
        ),
      forbidden_paths:
        scope_metadata_value(
          scope_metadata,
          :forbidden_paths,
          metadata(ticket, :forbidden_paths, [])
        ),
      allowed_actions:
        scope_metadata_value(
          scope_metadata,
          :allowed_actions,
          metadata(ticket, :allowed_actions, [:edit_files, :run_tests, :commit])
        ),
      affected_files:
        scope_metadata_value(
          scope_metadata,
          :affected_files,
          metadata(ticket, :affected_files, ["**/*"])
        ),
      allowed_modules:
        scope_metadata_value(
          scope_metadata,
          :allowed_modules,
          metadata(ticket, :allowed_modules, [])
        ),
      lockset:
        scope_metadata_value(scope_metadata, :lockset, metadata(ticket, :lockset, ["**/*"]))
    }
    |> normalize_scope_actions()
  end

  @spec put_scope_hash(map()) :: map()
  defp put_scope_hash(attrs) do
    attrs
    |> Scope.canonical_hash()
    |> put_scope_hash_result(attrs)
  end

  @spec put_scope_hash_result({:ok, String.t()} | {:error, Scope.new_error()}, map()) :: map()
  defp put_scope_hash_result({:ok, hash}, attrs), do: Map.put(attrs, :hash, hash)
  defp put_scope_hash_result({:error, _reason}, attrs), do: Map.put(attrs, :hash, "")

  @spec normalize_scope_actions(map()) :: map()
  defp normalize_scope_actions(%{allowed_actions: actions} = attrs) do
    attrs
    |> Map.put(:allowed_actions, normalize_action_set(actions))
  end

  @spec normalize_action_set(term()) :: MapSet.t(atom()) | term()
  defp normalize_action_set(%MapSet{} = actions), do: actions

  defp normalize_action_set(actions) when is_list(actions) do
    actions
    |> Enum.map(&normalize_action/1)
    |> Enum.reject(&is_nil/1)
    |> MapSet.new()
  end

  defp normalize_action_set(actions), do: actions

  @spec normalize_action(term()) :: atom() | nil
  defp normalize_action(action) when is_atom(action) and not is_nil(action), do: action
  defp normalize_action("edit_files"), do: :edit_files
  defp normalize_action("run_tests"), do: :run_tests
  defp normalize_action("commit"), do: :commit
  defp normalize_action(_action), do: nil

  @spec synthetic_commission(t()) :: {:ok, WorkCommission.t()} | {:error, commission_error()}
  defp synthetic_commission(%__MODULE__{} = ticket) do
    with {:ok, scope} <- synthetic_scope(ticket) do
      ticket
      |> synthetic_commission_attrs(scope)
      |> WorkCommission.new()
    end
  end

  @spec synthetic_commission_attrs(t(), Scope.t()) :: map()
  defp synthetic_commission_attrs(%__MODULE__{} = ticket, %Scope{} = scope) do
    %{
      id: metadata(ticket, :commission_id, "legacy-ticket:" <> ticket.id),
      decision_ref: metadata(ticket, :decision_ref, "legacy-ticket:" <> ticket.id),
      decision_revision_hash:
        metadata(ticket, :decision_revision_hash, legacy_hash(ticket, "decision")),
      problem_card_ref: ticket.problem_card_ref,
      implementation_plan_ref: metadata(ticket, :implementation_plan_ref, nil),
      implementation_plan_revision: metadata(ticket, :implementation_plan_revision, nil),
      scope: scope,
      scope_hash: metadata(ticket, :scope_hash, scope.hash),
      base_sha: metadata(ticket, :base_sha, scope.base_sha),
      lockset: metadata(ticket, :commission_lockset, scope.lockset),
      evidence_requirements: metadata(ticket, :evidence_requirements, []),
      projection_policy: metadata(ticket, :projection_policy, :local_only),
      autonomy_envelope_ref: metadata(ticket, :autonomy_envelope_ref, nil),
      autonomy_envelope_revision: metadata(ticket, :autonomy_envelope_revision, nil),
      state: metadata(ticket, :commission_state, :queued),
      valid_until:
        metadata(ticket, :valid_until, DateTime.add(ticket.fetched_at, 30 * 86_400, :second)),
      fetched_at: ticket.fetched_at
    }
    |> normalize_projection_policy()
    |> normalize_commission_state()
  end

  @spec normalize_projection_policy(map()) :: map()
  defp normalize_projection_policy(%{projection_policy: policy} = attrs) when is_binary(policy) do
    attrs
    |> Map.put(:projection_policy, normalize_projection_policy_value(policy))
  end

  defp normalize_projection_policy(attrs), do: attrs

  @spec normalize_projection_policy_value(String.t()) :: atom() | String.t()
  defp normalize_projection_policy_value("local_only"), do: :local_only
  defp normalize_projection_policy_value("external_optional"), do: :external_optional
  defp normalize_projection_policy_value("external_required"), do: :external_required
  defp normalize_projection_policy_value(policy), do: policy

  @spec normalize_commission_state(map()) :: map()
  defp normalize_commission_state(%{state: state} = attrs) when is_binary(state) do
    attrs
    |> Map.put(:state, normalize_commission_state_value(state))
  end

  defp normalize_commission_state(attrs), do: attrs

  @spec normalize_commission_state_value(String.t()) :: atom() | String.t()
  defp normalize_commission_state_value("draft"), do: :draft
  defp normalize_commission_state_value("queued"), do: :queued
  defp normalize_commission_state_value("ready"), do: :ready
  defp normalize_commission_state_value("preflighting"), do: :preflighting
  defp normalize_commission_state_value("running"), do: :running
  defp normalize_commission_state_value("blocked_stale"), do: :blocked_stale
  defp normalize_commission_state_value("blocked_policy"), do: :blocked_policy
  defp normalize_commission_state_value("blocked_conflict"), do: :blocked_conflict
  defp normalize_commission_state_value("needs_human_review"), do: :needs_human_review
  defp normalize_commission_state_value("completed"), do: :completed

  defp normalize_commission_state_value("completed_with_projection_debt"),
    do: :completed_with_projection_debt

  defp normalize_commission_state_value("failed"), do: :failed
  defp normalize_commission_state_value("cancelled"), do: :cancelled
  defp normalize_commission_state_value("expired"), do: :expired
  defp normalize_commission_state_value(state), do: state

  @spec metadata(t(), atom(), term()) :: term()
  defp metadata(%__MODULE__{metadata: metadata}, key, fallback) do
    metadata
    |> Map.get(key, Map.get(metadata, Atom.to_string(key), fallback))
  end

  @spec scope_metadata_value(term(), atom(), term()) :: term()
  defp scope_metadata_value(%{} = metadata, key, fallback) do
    metadata
    |> Map.get(key, Map.get(metadata, Atom.to_string(key), fallback))
  end

  defp scope_metadata_value(_metadata, _key, fallback), do: fallback

  @spec source_ref(source()) :: String.t()
  defp source_ref({kind, scope}), do: Atom.to_string(kind) <> ":" <> scope

  @spec legacy_hash(t(), String.t()) :: String.t()
  defp legacy_hash(%__MODULE__{} = ticket, suffix) do
    ConfigHash.from_iodata(["legacy-ticket:", ticket.id, ":", suffix])
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
