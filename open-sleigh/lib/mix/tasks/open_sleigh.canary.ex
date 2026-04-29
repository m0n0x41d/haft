defmodule Mix.Tasks.OpenSleigh.Canary do
  @shortdoc "Run the Open-Sleigh local canary"

  @moduledoc """
  Run the local Open-Sleigh canary.

      mix open_sleigh.canary
      mix open_sleigh.canary --duration=2m

  Options:

    * `--duration` - accepted duration marker for operator scripts.
      The current MVP-1 executable canary runs one mock-backed T3
      HumanGate pass per invocation.
    * `--help` - print this help.
  """

  use Mix.Task

  @impl true
  def run(args) do
    args
    |> parse_args()
    |> run_parsed()
  end

  @spec parse_args([String.t()]) :: {:help | :run, keyword()}
  defp parse_args(args) do
    {opts, _argv, invalid} =
      OptionParser.parse(
        args,
        switches: [duration: :string, help: :boolean],
        aliases: [h: :help]
      )

    if invalid == [] do
      parsed_mode(opts)
    else
      Mix.raise("Invalid options: #{inspect(invalid)}")
    end
  end

  @spec parsed_mode(keyword()) :: {:help | :run, keyword()}
  defp parsed_mode(opts) do
    if Keyword.get(opts, :help, false) do
      {:help, opts}
    else
      {:run, opts}
    end
  end

  @spec run_parsed({:help | :run, keyword()}) :: :ok
  defp run_parsed({:help, _opts}) do
    Mix.shell().info(@moduledoc)
  end

  defp run_parsed({:run, opts}) do
    case OpenSleigh.Canary.run(opts) do
      {:ok, summary} ->
        Mix.shell().info("Open-Sleigh canary passed: #{Jason.encode!(summary)}")

      {:error, reason} ->
        Mix.raise("Open-Sleigh canary failed: #{inspect(reason)}")
    end
  end
end
