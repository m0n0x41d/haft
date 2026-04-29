defmodule OpenSleigh.HaftServerTest do
  use ExUnit.Case, async: false

  alias OpenSleigh.{Haft.Mock, Haft.Protocol, HaftServer}

  setup do
    {:ok, haft} = Mock.start()
    invoke_fun = Mock.invoke_fun(haft)

    server_name = String.to_atom("HaftServer_#{:erlang.unique_integer([:positive])}")

    {:ok, server} =
      HaftServer.start_link(invoke_fun: invoke_fun, name: server_name)

    %{server: server, server_name: server_name, haft_mock: haft}
  end

  test "call/2 routes through injected invoke_fun", ctx do
    request =
      Jason.encode!(%{
        "jsonrpc" => "2.0",
        "id" => 1,
        "method" => "tools/call",
        "params" => %{"name" => "haft_query", "arguments" => %{}}
      })

    assert {:ok, response} = HaftServer.call(ctx.server_name, request)
    {:ok, decoded} = Jason.decode(response)
    assert decoded["id"] == 1
    assert decoded["result"]["artifact_id"] == "mock-haft-1"
  end

  test "invoke_fun/1 returns a function compatible with Haft.Client", ctx do
    fun = HaftServer.invoke_fun(ctx.server_name)

    request =
      Jason.encode!(%{"id" => 2, "params" => %{"name" => "haft_note", "arguments" => %{}}})

    assert {:ok, response} = fun.(request)
    {:ok, decoded} = Jason.decode(response)
    assert decoded["id"] == 2
  end

  test "command mode owns a Port and routes request/response lines" do
    workspace = tmp_workspace()
    script_path = Path.join(workspace, "mock_haft_serve.exs")
    File.write!(script_path, mock_haft_serve_script())
    File.chmod!(script_path, 0o755)
    on_exit(fn -> File.rm_rf!(workspace) end)

    server_name = String.to_atom("HaftServerPort_#{:erlang.unique_integer([:positive])}")
    command = "elixir #{shell_escape(script_path)}"

    {:ok, _server} =
      HaftServer.start_link(
        command: command,
        project_root: workspace,
        read_timeout_ms: 1_000,
        health_interval_ms: 0,
        name: server_name
      )

    request = Protocol.encode_health_ping(7)

    assert {:ok, response} = HaftServer.call(server_name, request)
    {:ok, decoded} = Jason.decode(response)
    assert decoded["id"] == 7
    assert decoded["result"]["artifact_id"] == "mock-haft-port-7"

    status = HaftServer.status(server_name)
    assert status.available
    assert status.mode == :port
  end

  test "health ping marks server unavailable after configured misses" do
    server_name = String.to_atom("HaftServerHealth_#{:erlang.unique_integer([:positive])}")
    failing_invoker = fn _request_line -> {:error, :haft_unavailable} end

    {:ok, _server} =
      HaftServer.start_link(
        invoke_fun: failing_invoker,
        health_interval_ms: 10,
        health_failure_threshold: 3,
        name: server_name
      )

    assert :ok = wait_until(fn -> not HaftServer.status(server_name).available end, 500)

    assert {:error, :haft_unavailable} =
             HaftServer.call(server_name, Protocol.encode_health_ping(99))

    assert HaftServer.status(server_name).health_misses >= 3
  end

  @tag :integration
  test "real haft serve supports a status round-trip when haft is installed" do
    case System.find_executable("haft") do
      nil ->
        :ok

      _haft ->
        server_name = String.to_atom("HaftServerReal_#{:erlang.unique_integer([:positive])}")

        {:ok, _server} =
          HaftServer.start_link(
            command: "haft serve",
            project_root: File.cwd!(),
            read_timeout_ms: 5_000,
            health_interval_ms: 0,
            name: server_name
          )

        assert {:ok, response} = HaftServer.call(server_name, Protocol.encode_health_ping(42))
        assert {:ok, {42, result}} = Protocol.decode_response(response)
        assert is_map(result)
    end
  end

  defp tmp_workspace do
    System.tmp_dir!()
    |> Path.join("open_sleigh_haft_server_#{System.unique_integer([:positive])}")
    |> tap(&File.mkdir_p!/1)
  end

  defp wait_until(check_fun, timeout_ms) do
    deadline = System.monotonic_time(:millisecond) + timeout_ms
    do_wait_until(check_fun, deadline)
  end

  defp do_wait_until(check_fun, deadline) do
    cond do
      check_fun.() ->
        :ok

      System.monotonic_time(:millisecond) > deadline ->
        {:error, :timeout}

      true ->
        Process.sleep(10)
        do_wait_until(check_fun, deadline)
    end
  end

  defp shell_escape(value) do
    "'" <> String.replace(value, "'", "'\"'\"'") <> "'"
  end

  defp mock_haft_serve_script do
    ~S"""
    defmodule MockHaftServe do
      def run do
        loop()
      end

      defp loop do
        case IO.read(:line) do
          :eof ->
            :ok

          line when is_binary(line) ->
            handle_line(line)
            loop()
        end
      end

      defp handle_line(line) do
        cond do
          method?(line, "initialize") ->
            line
            |> request_id()
            |> response(~s({"capabilities":{"tools":{}},"protocolVersion":"2024-11-05","serverInfo":{"name":"mock-haft","version":"test"}}))

          method?(line, "tools/call") ->
            id = request_id(line)
            response(id, ~s({"artifact_id":"mock-haft-port-#{id}","content":[{"type":"text","text":"ok"}]}))

          true ->
            :ok
        end
      end

      defp method?(line, method) do
        Regex.match?(~r/"method"\s*:\s*"#{Regex.escape(method)}"/, line)
      end

      defp request_id(line) do
        case Regex.run(~r/"id"\s*:\s*(-?\d+)/, line) do
          [_match, id] -> id
          _ -> "0"
        end
      end

      defp response(id, result) do
        IO.puts(~s({"jsonrpc":"2.0","id":#{id},"result":#{result}}))
      end
    end

    MockHaftServe.run()
    """
  end
end
