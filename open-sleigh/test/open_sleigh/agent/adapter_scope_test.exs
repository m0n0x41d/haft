defmodule OpenSleigh.Agent.AdapterScopeTest do
  use ExUnit.Case, async: false

  alias OpenSleigh.Adapter.PathGuard
  alias OpenSleigh.Agent.Adapter
  alias OpenSleigh.{AdapterSession, ConfigHash, Scope, SessionId, WorkCommission}

  @fetched_at ~U[2026-04-22 10:00:00Z]
  @valid_until ~U[2026-05-22 10:00:00Z]

  setup do
    tmp =
      Path.join(
        System.tmp_dir!(),
        "open_sleigh_adapter_scope_#{:erlang.unique_integer([:positive, :monotonic])}"
      )

    workspace = Path.join(tmp, "workspace")
    File.mkdir_p!(workspace)

    on_exit(fn -> File.rm_rf!(tmp) end)

    config = %{forbidden_paths: [], forbidden_remote_substrings: ["open-sleigh"]}

    %{workspace: workspace, config: config}
  end

  test "mutation outside Scope but inside workspace fails with commission reason", ctx do
    commission =
      ["lib/a.ex"]
      |> scope!([])
      |> commission!()

    session =
      ctx.workspace
      |> adapter_session!(commission, MapSet.new([:write]))

    target = Path.join(ctx.workspace, "lib/b.ex")

    assert {:ok, "lib/b.ex"} =
             PathGuard.relative_to_workspace(ctx.workspace, target, ctx.config)

    assert {:error, :mutation_outside_commission_scope} =
             Adapter.ensure_in_scope(session, :write, %{path: target})
  end

  test "mutation inside Scope passes before adapter execution", ctx do
    commission =
      ["lib/a.ex"]
      |> scope!([])
      |> commission!()

    session =
      ctx.workspace
      |> adapter_session!(commission, MapSet.new([:write]))

    target = Path.join(ctx.workspace, "lib/a.ex")

    assert :ok = Adapter.ensure_in_scope(session, :write, %{path: target})
  end

  test "mutating tool without target path fails closed under a commission", ctx do
    commission =
      ["lib/a.ex"]
      |> scope!([])
      |> commission!()

    session =
      ctx.workspace
      |> adapter_session!(commission, MapSet.new([:write]))

    assert {:error, :mutation_outside_commission_scope} =
             Adapter.ensure_in_scope(session, :write)
  end

  test "terminal diff validation rejects out-of-scope changed files", ctx do
    commission =
      ["lib/a.ex"]
      |> scope!([])
      |> commission!()

    session =
      ctx.workspace
      |> adapter_session!(commission, MapSet.new([:write]))

    assert :ok = Adapter.validate_terminal_diff(session, ["lib/a.ex"])

    assert {:error, :mutation_outside_commission_scope} =
             Adapter.validate_terminal_diff(session, ["lib/a.ex", "lib/b.ex"])
  end

  test "terminal diff validation ignores runtime-owned scratch paths", ctx do
    commission =
      ["lib/a.ex"]
      |> scope!([])
      |> commission!()

    session =
      ctx.workspace
      |> adapter_session!(commission, MapSet.new([:write]))

    assert :ok =
             Adapter.validate_terminal_diff(session, [
               "lib/a.ex",
               ".tmp/gocache/cache-entry",
               ".tmp/home/go/pkg/mod/cache"
             ])
  end

  test "forbidden path overrides broad allowed scope", ctx do
    commission =
      ["lib/**"]
      |> scope!(["lib/secret.ex"])
      |> commission!()

    session =
      ctx.workspace
      |> adapter_session!(commission, MapSet.new([:write]))

    assert {:error, :mutation_outside_commission_scope} =
             Adapter.ensure_in_scope(session, :write, %{path: "lib/secret.ex"})
  end

  test "broad scope does not authorize absolute paths outside workspace", ctx do
    commission =
      ["**/*"]
      |> scope!([])
      |> commission!()

    session =
      ctx.workspace
      |> adapter_session!(commission, MapSet.new([:write]))

    assert {:error, :mutation_outside_commission_scope} =
             Adapter.ensure_in_scope(session, :write, %{path: "/tmp/outside-workspace.ex"})
  end

  defp adapter_session!(workspace, commission, scoped_tools) do
    attrs = %{
      session_id: SessionId.generate(),
      config_hash: ConfigHash.from_iodata("adapter-scope-test"),
      scoped_tools: scoped_tools,
      workspace_path: workspace,
      adapter_kind: :codex,
      adapter_version: "0.14.0",
      max_turns: 20,
      max_tokens_per_turn: 80_000,
      wall_clock_timeout_s: 600
    }

    {:ok, session} = AdapterSession.new(attrs)

    session
    |> Adapter.attach_commission_context(commission)
  end

  defp commission!(scope) do
    attrs = %{
      id: "wc-adapter-scope",
      decision_ref: "dec-20260422-001",
      decision_revision_hash: "decision-r1",
      problem_card_ref: "pc-20260422-001",
      scope: scope,
      scope_hash: scope.hash,
      base_sha: scope.base_sha,
      lockset: scope.lockset,
      evidence_requirements: [],
      projection_policy: :local_only,
      state: :running,
      valid_until: @valid_until,
      fetched_at: @fetched_at
    }

    {:ok, commission} = WorkCommission.new(attrs)
    commission
  end

  defp scope!(allowed_paths, forbidden_paths) do
    attrs = %{
      repo_ref: "github:m0n0x41d/haft",
      base_sha: "abc123",
      target_branch: "feature/scope",
      allowed_paths: allowed_paths,
      forbidden_paths: forbidden_paths,
      allowed_actions: MapSet.new([:edit_files, :run_tests]),
      affected_files: allowed_paths,
      lockset: allowed_paths
    }

    {:ok, hash} = Scope.canonical_hash(attrs)

    attrs =
      attrs
      |> Map.put(:hash, hash)

    {:ok, scope} = Scope.new(attrs)
    scope
  end
end
