defmodule Mix.Tasks.OpenSleigh.Status do
  @shortdoc "Read the latest Open-Sleigh runtime status snapshot"

  @moduledoc """
  Read the latest Open-Sleigh runtime status snapshot written by
  `mix open_sleigh.start`.

      mix open_sleigh.status
      mix open_sleigh.status --path ~/.open-sleigh/status.json
      mix open_sleigh.status --json

  Options:

    * `--path` - status JSON path. Defaults to `OPEN_SLEIGH_STATUS_PATH`
      or `~/.open-sleigh/status.json`.
    * `--json` - print the stored JSON unchanged.
    * `--help` - print this help.
  """

  use Mix.Task

  @failure_display_limit 5
  @human_gate_display_limit 5
  @stale_after_seconds 15

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
        switches: [path: :string, json: :boolean, help: :boolean],
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
    opts
    |> status_path()
    |> read_status()
    |> emit_status(opts)
  end

  @spec status_path(keyword()) :: Path.t()
  defp status_path(opts) do
    opts
    |> Keyword.get(:path, System.get_env("OPEN_SLEIGH_STATUS_PATH"))
    |> present_string_or("~/.open-sleigh/status.json")
    |> expand_path()
  end

  @spec read_status(Path.t()) :: {:ok, binary(), map()} | {:error, term(), Path.t()}
  defp read_status(path) do
    path
    |> File.read()
    |> decode_status(path)
  end

  @spec decode_status({:ok, binary()} | {:error, term()}, Path.t()) ::
          {:ok, binary(), map()} | {:error, term(), Path.t()}
  defp decode_status({:ok, encoded}, path) do
    case Jason.decode(encoded) do
      {:ok, decoded} when is_map(decoded) -> {:ok, encoded, decoded}
      _ -> {:error, :status_malformed, path}
    end
  end

  defp decode_status({:error, reason}, path), do: {:error, reason, path}

  @spec emit_status({:ok, binary(), map()} | {:error, term(), Path.t()}, keyword()) :: :ok
  defp emit_status({:ok, encoded, _decoded}, opts) do
    if Keyword.get(opts, :json, false) do
      Mix.shell().info(String.trim_trailing(encoded))
    else
      emit_text_status(encoded)
    end
  end

  defp emit_status({:error, reason, path}, _opts) do
    Mix.raise("Open-Sleigh status unavailable at #{path}: #{inspect(reason)}")
  end

  @spec emit_text_status(binary()) :: :ok
  defp emit_text_status(encoded) do
    {:ok, status} = Jason.decode(encoded)
    age_seconds = snapshot_age_seconds(status)

    Mix.shell().info("Open-Sleigh status snapshot")
    Mix.shell().info("updated_at: #{Map.get(status, "updated_at", "unknown")}")
    Mix.shell().info("age_seconds: #{age_seconds_text(age_seconds)}")
    Mix.shell().info("stale: #{stale_text(age_seconds)}")
    Mix.shell().info("path: #{get_in(status, ["metadata", "config_path"]) || "unknown"}")
    Mix.shell().info("claimed: #{count_at(status, ["orchestrator", "claimed"])}")
    Mix.shell().info("running: #{count_at(status, ["orchestrator", "running"])}")
    Mix.shell().info("pending_human: #{pending_human_count(status)}")
    Mix.shell().info("retries: #{count_at(status, ["orchestrator", "retries"])}")
    Mix.shell().info("failures: #{count_at(status, ["failures"])}")
    Mix.shell().info("observations: #{count_at(status, ["observations"])}")
    emit_human_gate_lines(status)
    emit_failure_lines(status)
  end

  @spec snapshot_age_seconds(map()) :: non_neg_integer() | nil
  defp snapshot_age_seconds(status) do
    status
    |> Map.get("updated_at")
    |> parse_updated_at()
    |> diff_from_now()
  end

  @spec parse_updated_at(term()) :: DateTime.t() | nil
  defp parse_updated_at(updated_at) when is_binary(updated_at) do
    case DateTime.from_iso8601(updated_at) do
      {:ok, datetime, _offset} -> datetime
      {:error, _reason} -> nil
    end
  end

  defp parse_updated_at(_updated_at), do: nil

  @spec diff_from_now(DateTime.t() | nil) :: non_neg_integer() | nil
  defp diff_from_now(nil), do: nil

  defp diff_from_now(updated_at) do
    DateTime.utc_now()
    |> DateTime.diff(updated_at, :second)
    |> max(0)
  end

  @spec age_seconds_text(non_neg_integer() | nil) :: String.t()
  defp age_seconds_text(nil), do: "unknown"
  defp age_seconds_text(age_seconds), do: Integer.to_string(age_seconds)

  @spec stale_text(non_neg_integer() | nil) :: String.t()
  defp stale_text(nil), do: "unknown"
  defp stale_text(age_seconds) when age_seconds > @stale_after_seconds, do: "true"
  defp stale_text(_age_seconds), do: "false"

  @spec pending_human_count(map()) :: non_neg_integer()
  defp pending_human_count(%{"human_gates" => human_gates}) when is_list(human_gates) do
    length(human_gates)
  end

  defp pending_human_count(status) do
    count_at(status, ["orchestrator", "pending_human"])
  end

  @spec emit_human_gate_lines(map()) :: :ok
  defp emit_human_gate_lines(status) do
    status
    |> Map.get("human_gates", [])
    |> Enum.take(@human_gate_display_limit)
    |> emit_human_gate_section()
  end

  @spec emit_human_gate_section([map()]) :: :ok
  defp emit_human_gate_section([]), do: :ok

  defp emit_human_gate_section(human_gates) do
    Mix.shell().info("pending_human_details:")
    Enum.each(human_gates, &Mix.shell().info("- #{human_gate_summary(&1)}"))
    :ok
  end

  @spec human_gate_summary(map()) :: String.t()
  defp human_gate_summary(human_gate) do
    [
      labelled_value("ticket", Map.get(human_gate, "ticket_id")),
      labelled_value("session_id", Map.get(human_gate, "session_id")),
      labelled_value("gate", Map.get(human_gate, "gate_name")),
      labelled_value("requested_at", Map.get(human_gate, "requested_at"))
    ]
    |> Enum.reject(&is_nil/1)
    |> Enum.join(" ")
  end

  @spec emit_failure_lines(map()) :: :ok
  defp emit_failure_lines(status) do
    status
    |> Map.get("failures", [])
    |> Enum.take(@failure_display_limit)
    |> emit_failure_section()
  end

  @spec emit_failure_section([map()]) :: :ok
  defp emit_failure_section([]), do: :ok

  defp emit_failure_section(failures) do
    Mix.shell().info("recent_failures:")
    Enum.each(failures, &Mix.shell().info("- #{failure_summary(&1)}"))
    :ok
  end

  @spec failure_summary(map()) :: String.t()
  defp failure_summary(failure) do
    [
      Map.get(failure, "metric", "unknown"),
      labelled_value("reason", Map.get(failure, "reason")),
      labelled_value("ticket", Map.get(failure, "ticket")),
      labelled_value("session_id", Map.get(failure, "session_id")),
      labelled_value("phase", Map.get(failure, "phase")),
      labelled_value("hook", Map.get(failure, "hook")),
      labelled_value("policy", Map.get(failure, "policy")),
      labelled_value("target", Map.get(failure, "target"))
    ]
    |> Enum.reject(&is_nil/1)
    |> Enum.map(&to_string/1)
    |> Enum.join(" ")
  end

  @spec labelled_value(String.t(), term()) :: String.t() | nil
  defp labelled_value(_label, nil), do: nil

  defp labelled_value(label, value) do
    "#{label}=#{value}"
  end

  @spec count_at(map(), [String.t()]) :: non_neg_integer()
  defp count_at(status, path) do
    status
    |> get_in(path)
    |> count_value()
  end

  @spec count_value(term()) :: non_neg_integer()
  defp count_value(value) when is_list(value), do: length(value)
  defp count_value(value) when is_map(value), do: map_size(value)
  defp count_value(_value), do: 0

  @spec present_string_or(term(), String.t()) :: String.t()
  defp present_string_or(value, fallback) do
    value
    |> present_string()
    |> present_string_or_value(fallback)
  end

  @spec present_string_or_value(String.t() | nil, String.t()) :: String.t()
  defp present_string_or_value(nil, fallback), do: fallback
  defp present_string_or_value(value, _fallback), do: value

  @spec present_string(term()) :: String.t() | nil
  defp present_string(value) when is_binary(value) do
    value
    |> String.trim()
    |> blank_to_nil()
  end

  defp present_string(_value), do: nil

  @spec blank_to_nil(String.t()) :: String.t() | nil
  defp blank_to_nil(""), do: nil
  defp blank_to_nil(value), do: value

  @spec expand_path(String.t()) :: Path.t()
  defp expand_path("~/" <> rest) do
    System.user_home!()
    |> Path.join(rest)
    |> Path.expand()
  end

  defp expand_path(path), do: Path.expand(path)
end
