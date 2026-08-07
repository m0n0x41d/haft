package projecttypeenvassertionreport

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// GroundPosture distinguishes a witnessed contradiction from missing basis.
// Missing basis is never coerced to false or success.
type GroundPosture uint8

const (
	GroundInvalid GroundPosture = iota + 1
	GroundMissingBasis
)

func (posture GroundPosture) String() string {
	switch posture {
	case GroundInvalid:
		return "invalid"
	case GroundMissingBasis:
		return "missing_basis"
	default:
		return ""
	}
}

// GroundCode is a closed package-owned revalidation code or an exact
// typedmemory DiagnosticCode projected from the shared validators.
type GroundCode string

const (
	CodeTargetRelationFragmentUnavailable GroundCode = "target_typed_relation_declaration_fragment_unavailable"
	CodeRelationFragmentContextMismatch   GroundCode = "typed_relation_declaration_fragment_context_mismatch"
	// Historical codes remain decodable in sealed reports.
	CodeTargetSignatureUnavailable         GroundCode = "target_signature_unavailable"
	CodeTargetContextUnavailable           GroundCode = "target_context_unavailable"
	CodeSignatureContextMismatch           GroundCode = "signature_context_mismatch"
	CodeUnknownSlot                        GroundCode = "unknown_slot"
	CodeMissingSlot                        GroundCode = "missing_slot"
	CodeCardinalityMismatch                GroundCode = "cardinality_mismatch"
	CodeReferenceModeMismatch              GroundCode = "reference_mode_mismatch"
	CodeReferenceKindMismatch              GroundCode = "reference_kind_mismatch"
	CodeValueKindMismatch                  GroundCode = "value_kind_mismatch"
	CodeTargetKindUnavailable              GroundCode = "target_kind_unavailable"
	CodeValueBindingUnavailable            GroundCode = "value_binding_unavailable"
	CodeValueMigrationRequired             GroundCode = "value_migration_required"
	CodeValueCanonicalBytesChanged         GroundCode = "value_canonical_bytes_changed"
	CodeRefKindDefinitionMissing           GroundCode = "refkind_definition_unavailable"
	CodeRefKindReferentMismatch            GroundCode = "refkind_referent_mismatch"
	CodeStaticKindDisjointness             GroundCode = "static_kind_disjointness"
	CodeSlotGroupMismatch                  GroundCode = "slot_group_mismatch"
	CodeKindSignatureUnavailable           GroundCode = "kind_signature_unavailable"
	CodeEntitySetUnavailable               GroundCode = "entity_set_unavailable"
	CodeMemberOfEvaluatorMissing           GroundCode = "member_of_evaluator_unavailable"
	CodeMemberOfObservableMissing          GroundCode = "member_of_observable_unavailable"
	CodeMemberOfNotMember                  GroundCode = "member_of_not_member"
	CodeKindClassificationEvaluatorMissing GroundCode = "kind_classification_evaluator_unavailable"
	CodeKindClassificationBasisMissing     GroundCode = "kind_classification_basis_unavailable"
	CodeKindClassificationFalse            GroundCode = "kind_classification_false"
)

// GroundDetail is one immutable structured coordinate retained in a ground.
type GroundDetail struct {
	key    string
	values []string
}

func NewGroundDetail(
	key string,
	values []string,
) (GroundDetail, error) {
	normalized, err := normalizeGroundDetails([]GroundDetail{{
		key:    key,
		values: append([]string(nil), values...),
	}})
	if err != nil {
		return GroundDetail{}, err
	}
	return normalized[0], nil
}

func (detail GroundDetail) Key() string {
	return detail.key
}

func (detail GroundDetail) Values() []string {
	return append([]string(nil), detail.values...)
}

// Ground is one canonical contradiction or missing-basis statement.
// Exported constructors validate and normalize every field; callers cannot
// instantiate malformed private state directly.
type Ground struct {
	posture GroundPosture
	code    GroundCode
	path    string
	message string
	details []GroundDetail
	repair  string
}

func (ground Ground) Posture() GroundPosture {
	return ground.posture
}

func (ground Ground) Code() GroundCode {
	return ground.code
}

func (ground Ground) Path() string {
	return ground.path
}

func (ground Ground) Message() string {
	return ground.message
}

func (ground Ground) Details() []GroundDetail {
	result := make([]GroundDetail, 0, len(ground.details))
	for _, detail := range ground.details {
		result = append(result, GroundDetail{
			key:    detail.key,
			values: append([]string(nil), detail.values...),
		})
	}
	return result
}

func (ground Ground) RepairPointer() (string, bool) {
	return ground.repair, ground.repair != ""
}

func (ground Ground) CanonicalBytes() []byte {
	return canonicalGround(ground)
}

func NewInvalidGround(
	code GroundCode,
	path string,
	message string,
	details []GroundDetail,
) (Ground, error) {
	return newGround(
		GroundInvalid,
		code,
		path,
		message,
		details,
		"",
	)
}

func NewMissingBasisGround(
	code GroundCode,
	path string,
	message string,
	details []GroundDetail,
	repair string,
) (Ground, error) {
	return newGround(
		GroundMissingBasis,
		code,
		path,
		message,
		details,
		repair,
	)
}

func newGround(
	posture GroundPosture,
	code GroundCode,
	path string,
	message string,
	details []GroundDetail,
	repair string,
) (Ground, error) {
	if posture.String() == "" {
		return Ground{}, fmt.Errorf("ground posture is required")
	}
	if strings.TrimSpace(string(code)) == "" ||
		!utf8.ValidString(string(code)) {
		return Ground{}, fmt.Errorf("ground code is required")
	}
	path = strings.TrimSpace(path)
	message = strings.TrimSpace(message)
	repair = strings.TrimSpace(repair)
	if path == "" ||
		message == "" ||
		!utf8.ValidString(path) ||
		!utf8.ValidString(message) ||
		!utf8.ValidString(repair) {
		return Ground{}, fmt.Errorf("ground path and message are required")
	}
	if posture == GroundMissingBasis && repair == "" {
		return Ground{}, fmt.Errorf("missing-basis ground requires a repair pointer")
	}
	if posture == GroundInvalid && repair != "" {
		return Ground{}, fmt.Errorf("invalid ground cannot carry a missing-basis repair")
	}
	normalized, err := normalizeGroundDetails(details)
	if err != nil {
		return Ground{}, err
	}
	return Ground{
		posture: posture,
		code:    code,
		path:    path,
		message: message,
		details: normalized,
		repair:  repair,
	}, nil
}

func normalizeGroundDetails(details []GroundDetail) ([]GroundDetail, error) {
	owned := make([]GroundDetail, 0, len(details))
	for index, detail := range details {
		key := strings.TrimSpace(detail.key)
		if key == "" ||
			!utf8.ValidString(key) ||
			len(detail.values) == 0 ||
			len(detail.values) > maximumCanonicalElements {
			return nil, fmt.Errorf("ground detail %d is incomplete", index)
		}
		values := make([]string, 0, len(detail.values))
		for valueIndex, value := range detail.values {
			value = strings.TrimSpace(value)
			if value == "" || !utf8.ValidString(value) {
				return nil, fmt.Errorf(
					"ground detail %d value %d is empty",
					index,
					valueIndex,
				)
			}
			values = append(values, value)
		}
		owned = append(owned, GroundDetail{key: key, values: values})
	}
	if len(owned) > maximumCanonicalElements {
		return nil, fmt.Errorf("ground detail count exceeds the supported bound")
	}
	sort.Slice(owned, func(left, right int) bool {
		return owned[left].key < owned[right].key
	})
	for index := 1; index < len(owned); index++ {
		if owned[index-1].key == owned[index].key {
			return nil, fmt.Errorf(
				"ground repeats detail key %q",
				owned[index].key,
			)
		}
	}
	return owned, nil
}

func NewGroundFromDiagnostic(
	diagnostic typedmemory.Diagnostic,
) (Ground, error) {
	posture := GroundInvalid
	repair := ""
	if diagnostic.Posture() == typedmemory.DiagnosticUnderdetermined {
		posture = GroundMissingBasis
		pointer, found := diagnostic.Repair()
		if !found {
			return Ground{}, fmt.Errorf(
				"underdetermined diagnostic %s has no repair pointer",
				diagnostic.Code(),
			)
		}
		repair = pointer.String()
	}
	witness := diagnostic.Witness()
	details := []GroundDetail{
		datumGroundDetail("actual", witness.Actual()),
		datumGroundDetail("expected", witness.Expected()),
		{
			key:    "basis",
			values: diagnosticBasisCoordinates(diagnostic.GoverningBasis()),
		},
	}
	candidates := repairCandidateCoordinates(diagnostic.RepairCandidates())
	if len(candidates) > 0 {
		details = append(details, GroundDetail{
			key:    "repair_candidates",
			values: candidates,
		})
	}
	return newGround(
		posture,
		GroundCode(diagnostic.Code()),
		diagnostic.Path().String(),
		diagnostic.Message(),
		details,
		repair,
	)
}

func datumGroundDetail(
	prefix string,
	datum typedmemory.DiagnosticDatum,
) GroundDetail {
	values := []string{"kind=" + string(datum.Kind())}
	for _, value := range datum.Values() {
		values = append(values, "value="+value)
	}
	return GroundDetail{key: prefix, values: values}
}

func diagnosticBasisCoordinates(
	basis typedmemory.DiagnosticGoverningBasis,
) []string {
	values := []string{"kind=" + string(basis.Kind())}
	switch value := basis.(type) {
	case typedmemory.KnownDeclarationBasis:
		digest := sha256.Sum256(value.Provenance().CanonicalBytes())
		values = append(values, "provenance_sha256="+hex.EncodeToString(digest[:]))
	case typedmemory.CoreValidatorBasis:
		values = append(values, "rule="+value.Rule().String())
	case typedmemory.SnapshotRuleBasis:
		values = append(values, "rule="+value.Rule().String())
	case typedmemory.MissingTypeEnvDeclarationBasis:
		values = append(values, "typeenv="+value.TypeEnv().String())
		subject := value.Subject()
		values = append(values, "subject_kind="+string(subject.Kind()))
		for _, item := range subject.Values() {
			values = append(values, "subject_value="+item)
		}
	case typedmemory.MissingRuntimeBasis:
		values = append(values, "missing_kind="+string(value.MissingKind()))
		required := value.Required()
		values = append(values, "required_kind="+string(required.Kind()))
		for _, item := range required.Values() {
			values = append(values, "required_value="+item)
		}
	}
	return values
}

func repairCandidateCoordinates(
	candidates []typedmemory.RepairCandidate,
) []string {
	result := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		target := candidate.Target()
		parts := []string{
			"kind=" + string(candidate.Kind()),
			"pointer=" + candidate.Pointer().String(),
			"target_kind=" + string(target.Kind()),
			"human_choice=" + string(candidate.HumanChoiceRequirement()),
		}
		for _, value := range target.Values() {
			parts = append(parts, "target_value="+value)
		}
		result = append(result, strings.Join(parts, "\x1f"))
	}
	sort.Strings(result)
	return result
}

func canonicalGround(ground Ground) []byte {
	writer := newCanonicalWriter("haft.project-typeenv.assertion-revalidation-ground.v1")
	writer.addString(ground.posture.String())
	writer.addString(string(ground.code))
	writer.addString(ground.path)
	writer.addString(ground.message)
	writer.addString(ground.repair)
	writer.addUint64(uint64(len(ground.details)))
	for _, detail := range ground.details {
		writer.addString(detail.key)
		writer.addUint64(uint64(len(detail.values)))
		for _, value := range detail.values {
			writer.addString(value)
		}
	}
	return writer.bytes()
}

func normalizeGrounds(grounds []Ground) ([]Ground, error) {
	if len(grounds) > maximumCanonicalElements {
		return nil, fmt.Errorf("ground count exceeds the supported bound")
	}
	owned := append([]Ground(nil), grounds...)
	for index, ground := range owned {
		canonical, err := newGround(
			ground.posture,
			ground.code,
			ground.path,
			ground.message,
			ground.details,
			ground.repair,
		)
		if err != nil {
			return nil, fmt.Errorf("ground %d: %w", index, err)
		}
		owned[index] = canonical
	}
	sort.Slice(owned, func(left, right int) bool {
		return bytes.Compare(
			canonicalGround(owned[left]),
			canonicalGround(owned[right]),
		) < 0
	})
	result := make([]Ground, 0, len(owned))
	for _, ground := range owned {
		if len(result) > 0 && bytes.Equal(
			canonicalGround(result[len(result)-1]),
			canonicalGround(ground),
		) {
			continue
		}
		result = append(result, ground)
	}
	return result, nil
}
