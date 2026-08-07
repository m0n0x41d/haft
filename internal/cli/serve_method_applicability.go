package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/projectprofile"
)

const methodProfileApplicabilityAuthority = "canonical_project_profile_methodpack_applicability"

type methodProfileApplicabilityResponse struct {
	Kind                    string                                  `json:"kind"`
	SchemaVersion           int                                     `json:"schema_version"`
	Authority               string                                  `json:"authority"`
	Action                  string                                  `json:"action"`
	ArtifactCreated         bool                                    `json:"artifact_created"`
	Capability              string                                  `json:"capability"`
	Applicability           string                                  `json:"applicability,omitempty"`
	MissingBasis            string                                  `json:"missing_basis,omitempty"`
	ScopeID                 string                                  `json:"scope_id,omitempty"`
	ProfileApplicability    publicProjectSpecificationApplicability `json:"profile_applicability"`
	BlocksCurrentWork       bool                                    `json:"blocks_current_work"`
	RequiresMethodRun       bool                                    `json:"requires_method_run"`
	RequiresHumanGate       bool                                    `json:"requires_human_gate"`
	Continuation            string                                  `json:"continuation"`
	ForbiddenCompensations  []string                                `json:"forbidden_compensations"`
	Boundary                []string                                `json:"boundary"`
	ScopeSelectorDiagnostic *methodScopeSelectorDiagnostic          `json:"scope_selector_diagnostic,omitempty"`
}

type methodScopeSelectorDiagnostic struct {
	RequestedScopeID string `json:"requested_scope_id"`
	SelectedScopeID  string `json:"selected_scope_id"`
	Disposition      string `json:"disposition"`
	Detail           string `json:"detail"`
}

func handleHaftMethodForProject(
	ctx context.Context,
	store *artifact.Store,
	haftDir string,
	args map[string]any,
) (string, string, error) {
	action := strings.TrimSpace(stringArg(args, "action"))
	if action != "pull" && action != "catalog" {
		return handleHaftMethod(ctx, store, haftDir, args)
	}
	rawScopeID := stringArg(args, "scope_id")
	request, err := projectSpecificationScopeRequestFromFlag(rawScopeID)
	if err != nil {
		return "", "", err
	}
	resolution, err := resolveCanonicalProjectSpecificationApplicability(
		ctx,
		filepath.Dir(haftDir),
		request,
	)
	if err != nil {
		return "", "", err
	}
	diagnostic := ignoredSingletonMethodScopeSelector(
		rawScopeID,
		resolution,
	)
	if diagnostic != nil {
		request = automaticProjectSpecificationScopeRequest()
		resolution, err = resolveCanonicalProjectSpecificationApplicability(
			ctx,
			filepath.Dir(haftDir),
			request,
		)
		if err != nil {
			return "", "", err
		}
	}
	publicApplicability, err := publicProjectSpecificationApplicabilityFrom(
		resolution,
		request,
	)
	if err != nil {
		return "", "", err
	}
	scopeApplicability, _, resolved := resolution.Resolved()
	if !resolved {
		response := newMethodProfileApplicabilityResponse(
			action,
			publicApplicability,
			projectprofile.ScopedCapabilityApplicability{},
		)
		response.ScopeSelectorDiagnostic = diagnostic
		return encodeMethodProfileApplicabilityResponse(response)
	}
	applicability, err := scopeApplicability.ScopedCapabilityApplicability(
		projectprofile.SWEMethodPackCapability,
	)
	if err != nil {
		return "", "", err
	}
	if applicability.Kind() == projectprofile.CapabilityRequired {
		result, recovery, err := handleHaftMethod(ctx, store, haftDir, args)
		if err != nil || diagnostic == nil {
			return result, recovery, err
		}
		result, err = attachMethodScopeSelectorDiagnostic(result, diagnostic)
		return result, recovery, err
	}
	response := newMethodProfileApplicabilityResponse(
		action,
		publicApplicability,
		applicability,
	)
	response.ScopeSelectorDiagnostic = diagnostic
	return encodeMethodProfileApplicabilityResponse(response)
}

func ignoredSingletonMethodScopeSelector(
	rawScopeID string,
	resolution projectSpecificationApplicabilityResolution,
) *methodScopeSelectorDiagnostic {
	if strings.TrimSpace(rawScopeID) == "" ||
		resolution.Kind() != projectSpecificationRequestedScopeNotFound {
		return nil
	}
	available := resolution.AvailableScopeIDs()
	if len(available) != 1 {
		return nil
	}
	return &methodScopeSelectorDiagnostic{
		RequestedScopeID: rawScopeID,
		SelectedScopeID:  available[0].String(),
		Disposition:      "ignored_unnecessary_selector",
		Detail:           "The canonical profile has one scope, so the supplied task, thread, commission, work, or other non-scope identifier was ignored.",
	}
}

func attachMethodScopeSelectorDiagnostic(
	result string,
	diagnostic *methodScopeSelectorDiagnostic,
) (string, error) {
	trimmed := strings.TrimSpace(result)
	if strings.HasPrefix(trimmed, "{") {
		payload := map[string]any{}
		if err := json.Unmarshal([]byte(result), &payload); err != nil {
			return "", err
		}
		payload["scope_selector_diagnostic"] = diagnostic
		encoded, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return "", err
		}
		return string(encoded), nil
	}
	return result + fmt.Sprintf(
		"\nScope selector ignored: requested=%q selected=%q disposition=%s. %s\n",
		diagnostic.RequestedScopeID,
		diagnostic.SelectedScopeID,
		diagnostic.Disposition,
		diagnostic.Detail,
	), nil
}

func newMethodProfileApplicabilityResponse(
	action string,
	profileApplicability publicProjectSpecificationApplicability,
	applicability projectprofile.ScopedCapabilityApplicability,
) methodProfileApplicabilityResponse {
	response := methodProfileApplicabilityResponse{
		Kind:                 "haft_method_profile_applicability",
		SchemaVersion:        2,
		Authority:            methodProfileApplicabilityAuthority,
		Action:               action,
		ArtifactCreated:      false,
		Capability:           string(projectprofile.SWEMethodPackCapability),
		ProfileApplicability: profileApplicability,
		BlocksCurrentWork:    false,
		RequiresMethodRun:    false,
		RequiresHumanGate:    false,
		Continuation:         "continue_already_authorized_work_without_method_run",
		ForbiddenCompensations: []string{
			"do_not_request_profile_admission_only_to_obtain_method_run",
			"do_not_create_or_broaden_work_commission_to_compensate",
		},
		Boundary: []string{
			"NotApplicable is normal and creates no MethodRun.",
			"Underdetermined is one neutral cue and creates no MethodRun.",
			"Profile applicability is not method authority, evidence, or Work authorization.",
			"Continue already-authorized Work without a MethodRun; do not turn profile applicability into a human gate.",
		},
	}
	if !applicability.Valid() {
		response.Applicability = profileApplicability.Kind
		response.MissingBasis = profileApplicability.MissingBasis
		return response
	}
	response.Applicability = string(applicability.Kind())
	response.ScopeID = applicability.ScopeID().String()
	if missingBasis, found := applicability.MissingBasis(); found {
		response.MissingBasis = string(missingBasis)
	}
	return response
}

func encodeMethodProfileApplicabilityResponse(
	response methodProfileApplicabilityResponse,
) (string, string, error) {
	payload, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return "", "", fmt.Errorf(
			"encode MethodPack profile applicability: %w",
			err,
		)
	}
	return string(payload), "", nil
}
