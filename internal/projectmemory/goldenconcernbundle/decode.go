package goldenconcernbundle

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/recordmapping"
	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
)

// DecodeCanonical reconstructs one read-only GoldenConcernBundle proof from
// its exact v1 export. It does not admit, update, select, or infer graph state.
func DecodeCanonical(canonical []byte) (Bundle, error) {
	encoded := bundleCanonicalV1{}
	if err := decodeCanonicalJSON(canonical, &encoded); err != nil {
		return Bundle{}, fmt.Errorf(
			"decode GoldenConcernBundle: %w",
			err,
		)
	}
	if encoded.Schema != SchemaV1 {
		return Bundle{}, fmt.Errorf(
			"unsupported GoldenConcernBundle schema %q",
			encoded.Schema,
		)
	}
	expected, err := json.Marshal(encoded)
	if err != nil {
		return Bundle{}, fmt.Errorf(
			"re-encode GoldenConcernBundle: %w",
			err,
		)
	}
	if !bytes.Equal(canonical, expected) {
		return Bundle{}, fmt.Errorf(
			"GoldenConcernBundle export is not canonical",
		)
	}
	if !sameStrings(
		encoded.InterpretationBoundary,
		interpretationBoundaryV1(),
	) {
		return Bundle{}, fmt.Errorf(
			"GoldenConcernBundle interpretation boundary differs from v1",
		)
	}

	project, err := projectidentity.ParseProjectID(encoded.ProjectID)
	if err != nil || project.String() != encoded.ProjectID {
		return Bundle{}, fmt.Errorf(
			"GoldenConcernBundle project is invalid",
		)
	}
	contextRef, err := typedmemory.NewBoundedContextRef(
		encoded.BoundedContextRef,
	)
	if err != nil || contextRef.String() != encoded.BoundedContextRef {
		return Bundle{}, fmt.Errorf(
			"GoldenConcernBundle context is invalid",
		)
	}
	typeEnv, err := typedmemory.ParseTypeEnvRef(encoded.TypeEnvRef)
	if err != nil || typeEnv.String() != encoded.TypeEnvRef {
		return Bundle{}, fmt.Errorf(
			"GoldenConcernBundle TypeEnv is invalid",
		)
	}
	observedAt, err := parseCanonicalTime(encoded.ObservedAt)
	if err != nil {
		return Bundle{}, err
	}
	coordinate, err := NewSnapshotCoordinate(
		contextRef,
		typeEnv,
		typedmemory.NewGraphRevision(encoded.GraphRevision),
		observedAt,
	)
	if err != nil {
		return Bundle{}, err
	}
	concern, err := decodeConcernAdmission(
		project,
		typeEnv,
		encoded.ConcernAdmission,
	)
	if err != nil {
		return Bundle{}, err
	}
	if concern.reference.ReferenceID().String() !=
		encoded.EntityOfConcernRef {
		return Bundle{}, fmt.Errorf(
			"GoldenConcernBundle header and concern admission identify different entities",
		)
	}
	admissions, admissionIndexes, err := decodeAdapterAdmissions(
		project,
		encoded.AdapterAdmissions,
	)
	if err != nil {
		return Bundle{}, err
	}
	paths, err := decodeRelationPaths(
		typeEnv,
		encoded.ExpectedRelationPaths,
	)
	if err != nil {
		return Bundle{}, err
	}
	values, err := decodeValueWitnesses(
		typeEnv,
		encoded.ValueWitnesses,
	)
	if err != nil {
		return Bundle{}, err
	}
	if err := assignAdmissionWitnesses(
		concern,
		admissions,
		admissionIndexes,
		paths,
		values,
	); err != nil {
		return Bundle{}, err
	}
	if err := validateAdmissionSet(
		project,
		concern,
		coordinate,
		admissions,
	); err != nil {
		return Bundle{}, err
	}
	items, err := decodeAndRebuildItems(
		concern,
		coordinate,
		admissions,
		encoded.Items,
	)
	if err != nil {
		return Bundle{}, err
	}
	if err := validateGoldenShape(
		concern.reference,
		items,
		paths,
	); err != nil {
		return Bundle{}, err
	}
	bundle := Bundle{
		project:    project,
		coordinate: coordinate,
		concern:    concern,
		admissions: admissions,
		items:      items,
		paths:      paths,
		values:     values,
		canonical:  append([]byte(nil), canonical...),
	}
	digest, err := digestBytes(canonical)
	if err != nil {
		return Bundle{}, err
	}
	bundle.digest = digest
	if err := bundle.Verify(); err != nil {
		return Bundle{}, err
	}
	return bundle, nil
}

func decodeConcernAdmission(
	project projectidentity.ProjectID,
	typeEnv typedmemory.TypeEnvRef,
	encoded concernAdmissionCanonicalV1,
) (ConcernAdmission, error) {
	if encoded.ProjectID != project.String() {
		return ConcernAdmission{}, fmt.Errorf(
			"GoldenConcernBundle concern belongs to another project",
		)
	}
	reference, err := parsePersistedRef(
		encoded.ReferenceKind,
		encoded.ReferenceID,
	)
	if err != nil {
		return ConcernAdmission{}, err
	}
	if reference.RefKind().TypeEnv() != typeEnv {
		return ConcernAdmission{}, fmt.Errorf(
			"GoldenConcernBundle concern uses another TypeEnv",
		)
	}
	declaration, err := decodeDeclaration(encoded.Declaration)
	if err != nil {
		return ConcernAdmission{}, err
	}
	if reference.RefKind().ID().String() != "U.EntityRef" ||
		reference.ReferenceID().String() != declaration.entity.String() {
		return ConcernAdmission{}, fmt.Errorf(
			"GoldenConcernBundle concern reference is not its exact U.EntityRef",
		)
	}
	candidateDigest, err := typedmemory.NewSHA256Digest(
		encoded.CandidateDigest,
	)
	if err != nil {
		return ConcernAdmission{}, err
	}
	actualDigest, err := digestBytes(encoded.CandidateBytes)
	if err != nil {
		return ConcernAdmission{}, err
	}
	if actualDigest != candidateDigest {
		return ConcernAdmission{}, fmt.Errorf(
			"GoldenConcernBundle concern bytes differ from their digest",
		)
	}
	receipt, err := decodeReceipt(encoded.AdmissionReceipt)
	if err != nil {
		return ConcernAdmission{}, err
	}
	return ConcernAdmission{
		project:          project,
		declaration:      declaration,
		reference:        reference,
		candidateDigest:  candidateDigest,
		receipt:          receipt,
		canonicalChanges: append([]byte(nil), encoded.CandidateBytes...),
	}, nil
}

func decodeAdapterAdmissions(
	project projectidentity.ProjectID,
	encoded []adapterAdmissionCanonicalV1,
) ([]AdapterAdmission, map[string]int, error) {
	if len(encoded) == 0 {
		return nil, nil, fmt.Errorf(
			"GoldenConcernBundle export has no adapter admissions",
		)
	}
	admissions := make([]AdapterAdmission, 0, len(encoded))
	indexes := make(map[string]int, len(encoded))
	for _, value := range encoded {
		if value.ProjectID != project.String() {
			return nil, nil, fmt.Errorf(
				"GoldenConcernBundle adapter belongs to another project",
			)
		}
		manifest, err := recordmapping.ParseMappingManifestRef(
			value.MappingManifest,
		)
		if err != nil {
			return nil, nil, err
		}
		adapter, err := recordmapping.NewAdapterVersion(
			value.AdapterVersion,
		)
		if err != nil {
			return nil, nil, err
		}
		candidateDigest, err := typedmemory.NewSHA256Digest(
			value.CandidateDigest,
		)
		if err != nil {
			return nil, nil, err
		}
		actualDigest, err := digestBytes(value.CandidateBytes)
		if err != nil {
			return nil, nil, err
		}
		if actualDigest != candidateDigest {
			return nil, nil, fmt.Errorf(
				"GoldenConcernBundle adapter candidate bytes differ from their digest",
			)
		}
		signatures := make(
			[]typedmemory.SignatureID,
			0,
			len(value.Signatures),
		)
		for _, raw := range value.Signatures {
			signature, signatureErr := typedmemory.NewSignatureID(raw)
			if signatureErr != nil {
				return nil, nil, signatureErr
			}
			signatures = append(signatures, signature)
		}
		normalizedSignatures, err := normalizeSignatureIDs(signatures)
		if err != nil {
			return nil, nil, err
		}
		if !sameSignatureIDs(signatures, normalizedSignatures) {
			return nil, nil, fmt.Errorf(
				"GoldenConcernBundle adapter signatures are not canonical",
			)
		}
		declarations := make(
			[]DeclarationWitness,
			0,
			len(value.Declarations),
		)
		for _, raw := range value.Declarations {
			declaration, declarationErr := decodeDeclaration(raw)
			if declarationErr != nil {
				return nil, nil, declarationErr
			}
			declarations = append(declarations, declaration)
		}
		receipt, err := decodeReceipt(value.AdmissionReceipt)
		if err != nil {
			return nil, nil, err
		}
		if _, found := indexes[receipt.eventRef]; found {
			return nil, nil, fmt.Errorf(
				"GoldenConcernBundle repeats adapter event %q",
				receipt.eventRef,
			)
		}
		indexes[receipt.eventRef] = len(admissions)
		admissions = append(admissions, AdapterAdmission{
			project:          project,
			manifest:         manifest,
			adapter:          adapter,
			candidateDigest:  candidateDigest,
			receipt:          receipt,
			signatures:       normalizedSignatures,
			declarations:     declarations,
			canonicalChanges: append([]byte(nil), value.CandidateBytes...),
		})
	}
	return admissions, indexes, nil
}

func decodeRelationPaths(
	typeEnv typedmemory.TypeEnvRef,
	encoded []relationPathCanonicalV1,
) ([]RelationPath, error) {
	paths := make([]RelationPath, 0, len(encoded))
	for _, value := range encoded {
		assertion, err := typedmemory.NewAssertionID(value.Assertion)
		if err != nil {
			return nil, err
		}
		signature, err := typedmemory.NewSignatureID(value.Signature)
		if err != nil {
			return nil, err
		}
		contextRef, err := typedmemory.NewBoundedContextRef(value.Context)
		if err != nil {
			return nil, err
		}
		slot, err := typedmemory.NewSlotKindID(value.Slot)
		if err != nil {
			return nil, err
		}
		target, err := parsePersistedRef(
			value.TargetRefKind,
			value.TargetReferenceID,
		)
		if err != nil {
			return nil, err
		}
		if target.RefKind().TypeEnv() != typeEnv {
			return nil, fmt.Errorf(
				"GoldenConcernBundle relation target uses another TypeEnv",
			)
		}
		provenance, err := typedmemory.NewProvenanceRef(value.Provenance)
		if err != nil {
			return nil, err
		}
		eventRef, err := exactOneLine(
			"GoldenConcernBundle relation event",
			value.AdmissionEventRef,
		)
		if err != nil {
			return nil, err
		}
		paths = append(paths, RelationPath{
			assertion:         assertion,
			signature:         signature,
			context:           contextRef,
			slot:              slot,
			target:            target,
			provenance:        provenance,
			admissionEventRef: eventRef,
		})
	}
	return paths, nil
}

func decodeValueWitnesses(
	typeEnv typedmemory.TypeEnvRef,
	encoded []valueWitnessCanonicalV1,
) ([]ValueWitness, error) {
	values := make([]ValueWitness, 0, len(encoded))
	for _, value := range encoded {
		assertion, err := typedmemory.NewAssertionID(value.Assertion)
		if err != nil {
			return nil, err
		}
		signature, err := typedmemory.NewSignatureID(value.Signature)
		if err != nil {
			return nil, err
		}
		slot, err := typedmemory.NewSlotKindID(value.Slot)
		if err != nil {
			return nil, err
		}
		valueKind, err := parseValueKindRef(value.ValueKind)
		if err != nil {
			return nil, err
		}
		if valueKind.TypeEnv() != typeEnv {
			return nil, fmt.Errorf(
				"GoldenConcernBundle value witness uses another TypeEnv",
			)
		}
		shape, err := parseValueShapeRef(value.ValueShape)
		if err != nil {
			return nil, err
		}
		codec, err := parseCodecRef(value.Codec)
		if err != nil {
			return nil, err
		}
		inputDigest, err := typedmemory.NewSHA256Digest(value.InputDigest)
		if err != nil {
			return nil, err
		}
		eventRef, err := exactOneLine(
			"GoldenConcernBundle value event",
			value.AdmissionEventRef,
		)
		if err != nil {
			return nil, err
		}
		values = append(values, ValueWitness{
			assertion:         assertion,
			signature:         signature,
			slot:              slot,
			valueKind:         valueKind,
			valueShape:        shape,
			codec:             codec,
			inputDigest:       inputDigest,
			admissionEventRef: eventRef,
		})
	}
	return values, nil
}

func assignAdmissionWitnesses(
	concern ConcernAdmission,
	admissions []AdapterAdmission,
	indexes map[string]int,
	paths []RelationPath,
	values []ValueWitness,
) error {
	for _, path := range paths {
		if path.admissionEventRef == concern.receipt.eventRef {
			return fmt.Errorf(
				"GoldenConcernBundle concern-only event contains a relation path",
			)
		}
		index, found := indexes[path.admissionEventRef]
		if !found {
			return fmt.Errorf(
				"GoldenConcernBundle relation names unknown event %q",
				path.admissionEventRef,
			)
		}
		admissions[index].paths = append(
			admissions[index].paths,
			path,
		)
	}
	for _, value := range values {
		if value.admissionEventRef == concern.receipt.eventRef {
			return fmt.Errorf(
				"GoldenConcernBundle concern-only event contains a value witness",
			)
		}
		index, found := indexes[value.admissionEventRef]
		if !found {
			return fmt.Errorf(
				"GoldenConcernBundle value names unknown event %q",
				value.admissionEventRef,
			)
		}
		admissions[index].values = append(
			admissions[index].values,
			value,
		)
	}
	for index := range admissions {
		extracted := make([]typedmemory.SignatureID, 0)
		for _, path := range admissions[index].paths {
			extracted = append(extracted, path.signature)
		}
		for _, value := range admissions[index].values {
			extracted = append(extracted, value.signature)
		}
		normalized, err := normalizeSignatureIDs(extracted)
		if err != nil {
			return err
		}
		if !sameSignatureIDs(
			normalized,
			admissions[index].signatures,
		) {
			return fmt.Errorf(
				"GoldenConcernBundle relation witnesses differ from adapter signatures",
			)
		}
	}
	return nil
}

func decodeAndRebuildItems(
	concern ConcernAdmission,
	coordinate SnapshotCoordinate,
	admissions []AdapterAdmission,
	encoded []itemCanonicalV1,
) ([]BundleItem, error) {
	specs := make([]ItemSpec, 0, len(encoded))
	for _, value := range encoded {
		role, err := parseItemRole(value.Role)
		if err != nil {
			return nil, err
		}
		reference, err := parsePersistedRef(
			value.ReferenceKind,
			value.ReferenceID,
		)
		if err != nil {
			return nil, err
		}
		if role != ItemEntityOfConcern {
			spec, specErr := NewItemSpec(
				role,
				reference,
				value.AdmissionEventRef,
			)
			if specErr != nil {
				return nil, specErr
			}
			specs = append(specs, spec)
		}
	}
	items, err := materializeItems(
		concern,
		coordinate,
		admissions,
		specs,
	)
	if err != nil {
		return nil, err
	}
	if !sameItemCanonicalSlices(encodeItems(items), encoded) {
		return nil, fmt.Errorf(
			"GoldenConcernBundle item projection differs from admission witnesses",
		)
	}
	return items, nil
}

func decodeDeclaration(
	encoded declarationCanonicalV1,
) (DeclarationWitness, error) {
	entity, err := typedmemory.NewEntityID(encoded.EntityID)
	if err != nil {
		return DeclarationWitness{}, err
	}
	localRef, err := typedmemory.NewBatchLocalRef(encoded.LocalRef)
	if err != nil {
		return DeclarationWitness{}, err
	}
	contextRef, err := typedmemory.NewBoundedContextRef(encoded.Context)
	if err != nil {
		return DeclarationWitness{}, err
	}
	label, err := typedmemory.NewEntityLabel(encoded.Label)
	if err != nil {
		return DeclarationWitness{}, err
	}
	provenance, err := typedmemory.NewProvenanceRef(encoded.Provenance)
	if err != nil {
		return DeclarationWitness{}, err
	}
	return DeclarationWitness{
		entity:     entity,
		localRef:   localRef,
		context:    contextRef,
		label:      label,
		provenance: provenance,
	}, nil
}

func decodeReceipt(
	encoded receiptCanonicalV1,
) (receiptWitness, error) {
	disposition := typedmemorystore.CommitDisposition(encoded.Disposition)
	switch disposition {
	case typedmemorystore.CommitApplied,
		typedmemorystore.CommitReplay,
		typedmemorystore.CommitRecovered:
	default:
		return receiptWitness{}, fmt.Errorf(
			"GoldenConcernBundle receipt disposition is invalid",
		)
	}
	eventRef, err := exactOneLine(
		"GoldenConcernBundle receipt event",
		encoded.EventRef,
	)
	if err != nil {
		return receiptWitness{}, err
	}
	commitRef, err := exactOneLine(
		"GoldenConcernBundle receipt commit",
		encoded.CommitRef,
	)
	if err != nil {
		return receiptWitness{}, err
	}
	if encoded.GraphRevision == 0 {
		return receiptWitness{}, fmt.Errorf(
			"GoldenConcernBundle receipt revision is missing",
		)
	}
	result, err := typedmemory.NewSHA256Digest(encoded.ResultDigest)
	if err != nil {
		return receiptWitness{}, err
	}
	return receiptWitness{
		disposition: disposition,
		eventRef:    eventRef,
		commitRef:   commitRef,
		revision:    typedmemory.NewGraphRevision(encoded.GraphRevision),
		result:      result,
	}, nil
}

func parsePersistedRef(
	refKindRaw string,
	referenceIDRaw string,
) (typedmemory.PersistedRef, error) {
	refKind, err := parseRefKindRef(refKindRaw)
	if err != nil {
		return typedmemory.PersistedRef{}, err
	}
	referenceID, err := typedmemory.NewReferenceID(referenceIDRaw)
	if err != nil {
		return typedmemory.PersistedRef{}, err
	}
	return typedmemory.NewPersistedRef(refKind, referenceID)
}

func parseRefKindRef(raw string) (typedmemory.RefKindRef, error) {
	typeEnvRaw, idRaw, found := strings.Cut(raw, "/ref-kind/")
	if !found {
		return typedmemory.RefKindRef{}, fmt.Errorf(
			"GoldenConcernBundle RefKind is malformed",
		)
	}
	typeEnv, err := typedmemory.ParseTypeEnvRef(typeEnvRaw)
	if err != nil {
		return typedmemory.RefKindRef{}, err
	}
	id, err := typedmemory.NewRefKindID(idRaw)
	if err != nil {
		return typedmemory.RefKindRef{}, err
	}
	ref, err := typedmemory.NewRefKindRef(typeEnv, id)
	if err != nil {
		return typedmemory.RefKindRef{}, err
	}
	if ref.String() != raw {
		return typedmemory.RefKindRef{}, fmt.Errorf(
			"GoldenConcernBundle RefKind is not canonical",
		)
	}
	return ref, nil
}

func parseValueKindRef(raw string) (typedmemory.ValueKindRef, error) {
	typeEnvRaw, idRaw, found := strings.Cut(raw, "/value-kind/")
	if !found {
		return typedmemory.ValueKindRef{}, fmt.Errorf(
			"GoldenConcernBundle ValueKind is malformed",
		)
	}
	typeEnv, err := typedmemory.ParseTypeEnvRef(typeEnvRaw)
	if err != nil {
		return typedmemory.ValueKindRef{}, err
	}
	id, err := typedmemory.NewKindID(idRaw)
	if err != nil {
		return typedmemory.ValueKindRef{}, err
	}
	ref, err := typedmemory.NewValueKindRef(typeEnv, id)
	if err != nil {
		return typedmemory.ValueKindRef{}, err
	}
	if ref.String() != raw {
		return typedmemory.ValueKindRef{}, fmt.Errorf(
			"GoldenConcernBundle ValueKind is not canonical",
		)
	}
	return ref, nil
}

func parseValueShapeRef(raw string) (typedmemory.ValueShapeRef, error) {
	rest, found := strings.CutPrefix(raw, "shape:")
	if !found {
		return typedmemory.ValueShapeRef{}, fmt.Errorf(
			"GoldenConcernBundle ValueShape is malformed",
		)
	}
	digestIndex := strings.LastIndex(rest, "@sha256:")
	if digestIndex <= 0 {
		return typedmemory.ValueShapeRef{}, fmt.Errorf(
			"GoldenConcernBundle ValueShape digest is missing",
		)
	}
	id, err := typedmemory.NewShapeID(rest[:digestIndex])
	if err != nil {
		return typedmemory.ValueShapeRef{}, err
	}
	digest, err := typedmemory.NewSHA256Digest(
		rest[digestIndex+1:],
	)
	if err != nil {
		return typedmemory.ValueShapeRef{}, err
	}
	ref, err := typedmemory.NewValueShapeRef(id, digest)
	if err != nil {
		return typedmemory.ValueShapeRef{}, err
	}
	if ref.String() != raw {
		return typedmemory.ValueShapeRef{}, fmt.Errorf(
			"GoldenConcernBundle ValueShape is not canonical",
		)
	}
	return ref, nil
}

func parseCodecRef(raw string) (typedmemory.CodecRef, error) {
	rest, found := strings.CutPrefix(raw, "codec:")
	if !found {
		return typedmemory.CodecRef{}, fmt.Errorf(
			"GoldenConcernBundle CodecRef is malformed",
		)
	}
	idRaw, rest, err := consumeLengthPrefixed(rest)
	if err != nil {
		return typedmemory.CodecRef{}, err
	}
	versionRaw, digestRaw, err := consumeLengthPrefixed(rest)
	if err != nil {
		return typedmemory.CodecRef{}, err
	}
	id, err := typedmemory.NewCodecID(idRaw)
	if err != nil {
		return typedmemory.CodecRef{}, err
	}
	version, err := typedmemory.NewCanonicalizationVersion(versionRaw)
	if err != nil {
		return typedmemory.CodecRef{}, err
	}
	digest, err := typedmemory.NewSHA256Digest(digestRaw)
	if err != nil {
		return typedmemory.CodecRef{}, err
	}
	ref, err := typedmemory.NewCodecRef(id, version, digest)
	if err != nil {
		return typedmemory.CodecRef{}, err
	}
	if ref.String() != raw {
		return typedmemory.CodecRef{}, fmt.Errorf(
			"GoldenConcernBundle CodecRef is not canonical",
		)
	}
	return ref, nil
}

func consumeLengthPrefixed(raw string) (string, string, error) {
	lengthRaw, rest, found := strings.Cut(raw, ":")
	if !found {
		return "", "", fmt.Errorf(
			"GoldenConcernBundle length-prefixed identifier is malformed",
		)
	}
	length, err := strconv.Atoi(lengthRaw)
	if err != nil || length < 1 || len(rest) <= length ||
		rest[length] != ':' {
		return "", "", fmt.Errorf(
			"GoldenConcernBundle length-prefixed identifier is invalid",
		)
	}
	return rest[:length], rest[length+1:], nil
}

func parseCanonicalTime(raw string) (time.Time, error) {
	value, err := time.Parse(canonicalTimeLayout, raw)
	if err != nil || value.Format(canonicalTimeLayout) != raw {
		return time.Time{}, fmt.Errorf(
			"GoldenConcernBundle canonical time %q is invalid",
			raw,
		)
	}
	return value, nil
}

func decodeCanonicalJSON(canonical []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(canonical))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	trailing := json.RawMessage{}
	err := decoder.Decode(&trailing)
	if err == io.EOF {
		return nil
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("canonical JSON contains a second value")
}

func sameStrings(left []string, right []string) bool {
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

func sameItemCanonicalSlices(
	left []itemCanonicalV1,
	right []itemCanonicalV1,
) bool {
	leftBytes, leftErr := json.Marshal(left)
	rightBytes, rightErr := json.Marshal(right)
	return leftErr == nil &&
		rightErr == nil &&
		bytes.Equal(leftBytes, rightBytes)
}
