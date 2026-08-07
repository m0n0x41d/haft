package specmigrationv2

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/m0n0x41d/haft/internal/authority"
)

const (
	migrationReviewAcceptanceContentSchemaV2 = "haft.spec-migration-v2.review-acceptance-content/v2"
	migrationReviewAdmissionSchemaV2         = "haft.spec-migration-v2.semantic-review-admission/v2"
	migrationReviewEffectSchemaV1            = "haft.spec-migration-v2.review-instituted-effect/v1"

	migrationReviewBoundedContextValue = "bounded-context:haft-spec-migration-v2"
	migrationReviewActTypeValue        = "speech-act-type:accept"
	migrationReviewEffectRuleValue     = "institution-rule:accept-institutes-migration-review-admission:v2"
	migrationReviewObjectKindValue     = "haft.MigrationReviewAdmission"
	migrationReviewModalityValue       = "ADMITTED"
	migrationReviewActionValue         = "spec-migration-v2.review.admit"
	migrationReviewUtteranceValue      = "utterance:accept-reviewed-migration:v1"
	migrationReviewContextPolicyValue  = "context-policy:migration-review-acceptance:v2"
	migrationReviewUtteranceVerb       = "ACCEPT"
	migrationReviewUtteranceLiteral    = "REVIEWED MIGRATION"
	migrationReviewMethodValue         = "method:migration-review-acceptance"
	migrationReviewMethodDescValue     = "method-description:migration-review-acceptance:v1"
	migrationReviewMethodProcedure     = "procedure:review-exact-intent-capture-controlling-terminal:v1"
	migrationReviewSystemValue         = "system:haft-spec-migration-review"
	migrationReviewStatePlaneValue     = "state-plane:spec-migration-review-admission"
	migrationReviewDeltaValue          = "delta-predicate:review-admission-instituted"
	migrationReviewOutcomeValue        = "work-outcome:review-admission-instituted"
)

type migrationReviewAcceptanceContent struct {
	ref       string
	digest    SHA256
	root      ApplyProjectRoot
	carrier   FinalCandidatePacketCarrier
	audit     PacketPartitionAuditBinding
	summary   migrationReviewEffectSummaryJSONV1
	canonical []byte
}

// migrationReviewProtocolPins is the sealed protocol identity expected on
// both initial admission and every durable reload. The generic SpeechAct
// source remains reusable; these pins state which exact source protocol may
// institute a migration-review admission.
type migrationReviewProtocolPins struct {
	contextPolicyRef        string
	contextPolicyDigest     SHA256
	actTypeRef              string
	methodRef               string
	methodDescriptionRef    string
	methodDescriptionDigest SHA256
	boundedContextRef       string
	effectRuleRef           string
}

type migrationReviewAcceptanceContentJSONV2 struct {
	Schema                  string                              `json:"schema"`
	ReviewContentRef        string                              `json:"review_content_ref"`
	ProjectRoot             string                              `json:"project_root"`
	PacketDigest            string                              `json:"packet_digest"`
	PacketCarrierDigest     string                              `json:"packet_carrier_digest"`
	PartitionAuditSchema    string                              `json:"partition_audit_schema"`
	PartitionAuditStatus    string                              `json:"partition_audit_status"`
	PartitionAuditDigest    string                              `json:"partition_audit_digest"`
	SourceCarrier           string                              `json:"source_carrier"`
	SourceDigest            string                              `json:"source_digest"`
	TargetCarrierDigests    []reviewCarrierDigestJSONV1         `json:"target_carrier_digests"`
	FPFRevision             string                              `json:"fpf_revision"`
	SemanticZeroPassCarrier string                              `json:"semantic_zero_pass_carrier"`
	SemanticZeroPassDigest  string                              `json:"semantic_zero_pass_digest"`
	LifecycleIntent         []lifecycleIntentJSONV1             `json:"lifecycle_intent"`
	ReviewSummary           *migrationReviewEffectSummaryJSONV1 `json:"review_summary,omitempty"`
}

type migrationReviewEffectSummaryJSONV1 struct {
	InstallSoftwareSystemCarrier string                                   `json:"install_software_system_carrier"`
	ArchiveEnablingSystemSource  string                                   `json:"archive_enabling_system_source"`
	ArchiveCarrier               string                                   `json:"archive_carrier"`
	AuditCounts                  migrationReviewAuditCountsJSONV1         `json:"audit_counts"`
	SourceDispositions           []migrationReviewSourceDispositionJSONV1 `json:"source_dispositions"`
}

type migrationReviewAuditCountsJSONV1 struct {
	SourceSections       int `json:"source_sections"`
	TopLevelDispositions int `json:"top_level_dispositions"`
	SplitSections        int `json:"split_sections"`
	SplitLeaves          int `json:"split_leaves"`
	WholeSectionOutcomes int `json:"whole_section_outcomes"`
	LineageEntries       int `json:"lineage_entries"`
}

type migrationReviewSourceDispositionJSONV1 struct {
	SourceSection string   `json:"source_section"`
	Destinations  []string `json:"destinations"`
}

// PreparedMigrationReviewAdmission is the pre-TTY migration-review subject.
// It binds exact source material and a stable future admission ref, but contains
// no terminal capture, performed SpeechAct, or instituted effect.
type PreparedMigrationReviewAdmission struct {
	state *preparedMigrationReviewAdmissionState
}

type preparedMigrationReviewAdmissionState struct {
	content         migrationReviewAcceptanceContent
	admissionRef    ReviewRef
	speechActRef    authority.SpeechActRef
	captureRef      authority.CarrierRef
	manualSource    authority.PreparedManualSpeechAct
	canonicalDigest SHA256
}

func PrepareMigrationReviewAdmission(
	carrier FinalCandidatePacketCarrier,
	audit PacketPartitionAudit,
) (PreparedMigrationReviewAdmission, error) {
	root, err := validateMigrationReviewPreparationBasis(carrier, audit)
	if err != nil {
		return PreparedMigrationReviewAdmission{}, err
	}
	content, err := newMigrationReviewAcceptanceContent(root, carrier, audit.Binding())
	if err != nil {
		return PreparedMigrationReviewAdmission{}, err
	}
	identity := migrationReviewIdentity(root.String(), content.digest.String())
	admissionRef, err := newReviewRef("review-admission:v2:" + identity)
	if err != nil {
		return PreparedMigrationReviewAdmission{}, err
	}
	speechActRef, err := authority.NewSpeechActRef("speech-act:migration-review:" + identity)
	if err != nil {
		return PreparedMigrationReviewAdmission{}, err
	}
	captureRef, err := authority.NewCarrierRef("carrier:terminal-capture:migration-review:" + identity)
	if err != nil {
		return PreparedMigrationReviewAdmission{}, err
	}
	intent, err := buildMigrationReviewSpeechActIntent(
		root,
		content,
		admissionRef,
		speechActRef,
		captureRef,
		identity,
	)
	if err != nil {
		return PreparedMigrationReviewAdmission{}, err
	}
	reviewText := migrationReviewText(content)
	manualSource, err := authority.PrepareManualSpeechAct(intent, reviewText)
	if err != nil {
		return PreparedMigrationReviewAdmission{}, err
	}
	state := preparedMigrationReviewAdmissionState{
		content:         content,
		admissionRef:    admissionRef,
		speechActRef:    speechActRef,
		captureRef:      captureRef,
		manualSource:    manualSource,
		canonicalDigest: DigestBytes([]byte(reviewText)),
	}
	prepared := PreparedMigrationReviewAdmission{state: &state}
	if err := validatePreparedMigrationReviewAdmission(prepared); err != nil {
		return PreparedMigrationReviewAdmission{}, err
	}
	return prepared, nil
}

func newMigrationReviewAcceptanceContent(
	root ApplyProjectRoot,
	carrier FinalCandidatePacketCarrier,
	audit PacketPartitionAuditBinding,
) (migrationReviewAcceptanceContent, error) {
	refIdentity := migrationReviewIdentity(
		root.String(),
		carrier.CarrierDigest().String(),
		audit.Digest().String(),
	)
	ref := "review-content:migration-v2:" + refIdentity
	summary, err := migrationReviewEffectSummary(carrier.Packet())
	if err != nil {
		return migrationReviewAcceptanceContent{}, err
	}
	dto := migrationReviewAcceptanceContentDTO(ref, root, carrier, audit, summary)
	canonical, err := marshalCanonicalJSON(dto)
	if err != nil {
		return migrationReviewAcceptanceContent{}, err
	}
	content := migrationReviewAcceptanceContent{
		ref:       ref,
		digest:    DigestBytes(canonical),
		root:      root,
		carrier:   carrier,
		audit:     audit,
		summary:   summary,
		canonical: canonical,
	}
	if err := validateMigrationReviewAcceptanceContent(content); err != nil {
		return migrationReviewAcceptanceContent{}, err
	}
	return content, nil
}

func migrationReviewAcceptanceContentDTO(
	ref string,
	root ApplyProjectRoot,
	carrier FinalCandidatePacketCarrier,
	audit PacketPartitionAuditBinding,
	summary migrationReviewEffectSummaryJSONV1,
) migrationReviewAcceptanceContentJSONV2 {
	packet := carrier.Packet()
	basis := carrier.ReviewBasis()
	zeroPass := basis.SemanticZeroPass()
	return migrationReviewAcceptanceContentJSONV2{
		Schema:                  migrationReviewAcceptanceContentSchemaV2,
		ReviewContentRef:        ref,
		ProjectRoot:             root.String(),
		PacketDigest:            carrier.PacketDigest().String(),
		PacketCarrierDigest:     carrier.CarrierDigest().String(),
		PartitionAuditSchema:    audit.Schema(),
		PartitionAuditStatus:    string(audit.Status()),
		PartitionAuditDigest:    audit.Digest().String(),
		SourceCarrier:           packet.Source().Carrier().String(),
		SourceDigest:            packet.Source().Digest().String(),
		TargetCarrierDigests:    canonicalReviewCarrierDTOs(basis.CarrierDigests()),
		FPFRevision:             basis.FPFRevision().String(),
		SemanticZeroPassCarrier: zeroPass.Carrier().String(),
		SemanticZeroPassDigest:  zeroPass.Digest().String(),
		LifecycleIntent:         canonicalLifecycleIntentDTOs(basis.LifecycleIntent()),
		ReviewSummary:           cloneMigrationReviewEffectSummary(summary),
	}
}

func validateMigrationReviewAcceptanceContent(
	content migrationReviewAcceptanceContent,
) error {
	if content.ref == "" || !content.digest.valid() || !content.root.valid() {
		return fmt.Errorf("migration-review acceptance content identity is invalid")
	}
	if !content.audit.valid() {
		return fmt.Errorf("migration-review acceptance content audit is invalid")
	}
	if err := validatePacketCarrierForReviewAdmission(content.carrier); err != nil {
		return err
	}
	root, err := reviewCarrierProjectRoot(content.carrier)
	if err != nil {
		return err
	}
	if root.String() != content.root.String() {
		return fmt.Errorf("migration-review acceptance content belongs to another project root")
	}
	expectedSummary, err := migrationReviewEffectSummary(content.carrier.Packet())
	if err != nil {
		return err
	}
	providedSummary, err := marshalCanonicalJSON(content.summary)
	if err != nil {
		return err
	}
	expectedSummaryBytes, err := marshalCanonicalJSON(expectedSummary)
	if err != nil {
		return err
	}
	if !slices.Equal(providedSummary, expectedSummaryBytes) {
		return fmt.Errorf("migration-review effect summary does not match the exact packet")
	}
	dto := migrationReviewAcceptanceContentDTO(
		content.ref,
		content.root,
		content.carrier,
		content.audit,
		content.summary,
	)
	canonical, err := marshalCanonicalJSON(dto)
	if err != nil {
		return err
	}
	if !slices.Equal(canonical, content.canonical) {
		return fmt.Errorf("migration-review acceptance content is not canonical")
	}
	observed := DigestBytes(canonical)
	if !observed.Equal(content.digest) {
		return fmt.Errorf("migration-review acceptance content digest does not bind its canonical record")
	}
	return nil
}

func buildMigrationReviewSpeechActIntent(
	root ApplyProjectRoot,
	content migrationReviewAcceptanceContent,
	admissionRef ReviewRef,
	speechActRef authority.SpeechActRef,
	captureRef authority.CarrierRef,
	identity string,
) (authority.PreparedSpeechActIntent, error) {
	policy, err := migrationReviewContextPolicy()
	if err != nil {
		return authority.PreparedSpeechActIntent{}, err
	}
	frame, err := migrationReviewExecutionFrame(content, admissionRef)
	if err != nil {
		return authority.PreparedSpeechActIntent{}, err
	}
	authorityRoot, err := authority.NewProjectRoot(root.String())
	if err != nil {
		return authority.PreparedSpeechActIntent{}, err
	}
	sessionRef, err := authority.NewSessionRef("session:migration-review:" + identity)
	if err != nil {
		return authority.PreparedSpeechActIntent{}, err
	}
	subjectRef, err := authority.NewSpeechActReviewSubjectRef(content.ref)
	if err != nil {
		return authority.PreparedSpeechActIntent{}, err
	}
	subjectDigest, err := authority.NewDigest(content.digest.String())
	if err != nil {
		return authority.PreparedSpeechActIntent{}, err
	}
	institutedRef, err := authority.NewInstitutedObjectRef(admissionRef.String())
	if err != nil {
		return authority.PreparedSpeechActIntent{}, err
	}
	builder := authority.NewPreparedSpeechActIntentBuilder(speechActRef, captureRef)
	builder = builder.ForProject(authorityRoot)
	builder = builder.InSession(sessionRef)
	builder = builder.Reviewing(subjectRef, subjectDigest)
	builder = builder.Institutes(institutedRef)
	builder = builder.UnderContextPolicy(policy)
	builder = builder.WithExecutionFrame(frame)
	return builder.Build()
}

func migrationReviewContextPolicy() (authority.SpeechActContextPolicy, error) {
	policyRef, err := authority.NewContextPolicyRef(migrationReviewContextPolicyValue)
	if err != nil {
		return authority.SpeechActContextPolicy{}, err
	}
	boundedContext, err := authority.NewBoundedContextRef(migrationReviewBoundedContextValue)
	if err != nil {
		return authority.SpeechActContextPolicy{}, err
	}
	actType, err := authority.NewSpeechActTypeRef(migrationReviewActTypeValue)
	if err != nil {
		return authority.SpeechActContextPolicy{}, err
	}
	ruleRef, err := authority.NewInstitutionalEffectRuleRef(migrationReviewEffectRuleValue)
	if err != nil {
		return authority.SpeechActContextPolicy{}, err
	}
	objectKind, err := authority.NewInstitutedObjectKind(migrationReviewObjectKindValue)
	if err != nil {
		return authority.SpeechActContextPolicy{}, err
	}
	modality, err := authority.NewInstitutionalModality(migrationReviewModalityValue)
	if err != nil {
		return authority.SpeechActContextPolicy{}, err
	}
	action, err := authority.NewActionKind(migrationReviewActionValue)
	if err != nil {
		return authority.SpeechActContextPolicy{}, err
	}
	utterance, err := authority.NewUtteranceRef(migrationReviewUtteranceValue)
	if err != nil {
		return authority.SpeechActContextPolicy{}, err
	}
	utteranceRule, err := authority.NewLiteralSpeechActUtteranceRule(
		migrationReviewUtteranceVerb,
		migrationReviewUtteranceLiteral,
	)
	if err != nil {
		return authority.SpeechActContextPolicy{}, err
	}
	rule, err := authority.NewInstitutionalEffectRule(
		ruleRef,
		objectKind,
		modality,
		action,
		utteranceRule,
		utterance,
	)
	if err != nil {
		return authority.SpeechActContextPolicy{}, err
	}
	return authority.NewSpeechActContextPolicy(policyRef, boundedContext, actType, rule)
}

func canonicalMigrationReviewProtocolPins() (migrationReviewProtocolPins, error) {
	policy, err := migrationReviewContextPolicy()
	if err != nil {
		return migrationReviewProtocolPins{}, err
	}
	policyRef, policyRefOK := policy.Ref()
	policyDigest, policyDigestOK := policy.Digest()
	boundedContext, boundedContextOK := policy.BoundedContext()
	actType, actTypeOK := policy.RecognizedActType()
	method, err := migrationReviewMethodDescription()
	if err != nil {
		return migrationReviewProtocolPins{}, err
	}
	methodRef, methodRefOK := method.MethodRef()
	methodDescriptionRef, methodDescriptionRefOK := method.Ref()
	methodDescriptionDigest, methodDescriptionDigestOK := method.Digest()
	complete := policyRefOK && policyDigestOK && boundedContextOK && actTypeOK
	complete = complete && methodRefOK && methodDescriptionRefOK && methodDescriptionDigestOK
	if !complete {
		return migrationReviewProtocolPins{}, fmt.Errorf("migration-review protocol pins are unavailable")
	}
	policyDigestValue, err := NewSHA256(policyDigest.String())
	if err != nil {
		return migrationReviewProtocolPins{}, err
	}
	methodDigestValue, err := NewSHA256(methodDescriptionDigest.String())
	if err != nil {
		return migrationReviewProtocolPins{}, err
	}
	return migrationReviewProtocolPins{
		contextPolicyRef:        policyRef.String(),
		contextPolicyDigest:     policyDigestValue,
		actTypeRef:              actType.String(),
		methodRef:               methodRef.String(),
		methodDescriptionRef:    methodDescriptionRef.String(),
		methodDescriptionDigest: methodDigestValue,
		boundedContextRef:       boundedContext.String(),
		effectRuleRef:           migrationReviewEffectRuleValue,
	}, nil
}

func migrationReviewExecutionFrame(
	content migrationReviewAcceptanceContent,
	admissionRef ReviewRef,
) (authority.SpeechActExecutionFrame, error) {
	systemRef, err := authority.NewSystemRef(migrationReviewSystemValue)
	if err != nil {
		return authority.SpeechActExecutionFrame{}, err
	}
	statePlane, err := authority.NewStatePlaneRef(migrationReviewStatePlaneValue)
	if err != nil {
		return authority.SpeechActExecutionFrame{}, err
	}
	delta, err := authority.NewDeltaPredicateRef(migrationReviewDeltaValue)
	if err != nil {
		return authority.SpeechActExecutionFrame{}, err
	}
	outcome, err := authority.NewWorkOutcomeRef(migrationReviewOutcomeValue)
	if err != nil {
		return authority.SpeechActExecutionFrame{}, err
	}
	utterance, err := authority.NewUtteranceRef(migrationReviewUtteranceValue)
	if err != nil {
		return authority.SpeechActExecutionFrame{}, err
	}
	parameter, err := authority.NewWorkParameterBinding(
		"parameter:review-content-digest",
		content.digest.String(),
	)
	if err != nil {
		return authority.SpeechActExecutionFrame{}, err
	}
	resource, err := authority.NewWorkResourceRef("resource:controlling-terminal")
	if err != nil {
		return authority.SpeechActExecutionFrame{}, err
	}
	affectedValues := []string{
		"affected:" + content.ref,
		"affected:" + admissionRef.String(),
		"affected:packet-carrier:" + content.carrier.CarrierDigest().String(),
	}
	affected, err := migrationReviewAffectedRefs(affectedValues, nil)
	if err != nil {
		return authority.SpeechActExecutionFrame{}, err
	}
	method, err := migrationReviewMethodDescription()
	if err != nil {
		return authority.SpeechActExecutionFrame{}, err
	}
	builder := authority.NewSpeechActExecutionFrameBuilder(method)
	builder = builder.ExecutedWithin(systemRef)
	builder = builder.OnStatePlane(statePlane, delta)
	builder = builder.WithOutcome(outcome)
	builder = builder.WithUtteranceDescription(utterance)
	builder = builder.BindParameter(parameter)
	builder = builder.UseResource(resource)
	builder = addMigrationReviewAffectedRefs(builder, affected, 0)
	return builder.Build()
}

func migrationReviewMethodDescription() (authority.SpeechActMethodDescription, error) {
	methodRef, err := authority.NewMethodRef(migrationReviewMethodValue)
	if err != nil {
		return authority.SpeechActMethodDescription{}, err
	}
	descriptionRef, err := authority.NewMethodDescriptionRef(migrationReviewMethodDescValue)
	if err != nil {
		return authority.SpeechActMethodDescription{}, err
	}
	procedureRef, err := authority.NewMethodProcedureRef(migrationReviewMethodProcedure)
	if err != nil {
		return authority.SpeechActMethodDescription{}, err
	}
	boundedContext, err := authority.NewBoundedContextRef(migrationReviewBoundedContextValue)
	if err != nil {
		return authority.SpeechActMethodDescription{}, err
	}
	return authority.NewManualControllingTTYMethodDescription(
		methodRef,
		descriptionRef,
		procedureRef,
		boundedContext,
	)
}

func migrationReviewAffectedRefs(
	values []string,
	result []authority.AffectedRef,
) ([]authority.AffectedRef, error) {
	if len(values) == 0 {
		return result, nil
	}
	ref, err := authority.NewAffectedRef(values[0])
	if err != nil {
		return nil, err
	}
	return migrationReviewAffectedRefs(values[1:], append(result, ref))
}

func addMigrationReviewAffectedRefs(
	builder authority.SpeechActExecutionFrameBuilder,
	refs []authority.AffectedRef,
	index int,
) authority.SpeechActExecutionFrameBuilder {
	if index == len(refs) {
		return builder
	}
	next := builder.Affect(refs[index])
	return addMigrationReviewAffectedRefs(next, refs, index+1)
}

func migrationReviewText(content migrationReviewAcceptanceContent) string {
	dto := migrationReviewAcceptanceContentDTO(
		content.ref,
		content.root,
		content.carrier,
		content.audit,
		content.summary,
	)
	return migrationReviewTextFromDTO(dto)
}

func migrationReviewEffectSummary(
	packet Packet,
) (migrationReviewEffectSummaryJSONV1, error) {
	counts := packetPartitionCounts(packet)
	registry := make(map[string]string, len(packet.OutsideRegistry().Values()))
	for _, registration := range packet.OutsideRegistry().Values() {
		registry[registration.ID().String()] = registration.Carrier().String()
	}
	dispositions := make([]migrationReviewSourceDispositionJSONV1, 0, len(packet.SourceDispositions()))
	for _, sourceDisposition := range packet.SourceDispositions() {
		destinations, err := migrationReviewDispositionDestinations(
			packet.Source().Archive().Carrier().String(),
			registry,
			sourceDisposition.Disposition(),
		)
		if err != nil {
			return migrationReviewEffectSummaryJSONV1{}, err
		}
		slices.Sort(destinations)
		destinations = slices.Compact(destinations)
		dispositions = append(dispositions, migrationReviewSourceDispositionJSONV1{
			SourceSection: sourceDisposition.Source().String(),
			Destinations:  destinations,
		})
	}
	slices.SortFunc(dispositions, func(
		left migrationReviewSourceDispositionJSONV1,
		right migrationReviewSourceDispositionJSONV1,
	) int {
		return strings.Compare(left.SourceSection, right.SourceSection)
	})
	return migrationReviewEffectSummaryJSONV1{
		InstallSoftwareSystemCarrier: packet.Target().Carrier().String(),
		ArchiveEnablingSystemSource:  packet.Source().Carrier().String(),
		ArchiveCarrier:               packet.Source().Archive().Carrier().String(),
		AuditCounts: migrationReviewAuditCountsJSONV1{
			SourceSections:       counts.SourceSections(),
			TopLevelDispositions: counts.TopLevelDispositions(),
			SplitSections:        counts.SplitSections(),
			SplitLeaves:          counts.SplitLeaves(),
			WholeSectionOutcomes: counts.WholeSectionOutcomes(),
			LineageEntries:       counts.LineageEntries(),
		},
		SourceDispositions: dispositions,
	}, nil
}

func migrationReviewDispositionDestinations(
	archiveCarrier string,
	outsideRegistry map[string]string,
	disposition Disposition,
) ([]string, error) {
	switch value := disposition.(type) {
	case MapOne:
		return []string{"target section " + value.TargetClaims().TargetSection().String()}, nil
	case RetireHistory:
		return []string{"history archive " + archiveCarrier}, nil
	case OutsidePSS:
		return migrationReviewOutsideDestinations(value, outsideRegistry)
	case SplitOneToMany:
		return migrationReviewSplitDestinations(archiveCarrier, outsideRegistry, value)
	default:
		return nil, fmt.Errorf("migration-review summary encountered an unknown disposition variant")
	}
}

func migrationReviewOutsideDestinations(
	disposition OutsidePSS,
	outsideRegistry map[string]string,
) ([]string, error) {
	destinations := make([]string, 0, len(disposition.Carriers().Values()))
	for _, carrierID := range disposition.Carriers().Values() {
		carrier, found := outsideRegistry[carrierID.String()]
		if !found {
			return nil, fmt.Errorf(
				"migration-review summary cannot resolve OutsidePSS carrier %q",
				carrierID.String(),
			)
		}
		destinations = append(destinations, "outside PSS carrier "+carrier)
	}
	return destinations, nil
}

func migrationReviewSplitDestinations(
	archiveCarrier string,
	outsideRegistry map[string]string,
	disposition SplitOneToMany,
) ([]string, error) {
	destinations := make([]string, 0, len(disposition.Branches()))
	for _, branch := range disposition.Branches() {
		branchDestinations, err := migrationReviewDispositionDestinations(
			archiveCarrier,
			outsideRegistry,
			branch.Disposition(),
		)
		if err != nil {
			return nil, err
		}
		destinations = append(destinations, branchDestinations...)
	}
	return destinations, nil
}

func cloneMigrationReviewEffectSummary(
	summary migrationReviewEffectSummaryJSONV1,
) *migrationReviewEffectSummaryJSONV1 {
	clone := summary
	clone.SourceDispositions = make(
		[]migrationReviewSourceDispositionJSONV1,
		0,
		len(summary.SourceDispositions),
	)
	for _, disposition := range summary.SourceDispositions {
		clone.SourceDispositions = append(
			clone.SourceDispositions,
			migrationReviewSourceDispositionJSONV1{
				SourceSection: disposition.SourceSection,
				Destinations:  append([]string{}, disposition.Destinations...),
			},
		)
	}
	return &clone
}

func migrationReviewTextFromDTO(
	dto migrationReviewAcceptanceContentJSONV2,
) string {
	if dto.ReviewSummary == nil {
		return migrationReviewLegacyTextFromDTO(dto)
	}
	summary := dto.ReviewSummary
	lines := []string{
		"Haft semantic review: SoftwareSystemSpec migration",
		"",
		"You are accepting the reviewed meaning of one exact migration candidate.",
		"This acceptance is recorded now; the migration work happens only on a later invocation.",
		"",
		"Scope",
		"- project: " + dto.ProjectRoot,
		"- source specification: " + dto.SourceCarrier,
		"- FPF source: the exact reviewed repository revision (bound internally)",
		"",
		"Migration effects reviewed for the later invocation",
		"- install the reviewed SoftwareSystemSpec at " + summary.InstallSoftwareSystemCarrier + ";",
		"- move the current EnablingSystemSpec source " + summary.ArchiveEnablingSystemSource +
			" to archive " + summary.ArchiveCarrier + ";",
		fmt.Sprintf(
			"- preserve explicit lineage for %d source sections through %d dispositions and %d lineage entries;",
			summary.AuditCounts.SourceSections,
			summary.AuditCounts.TopLevelDispositions,
			summary.AuditCounts.LineageEntries,
		),
		fmt.Sprintf(
			"- verified partition detail: %d whole-section outcomes, %d split sections, %d split leaves.",
			summary.AuditCounts.WholeSectionOutcomes,
			summary.AuditCounts.SplitSections,
			summary.AuditCounts.SplitLeaves,
		),
		"",
		"Source-section dispositions",
	}
	for _, disposition := range summary.SourceDispositions {
		lines = append(
			lines,
			"- "+disposition.SourceSection+" -> "+strings.Join(disposition.Destinations, "; "),
		)
	}
	lines = append(lines,
		"",
		"What this acceptance does now",
		"- bind this review to the exact candidate, partition audit, and reviewed carrier bytes;",
		"- make only that exact reviewed candidate eligible for a later migration preflight.",
		"",
		"What this acceptance does not do",
		"- apply the migration or edit, move, install, or archive any file;",
		"- approve, activate, reopen, or rebaseline any SpecSection;",
		"- change TypeEnv, project profile, code, Git history, tags, or publication state.",
		"",
		"Run `haft spec migrate` again later to perform the reviewed migration work.",
		"Cancel by interrupting or by entering anything other than the exact phrase.",
		"To accept, type exactly: "+migrationReviewUtteranceVerb+" "+migrationReviewUtteranceLiteral,
		"",
		"Reviewed supporting carriers",
	)
	for _, binding := range dto.TargetCarrierDigests {
		line := "- " + migrationReviewCarrierRoleLabel(binding.Role) + ": " + binding.Carrier
		lines = append(lines, line)
	}
	lines = append(lines, "", "Lifecycle intent carried by the candidate")
	for _, item := range dto.LifecycleIntent {
		lines = append(lines, "- "+item.Operation+" "+item.SectionRef)
	}
	return strings.Join(lines, "\n")
}

func migrationReviewCarrierRoleLabel(role string) string {
	return strings.ReplaceAll(role, "_", " ")
}

func migrationReviewLegacyTextFromDTO(
	dto migrationReviewAcceptanceContentJSONV2,
) string {
	lines := []string{
		"Haft semantic review: SoftwareSystemSpec migration",
		"",
		"You are accepting the reviewed meaning of one exact migration candidate.",
		"This records a semantic-review admission for a later, separate apply step.",
		"",
		"Scope",
		"- project: " + dto.ProjectRoot,
		"- source specification: " + dto.SourceCarrier,
		"- FPF source: the exact reviewed repository revision (bound internally)",
		"",
		"This SpeechAct will",
		"- bind the review to the exact packet, partition audit, and reviewed carrier bytes;",
		"- make that exact review available to a later migration-v2 preflight.",
		"",
		"This SpeechAct will not",
		"- apply the migration or edit, move, install, or archive any file;",
		"- approve, activate, reopen, or rebaseline any SpecSection;",
		"- change TypeEnv, project profile, code, Git history, tags, or publication state.",
		"",
		"A later invocation of `haft spec migrate` performs the migration work.",
		"Cancel by interrupting or by entering anything other than the exact phrase.",
		"To accept, type exactly: " + migrationReviewUtteranceVerb + " " + migrationReviewUtteranceLiteral,
		"",
		"Reviewed carriers",
	}
	for _, binding := range dto.TargetCarrierDigests {
		lines = append(lines, "- "+binding.Role+": "+binding.Carrier)
	}
	lines = append(lines, "", "Lifecycle intent carried by the candidate")
	for _, item := range dto.LifecycleIntent {
		lines = append(lines, "- "+item.Operation+" "+item.SectionRef)
	}
	return strings.Join(lines, "\n")
}

func validatePreparedMigrationReviewAdmission(
	prepared PreparedMigrationReviewAdmission,
) error {
	if prepared.state == nil {
		return fmt.Errorf("prepared migration-review admission is absent")
	}
	state := prepared.state
	if err := validateMigrationReviewAcceptanceContent(state.content); err != nil {
		return err
	}
	if state.admissionRef.String() == "" || state.speechActRef.String() == "" || state.captureRef.String() == "" {
		return fmt.Errorf("prepared migration-review admission identities are incomplete")
	}
	intent, intentOK := state.manualSource.Intent()
	reviewText, textOK := state.manualSource.ReviewText()
	reviewDigest, digestOK := state.manualSource.ReviewDigest()
	if !intentOK || !textOK || !digestOK {
		return fmt.Errorf("prepared migration-review manual SpeechAct source is invalid")
	}
	intentDigest, intentDigestOK := intent.Digest()
	if !intentDigestOK || intentDigest.String() == "" || reviewDigest.String() == "" {
		return fmt.Errorf("prepared migration-review SpeechAct bindings are absent")
	}
	if !DigestBytes([]byte(reviewText)).Equal(state.canonicalDigest) {
		return fmt.Errorf("prepared migration-review text digest is inconsistent")
	}
	identity := migrationReviewIdentity(state.content.root.String(), state.content.digest.String())
	if state.admissionRef.String() != "review-admission:v2:"+identity {
		return fmt.Errorf("prepared migration-review admission ref is not stable from its pre-TTY content")
	}
	return nil
}

func CaptureVerifiedMigrationReview(
	ctx context.Context,
	prepared PreparedMigrationReviewAdmission,
) (authority.VerifiedSpeechActSource, error) {
	if err := validatePreparedMigrationReviewAdmission(prepared); err != nil {
		return authority.VerifiedSpeechActSource{}, err
	}
	state := prepared.state
	return authority.CaptureVerifiedSpeechAct(
		ctx,
		state.manualSource,
	)
}

func (prepared PreparedMigrationReviewAdmission) ReviewContentRef() string {
	if validatePreparedMigrationReviewAdmission(prepared) != nil {
		return ""
	}
	return prepared.state.content.ref
}

func (prepared PreparedMigrationReviewAdmission) ReviewContentDigest() SHA256 {
	if validatePreparedMigrationReviewAdmission(prepared) != nil {
		return SHA256{}
	}
	return prepared.state.content.digest
}

func (prepared PreparedMigrationReviewAdmission) AdmissionRef() ReviewRef {
	if validatePreparedMigrationReviewAdmission(prepared) != nil {
		return ReviewRef{}
	}
	return prepared.state.admissionRef
}

func (prepared PreparedMigrationReviewAdmission) SpeechActRef() string {
	if validatePreparedMigrationReviewAdmission(prepared) != nil {
		return ""
	}
	return prepared.state.speechActRef.String()
}

func (prepared PreparedMigrationReviewAdmission) ReviewText() string {
	if validatePreparedMigrationReviewAdmission(prepared) != nil {
		return ""
	}
	reviewText, _ := prepared.state.manualSource.ReviewText()
	return reviewText
}

func (prepared PreparedMigrationReviewAdmission) ReviewDigest() string {
	if validatePreparedMigrationReviewAdmission(prepared) != nil {
		return ""
	}
	reviewDigest, _ := prepared.state.manualSource.ReviewDigest()
	return reviewDigest.String()
}

func migrationReviewIdentity(values ...string) string {
	hash := sha256.New()
	_, _ = hash.Write([]byte("haft.spec-migration-v2.review-identity/v2\x00"))
	for _, value := range values {
		_, _ = hash.Write([]byte(value))
		_, _ = hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func decodeMigrationReviewAcceptanceContent(
	canonical []byte,
) (migrationReviewAcceptanceContentJSONV2, error) {
	dto := migrationReviewAcceptanceContentJSONV2{}
	decoder := json.NewDecoder(strings.NewReader(string(canonical)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&dto); err != nil {
		return migrationReviewAcceptanceContentJSONV2{}, err
	}
	reencoded, err := marshalCanonicalJSON(dto)
	if err != nil {
		return migrationReviewAcceptanceContentJSONV2{}, err
	}
	if !slices.Equal(reencoded, canonical) {
		return migrationReviewAcceptanceContentJSONV2{}, fmt.Errorf("migration-review acceptance content JSON is not canonical")
	}
	return dto, nil
}
