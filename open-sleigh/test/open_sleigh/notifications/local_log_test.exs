defmodule OpenSleigh.Notifications.LocalLogTest do
  use ExUnit.Case, async: true

  alias OpenSleigh.Notifications.{Adapter, LocalLog}

  test "validates the closed notification shape" do
    assert Adapter.valid?(%{
             kind: :human_gate_pending,
             message: "Approval needed",
             metadata: %{ticket: "OCT-1"}
           })

    refute Adapter.valid?(%{kind: :anything, message: "x", metadata: %{}})
  end

  test "writes local JSONL notification events" do
    path = tmp_notification_path()
    handle = LocalLog.handle(path)

    assert :ok =
             LocalLog.notify(handle, %{
               kind: :blocking_failure,
               message: "Session failed",
               metadata: %{ticket: "OCT-2", reason: "thread_start_failed"}
             })

    assert [encoded] =
             path
             |> File.read!()
             |> String.split("\n", trim: true)

    assert {:ok, event} = Jason.decode(encoded)
    assert event["kind"] == "blocking_failure"
    assert event["metadata"]["ticket"] == "OCT-2"
    assert is_binary(event["at"])
  end

  test "rejects invalid notification events" do
    path = tmp_notification_path()
    handle = LocalLog.handle(path)

    assert {:error, :invalid_notification} =
             LocalLog.notify(handle, %{kind: :unknown, message: "x", metadata: %{}})
  end

  defp tmp_notification_path do
    dir =
      System.tmp_dir!()
      |> Path.join("open_sleigh_notifications_#{System.unique_integer([:positive])}")

    on_exit(fn -> File.rm_rf!(dir) end)
    Path.join(dir, "notifications.jsonl")
  end
end
