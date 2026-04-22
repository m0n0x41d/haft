defmodule OpenSleigh.Sleigh.SizeBudget do
  @moduledoc """
  Size-budget guardrail for `sleigh.md`.

  This is the first L6 wall: the compiler refuses overlarge files,
  overlarge prompt templates, and include-style directives before any
  semantic config validation runs.
  """

  alias OpenSleigh.Sleigh.CompilerError

  @max_file_lines 300
  @max_prompt_lines 150
  @max_file_bytes 50 * 1024

  @doc "Check whole-file CF1, CF9, and CF10 constraints."
  @spec check(binary()) :: :ok | {:error, [CompilerError.t()]}
  def check(source) when is_binary(source) do
    source
    |> source_errors()
    |> result()
  end

  @doc "Check per-prompt CF2 constraints after prompt extraction."
  @spec check_prompts(%{atom() => String.t()}) :: :ok | {:error, [CompilerError.t()]}
  def check_prompts(prompts) when is_map(prompts) do
    prompts
    |> Enum.flat_map(&prompt_errors/1)
    |> result()
  end

  @spec source_errors(binary()) :: [CompilerError.t()]
  defp source_errors(source) do
    []
    |> maybe_add(over_budget_file?(source), :over_budget_file)
    |> maybe_add(over_budget_bytes?(source), :over_budget_bytes)
    |> maybe_add(include_directive?(source), :include_directive_forbidden)
    |> Enum.reverse()
  end

  @spec prompt_errors({atom(), String.t()}) :: [CompilerError.t()]
  defp prompt_errors({phase, prompt}) do
    []
    |> maybe_add(prompt_over_budget?(prompt), {:over_budget_prompt, phase})
  end

  @spec over_budget_file?(binary()) :: boolean()
  defp over_budget_file?(source) do
    source
    |> line_count()
    |> Kernel.>(@max_file_lines)
  end

  @spec over_budget_bytes?(binary()) :: boolean()
  defp over_budget_bytes?(source), do: byte_size(source) > @max_file_bytes

  @spec include_directive?(binary()) :: boolean()
  defp include_directive?(source) do
    Regex.match?(~r/(^|\n)\s*(@include|{{\s*file\s+)/, source)
  end

  @spec prompt_over_budget?(binary()) :: boolean()
  defp prompt_over_budget?(prompt) do
    prompt
    |> line_count()
    |> Kernel.>(@max_prompt_lines)
  end

  @spec line_count(binary()) :: non_neg_integer()
  defp line_count(""), do: 0

  defp line_count(source) do
    source
    |> String.split("\n")
    |> length()
  end

  @spec maybe_add([CompilerError.t()], boolean(), CompilerError.t()) :: [CompilerError.t()]
  defp maybe_add(errors, true, error), do: [error | errors]
  defp maybe_add(errors, false, _error), do: errors

  @spec result([CompilerError.t()]) :: :ok | {:error, [CompilerError.t()]}
  defp result([]), do: :ok
  defp result(errors), do: {:error, errors}
end
