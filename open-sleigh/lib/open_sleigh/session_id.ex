defmodule OpenSleigh.SessionId do
  @moduledoc """
  Opaque unique identifier for a `(Ticket × Phase × ConfigHash ×
  AdapterSession)` unit of work owned by exactly one `AgentWorker`.

  Per `specs/target-system/TARGET_SYSTEM_MODEL.md §Session`: a
  `SessionId` is "opaque binary". Current encoding: 16 random bytes
  base64url-encoded without padding (22 chars). Uniqueness is
  probabilistic (2^128 bits); for an engine running < 10^12 sessions
  over its lifetime, collision probability is negligible.

  The opaque tag stops Dialyzer from accepting a raw binary where a
  `SessionId` is expected.
  """

  @opaque t :: binary()

  @expected_length 22

  @doc "Generate a fresh, strongly-random `SessionId`."
  @spec generate() :: t()
  def generate do
    16
    |> :crypto.strong_rand_bytes()
    |> Base.url_encode64(padding: false)
  end

  @doc """
  Wrap an existing serialised id (e.g. from WAL replay). Rejects
  strings that don't match the canonical 22-char base64url shape.
  """
  @spec new(binary()) :: {:ok, t()} | {:error, :invalid_format}
  def new(binary) when is_binary(binary) and byte_size(binary) == @expected_length do
    if String.match?(binary, ~r/^[A-Za-z0-9_-]{22}$/),
      do: {:ok, binary},
      else: {:error, :invalid_format}
  end

  def new(_), do: {:error, :invalid_format}

  @doc "Serialise as the canonical string form."
  @spec to_string(t()) :: String.t()
  def to_string(id) when is_binary(id), do: id

  @doc "Runtime shape check."
  @spec valid?(term()) :: boolean()
  def valid?(value) when is_binary(value) and byte_size(value) == @expected_length do
    String.match?(value, ~r/^[A-Za-z0-9_-]{22}$/)
  end

  def valid?(_), do: false
end
