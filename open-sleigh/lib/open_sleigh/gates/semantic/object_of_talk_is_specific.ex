defmodule OpenSleigh.Gates.Semantic.ObjectOfTalkIsSpecific do
  @moduledoc """
  Semantic gate — Frame exit. Per `specs/target-system/GATES.md §2`:

  > Is `describedEntity` specific (file path, module, subsystem) or
  > vacuous ("the system", "the code")?

  Catches the single most common framing anti-pattern: a
  ProblemCard whose `describedEntity` is an umbrella word that
  applies to anything. Per FPF A.7 Strict Distinction — every object
  of talk must have grounding.

  This module is the **contract**. Actual judge invocation lives at
  L4 `OpenSleigh.JudgeClient` (MVP-1 stub; Codex with a dedicated
  judge prompt). L2 supplies the prompt and response parser.
  """

  @behaviour OpenSleigh.Gates.Semantic

  alias OpenSleigh.GateContext

  @gate_name :object_of_talk_is_specific

  @impl true
  @spec gate_name() :: atom()
  def gate_name, do: @gate_name

  @impl true
  @spec render_prompt(GateContext.t()) :: String.t()
  def render_prompt(%GateContext{upstream_problem_card: pc}) when is_map(pc) do
    described =
      Map.get(pc, "describedEntity") || Map.get(pc, :describedEntity) || ""

    """
    You are evaluating whether a ProblemCard's `describedEntity` is
    specific enough to serve as an object of talk for FPF-compliant
    engineering work.

    PASS criterion: `describedEntity` names a concrete artifact —
    a file path, a module, a subsystem, an API endpoint, a named
    component, a specific data structure.

    FAIL criterion: `describedEntity` is an umbrella word or vague
    reference — "the system", "the code", "it", "the app", "the
    service", "everything", or similarly un-grounded.

    The described entity is:

    ---
    #{described}
    ---

    Respond ONLY with a single JSON object with these exact keys:

      {"verdict": "pass" | "fail", "cl": 0-3, "rationale": "<one sentence>"}

    Where `cl` is your congruence level (0 = opposed, 3 = exact
    match to known pattern). Respond with `"verdict": "fail"` when
    unsure.
    """
  end

  def render_prompt(%GateContext{}),
    do: "Error: no upstream ProblemCard available to judge."

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
    do: "Upstream ProblemCard's describedEntity is a concrete artifact, not an umbrella word."
end
