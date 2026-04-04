package agent

import "time"

// ---------------------------------------------------------------------------
// L1: Cycle Derivation — state functions for cycle management.
//
// No DB access. Depends on L0 types only.
// The coordinator (L4) calls these to derive phase transitions and
// bind artifact refs from typed tool results.
// ---------------------------------------------------------------------------

// DerivePhaseFromCycle determines the current phase purely from cycle state.
// All phases are mandatory (FPF B.5.1 CC-B5.1.2). No phase skipping.
// Ceremony density varies by problem characteristics, not by skipping phases.
func DerivePhaseFromCycle(cycle *Cycle) Phase {
	if cycle == nil || cycle.Status != CycleActive {
		return PhaseReady
	}
	if cycle.DecisionRef != "" {
		return PhaseWorker
	}
	if cycle.PortfolioRef != "" {
		if cycle.ComparedPortfolioRef == cycle.PortfolioRef {
			return PhaseDecider
		}
		return PhaseExplorer
	}
	if cycle.ProblemRef != "" {
		return PhaseExplorer
	}
	return PhaseFramer
}

// ValidateTransition checks if moving to the proposed phase is legal
// given the current cycle state. Returns nil if valid, error if not.
func ValidateTransition(cycle *Cycle, proposed Phase) error {
	if cycle == nil {
		return nil // no cycle = no constraints
	}
	current := DerivePhaseFromCycle(cycle)

	// Same phase is always valid (re-entering)
	if proposed == current {
		return nil
	}

	// PhaseReady is always valid (cycle complete or no cycle)
	if proposed == PhaseReady {
		return nil
	}

	// Can't go backward in the cycle
	order := phaseOrder(proposed)
	currentOrder := phaseOrder(current)
	if order < currentOrder {
		return &TransitionError{From: current, To: proposed, Reason: "backward transition"}
	}

	return nil
}

// TransitionError represents an invalid phase transition attempt.
type TransitionError struct {
	From   Phase
	To     Phase
	Reason string
}

func (e *TransitionError) Error() string {
	return "invalid transition " + string(e.From) + " → " + string(e.To) + ": " + e.Reason
}

func phaseOrder(p Phase) int {
	switch p {
	case PhaseReady:
		return 0
	case PhaseFramer:
		return 1
	case PhaseExplorer:
		return 2
	case PhaseDecider:
		return 3
	case PhaseWorker:
		return 4
	case PhaseMeasure:
		return 5
	default:
		return -1
	}
}

// PhaseAfter returns true if `a` comes after `b` in the phase sequence.
func PhaseAfter(a, b Phase) bool {
	return phaseOrder(a) > phaseOrder(b)
}

// BindArtifact updates the cycle's artifact ref based on typed tool result meta.
// Returns a new Cycle (does not mutate input). Returns nil if meta doesn't bind.
func BindArtifact(cycle *Cycle, meta ArtifactMeta) *Cycle {
	if cycle == nil {
		return nil
	}
	updated := *cycle // shallow copy
	updated.UpdatedAt = time.Now().UTC()

	bound := false
	switch meta.Kind {
	case "problem":
		if meta.Operation == "frame" {
			updated.ProblemRef = meta.ArtifactRef
			bound = true
		}
		if meta.Operation == "adopt" {
			updated.ProblemRef = meta.ArtifactRef
			updated.PortfolioRef = meta.AdoptPortfolioRef
			updated.ComparedPortfolioRef = meta.ComparedPortfolioRef
			updated.DecisionRef = meta.AdoptDecisionRef
			bound = true
		}
	case "solution":
		if meta.Operation == "explore" {
			updated.PortfolioRef = meta.ArtifactRef
			updated.ComparedPortfolioRef = ""
			bound = true
		}
		if meta.Operation == "compare" {
			updated.ComparedPortfolioRef = meta.ComparedPortfolioRef
			bound = updated.ComparedPortfolioRef != ""
		}
	case "decision":
		if meta.Operation == "decide" {
			updated.DecisionRef = meta.ArtifactRef
			bound = true
		}
	}
	if !bound {
		return nil
	}

	// Derive new phase from updated state
	updated.Phase = DerivePhaseFromCycle(&updated)
	return &updated
}

// IsSkipLegal always returns an error — phases cannot be skipped (FPF B.5.1 CC-B5.1.2).
// Ceremony density varies, but all phases are mandatory.
func IsSkipLegal(_ Depth, skipped Phase) error {
	return &TransitionError{
		From:   skipped,
		To:     skipped,
		Reason: "phase " + string(skipped) + " cannot be skipped (FPF B.5.1 CC-B5.1.2)",
	}
}

// BuildGovernanceEntry creates a governance record from framer recommendation
// and user response.
func BuildGovernanceEntry(recommended, chosen Depth, mode Interaction, skipped []Phase) GovernanceEntry {
	chosenBy := "user"
	if mode == InteractionAutonomous {
		chosenBy = "autonomous_delegation"
	}
	return GovernanceEntry{
		Recommended:   recommended,
		Chosen:        chosen,
		ChosenBy:      chosenBy,
		Mode:          mode,
		SkippedPhases: skipped,
		Timestamp:     time.Now().UTC(),
	}
}

// BuildSkipEntry creates a structured skip justification.
func BuildSkipEntry(phase Phase, reason, acceptedRisk, residualEvidence, reopenTrigger string) SkipEntry {
	return SkipEntry{
		Phase:            phase,
		Reason:           reason,
		AcceptedRisk:     acceptedRisk,
		ResidualEvidence: residualEvidence,
		ReopenTrigger:    reopenTrigger,
	}
}

// CompleteCycle marks a cycle as complete after successful measurement.
func CompleteCycle(cycle *Cycle, weakestLink string, assurance AssuranceTuple) *Cycle {
	updated := *cycle
	updated.Status = CycleComplete
	updated.Phase = PhaseReady
	updated.WeakestLink = weakestLink
	updated.Assurance = assurance
	updated.REff = assurance.R
	updated.UpdatedAt = time.Now().UTC()
	return &updated
}

// AbandonCycle marks a cycle as abandoned (measurement failed, reframing needed).
func AbandonCycle(cycle *Cycle) *Cycle {
	updated := *cycle
	updated.Status = CycleAbandoned
	updated.UpdatedAt = time.Now().UTC()
	return &updated
}

// NewCycleFromLineage creates a new cycle linked to a failed predecessor.
func NewCycleFromLineage(id, sessionID string, failed *Cycle) *Cycle {
	now := time.Now().UTC()
	return &Cycle{
		ID:         id,
		SessionID:  sessionID,
		Phase:      PhaseFramer,
		Depth:      failed.Depth,
		Status:     CycleActive,
		LineageRef: failed.ID,
		CLMin:      3,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

// AdoptCycle creates a cycle that picks up existing artifacts.
// Used when a new session continues work on a problem framed in another session.
// Phase is derived from which refs are populated.
func AdoptCycle(id, sessionID, problemRef, portfolioRef, decisionRef string) *Cycle {
	now := time.Now().UTC()
	c := &Cycle{
		ID:           id,
		SessionID:    sessionID,
		ProblemRef:   problemRef,
		PortfolioRef: portfolioRef,
		DecisionRef:  decisionRef,
		Status:       CycleActive,
		CLMin:        3,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	c.Phase = DerivePhaseFromCycle(c)
	return c
}

// ---------------------------------------------------------------------------
// Phase→Spec mapping table
// ---------------------------------------------------------------------------

// phaseSpecs is the canonical mapping from runtime phases to FPF spec concepts.
var phaseSpecs = map[Phase]PhaseSpec{
	PhaseFramer: {
		Phase:            PhaseFramer,
		FPFStage:         "Explore/Shape",
		ADIRole:          "abduction",
		AllowedCreate:    []string{"problem"},
		AllowedUpdate:    nil,
		CompletionSignal: "problem",
		SkipDepths:       nil, // framer never skipped
	},
	PhaseExplorer: {
		Phase:            PhaseExplorer,
		FPFStage:         "Explore",
		ADIRole:          "abduction",
		AllowedCreate:    []string{"solution"},
		AllowedUpdate:    []string{"problem"},
		CompletionSignal: "solution",
		SkipDepths:       nil, // explorer is mandatory (FPF B.5.2 CC-B5.2-2)
	},
	PhaseDecider: {
		Phase:            PhaseDecider,
		FPFStage:         "Shape/Evidence",
		ADIRole:          "deduction",
		AllowedCreate:    []string{"decision"},
		AllowedUpdate:    []string{"solution"},
		CompletionSignal: "decision",
		SkipDepths:       nil, // decider never skipped
	},
	PhaseWorker: {
		Phase:            PhaseWorker,
		FPFStage:         "Operate",
		ADIRole:          "",
		AllowedCreate:    []string{"note"},
		AllowedUpdate:    nil,
		CompletionSignal: "", // write/edit tool success
		SkipDepths:       nil,
	},
	PhaseMeasure: {
		Phase:            PhaseMeasure,
		FPFStage:         "Evidence",
		ADIRole:          "induction",
		AllowedCreate:    []string{"evidence"},
		AllowedUpdate:    []string{"decision"},
		CompletionSignal: "measure",
		SkipDepths:       nil, // measure never skipped
	},
}

// SpecForPhase returns the PhaseSpec for a given phase.
func SpecForPhase(phase Phase) PhaseSpec {
	if spec, ok := phaseSpecs[phase]; ok {
		return spec
	}
	return PhaseSpec{Phase: phase}
}

// IsArtifactAllowed checks if a phase may create/update an artifact of the given kind.
func IsArtifactAllowed(phase Phase, artifactKind string) bool {
	spec := SpecForPhase(phase)
	for _, k := range spec.AllowedCreate {
		if k == artifactKind {
			return true
		}
	}
	for _, k := range spec.AllowedUpdate {
		if k == artifactKind {
			return true
		}
	}
	return false
}
