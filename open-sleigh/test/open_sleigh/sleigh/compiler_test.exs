defmodule OpenSleigh.Sleigh.CompilerTest do
  use ExUnit.Case, async: true

  alias OpenSleigh.ConfigHash
  alias OpenSleigh.Sleigh.{Compiler, CompilerError}

  test "valid sleigh.md.example compiles to a WorkflowStore bundle" do
    assert {:ok, bundle} =
             "sleigh.md.example"
             |> File.read!()
             |> Compiler.compile()

    assert %{phase: :frame, max_turns: 1} = bundle.phase_configs.frame
    assert %{phase: :execute, max_turns: 20} = bundle.phase_configs.execute
    assert %{phase: :measure, max_turns: 1} = bundle.phase_configs.measure
    assert bundle.external_publication.approvers == ["ivan@weareocta.com"]
    assert bundle.workspace["root"] == "~/.open-sleigh/workspaces"
    assert bundle.judge["kind"] == "codex"
    assert ConfigHash.valid?(bundle.config_hashes.execute)
    assert String.contains?(bundle.prompts.execute, "<!-- config_hash: ")
  end

  test "commission local example compiles without tracker credentials" do
    assert {:ok, bundle} =
             "sleigh.commission.md.example"
             |> File.read!()
             |> Compiler.compile()

    assert bundle.workflow == :mvp1r
    assert bundle.commission_source["kind"] == "local"
    assert bundle.projection["mode"] == "local_only"
    assert %{phase: :preflight, agent_role: :preflight_checker} = bundle.phase_configs.preflight
    assert :commission_runnable in bundle.phase_configs.preflight.gates.structural
  end

  test "Claude adapter skeleton compiles through the same adapter registry" do
    assert {:ok, bundle} =
             valid_source()
             |> String.replace("kind: codex", "kind: claude", global: false)
             |> Compiler.compile()

    assert bundle.agent["kind"] == "claude"
    assert :bash in bundle.phase_configs.execute.tools
  end

  test "compiler errors stay inside the typed error alphabet" do
    assert CompilerError.valid?(:over_budget_file)
    assert CompilerError.valid?({:unknown_tool, :codex, "rocket"})
    refute CompilerError.valid?({:freeform, "bad"})
  end

  test "CF1 rejects files over 300 lines" do
    source = Enum.map_join(1..301, "\n", &"line-#{&1}")

    assert {:error, errors} = Compiler.compile(source)
    assert :over_budget_file in errors
  end

  test "CF2 rejects prompt templates over 150 lines" do
    prompt =
      1..151
      |> Enum.map_join("\n", &"Frame line #{&1}")

    source =
      valid_source()
      |> replace_prompt(:frame, prompt)

    assert {:error, errors} = Compiler.compile(source)
    assert {:over_budget_prompt, :frame} in errors
  end

  test "CF3 rejects tools outside the declared adapter registry" do
    source =
      valid_source()
      |> String.replace("tools: [haft_query, read, grep]", "tools: [haft_query, rocket, grep]")

    assert {:error, errors} = Compiler.compile(source)
    assert {:unknown_tool, :codex, "rocket"} in errors
  end

  test "CF4 rejects phases outside the workflow alphabet" do
    source =
      valid_source()
      |> String.replace("  measure:\n", "  banana:\n")

    assert {:error, errors} = Compiler.compile(source)
    assert {:unknown_phase, "banana"} in errors
  end

  test "CF5 rejects prompt variables outside the phase input schema" do
    source =
      valid_source()
      |> String.replace("{{ticket.title}}", "{{ticket.unknown}}")

    assert {:error, errors} = Compiler.compile(source)
    assert {:unknown_prompt_variable, :frame, "ticket.unknown"} in errors
  end

  test "CF6 rejects publication regex without approvers" do
    source =
      valid_source()
      |> String.replace("approvers: [\"ivan@weareocta.com\"]", "approvers: []")

    assert {:error, errors} = Compiler.compile(source)
    assert :missing_approvers in errors
  end

  test "CF9 rejects include-style directives" do
    source = valid_source() <> "\n@include ./large.md\n"

    assert {:error, errors} = Compiler.compile(source)
    assert :include_directive_forbidden in errors
  end

  test "CF10 rejects byte-budget overflow" do
    source = String.duplicate("x", 51 * 1024)

    assert {:error, errors} = Compiler.compile(source)
    assert :over_budget_bytes in errors
  end

  test "gate names are validated against the registry" do
    source =
      valid_source()
      |> String.replace("problem_card_ref_present", "missing_gate")

    assert {:error, errors} = Compiler.compile(source)
    assert {:unknown_gate, "missing_gate"} in errors
  end

  test "CT3 rejects max_turns below one" do
    source =
      valid_source()
      |> String.replace("max_turns: 20", "max_turns: 0")

    assert {:error, errors} = Compiler.compile(source)
    assert :max_turns_invalid in errors
  end

  test "CT4 rejects multi-turn Frame configuration" do
    source =
      valid_source()
      |> String.replace(
        "    agent_role: frame_verifier\n",
        "    agent_role: frame_verifier\n    max_turns: 2\n"
      )

    assert {:error, errors} = Compiler.compile(source)
    assert :single_turn_phase_max_turns_must_be_one in errors
  end

  test "missing prompt section is a compiler error" do
    source =
      valid_source()
      |> String.replace(~r/\n## Measure\n.*/s, "")

    assert {:error, errors} = Compiler.compile(source)
    assert {:missing_prompt, :measure} in errors
  end

  defp valid_source do
    File.read!("sleigh.md.example")
  end

  defp replace_prompt(source, :frame, prompt) do
    Regex.replace(~r/## Frame\n.*?\n\n## Execute/s, source, "## Frame\n#{prompt}\n\n## Execute")
  end
end
