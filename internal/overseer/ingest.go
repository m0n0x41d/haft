package overseer

import (
	"fmt"
	"strings"
)

func IngestReviewResult(stored StoredRun, input ReviewResultInput, reviewedAt string) (StoredRun, error) {
	run := stored.Run
	if run.ReviewRunID == "" {
		return StoredRun{}, fmt.Errorf("review run is required")
	}

	run.SchemaVersion = ReviewResultSchemaVersion
	run.ReviewedAt = strings.TrimSpace(reviewedAt)
	run.Mode = firstNonEmpty(input.Mode, "external_review")
	run.Reviewer = normalizeReviewer(input.Reviewer, stored.Packet)
	run.Authority = DefaultReviewAuthority()
	run.ScopeCoverage = normalizeScopeCoverage(input.ScopeCoverage, stored.Packet)
	run.Findings = normalizeReviewFindings(input.Findings, run.ReviewRunID)
	run.NonFindings = normalizeNonFindings(input.NonFindings)
	run.Verdict = normalizeReviewVerdict(input.Verdict, run.Findings)
	run.Dispositions = preserveMatchingDispositions(run.Dispositions, run.Findings)

	stored.Run = run
	return stored, nil
}

func ApplyDisposition(stored StoredRun, disposition ReviewDisposition) (StoredRun, error) {
	disposition.FindingID = strings.TrimSpace(disposition.FindingID)
	disposition.Status = strings.TrimSpace(disposition.Status)
	disposition.Actor = strings.TrimSpace(disposition.Actor)
	disposition.Reason = strings.TrimSpace(disposition.Reason)
	disposition.CommitSHA = strings.TrimSpace(disposition.CommitSHA)
	disposition.CreatedAt = strings.TrimSpace(disposition.CreatedAt)

	if disposition.FindingID == "" {
		return StoredRun{}, fmt.Errorf("finding_id is required")
	}
	if disposition.Status == "" {
		return StoredRun{}, fmt.Errorf("status is required")
	}
	if !dispositionStatusAllowed(disposition.Status) {
		return StoredRun{}, fmt.Errorf("unknown disposition status %q", disposition.Status)
	}
	if !reviewRunHasFinding(stored.Run, disposition.FindingID) {
		return StoredRun{}, fmt.Errorf("finding %s not found in review run %s", disposition.FindingID, stored.Run.ReviewRunID)
	}

	dispositions := make([]ReviewDisposition, 0, len(stored.Run.Dispositions)+1)
	for _, existing := range stored.Run.Dispositions {
		if existing.FindingID == disposition.FindingID {
			continue
		}
		dispositions = append(dispositions, existing)
	}
	dispositions = append(dispositions, disposition)
	stored.Run.Dispositions = dispositions
	return stored, nil
}

func UnresolvedFindings(run ReviewRun) []ReviewFinding {
	dispositions := dispositionByFinding(run.Dispositions)
	out := make([]ReviewFinding, 0, len(run.Findings))
	for _, finding := range run.Findings {
		disposition := dispositions[finding.ID]
		if dispositionClosesFinding(disposition.Status) {
			continue
		}
		out = append(out, finding)
	}
	return out
}

func normalizeReviewer(reviewer Reviewer, packet Packet) Reviewer {
	reviewer.Agent = firstNonEmpty(reviewer.Agent, "external_reviewer")
	reviewer.SessionRelationToAuthor = firstNonEmpty(reviewer.SessionRelationToAuthor, "independent_review_session")
	if len(reviewer.InputSources) == 0 {
		reviewer.InputSources = []string{packet.PacketID}
	}
	return reviewer
}

func normalizeScopeCoverage(scope ScopeCoverage, packet Packet) ScopeCoverage {
	if len(scope.ModesReviewed) == 0 {
		scope.ModesReviewed = append([]string(nil), packet.ReviewRequest.Modes...)
	}
	if len(scope.FilesReviewed) == 0 {
		scope.FilesReviewed = packetPaths(packet)
	}
	if scope.FetchesUsed == nil {
		scope.FetchesUsed = []string{}
	}
	if scope.Abstentions == nil {
		scope.Abstentions = []string{}
	}
	return scope
}

func normalizeReviewFindings(findings []ReviewFinding, runID string) []ReviewFinding {
	out := make([]ReviewFinding, 0, len(findings))
	seen := make(map[string]bool)
	for index, finding := range findings {
		finding.ID = strings.TrimSpace(finding.ID)
		if finding.ID == "" {
			finding.ID = fmt.Sprintf("%s-finding-%02d", runID, index+1)
		}
		if seen[finding.ID] {
			finding.ID = fmt.Sprintf("%s-finding-%02d", runID, index+1)
		}
		seen[finding.ID] = true

		finding.Severity = normalizeSeverity(finding.Severity)
		finding.Confidence = firstNonEmpty(finding.Confidence, "medium")
		finding.Category = strings.TrimSpace(finding.Category)
		finding.Claim = firstNonEmpty(finding.Claim, finding.Title)
		finding.ConcreteHarm = firstNonEmpty(finding.ConcreteHarm, finding.Description)
		finding.MinimalFix = firstNonEmpty(finding.MinimalFix, finding.Recommendation)
		finding.Title = ""
		finding.Description = ""
		finding.Recommendation = ""
		finding = AdvisoryFindingDefaults(finding)
		if finding.Claim == "" {
			continue
		}
		out = append(out, finding)
	}
	return out
}

func normalizeNonFindings(nonFindings []NonFinding) []NonFinding {
	out := make([]NonFinding, 0, len(nonFindings))
	for _, item := range nonFindings {
		item.Claim = strings.TrimSpace(item.Claim)
		item.Basis = strings.TrimSpace(item.Basis)
		item.Scope = strings.TrimSpace(item.Scope)
		if item.Claim == "" {
			continue
		}
		out = append(out, item)
	}
	return out
}

func normalizeReviewVerdict(verdict string, findings []ReviewFinding) string {
	verdict = strings.TrimSpace(verdict)
	if verdict == "review_abstained" {
		return verdict
	}
	if len(findings) > 0 {
		return "findings_recorded"
	}
	return "reviewed_no_findings"
}

func preserveMatchingDispositions(dispositions []ReviewDisposition, findings []ReviewFinding) []ReviewDisposition {
	valid := make(map[string]bool)
	for _, finding := range findings {
		valid[finding.ID] = true
	}
	out := make([]ReviewDisposition, 0, len(dispositions))
	for _, disposition := range dispositions {
		if valid[disposition.FindingID] {
			out = append(out, disposition)
		}
	}
	return out
}

func reviewRunHasFinding(run ReviewRun, findingID string) bool {
	for _, finding := range run.Findings {
		if finding.ID == findingID {
			return true
		}
	}
	return false
}

func dispositionByFinding(dispositions []ReviewDisposition) map[string]ReviewDisposition {
	out := make(map[string]ReviewDisposition, len(dispositions))
	for _, disposition := range dispositions {
		out[disposition.FindingID] = disposition
	}
	return out
}

func dispositionClosesFinding(status string) bool {
	switch strings.TrimSpace(status) {
	case "fixed", "fixed_by_commit", "false_positive", "waived_by_human", "escalated_to_decision", "superseded_by_rewrite":
		return true
	default:
		return false
	}
}

func dispositionStatusAllowed(status string) bool {
	switch strings.TrimSpace(status) {
	case "open", "fixed", "fixed_by_commit", "false_positive", "waived_by_human", "escalated_to_decision", "superseded_by_rewrite":
		return true
	default:
		return false
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
