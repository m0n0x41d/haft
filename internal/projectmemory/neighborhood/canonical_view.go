package neighborhood

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

type exactNeighborhoodCanonicalV1 struct {
	Schema         string                     `json:"schema"`
	View           map[string]any             `json:"memory_view_context"`
	Snapshot       map[string]any             `json:"snapshot_basis"`
	Projection     projectionBasisCanonicalV1 `json:"projection_basis"`
	ProjectionHash string                     `json:"projection_basis_digest"`
	Root           map[string]any             `json:"root"`
	Facets         []map[string]any           `json:"facets"`
	Boundaries     map[string]any             `json:"boundaries"`
	Interpretation map[string]any             `json:"interpretation_contract"`
	Affordances    []map[string]any           `json:"read_affordances"`
	AppliedBudget  map[string]any             `json:"applied_budget"`
}

type exactNeighborhoodBoundedContentV1 struct {
	Root        map[string]any   `json:"root"`
	Facets      []map[string]any `json:"facets"`
	Boundaries  map[string]any   `json:"boundaries"`
	Affordances []map[string]any `json:"read_affordances"`
}

func encodeExactNeighborhoodBoundedContent(
	result ExactNeighborhood,
) ([]byte, error) {
	carrier := exactNeighborhoodBoundedContentV1{
		Root:        encodeProjectedRoot(result.root),
		Facets:      encodeNeighborhoodFacets(result.facets),
		Boundaries:  encodeNeighborhoodBoundaries(result.boundaries),
		Affordances: encodeReadAffordances(result.affordances),
	}
	canonical, err := json.Marshal(carrier)
	if err != nil {
		return nil, fmt.Errorf(
			"encode exact-neighborhood bounded content: %w",
			err,
		)
	}
	return canonical, nil
}

func encodeExactNeighborhoodCanonical(
	result ExactNeighborhood,
) ([]byte, error) {
	carrier := exactNeighborhoodCanonicalV1{
		Schema: NeighborhoodContractV1,
		View:   memoryViewContextPayload(result.view),
		Snapshot: map[string]any{
			"graph_revision":  result.snapshot.GraphRevision().Value(),
			"type_env_ref":    result.snapshot.TypeEnv().String(),
			"type_env_digest": result.snapshot.TypeEnvDigest().String(),
		},
		Projection:     encodeProjectionBasisCanonical(result.projection),
		ProjectionHash: result.projection.Digest().String(),
		Root:           encodeProjectedRoot(result.root),
		Facets:         encodeNeighborhoodFacets(result.facets),
		Boundaries:     encodeNeighborhoodBoundaries(result.boundaries),
		Interpretation: encodeInterpretation(result.interpretation),
		Affordances:    encodeReadAffordances(result.affordances),
		AppliedBudget:  encodeAppliedReadBudget(result.budget),
	}
	canonical, err := json.Marshal(carrier)
	if err != nil {
		return nil, fmt.Errorf(
			"encode exact-neighborhood canonical JSON: %w",
			err,
		)
	}
	return canonical, nil
}

func memoryViewContextPayload(view MemoryViewContext) map[string]any {
	return map[string]any{
		"entity_ref": map[string]any{
			"ref_kind_id": view.Entity().
				RefKind().
				ID().
				String(),
			"reference_id": view.Entity().
				ReferenceID().
				String(),
		},
		"bounded_context_ref":    view.Context().String(),
		"projection_profile_ref": view.ProfileRef().String(),
	}
}

func digestExactNeighborhoodCanonical(
	canonical []byte,
) (typedmemory.SHA256Digest, error) {
	sum := sha256.Sum256(canonical)
	raw := "sha256:" + hex.EncodeToString(sum[:])
	return typedmemory.NewSHA256Digest(raw)
}

func encodeProjectionBasisCanonical(
	basis ProjectionBasis,
) projectionBasisCanonicalV1 {
	readSet := basis.DeclaredReadSet()
	readInputs := make([]string, 0, len(readSet.Inputs()))
	for _, input := range readSet.Inputs() {
		readInputs = append(readInputs, string(input))
	}
	readSlots := make([]string, 0, len(readSet.SlotKinds()))
	for _, slot := range readSet.SlotKinds() {
		readSlots = append(readSlots, slot.String())
	}
	return projectionBasisCanonicalV1{
		Schema:           ProjectionBasisSchemaV1,
		ProfileRef:       basis.ProfileRef().String(),
		ProfileEdition:   basis.ProfileEdition(),
		ProfileDigest:    basis.ProfileDigest().String(),
		ProjectionSchema: basis.ProjectionSchemaVersion(),
		CanonicalInputs:  encodeCanonicalInputs(basis.CanonicalInputs()),
		DerivedInputs:    encodeDerivedInputs(basis.DerivedInputs()),
		ReadInputs:       readInputs,
		ReadSlots:        readSlots,
		Manifests: encodeCorrespondenceManifests(
			basis.CorrespondenceManifests(),
		),
		ItemBases: encodeItemBases(basis.ItemBases()),
	}
}

func encodeProjectedRoot(root ProjectedRoot) map[string]any {
	return map[string]any{
		"coordinate":     encodeOutputCoordinate(root.Coordinate()),
		"text":           root.Text().String(),
		"postures":       encodeItemPostures(root.Postures()),
		"provenance_ref": root.Provenance().String(),
	}
}

func encodeNeighborhoodFacets(
	facets []NeighborhoodFacet,
) []map[string]any {
	result := make([]map[string]any, 0, len(facets))
	for _, facet := range facets {
		items := make([]map[string]any, 0, len(facet.Items()))
		for _, item := range facet.Items() {
			items = append(items, encodeNeighborhoodItem(item))
		}
		result = append(result, map[string]any{
			"facet":    string(facet.Kind()),
			"coverage": encodeFacetCoverage(facet.Coverage()),
			"items":    items,
		})
	}
	return result
}

func encodeNeighborhoodItem(item NeighborhoodItem) map[string]any {
	return map[string]any{
		"coordinate":     encodeOutputCoordinate(item.Coordinate()),
		"item_kind":      string(item.ItemKind()),
		"text":           item.Text().String(),
		"postures":       encodeItemPostures(item.Postures()),
		"provenance_ref": item.Provenance().String(),
		"why_included":   encodeRelationWitnesses(item.WhyIncluded()),
	}
}

func encodeItemPostures(postures ItemPostures) map[string]any {
	return map[string]any{
		"semantic":             string(postures.Semantic()),
		"lifecycle":            string(postures.Lifecycle()),
		"evidence_currentness": string(postures.Evidence()),
		"projection_freshness": string(postures.Projection()),
	}
}

func encodeFacetCoverage(coverage FacetCoverage) map[string]any {
	result := map[string]any{
		"kind":     string(coverage.Kind()),
		"included": coverage.Included(),
	}
	switch value := coverage.(type) {
	case CompleteCoverage:
		return result
	case PartialCoverage:
		result["omitted_at_least"] = value.OmittedAtLeast()
		result["snapshot_cursor"] = encodeSnapshotCursor(value.Cursor())
	case NotApplicableCoverage:
		result["applicability_basis_ref"] = value.Basis().String()
	case UnavailableCoverage:
		result["missing_basis_ref"] = value.MissingBasis().String()
	case StaleCoverage:
		result["retry_basis_ref"] = value.RetryBasis().String()
	}
	return result
}

func encodeSnapshotCursor(cursor SnapshotCursor) map[string]any {
	return map[string]any{
		"digest":         cursor.Digest().String(),
		"graph_revision": cursor.GraphRevision().Value(),
		"type_env_ref":   cursor.TypeEnv().String(),
		"profile_ref":    cursor.ProfileRef().String(),
		"facet":          string(cursor.Facet()),
		"next_offset":    cursor.NextOffset(),
	}
}

func encodeNeighborhoodBoundaries(
	boundaries NeighborhoodBoundaries,
) map[string]any {
	crossContext := make(
		[]map[string]any,
		0,
		len(boundaries.CrossContextRefs()),
	)
	for _, value := range boundaries.CrossContextRefs() {
		bridge := map[string]any{
			"kind": string(value.Bridge().Kind()),
		}
		known, ok := value.Bridge().(KnownBridge)
		if ok {
			bridge["bridge_ref"] = known.Ref().String()
		}
		crossContext = append(crossContext, map[string]any{
			"source_context_ref": value.SourceContext().String(),
			"target_context_ref": value.TargetContext().String(),
			"bridge":             bridge,
		})
	}
	unresolved := make(
		[]string,
		0,
		len(boundaries.UnresolvedItems()),
	)
	for _, value := range boundaries.UnresolvedItems() {
		unresolved = append(unresolved, value.String())
	}
	omitted := make([]string, 0, len(boundaries.OmittedFacets()))
	for _, value := range boundaries.OmittedFacets() {
		omitted = append(omitted, string(value))
	}
	issues := make(
		[]map[string]any,
		0,
		len(boundaries.FacetBasisIssues()),
	)
	for _, value := range boundaries.FacetBasisIssues() {
		issues = append(issues, encodeFacetBasisIssue(value))
	}
	return map[string]any{
		"cross_context_refs": crossContext,
		"unresolved_items":   unresolved,
		"omitted_facets":     omitted,
		"facet_basis_issues": issues,
	}
}

func encodeFacetBasisIssue(issue FacetBasisIssue) map[string]any {
	result := map[string]any{
		"kind":  string(issue.Kind()),
		"facet": string(issue.Facet()),
	}
	switch value := issue.(type) {
	case MissingTypeBasisIssue:
		result["required_ref_or_kind"] = value.Required().String()
	case MissingCorrespondenceBasisIssue:
		result["required_correspondence"] = value.Required().String()
	case UnresolvedLegacyIdentityIssue:
		result["legacy_ref"] = value.LegacyRef().String()
		result["resolution_ref"] = value.ResolutionRef().String()
	case StaleDerivedProjectionIssue:
		result["projection_ref"] = value.ProjectionRef().String()
		result["observed_version"] = value.ObservedVersion().String()
		result["required_version"] = value.RequiredVersion().String()
	case ExplicitBridgeRequiredIssue:
		result["source_context_ref"] = value.SourceContext().String()
		result["target_context_ref"] = value.TargetContext().String()
		result["bridge"] = string(value.Bridge().Kind())
		known, ok := value.Bridge().(KnownBridge)
		if ok {
			result["known_bridge_ref"] = known.Ref().String()
		}
	}
	return result
}

func encodeInterpretation(
	contract InterpretationContract,
) map[string]any {
	return map[string]any{
		"structure":               string(contract.Structure()),
		"identity":                string(contract.Identity()),
		"relational_records":      string(contract.RelationalRecords()),
		"ranking":                 string(contract.Ranking()),
		"truth":                   string(contract.Truth()),
		"applicability":           string(contract.Applicability()),
		"authority":               string(contract.Authority()),
		"work_order":              string(contract.WorkOrder()),
		"completeness":            string(contract.Completeness()),
		"hydrate_before_reliance": contract.HydrateBeforeReliance(),
	}
}

func encodeReadAffordances(
	affordances []ReadAffordance,
) []map[string]any {
	result := make([]map[string]any, 0, len(affordances))
	for _, affordance := range affordances {
		current := map[string]any{
			"kind": string(affordance.Kind()),
		}
		switch value := affordance.(type) {
		case ExpandFacetAffordance:
			current["facet"] = string(value.Facet())
			current["snapshot_cursor"] = encodeSnapshotCursor(
				value.Cursor(),
			)
		case InspectEntityAffordance:
			current["entity_reference_kind"] = value.Entity().
				RefKind().
				String()
			current["entity_reference_id"] = value.Entity().
				ReferenceID().
				String()
			current["bounded_context_ref"] = value.Context().String()
		case HydrateCarrierAffordance:
			current["carrier_ref"] = value.Carrier().String()
			current["edition"] = value.Edition().String()
		case FollowContextBridgeAffordance:
			current["bridge_ref"] = value.Bridge().String()
			current["target_context_ref"] = value.TargetContext().String()
		}
		result = append(result, current)
	}
	return result
}

func encodeAppliedReadBudget(
	budget AppliedReadBudget,
) map[string]any {
	perFacet := make([]map[string]any, 0, len(budget.PerFacet()))
	for _, value := range budget.PerFacet() {
		perFacet = append(perFacet, map[string]any{
			"facet":                  string(value.Facet()),
			"included_items":         value.IncludedItems(),
			"omitted_items":          value.OmittedItems(),
			"profile_filtered_items": value.ProfileFilteredItems(),
		})
	}
	cursors := make(
		[]map[string]any,
		0,
		len(budget.ContinuationCursors()),
	)
	for _, cursor := range budget.ContinuationCursors() {
		cursors = append(cursors, encodeSnapshotCursor(cursor))
	}
	return map[string]any{
		"requested_limits": encodeReadBudget(
			budget.RequestedLimits(),
		),
		"applied_limits": encodeReadBudget(
			budget.AppliedLimits(),
		),
		"per_facet":                       perFacet,
		"emitted_relation_path_count":     budget.EmittedRelationPathCount(),
		"omitted_relation_path_count":     budget.OmittedRelationPathCount(),
		"emitted_excerpt_character_count": budget.EmittedExcerptCharacterCount(),
		"emitted_provenance_depth":        budget.EmittedProvenanceDepth(),
		"bounded_content_utf8_bytes":      budget.BoundedContentUTF8Bytes(),
		"continuation_cursors":            cursors,
	}
}

func encodeReadBudget(
	budget DimensionedReadBudget,
) map[string]any {
	return map[string]any{
		"max_facets":                     budget.MaxFacets(),
		"max_items_per_facet":            budget.MaxItemsPerFacet(),
		"max_relation_paths_per_item":    budget.MaxRelationPathsPerItem(),
		"max_carrier_excerpt_characters": budget.MaxCarrierExcerptCharacters(),
		"max_provenance_depth":           budget.MaxProvenanceDepth(),
	}
}
