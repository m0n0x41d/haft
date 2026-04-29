defmodule OpenSleigh.Adapter.PathGuard do
  @moduledoc """
  Canonical path resolution with enumerated bypass-class rejection.

  Per `specs/enabling-system/FUNCTIONAL_ARCHITECTURE.md` LAYER 4 +
  `ILLEGAL_STATES.md` CL5–CL11. The **single source of truth** for
  "is this path safe to write?" — no L4 adapter performs ad-hoc path
  comparisons (enforced by Credo rule `NoDirectFilesystemIO`, CL11).

  Algorithm (MVP-1):

  1. `Path.expand/1` — resolve `..` and `~`.
  2. Recursive symlink dereference via `File.read_link/1`, max depth 8
     (CL8 loop prevention).
  3. Prefix check against forbidden trees (CL5 / CL6 / CL9): any
     forbidden path is a hard reject.
  4. `.git/config remote.origin.url` check: if the resolved workspace
     contains a git remote whose URL matches Open-Sleigh's canonical
     remote, reject with `:workspace_is_self` (CL9).
  5. Hardlink inode check (CL7) — **MVP-1 acknowledged weakness.**
     Not implemented; full walk of forbidden trees at each
     validation is too expensive. Documented honestly here and in
     `ILLEGAL_STATES.md §Guardrail strength ratings`.

  All paths returned from `canonical/2` are absolute and
  symlink-resolved. Callers receive either a canonicalised path
  (safe to hand to `File.write/2`) or a typed error from
  `OpenSleigh.EffectError`.

  PathGuard is only a workspace-safety guard. It does not know about
  WorkCommission Scope and must not return commission-authority errors.
  """

  alias OpenSleigh.EffectError

  @max_depth 8

  @typedoc """
  Guard config. `forbidden_paths` are absolute canonicalised
  directory roots; any resolved path prefixed by one of them is
  rejected. `forbidden_remote_substrings` matches against
  `remote.origin.url` in `.git/config` — substring match (CL9).
  """
  @type config :: %{
          required(:forbidden_paths) => [Path.t()],
          required(:forbidden_remote_substrings) => [String.t()]
        }

  @doc """
  Resolve `path` to its canonical absolute form and check it against
  every bypass class. Returns `{:ok, canonical_path}` on success or
  `{:error, EffectError.t()}` on any rejection.

  `config.forbidden_paths` MUST be absolute, pre-canonicalised
  directory roots — typically the Open-Sleigh source tree and
  `~/.open-sleigh/`.
  """
  @spec canonical(Path.t(), config()) :: {:ok, Path.t()} | {:error, EffectError.t()}
  def canonical(
        path,
        %{
          forbidden_paths: forbidden,
          forbidden_remote_substrings: remote_patterns
        } = _config
      )
      when is_binary(path) do
    with {:ok, expanded} <- expand(path),
         {:ok, resolved} <- resolve_symlinks(expanded, @max_depth),
         :ok <- check_forbidden_prefix(resolved, forbidden),
         :ok <- check_not_self_clone(resolved, remote_patterns) do
      {:ok, resolved}
    end
  end

  def canonical(_path, _config), do: {:error, :invalid_workspace_cwd}

  @doc "Validate a workspace_path at `Session.new/1` time."
  @spec validate_workspace(Path.t(), config()) ::
          {:ok, Path.t()} | {:error, EffectError.t()}
  def validate_workspace(path, config), do: canonical(path, config)

  @doc """
  Resolve a target path and return its path relative to `workspace_path`.

  This is still only the workspace guard. A returned relative path means
  the target stayed inside the workspace; WorkCommission authorization is
  checked separately by `OpenSleigh.Agent.Adapter`.
  """
  @spec relative_to_workspace(Path.t(), Path.t(), config()) ::
          {:ok, Path.t()} | {:error, EffectError.t()}
  def relative_to_workspace(workspace_path, path, config)
      when is_binary(workspace_path) and is_binary(path) do
    path =
      path
      |> Path.expand(workspace_path)

    with {:ok, workspace} <- canonical(workspace_path, config),
         {:ok, target} <- canonical(path, config) do
      relative_workspace_result(target, workspace)
    end
  end

  def relative_to_workspace(_workspace_path, _path, _config), do: {:error, :invalid_workspace_cwd}

  # ——— internals ———

  @spec expand(Path.t()) :: {:ok, Path.t()} | {:error, EffectError.t()}
  defp expand(path) do
    {:ok, Path.expand(path)}
  rescue
    ArgumentError -> {:error, :invalid_workspace_cwd}
  end

  @spec resolve_symlinks(Path.t(), non_neg_integer()) ::
          {:ok, Path.t()} | {:error, EffectError.t()}
  defp resolve_symlinks(_path, 0), do: {:error, :path_symlink_loop}

  defp resolve_symlinks(path, depth) do
    case File.lstat(path) do
      {:ok, %File.Stat{type: :symlink}} -> follow_symlink(path, depth)
      {:ok, _stat} -> {:ok, path}
      # The path need not exist for a workspace creation; permit it
      # and let the creator attempt `mkdir -p`. This preserves the
      # "validate *future* write target" use-case.
      {:error, :enoent} -> {:ok, path}
      {:error, _reason} -> {:error, :invalid_workspace_cwd}
    end
  end

  @spec follow_symlink(Path.t(), non_neg_integer()) ::
          {:ok, Path.t()} | {:error, EffectError.t()}
  defp follow_symlink(path, depth) do
    case :file.read_link_all(String.to_charlist(path)) do
      {:ok, target} ->
        resolved =
          target
          |> IO.chardata_to_string()
          |> Path.expand(Path.dirname(path))

        resolve_symlinks(resolved, depth - 1)

      {:error, _} ->
        {:error, :path_symlink_escape}
    end
  end

  @spec check_forbidden_prefix(Path.t(), [Path.t()]) ::
          :ok | {:error, EffectError.t()}
  defp check_forbidden_prefix(resolved, forbidden) do
    if Enum.any?(forbidden, &under?(resolved, &1)) do
      {:error, :path_outside_workspace}
    else
      :ok
    end
  end

  # `under?` — is `child` inside `parent` as a directory prefix?
  # Uses the relative-path trick: if the relative path goes out of
  # `parent` it starts with `..`.
  @spec under?(Path.t(), Path.t()) :: boolean()
  defp under?(child, parent) do
    relative = Path.relative_to(child, parent)
    # If Path.relative_to couldn't make it relative (different root,
    # etc.), it returns the original absolute path.
    relative != child and not String.starts_with?(relative, "..")
  end

  @spec relative_workspace_result(Path.t(), Path.t()) ::
          {:ok, Path.t()} | {:error, EffectError.t()}
  defp relative_workspace_result(target, workspace) do
    target
    |> Path.relative_to(workspace)
    |> relative_path_result(target)
  end

  @spec relative_path_result(Path.t(), Path.t()) ::
          {:ok, Path.t()} | {:error, EffectError.t()}
  defp relative_path_result(relative, target) do
    cond do
      relative == target ->
        {:error, :path_outside_workspace}

      Path.type(relative) == :absolute ->
        {:error, :path_outside_workspace}

      String.starts_with?(relative, "..") ->
        {:error, :path_outside_workspace}

      true ->
        {:ok, relative}
    end
  end

  @spec check_not_self_clone(Path.t(), [String.t()]) ::
          :ok | {:error, EffectError.t()}
  defp check_not_self_clone(resolved, patterns) do
    git_config = Path.join([resolved, ".git", "config"])

    case File.read(git_config) do
      {:ok, content} ->
        if matches_any_remote?(content, patterns) do
          {:error, :workspace_is_self}
        else
          :ok
        end

      {:error, _} ->
        # No .git/config → not a clone. OK.
        :ok
    end
  end

  @spec matches_any_remote?(String.t(), [String.t()]) :: boolean()
  defp matches_any_remote?(content, patterns) do
    Enum.any?(patterns, fn pat -> String.contains?(content, pat) end)
  end
end
