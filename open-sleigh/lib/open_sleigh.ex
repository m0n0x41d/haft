defmodule OpenSleigh do
  @moduledoc """
  Open-Sleigh — a long-running, OTP-supervised harness engine for AI coding
  agents that enforces an FPF-compliant governance lifecycle around every
  ticket the agent touches.

  This module is the library root. It defines no functions; all behaviour
  lives in the layered submodule tree:

      OpenSleigh.*            — L1 core types (this file's namespace)
      OpenSleigh.Gates.*      — L2 gate algebra
      OpenSleigh.PhaseMachine — L3 phase graph semantics
      OpenSleigh.Adapter.*    — L4 typed adapter boundary (stateless)
      OpenSleigh.*            — L5 OTP processes (Orchestrator, AgentWorker, …)
      OpenSleigh.Sleigh.*     — L6 operator DSL compiler

  The authoritative description of what Open-Sleigh is and how its layers
  compose lives in the spec set at:

    * `SPEC.md`                                     — operator-facing umbrella
    * `specs/target-system/`                        — target-system decomposition
    * `specs/enabling-system/FUNCTIONAL_ARCHITECTURE.md` — layer hierarchy
    * `specs/enabling-system/IMPLEMENTATION_PLAN.md`    — L1 → L6 build order

  When the spec set and this code disagree, the spec is the source of truth
  for design intent. Drift in either direction is a review finding.
  """
end
