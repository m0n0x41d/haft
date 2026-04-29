defmodule OpenSleigh.Gates.Semantic.NoSelfEvidenceSemantic do
  @moduledoc """
  Semantic gate — Measure exit. Two-part semantic check (per
  `specs/target-system/GATES.md §2`, v0.3 refinement):

  **(a)** The cited evidence is produced by a role **external to the
  authoring role** (FPF-Spec A.10 CC-A10.6: no self-evidence).

  **(b)** The **evidence carrier** (PR sha, CI run id, test log) is
  distinguished from the **work-effect** (what the merged code
  actually does in runtime). A carrier is not a work-effect;
  conflating them is the trap this gate exists to catch (per v0.3
  LADE work-effect/evidence distinction).

  The structural gate `evidence_ref_not_self` handles the shallow
  string-equality check (PR5). This semantic gate catches the deeper
  "same author / conflated role" version that requires judgement.
  """

  @behaviour OpenSleigh.Gates.Semantic

  alias OpenSleigh.GateContext

  @gate_name :no_self_evidence_semantic

  @impl true
  @spec gate_name() :: atom()
  def gate_name, do: @gate_name

  @impl true
  @spec render_prompt(GateContext.t()) :: String.t()
  def render_prompt(%GateContext{evidence: evidence, turn_result: turn_result}) do
    evidence_summary =
      evidence
      |> Enum.map(fn e ->
        "- kind=#{e.kind} ref=#{inspect(e.ref)} authoring_source=#{e.authoring_source} cl=#{e.cl}"
      end)
      |> Enum.join("\n")

    claim = Map.get(turn_result, :claim) || Map.get(turn_result, "claim") || "(no claim text)"

    """
    You are evaluating whether a Measure-phase outcome's cited
    evidence actually supports its claim, under two FPF rules:

    (a) No self-evidence (FPF A.10 CC-A10.6) — evidence produced by
        the same role that authored the claim does NOT count. Look
        at each evidence's `authoring_source`. If it matches the
        claim's authoring role (e.g. both are the agent/executor
        with no external reviewer/CI carrier), that is self-
        evidence and fails.

    (b) Evidence carrier ≠ work-effect (v0.3 LADE distinction) —
        a PR merge sha is a CARRIER. A CI run id is a CARRIER. A
        test-count claim is a CARRIER. None of these are the
        WORK-EFFECT itself (what the merged code actually does in
        production). If the outcome conflates the carrier with
        the work-effect as if they were the same thing, that
        fails.

    Claim:
    ---
    #{claim}
    ---

    Evidence:
    ---
    #{evidence_summary}
    ---

    Respond ONLY with:

      {"verdict": "pass" | "fail", "cl": 0-3, "rationale": "<one sentence; if fail, name which rule (a) or (b)>"}
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
      "Evidence is externally authored AND carrier-vs-work-effect distinction is respected (FPF A.10 CC-A10.6 + v0.3 LADE work-effect)."
end
