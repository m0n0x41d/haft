package goldenconcernbundle

import (
	"fmt"
	"sort"

	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projectmemory/carrierfamily"
	"github.com/m0n0x41d/haft/internal/projectmemory/codeanchoradapter"
	"github.com/m0n0x41d/haft/internal/projectmemory/evidenceworkadapter"
	"github.com/m0n0x41d/haft/internal/projectmemory/recordatconcern"
	"github.com/m0n0x41d/haft/internal/recordmapping"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
)

func NewConcernAdmission(
	project projectidentity.ProjectID,
	declaration typedmemory.DeclareEntity,
	reference typedmemory.PersistedRef,
	receipt typedmemorystore.CommitReceipt,
) (ConcernAdmission, error) {
	parsedProject, err := projectidentity.ParseProjectID(project.String())
	if err != nil || parsedProject != project {
		return ConcernAdmission{}, fmt.Errorf(
			"GoldenConcernBundle concern project is invalid",
		)
	}
	if reference.RefKind().ID().String() != "U.EntityRef" {
		return ConcernAdmission{}, fmt.Errorf(
			"GoldenConcernBundle concern must use U.EntityRef",
		)
	}
	if reference.ReferenceID().String() != declaration.Entity().String() {
		return ConcernAdmission{}, fmt.Errorf(
			"GoldenConcernBundle concern reference differs from its EntityID",
		)
	}
	changeSet, err := typedmemory.NewMemoryChangeSet(
		[]typedmemory.MemoryChange{declaration},
	)
	if err != nil {
		return ConcernAdmission{}, fmt.Errorf(
			"GoldenConcernBundle concern declaration: %w",
			err,
		)
	}
	candidateDigest, err := changeSet.Digest()
	if err != nil {
		return ConcernAdmission{}, err
	}
	canonical, err := changeSet.CanonicalBytes()
	if err != nil {
		return ConcernAdmission{}, err
	}
	receiptValue, err := sealReceipt(receipt)
	if err != nil {
		return ConcernAdmission{}, err
	}
	return ConcernAdmission{
		project: project,
		declaration: DeclarationWitness{
			entity:     declaration.Entity(),
			localRef:   declaration.LocalRef(),
			context:    declaration.Context(),
			label:      declaration.Label(),
			provenance: declaration.Provenance(),
		},
		reference:        reference,
		candidateDigest:  candidateDigest,
		receipt:          receiptValue,
		canonicalChanges: canonical,
	}, nil
}

func NewRecordAdapterAdmission(
	project projectidentity.ProjectID,
	candidate recordatconcern.ValidCandidate,
	receipt typedmemorystore.CommitReceipt,
) (AdapterAdmission, error) {
	if candidate == nil {
		return AdapterAdmission{}, fmt.Errorf(
			"GoldenConcernBundle record-adapter candidate is required",
		)
	}
	binding := candidate.CarrierBinding()
	if binding.ProjectID() != project {
		return AdapterAdmission{}, fmt.Errorf(
			"GoldenConcernBundle record candidate belongs to another project",
		)
	}
	signatures := []typedmemory.SignatureID{
		candidate.RelationDeclarationFragmentID(),
	}
	return sealAdapterAdmission(
		project,
		candidate.ChangeSet(),
		candidate.MappingManifestRef(),
		candidate.AdapterVersion(),
		signatures,
		receipt,
	)
}

func NewCodeAnchorAdapterAdmission(
	project projectidentity.ProjectID,
	candidate codeanchoradapter.ValidCandidate,
	receipt typedmemorystore.CommitReceipt,
) (AdapterAdmission, error) {
	if candidate == nil {
		return AdapterAdmission{}, fmt.Errorf(
			"GoldenConcernBundle CodeAnchor candidate is required",
		)
	}
	binding := candidate.CarrierBinding()
	if binding.ProjectID() != project {
		return AdapterAdmission{}, fmt.Errorf(
			"GoldenConcernBundle CodeAnchor candidate belongs to another project",
		)
	}
	return sealAdapterAdmission(
		project,
		candidate.ChangeSet(),
		candidate.MappingManifestRef(),
		candidate.AdapterVersion(),
		candidate.RelationDeclarationFragmentIDs(),
		receipt,
	)
}

func NewEvidenceWorkAdapterAdmission(
	project projectidentity.ProjectID,
	candidate evidenceworkadapter.ValidCandidate,
	receipt typedmemorystore.CommitReceipt,
) (AdapterAdmission, error) {
	if candidate == nil {
		return AdapterAdmission{}, fmt.Errorf(
			"GoldenConcernBundle Evidence/Work candidate is required",
		)
	}
	for _, source := range candidate.RecordMembershipSources() {
		if source.ProjectID() != project {
			return AdapterAdmission{}, fmt.Errorf(
				"GoldenConcernBundle Evidence/Work record source belongs to another project",
			)
		}
	}
	occurrence := candidate.OccurrenceMembershipSource()
	if occurrence.ProjectID() != project {
		return AdapterAdmission{}, fmt.Errorf(
			"GoldenConcernBundle Work occurrence source belongs to another project",
		)
	}
	return sealAdapterAdmission(
		project,
		candidate.ChangeSet(),
		candidate.MappingManifestRef(),
		candidate.AdapterVersion(),
		candidate.RelationDeclarationFragmentIDs(),
		receipt,
	)
}

// NewGovernedProjectClaimAdmission seals the one foundation relation that
// existing task adapters consume but do not yet author: a source-classified
// ProjectClaim related to the concern and explicitly stated by a project
// record. It still requires canonical AdmissionService evidence and the sealed
// ProjectClaim carrier-family source. No arbitrary signature set is accepted.
func NewGovernedProjectClaimAdmission(
	project projectidentity.ProjectID,
	changeSet typedmemory.MemoryChangeSet,
	source carrierfamily.MembershipSourceV1,
	receipt typedmemorystore.CommitReceipt,
) (AdapterAdmission, error) {
	if source.ProjectID() != project {
		return AdapterAdmission{}, fmt.Errorf(
			"GoldenConcernBundle ProjectClaim source belongs to another project",
		)
	}
	if source.EvaluatorRule() != carrierfamily.ProjectClaimEvaluatorRuleV1() {
		return AdapterAdmission{}, fmt.Errorf(
			"GoldenConcernBundle claim source is not a ProjectClaim source",
		)
	}
	binding := source.Binding()
	signatures, err := exactSignatureIDs(
		"Haft.ProjectClaimAtConcern",
		"Haft.RecordStatesClaim",
	)
	if err != nil {
		return AdapterAdmission{}, err
	}
	admission, err := sealAdapterAdmission(
		project,
		changeSet,
		binding.MappingManifestRef(),
		binding.AdapterVersion(),
		signatures,
		receipt,
	)
	if err != nil {
		return AdapterAdmission{}, err
	}
	if len(admission.declarations) != 1 ||
		admission.declarations[0].entity != source.EntityID() ||
		admission.declarations[0].context != source.BoundedContext() {
		return AdapterAdmission{}, fmt.Errorf(
			"GoldenConcernBundle ProjectClaim declaration differs from its sealed source",
		)
	}
	return admission, nil
}

func sealAdapterAdmission(
	project projectidentity.ProjectID,
	changeSet typedmemory.MemoryChangeSet,
	manifest recordmapping.MappingManifestRef,
	adapter recordmapping.AdapterVersion,
	expectedSignatures []typedmemory.SignatureID,
	receipt typedmemorystore.CommitReceipt,
) (AdapterAdmission, error) {
	parsedProject, err := projectidentity.ParseProjectID(project.String())
	if err != nil || parsedProject != project {
		return AdapterAdmission{}, fmt.Errorf(
			"GoldenConcernBundle adapter project is invalid",
		)
	}
	if err := manifest.Verify(); err != nil {
		return AdapterAdmission{}, fmt.Errorf(
			"GoldenConcernBundle mapping manifest: %w",
			err,
		)
	}
	if err := adapter.Verify(); err != nil {
		return AdapterAdmission{}, fmt.Errorf(
			"GoldenConcernBundle adapter version: %w",
			err,
		)
	}
	receiptValue, err := sealReceipt(receipt)
	if err != nil {
		return AdapterAdmission{}, err
	}
	candidateDigest, err := changeSet.Digest()
	if err != nil {
		return AdapterAdmission{}, fmt.Errorf(
			"GoldenConcernBundle candidate digest: %w",
			err,
		)
	}
	canonical, err := changeSet.CanonicalBytes()
	if err != nil {
		return AdapterAdmission{}, fmt.Errorf(
			"GoldenConcernBundle candidate bytes: %w",
			err,
		)
	}
	extracted, err := extractCandidateWitnesses(
		changeSet,
		receiptValue.eventRef,
	)
	if err != nil {
		return AdapterAdmission{}, err
	}
	expected, err := normalizeSignatureIDs(expectedSignatures)
	if err != nil {
		return AdapterAdmission{}, err
	}
	if !sameSignatureIDs(expected, extracted.signatures) {
		return AdapterAdmission{}, fmt.Errorf(
			"GoldenConcernBundle adapter signature witness differs from candidate changes",
		)
	}
	return AdapterAdmission{
		project:          project,
		manifest:         manifest,
		adapter:          adapter,
		candidateDigest:  candidateDigest,
		receipt:          receiptValue,
		signatures:       expected,
		declarations:     extracted.declarations,
		paths:            extracted.paths,
		values:           extracted.values,
		canonicalChanges: canonical,
	}, nil
}

func sealReceipt(
	receipt typedmemorystore.CommitReceipt,
) (receiptWitness, error) {
	switch receipt.Disposition() {
	case typedmemorystore.CommitApplied,
		typedmemorystore.CommitReplay,
		typedmemorystore.CommitRecovered:
	default:
		return receiptWitness{}, fmt.Errorf(
			"GoldenConcernBundle requires an AdmissionService commit receipt",
		)
	}
	eventRef, err := exactOneLine(
		"GoldenConcernBundle event reference",
		receipt.EventRef(),
	)
	if err != nil {
		return receiptWitness{}, err
	}
	commitRef, err := exactOneLine(
		"GoldenConcernBundle commit reference",
		receipt.CommitRef(),
	)
	if err != nil {
		return receiptWitness{}, err
	}
	if receipt.GraphRevision().Value() == 0 {
		return receiptWitness{}, fmt.Errorf(
			"GoldenConcernBundle receipt graph revision is missing",
		)
	}
	digest, err := typedmemory.NewSHA256Digest(
		receipt.ResultDigest().String(),
	)
	if err != nil || digest != receipt.ResultDigest() {
		return receiptWitness{}, fmt.Errorf(
			"GoldenConcernBundle receipt result digest is invalid",
		)
	}
	return receiptWitness{
		disposition: receipt.Disposition(),
		eventRef:    eventRef,
		commitRef:   commitRef,
		revision:    receipt.GraphRevision(),
		result:      receipt.ResultDigest(),
	}, nil
}

type extractedCandidateWitnesses struct {
	signatures   []typedmemory.SignatureID
	declarations []DeclarationWitness
	paths        []RelationPath
	values       []ValueWitness
}

func extractCandidateWitnesses(
	changeSet typedmemory.MemoryChangeSet,
	eventRef string,
) (extractedCandidateWitnesses, error) {
	declarations := make([]DeclarationWitness, 0)
	localEntities := make(map[string]typedmemory.EntityID)
	for _, change := range changeSet.Changes() {
		declaration, ok := change.(typedmemory.DeclareEntity)
		if !ok {
			continue
		}
		local := declaration.LocalRef().String()
		localEntities[local] = declaration.Entity()
		declarations = append(declarations, DeclarationWitness{
			entity:     declaration.Entity(),
			localRef:   declaration.LocalRef(),
			context:    declaration.Context(),
			label:      declaration.Label(),
			provenance: declaration.Provenance(),
		})
	}
	if len(declarations) == 0 {
		return extractedCandidateWitnesses{}, fmt.Errorf(
			"GoldenConcernBundle adapter candidate declares no entity",
		)
	}
	sort.Slice(declarations, func(left int, right int) bool {
		return declarations[left].entity.String() <
			declarations[right].entity.String()
	})
	signatures := make([]typedmemory.SignatureID, 0)
	paths := make([]RelationPath, 0)
	values := make([]ValueWitness, 0)
	for _, change := range changeSet.Changes() {
		relationChange, ok := change.(typedmemory.AssertRelation)
		if !ok {
			continue
		}
		relation := relationChange.Assertion()
		if relation.Modality().Kind() != typedmemory.AssertionModalityAffirmsObtaining {
			return extractedCandidateWitnesses{}, fmt.Errorf(
				"GoldenConcernBundle adapter relation %s does not affirm obtaining",
				relation.Assertion().String(),
			)
		}
		signature := relation.Signature().ID()
		signatures = append(signatures, signature)
		for _, binding := range relation.Bindings() {
			for _, filler := range binding.Fillers() {
				switch value := filler.(type) {
				case typedmemory.ByReferenceCandidate:
					target, resolveErr := resolveCandidateReference(
						value.Reference(),
						localEntities,
					)
					if resolveErr != nil {
						return extractedCandidateWitnesses{}, resolveErr
					}
					paths = append(paths, RelationPath{
						assertion:         relation.Assertion(),
						signature:         signature,
						context:           relation.Context(),
						slot:              binding.Name(),
						target:            target,
						provenance:        relation.Provenance(),
						admissionEventRef: eventRef,
					})
				case typedmemory.ByValueCandidate:
					inputDigest, digestErr := digestBytes(
						value.Value().InputBytes(),
					)
					if digestErr != nil {
						return extractedCandidateWitnesses{}, digestErr
					}
					values = append(values, ValueWitness{
						assertion:         relation.Assertion(),
						signature:         signature,
						slot:              binding.Name(),
						valueKind:         value.Value().ValueKind(),
						valueShape:        value.Value().ValueShape(),
						codec:             value.Value().Codec(),
						inputDigest:       inputDigest,
						admissionEventRef: eventRef,
					})
				default:
					return extractedCandidateWitnesses{}, fmt.Errorf(
						"GoldenConcernBundle candidate contains an unknown slot-filler variant",
					)
				}
			}
		}
	}
	if len(signatures) == 0 {
		return extractedCandidateWitnesses{}, fmt.Errorf(
			"GoldenConcernBundle adapter candidate instantiates no relation",
		)
	}
	normalizedSignatures, err := normalizeSignatureIDs(signatures)
	if err != nil {
		return extractedCandidateWitnesses{}, err
	}
	sort.Slice(paths, func(left int, right int) bool {
		return relationPathKey(paths[left]) < relationPathKey(paths[right])
	})
	sort.Slice(values, func(left int, right int) bool {
		return valueWitnessKey(values[left]) < valueWitnessKey(values[right])
	})
	return extractedCandidateWitnesses{
		signatures:   normalizedSignatures,
		declarations: declarations,
		paths:        paths,
		values:       values,
	}, nil
}

func resolveCandidateReference(
	reference typedmemory.StrongRef,
	localEntities map[string]typedmemory.EntityID,
) (typedmemory.PersistedRef, error) {
	switch value := reference.(type) {
	case typedmemory.PersistedRef:
		return value, nil
	case typedmemory.LocalRef:
		entity, found := localEntities[value.BatchLocalRef().String()]
		if !found {
			return typedmemory.PersistedRef{}, fmt.Errorf(
				"GoldenConcernBundle relation uses an undeclared local reference %q",
				value.BatchLocalRef().String(),
			)
		}
		referenceID, err := typedmemory.NewReferenceID(entity.String())
		if err != nil {
			return typedmemory.PersistedRef{}, err
		}
		return typedmemory.NewPersistedRef(value.RefKind(), referenceID)
	default:
		return typedmemory.PersistedRef{}, fmt.Errorf(
			"GoldenConcernBundle relation uses an unsupported strong-reference variant",
		)
	}
}

func normalizeSignatureIDs(
	values []typedmemory.SignatureID,
) ([]typedmemory.SignatureID, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf(
			"GoldenConcernBundle adapter signature set is empty",
		)
	}
	seen := make(map[string]struct{})
	result := make([]typedmemory.SignatureID, 0, len(values))
	for _, value := range values {
		parsed, err := typedmemory.NewSignatureID(value.String())
		if err != nil || parsed != value {
			return nil, fmt.Errorf(
				"GoldenConcernBundle adapter signature is invalid",
			)
		}
		if _, found := seen[value.String()]; found {
			continue
		}
		seen[value.String()] = struct{}{}
		result = append(result, value)
	}
	sort.Slice(result, func(left int, right int) bool {
		return result[left].String() < result[right].String()
	})
	return result, nil
}

func sameSignatureIDs(
	left []typedmemory.SignatureID,
	right []typedmemory.SignatureID,
) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func exactSignatureIDs(values ...string) ([]typedmemory.SignatureID, error) {
	result := make([]typedmemory.SignatureID, 0, len(values))
	for _, raw := range values {
		value, err := typedmemory.NewSignatureID(raw)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return normalizeSignatureIDs(result)
}
