defmodule OpenSleigh.Agent.ToolRuntimeTest do
  use ExUnit.Case, async: true

  alias OpenSleigh.Agent.Adapter
  alias OpenSleigh.Agent.ToolRuntime
  alias OpenSleigh.Fixtures
  alias OpenSleigh.Haft.Mock
  alias OpenSleigh.{Scope, WorkCommission}

  @fetched_at ~U[2026-04-22 10:00:00Z]
  @valid_until ~U[2026-05-22 10:00:00Z]

  setup do
    workspace =
      System.tmp_dir!()
      |> Path.join("open_sleigh_tool_runtime_#{System.unique_integer([:positive])}")

    File.mkdir_p!(workspace)

    on_exit(fn ->
      File.rm_rf!(workspace)
    end)

    %{workspace: workspace}
  end

  test "executes local read and edit tools inside the workspace", ctx do
    path = Path.join(ctx.workspace, "notes.txt")
    File.write!(path, "alpha beta")

    session = adapter_session(ctx.workspace)

    assert {:ok, %{tool: :read, output: read_output}} =
             ToolRuntime.execute("read", %{"path" => "notes.txt"}, session)

    assert read_output =~ "path: notes.txt"
    assert read_output =~ "alpha beta"

    assert {:ok, %{tool: :edit, output: edit_output}} =
             ToolRuntime.execute(
               "edit",
               %{"path" => "notes.txt", "old" => "beta", "new" => "gamma"},
               session
             )

    assert edit_output =~ "edited notes.txt"
    assert File.read!(path) == "alpha gamma"
  end

  test "routes Haft MCP tools through the provided invoke fun", ctx do
    {:ok, handle} = Mock.start()

    session = adapter_session(ctx.workspace)
    invoke_fun = Mock.invoke_fun(handle)

    assert {:ok, %{tool: :haft_query, output: output}} =
             ToolRuntime.execute(
               "haft_query",
               %{"action" => "status"},
               session,
               haft_invoker: invoke_fun
             )

    assert output =~ "\"artifact_id\":\"mock-haft-"
  end

  test "rejects write outside commission scope before creating a file", ctx do
    commission =
      ["allowed.txt"]
      |> scoped_commission!([], MapSet.new([:edit_files]))

    session =
      ctx.workspace
      |> adapter_session(commission, MapSet.new([:write]))

    target = Path.join(ctx.workspace, "outside.txt")

    assert {:error, :mutation_outside_commission_scope} =
             ToolRuntime.execute(
               "write",
               %{"path" => "outside.txt", "content" => "outside"},
               session
             )

    refute File.exists?(target)
  end

  test "rejects edit outside commission scope before changing a file", ctx do
    outside = Path.join(ctx.workspace, "outside.txt")
    File.write!(outside, "original")

    commission =
      ["allowed.txt"]
      |> scoped_commission!([], MapSet.new([:edit_files]))

    session =
      ctx.workspace
      |> adapter_session(commission, MapSet.new([:edit]))

    assert {:error, :mutation_outside_commission_scope} =
             ToolRuntime.execute(
               "edit",
               %{"path" => "outside.txt", "old" => "original", "new" => "mutated"},
               session
             )

    assert File.read!(outside) == "original"
  end

  test "dynamic tool response rejects forbidden path before mutation", ctx do
    commission =
      ["lib/**"]
      |> scoped_commission!(["lib/secret.ex"], MapSet.new([:edit_files]))

    session =
      ctx.workspace
      |> adapter_session(commission, MapSet.new([:write]))

    target = Path.join([ctx.workspace, "lib", "secret.ex"])

    assert {:ok, response} =
             ToolRuntime.dynamic_response(
               "write",
               %{"path" => "lib/secret.ex", "content" => "secret"},
               session
             )

    refute response["success"]
    assert response_text(response) =~ "tool_error: mutation_outside_commission_scope"
    refute File.exists?(target)
  end

  test "bash without run_tests action is rejected before command execution", ctx do
    commission =
      ["**/*"]
      |> scoped_commission!([], MapSet.new([:edit_files]))

    session =
      ctx.workspace
      |> adapter_session(commission, MapSet.new([:bash]))

    target = Path.join(ctx.workspace, "should-not-exist.txt")

    assert {:error, :mutation_outside_commission_scope} =
             ToolRuntime.execute(
               "bash",
               %{"command" => "printf forbidden > should-not-exist.txt"},
               session
             )

    refute File.exists?(target)
  end

  test "bash path mutation is terminal-diff checked because shell text is opaque", ctx do
    commission =
      ["allowed.txt"]
      |> scoped_commission!([], MapSet.new([:run_tests]))

    session =
      ctx.workspace
      |> adapter_session(commission, MapSet.new([:bash]))

    assert {:ok, %{tool: :bash, output: output}} =
             ToolRuntime.execute(
               "bash",
               %{"command" => "printf shell > outside.txt"},
               session
             )

    assert output =~ "exit_status: 0"
    assert File.read!(Path.join(ctx.workspace, "outside.txt")) == "shell"

    assert {:error, :mutation_outside_commission_scope} =
             Adapter.validate_terminal_diff(session, ["outside.txt"])
  end

  defp adapter_session(workspace) do
    Fixtures.adapter_session(%{
      workspace_path: workspace,
      scoped_tools: MapSet.new([:read, :edit, :haft_query])
    })
  end

  defp adapter_session(workspace, commission, scoped_tools) do
    %{workspace_path: workspace, scoped_tools: scoped_tools}
    |> Fixtures.adapter_session()
    |> Adapter.attach_commission_context(commission)
  end

  defp scoped_commission!(allowed_paths, forbidden_paths, allowed_actions) do
    allowed_paths
    |> scope!(forbidden_paths, allowed_actions)
    |> commission!()
  end

  defp commission!(scope) do
    attrs = %{
      id: "wc-tool-runtime",
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

    {:ok, commission} =
      attrs
      |> WorkCommission.new()

    commission
  end

  defp scope!(allowed_paths, forbidden_paths, allowed_actions) do
    attrs = %{
      repo_ref: "github:m0n0x41d/haft",
      base_sha: "abc123",
      target_branch: "feature/tool-runtime-scope",
      allowed_paths: allowed_paths,
      forbidden_paths: forbidden_paths,
      allowed_actions: allowed_actions,
      affected_files: allowed_paths,
      lockset: allowed_paths
    }

    {:ok, hash} =
      attrs
      |> Scope.canonical_hash()

    {:ok, scope} =
      attrs
      |> Map.put(:hash, hash)
      |> Scope.new()

    scope
  end

  defp response_text(response) do
    response
    |> Map.fetch!("contentItems")
    |> Enum.map(&Map.fetch!(&1, "text"))
    |> Enum.join("\n")
  end
end
