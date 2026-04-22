defmodule OpenSleigh.HumanGateApproval do
  @moduledoc """
  Evidence that a human principal approved a specific transition.

  Per `specs/target-system/TARGET_SYSTEM_MODEL.md §HumanGateApproval`
  and `ILLEGAL_STATES.md` PR8 / PR9 / UP2.

  Constructed only when a valid `/approve` signal arrives through one
  of the declared signal sources. In MVP-1 this happens in the L5
  `HumanGateListener`; the approver-authorisation check (`approver ∈
  SleighConfig.approvers`) lives there because L1 does not have
  access to the compiled config. At L1 we enforce:

  * `@enforce_keys` on all required fields (PR9)
  * `config_hash` is the active session's hash (pinned at approval
    time; defence against approval reuse across sleigh.md reloads)
  * `reason`, when present, is bounded at 500 chars

  `PhaseOutcome.new/2` consumes `HumanGateApproval` values through the
  `gate_results` list and validates gate-config consistency per PR10.
  """

  @typedoc "Source of the `/approve` signal."
  @type signal_source :: :tracker_comment | :github_review | :cli_ack

  @enforce_keys [:approver, :at, :config_hash, :signal_source, :signal_ref]
  defstruct [:approver, :at, :config_hash, :signal_source, :signal_ref, :reason]

  @type t :: %__MODULE__{
          approver: String.t(),
          at: DateTime.t(),
          config_hash: OpenSleigh.ConfigHash.t(),
          reason: String.t() | nil,
          signal_source: signal_source(),
          signal_ref: String.t()
        }

  @type new_error ::
          :invalid_approver
          | :invalid_at
          | :invalid_config_hash
          | :reason_too_long
          | :invalid_signal_source
          | :invalid_signal_ref

  @reason_max_chars 500

  @doc """
  Construct a `HumanGateApproval`. All fields required except `reason`
  (which may be `nil` but, if present, is capped at 500 chars).

  Time-as-parameter per L1 discipline: the caller supplies `at`.
  """
  @spec new(
          String.t(),
          DateTime.t(),
          OpenSleigh.ConfigHash.t(),
          signal_source(),
          String.t(),
          String.t() | nil
        ) :: {:ok, t()} | {:error, new_error()}
  def new(approver, at, config_hash, signal_source, signal_ref, reason) do
    with :ok <- validate_approver(approver),
         :ok <- validate_at(at),
         :ok <- validate_config_hash(config_hash),
         :ok <- validate_signal_source(signal_source),
         :ok <- validate_signal_ref(signal_ref),
         :ok <- validate_reason(reason) do
      {:ok,
       %__MODULE__{
         approver: approver,
         at: at,
         config_hash: config_hash,
         signal_source: signal_source,
         signal_ref: signal_ref,
         reason: reason
       }}
    end
  end

  # ——— validators ———

  @spec validate_approver(term()) :: :ok | {:error, :invalid_approver}
  defp validate_approver(approver) when is_binary(approver) and byte_size(approver) > 0, do: :ok
  defp validate_approver(_), do: {:error, :invalid_approver}

  @spec validate_at(term()) :: :ok | {:error, :invalid_at}
  defp validate_at(%DateTime{}), do: :ok
  defp validate_at(_), do: {:error, :invalid_at}

  @spec validate_config_hash(term()) :: :ok | {:error, :invalid_config_hash}
  defp validate_config_hash(hash) do
    if OpenSleigh.ConfigHash.valid?(hash) do
      :ok
    else
      {:error, :invalid_config_hash}
    end
  end

  @spec validate_signal_source(term()) :: :ok | {:error, :invalid_signal_source}
  defp validate_signal_source(source)
       when source in [:tracker_comment, :github_review, :cli_ack],
       do: :ok

  defp validate_signal_source(_), do: {:error, :invalid_signal_source}

  @spec validate_signal_ref(term()) :: :ok | {:error, :invalid_signal_ref}
  defp validate_signal_ref(ref) when is_binary(ref) and byte_size(ref) > 0, do: :ok
  defp validate_signal_ref(_), do: {:error, :invalid_signal_ref}

  @spec validate_reason(term()) :: :ok | {:error, :reason_too_long | :invalid_approver}
  defp validate_reason(nil), do: :ok

  defp validate_reason(reason) when is_binary(reason) do
    if String.length(reason) <= @reason_max_chars do
      :ok
    else
      {:error, :reason_too_long}
    end
  end

  defp validate_reason(_), do: {:error, :invalid_approver}
end
