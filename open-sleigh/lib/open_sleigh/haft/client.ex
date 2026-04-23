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
      {:ok, Jason.encode!(result)}
    else
      {:ok, {_other_id, _}} -> {:error, :response_parse_error}
      {:error, _} = err -> err
    end
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
      Map.get(card, :artifact_id)
    ]
    |> Enum.any?(&(&1 == problem_card_ref))
  end

  defp problem_card_match?(_card, _problem_card_ref), do: false

  @spec problem_card_shape?(map()) :: boolean()
  defp problem_card_shape?(card) do
    card
    |> Map.keys()
    |> Enum.any?(&problem_card_key?/1)
  end

  @spec problem_card_key?(term()) :: boolean()
  defp problem_card_key?("describedEntity"), do: true
  defp problem_card_key?("groundingHolon"), do: true
  defp problem_card_key?(:describedEntity), do: true
  defp problem_card_key?(:groundingHolon), do: true
  defp problem_card_key?(_key), do: false

  @spec normalize_problem_card(map(), String.t()) :: map()
  defp normalize_problem_card(card, problem_card_ref) do
    card
    |> Map.put_new("ref", problem_card_ref)
    |> put_normalized_authoring_source()
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

  @spec action_for_phase(atom()) :: atom()
  defp action_for_phase(:frame), do: :frame
  defp action_for_phase(:execute), do: :apply
  defp action_for_phase(:measure), do: :measure
  defp action_for_phase(_), do: :note

  @spec tool_for_phase(atom()) :: Protocol.tool()
  defp tool_for_phase(:frame), do: :haft_problem
  defp tool_for_phase(:execute), do: :haft_note
  defp tool_for_phase(:measure), do: :haft_decision
  defp tool_for_phase(_), do: :haft_note

  @spec serialise_outcome(PhaseOutcome.t(), artifact_identity()) :: map()
  defp serialise_outcome(%PhaseOutcome{} = o, identity) do
    %{
      "phase" => Atom.to_string(o.phase),
      "config_hash" => o.config_hash,
      "valid_until" => DateTime.to_iso8601(o.valid_until),
      "authoring_role" => Atom.to_string(o.authoring_role),
      "self_id" => o.self_id,
      "rationale" => o.rationale,
      "work_product" => o.work_product,
      "evidence" => Enum.map(o.evidence, &serialise_evidence/1)
    }
    |> put_artifact_identity(identity)
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
