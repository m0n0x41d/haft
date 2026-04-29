defmodule OpenSleigh.Haft.Client do
  @moduledoc """
  Stateless typed API to the Haft MCP server.

  Per v0.6.1 L4/L5 ownership seam:

  * This module lives at **L4** — it is stateless and its functions
    take an `AdapterSession` + a handle that refers to an L5-owned
    process.
  * The `haft serve` subprocess is owned by the **L5**
    `OpenSleigh.Haft.Server` (lands with L5). Every function here
    is a thin typed wrapper that delegates to the L5 server via
    `GenServer.call` or equivalent.

  For MVP-1 pre-L5, this module can be invoked with an injected
  `invoke_fun` (similar to `GateChain`) so L1/L2/L3/L4 tests don't
  need a live `haft serve`. `invoke_fun` is typically supplied by
  L5 to the L4 call sites.
  """

  alias OpenSleigh.{AdapterSession, EffectError, Haft.Protocol, PhaseOutcome}

  @typedoc """
  Invoker — encapsulates how a tool call actually reaches `haft serve`.
  L5 supplies a function that sends to the `HaftServer` GenServer;
  tests supply a stub.
  """
  @type invoke_fun ::
          (binary() -> {:ok, binary()} | {:error, EffectError.t()})

  @type artifact_identity ::
          {:commission, String.t(), legacy_ticket_id :: String.t() | nil}
          | {:ticket, String.t()}
          | :none

  @doc """
  Write a completed `PhaseOutcome` as a Haft artifact. Attaches the
  session's `config_hash` via the protocol codec (SE7 provenance).
  """
  @spec write_artifact(AdapterSession.t(), PhaseOutcome.t(), invoke_fun()) ::
          {:ok, binary()} | {:error, EffectError.t()}
  def write_artifact(%AdapterSession{} = session, %PhaseOutcome{} = outcome, invoke_fun)
      when is_function(invoke_fun, 1) do
    identity =
      session
      |> commission_id()
      |> commission_identity()

    write_artifact_with_identity(session, outcome, identity, invoke_fun)
  end

  @doc "Write a completed `PhaseOutcome` with the owning tracker ticket id."
  @spec write_ticket_artifact(AdapterSession.t(), PhaseOutcome.t(), String.t(), invoke_fun()) ::
          {:ok, binary()} | {:error, EffectError.t()}
  def write_ticket_artifact(
        %AdapterSession{} = session,
        %PhaseOutcome{} = outcome,
        ticket_id,
        invoke_fun
      )
      when is_binary(ticket_id) and is_function(invoke_fun, 1) do
    identity =
      session
      |> commission_id()
      |> identity_for_ticket(ticket_id)

    write_artifact_with_identity(session, outcome, identity, invoke_fun)
  end

  @doc "Write a completed `PhaseOutcome` with the owning Haft WorkCommission id."
  @spec write_commission_artifact(AdapterSession.t(), PhaseOutcome.t(), String.t(), invoke_fun()) ::
          {:ok, binary()} | {:error, EffectError.t()}
  def write_commission_artifact(
        %AdapterSession{} = session,
        %PhaseOutcome{} = outcome,
        commission_id,
        invoke_fun
      )
      when is_binary(commission_id) and is_function(invoke_fun, 1) do
    identity =
      commission_id
      |> commission_identity()

    write_artifact_with_identity(session, outcome, identity, invoke_fun)
  end

  @spec write_artifact_with_identity(
          AdapterSession.t(),
          PhaseOutcome.t(),
          artifact_identity(),
          invoke_fun()
        ) :: {:ok, binary()} | {:error, EffectError.t()}
  defp write_artifact_with_identity(session, outcome, identity, invoke_fun) do
    action = action_for_phase(outcome.phase)
    params = serialise_outcome(outcome, identity)
    tool = tool_for_phase(outcome.phase)

    call_tool(session, tool, action, params, invoke_fun)
  end

  @doc """
  Generic tool call — use for `haft_query`, `haft_note`, etc.
  """
  @spec call_tool(AdapterSession.t(), Protocol.tool(), atom(), map(), invoke_fun()) ::
          {:ok, binary()} | {:error, EffectError.t()}
  def call_tool(%AdapterSession{} = session, tool, action, params, invoke_fun)
      when is_function(invoke_fun, 1) do
    id = next_id()

    with {:ok, encoded} <- Protocol.encode_call(id, tool, action, params, session),
         {:ok, response_line} <- invoke_fun.(encoded),
         {:ok, {^id, result}} <- Protocol.decode_response(response_line) do
      tool_result(result)
    else
      {:ok, {_other_id, _}} -> {:error, :response_parse_error}
      {:error, _} = err -> err
    end
  end

  @doc """
  Record a WorkCommission lifecycle event through Haft.

  Legacy tracker-first sessions do not carry a `commission_id`; for those,
  lifecycle recording is intentionally a no-op so existing tracker canaries
  keep their old semantics.
  """
  @spec record_commission_lifecycle(AdapterSession.t(), atom(), map(), invoke_fun()) ::
          :ok | {:error, EffectError.t()}
  def record_commission_lifecycle(%AdapterSession{} = session, action, params, invoke_fun)
      when is_atom(action) and is_map(params) and is_function(invoke_fun, 1) do
    session
    |> commission_id()
    |> record_commission_lifecycle_for_id(session, action, params, invoke_fun)
  end

  @doc "Fetch the upstream ProblemCard referenced by a tracker ticket."
  @spec fetch_problem_card(AdapterSession.t(), String.t(), invoke_fun()) ::
          {:ok, map()} | {:error, EffectError.t()}
  def fetch_problem_card(%AdapterSession{} = session, problem_card_ref, invoke_fun)
      when is_binary(problem_card_ref) and is_function(invoke_fun, 1) do
    params = %{"artifact_id" => problem_card_ref, "ref" => problem_card_ref}

    with {:ok, encoded} <- call_tool(session, :haft_query, :related, params, invoke_fun),
         {:ok, decoded} <- decode_tool_result(encoded),
         {:ok, card} <- extract_problem_card(decoded, problem_card_ref) do
      {:ok, card}
    end
  end

  # ——— helpers ———

  @spec tool_result(map()) :: {:ok, binary()} | {:error, EffectError.t()}
  defp tool_result(%{"isError" => true} = result) do
    result
    |> tool_error_reason()
    |> then(&{:error, &1})
  end

  defp tool_result(result), do: {:ok, Jason.encode!(result)}

  @spec tool_error_reason(map()) :: EffectError.t()
  defp tool_error_reason(%{"content" => content}) when is_list(content) do
    content
    |> Enum.find_value(&content_error_reason/1)
    |> known_tool_error()
  end

  defp tool_error_reason(_result), do: :tool_execution_failed

  @spec content_error_reason(map() | term()) :: String.t() | nil
  defp content_error_reason(%{"text" => text}) when is_binary(text), do: String.trim(text)
  defp content_error_reason(_content), do: nil

  @spec known_tool_error(String.t() | nil) :: EffectError.t()
  defp known_tool_error("commission_not_found"), do: :commission_not_found
  defp known_tool_error("commission_not_runnable"), do: :commission_not_runnable
  defp known_tool_error("commission_lock_conflict"), do: :commission_lock_conflict
  defp known_tool_error(_reason), do: :tool_execution_failed

  @spec decode_tool_result(binary()) :: {:ok, map()} | {:error, :response_parse_error}
  defp decode_tool_result(encoded) do
    case Jason.decode(encoded) do
      {:ok, decoded} when is_map(decoded) -> {:ok, decoded}
      _ -> {:error, :response_parse_error}
    end
  end

  @spec extract_problem_card(map(), String.t()) :: {:ok, map()} | {:error, EffectError.t()}
  defp extract_problem_card(%{"problem_card" => card}, problem_card_ref) when is_map(card) do
    {:ok, normalize_problem_card(card, problem_card_ref)}
  end

  defp extract_problem_card(%{"problemCard" => card}, problem_card_ref) when is_map(card) do
    {:ok, normalize_problem_card(card, problem_card_ref)}
  end

  defp extract_problem_card(%{"artifact" => card}, problem_card_ref) when is_map(card) do
    {:ok, normalize_problem_card(card, problem_card_ref)}
  end

  defp extract_problem_card(%{"artifacts" => cards}, problem_card_ref) when is_list(cards) do
    cards
    |> Enum.find(&problem_card_match?(&1, problem_card_ref))
    |> problem_card_result(problem_card_ref)
  end

  defp extract_problem_card(%{"related" => cards}, problem_card_ref) when is_list(cards) do
    cards
    |> Enum.find(&problem_card_match?(&1, problem_card_ref))
    |> problem_card_result(problem_card_ref)
  end

  defp extract_problem_card(%{"content" => content}, problem_card_ref) when is_list(content) do
    content
    |> Enum.find_value(&decode_content_problem_card(&1, problem_card_ref))
    |> problem_card_result(problem_card_ref)
  end

  defp extract_problem_card(%{} = card, problem_card_ref) do
    if problem_card_shape?(card) do
      {:ok, normalize_problem_card(card, problem_card_ref)}
    else
      {:error, :tracker_response_malformed}
    end
  end

  @spec decode_content_problem_card(map() | term(), String.t()) :: map() | nil
  defp decode_content_problem_card(%{"text" => text}, problem_card_ref) when is_binary(text) do
    case Jason.decode(text) do
      {:ok, decoded} when is_map(decoded) ->
        decoded
        |> extract_problem_card(problem_card_ref)
        |> problem_card_or_nil()

      _ ->
        nil
    end
  end

  defp decode_content_problem_card(_content, _problem_card_ref), do: nil

  @spec problem_card_or_nil({:ok, map()} | {:error, EffectError.t()}) :: map() | nil
  defp problem_card_or_nil({:ok, card}), do: card
  defp problem_card_or_nil({:error, _reason}), do: nil

  @spec problem_card_result(map() | nil, String.t()) :: {:ok, map()} | {:error, EffectError.t()}
  defp problem_card_result(nil, _problem_card_ref), do: {:error, :tracker_response_malformed}

  defp problem_card_result(card, problem_card_ref) when is_map(card) do
    {:ok, normalize_problem_card(card, problem_card_ref)}
  end

  @spec problem_card_match?(map() | term(), String.t()) :: boolean()
  defp problem_card_match?(%{} = card, problem_card_ref) do
    [
      Map.get(card, "id"),
      Map.get(card, :id),
      Map.get(card, "ref"),
      Map.get(card, :ref),
      Map.get(card, "artifact_id"),
      Map.get(card, :artifact_id),
      get_in(card, ["meta", "id"]),
      get_in(card, [:meta, :id])
    ]
    |> Enum.any?(&(&1 == problem_card_ref))
  end

  defp problem_card_match?(_card, _problem_card_ref), do: false

  @spec problem_card_shape?(map()) :: boolean()
  defp problem_card_shape?(card) do
    legacy_shape =
      card
      |> Map.keys()
      |> Enum.any?(&problem_card_key?/1)

    legacy_shape or problem_card_artifact_shape?(card)
  end

  @spec problem_card_key?(term()) :: boolean()
  defp problem_card_key?("describedEntity"), do: true
  defp problem_card_key?("groundingHolon"), do: true
  defp problem_card_key?(:describedEntity), do: true
  defp problem_card_key?(:groundingHolon), do: true
  defp problem_card_key?(_key), do: false

  @spec problem_card_artifact_shape?(map()) :: boolean()
  defp problem_card_artifact_shape?(card) do
    Enum.any?([
      Map.get(card, "kind") == "ProblemCard",
      Map.get(card, :kind) == "ProblemCard",
      get_in(card, ["meta", "kind"]) == "ProblemCard",
      get_in(card, [:meta, :kind]) == "ProblemCard"
    ])
  end

  @spec normalize_problem_card(map(), String.t()) :: map()
  defp normalize_problem_card(card, problem_card_ref) do
    card
    |> put_artifact_problem_card_fields()
    |> Map.put_new("ref", problem_card_ref)
    |> put_normalized_authoring_source()
  end

  @spec put_artifact_problem_card_fields(map()) :: map()
  defp put_artifact_problem_card_fields(card) do
    meta = Map.get(card, "meta", Map.get(card, :meta, %{}))

    card
    |> Map.put_new("id", Map.get(meta, "id", Map.get(meta, :id)))
    |> Map.put_new("title", Map.get(meta, "title", Map.get(meta, :title)))
    |> Map.put_new("valid_until", Map.get(meta, "valid_until", Map.get(meta, :valid_until)))
    |> put_problem_card_text_field("body")
    |> put_problem_card_text_field("description")
  end

  @spec put_problem_card_text_field(map(), String.t()) :: map()
  defp put_problem_card_text_field(card, field) do
    text =
      card
      |> Map.get(field, Map.get(card, String.to_atom(field), nil))
      |> problem_card_text_fallback(card)

    Map.put_new(card, field, text)
  end

  @spec problem_card_text_fallback(term(), map()) :: term()
  defp problem_card_text_fallback(value, _card) when is_binary(value) and value != "", do: value

  defp problem_card_text_fallback(_value, card) do
    Map.get(
      card,
      "content",
      Map.get(card, :content, Map.get(card, "body", Map.get(card, :body, "")))
    )
  end

  @spec put_normalized_authoring_source(map()) :: map()
  defp put_normalized_authoring_source(card) do
    source =
      card
      |> Map.get(:authoring_source, Map.get(card, "authoring_source"))
      |> normalize_authoring_source()

    case source do
      nil -> card
      value -> Map.put(card, :authoring_source, value)
    end
  end

  @spec normalize_authoring_source(term()) :: term()
  defp normalize_authoring_source("open_sleigh_self"), do: :open_sleigh_self
  defp normalize_authoring_source(:open_sleigh_self), do: :open_sleigh_self
  defp normalize_authoring_source(value), do: value

  @spec action_for_phase(atom()) :: :note
  defp action_for_phase(_phase), do: :note

  @spec tool_for_phase(atom()) :: Protocol.tool()
  defp tool_for_phase(_phase), do: :haft_note

  @spec serialise_outcome(PhaseOutcome.t(), artifact_identity()) :: map()
  defp serialise_outcome(%PhaseOutcome{} = o, identity) do
    %{
      "title" => phase_note_title(o),
      "phase" => Atom.to_string(o.phase),
      "config_hash" => o.config_hash,
      "valid_until" => DateTime.to_iso8601(o.valid_until),
      "authoring_role" => Atom.to_string(o.authoring_role),
      "self_id" => o.self_id,
      "rationale" => phase_note_rationale(o),
      "work_product" => o.work_product,
      "evidence" => Enum.map(o.evidence, &serialise_evidence/1)
    }
    |> put_artifact_identity(identity)
  end

  @spec phase_note_title(PhaseOutcome.t()) :: String.t()
  defp phase_note_title(%PhaseOutcome{} = outcome) do
    "Open-Sleigh " <> Atom.to_string(outcome.phase) <> " outcome"
  end

  @spec phase_note_rationale(PhaseOutcome.t()) :: String.t()
  defp phase_note_rationale(%PhaseOutcome{rationale: rationale})
       when is_binary(rationale) and rationale != "" do
    rationale
  end

  defp phase_note_rationale(%PhaseOutcome{work_product: %{text: text}})
       when is_binary(text) and text != "" do
    "Open-Sleigh phase output: " <> text
  end

  defp phase_note_rationale(%PhaseOutcome{} = outcome) do
    "Open-Sleigh recorded " <> Atom.to_string(outcome.phase) <> " phase outcome."
  end

  @spec put_artifact_identity(map(), artifact_identity()) :: map()
  defp put_artifact_identity(params, {:commission, commission_id, legacy_ticket_id}) do
    params
    |> Map.put("commission_id", commission_id)
    |> Map.put("ticket_id", commission_id)
    |> maybe_put_legacy_ticket_id(legacy_ticket_id, commission_id)
  end

  defp put_artifact_identity(params, {:ticket, ticket_id}) do
    Map.put(params, "ticket_id", ticket_id)
  end

  defp put_artifact_identity(params, :none), do: params

  @spec maybe_put_legacy_ticket_id(map(), String.t() | nil, String.t()) :: map()
  defp maybe_put_legacy_ticket_id(params, legacy_ticket_id, commission_id)
       when is_binary(legacy_ticket_id) and legacy_ticket_id != commission_id do
    Map.put(params, "legacy_ticket_id", legacy_ticket_id)
  end

  defp maybe_put_legacy_ticket_id(params, _legacy_ticket_id, _commission_id), do: params

  @spec identity_for_ticket(String.t() | nil, String.t()) :: artifact_identity()
  defp identity_for_ticket(commission_id, ticket_id)
       when is_binary(commission_id) and commission_id != "" do
    {:commission, commission_id, ticket_id}
  end

  defp identity_for_ticket(_commission_id, ticket_id), do: {:ticket, ticket_id}

  @spec commission_identity(String.t() | nil) :: artifact_identity()
  defp commission_identity(commission_id) when is_binary(commission_id) and commission_id != "" do
    {:commission, commission_id, nil}
  end

  defp commission_identity(_commission_id), do: :none

  @spec commission_id(AdapterSession.t()) :: String.t() | nil
  defp commission_id(%AdapterSession{} = session) do
    Map.get(session, :commission_id)
  end

  @spec record_commission_lifecycle_for_id(
          String.t() | nil,
          AdapterSession.t(),
          atom(),
          map(),
          invoke_fun()
        ) :: :ok | {:error, EffectError.t()}
  defp record_commission_lifecycle_for_id(nil, _session, _action, _params, _invoke_fun), do: :ok

  defp record_commission_lifecycle_for_id("", _session, _action, _params, _invoke_fun), do: :ok

  defp record_commission_lifecycle_for_id(
         "legacy-ticket:" <> _id,
         _session,
         _action,
         _params,
         _invoke_fun
       ),
       do: :ok

  defp record_commission_lifecycle_for_id(commission_id, session, action, params, invoke_fun)
       when is_binary(commission_id) do
    params
    |> Map.put("commission_id", commission_id)
    |> Map.put_new("runner_id", runner_id(session))
    |> call_tool_ok(session, :haft_commission, action, invoke_fun)
  end

  @spec call_tool_ok(map(), AdapterSession.t(), Protocol.tool(), atom(), invoke_fun()) ::
          :ok | {:error, EffectError.t()}
  defp call_tool_ok(params, session, tool, action, invoke_fun) do
    case call_tool(session, tool, action, params, invoke_fun) do
      {:ok, _result} -> :ok
      {:error, _reason} = error -> error
    end
  end

  @spec runner_id(AdapterSession.t()) :: String.t()
  defp runner_id(%AdapterSession{} = session) do
    "open-sleigh:" <> session.session_id
  end

  @spec serialise_evidence(OpenSleigh.Evidence.t()) :: map()
  defp serialise_evidence(e) do
    %{
      "kind" => Atom.to_string(e.kind),
      "ref" => e.ref,
      "hash" => e.hash,
      "cl" => e.cl,
      "authoring_source" => Atom.to_string(e.authoring_source),
      "captured_at" => DateTime.to_iso8601(e.captured_at)
    }
  end

  @spec next_id() :: integer()
  defp next_id do
    :erlang.unique_integer([:positive, :monotonic])
  end
end
