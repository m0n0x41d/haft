defmodule OpenSleigh.Sleigh.Compiler do
  @moduledoc """
  L6 compiler for the operator-facing `sleigh.md` file.

  The compiler turns a YAML-front-matter + Markdown prompt carrier into
  the compiled bundle consumed by `OpenSleigh.WorkflowStore`. It is pure:
  no filesystem reads, no process state, and no hot-reload side effects.
  """

  alias OpenSleigh.{
    AuthoringRole,
    ConfigHash,
    Phase,
    PhaseConfig,
    Workflow
  }

  alias OpenSleigh.Agent
  alias OpenSleigh.Gates.Registry
  alias OpenSleigh.Sleigh.{CompilerError, SizeBudget}

  @adapter_modules %{
    "claude" => Agent.Claude,
    "codex" => Agent.Codex,
    "mock" => Agent.Mock
  }

  @default_valid_until_days %{
    frame: 7,
    execute: 30,
    measure: 30
  }

  @ticket_variables [
    "ticket.id",
    "ticket.title",
    "ticket.body",
    "ticket.problem_card_ref",
    "ticket.target_branch"
  ]

  @prompt_variables %{
    frame: @ticket_variables,
    execute:
      @ticket_variables ++
        [
          "problem_card.id",
          "problem_card.ref",
          "problem_card.title",
          "problem_card.body",
          "problem_card.description"
        ],
    measure:
      @ticket_variables ++
        [
          "problem_card.id",
          "problem_card.ref",
          "pr.url",
          "pr.sha",
          "ci.run_id",
          "ci.status"
        ]
  }

  @doc "Compile a complete `sleigh.md` source string."
  @spec compile(String.t()) ::
          {:ok, OpenSleigh.WorkflowStore.bundle()} | {:error, [CompilerError.t()]}
  def compile(source) when is_binary(source) do
    with :ok <- SizeBudget.check(source),
         {:ok, parts} <- split_front_matter(source),
         {:ok, config} <- parse_yaml(parts.front_matter),
         :ok <- parse_markdown(parts.body),
         {:ok, prompts} <- extract_prompts(parts.body),
         :ok <- SizeBudget.check_prompts(prompts) do
      build_bundle(config, prompts)
    end
  end

  @spec split_front_matter(String.t()) ::
          {:ok, %{front_matter: String.t(), body: String.t()}} | {:error, [CompilerError.t()]}
  defp split_front_matter(source) do
    case Regex.run(~r/\A---\s*\n(.*?)\n---\s*\n?(.*)\z/s, source, capture: :all_but_first) do
      [front_matter, body] ->
        {:ok, %{front_matter: front_matter, body: body}}

      _ ->
        {:error, [:front_matter_missing]}
    end
  end

  @spec parse_yaml(String.t()) :: {:ok, map()} | {:error, [CompilerError.t()]}
  defp parse_yaml(front_matter) do
    case YamlElixir.read_from_string(front_matter) do
      {:ok, config} when is_map(config) -> {:ok, config}
      {:ok, _other} -> {:error, [:yaml_parse_failed]}
      {:error, _reason} -> {:error, [:yaml_parse_failed]}
    end
  end

  @spec parse_markdown(String.t()) :: :ok | {:error, [CompilerError.t()]}
  defp parse_markdown(body) do
    case EarmarkParser.as_ast(body) do
      {:ok, _ast, _messages} -> :ok
      {:error, _ast, _messages} -> {:error, [:markdown_parse_failed]}
    end
  end

  @spec extract_prompts(String.t()) :: {:ok, %{atom() => String.t()}}
  defp extract_prompts(body) do
    prompts =
      body
      |> String.split("\n", trim: false)
      |> Enum.reduce(empty_prompt_state(), &accumulate_prompt_line/2)
      |> flush_prompt()
      |> Map.fetch!(:prompts)

    {:ok, prompts}
  end

  @spec empty_prompt_state() :: map()
  defp empty_prompt_state do
    %{current: nil, lines: [], prompts: %{}}
  end

  @spec accumulate_prompt_line(String.t(), map()) :: map()
  defp accumulate_prompt_line(line, state) do
    line
    |> prompt_heading()
    |> apply_prompt_heading(line, state)
  end

  @spec apply_prompt_heading({:ok, atom()} | :ignore, String.t(), map()) :: map()
  defp apply_prompt_heading({:ok, phase}, _line, state) do
    state
    |> flush_prompt()
    |> Map.merge(%{current: phase, lines: []})
  end

  defp apply_prompt_heading(:ignore, _line, %{current: nil} = state), do: state

  defp apply_prompt_heading(:ignore, line, state) do
    Map.update!(state, :lines, &[line | &1])
  end

  @spec flush_prompt(map()) :: map()
  defp flush_prompt(%{current: nil} = state), do: state

  defp flush_prompt(%{current: phase, lines: lines, prompts: prompts} = state) do
    prompt =
      lines
      |> Enum.reverse()
      |> Enum.join("\n")
      |> String.trim()

    %{state | lines: [], prompts: Map.put(prompts, phase, prompt)}
  end

  @spec prompt_heading(String.t()) :: {:ok, atom()} | :ignore
  defp prompt_heading(line) do
    case Regex.run(~r/^##\s+(.+?)\s*$/, line, capture: :all_but_first) do
      [heading] -> prompt_phase(heading)
      _ -> :ignore
    end
  end

  @spec prompt_phase(String.t()) :: {:ok, atom()} | :ignore
  defp prompt_phase(heading) do
    heading
    |> String.trim()
    |> String.downcase()
    |> prompt_phase_by_name()
  end

  @spec prompt_phase_by_name(String.t()) :: {:ok, atom()} | :ignore
  defp prompt_phase_by_name("frame"), do: {:ok, :frame}
  defp prompt_phase_by_name("execute"), do: {:ok, :execute}
  defp prompt_phase_by_name("measure"), do: {:ok, :measure}
  defp prompt_phase_by_name(_), do: :ignore

  @spec build_bundle(map(), %{atom() => String.t()}) ::
          {:ok, OpenSleigh.WorkflowStore.bundle()} | {:error, [CompilerError.t()]}
  defp build_bundle(config, prompts) do
    errors = section_errors(config)

    if errors == [] do
      build_validated_bundle(config, prompts)
    else
      {:error, errors}
    end
  end

  @spec build_validated_bundle(map(), %{atom() => String.t()}) ::
          {:ok, OpenSleigh.WorkflowStore.bundle()} | {:error, [CompilerError.t()]}
  defp build_validated_bundle(config, prompts) do
    workflow = Workflow.mvp1()

    with {:ok, adapter_kind, adapter_module} <- adapter_module(config),
         {:ok, phase_configs} <-
           compile_phase_configs(config, workflow, adapter_kind, adapter_module),
         :ok <- validate_prompts(prompts, phase_configs),
         :ok <- validate_external_publication(config) do
      {:ok, bundle(config, workflow, phase_configs, prompts)}
    else
      {:error, errors} when is_list(errors) -> {:error, errors}
    end
  end

  @spec section_errors(map()) :: [CompilerError.t()]
  defp section_errors(config) do
    [:engine, :tracker, :agent, :haft, :phases]
    |> Enum.reject(&section_present?(config, &1))
    |> Enum.map(&{:missing_section, &1})
  end

  @spec section_present?(map(), atom()) :: boolean()
  defp section_present?(config, section) do
    config
    |> value_at(section)
    |> is_map()
  end

  @spec adapter_module(map()) :: {:ok, atom(), module()} | {:error, [CompilerError.t()]}
  defp adapter_module(config) do
    adapter_name =
      config
      |> value_at(:agent, %{})
      |> value_at(:kind)
      |> config_string()

    case Map.fetch(@adapter_modules, adapter_name) do
      {:ok, module} -> {:ok, known_atom_from_string(adapter_name, [:codex, :mock]), module}
      :error -> {:error, [{:unknown_adapter, adapter_name}]}
    end
  end

  @spec compile_phase_configs(map(), Workflow.t(), atom(), module()) ::
          {:ok, %{Phase.t() => PhaseConfig.t()}} | {:error, [CompilerError.t()]}
  defp compile_phase_configs(config, workflow, adapter_kind, adapter_module) do
    phase_section = value_at(config, :phases, %{})

    errors =
      phase_section
      |> unknown_phase_errors(workflow)
      |> Kernel.++(missing_phase_errors(phase_section, workflow))

    if errors == [] do
      build_phase_configs(config, workflow, adapter_kind, adapter_module)
    else
      {:error, errors}
    end
  end

  @spec build_phase_configs(map(), Workflow.t(), atom(), module()) ::
          {:ok, %{Phase.t() => PhaseConfig.t()}} | {:error, [CompilerError.t()]}
  defp build_phase_configs(config, workflow, adapter_kind, adapter_module) do
    results =
      workflow
      |> required_phases()
      |> Enum.map(&compile_phase_config(&1, config, adapter_kind, adapter_module))

    errors = Enum.flat_map(results, &phase_result_errors/1)

    if errors == [] do
      {:ok, Map.new(results, &phase_result_pair/1)}
    else
      {:error, errors}
    end
  end

  @spec required_phases(Workflow.t()) :: [Phase.t()]
  defp required_phases(%Workflow{} = workflow) do
    Enum.reject(workflow.phases, &Workflow.terminal?(workflow, &1))
  end

  @spec compile_phase_config(Phase.t(), map(), atom(), module()) ::
          {:ok, {Phase.t(), PhaseConfig.t()}} | {:error, [CompilerError.t()]}
  defp compile_phase_config(phase, config, adapter_kind, adapter_module) do
    phase_attrs =
      config
      |> value_at(:phases, %{})
      |> phase_config_input(phase)

    with {:ok, role} <- role_atom(value_at(phase_attrs, :agent_role), phase),
         {:ok, tools} <-
           resolve_tools(value_at(phase_attrs, :tools), adapter_kind, adapter_module),
         {:ok, gates} <- resolve_gates(value_at(phase_attrs, :gates, %{})),
         {:ok, phase_config} <- new_phase_config(phase, config, phase_attrs, role, tools, gates) do
      {:ok, {phase, phase_config}}
    else
      {:error, errors} when is_list(errors) -> {:error, errors}
    end
  end

  @spec phase_config_input(map(), Phase.t()) :: map()
  defp phase_config_input(phase_section, phase) do
    phase_section
    |> value_at(phase, %{})
  end

  @spec role_atom(term(), Phase.t()) :: {:ok, AuthoringRole.t()} | {:error, [CompilerError.t()]}
  defp role_atom(value, phase) do
    value
    |> config_string()
    |> known_atom_from_string(AuthoringRole.all())
    |> role_result(phase)
  end

  @spec role_result(atom() | nil, Phase.t()) ::
          {:ok, AuthoringRole.t()} | {:error, [CompilerError.t()]}
  defp role_result(role, _phase) when is_atom(role) and not is_nil(role), do: {:ok, role}

  defp role_result(nil, phase) do
    {:error, [{:invalid_phase_config, phase, :invalid_agent_role}]}
  end

  @spec resolve_tools(term(), atom(), module()) :: {:ok, [atom()]} | {:error, [CompilerError.t()]}
  defp resolve_tools(tools, adapter_kind, adapter_module) when is_list(tools) do
    allowed_tools = adapter_module.tool_registry()

    results =
      Enum.map(tools, &resolve_tool(&1, adapter_kind, allowed_tools))

    errors = Enum.flat_map(results, &tool_result_errors/1)

    if errors == [] do
      {:ok, Enum.map(results, &tool_result_atom/1)}
    else
      {:error, errors}
    end
  end

  defp resolve_tools(_tools, _adapter_kind, _adapter_module) do
    {:error, [{:invalid_phase_config, :unknown, :invalid_tools}]}
  end

  @spec resolve_tool(term(), atom(), [atom()]) ::
          {:ok, atom()} | {:error, CompilerError.t()}
  defp resolve_tool(tool, adapter_kind, allowed_tools) do
    tool_name = config_string(tool)

    case known_atom_from_string(tool_name, allowed_tools) do
      atom when is_atom(atom) and not is_nil(atom) -> {:ok, atom}
      nil -> {:error, {:unknown_tool, adapter_kind, tool_name}}
    end
  end

  @spec resolve_gates(term()) :: {:ok, PhaseConfig.gates()} | {:error, [CompilerError.t()]}
  defp resolve_gates(gates) when is_map(gates) do
    structural = resolve_gate_list(value_at(gates, :structural, []), Registry.structural_gates())
    semantic = resolve_gate_list(value_at(gates, :semantic, []), Registry.semantic_gates())
    human = resolve_gate_list(value_at(gates, :human, []), Registry.human_gates())

    results = structural ++ semantic ++ human
    errors = Enum.flat_map(results, &gate_result_errors/1)

    if errors == [] do
      {:ok,
       %{
         structural: Enum.map(structural, &gate_result_atom/1),
         semantic: Enum.map(semantic, &gate_result_atom/1),
         human: Enum.map(human, &gate_result_atom/1)
       }}
    else
      {:error, errors}
    end
  end

  defp resolve_gates(_gates), do: {:error, [{:invalid_phase_config, :unknown, :invalid_gates}]}

  @spec resolve_gate_list(term(), [atom()]) :: [{:ok, atom()} | {:error, CompilerError.t()}]
  defp resolve_gate_list(gates, known_gates) when is_list(gates) do
    Enum.map(gates, &resolve_gate(&1, known_gates))
  end

  defp resolve_gate_list(_gates, _known_gates) do
    [{:error, {:invalid_phase_config, :unknown, :invalid_gates}}]
  end

  @spec resolve_gate(term(), [atom()]) :: {:ok, atom()} | {:error, CompilerError.t()}
  defp resolve_gate(gate, known_gates) do
    gate_name = config_string(gate)

    case known_atom_from_string(gate_name, known_gates) do
      atom when is_atom(atom) and not is_nil(atom) -> {:ok, atom}
      nil -> {:error, {:unknown_gate, gate_name}}
    end
  end

  @spec new_phase_config(
          Phase.t(),
          map(),
          map(),
          AuthoringRole.t(),
          [atom()],
          PhaseConfig.gates()
        ) ::
          {:ok, PhaseConfig.t()} | {:error, [CompilerError.t()]}
  defp new_phase_config(phase, config, phase_attrs, role, tools, gates) do
    attrs = %{
      phase: phase,
      agent_role: role,
      tools: tools,
      gates: gates,
      prompt_template_key: phase,
      max_turns: max_turns_for_phase(phase, config, phase_attrs),
      default_valid_until_days: default_valid_until_days(phase, phase_attrs)
    }

    attrs
    |> PhaseConfig.new()
    |> phase_config_result(phase)
  end

  @spec max_turns_for_phase(Phase.t(), map(), map()) :: term()
  defp max_turns_for_phase(phase, config, phase_attrs) do
    phase_attrs
    |> value_at(:max_turns)
    |> max_turns_or_default(phase, config)
  end

  @spec max_turns_or_default(term(), Phase.t(), map()) :: term()
  defp max_turns_or_default(nil, phase, config) do
    if Phase.single_turn?(phase) do
      1
    else
      config
      |> value_at(:agent, %{})
      |> value_at(:max_turns)
    end
  end

  defp max_turns_or_default(max_turns, _phase, _config), do: max_turns

  @spec default_valid_until_days(Phase.t(), map()) :: pos_integer()
  defp default_valid_until_days(phase, phase_attrs) do
    phase_attrs
    |> value_at(:default_valid_until_days)
    |> valid_until_days_or_default(phase)
  end

  @spec valid_until_days_or_default(term(), Phase.t()) :: pos_integer()
  defp valid_until_days_or_default(nil, phase), do: Map.fetch!(@default_valid_until_days, phase)
  defp valid_until_days_or_default(days, _phase), do: days

  @spec phase_config_result({:ok, PhaseConfig.t()} | {:error, atom()}, Phase.t()) ::
          {:ok, PhaseConfig.t()} | {:error, [CompilerError.t()]}
  defp phase_config_result({:ok, phase_config}, _phase), do: {:ok, phase_config}

  defp phase_config_result({:error, :invalid_max_turns}, _phase),
    do: {:error, [:max_turns_invalid]}

  defp phase_config_result({:error, :single_turn_phase_max_turns_must_be_one}, _phase) do
    {:error, [:single_turn_phase_max_turns_must_be_one]}
  end

  defp phase_config_result({:error, reason}, phase) do
    {:error, [{:invalid_phase_config, phase, reason}]}
  end

  @spec validate_prompts(%{atom() => String.t()}, %{Phase.t() => PhaseConfig.t()}) ::
          :ok | {:error, [CompilerError.t()]}
  defp validate_prompts(prompts, phase_configs) do
    errors =
      phase_configs
      |> Map.keys()
      |> Enum.flat_map(&prompt_errors(&1, prompts))

    if errors == [] do
      :ok
    else
      {:error, errors}
    end
  end

  @spec prompt_errors(Phase.t(), %{atom() => String.t()}) :: [CompilerError.t()]
  defp prompt_errors(phase, prompts) do
    case Map.fetch(prompts, phase) do
      {:ok, prompt} -> prompt_variable_errors(phase, prompt)
      :error -> [{:missing_prompt, phase}]
    end
  end

  @spec prompt_variable_errors(Phase.t(), String.t()) :: [CompilerError.t()]
  defp prompt_variable_errors(phase, prompt) do
    allowed = Map.fetch!(@prompt_variables, phase)

    prompt
    |> prompt_variables()
    |> Enum.reject(&(&1 in allowed))
    |> Enum.map(&{:unknown_prompt_variable, phase, &1})
  end

  @spec prompt_variables(String.t()) :: [String.t()]
  defp prompt_variables(prompt) do
    ~r/{{\s*([^}]+?)\s*}}/
    |> Regex.scan(prompt, capture: :all_but_first)
    |> List.flatten()
    |> Enum.map(&String.trim/1)
    |> Enum.uniq()
  end

  @spec validate_external_publication(map()) :: :ok | {:error, [CompilerError.t()]}
  defp validate_external_publication(config) do
    config
    |> normalized_external_publication()
    |> external_publication_errors()
    |> errors_result()
  end

  @spec external_publication_errors(map()) :: [CompilerError.t()]
  defp external_publication_errors(%{branch_regex: branch_regex, approvers: approvers})
       when is_binary(branch_regex) and branch_regex != "" and approvers == [] do
    [:missing_approvers]
  end

  defp external_publication_errors(_external_publication), do: []

  @spec bundle(map(), Workflow.t(), %{Phase.t() => PhaseConfig.t()}, %{atom() => String.t()}) ::
          OpenSleigh.WorkflowStore.bundle()
  defp bundle(config, workflow, phase_configs, raw_prompts) do
    config_hashes = config_hashes(config, phase_configs, raw_prompts)

    %{
      phase_configs: phase_configs,
      prompts: prompts_with_hash(raw_prompts, config_hashes),
      config_hashes: config_hashes,
      external_publication: normalized_external_publication(config),
      engine: value_at(config, :engine, %{}),
      tracker: value_at(config, :tracker, %{}),
      agent: value_at(config, :agent, %{}),
      codex: value_at(config, :codex, %{}),
      judge: value_at(config, :judge, %{}),
      hooks: value_at(config, :hooks, %{}),
      haft: value_at(config, :haft, %{}),
      workspace: value_at(config, :workspace, %{}),
      workflow: workflow.id
    }
  end

  @spec config_hashes(map(), %{Phase.t() => PhaseConfig.t()}, %{atom() => String.t()}) ::
          %{Phase.t() => ConfigHash.t()}
  defp config_hashes(config, phase_configs, raw_prompts) do
    Map.new(phase_configs, fn {phase, _phase_config} ->
      hash =
        config
        |> hash_input(phase, Map.fetch!(raw_prompts, phase))
        |> :erlang.term_to_binary()
        |> ConfigHash.from_iodata()

      {phase, hash}
    end)
  end

  @spec hash_input(map(), Phase.t(), String.t()) :: map()
  defp hash_input(config, phase, prompt) do
    %{
      engine: value_at(config, :engine, %{}),
      tracker: value_at(config, :tracker, %{}),
      agent: value_at(config, :agent, %{}),
      codex: value_at(config, :codex, %{}),
      judge: value_at(config, :judge, %{}),
      hooks: value_at(config, :hooks, %{}),
      haft: value_at(config, :haft, %{}),
      workspace: value_at(config, :workspace, %{}),
      external_publication: value_at(config, :external_publication, %{}),
      phase: phase_config_input(value_at(config, :phases, %{}), phase),
      prompt: prompt
    }
  end

  @spec prompts_with_hash(%{atom() => String.t()}, %{Phase.t() => ConfigHash.t()}) ::
          %{Phase.t() => String.t()}
  defp prompts_with_hash(raw_prompts, config_hashes) do
    Map.new(config_hashes, fn {phase, hash} ->
      prompt =
        raw_prompts
        |> Map.fetch!(phase)
        |> append_config_hash(hash)

      {phase, prompt}
    end)
  end

  @spec append_config_hash(String.t(), ConfigHash.t()) :: String.t()
  defp append_config_hash(prompt, hash) do
    prompt <> "\n\n<!-- config_hash: #{hash} -->"
  end

  @spec normalized_external_publication(map()) :: map()
  defp normalized_external_publication(config) do
    external_publication = value_at(config, :external_publication, %{})

    %{
      branch_regex: value_at(external_publication, :branch_regex),
      tracker_transition_to: value_at(external_publication, :tracker_transition_to, []),
      approvers: value_at(external_publication, :approvers, []),
      timeout_h: value_at(external_publication, :timeout_h)
    }
  end

  @spec unknown_phase_errors(map(), Workflow.t()) :: [CompilerError.t()]
  defp unknown_phase_errors(phase_section, workflow) do
    phase_section
    |> Map.keys()
    |> Enum.reject(&phase_key_known?(workflow, &1))
    |> Enum.map(&{:unknown_phase, config_string(&1)})
  end

  @spec missing_phase_errors(map(), Workflow.t()) :: [CompilerError.t()]
  defp missing_phase_errors(phase_section, workflow) do
    workflow
    |> required_phases()
    |> Enum.reject(&phase_key_present?(phase_section, &1))
    |> Enum.map(&{:missing_phase, &1})
  end

  @spec phase_key_known?(Workflow.t(), term()) :: boolean()
  defp phase_key_known?(workflow, key) do
    key
    |> config_string()
    |> known_atom_from_string(workflow.phases)
    |> known_atom?()
  end

  @spec known_atom?(atom() | nil) :: boolean()
  defp known_atom?(atom) when is_atom(atom) and not is_nil(atom), do: true
  defp known_atom?(_), do: false

  @spec phase_key_present?(map(), Phase.t()) :: boolean()
  defp phase_key_present?(phase_section, phase) do
    phase_section
    |> value_at(phase)
    |> is_map()
  end

  @spec phase_result_errors({:ok, {Phase.t(), PhaseConfig.t()}} | {:error, [CompilerError.t()]}) ::
          [CompilerError.t()]
  defp phase_result_errors({:ok, _pair}), do: []
  defp phase_result_errors({:error, errors}), do: errors

  @spec phase_result_pair({:ok, {Phase.t(), PhaseConfig.t()}} | {:error, [CompilerError.t()]}) ::
          {Phase.t(), PhaseConfig.t()}
  defp phase_result_pair({:ok, pair}), do: pair

  @spec tool_result_errors({:ok, atom()} | {:error, CompilerError.t()}) :: [CompilerError.t()]
  defp tool_result_errors({:ok, _tool}), do: []
  defp tool_result_errors({:error, error}), do: [error]

  @spec tool_result_atom({:ok, atom()} | {:error, CompilerError.t()}) :: atom()
  defp tool_result_atom({:ok, tool}), do: tool

  @spec gate_result_errors({:ok, atom()} | {:error, CompilerError.t()}) :: [CompilerError.t()]
  defp gate_result_errors({:ok, _gate}), do: []
  defp gate_result_errors({:error, error}), do: [error]

  @spec gate_result_atom({:ok, atom()} | {:error, CompilerError.t()}) :: atom()
  defp gate_result_atom({:ok, gate}), do: gate

  @spec known_atom_from_string(String.t(), [atom()]) :: atom() | nil
  defp known_atom_from_string(value, allowed_atoms) do
    Enum.find(allowed_atoms, &(Atom.to_string(&1) == value))
  end

  @spec value_at(term(), atom()) :: term()
  defp value_at(value, key), do: value_at(value, key, nil)

  @spec value_at(term(), atom(), term()) :: term()
  defp value_at(%{} = map, key, fallback) do
    Map.get(map, Atom.to_string(key), Map.get(map, key, fallback))
  end

  defp value_at(_value, _key, fallback), do: fallback

  @spec config_string(term()) :: String.t()
  defp config_string(value) when is_binary(value), do: value
  defp config_string(value) when is_atom(value), do: Atom.to_string(value)
  defp config_string(value), do: to_string(value)

  @spec errors_result([CompilerError.t()]) :: :ok | {:error, [CompilerError.t()]}
  defp errors_result([]), do: :ok
  defp errors_result(errors), do: {:error, errors}
end
