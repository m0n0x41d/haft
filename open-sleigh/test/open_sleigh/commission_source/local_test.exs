defmodule OpenSleigh.CommissionSource.LocalTest do
  use ExUnit.Case, async: true

  alias OpenSleigh.CommissionSource.Local
  alias OpenSleigh.WorkCommission

  setup do
    tmp =
      Path.join(
        System.tmp_dir!(),
        "open_sleigh_commission_source_#{:erlang.unique_integer([:positive, :monotonic])}"
      )

    File.mkdir_p!(tmp)

    on_exit(fn -> File.rm_rf!(tmp) end)

    %{tmp: tmp}
  end

  test "lists runnable commissions from JSON fixture path configured in commission_source", ctx do
    path = Path.join(ctx.tmp, "commissions.json")

    path
    |> write_json_fixture([
      commission_payload("wc-json-queued", "queued", "2099-01-01T00:00:00Z"),
      commission_payload("wc-json-ready", "ready", "2099-01-01T00:00:00Z"),
      commission_payload("wc-json-expired", "queued", "2000-01-01T00:00:00Z"),
      commission_payload("wc-json-completed", "completed", "2099-01-01T00:00:00Z")
    ])

    assert {:ok, handle} =
             Local.new(%{
               "commission_source" => %{
                 "kind" => "local",
                 "fixture_path" => path
               }
             })

    assert Local.adapter_kind() == :local

    assert {:ok, commissions} = Local.list_runnable(handle)

    assert Enum.map(commissions, & &1.id) == ["wc-json-queued", "wc-json-ready"]
    assert [%WorkCommission{} = commission | _rest] = commissions
    assert commission.projection_policy == :local_only
    assert commission.scope.allowed_actions == MapSet.new([:edit_files, :run_tests])
    assert commission.scope_hash == commission.scope.hash
  end

  test "claims the first runnable JSON commission for preflight", ctx do
    path = Path.join(ctx.tmp, "commissions.json")

    path
    |> write_json_fixture([
      commission_payload("wc-json-first", "queued", "2099-01-01T00:00:00Z")
    ])

    assert {:ok, handle} = Local.new(%{commission_source: %{fixture_path: path}})
    assert {:ok, %WorkCommission{} = claimed} = Local.claim_for_preflight(handle)

    assert claimed.id == "wc-json-first"
    assert claimed.state == :preflighting
  end

  test "claims a named runnable YAML commission for preflight", ctx do
    path = Path.join(ctx.tmp, "commissions.yaml")

    path
    |> write_yaml_fixture()

    assert {:ok, handle} = Local.new(%{commission_source: %{fixture_path: path}})

    assert {:ok, %WorkCommission{} = claimed} =
             Local.claim_for_preflight(handle, "wc-yaml-second")

    assert claimed.id == "wc-yaml-second"
    assert claimed.state == :preflighting
    assert claimed.scope.base_sha == "abc123"
  end

  test "rejects missing local fixture path" do
    assert {:error, :fixture_path_missing} = Local.new(%{commission_source: %{kind: "local"}})
  end

  defp write_json_fixture(path, commissions) do
    payload =
      commissions
      |> fixture_payload()
      |> Jason.encode!()

    File.write!(path, payload)
  end

  defp write_yaml_fixture(path) do
    File.write!(path, """
    commissions:
      - id: "wc-yaml-first"
        decision_ref: "dec-20260422-001"
        decision_revision_hash: "decision-r1"
        problem_card_ref: "pc-20260422-001"
        evidence_requirements:
          - kind: "mix_test"
            command: "mix test"
        projection_policy: "local_only"
        state: "ready"
        valid_until: "2099-01-01T00:00:00Z"
        fetched_at: "2026-04-22T10:00:00Z"
        scope:
          repo_ref: "github:m0n0x41d/haft"
          base_sha: "abc123"
          target_branch: "feature/commission-source"
          allowed_paths:
            - "open-sleigh/lib/open_sleigh/commission_source/local.ex"
          forbidden_paths: []
          allowed_actions:
            - "edit_files"
            - "run_tests"
          affected_files:
            - "open-sleigh/lib/open_sleigh/commission_source/local.ex"
          allowed_modules:
            - "OpenSleigh.CommissionSource.Local"
          lockset:
            - "open-sleigh/lib/open_sleigh/commission_source/local.ex"
      - id: "wc-yaml-second"
        decision_ref: "dec-20260422-001"
        decision_revision_hash: "decision-r1"
        problem_card_ref: "pc-20260422-001"
        evidence_requirements:
          - kind: "mix_test"
            command: "mix test"
        projection_policy: "local_only"
        state: "queued"
        valid_until: "2099-01-01T00:00:00Z"
        fetched_at: "2026-04-22T10:00:00Z"
        scope:
          repo_ref: "github:m0n0x41d/haft"
          base_sha: "abc123"
          target_branch: "feature/commission-source"
          allowed_paths:
            - "open-sleigh/lib/open_sleigh/commission_source/local.ex"
          forbidden_paths: []
          allowed_actions:
            - "edit_files"
            - "run_tests"
          affected_files:
            - "open-sleigh/lib/open_sleigh/commission_source/local.ex"
          lockset:
            - "open-sleigh/lib/open_sleigh/commission_source/local.ex"
    """)
  end

  defp fixture_payload(commissions) do
    %{
      "commissions" => commissions
    }
  end

  defp commission_payload(id, state, valid_until) do
    %{
      "id" => id,
      "decision_ref" => "dec-20260422-001",
      "decision_revision_hash" => "decision-r1",
      "problem_card_ref" => "pc-20260422-001",
      "evidence_requirements" => [
        %{
          "kind" => "mix_test",
          "command" => "mix test"
        }
      ],
      "projection_policy" => "local_only",
      "state" => state,
      "valid_until" => valid_until,
      "fetched_at" => "2026-04-22T10:00:00Z",
      "scope" => %{
        "repo_ref" => "github:m0n0x41d/haft",
        "base_sha" => "abc123",
        "target_branch" => "feature/commission-source",
        "allowed_paths" => [
          "open-sleigh/lib/open_sleigh/commission_source/local.ex"
        ],
        "forbidden_paths" => [],
        "allowed_actions" => [
          "edit_files",
          "run_tests"
        ],
        "affected_files" => [
          "open-sleigh/lib/open_sleigh/commission_source/local.ex"
        ],
        "lockset" => [
          "open-sleigh/lib/open_sleigh/commission_source/local.ex"
        ]
      }
    }
  end
end
