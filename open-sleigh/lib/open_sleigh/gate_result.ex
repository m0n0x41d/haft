defmodule OpenSleigh.GateResult do
  @moduledoc """
  Closed sum over the three `GateKind` variants. Pattern-match on the
  kind tag first — untyped merges are a type error per GK1.

  Per `specs/target-system/TARGET_SYSTEM_MODEL.md §GateResult`:

      @type t ::
              {:structural, :ok}
            | {:structural, {:error, reason :: atom()}}
            | {:semantic, %{verdict, cl, rationale}}
            | {:semantic, {:error, reason :: atom()}}
            | {:human, HumanGateApproval.t()}
            | {:human, :rejected}
            | {:human, :timeout}

  `combine/1` is kind-aware: it returns `{:advance | :block |
  :await_human, reasons}` — a sum, not a boolean (GK6).
  """

  alias OpenSleigh.HumanGateApproval

  @typedoc "Semantic-gate payload (pass-verdict form)."
  @type semantic_payload :: %{
          verdict: OpenSleigh.Verdict.t(),
          cl: 0..3,
          rationale: String.t()
        }

  @typedoc "A single gate result."
  @type t ::
          {:structural, :ok}
          | {:structural, {:error, atom()}}
          | {:semantic, semantic_payload()}
          | {:semantic, {:error, atom()}}
          | {:human, HumanGateApproval.t()}
          | {:human, :rejected}
          | {:human, :timeout}

  @typedoc "Combined decision after evaluating a list of results."
  @type decision :: :advance | :block | :await_human

  @doc "Is `value` a valid `GateResult`?"
  @spec valid?(term()) :: boolean()
  def valid?({:structural, :ok}), do: true
  def valid?({:structural, {:error, atom}}) when is_atom(atom) and not is_nil(atom), do: true

  def valid?({:semantic, %{verdict: v, cl: cl, rationale: r}})
      when is_integer(cl) and cl >= 0 and cl <= 3 and is_binary(r) do
    OpenSleigh.Verdict.valid?(v)
  end

  def valid?({:semantic, {:error, atom}}) when is_atom(atom) and not is_nil(atom), do: true
  def valid?({:human, %HumanGateApproval{}}), do: true
  def valid?({:human, :rejected}), do: true
  def valid?({:human, :timeout}), do: true
  def valid?(_), do: false

  @doc """
  Pattern-match on the kind tag to identify a result.
  """
  @spec kind(t()) :: OpenSleigh.GateKind.t()
  def kind({:structural, _}), do: :structural
  def kind({:semantic, _}), do: :semantic
  def kind({:human, _}), do: :human

  @doc """
  Did this single result pass? The answer depends on the kind — a
  semantic `:fail` verdict is a pass-shape (well-formed response) but
  a fail-verdict, whereas a structural `{:error, _}` is a hard fail.
  """
  @spec pass?(t()) :: boolean()
  def pass?({:structural, :ok}), do: true
  def pass?({:structural, {:error, _}}), do: false
  def pass?({:semantic, %{verdict: :pass}}), do: true
  def pass?({:semantic, %{verdict: _}}), do: false
  def pass?({:semantic, {:error, _}}), do: false
  def pass?({:human, %HumanGateApproval{}}), do: true
  def pass?({:human, :rejected}), do: false
  def pass?({:human, :timeout}), do: false

  @doc """
  Combine a list of `GateResult`s into a single decision.

  Rules (per `PHASE_ONTOLOGY.md §Combining gate results`):

  * Any `:structural` failure → `:block` (blocks without retry —
    indicates a bug in prompt or adapter).
  * Any `:semantic` failure → `:block` (blocks with retry — agent
    asked to revise).
  * Any `:human` that's neither an approval nor a timeout (i.e.
    `:rejected`) → `:block` (regress to previous phase).
  * Any `:human` with `:timeout` → `:block`.
  * Any pending human gate (approval struct missing) is represented
    in-band by the caller NOT including it; if a human gate was
    declared and no tuple for it is present, the caller should emit
    `:await_human` upstream. This function assumes a complete list of
    results for gates that actually ran.
  * Otherwise → `:advance`.

  Returns `{:advance | :block, [reasons]}` where `reasons` is an
  empty list on `:advance`.
  """
  @spec combine([t()]) :: {decision(), [term()]}
  def combine(results) when is_list(results) do
    failures = Enum.reject(results, &pass?/1)

    case failures do
      [] ->
        {:advance, []}

      _ ->
        reasons = Enum.map(failures, &reason_of/1)
        {:block, reasons}
    end
  end

  @spec reason_of(t()) :: term()
  defp reason_of({:structural, {:error, r}}), do: {:structural, r}
  defp reason_of({:semantic, %{verdict: v, rationale: r}}), do: {:semantic, v, r}
  defp reason_of({:semantic, {:error, r}}), do: {:semantic, :error, r}
  defp reason_of({:human, :rejected}), do: {:human, :rejected}
  defp reason_of({:human, :timeout}), do: {:human, :timeout}
end
