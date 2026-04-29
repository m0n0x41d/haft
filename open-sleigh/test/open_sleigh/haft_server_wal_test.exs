defmodule OpenSleigh.HaftServerWalTest do
  use ExUnit.Case, async: false

  alias OpenSleigh.HaftServer

  setup do
    wal_dir = tmp_wal_dir()
    on_exit(fn -> File.rm_rf!(wal_dir) end)

    %{wal_dir: wal_dir}
  end

  test "failed dispatch is stored and replayed after health recovery", %{wal_dir: wal_dir} do
    {:ok, calls} = Agent.start_link(fn -> %{failed?: false, replayed: []} end)
    invoke_fun = recoverable_invoker(calls)
    server_name = server_name("HaftServerWalReplay")

    {:ok, server} =
      HaftServer.start_link(
        invoke_fun: invoke_fun,
        health_interval_ms: 0,
        wal_dir: wal_dir,
        now_ms_fun: fn -> 1_000 end,
        name: server_name
      )

    request = request_line("OPS-1", 11)

    assert {:error, :haft_unavailable} = HaftServer.call(server_name, request)
    assert wal_files(wal_dir) == [Path.join(wal_dir, "OPS-1.jsonl")]

    send(server, :health_ping)

    assert :ok = wait_until(fn -> recovered?(server_name, wal_dir) end, 500)
    assert Agent.get(calls, & &1.replayed) == [request]
  end

  test "requests are appended while the server is already unavailable", %{wal_dir: wal_dir} do
    invoke_fun = fn _request_line -> {:error, :haft_unavailable} end
    server_name = server_name("HaftServerWalUnavailable")

    {:ok, _server} =
      HaftServer.start_link(
        invoke_fun: invoke_fun,
        health_interval_ms: 10,
        health_failure_threshold: 1,
        wal_dir: wal_dir,
        now_ms_fun: fn -> 2_000 end,
        name: server_name
      )

    assert :ok = wait_until(fn -> not HaftServer.status(server_name).available end, 500)

    request = request_line("OPS-2", 12)

    assert {:error, :haft_unavailable} = HaftServer.call(server_name, request)
    assert wal_files(wal_dir) == [Path.join(wal_dir, "OPS-2.jsonl")]
  end

  defp recoverable_invoker(calls) do
    fn request_line ->
      request_line
      |> classify_request()
      |> invoke_classified_request(calls, request_line)
    end
  end

  defp invoke_classified_request(:health, _calls, request_line) do
    {:ok, response_line(request_line)}
  end

  defp invoke_classified_request(:artifact, calls, request_line) do
    Agent.get_and_update(calls, fn state ->
      state
      |> artifact_reply(request_line)
    end)
  end

  defp artifact_reply(%{failed?: false} = state, _request_line) do
    reply = {:error, :haft_unavailable}
    next_state = %{state | failed?: true}

    {reply, next_state}
  end

  defp artifact_reply(%{failed?: true} = state, request_line) do
    reply = {:ok, response_line(request_line)}
    next_state = %{state | replayed: state.replayed ++ [request_line]}

    {reply, next_state}
  end

  defp classify_request(request_line) do
    request_line
    |> Jason.decode!()
    |> get_in(["params", "arguments", "config_hash"])
    |> request_class()
  end

  defp request_class("open-sleigh-health"), do: :health
  defp request_class(_config_hash), do: :artifact

  defp recovered?(server_name, wal_dir) do
    status = HaftServer.status(server_name)

    status.available and wal_files(wal_dir) == []
  end

  defp request_line(ticket_id, id) do
    Jason.encode!(%{
      "jsonrpc" => "2.0",
      "id" => id,
      "method" => "tools/call",
      "params" => %{
        "name" => "haft_note",
        "arguments" => %{
          "ticket_id" => ticket_id,
          "action" => "apply",
          "config_hash" => "test"
        }
      }
    }) <> "\n"
  end

  defp response_line(request_line) do
    id =
      request_line
      |> Jason.decode!()
      |> Map.fetch!("id")

    Jason.encode!(%{
      "jsonrpc" => "2.0",
      "id" => id,
      "result" => %{"artifact_id" => "mock-haft-#{id}"}
    }) <> "\n"
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

  defp wal_files(wal_dir) do
    wal_dir
    |> Path.join("*.jsonl")
    |> Path.wildcard()
    |> Enum.sort()
  end

  defp tmp_wal_dir do
    System.tmp_dir!()
    |> Path.join("open_sleigh_haft_server_wal_#{System.unique_integer([:positive])}")
  end

  defp server_name(prefix) do
    String.to_atom("#{prefix}_#{:erlang.unique_integer([:positive])}")
  end
end
