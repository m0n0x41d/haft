defmodule OpenSleigh.Verdict do
  @moduledoc """
  Closed sum of phase / outcome verdicts.

  Per `specs/target-system/PHASE_ONTOLOGY.md` axis 3: `:pass | :fail |
  :partial`. A free-string verdict is a type error — there are three
  atoms, no extension mechanism, and the constructor APIs refuse anything
  else.

  `:partial` is reserved for MVP-2 Solution-Factory outcomes where a
  Pareto selection lands one variant but comparison wasn't clean. In
  MVP-1, only `:pass` and `:fail` are produced; `:partial` is declared
  for type-level completeness.
  """

  @typedoc """
  A verdict atom.
  """
  @type t :: :pass | :fail | :partial

  @all [:pass, :fail, :partial]

  @doc "All admissible verdict atoms."
  @spec all() :: [t(), ...]
  def all, do: @all

  @doc """
  Is `value` a valid verdict? False for anything outside the closed sum.
  """
  @spec valid?(term()) :: boolean()
  def valid?(value) when value in @all, do: true
  def valid?(_), do: false

  @doc "Convenience — did the phase / outcome pass?"
  @spec pass?(t()) :: boolean()
  def pass?(:pass), do: true
  def pass?(_), do: false
end
