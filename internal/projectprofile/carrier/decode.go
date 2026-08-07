package carrier

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/projectprofile"
	"gopkg.in/yaml.v3"
)

// SchemaVersion identifies the legacy, read-only project-profile carrier.
const SchemaVersion = "haft.project-profile/v1"

// Decode parses a legacy project-profile carrier as compatibility input.
// The returned profile carries no canonical admission or binding authority.
func Decode(source []byte) (projectprofile.ConfiguredProjectProfile, error) {
	root, err := decodeSingleDocument(source)
	if err != nil {
		return nil, err
	}
	if err := rejectUnsafeYAML(root); err != nil {
		return nil, err
	}
	fields, err := mappingFields(
		root,
		"project profile carrier",
		[]string{"schema_version", "profile"},
		[]string{},
	)
	if err != nil {
		return nil, err
	}
	version, err := stringScalar(fields["schema_version"], "schema_version")
	if err != nil {
		return nil, err
	}
	if version != SchemaVersion {
		return nil, fmt.Errorf("unsupported project profile schema_version %q; want %q", version, SchemaVersion)
	}
	return decodeProfile(fields["profile"])
}

func decodeSingleDocument(source []byte) (*yaml.Node, error) {
	if len(bytes.TrimSpace(source)) == 0 {
		return nil, fmt.Errorf("project profile carrier is empty")
	}
	decoder := yaml.NewDecoder(bytes.NewReader(source))
	var document yaml.Node
	if err := decoder.Decode(&document); err != nil {
		return nil, fmt.Errorf("decode project profile YAML: %w", err)
	}
	var trailing yaml.Node
	err := decoder.Decode(&trailing)
	if err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("project profile carrier must contain exactly one YAML document")
		}
		return nil, fmt.Errorf("decode trailing project profile YAML: %w", err)
	}
	if len(document.Content) != 1 {
		return nil, fmt.Errorf("project profile carrier has no document root")
	}
	return document.Content[0], nil
}

func decodeProfile(node *yaml.Node) (projectprofile.ConfiguredProjectProfile, error) {
	initial, err := mappingFields(
		node,
		"profile",
		[]string{"kind"},
		[]string{"scopes", "declaration_record"},
	)
	if err != nil {
		return nil, err
	}
	kind, err := stringScalar(initial["kind"], "profile.kind")
	if err != nil {
		return nil, err
	}
	switch kind {
	case "auto":
		_, exactErr := mappingFields(node, "auto profile", []string{"kind"}, []string{})
		if exactErr != nil {
			return nil, exactErr
		}
		return projectprofile.Auto{}, nil
	case "declared":
		return decodeDeclared(node)
	default:
		return nil, fmt.Errorf("unknown profile.kind %q", kind)
	}
}

func decodeDeclared(node *yaml.Node) (projectprofile.ConfiguredProjectProfile, error) {
	fields, err := mappingFields(
		node,
		"declared profile",
		[]string{"kind", "scopes", "declaration_record"},
		[]string{},
	)
	if err != nil {
		return nil, err
	}
	scopeNodes, err := sequenceItems(fields["scopes"], "profile.scopes")
	if err != nil {
		return nil, err
	}
	scopes, err := decodeScopes(scopeNodes)
	if err != nil {
		return nil, err
	}
	declaration, err := decodeProfileDeclarationReceipt(fields["declaration_record"])
	if err != nil {
		return nil, err
	}
	profile, err := projectprofile.NewDeclared(scopes, declaration)
	if err != nil {
		return nil, fmt.Errorf("construct declared profile: %w", err)
	}
	return profile, nil
}

func decodeScopes(nodes []*yaml.Node) (projectprofile.ScopeSet, error) {
	values := make([]projectprofile.RealizationScope, 0, len(nodes))
	for index, node := range nodes {
		value, err := decodeScope(node, index)
		if err != nil {
			return projectprofile.ScopeSet{}, err
		}
		values = append(values, value)
	}
	scopes, err := projectprofile.NewScopeSet(values)
	if err != nil {
		return projectprofile.ScopeSet{}, fmt.Errorf("profile.scopes: %w", err)
	}
	return scopes, nil
}

func decodeScope(node *yaml.Node, index int) (projectprofile.RealizationScope, error) {
	context := fmt.Sprintf("profile.scopes[%d]", index)
	initial, err := mappingFields(
		node,
		context,
		[]string{"kind", "scope_id"},
		[]string{"entity_ref", "admitted_kind_ref", "governing_pattern_refs", "contract_refs"},
	)
	if err != nil {
		return nil, err
	}
	kind, err := stringScalar(initial["kind"], context+".kind")
	if err != nil {
		return nil, err
	}
	switch kind {
	case "software":
		return decodeSoftwareScope(node, context)
	case "non_software":
		return decodeNonSoftwareScope(node, context)
	default:
		return nil, fmt.Errorf("%s has unknown kind %q", context, kind)
	}
}

func decodeSoftwareScope(node *yaml.Node, context string) (projectprofile.RealizationScope, error) {
	fields, err := mappingFields(node, context, []string{"kind", "scope_id"}, []string{"entity_ref"})
	if err != nil {
		return nil, err
	}
	scopeID, err := decodeScopeID(fields["scope_id"], context+".scope_id")
	if err != nil {
		return nil, err
	}
	entityReference, err := decodeEntityReference(fields, context)
	if err != nil {
		return nil, err
	}
	scope, err := projectprofile.NewSoftwareRealization(scopeID, entityReference)
	if err != nil {
		return nil, fmt.Errorf("construct %s: %w", context, err)
	}
	return scope, nil
}

func decodeNonSoftwareScope(node *yaml.Node, context string) (projectprofile.RealizationScope, error) {
	fields, err := mappingFields(
		node,
		context,
		[]string{"kind", "scope_id"},
		[]string{"entity_ref", "admitted_kind_ref", "governing_pattern_refs", "contract_refs"},
	)
	if err != nil {
		return nil, err
	}
	scopeID, err := decodeScopeID(fields["scope_id"], context+".scope_id")
	if err != nil {
		return nil, err
	}
	entityReference, err := decodeEntityReference(fields, context)
	if err != nil {
		return nil, err
	}
	kindOrientation, err := decodeLegacyKindOrientation(fields, context)
	if err != nil {
		return nil, err
	}
	patternRefs, err := decodeSourceUnitRefs(fields, context)
	if err != nil {
		return nil, err
	}
	contractRefs, err := decodeSpecSectionRefs(fields, context)
	if err != nil {
		return nil, err
	}
	scope, err := projectprofile.NewNonSoftwareRealization(
		scopeID,
		entityReference,
		kindOrientation,
		patternRefs,
		contractRefs,
	)
	if err != nil {
		return nil, fmt.Errorf("construct %s: %w", context, err)
	}
	return scope, nil
}

func decodeScopeID(node *yaml.Node, context string) (projectprofile.ScopeID, error) {
	raw, err := stringScalar(node, context)
	if err != nil {
		return projectprofile.ScopeID{}, err
	}
	value, err := projectprofile.NewScopeID(raw)
	if err != nil {
		return projectprofile.ScopeID{}, fmt.Errorf("%s: %w", context, err)
	}
	return value, nil
}

func decodeEntityReference(
	fields map[string]*yaml.Node,
	context string,
) (projectprofile.EntityReference, error) {
	node, exists := fields["entity_ref"]
	if !exists {
		return projectprofile.NoEntityReference{}, nil
	}
	raw, err := stringScalar(node, context+".entity_ref")
	if err != nil {
		return nil, err
	}
	ref, err := projectprofile.NewEntityRef(raw)
	if err != nil {
		return nil, fmt.Errorf("%s.entity_ref: %w", context, err)
	}
	return projectprofile.NewReferencedEntity(ref), nil
}

func decodeLegacyKindOrientation(
	fields map[string]*yaml.Node,
	context string,
) (projectprofile.KindOrientation, error) {
	node, exists := fields["admitted_kind_ref"]
	if !exists {
		return projectprofile.UnspecifiedKindOrientation{}, nil
	}
	raw, err := stringScalar(node, context+".admitted_kind_ref")
	if err != nil {
		return nil, err
	}
	ref, err := projectprofile.NewKindRef(raw)
	if err != nil {
		return nil, fmt.Errorf("%s.admitted_kind_ref: %w", context, err)
	}
	return projectprofile.NewReferencedKindOrientation(ref), nil
}

func decodeSourceUnitRefs(
	fields map[string]*yaml.Node,
	context string,
) ([]projectprofile.SourceUnitRef, error) {
	node, exists := fields["governing_pattern_refs"]
	if !exists {
		return []projectprofile.SourceUnitRef{}, nil
	}
	values, err := decodeStringSequence(node, context+".governing_pattern_refs")
	if err != nil {
		return nil, err
	}
	result := make([]projectprofile.SourceUnitRef, 0, len(values))
	for index, value := range values {
		ref, refErr := projectprofile.NewSourceUnitRef(value)
		if refErr != nil {
			return nil, fmt.Errorf("%s.governing_pattern_refs[%d]: %w", context, index, refErr)
		}
		result = append(result, ref)
	}
	return result, nil
}

func decodeSpecSectionRefs(
	fields map[string]*yaml.Node,
	context string,
) ([]projectprofile.SpecSectionRef, error) {
	node, exists := fields["contract_refs"]
	if !exists {
		return []projectprofile.SpecSectionRef{}, nil
	}
	values, err := decodeStringSequence(node, context+".contract_refs")
	if err != nil {
		return nil, err
	}
	result := make([]projectprofile.SpecSectionRef, 0, len(values))
	for index, value := range values {
		ref, refErr := projectprofile.NewSpecSectionRef(value)
		if refErr != nil {
			return nil, fmt.Errorf("%s.contract_refs[%d]: %w", context, index, refErr)
		}
		result = append(result, ref)
	}
	return result, nil
}

func decodeProfileDeclarationReceipt(node *yaml.Node) (projectprofile.ProfileDeclarationReceipt, error) {
	initial, err := mappingFields(
		node,
		"profile.declaration_record",
		[]string{"kind"},
		[]string{
			"declaration_authority_basis_ref",
			"declaration_work_ref",
			"candidate_provenance_digest",
			"admission_event_ref",
			"project_root",
			"scope_payload_digest",
			"observed_basis_digest",
			"observation_window",
			"carrier_revision",
			"source_ref",
			"source_digest",
		},
	)
	if err != nil {
		return nil, err
	}
	kind, err := stringScalar(initial["kind"], "profile.declaration_record.kind")
	if err != nil {
		return nil, err
	}
	switch kind {
	case "operator_declared_record":
		return decodeOperatorDeclaredRecord(node)
	case "onboarding_agent_declared_record":
		return decodeOnboardingAgentDeclaredRecord(node)
	case "imported_declaration_record":
		return decodeImportedDeclarationRecord(node)
	default:
		return nil, fmt.Errorf("unknown profile.declaration_record.kind %q", kind)
	}
}

func decodeOperatorDeclaredRecord(node *yaml.Node) (projectprofile.ProfileDeclarationReceipt, error) {
	fields, err := mappingFields(
		node,
		"operator declaration record",
		[]string{
			"kind",
			"declaration_authority_basis_ref",
			"declaration_work_ref",
			"project_root",
			"scope_payload_digest",
			"observed_basis_digest",
			"observation_window",
			"carrier_revision",
		},
		[]string{},
	)
	if err != nil {
		return nil, err
	}
	authorityBasisRef, err := stringScalar(
		fields["declaration_authority_basis_ref"],
		"profile.declaration_record.declaration_authority_basis_ref",
	)
	if err != nil {
		return nil, err
	}
	workRef, err := stringScalar(
		fields["declaration_work_ref"],
		"profile.declaration_record.declaration_work_ref",
	)
	if err != nil {
		return nil, err
	}
	projectRoot, err := projectRootScalar(
		fields["project_root"],
		"profile.declaration_record.project_root",
	)
	if err != nil {
		return nil, err
	}
	scopeDigest, err := contentDigestScalar(
		fields["scope_payload_digest"],
		"profile.declaration_record.scope_payload_digest",
	)
	if err != nil {
		return nil, err
	}
	basisDigest, err := contentDigestScalar(
		fields["observed_basis_digest"],
		"profile.declaration_record.observed_basis_digest",
	)
	if err != nil {
		return nil, err
	}
	window, err := decodeObservationWindow(fields["observation_window"])
	if err != nil {
		return nil, err
	}
	revision, err := decodeCarrierRevision(
		fields["carrier_revision"],
		"profile.declaration_record.carrier_revision",
	)
	if err != nil {
		return nil, err
	}
	record, err := projectprofile.NewOperatorDeclaredRecordBuilder(authorityBasisRef, workRef).
		ForProject(projectRoot).
		ForScopePayload(scopeDigest).
		ForObservedBasis(basisDigest).
		ObservedWithin(window).
		AtCarrierRevision(revision).
		Build()
	if err != nil {
		return nil, fmt.Errorf("construct operator declaration record: %w", err)
	}
	return record, nil
}

func decodeOnboardingAgentDeclaredRecord(node *yaml.Node) (projectprofile.ProfileDeclarationReceipt, error) {
	fields, err := mappingFields(
		node,
		"onboarding-agent declaration record",
		[]string{
			"kind",
			"declaration_authority_basis_ref",
			"candidate_provenance_digest",
			"admission_event_ref",
			"project_root",
			"scope_payload_digest",
			"observed_basis_digest",
			"observation_window",
			"carrier_revision",
		},
		[]string{},
	)
	if err != nil {
		return nil, err
	}
	authorityBasisRef, err := stringScalar(
		fields["declaration_authority_basis_ref"],
		"profile.declaration_record.declaration_authority_basis_ref",
	)
	if err != nil {
		return nil, err
	}
	candidateDigest, err := contentDigestScalar(
		fields["candidate_provenance_digest"],
		"profile.declaration_record.candidate_provenance_digest",
	)
	if err != nil {
		return nil, err
	}
	admissionEventRef, err := stringScalar(
		fields["admission_event_ref"],
		"profile.declaration_record.admission_event_ref",
	)
	if err != nil {
		return nil, err
	}
	projectRoot, err := projectRootScalar(
		fields["project_root"],
		"profile.declaration_record.project_root",
	)
	if err != nil {
		return nil, err
	}
	scopeDigest, err := contentDigestScalar(
		fields["scope_payload_digest"],
		"profile.declaration_record.scope_payload_digest",
	)
	if err != nil {
		return nil, err
	}
	basisDigest, err := contentDigestScalar(
		fields["observed_basis_digest"],
		"profile.declaration_record.observed_basis_digest",
	)
	if err != nil {
		return nil, err
	}
	window, err := decodeObservationWindow(fields["observation_window"])
	if err != nil {
		return nil, err
	}
	revision, err := decodeCarrierRevision(
		fields["carrier_revision"],
		"profile.declaration_record.carrier_revision",
	)
	if err != nil {
		return nil, err
	}
	record, err := projectprofile.NewOnboardingAgentDeclaredRecordBuilder(
		authorityBasisRef,
		candidateDigest,
		admissionEventRef,
	).
		ForProject(projectRoot).
		ForScopePayload(scopeDigest).
		ForObservedBasis(basisDigest).
		ObservedWithin(window).
		AtCarrierRevision(revision).
		Build()
	if err != nil {
		return nil, fmt.Errorf("construct onboarding-agent declaration record: %w", err)
	}
	return record, nil
}

func decodeImportedDeclarationRecord(node *yaml.Node) (projectprofile.ProfileDeclarationReceipt, error) {
	fields, err := mappingFields(
		node,
		"imported declaration record",
		[]string{
			"kind",
			"source_ref",
			"source_digest",
			"declaration_authority_basis_ref",
			"scope_payload_digest",
			"observed_basis_digest",
			"carrier_revision",
		},
		[]string{},
	)
	if err != nil {
		return nil, err
	}
	sourceRef, err := stringScalar(fields["source_ref"], "profile.declaration_record.source_ref")
	if err != nil {
		return nil, err
	}
	sourceDigest, err := contentDigestScalar(
		fields["source_digest"],
		"profile.declaration_record.source_digest",
	)
	if err != nil {
		return nil, err
	}
	authorityBasisRef, err := stringScalar(
		fields["declaration_authority_basis_ref"],
		"profile.declaration_record.declaration_authority_basis_ref",
	)
	if err != nil {
		return nil, err
	}
	scopeDigest, err := contentDigestScalar(
		fields["scope_payload_digest"],
		"profile.declaration_record.scope_payload_digest",
	)
	if err != nil {
		return nil, err
	}
	basisDigest, err := contentDigestScalar(
		fields["observed_basis_digest"],
		"profile.declaration_record.observed_basis_digest",
	)
	if err != nil {
		return nil, err
	}
	revision, err := decodeCarrierRevision(
		fields["carrier_revision"],
		"profile.declaration_record.carrier_revision",
	)
	if err != nil {
		return nil, err
	}
	record, err := projectprofile.NewImportedDeclarationRecordBuilder(
		sourceRef,
		sourceDigest,
		authorityBasisRef,
	).
		ForScopePayload(scopeDigest).
		ForObservedBasis(basisDigest).
		AtCarrierRevision(revision).
		Build()
	if err != nil {
		return nil, fmt.Errorf("construct imported declaration record: %w", err)
	}
	return record, nil
}

func decodeObservationWindow(node *yaml.Node) (projectprofile.ObservationWindow, error) {
	fields, err := mappingFields(
		node,
		"profile.declaration_record.observation_window",
		[]string{"execution_context_ref", "from", "until"},
		[]string{},
	)
	if err != nil {
		return projectprofile.ObservationWindow{}, err
	}
	contextRef, err := stringScalar(
		fields["execution_context_ref"],
		"profile.declaration_record.observation_window.execution_context_ref",
	)
	if err != nil {
		return projectprofile.ObservationWindow{}, err
	}
	from, err := timeScalar(fields["from"], "profile.declaration_record.observation_window.from")
	if err != nil {
		return projectprofile.ObservationWindow{}, err
	}
	until, err := timeScalar(fields["until"], "profile.declaration_record.observation_window.until")
	if err != nil {
		return projectprofile.ObservationWindow{}, err
	}
	window, err := projectprofile.NewObservationWindow(contextRef, from, until)
	if err != nil {
		return projectprofile.ObservationWindow{}, fmt.Errorf("construct observation window: %w", err)
	}
	return window, nil
}

func decodeCarrierRevision(node *yaml.Node, context string) (projectprofile.CarrierRevision, error) {
	if node.Kind != yaml.ScalarNode || node.Tag != "!!int" {
		return projectprofile.CarrierRevision{}, fmt.Errorf("%s must be a canonical positive integer", context)
	}
	if strings.HasPrefix(node.Value, "0") || strings.HasPrefix(node.Value, "+") || strings.HasPrefix(node.Value, "-") {
		return projectprofile.CarrierRevision{}, fmt.Errorf("%s must be a canonical positive integer", context)
	}
	value, err := strconv.ParseUint(node.Value, 10, 64)
	if err != nil {
		return projectprofile.CarrierRevision{}, fmt.Errorf("parse %s: %w", context, err)
	}
	revision, err := projectprofile.NewCarrierRevision(value)
	if err != nil {
		return projectprofile.CarrierRevision{}, fmt.Errorf("%s: %w", context, err)
	}
	return revision, nil
}

func contentDigestScalar(node *yaml.Node, context string) (projectprofile.ContentDigest, error) {
	raw, err := stringScalar(node, context)
	if err != nil {
		return projectprofile.ContentDigest{}, err
	}
	digest, err := projectprofile.NewContentDigest(raw)
	if err != nil {
		return projectprofile.ContentDigest{}, fmt.Errorf("%s: %w", context, err)
	}
	return digest, nil
}

func projectRootScalar(node *yaml.Node, context string) (string, error) {
	raw, err := stringScalar(node, context)
	if err != nil {
		return "", err
	}
	root, err := canonicalProjectRoot(raw)
	if err != nil {
		return "", fmt.Errorf("%s: %w", context, err)
	}
	return root, nil
}

func timeScalar(node *yaml.Node, context string) (time.Time, error) {
	raw, err := stringScalar(node, context)
	if err != nil {
		return time.Time{}, err
	}
	value, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s must be canonical RFC3339: %w", context, err)
	}
	if value.Format(time.RFC3339Nano) != raw {
		return time.Time{}, fmt.Errorf("%s must use canonical RFC3339 representation", context)
	}
	return value, nil
}

func decodeStringSequence(node *yaml.Node, context string) ([]string, error) {
	items, err := sequenceItems(node, context)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(items))
	for index, item := range items {
		value, valueErr := stringScalar(item, fmt.Sprintf("%s[%d]", context, index))
		if valueErr != nil {
			return nil, valueErr
		}
		result = append(result, value)
	}
	return result, nil
}
