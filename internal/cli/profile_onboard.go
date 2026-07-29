package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	profileadmissionsqlite "github.com/m0n0x41d/haft/internal/profileadmission/sqlite"
	"github.com/m0n0x41d/haft/internal/profileonboarding"
	"github.com/m0n0x41d/haft/internal/profileprojection"
)

var profileOnboardJSON bool

var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Manage the canonical project profile",
}

var profileOnboardHaftSoftwareCmd = &cobra.Command{
	Use:    "onboard-haft-software",
	Short:  "Compatibility alias for declaring the reviewed project profile",
	Hidden: true,
	Long: `Compatibility alias for the canonical reviewed-profile declaration.

It reads .haft/profile-declaration-review.json, exactly like "haft profile
declare". If the review carrier is absent, run "haft profile propose" and then
"haft profile declare". The alias never mints the historical built-in payload
or a fresh v1/v2 authority record.`,
	Args: cobra.NoArgs,
	RunE: runProfileOnboardHaftSoftware,
}

func init() {
	profileOnboardHaftSoftwareCmd.Flags().BoolVar(
		&profileOnboardJSON,
		"json",
		false,
		"print the post-authority result as structured JSON",
	)
	profileCmd.AddCommand(profileOnboardHaftSoftwareCmd)
	rootCmd.AddCommand(profileCmd)
}

type profileOnboardAdmission struct {
	RecordRef                 string `json:"record_ref"`
	RecordDigest              string `json:"record_digest"`
	PayloadDigest             string `json:"payload_digest"`
	ReceiptDigest             string `json:"receipt_digest"`
	WorkRecordRef             string `json:"work_record_ref"`
	WorkRecordDigest          string `json:"work_record_digest"`
	AuthorityBasisRef         string `json:"authority_basis_ref"`
	AuthorityBasisDigest      string `json:"authority_basis_digest"`
	AuthorityResolutionRef    string `json:"authority_resolution_ref"`
	AuthorityResolutionDigest string `json:"authority_resolution_digest"`
	Delivery                  string `json:"delivery"`
	RecordedAt                string `json:"recorded_at"`
}

type profileOnboardRevision struct {
	Expected uint64 `json:"expected"`
	Current  uint64 `json:"current"`
}

type profileOnboardProjection struct {
	Kind           string `json:"kind"`
	Path           string `json:"path,omitempty"`
	ExpectedDigest string `json:"expected_digest,omitempty"`
	ObservedDigest string `json:"observed_digest,omitempty"`
	DebtID         string `json:"debt_id,omitempty"`
	DiagnosticCode string `json:"diagnostic_code,omitempty"`
	Detail         string `json:"detail,omitempty"`
}

type profileOnboardRejection struct {
	Code   string `json:"code"`
	Detail string `json:"detail"`
}

type profileOnboardFailure struct {
	Stage         string `json:"stage"`
	Code          string `json:"code"`
	Detail        string `json:"detail"`
	CommitPosture string `json:"commit_posture,omitempty"`
	FailureRef    string `json:"failure_ref,omitempty"`
}

type profileOnboardOutcome struct {
	State      string
	Admission  *profileOnboardAdmission
	Revision   *profileOnboardRevision
	Projection *profileOnboardProjection
	Rejections []profileOnboardRejection
	Failure    *profileOnboardFailure
}

type profileOnboardResponse struct {
	Kind          string                    `json:"kind"`
	State         string                    `json:"state"`
	ProjectRoot   string                    `json:"project_root"`
	ProjectID     string                    `json:"project_id"`
	ReviewInput   string                    `json:"review_input,omitempty"`
	AuthorityMode string                    `json:"authority_mode,omitempty"`
	Admission     *profileOnboardAdmission  `json:"admission,omitempty"`
	Revision      *profileOnboardRevision   `json:"revision,omitempty"`
	Projection    *profileOnboardProjection `json:"projection,omitempty"`
	Rejections    []profileOnboardRejection `json:"rejections,omitempty"`
	Failure       *profileOnboardFailure    `json:"failure,omitempty"`
}

func runProfileOnboardHaftSoftware(cmd *cobra.Command, _ []string) error {
	return runProfileDeclarationCommand(cmd, "", profileOnboardJSON)
}

func normalizeProfileOnboardResult(
	result profileonboarding.Result,
) (profileOnboardOutcome, error) {
	state := string(result.Kind())
	outcome := profileOnboardOutcome{State: state}
	if admission, ok := result.Admission(); ok {
		outcome.Admission = profileOnboardAdmissionFromCanonical(admission)
		outcome.Revision = profileOnboardRevisionFromCanonical(admission)
	}
	if projection, ok := result.Projection(); ok {
		outcome.Projection = profileOnboardProjectionFromCanonical(projection)
	}
	if rejections, ok := result.Rejections(); ok {
		outcome.Rejections = profileOnboardRejectionsFromCanonical(rejections)
	}
	if failure, ok := result.Failure(); ok {
		outcome.Failure = profileOnboardFailureFromCanonical(failure)
	}
	if err := validateProfileOnboardOutcome(outcome); err != nil {
		return profileOnboardOutcome{}, err
	}
	return outcome, nil
}

func profileOnboardAdmissionFromCanonical(
	admission profileadmissionsqlite.CanonicalProfileAdmission,
) *profileOnboardAdmission {
	recordedAt := admission.RecordedAt().UTC().Format(time.RFC3339Nano)
	return &profileOnboardAdmission{
		RecordRef:                 admission.AdmissionRecordRef().String(),
		RecordDigest:              admission.AdmissionRecordDigest().String(),
		PayloadDigest:             admission.PayloadDigest().String(),
		ReceiptDigest:             admission.ReceiptDigest().String(),
		WorkRecordRef:             admission.WorkRecordRef().String(),
		WorkRecordDigest:          admission.WorkRecordDigest().String(),
		AuthorityBasisRef:         admission.AuthorityBasisRef().String(),
		AuthorityBasisDigest:      admission.AuthorityBasisDigest().String(),
		AuthorityResolutionRef:    admission.AuthorityResolutionRef().String(),
		AuthorityResolutionDigest: admission.AuthorityResolutionDigest().String(),
		Delivery:                  string(admission.Delivery()),
		RecordedAt:                recordedAt,
	}
}

func profileOnboardRevisionFromCanonical(
	admission profileadmissionsqlite.CanonicalProfileAdmission,
) *profileOnboardRevision {
	return &profileOnboardRevision{
		Expected: admission.ExpectedLedgerRevision().Value(),
		Current:  admission.LedgerRevision().Value(),
	}
}

func profileOnboardProjectionFromCanonical(
	projection profileprojection.Result,
) *profileOnboardProjection {
	return &profileOnboardProjection{
		Kind:           string(projection.Kind()),
		Path:           projection.ProjectionPath(),
		ExpectedDigest: projection.ExpectedDigest().String(),
		ObservedDigest: projection.ObservedDigest().String(),
		DebtID:         projection.DebtID(),
		DiagnosticCode: projection.DiagnosticCode(),
		Detail:         projection.Detail(),
	}
}

func profileOnboardRejectionsFromCanonical(
	rejections []profileonboarding.Rejection,
) []profileOnboardRejection {
	result := make([]profileOnboardRejection, len(rejections))
	for index, rejection := range rejections {
		result[index] = profileOnboardRejection{
			Code:   rejection.Code(),
			Detail: rejection.Detail(),
		}
	}
	return result
}

func profileOnboardFailureFromCanonical(
	failure profileonboarding.Failure,
) *profileOnboardFailure {
	return &profileOnboardFailure{
		Stage:         failure.Stage(),
		Code:          failure.Code(),
		Detail:        failure.Detail(),
		CommitPosture: string(failure.CommitPosture()),
		FailureRef:    failure.FailureRef(),
	}
}

func validateProfileOnboardOutcome(outcome profileOnboardOutcome) error {
	switch outcome.State {
	case string(profileonboarding.ResultSynchronized):
		if outcome.Admission == nil || outcome.Revision == nil || outcome.Projection == nil {
			return fmt.Errorf("synchronized profile result omitted admission, revision, or projection")
		}
	case string(profileonboarding.ResultProjectionDebt):
		if outcome.Admission == nil || outcome.Revision == nil || outcome.Projection == nil || outcome.Failure == nil {
			return fmt.Errorf("projection-debt result omitted committed admission or debt evidence")
		}
	case string(profileonboarding.ResultProjectionFailed):
		if outcome.Admission == nil || outcome.Revision == nil || outcome.Failure == nil {
			return fmt.Errorf("projection-failed result omitted committed admission or failure evidence")
		}
	case string(profileonboarding.ResultNotAdmitted):
		if len(outcome.Rejections) == 0 {
			return fmt.Errorf("not-admitted profile result omitted rejection evidence")
		}
	case string(profileonboarding.ResultFailed):
		if outcome.Failure == nil {
			return fmt.Errorf("failed profile result omitted failure evidence")
		}
	default:
		return fmt.Errorf("unknown profile onboarding result %q", outcome.State)
	}
	return nil
}

func writeProfileOnboardResponse(
	writer io.Writer,
	response profileOnboardResponse,
	asJSON bool,
) error {
	if asJSON {
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(response)
	}
	if response.State == string(profileonboarding.ResultSynchronized) {
		return writeProfileOnboardSuccess(writer, response)
	}
	lines := []string{
		"Project profile declaration: " + response.State,
		"Project: " + response.ProjectRoot,
	}
	if response.ReviewInput != "" {
		lines = append(lines, "Reviewed input: "+response.ReviewInput+".")
	}
	if response.AuthorityMode != "" {
		lines = append(lines, "Authority policy: "+response.AuthorityMode+".")
	}
	lines = appendProfileOnboardAdmissionSummary(lines, response.Admission, response.Revision)
	lines = appendProfileOnboardProjectionSummary(lines, response.Projection)
	lines = appendProfileOnboardRejectionSummary(lines, response.Rejections)
	lines = appendProfileOnboardFailureSummary(lines, response.Failure)
	lines = append(lines, "Use --json for structured audit details.")
	_, err := fmt.Fprintln(writer, strings.Join(lines, "\n"))
	return err
}

func writeProfileOnboardSuccess(
	writer io.Writer,
	response profileOnboardResponse,
) error {
	if response.Admission == nil || response.Revision == nil || response.Projection == nil {
		return fmt.Errorf("synchronized profile result omitted admission, revision, or projection")
	}
	lines := []string{
		"Project profile declaration succeeded.",
		"",
		"Project: " + response.ProjectRoot,
	}
	if response.ReviewInput != "" {
		lines = append(lines, "Reviewed input: "+response.ReviewInput+".")
	}
	if response.AuthorityMode != "" {
		lines = append(lines, "Authority policy: "+response.AuthorityMode+".")
	}
	lines = append(lines,
		"Authority: durable and bound to the admitted profile.",
		fmt.Sprintf(
			"Admission: committed to project memory at ledger revision %d.",
			response.Revision.Current,
		),
		"Profile projection: synchronized at "+response.Projection.Path+".",
	)
	_, err := fmt.Fprintln(writer, strings.Join(lines, "\n"))
	return err
}

func appendProfileOnboardAdmissionSummary(
	lines []string,
	admission *profileOnboardAdmission,
	revision *profileOnboardRevision,
) []string {
	if admission == nil || revision == nil {
		return lines
	}
	return append(lines,
		fmt.Sprintf(
			"Admission: committed to project memory at ledger revision %d.",
			revision.Current,
		),
	)
}

func appendProfileOnboardProjectionSummary(
	lines []string,
	projection *profileOnboardProjection,
) []string {
	if projection == nil {
		return lines
	}
	lines = append(lines, "Profile projection: "+projection.Kind+".")
	if projection.Path == "" {
		return lines
	}
	return append(lines, "Projection path: "+projection.Path+".")
}

func appendProfileOnboardRejectionSummary(
	lines []string,
	rejections []profileOnboardRejection,
) []string {
	for _, rejection := range rejections {
		lines = append(lines, "Rejection code: "+rejection.Code+".")
	}
	return lines
}

func appendProfileOnboardFailureSummary(
	lines []string,
	failure *profileOnboardFailure,
) []string {
	if failure == nil {
		return lines
	}
	lines = append(lines,
		"Failure stage: "+failure.Stage+".",
		"Failure code: "+failure.Code+".",
	)
	if failure.CommitPosture == "" {
		return lines
	}
	return append(lines, "Commit posture: "+failure.CommitPosture+".")
}

func profileOnboardOutcomeError(response profileOnboardResponse) error {
	if response.State == string(profileonboarding.ResultSynchronized) {
		return nil
	}
	if response.State == string(profileonboarding.ResultProjectionDebt) ||
		response.State == string(profileonboarding.ResultProjectionFailed) {
		return fmt.Errorf(
			"profile declaration committed but projection ended as %s; inspect the emitted typed result",
			response.State,
		)
	}
	return fmt.Errorf(
		"profile declaration ended as %s; inspect the emitted typed result",
		response.State,
	)
}
