defmodule OpenSleigh.GateContext do
  @moduledoc """
  Input bundle for gate evaluation — everything a structural / semantic /
  human gate needs to reason about the phase about to exit.

  Per `specs/enabling-system/FUNCTIONAL_ARCHITECTURE.md` L2 +
  `specs/enabling-system/REFERENCE_ALGORITHMS.md §4` worker loop.
  Built by L5 `AgentWorker` from the session state, the agent's turn
  result, the evidence collected, and any upstream artifacts already
  fetched (e.g. the upstream ProblemCard for Frame gates).

  Gates operate on this context, NOT on a constructed `PhaseOutcome` —
  gate_results are produced BEFORE `PhaseOutcome.new/2` is called (the
  reference algorithm in `REFERENCE_ALGORITHMS.md §4` shows the
  ordering). This is also why `OpenSleigh.GateResult.valid?/1` doesn't
  depend on a full outcome.
  """

  alias OpenSleigh.{ConfigHash, Evidence, Phase, PhaseConfig, SessionScopedArtifactId, Ticket}

  @enforce_keys [
    :phase,
    :phase_config,
    :ticket,
    :self_id,
    :config_hash,
    :turn_result,
    :evidence
  ]
  defstruct [
    :phase,
    :phase_config,
    :ticket,
    :self_id,
    :config_hash,
    :turn_result,
    :evidence,
    :upstream_problem_card,
    :proposed_valid_until
  ]

  @type t :: %__MODULE__{
          phase: Phase.t(),
          phase_config: PhaseConfig.t(),
          ticket: Ticket.t(),
          self_id: SessionScopedArtifactId.t(),
          config_hash: ConfigHash.t(),
          turn_result: map(),
          evidence: [Evidence.t()],
          upstream_problem_card: map() | nil,
          proposed_valid_until: DateTime.t() | nil
        }

  @type new_error ::
          :invalid_phase
          | :invalid_phase_config
          | :invalid_ticket
          | :invalid_self_id
          | :invalid_config_hash
          | :invalid_turn_result
          | :invalid_evidence

  @doc """
  Build a `GateContext` with shape validation of required fields.
  """
  @spec new(keyword() | map()) :: {:ok, t()} | {:error, new_error()}
  def new(attrs) when is_list(attrs), do: new(Map.new(attrs))

  def new(%{} = attrs) do
    with :ok <- validate_phase(attrs[:phase]),
         :ok <- validate_phase_config(attrs[:phase_config]),
         :ok <- validate_ticket(attrs[:ticket]),
         :ok <- validate_self_id(attrs[:self_id]),
         :ok <- validate_config_hash(attrs[:config_hash]),
         :ok <- validate_turn_result(attrs[:turn_result]),
         :ok <- validate_evidence(attrs[:evidence]) do
      {:ok,
       %__MODULE__{
         phase: attrs.phase,
         phase_config: attrs.phase_config,
         ticket: attrs.ticket,
         self_id: attrs.self_id,
         config_hash: attrs.config_hash,
         turn_result: attrs.turn_result,
         evidence: attrs.evidence,
         upstream_problem_card: Map.get(attrs, :upstream_problem_card),
         proposed_valid_until: Map.get(attrs, :proposed_valid_until)
       }}
    end
  end

  @spec validate_phase(term()) :: :ok | {:error, :invalid_phase}
  defp validate_phase(p) do
    if Phase.valid?(p), do: :ok, else: {:error, :invalid_phase}
  end

  @spec validate_phase_config(term()) :: :ok | {:error, :invalid_phase_config}
  defp validate_phase_config(%PhaseConfig{}), do: :ok
  defp validate_phase_config(_), do: {:error, :invalid_phase_config}

  @spec validate_ticket(term()) :: :ok | {:error, :invalid_ticket}
  defp validate_ticket(%Ticket{}), do: :ok
  defp validate_ticket(_), do: {:error, :invalid_ticket}

  @spec validate_self_id(term()) :: :ok | {:error, :invalid_self_id}
  defp validate_self_id(id) do
    if SessionScopedArtifactId.valid?(id), do: :ok, else: {:error, :invalid_self_id}
  end

  @spec validate_config_hash(term()) :: :ok | {:error, :invalid_config_hash}
  defp validate_config_hash(h) do
    if ConfigHash.valid?(h), do: :ok, else: {:error, :invalid_config_hash}
  end

  @spec validate_turn_result(term()) :: :ok | {:error, :invalid_turn_result}
  defp validate_turn_result(t) when is_map(t), do: :ok
  defp validate_turn_result(_), do: {:error, :invalid_turn_result}

  @spec validate_evidence(term()) :: :ok | {:error, :invalid_evidence}
  defp validate_evidence(list) when is_list(list) do
    if Enum.all?(list, &match?(%Evidence{}, &1)),
      do: :ok,
      else: {:error, :invalid_evidence}
  end

  defp validate_evidence(_), do: {:error, :invalid_evidence}
end
