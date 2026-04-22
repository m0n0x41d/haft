defmodule OpenSleigh.Gates.Semantic do
  @moduledoc """
  Behaviour contract for semantic gates (L2).

  Per `specs/target-system/PHASE_ONTOLOGY.md §Semantic` +
  `AGENT_PROTOCOL.md` + `ILLEGAL_STATES.md` GK + §6b.1 (LLM-judge
  calibration):

  * Non-deterministic. The actual LLM-judge call lives at L4 in
    `OpenSleigh.JudgeClient` — this module defines only the contract
    (prompt, response shape, description).
  * Returns `SemanticGateResult` = `%{verdict, cl, rationale}` on
    pass-shape, or `{:error, atom()}` on judge failure.
  * Every semantic gate MUST have a calibrated GoldenSet (§6b.1 /
    GK5). L4 `JudgeClient.evaluate/2` refuses to evaluate
    uncalibrated gates with `{:error, :uncalibrated}`.

  An implementing module provides:

  * The prompt template used to ask the judge.
  * A parser that maps the judge's raw JSON response into
    `SemanticGateResult`.
  """

  alias OpenSleigh.{GateContext, GateResult, Verdict}

  @typedoc "Semantic-gate result payload."
  @type result :: %{
          verdict: Verdict.t(),
          cl: 0..3,
          rationale: String.t()
        }

  @doc "The canonical atom name of this gate."
  @callback gate_name() :: atom()

  @doc """
  Render the prompt string that will be sent to the judge LLM. Takes
  the `GateContext` and returns a complete prompt (no further
  rendering needed — L4 just ships the string).
  """
  @callback render_prompt(GateContext.t()) :: String.t()

  @doc """
  Parse the judge's raw JSON response into a `SemanticGateResult`. On
  malformed response returns `{:error, :judge_response_malformed}`.
  """
  @callback parse_response(map()) :: {:ok, result()} | {:error, atom()}

  @doc "Short human-readable description (used in docs + golden sets)."
  @callback description() :: String.t()

  @doc """
  Helper: wrap a semantic gate result into a `GateResult` tuple ready
  for the gate chain.
  """
  @spec to_gate_result({:ok, result()} | {:error, atom()}) :: GateResult.t()
  def to_gate_result({:ok, %{verdict: _, cl: _, rationale: _} = payload}),
    do: {:semantic, payload}

  def to_gate_result({:error, reason}) when is_atom(reason),
    do: {:semantic, {:error, reason}}
end
