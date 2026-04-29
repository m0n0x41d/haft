defmodule OpenSleigh.PhaseConfig do
  @moduledoc """
  Per-phase compiled configuration: agent role, scoped tool set, gate
  bindings, prompt-template key, turn limits.

  Per `specs/target-system/TARGET_SYSTEM_MODEL.md §PhaseConfig` +
  `ILLEGAL_STATES.md` CF3–CF5.

  Produced by `OpenSleigh.Sleigh.Compiler.compile/1` (L6) when
  `sleigh.md` is parsed. L1 defines the struct shape; L6 resolves
  gate-name atoms against the `OpenSleigh.Gates.Registry` and tool
  atoms against the declared adapter's `@tool_registry`.
  """

  alias OpenSleigh.{AuthoringRole, Phase}

  @typedoc """
  Gate bindings by kind. Each value is a list of gate-name atoms that
  must be in `OpenSleigh.Gates.Registry` at L6 compile time (CF3/GK4).
  """
  @type gates :: %{
          structural: [atom()],
          semantic: [atom()],
          human: [atom()]
        }

  @enforce_keys [
    :phase,
    :agent_role,
    :tools,
    :gates,
    :prompt_template_key,
    :max_turns,
    :default_valid_until_days
  ]
  defstruct [
    :phase,
    :agent_role,
    :tools,
    :gates,
    :prompt_template_key,
    :max_turns,
    :default_valid_until_days
  ]

  @type t :: %__MODULE__{
          phase: Phase.t(),
          agent_role: AuthoringRole.t(),
          tools: [atom()],
          gates: gates(),
          prompt_template_key: atom(),
          max_turns: pos_integer(),
          default_valid_until_days: pos_integer()
        }

  @type new_error ::
          :invalid_phase
          | :invalid_agent_role
          | :invalid_tools
          | :invalid_gates
          | :invalid_prompt_template_key
          | :invalid_max_turns
          | :invalid_default_valid_until_days
          | :single_turn_phase_max_turns_must_be_one

  @doc """
  Construct a `PhaseConfig`. Enforces per-phase constraints:

  * `max_turns >= 1` (CT3)
  * If the phase is single-turn (`Phase.single_turn?/1`), `max_turns
    == 1` (CT4)
  * `agent_role` is in `AuthoringRole.agent_phase_role?/1`
    — you can't assign `:human` or `:judge` as the turn's agent role
  """
  @spec new(keyword() | map()) :: {:ok, t()} | {:error, new_error()}
  def new(attrs) when is_list(attrs), do: new(Map.new(attrs))

  def new(%{} = attrs) do
    with :ok <- validate_phase(attrs[:phase]),
         :ok <- validate_agent_role(attrs[:agent_role]),
         :ok <- validate_tools(attrs[:tools]),
         :ok <- validate_gates(attrs[:gates]),
         :ok <- validate_prompt_template_key(attrs[:prompt_template_key]),
         :ok <- validate_max_turns(attrs[:max_turns]),
         :ok <- validate_max_turns_vs_phase(attrs[:phase], attrs[:max_turns]),
         :ok <- validate_valid_until_days(attrs[:default_valid_until_days]) do
      {:ok,
       %__MODULE__{
         phase: attrs.phase,
         agent_role: attrs.agent_role,
         tools: attrs.tools,
         gates: attrs.gates,
         prompt_template_key: attrs.prompt_template_key,
         max_turns: attrs.max_turns,
         default_valid_until_days: attrs.default_valid_until_days
       }}
    end
  end

  @spec validate_phase(term()) :: :ok | {:error, :invalid_phase}
  defp validate_phase(phase) do
    if Phase.valid?(phase), do: :ok, else: {:error, :invalid_phase}
  end

  @spec validate_agent_role(term()) :: :ok | {:error, :invalid_agent_role}
  defp validate_agent_role(role) do
    if AuthoringRole.valid?(role) and AuthoringRole.agent_phase_role?(role) do
      :ok
    else
      {:error, :invalid_agent_role}
    end
  end

  @spec validate_tools(term()) :: :ok | {:error, :invalid_tools}
  defp validate_tools(tools) when is_list(tools) do
    if Enum.all?(tools, &(is_atom(&1) and not is_nil(&1))),
      do: :ok,
      else: {:error, :invalid_tools}
  end

  defp validate_tools(_), do: {:error, :invalid_tools}

  @spec validate_gates(term()) :: :ok | {:error, :invalid_gates}
  defp validate_gates(%{structural: s, semantic: se, human: h})
       when is_list(s) and is_list(se) and is_list(h) do
    all_atoms? =
      Enum.all?(s ++ se ++ h, &(is_atom(&1) and not is_nil(&1)))

    if all_atoms?, do: :ok, else: {:error, :invalid_gates}
  end

  defp validate_gates(_), do: {:error, :invalid_gates}

  @spec validate_prompt_template_key(term()) :: :ok | {:error, :invalid_prompt_template_key}
  defp validate_prompt_template_key(key) when is_atom(key) and not is_nil(key), do: :ok
  defp validate_prompt_template_key(_), do: {:error, :invalid_prompt_template_key}

  @spec validate_max_turns(term()) :: :ok | {:error, :invalid_max_turns}
  defp validate_max_turns(n) when is_integer(n) and n >= 1, do: :ok
  defp validate_max_turns(_), do: {:error, :invalid_max_turns}

  @spec validate_max_turns_vs_phase(term(), term()) ::
          :ok | {:error, :single_turn_phase_max_turns_must_be_one}
  defp validate_max_turns_vs_phase(phase, max_turns) when is_integer(max_turns) do
    cond do
      not Phase.valid?(phase) ->
        :ok

      Phase.single_turn?(phase) and max_turns > 1 ->
        {:error, :single_turn_phase_max_turns_must_be_one}

      true ->
        :ok
    end
  end

  defp validate_max_turns_vs_phase(_, _), do: :ok

  @spec validate_valid_until_days(term()) ::
          :ok | {:error, :invalid_default_valid_until_days}
  defp validate_valid_until_days(n) when is_integer(n) and n >= 1, do: :ok
  defp validate_valid_until_days(_), do: {:error, :invalid_default_valid_until_days}

  @doc """
  Compute the default `valid_until` datetime from a reference
  `produced_at`. Pure: time is always a parameter.
  """
  @spec default_valid_until(t(), DateTime.t()) :: DateTime.t()
  def default_valid_until(%__MODULE__{default_valid_until_days: days}, %DateTime{} = produced_at) do
    DateTime.add(produced_at, days * 86_400, :second)
  end

  @doc "Does this phase's config declare any human gate?"
  @spec declares_human_gate?(t()) :: boolean()
  def declares_human_gate?(%__MODULE__{gates: %{human: human_gates}}) do
    human_gates != []
  end
end
