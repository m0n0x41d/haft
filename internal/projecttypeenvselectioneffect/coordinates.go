package projecttypeenvselectioneffect

import (
	"bytes"
	"fmt"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/fpf/projecttypeenv"
	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectionauthority"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// ProjectTypeEnvHeadSelectionTarget is the exact B, ordered E, X, C, Stage
// target carried by every success record. Ordered extensions are never sorted.
type ProjectTypeEnvHeadSelectionTarget struct {
	base              typedmemory.TypeEnvRef
	orderedExtensions []typedmemory.TypeEnvExtensionRef
	runtimeBasis      projecttypeenv.RuntimeEvaluationBasisRef
	composite         typedmemory.TypeEnvRef
	stage             projecttypeenvselection.ProjectTypeEnvStageRef
}

type ProjectTypeEnvHeadSelectionTargetInput struct {
	Base              typedmemory.TypeEnvRef
	OrderedExtensions []typedmemory.TypeEnvExtensionRef
	RuntimeBasis      projecttypeenv.RuntimeEvaluationBasisRef
	Composite         typedmemory.TypeEnvRef
	Stage             projecttypeenvselection.ProjectTypeEnvStageRef
}

func NewProjectTypeEnvHeadSelectionTarget(
	input ProjectTypeEnvHeadSelectionTargetInput,
) (ProjectTypeEnvHeadSelectionTarget, error) {
	base, err := typedmemory.ParseTypeEnvRef(input.Base.String())
	if err != nil || base != input.Base {
		return ProjectTypeEnvHeadSelectionTarget{}, fmt.Errorf("target base TypeEnv is required")
	}
	if len(input.OrderedExtensions) > maximumOrderedExtensions {
		return ProjectTypeEnvHeadSelectionTarget{}, fmt.Errorf(
			"target extension count exceeds %d",
			maximumOrderedExtensions,
		)
	}
	extensions := make([]typedmemory.TypeEnvExtensionRef, 0, len(input.OrderedExtensions))
	seen := make(map[string]struct{}, len(input.OrderedExtensions))
	for _, candidate := range input.OrderedExtensions {
		extension, parseErr := typedmemory.ParseTypeEnvExtensionRef(candidate.String())
		if parseErr != nil || extension != candidate {
			return ProjectTypeEnvHeadSelectionTarget{}, fmt.Errorf(
				"target ordered extension is invalid",
			)
		}
		if _, exists := seen[extension.String()]; exists {
			return ProjectTypeEnvHeadSelectionTarget{}, fmt.Errorf(
				"target ordered extensions contain a duplicate",
			)
		}
		seen[extension.String()] = struct{}{}
		extensions = append(extensions, extension)
	}
	runtimeBasis, err := projecttypeenv.ParseRuntimeEvaluationBasisRef(
		input.RuntimeBasis.String(),
	)
	if err != nil || runtimeBasis != input.RuntimeBasis {
		return ProjectTypeEnvHeadSelectionTarget{}, fmt.Errorf("target runtime basis is required")
	}
	composite, err := typedmemory.ParseTypeEnvRef(input.Composite.String())
	if err != nil || composite != input.Composite {
		return ProjectTypeEnvHeadSelectionTarget{}, fmt.Errorf("target composite TypeEnv is required")
	}
	stage, err := projecttypeenvselection.ParseProjectTypeEnvStageRef(input.Stage.String())
	if err != nil || stage != input.Stage {
		return ProjectTypeEnvHeadSelectionTarget{}, fmt.Errorf("target Stage is required")
	}
	return ProjectTypeEnvHeadSelectionTarget{
		base:              base,
		orderedExtensions: extensions,
		runtimeBasis:      runtimeBasis,
		composite:         composite,
		stage:             stage,
	}, nil
}

func ProjectTypeEnvHeadSelectionTargetFromRequest(
	request projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest,
) (ProjectTypeEnvHeadSelectionTarget, error) {
	if err := request.Verify(); err != nil {
		return ProjectTypeEnvHeadSelectionTarget{}, fmt.Errorf(
			"verify head-selection request: %w",
			err,
		)
	}
	target := request.Target()
	return NewProjectTypeEnvHeadSelectionTarget(
		ProjectTypeEnvHeadSelectionTargetInput{
			Base:              target.Base(),
			OrderedExtensions: target.OrderedExtensions(),
			RuntimeBasis:      target.RuntimeBasis(),
			Composite:         target.VerifiedComposite(),
			Stage:             target.Stage(),
		},
	)
}

func (target ProjectTypeEnvHeadSelectionTarget) Base() typedmemory.TypeEnvRef {
	return target.base
}

func (target ProjectTypeEnvHeadSelectionTarget) OrderedExtensions() []typedmemory.TypeEnvExtensionRef {
	return append([]typedmemory.TypeEnvExtensionRef(nil), target.orderedExtensions...)
}

func (target ProjectTypeEnvHeadSelectionTarget) RuntimeBasis() projecttypeenv.RuntimeEvaluationBasisRef {
	return target.runtimeBasis
}

func (target ProjectTypeEnvHeadSelectionTarget) Composite() typedmemory.TypeEnvRef {
	return target.composite
}

func (target ProjectTypeEnvHeadSelectionTarget) Stage() projecttypeenvselection.ProjectTypeEnvStageRef {
	return target.stage
}

func encodeTarget(
	writer *canonicalWriter,
	target ProjectTypeEnvHeadSelectionTarget,
) {
	writer.writeString(target.base.String())
	count, _ := countToUint32("target extensions", len(target.orderedExtensions))
	writer.writeUint32(count)
	for _, extension := range target.orderedExtensions {
		writer.writeString(extension.String())
	}
	writer.writeString(target.runtimeBasis.String())
	writer.writeString(target.composite.String())
	writer.writeString(target.stage.String())
}

func decodeTarget(
	reader *canonicalReader,
) (ProjectTypeEnvHeadSelectionTarget, error) {
	baseText, err := reader.readString("target base")
	if err != nil {
		return ProjectTypeEnvHeadSelectionTarget{}, err
	}
	base, err := typedmemory.ParseTypeEnvRef(baseText)
	if err != nil {
		return ProjectTypeEnvHeadSelectionTarget{}, err
	}
	count, err := reader.readUint32("target extension count")
	if err != nil {
		return ProjectTypeEnvHeadSelectionTarget{}, err
	}
	if count > maximumOrderedExtensions {
		return ProjectTypeEnvHeadSelectionTarget{}, fmt.Errorf("target extension count is too large")
	}
	extensions := make([]typedmemory.TypeEnvExtensionRef, 0, count)
	for index := uint32(0); index < count; index++ {
		extensionText, readErr := reader.readString("target extension")
		if readErr != nil {
			return ProjectTypeEnvHeadSelectionTarget{}, readErr
		}
		extension, parseErr := typedmemory.ParseTypeEnvExtensionRef(extensionText)
		if parseErr != nil {
			return ProjectTypeEnvHeadSelectionTarget{}, parseErr
		}
		extensions = append(extensions, extension)
	}
	runtimeText, err := reader.readString("target runtime basis")
	if err != nil {
		return ProjectTypeEnvHeadSelectionTarget{}, err
	}
	runtimeBasis, err := projecttypeenv.ParseRuntimeEvaluationBasisRef(runtimeText)
	if err != nil {
		return ProjectTypeEnvHeadSelectionTarget{}, err
	}
	compositeText, err := reader.readString("target composite")
	if err != nil {
		return ProjectTypeEnvHeadSelectionTarget{}, err
	}
	composite, err := typedmemory.ParseTypeEnvRef(compositeText)
	if err != nil {
		return ProjectTypeEnvHeadSelectionTarget{}, err
	}
	stageText, err := reader.readString("target Stage")
	if err != nil {
		return ProjectTypeEnvHeadSelectionTarget{}, err
	}
	stage, err := projecttypeenvselection.ParseProjectTypeEnvStageRef(stageText)
	if err != nil {
		return ProjectTypeEnvHeadSelectionTarget{}, err
	}
	return NewProjectTypeEnvHeadSelectionTarget(
		ProjectTypeEnvHeadSelectionTargetInput{
			Base:              base,
			OrderedExtensions: extensions,
			RuntimeBasis:      runtimeBasis,
			Composite:         composite,
			Stage:             stage,
		},
	)
}

func normalizeProject(
	project projectidentity.ProjectID,
) (projectidentity.ProjectID, error) {
	parsed, err := projectidentity.ParseProjectID(project.String())
	if err != nil || parsed != project {
		return projectidentity.ProjectID{}, fmt.Errorf("project identity is required")
	}
	return parsed, nil
}

func normalizeRequestRef(
	ref projecttypeenvselection.ProjectTypeEnvHeadSelectionRequestRef,
	digest typedmemory.SHA256Digest,
) (projecttypeenvselection.ProjectTypeEnvHeadSelectionRequestRef, error) {
	parsed, err := projecttypeenvselection.ParseProjectTypeEnvHeadSelectionRequestRef(
		ref.String(),
	)
	if err != nil || parsed != ref || ref.Digest() != digest {
		return projecttypeenvselection.ProjectTypeEnvHeadSelectionRequestRef{},
			fmt.Errorf("request ref and digest must match")
	}
	return parsed, nil
}

func encodePredecessor(
	writer *canonicalWriter,
	predecessor projecttypeenvselection.ProjectTypeEnvHeadSelectionPredecessor,
) {
	switch value := predecessor.(type) {
	case projecttypeenvselection.GenesisStagePredecessor:
		writer.writeString("genesis")
	case projecttypeenvselection.TransitionStagePredecessor:
		writer.writeString("transition")
		writer.writeString(value.Project().String())
		writer.writeString(value.Head().String())
		writer.writeUint64(value.HeadRevision().Value())
		writer.writeString(value.SelectedComposite().String())
	}
}

func decodePredecessor(
	reader *canonicalReader,
	project projectidentity.ProjectID,
) (projecttypeenvselection.ProjectTypeEnvHeadSelectionPredecessor, error) {
	kind, err := reader.readString("predecessor kind")
	if err != nil {
		return nil, err
	}
	switch kind {
	case "genesis":
		// Migration-47 effect carriers embedded the request-owned absence
		// proof immediately after the Genesis tag. Current carriers bind the
		// effect-owned proof through the CAS comparison and database closure
		// instead. Consume the old coordinate only while decoding historical
		// bytes; never project it back into the current predecessor.
		checkpoint := reader.offset
		proofText, readErr := reader.readString("legacy Genesis proof")
		if readErr == nil {
			_, parseErr := projecttypeenvselection.ParseNoPriorHeadProofRef(
				proofText,
			)
			if parseErr == nil {
				return projecttypeenvselection.NewGenesisStagePredecessor(), nil
			}
		}
		reader.offset = checkpoint
		return projecttypeenvselection.NewGenesisStagePredecessor(), nil
	case "transition":
		projectText, readErr := reader.readString("Transition project")
		if readErr != nil {
			return nil, readErr
		}
		priorProject, parseErr := projectidentity.ParseProjectID(projectText)
		if parseErr != nil || priorProject != project {
			return nil, fmt.Errorf("transition predecessor project mismatch")
		}
		headText, readErr := reader.readString("Transition head")
		if readErr != nil {
			return nil, readErr
		}
		head, parseErr := projecttypeenvselection.ParseProjectTypeEnvHeadRef(headText)
		if parseErr != nil {
			return nil, parseErr
		}
		revisionValue, readErr := reader.readUint64("Transition HeadRevision")
		if readErr != nil {
			return nil, readErr
		}
		revision, parseErr := projecttypeenvselection.NewHeadRevision(revisionValue)
		if parseErr != nil {
			return nil, parseErr
		}
		compositeText, readErr := reader.readString("Transition composite")
		if readErr != nil {
			return nil, readErr
		}
		composite, parseErr := typedmemory.ParseTypeEnvRef(compositeText)
		if parseErr != nil {
			return nil, parseErr
		}
		return projecttypeenvselection.NewTransitionStagePredecessor(
			projecttypeenvselection.TransitionStagePredecessorInput{
				Project:           project,
				Head:              head,
				HeadRevision:      revision,
				SelectedComposite: composite,
			},
		)
	default:
		return nil, fmt.Errorf("head-selection predecessor kind is invalid")
	}
}

// ProjectTypeEnvHeadSelectionAuthorityCoordinatesKind is the durable
// discriminator for the two non-coercible authority source/resolution
// variants. It is not authority itself.
type ProjectTypeEnvHeadSelectionAuthorityCoordinatesKind uint8

const (
	ProjectTypeEnvHeadSelectionAuthorityCoordinatesTrustedDedicatedCLI ProjectTypeEnvHeadSelectionAuthorityCoordinatesKind = iota + 1
	ProjectTypeEnvHeadSelectionAuthorityCoordinatesVerifiedSpeechAct
)

func (kind ProjectTypeEnvHeadSelectionAuthorityCoordinatesKind) String() string {
	switch kind {
	case ProjectTypeEnvHeadSelectionAuthorityCoordinatesTrustedDedicatedCLI:
		return "trusted_dedicated_cli_invocation"
	case ProjectTypeEnvHeadSelectionAuthorityCoordinatesVerifiedSpeechAct:
		return "verified_speech_act"
	default:
		return ""
	}
}

type ProjectTypeEnvHeadSelectionAuthorityCommonInput struct {
	ContentRef        authority.DescriptionRef
	ContentDigest     authority.Digest
	PolicyRef         projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionModePolicyRef
	PolicyDigest      authority.Digest
	ConfigBasisRef    projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionConfigAuthorityBasisRef
	ConfigBasisDigest authority.Digest
	ExecutionSubject  projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionPermissionSubject
}

type projectTypeEnvHeadSelectionAuthorityCommon struct {
	contentRef        authority.DescriptionRef
	contentDigest     authority.Digest
	policyRef         projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionModePolicyRef
	policyDigest      authority.Digest
	configBasisRef    projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionConfigAuthorityBasisRef
	configBasisDigest authority.Digest
	executionSubject  projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionPermissionSubject
}

// TrustedDedicatedCLIAuthorityCoordinates records the exact lower-assurance
// dedicated-CLI source and the configured-policy acceptance resolution. It
// contains no SpeechAct, Permission, Work, or human-act receipt.
type TrustedDedicatedCLIAuthorityCoordinates struct {
	sourceRef        projecttypeenvselectionauthority.TrustedDedicatedCLIInvocationSourceRecordRef
	sourceDigest     authority.Digest
	resolutionRef    projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorityResolutionRef
	resolutionDigest authority.Digest
}

type TrustedDedicatedCLIAuthorityCoordinatesInput struct {
	Common           ProjectTypeEnvHeadSelectionAuthorityCommonInput
	SourceRef        projecttypeenvselectionauthority.TrustedDedicatedCLIInvocationSourceRecordRef
	SourceDigest     authority.Digest
	ResolutionRef    projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorityResolutionRef
	ResolutionDigest authority.Digest
}

func (value TrustedDedicatedCLIAuthorityCoordinates) SourceRef() projecttypeenvselectionauthority.TrustedDedicatedCLIInvocationSourceRecordRef {
	return value.sourceRef
}

func (value TrustedDedicatedCLIAuthorityCoordinates) SourceDigest() authority.Digest {
	return value.sourceDigest
}

func (value TrustedDedicatedCLIAuthorityCoordinates) AuthorityResolutionRef() projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorityResolutionRef {
	return value.resolutionRef
}

func (value TrustedDedicatedCLIAuthorityCoordinates) AuthorityResolutionDigest() authority.Digest {
	return value.resolutionDigest
}

// VerifiedSpeechActAuthorityCoordinates records the exact strict durable
// SpeechAct, instituted Permission, and authority resolution. It cannot be
// inhabited by a dedicated-CLI policy-acceptance source.
type VerifiedSpeechActAuthorityCoordinates struct {
	speechActRef           authority.SpeechActRef
	speechActWorkRef       authority.WorkRef
	speechActRecordRef     projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionSpeechActRecordRef
	speechActRecordDigest  authority.Digest
	permissionRef          projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionPermissionRef
	permissionDigest       authority.Digest
	authorityResolutionRef projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorityResolutionRef
	authorityResolutionDig authority.Digest
}

type VerifiedSpeechActAuthorityCoordinatesInput struct {
	Common                    ProjectTypeEnvHeadSelectionAuthorityCommonInput
	SpeechActRef              authority.SpeechActRef
	SpeechActWorkRef          authority.WorkRef
	SpeechActRecordRef        projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionSpeechActRecordRef
	SpeechActRecordDigest     authority.Digest
	PermissionRef             projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionPermissionRef
	PermissionDigest          authority.Digest
	AuthorityResolutionRef    projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorityResolutionRef
	AuthorityResolutionDigest authority.Digest
}

func (value VerifiedSpeechActAuthorityCoordinates) SpeechActRef() authority.SpeechActRef {
	return value.speechActRef
}

func (value VerifiedSpeechActAuthorityCoordinates) SpeechActWorkRef() authority.WorkRef {
	return value.speechActWorkRef
}

func (value VerifiedSpeechActAuthorityCoordinates) SpeechActRecordRef() projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionSpeechActRecordRef {
	return value.speechActRecordRef
}

func (value VerifiedSpeechActAuthorityCoordinates) SpeechActRecordDigest() authority.Digest {
	return value.speechActRecordDigest
}

func (value VerifiedSpeechActAuthorityCoordinates) PermissionRef() projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionPermissionRef {
	return value.permissionRef
}

func (value VerifiedSpeechActAuthorityCoordinates) PermissionDigest() authority.Digest {
	return value.permissionDigest
}

func (value VerifiedSpeechActAuthorityCoordinates) AuthorityResolutionRef() projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorityResolutionRef {
	return value.authorityResolutionRef
}

func (value VerifiedSpeechActAuthorityCoordinates) AuthorityResolutionDigest() authority.Digest {
	return value.authorityResolutionDig
}

type projectTypeEnvHeadSelectionAuthorityCoordinatesVariant interface {
	projectTypeEnvHeadSelectionAuthorityCoordinatesVariant()
}

func (TrustedDedicatedCLIAuthorityCoordinates) projectTypeEnvHeadSelectionAuthorityCoordinatesVariant() {
}

func (VerifiedSpeechActAuthorityCoordinates) projectTypeEnvHeadSelectionAuthorityCoordinatesVariant() {
}

// ProjectTypeEnvHeadSelectionAuthorityCoordinates is a closed durable sum.
// It grants no authority and records no use. Only the live authority-basis
// wrapper can authorize the original effect branch.
type ProjectTypeEnvHeadSelectionAuthorityCoordinates struct {
	common  projectTypeEnvHeadSelectionAuthorityCommon
	variant projectTypeEnvHeadSelectionAuthorityCoordinatesVariant
}

func NewTrustedDedicatedCLIAuthorityCoordinates(
	input TrustedDedicatedCLIAuthorityCoordinatesInput,
) (ProjectTypeEnvHeadSelectionAuthorityCoordinates, error) {
	common, err := normalizeAuthorityCommon(input.Common)
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityCoordinates{}, err
	}
	sourceRef, err :=
		projecttypeenvselectionauthority.ParseTrustedDedicatedCLIInvocationSourceRecordRef(
			input.SourceRef.String(),
		)
	if err != nil ||
		sourceRef != input.SourceRef ||
		sourceRef.Digest() != input.SourceDigest {
		return ProjectTypeEnvHeadSelectionAuthorityCoordinates{},
			fmt.Errorf("trusted dedicated CLI source ref and digest must match")
	}
	resolutionRef, err :=
		projecttypeenvselectionauthority.ParseProjectTypeEnvHeadSelectionAuthorityResolutionRef(
			input.ResolutionRef.String(),
		)
	if err != nil ||
		resolutionRef != input.ResolutionRef ||
		resolutionRef.Digest() != input.ResolutionDigest {
		return ProjectTypeEnvHeadSelectionAuthorityCoordinates{},
			fmt.Errorf("explicit policy-acceptance resolution ref and digest must match")
	}
	return ProjectTypeEnvHeadSelectionAuthorityCoordinates{
		common: common,
		variant: TrustedDedicatedCLIAuthorityCoordinates{
			sourceRef:        sourceRef,
			sourceDigest:     input.SourceDigest,
			resolutionRef:    resolutionRef,
			resolutionDigest: input.ResolutionDigest,
		},
	}, nil
}

func NewVerifiedSpeechActAuthorityCoordinates(
	input VerifiedSpeechActAuthorityCoordinatesInput,
) (ProjectTypeEnvHeadSelectionAuthorityCoordinates, error) {
	common, err := normalizeAuthorityCommon(input.Common)
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityCoordinates{}, err
	}
	speechActRef, err := authority.NewSpeechActRef(input.SpeechActRef.String())
	if err != nil || speechActRef != input.SpeechActRef {
		return ProjectTypeEnvHeadSelectionAuthorityCoordinates{},
			fmt.Errorf("SpeechAct occurrence ref is required")
	}
	speechActWorkRef, err := authority.NewWorkRef(input.SpeechActWorkRef.String())
	if err != nil || speechActWorkRef != input.SpeechActWorkRef {
		return ProjectTypeEnvHeadSelectionAuthorityCoordinates{},
			fmt.Errorf("SpeechAct Work ref is required")
	}
	speechActRecordRef, err :=
		projecttypeenvselectionauthority.ParseProjectTypeEnvHeadSelectionSpeechActRecordRef(
			input.SpeechActRecordRef.String(),
		)
	if err != nil ||
		speechActRecordRef != input.SpeechActRecordRef ||
		speechActRecordRef.Digest() != input.SpeechActRecordDigest {
		return ProjectTypeEnvHeadSelectionAuthorityCoordinates{},
			fmt.Errorf("SpeechAct record ref and digest must match")
	}
	permissionRef, err :=
		projecttypeenvselectionauthority.ParseProjectTypeEnvHeadSelectionPermissionRef(
			input.PermissionRef.String(),
		)
	if err != nil ||
		permissionRef != input.PermissionRef ||
		input.PermissionDigest.String() == "" {
		return ProjectTypeEnvHeadSelectionAuthorityCoordinates{},
			fmt.Errorf("permission ref and digest are required")
	}
	permissionDigest, err := authority.NewDigest(input.PermissionDigest.String())
	if err != nil || permissionDigest != input.PermissionDigest {
		return ProjectTypeEnvHeadSelectionAuthorityCoordinates{},
			fmt.Errorf("permission digest is invalid")
	}
	authorityResolutionRef, err :=
		projecttypeenvselectionauthority.ParseProjectTypeEnvHeadSelectionAuthorityResolutionRef(
			input.AuthorityResolutionRef.String(),
		)
	if err != nil ||
		authorityResolutionRef != input.AuthorityResolutionRef ||
		authorityResolutionRef.Digest() != input.AuthorityResolutionDigest {
		return ProjectTypeEnvHeadSelectionAuthorityCoordinates{},
			fmt.Errorf("authority resolution ref and digest must match")
	}
	return ProjectTypeEnvHeadSelectionAuthorityCoordinates{
		common: common,
		variant: VerifiedSpeechActAuthorityCoordinates{
			speechActRef:           speechActRef,
			speechActWorkRef:       speechActWorkRef,
			speechActRecordRef:     speechActRecordRef,
			speechActRecordDigest:  input.SpeechActRecordDigest,
			permissionRef:          permissionRef,
			permissionDigest:       permissionDigest,
			authorityResolutionRef: authorityResolutionRef,
			authorityResolutionDig: input.AuthorityResolutionDigest,
		},
	}, nil
}

// ProjectTypeEnvHeadSelectionAuthorityCoordinatesFromResolution projects
// durable coordinates from one exact durable resolution. It never accepts or
// recreates the service-private live authority use.
func ProjectTypeEnvHeadSelectionAuthorityCoordinatesFromResolution(
	policy projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorityPolicyRecord,
	record projecttypeenvselectionauthority.AuthorityResolutionRecord,
) (ProjectTypeEnvHeadSelectionAuthorityCoordinates, error) {
	explicit, explicitOK := record.ExplicitPolicyAcceptance()
	if explicitOK {
		source := explicit.Source()
		coordinates := source.Coordinates()
		common, err := authorityCommonFromPolicyAndContent(
			policy,
			source.Content(),
		)
		if err != nil {
			return ProjectTypeEnvHeadSelectionAuthorityCoordinates{}, err
		}
		if coordinates.PolicyRef() != policy.Ref() ||
			coordinates.PolicyDigest() != policy.Digest() ||
			coordinates.ConfigBasisRef() != policy.ConfigBasis().Ref() ||
			coordinates.ConfigBasisDigest() != policy.ConfigBasis().Digest() {
			return ProjectTypeEnvHeadSelectionAuthorityCoordinates{},
				fmt.Errorf("explicit authority resolution differs from current mode policy")
		}
		return NewTrustedDedicatedCLIAuthorityCoordinates(
			TrustedDedicatedCLIAuthorityCoordinatesInput{
				Common:           common,
				SourceRef:        source.Ref(),
				SourceDigest:     source.Digest(),
				ResolutionRef:    explicit.Ref(),
				ResolutionDigest: explicit.Digest(),
			},
		)
	}
	strict, strictOK := record.StrictPermission()
	if !strictOK {
		return ProjectTypeEnvHeadSelectionAuthorityCoordinates{},
			fmt.Errorf("head-selection authority resolution variant is unavailable")
	}
	basis := strict.Basis()
	strictPolicy, strictPolicyOK := policy.StrictCLISpeechAct()
	if !strictPolicyOK ||
		strictPolicy.ResolverPolicy().Ref() != basis.Policy().Ref() ||
		strictPolicy.ResolverPolicy().Digest() != basis.Policy().Digest() {
		return ProjectTypeEnvHeadSelectionAuthorityCoordinates{},
			fmt.Errorf("strict authority resolution differs from current mode policy")
	}
	content := basis.Content()
	common, err := authorityCommonFromPolicyAndContent(policy, content)
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityCoordinates{}, err
	}
	permission := strict.Permission()
	if err := permission.Subject().VerifyCurrentForUse(content); err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityCoordinates{},
			fmt.Errorf("strict Permission execution subject: %w", err)
	}
	if !executionSubjectsEqual(
		common.ExecutionSubject,
		permission.Subject(),
	) {
		return ProjectTypeEnvHeadSelectionAuthorityCoordinates{},
			fmt.Errorf("strict Permission names another execution subject")
	}
	sourceRecord := strict.Source().Record()
	verifiedSource := sourceRecord.Source()
	speechAct, speechActOK := verifiedSource.SpeechActRef()
	speechWork, speechWorkOK := verifiedSource.WorkRef()
	if !speechActOK || !speechWorkOK {
		return ProjectTypeEnvHeadSelectionAuthorityCoordinates{},
			fmt.Errorf("strict SpeechAct source coordinates are unavailable")
	}
	return NewVerifiedSpeechActAuthorityCoordinates(
		VerifiedSpeechActAuthorityCoordinatesInput{
			Common:                    common,
			SpeechActRef:              speechAct,
			SpeechActWorkRef:          speechWork,
			SpeechActRecordRef:        sourceRecord.Ref(),
			SpeechActRecordDigest:     sourceRecord.Digest(),
			PermissionRef:             permission.Ref(),
			PermissionDigest:          permission.Digest(),
			AuthorityResolutionRef:    strict.Ref(),
			AuthorityResolutionDigest: strict.Digest(),
		},
	)
}

func authorityCommonFromPolicy(
	policy projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorityPolicyRecord,
) (ProjectTypeEnvHeadSelectionAuthorityCommonInput, error) {
	configBasis := policy.ConfigBasis()
	if err := configBasis.Verify(
		policy.Project(),
		policy.Mode(),
		policy.ConfigCarrier(),
	); err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityCommonInput{}, err
	}
	var contentRef authority.DescriptionRef
	var contentDigest authority.Digest
	return ProjectTypeEnvHeadSelectionAuthorityCommonInput{
		ContentRef:        contentRef,
		ContentDigest:     contentDigest,
		PolicyRef:         policy.Ref(),
		PolicyDigest:      policy.Digest(),
		ConfigBasisRef:    configBasis.Ref(),
		ConfigBasisDigest: configBasis.Digest(),
	}, nil
}

func authorityCommonFromPolicyAndContent(
	policy projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorityPolicyRecord,
	content projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorizationContent,
) (ProjectTypeEnvHeadSelectionAuthorityCommonInput, error) {
	common, err := authorityCommonFromPolicy(policy)
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityCommonInput{}, err
	}
	binding := policy.ProjectBinding()
	if content.Project() != binding.Project() {
		return ProjectTypeEnvHeadSelectionAuthorityCommonInput{},
			fmt.Errorf("authorization content project differs from policy project binding")
	}
	if content.JudgementContext() != binding.Context() {
		return ProjectTypeEnvHeadSelectionAuthorityCommonInput{},
			fmt.Errorf(
				"authorization content judgement context differs from policy project binding",
			)
	}
	subject, err :=
		projecttypeenvselectionauthority.SealProjectTypeEnvHeadSelectionPermissionSubject(
			content,
		)
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityCommonInput{}, err
	}
	return ProjectTypeEnvHeadSelectionAuthorityCommonInput{
		ContentRef:        content.DescriptionRef(),
		ContentDigest:     content.Digest(),
		PolicyRef:         common.PolicyRef,
		PolicyDigest:      common.PolicyDigest,
		ConfigBasisRef:    common.ConfigBasisRef,
		ConfigBasisDigest: common.ConfigBasisDigest,
		ExecutionSubject:  subject,
	}, nil
}

func (value ProjectTypeEnvHeadSelectionAuthorityCoordinates) Kind() ProjectTypeEnvHeadSelectionAuthorityCoordinatesKind {
	switch value.variant.(type) {
	case TrustedDedicatedCLIAuthorityCoordinates:
		return ProjectTypeEnvHeadSelectionAuthorityCoordinatesTrustedDedicatedCLI
	case VerifiedSpeechActAuthorityCoordinates:
		return ProjectTypeEnvHeadSelectionAuthorityCoordinatesVerifiedSpeechAct
	default:
		return 0
	}
}

func (value ProjectTypeEnvHeadSelectionAuthorityCoordinates) ContentRef() authority.DescriptionRef {
	return value.common.contentRef
}

func (value ProjectTypeEnvHeadSelectionAuthorityCoordinates) ContentDigest() authority.Digest {
	return value.common.contentDigest
}

func (value ProjectTypeEnvHeadSelectionAuthorityCoordinates) PolicyRef() projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionModePolicyRef {
	return value.common.policyRef
}

func (value ProjectTypeEnvHeadSelectionAuthorityCoordinates) PolicyDigest() authority.Digest {
	return value.common.policyDigest
}

func (value ProjectTypeEnvHeadSelectionAuthorityCoordinates) ConfigBasisRef() projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionConfigAuthorityBasisRef {
	return value.common.configBasisRef
}

func (value ProjectTypeEnvHeadSelectionAuthorityCoordinates) ConfigBasisDigest() authority.Digest {
	return value.common.configBasisDigest
}

func (value ProjectTypeEnvHeadSelectionAuthorityCoordinates) ExecutionSubject() projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionPermissionSubject {
	return value.common.executionSubject
}

func (value ProjectTypeEnvHeadSelectionAuthorityCoordinates) ExactEqual(
	other ProjectTypeEnvHeadSelectionAuthorityCoordinates,
) bool {
	if value.Kind() != other.Kind() ||
		value.ContentRef() != other.ContentRef() ||
		value.ContentDigest() != other.ContentDigest() ||
		value.PolicyRef() != other.PolicyRef() ||
		value.PolicyDigest() != other.PolicyDigest() ||
		value.ConfigBasisRef() != other.ConfigBasisRef() ||
		value.ConfigBasisDigest() != other.ConfigBasisDigest() {
		return false
	}
	leftSubject := value.ExecutionSubject()
	rightSubject := other.ExecutionSubject()
	if leftSubject.Ref() != rightSubject.Ref() ||
		leftSubject.Digest() != rightSubject.Digest() {
		return false
	}
	if left, ok := value.TrustedDedicatedCLIInvocation(); ok {
		right, rightOK := other.TrustedDedicatedCLIInvocation()
		return rightOK &&
			left.SourceRef() == right.SourceRef() &&
			left.SourceDigest() == right.SourceDigest() &&
			left.AuthorityResolutionRef() == right.AuthorityResolutionRef() &&
			left.AuthorityResolutionDigest() == right.AuthorityResolutionDigest()
	}
	left, ok := value.VerifiedSpeechAct()
	if !ok {
		return false
	}
	right, rightOK := other.VerifiedSpeechAct()
	return rightOK &&
		left.SpeechActRef() == right.SpeechActRef() &&
		left.SpeechActWorkRef() == right.SpeechActWorkRef() &&
		left.SpeechActRecordRef() == right.SpeechActRecordRef() &&
		left.SpeechActRecordDigest() == right.SpeechActRecordDigest() &&
		left.PermissionRef() == right.PermissionRef() &&
		left.PermissionDigest() == right.PermissionDigest() &&
		left.AuthorityResolutionRef() == right.AuthorityResolutionRef() &&
		left.AuthorityResolutionDigest() == right.AuthorityResolutionDigest()
}

func (value ProjectTypeEnvHeadSelectionAuthorityCoordinates) TrustedDedicatedCLIInvocation() (
	TrustedDedicatedCLIAuthorityCoordinates,
	bool,
) {
	coordinates, ok := value.variant.(TrustedDedicatedCLIAuthorityCoordinates)
	return coordinates, ok
}

func (value ProjectTypeEnvHeadSelectionAuthorityCoordinates) VerifiedSpeechAct() (
	VerifiedSpeechActAuthorityCoordinates,
	bool,
) {
	coordinates, ok := value.variant.(VerifiedSpeechActAuthorityCoordinates)
	return coordinates, ok
}

func encodeAuthorityCoordinates(
	writer *canonicalWriter,
	value ProjectTypeEnvHeadSelectionAuthorityCoordinates,
) {
	writer.writeString(value.Kind().String())
	writer.writeString(string(value.common.contentRef.Kind()))
	writer.writeString(value.common.contentRef.String())
	writer.writeString(value.common.contentDigest.String())
	writer.writeString(value.common.policyRef.String())
	writer.writeString(value.common.policyDigest.String())
	writer.writeString(value.common.configBasisRef.String())
	writer.writeString(value.common.configBasisDigest.String())
	writer.writeBytes(value.common.executionSubject.CanonicalJSON())
	switch coordinates := value.variant.(type) {
	case TrustedDedicatedCLIAuthorityCoordinates:
		writer.writeString(coordinates.sourceRef.String())
		writer.writeString(coordinates.sourceDigest.String())
		writer.writeString(coordinates.resolutionRef.String())
		writer.writeString(coordinates.resolutionDigest.String())
	case VerifiedSpeechActAuthorityCoordinates:
		writer.writeString(coordinates.speechActRef.String())
		writer.writeString(coordinates.speechActWorkRef.String())
		writer.writeString(coordinates.speechActRecordRef.String())
		writer.writeString(coordinates.speechActRecordDigest.String())
		writer.writeString(coordinates.permissionRef.String())
		writer.writeString(coordinates.permissionDigest.String())
		writer.writeString(coordinates.authorityResolutionRef.String())
		writer.writeString(coordinates.authorityResolutionDig.String())
	}
}

func decodeAuthorityCoordinates(
	reader *canonicalReader,
) (ProjectTypeEnvHeadSelectionAuthorityCoordinates, error) {
	kind, common, err := decodeAuthorityCommon(reader)
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityCoordinates{}, err
	}
	switch kind {
	case ProjectTypeEnvHeadSelectionAuthorityCoordinatesTrustedDedicatedCLI:
		return decodeTrustedDedicatedCLIAuthorityCoordinates(reader, common)
	case ProjectTypeEnvHeadSelectionAuthorityCoordinatesVerifiedSpeechAct:
		return decodeVerifiedSpeechActAuthorityCoordinates(reader, common)
	default:
		return ProjectTypeEnvHeadSelectionAuthorityCoordinates{},
			fmt.Errorf("head-selection authority coordinates kind is invalid")
	}
}

func decodeAuthorityCommon(
	reader *canonicalReader,
) (
	ProjectTypeEnvHeadSelectionAuthorityCoordinatesKind,
	ProjectTypeEnvHeadSelectionAuthorityCommonInput,
	error,
) {
	kindText, err := reader.readString("authority coordinates kind")
	if err != nil {
		return 0, ProjectTypeEnvHeadSelectionAuthorityCommonInput{}, err
	}
	kind, err := parseAuthorityCoordinatesKind(kindText)
	if err != nil {
		return 0, ProjectTypeEnvHeadSelectionAuthorityCommonInput{}, err
	}
	contentKind, err := reader.readString("authorization-content ref kind")
	if err != nil {
		return 0, ProjectTypeEnvHeadSelectionAuthorityCommonInput{}, err
	}
	contentText, err := reader.readString("authorization-content ref")
	if err != nil {
		return 0, ProjectTypeEnvHeadSelectionAuthorityCommonInput{}, err
	}
	contentRef, err := parseDescriptionRef(
		authority.DescriptionRefKind(contentKind),
		contentText,
	)
	if err != nil {
		return 0, ProjectTypeEnvHeadSelectionAuthorityCommonInput{}, err
	}
	contentDigest, err := readAuthorityDigest(reader, "authorization-content digest")
	if err != nil {
		return 0, ProjectTypeEnvHeadSelectionAuthorityCommonInput{}, err
	}
	policyText, err := reader.readString("authority mode-policy ref")
	if err != nil {
		return 0, ProjectTypeEnvHeadSelectionAuthorityCommonInput{}, err
	}
	policyRef, err :=
		projecttypeenvselectionauthority.ParseProjectTypeEnvHeadSelectionModePolicyRef(
			policyText,
		)
	if err != nil {
		return 0, ProjectTypeEnvHeadSelectionAuthorityCommonInput{}, err
	}
	policyDigest, err := readAuthorityDigest(reader, "authority mode-policy digest")
	if err != nil {
		return 0, ProjectTypeEnvHeadSelectionAuthorityCommonInput{}, err
	}
	configText, err := reader.readString("config authority-basis ref")
	if err != nil {
		return 0, ProjectTypeEnvHeadSelectionAuthorityCommonInput{}, err
	}
	configRef, err :=
		projecttypeenvselectionauthority.ParseProjectTypeEnvHeadSelectionConfigAuthorityBasisRef(
			configText,
		)
	if err != nil {
		return 0, ProjectTypeEnvHeadSelectionAuthorityCommonInput{}, err
	}
	configDigest, err := readAuthorityDigest(reader, "config authority-basis digest")
	if err != nil {
		return 0, ProjectTypeEnvHeadSelectionAuthorityCommonInput{}, err
	}
	subjectBytes, err := reader.readBytes("head-selection execution subject")
	if err != nil {
		return 0, ProjectTypeEnvHeadSelectionAuthorityCommonInput{}, err
	}
	subject, err :=
		projecttypeenvselectionauthority.DecodeProjectTypeEnvHeadSelectionPermissionSubject(
			subjectBytes,
		)
	if err != nil {
		return 0, ProjectTypeEnvHeadSelectionAuthorityCommonInput{}, err
	}
	return kind, ProjectTypeEnvHeadSelectionAuthorityCommonInput{
		ContentRef:        contentRef,
		ContentDigest:     contentDigest,
		PolicyRef:         policyRef,
		PolicyDigest:      policyDigest,
		ConfigBasisRef:    configRef,
		ConfigBasisDigest: configDigest,
		ExecutionSubject:  subject,
	}, nil
}

func decodeTrustedDedicatedCLIAuthorityCoordinates(
	reader *canonicalReader,
	common ProjectTypeEnvHeadSelectionAuthorityCommonInput,
) (ProjectTypeEnvHeadSelectionAuthorityCoordinates, error) {
	sourceText, err := reader.readString("trusted dedicated CLI source ref")
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityCoordinates{}, err
	}
	sourceRef, err :=
		projecttypeenvselectionauthority.ParseTrustedDedicatedCLIInvocationSourceRecordRef(
			sourceText,
		)
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityCoordinates{}, err
	}
	sourceDigest, err := readAuthorityDigest(
		reader,
		"trusted dedicated CLI source digest",
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityCoordinates{}, err
	}
	resolutionText, err := reader.readString(
		"explicit policy-acceptance resolution ref",
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityCoordinates{}, err
	}
	resolutionRef, err :=
		projecttypeenvselectionauthority.ParseProjectTypeEnvHeadSelectionAuthorityResolutionRef(
			resolutionText,
		)
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityCoordinates{}, err
	}
	resolutionDigest, err := readAuthorityDigest(
		reader,
		"explicit policy-acceptance resolution digest",
	)
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityCoordinates{}, err
	}
	return NewTrustedDedicatedCLIAuthorityCoordinates(
		TrustedDedicatedCLIAuthorityCoordinatesInput{
			Common:           common,
			SourceRef:        sourceRef,
			SourceDigest:     sourceDigest,
			ResolutionRef:    resolutionRef,
			ResolutionDigest: resolutionDigest,
		},
	)
}

func decodeVerifiedSpeechActAuthorityCoordinates(
	reader *canonicalReader,
	common ProjectTypeEnvHeadSelectionAuthorityCommonInput,
) (ProjectTypeEnvHeadSelectionAuthorityCoordinates, error) {
	speechActText, err := reader.readString("SpeechAct ref")
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityCoordinates{}, err
	}
	speechActRef, err := authority.NewSpeechActRef(speechActText)
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityCoordinates{}, err
	}
	speechActWorkText, err := reader.readString("SpeechAct Work ref")
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityCoordinates{}, err
	}
	speechActWorkRef, err := authority.NewWorkRef(speechActWorkText)
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityCoordinates{}, err
	}
	recordText, err := reader.readString("SpeechAct record ref")
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityCoordinates{}, err
	}
	recordRef, err :=
		projecttypeenvselectionauthority.ParseProjectTypeEnvHeadSelectionSpeechActRecordRef(
			recordText,
		)
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityCoordinates{}, err
	}
	recordDigest, err := readAuthorityDigest(reader, "SpeechAct record digest")
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityCoordinates{}, err
	}
	permissionText, err := reader.readString("Permission ref")
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityCoordinates{}, err
	}
	permissionRef, err :=
		projecttypeenvselectionauthority.ParseProjectTypeEnvHeadSelectionPermissionRef(
			permissionText,
		)
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityCoordinates{}, err
	}
	permissionDigest, err := readAuthorityDigest(reader, "Permission digest")
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityCoordinates{}, err
	}
	resolutionText, err := reader.readString("authority resolution ref")
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityCoordinates{}, err
	}
	resolutionRef, err :=
		projecttypeenvselectionauthority.ParseProjectTypeEnvHeadSelectionAuthorityResolutionRef(
			resolutionText,
		)
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityCoordinates{}, err
	}
	resolutionDigest, err := readAuthorityDigest(reader, "authority resolution digest")
	if err != nil {
		return ProjectTypeEnvHeadSelectionAuthorityCoordinates{}, err
	}
	return NewVerifiedSpeechActAuthorityCoordinates(
		VerifiedSpeechActAuthorityCoordinatesInput{
			Common:                    common,
			SpeechActRef:              speechActRef,
			SpeechActWorkRef:          speechActWorkRef,
			SpeechActRecordRef:        recordRef,
			SpeechActRecordDigest:     recordDigest,
			PermissionRef:             permissionRef,
			PermissionDigest:          permissionDigest,
			AuthorityResolutionRef:    resolutionRef,
			AuthorityResolutionDigest: resolutionDigest,
		},
	)
}

func normalizeAuthorityCommon(
	input ProjectTypeEnvHeadSelectionAuthorityCommonInput,
) (projectTypeEnvHeadSelectionAuthorityCommon, error) {
	contentRef, err := normalizeDescriptionRef(input.ContentRef)
	if err != nil {
		return projectTypeEnvHeadSelectionAuthorityCommon{}, err
	}
	contentDigest, err := authority.NewDigest(input.ContentDigest.String())
	if err != nil || contentDigest != input.ContentDigest {
		return projectTypeEnvHeadSelectionAuthorityCommon{},
			fmt.Errorf("authorization-content digest is required")
	}
	policyRef, err :=
		projecttypeenvselectionauthority.ParseProjectTypeEnvHeadSelectionModePolicyRef(
			input.PolicyRef.String(),
		)
	if err != nil ||
		policyRef != input.PolicyRef ||
		policyRef.Digest() != input.PolicyDigest {
		return projectTypeEnvHeadSelectionAuthorityCommon{},
			fmt.Errorf("authority mode-policy ref and digest must match")
	}
	configRef, err :=
		projecttypeenvselectionauthority.ParseProjectTypeEnvHeadSelectionConfigAuthorityBasisRef(
			input.ConfigBasisRef.String(),
		)
	if err != nil ||
		configRef != input.ConfigBasisRef ||
		configRef.Digest() != input.ConfigBasisDigest {
		return projectTypeEnvHeadSelectionAuthorityCommon{},
			fmt.Errorf("config authority-basis ref and digest must match")
	}
	subject, err :=
		projecttypeenvselectionauthority.DecodeProjectTypeEnvHeadSelectionPermissionSubject(
			input.ExecutionSubject.CanonicalJSON(),
		)
	if err != nil || !executionSubjectsEqual(subject, input.ExecutionSubject) {
		return projectTypeEnvHeadSelectionAuthorityCommon{},
			fmt.Errorf("head-selection execution subject is invalid")
	}
	if subject.AuthorizationDescriptionRef() != contentRef ||
		subject.AuthorizationContentDigest() != contentDigest {
		return projectTypeEnvHeadSelectionAuthorityCommon{},
			fmt.Errorf(
				"head-selection execution subject names another authorization content",
			)
	}
	return projectTypeEnvHeadSelectionAuthorityCommon{
		contentRef:        contentRef,
		contentDigest:     contentDigest,
		policyRef:         policyRef,
		policyDigest:      input.PolicyDigest,
		configBasisRef:    configRef,
		configBasisDigest: input.ConfigBasisDigest,
		executionSubject:  subject,
	}, nil
}

func executionSubjectsEqual(
	left projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionPermissionSubject,
	right projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionPermissionSubject,
) bool {
	return left.Ref() == right.Ref() &&
		left.Digest() == right.Digest() &&
		left.Project() == right.Project() &&
		left.HolderSystemRef() == right.HolderSystemRef() &&
		left.RoleRef() == right.RoleRef() &&
		left.BoundedContext() == right.BoundedContext() &&
		left.AssignmentWindow().From().Equal(right.AssignmentWindow().From()) &&
		left.AssignmentWindow().Until().Equal(right.AssignmentWindow().Until()) &&
		bytes.Equal(left.CanonicalJSON(), right.CanonicalJSON())
}

func parseAuthorityCoordinatesKind(
	raw string,
) (ProjectTypeEnvHeadSelectionAuthorityCoordinatesKind, error) {
	switch raw {
	case ProjectTypeEnvHeadSelectionAuthorityCoordinatesTrustedDedicatedCLI.String():
		return ProjectTypeEnvHeadSelectionAuthorityCoordinatesTrustedDedicatedCLI, nil
	case ProjectTypeEnvHeadSelectionAuthorityCoordinatesVerifiedSpeechAct.String():
		return ProjectTypeEnvHeadSelectionAuthorityCoordinatesVerifiedSpeechAct, nil
	default:
		return 0, fmt.Errorf("head-selection authority coordinates kind is invalid")
	}
}

func readAuthorityDigest(
	reader *canonicalReader,
	label string,
) (authority.Digest, error) {
	raw, err := reader.readString(label)
	if err != nil {
		return authority.Digest{}, err
	}
	return authority.NewDigest(raw)
}

func normalizeDescriptionRef(
	value authority.DescriptionRef,
) (authority.DescriptionRef, error) {
	return parseDescriptionRef(value.Kind(), value.String())
}

func parseDescriptionRef(
	kind authority.DescriptionRefKind,
	raw string,
) (authority.DescriptionRef, error) {
	switch kind {
	case authority.DescriptionRefClaimID:
		return authority.NewClaimIDDescriptionRef(raw)
	case authority.DescriptionRefEpisteme:
		return authority.NewEpistemeDescriptionRef(raw)
	default:
		return authority.DescriptionRef{}, fmt.Errorf("DescriptionRef kind is invalid")
	}
}
