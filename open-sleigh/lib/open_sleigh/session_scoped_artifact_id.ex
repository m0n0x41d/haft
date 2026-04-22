defmodule OpenSleigh.SessionScopedArtifactId do
  @moduledoc """
  Opaque identifier for an artifact produced inside a session — used as
  the `self_id` field on `PhaseOutcome` (required by Q-OS-3 resolution,
  v0.5, and the v0.5 PR5 fix to the 5.4 Pro CRITICAL-2 finding).

  `PhaseOutcome.new/2` rejects evidence whose `ref == self_id` because
  self-referential evidence is a degenerate proof (FPF A.10 CC-A10.6).
  The id is generated per outcome, not per session — every outcome
  within a session has a distinct `self_id`.
  """

  @opaque t :: binary()

  @expected_length 16

  @doc "Generate a fresh session-scoped artifact id."
  @spec generate() :: t()
  def generate do
    12
    |> :crypto.strong_rand_bytes()
    |> Base.url_encode64(padding: false)
  end

  @doc "Serialise as canonical string."
  @spec to_string(t()) :: String.t()
  def to_string(id) when is_binary(id), do: id

  @doc "Shape check — 16-char base64url."
  @spec valid?(term()) :: boolean()
  def valid?(value) when is_binary(value) and byte_size(value) == @expected_length do
    String.match?(value, ~r/^[A-Za-z0-9_-]{16}$/)
  end

  def valid?(_), do: false
end
