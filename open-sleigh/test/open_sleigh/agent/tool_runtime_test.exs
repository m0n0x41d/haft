defmodule OpenSleigh.Agent.ToolRuntimeTest do
  use ExUnit.Case, async: true

  alias OpenSleigh.Agent.ToolRuntime
  alias OpenSleigh.Fixtures
  alias OpenSleigh.Haft.Mock

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

  defp adapter_session(workspace) do
    Fixtures.adapter_session(%{
      workspace_path: workspace,
      scoped_tools: MapSet.new([:read, :edit, :haft_query])
    })
  end
end
