package cli

import (
	"context"
	"fmt"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/projectprofile"
)

type processCheckProjectMethodResultSet struct {
	HardGates    ProcessCheckResult
	CarryThrough ProcessCheckResult
}

func processCheckProjectMethodResults(
	ctx context.Context,
	store *artifact.Store,
	observedAt string,
	validUntil string,
) processCheckProjectMethodResultSet {
	hardGates := processCheckMethodRunHardGates(
		ctx,
		store,
		observedAt,
		validUntil,
	)
	carryThrough := processCheckCarryThroughAcceptancePosture(
		ctx,
		store,
		observedAt,
		validUntil,
	)
	return processCheckProjectMethodResultSet{
		HardGates:    hardGates,
		CarryThrough: carryThrough,
	}
}

func processCheckProjectMethodResultsForProfileResolution(
	ctx context.Context,
	store *artifact.Store,
	observedAt string,
	validUntil string,
	resolution projectSpecificationApplicabilityResolution,
) (processCheckProjectMethodResultSet, error) {
	applicability, _, resolved := resolution.Resolved()
	if !resolved {
		readiness := canonicalProjectReadiness{
			profileEvaluated: true,
			resolution:       resolution,
		}
		return processCheckProjectMethodProfileCue(
			observedAt,
			validUntil,
			readiness.profileCue(),
			nil,
		), nil
	}
	processApplicability, err := applicability.ScopedCapabilityApplicability(
		projectprofile.ProcessChecksCapability,
	)
	if err != nil {
		return processCheckProjectMethodResultSet{}, err
	}
	builders := map[projectprofile.CapabilityApplicabilityKind]func() processCheckProjectMethodResultSet{
		projectprofile.CapabilityRequired: func() processCheckProjectMethodResultSet {
			return processCheckProjectMethodResults(
				ctx,
				store,
				observedAt,
				validUntil,
			)
		},
		projectprofile.CapabilityNotApplicable: func() processCheckProjectMethodResultSet {
			return processCheckProjectMethodNotApplicable(
				observedAt,
				validUntil,
				processApplicability,
			)
		},
		projectprofile.CapabilityUnderdetermined: func() processCheckProjectMethodResultSet {
			scopeID := processApplicability.ScopeID()
			scopeIDText := scopeID.String()
			cue := fmt.Sprintf(
				"Process-check applicability is underdetermined in exact project-profile scope %q.",
				scopeIDText,
			)
			applicabilities := []projectprofile.ScopedCapabilityApplicability{
				processApplicability,
			}
			return processCheckProjectMethodProfileCue(
				observedAt,
				validUntil,
				cue,
				applicabilities,
			)
		},
	}
	build, found := builders[processApplicability.Kind()]
	if !found {
		return processCheckProjectMethodResultSet{}, fmt.Errorf(
			"process-check applicability kind %q is unsupported",
			processApplicability.Kind(),
		)
	}
	return build(), nil
}

func processCheckProjectMethodProfileUnavailable(
	observedAt string,
	validUntil string,
) processCheckProjectMethodResultSet {
	readiness := canonicalProjectReadiness{
		profileEvaluated:   true,
		profileUnavailable: true,
	}
	return processCheckProjectMethodProfileCue(
		observedAt,
		validUntil,
		readiness.profileCue(),
		nil,
	)
}

func processCheckProjectMethodProfileCue(
	observedAt string,
	validUntil string,
	cue string,
	applicabilities []projectprofile.ScopedCapabilityApplicability,
) processCheckProjectMethodResultSet {
	evidence := scopedCapabilityApplicabilityEvidence(applicabilities)
	hardGates := processCheckResult(
		"method_run_hard_gates",
		"MethodRun",
		"project_method_runs",
		processCheckStatusUnknown,
		"info",
		observedAt,
		validUntil,
		cue+" SWE MethodRun hard-gate records were not scanned.",
		evidence,
		"No SWE process-check action; establish the named profile basis only when this capability is current.",
	)
	carryThrough := processCheckResult(
		"carry_through_acceptance_ref_posture",
		"MethodRun.carry_through",
		"project_method_runs",
		processCheckStatusUnknown,
		"info",
		observedAt,
		validUntil,
		cue+" SWE MethodRun carry-through records were not scanned.",
		evidence,
		"No SWE process-check action; establish the named profile basis only when this capability is current.",
	)
	return processCheckProjectMethodResultSet{
		HardGates:    hardGates,
		CarryThrough: carryThrough,
	}
}

func processCheckProjectMethodNotApplicable(
	observedAt string,
	validUntil string,
	applicability projectprofile.ScopedCapabilityApplicability,
) processCheckProjectMethodResultSet {
	scopeID := applicability.ScopeID()
	scopeIDText := scopeID.String()
	applicabilities := []projectprofile.ScopedCapabilityApplicability{
		applicability,
	}
	evidence := scopedCapabilityApplicabilityEvidence(applicabilities)
	hardGateFinding := fmt.Sprintf(
		"SWE MethodRun hard-gate checks are not applicable in exact project-profile scope %q; MethodRun artifacts were not scanned.",
		scopeIDText,
	)
	carryThroughFinding := fmt.Sprintf(
		"SWE MethodRun carry-through checks are not applicable in exact project-profile scope %q; MethodRun artifacts were not scanned.",
		scopeIDText,
	)
	hardGates := processCheckResult(
		"method_run_hard_gates",
		"MethodRun",
		"project_method_runs",
		processCheckStatusNotApplicable,
		"info",
		observedAt,
		validUntil,
		hardGateFinding,
		evidence,
		"No action; NotApplicable is normal for this scope and requires neither SWE MethodRuns nor a waiver.",
	)
	carryThrough := processCheckResult(
		"carry_through_acceptance_ref_posture",
		"MethodRun.carry_through",
		"project_method_runs",
		processCheckStatusNotApplicable,
		"info",
		observedAt,
		validUntil,
		carryThroughFinding,
		evidence,
		"No action; NotApplicable is normal for this scope and requires neither SWE MethodRuns nor a waiver.",
	)
	return processCheckProjectMethodResultSet{
		HardGates:    hardGates,
		CarryThrough: carryThrough,
	}
}
