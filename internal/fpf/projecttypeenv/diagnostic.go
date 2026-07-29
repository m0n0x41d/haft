package projecttypeenv

import (
	"fmt"
	"sort"

	"github.com/m0n0x41d/haft/internal/fpf/localpractice"
)

type LinkIssueCode string

const (
	IssueBaseArtifactInvalid         LinkIssueCode = "base_artifact_invalid"
	IssueBaseNotCompiled             LinkIssueCode = "base_not_compiled"
	IssueBaseRefMismatch             LinkIssueCode = "base_ref_mismatch"
	IssueCompilerVersionMismatch     LinkIssueCode = "compiler_version_mismatch"
	IssueCarrierManifestMismatch     LinkIssueCode = "carrier_manifest_mismatch"
	IssueDuplicateManifestCoordinate LinkIssueCode = "duplicate_manifest_coordinate"
	IssueDuplicateSignatureID        LinkIssueCode = "duplicate_signature_id"
	IssueMissingImport               LinkIssueCode = "missing_import"
	IssueSelfImport                  LinkIssueCode = "self_import"
	IssueImportCycle                 LinkIssueCode = "import_cycle"
)

type IssueLocation interface {
	String() string
	issueLocationVariant()
}

type BaseArtifactLocation struct{}

func (BaseArtifactLocation) String() string { return "compiled-fpf-base" }

func (BaseArtifactLocation) issueLocationVariant() {}

type CarrierSourceLocation struct {
	carrierID string
	span      localpractice.SourceLineRange
}

func newCarrierSourceLocation(
	carrierID string,
	span localpractice.SourceLineRange,
) CarrierSourceLocation {
	return CarrierSourceLocation{carrierID: carrierID, span: span}
}

func (location CarrierSourceLocation) CarrierID() string { return location.carrierID }

func (location CarrierSourceLocation) Span() localpractice.SourceLineRange {
	return location.span
}

func (location CarrierSourceLocation) String() string {
	return fmt.Sprintf(
		"%s:%d-%d",
		location.carrierID,
		location.span.Start(),
		location.span.End(),
	)
}

func (CarrierSourceLocation) issueLocationVariant() {}

type LinkIssue struct {
	code     LinkIssueCode
	location IssueLocation
	subject  string
	detail   string
	repair   string
}

func newLinkIssue(
	code LinkIssueCode,
	location IssueLocation,
	subject string,
	detail string,
	repair string,
) LinkIssue {
	return LinkIssue{
		code:     code,
		location: location,
		subject:  subject,
		detail:   detail,
		repair:   repair,
	}
}

func (issue LinkIssue) Code() LinkIssueCode { return issue.code }

func (issue LinkIssue) Location() IssueLocation { return issue.location }

func (issue LinkIssue) Subject() string { return issue.subject }

func (issue LinkIssue) Detail() string { return issue.detail }

func (issue LinkIssue) Repair() string { return issue.repair }

func cloneIssues(issues []LinkIssue) []LinkIssue {
	return append([]LinkIssue(nil), issues...)
}

func sortIssues(issues []LinkIssue) {
	sort.Slice(issues, func(left, right int) bool {
		return issueKey(issues[left]) < issueKey(issues[right])
	})
}

func issueKey(issue LinkIssue) string {
	return string(issue.code) + "\x00" + issue.location.String() + "\x00" + issue.subject
}
