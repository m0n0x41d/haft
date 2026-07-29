package goldenconcernbundle

import (
	"fmt"
	"sort"

	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func validateAdmissionSet(
	project projectidentity.ProjectID,
	concern ConcernAdmission,
	coordinate SnapshotCoordinate,
	admissions []AdapterAdmission,
) error {
	if concern.project != project {
		return fmt.Errorf(
			"GoldenConcernBundle concern belongs to another project",
		)
	}
	if concern.declaration.context != coordinate.context {
		return fmt.Errorf(
			"GoldenConcernBundle concern and snapshot contexts differ",
		)
	}
	if concern.reference.RefKind().TypeEnv() != coordinate.typeEnv {
		return fmt.Errorf(
			"GoldenConcernBundle concern reference uses another TypeEnv",
		)
	}
	if len(admissions) == 0 {
		return fmt.Errorf(
			"GoldenConcernBundle requires task-adapter admissions",
		)
	}
	eventRefs := map[string]struct{}{
		concern.receipt.eventRef: {},
	}
	maxRevision := concern.receipt.revision.Value()
	if maxRevision > coordinate.revision.Value() {
		return fmt.Errorf(
			"GoldenConcernBundle snapshot precedes its concern admission",
		)
	}
	for _, admission := range admissions {
		if admission.project != project {
			return fmt.Errorf(
				"GoldenConcernBundle adapter admission belongs to another project",
			)
		}
		if _, found := eventRefs[admission.receipt.eventRef]; found {
			return fmt.Errorf(
				"GoldenConcernBundle repeats admission event %q",
				admission.receipt.eventRef,
			)
		}
		eventRefs[admission.receipt.eventRef] = struct{}{}
		if admission.receipt.revision.Value() > coordinate.revision.Value() {
			return fmt.Errorf(
				"GoldenConcernBundle snapshot precedes adapter event %q",
				admission.receipt.eventRef,
			)
		}
		if admission.receipt.revision.Value() > maxRevision {
			maxRevision = admission.receipt.revision.Value()
		}
		if err := validateAdapterAdmissionCoordinate(
			admission,
			coordinate,
		); err != nil {
			return err
		}
	}
	if maxRevision != coordinate.revision.Value() {
		return fmt.Errorf(
			"GoldenConcernBundle snapshot revision %d does not equal the latest admitted bundle revision %d",
			coordinate.revision.Value(),
			maxRevision,
		)
	}
	return nil
}

func validateAdapterAdmissionCoordinate(
	admission AdapterAdmission,
	coordinate SnapshotCoordinate,
) error {
	if err := admission.manifest.Verify(); err != nil {
		return err
	}
	if err := admission.adapter.Verify(); err != nil {
		return err
	}
	digest, err := digestBytes(admission.canonicalChanges)
	if err != nil {
		return err
	}
	if digest != admission.candidateDigest {
		return fmt.Errorf(
			"GoldenConcernBundle candidate %q bytes differ from its digest",
			admission.receipt.eventRef,
		)
	}
	for _, declaration := range admission.declarations {
		if declaration.context != coordinate.context {
			return fmt.Errorf(
				"GoldenConcernBundle declaration %q uses another context",
				declaration.entity.String(),
			)
		}
	}
	for _, path := range admission.paths {
		if path.context != coordinate.context {
			return fmt.Errorf(
				"GoldenConcernBundle relation %q uses another context",
				path.assertion.String(),
			)
		}
		if path.target.RefKind().TypeEnv() != coordinate.typeEnv {
			return fmt.Errorf(
				"GoldenConcernBundle relation %q target uses another TypeEnv",
				path.assertion.String(),
			)
		}
	}
	for _, value := range admission.values {
		if value.valueKind.TypeEnv() != coordinate.typeEnv {
			return fmt.Errorf(
				"GoldenConcernBundle relation %q value uses another TypeEnv",
				value.assertion.String(),
			)
		}
	}
	return nil
}

func materializeItems(
	concern ConcernAdmission,
	coordinate SnapshotCoordinate,
	admissions []AdapterAdmission,
	specs []ItemSpec,
) ([]BundleItem, error) {
	declarations := make(map[string]map[string]DeclarationWitness)
	revisions := make(map[string]typedmemory.GraphRevision)
	concernEvent := concern.receipt.eventRef
	declarations[concernEvent] = map[string]DeclarationWitness{
		concern.declaration.entity.String(): concern.declaration,
	}
	revisions[concernEvent] = concern.receipt.revision
	for _, admission := range admissions {
		event := admission.receipt.eventRef
		revisions[event] = admission.receipt.revision
		eventDeclarations := make(map[string]DeclarationWitness)
		for _, declaration := range admission.declarations {
			eventDeclarations[declaration.entity.String()] = declaration
		}
		declarations[event] = eventDeclarations
	}
	concernItem := BundleItem{
		role:              ItemEntityOfConcern,
		reference:         concern.reference,
		entity:            concern.declaration.entity,
		label:             concern.declaration.label,
		provenance:        concern.declaration.provenance,
		admissionEventRef: concernEvent,
		admittedRevision:  concern.receipt.revision,
		observedRevision:  coordinate.revision,
		observedAt:        coordinate.observedAt,
	}
	result := []BundleItem{concernItem}
	seen := map[string]struct{}{
		itemReferenceKey(concernItem.reference): {},
	}
	for _, spec := range specs {
		if spec.role == ItemEntityOfConcern {
			return nil, fmt.Errorf(
				"GoldenConcernBundle concern item is derived from its concern admission",
			)
		}
		eventDeclarations, found := declarations[spec.admissionEventRef]
		if !found {
			return nil, fmt.Errorf(
				"GoldenConcernBundle item names unknown event %q",
				spec.admissionEventRef,
			)
		}
		entityID := spec.reference.ReferenceID().String()
		declaration, found := eventDeclarations[entityID]
		if !found {
			return nil, fmt.Errorf(
				"GoldenConcernBundle event %q did not declare item %q",
				spec.admissionEventRef,
				entityID,
			)
		}
		if spec.reference.RefKind().TypeEnv() != coordinate.typeEnv {
			return nil, fmt.Errorf(
				"GoldenConcernBundle item %q uses another TypeEnv",
				entityID,
			)
		}
		key := itemReferenceKey(spec.reference)
		if _, found := seen[key]; found {
			return nil, fmt.Errorf(
				"GoldenConcernBundle repeats exact item reference %q",
				key,
			)
		}
		seen[key] = struct{}{}
		result = append(result, BundleItem{
			role:              spec.role,
			reference:         spec.reference,
			entity:            declaration.entity,
			label:             declaration.label,
			provenance:        declaration.provenance,
			admissionEventRef: spec.admissionEventRef,
			admittedRevision:  revisions[spec.admissionEventRef],
			observedRevision:  coordinate.revision,
			observedAt:        coordinate.observedAt,
		})
	}
	sort.Slice(result, func(left int, right int) bool {
		return itemKey(result[left]) < itemKey(result[right])
	})
	return result, nil
}

func validateGoldenShape(
	concern typedmemory.PersistedRef,
	items []BundleItem,
	paths []RelationPath,
) error {
	if err := validateRequiredItemRoles(items); err != nil {
		return err
	}
	if err := validateRequiredSignatures(paths); err != nil {
		return err
	}
	pathTargets := make(map[string]struct{})
	for _, path := range paths {
		pathTargets[itemReferenceKey(path.target)] = struct{}{}
	}
	for _, item := range items {
		if item.role == ItemEntityOfConcern {
			continue
		}
		if _, found := pathTargets[itemReferenceKey(item.reference)]; !found {
			return fmt.Errorf(
				"GoldenConcernBundle item %q has no exact relation path",
				item.reference.ReferenceID().String(),
			)
		}
	}
	if err := validateConcernConnectivity(concern, items, paths); err != nil {
		return err
	}
	return nil
}

func validateRequiredItemRoles(items []BundleItem) error {
	counts := make(map[ItemRole]int)
	for _, item := range items {
		counts[item.role]++
	}
	exactlyOne := []ItemRole{
		ItemEntityOfConcern,
		ItemProblemCard,
		ItemSolutionPortfolio,
		ItemPortfolioComparison,
		ItemDecisionRecord,
		ItemProjectClaim,
		ItemEvidenceRecord,
		ItemSupportingEpistemeRecord,
		ItemWorkRecord,
		ItemPerformedWorkOccurrence,
		ItemCodeAnchor,
	}
	for _, role := range exactlyOne {
		if counts[role] != 1 {
			return fmt.Errorf(
				"GoldenConcernBundle requires exactly one %s item, got %d",
				role.String(),
				counts[role],
			)
		}
	}
	if counts[ItemSolutionOption] < 2 {
		return fmt.Errorf(
			"GoldenConcernBundle requires at least two retained solution options",
		)
	}
	if counts[ItemSpecSection] < 1 {
		return fmt.Errorf(
			"GoldenConcernBundle requires at least one typed SpecSection",
		)
	}
	return nil
}

func validateRequiredSignatures(paths []RelationPath) error {
	required := []string{
		"Haft.ProblemCardAtConcern",
		"Haft.NoteAtConcern",
		"Haft.SolutionPortfolioAtConcern",
		"Haft.PortfolioComparison",
		"Haft.DecisionChoiceAtConcern",
		"Haft.SpecSectionAtConcern",
		"Haft.ProjectClaimAtConcern",
		"Haft.RecordStatesClaim",
		"Haft.SupportingEpistemeRecordAtConcern",
		"Haft.WorkOccurrenceRecord",
		"Haft.EvidenceUse",
		"Haft.CodeAnchorDefinition",
		"Haft.CodeRealizesClaim",
		"Haft.CodeChangedByWork",
	}
	present := make(map[string]struct{})
	for _, path := range paths {
		present[path.signature.String()] = struct{}{}
	}
	for _, signature := range required {
		if _, found := present[signature]; !found {
			return fmt.Errorf(
				"GoldenConcernBundle is missing required relation %s",
				signature,
			)
		}
	}
	return nil
}

func validateConcernConnectivity(
	concern typedmemory.PersistedRef,
	items []BundleItem,
	paths []RelationPath,
) error {
	relationTargets := make(map[string][]string)
	for _, path := range paths {
		relationKey := path.admissionEventRef +
			"\x00" +
			path.assertion.String()
		entityKey := path.target.ReferenceID().String()
		relationTargets[relationKey] = append(
			relationTargets[relationKey],
			entityKey,
		)
	}
	adjacency := make(map[string]map[string]struct{})
	for _, targets := range relationTargets {
		for left := range targets {
			if _, found := adjacency[targets[left]]; !found {
				adjacency[targets[left]] = make(map[string]struct{})
			}
			for right := range targets {
				if left == right {
					continue
				}
				adjacency[targets[left]][targets[right]] = struct{}{}
			}
		}
	}
	root := concern.ReferenceID().String()
	seen := map[string]struct{}{root: {}}
	queue := []string{root}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for next := range adjacency[current] {
			if _, found := seen[next]; found {
				continue
			}
			seen[next] = struct{}{}
			queue = append(queue, next)
		}
	}
	for _, item := range items {
		if _, found := seen[item.entity.String()]; !found {
			return fmt.Errorf(
				"GoldenConcernBundle item %q is outside the exact concern neighborhood",
				item.entity.String(),
			)
		}
	}
	return nil
}

func itemReferenceKey(reference typedmemory.PersistedRef) string {
	return reference.RefKind().String() +
		"\x00" +
		reference.ReferenceID().String()
}

func itemKey(item BundleItem) string {
	return item.role.String() +
		"\x00" +
		itemReferenceKey(item.reference)
}
