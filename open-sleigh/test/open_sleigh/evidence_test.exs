defmodule OpenSleigh.EvidenceTest do
  use ExUnit.Case, async: true
  use ExUnitProperties

  alias OpenSleigh.Evidence

  @valid_ts ~U[2026-04-22 10:00:00Z]

  describe "new/6 happy path" do
    test "builds an Evidence with required fields" do
      assert {:ok, %Evidence{} = e} =
               Evidence.new(:pr_merge_sha, "abc123", nil, 3, :git_host, @valid_ts)

      assert e.kind == :pr_merge_sha
      assert e.ref == "abc123"
      assert e.cl == 3
      assert e.authoring_source == :git_host
    end

    test "accepts optional hash" do
      assert {:ok, e} =
               Evidence.new(:pr_merge_sha, "abc", "deadbeef", 2, :git_host, @valid_ts)

      assert e.hash == "deadbeef"
    end
  end

  describe "PR6 — cl bounds" do
    test "rejects cl < 0" do
      assert {:error, :invalid_cl} =
               Evidence.new(:pr_merge_sha, "abc", nil, -1, :git_host, @valid_ts)
    end

    test "rejects cl > 3" do
      assert {:error, :invalid_cl} =
               Evidence.new(:pr_merge_sha, "abc", nil, 4, :git_host, @valid_ts)
    end

    test "rejects non-integer cl" do
      assert {:error, :invalid_cl} =
               Evidence.new(:pr_merge_sha, "abc", nil, 2.5, :git_host, @valid_ts)
    end

    property "accepts every cl in 0..3" do
      check all(cl <- StreamData.integer(0..3)) do
        assert {:ok, _} = Evidence.new(:test, "ref", nil, cl, :ci, @valid_ts)
      end
    end
  end

  describe "OB4 — :open_sleigh_self blacklist" do
    test "rejects authoring_source = :open_sleigh_self (OB4 reserved)" do
      assert {:error, :forbidden_authoring_source} =
               Evidence.new(:pr_merge_sha, "abc", nil, 3, :open_sleigh_self, @valid_ts)
    end

    test "rejects non-atom authoring_source" do
      assert {:error, :forbidden_authoring_source} =
               Evidence.new(:pr_merge_sha, "abc", nil, 3, "git_host", @valid_ts)
    end
  end

  describe "ref / captured_at shape" do
    test "rejects empty ref" do
      assert {:error, :empty_ref} =
               Evidence.new(:pr_merge_sha, "", nil, 3, :git_host, @valid_ts)
    end

    test "rejects non-DateTime captured_at" do
      assert {:error, :invalid_captured_at} =
               Evidence.new(:pr_merge_sha, "abc", nil, 3, :git_host, "yesterday")
    end
  end

  describe "PR5 is NOT enforced here (lives at PhaseOutcome.new/2)" do
    test "Evidence.new/6 does NOT check ref vs some authoring artifact's self_id" do
      # This test documents the v0.5 CRITICAL-2 fix: self-reference
      # check is at the layer that knows self_id, not at Evidence.new.
      # So Evidence can legitimately be built with any non-empty ref.
      assert {:ok, %Evidence{}} =
               Evidence.new(:test, "any-self-id-shaped-string", nil, 0, :ci, @valid_ts)
    end
  end
end
