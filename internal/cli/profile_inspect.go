package cli

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	profileadmissionsqlite "github.com/m0n0x41d/haft/internal/profileadmission/sqlite"
	"github.com/m0n0x41d/haft/internal/profiledetector"
	"github.com/m0n0x41d/haft/internal/projectledger"
	"github.com/m0n0x41d/haft/internal/projectprofile"
)

const (
	profileInspectionRecordKind = "haft_project_profile_inspection"
	profileProposalRecordKind   = "haft_project_profile_proposal"
	profileEvidenceWindowLimit  = 20
)

var profileInspectJSON bool
var profileProposeJSON bool
var profileInspectFullEvidence bool
var profileProposeFullEvidence bool

var profileInspectCmd = &cobra.Command{
	Use:   "inspect",
	Short: "Inspect canonical profile state and read-only repository signals",
	Long: `Inspect the canonical profile-admission ledger and run the read-only
repository detector. Detector output is orientation evidence only: it cannot
by itself declare a profile, establish applicability, or mutate project state.
haft init may consume only a complete supported singleton through its separate
deterministic initial-bootstrap authority contract.`,
	Args: cobra.NoArgs,
	RunE: runProfileInspect,
}

var profileProposeCmd = &cobra.Command{
	Use:   "propose",
	Short: "Produce a non-binding project-profile proposal from repository evidence",
	Long: `Produce a project-neutral, evidence-bearing, non-binding orientation proposal.

The proposal preserves exact detector inputs and candidate scopes in a local,
non-binding review carrier. It performs no SpeechAct, Work, admission,
projection, or canonical-profile mutation. A declared canonical profile
remains authoritative even when detector evidence conflicts.`,
	Args: cobra.NoArgs,
	RunE: runProfilePropose,
}

func init() {
	profileInspectCmd.Flags().BoolVar(
		&profileInspectJSON,
		"json",
		false,
		"print the inspection as structured JSON",
	)
	profileInspectCmd.Flags().BoolVar(
		&profileInspectFullEvidence,
		"full-evidence",
		false,
		"include every observed file and detector signal in JSON output",
	)
	profileProposeCmd.Flags().BoolVar(
		&profileProposeJSON,
		"json",
		false,
		"print the non-binding proposal as structured JSON",
	)
	profileProposeCmd.Flags().BoolVar(
		&profileProposeFullEvidence,
		"full-evidence",
		false,
		"include every observed file and detector signal in JSON output",
	)
	profileCmd.AddCommand(profileInspectCmd)
	profileCmd.AddCommand(profileProposeCmd)
}

type profileSignalView struct {
	RuleID                string `json:"rule_id"`
	Path                  string `json:"path"`
	ComponentCandidateKey string `json:"component_candidate_key,omitempty"`
	RealizationKind       string `json:"realization_kind,omitempty"`
	Strength              string `json:"strength"`
}

type profileSuggestedScopeView struct {
	ComponentCandidateRef    string              `json:"component_candidate_ref"`
	RealizationKind          string              `json:"realization_kind"`
	Orientation              string              `json:"orientation"`
	PositiveSignalCount      int                 `json:"positive_signal_count"`
	PositiveSignals          []profileSignalView `json:"positive_signals"`
	PositiveSignalsTruncated bool                `json:"positive_signals_truncated"`
	NegativeSignalCount      int                 `json:"negative_signal_count"`
	NegativeSignals          []profileSignalView `json:"negative_signals"`
	NegativeSignalsTruncated bool                `json:"negative_signals_truncated"`
}

type profileScanView struct {
	ScannedFileCount       int      `json:"scanned_file_count"`
	ScanTruncated          bool     `json:"scan_truncated"`
	ObservedFileCount      int      `json:"observed_file_count"`
	ObservedFiles          []string `json:"observed_files"`
	ObservedFilesTruncated bool     `json:"observed_files_truncated"`
}

type profileSuggestionView struct {
	Kind                        string                      `json:"kind"`
	SemanticRole                string                      `json:"semantic_role"`
	SuggestionRef               string                      `json:"suggestion_ref"`
	DetectorVersion             string                      `json:"detector_version"`
	PolicyVersion               string                      `json:"policy_version"`
	ObservationBasis            string                      `json:"observation_basis"`
	ObservationDigest           string                      `json:"observation_digest"`
	Classification              string                      `json:"classification"`
	ConfidencePosture           string                      `json:"confidence_posture"`
	SuggestedScopes             []profileSuggestedScopeView `json:"suggested_scopes"`
	PartialSuggestions          []profileSuggestedScopeView `json:"partial_suggestions"`
	MissingBasis                []string                    `json:"missing_basis"`
	ConflictingSignalCount      int                         `json:"conflicting_signal_count"`
	ConflictingSignals          []profileSignalView         `json:"conflicting_signals"`
	ConflictingSignalsTruncated bool                        `json:"conflicting_signals_truncated"`
	ExcludedSignalCount         int                         `json:"excluded_signal_count"`
	ExcludedSignals             []profileSignalView         `json:"excluded_signals"`
	ExcludedSignalsTruncated    bool                        `json:"excluded_signals_truncated"`
	Scan                        profileScanView             `json:"scan"`
}

type canonicalProfileScopeView struct {
	ScopeID              string   `json:"scope_id"`
	RealizationKind      string   `json:"realization_kind"`
	EntityRef            string   `json:"entity_ref,omitempty"`
	AdmittedKindRef      string   `json:"admitted_kind_ref,omitempty"`
	GoverningPatternRefs []string `json:"governing_pattern_refs,omitempty"`
	ContractRefs         []string `json:"contract_refs,omitempty"`
}

type canonicalProfileView struct {
	Kind                  string                      `json:"kind"`
	SemanticRole          string                      `json:"semantic_role"`
	Origin                string                      `json:"origin,omitempty"`
	LedgerRevision        uint64                      `json:"ledger_revision,omitempty"`
	PayloadDigest         string                      `json:"payload_digest,omitempty"`
	AdmissionRecordRef    string                      `json:"admission_record_ref,omitempty"`
	AdmissionRecordDigest string                      `json:"admission_record_digest,omitempty"`
	RecordedAt            string                      `json:"recorded_at,omitempty"`
	Scopes                []canonicalProfileScopeView `json:"scopes"`
}

type profileInspectionRelation struct {
	Kind                   string `json:"kind"`
	BindingSource          string `json:"binding_source"`
	DetectorMayMutate      bool   `json:"detector_may_mutate"`
	DetectorMaySatisfyGate bool   `json:"detector_may_satisfy_gate"`
	Detail                 string `json:"detail"`
}

type profileInspectionResponse struct {
	Kind             string                    `json:"kind"`
	ProjectRoot      string                    `json:"project_root"`
	ProjectID        string                    `json:"project_id"`
	CanonicalProfile canonicalProfileView      `json:"canonical_profile"`
	Suggestion       profileSuggestionView     `json:"suggestion"`
	Relation         profileInspectionRelation `json:"relation"`
}

type profileProposalResponse struct {
	Kind               string                    `json:"kind"`
	ProjectRoot        string                    `json:"project_root"`
	ProjectID          string                    `json:"project_id"`
	SemanticRole       string                    `json:"semantic_role"`
	Suggestion         profileSuggestionView     `json:"suggestion"`
	CurrentRelation    profileInspectionRelation `json:"current_relation"`
	ReviewCandidate    profileReviewCandidate    `json:"review_candidate"`
	MutationPerformed  bool                      `json:"mutation_performed"`
	AdmissionPerformed bool                      `json:"admission_performed"`
}

func runProfileInspect(cmd *cobra.Command, _ []string) error {
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("not a haft project: %w", err)
	}
	ctx := commandContext(cmd)
	response, err := executeProfileInspection(ctx, projectRoot, profileInspectFullEvidence)
	if err != nil {
		return err
	}
	return writeProfileInspectionResponse(cmd.OutOrStdout(), response, profileInspectJSON)
}

func runProfilePropose(cmd *cobra.Command, _ []string) error {
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("not a haft project: %w", err)
	}
	ctx := commandContext(cmd)
	inspection, suggestion, err := executeProfileInspectionWithSuggestion(
		ctx,
		projectRoot,
		profileProposeFullEvidence,
	)
	if err != nil {
		return err
	}
	declaredProfile := inspection.CanonicalProfile.Kind == "declared"
	detectorDefault := inspection.CanonicalProfile.Origin ==
		string(projectprofile.ProfileAdmissionOriginDetectorDefault)
	if inspection.CanonicalProfile.Kind != "auto" &&
		(!declaredProfile || !detectorDefault) {
		return fmt.Errorf(
			"the canonical project profile is not detector_default; explicit-over-explicit and legacy profile changes require their own mutation contract",
		)
	}
	review, err := prepareProfileReviewCandidate(projectRoot, suggestion)
	if err != nil {
		return err
	}
	response := profileProposalResponse{
		Kind:               profileProposalRecordKind,
		ProjectRoot:        inspection.ProjectRoot,
		ProjectID:          inspection.ProjectID,
		SemanticRole:       "non_binding_review_input",
		Suggestion:         inspection.Suggestion,
		CurrentRelation:    inspection.Relation,
		ReviewCandidate:    review,
		MutationPerformed:  true,
		AdmissionPerformed: false,
	}
	return writeProfileProposalResponse(cmd.OutOrStdout(), response, profileProposeJSON)
}

func commandContext(cmd *cobra.Command) context.Context {
	ctx := cmd.Context()
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func executeProfileInspection(
	ctx context.Context,
	projectRoot string,
	fullEvidence bool,
) (response profileInspectionResponse, runErr error) {
	response, _, runErr = executeProfileInspectionWithSuggestion(
		ctx,
		projectRoot,
		fullEvidence,
	)
	return response, runErr
}

func executeProfileInspectionWithSuggestion(
	ctx context.Context,
	projectRoot string,
	fullEvidence bool,
) (
	response profileInspectionResponse,
	suggestion profiledetector.Suggestion,
	runErr error,
) {
	if ctx == nil {
		return profileInspectionResponse{}, profiledetector.Suggestion{},
			fmt.Errorf("profile inspection requires a context")
	}
	handle, err := projectledger.OpenExisting(ctx, projectRoot, projectledger.ReadOnly)
	if err != nil {
		return profileInspectionResponse{}, profiledetector.Suggestion{},
			fmt.Errorf("open checked project ledger: %w", err)
	}
	defer func() {
		runErr = errors.Join(runErr, handle.Close())
	}()
	canonical, err := readCanonicalProfile(ctx, handle.Database(), handle.ProjectRoot().String())
	if err != nil {
		return profileInspectionResponse{}, profiledetector.Suggestion{}, err
	}
	suggestion, err = profiledetector.Inspect(handle.ProjectRoot().String())
	if err != nil {
		return profileInspectionResponse{}, profiledetector.Suggestion{},
			fmt.Errorf("inspect repository profile evidence: %w", err)
	}
	suggestionView := profileSuggestionViewFromDomain(suggestion, fullEvidence)
	relation := compareCanonicalProfileAndSuggestion(canonical, suggestionView)
	return profileInspectionResponse{
		Kind:             profileInspectionRecordKind,
		ProjectRoot:      handle.ProjectRoot().String(),
		ProjectID:        handle.ProjectID().String(),
		CanonicalProfile: canonical,
		Suggestion:       suggestionView,
		Relation:         relation,
	}, suggestion, nil
}

func readCanonicalProfile(
	ctx context.Context,
	database *sql.DB,
	projectRoot string,
) (canonicalProfileView, error) {
	root, err := projectprofile.NewProjectRootV1(projectRoot)
	if err != nil {
		return canonicalProfileView{}, fmt.Errorf("parse canonical project root: %w", err)
	}
	service, err := profileadmissionsqlite.NewService(database)
	if err != nil {
		return canonicalProfileView{}, fmt.Errorf("open canonical profile reader: %w", err)
	}
	result := service.ResolveCurrent(ctx, root)
	if admission, ok := result.Admission(); ok {
		return canonicalProfileViewFromAdmission(admission)
	}
	if canonicalProfileAbsent(result) {
		return canonicalProfileView{
			Kind:         "auto",
			SemanticRole: "no_canonical_admission",
			Scopes:       []canonicalProfileScopeView{},
		}, nil
	}
	return canonicalProfileView{}, canonicalProfileResolutionError(result)
}

func canonicalProfileViewFromAdmission(
	admission profileadmissionsqlite.CanonicalProfileAdmission,
) (canonicalProfileView, error) {
	scopes, err := canonicalProfileScopeViews(admission.Payload().Scopes().Values())
	if err != nil {
		return canonicalProfileView{}, err
	}
	return canonicalProfileView{
		Kind:                  "declared",
		SemanticRole:          "canonical_admitted_profile",
		Origin:                string(admission.Origin()),
		LedgerRevision:        admission.LedgerRevision().Value(),
		PayloadDigest:         admission.PayloadDigest().String(),
		AdmissionRecordRef:    admission.AdmissionRecordRef().String(),
		AdmissionRecordDigest: admission.AdmissionRecordDigest().String(),
		RecordedAt:            admission.RecordedAt().UTC().Format(time.RFC3339Nano),
		Scopes:                scopes,
	}, nil
}

func canonicalProfileScopeViews(
	values []projectprofile.RealizationScope,
) ([]canonicalProfileScopeView, error) {
	result := make([]canonicalProfileScopeView, len(values))
	for index, value := range values {
		view, err := canonicalProfileScopeViewFromDomain(value)
		if err != nil {
			return nil, fmt.Errorf("canonical profile scope %d: %w", index, err)
		}
		result[index] = view
	}
	slices.SortFunc(result, func(left canonicalProfileScopeView, right canonicalProfileScopeView) int {
		return strings.Compare(left.ScopeID, right.ScopeID)
	})
	return result, nil
}

func canonicalProfileScopeViewFromDomain(
	value projectprofile.RealizationScope,
) (canonicalProfileScopeView, error) {
	switch scope := value.(type) {
	case projectprofile.SoftwareRealization:
		return canonicalSoftwareScopeView(scope), nil
	case projectprofile.NonSoftwareRealization:
		return canonicalNonSoftwareScopeView(scope), nil
	default:
		return canonicalProfileScopeView{}, fmt.Errorf("unknown realization-scope variant")
	}
}

func canonicalSoftwareScopeView(
	scope projectprofile.SoftwareRealization,
) canonicalProfileScopeView {
	return canonicalProfileScopeView{
		ScopeID:         scope.ScopeID().String(),
		RealizationKind: string(profiledetector.SoftwareRealization),
		EntityRef:       entityReferenceText(scope.EntityReference()),
	}
}

func canonicalNonSoftwareScopeView(
	scope projectprofile.NonSoftwareRealization,
) canonicalProfileScopeView {
	return canonicalProfileScopeView{
		ScopeID:              scope.ScopeID().String(),
		RealizationKind:      string(profiledetector.NonSoftwareRealization),
		EntityRef:            entityReferenceText(scope.EntityReference()),
		AdmittedKindRef:      kindOrientationText(scope.KindOrientation()),
		GoverningPatternRefs: sourceUnitReferenceTexts(scope.GoverningPatternRefs()),
		ContractRefs:         specSectionReferenceTexts(scope.ContractRefs()),
	}
}

func entityReferenceText(reference projectprofile.EntityReference) string {
	value, ok := reference.(projectprofile.ReferencedEntity)
	if !ok {
		return ""
	}
	return value.Ref().String()
}

func kindOrientationText(orientation projectprofile.KindOrientation) string {
	value, ok := orientation.(projectprofile.ReferencedKindOrientation)
	if !ok {
		return ""
	}
	return value.Ref().String()
}

func sourceUnitReferenceTexts(values []projectprofile.SourceUnitRef) []string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = value.String()
	}
	return result
}

func specSectionReferenceTexts(values []projectprofile.SpecSectionRef) []string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = value.String()
	}
	return result
}

func canonicalProfileAbsent(result profileadmissionsqlite.AdmissionResult) bool {
	denials, ok := result.Denials()
	if !ok || len(denials) != 1 {
		return false
	}
	return denials[0].Code() == "profile_not_declared"
}

func canonicalProfileResolutionError(
	result profileadmissionsqlite.AdmissionResult,
) error {
	if denials, ok := result.Denials(); ok {
		parts := make([]string, len(denials))
		for index, denial := range denials {
			parts[index] = denial.Code() + ": " + denial.Detail()
		}
		return fmt.Errorf("canonical profile resolution denied: %s", strings.Join(parts, "; "))
	}
	if failure, ok := result.Failure(); ok {
		return fmt.Errorf(
			"canonical profile resolution failed at %s with posture %s",
			failure.FailureRef(),
			failure.CommitPosture(),
		)
	}
	return fmt.Errorf("canonical profile resolver returned invalid result %q", result.Kind())
}

func profileSuggestionViewFromDomain(
	suggestion profiledetector.Suggestion,
	fullEvidence bool,
) profileSuggestionView {
	snapshot := suggestion.Snapshot()
	kind, scopes, partial, missing := profileSuggestionOutcome(suggestion, fullEvidence)
	conflictingSignals, conflictingCount, conflictingTruncated := profileSignalWindow(
		suggestion.ConflictingSignals(),
		fullEvidence,
	)
	excludedSignals, excludedCount, excludedTruncated := profileSignalWindow(
		suggestion.ExcludedSignals(),
		fullEvidence,
	)
	observedFiles, observedFilesTruncated := profileStringWindow(
		snapshot.RelativeFiles(),
		fullEvidence,
	)
	return profileSuggestionView{
		Kind:                        kind,
		SemanticRole:                "non_binding_orientation",
		SuggestionRef:               suggestion.SuggestionRef(),
		DetectorVersion:             suggestion.DetectorVersion(),
		PolicyVersion:               profiledetector.PolicyVersion,
		ObservationBasis:            "normalized_project_relative_file_paths",
		ObservationDigest:           snapshot.ObservationDigest(),
		Classification:              string(suggestion.Classification()),
		ConfidencePosture:           string(suggestion.ConfidencePosture()),
		SuggestedScopes:             scopes,
		PartialSuggestions:          partial,
		MissingBasis:                missing,
		ConflictingSignalCount:      conflictingCount,
		ConflictingSignals:          conflictingSignals,
		ConflictingSignalsTruncated: conflictingTruncated,
		ExcludedSignalCount:         excludedCount,
		ExcludedSignals:             excludedSignals,
		ExcludedSignalsTruncated:    excludedTruncated,
		Scan: profileScanView{
			ScannedFileCount:       snapshot.ScannedFileCount(),
			ScanTruncated:          snapshot.Truncated(),
			ObservedFileCount:      len(snapshot.RelativeFiles()),
			ObservedFiles:          observedFiles,
			ObservedFilesTruncated: observedFilesTruncated,
		},
	}
}

func profileSuggestionOutcome(
	suggestion profiledetector.Suggestion,
	fullEvidence bool,
) (string, []profileSuggestedScopeView, []profileSuggestedScopeView, []string) {
	views := profileSuggestedScopeViews(suggestion.SuggestedScopes(), fullEvidence)
	if suggestion.ScopeIdentityPosture() ==
		profiledetector.StableScopeIdentity {
		return "suggested_scopes", views, []profileSuggestedScopeView{}, []string{}
	}
	if suggestion.Classification() ==
		profiledetector.InsufficientDetectorBasis {
		return "underdetermined", []profileSuggestedScopeView{}, views, []string{"classification_basis"}
	}
	return "underdetermined", []profileSuggestedScopeView{}, views, []string{"stable_scope_identity"}
}

func profileSuggestedScopeViews(
	values []profiledetector.SuggestedScope,
	fullEvidence bool,
) []profileSuggestedScopeView {
	result := make([]profileSuggestedScopeView, len(values))
	for index, value := range values {
		positive, positiveCount, positiveTruncated := profileSignalWindow(
			value.PositiveSignals(),
			fullEvidence,
		)
		negative, negativeCount, negativeTruncated := profileSignalWindow(
			value.NegativeSignals(),
			fullEvidence,
		)
		result[index] = profileSuggestedScopeView{
			ComponentCandidateRef:    value.ComponentCandidateRef(),
			RealizationKind:          string(value.RealizationKind()),
			Orientation:              value.Orientation(),
			PositiveSignalCount:      positiveCount,
			PositiveSignals:          positive,
			PositiveSignalsTruncated: positiveTruncated,
			NegativeSignalCount:      negativeCount,
			NegativeSignals:          negative,
			NegativeSignalsTruncated: negativeTruncated,
		}
	}
	return result
}

func profileSignalWindow(
	values []profiledetector.Signal,
	fullEvidence bool,
) ([]profileSignalView, int, bool) {
	count := len(values)
	window := values
	if !fullEvidence && count > profileEvidenceWindowLimit {
		window = values[:profileEvidenceWindowLimit]
	}
	return profileSignalViews(window), count, len(window) < count
}

func profileStringWindow(values []string, fullEvidence bool) ([]string, bool) {
	window := values
	if !fullEvidence && len(values) > profileEvidenceWindowLimit {
		window = values[:profileEvidenceWindowLimit]
	}
	return append([]string{}, window...), len(window) < len(values)
}

func profileSignalViews(values []profiledetector.Signal) []profileSignalView {
	result := make([]profileSignalView, len(values))
	for index, value := range values {
		result[index] = profileSignalView{
			RuleID:                value.RuleID(),
			Path:                  value.Path(),
			ComponentCandidateKey: value.ComponentCandidateKey(),
			RealizationKind:       string(value.RealizationKind()),
			Strength:              string(value.Strength()),
		}
	}
	return result
}

func compareCanonicalProfileAndSuggestion(
	canonical canonicalProfileView,
	suggestion profileSuggestionView,
) profileInspectionRelation {
	if canonical.Kind == "auto" && suggestion.Classification == string(profiledetector.InsufficientDetectorBasis) {
		return newProfileInspectionRelation(
			"not_declared",
			"none",
			"no canonical profile exists and repository evidence is insufficient",
		)
	}
	if canonical.Kind == "auto" {
		return newProfileInspectionRelation(
			"not_declared",
			"none",
			"repository evidence produced a non-binding suggestion; no canonical profile exists",
		)
	}
	if suggestion.Classification == string(profiledetector.InsufficientDetectorBasis) {
		return newProfileInspectionRelation(
			"not_comparable",
			"sqlite_profile_admission_ledger",
			"the canonical declared profile remains authoritative; current detector evidence is insufficient",
		)
	}
	canonicalKinds := canonicalRealizationKinds(canonical.Scopes)
	suggestedKinds := suggestedRealizationKinds(profileSuggestionScopes(suggestion))
	if slices.Equal(canonicalKinds, suggestedKinds) {
		return newProfileInspectionRelation(
			"consistent_with_declared",
			"sqlite_profile_admission_ledger",
			"detector orientation aligns by realization class; canonical admitted scopes remain authoritative",
		)
	}
	return newProfileInspectionRelation(
		"conflicts_with_declared",
		"sqlite_profile_admission_ledger",
		"detector evidence differs from the admitted realization classes; preserve the declared profile and treat this as review input",
	)
}

func newProfileInspectionRelation(
	kind string,
	bindingSource string,
	detail string,
) profileInspectionRelation {
	return profileInspectionRelation{
		Kind:                   kind,
		BindingSource:          bindingSource,
		DetectorMayMutate:      false,
		DetectorMaySatisfyGate: false,
		Detail:                 detail,
	}
}

func canonicalRealizationKinds(values []canonicalProfileScopeView) []string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = value.RealizationKind
	}
	slices.Sort(result)
	return slices.Compact(result)
}

func suggestedRealizationKinds(values []profileSuggestedScopeView) []string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = value.RealizationKind
	}
	slices.Sort(result)
	return slices.Compact(result)
}

func profileSuggestionScopes(value profileSuggestionView) []profileSuggestedScopeView {
	if len(value.SuggestedScopes) > 0 {
		return value.SuggestedScopes
	}
	return value.PartialSuggestions
}

func writeProfileInspectionResponse(
	writer io.Writer,
	response profileInspectionResponse,
	asJSON bool,
) error {
	if asJSON {
		return writeIndentedJSON(writer, response)
	}
	lines := []string{
		"Project profile inspection",
		"Project: " + response.ProjectRoot,
		"Canonical profile: " + response.CanonicalProfile.Kind + ".",
		"Detector: " + response.Suggestion.Classification + " (" + response.Suggestion.ConfidencePosture + ").",
	}
	if response.CanonicalProfile.Origin != "" {
		lines = append(lines, "Profile origin: "+response.CanonicalProfile.Origin+".")
	}
	lines = appendProfileSuggestedScopeSummary(lines, profileSuggestionScopes(response.Suggestion))
	lines = append(lines,
		"Relation: "+response.Relation.Kind+".",
		"Authority: detector output alone cannot mutate the profile or satisfy a binding gate; haft init may admit only an exact supported singleton through the separate deterministic bootstrap contract.",
		"Use --json for exact signals, scopes, and canonical provenance.",
	)
	_, err := fmt.Fprintln(writer, strings.Join(lines, "\n"))
	return err
}

func writeProfileProposalResponse(
	writer io.Writer,
	response profileProposalResponse,
	asJSON bool,
) error {
	if asJSON {
		return writeIndentedJSON(writer, response)
	}
	lines := []string{
		"Project profile proposal (non-binding)",
		"Project: " + response.ProjectRoot,
		"Detector: " + response.Suggestion.Classification + " (" + response.Suggestion.ConfidencePosture + ").",
	}
	lines = appendProfileSuggestedScopeSummary(lines, profileSuggestionScopes(response.Suggestion))
	lines = append(lines,
		"Review candidate: "+response.ReviewCandidate.Path+" ("+response.ReviewCandidate.State+").",
		"Edit scope_id and optional semantic references there when the detector orientation is not the intended project scope.",
		"No profile declaration or admission was performed.",
		"An edited or foreign review is input to later explicit `haft profile declare`. An unchanged Haft-generated review does not block eligible haft init bootstrap and is removed only after successful admission and carrier installation.",
		"Use --json for exact detector evidence.",
	)
	_, err := fmt.Fprintln(writer, strings.Join(lines, "\n"))
	return err
}

func appendProfileSuggestedScopeSummary(
	lines []string,
	scopes []profileSuggestedScopeView,
) []string {
	if len(scopes) == 0 {
		return append(lines, "Suggested scopes: none; detector basis is insufficient.")
	}
	parts := make([]string, len(scopes))
	for index, scope := range scopes {
		parts[index] = scope.Orientation + " [" + scope.RealizationKind + "]"
	}
	return append(lines, "Suggested scopes: "+strings.Join(parts, ", ")+".")
}
