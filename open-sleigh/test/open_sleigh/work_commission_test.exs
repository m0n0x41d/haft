defmodule OpenSleigh.WorkCommissionTest do
  use ExUnit.Case, async: true

  alias OpenSleigh.{CommissionRevisionSnapshot, Scope, WorkCommission}

  @valid_ts ~U[2026-04-22 10:00:00Z]
  @valid_until ~U[2026-05-22 10:00:00Z]

  defp scope_attrs_without_hash do
    %{
      repo_ref: "github:m0n0x41d/haft",
      base_sha: "abc123",
      target_branch: "feature/commission-first",
      allowed_paths: [
        "open-sleigh/lib/open_sleigh/work_commission.ex",
        "open-sleigh/lib/open_sleigh/scope.ex"
      ],
      forbidden_paths: [],
      allowed_actions: MapSet.new([:edit_files, :run_tests]),
      affected_files: [
        "open-sleigh/lib/open_sleigh/work_commission.ex",
        "open-sleigh/lib/open_sleigh/scope.ex"
      ],
      lockset: [
        "open-sleigh/lib/open_sleigh/work_commission.ex",
        "open-sleigh/lib/open_sleigh/scope.ex"
      ]
    }
  end

  defp scope do
    attrs = scope_attrs_without_hash()
    {:ok, hash} = Scope.canonical_hash(attrs)
    attrs = Map.put(attrs, :hash, hash)

    {:ok, scope} = Scope.new(attrs)
    scope
  end

  defp work_commission_attrs do
    scope = scope()

    %{
      id: "wc-123",
      decision_ref: "dec-20260422-001",
      decision_revision_hash: "decision-r1",
      problem_card_ref: "pc-20260422-001",
      implementation_plan_ref: "plan-20260422-001",
      implementation_plan_revision: "plan-r1",
      scope: scope,
      scope_hash: scope.hash,
      base_sha: scope.base_sha,
      lockset: scope.lockset,
      evidence_requirements: [%{kind: :mix_test, command: "mix test"}],
      projection_policy: :local_only,
      autonomy_envelope_ref: nil,
      autonomy_envelope_revision: nil,
      state: :queued,
      valid_until: @valid_until,
      fetched_at: @valid_ts
    }
  end

  test "new/1 builds a WorkCommission with a hash-matched Scope" do
    assert {:ok, %WorkCommission{} = commission} =
             work_commission_attrs()
             |> WorkCommission.new()

    assert commission.id == "wc-123"
    assert commission.scope_hash == commission.scope.hash
    assert commission.base_sha == commission.scope.base_sha
  end

  test "new/1 rejects scope_hash drift" do
    attrs = Map.put(work_commission_attrs(), :scope_hash, String.duplicate("0", 64))

    assert {:error, :scope_hash_mismatch} = WorkCommission.new(attrs)
  end

  test "new/1 rejects embedded Scope hash drift" do
    attrs = work_commission_attrs()
    scope = %{attrs.scope | hash: String.duplicate("0", 64)}
    attrs = Map.put(attrs, :scope, scope)

    assert {:error, :scope_hash_mismatch} = WorkCommission.new(attrs)
  end

  test "new/1 rejects base_sha drift from Scope" do
    attrs = Map.put(work_commission_attrs(), :base_sha, "different-sha")

    assert {:error, :base_sha_scope_mismatch} = WorkCommission.new(attrs)
  end

  test "new/1 rejects lockset drift from Scope" do
    attrs = Map.put(work_commission_attrs(), :lockset, ["open-sleigh/lib/other.ex"])

    assert {:error, :lockset_scope_mismatch} = WorkCommission.new(attrs)
  end

  test "new/1 rejects missing problem_card_ref" do
    attrs = Map.put(work_commission_attrs(), :problem_card_ref, nil)

    assert {:error, :invalid_problem_card_ref} = WorkCommission.new(attrs)
  end

  test "revision_snapshot/2 carries the deterministic equality inputs" do
    {:ok, commission} = WorkCommission.new(work_commission_attrs())

    assert {:ok, %CommissionRevisionSnapshot{} = snapshot} =
             WorkCommission.revision_snapshot(commission, %{
               problem_revision_hash: "problem-r1",
               lease_id: "lease-1",
               lease_state: :claimed_for_preflight
             })

    assert snapshot.commission_id == commission.id
    assert snapshot.scope_hash == commission.scope_hash
    assert snapshot.projection_policy == :local_only
    assert CommissionRevisionSnapshot.valid_hash?(snapshot.hash)
  end

  test "revision_snapshot/2 rejects missing equality inputs" do
    {:ok, commission} = WorkCommission.new(work_commission_attrs())

    assert {:error, :invalid_problem_revision_hash} =
             WorkCommission.revision_snapshot(commission, %{
               lease_id: "lease-1",
               lease_state: :claimed_for_preflight
             })
  end

  test "CommissionRevisionSnapshot.equal?/2 compares canonical inputs" do
    {:ok, commission} = WorkCommission.new(work_commission_attrs())

    snapshot_attrs = %{
      problem_revision_hash: "problem-r1",
      lease_id: "lease-1",
      lease_state: :claimed_for_preflight
    }

    {:ok, left} = WorkCommission.revision_snapshot(commission, snapshot_attrs)
    {:ok, right} = WorkCommission.revision_snapshot(commission, snapshot_attrs)

    assert CommissionRevisionSnapshot.equal?(left, right)
  end
end
