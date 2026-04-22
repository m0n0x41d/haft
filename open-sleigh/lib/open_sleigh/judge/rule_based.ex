defmodule OpenSleigh.Judge.RuleBased do
  @moduledoc """
  Deterministic judge invoker for golden-set reports and mock runs.

  This is not the live semantic judge. The live path uses
  `OpenSleigh.Judge.AgentInvoker` through the configured agent provider.
  This module gives the local calibration task a stable baseline and keeps
  mock `open_sleigh.start --once` deterministic when semantic gates are
  enabled in the example config.
  """

  alias OpenSleigh.Judge.GoldenSets

  @type response :: %{
          required(String.t()) => String.t() | integer()
        }

  @doc "Evaluate a rendered semantic-gate prompt with deterministic rules."
  @spec invoke(String.t()) :: {:ok, response()}
  def invoke(prompt) when is_binary(prompt) do
    prompt
    |> response_fun()
    |> apply_response(prompt)
    |> then(&{:ok, &1})
  end

  @spec calibration() :: OpenSleigh.JudgeClient.calibration()
  def calibration, do: GoldenSets.calibration()

  @spec response_fun(String.t()) :: (String.t() -> response())
  defp response_fun(prompt) do
    prompt
    |> response_rules()
    |> Enum.find(&rule_matches?/1)
    |> response_rule_fun()
  end

  @spec response_rules(String.t()) :: [{boolean(), (String.t() -> response())}]
  defp response_rules(prompt) do
    [
      {String.contains?(prompt, "`describedEntity`"), &object_of_talk_response/1},
      {String.contains?(prompt, "Boundary Norm Square"), &lade_response/1},
      {String.contains?(prompt, "No self-evidence"), &no_self_evidence_response/1}
    ]
  end

  @spec rule_matches?({boolean(), function()}) :: boolean()
  defp rule_matches?({matches?, _fun}), do: matches?

  @spec response_rule_fun({boolean(), (String.t() -> response())} | nil) ::
          (String.t() -> response())
  defp response_rule_fun({_matches?, fun}), do: fun
  defp response_rule_fun(nil), do: &unknown_gate_response/1

  @spec apply_response((String.t() -> response()), String.t()) :: response()
  defp apply_response(fun, prompt), do: fun.(prompt)

  @spec object_of_talk_response(String.t()) :: response()
  defp object_of_talk_response(prompt) do
    prompt
    |> section_after("The described entity is:")
    |> fenced_value()
    |> object_verdict()
  end

  @spec object_verdict(String.t()) :: response()
  defp object_verdict(value) do
    value
    |> specific_object?()
    |> object_response(value)
  end

  @spec object_response(boolean(), String.t()) :: response()
  defp object_response(true, value) do
    pass(3, "describedEntity names a concrete artifact: #{String.trim(value)}")
  end

  defp object_response(false, _value) do
    fail(1, "describedEntity is too vague to ground the object of talk")
  end

  @spec specific_object?(String.t()) :: boolean()
  defp specific_object?(value) do
    normalized = normalize(value)

    [
      fn -> normalized not in vague_objects() end,
      fn -> String.length(normalized) >= 4 end,
      fn -> artifact_shaped?(value) end
    ]
    |> Enum.all?(& &1.())
  end

  @spec artifact_shaped?(String.t()) :: boolean()
  defp artifact_shaped?(value) do
    Enum.any?([
      String.contains?(value, "/"),
      String.contains?(value, "."),
      String.contains?(value, "::"),
      Regex.match?(~r/[A-Z][A-Za-z0-9_]+\.[A-Z][A-Za-z0-9_]+/, value)
    ])
  end

  @spec vague_objects() :: [String.t()]
  defp vague_objects do
    ["the system", "system", "the code", "code", "it", "the app", "app", "service", "everything"]
  end

  @spec lade_response(String.t()) :: response()
  defp lade_response(prompt) do
    prompt
    |> section_after("The artifact text is:")
    |> fenced_value()
    |> lade_verdict()
  end

  @spec lade_verdict(String.t()) :: response()
  defp lade_verdict(text) do
    text
    |> lade_pass?()
    |> lade_response_for(text)
  end

  @spec lade_response_for(boolean(), String.t()) :: response()
  defp lade_response_for(true, _text) do
    pass(3, "obligation language is absent or split across LADE quadrants")
  end

  defp lade_response_for(false, _text) do
    fail(1, "obligation language conflates duty, acceptance, and evidence")
  end

  @spec lade_pass?(String.t()) :: boolean()
  defp lade_pass?(text) do
    normalized = normalize(text)

    Enum.any?([
      not obligation_language?(normalized),
      explicit_lade_split?(normalized)
    ])
  end

  @spec obligation_language?(String.t()) :: boolean()
  defp obligation_language?(text) do
    ["must", "guarantee", "guarantees", "accepted", "evidence", "shall", "required"]
    |> Enum.any?(&String.contains?(text, &1))
  end

  @spec explicit_lade_split?(String.t()) :: boolean()
  defp explicit_lade_split?(text) do
    [
      fn -> String.contains?(text, "law") or String.contains?(text, "l -") end,
      fn -> String.contains?(text, "admissibility") or String.contains?(text, "a -") end,
      fn -> String.contains?(text, "deontics") or String.contains?(text, "d -") end,
      fn -> String.contains?(text, "work-effect") or String.contains?(text, "w-e") end
    ]
    |> Enum.all?(& &1.())
  end

  @spec no_self_evidence_response(String.t()) :: response()
  defp no_self_evidence_response(prompt) do
    prompt
    |> self_evidence_pass?()
    |> no_self_evidence_response_for()
  end

  @spec no_self_evidence_response_for(boolean()) :: response()
  defp no_self_evidence_response_for(true) do
    pass(3, "evidence is external and remains distinct from the work-effect")
  end

  defp no_self_evidence_response_for(false) do
    fail(1, "evidence is self-authored or conflates carrier with work-effect")
  end

  @spec self_evidence_pass?(String.t()) :: boolean()
  defp self_evidence_pass?(prompt) do
    prompt
    |> normalize()
    |> self_evidence_fail?()
    |> Kernel.not()
  end

  @spec self_evidence_fail?(String.t()) :: boolean()
  defp self_evidence_fail?(text) do
    Enum.any?([
      String.contains?(text, "authoring_source=:agent"),
      String.contains?(text, "authoring_source=:executor"),
      String.contains?(text, "sha proves the runtime effect"),
      String.contains?(text, "carrier is the work-effect")
    ])
  end

  @spec unknown_gate_response(String.t()) :: response()
  defp unknown_gate_response(_prompt), do: fail(0, "semantic gate prompt is not recognized")

  @spec section_after(String.t(), String.t()) :: String.t()
  defp section_after(text, marker) do
    text
    |> String.split(marker, parts: 2)
    |> after_split()
  end

  @spec after_split([String.t()]) :: String.t()
  defp after_split([_before, after_marker]), do: after_marker
  defp after_split([single]), do: single

  @spec fenced_value(String.t()) :: String.t()
  defp fenced_value(text) do
    text
    |> String.split("---", parts: 3)
    |> fenced_split()
    |> String.trim()
  end

  @spec fenced_split([String.t()]) :: String.t()
  defp fenced_split([_before, value, _after]), do: value
  defp fenced_split([single]), do: single

  @spec pass(0..3, String.t()) :: response()
  defp pass(cl, rationale), do: response("pass", cl, rationale)

  @spec fail(0..3, String.t()) :: response()
  defp fail(cl, rationale), do: response("fail", cl, rationale)

  @spec response(String.t(), 0..3, String.t()) :: response()
  defp response(verdict, cl, rationale) do
    %{"verdict" => verdict, "cl" => cl, "rationale" => rationale}
  end

  @spec normalize(String.t()) :: String.t()
  defp normalize(text) do
    text
    |> String.trim()
    |> String.downcase()
  end
end
