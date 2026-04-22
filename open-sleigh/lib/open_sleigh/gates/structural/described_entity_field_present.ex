defmodule OpenSleigh.Gates.Structural.DescribedEntityFieldPresent do
  @moduledoc """
  Structural gate — Frame exit. Checks that the **upstream**
  ProblemCard (referenced by `ticket.problem_card_ref` and fetched by
  L5 into `ctx.upstream_problem_card`) has non-empty
  `describedEntity` and `groundingHolon` fields.

  Per `specs/target-system/GATES.md §1`: "Upstream ProblemCard has
  non-empty `describedEntity` and `groundingHolon`. Checks upstream
  human-authored content; Open-Sleigh never creates these fields."

  This is a **field-presence** check only. Whether `describedEntity`
  is *specific* ("file path / module") vs vacuous ("the system") is
  the semantic gate `object_of_talk_is_specific` (`GATES.md §2`).
  """

  @behaviour OpenSleigh.Gates.Structural

  alias OpenSleigh.GateContext

  @gate_name :described_entity_field_present

  @impl true
  @spec gate_name() :: atom()
  def gate_name, do: @gate_name

  @impl true
  @spec apply(GateContext.t()) ::
          :ok
          | {:error, :no_upstream_problem_card}
          | {:error, :missing_described_entity}
          | {:error, :missing_grounding_holon}
  def apply(%GateContext{upstream_problem_card: nil}),
    do: {:error, :no_upstream_problem_card}

  def apply(%GateContext{upstream_problem_card: pc}) when is_map(pc) do
    with :ok <- check_non_empty(pc, "describedEntity", :missing_described_entity),
         :ok <- check_non_empty(pc, "groundingHolon", :missing_grounding_holon) do
      :ok
    end
  end

  @spec check_non_empty(map(), String.t(), atom()) :: :ok | {:error, atom()}
  defp check_non_empty(pc, field, error_atom) do
    case Map.get(pc, field) || Map.get(pc, String.to_existing_atom(field)) do
      value when is_binary(value) and byte_size(value) > 0 -> :ok
      _ -> {:error, error_atom}
    end
  rescue
    # String.to_existing_atom/1 raises if the atom isn't known — that
    # means the field is definitely not in the map.
    ArgumentError -> {:error, error_atom}
  end

  @impl true
  @spec description() :: String.t()
  def description,
    do: "Upstream ProblemCard has non-empty describedEntity and groundingHolon fields."
end
