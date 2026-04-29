defmodule OpenSleigh.JudgeClient do
  @moduledoc """
  L4 — wraps an `Agent.Adapter` for semantic-gate evaluation.

  Per `specs/target-system/GATES.md §2` + `AGENT_PROTOCOL.md §10`:

  * Semantic gates are effectful. At L2 the gate module supplies the
    prompt template + response parser. The actual LLM invocation
    happens here, via an `Agent.Adapter` (typically Codex) spawned
    with a scoped judge-specific session.
  * Before evaluation, `JudgeClient` checks that the gate has a
    `JudgeCalibration` (golden-set) — absence returns
    `{:error, :uncalibrated}` (GK5).
  * Returns a `SemanticGateResult` or typed error; callers wrap
    into a `GateResult` via `Gates.Semantic.to_gate_result/1`.

  This module is a **contract adaptor** — it takes a judge-invoker
  function so we don't have to own a real LLM connection in L4
  tests. L5 supplies a concrete invoker backed by the adapter.
  """

  alias OpenSleigh.{EffectError, GateContext, Gates}

  @typedoc """
  Judge invoker — takes a rendered prompt, returns the parsed JSON
  response (map). Concrete impl lives at L5 (wraps the agent
  adapter with a judge prompt + structured-JSON response mode).
  """
  @type invoke_fun ::
          (String.t() -> {:ok, map()} | {:error, EffectError.t()})

  @typedoc """
  Optional calibration table: `%{gate_atom => true}`. Absence of a
  gate atom means the gate is uncalibrated per GK5.
  """
  @type calibration :: %{optional(atom()) => boolean()}

  @doc """
  Evaluate a semantic gate via the injected judge invoker. `gate_module`
  is the L2 `Gates.Semantic` impl (supplies prompt + parser).

  Conforms to `OpenSleigh.GateChain.judge_fun` shape so you can
  partial-apply the invoker and calibration and get a drop-in
  judge_fun for `GateChain.evaluate/3`.
  """
  @spec evaluate(module(), GateContext.t(), invoke_fun(), calibration()) ::
          {:ok, Gates.Semantic.result()} | {:error, EffectError.t()}
  def evaluate(gate_module, %GateContext{} = ctx, invoke_fun, calibration)
      when is_atom(gate_module) and is_function(invoke_fun, 1) and is_map(calibration) do
    with :ok <- check_calibrated(gate_module, calibration),
         prompt <- gate_module.render_prompt(ctx),
         {:ok, response} <- invoke_fun.(prompt),
         {:ok, result} <- parse_with_gate(gate_module, response) do
      {:ok, result}
    end
  end

  @doc """
  Build a `GateChain.judge_fun` from an invoker + calibration map.
  Pass the result directly to `GateChain.evaluate/3`.
  """
  @spec judge_fun(invoke_fun(), calibration()) ::
          (module(), GateContext.t() ->
             {:ok, Gates.Semantic.result()} | {:error, EffectError.t()})
  def judge_fun(invoke_fun, calibration) do
    fn module, ctx -> evaluate(module, ctx, invoke_fun, calibration) end
  end

  @spec check_calibrated(module(), calibration()) ::
          :ok | {:error, :uncalibrated}
  defp check_calibrated(gate_module, calibration) do
    case Map.get(calibration, gate_module.gate_name()) do
      true -> :ok
      _ -> {:error, :uncalibrated}
    end
  end

  @spec parse_with_gate(module(), map()) ::
          {:ok, Gates.Semantic.result()} | {:error, :judge_response_malformed}
  defp parse_with_gate(gate_module, response) do
    gate_module.parse_response(response)
  end
end
