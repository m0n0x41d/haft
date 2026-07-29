package profileprojection

import "github.com/m0n0x41d/haft/internal/projectprofile"

type ResultKind string

const (
	ResultSynchronized            ResultKind = "synchronized"
	ResultProjectionDebt          ResultKind = "projection_debt"
	ResultAuto                    ResultKind = "auto"
	ResultProjectionWithoutLedger ResultKind = "projection_without_ledger"
)

// Result reports the relation between canonical profile state and its YAML
// projection. It never carries authority and cannot be used for admission.
type Result struct {
	kind           ResultKind
	projectionPath string
	expectedDigest projectprofile.ContentDigest
	observedDigest projectprofile.ContentDigest
	debtID         string
	diagnosticCode string
	detail         string
}

func (result Result) Kind() ResultKind {
	return result.kind
}

func (result Result) ProjectionPath() string {
	return result.projectionPath
}

func (result Result) ExpectedDigest() projectprofile.ContentDigest {
	return result.expectedDigest
}

func (result Result) ObservedDigest() projectprofile.ContentDigest {
	return result.observedDigest
}

func (result Result) DebtID() string {
	return result.debtID
}

func (result Result) DiagnosticCode() string {
	return result.diagnosticCode
}

func (result Result) Detail() string {
	return result.detail
}
