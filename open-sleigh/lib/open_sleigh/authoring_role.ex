defmodule OpenSleigh.AuthoringRole do
  @moduledoc """
  Closed sum of authoring roles — who produced a given artifact.

  Per `specs/target-system/PHASE_ONTOLOGY.md` axis 4 and
  `specs/target-system/TERM_MAP.md`:

  * `:frame_verifier` — agent in Frame phase, **verifying** upstream
    human-authored framing. Renamed from `:framer` in v0.5 because the
    Frame-phase role is verification, not authorship (UP1–UP3 framing-
    ownership lock).
  * `:executor`   — agent in Execute phase; produces code changes + PR.
  * `:measurer`   — agent in Measure phase; assembles external evidence.
  * `:judge`      — LLM-judge behind a semantic gate.
  * `:human`      — the human principal, via HumanGateListener.

  `:open_sleigh_self` is **not** in the alphabet — it is a reserved
  blacklisted value per OB4 and UP3. `Evidence.new/5` explicitly rejects
  it; any artifact whose authoring-source resolves to Open-Sleigh's own
  telemetry is structurally refused from entering the Haft graph.
  """

  @typedoc "An authoring-role atom."
  @type t :: :frame_verifier | :executor | :measurer | :judge | :human

  @all [:frame_verifier, :executor, :measurer, :judge, :human]

  @doc "All admissible authoring-role atoms."
  @spec all() :: [t(), ...]
  def all, do: @all

  @doc "Is `value` a valid authoring role?"
  @spec valid?(term()) :: boolean()
  def valid?(value) when value in @all, do: true
  def valid?(_), do: false

  @doc """
  Is this role an agent adapter role? (As opposed to `:judge` which is
  also an agent but scoped to semantic-gate evaluation, or `:human` which
  is a separate principal.)
  """
  @spec agent_phase_role?(t()) :: boolean()
  def agent_phase_role?(role) when role in [:frame_verifier, :executor, :measurer], do: true
  def agent_phase_role?(_), do: false
end
