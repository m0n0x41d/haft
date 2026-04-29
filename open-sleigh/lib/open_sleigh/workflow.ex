defmodule OpenSleigh.Workflow do
  @moduledoc """
  Immutable graph data describing the legal phase transitions for a
  workflow (`:mvp1` / `:mvp1r` / `:mvp2`).

  Per `specs/target-system/TARGET_SYSTEM_MODEL.md §Workflow` +
  `ILLEGAL_STATES.md` TR1–TR7.

  Constructed at compile time via `mvp1/0` / `mvp2/0` — no runtime
  graph mutation. `OpenSleigh.PhaseMachine` (L3) dispatches on this
  data; its pattern-match exhaustiveness is total over
  `Phase.t()` because the alphabet is closed (Q-OS-2 v0.5).

  MVP-1 transitions (pass-path; fail routes to `:terminal` with
  `Verdict.fail`):

      :frame    → :execute
      :execute  → :measure
      :measure  → :terminal

  MVP-1R inserts commission Preflight before Frame:

      :preflight → :frame → :execute → :measure → :terminal

  MVP-2 transitions implement the full lemniscate per
  `specs/target-system/PHASE_ONTOLOGY.md §MVP-2 phase graph`; SPEC §4
  carries only the overview diagram.
  """

  alias OpenSleigh.Phase

  @enforce_keys [:id, :phases, :entry_phase, :advance_map, :terminal_phases]
  defstruct [:id, :phases, :entry_phase, :advance_map, :terminal_phases]

  @type t :: %__MODULE__{
          id: :mvp1 | :mvp1r | :mvp2,
          phases: [Phase.t()],
          entry_phase: Phase.t(),
          advance_map: %{Phase.t() => Phase.t()},
          terminal_phases: [Phase.t()]
        }

  # Structs are built in function bodies (not module attributes)
  # because `%__MODULE__{}` is not available at attribute-expansion
  # time. These functions are pure and inlineable.

  @doc """
  MVP-1 workflow — three phases + terminal, strict linear advance.
  """
  @spec mvp1() :: t()
  def mvp1 do
    %__MODULE__{
      id: :mvp1,
      phases: [:frame, :execute, :measure, :terminal],
      entry_phase: :frame,
      advance_map: %{
        frame: :execute,
        execute: :measure,
        measure: :terminal
      },
      terminal_phases: [:terminal]
    }
  end

  @doc """
  MVP-1R workflow — commission-first Preflight plus the MVP-1 governed pipeline.
  """
  @spec mvp1r() :: t()
  def mvp1r do
    %__MODULE__{
      id: :mvp1r,
      phases: [:preflight, :frame, :execute, :measure, :terminal],
      entry_phase: :preflight,
      advance_map: %{
        preflight: :frame,
        frame: :execute,
        execute: :measure,
        measure: :terminal
      },
      terminal_phases: [:terminal]
    }
  end

  @doc """
  MVP-2 workflow — full lemniscate. Not routed until MVP-2 lands.
  """
  @spec mvp2() :: t()
  def mvp2 do
    %__MODULE__{
      id: :mvp2,
      phases: [
        :characterize_situation,
        :measure_situation,
        :problematize,
        :select_spec,
        :accept_spec,
        :generate,
        :parity_run,
        :select,
        :commission,
        :measure_impact,
        :terminal
      ],
      entry_phase: :characterize_situation,
      advance_map: %{
        characterize_situation: :measure_situation,
        measure_situation: :problematize,
        problematize: :select_spec,
        select_spec: :accept_spec,
        accept_spec: :generate,
        generate: :parity_run,
        parity_run: :select,
        select: :commission,
        commission: :measure_impact,
        # MVP-2 loops back to Characterize; here we model it as a
        # fresh workflow entry rather than in-graph loop. A separate
        # Session is opened for the new cycle.
        measure_impact: :terminal
      },
      terminal_phases: [:terminal]
    }
  end

  @doc """
  Legal phase to advance to from `from_phase`, or `nil` if `from_phase`
  is terminal. Total function over `Phase.t()` in the workflow's
  alphabet — arbitrary atoms raise `FunctionClauseError`.
  """
  @spec advance_from(t(), Phase.t()) :: Phase.t() | nil
  def advance_from(%__MODULE__{advance_map: map}, from_phase)
      when is_map_key(map, from_phase) do
    Map.fetch!(map, from_phase)
  end

  def advance_from(%__MODULE__{terminal_phases: terminals}, from_phase) do
    if from_phase in terminals do
      nil
    else
      raise ArgumentError,
            "phase #{inspect(from_phase)} not in workflow alphabet"
    end
  end

  @doc "Is `phase` terminal in this workflow?"
  @spec terminal?(t(), Phase.t()) :: boolean()
  def terminal?(%__MODULE__{terminal_phases: terminals}, phase) do
    phase in terminals
  end

  @doc "Is `phase` in this workflow's alphabet?"
  @spec contains_phase?(t(), Phase.t()) :: boolean()
  def contains_phase?(%__MODULE__{phases: phases}, phase) do
    phase in phases
  end
end
