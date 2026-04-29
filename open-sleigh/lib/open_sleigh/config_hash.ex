defmodule OpenSleigh.ConfigHash do
  @moduledoc """
  Opaque sha256 hash of the effective per-phase `sleigh.md` slice.

  Per `specs/target-system/SLEIGH_CONFIG.md §2` (hash-pinning, v0.5
  narrowed to per-phase scope):

      config_hash = sha256(
        engine || tracker || agent || haft || external_publication
        || phases[this_phase] || prompts[this_phase]
      )

  Serialised as lowercase hex (64 chars). The opaque tag exists to stop
  callers from confusing a `ConfigHash` with an arbitrary string at
  Dialyzer boundaries (SE7: every Haft write requires an attached hash).

  Cross-refs: `ILLEGAL_STATES.md` PR1 (PhaseOutcome without hash),
  SE7 (Haft write without hash).
  """

  @opaque t :: binary()

  @hex_length 64

  @doc """
  Wrap an already-computed hex string. Used for decoding / round-tripping
  — NOT for computing a hash from raw input (use `from_iodata/1` for
  that).
  """
  @spec new(binary()) :: {:ok, t()} | {:error, :invalid_length | :not_hex}
  def new(hex) when is_binary(hex) and byte_size(hex) == @hex_length do
    if String.match?(hex, ~r/^[0-9a-f]{64}$/) do
      {:ok, hex}
    else
      {:error, :not_hex}
    end
  end

  def new(hex) when is_binary(hex), do: {:error, :invalid_length}

  @doc """
  Compute a `ConfigHash` from arbitrary iodata — the canonical path for
  L6 `Sleigh.Compiler` to pin a per-phase slice.
  """
  @spec from_iodata(iodata()) :: t()
  def from_iodata(data) do
    :sha256
    |> :crypto.hash(data)
    |> Base.encode16(case: :lower)
  end

  @doc "Serialise as the 64-char hex string."
  @spec to_string(t()) :: String.t()
  def to_string(hash) when is_binary(hash), do: hash

  @doc """
  Is `value` a structurally-valid `ConfigHash` payload? (64 lowercase
  hex chars.) Runtime backstop for functions that take an untyped
  binary at the L4/L5 boundary.
  """
  @spec valid?(term()) :: boolean()
  def valid?(value) when is_binary(value) and byte_size(value) == @hex_length do
    String.match?(value, ~r/^[0-9a-f]{64}$/)
  end

  def valid?(_), do: false
end
