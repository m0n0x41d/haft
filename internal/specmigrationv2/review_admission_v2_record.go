package specmigrationv2

import (
	"fmt"
	"slices"
	"time"

	"github.com/m0n0x41d/haft/internal/authority"
)

const migrationReviewEffectDigestDomain = "haft.spec-migration-v2.review-instituted-effect-digest/v1\x00"

type migrationReviewAdmissionJSONV2 struct {
	Schema                     string `json:"schema"`
	AdmissionRef               string `json:"admission_ref"`
	ProjectRoot                string `json:"project_root"`
	PacketCarrierDigest        string `json:"packet_carrier_digest"`
	ReviewContentRef           string `json:"review_content_ref"`
	ReviewContentDigest        string `json:"review_content_digest"`
	ReviewText                 string `json:"review_text"`
	ReviewDigest               string `json:"review_digest"`
	CaptureCarrierRef          string `json:"capture_carrier_ref"`
	CaptureCarrierDigest       string `json:"capture_carrier_digest"`
	SpeechActRef               string `json:"speech_act_ref"`
	SpeechActDigest            string `json:"speech_act_digest"`
	ContextPolicyRef           string `json:"context_policy_ref"`
	ContextPolicyDigest        string `json:"context_policy_digest"`
	ActTypeRef                 string `json:"act_type_ref"`
	MethodRef                  string `json:"method_ref"`
	MethodDescriptionRef       string `json:"method_description_ref"`
	MethodDescriptionDigest    string `json:"method_description_digest"`
	BoundedContextRef          string `json:"bounded_context_ref"`
	InstitutionalEffectRuleRef string `json:"institutional_effect_rule_ref"`
	AdmittedAt                 string `json:"admitted_at"`
}

type migrationReviewEffectPayloadV1 struct {
	Schema          string `json:"schema"`
	ProjectRoot     string `json:"project_root"`
	SpeechActRef    string `json:"speech_act_ref"`
	SpeechActDigest string `json:"speech_act_digest"`
	AdmissionRef    string `json:"admission_ref"`
	AdmissionDigest string `json:"admission_digest"`
}

type migrationReviewEffectJSONV1 struct {
	Schema          string `json:"schema"`
	EffectDigest    string `json:"effect_digest"`
	ProjectRoot     string `json:"project_root"`
	SpeechActRef    string `json:"speech_act_ref"`
	SpeechActDigest string `json:"speech_act_digest"`
	AdmissionRef    string `json:"admission_ref"`
	AdmissionDigest string `json:"admission_digest"`
}

type migrationReviewAdmissionRecordV2 struct {
	admittedMigrationReview
	content         migrationReviewAcceptanceContent
	reviewText      string
	reviewDigest    SHA256
	captureRef      string
	captureDigest   SHA256
	protocol        migrationReviewProtocolPins
	admittedAt      time.Time
	canonical       []byte
	effectDigest    SHA256
	effectCanonical []byte
}

func newMigrationReviewAdmissionRecordV2(
	prepared PreparedMigrationReviewAdmission,
	source authority.RecordedSpeechActSource,
) (migrationReviewAdmissionRecordV2, error) {
	if err := validatePreparedMigrationReviewAdmission(prepared); err != nil {
		return migrationReviewAdmissionRecordV2{}, err
	}
	bindings, err := exactRecordedMigrationReviewSource(prepared, source)
	if err != nil {
		return migrationReviewAdmissionRecordV2{}, err
	}
	content := prepared.state.content
	packet := content.carrier.Packet()
	basis := content.carrier.ReviewBasis()
	speechActRef, err := NewSemanticReviewSpeechActRef(bindings.speechActRef)
	if err != nil {
		return migrationReviewAdmissionRecordV2{}, err
	}
	speechActDigest, err := NewSHA256(bindings.speechActDigest)
	if err != nil {
		return migrationReviewAdmissionRecordV2{}, err
	}
	reviewDigest, err := NewSHA256(bindings.reviewDigest)
	if err != nil {
		return migrationReviewAdmissionRecordV2{}, err
	}
	captureDigest, err := NewSHA256(bindings.captureDigest)
	if err != nil {
		return migrationReviewAdmissionRecordV2{}, err
	}
	protocol, err := canonicalMigrationReviewProtocolPins()
	if err != nil {
		return migrationReviewAdmissionRecordV2{}, err
	}
	review := admittedMigrationReview{
		reviewRef:            prepared.state.admissionRef,
		speechActRef:         speechActRef,
		speechActDigest:      speechActDigest,
		projectRoot:          content.root,
		packetDigest:         content.carrier.PacketDigest(),
		packetCarrierDigest:  content.carrier.CarrierDigest(),
		partitionAudit:       content.audit,
		sourceCarrier:        packet.Source().Carrier(),
		sourceDigest:         packet.Source().Digest(),
		targetCarrierDigests: basis.CarrierDigests(),
		fpfRevision:          basis.FPFRevision(),
		semanticZeroPass:     basis.SemanticZeroPass(),
		lifecycleIntent:      basis.LifecycleIntent(),
	}
	record := migrationReviewAdmissionRecordV2{
		admittedMigrationReview: review,
		content:                 content,
		reviewText:              prepared.ReviewText(),
		reviewDigest:            reviewDigest,
		captureRef:              bindings.captureRef,
		captureDigest:           captureDigest,
		protocol:                protocol,
		admittedAt:              canonicalReviewTime(bindings.occurredAt),
	}
	canonical, err := encodeMigrationReviewAdmissionV2(record)
	if err != nil {
		return migrationReviewAdmissionRecordV2{}, err
	}
	record.canonical = canonical
	record.admissionDigest = DigestBytes(canonical)
	effectDigest, effectCanonical, err := encodeMigrationReviewEffectV1(record)
	if err != nil {
		return migrationReviewAdmissionRecordV2{}, err
	}
	record.effectDigest = effectDigest
	record.effectCanonical = effectCanonical
	return record, validateMigrationReviewAdmissionRecordV2(record)
}

type recordedMigrationReviewSourceBindings struct {
	projectRoot     string
	speechActRef    string
	speechActDigest string
	captureRef      string
	captureDigest   string
	reviewDigest    string
	reviewText      string
	subjectRef      string
	subjectDigest   string
	occurredAt      time.Time
}

func exactRecordedMigrationReviewSource(
	prepared PreparedMigrationReviewAdmission,
	source authority.RecordedSpeechActSource,
) (recordedMigrationReviewSourceBindings, error) {
	if !source.Valid() {
		return recordedMigrationReviewSourceBindings{}, fmt.Errorf("migration-review admission requires a durable canonical SpeechAct source")
	}
	projectRoot, rootOK := source.ProjectRoot()
	speechActRef, refOK := source.SpeechActRef()
	speechActDigest, actDigestOK := source.SpeechActDigest()
	captureRef, captureRefOK := source.TerminalCaptureRef()
	captureDigest, captureDigestOK := source.TerminalCaptureDigest()
	reviewDigest, reviewDigestOK := source.ReviewDigest()
	reviewText, reviewTextOK := source.ReviewText()
	subjectRef, subjectRefOK := source.ReviewSubjectRef()
	subjectDigest, subjectDigestOK := source.ReviewSubjectDigest()
	occurredAt, occurredAtOK := source.OccurredAt()
	complete := rootOK && refOK && actDigestOK && captureRefOK && captureDigestOK
	complete = complete && reviewDigestOK && reviewTextOK && subjectRefOK && subjectDigestOK && occurredAtOK
	if !complete {
		return recordedMigrationReviewSourceBindings{}, fmt.Errorf("durable migration-review SpeechAct source is incomplete")
	}
	bindings := recordedMigrationReviewSourceBindings{
		projectRoot:     projectRoot.String(),
		speechActRef:    speechActRef.String(),
		speechActDigest: speechActDigest.String(),
		captureRef:      captureRef.String(),
		captureDigest:   captureDigest.String(),
		reviewDigest:    reviewDigest.String(),
		reviewText:      reviewText,
		subjectRef:      subjectRef.String(),
		subjectDigest:   subjectDigest.String(),
		occurredAt:      occurredAt,
	}
	if err := validateRecordedMigrationReviewBindings(prepared, bindings); err != nil {
		return recordedMigrationReviewSourceBindings{}, err
	}
	return bindings, nil
}

func validateRecordedMigrationReviewBindings(
	prepared PreparedMigrationReviewAdmission,
	bindings recordedMigrationReviewSourceBindings,
) error {
	state := prepared.state
	reviewText, textOK := state.manualSource.ReviewText()
	reviewDigest, digestOK := state.manualSource.ReviewDigest()
	if !textOK || !digestOK {
		return fmt.Errorf("prepared migration-review manual SpeechAct source is invalid")
	}
	checks := []struct {
		matches bool
		name    string
	}{
		{matches: bindings.projectRoot == state.content.root.String(), name: "project root"},
		{matches: bindings.speechActRef == state.speechActRef.String(), name: "SpeechAct ref"},
		{matches: bindings.captureRef == state.captureRef.String(), name: "capture ref"},
		{matches: bindings.reviewDigest == reviewDigest.String(), name: "review digest"},
		{matches: bindings.reviewText == reviewText, name: "review text"},
		{matches: bindings.subjectRef == state.content.ref, name: "review subject ref"},
		{matches: bindings.subjectDigest == state.content.digest.String(), name: "review subject digest"},
		{matches: !bindings.occurredAt.IsZero(), name: "SpeechAct occurrence"},
	}
	for _, check := range checks {
		if !check.matches {
			return fmt.Errorf("durable migration-review SpeechAct source has another %s", check.name)
		}
	}
	return nil
}

func encodeMigrationReviewAdmissionV2(
	record migrationReviewAdmissionRecordV2,
) ([]byte, error) {
	dto := migrationReviewAdmissionDTO(record)
	return marshalCanonicalJSON(dto)
}

func migrationReviewAdmissionDTO(
	record migrationReviewAdmissionRecordV2,
) migrationReviewAdmissionJSONV2 {
	return migrationReviewAdmissionJSONV2{
		Schema:                     migrationReviewAdmissionSchemaV2,
		AdmissionRef:               record.reviewRef.String(),
		ProjectRoot:                record.projectRoot.String(),
		PacketCarrierDigest:        record.packetCarrierDigest.String(),
		ReviewContentRef:           record.content.ref,
		ReviewContentDigest:        record.content.digest.String(),
		ReviewText:                 record.reviewText,
		ReviewDigest:               record.reviewDigest.String(),
		CaptureCarrierRef:          record.captureRef,
		CaptureCarrierDigest:       record.captureDigest.String(),
		SpeechActRef:               record.speechActRef.String(),
		SpeechActDigest:            record.speechActDigest.String(),
		ContextPolicyRef:           record.protocol.contextPolicyRef,
		ContextPolicyDigest:        record.protocol.contextPolicyDigest.String(),
		ActTypeRef:                 record.protocol.actTypeRef,
		MethodRef:                  record.protocol.methodRef,
		MethodDescriptionRef:       record.protocol.methodDescriptionRef,
		MethodDescriptionDigest:    record.protocol.methodDescriptionDigest.String(),
		BoundedContextRef:          record.protocol.boundedContextRef,
		InstitutionalEffectRuleRef: record.protocol.effectRuleRef,
		AdmittedAt:                 formatReviewTime(record.admittedAt),
	}
}

func encodeMigrationReviewEffectV1(
	record migrationReviewAdmissionRecordV2,
) (SHA256, []byte, error) {
	payload := migrationReviewEffectPayload(record)
	canonicalPayload, err := marshalCanonicalJSON(payload)
	if err != nil {
		return SHA256{}, nil, err
	}
	digestInput := append([]byte(migrationReviewEffectDigestDomain), canonicalPayload...)
	digest := DigestBytes(digestInput)
	dto := migrationReviewEffectJSONV1{
		Schema:          payload.Schema,
		EffectDigest:    digest.String(),
		ProjectRoot:     payload.ProjectRoot,
		SpeechActRef:    payload.SpeechActRef,
		SpeechActDigest: payload.SpeechActDigest,
		AdmissionRef:    payload.AdmissionRef,
		AdmissionDigest: payload.AdmissionDigest,
	}
	canonical, err := marshalCanonicalJSON(dto)
	if err != nil {
		return SHA256{}, nil, err
	}
	return digest, canonical, nil
}

func migrationReviewEffectPayload(
	record migrationReviewAdmissionRecordV2,
) migrationReviewEffectPayloadV1 {
	return migrationReviewEffectPayloadV1{
		Schema:          migrationReviewEffectSchemaV1,
		ProjectRoot:     record.projectRoot.String(),
		SpeechActRef:    record.speechActRef.String(),
		SpeechActDigest: record.speechActDigest.String(),
		AdmissionRef:    record.reviewRef.String(),
		AdmissionDigest: record.admissionDigest.String(),
	}
}

func validateMigrationReviewAdmissionRecordV2(
	record migrationReviewAdmissionRecordV2,
) error {
	if err := validateAdmittedMigrationReview(record.admittedMigrationReview); err != nil {
		return err
	}
	if err := validateMigrationReviewAcceptanceContent(record.content); err != nil {
		return err
	}
	if record.reviewText == "" || !record.reviewDigest.valid() || record.captureRef == "" || !record.captureDigest.valid() {
		return fmt.Errorf("migration-review admission source bindings are invalid")
	}
	expectedReviewText := migrationReviewText(record.content)
	if record.reviewText != expectedReviewText {
		return fmt.Errorf("migration-review admission review text does not bind exact acceptance content")
	}
	expectedProtocol, err := canonicalMigrationReviewProtocolPins()
	if err != nil {
		return err
	}
	if !sameMigrationReviewProtocolPins(record.protocol, expectedProtocol) {
		return fmt.Errorf("migration-review admission does not bind the sealed protocol")
	}
	if record.admittedAt.IsZero() {
		return fmt.Errorf("migration-review admission occurrence is required")
	}
	canonical, err := encodeMigrationReviewAdmissionV2(record)
	if err != nil {
		return err
	}
	if !slices.Equal(canonical, record.canonical) {
		return fmt.Errorf("migration-review admission JSON is not canonical")
	}
	if !DigestBytes(canonical).Equal(record.admissionDigest) {
		return fmt.Errorf("migration-review admission digest does not bind its canonical record")
	}
	effectDigest, effectCanonical, err := encodeMigrationReviewEffectV1(record)
	if err != nil {
		return err
	}
	if !effectDigest.Equal(record.effectDigest) || !slices.Equal(effectCanonical, record.effectCanonical) {
		return fmt.Errorf("migration-review instituted effect is not canonical")
	}
	return nil
}

func sameMigrationReviewProtocolPins(
	left migrationReviewProtocolPins,
	right migrationReviewProtocolPins,
) bool {
	return left.contextPolicyRef == right.contextPolicyRef &&
		left.contextPolicyDigest.Equal(right.contextPolicyDigest) &&
		left.actTypeRef == right.actTypeRef &&
		left.methodRef == right.methodRef &&
		left.methodDescriptionRef == right.methodDescriptionRef &&
		left.methodDescriptionDigest.Equal(right.methodDescriptionDigest) &&
		left.boundedContextRef == right.boundedContextRef &&
		left.effectRuleRef == right.effectRuleRef
}
