defmodule OpenSleigh.WorkflowState do
  @moduledoc """
  Per-ticket runtime state for the phase machine. Tracks the workflow
  graph, current phase, and accumulated outcomes.

  Per `specs/enabling-system/FUNCTIONAL_ARCHITECTURE.md` L3 Concepts:

      `WorkflowState` (current phase + accumulated outcomes + pending
      human gates)

  In MVP-1 we don't persist pending human gates on this struct —
  that's L5 `AgentWorker` sub-state. `WorkflowState` is purely the
  L3 logical state that `PhaseMachine.next/2` operates on.

  **Invariants (enforced by construction + transition functions):**

  * `current` is always in `workflow.phases`
  * `completed_outcomes` are in phase-order (no out-of-order
    transitions per TR1)
  * Once `current` is terminal, no further `apply_outcome/2` calls
    are accepted (TR2)
  """

  alias OpenSleigh.{Phase, PhaseOutcome, Workflow}

  @enforce_keys [:workflow, :current, :completed_outcomes]
  defstruct [:workflow, :current, :completed_outcomes]

  @type t :: %__MODULE__{
          workflow: Workflow.t(),
          current: Phase.t(),
          completed_outcomes: [PhaseOutcome.t()]
        }

  @type new_error ::
          :invalid_workflow
          | :invalid_current_phase
          | :current_phase_not_in_workflow

  @doc """
  Start a fresh workflow state at its entry phase.
  """
  @spec start(Workflow.t()) :: t()
  def start(%Workflow{} = workflow) do
    %__MODULE__{
      workflow: workflow,
      current: workflow.entry_phase,
      completed_outcomes: []
    }
  end

  @doc """
  Build a workflow state at an arbitrary phase — used by L5 when
  resuming a session on restart (WAL replay).
  """
  @spec new(Workflow.t(), Phase.t(), [PhaseOutcome.t()]) ::
          {:ok, t()} | {:error, new_error()}
  def new(%Workflow{} = workflow, current, outcomes)
      when is_list(outcomes) do
    cond do
      not Phase.valid?(current) ->
        {:error, :invalid_current_phase}

      not Workflow.contains_phase?(workflow, current) ->
        {:error, :current_phase_not_in_workflow}

      true ->
        {:ok, %__MODULE__{workflow: workflow, current: current, completed_outcomes: outcomes}}
    end
  end

  def new(_, _, _), do: {:error, :invalid_workflow}

  @doc """
  Apply a `PhaseOutcome` — prepend to history and advance `current`
  to the next phase per the workflow's `advance_map`.

  Returns `{:ok, new_state}` on advance; `{:error, :terminal_state}`
  if the current phase is already terminal (TR2 absorbing state);
  `{:error, :phase_mismatch}` if the outcome's phase doesn't match
  the state's `current` (this would be out-of-order application).
  """
  @spec apply_outcome(t(), PhaseOutcome.t()) ::
          {:ok, t()}
          | {:error, :terminal_state | :phase_mismatch | :workflow_has_no_successor}
  def apply_outcome(%__MODULE__{} = state, %PhaseOutcome{phase: outcome_phase}) do
    with :ok <- ensure_not_terminal(state),
         :ok <- ensure_phase_matches(state, outcome_phase) do
      do_advance(state)
    end
  end

  @spec ensure_not_terminal(t()) :: :ok | {:error, :terminal_state}
  defp ensure_not_terminal(%__MODULE__{workflow: wf, current: current}) do
    if Workflow.terminal?(wf, current), do: {:error, :terminal_state}, else: :ok
  end

  @spec ensure_phase_matches(t(), Phase.t()) :: :ok | {:error, :phase_mismatch}
  defp ensure_phase_matches(%__MODULE__{current: current}, outcome_phase) do
    if outcome_phase == current, do: :ok, else: {:error, :phase_mismatch}
  end

  @spec do_advance(t()) :: {:ok, t()} | {:error, :workflow_has_no_successor}
  defp do_advance(%__MODULE__{workflow: wf, current: current} = state) do
    case Workflow.advance_from(wf, current) do
      nil ->
        {:error, :workflow_has_no_successor}

      next_phase ->
        {:ok,
         %{
           state
           | current: next_phase,
             completed_outcomes: state.completed_outcomes ++ [%{outcome: :recorded}]
         }}
    end
  end

  @doc "Is this state at a terminal phase?"
  @spec terminal?(t()) :: boolean()
  def terminal?(%__MODULE__{workflow: workflow, current: current}) do
    Workflow.terminal?(workflow, current)
  end
end
