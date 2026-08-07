package specmigrationv2

import (
	"fmt"
	"sort"

	"github.com/m0n0x41d/haft/internal/projectprofile"
)

type validationState struct {
	request     StructuralRequest
	diagnostics []Diagnostic
}

func AnalyzeStructure(request StructuralRequest) StructuralAnalysisResult {
	validated, err := NewStructuralRequest(StructuralRequestInput{
		Packet:           request.packet,
		ProjectRoot:      request.projectRoot,
		Source:           request.source,
		Target:           request.target,
		TargetClaims:     request.targetClaims,
		OutsideSnapshots: request.outsideSnapshots,
	})
	if err != nil {
		diagnostic := newDiagnostic(
			DiagnosticInvalidCoreVariant,
			"structural-request",
			err.Error(),
		)
		set := newDiagnosticSet([]Diagnostic{diagnostic})
		return invalidDiagnostics{diagnostics: set}
	}
	request = validated
	state := validationState{request: request, diagnostics: []Diagnostic{}}
	state = validateCarrierIdentities(state)
	state = validateSourceProvenance(state)
	state = validateSourceSnapshot(state)
	state = validateTargetSnapshot(state)
	state = validateSourceInventory(state)
	state = validateSourceCoverage(state)
	if len(state.diagnostics) > 0 {
		set := newDiagnosticSet(state.diagnostics)
		return invalidDiagnostics{diagnostics: set}
	}
	packet := request.packet
	packetDigest, err := PacketDigestOf(packet)
	if err != nil {
		diagnostic := newDiagnostic(
			DiagnosticInvalidCoreVariant,
			packet.id.String(),
			err.Error(),
		)
		set := newDiagnosticSet([]Diagnostic{diagnostic})
		return invalidDiagnostics{diagnostics: set}
	}
	lineageDigest, err := LineagePolicyDigestOf(packet.lineagePolicy)
	if err != nil {
		diagnostic := newDiagnostic(
			DiagnosticInvalidCoreVariant,
			packet.id.String(),
			err.Error(),
		)
		set := newDiagnosticSet([]Diagnostic{diagnostic})
		return invalidDiagnostics{diagnostics: set}
	}
	analysis := structuralAnalysis{
		packetID:         packet.id,
		packetDigest:     packetDigest,
		sourceCarrier:    packet.source.carrier,
		sourceDigest:     packet.source.digest,
		targetCarrier:    packet.target.carrier,
		targetDigest:     packet.target.digest,
		archiveCarrier:   packet.source.archive.carrier,
		sourceProvenance: packet.source.provenance,
		lineagePolicy:    packet.lineagePolicy,
		lineageDigest:    lineageDigest,
		dispositionCount: len(packet.sourceDispositions),
	}
	return validAnalysis{analysis: analysis}
}

func DryRun(request DryRunRequest) DryRunResult {
	if request.profileBasis == dryRunCanonicalProfileBasis {
		return dryRunCanonical(request)
	}
	applicability := projectprofile.ResolveSoftwareSystemSpecMigration(request.profile)
	switch result := applicability.(type) {
	case projectprofile.Underdetermined:
		return underdetermined{applicability: result}
	default:
		diagnostic := newDiagnostic(
			DiagnosticInvalidCoreVariant,
			"project-profile-applicability",
			"projectprofile resolver returned an unknown applicability variant",
		)
		set := newDiagnosticSet([]Diagnostic{diagnostic})
		return invalid{diagnostics: set}
	}
}

func dryRunCanonical(request DryRunRequest) DryRunResult {
	applicability := request.canonicalApplicability
	if underdeterminedValue, ok := applicability.Underdetermined(); ok {
		return canonicalUnderdetermined{applicability: underdeterminedValue}
	}
	if notApplicableValue, ok := applicability.NotApplicable(); ok {
		return notApplicable{applicability: notApplicableValue}
	}
	required, ok := applicability.Required()
	if !ok {
		return invalidCanonicalDryRun("canonical applicability has no exact variant")
	}
	analysisResult := AnalyzeStructure(request.structural)
	valid, ok := analysisResult.(validAnalysis)
	if !ok {
		invalidResult, found := analysisResult.(invalidDiagnostics)
		if found {
			return invalid(invalidResult)
		}
		return invalidCanonicalDryRun("structural analysis returned an unknown variant")
	}
	analysis, ok := valid.analysis.(structuralAnalysis)
	if !ok {
		return invalidCanonicalDryRun("structural analysis is not package-owned")
	}
	switch review := request.review.(type) {
	case pendingMigrationReview:
		return pendingReview{
			applicability: required,
			missing:       review.missing,
		}
	case admittedMigrationReview:
		diagnostics := reviewAnalysisDiagnostics(review, analysis)
		if len(diagnostics.Values()) > 0 {
			return invalid{diagnostics: diagnostics}
		}
		return applicable{
			analysis:      analysis,
			applicability: required,
			review:        review,
		}
	default:
		return invalidCanonicalDryRun("migration review resolution is not package-owned")
	}
}

func reviewAnalysisDiagnostics(
	review admittedMigrationReview,
	analysis structuralAnalysis,
) DiagnosticSet {
	diagnostics := make([]Diagnostic, 0, 6)
	analysisRoot := analysis.sourceProvenance.origin.ProjectRoot().String()
	if review.projectRoot.String() != analysisRoot {
		diagnostics = append(diagnostics, newDiagnostic(
			DiagnosticReviewProjectRootMismatch,
			analysisRoot,
			"semantic review belongs to another project root",
		))
	}
	if !review.packetDigest.Equal(analysis.packetDigest) {
		diagnostics = append(diagnostics, newDiagnostic(
			DiagnosticReviewPacketDigestMismatch,
			analysis.packetID.String(),
			"semantic review does not bind the exact migration packet",
		))
	}
	if review.sourceCarrier.String() != analysis.sourceCarrier.String() {
		diagnostics = append(diagnostics, newDiagnostic(
			DiagnosticReviewSourceCarrierMismatch,
			analysis.sourceCarrier.String(),
			"semantic review does not bind the exact designated source carrier",
		))
	}
	if !review.sourceDigest.Equal(analysis.sourceDigest) {
		diagnostics = append(diagnostics, newDiagnostic(
			DiagnosticReviewSourceDigestMismatch,
			analysis.sourceCarrier.String(),
			"semantic review does not bind the exact designated source",
		))
	}
	softwareBinding, found := softwareReviewBinding(review)
	if !found {
		diagnostics = append(diagnostics, newDiagnostic(
			DiagnosticReviewTargetCarrierMismatch,
			string(ReviewSoftwareSystemCarrier),
			"semantic review has no exact SoftwareSystemSpec review-source carrier binding",
		))
		return newDiagnosticSet(diagnostics)
	}
	if softwareBinding.digest.String() != analysis.targetDigest.String() {
		diagnostics = append(diagnostics, newDiagnostic(
			DiagnosticReviewTargetDigestMismatch,
			analysis.targetCarrier.String(),
			"reviewed SoftwareSystemSpec bytes do not match the exact packet target digest",
		))
	}
	return newDiagnosticSet(diagnostics)
}

func softwareReviewBinding(
	review admittedMigrationReview,
) (ReviewCarrierDigest, bool) {
	for _, binding := range review.targetCarrierDigests.values {
		if binding.role == ReviewSoftwareSystemCarrier {
			return binding, true
		}
	}
	return ReviewCarrierDigest{}, false
}

func invalidCanonicalDryRun(detail string) DryRunResult {
	diagnostic := newDiagnostic(
		DiagnosticInvalidCoreVariant,
		"canonical-dry-run",
		detail,
	)
	return invalid{diagnostics: newDiagnosticSet([]Diagnostic{diagnostic})}
}

func validateSourceProvenance(state validationState) validationState {
	provenance := state.request.packet.source.provenance
	projectRoot := provenance.origin.ProjectRoot().String()
	if projectRoot == state.request.projectRoot.String() {
		return state
	}
	return state.add(
		DiagnosticSourceProvenanceRootMismatch,
		state.request.packet.source.carrier.String(),
		"designated-source provenance project root does not match the current applicability context",
	)
}

func validateCarrierIdentities(state validationState) validationState {
	packet := state.request.packet
	sourceCarrier := packet.source.carrier.String()
	targetCarrier := packet.target.carrier.String()
	archiveCarrier := packet.source.archive.carrier.String()
	if sourceCarrier != state.request.source.carrier.String() {
		state = state.add(
			DiagnosticSourceCarrierMismatch,
			sourceCarrier,
			"source snapshot carrier does not match the packet source carrier",
		)
	}
	if targetCarrier != state.request.target.carrier.String() {
		state = state.add(
			DiagnosticTargetCarrierMismatch,
			targetCarrier,
			"target snapshot carrier does not match the packet target carrier",
		)
	}
	if targetCarrier != state.request.targetClaims.carrier.String() {
		state = state.add(
			DiagnosticTargetCatalogCarrierMismatch,
			targetCarrier,
			"target claim catalog carrier does not match the packet target carrier",
		)
	}
	if sourceCarrier == targetCarrier {
		state = state.add(
			DiagnosticCarrierCollision,
			sourceCarrier,
			"source and target carriers must be distinct",
		)
	}
	if sourceCarrier == archiveCarrier {
		state = state.add(
			DiagnosticCarrierCollision,
			archiveCarrier,
			"archive and source carriers must be distinct",
		)
	}
	if targetCarrier == archiveCarrier {
		state = state.add(
			DiagnosticCarrierCollision,
			archiveCarrier,
			"archive and target carriers must be distinct",
		)
	}
	return state
}

func validateSourceSnapshot(state validationState) validationState {
	manifest := state.request.packet.source
	bytes := state.request.source.bytes
	observedLength := uint64(len(bytes))
	if observedLength != manifest.byteLength.Value() {
		detail := fmt.Sprintf("observed %d bytes; packet pins %d", observedLength, manifest.byteLength.Value())
		state = state.add(DiagnosticSourceLengthMismatch, manifest.carrier.String(), detail)
	}
	if !manifest.digest.equalBytes(bytes) {
		observed := SourceDigestOf(bytes)
		detail := fmt.Sprintf("observed %s; packet pins %s", observed.String(), manifest.digest.String())
		state = state.add(DiagnosticSourceDigestMismatch, manifest.carrier.String(), detail)
	}
	if !manifest.archive.sourceDigest.Equal(manifest.digest) {
		detail := "archive digest must equal the exact designated source digest"
		state = state.add(DiagnosticArchiveDigestMismatch, manifest.archive.carrier.String(), detail)
	}
	return state
}

func validateTargetSnapshot(state validationState) validationState {
	manifest := state.request.packet.target
	bytes := state.request.target.bytes
	observedLength := uint64(len(bytes))
	if observedLength != manifest.byteLength.Value() {
		detail := fmt.Sprintf("observed %d bytes; packet pins %d", observedLength, manifest.byteLength.Value())
		state = state.add(DiagnosticTargetLengthMismatch, manifest.carrier.String(), detail)
	}
	if !manifest.digest.equalBytes(bytes) {
		observed := TargetDigestOf(bytes)
		detail := fmt.Sprintf("observed %s; packet pins %s", observed.String(), manifest.digest.String())
		state = state.add(DiagnosticTargetDigestMismatch, manifest.carrier.String(), detail)
	}
	if !state.request.targetClaims.digest.Equal(manifest.digest) {
		detail := "target claim catalog digest does not equal the packet target digest"
		state = state.add(DiagnosticTargetCatalogDigestMismatch, manifest.carrier.String(), detail)
	}
	return state
}

func validateSourceInventory(state validationState) validationState {
	sections := state.request.packet.source.sections
	derived, err := deriveSourceSections(state.request.source.bytes)
	if err != nil {
		return state.add(
			DiagnosticSourceInventoryParseFailed,
			state.request.packet.source.carrier.String(),
			err.Error(),
		)
	}
	state = validateDerivedSourceInventory(state, sections, derived)
	seen := make(map[string]struct{}, len(sections))
	ordered := append([]SourceSection{}, sections...)
	sort.Slice(ordered, func(left, right int) bool {
		return ordered[left].span.Start() < ordered[right].span.Start()
	})
	var previousEnd uint64
	for index, section := range ordered {
		id := section.id.String()
		if _, exists := seen[id]; exists {
			state = state.add(DiagnosticDuplicateSourceSection, id, "source inventory contains the section more than once")
		}
		seen[id] = struct{}{}
		state = validateExactSpan(
			state,
			section.span,
			state.request.source.bytes,
			DiagnosticSourceSectionOutOfBounds,
			DiagnosticSourceSectionDigestMismatch,
			id,
		)
		if index > 0 && section.span.Start() < previousEnd {
			state = state.add(DiagnosticSourceSectionOverlap, id, "source inventory spans overlap")
		}
		if section.span.End() > previousEnd {
			previousEnd = section.span.End()
		}
	}
	return state
}

func validateDerivedSourceInventory(
	state validationState,
	declared []SourceSection,
	derived []SourceSection,
) validationState {
	declaredByID := indexSourceSections(declared)
	derivedByID := indexSourceSections(derived)
	for id, derivedSection := range derivedByID {
		declaredSection, exists := declaredByID[id]
		if !exists {
			state = state.add(
				DiagnosticSourceInventoryMissing,
				id,
				"byte-derived source section is absent from the packet inventory",
			)
			continue
		}
		if exactSpansEqual(declaredSection.span, derivedSection.span) {
			continue
		}
		state = state.add(
			DiagnosticSourceInventorySpanMismatch,
			id,
			"packet source span or digest does not match the byte-derived section boundary",
		)
	}
	for id := range declaredByID {
		if _, exists := derivedByID[id]; exists {
			continue
		}
		state = state.add(
			DiagnosticSourceInventoryUnexpected,
			id,
			"packet inventory section is absent from byte-derived source headings",
		)
	}
	return state
}

func exactSpansEqual(left, right ExactByteSpan) bool {
	return left.Start() == right.Start() &&
		left.Length().Value() == right.Length().Value() &&
		left.Digest().String() == right.Digest().String()
}

func validateSourceCoverage(state validationState) validationState {
	sections := indexSourceSections(state.request.packet.source.sections)
	counts := make(map[string]int, len(state.request.packet.sourceDispositions))
	for _, mapping := range state.request.packet.sourceDispositions {
		key := mapping.source.String()
		counts[key]++
		section, known := sections[key]
		if !known {
			state = state.add(
				DiagnosticUnknownSourceDisposition,
				key,
				"source disposition does not resolve to an exact source inventory section",
			)
			continue
		}
		state = validateDisposition(state, section, mapping.disposition)
	}
	for key := range sections {
		count := counts[key]
		if count == 0 {
			state = state.add(
				DiagnosticMissingSourceDisposition,
				key,
				"source section has no top-level disposition",
			)
			continue
		}
		if count > 1 {
			state = state.add(
				DiagnosticDuplicateSourceDisposition,
				key,
				"source section has more than one top-level disposition",
			)
		}
	}
	return state
}

func validateDisposition(
	state validationState,
	section SourceSection,
	disposition Disposition,
) validationState {
	switch value := disposition.(type) {
	case MapOne:
		return validateMapOne(state, value)
	case RetireHistory:
		return state
	case OutsidePSS:
		return validateOutsidePSS(state, value)
	case SplitOneToMany:
		return validateSplit(state, section, value)
	default:
		return state.add(
			DiagnosticInvalidCoreVariant,
			section.id.String(),
			"packet contains an unknown disposition variant",
		)
	}
}

func validateMapOne(state validationState, mapping MapOne) validationState {
	available := indexTargetClaims(state.request.targetClaims.claims)
	for _, claim := range mapping.targetClaims.values {
		if _, exists := available[claim.String()]; exists {
			continue
		}
		state = state.add(
			DiagnosticTargetClaimMissing,
			claim.String(),
			"exact target atomic claim ID is absent from the digest-pinned target claim catalog",
		)
	}
	return state
}

func validateOutsidePSS(state validationState, outside OutsidePSS) validationState {
	registrations := indexOutsideRegistrations(state.request.packet.outsideRegistry.values)
	snapshots := indexOutsideSnapshots(state.request.outsideSnapshots.values)
	for _, carrierID := range outside.carriers.values {
		key := carrierID.String()
		registration, registered := registrations[key]
		if !registered {
			state = state.add(
				DiagnosticOutsideCarrierUnregistered,
				key,
				"OutsidePSS reference is absent from the packet registry",
			)
			continue
		}
		snapshot, resolved := snapshots[key]
		if !resolved {
			state = state.add(
				DiagnosticOutsideCarrierUnresolved,
				key,
				"registered OutsidePSS carrier has no exact observed snapshot",
			)
			continue
		}
		if registration.carrier.String() != snapshot.carrier.String() {
			state = state.add(
				DiagnosticOutsideCarrierPathMismatch,
				key,
				"observed OutsidePSS carrier path does not match the packet registry",
			)
		}
		state = validateOutsideCarrierCollision(state, key, registration.carrier)
		if !registration.digest.equalBytes(snapshot.bytes) {
			observed := OutsideCarrierDigestOf(snapshot.bytes)
			detail := fmt.Sprintf("observed %s; registry pins %s", observed.String(), registration.digest.String())
			state = state.add(DiagnosticOutsideCarrierDigestMismatch, key, detail)
		}
	}
	return state
}

func validateOutsideCarrierCollision(
	state validationState,
	id string,
	carrier SourceCarrierID,
) validationState {
	path := carrier.String()
	packet := state.request.packet
	reserved := []string{
		packet.source.carrier.String(),
		packet.target.carrier.String(),
		packet.source.archive.carrier.String(),
	}
	for _, reservedPath := range reserved {
		if path != reservedPath {
			continue
		}
		detail := fmt.Sprintf("OutsidePSS carrier %q collides with a migration source, target, or archive carrier", path)
		state = state.add(DiagnosticCarrierCollision, id, detail)
	}
	return state
}

func validateSplit(
	state validationState,
	section SourceSection,
	split SplitOneToMany,
) validationState {
	branches := append([]SplitBranch{}, split.branches...)
	sort.Slice(branches, func(left, right int) bool {
		return branches[left].fragment.Start() < branches[right].fragment.Start()
	})
	cursor := section.span.Start()
	for index, branch := range branches {
		subject := fmt.Sprintf("%s#branch-%d", section.id.String(), index+1)
		fragment := branch.fragment
		if fragment.Start() < section.span.Start() || fragment.End() > section.span.End() {
			state = state.add(
				DiagnosticSplitFragmentOutOfBounds,
				subject,
				"split branch fragment is outside its exact source section span",
			)
		}
		if fragment.Start() < cursor {
			state = state.add(
				DiagnosticSplitFragmentOverlap,
				subject,
				"split branch fragment overlaps a preceding branch",
			)
		}
		if fragment.Start() > cursor {
			state = state.add(
				DiagnosticSplitFragmentGap,
				subject,
				"split branch partition leaves an uncovered byte range",
			)
		}
		state = validateExactSpan(
			state,
			fragment,
			state.request.source.bytes,
			DiagnosticSplitFragmentOutOfBounds,
			DiagnosticSplitFragmentDigestMismatch,
			subject,
		)
		state = validateBranchDisposition(state, branch.disposition)
		if fragment.End() > cursor {
			cursor = fragment.End()
		}
	}
	if cursor < section.span.End() {
		state = state.add(
			DiagnosticSplitFragmentGap,
			section.id.String(),
			"split branch partition does not cover the end of the exact source section",
		)
	}
	return state
}

func validateBranchDisposition(
	state validationState,
	disposition BranchDisposition,
) validationState {
	switch value := disposition.(type) {
	case MapOne:
		return validateMapOne(state, value)
	case RetireHistory:
		return state
	case OutsidePSS:
		return validateOutsidePSS(state, value)
	default:
		return state.add(
			DiagnosticInvalidCoreVariant,
			"split-branch",
			"split contains an unknown branch disposition variant",
		)
	}
}

func validateExactSpan(
	state validationState,
	span ExactByteSpan,
	bytes []byte,
	outOfBoundsCode DiagnosticCode,
	digestMismatchCode DiagnosticCode,
	subject string,
) validationState {
	byteCount := uint64(len(bytes))
	if span.End() > byteCount {
		return state.add(outOfBoundsCode, subject, "exact byte span exceeds observed source bytes")
	}
	fragment := bytes[span.Start():span.End()]
	if span.digest.equalBytes(fragment) {
		return state
	}
	observed := FragmentDigestOf(fragment)
	detail := fmt.Sprintf("observed %s; packet pins %s", observed.String(), span.digest.String())
	return state.add(digestMismatchCode, subject, detail)
}

func indexSourceSections(values []SourceSection) map[string]SourceSection {
	result := make(map[string]SourceSection, len(values))
	for _, value := range values {
		key := value.id.String()
		if _, exists := result[key]; exists {
			continue
		}
		result[key] = value
	}
	return result
}

func indexTargetClaims(values []TargetAtomicClaimID) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value.String()] = struct{}{}
	}
	return result
}

func indexOutsideRegistrations(values []OutsideCarrierRegistration) map[string]OutsideCarrierRegistration {
	result := make(map[string]OutsideCarrierRegistration, len(values))
	for _, value := range values {
		result[value.id.String()] = value
	}
	return result
}

func indexOutsideSnapshots(values []OutsideCarrierSnapshot) map[string]OutsideCarrierSnapshot {
	result := make(map[string]OutsideCarrierSnapshot, len(values))
	for _, value := range values {
		result[value.id.String()] = value
	}
	return result
}

func (state validationState) add(
	code DiagnosticCode,
	subject string,
	detail string,
) validationState {
	diagnostic := newDiagnostic(code, subject, detail)
	diagnostics := append([]Diagnostic{}, state.diagnostics...)
	diagnostics = append(diagnostics, diagnostic)
	return validationState{request: state.request, diagnostics: diagnostics}
}
