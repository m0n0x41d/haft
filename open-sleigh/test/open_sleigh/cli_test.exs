defmodule OpenSleigh.CLITest do
  use ExUnit.Case, async: false

  setup do
    Mix.shell(Mix.Shell.Process)

    on_exit(fn ->
      Mix.shell(Mix.Shell.IO)
      Mix.Task.reenable("open_sleigh.gate_report")
    end)

    :ok
  end

  test "prints top-level help" do
    assert :ok = OpenSleigh.CLI.main(["--help"])

    assert_receive {:mix_shell, :info, [help]}, 500
    assert String.contains?(help, "open_sleigh <command>")
    assert String.contains?(help, "gate_report")
  end

  test "delegates a command to the task module" do
    assert :ok = OpenSleigh.CLI.main(["gate_report", "--json"])

    assert_receive {:mix_shell, :info, [encoded]}, 500
    assert {:ok, %{"failed" => 0}} = Jason.decode(encoded)
  end
end
