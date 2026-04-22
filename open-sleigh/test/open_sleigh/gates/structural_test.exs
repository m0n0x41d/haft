defmodule OpenSleigh.Gates.StructuralTest do
  use ExUnit.Case, async: true

  alias OpenSleigh.Fixtures

  alias OpenSleigh.Gates.Structural.{
    DescribedEntityFieldPresent,
    DesignRuntimeSplitOk,
    EvidenceRefNotSelf,
    ProblemCardRefPresent,
    ValidUntilFieldPresent
  }

  alias OpenSleigh.{SessionScopedArtifactId}

  describe "ProblemCardRefPresent (UP1)" do
    test "passes when upstream_problem_card is present and externally authored" do
      ctx = Fixtures.gate_context()
      assert :ok = ProblemCardRefPresent.apply(ctx)
    end

    test "fails when upstream_problem_card is nil" do
      ctx = Fixtures.gate_context(%{upstream_problem_card: nil})
      assert {:error, :no_upstream_frame} = ProblemCardRefPresent.apply(ctx)
    end

    test "fails when upstream_problem_card is self-authored (UP3)" do
      ctx =
        Fixtures.gate_context(%{
          upstream_problem_card: %{authoring_source: :open_sleigh_self}
        })

      assert {:error, :upstream_self_authored} = ProblemCardRefPresent.apply(ctx)
    end

    test "gate_name is canonical atom" do
      assert ProblemCardRefPresent.gate_name() == :problem_card_ref_present
    end
  end

  describe "DescribedEntityFieldPresent" do
    test "passes with both fields non-empty" do
      ctx = Fixtures.gate_context()
      assert :ok = DescribedEntityFieldPresent.apply(ctx)
    end

    test "fails on missing describedEntity" do
      ctx =
        Fixtures.gate_context(%{
          upstream_problem_card: %{"groundingHolon" => "MyApp.Auth"}
        })

      assert {:error, :missing_described_entity} = DescribedEntityFieldPresent.apply(ctx)
    end

    test "fails on empty describedEntity" do
      ctx =
        Fixtures.gate_context(%{
          upstream_problem_card: %{"describedEntity" => "", "groundingHolon" => "x"}
        })

      assert {:error, :missing_described_entity} = DescribedEntityFieldPresent.apply(ctx)
    end

    test "fails on missing groundingHolon" do
      ctx =
        Fixtures.gate_context(%{
          upstream_problem_card: %{"describedEntity" => "lib/x.ex"}
        })

      assert {:error, :missing_grounding_holon} = DescribedEntityFieldPresent.apply(ctx)
    end

    test "fails when upstream_problem_card is nil" do
      ctx = Fixtures.gate_context(%{upstream_problem_card: nil})
      assert {:error, :no_upstream_problem_card} = DescribedEntityFieldPresent.apply(ctx)
    end
  end

  describe "ValidUntilFieldPresent" do
    test "passes when proposed_valid_until is set and in the future" do
      future = DateTime.add(DateTime.utc_now(), 86_400, :second)
      ctx = Fixtures.gate_context(%{proposed_valid_until: future})
      assert :ok = ValidUntilFieldPresent.apply(ctx)
    end

    test "fails when proposed_valid_until is nil" do
      ctx = Fixtures.gate_context(%{proposed_valid_until: nil})
      assert {:error, :missing_valid_until} = ValidUntilFieldPresent.apply(ctx)
    end
  end

  describe "DesignRuntimeSplitOk (MVP-1 stub)" do
    test "returns :ok unconditionally (stub)" do
      assert :ok = DesignRuntimeSplitOk.apply(Fixtures.gate_context())
    end
  end

  describe "EvidenceRefNotSelf (PR5 defensive gate)" do
    test "passes when no evidence.ref matches self_id" do
      ctx = Fixtures.gate_context()
      assert :ok = EvidenceRefNotSelf.apply(ctx)
    end

    test "fails when an evidence.ref == self_id" do
      sid = Fixtures.self_id()
      sid_str = SessionScopedArtifactId.to_string(sid)

      self_refing = Fixtures.evidence(%{ref: sid_str})
      ctx = Fixtures.gate_context(%{self_id: sid, evidence: [self_refing]})

      assert {:error, :evidence_self_reference} = EvidenceRefNotSelf.apply(ctx)
    end

    test "fails when any evidence.ref is empty" do
      empty_ref_evidence = %{Fixtures.evidence() | ref: ""}
      ctx = Fixtures.gate_context(%{evidence: [empty_ref_evidence]})
      assert {:error, :evidence_empty_ref} = EvidenceRefNotSelf.apply(ctx)
    end
  end
end
