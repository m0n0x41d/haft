defmodule OpenSleigh.WorkspaceManager do
  @moduledoc """
  Per-ticket workspace lifecycle + hook execution.

  Per `specs/target-system/WORKSPACE.md` + SPEC §7.2:

  * Workspace path is `<workspace_root>/<sanitized_identifier>`.
  * Workspace reuse across runs for the same ticket (successful
    runs do NOT delete the directory).
  * Every path validation goes through `Adapter.PathGuard` — NO
    ad-hoc `File.*` calls against paths that haven't been
    canonicalised (CL11).
  * Hooks are trusted `bash -lc` shell scripts from `sleigh.md`
    with `hooks.timeout_ms` timeouts.

  All functions are thin L5 shells over the L4 `PathGuard` +
  stdlib primitives. No GenServer here — a WorkspaceManager is a
  stateless helper; the L5 `AgentWorker` calls these functions
  directly.
  """

  alias OpenSleigh.Adapter.PathGuard
  alias OpenSleigh.EffectError

  @sanitize_re ~r/[^A-Za-z0-9._-]/

  @typedoc "Outcome of `create_for_ticket/3`."
  @type create_result ::
          {:ok, Path.t(), :new | :reused}
          | {:error, EffectError.t()}

  @typedoc "Hook execution result."
  @type hook_result ::
          :ok
          | {:error, :hook_timeout | :hook_failed}

  @typedoc "Result of resetting a reused git workspace before a fresh preflight."
  @type reset_result ::
          :ok
          | {:error, :workspace_reset_timeout | :workspace_reset_failed}

  @doc """
  Ensure the per-ticket workspace exists. Returns `:new` when the
  directory was created on this call, `:reused` if it already
  existed. Calls `PathGuard.canonical/2` against the resolved path
  before any filesystem mutation.
  """
  @spec create_for_ticket(Path.t(), String.t(), PathGuard.config()) :: create_result()
  def create_for_ticket(workspace_root, ticket_identifier, guard_config)
      when is_binary(workspace_root) and is_binary(ticket_identifier) do
    key = sanitize(ticket_identifier)
    path = Path.join(workspace_root, key)

    with {:ok, canonical} <- PathGuard.canonical(path, guard_config) do
      fresh_or_reused(canonical)
    end
  end

  @doc """
  Sanitise a ticket identifier for use as a workspace directory
  name — replaces any char not in `[A-Za-z0-9._-]` with `_`.
  """
  @spec sanitize(String.t()) :: String.t()
  def sanitize(identifier) when is_binary(identifier) do
    String.replace(identifier, @sanitize_re, "_")
  end

  @doc """
  Run a hook script. Returns `:ok` on success, `{:error,
  :hook_timeout}` on timeout, `{:error, :hook_failed}` on non-zero
  exit. Output is captured and discarded (operator logs would
  capture from the L5 orchestrator if desired).
  """
  @spec run_hook(Path.t(), String.t(), pos_integer()) :: hook_result()
  def run_hook(workspace_path, script, timeout_ms)
      when is_binary(workspace_path) and is_binary(script) and is_integer(timeout_ms) and
             timeout_ms > 0 do
    fn -> exec_hook(workspace_path, script) end
    |> Task.async()
    |> hook_result(timeout_ms)
  end

  @doc """
  Reset a reused git workspace back to clean `HEAD` state before a new
  preflight attempt. Non-git directories or repos without commits are
  treated as already clean.
  """
  @spec reset_git_workspace(Path.t(), pos_integer()) :: reset_result()
  def reset_git_workspace(workspace_path, timeout_ms)
      when is_binary(workspace_path) and is_integer(timeout_ms) and timeout_ms > 0 do
    script =
      """
      git rev-parse --is-inside-work-tree >/dev/null 2>&1 || exit 0
      git rev-parse --verify HEAD >/dev/null 2>&1 || exit 0
      git reset --hard HEAD
      git clean -fd
      """
      |> String.trim()

    fn -> exec_hook(workspace_path, script) end
    |> Task.async()
    |> reset_result(timeout_ms)
  end

  @spec hook_result(Task.t(), pos_integer()) :: hook_result()
  defp hook_result(task, timeout_ms) do
    task
    |> Task.yield(timeout_ms)
    |> interpret_hook_yield(task)
  end

  @spec interpret_hook_yield({:ok, {binary(), integer()}} | nil, Task.t()) :: hook_result()
  defp interpret_hook_yield({:ok, {_out, 0}}, _task), do: :ok
  defp interpret_hook_yield({:ok, {_out, _nonzero}}, _task), do: {:error, :hook_failed}

  defp interpret_hook_yield(nil, task) do
    _ = Task.shutdown(task, :brutal_kill)
    {:error, :hook_timeout}
  end

  @spec reset_result(Task.t(), pos_integer()) :: reset_result()
  defp reset_result(task, timeout_ms) do
    task
    |> Task.yield(timeout_ms)
    |> interpret_reset_yield(task)
  end

  @spec interpret_reset_yield({:ok, {binary(), integer()}} | nil, Task.t()) :: reset_result()
  defp interpret_reset_yield({:ok, {_out, 0}}, _task), do: :ok
  defp interpret_reset_yield({:ok, {_out, _nonzero}}, _task), do: {:error, :workspace_reset_failed}

  defp interpret_reset_yield(nil, task) do
    _ = Task.shutdown(task, :brutal_kill)
    {:error, :workspace_reset_timeout}
  end

  @doc """
  Cleanup — `rm -rf` the workspace directory. Used for terminal-
  state workspace cleanup (SPEC §7.5). Returns `:ok` whether the
  directory existed or not.
  """
  @spec cleanup(Path.t()) :: :ok
  def cleanup(workspace_path) when is_binary(workspace_path) do
    _ = File.rm_rf(workspace_path)
    :ok
  end

  # ——— internals ———

  @spec fresh_or_reused(Path.t()) :: create_result()
  defp fresh_or_reused(canonical_path) do
    case File.mkdir_p(canonical_path) do
      :ok -> {:ok, canonical_path, freshness(canonical_path)}
      {:error, _reason} -> {:error, :invalid_workspace_cwd}
    end
  end

  @spec freshness(Path.t()) :: :new | :reused
  defp freshness(path) do
    # We treat "empty directory now" as `:new`, "had entries" as
    # `:reused`. Good-enough heuristic; true tracking of whether
    # this call created the dir vs found it would need an atomic
    # mkdir with error handling.
    case File.ls(path) do
      {:ok, []} -> :new
      {:ok, _entries} -> :reused
      {:error, _} -> :new
    end
  end

  @spec exec_hook(Path.t(), String.t()) :: {binary(), integer()}
  defp exec_hook(workspace_path, script) do
    System.cmd("bash", ["-lc", script], cd: workspace_path, stderr_to_stdout: true)
  rescue
    _ -> {"", 1}
  end
end
