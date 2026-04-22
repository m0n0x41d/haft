defmodule OpenSleigh.PhaseOutcome do
  @moduledoc """
  Immutable artifact produced when a phase completes. The primary data
  type flowing through the system.

  Per `specs/target-system/TARGET_SYSTEM_MODEL.md §PhaseOutcome` +
  `ILLEGAL_STATES.md` PR1–PR10.

  **This is the single constructor** per Q-OS-3 v0.5 resolution. No
  `new_external/3` special path. Every field is validated; gate-
  config consistency (PR10) and evidence self-reference (PR5, v0.5
  CRITICAL-2 fix) are enforced here, at the layer that has access to
  `self_id`, `gate_results`, and `phase_config` in one call.
  """

  alias OpenSleigh.{
    AuthoringRole,
    ConfigHash,
    Evidence,
    GateResult,
    HumanGateApproval,
    Phase,
    PhaseConfig,
    SessionId,
    SessionScopedArtifactId,
    Verdict
  }

  @rationale_max_chars 1000

  @enforce_keys [
    :session_id,
    :phase,
    :work_product,
    :evidence,
    :gate_results,
    :config_hash,
    :valid_until,
    :authoring_role,
    :self_id,
    :produced_at,
    :phase_config
  ]
  defstruct [
    :session_id,
    :phase,
    :verdict,
    :work_product,
    :evidence,
    :gate_results,
    :config_hash,
    :valid_until,
    :authoring_role,
    :self_id,
    :rationale,
    :produced_at,
    :phase_config
  ]

  @type t :: %__MODULE__{
          session_id: SessionId.t(),
          phase: Phase.t(),
          verdict: Verdict.t() | nil,
          work_product: map(),
          evidence: [Evidence.t()],
          gate_results: [GateResult.t()],
          config_hash: ConfigHash.t(),
          valid_until: DateTime.t(),
          authoring_role: AuthoringRole.t(),
          self_id: SessionScopedArtifactId.t(),
          rationale: String.t() | nil,
          produced_at: DateTime.t(),
          phase_config: PhaseConfig.t()
        }

  @type new_error ::
          :invalid_session_id
          | :invalid_phase
          | :invalid_work_product
          | :invalid_evidence
          | :invalid_gate_results
          | :invalid_config_hash
          | :invalid_valid_until
          | :invalid_authoring_role
          | :invalid_self_id
          | :rationale_too_long
          | :invalid_produced_at
          | :invalid_phase_config
          | :evidence_self_reference
          | :evidence_required_on_measure
          | :human_gate_required_by_phase_config_but_missing
          | :verdict_only_on_terminal
          | :authoring_role_does_not_match_phase_config

  @doc """
  Construct a `PhaseOutcome`.

  Expects `attrs` (map or keyword list) with all `@enforce_keys`
  fields set. Enforces (per v0.5/v0.6 taxonomy):

  * PR1–PR3 — `@enforce_keys` catches missing provenance fields
  * PR4 — `rationale` ≤ 1000 chars (nil allowed)
  * **PR5 — evidence self-reference: no `evidence[i].ref ==
    SessionScopedArtifactId.to_string(self_id)`** (v0.5 CRITICAL-2
    fix: the check lives where `self_id` is known)
  * PR10 — gate-config consistency: if `phase_config.gates.human`
    declares any human gate, `gate_results` must contain at least one
    `{:human, %HumanGateApproval{}}` entry (Q-OS-3 v0.5 single-
    constructor path; no `new_external/3`)
  * Verdict is non-nil only on `:terminal` phase
  * Measure phase requires non-empty evidence list
  * `authoring_role` matches `phase_config.agent_role` (or is
    `:judge` / `:human` for gate-authored outcomes)
  """
  @spec new(keyword() | map()) :: {:ok, t()} | {:error, new_error()}
  def new(attrs) when is_list(attrs), do: new(Map.new(attrs))

  def new(%{} = attrs) do
    with :ok <- validate_session_id(attrs[:session_id]),
         :ok <- validate_phase(attrs[:phase]),
         :ok <- validate_work_product(attrs[:work_product]),
         :ok <- validate_evidence_list(attrs[:evidence]),
         :ok <- validate_gate_results(attrs[:gate_results]),
         :ok <- validate_config_hash(attrs[:config_hash]),
         :ok <- validate_valid_until(attrs[:valid_until]),
         :ok <- validate_authoring_role(attrs[:authoring_role]),
         :ok <- validate_self_id(attrs[:self_id]),
         :ok <- validate_rationale(Map.get(attrs, :rationale)),
         :ok <- validate_produced_at(attrs[:produced_at]),
         :ok <- validate_phase_config(attrs[:phase_config]),
         :ok <- validate_evidence_self_ref(attrs[:evidence], attrs[:self_id]),
         :ok <- validate_gate_config_consistency(attrs[:gate_results], attrs[:phase_config]),
         :ok <- validate_verdict(Map.get(attrs, :verdict), attrs[:phase]),
         :ok <- validate_measure_has_evidence(attrs[:phase], attrs[:evidence]),
         :ok <- validate_role_phase_match(attrs[:authoring_role], attrs[:phase_config]) do
      {:ok,
       %__MODULE__{
         session_id: attrs.session_id,
         phase: attrs.phase,
         verdict: Map.get(attrs, :verdict),
         work_product: attrs.work_product,
         evidence: attrs.evidence,
         gate_results: attrs.gate_results,
         config_hash: attrs.config_hash,
         valid_until: attrs.valid_until,
         authoring_role: attrs.authoring_role,
         self_id: attrs.self_id,
         rationale: Map.get(attrs, :rationale),
         produced_at: attrs.produced_at,
         phase_config: attrs.phase_config
       }}
    end
  end

  # ——— field-shape validators ———

  @spec validate_session_id(term()) :: :ok | {:error, :invalid_session_id}
  defp validate_session_id(id) do
    if SessionId.valid?(id), do: :ok, else: {:error, :invalid_session_id}
  end

  @spec validate_phase(term()) :: :ok | {:error, :invalid_phase}
  defp validate_phase(p) do
    if Phase.valid?(p), do: :ok, else: {:error, :invalid_phase}
  end

  @spec validate_work_product(term()) :: :ok | {:error, :invalid_work_product}
  defp validate_work_product(wp) when is_map(wp), do: :ok
  defp validate_work_product(_), do: {:error, :invalid_work_product}

  @spec validate_evidence_list(term()) :: :ok | {:error, :invalid_evidence}
  defp validate_evidence_list(list) when is_list(list) do
    if Enum.all?(list, &match?(%Evidence{}, &1)),
      do: :ok,
      else: {:error, :invalid_evidence}
  end

  defp validate_evidence_list(_), do: {:error, :invalid_evidence}

  @spec validate_gate_results(term()) :: :ok | {:error, :invalid_gate_results}
  defp validate_gate_results(list) when is_list(list) do
    if Enum.all?(list, &GateResult.valid?/1),
      do: :ok,
      else: {:error, :invalid_gate_results}
  end

  defp validate_gate_results(_), do: {:error, :invalid_gate_results}

  @spec validate_config_hash(term()) :: :ok | {:error, :invalid_config_hash}
  defp validate_config_hash(h) do
    if ConfigHash.valid?(h), do: :ok, else: {:error, :invalid_config_hash}
  end

  @spec validate_valid_until(term()) :: :ok | {:error, :invalid_valid_until}
  defp validate_valid_until(%DateTime{}), do: :ok
  defp validate_valid_until(_), do: {:error, :invalid_valid_until}

  @spec validate_authoring_role(term()) :: :ok | {:error, :invalid_authoring_role}
  defp validate_authoring_role(role) do
    if AuthoringRole.valid?(role), do: :ok, else: {:error, :invalid_authoring_role}
  end

  @spec validate_self_id(term()) :: :ok | {:error, :invalid_self_id}
  defp validate_self_id(id) do
    if SessionScopedArtifactId.valid?(id), do: :ok, else: {:error, :invalid_self_id}
  end

  @spec validate_rationale(term()) :: :ok | {:error, :rationale_too_long}
  defp validate_rationale(nil), do: :ok

  defp validate_rationale(text) when is_binary(text) do
    if String.length(text) <= @rationale_max_chars,
      do: :ok,
      else: {:error, :rationale_too_long}
  end

  defp validate_rationale(_), do: {:error, :rationale_too_long}

  @spec validate_produced_at(term()) :: :ok | {:error, :invalid_produced_at}
  defp validate_produced_at(%DateTime{}), do: :ok
  defp validate_produced_at(_), do: {:error, :invalid_produced_at}

  @spec validate_phase_config(term()) :: :ok | {:error, :invalid_phase_config}
  defp validate_phase_config(%PhaseConfig{}), do: :ok
  defp validate_phase_config(_), do: {:error, :invalid_phase_config}

  # ——— cross-field invariants ———

  # PR5 — evidence self-reference check (v0.5 CRITICAL-2 fix).
  # The check lives here because this is the layer that knows
  # `self_id`. Evidence.new/5 cannot do it in isolation.
  @spec validate_evidence_self_ref([Evidence.t()], SessionScopedArtifactId.t()) ::
          :ok | {:error, :evidence_self_reference}
  defp validate_evidence_self_ref(evidence, self_id) do
    self_id_str = SessionScopedArtifactId.to_string(self_id)

    if Enum.any?(evidence, &(&1.ref == self_id_str)) do
      {:error, :evidence_self_reference}
    else
      :ok
    end
  end

  # PR10 — gate-config consistency (v0.5 Q-OS-3).
  # If the phase's PhaseConfig declares any human gate, gate_results
  # must contain at least one approved :human result.
  @spec validate_gate_config_consistency([GateResult.t()], PhaseConfig.t()) ::
          :ok | {:error, :human_gate_required_by_phase_config_but_missing}
  defp validate_gate_config_consistency(gate_results, %PhaseConfig{} = pc) do
    if PhaseConfig.declares_human_gate?(pc) do
      if has_approved_human_result?(gate_results) do
        :ok
      else
        {:error, :human_gate_required_by_phase_config_but_missing}
      end
    else
      :ok
    end
  end

  @spec has_approved_human_result?([GateResult.t()]) :: boolean()
  defp has_approved_human_result?(gate_results) do
    Enum.any?(gate_results, fn
      {:human, %HumanGateApproval{}} -> true
      _ -> false
    end)
  end

  # Verdict only on :terminal phase.
  @spec validate_verdict(term(), Phase.t()) ::
          :ok | {:error, :verdict_only_on_terminal}
  defp validate_verdict(nil, _), do: :ok
  defp validate_verdict(_verdict, :terminal), do: :ok
  defp validate_verdict(_, _), do: {:error, :verdict_only_on_terminal}

  # Measure must have non-empty evidence.
  @spec validate_measure_has_evidence(Phase.t(), [Evidence.t()]) ::
          :ok | {:error, :evidence_required_on_measure}
  defp validate_measure_has_evidence(:measure, []),
    do: {:error, :evidence_required_on_measure}

  defp validate_measure_has_evidence(_, _), do: :ok

  # The authoring_role should match phase_config.agent_role (for the
  # agent's own outcomes), OR be :judge (for a reviewer-authored
  # outcome), OR :human (for HumanGateListener-authored events).
  @spec validate_role_phase_match(AuthoringRole.t(), PhaseConfig.t()) ::
          :ok | {:error, :authoring_role_does_not_match_phase_config}
  defp validate_role_phase_match(role, _) when role in [:judge, :human], do: :ok

  defp validate_role_phase_match(role, %PhaseConfig{agent_role: expected_role}) do
    if role == expected_role,
      do: :ok,
      else: {:error, :authoring_role_does_not_match_phase_config}
  end
end
