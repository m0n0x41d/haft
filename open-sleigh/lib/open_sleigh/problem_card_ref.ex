defmodule OpenSleigh.ProblemCardRef do
  @moduledoc """
  Opaque pointer to a Haft ProblemCard produced upstream by the human
  via Haft + `/h-reason`.

  Per `specs/target-system/TERM_MAP.md` and `ILLEGAL_STATES.md` UP1:
  every MVP-1 `Ticket` MUST carry a valid `ProblemCardRef`. Tickets
  without one fail-fast at Frame entry with `:no_upstream_frame`.

  Open-Sleigh never *constructs* a `ProblemCardRef` as an authored
  artifact (UP2, UP3, OB4). This module's `new/1` wraps a reference
  pulled from the tracker ticket's metadata — the authorship check
  ("is this artifact self-authored by Open-Sleigh?") is a runtime gate
  at Frame entry (`problem_card_ref_present`), not a constructor-time
  check, because resolving the authorship requires a Haft query (L4).

  The opaque tag exists to make Dialyzer refuse raw strings where a
  validated ref is expected.
  """

  @opaque t :: binary()

  @doc """
  Wrap a non-empty string as a `ProblemCardRef`. Returns `{:error,
  :empty_or_invalid}` for anything not a non-empty binary.

  Validation is deliberately thin at L1: we do not know Haft's exact
  artifact-id format, and binding to a specific format here would
  couple L1 to a Haft-internal detail. The resolvability and
  `:open_sleigh_self` checks happen at the Frame-entry gate (L2).
  """
  @spec new(binary()) :: {:ok, t()} | {:error, :empty_or_invalid}
  def new(ref) when is_binary(ref) and byte_size(ref) > 0, do: {:ok, ref}
  def new(_), do: {:error, :empty_or_invalid}

  @doc "Serialise as the canonical string form."
  @spec to_string(t()) :: String.t()
  def to_string(ref) when is_binary(ref), do: ref

  @doc "Runtime shape check."
  @spec valid?(term()) :: boolean()
  def valid?(value) when is_binary(value) and byte_size(value) > 0, do: true
  def valid?(_), do: false
end
