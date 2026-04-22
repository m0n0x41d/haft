defmodule OpenSleigh.Haft.WalTest do
  use ExUnit.Case, async: true

  alias OpenSleigh.Haft.Wal

  setup do
    wal_dir = tmp_wal_dir()
    on_exit(fn -> File.rm_rf!(wal_dir) end)

    %{wal_dir: wal_dir}
  end

  test "append writes per-ticket JSONL entries", %{wal_dir: wal_dir} do
    first_request = request_line("OPS-1", 1)
    second_request = request_line("OPS-1", 2)

    assert :ok = Wal.append(wal_dir, first_request, 1_000)
    assert :ok = Wal.append(wal_dir, second_request, 1_001)

    entries =
      wal_dir
      |> Path.join("OPS-1.jsonl")
      |> read_entries()

    assert Enum.map(entries, & &1["ticket_id"]) == ["OPS-1", "OPS-1"]
    assert Enum.map(entries, & &1["appended_at_unix_ms"]) == [1_000, 1_001]
    assert Enum.map(entries, & &1["request"]) == [first_request, second_request]
  end

  test "replay sends stored requests and removes completed files", %{wal_dir: wal_dir} do
    first_request = request_line("OPS-1", 1)
    second_request = request_line("OPS-1", 2)

    :ok = Wal.append(wal_dir, first_request, 1_000)
    :ok = Wal.append(wal_dir, second_request, 1_001)

    {:ok, calls} = Agent.start_link(fn -> [] end)

    invoker = fn request_line ->
      Agent.update(calls, fn existing -> existing ++ [request_line] end)
      {:ok, response_line(request_line)}
    end

    assert :ok = Wal.replay(wal_dir, invoker)
    assert Agent.get(calls, & &1) == [first_request, second_request]
    assert wal_files(wal_dir) == []
  end

  test "replay leaves a file intact when any request fails", %{wal_dir: wal_dir} do
    request = request_line("OPS-1", 1)
    :ok = Wal.append(wal_dir, request, 1_000)

    invoker = fn _request_line -> {:error, :haft_unavailable} end

    assert {:error, :haft_unavailable} = Wal.replay(wal_dir, invoker)
    assert wal_files(wal_dir) == [Path.join(wal_dir, "OPS-1.jsonl")]
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
      "result" => %{"artifact_id" => "replayed-#{id}"}
    }) <> "\n"
  end

  defp read_entries(path) do
    path
    |> File.read!()
    |> String.split("\n", trim: true)
    |> Enum.map(&Jason.decode!/1)
  end

  defp wal_files(wal_dir) do
    wal_dir
    |> Path.join("*.jsonl")
    |> Path.wildcard()
    |> Enum.sort()
  end

  defp tmp_wal_dir do
    System.tmp_dir!()
    |> Path.join("open_sleigh_haft_wal_#{System.unique_integer([:positive])}")
  end
end
