package profileonboarding

import (
	profileadmissionsqlite "github.com/m0n0x41d/haft/internal/profileadmission/sqlite"
	"github.com/m0n0x41d/haft/internal/profileprojection"
)

type ResultKind string

const (
	ResultSynchronized     ResultKind = "synchronized"
	ResultProjectionDebt   ResultKind = "projection_debt"
	ResultProjectionFailed ResultKind = "projection_failed"
	ResultNotAdmitted      ResultKind = "not_admitted"
	ResultFailed           ResultKind = "failed"
)

// Rejection is a non-binding explanation for why the requested Work result
// was not admitted. It is not an authority denial or a receipt.
type Rejection struct {
	code   string
	detail string
}

func (rejection Rejection) Code() string   { return rejection.code }
func (rejection Rejection) Detail() string { return rejection.detail }

// Failure reports the strongest known failure boundary. Admission posture is
// retained when the canonical admission effect could not be proved.
type Failure struct {
	stage         string
	code          string
	detail        string
	commitPosture profileadmissionsqlite.AdmissionCommitPosture
	failureRef    string
}

func (failure Failure) Stage() string { return failure.stage }
func (failure Failure) Code() string  { return failure.code }
func (failure Failure) Detail() string {
	return failure.detail
}
func (failure Failure) CommitPosture() profileadmissionsqlite.AdmissionCommitPosture {
	return failure.commitPosture
}
func (failure Failure) FailureRef() string { return failure.failureRef }

// Result is a closed orchestration result. Synchronized carries admission and
// projection; ProjectionDebt additionally carries durable debt detail;
// ProjectionFailed preserves the already-committed admission when projection
// failure has no durable debt result; NotAdmitted carries rejections; Failed
// carries a pre-admission failure. Canonical authority and profile receipts are
// never synthesized here.
type Result struct {
	kind       ResultKind
	admission  profileadmissionsqlite.CanonicalProfileAdmission
	projection profileprojection.Result
	rejections []Rejection
	failure    Failure
}

func (result Result) Kind() ResultKind {
	return result.kind
}

func (result Result) Admission() (
	profileadmissionsqlite.CanonicalProfileAdmission,
	bool,
) {
	withAdmission := result.kind == ResultSynchronized ||
		result.kind == ResultProjectionDebt ||
		result.kind == ResultProjectionFailed
	if !withAdmission || !result.admission.Valid() {
		return profileadmissionsqlite.CanonicalProfileAdmission{}, false
	}
	return result.admission, true
}

func (result Result) Projection() (profileprojection.Result, bool) {
	withProjection := result.kind == ResultSynchronized || result.kind == ResultProjectionDebt
	if !withProjection {
		return profileprojection.Result{}, false
	}
	return result.projection, true
}

func (result Result) Rejections() ([]Rejection, bool) {
	if result.kind != ResultNotAdmitted || len(result.rejections) == 0 {
		return nil, false
	}
	return append([]Rejection{}, result.rejections...), true
}

func (result Result) Failure() (Failure, bool) {
	withFailure := result.kind == ResultFailed ||
		result.kind == ResultProjectionDebt ||
		result.kind == ResultProjectionFailed
	if !withFailure || result.failure.code == "" {
		return Failure{}, false
	}
	return result.failure, true
}

func synchronizedResult(
	admission profileadmissionsqlite.CanonicalProfileAdmission,
	projection profileprojection.Result,
) Result {
	return Result{
		kind:       ResultSynchronized,
		admission:  admission,
		projection: projection,
	}
}

func projectionDebtResult(
	admission profileadmissionsqlite.CanonicalProfileAdmission,
	projection profileprojection.Result,
	projectionErr error,
) Result {
	detail := projection.Detail()
	if projectionErr != nil {
		detail = detail + ": " + projectionErr.Error()
	}
	return Result{
		kind:       ResultProjectionDebt,
		admission:  admission,
		projection: projection,
		failure: Failure{
			stage:  "projection",
			code:   projection.DiagnosticCode(),
			detail: detail,
		},
	}
}

func projectionFailedResult(
	admission profileadmissionsqlite.CanonicalProfileAdmission,
	code string,
	detail string,
) Result {
	return Result{
		kind:      ResultProjectionFailed,
		admission: admission,
		failure: Failure{
			stage:  "projection",
			code:   code,
			detail: detail,
		},
	}
}

func rejectedResult(code string, detail string) Result {
	return Result{
		kind: ResultNotAdmitted,
		rejections: []Rejection{{
			code:   code,
			detail: detail,
		}},
	}
}

func admissionDeniedResult(
	denials []profileadmissionsqlite.AdmissionDenial,
) Result {
	rejections := make([]Rejection, len(denials))
	mapAdmissionDenials(denials, rejections, 0)
	return Result{
		kind:       ResultNotAdmitted,
		rejections: rejections,
	}
}

func mapAdmissionDenials(
	denials []profileadmissionsqlite.AdmissionDenial,
	rejections []Rejection,
	index int,
) {
	if index == len(denials) {
		return
	}
	denial := denials[index]
	rejections[index] = Rejection{
		code:   denial.Code(),
		detail: denial.Detail(),
	}
	mapAdmissionDenials(denials, rejections, index+1)
}

func admissionFailedResult(
	failure profileadmissionsqlite.AdmissionFailure,
) Result {
	return Result{
		kind: ResultFailed,
		failure: Failure{
			stage:         "admission",
			code:          "admission_write_failed",
			detail:        "canonical admission effect was not proved",
			commitPosture: failure.CommitPosture(),
			failureRef:    failure.FailureRef(),
		},
	}
}

func failedResult(stage string, code string, detail string) Result {
	return Result{
		kind: ResultFailed,
		failure: Failure{
			stage:  stage,
			code:   code,
			detail: detail,
		},
	}
}
