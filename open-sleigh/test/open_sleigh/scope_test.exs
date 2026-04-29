defmodule OpenSleigh.ScopeTest do
  use ExUnit.Case, async: true

  alias OpenSleigh.Scope

  defp scope_attrs_without_hash do
    %{
      repo_ref: "github:m0n0x41d/haft",
      base_sha: "abc123",
      target_branch: "feature/commission-first",
      allowed_paths: ["open-sleigh/lib/open_sleigh/work_commission.ex"],
      forbidden_paths: ["open-sleigh/config/prod.secret.exs"],
      allowed_actions: MapSet.new([:edit_files, :run_tests]),
      affected_files: ["open-sleigh/lib/open_sleigh/work_commission.ex"],
      allowed_modules: ["OpenSleigh.WorkCommission"],
      lockset: ["open-sleigh/lib/open_sleigh/work_commission.ex"]
    }
  end

  defp scope_attrs do
    attrs = scope_attrs_without_hash()
    {:ok, hash} = Scope.canonical_hash(attrs)

    Map.put(attrs, :hash, hash)
  end

  test "new/1 builds a Scope when the supplied hash is canonical" do
    assert {:ok, %Scope{} = scope} = Scope.new(scope_attrs())

    assert scope.repo_ref == "github:m0n0x41d/haft"
    assert scope.allowed_actions == MapSet.new([:edit_files, :run_tests])
    assert Scope.valid_hash?(scope.hash)
  end

  test "canonical hash ignores collection ordering" do
    reordered =
      scope_attrs_without_hash()
      |> Map.put(:allowed_paths, [
        "open-sleigh/test/open_sleigh/work_commission_test.exs",
        "open-sleigh/lib/open_sleigh/work_commission.ex"
      ])
      |> Map.put(:affected_files, [
        "open-sleigh/test/open_sleigh/work_commission_test.exs",
        "open-sleigh/lib/open_sleigh/work_commission.ex"
      ])
      |> Map.put(:lockset, [
        "open-sleigh/test/open_sleigh/work_commission_test.exs",
        "open-sleigh/lib/open_sleigh/work_commission.ex"
      ])
      |> Map.put(:allowed_actions, MapSet.new([:run_tests, :edit_files]))

    reversed =
      reordered
      |> Map.update!(:allowed_paths, &Enum.reverse/1)
      |> Map.update!(:affected_files, &Enum.reverse/1)
      |> Map.update!(:lockset, &Enum.reverse/1)

    assert Scope.canonical_hash(reordered) == Scope.canonical_hash(reversed)
  end

  test "new/1 rejects a non-canonical hash" do
    attrs = Map.put(scope_attrs(), :hash, String.duplicate("0", 64))

    assert {:error, :scope_hash_mismatch} = Scope.new(attrs)
  end

  test "new/1 rejects missing required collection fields" do
    attrs = Map.delete(scope_attrs(), :allowed_paths)

    assert {:error, :invalid_allowed_paths} = Scope.new(attrs)
  end

  test "new/1 rejects empty action authority" do
    attrs =
      scope_attrs_without_hash()
      |> Map.put(:allowed_actions, MapSet.new())

    assert {:error, :invalid_allowed_actions} = Scope.canonical_hash(attrs)
  end
end
