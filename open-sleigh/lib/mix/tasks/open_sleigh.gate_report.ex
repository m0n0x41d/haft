defmodule Mix.Tasks.OpenSleigh.GateReport do
  @shortdoc "Run the semantic-gate golden set"

  @moduledoc """
  Run the semantic-gate golden set and print a calibration/drift report.

      mix open_sleigh.gate_report
      mix open_sleigh.gate_report --json
      mix open_sleigh.gate_report --path=sleigh.md --live

  Options:

    * `--path` - config file path used by `--live`. Defaults to `sleigh.md`.
    * `--live` - call the configured agent provider instead of the deterministic baseline.
    * `--json` - print a machine-readable report.
    * `--help` - print this help.
  """

  use Mix.Task

  alias OpenSleigh.{Agent, ConfigHash}
  alias OpenSleigh.Judge.{AgentInvoker, GoldenSets, RuleBased}
  alias OpenSleigh.Sleigh.Compiler

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
        switches: [path: :string, live: :boolean, json: :boolean, help: :boolean],
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
    report =
      opts
      |> invoke_fun()
      |> report()

    report
    |> emit_report(opts)
    |> raise_on_failure(report)
  end

  @spec invoke_fun(keyword()) :: OpenSleigh.JudgeClient.invoke_fun()
  defp invoke_fun(opts) do
    opts
    |> Keyword.get(:live, false)
    |> invoke_fun_for_mode(opts)
  end

  @spec invoke_fun_for_mode(boolean(), keyword()) :: OpenSleigh.JudgeClient.invoke_fun()
  defp invoke_fun_for_mode(false, _opts), do: &RuleBased.invoke/1

  defp invoke_fun_for_mode(true, opts) do
    opts
    |> config_path()
    |> live_invoker()
  end

  @spec config_path(keyword()) :: Path.t()
  defp config_path(opts), do: Keyword.get(opts, :path, "sleigh.md")

  @spec live_invoker(Path.t()) :: OpenSleigh.JudgeClient.invoke_fun()
  defp live_invoker(path) do
    path
    |> File.read()
    |> compile_source(path)
    |> live_invoker_from_bundle()
  end

  @spec compile_source({:ok, binary()} | {:error, term()}, Path.t()) ::
          {:ok, map()} | {:error, term()}
  defp compile_source({:ok, source}, _path), do: Compiler.compile(source)
  defp compile_source({:error, reason}, path), do: {:error, {:config_read_failed, path, reason}}

  @spec live_invoker_from_bundle({:ok, map()} | {:error, term()}) ::
          OpenSleigh.JudgeClient.invoke_fun()
  defp live_invoker_from_bundle({:ok, bundle}) do
    bundle
    |> live_invoker_config()
    |> AgentInvoker.invoke_fun()
  end

  defp live_invoker_from_bundle({:error, reason}) do
    Mix.raise("Open-Sleigh gate report failed to load config: #{inspect(reason)}")
  end

  @spec live_invoker_config(map()) :: AgentInvoker.config()
  defp live_invoker_config(bundle) do
    AgentInvoker.config(
      judge_adapter(bundle),
      judge_workspace(bundle),
      judge_config_hash(bundle),
      judge_attrs(bundle)
    )
  end

  @spec judge_adapter(map()) :: module()
  defp judge_adapter(bundle) do
    bundle
    |> judge_attrs()
    |> value_at(:kind, value_at(bundle.agent, :kind, "codex"))
    |> config_string()
    |> adapter_module()
  end

  @spec adapter_module(String.t()) :: module()
  defp adapter_module("claude"), do: Agent.Claude
  defp adapter_module("mock"), do: Agent.Mock
  defp adapter_module(_kind), do: Agent.Codex

  @spec judge_workspace(map()) :: Path.t()
  defp judge_workspace(bundle) do
    bundle
    |> Map.get(:workspace, %{})
    |> value_at(:root, ".")
    |> expand_path()
    |> Path.join("_judge")
  end

  @spec judge_config_hash(map()) :: ConfigHash.t()
  defp judge_config_hash(bundle) do
    bundle
    |> :erlang.term_to_binary()
    |> ConfigHash.from_iodata()
  end

  @spec judge_attrs(map()) :: map()
  defp judge_attrs(bundle), do: Map.get(bundle, :judge, %{})

  @spec report(OpenSleigh.JudgeClient.invoke_fun()) :: map()
  defp report(invoke_fun) do
    rows = GoldenSets.evaluate(invoke_fun)
    summary = GoldenSets.summary(rows)
    Map.put(summary, :rows, rows)
  end

  @spec emit_report(map(), keyword()) :: :ok
  defp emit_report(report, opts) do
    if Keyword.get(opts, :json, false) do
      Mix.shell().info(Jason.encode!(serialise_report(report)))
    else
      emit_text_report(report)
    end
  end

  @spec emit_text_report(map()) :: :ok
  defp emit_text_report(report) do
    Mix.shell().info("Open-Sleigh semantic gate report")
    Mix.shell().info("total: #{report.total}")
    Mix.shell().info("passed: #{report.passed}")
    Mix.shell().info("failed: #{report.failed}")

    report.rows
    |> Enum.each(&emit_row/1)
  end

  @spec emit_row(GoldenSets.row()) :: :ok
  defp emit_row(row) do
    Mix.shell().info(
      "- #{row.status} #{row.id} gate=#{row.gate} expected=#{row.expected} actual=#{row.actual}#{cl_suffix(row)}"
    )
  end

  @spec cl_suffix(GoldenSets.row()) :: String.t()
  defp cl_suffix(%{cl: cl}), do: " cl=#{cl}"
  defp cl_suffix(_row), do: ""

  @spec raise_on_failure(:ok, map()) :: :ok
  defp raise_on_failure(:ok, %{failed: 0}), do: :ok

  defp raise_on_failure(:ok, report) do
    Mix.raise("Open-Sleigh gate report failed: #{report.failed} failed row(s)")
  end

  @spec serialise_report(map()) :: map()
  defp serialise_report(report) do
    %{
      "total" => report.total,
      "passed" => report.passed,
      "failed" => report.failed,
      "rows" => Enum.map(report.rows, &serialise_row/1)
    }
  end

  @spec serialise_row(GoldenSets.row()) :: map()
  defp serialise_row(row) do
    row
    |> Enum.map(fn {key, value} -> {Atom.to_string(key), serialise_value(value)} end)
    |> Map.new()
  end

  @spec serialise_value(term()) :: term()
  defp serialise_value(value) when is_atom(value), do: Atom.to_string(value)
  defp serialise_value(value), do: value

  @spec value_at(term(), atom(), term()) :: term()
  defp value_at(%{} = map, key, fallback) do
    Map.get(map, Atom.to_string(key), Map.get(map, key, fallback))
  end

  defp value_at(_value, _key, fallback), do: fallback

  @spec config_string(term()) :: String.t()
  defp config_string(value) when is_binary(value), do: value
  defp config_string(value) when is_atom(value), do: Atom.to_string(value)
  defp config_string(value), do: to_string(value)

  @spec expand_path(String.t()) :: Path.t()
  defp expand_path("~/" <> rest) do
    System.user_home!()
    |> Path.join(rest)
    |> Path.expand()
  end

  defp expand_path(path), do: Path.expand(path)
end
