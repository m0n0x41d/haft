defmodule OpenSleigh.Agent.ToolRuntime do
  @moduledoc """
  Runtime executor for dynamic agent tools.

  Codex app-server issues `item/tool/call` requests with dynamic tool names and
  arbitrary JSON arguments. This module is the shared execution surface for the
  supported local tools (`read`, `write`, `edit`, `grep`, `bash`) and Haft MCP
  tools (`haft_query`, `haft_note`, `haft_problem`, `haft_decision`,
  `haft_refresh`, `haft_solution`).

  Scope policy:

  * Every dynamic tool call passes through `OpenSleigh.Agent.Adapter.ensure_in_scope/3`
    before this module performs IO.
  * `write` and `edit` are pre-authorized against WorkCommission path Scope.
  * `bash` is authorized only as the `:run_tests` action before execution. Arbitrary
    shell commands cannot be safely path-inspected here, so any filesystem mutation
    caused by shell remains subject to terminal diff validation as defense-in-depth.
  """

  alias OpenSleigh.{AdapterSession, EffectError, Haft.Client}
  alias OpenSleigh.Adapter.PathGuard
  alias OpenSleigh.Agent.Adapter, as: AgentAdapter

  @bash_timeout_ms 60_000
  @empty_guard %{forbidden_paths: [], forbidden_remote_substrings: []}

  @tool_names %{
    "bash" => :bash,
    "edit" => :edit,
    "grep" => :grep,
    "haft_decision" => :haft_decision,
    "haft_note" => :haft_note,
    "haft_problem" => :haft_problem,
    "haft_query" => :haft_query,
    "haft_refresh" => :haft_refresh,
    "haft_solution" => :haft_solution,
    "read" => :read,
    "write" => :write
  }

  @haft_actions %{
    "baseline" => :baseline,
    "characterize" => :characterize,
    "claim_for_preflight" => :claim_for_preflight,
    "compare" => :compare,
    "complete_or_block" => :complete_or_block,
    "create" => :create,
    "create_batch" => :create_batch,
    "create_from_decision" => :create_from_decision,
    "create_from_plan" => :create_from_plan,
    "decide" => :decide,
    "deprecate" => :deprecate,
    "evidence" => :evidence,
    "explore" => :explore,
    "frame" => :frame,
    "list_runnable" => :list_runnable,
    "measure" => :measure,
    "note" => :note,
    "record_preflight" => :record_preflight,
    "record_run_event" => :record_run_event,
    "related" => :related,
    "reopen" => :reopen,
    "requeue" => :requeue,
    "search" => :search,
    "select" => :select,
    "show" => :show,
    "start_after_preflight" => :start_after_preflight,
    "status" => :status,
    "supersede" => :supersede,
    "waive" => :waive
  }

  @type opts :: [
          haft_invoker: Client.invoke_fun(),
          bash_timeout_ms: pos_integer()
        ]

  @type execution :: %{
          required(:tool) => atom(),
          required(:output) => String.t()
        }

  @spec execute(String.t() | atom(), term(), AdapterSession.t()) ::
          {:ok, execution()} | {:error, EffectError.t()}
  def execute(tool, args, %AdapterSession{} = session) do
    execute(tool, args, session, [])
  end

  @spec execute(String.t() | atom(), term(), AdapterSession.t(), opts()) ::
          {:ok, execution()} | {:error, EffectError.t()}
  def execute(tool, args, %AdapterSession{} = session, opts) do
    with {:ok, tool} <- normalize_tool(tool),
         {:ok, args} <- normalize_args(args),
         :ok <- AgentAdapter.ensure_in_scope(session, tool, args) do
      execute_tool(tool, args, session, opts)
    end
  end

  @spec dynamic_response(String.t() | atom(), term(), AdapterSession.t()) :: {:ok, map()}
  def dynamic_response(tool, args, %AdapterSession{} = session) do
    dynamic_response(tool, args, session, [])
  end

  @spec dynamic_response(String.t() | atom(), term(), AdapterSession.t(), opts()) :: {:ok, map()}
  def dynamic_response(tool, args, %AdapterSession{} = session, opts) do
    tool
    |> execute(args, session, opts)
    |> dynamic_response_result()
  end

  @spec dynamic_response_result({:ok, execution()} | {:error, EffectError.t()}) :: {:ok, map()}
  defp dynamic_response_result({:ok, execution}) do
    execution
    |> Map.fetch!(:output)
    |> success_response()
    |> then(&{:ok, &1})
  end

  defp dynamic_response_result({:error, reason}) do
    reason
    |> error_text()
    |> failure_response()
    |> then(&{:ok, &1})
  end

  @spec execute_tool(atom(), map(), AdapterSession.t(), opts()) ::
          {:ok, execution()} | {:error, EffectError.t()}
  defp execute_tool(:read, args, session, _opts) do
    session
    |> target_path(args)
    |> read_target()
    |> execution_result(:read)
  end

  defp execute_tool(:write, args, session, _opts) do
    with {:ok, path} <- target_path(session, args),
         {:ok, content} <- write_content(args),
         :ok <- ensure_parent_directory(path),
         :ok <- write_file(path, content) do
      {:ok, %{tool: :write, output: write_summary(path, content)}}
    end
  end

  defp execute_tool(:edit, args, session, _opts) do
    with {:ok, path} <- target_path(session, args),
         {:ok, replacements} <- replacement_pairs(args),
         {:ok, original} <- read_file(path),
         {:ok, updated} <- apply_replacements(original, replacements),
         :ok <- write_file(path, updated) do
      {:ok, %{tool: :edit, output: edit_summary(path, replacements)}}
    end
  end

  defp execute_tool(:grep, args, session, _opts) do
    with {:ok, pattern} <- grep_pattern(args),
         {:ok, search_path} <- grep_target_path(session, args),
         {:ok, output} <- run_grep(pattern, search_path, session.workspace_path) do
      {:ok, %{tool: :grep, output: grep_output(output)}}
    end
  end

  defp execute_tool(:bash, args, session, opts) do
    with {:ok, command} <- bash_command(args),
         {:ok, output} <- run_bash(command, session.workspace_path, bash_timeout_ms(opts)) do
      {:ok, %{tool: :bash, output: output}}
    end
  end

  defp execute_tool(tool, args, session, opts)
       when tool in [
              :haft_decision,
              :haft_note,
              :haft_problem,
              :haft_query,
              :haft_refresh,
              :haft_solution
            ] do
    with {:ok, action} <- haft_action(args),
         {:ok, invoke_fun} <- haft_invoker(opts),
         {:ok, output} <- Client.call_tool(session, tool, action, args, invoke_fun) do
      {:ok, %{tool: tool, output: output}}
    end
  end

  @spec normalize_tool(String.t() | atom()) :: {:ok, atom()} | {:error, EffectError.t()}
  defp normalize_tool(tool) when is_atom(tool) do
    if tool in Map.values(@tool_names) do
      {:ok, tool}
    else
      {:error, :tool_unknown_to_adapter}
    end
  end

  defp normalize_tool(tool) when is_binary(tool) do
    tool
    |> String.trim()
    |> then(&Map.fetch(@tool_names, &1))
    |> normalize_tool_lookup()
  end

  defp normalize_tool(_tool), do: {:error, :tool_unknown_to_adapter}

  @spec normalize_tool_lookup({:ok, atom()} | :error) ::
          {:ok, atom()} | {:error, EffectError.t()}
  defp normalize_tool_lookup({:ok, tool}), do: {:ok, tool}
  defp normalize_tool_lookup(:error), do: {:error, :tool_unknown_to_adapter}

  @spec normalize_args(term()) :: {:ok, map()} | {:error, EffectError.t()}
  defp normalize_args(args) when is_map(args) do
    args
    |> Enum.map(&string_key_value/1)
    |> Map.new()
    |> then(&{:ok, &1})
  end

  defp normalize_args(_args), do: {:error, :tool_arg_invalid}

  @spec string_key_value({term(), term()}) :: {String.t(), term()}
  defp string_key_value({key, value}) when is_atom(key), do: {Atom.to_string(key), value}
  defp string_key_value({key, value}) when is_binary(key), do: {key, value}
  defp string_key_value({key, value}), do: {to_string(key), value}

  @spec target_path(AdapterSession.t(), map()) ::
          {:ok, %{absolute: Path.t(), relative: Path.t()}} | {:error, EffectError.t()}
  defp target_path(%AdapterSession{} = session, args) do
    args
    |> path_argument()
    |> target_path_result(session)
  end

  @spec path_argument(map()) :: String.t() | nil
  defp path_argument(args) do
    args
    |> first_present(["path", "file", "filepath", "target", "filename"])
    |> binary_value()
  end

  @spec target_path_result(String.t() | nil, AdapterSession.t()) ::
          {:ok, %{absolute: Path.t(), relative: Path.t()}} | {:error, EffectError.t()}
  defp target_path_result(nil, _session), do: {:error, :tool_arg_invalid}

  defp target_path_result(path, %AdapterSession{} = session) do
    session.workspace_path
    |> PathGuard.relative_to_workspace(path, @empty_guard)
    |> workspace_relative_result(session.workspace_path)
  end

  @spec workspace_relative_result(
          {:ok, Path.t()} | {:error, EffectError.t()},
          Path.t()
        ) :: {:ok, %{absolute: Path.t(), relative: Path.t()}} | {:error, EffectError.t()}
  defp workspace_relative_result({:ok, relative}, workspace_path) do
    {:ok, %{absolute: Path.join(workspace_path, relative), relative: relative}}
  end

  defp workspace_relative_result({:error, _reason} = error, _workspace_path), do: error

  @spec read_target({:ok, map()} | {:error, EffectError.t()}) ::
          {:ok, String.t()} | {:error, EffectError.t()}
  defp read_target({:ok, path}) do
    path
    |> read_file()
    |> read_target_result(path.relative)
  end

  defp read_target({:error, _reason} = error), do: error

  @spec read_target_result({:ok, String.t()} | {:error, EffectError.t()}, Path.t()) ::
          {:ok, String.t()} | {:error, EffectError.t()}
  defp read_target_result({:ok, content}, relative) do
    relative
    |> read_summary(content)
    |> then(&{:ok, &1})
  end

  defp read_target_result({:error, _reason} = error, _relative), do: error

  @spec read_file(%{absolute: Path.t(), relative: Path.t()}) ::
          {:ok, String.t()} | {:error, EffectError.t()}
  defp read_file(path) do
    path.absolute
    |> File.read()
    |> file_read_result()
  end

  @spec file_read_result({:ok, binary()} | {:error, term()}) ::
          {:ok, String.t()} | {:error, EffectError.t()}
  defp file_read_result({:ok, content}), do: {:ok, content}

  defp file_read_result({:error, _reason}), do: {:error, :tool_execution_failed}

  @spec write_content(map()) :: {:ok, String.t()} | {:error, EffectError.t()}
  defp write_content(args) do
    args
    |> first_present(["content", "text", "body", "new_content"])
    |> binary_result()
  end

  @spec ensure_parent_directory(%{absolute: Path.t()}) :: :ok | {:error, EffectError.t()}
  defp ensure_parent_directory(path) do
    path.absolute
    |> Path.dirname()
    |> File.mkdir_p()
    |> mkdir_result()
  end

  @spec mkdir_result(:ok | {:error, term()}) :: :ok | {:error, EffectError.t()}
  defp mkdir_result(:ok), do: :ok
  defp mkdir_result({:error, _reason}), do: {:error, :tool_execution_failed}

  @spec write_file(%{absolute: Path.t()}, String.t()) :: :ok | {:error, EffectError.t()}
  defp write_file(path, content) do
    path.absolute
    |> File.write(content)
    |> write_file_result()
  end

  @spec write_file_result(:ok | {:error, term()}) :: :ok | {:error, EffectError.t()}
  defp write_file_result(:ok), do: :ok
  defp write_file_result({:error, _reason}), do: {:error, :tool_execution_failed}

  @spec replacement_pairs(map()) :: {:ok, [{String.t(), String.t()}]} | {:error, EffectError.t()}
  defp replacement_pairs(args) do
    args
    |> Map.get("replacements")
    |> replacement_pairs_result(args)
  end

  @spec replacement_pairs_result(term(), map()) ::
          {:ok, [{String.t(), String.t()}]} | {:error, EffectError.t()}
  defp replacement_pairs_result(replacements, _args) when is_list(replacements) do
    replacements
    |> Enum.map(&replacement_pair/1)
    |> Enum.reduce_while({:ok, []}, &replacement_pair_reduce/2)
    |> reverse_pairs()
  end

  defp replacement_pairs_result(_replacements, args) do
    with {:ok, old_text} <- old_text(args),
         {:ok, new_text} <- new_text(args) do
      {:ok, [{old_text, new_text}]}
    end
  end

  @spec replacement_pair(term()) :: {:ok, {String.t(), String.t()}} | {:error, EffectError.t()}
  defp replacement_pair(replacement) when is_map(replacement) do
    with {:ok, old_text} <- replacement |> Map.new() |> old_text(),
         {:ok, new_text} <- replacement |> Map.new() |> new_text() do
      {:ok, {old_text, new_text}}
    end
  end

  defp replacement_pair(_replacement), do: {:error, :tool_arg_invalid}

  @spec replacement_pair_reduce(
          {:ok, {String.t(), String.t()}} | {:error, EffectError.t()},
          {:ok, [{String.t(), String.t()}]} | {:error, EffectError.t()}
        ) :: {:cont, {:ok, [{String.t(), String.t()}]}} | {:halt, {:error, EffectError.t()}}
  defp replacement_pair_reduce({:ok, pair}, {:ok, pairs}), do: {:cont, {:ok, [pair | pairs]}}
  defp replacement_pair_reduce({:error, reason}, _acc), do: {:halt, {:error, reason}}

  @spec reverse_pairs({:ok, [{String.t(), String.t()}]} | {:error, EffectError.t()}) ::
          {:ok, [{String.t(), String.t()}]} | {:error, EffectError.t()}
  defp reverse_pairs({:ok, pairs}), do: {:ok, Enum.reverse(pairs)}
  defp reverse_pairs({:error, _reason} = error), do: error

  @spec old_text(map()) :: {:ok, String.t()} | {:error, EffectError.t()}
  defp old_text(args) do
    args
    |> first_present(["old", "old_text", "oldText", "find", "search"])
    |> binary_result()
  end

  @spec new_text(map()) :: {:ok, String.t()} | {:error, EffectError.t()}
  defp new_text(args) do
    args
    |> first_present(["new", "new_text", "newText", "replace", "replacement"])
    |> binary_result()
  end

  @spec apply_replacements(String.t(), [{String.t(), String.t()}]) ::
          {:ok, String.t()} | {:error, EffectError.t()}
  defp apply_replacements(content, replacements) do
    replacements
    |> Enum.reduce_while({:ok, content}, &apply_one_replacement/2)
  end

  @spec apply_one_replacement(
          {String.t(), String.t()},
          {:ok, String.t()} | {:error, EffectError.t()}
        ) :: {:cont, {:ok, String.t()}} | {:halt, {:error, EffectError.t()}}
  defp apply_one_replacement({old_text, new_text}, {:ok, content}) do
    content
    |> String.contains?(old_text)
    |> replacement_content_result(content, old_text, new_text)
  end

  @spec replacement_content_result(boolean(), String.t(), String.t(), String.t()) ::
          {:cont, {:ok, String.t()}} | {:halt, {:error, EffectError.t()}}
  defp replacement_content_result(true, content, old_text, new_text) do
    content
    |> String.replace(old_text, new_text, global: false)
    |> then(&{:cont, {:ok, &1}})
  end

  defp replacement_content_result(false, _content, _old_text, _new_text) do
    {:halt, {:error, :tool_execution_failed}}
  end

  @spec grep_pattern(map()) :: {:ok, String.t()} | {:error, EffectError.t()}
  defp grep_pattern(args) do
    args
    |> first_present(["pattern", "query", "regex", "text"])
    |> binary_result()
  end

  @spec grep_target_path(AdapterSession.t(), map()) :: {:ok, Path.t()} | {:error, EffectError.t()}
  defp grep_target_path(%AdapterSession{} = session, args) do
    args
    |> path_argument()
    |> grep_target_path_result(session)
  end

  @spec grep_target_path_result(String.t() | nil, AdapterSession.t()) ::
          {:ok, Path.t()} | {:error, EffectError.t()}
  defp grep_target_path_result(nil, session), do: {:ok, session.workspace_path}

  defp grep_target_path_result(path, %AdapterSession{} = session) do
    session
    |> target_path(%{"path" => path})
    |> grep_search_path_result()
  end

  @spec grep_search_path_result({:ok, map()} | {:error, EffectError.t()}) ::
          {:ok, Path.t()} | {:error, EffectError.t()}
  defp grep_search_path_result({:ok, path}), do: {:ok, path.absolute}
  defp grep_search_path_result({:error, _reason} = error), do: error

  @spec run_grep(String.t(), Path.t(), Path.t()) :: {:ok, String.t()} | {:error, EffectError.t()}
  defp run_grep(pattern, search_path, workspace_path) do
    "rg"
    |> System.find_executable()
    |> grep_command_result(pattern, search_path, workspace_path)
  end

  @spec grep_command_result(String.t() | nil, String.t(), Path.t(), Path.t()) ::
          {:ok, String.t()} | {:error, EffectError.t()}
  defp grep_command_result(nil, pattern, search_path, workspace_path) do
    run_grep_with("grep", ["-R", "-n", pattern, search_path], workspace_path)
  end

  defp grep_command_result(_rg, pattern, search_path, workspace_path) do
    run_grep_with(
      "rg",
      ["--line-number", "--no-heading", "--color", "never", pattern, search_path],
      workspace_path
    )
  end

  @spec run_grep_with(String.t(), [String.t()], Path.t()) ::
          {:ok, String.t()} | {:error, EffectError.t()}
  defp run_grep_with(command, args, workspace_path) do
    command
    |> System.cmd(args, cd: workspace_path, stderr_to_stdout: true)
    |> grep_output_result()
  rescue
    _ -> {:error, :tool_execution_failed}
  end

  @spec grep_output_result({String.t(), non_neg_integer()}) ::
          {:ok, String.t()} | {:error, EffectError.t()}
  defp grep_output_result({output, 0}), do: {:ok, output}
  defp grep_output_result({_output, 1}), do: {:ok, ""}
  defp grep_output_result({_output, _status}), do: {:error, :tool_execution_failed}

  @spec grep_output(String.t()) :: String.t()
  defp grep_output(""), do: "No matches."
  defp grep_output(output), do: output

  @spec bash_command(map()) :: {:ok, String.t()} | {:error, EffectError.t()}
  defp bash_command(args) do
    args
    |> first_present(["cmd", "command", "script", "bash"])
    |> binary_result()
  end

  @spec run_bash(String.t(), Path.t(), pos_integer()) ::
          {:ok, String.t()} | {:error, EffectError.t()}
  defp run_bash(command, workspace_path, timeout_ms) do
    task =
      Task.async(fn ->
        System.cmd("bash", ["-lc", command], cd: workspace_path, stderr_to_stdout: true)
      end)

    task
    |> bash_task_result(timeout_ms)
  rescue
    _ -> {:error, :tool_execution_failed}
  end

  @spec bash_task_result(Task.t(), pos_integer()) :: {:ok, String.t()} | {:error, EffectError.t()}
  defp bash_task_result(task, timeout_ms) do
    task
    |> Task.yield(timeout_ms)
    |> bash_yield_result(task)
  end

  @spec bash_yield_result({:ok, {String.t(), integer()}} | nil, Task.t()) ::
          {:ok, String.t()} | {:error, EffectError.t()}
  defp bash_yield_result({:ok, {output, status}}, _task) do
    output
    |> bash_output(status)
    |> then(&{:ok, &1})
  end

  defp bash_yield_result(nil, task) do
    _ = Task.shutdown(task, :brutal_kill)
    {:error, :tool_execution_failed}
  end

  @spec bash_output(String.t(), integer()) :: String.t()
  defp bash_output(output, status) do
    [
      "exit_status: #{status}",
      String.trim_trailing(output)
    ]
    |> Enum.reject(&(&1 == ""))
    |> Enum.join("\n")
  end

  @spec haft_action(map()) :: {:ok, atom()} | {:error, EffectError.t()}
  defp haft_action(args) do
    args
    |> Map.get("action")
    |> haft_action_result()
  end

  @spec haft_action_result(term()) :: {:ok, atom()} | {:error, EffectError.t()}
  defp haft_action_result(action) when is_atom(action), do: {:ok, action}

  defp haft_action_result(action) when is_binary(action) do
    action
    |> String.trim()
    |> then(&Map.fetch(@haft_actions, &1))
    |> haft_action_lookup()
  end

  defp haft_action_result(_action), do: {:error, :tool_arg_invalid}

  @spec haft_action_lookup({:ok, atom()} | :error) ::
          {:ok, atom()} | {:error, EffectError.t()}
  defp haft_action_lookup({:ok, action}), do: {:ok, action}
  defp haft_action_lookup(:error), do: {:error, :tool_arg_invalid}

  @spec haft_invoker(opts()) :: {:ok, Client.invoke_fun()} | {:error, EffectError.t()}
  defp haft_invoker(opts) do
    opts
    |> Keyword.get(:haft_invoker)
    |> haft_invoker_result()
  end

  @spec haft_invoker_result(term()) :: {:ok, Client.invoke_fun()} | {:error, EffectError.t()}
  defp haft_invoker_result(invoke_fun) when is_function(invoke_fun, 1), do: {:ok, invoke_fun}
  defp haft_invoker_result(_invoke_fun), do: {:error, :haft_unavailable}

  @spec bash_timeout_ms(opts()) :: pos_integer()
  defp bash_timeout_ms(opts) do
    opts
    |> Keyword.get(:bash_timeout_ms, @bash_timeout_ms)
    |> bash_timeout_value()
  end

  @spec bash_timeout_value(term()) :: pos_integer()
  defp bash_timeout_value(value) when is_integer(value) and value > 0, do: value
  defp bash_timeout_value(_value), do: @bash_timeout_ms

  @spec binary_result(term()) :: {:ok, String.t()} | {:error, EffectError.t()}
  defp binary_result(value) when is_binary(value) and value != "", do: {:ok, value}
  defp binary_result(_value), do: {:error, :tool_arg_invalid}

  @spec binary_value(term()) :: String.t() | nil
  defp binary_value(value) when is_binary(value) and value != "", do: value
  defp binary_value(_value), do: nil

  @spec first_present(map(), [String.t()]) :: term()
  defp first_present(args, keys) do
    Enum.find_value(keys, &Map.get(args, &1))
  end

  @spec execution_result({:ok, String.t()} | {:error, EffectError.t()}, atom()) ::
          {:ok, execution()} | {:error, EffectError.t()}
  defp execution_result({:ok, output}, tool), do: {:ok, %{tool: tool, output: output}}
  defp execution_result({:error, _reason} = error, _tool), do: error

  @spec success_response(String.t()) :: map()
  defp success_response(text) do
    %{
      "success" => true,
      "contentItems" => [
        %{
          "type" => "inputText",
          "text" => text
        }
      ]
    }
  end

  @spec failure_response(String.t()) :: map()
  defp failure_response(text) do
    %{
      "success" => false,
      "contentItems" => [
        %{
          "type" => "inputText",
          "text" => text
        }
      ]
    }
  end

  @spec error_text(EffectError.t()) :: String.t()
  defp error_text(reason), do: "tool_error: #{reason}"

  @spec read_summary(Path.t(), String.t()) :: String.t()
  defp read_summary(relative, content) do
    [
      "path: #{relative}",
      content
    ]
    |> Enum.join("\n")
    |> String.slice(0, 100_000)
  end

  @spec write_summary(%{relative: Path.t()}, String.t()) :: String.t()
  defp write_summary(path, content) do
    "wrote #{path.relative} (#{byte_size(content)} bytes)"
  end

  @spec edit_summary(%{relative: Path.t()}, [{String.t(), String.t()}]) :: String.t()
  defp edit_summary(path, replacements) do
    "edited #{path.relative} (#{length(replacements)} replacement(s))"
  end
end
