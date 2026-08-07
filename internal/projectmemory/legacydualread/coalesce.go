package legacydualread

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/m0n0x41d/haft/internal/projectmemory/legacyimport"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
)

type carrierLocatorKey struct {
	ref     string
	edition string
	digest  string
}

type bridgeResolutionIndex struct {
	exact      map[string]ExactIdentityResolution
	ambiguous  map[string]AmbiguousIdentityResolution
	issues     []CoalescingIssue
	allBridges []IdentityBridge
}

func Coalesce(
	directory typedmemorystore.CurrentEntityDirectory,
	graph typedmemorystore.CurrentProjectGraphObservation,
	report legacyimport.DryRunReport,
	bridges []IdentityBridge,
) (View, error) {
	if err := validateInput(directory, graph, report); err != nil {
		return View{}, err
	}
	canonicalInputBridges, err := validateBridges(
		directory.ProjectID(),
		bridges,
	)
	if err != nil {
		return View{}, err
	}
	carrierIndex := indexCarriers(report.CarrierCatalog())
	legacyIdentities, err := collectLegacyIdentities(
		report,
		carrierIndex,
	)
	if err != nil {
		return View{}, err
	}
	resolutions := buildBridgeResolutionIndex(
		directory,
		legacyIdentities,
		canonicalInputBridges,
	)
	carriers, associations, err := projectLegacyReads(
		report,
		carrierIndex,
		resolutions,
	)
	if err != nil {
		return View{}, err
	}
	canonical, err := encodeView(
		directory,
		graph,
		report,
		canonicalInputBridges,
		carriers,
		associations,
		resolutions.issues,
	)
	if err != nil {
		return View{}, err
	}
	digest, err := digestBytes(canonical)
	if err != nil {
		return View{}, err
	}
	return View{
		directory:    directory,
		graph:        graph,
		legacyReport: report,
		bridges:      canonicalInputBridges,
		carriers:     carriers,
		associations: associations,
		issues:       append([]CoalescingIssue(nil), resolutions.issues...),
		canonical:    canonical,
		digest:       digest,
	}, nil
}

func validateInput(
	directory typedmemorystore.CurrentEntityDirectory,
	graph typedmemorystore.CurrentProjectGraphObservation,
	report legacyimport.DryRunReport,
) error {
	if err := directory.Verify(); err != nil {
		return fmt.Errorf("dual-read typed entity directory: %w", err)
	}
	if err := graph.Verify(); err != nil {
		return fmt.Errorf("dual-read typed graph observation: %w", err)
	}
	basis := graph.GraphSnapshotBasis()
	correlated := directory.ProjectID() == basis.Project() &&
		directory.ProjectID() == report.ProjectID() &&
		directory.GraphSnapshotBasis().Ref() == basis.Ref() &&
		directory.ActiveTypeEnv() == graph.ActiveTypeEnv()
	if !correlated {
		return fmt.Errorf(
			"dual-read typed and legacy sources are not project/snapshot correlated",
		)
	}
	if len(report.CanonicalBytes()) == 0 ||
		report.SourceSnapshotDigest().String() == "" ||
		report.Digest().String() == "" {
		return fmt.Errorf("dual-read legacy report is invalid")
	}
	return nil
}

func validateBridges(
	projectID interface{ String() string },
	bridges []IdentityBridge,
) ([]IdentityBridge, error) {
	owned := canonicalBridges(bridges)
	for index, bridge := range owned {
		if !bridge.valid() {
			return nil, fmt.Errorf(
				"dual-read identity bridge %d is invalid",
				index,
			)
		}
		if bridge.ProjectID().String() != projectID.String() {
			return nil, fmt.Errorf(
				"dual-read identity bridge %d belongs to another project",
				index,
			)
		}
	}
	return owned, nil
}

func indexCarriers(
	catalog legacyimport.CarrierCatalog,
) map[carrierLocatorKey]legacyimport.CarrierSnapshot {
	result := make(
		map[carrierLocatorKey]legacyimport.CarrierSnapshot,
		catalog.Len(),
	)
	for _, carrier := range catalog.Snapshots() {
		result[carrierKey(carrier.Locator())] = carrier
	}
	return result
}

func carrierKey(locator legacyimport.CarrierLocator) carrierLocatorKey {
	return carrierLocatorKey{
		ref:     locator.Ref().String(),
		edition: locator.Edition().String(),
		digest:  locator.Digest().String(),
	}
}

func collectLegacyIdentities(
	report legacyimport.DryRunReport,
	carriers map[carrierLocatorKey]legacyimport.CarrierSnapshot,
) (map[string]legacyimport.LegacyIdentityRef, error) {
	result := make(map[string]legacyimport.LegacyIdentityRef)
	for _, item := range report.Items() {
		for _, observation := range item.Observations() {
			carrier, found := carriers[carrierKey(observation.Carrier())]
			if !found {
				return nil, fmt.Errorf(
					"dual-read observation names an unavailable carrier",
				)
			}
			collectCarrierIdentity(result, carrier)
			association, associationFound :=
				observation.(legacyimport.AssociationObservation)
			if !associationFound {
				continue
			}
			result[association.Source().String()] = association.Source()
			result[association.Target().String()] = association.Target()
		}
	}
	return result, nil
}

func collectCarrierIdentity(
	result map[string]legacyimport.LegacyIdentityRef,
	carrier legacyimport.CarrierSnapshot,
) {
	identified, found :=
		carrier.LegacyIdentity().(legacyimport.IdentifiedLegacyCarrier)
	if !found {
		return
	}
	result[identified.Ref().String()] = identified.Ref()
}

func buildBridgeResolutionIndex(
	directory typedmemorystore.CurrentEntityDirectory,
	legacyIdentities map[string]legacyimport.LegacyIdentityRef,
	bridges []IdentityBridge,
) bridgeResolutionIndex {
	currentTargets := indexTypedTargets(directory)
	grouped := groupBridgesByLegacy(bridges)
	index := bridgeResolutionIndex{
		exact:      make(map[string]ExactIdentityResolution),
		ambiguous:  make(map[string]AmbiguousIdentityResolution),
		allBridges: bridges,
	}
	legacyRefs := make([]string, 0, len(grouped))
	for legacyRef := range grouped {
		legacyRefs = append(legacyRefs, legacyRef)
	}
	sort.Strings(legacyRefs)
	for _, legacyRef := range legacyRefs {
		group := grouped[legacyRef]
		candidates := distinctBridgeTargets(group)
		legacy := group[0].LegacyIdentity()
		_, sourcePresent := legacyIdentities[legacyRef]
		appendBridgePresenceIssues(
			&index,
			group,
			sourcePresent,
			currentTargets,
		)
		if len(candidates) > 1 {
			ambiguous := newAmbiguousIdentityResolution(
				legacy,
				candidates,
			)
			index.ambiguous[legacyRef] = ambiguous
			index.issues = append(
				index.issues,
				newIdentityCollisionIssue(legacy, candidates, group),
			)
			continue
		}
		target := candidates[0]
		_, targetPresent := currentTargets[target.key()]
		if !sourcePresent || !targetPresent {
			continue
		}
		index.exact[legacyRef] = newExactIdentityResolution(
			legacy,
			target,
			group,
		)
	}
	sortIssues(index.issues)
	return index
}

func indexTypedTargets(
	directory typedmemorystore.CurrentEntityDirectory,
) map[string]TypedTarget {
	result := make(map[string]TypedTarget)
	for _, entry := range directory.Entries() {
		target := newTypedTarget(entry.Entity(), entry.Context())
		result[target.key()] = target
	}
	return result
}

func groupBridgesByLegacy(
	bridges []IdentityBridge,
) map[string][]IdentityBridge {
	result := make(map[string][]IdentityBridge)
	for _, bridge := range bridges {
		key := bridge.LegacyIdentity().String()
		result[key] = append(result[key], bridge)
	}
	for key, values := range result {
		result[key] = canonicalBridges(values)
	}
	return result
}

func distinctBridgeTargets(bridges []IdentityBridge) []TypedTarget {
	byKey := make(map[string]TypedTarget)
	for _, bridge := range bridges {
		target := newTypedTarget(
			bridge.EntityID(),
			bridge.BoundedContext(),
		)
		byKey[target.key()] = target
	}
	result := make([]TypedTarget, 0, len(byKey))
	for _, target := range byKey {
		result = append(result, target)
	}
	sort.Slice(result, func(left, right int) bool {
		return result[left].key() < result[right].key()
	})
	return result
}

func appendBridgePresenceIssues(
	index *bridgeResolutionIndex,
	bridges []IdentityBridge,
	sourcePresent bool,
	currentTargets map[string]TypedTarget,
) {
	for _, bridge := range bridges {
		if !sourcePresent {
			index.issues = append(
				index.issues,
				newBridgeLegacySourceAbsentIssue(bridge),
			)
		}
		target := newTypedTarget(
			bridge.EntityID(),
			bridge.BoundedContext(),
		)
		if _, present := currentTargets[target.key()]; present {
			continue
		}
		index.issues = append(
			index.issues,
			newBridgeTargetAbsentIssue(bridge),
		)
	}
}

func projectLegacyReads(
	report legacyimport.DryRunReport,
	carriers map[carrierLocatorKey]legacyimport.CarrierSnapshot,
	resolutions bridgeResolutionIndex,
) (
	[]LegacyCarrierRead,
	[]LegacyAssociationRead,
	error,
) {
	carrierReads := make([]LegacyCarrierRead, 0)
	associationReads := make([]LegacyAssociationRead, 0)
	for _, item := range report.Items() {
		for _, observation := range item.Observations() {
			carrier, found := carriers[carrierKey(observation.Carrier())]
			if !found {
				return nil, nil, fmt.Errorf(
					"dual-read classification names an unavailable carrier",
				)
			}
			switch exact := observation.(type) {
			case legacyimport.CarrierObservation:
				carrierReads = append(
					carrierReads,
					projectCarrierRead(item, carrier, resolutions),
				)
			case legacyimport.AssociationObservation:
				associationReads = append(
					associationReads,
					projectAssociationRead(
						item,
						carrier,
						exact,
						resolutions,
					),
				)
			default:
				return nil, nil, fmt.Errorf(
					"dual-read observation variant %T is unsupported",
					observation,
				)
			}
		}
	}
	sortCarrierReads(carrierReads)
	sortAssociationReads(associationReads)
	return carrierReads, associationReads, nil
}

func projectCarrierRead(
	classification legacyimport.SubjectClassification,
	carrier legacyimport.CarrierSnapshot,
	resolutions bridgeResolutionIndex,
) LegacyCarrierRead {
	return LegacyCarrierRead{
		subject:        classification.Subject(),
		classification: classification.Kind(),
		carrier:        carrier,
		resolution:     resolveCarrierIdentity(carrier, resolutions),
	}
}

func resolveCarrierIdentity(
	carrier legacyimport.CarrierSnapshot,
	resolutions bridgeResolutionIndex,
) IdentityResolution {
	identified, found :=
		carrier.LegacyIdentity().(legacyimport.IdentifiedLegacyCarrier)
	if !found {
		return IdentityUnavailableResolution{}
	}
	return resolveLegacyIdentity(identified.Ref(), resolutions)
}

func projectAssociationRead(
	classification legacyimport.SubjectClassification,
	carrier legacyimport.CarrierSnapshot,
	observation legacyimport.AssociationObservation,
	resolutions bridgeResolutionIndex,
) LegacyAssociationRead {
	return LegacyAssociationRead{
		subject:        classification.Subject(),
		classification: classification.Kind(),
		carrier:        carrier,
		observation:    observation,
		source: resolveLegacyIdentity(
			observation.Source(),
			resolutions,
		),
		target: resolveLegacyIdentity(
			observation.Target(),
			resolutions,
		),
	}
}

func resolveLegacyIdentity(
	legacy legacyimport.LegacyIdentityRef,
	resolutions bridgeResolutionIndex,
) IdentityResolution {
	if exact, found := resolutions.exact[legacy.String()]; found {
		return exact
	}
	if ambiguous, found := resolutions.ambiguous[legacy.String()]; found {
		return ambiguous
	}
	return newUnboundIdentityResolution(legacy)
}

func sortCarrierReads(values []LegacyCarrierRead) {
	sort.Slice(values, func(left, right int) bool {
		leftKey := carrierReadSortKey(values[left])
		rightKey := carrierReadSortKey(values[right])
		return leftKey < rightKey
	})
}

func carrierReadSortKey(read LegacyCarrierRead) string {
	return read.Subject().String() +
		"\x00" +
		read.Carrier().Ref().String() +
		"\x00" +
		read.Carrier().Edition().String()
}

func sortAssociationReads(values []LegacyAssociationRead) {
	sort.Slice(values, func(left, right int) bool {
		leftKey := associationReadSortKey(values[left])
		rightKey := associationReadSortKey(values[right])
		return leftKey < rightKey
	})
}

func associationReadSortKey(read LegacyAssociationRead) string {
	return read.Subject().String() +
		"\x00" +
		read.Carrier().Ref().String() +
		"\x00" +
		read.Carrier().Edition().String()
}

func sortIssues(values []CoalescingIssue) {
	sort.Slice(values, func(left, right int) bool {
		leftBytes, _ := json.Marshal(values[left].issueDTO())
		rightBytes, _ := json.Marshal(values[right].issueDTO())
		return string(leftBytes) < string(rightBytes)
	})
}

type viewDTO struct {
	SchemaVersion       string                  `json:"schema_version"`
	ProjectID           string                  `json:"project_id"`
	TypeEnvRef          string                  `json:"typeenv_ref"`
	GraphRevision       uint64                  `json:"graph_revision"`
	GraphBasisRef       string                  `json:"graph_basis_ref"`
	EntityDirectoryHash string                  `json:"entity_directory_digest"`
	TypedAssertions     []typedAssertionDTO     `json:"typed_current_assertions"`
	LegacyReportDigest  string                  `json:"legacy_report_digest"`
	LegacySourceDigest  string                  `json:"legacy_source_digest"`
	IdentityBridges     []identityBridgeViewDTO `json:"identity_bridges"`
	LegacyCarriers      []legacyCarrierDTO      `json:"legacy_carriers"`
	LegacyAssociations  []legacyAssociationDTO  `json:"legacy_associations"`
	Issues              []coalescingIssueDTO    `json:"issues"`
}

type typedAssertionDTO struct {
	AssertionID    string `json:"assertion_id"`
	Digest         string `json:"digest"`
	OriginEvent    string `json:"origin_event"`
	OriginRevision uint64 `json:"origin_revision"`
	ChangeOrdinal  uint64 `json:"change_ordinal"`
}

type identityBridgeViewDTO struct {
	Digest         string            `json:"digest"`
	LegacyIdentity string            `json:"legacy_identity"`
	EntityID       string            `json:"entity_id"`
	BoundedContext string            `json:"bounded_context"`
	MappingCarrier mappingCarrierDTO `json:"mapping_carrier"`
}

type legacyCarrierDTO struct {
	Subject        string                `json:"subject"`
	Classification string                `json:"classification"`
	Carrier        legacyCarrierBasisDTO `json:"carrier"`
	Resolution     identityResolutionDTO `json:"identity_resolution"`
}

type legacyCarrierBasisDTO struct {
	SourceCoordinate string `json:"source_coordinate"`
	Ref              string `json:"ref"`
	Edition          string `json:"edition"`
	Digest           string `json:"digest"`
	Format           string `json:"format"`
}

type identityResolutionDTO struct {
	Kind       string           `json:"kind"`
	LegacyRef  string           `json:"legacy_ref,omitempty"`
	Target     *typedTargetDTO  `json:"target,omitempty"`
	Candidates []typedTargetDTO `json:"candidates,omitempty"`
	BridgeRefs []string         `json:"bridge_digests,omitempty"`
}

type legacyAssociationDTO struct {
	Subject          string                `json:"subject"`
	Classification   string                `json:"classification"`
	SemanticPosture  string                `json:"semantic_posture"`
	Carrier          legacyCarrierBasisDTO `json:"carrier"`
	SourceLegacyRef  string                `json:"source_legacy_ref"`
	TargetLegacyRef  string                `json:"target_legacy_ref"`
	OpaqueLabel      string                `json:"opaque_label"`
	SourceResolution identityResolutionDTO `json:"source_resolution"`
	TargetResolution identityResolutionDTO `json:"target_resolution"`
}

func encodeView(
	directory typedmemorystore.CurrentEntityDirectory,
	graph typedmemorystore.CurrentProjectGraphObservation,
	report legacyimport.DryRunReport,
	bridges []IdentityBridge,
	carriers []LegacyCarrierRead,
	associations []LegacyAssociationRead,
	issues []CoalescingIssue,
) ([]byte, error) {
	basis := graph.GraphSnapshotBasis()
	dto := viewDTO{
		SchemaVersion:       ViewSchemaVersionV1,
		ProjectID:           directory.ProjectID().String(),
		TypeEnvRef:          directory.ActiveTypeEnv().String(),
		GraphRevision:       basis.GraphRevision().Value(),
		GraphBasisRef:       basis.Ref().String(),
		EntityDirectoryHash: directory.Digest().String(),
		TypedAssertions:     typedAssertionDTOs(graph),
		LegacyReportDigest:  report.Digest().String(),
		LegacySourceDigest:  report.SourceSnapshotDigest().String(),
		IdentityBridges:     identityBridgeViewDTOs(bridges),
		LegacyCarriers:      legacyCarrierDTOs(carriers),
		LegacyAssociations:  legacyAssociationDTOs(associations),
		Issues:              coalescingIssueDTOs(issues),
	}
	canonical, err := json.Marshal(dto)
	if err != nil {
		return nil, fmt.Errorf("encode legacy dual-read view: %w", err)
	}
	return canonical, nil
}

func typedAssertionDTOs(
	graph typedmemorystore.CurrentProjectGraphObservation,
) []typedAssertionDTO {
	assertions := graph.ActiveAssertions().Relations()
	result := make([]typedAssertionDTO, 0, len(assertions))
	for _, assertion := range assertions {
		result = append(result, typedAssertionDTO{
			AssertionID:    assertion.AssertionID().String(),
			Digest:         assertion.Digest().String(),
			OriginEvent:    assertion.OriginEvent().String(),
			OriginRevision: assertion.OriginRevision().Value(),
			ChangeOrdinal:  assertion.ChangeOrdinal(),
		})
	}
	return result
}

func identityBridgeViewDTOs(
	bridges []IdentityBridge,
) []identityBridgeViewDTO {
	result := make([]identityBridgeViewDTO, 0, len(bridges))
	for _, bridge := range bridges {
		result = append(result, identityBridgeViewDTO{
			Digest:         bridge.Digest().String(),
			LegacyIdentity: bridge.LegacyIdentity().String(),
			EntityID:       bridge.EntityID().String(),
			BoundedContext: bridge.BoundedContext().String(),
			MappingCarrier: mappingCarrierDTOOf(
				bridge.MappingCarrier(),
			),
		})
	}
	return result
}

func legacyCarrierDTOs(values []LegacyCarrierRead) []legacyCarrierDTO {
	result := make([]legacyCarrierDTO, 0, len(values))
	for _, read := range values {
		result = append(result, legacyCarrierDTO{
			Subject:        read.Subject().String(),
			Classification: string(read.Classification()),
			Carrier:        legacyCarrierBasisDTOOf(read.Carrier()),
			Resolution: identityResolutionDTOOf(
				read.IdentityResolution(),
			),
		})
	}
	return result
}

func legacyCarrierBasisDTOOf(
	carrier legacyimport.CarrierSnapshot,
) legacyCarrierBasisDTO {
	return legacyCarrierBasisDTO{
		SourceCoordinate: carrier.SourceCoordinate().String(),
		Ref:              carrier.Ref().String(),
		Edition:          carrier.Edition().String(),
		Digest:           carrier.Digest().String(),
		Format:           carrier.Format().String(),
	}
}

func legacyAssociationDTOs(
	values []LegacyAssociationRead,
) []legacyAssociationDTO {
	result := make([]legacyAssociationDTO, 0, len(values))
	for _, read := range values {
		observation := read.Observation()
		result = append(result, legacyAssociationDTO{
			Subject:         read.Subject().String(),
			Classification:  string(read.Classification()),
			SemanticPosture: read.SemanticPosture(),
			Carrier:         legacyCarrierBasisDTOOf(read.Carrier()),
			SourceLegacyRef: observation.Source().String(),
			TargetLegacyRef: observation.Target().String(),
			OpaqueLabel:     observation.Label().String(),
			SourceResolution: identityResolutionDTOOf(
				read.SourceResolution(),
			),
			TargetResolution: identityResolutionDTOOf(
				read.TargetResolution(),
			),
		})
	}
	return result
}

func identityResolutionDTOOf(
	resolution IdentityResolution,
) identityResolutionDTO {
	switch exact := resolution.(type) {
	case ExactIdentityResolution:
		target := targetDTOs([]TypedTarget{exact.Target()})[0]
		return identityResolutionDTO{
			Kind:       string(exact.Kind()),
			LegacyRef:  exact.LegacyIdentity().String(),
			Target:     &target,
			BridgeRefs: bridgeDigestStrings(exact.Bridges()),
		}
	case UnboundIdentityResolution:
		return identityResolutionDTO{
			Kind:      string(exact.Kind()),
			LegacyRef: exact.LegacyIdentity().String(),
		}
	case AmbiguousIdentityResolution:
		return identityResolutionDTO{
			Kind:       string(exact.Kind()),
			LegacyRef:  exact.LegacyIdentity().String(),
			Candidates: targetDTOs(exact.Candidates()),
		}
	case IdentityUnavailableResolution:
		return identityResolutionDTO{Kind: string(exact.Kind())}
	default:
		return identityResolutionDTO{Kind: "invalid"}
	}
}

func coalescingIssueDTOs(
	issues []CoalescingIssue,
) []coalescingIssueDTO {
	result := make([]coalescingIssueDTO, 0, len(issues))
	for _, issue := range issues {
		result = append(result, issue.issueDTO())
	}
	return result
}
