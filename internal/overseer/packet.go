package overseer

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

func BuildPacket(input BuildInput) (Packet, error) {
	packet := Packet{
		SchemaVersion:         ReviewPacketSchemaVersion,
		CreatedAt:             strings.TrimSpace(input.CreatedAt),
		Producer:              normalizeProducer(input.Producer),
		Subject:               input.Subject,
		RepoState:             input.RepoState,
		ChangedFiles:          normalizeChangedFiles(input.ChangedFiles),
		DeterministicFindings: normalizeDeterministicFindings(input.Governance),
		ContextBudget:         normalizeBudget(input.Budget),
	}

	packet = applyBudget(packet)
	packet.Subject.ArtifactSnapshotHash = artifactSnapshotHash(packet)
	packet.Risk = AssessRisk(packet.ChangedFiles)
	packet.ReviewRequest = ReviewRequestForRisk(packet.Risk)
	packet = enforceByteBudget(packet)

	hash, err := stablePacketHash(packet)
	if err != nil {
		return Packet{}, err
	}

	packet.PacketHash = "sha256:" + hash
	packet.PacketID = "rpkt-" + hash[:16]
	packet = enforceByteBudget(packet)
	packet.Subject.ArtifactSnapshotHash = artifactSnapshotHash(packet)

	hash, err = stablePacketHash(packet)
	if err != nil {
		return Packet{}, err
	}

	packet.PacketHash = "sha256:" + hash
	packet.PacketID = "rpkt-" + hash[:16]
	return packet, nil
}

func stablePacketHash(packet Packet) (string, error) {
	canonical := packet
	canonical.PacketID = ""
	canonical.PacketHash = ""
	canonical.CreatedAt = ""

	encoded, err := json.Marshal(canonical)
	if err != nil {
		return "", fmt.Errorf("marshal stable packet: %w", err)
	}

	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func artifactSnapshotHash(packet Packet) string {
	type stableArtifactSnapshot struct {
		ChangedFiles          []ChangedFile         `json:"changed_files"`
		DeterministicFindings DeterministicFindings `json:"deterministic_findings"`
	}

	encoded, _ := json.Marshal(stableArtifactSnapshot{
		ChangedFiles:          packet.ChangedFiles,
		DeterministicFindings: packet.DeterministicFindings,
	})
	sum := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func normalizeProducer(producer Producer) Producer {
	if strings.TrimSpace(producer.Tool) == "" {
		producer.Tool = DefaultToolName
	}
	if producer.PolicyVersions == nil {
		producer.PolicyVersions = map[string]string{}
	}
	if strings.TrimSpace(producer.PolicyVersions["risk"]) == "" {
		producer.PolicyVersions["risk"] = RiskPolicyVersion
	}
	if strings.TrimSpace(producer.PolicyVersions["scope"]) == "" {
		producer.PolicyVersions["scope"] = ScopePolicyVersion
	}
	return producer
}

func normalizeBudget(budget ContextBudget) ContextBudget {
	defaults := DefaultContextBudget()
	if budget.MaxPacketBytes == 0 {
		budget.MaxPacketBytes = defaults.MaxPacketBytes
	}
	if budget.MaxChangedFilesListed == 0 {
		budget.MaxChangedFilesListed = defaults.MaxChangedFilesListed
	}
	if budget.MaxInlineDiffBytes == 0 {
		budget.MaxInlineDiffBytes = defaults.MaxInlineDiffBytes
	}
	if budget.MaxArtifactRefs == 0 {
		budget.MaxArtifactRefs = defaults.MaxArtifactRefs
	}
	if strings.TrimSpace(budget.FullSourcePolicy) == "" {
		budget.FullSourcePolicy = defaults.FullSourcePolicy
	}
	if strings.TrimSpace(budget.OmissionPolicy) == "" {
		budget.OmissionPolicy = defaults.OmissionPolicy
	}
	return budget
}

func normalizeChangedFiles(files []ChangedFile) []ChangedFile {
	out := make([]ChangedFile, 0, len(files))

	for _, file := range files {
		file.Path = normalizePath(file.Path)
		file.Status = strings.TrimSpace(file.Status)
		file.Language = strings.TrimSpace(file.Language)
		file.InlineDiffRef = strings.TrimSpace(file.InlineDiffRef)
		file.FullDiffHandle = strings.TrimSpace(file.FullDiffHandle)
		file.Governance = normalizeFileGovernance(file.Governance)
		out = append(out, file)
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Path < out[j].Path
	})

	return out
}

func normalizeFileGovernance(governance ChangedFileGovernance) ChangedFileGovernance {
	if strings.TrimSpace(governance.ModuleState) == "" {
		governance.ModuleState = "blind"
	}

	governance.AffectedDecisions = normalizeArtifactRefs(governance.AffectedDecisions)
	governance.AffectedSpecSections = stableUniqueStrings(governance.AffectedSpecSections)
	governance.AffectedInvariants = normalizeInvariantRefs(governance.AffectedInvariants)
	governance.PathPolicies = stableUniqueStrings(governance.PathPolicies)
	return governance
}

func normalizeArtifactRefs(refs []ArtifactRef) []ArtifactRef {
	byID := make(map[string]ArtifactRef)

	for _, ref := range refs {
		ref.ID = strings.TrimSpace(ref.ID)
		ref.Title = strings.TrimSpace(ref.Title)
		if ref.ID == "" {
			continue
		}
		if existing, ok := byID[ref.ID]; ok && existing.Title != "" {
			continue
		}
		byID[ref.ID] = ref
	}

	out := make([]ArtifactRef, 0, len(byID))
	for _, ref := range byID {
		out = append(out, ref)
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})

	return out
}

func normalizeInvariantRefs(refs []InvariantRef) []InvariantRef {
	byID := make(map[string]InvariantRef)

	for _, ref := range refs {
		ref.ID = strings.TrimSpace(ref.ID)
		ref.Text = strings.TrimSpace(ref.Text)
		ref.SourceRef = strings.TrimSpace(ref.SourceRef)
		if ref.ID == "" || ref.Text == "" {
			continue
		}
		byID[ref.ID] = ref
	}

	out := make([]InvariantRef, 0, len(byID))
	for _, ref := range byID {
		out = append(out, ref)
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})

	return out
}

func normalizeDeterministicFindings(input GovernanceInput) DeterministicFindings {
	return DeterministicFindings{
		Stale:        normalizeFindingSummaries(input.Stale),
		Drift:        normalizeFindingSummaries(input.Drift),
		SpecHealth:   normalizeFindingSummaries(input.SpecHealth),
		CoverageGaps: normalizeFindingSummaries(input.CoverageGaps),
		Suppressed:   input.Suppressed,
	}
}

func normalizeFindingSummaries(findings []FindingSummary) []FindingSummary {
	out := make([]FindingSummary, 0, len(findings))

	for _, finding := range findings {
		finding.ID = strings.TrimSpace(finding.ID)
		finding.Title = strings.TrimSpace(finding.Title)
		finding.Kind = strings.TrimSpace(finding.Kind)
		finding.Category = strings.TrimSpace(finding.Category)
		finding.Reason = strings.TrimSpace(finding.Reason)
		finding.Paths = stableUniqueStrings(finding.Paths)
		if finding.ID == "" && finding.Title == "" && finding.Reason == "" {
			continue
		}
		out = append(out, finding)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].ID == out[j].ID {
			return out[i].Title < out[j].Title
		}
		return out[i].ID < out[j].ID
	})

	return out
}

func applyBudget(packet Packet) Packet {
	maxFiles := packet.ContextBudget.MaxChangedFilesListed
	if maxFiles > 0 && len(packet.ChangedFiles) > maxFiles {
		omitted := packet.ChangedFiles[maxFiles:]
		packet.ChangedFiles = packet.ChangedFiles[:maxFiles]
		packet.Omissions = append(packet.Omissions, Omission{
			Kind:   "changed_files_truncated",
			Count:  len(omitted),
			Reason: "max_changed_files_listed exceeded",
		})
	}

	packet = capArtifactRefs(packet)
	packet = capInvariantRefs(packet)
	packet = capFindingSummaries(packet)
	packet.Omissions = normalizeOmissions(packet.Omissions)
	return packet
}

func capArtifactRefs(packet Packet) Packet {
	remaining := packet.ContextBudget.MaxArtifactRefs
	if remaining <= 0 {
		return packet
	}

	for fileIndex := range packet.ChangedFiles {
		decisions := packet.ChangedFiles[fileIndex].Governance.AffectedDecisions
		if len(decisions) <= remaining {
			remaining -= len(decisions)
			continue
		}

		omitted := len(decisions) - remaining
		kept := decisions[:remaining]
		packet.ChangedFiles[fileIndex].Governance.AffectedDecisions = kept
		packet.ChangedFiles[fileIndex].Governance.AffectedInvariants = invariantsForDecisions(
			packet.ChangedFiles[fileIndex].Governance.AffectedInvariants,
			kept,
		)
		packet.ChangedFiles[fileIndex].Governance.AffectedSpecSections = specSectionsForDecisions(
			packet.ChangedFiles[fileIndex].Governance.AffectedSpecSections,
			kept,
		)
		packet.Omissions = append(packet.Omissions, Omission{
			Kind:   "artifact_refs_truncated",
			Path:   packet.ChangedFiles[fileIndex].Path,
			Count:  omitted,
			Reason: "max_artifact_refs exceeded",
		})
		remaining = 0
	}

	return packet
}

func capInvariantRefs(packet Packet) Packet {
	remaining := packet.ContextBudget.MaxArtifactRefs
	if remaining <= 0 {
		return packet
	}

	for fileIndex := range packet.ChangedFiles {
		invariants := packet.ChangedFiles[fileIndex].Governance.AffectedInvariants
		if len(invariants) <= remaining {
			remaining -= len(invariants)
			continue
		}

		omitted := len(invariants) - remaining
		packet.ChangedFiles[fileIndex].Governance.AffectedInvariants = invariants[:remaining]
		packet.Omissions = append(packet.Omissions, Omission{
			Kind:   "invariant_refs_truncated",
			Path:   packet.ChangedFiles[fileIndex].Path,
			Count:  omitted,
			Reason: "max_artifact_refs exceeded",
		})
		remaining = 0
	}

	return packet
}

func capFindingSummaries(packet Packet) Packet {
	maxFindings := packet.ContextBudget.MaxArtifactRefs
	if maxFindings <= 0 {
		return packet
	}

	var omitted int
	packet.DeterministicFindings.Stale, omitted = capOneFindingList(packet.DeterministicFindings.Stale, maxFindings)
	packet.DeterministicFindings.Suppressed.UnrelatedStale += omitted
	appendFindingOmission(&packet, "stale_findings_truncated", omitted)

	packet.DeterministicFindings.Drift, omitted = capOneFindingList(packet.DeterministicFindings.Drift, maxFindings)
	packet.DeterministicFindings.Suppressed.UnrelatedDrift += omitted
	appendFindingOmission(&packet, "drift_findings_truncated", omitted)

	packet.DeterministicFindings.SpecHealth, omitted = capOneFindingList(packet.DeterministicFindings.SpecHealth, maxFindings)
	packet.DeterministicFindings.Suppressed.UnrelatedSpecHealth += omitted
	appendFindingOmission(&packet, "spec_health_findings_truncated", omitted)

	packet.DeterministicFindings.CoverageGaps, omitted = capOneFindingList(packet.DeterministicFindings.CoverageGaps, maxFindings)
	packet.DeterministicFindings.Suppressed.UnrelatedCoverageGaps += omitted
	appendFindingOmission(&packet, "coverage_gap_findings_truncated", omitted)

	return packet
}

func capOneFindingList(findings []FindingSummary, maxFindings int) ([]FindingSummary, int) {
	if len(findings) <= maxFindings {
		return findings, 0
	}
	return findings[:maxFindings], len(findings) - maxFindings
}

func appendFindingOmission(packet *Packet, kind string, omitted int) {
	if omitted == 0 {
		return
	}
	packet.Omissions = append(packet.Omissions, Omission{
		Kind:   kind,
		Count:  omitted,
		Reason: "max_artifact_refs exceeded",
	})
}

func invariantsForDecisions(invariants []InvariantRef, decisions []ArtifactRef) []InvariantRef {
	decisionIDs := artifactRefIDs(decisions)
	out := make([]InvariantRef, 0, len(invariants))

	for _, invariant := range invariants {
		if decisionIDs[invariant.SourceRef] {
			out = append(out, invariant)
		}
	}

	return out
}

func specSectionsForDecisions(sections []string, decisions []ArtifactRef) []string {
	if len(decisions) == 0 {
		return nil
	}
	return sections
}

func artifactRefIDs(decisions []ArtifactRef) map[string]bool {
	ids := make(map[string]bool)
	for _, decision := range decisions {
		ids[decision.ID] = true
	}
	return ids
}

func enforceByteBudget(packet Packet) Packet {
	maxBytes := packet.ContextBudget.MaxPacketBytes
	if maxBytes <= 0 {
		return packet
	}

	if packetJSONSize(packet) <= maxBytes {
		return packet
	}

	packet = trimFindingListsForByteBudget(packet, 3)
	if packetJSONSize(packet) <= maxBytes {
		return packet
	}

	packet = trimInvariantRefsForByteBudget(packet, 4)
	if packetJSONSize(packet) <= maxBytes {
		return packet
	}

	return trimChangedFilesForByteBudget(packet, 12)
}

func packetJSONSize(packet Packet) int {
	encoded, _ := json.Marshal(packet)
	return len(encoded)
}

func trimFindingListsForByteBudget(packet Packet, limit int) Packet {
	var omitted int
	packet.DeterministicFindings.Stale, omitted = capOneFindingList(packet.DeterministicFindings.Stale, limit)
	packet.DeterministicFindings.Suppressed.UnrelatedStale += omitted
	appendFindingOmission(&packet, "byte_budget_stale_findings_truncated", omitted)

	packet.DeterministicFindings.Drift, omitted = capOneFindingList(packet.DeterministicFindings.Drift, limit)
	packet.DeterministicFindings.Suppressed.UnrelatedDrift += omitted
	appendFindingOmission(&packet, "byte_budget_drift_findings_truncated", omitted)

	packet.DeterministicFindings.SpecHealth, omitted = capOneFindingList(packet.DeterministicFindings.SpecHealth, limit)
	packet.DeterministicFindings.Suppressed.UnrelatedSpecHealth += omitted
	appendFindingOmission(&packet, "byte_budget_spec_health_findings_truncated", omitted)

	packet.DeterministicFindings.CoverageGaps, omitted = capOneFindingList(packet.DeterministicFindings.CoverageGaps, limit)
	packet.DeterministicFindings.Suppressed.UnrelatedCoverageGaps += omitted
	appendFindingOmission(&packet, "byte_budget_coverage_gap_findings_truncated", omitted)

	return packet
}

func trimInvariantRefsForByteBudget(packet Packet, limit int) Packet {
	remaining := limit

	for fileIndex := range packet.ChangedFiles {
		invariants := packet.ChangedFiles[fileIndex].Governance.AffectedInvariants
		if len(invariants) <= remaining {
			remaining -= len(invariants)
			continue
		}

		omitted := len(invariants) - remaining
		packet.ChangedFiles[fileIndex].Governance.AffectedInvariants = invariants[:remaining]
		packet.Omissions = append(packet.Omissions, Omission{
			Kind:   "byte_budget_invariant_refs_truncated",
			Path:   packet.ChangedFiles[fileIndex].Path,
			Count:  omitted,
			Reason: "max_packet_bytes exceeded",
		})
		remaining = 0
	}

	return packet
}

func trimChangedFilesForByteBudget(packet Packet, limit int) Packet {
	if len(packet.ChangedFiles) <= limit {
		return packet
	}

	omitted := len(packet.ChangedFiles) - limit
	packet.ChangedFiles = packet.ChangedFiles[:limit]
	packet.Omissions = append(packet.Omissions, Omission{
		Kind:   "byte_budget_changed_files_truncated",
		Count:  omitted,
		Reason: "max_packet_bytes exceeded",
	})

	return packet
}

func normalizeOmissions(omissions []Omission) []Omission {
	out := make([]Omission, 0, len(omissions))

	for _, omission := range omissions {
		omission.Kind = strings.TrimSpace(omission.Kind)
		omission.Path = normalizePath(omission.Path)
		omission.FetchHandle = strings.TrimSpace(omission.FetchHandle)
		omission.Reason = strings.TrimSpace(omission.Reason)
		if omission.Kind == "" {
			continue
		}
		out = append(out, omission)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind == out[j].Kind {
			return out[i].Path < out[j].Path
		}
		return out[i].Kind < out[j].Kind
	})

	return out
}
