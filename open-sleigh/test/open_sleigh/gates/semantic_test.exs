defmodule OpenSleigh.Gates.SemanticTest do
  use ExUnit.Case, async: true

  alias OpenSleigh.Fixtures
  alias OpenSleigh.Gates.Semantic

  alias OpenSleigh.Gates.Semantic.{
    LadeQuadrantsSplitOk,
    NoSelfEvidenceSemantic,
    ObjectOfTalkIsSpecific
  }

  describe "ObjectOfTalkIsSpecific contract" do
    test "gate_name" do
      assert ObjectOfTalkIsSpecific.gate_name() == :object_of_talk_is_specific
    end

    test "render_prompt includes the describedEntity" do
      ctx =
        Fixtures.gate_context(%{
          upstream_problem_card: %{"describedEntity" => "lib/my_app/auth.ex"}
        })

      prompt = ObjectOfTalkIsSpecific.render_prompt(ctx)
      assert prompt =~ "lib/my_app/auth.ex"
      assert prompt =~ "PASS criterion"
      assert prompt =~ "FAIL criterion"
    end

    test "render_prompt handles missing upstream_problem_card" do
      ctx = Fixtures.gate_context(%{upstream_problem_card: nil})
      prompt = ObjectOfTalkIsSpecific.render_prompt(ctx)
      assert prompt =~ "Error"
    end

    test "parse_response accepts well-shaped judge output" do
      response = %{"verdict" => "pass", "cl" => 3, "rationale" => "concrete file path"}

      assert {:ok, %{verdict: :pass, cl: 3, rationale: "concrete file path"}} =
               ObjectOfTalkIsSpecific.parse_response(response)
    end

    test "parse_response rejects malformed judge output" do
      assert {:error, :judge_response_malformed} =
               ObjectOfTalkIsSpecific.parse_response(%{"verdict" => "maybe"})

      assert {:error, :judge_response_malformed} =
               ObjectOfTalkIsSpecific.parse_response(%{})

      assert {:error, :judge_response_malformed} =
               ObjectOfTalkIsSpecific.parse_response(%{
                 "verdict" => "pass",
                 "cl" => 99,
                 "rationale" => "x"
               })
    end
  end

  describe "LadeQuadrantsSplitOk contract (v0.3 4th quadrant = Work-effect/Evidence)" do
    test "prompt mentions all four quadrants with the correct 4th name" do
      ctx = Fixtures.gate_context(%{turn_result: %{text: "System MUST guarantee X"}})
      prompt = LadeQuadrantsSplitOk.render_prompt(ctx)
      assert prompt =~ "Law"
      assert prompt =~ "Admissibility"
      assert prompt =~ "Deontics"
      assert prompt =~ "Work-effect"
    end

    test "parse_response with valid verdict/cl/rationale" do
      response = %{
        "verdict" => "fail",
        "cl" => 2,
        "rationale" => "mixes L and D"
      }

      assert {:ok, %{verdict: :fail, cl: 2}} = LadeQuadrantsSplitOk.parse_response(response)
    end
  end

  describe "NoSelfEvidenceSemantic contract" do
    test "prompt covers both sub-checks (a) external author and (b) carrier vs work-effect" do
      ctx = Fixtures.gate_context()
      prompt = NoSelfEvidenceSemantic.render_prompt(ctx)
      assert prompt =~ "(a)"
      assert prompt =~ "(b)"
      assert prompt =~ "carrier"
      assert prompt =~ "work-effect"
    end

    test "parse_response happy path" do
      response = %{"verdict" => "pass", "cl" => 3, "rationale" => "CI is external"}

      assert {:ok, %{verdict: :pass, cl: 3}} = NoSelfEvidenceSemantic.parse_response(response)
    end
  end

  describe "Semantic.to_gate_result/1 helper" do
    test "wraps {:ok, payload} as {:semantic, payload}" do
      payload = %{verdict: :pass, cl: 3, rationale: "x"}
      assert {:semantic, ^payload} = Semantic.to_gate_result({:ok, payload})
    end

    test "wraps {:error, reason} as {:semantic, {:error, reason}}" do
      assert {:semantic, {:error, :judge_unavailable}} =
               Semantic.to_gate_result({:error, :judge_unavailable})
    end
  end
end
