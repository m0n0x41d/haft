defmodule OpenSleigh.Agent.CodexTest do
  use ExUnit.Case, async: false

  alias OpenSleigh.Agent.Codex
  alias OpenSleigh.Fixtures

  setup do
    old_env = Application.get_env(:open_sleigh, Codex)

    workspace =
      System.tmp_dir!()
      |> Path.join("open_sleigh_codex_test_#{System.unique_integer([:positive])}")

    File.mkdir_p!(workspace)

    script_path = Path.join(workspace, "mock_codex_app_server.exs")
    File.write!(script_path, mock_app_server_script())
    File.chmod!(script_path, 0o755)

    on_exit(fn ->
      restore_codex_env(old_env)
      File.rm_rf!(workspace)
    end)

    %{script_path: script_path, workspace: workspace}
  end

  test "starts a mock app-server, runs one turn, and closes", ctx do
    command = "elixir #{shell_escape(ctx.script_path)}"

    put_codex_env(
      command: command,
      read_timeout_ms: 2_000,
      turn_timeout_ms: 1_000,
      stall_timeout_ms: 500
    )

    session = adapter_session(ctx.workspace)

    assert {:ok, handle} = Codex.start_session(session)
    assert handle.thread_id == "thread-1"
    assert is_binary(handle.codex_app_server_pid)

    assert {:ok, reply} = Codex.send_turn(handle, "Implement the requested change", session)
    assert reply.status == :completed
    assert reply.turn_id == "turn-1"
    assert reply.text == "mock turn completed"
    assert reply.usage == %{"input_tokens" => 11, "output_tokens" => 7, "total_tokens" => 18}
    assert Enum.any?(reply.events, &(&1.event == :turn_completed))

    assert :ok = Codex.close_session(handle)
  end

  test "stall detection returns :stall_timeout when no turn event arrives", ctx do
    command = "MOCK_CODEX_MODE=stall elixir #{shell_escape(ctx.script_path)}"

    put_codex_env(
      command: command,
      read_timeout_ms: 2_000,
      turn_timeout_ms: 1_000,
      stall_timeout_ms: 50
    )

    session = adapter_session(ctx.workspace)

    assert {:ok, handle} = Codex.start_session(session)
    assert {:error, :stall_timeout} = Codex.send_turn(handle, "Pause after turn start", session)
    assert :ok = Codex.close_session(handle)
  end

  test "answers app-server approval requests during a turn", ctx do
    command = "MOCK_CODEX_MODE=approval elixir #{shell_escape(ctx.script_path)}"

    put_codex_env(
      command: command,
      read_timeout_ms: 2_000,
      turn_timeout_ms: 1_000,
      stall_timeout_ms: 500,
      approval_policy: "never"
    )

    session = adapter_session(ctx.workspace)

    assert {:ok, handle} = Codex.start_session(session)
    assert {:ok, reply} = Codex.send_turn(handle, "Run a command that needs approval", session)
    assert reply.status == :completed
    assert reply.turn_id == "turn-1"
    assert :ok = Codex.close_session(handle)
  end

  @tag :integration
  test "real CODEX_CMD app-server completes handshake and one turn", ctx do
    case System.get_env("CODEX_CMD") do
      nil ->
        :ok

      "" ->
        :ok

      command ->
        put_codex_env(
          command: command,
          read_timeout_ms: 5_000,
          turn_timeout_ms: 120_000,
          stall_timeout_ms: 30_000
        )

        session = adapter_session(ctx.workspace)

        assert {:ok, handle} = Codex.start_session(session)

        assert {:ok, reply} =
                 Codex.send_turn(handle, "Reply with exactly: open-sleigh-ok", session)

        assert reply.status == :completed
        assert :ok = Codex.close_session(handle)
    end
  end

  defp adapter_session(workspace) do
    Fixtures.adapter_session(%{
      adapter_kind: :codex,
      workspace_path: workspace,
      scoped_tools: MapSet.new([:read, :write, :bash, :haft_query])
    })
  end

  defp put_codex_env(opts) do
    Application.put_env(:open_sleigh, Codex, opts)
  end

  defp restore_codex_env(nil) do
    Application.delete_env(:open_sleigh, Codex)
  end

  defp restore_codex_env(value) do
    Application.put_env(:open_sleigh, Codex, value)
  end

  defp shell_escape(value) do
    "'" <> String.replace(value, "'", "'\"'\"'") <> "'"
  end

  defp mock_app_server_script do
    ~S"""
    defmodule MockCodexAppServer do
      def run do
        loop(0)
      end

      defp loop(turn_count) do
        case IO.read(:line) do
          :eof ->
            :ok

          line when is_binary(line) ->
            handle_line(line, turn_count)
        end
      end

      defp handle_line(line, turn_count) do
        cond do
          method?(line, "initialize") ->
            line
            |> request_id()
            |> response("{}")

            loop(turn_count)

          method?(line, "thread/start") ->
            line
            |> request_id()
            |> response(~s({"thread":{"id":"thread-1"}}))

            loop(turn_count)

          method?(line, "turn/start") ->
            next_turn = turn_count + 1

            line
            |> request_id()
            |> response(~s({"turn":{"id":"turn-#{next_turn}"}}))

            maybe_complete_turn(next_turn)
            loop(next_turn)

          true ->
            loop(turn_count)
        end
      end

      defp maybe_complete_turn(turn_number) do
        case System.get_env("MOCK_CODEX_MODE") do
          "stall" ->
            Process.sleep(:infinity)

          "approval" ->
            turn_number
            |> approval_request()
            |> IO.puts()

            case IO.read(:line) do
              response when is_binary(response) ->
                if String.contains?(response, "acceptForSession") do
                  turn_number
                  |> completed_event()
                  |> IO.puts()
                else
                  failed_event(turn_number)
                  |> IO.puts()
                end

              _ ->
                failed_event(turn_number)
                |> IO.puts()
            end

          _ ->
            turn_number
            |> completed_event()
            |> IO.puts()
        end
      end

      defp approval_request(turn_number) do
        ~s({"jsonrpc":"2.0","id":101,"method":"item/commandExecution/requestApproval","params":{"turn":{"id":"turn-#{turn_number}"},"command":"echo ok"}})
      end

      defp completed_event(turn_number) do
        ~s({"jsonrpc":"2.0","method":"turn/completed","params":{"turn":{"id":"turn-#{turn_number}"},"usage":{"input_tokens":11,"output_tokens":7,"total_tokens":18},"text":"mock turn completed"}})
      end

      defp failed_event(turn_number) do
        ~s({"jsonrpc":"2.0","method":"turn/failed","params":{"turn":{"id":"turn-#{turn_number}"},"message":"approval response missing"}})
      end

      defp method?(line, method) do
        Regex.match?(~r/"method"\s*:\s*"#{Regex.escape(method)}"/, line)
      end

      defp request_id(line) do
        case Regex.run(~r/"id"\s*:\s*(\d+)/, line) do
          [_match, id] -> id
          _ -> "0"
        end
      end

      defp response(id, result) do
        IO.puts(~s({"jsonrpc":"2.0","id":#{id},"result":#{result}}))
      end
    end

    MockCodexAppServer.run()
    """
  end
end
