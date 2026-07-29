package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/project/specflow"
	"github.com/m0n0x41d/haft/internal/projectprofile"
)

const projectSpecificationReadOnlyAuthority = "read_only_profile_applicability_projection"

type publicProjectSpecificationScopeRequest struct {
	Kind    string `json:"kind"`
	ScopeID string `json:"scope_id,omitempty"`
}

type publicProjectSpecificationApplicabilityBasis struct {
	ProjectRoot           string `json:"project_root"`
	AdmissionRecordRef    string `json:"admission_record_ref"`
	AdmissionRecordDigest string `json:"admission_record_digest"`
	ProfilePayloadDigest  string `json:"profile_payload_digest"`
	LedgerRevision        uint64 `json:"ledger_revision"`
}

type publicProjectSpecificationMemberApplicability struct {
	DocumentKind string `json:"document_kind"`
	Capability   string `json:"capability"`
	Kind         string `json:"kind"`
	MissingBasis string `json:"missing_basis,omitempty"`
}

type publicProjectSpecificationApplicabilityCue struct {
	Code          string `json:"code"`
	Message       string `json:"message"`
	MissingBasis  string `json:"missing_basis,omitempty"`
	RequiredInput string `json:"required_input,omitempty"`
}

// publicProjectSpecificationApplicability is an additive read projection over
// one canonical profile resolution. It neither declares a profile nor changes
// SpecSection lifecycle state. Cue is singular so outer uncertainty cannot be
// rendered as repeated carrier debt.
type publicProjectSpecificationApplicability struct {
	Authority               string                                          `json:"authority"`
	Kind                    string                                          `json:"kind"`
	Request                 publicProjectSpecificationScopeRequest          `json:"request"`
	ProjectRoot             string                                          `json:"project_root"`
	ScopeID                 string                                          `json:"scope_id,omitempty"`
	RequestedScopeID        string                                          `json:"requested_scope_id,omitempty"`
	AvailableScopeIDs       []string                                        `json:"available_scope_ids,omitempty"`
	Members                 []publicProjectSpecificationMemberApplicability `json:"members,omitempty"`
	ApplicableDocumentKinds []string                                        `json:"applicable_document_kinds,omitempty"`
	ExcludedDocumentKinds   []string                                        `json:"excluded_document_kinds,omitempty"`
	UnderdeterminedKinds    []string                                        `json:"underdetermined_document_kinds,omitempty"`
	MissingBasis            string                                          `json:"missing_basis,omitempty"`
	Basis                   *publicProjectSpecificationApplicabilityBasis   `json:"basis,omitempty"`
	Cue                     *publicProjectSpecificationApplicabilityCue     `json:"cue,omitempty"`
}

type publicSpecCheckResult struct {
	*project.SpecCheckReport
	ProfileApplicability publicProjectSpecificationApplicability `json:"profile_applicability"`
}

type publicSpecLifecycleResult struct {
	*specflow.SpecLifecycleProjection
	ProfileApplicability publicProjectSpecificationApplicability `json:"profile_applicability"`
}

func projectSpecificationScopeRequestFromFlag(
	rawScopeID string,
) (projectSpecificationScopeRequest, error) {
	if strings.TrimSpace(rawScopeID) == "" {
		return automaticProjectSpecificationScopeRequest(), nil
	}
	scopeID, err := projectprofile.NewScopeID(rawScopeID)
	if err != nil {
		return projectSpecificationScopeRequest{}, fmt.Errorf(
			"invalid --scope-id: %w",
			err,
		)
	}
	return exactProjectSpecificationScopeRequest(scopeID)
}

func buildPublicSpecCheck(
	ctx context.Context,
	projectRoot string,
	request projectSpecificationScopeRequest,
) (publicSpecCheckResult, error) {
	specSet, resolution, err := loadProjectSpecificationSetSQLFirstFromCanonicalProfile(
		ctx,
		projectRoot,
		request,
	)
	if err != nil {
		return publicSpecCheckResult{}, err
	}
	applicability, err := publicProjectSpecificationApplicabilityFrom(
		resolution,
		request,
	)
	if err != nil {
		return publicSpecCheckResult{}, err
	}
	if _, _, resolved := resolution.Resolved(); !resolved {
		return publicSpecCheckResult{
			ProfileApplicability: applicability,
		}, nil
	}
	canonicalRoot := applicability.ProjectRoot
	report := project.SpecCheckReportFromSpecificationSet(specSet)
	report = appendSpecHealthFindingsFromSet(
		report,
		specSet,
		canonicalRoot,
	)
	return publicSpecCheckResult{
		SpecCheckReport:      &report,
		ProfileApplicability: applicability,
	}, nil
}

func buildPublicSpecLifecycle(
	ctx context.Context,
	projectRoot string,
	request projectSpecificationScopeRequest,
) (publicSpecLifecycleResult, error) {
	specSet, resolution, err := loadProjectSpecificationSetSQLFirstFromCanonicalProfile(
		ctx,
		projectRoot,
		request,
	)
	if err != nil {
		return publicSpecLifecycleResult{}, err
	}
	applicability, err := publicProjectSpecificationApplicabilityFrom(
		resolution,
		request,
	)
	if err != nil {
		return publicSpecLifecycleResult{}, err
	}
	scopeApplicability, _, resolved := resolution.Resolved()
	if !resolved {
		return publicSpecLifecycleResult{
			ProfileApplicability: applicability,
		}, nil
	}
	canonicalRoot := applicability.ProjectRoot
	store, projectID, closeStore, err := projectBaselineReadOnly(
		ctx,
		canonicalRoot,
	)
	if closeStore != nil {
		defer closeStore()
	}
	if err != nil {
		return publicSpecLifecycleResult{}, err
	}
	state := specflow.DeriveStateWithBaselines(specSet, store, projectID)
	phaseSet, err := specflow.DeriveApplicablePhaseSet(scopeApplicability)
	if err != nil {
		return publicSpecLifecycleResult{}, err
	}
	projection, err := specflow.ProjectLifecycleForPhaseSet(state, phaseSet)
	if err != nil {
		return publicSpecLifecycleResult{}, err
	}
	return publicSpecLifecycleResult{
		SpecLifecycleProjection: &projection,
		ProfileApplicability:    applicability,
	}, nil
}

func publicProjectSpecificationApplicabilityFrom(
	resolution projectSpecificationApplicabilityResolution,
	request projectSpecificationScopeRequest,
) (publicProjectSpecificationApplicability, error) {
	if !resolution.Valid() ||
		!request.valid() ||
		!sameProjectSpecificationScopeRequest(resolution.request, request) {
		return publicProjectSpecificationApplicability{}, fmt.Errorf(
			"public project specification applicability input is invalid",
		)
	}
	kind := resolution.Kind()
	projectRoot := resolution.ProjectRoot()
	response := publicProjectSpecificationApplicability{
		Authority:   projectSpecificationReadOnlyAuthority,
		Kind:        string(kind),
		Request:     publicProjectSpecificationScopeRequestFrom(resolution.request),
		ProjectRoot: projectRoot.String(),
	}
	basis := publicProjectSpecificationApplicabilityBasisFrom(resolution.basis)
	if basis != nil {
		response.Basis = basis
	}
	availableScopeIDs := resolution.AvailableScopeIDs()
	response.AvailableScopeIDs = scopeIDStrings(availableScopeIDs)
	switch kind {
	case projectSpecificationApplicabilityResolved:
		return publicResolvedProjectSpecificationApplicability(response, resolution)
	case projectSpecificationProfileUnderdetermined:
		missingBasis, _ := resolution.MissingBasis()
		response.MissingBasis = string(missingBasis)
		response.Cue = &publicProjectSpecificationApplicabilityCue{
			Code:         string(projectSpecificationProfileUnderdetermined),
			Message:      "Canonical project-profile applicability is underdetermined; specification carriers were not evaluated.",
			MissingBasis: string(missingBasis),
		}
		return response, nil
	case projectSpecificationScopeChoiceRequired:
		response.Cue = &publicProjectSpecificationApplicabilityCue{
			Code:          string(projectSpecificationScopeChoiceRequired),
			Message:       "Several canonical realization scopes are available; select one exact ScopeID.",
			RequiredInput: "scope_id",
		}
		return response, nil
	case projectSpecificationRequestedScopeNotFound:
		response.RequestedScopeID = resolution.request.scopeID.String()
		response.Cue = &publicProjectSpecificationApplicabilityCue{
			Code:          string(projectSpecificationRequestedScopeNotFound),
			Message:       "The requested ScopeID is not present in the current canonical project profile.",
			RequiredInput: "scope_id",
		}
		return response, nil
	default:
		return publicProjectSpecificationApplicability{}, fmt.Errorf(
			"unknown project specification applicability result %q",
			kind,
		)
	}
}

func sameProjectSpecificationScopeRequest(
	left projectSpecificationScopeRequest,
	right projectSpecificationScopeRequest,
) bool {
	return left.kind == right.kind && left.scopeID == right.scopeID
}

func publicResolvedProjectSpecificationApplicability(
	response publicProjectSpecificationApplicability,
	resolution projectSpecificationApplicabilityResolution,
) (publicProjectSpecificationApplicability, error) {
	applicability, _, resolved := resolution.Resolved()
	if !resolved {
		return publicProjectSpecificationApplicability{}, fmt.Errorf(
			"resolved public applicability omitted the scope-local projection",
		)
	}
	scopeID := applicability.ScopeID()
	members := applicability.Members()
	applicableDocumentKinds := applicability.ApplicableDocumentKinds()
	excludedDocumentKinds := applicability.ExcludedDocumentKinds()
	underdeterminedDocumentKinds := applicability.UnderdeterminedDocumentKinds()
	response.ScopeID = scopeID.String()
	response.Members = publicProjectSpecificationMembers(members)
	response.ApplicableDocumentKinds = specDocumentKindStrings(applicableDocumentKinds)
	response.ExcludedDocumentKinds = specDocumentKindStrings(excludedDocumentKinds)
	response.UnderdeterminedKinds = specDocumentKindStrings(underdeterminedDocumentKinds)
	return response, nil
}

func publicProjectSpecificationScopeRequestFrom(
	request projectSpecificationScopeRequest,
) publicProjectSpecificationScopeRequest {
	scopeID := request.scopeID.String()
	return publicProjectSpecificationScopeRequest{
		Kind:    string(request.kind),
		ScopeID: scopeID,
	}
}

func publicProjectSpecificationApplicabilityBasisFrom(
	basis canonicalProfileApplicabilityBasis,
) *publicProjectSpecificationApplicabilityBasis {
	if !basis.valid() {
		return nil
	}
	projectRoot := basis.projectRoot.String()
	admissionRecordRef := basis.admissionRecordRef.String()
	admissionRecordDigest := basis.admissionRecordDigest.String()
	profilePayloadDigest := basis.payloadDigest.String()
	ledgerRevision := basis.ledgerRevision.Value()
	return &publicProjectSpecificationApplicabilityBasis{
		ProjectRoot:           projectRoot,
		AdmissionRecordRef:    admissionRecordRef,
		AdmissionRecordDigest: admissionRecordDigest,
		ProfilePayloadDigest:  profilePayloadDigest,
		LedgerRevision:        ledgerRevision,
	}
}

func publicProjectSpecificationMembers(
	members []project.ProjectSpecificationMemberApplicability,
) []publicProjectSpecificationMemberApplicability {
	result := make(
		[]publicProjectSpecificationMemberApplicability,
		len(members),
	)
	fillPublicProjectSpecificationMembers(members, result, 0)
	return result
}

func fillPublicProjectSpecificationMembers(
	members []project.ProjectSpecificationMemberApplicability,
	result []publicProjectSpecificationMemberApplicability,
	index int,
) {
	if index == len(members) {
		return
	}
	member := members[index]
	projection := publicProjectSpecificationMemberApplicability{
		DocumentKind: string(member.DocumentKind()),
		Capability:   string(member.Capability()),
		Kind:         string(member.Kind()),
	}
	missingBasis, present := member.MissingBasis()
	if present {
		projection.MissingBasis = string(missingBasis)
	}
	result[index] = projection
	fillPublicProjectSpecificationMembers(members, result, index+1)
}

func scopeIDStrings(values []projectprofile.ScopeID) []string {
	if len(values) == 0 {
		return nil
	}
	result := make([]string, len(values))
	fillScopeIDStrings(values, result, 0)
	return result
}

func fillScopeIDStrings(
	values []projectprofile.ScopeID,
	result []string,
	index int,
) {
	if index == len(values) {
		return
	}
	result[index] = values[index].String()
	fillScopeIDStrings(values, result, index+1)
}

func specDocumentKindStrings(values []project.SpecDocumentKind) []string {
	if len(values) == 0 {
		return nil
	}
	result := make([]string, len(values))
	fillSpecDocumentKindStrings(values, result, 0)
	return result
}

func fillSpecDocumentKindStrings(
	values []project.SpecDocumentKind,
	result []string,
	index int,
) {
	if index == len(values) {
		return
	}
	result[index] = string(values[index])
	fillSpecDocumentKindStrings(values, result, index+1)
}

func writePublicSpecCheckJSON(
	writer io.Writer,
	result publicSpecCheckResult,
) error {
	return writeIndentedJSON(writer, result)
}

func writePublicSpecLifecycleJSON(
	writer io.Writer,
	result publicSpecLifecycleResult,
) error {
	return writeIndentedJSON(writer, result)
}

func writeIndentedJSON(writer io.Writer, value any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func writeProjectSpecificationApplicabilityCue(
	writer io.Writer,
	surface string,
	applicability publicProjectSpecificationApplicability,
) error {
	if applicability.Cue == nil {
		return fmt.Errorf(
			"%s omitted its unresolved profile-applicability cue",
			surface,
		)
	}
	builder := strings.Builder{}
	fmt.Fprintf(
		&builder,
		"%s: not evaluated (%s)\n",
		surface,
		applicability.Cue.Code,
	)
	fmt.Fprintf(&builder, "Profile cue: %s\n", applicability.Cue.Message)
	if applicability.Cue.MissingBasis != "" {
		fmt.Fprintf(&builder, "Missing basis: %s\n", applicability.Cue.MissingBasis)
	}
	if len(applicability.AvailableScopeIDs) > 0 {
		availableScopeIDs := strings.Join(
			applicability.AvailableScopeIDs,
			", ",
		)
		fmt.Fprintf(
			&builder,
			"Available ScopeIDs: %s\n",
			availableScopeIDs,
		)
	}
	if applicability.RequestedScopeID != "" {
		fmt.Fprintf(
			&builder,
			"Requested ScopeID: %s\n",
			applicability.RequestedScopeID,
		)
	}
	_, err := io.WriteString(writer, builder.String())
	return err
}
