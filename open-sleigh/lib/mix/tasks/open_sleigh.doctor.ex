defmodule Mix.Tasks.OpenSleigh.Doctor do
  @shortdoc "Check whether the real Open-Sleigh runtime can start"

  @moduledoc """
  Check whether the real Open-Sleigh runtime has the external pieces it needs.

      mix open_sleigh.doctor
      mix open_sleigh.doctor --path=sleigh.md
      mix open_sleigh.doctor --path=sleigh.md --json
      mix open_sleigh.doctor --path=sleigh.md --mock-haft

  Options:

    * `--path` - config file path. Defaults to `sleigh.md`.
    * `--json` - print a machine-readable report.
    * `--mock-haft` - skip the real Haft command check for local harness runs.
    * `--help` - print this help.
  """

  use Mix.Task

  alias OpenSleigh.Judge.GoldenSets
  alias OpenSleigh.Sleigh.{Compiler, CompilerError}

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
        switches: [path: :string, json: :boolean, mock_haft: :boolean, help: :boolean],
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
      |> doctor_report()

    report
    |> emit_report(opts)
    |> raise_on_failure(report)
  end

  @spec config_path(keyword()) :: Path.t()
  defp config_path(opts), do: Keyword.get(opts, :path, "sleigh.md")

  @spec doctor_report(keyword()) :: map()
  defp doctor_report(opts) do
    path = config_path(opts)

    case compile_config(path) do
      {:ok, bundle} -> runtime_report(path, bundle, opts)
      {:error, reason} -> failed_config_report(path, reason)
    end
  end

  @spec compile_config(Path.t()) :: {:ok, map()} | {:error, term()}
  defp compile_config(path) do
    path
    |> File.read()
    |> compile_source()
  end

  @spec compile_source({:ok, binary()} | {:error, term()}) :: {:ok, map()} | {:error, term()}
  defp compile_source({:ok, source}), do: Compiler.compile(source)
  defp compile_source({:error, reason}), do: {:error, {:config_read_failed, reason}}

  @spec failed_config_report(Path.t(), term()) :: map()
  defp failed_config_report(path, reason) do
    checks = config_failure_checks(path, reason)

    %{
      path: path,
      ready: false,
      errors: length(checks),
      warnings: 0,
      checks: checks
    }
  end

  @spec config_failure_checks(Path.t(), term()) :: [map()]
  defp config_failure_checks(_path, errors) when is_list(errors) do
    errors
    |> Enum.map(&compiler_error_check/1)
  end

  defp config_failure_checks(path, reason) do
    [
      error_check("config.compile", "Failed to compile #{path}: #{inspect(reason)}")
    ]
  end

  @spec compiler_error_check(CompilerError.t()) :: map()
  defp compiler_error_check(error) do
    report = CompilerError.report(error)

    error_check(
      "config.schema",
      CompilerError.message(error),
      %{
        field_path: report.path,
        expected: report.expected,
        actual: report.actual,
        hint: report.hint
      }
    )
  end

  @spec runtime_report(Path.t(), map(), keyword()) :: map()
  defp runtime_report(path, bundle, opts) do
    checks =
      [
        ok_check("config.compile", "Compiled #{path}")
      ]
      |> Kernel.++(work_source_checks(bundle))
      |> Kernel.++([
        command_check("codex.command", codex_command(bundle)),
        haft_command_check(bundle, opts)
      ])
      |> Kernel.++(hook_checks(bundle))
      |> Kernel.++(workspace_checks(bundle))
      |> Kernel.++(publication_checks(bundle))
      |> Kernel.++(semantic_gate_checks(bundle))

    %{
      path: path,
      ready: Enum.all?(checks, &(&1.status != :error)),
      errors: Enum.count(checks, &(&1.status == :error)),
      warnings: Enum.count(checks, &(&1.status == :warning)),
      checks: checks
    }
  end

  @spec work_source_checks(map()) :: [map()]
  defp work_source_checks(bundle) do
    bundle
    |> commission_source_kind()
    |> work_source_checks_for_kind(bundle)
  end

  @spec work_source_checks_for_kind(String.t() | nil, map()) :: [map()]
  defp work_source_checks_for_kind("local", bundle), do: [local_commission_source_check(bundle)]
  defp work_source_checks_for_kind("haft", bundle), do: [haft_commission_source_check(bundle)]
  defp work_source_checks_for_kind(_kind, bundle), do: linear_source_checks(bundle)

  @spec haft_commission_source_check(map()) :: map()
  defp haft_commission_source_check(bundle) do
    bundle.commission_source
    |> value_at(:selector, "runnable")
    |> present_string()
    |> haft_commission_selector_result()
  end

  @spec haft_commission_selector_result(String.t() | nil) :: map()
  defp haft_commission_selector_result(nil) do
    error_check("commission_source.selector", "Haft commission selector is missing")
  end

  defp haft_commission_selector_result(selector) do
    ok_check("commission_source.selector", "Haft commission selector: #{selector}")
  end

  @spec local_commission_source_check(map()) :: map()
  defp local_commission_source_check(bundle) do
    bundle
    |> local_commission_fixture_path()
    |> local_commission_fixture_result()
  end

  @spec local_commission_fixture_path(map()) :: String.t() | nil
  defp local_commission_fixture_path(bundle) do
    bundle.commission_source
    |> value_at(:fixture_path, nil)
    |> present_string()
  end

  @spec local_commission_fixture_result(String.t() | nil) :: map()
  defp local_commission_fixture_result(nil) do
    error_check("commission_source.fixture_path", "Local commission fixture_path is missing")
  end

  defp local_commission_fixture_result(path) do
    path
    |> Path.expand()
    |> File.exists?()
    |> local_commission_fixture_exists(path)
  end

  @spec local_commission_fixture_exists(boolean(), String.t()) :: map()
  defp local_commission_fixture_exists(true, path) do
    ok_check("commission_source.fixture_path", "Local commission fixture exists: #{path}")
  end

  defp local_commission_fixture_exists(false, path) do
    error_check("commission_source.fixture_path", "Local commission fixture is missing: #{path}")
  end

  @spec linear_source_checks(map()) :: [map()]
  defp linear_source_checks(bundle) do
    [
      linear_api_key_check(bundle),
      linear_project_check(bundle),
      linear_states_check(bundle)
    ]
  end

  @spec commission_source_kind(map()) :: String.t() | nil
  defp commission_source_kind(bundle) do
    bundle.commission_source
    |> value_at(:kind, nil)
    |> config_string_or_nil()
  end

  @spec linear_api_key_check(map()) :: map()
  defp linear_api_key_check(bundle) do
    bundle.tracker
    |> value_at(:kind, "linear")
    |> config_string()
    |> linear_api_key_check_for_kind()
  end

  @spec linear_api_key_check_for_kind(String.t()) :: map()
  defp linear_api_key_check_for_kind("linear") do
    System.get_env("LINEAR_API_KEY")
    |> present_string()
    |> env_check("linear.api_key", "LINEAR_API_KEY is present", "LINEAR_API_KEY is missing")
  end

  defp linear_api_key_check_for_kind(kind) do
    warning_check("linear.api_key", "Tracker kind #{kind} is not validated by this doctor yet")
  end

  @spec linear_project_check(map()) :: map()
  defp linear_project_check(bundle) do
    bundle.tracker
    |> value_at(
      :project_slug,
      value_at(bundle.tracker, :team, System.get_env("LINEAR_PROJECT_SLUG"))
    )
    |> present_string()
    |> env_check(
      "linear.project",
      "Linear project slug is configured",
      "Linear project_slug/team is missing"
    )
  end

  @spec linear_states_check(map()) :: map()
  defp linear_states_check(bundle) do
    bundle.tracker
    |> value_at(:active_states, [])
    |> active_states_check()
  end

  @spec active_states_check(term()) :: map()
  defp active_states_check(states) when is_list(states) do
    states
    |> Enum.map(&config_string/1)
    |> Enum.map(&String.trim/1)
    |> Enum.reject(&(&1 == ""))
    |> active_states_result()
  end

  defp active_states_check(_states) do
    error_check("linear.active_states", "Linear active_states must be a non-empty list")
  end

  @spec active_states_result([String.t()]) :: map()
  defp active_states_result([]) do
    error_check("linear.active_states", "Linear active_states is empty")
  end

  defp active_states_result(states) do
    ok_check("linear.active_states", "Linear active states: #{Enum.join(states, ", ")}")
  end

  @spec codex_command(map()) :: String.t()
  defp codex_command(bundle) do
    bundle.codex
    |> value_at(:command, value_at(bundle.agent, :command, "codex app-server"))
    |> config_string()
  end

  @spec haft_command(map()) :: String.t()
  defp haft_command(bundle) do
    bundle.haft
    |> value_at(:command, "haft serve")
    |> config_string()
  end

  @spec haft_command_check(map(), keyword()) :: map()
  defp haft_command_check(bundle, opts) do
    if Keyword.get(opts, :mock_haft, false) do
      ok_check("haft.command", "Haft command check skipped by --mock-haft")
    else
      command_check("haft.command", haft_command(bundle))
    end
  end

  @spec command_check(String.t(), String.t()) :: map()
  defp command_check(name, command) do
    command
    |> command_executable()
    |> executable_check(name, command)
  end

  @spec command_executable(String.t()) :: String.t()
  defp command_executable(command) do
    command
    |> OptionParser.split()
    |> first_command_token()
  end

  @spec first_command_token([String.t()]) :: String.t()
  defp first_command_token(["env" | rest]), do: first_command_token(rest)
  defp first_command_token(["-" <> _token | rest]), do: first_command_token(rest)

  defp first_command_token([token | rest]) do
    if String.contains?(token, "=") do
      first_command_token(rest)
    else
      token
    end
  end

  defp first_command_token([]), do: ""

  @spec executable_check(String.t(), String.t(), String.t()) :: map()
  defp executable_check("", name, command) do
    error_check(name, "Command is empty: #{command}")
  end

  defp executable_check(executable, name, command) do
    executable
    |> System.find_executable()
    |> command_lookup_result(name, command, executable)
  end

  @spec command_lookup_result(String.t() | nil, String.t(), String.t(), String.t()) :: map()
  defp command_lookup_result(nil, name, command, executable) do
    error_check(name, "Executable #{executable} is missing for command: #{command}")
  end

  defp command_lookup_result(_path, name, command, _executable) do
    ok_check(name, "Command is available: #{command}")
  end

  @spec hook_checks(map()) :: [map()]
  defp hook_checks(bundle) do
    [
      repo_url_check(bundle),
      repo_url_format_check(bundle),
      git_check(bundle),
      hook_failure_policy_check(bundle)
    ]
    |> Enum.reject(&is_nil/1)
  end

  @spec repo_url_check(map()) :: map() | nil
  defp repo_url_check(bundle) do
    if hooks_contain?(bundle, "$REPO_URL") do
      System.get_env("REPO_URL")
      |> present_string()
      |> env_check("hooks.repo_url", "REPO_URL is present", "REPO_URL is missing")
    end
  end

  @spec repo_url_format_check(map()) :: map() | nil
  defp repo_url_format_check(bundle) do
    if hooks_contain?(bundle, "$REPO_URL") do
      System.get_env("REPO_URL")
      |> present_string()
      |> repo_url_format_result()
    end
  end

  @spec repo_url_format_result(String.t() | nil) :: map() | nil
  defp repo_url_format_result(nil), do: nil

  defp repo_url_format_result(url) do
    if repo_url_valid?(url) do
      ok_check("hooks.repo_url_format", "REPO_URL looks like a supported git remote")
    else
      error_check(
        "hooks.repo_url_format",
        "REPO_URL must be an SSH/HTTPS/file git remote or local repository path"
      )
    end
  end

  @spec repo_url_valid?(String.t()) :: boolean()
  defp repo_url_valid?(url) do
    url_without_whitespace?(url) and
      [
        fn -> uri_git_url?(url) end,
        fn -> scp_git_url?(url) end,
        fn -> local_git_path?(url) end
      ]
      |> Enum.any?(& &1.())
  end

  @spec url_without_whitespace?(String.t()) :: boolean()
  defp url_without_whitespace?(url) do
    not Regex.match?(~r/\s/, url)
  end

  @spec uri_git_url?(String.t()) :: boolean()
  defp uri_git_url?(url) do
    case URI.parse(url) do
      %URI{scheme: scheme, host: host, path: path}
      when scheme in ["http", "https", "ssh", "git"] ->
        present_string(host) != nil and present_string(path) != nil

      %URI{scheme: "file", path: path} ->
        present_string(path) != nil

      _uri ->
        false
    end
  end

  @spec scp_git_url?(String.t()) :: boolean()
  defp scp_git_url?(url) do
    Regex.match?(~r/^[^@\s]+@[^:\s]+:[^\s]+$/, url)
  end

  @spec local_git_path?(String.t()) :: boolean()
  defp local_git_path?(url) do
    [
      fn -> String.starts_with?(url, ["/", "./", "../", "~/"]) end,
      fn -> File.exists?(Path.expand(url)) end,
      fn -> String.ends_with?(url, ".git") and not String.contains?(url, " ") end
    ]
    |> Enum.any?(& &1.())
  end

  @spec git_check(map()) :: map() | nil
  defp git_check(bundle) do
    if hooks_contain?(bundle, "git ") do
      "git"
      |> System.find_executable()
      |> command_lookup_result("hooks.git", "git", "git")
    end
  end

  @spec hooks_contain?(map(), String.t()) :: boolean()
  defp hooks_contain?(bundle, needle) do
    bundle.hooks
    |> Map.values()
    |> Enum.filter(&is_binary/1)
    |> Enum.any?(&String.contains?(&1, needle))
  end

  @spec hook_failure_policy_check(map()) :: map()
  defp hook_failure_policy_check(bundle) do
    bundle.hooks
    |> value_at(:failure_policy, %{})
    |> hook_failure_policy_result()
  end

  @spec hook_failure_policy_result(term()) :: map()
  defp hook_failure_policy_result(policy) when policy == %{} do
    ok_check("hooks.failure_policy", "Hook failure policy uses defaults")
  end

  defp hook_failure_policy_result(%{} = policy) do
    policy
    |> hook_failure_policy_errors()
    |> hook_failure_policy_errors_result()
  end

  defp hook_failure_policy_result(_policy) do
    error_check("hooks.failure_policy", "hooks.failure_policy must be a map")
  end

  @spec hook_failure_policy_errors(map()) :: [String.t()]
  defp hook_failure_policy_errors(policy) do
    policy
    |> Enum.flat_map(&hook_failure_policy_pair_errors/1)
  end

  @spec hook_failure_policy_pair_errors({term(), term()}) :: [String.t()]
  defp hook_failure_policy_pair_errors({hook_name, policy}) do
    [
      hook_name_error(hook_name),
      hook_policy_error(policy)
    ]
    |> Enum.reject(&is_nil/1)
  end

  @spec hook_name_error(term()) :: String.t() | nil
  defp hook_name_error(hook_name) do
    hook =
      hook_name
      |> config_string()

    if hook in ["after_create", "before_run", "after_run"] do
      nil
    else
      "unknown hook #{inspect(hook_name)}"
    end
  end

  @spec hook_policy_error(term()) :: String.t() | nil
  defp hook_policy_error(policy) do
    value = config_string(policy)

    if value in ["blocking", "warning", "ignore"] do
      nil
    else
      "invalid policy #{inspect(policy)}"
    end
  end

  @spec hook_failure_policy_errors_result([String.t()]) :: map()
  defp hook_failure_policy_errors_result([]) do
    ok_check("hooks.failure_policy", "Hook failure policy is valid")
  end

  defp hook_failure_policy_errors_result(errors) do
    error_check("hooks.failure_policy", "Invalid hook failure policy: #{Enum.join(errors, ", ")}")
  end

  @spec workspace_checks(map()) :: [map()]
  defp workspace_checks(bundle) do
    [
      workspace_root_check(bundle),
      workspace_cleanup_policy_check(bundle)
    ]
  end

  @spec workspace_root_check(map()) :: map()
  defp workspace_root_check(bundle) do
    bundle.workspace
    |> value_at(:root, "~/.open-sleigh/workspaces")
    |> config_string()
    |> present_string()
    |> workspace_root_result()
  end

  @spec workspace_root_result(String.t() | nil) :: map()
  defp workspace_root_result(nil) do
    error_check("workspace.root", "workspace.root is missing")
  end

  defp workspace_root_result(root) do
    root
    |> expand_path()
    |> ensure_workspace_root_writable()
  end

  @spec ensure_workspace_root_writable(Path.t()) :: map()
  defp ensure_workspace_root_writable(root) do
    root
    |> File.mkdir_p()
    |> workspace_root_mkdir_result(root)
  end

  @spec workspace_root_mkdir_result(:ok | {:error, term()}, Path.t()) :: map()
  defp workspace_root_mkdir_result(:ok, root) do
    root
    |> workspace_probe_path()
    |> write_workspace_probe(root)
  end

  defp workspace_root_mkdir_result({:error, reason}, root) do
    error_check("workspace.root", "Workspace root #{root} is not creatable: #{inspect(reason)}")
  end

  @spec workspace_probe_path(Path.t()) :: Path.t()
  defp workspace_probe_path(root) do
    suffix =
      [:positive, :monotonic]
      |> System.unique_integer()
      |> Integer.to_string()

    Path.join(root, ".open_sleigh_doctor_" <> suffix)
  end

  @spec write_workspace_probe(Path.t(), Path.t()) :: map()
  defp write_workspace_probe(probe_path, root) do
    probe_path
    |> File.write("ok")
    |> workspace_probe_result(probe_path, root)
  end

  @spec workspace_probe_result(:ok | {:error, term()}, Path.t(), Path.t()) :: map()
  defp workspace_probe_result(:ok, probe_path, root) do
    _ = File.rm(probe_path)
    ok_check("workspace.root", "Workspace root is writable: #{root}")
  end

  defp workspace_probe_result({:error, reason}, _probe_path, root) do
    error_check("workspace.root", "Workspace root #{root} is not writable: #{inspect(reason)}")
  end

  @spec workspace_cleanup_policy_check(map()) :: map()
  defp workspace_cleanup_policy_check(bundle) do
    bundle.workspace
    |> value_at(:cleanup_policy, "keep")
    |> config_string()
    |> workspace_cleanup_policy_result()
  end

  @spec workspace_cleanup_policy_result(String.t()) :: map()
  defp workspace_cleanup_policy_result("keep") do
    ok_check("workspace.cleanup_policy", "Workspace cleanup policy is explicit: keep")
  end

  defp workspace_cleanup_policy_result(policy) do
    error_check(
      "workspace.cleanup_policy",
      "Only non-destructive workspace cleanup policy is supported now: keep, got #{inspect(policy)}"
    )
  end

  @spec publication_checks(map()) :: [map()]
  defp publication_checks(bundle) do
    [
      branch_regex_check(bundle)
    ]
  end

  @spec branch_regex_check(map()) :: map()
  defp branch_regex_check(bundle) do
    bundle.external_publication
    |> value_at(:branch_regex, nil)
    |> present_string()
    |> branch_regex_result()
  end

  @spec branch_regex_result(String.t() | nil) :: map()
  defp branch_regex_result(nil) do
    error_check("external_publication.branch_regex", "Publication branch_regex is missing")
  end

  defp branch_regex_result(branch_regex) do
    branch_regex
    |> Regex.compile()
    |> branch_regex_compile_result(branch_regex)
  end

  @spec branch_regex_compile_result({:ok, Regex.t()} | {:error, term()}, String.t()) :: map()
  defp branch_regex_compile_result({:ok, _regex}, branch_regex) do
    ok_check(
      "external_publication.branch_regex",
      "Publication branch_regex compiles: #{branch_regex}"
    )
  end

  defp branch_regex_compile_result({:error, reason}, branch_regex) do
    error_check(
      "external_publication.branch_regex",
      "Publication branch_regex #{inspect(branch_regex)} is invalid: #{inspect(reason)}"
    )
  end

  @spec semantic_gate_checks(map()) :: [map()]
  defp semantic_gate_checks(bundle) do
    names =
      bundle.phase_configs
      |> Map.values()
      |> Enum.flat_map(& &1.gates.semantic)
      |> Enum.uniq()

    names
    |> semantic_gates_message()
    |> List.wrap()
  end

  @spec semantic_gates_message([atom()]) :: map()
  defp semantic_gates_message([]) do
    ok_check("gates.semantic", "Semantic gates are disabled")
  end

  defp semantic_gates_message(names) do
    names
    |> uncalibrated_semantic_gates()
    |> semantic_gates_result(names)
  end

  @spec uncalibrated_semantic_gates([atom()]) :: [atom()]
  defp uncalibrated_semantic_gates(names) do
    calibration = GoldenSets.calibration()

    names
    |> Enum.reject(&Map.get(calibration, &1, false))
  end

  @spec semantic_gates_result([atom()], [atom()]) :: map()
  defp semantic_gates_result([], names) do
    ok_check("gates.semantic", "Semantic gates are calibrated: #{inspect(names)}")
  end

  defp semantic_gates_result(missing, _names) do
    error_check(
      "gates.semantic",
      "Semantic gates missing golden-set calibration: #{inspect(missing)}"
    )
  end

  @spec env_check(String.t() | nil, String.t(), String.t(), String.t()) :: map()
  defp env_check(nil, name, _ok_message, error_message), do: error_check(name, error_message)
  defp env_check(_value, name, ok_message, _error_message), do: ok_check(name, ok_message)

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
    Mix.shell().info("Open-Sleigh doctor report for #{report.path}")

    report.checks
    |> Enum.each(&emit_check/1)

    emit_summary(report)
  end

  @spec emit_check(map()) :: :ok
  defp emit_check(check) do
    Mix.shell().info("#{check.status} #{check.name}: #{check.message}")
  end

  @spec emit_summary(map()) :: :ok
  defp emit_summary(%{ready: true}) do
    Mix.shell().info("Open-Sleigh doctor passed")
  end

  defp emit_summary(report) do
    Mix.shell().info("Open-Sleigh doctor found #{report.errors} error(s)")
  end

  @spec raise_on_failure(:ok, map()) :: :ok
  defp raise_on_failure(:ok, %{ready: true}), do: :ok

  defp raise_on_failure(:ok, report) do
    Mix.raise("Open-Sleigh doctor failed: #{report.errors} error(s)")
  end

  @spec serialise_report(map()) :: map()
  defp serialise_report(report) do
    %{
      path: report.path,
      ready: report.ready,
      errors: report.errors,
      warnings: report.warnings,
      checks: Enum.map(report.checks, &serialise_check/1)
    }
  end

  @spec serialise_check(map()) :: map()
  defp serialise_check(check) do
    base = %{
      "status" => Atom.to_string(check.status),
      "name" => check.name,
      "message" => check.message
    }

    check
    |> schema_fields()
    |> Map.merge(base)
  end

  @spec schema_fields(map()) :: map()
  defp schema_fields(check) do
    [:field_path, :expected, :actual, :hint]
    |> Enum.filter(&Map.has_key?(check, &1))
    |> Map.new(&{Atom.to_string(&1), Map.fetch!(check, &1)})
  end

  @spec ok_check(String.t(), String.t()) :: map()
  defp ok_check(name, message), do: check(:ok, name, message)

  @spec warning_check(String.t(), String.t()) :: map()
  defp warning_check(name, message), do: check(:warning, name, message)

  @spec error_check(String.t(), String.t()) :: map()
  defp error_check(name, message), do: check(:error, name, message)

  @spec error_check(String.t(), String.t(), map()) :: map()
  defp error_check(name, message, metadata), do: check(:error, name, message, metadata)

  @spec check(:ok | :warning | :error, String.t(), String.t()) :: map()
  defp check(status, name, message), do: check(status, name, message, %{})

  @spec check(:ok | :warning | :error, String.t(), String.t(), map()) :: map()
  defp check(status, name, message, metadata) do
    metadata
    |> Map.merge(%{status: status, name: name, message: message})
  end

  @spec value_at(term(), atom(), term()) :: term()
  defp value_at(%{} = map, key, fallback) do
    Map.get(map, Atom.to_string(key), Map.get(map, key, fallback))
  end

  defp value_at(_value, _key, fallback), do: fallback

  @spec config_string(term()) :: String.t()
  defp config_string(value) when is_binary(value), do: value
  defp config_string(value) when is_atom(value), do: Atom.to_string(value)
  defp config_string(value), do: to_string(value)

  @spec config_string_or_nil(term()) :: String.t() | nil
  defp config_string_or_nil(nil), do: nil
  defp config_string_or_nil(value), do: config_string(value)

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
