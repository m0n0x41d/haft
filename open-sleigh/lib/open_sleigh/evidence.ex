defmodule OpenSleigh.Evidence do
  @moduledoc """
  A typed reference to a piece of external proof that supports a
  `PhaseOutcome`'s claim.

  Per `specs/target-system/TARGET_SYSTEM_MODEL.md §Evidence` +
  `ILLEGAL_STATES.md` PR5–PR7.

  **This constructor does NOT enforce the self-reference check
  (PR5).** Evidence in isolation does not know which artifact it is
  evidence *for*; the check lives at `PhaseOutcome.new/2` where
  `self_id` is a required provenance input. This is the v0.5 fix to
  the 5.4 Pro CRITICAL-2 finding ("constructor-signature impossibility"
  in v0.4).

  What this constructor DOES enforce:

  * `cl ∈ 0..3` (PR6)
  * `ref` non-empty (PR7-adjacent)
  * `authoring_source` is an atom AND not the reserved
    `:open_sleigh_self` value (OB4)
  * `@enforce_keys` on required fields (PR7)

  Banned `authoring_source` values:

  * `:open_sleigh_self` — reserved per OB4 / UP3. Any artifact
    authored by Open-Sleigh's own telemetry cannot enter the Haft
    graph as Evidence.
  """

  @typedoc """
  Kind of evidence — what this carrier is. Open atom set (new
  integrations may add new kinds), but common values are enumerated
  for documentation.
  """
  @type kind ::
          :pr_merge_sha
          | :ci_run_id
          | :test_count
          | :human_comment
          | :external_measurement
          | atom()

  @typedoc """
  Who produced this evidence item. Open atom set; `:open_sleigh_self`
  is blacklisted per OB4.
  """
  @type authoring_source ::
          :ci
          | :git_host
          | :tracker
          | :human
          | :external
          | :canary
          | atom()

  @enforce_keys [:kind, :ref, :cl, :authoring_source, :captured_at]
  defstruct [:kind, :ref, :hash, :cl, :authoring_source, :captured_at]

  @type t :: %__MODULE__{
          kind: kind(),
          ref: String.t(),
          hash: String.t() | nil,
          cl: 0..3,
          authoring_source: authoring_source(),
          captured_at: DateTime.t()
        }

  @type new_error ::
          :invalid_cl
          | :empty_ref
          | :invalid_kind
          | :forbidden_authoring_source
          | :invalid_captured_at

  @doc """
  Construct an `Evidence` struct. Returns `{:ok, t()}` on success,
  `{:error, reason}` on any validation failure.

  Time-as-parameter: `captured_at` is supplied by the caller; this
  module never calls `DateTime.utc_now/0`. Keeps L1 pure.
  """
  @spec new(
          kind(),
          String.t(),
          String.t() | nil,
          0..3,
          authoring_source(),
          DateTime.t()
        ) :: {:ok, t()} | {:error, new_error()}
  def new(kind, ref, hash, cl, authoring_source, captured_at) do
    with :ok <- validate_kind(kind),
         :ok <- validate_ref(ref),
         :ok <- validate_cl(cl),
         :ok <- validate_authoring_source(authoring_source),
         :ok <- validate_captured_at(captured_at),
         :ok <- validate_hash(hash) do
      {:ok,
       %__MODULE__{
         kind: kind,
         ref: ref,
         hash: hash,
         cl: cl,
         authoring_source: authoring_source,
         captured_at: captured_at
       }}
    end
  end

  # ——— validators ———

  @spec validate_kind(term()) :: :ok | {:error, :invalid_kind}
  defp validate_kind(kind) when is_atom(kind) and not is_nil(kind), do: :ok
  defp validate_kind(_), do: {:error, :invalid_kind}

  @spec validate_ref(term()) :: :ok | {:error, :empty_ref}
  defp validate_ref(ref) when is_binary(ref) and byte_size(ref) > 0, do: :ok
  defp validate_ref(_), do: {:error, :empty_ref}

  @spec validate_cl(term()) :: :ok | {:error, :invalid_cl}
  defp validate_cl(cl) when is_integer(cl) and cl >= 0 and cl <= 3, do: :ok
  defp validate_cl(_), do: {:error, :invalid_cl}

  @spec validate_authoring_source(term()) ::
          :ok | {:error, :forbidden_authoring_source}
  defp validate_authoring_source(:open_sleigh_self),
    do: {:error, :forbidden_authoring_source}

  defp validate_authoring_source(atom) when is_atom(atom) and not is_nil(atom), do: :ok
  defp validate_authoring_source(_), do: {:error, :forbidden_authoring_source}

  @spec validate_captured_at(term()) :: :ok | {:error, :invalid_captured_at}
  defp validate_captured_at(%DateTime{}), do: :ok
  defp validate_captured_at(_), do: {:error, :invalid_captured_at}

  @spec validate_hash(term()) :: :ok | {:error, :empty_ref}
  defp validate_hash(nil), do: :ok
  defp validate_hash(hash) when is_binary(hash) and byte_size(hash) > 0, do: :ok
  defp validate_hash(_), do: {:error, :empty_ref}
end
