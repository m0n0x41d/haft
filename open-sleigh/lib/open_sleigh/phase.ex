defmodule OpenSleigh.Phase do
  @moduledoc """
  Closed sum of all Open-Sleigh phase atoms — MVP-1 and MVP-2.

  Per Q-OS-2 resolution (v0.5), the full phase alphabet is pre-declared so
  that `OpenSleigh.PhaseMachine.next/2` exhaustiveness is total and TR5
  ("dynamic phase outside workflow alphabet") is genuinely type-level
  inexpressible — arbitrary atoms fail the guard / pattern-match.

  See `specs/target-system/PHASE_ONTOLOGY.md` axis 1 for the canonical
  alphabet and `specs/target-system/ILLEGAL_STATES.md` TR5 for the
  enforcement label.

  MVP-1 uses `:frame, :execute, :measure, :terminal`. MVP-1R adds
  `:preflight` before Frame for commission-first intake. MVP-2 phases are
  declared but not yet routed through — `OpenSleigh.Workflow.mvp1/0` uses
  a strict subset; `OpenSleigh.Workflow.mvp2/0` uses the full alphabet.
  """

  @typedoc """
  A phase atom. Closed sum — any atom not in this union is not a phase.
  """
  # MVP-1 alphabet (per `specs/target-system/SCOPE_FREEZE.md §MVP-1`
  # + `PHASE_ONTOLOGY.md §MVP-1 phase graph`).
  @type t ::
          :preflight
          | :frame
          | :execute
          | :measure
          | :terminal
          # MVP-2 alphabet (per `specs/target-system/PHASE_ONTOLOGY.md
          # §MVP-2 phase graph` + `.context/development_for_the_developed.md`
          # Slide 12). Declared but not yet routed by Workflow.mvp1/0.
          | :characterize_situation
          | :measure_situation
          | :problematize
          | :select_spec
          | :accept_spec
          | :generate
          | :parity_run
          | :select
          | :commission
          | :measure_impact

  @mvp1 [:frame, :execute, :measure, :terminal]
  @mvp1r [:preflight, :frame, :execute, :measure, :terminal]

  @mvp2_additions [
    :characterize_situation,
    :measure_situation,
    :problematize,
    :select_spec,
    :accept_spec,
    :generate,
    :parity_run,
    :select,
    :commission,
    :measure_impact
  ]

  @all @mvp1r ++ @mvp2_additions

  @doc """
  Full phase alphabet (MVP-1 + MVP-2). Returned as a list for enumeration;
  the type `t()` is the authoritative closure.
  """
  @spec all() :: [t(), ...]
  def all, do: @all

  @doc """
  MVP-1 subset — phases actually routed by `Workflow.mvp1/0`.
  """
  @spec mvp1() :: [t(), ...]
  def mvp1, do: @mvp1

  @doc """
  MVP-1R subset — phases routed by `Workflow.mvp1r/0`.
  """
  @spec mvp1r() :: [t(), ...]
  def mvp1r, do: @mvp1r

  @doc """
  MVP-2 additions — phases added to the alphabet in MVP-2 on top of MVP-1.
  """
  @spec mvp2_additions() :: [t(), ...]
  def mvp2_additions, do: @mvp2_additions

  @doc """
  Is `value` a valid phase atom? Returns `false` for anything not in the
  closed sum — strings, maps, tuples, or unknown atoms.

  The type system cannot prevent arbitrary atoms at runtime (Elixir is not
  Haskell); this guard is the constructor-level backstop per TR5.
  """
  @spec valid?(term()) :: boolean()
  def valid?(value) when value in @all, do: true
  def valid?(_), do: false

  @doc """
  Is `phase` in the MVP-1 routed subset?
  """
  @spec mvp1?(t()) :: boolean()
  def mvp1?(phase) when phase in @mvp1, do: true
  def mvp1?(_), do: false

  @doc """
  Is `phase` terminal (absorbing in the workflow graph)? Per TR2 — terminal
  phases cannot transition to any non-terminal phase.
  """
  @spec terminal?(t()) :: boolean()
  def terminal?(:terminal), do: true
  def terminal?(_), do: false

  @doc """
  Is `phase` single-turn (no continuation turns allowed)? Per CT4 —
  Frame and Measure are structurally single-turn; continuation-turn loops
  only apply to Execute and MVP-2 Solution-Factory phases.
  """
  @spec single_turn?(t()) :: boolean()
  def single_turn?(phase) when phase in [:preflight, :frame, :measure, :terminal], do: true

  def single_turn?(phase)
      when phase in [
             :characterize_situation,
             :measure_situation,
             :problematize,
             :select_spec,
             :accept_spec,
             :select,
             :commission,
             :measure_impact
           ],
      do: true

  def single_turn?(phase) when phase in [:execute, :generate, :parity_run], do: false
end
