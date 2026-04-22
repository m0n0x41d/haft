defmodule OpenSleigh.Gates.Semantic.LadeQuadrantsSplitOk do
  @moduledoc """
  Semantic gate — any obligation-language artifact (typically Execute
  or Commission output). Per `specs/target-system/GATES.md §2` +
  `.context/semiotics_slideument.md` §A.6.B Slide 33:

  Obligation-language sentences (containing "MUST", "guarantees",
  "accepted", "evidence", etc.) must be decomposed into the four
  Boundary Norm Square quadrants:

  * **L** — Law (definition)
  * **A** — Admissibility (gate)
  * **D** — Deontics (duty)
  * **W-E** — Work-effect / Evidence (the 4th axis literally named
    "Work-effect / Evidence" per Slide 33, NOT just "Evidence" —
    conflating work-effect with evidence is itself a reportable error)

  Geometrically `L D / E A` (Slide 33 literal).
  """

  @behaviour OpenSleigh.Gates.Semantic

  alias OpenSleigh.GateContext

  @gate_name :lade_quadrants_split_ok

  @impl true
  @spec gate_name() :: atom()
  def gate_name, do: @gate_name

  @impl true
  @spec render_prompt(GateContext.t()) :: String.t()
  def render_prompt(%GateContext{turn_result: turn_result}) do
    text = Map.get(turn_result, :text) || Map.get(turn_result, "text") || ""

    """
    You are evaluating whether obligation-language in an engineering
    artifact is cleanly decomposed into the four quadrants of the
    Boundary Norm Square (FPF A.6.B, `semiotics_slideument.md`
    Slide 33):

      L — Law (definition)
      A — Admissibility (gate)
      D — Deontics (duty)
      W-E — Work-effect / Evidence (the 4th axis is EXPLICITLY
            "Work-effect / Evidence" — conflating work-effect with
            evidence is itself a reportable error)

    PASS criterion: sentences that use "MUST" / "guarantees" /
    "accepted" / "evidence" / "shall" / "required" are clearly
    attributable to one of the four quadrants, OR the text
    acknowledges the split explicitly.

    FAIL criterion: obligation-language is mixed — a single
    sentence conflates two or more quadrants (e.g., "the system
    MUST guarantee X, evidence of Y accepted" mixes L + D + A + W-E).

    The artifact text is:

    ---
    #{text}
    ---

    Respond ONLY with a single JSON object:

      {"verdict": "pass" | "fail", "cl": 0-3, "rationale": "<one sentence naming which quadrants are conflated if any>"}
    """
  end

  @impl true
  @spec parse_response(map()) ::
          {:ok, OpenSleigh.Gates.Semantic.result()} | {:error, :judge_response_malformed}
  def parse_response(%{"verdict" => verdict, "cl" => cl, "rationale" => rationale})
      when verdict in ["pass", "fail"] and is_integer(cl) and cl in 0..3 and is_binary(rationale) do
    {:ok,
     %{
       verdict: String.to_existing_atom(verdict),
       cl: cl,
       rationale: rationale
     }}
  end

  def parse_response(_), do: {:error, :judge_response_malformed}

  @impl true
  @spec description() :: String.t()
  def description,
    do:
      "Obligation-language is decomposed into L / A / D / W-E quadrants (Boundary Norm Square, A.6.B)."
end
