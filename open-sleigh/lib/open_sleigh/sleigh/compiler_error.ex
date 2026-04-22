defmodule OpenSleigh.Sleigh.CompilerError do
  @moduledoc """
  Closed error alphabet for L6 `sleigh.md` compilation.

  The compiler returns a list of these values so callers can show every
  actionable operator error without mixing parser failures, registry
  failures, and size-budget failures into untyped strings.
  """

  @typedoc "A precise compiler failure."
  @type t ::
          :over_budget_file
          | {:over_budget_prompt, atom()}
          | :over_budget_bytes
          | :include_directive_forbidden
          | :front_matter_missing
          | :yaml_parse_failed
          | :markdown_parse_failed
          | {:missing_section, atom()}
          | {:missing_phase, atom()}
          | {:unknown_adapter, binary()}
          | {:unknown_phase, binary()}
          | {:unknown_tool, atom(), binary()}
          | {:unknown_gate, binary()}
          | {:unknown_prompt_variable, atom(), binary()}
          | {:missing_prompt, atom()}
          | :missing_approvers
          | :max_turns_invalid
          | :single_turn_phase_max_turns_must_be_one
          | {:invalid_phase_config, atom(), atom()}

  @type report :: %{
          required(:path) => String.t(),
          required(:expected) => String.t(),
          required(:actual) => String.t(),
          required(:hint) => String.t()
        }

  @doc "Human- and JSON-facing schema report fields for a compiler error."
  @spec report(t()) :: report()
  def report({:missing_section, section}) do
    %{
      path: Atom.to_string(section),
      expected: "top-level mapping section",
      actual: "missing",
      hint: "Add the #{section} section to the YAML front matter."
    }
  end

  def report({:missing_phase, phase}) do
    %{
      path: "phases.#{phase}",
      expected: "phase config mapping",
      actual: "missing",
      hint: "Add phases.#{phase} with agent_role, tools, and gates."
    }
  end

  def report({:unknown_adapter, adapter}) do
    %{
      path: "agent.kind",
      expected: "one of: codex, mock",
      actual: adapter,
      hint: "Set agent.kind to a supported adapter."
    }
  end

  def report({:unknown_phase, phase}) do
    %{
      path: "phases.#{phase}",
      expected: "one of: frame, execute, measure",
      actual: phase,
      hint: "Remove the unknown phase or rename it to a workflow phase."
    }
  end

  def report({:unknown_tool, adapter, tool}) do
    %{
      path: "phases.*.tools",
      expected: "tool supported by #{adapter}",
      actual: tool,
      hint: "Remove the tool or add adapter support before using it in a phase."
    }
  end

  def report({:unknown_gate, gate}) do
    %{
      path: "phases.*.gates",
      expected: "registered structural, semantic, or human gate",
      actual: gate,
      hint: "Use a gate from OpenSleigh.Gates.Registry or register the new gate."
    }
  end

  def report({:unknown_prompt_variable, phase, variable}) do
    %{
      path: "prompts.#{phase}",
      expected: "known prompt variable for #{phase}",
      actual: variable,
      hint: "Replace the placeholder or add it to the phase prompt contract."
    }
  end

  def report({:missing_prompt, phase}) do
    %{
      path: "prompts.#{phase}",
      expected: "Markdown heading with prompt text",
      actual: "missing",
      hint: "Add a `## #{phase}` prompt section below the YAML front matter."
    }
  end

  def report({:over_budget_prompt, phase}) do
    %{
      path: "prompts.#{phase}",
      expected: "prompt within configured size budget",
      actual: "over budget",
      hint: "Shorten the phase prompt."
    }
  end

  def report({:invalid_phase_config, phase, reason}) do
    %{
      path: "phases.#{phase}",
      expected: "valid PhaseConfig shape",
      actual: Atom.to_string(reason),
      hint: "Check agent_role, tools, gates, max_turns, and valid_until settings."
    }
  end

  def report(:front_matter_missing) do
    %{
      path: "front_matter",
      expected: "YAML front matter delimited by ---",
      actual: "missing",
      hint: "Wrap the config YAML in a leading and closing `---` block."
    }
  end

  def report(:yaml_parse_failed) do
    %{
      path: "front_matter",
      expected: "valid YAML mapping",
      actual: "parse error",
      hint: "Fix YAML indentation, quoting, or scalar syntax."
    }
  end

  def report(:markdown_parse_failed) do
    %{
      path: "prompts",
      expected: "valid Markdown prompt body",
      actual: "parse error",
      hint: "Fix the Markdown below the YAML front matter."
    }
  end

  def report(:missing_approvers) do
    %{
      path: "external_publication.approvers",
      expected: "non-empty list when branch_regex is configured",
      actual: "empty",
      hint: "Add at least one approver or remove the publication gate."
    }
  end

  def report(:max_turns_invalid) do
    %{
      path: "phases.*.max_turns",
      expected: "positive integer",
      actual: "invalid",
      hint: "Set max_turns to an integer greater than zero."
    }
  end

  def report(:single_turn_phase_max_turns_must_be_one) do
    %{
      path: "phases.frame|measure.max_turns",
      expected: "1 for single-turn phases",
      actual: "greater than 1",
      hint: "Set frame and measure max_turns to 1."
    }
  end

  def report(:over_budget_file) do
    %{
      path: "sleigh.md",
      expected: "file within configured size budget",
      actual: "over budget",
      hint: "Move long reference material out of sleigh.md."
    }
  end

  def report(:over_budget_bytes) do
    %{
      path: "sleigh.md",
      expected: "UTF-8 text within byte budget",
      actual: "over budget",
      hint: "Shorten the config or prompt body."
    }
  end

  def report(:include_directive_forbidden) do
    %{
      path: "sleigh.md",
      expected: "self-contained config and prompts",
      actual: "include directive",
      hint: "Inline required prompt text instead of using include directives."
    }
  end

  @doc "Concise compiler-error message."
  @spec message(t()) :: String.t()
  def message(error) do
    report = report(error)
    "#{report.path}: expected #{report.expected}, got #{report.actual}. #{report.hint}"
  end

  @doc "Runtime backstop used by tests and future presentation layers."
  @spec valid?(term()) :: boolean()
  def valid?(:over_budget_file), do: true
  def valid?({:over_budget_prompt, phase}) when is_atom(phase), do: true
  def valid?(:over_budget_bytes), do: true
  def valid?(:include_directive_forbidden), do: true
  def valid?(:front_matter_missing), do: true
  def valid?(:yaml_parse_failed), do: true
  def valid?(:markdown_parse_failed), do: true
  def valid?({:missing_section, section}) when is_atom(section), do: true
  def valid?({:missing_phase, phase}) when is_atom(phase), do: true
  def valid?({:unknown_adapter, adapter}) when is_binary(adapter), do: true
  def valid?({:unknown_phase, phase}) when is_binary(phase), do: true
  def valid?({:unknown_tool, adapter, tool}) when is_atom(adapter) and is_binary(tool), do: true
  def valid?({:unknown_gate, gate}) when is_binary(gate), do: true

  def valid?({:unknown_prompt_variable, phase, variable})
      when is_atom(phase) and is_binary(variable),
      do: true

  def valid?({:missing_prompt, phase}) when is_atom(phase), do: true
  def valid?(:missing_approvers), do: true
  def valid?(:max_turns_invalid), do: true
  def valid?(:single_turn_phase_max_turns_must_be_one), do: true

  def valid?({:invalid_phase_config, phase, reason}) when is_atom(phase) and is_atom(reason),
    do: true

  def valid?(_), do: false
end
