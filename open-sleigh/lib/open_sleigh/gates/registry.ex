defmodule OpenSleigh.Gates.Registry do
  @moduledoc """
  Compile-time registry of gate-name atoms → implementing modules.

  Per `specs/target-system/ILLEGAL_STATES.md` CF3 + GK4: the L6
  `Sleigh.Compiler` uses this registry to validate that every gate
  name in `sleigh.md` resolves to an implementation. Unknown names
  fail compilation with `{:error, :unknown_gate, name}`.

  Adding a gate is two steps:
  1. Create the module implementing the behaviour.
  2. Add one line to the appropriate map below.

  Adding it to only one half (module exists but not registered, or
  registered but no module) is caught by `mix credo` (readability
  rules) + the L6 compiler (CF3).
  """

  alias OpenSleigh.Gates.Human.CommissionApproved

  alias OpenSleigh.Gates.Semantic.{
    LadeQuadrantsSplitOk,
    NoSelfEvidenceSemantic,
    ObjectOfTalkIsSpecific
  }

  alias OpenSleigh.Gates.Structural.{
    DescribedEntityFieldPresent,
    DesignRuntimeSplitOk,
    EvidenceRefNotSelf,
    ProblemCardRefPresent,
    ValidUntilFieldPresent
  }

  @structural %{
    problem_card_ref_present: ProblemCardRefPresent,
    described_entity_field_present: DescribedEntityFieldPresent,
    valid_until_field_present: ValidUntilFieldPresent,
    design_runtime_split_ok: DesignRuntimeSplitOk,
    evidence_ref_not_self: EvidenceRefNotSelf
  }

  @semantic %{
    object_of_talk_is_specific: ObjectOfTalkIsSpecific,
    lade_quadrants_split_ok: LadeQuadrantsSplitOk,
    no_self_evidence_semantic: NoSelfEvidenceSemantic
  }

  @human %{
    commission_approved: CommissionApproved
  }

  @doc "Resolve a structural gate atom to its module, or `{:error, :unknown_gate}`."
  @spec structural_module(atom()) :: {:ok, module()} | {:error, :unknown_gate}
  def structural_module(name) when is_map_key(@structural, name),
    do: {:ok, Map.fetch!(@structural, name)}

  def structural_module(_), do: {:error, :unknown_gate}

  @doc "Resolve a semantic gate atom to its module."
  @spec semantic_module(atom()) :: {:ok, module()} | {:error, :unknown_gate}
  def semantic_module(name) when is_map_key(@semantic, name),
    do: {:ok, Map.fetch!(@semantic, name)}

  def semantic_module(_), do: {:error, :unknown_gate}

  @doc "Resolve a human gate atom to its module."
  @spec human_module(atom()) :: {:ok, module()} | {:error, :unknown_gate}
  def human_module(name) when is_map_key(@human, name),
    do: {:ok, Map.fetch!(@human, name)}

  def human_module(_), do: {:error, :unknown_gate}

  @doc "All registered structural gate atoms."
  @spec structural_gates() :: [atom()]
  def structural_gates, do: Map.keys(@structural)

  @doc "All registered semantic gate atoms."
  @spec semantic_gates() :: [atom()]
  def semantic_gates, do: Map.keys(@semantic)

  @doc "All registered human gate atoms."
  @spec human_gates() :: [atom()]
  def human_gates, do: Map.keys(@human)

  @doc """
  Is `name` known to the registry (any kind)? Used by L6 compiler.
  """
  @spec known?(atom()) :: boolean()
  def known?(name) do
    is_map_key(@structural, name) or is_map_key(@semantic, name) or
      is_map_key(@human, name)
  end
end
