defmodule OpenSleigh.RuntimeLogWriterTest do
  use ExUnit.Case, async: true

  alias OpenSleigh.RuntimeLogWriter

  setup do
    root =
      System.tmp_dir!()
      |> Path.join("open-sleigh-runtime-log-" <> unique_suffix())

    File.rm_rf!(root)

    on_exit(fn -> File.rm_rf!(root) end)

    %{root: root}
  end

  test "commission-first metadata scopes path and payload by commission_id", ctx do
    log_root = Path.join(ctx.root, "wal")

    {:ok, writer} =
      RuntimeLogWriter.start_link(
        path: log_root,
        metadata: %{
          commission_id: "wc/local-1",
          ticket_id: "OCT-1",
          source_mode: :commission_first
        }
      )

    :ok = RuntimeLogWriter.event(writer, :phase_completed, %{phase: :execute})
    :ok = RuntimeLogWriter.stop(writer)

    log_path = Path.join(log_root, "wc_local-1.jsonl")
    events = log_events(log_path)

    assert Enum.all?(events, &(&1["commission_id"] == "wc/local-1"))
    assert Enum.all?(events, &(&1["legacy_ticket_id"] == "OCT-1"))
    assert Enum.any?(events, &(&1["event"] == "phase_completed"))
  end

  test "legacy metadata keeps ticket_id scoped path and payload", ctx do
    log_root = Path.join(ctx.root, "wal")

    {:ok, writer} =
      RuntimeLogWriter.start_link(
        path: log_root,
        metadata: %{ticket_id: "OCT-2"}
      )

    :ok = RuntimeLogWriter.stop(writer)

    log_path = Path.join(log_root, "OCT-2.jsonl")
    events = log_events(log_path)

    assert Enum.all?(events, &(&1["ticket_id"] == "OCT-2"))
    refute Enum.any?(events, &Map.has_key?(&1, "commission_id"))
  end

  defp log_events(path) do
    path
    |> File.read!()
    |> String.split("\n", trim: true)
    |> Enum.map(&Jason.decode!/1)
  end

  defp unique_suffix do
    [:positive, :monotonic]
    |> System.unique_integer()
    |> Integer.to_string()
  end
end
