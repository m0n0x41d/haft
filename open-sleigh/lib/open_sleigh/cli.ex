defmodule OpenSleigh.CLI do
  @moduledoc """
  Escript entrypoint for local operator use.

  The CLI is intentionally thin: command implementations remain in the same
  task modules used by development workflows, while operators can run
  `open_sleigh <command>` after `mix escript.build`.
  """

  @commands %{
    "start" => Mix.Tasks.OpenSleigh.Start,
    "doctor" => Mix.Tasks.OpenSleigh.Doctor,
    "status" => Mix.Tasks.OpenSleigh.Status,
    "canary" => Mix.Tasks.OpenSleigh.Canary,
    "gate_report" => Mix.Tasks.OpenSleigh.GateReport,
    "gate-report" => Mix.Tasks.OpenSleigh.GateReport
  }

  @doc "Escript main entrypoint."
  @spec main([String.t()]) :: :ok
  def main(args) when is_list(args) do
    :open_sleigh
    |> Application.ensure_all_started()
    |> ensure_started()

    args
    |> command()
    |> run_command()
  end

  @spec ensure_started({:ok, [atom()]} | {:error, term()}) :: :ok
  defp ensure_started({:ok, _apps}), do: :ok

  defp ensure_started({:error, reason}),
    do: Mix.raise("Open-Sleigh failed to start: #{inspect(reason)}")

  @spec command([String.t()]) :: {:help | :run, String.t() | nil, [String.t()]}
  defp command([]), do: {:help, nil, []}
  defp command(["--help" | rest]), do: {:help, nil, rest}
  defp command(["-h" | rest]), do: {:help, nil, rest}
  defp command([name | rest]), do: {:run, name, rest}

  @spec run_command({:help | :run, String.t() | nil, [String.t()]}) :: :ok
  defp run_command({:help, _name, _rest}) do
    Mix.shell().info(help_text())
  end

  defp run_command({:run, name, args}) do
    name
    |> task_module()
    |> run_task(args)
  end

  @spec task_module(String.t()) :: module() | nil
  defp task_module(name), do: Map.get(@commands, name)

  @spec run_task(module() | nil, [String.t()]) :: :ok
  defp run_task(nil, _args), do: Mix.raise("Unknown Open-Sleigh command. #{command_list()}")
  defp run_task(module, args), do: module.run(args)

  @spec help_text() :: String.t()
  defp help_text do
    """
    Open-Sleigh local operator CLI

    Usage:
      open_sleigh <command> [options]

    Commands:
      start        Start the runtime
      doctor       Check runtime readiness
      status       Read runtime status JSON
      canary       Run the local mock canary
      gate_report  Run semantic-gate golden-set report

    Use `open_sleigh <command> --help` for command-specific options.
    """
    |> String.trim()
  end

  @spec command_list() :: String.t()
  defp command_list do
    @commands
    |> Map.keys()
    |> Enum.sort()
    |> Enum.join(", ")
    |> then(&"Known commands: #{&1}")
  end
end
