package initplanning

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/m0n0x41d/haft/internal/projectidentity"
)

var sha256DigestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

func validPermissionMode(mode fs.FileMode) bool {
	return mode != 0 && mode&^fs.FileMode(0o777) == 0
}

type BasisReadinessKind string

const (
	BasisUnavailable BasisReadinessKind = "unavailable"
	BasisSelected    BasisReadinessKind = "selected"
)

type BasisReadiness struct {
	kind         BasisReadinessKind
	reason       string
	headRef      string
	headRevision int64
	compositeRef string
}

func NewUnavailableBasis(reason string) (BasisReadiness, error) {
	validated, err := validateReason(reason, "unavailable basis reason")
	if err != nil {
		return BasisReadiness{}, err
	}
	return BasisReadiness{
		kind:   BasisUnavailable,
		reason: validated,
	}, nil
}

func NewSelectedBasis(
	headRef string,
	headRevision int64,
	compositeRef string,
) (BasisReadiness, error) {
	if headRef == "" || headRef != strings.TrimSpace(headRef) {
		return BasisReadiness{}, fmt.Errorf("selected basis head ref is required")
	}
	if headRevision <= 0 {
		return BasisReadiness{}, fmt.Errorf("selected basis head revision must be positive")
	}
	if compositeRef == "" || compositeRef != strings.TrimSpace(compositeRef) {
		return BasisReadiness{}, fmt.Errorf("selected basis composite ref is required")
	}
	return BasisReadiness{
		kind:         BasisSelected,
		headRef:      headRef,
		headRevision: headRevision,
		compositeRef: compositeRef,
	}, nil
}

func (basis BasisReadiness) Kind() BasisReadinessKind {
	return basis.kind
}

func (basis BasisReadiness) Reason() string {
	return basis.reason
}

func (basis BasisReadiness) HeadRef() string {
	return basis.headRef
}

func (basis BasisReadiness) HeadRevision() int64 {
	return basis.headRevision
}

func (basis BasisReadiness) CompositeRef() string {
	return basis.compositeRef
}

func (basis BasisReadiness) valid() bool {
	switch basis.kind {
	case BasisUnavailable:
		return basis.reason != "" &&
			basis.headRef == "" &&
			basis.headRevision == 0 &&
			basis.compositeRef == ""
	case BasisSelected:
		return basis.reason == "" &&
			basis.headRef != "" &&
			basis.headRevision > 0 &&
			basis.compositeRef != ""
	default:
		return false
	}
}

type CoreEffectKind string

const (
	CoreInitialize    CoreEffectKind = "initialize"
	CoreMigrate       CoreEffectKind = "migrate"
	CoreVerifyCurrent CoreEffectKind = "verify_current"
)

type CoreDatabaseSeedKind string

const (
	CoreDatabaseSeedEmpty      CoreDatabaseSeedKind = "empty"
	CoreDatabaseSeedLegacyCopy CoreDatabaseSeedKind = "legacy_copy"
)

type CoreRootMigrationKind string

const (
	CoreRootMigrationNone        CoreRootMigrationKind = "none"
	CoreRootMigrationQuintToHaft CoreRootMigrationKind = "quint_to_haft"
)

type CoreRootMigration struct {
	kind   CoreRootMigrationKind
	source string
	target string
}

func (migration CoreRootMigration) Kind() CoreRootMigrationKind {
	return migration.kind
}

func (migration CoreRootMigration) Source() string {
	return migration.source
}

func (migration CoreRootMigration) Target() string {
	return migration.target
}

func (migration CoreRootMigration) valid(
	projectRoot string,
) bool {
	switch migration.kind {
	case CoreRootMigrationNone:
		return migration.source == "" && migration.target == ""
	case CoreRootMigrationQuintToHaft:
		return migration.source ==
			filepath.Join(projectRoot, ".quint") &&
			migration.target ==
				filepath.Join(projectRoot, ".haft")
	default:
		return false
	}
}

type CoreDatabaseSeed struct {
	kind            CoreDatabaseSeedKind
	observationPath string
	sourcePath      string
	digest          string
}

type CoreFileEffectKind string

const (
	CoreFileCreate   CoreFileEffectKind = "create"
	CoreFilePreserve CoreFileEffectKind = "preserve"
	CoreFileReplace  CoreFileEffectKind = "replace"
)

type CoreFileEffect struct {
	kind           CoreFileEffectKind
	path           string
	content        []byte
	mode           fs.FileMode
	renderedDigest string
	expectedDigest string
	expectedMode   fs.FileMode
}

func NewCoreFileEffect(
	kind CoreFileEffectKind,
	path string,
	content []byte,
	mode fs.FileMode,
	renderedDigest string,
	expectedDigest string,
	expectedMode fs.FileMode,
) (CoreFileEffect, error) {
	if kind != CoreFileCreate &&
		kind != CoreFilePreserve &&
		kind != CoreFileReplace {
		return CoreFileEffect{}, fmt.Errorf(
			"core file effect kind is invalid",
		)
	}
	canonical, err := parseCanonicalAbsolutePath(path)
	if err != nil {
		return CoreFileEffect{}, fmt.Errorf(
			"core file effect path: %w",
			err,
		)
	}
	if !validPermissionMode(mode) ||
		!sha256DigestPattern.MatchString(renderedDigest) {
		return CoreFileEffect{}, fmt.Errorf(
			"core file rendered state is invalid",
		)
	}
	hasExpected := expectedDigest != ""
	if hasExpected != (expectedMode != 0) {
		return CoreFileEffect{}, fmt.Errorf(
			"core file predecessor state is incomplete",
		)
	}
	if hasExpected &&
		(!sha256DigestPattern.MatchString(expectedDigest) ||
			!validPermissionMode(expectedMode)) {
		return CoreFileEffect{}, fmt.Errorf(
			"core file predecessor state is invalid",
		)
	}
	if kind == CoreFileCreate && hasExpected {
		return CoreFileEffect{}, fmt.Errorf(
			"core file create cannot have a predecessor",
		)
	}
	if kind != CoreFileCreate && !hasExpected {
		return CoreFileEffect{}, fmt.Errorf(
			"core file preserve/replace requires a predecessor",
		)
	}
	if kind == CoreFilePreserve &&
		(renderedDigest != expectedDigest ||
			mode.Perm() != expectedMode.Perm()) {
		return CoreFileEffect{}, fmt.Errorf(
			"core file preserve must keep exact bytes and mode",
		)
	}
	return CoreFileEffect{
		kind:           kind,
		path:           canonical,
		content:        slices.Clone(content),
		mode:           mode,
		renderedDigest: renderedDigest,
		expectedDigest: expectedDigest,
		expectedMode:   expectedMode,
	}, nil
}

func (effect CoreFileEffect) Kind() CoreFileEffectKind {
	return effect.kind
}

func (effect CoreFileEffect) Path() string {
	return effect.path
}

func (effect CoreFileEffect) Content() []byte {
	return slices.Clone(effect.content)
}

func (effect CoreFileEffect) Mode() fs.FileMode {
	return effect.mode
}

func (effect CoreFileEffect) RenderedDigest() string {
	return effect.renderedDigest
}

func (effect CoreFileEffect) ExpectedDigest() string {
	return effect.expectedDigest
}

func (effect CoreFileEffect) ExpectedMode() fs.FileMode {
	return effect.expectedMode
}

func (effect CoreFileEffect) valid() bool {
	if effect.kind != CoreFileCreate &&
		effect.kind != CoreFilePreserve &&
		effect.kind != CoreFileReplace {
		return false
	}
	if effect.path == "" ||
		!validPermissionMode(effect.mode) ||
		!sha256DigestPattern.MatchString(effect.renderedDigest) {
		return false
	}
	hasExpected := effect.expectedDigest != ""
	if hasExpected != (effect.expectedMode != 0) {
		return false
	}
	if effect.kind == CoreFileCreate {
		return !hasExpected
	}
	return hasExpected &&
		sha256DigestPattern.MatchString(effect.expectedDigest) &&
		validPermissionMode(effect.expectedMode)
}

func (seed CoreDatabaseSeed) Kind() CoreDatabaseSeedKind {
	return seed.kind
}

func (seed CoreDatabaseSeed) SourcePath() string {
	return seed.sourcePath
}

func (seed CoreDatabaseSeed) ObservationPath() string {
	return seed.observationPath
}

func (seed CoreDatabaseSeed) Digest() string {
	return seed.digest
}

func (seed CoreDatabaseSeed) validFor(
	effect CoreEffectKind,
) bool {
	switch seed.kind {
	case CoreDatabaseSeedEmpty:
		return seed.observationPath == "" &&
			seed.sourcePath == "" &&
			seed.digest == ""
	case CoreDatabaseSeedLegacyCopy:
		return effect == CoreInitialize &&
			seed.observationPath != "" &&
			filepath.IsAbs(seed.observationPath) &&
			filepath.Clean(seed.observationPath) ==
				seed.observationPath &&
			seed.sourcePath != "" &&
			filepath.IsAbs(seed.sourcePath) &&
			filepath.Clean(seed.sourcePath) == seed.sourcePath &&
			sha256DigestPattern.MatchString(seed.digest)
	default:
		return false
	}
}

type CoreProjectPlan struct {
	projectRoot   string
	projectID     projectidentity.ProjectID
	databasePath  string
	databaseSeed  CoreDatabaseSeed
	rootMigration CoreRootMigration
	files         []CoreFileEffect
	effect        CoreEffectKind
	beforeSchema  int
	afterSchema   int
	basis         BasisReadiness
}

type CoreProjectPlanBuilder struct {
	projectRoot   string
	projectID     string
	databasePath  string
	databaseSeed  CoreDatabaseSeed
	rootMigration CoreRootMigration
	files         []CoreFileEffect
	effect        CoreEffectKind
	beforeSchema  int
	afterSchema   int
	basis         BasisReadiness
}

func NewCoreProjectPlanBuilder() CoreProjectPlanBuilder {
	return CoreProjectPlanBuilder{}
}

func (builder CoreProjectPlanBuilder) ForProject(
	root string,
	projectID string,
) CoreProjectPlanBuilder {
	next := builder
	next.projectRoot = root
	next.projectID = projectID
	return next
}

func (builder CoreProjectPlanBuilder) AtDatabase(path string) CoreProjectPlanBuilder {
	next := builder
	next.databasePath = path
	return next
}

func (builder CoreProjectPlanBuilder) WithLegacyDatabaseSeed(
	observationPath string,
	sourcePath string,
	digest string,
) CoreProjectPlanBuilder {
	next := builder
	next.databaseSeed = CoreDatabaseSeed{
		kind:            CoreDatabaseSeedLegacyCopy,
		observationPath: observationPath,
		sourcePath:      sourcePath,
		digest:          digest,
	}
	return next
}

func (builder CoreProjectPlanBuilder) WithFileEffects(
	effects []CoreFileEffect,
) CoreProjectPlanBuilder {
	next := builder
	next.files = slices.Clone(effects)
	return next
}

func (builder CoreProjectPlanBuilder) WithLegacyRootMigration(
	source string,
	target string,
) CoreProjectPlanBuilder {
	next := builder
	next.rootMigration = CoreRootMigration{
		kind:   CoreRootMigrationQuintToHaft,
		source: source,
		target: target,
	}
	return next
}

func (builder CoreProjectPlanBuilder) WithSchemaTransition(
	effect CoreEffectKind,
	before int,
	after int,
) CoreProjectPlanBuilder {
	next := builder
	next.effect = effect
	next.beforeSchema = before
	next.afterSchema = after
	return next
}

func (builder CoreProjectPlanBuilder) WithBasis(
	basis BasisReadiness,
) CoreProjectPlanBuilder {
	next := builder
	next.basis = basis
	return next
}

func (builder CoreProjectPlanBuilder) Build() (CoreProjectPlan, error) {
	projectRoot, err := parseCanonicalAbsolutePath(builder.projectRoot)
	if err != nil {
		return CoreProjectPlan{}, fmt.Errorf("core project root: %w", err)
	}
	projectID, err := projectidentity.ParseProjectID(builder.projectID)
	if err != nil {
		return CoreProjectPlan{}, fmt.Errorf("core project identity: %w", err)
	}
	databasePath, err := parseCanonicalAbsolutePath(builder.databasePath)
	if err != nil {
		return CoreProjectPlan{}, fmt.Errorf("core database path: %w", err)
	}
	if err := validateSchemaTransition(
		builder.effect,
		builder.beforeSchema,
		builder.afterSchema,
	); err != nil {
		return CoreProjectPlan{}, err
	}
	if !builder.basis.valid() {
		return CoreProjectPlan{}, fmt.Errorf("core TypeEnv basis readiness is invalid")
	}
	databaseSeed := builder.databaseSeed
	if databaseSeed.kind == "" {
		databaseSeed.kind = CoreDatabaseSeedEmpty
	}
	if !databaseSeed.validFor(builder.effect) {
		return CoreProjectPlan{}, fmt.Errorf(
			"core database seed is invalid for %s",
			builder.effect,
		)
	}
	rootMigration := builder.rootMigration
	if rootMigration.kind == "" {
		rootMigration.kind = CoreRootMigrationNone
	}
	if !rootMigration.valid(projectRoot) {
		return CoreProjectPlan{}, fmt.Errorf(
			"core root migration is invalid",
		)
	}
	seenFiles := make(map[string]struct{}, len(builder.files))
	for _, effect := range builder.files {
		if !effect.valid() {
			return CoreProjectPlan{}, fmt.Errorf(
				"core file effect is invalid",
			)
		}
		if _, duplicate := seenFiles[effect.path]; duplicate {
			return CoreProjectPlan{}, fmt.Errorf(
				"core file effect repeats path %s",
				effect.path,
			)
		}
		seenFiles[effect.path] = struct{}{}
	}
	files := slices.Clone(builder.files)
	sort.Slice(files, func(left int, right int) bool {
		return files[left].path < files[right].path
	})
	return CoreProjectPlan{
		projectRoot:   projectRoot,
		projectID:     projectID,
		databasePath:  databasePath,
		databaseSeed:  databaseSeed,
		rootMigration: rootMigration,
		files:         files,
		effect:        builder.effect,
		beforeSchema:  builder.beforeSchema,
		afterSchema:   builder.afterSchema,
		basis:         builder.basis,
	}, nil
}

func validateSchemaTransition(
	effect CoreEffectKind,
	before int,
	after int,
) error {
	switch effect {
	case CoreInitialize:
		if before == 0 && after > 0 {
			return nil
		}
	case CoreMigrate:
		if before > 0 && after > before {
			return nil
		}
	case CoreVerifyCurrent:
		if before > 0 && after == before {
			return nil
		}
	}
	return fmt.Errorf(
		"invalid core schema transition %s %d -> %d",
		effect,
		before,
		after,
	)
}

func (plan CoreProjectPlan) ProjectRoot() string {
	return plan.projectRoot
}

func (plan CoreProjectPlan) ProjectID() projectidentity.ProjectID {
	return plan.projectID
}

func (plan CoreProjectPlan) DatabasePath() string {
	return plan.databasePath
}

func (plan CoreProjectPlan) DatabaseSeed() CoreDatabaseSeed {
	return plan.databaseSeed
}

func (plan CoreProjectPlan) RootMigration() CoreRootMigration {
	return plan.rootMigration
}

func (plan CoreProjectPlan) FileEffects() []CoreFileEffect {
	return slices.Clone(plan.files)
}

func (plan CoreProjectPlan) Effect() CoreEffectKind {
	return plan.effect
}

func (plan CoreProjectPlan) BeforeSchema() int {
	return plan.beforeSchema
}

func (plan CoreProjectPlan) AfterSchema() int {
	return plan.afterSchema
}

func (plan CoreProjectPlan) Basis() BasisReadiness {
	return plan.basis
}

type PredecessorKind string

const (
	PredecessorMissing              PredecessorKind = "missing"
	PredecessorCurrentOwned         PredecessorKind = "current_owned"
	PredecessorOutdatedOwned        PredecessorKind = "outdated_owned"
	PredecessorLocallyModifiedOwned PredecessorKind = "locally_modified_owned"
	PredecessorKnownLegacyExact     PredecessorKind = "known_legacy_exact"
	PredecessorForeign              PredecessorKind = "foreign"
	PredecessorOrphanedOwned        PredecessorKind = "orphaned_owned"
	PredecessorMissingOwned         PredecessorKind = "missing_owned"
	PredecessorSharedCarrierExact   PredecessorKind = "shared_carrier_exact"
)

type OwnershipBasisKind string

const (
	OwnershipManifestReceipt OwnershipBasisKind = "installation_manifest_receipt"
	OwnershipLegacyRegistry  OwnershipBasisKind = "known_legacy_digest_registry"
)

type OwnershipBasis struct {
	kind   OwnershipBasisKind
	ref    string
	digest string
}

func NewOwnershipBasis(
	kind OwnershipBasisKind,
	ref string,
	digest string,
) (OwnershipBasis, error) {
	if kind != OwnershipManifestReceipt && kind != OwnershipLegacyRegistry {
		return OwnershipBasis{}, fmt.Errorf("ownership basis kind is invalid")
	}
	if ref == "" || ref != strings.TrimSpace(ref) {
		return OwnershipBasis{}, fmt.Errorf("ownership basis ref is required")
	}
	if !sha256DigestPattern.MatchString(digest) {
		return OwnershipBasis{}, fmt.Errorf("ownership basis digest is invalid")
	}
	return OwnershipBasis{
		kind:   kind,
		ref:    ref,
		digest: digest,
	}, nil
}

func (basis OwnershipBasis) Kind() OwnershipBasisKind {
	return basis.kind
}

func (basis OwnershipBasis) Ref() string {
	return basis.ref
}

func (basis OwnershipBasis) Digest() string {
	return basis.digest
}

func (basis OwnershipBasis) valid() bool {
	validKind := basis.kind == OwnershipManifestReceipt || basis.kind == OwnershipLegacyRegistry
	return validKind && basis.ref != "" && sha256DigestPattern.MatchString(basis.digest)
}

type PathExpectation struct {
	path           string
	kind           PredecessorKind
	digest         string
	mode           fs.FileMode
	manifestDigest string
	manifestMode   fs.FileMode
	basis          OwnershipBasis
}

func ExpectMissing(path string) (PathExpectation, error) {
	canonical, err := parseCanonicalAbsolutePath(path)
	if err != nil {
		return PathExpectation{}, err
	}
	return PathExpectation{
		path: canonical,
		kind: PredecessorMissing,
	}, nil
}

func expectExact(
	path string,
	kind PredecessorKind,
	digest string,
	mode fs.FileMode,
	basis OwnershipBasis,
) (PathExpectation, error) {
	canonical, err := parseCanonicalAbsolutePath(path)
	if err != nil {
		return PathExpectation{}, err
	}
	if kind != PredecessorCurrentOwned &&
		kind != PredecessorOutdatedOwned &&
		kind != PredecessorKnownLegacyExact &&
		kind != PredecessorOrphanedOwned {
		return PathExpectation{}, fmt.Errorf("exact predecessor kind is invalid")
	}
	if !sha256DigestPattern.MatchString(digest) {
		return PathExpectation{}, fmt.Errorf("exact predecessor digest is invalid")
	}
	if !validPermissionMode(mode) {
		return PathExpectation{}, fmt.Errorf("exact predecessor mode is invalid")
	}
	if !basis.valid() {
		return PathExpectation{}, fmt.Errorf("exact predecessor ownership basis is invalid")
	}
	manifestOwned := kind == PredecessorCurrentOwned ||
		kind == PredecessorOutdatedOwned ||
		kind == PredecessorOrphanedOwned
	if manifestOwned && basis.kind != OwnershipManifestReceipt {
		return PathExpectation{}, fmt.Errorf("owned predecessor requires an installation manifest receipt")
	}
	if kind == PredecessorKnownLegacyExact && basis.kind != OwnershipLegacyRegistry {
		return PathExpectation{}, fmt.Errorf("known legacy predecessor requires the digest registry")
	}
	return PathExpectation{
		path:           canonical,
		kind:           kind,
		digest:         digest,
		mode:           mode,
		manifestDigest: ownedManifestDigest(kind, digest),
		manifestMode:   ownedManifestMode(kind, mode),
		basis:          basis,
	}, nil
}

func ownedManifestMode(kind PredecessorKind, mode fs.FileMode) fs.FileMode {
	switch kind {
	case PredecessorCurrentOwned, PredecessorOutdatedOwned, PredecessorOrphanedOwned:
		return mode
	default:
		return 0
	}
}

func ownedManifestDigest(kind PredecessorKind, digest string) string {
	switch kind {
	case PredecessorCurrentOwned, PredecessorOutdatedOwned, PredecessorOrphanedOwned:
		return digest
	default:
		return ""
	}
}

func ExpectCurrentOwned(
	path string,
	digest string,
	mode fs.FileMode,
	basis OwnershipBasis,
) (PathExpectation, error) {
	return expectExact(path, PredecessorCurrentOwned, digest, mode, basis)
}

func ExpectOutdatedOwned(
	path string,
	digest string,
	mode fs.FileMode,
	basis OwnershipBasis,
) (PathExpectation, error) {
	return expectExact(path, PredecessorOutdatedOwned, digest, mode, basis)
}

func ExpectKnownLegacyExact(
	path string,
	digest string,
	mode fs.FileMode,
	basis OwnershipBasis,
) (PathExpectation, error) {
	return expectExact(path, PredecessorKnownLegacyExact, digest, mode, basis)
}

func ExpectOrphanedOwned(
	path string,
	digest string,
	mode fs.FileMode,
	basis OwnershipBasis,
) (PathExpectation, error) {
	return expectExact(path, PredecessorOrphanedOwned, digest, mode, basis)
}

func ExpectMissingOwned(
	path string,
	manifestDigest string,
	manifestMode fs.FileMode,
	basis OwnershipBasis,
) (PathExpectation, error) {
	canonical, err := parseCanonicalAbsolutePath(path)
	if err != nil {
		return PathExpectation{}, err
	}
	if !sha256DigestPattern.MatchString(manifestDigest) {
		return PathExpectation{}, fmt.Errorf("missing-owned manifest digest is invalid")
	}
	if !validPermissionMode(manifestMode) {
		return PathExpectation{}, fmt.Errorf("missing-owned manifest mode is invalid")
	}
	if !basis.valid() || basis.kind != OwnershipManifestReceipt {
		return PathExpectation{}, fmt.Errorf("missing-owned predecessor requires an installation manifest receipt")
	}
	return PathExpectation{
		path:           canonical,
		kind:           PredecessorMissingOwned,
		manifestDigest: manifestDigest,
		manifestMode:   manifestMode,
		basis:          basis,
	}, nil
}

func ExpectLocallyModifiedOwned(
	path string,
	observedDigest string,
	observedMode fs.FileMode,
	manifestDigest string,
	manifestMode fs.FileMode,
	basis OwnershipBasis,
) (PathExpectation, error) {
	canonical, err := parseCanonicalAbsolutePath(path)
	if err != nil {
		return PathExpectation{}, err
	}
	if !sha256DigestPattern.MatchString(observedDigest) ||
		!sha256DigestPattern.MatchString(manifestDigest) ||
		!validPermissionMode(observedMode) ||
		!validPermissionMode(manifestMode) ||
		(observedDigest == manifestDigest && observedMode == manifestMode) {
		return PathExpectation{}, fmt.Errorf("locally-modified predecessor digests are invalid")
	}
	if !basis.valid() || basis.kind != OwnershipManifestReceipt {
		return PathExpectation{}, fmt.Errorf("locally-modified predecessor requires an installation manifest receipt")
	}
	return PathExpectation{
		path:           canonical,
		kind:           PredecessorLocallyModifiedOwned,
		digest:         observedDigest,
		mode:           observedMode,
		manifestDigest: manifestDigest,
		manifestMode:   manifestMode,
		basis:          basis,
	}, nil
}

func ExpectForeign(
	path string,
	observedDigest string,
	observedMode fs.FileMode,
) (PathExpectation, error) {
	canonical, err := parseCanonicalAbsolutePath(path)
	if err != nil {
		return PathExpectation{}, err
	}
	if !sha256DigestPattern.MatchString(observedDigest) {
		return PathExpectation{}, fmt.Errorf("foreign predecessor digest is invalid")
	}
	if !validPermissionMode(observedMode) {
		return PathExpectation{}, fmt.Errorf("foreign predecessor mode is invalid")
	}
	return PathExpectation{
		path:   canonical,
		kind:   PredecessorForeign,
		digest: observedDigest,
		mode:   observedMode,
	}, nil
}

func ExpectSharedCarrierExact(
	path string,
	observedDigest string,
	observedMode fs.FileMode,
) (PathExpectation, error) {
	canonical, err := parseCanonicalAbsolutePath(path)
	if err != nil {
		return PathExpectation{}, err
	}
	if !sha256DigestPattern.MatchString(observedDigest) {
		return PathExpectation{}, fmt.Errorf(
			"shared carrier predecessor digest is invalid",
		)
	}
	if !validPermissionMode(observedMode) {
		return PathExpectation{}, fmt.Errorf(
			"shared carrier predecessor mode is invalid",
		)
	}
	return PathExpectation{
		path:   canonical,
		kind:   PredecessorSharedCarrierExact,
		digest: observedDigest,
		mode:   observedMode,
	}, nil
}

func (expectation PathExpectation) Path() string {
	return expectation.path
}

func (expectation PathExpectation) Kind() PredecessorKind {
	return expectation.kind
}

func (expectation PathExpectation) Digest() string {
	return expectation.digest
}

func (expectation PathExpectation) Mode() fs.FileMode {
	return expectation.mode
}

func (expectation PathExpectation) ManifestDigest() string {
	return expectation.manifestDigest
}

func (expectation PathExpectation) ManifestMode() fs.FileMode {
	return expectation.manifestMode
}

func (expectation PathExpectation) OwnershipBasis() OwnershipBasis {
	return expectation.basis
}

func (expectation PathExpectation) MatchesObservation(
	observation PathObservation,
) bool {
	if expectation.path != observation.path {
		return false
	}
	missing := expectation.kind == PredecessorMissing ||
		expectation.kind == PredecessorMissingOwned
	if missing {
		return observation.kind == PathObservedMissing
	}
	return observation.kind == PathObservedPresent &&
		observation.digest == expectation.digest &&
		observation.mode == expectation.mode.Perm()
}

func (expectation PathExpectation) valid() bool {
	if expectation.path == "" {
		return false
	}
	if expectation.kind == PredecessorMissing {
		return expectation.digest == "" &&
			expectation.mode == 0 &&
			expectation.manifestDigest == "" &&
			expectation.manifestMode == 0 &&
			!expectation.basis.valid()
	}
	if expectation.kind == PredecessorMissingOwned {
		return expectation.digest == "" &&
			expectation.mode == 0 &&
			sha256DigestPattern.MatchString(expectation.manifestDigest) &&
			validPermissionMode(expectation.manifestMode) &&
			expectation.basis.kind == OwnershipManifestReceipt &&
			expectation.basis.valid()
	}
	if expectation.kind == PredecessorLocallyModifiedOwned {
		return sha256DigestPattern.MatchString(expectation.digest) &&
			validPermissionMode(expectation.mode) &&
			sha256DigestPattern.MatchString(expectation.manifestDigest) &&
			validPermissionMode(expectation.manifestMode) &&
			(expectation.digest != expectation.manifestDigest ||
				expectation.mode != expectation.manifestMode) &&
			expectation.basis.kind == OwnershipManifestReceipt &&
			expectation.basis.valid()
	}
	if expectation.kind == PredecessorForeign {
		return sha256DigestPattern.MatchString(expectation.digest) &&
			validPermissionMode(expectation.mode) &&
			expectation.manifestDigest == "" &&
			expectation.manifestMode == 0 &&
			!expectation.basis.valid()
	}
	if expectation.kind == PredecessorSharedCarrierExact {
		return sha256DigestPattern.MatchString(expectation.digest) &&
			validPermissionMode(expectation.mode) &&
			expectation.manifestDigest == "" &&
			expectation.manifestMode == 0 &&
			!expectation.basis.valid()
	}
	if expectation.kind == PredecessorKnownLegacyExact {
		return sha256DigestPattern.MatchString(expectation.digest) &&
			validPermissionMode(expectation.mode) &&
			expectation.manifestDigest == "" &&
			expectation.manifestMode == 0 &&
			expectation.basis.kind == OwnershipLegacyRegistry &&
			expectation.basis.valid()
	}
	manifestOwned := expectation.kind == PredecessorCurrentOwned ||
		expectation.kind == PredecessorOutdatedOwned ||
		expectation.kind == PredecessorOrphanedOwned
	return manifestOwned &&
		sha256DigestPattern.MatchString(expectation.digest) &&
		validPermissionMode(expectation.mode) &&
		expectation.manifestDigest == expectation.digest &&
		expectation.manifestMode == expectation.mode &&
		expectation.basis.kind == OwnershipManifestReceipt &&
		expectation.basis.valid()
}

type RenderedOutput struct {
	path       string
	components ComponentSet
	digest     string
	content    []byte
	mode       fs.FileMode
}

func NewRenderedOutput(
	path string,
	component Component,
	content []byte,
	mode fs.FileMode,
) (RenderedOutput, error) {
	components, err := singletonComponentSet(component)
	if err != nil {
		return RenderedOutput{}, fmt.Errorf(
			"rendered output component is not closed",
		)
	}
	return NewRenderedOutputForComponents(
		path,
		components,
		content,
		mode,
	)
}

func NewRenderedOutputForComponents(
	path string,
	components ComponentSet,
	content []byte,
	mode fs.FileMode,
) (RenderedOutput, error) {
	canonical, err := parseCanonicalAbsolutePath(path)
	if err != nil {
		return RenderedOutput{}, err
	}
	if len(content) == 0 {
		return RenderedOutput{}, fmt.Errorf("rendered output content cannot be empty")
	}
	if err := validateComponentSet(components); err != nil {
		return RenderedOutput{}, fmt.Errorf(
			"rendered output components are not closed: %w",
			err,
		)
	}
	if !validPermissionMode(mode) {
		return RenderedOutput{}, fmt.Errorf("rendered output mode must be permission bits only")
	}
	digest := sha256.Sum256(content)
	return RenderedOutput{
		path:       canonical,
		components: ComponentSet{values: components.Values()},
		digest:     fmt.Sprintf("sha256:%x", digest),
		content:    slices.Clone(content),
		mode:       mode,
	}, nil
}

func (output RenderedOutput) Path() string {
	return output.path
}

func (output RenderedOutput) Digest() string {
	return output.digest
}

func (output RenderedOutput) Component() Component {
	component, _ := output.components.single()
	return component
}

func (output RenderedOutput) Components() ComponentSet {
	return ComponentSet{values: output.components.Values()}
}

func (output RenderedOutput) Content() []byte {
	return slices.Clone(output.content)
}

func (output RenderedOutput) Mode() fs.FileMode {
	return output.mode
}

type PlannedRemoval struct {
	expectation PathExpectation
	component   Component
}

func NewPlannedRemoval(
	expectation PathExpectation,
	component Component,
) (PlannedRemoval, error) {
	removable := expectation.kind == PredecessorOrphanedOwned ||
		expectation.kind == PredecessorKnownLegacyExact
	if !removable || !expectation.valid() {
		return PlannedRemoval{}, fmt.Errorf("removal requires exact manifest or legacy ownership evidence")
	}
	if _, known := knownComponents[component]; !known {
		return PlannedRemoval{}, fmt.Errorf("removal component is invalid")
	}
	return PlannedRemoval{
		expectation: expectation,
		component:   component,
	}, nil
}

func (removal PlannedRemoval) Expectation() PathExpectation {
	return removal.expectation
}

func (removal PlannedRemoval) Component() Component {
	return removal.component
}

type ConflictKind string

const (
	ConflictLocallyModifiedOwned ConflictKind = "locally_modified_owned"
	ConflictForeign              ConflictKind = "foreign"
	ConflictOtherProjectBinding  ConflictKind = "other_project_binding"
)

type InstallConflict struct {
	path   string
	kind   ConflictKind
	reason string
	basis  OwnershipBasis
}

func newInstallConflict(
	path string,
	kind ConflictKind,
	reason string,
	basis OwnershipBasis,
) (InstallConflict, error) {
	canonical, err := parseCanonicalAbsolutePath(path)
	if err != nil {
		return InstallConflict{}, err
	}
	if kind != ConflictLocallyModifiedOwned &&
		kind != ConflictForeign &&
		kind != ConflictOtherProjectBinding {
		return InstallConflict{}, fmt.Errorf("install conflict kind is invalid")
	}
	validated, err := validateReason(reason, "install conflict reason")
	if err != nil {
		return InstallConflict{}, err
	}
	owned := kind == ConflictLocallyModifiedOwned || kind == ConflictOtherProjectBinding
	if owned && (!basis.valid() || basis.kind != OwnershipManifestReceipt) {
		return InstallConflict{}, fmt.Errorf("owned conflict requires an installation manifest receipt")
	}
	if kind == ConflictForeign && basis.valid() {
		return InstallConflict{}, fmt.Errorf("foreign conflict cannot claim Haft ownership")
	}
	return InstallConflict{
		path:   canonical,
		kind:   kind,
		reason: validated,
		basis:  basis,
	}, nil
}

func NewForeignConflict(path string, reason string) (InstallConflict, error) {
	return newInstallConflict(path, ConflictForeign, reason, OwnershipBasis{})
}

func NewLocallyModifiedOwnedConflict(
	path string,
	reason string,
	basis OwnershipBasis,
) (InstallConflict, error) {
	return newInstallConflict(path, ConflictLocallyModifiedOwned, reason, basis)
}

func NewOtherProjectBindingConflict(
	path string,
	reason string,
	basis OwnershipBasis,
) (InstallConflict, error) {
	return newInstallConflict(path, ConflictOtherProjectBinding, reason, basis)
}

func (conflict InstallConflict) Path() string {
	return conflict.path
}

func (conflict InstallConflict) Kind() ConflictKind {
	return conflict.kind
}

func (conflict InstallConflict) Reason() string {
	return conflict.reason
}

func (conflict InstallConflict) OwnershipBasis() OwnershipBasis {
	return conflict.basis
}

type RecoveryOperation struct {
	argv []string
}

func NewRecoveryOperation(argv []string) (RecoveryOperation, error) {
	if len(argv) == 0 {
		return RecoveryOperation{}, fmt.Errorf("recovery operation cannot be empty")
	}
	values := make([]string, len(argv))
	for index, argument := range argv {
		if argument == "" || argument != strings.TrimSpace(argument) {
			return RecoveryOperation{}, fmt.Errorf("recovery argument %d is invalid", index)
		}
		values[index] = argument
	}
	return RecoveryOperation{argv: values}, nil
}

func (operation RecoveryOperation) Argv() []string {
	return slices.Clone(operation.argv)
}

type HostAdapterInstallPlan struct {
	host             HostID
	edition          string
	publication      PublicationIdentity
	projectRoot      string
	projectID        projectidentity.ProjectID
	scope            InstallScope
	components       ComponentSet
	targetRoots      []string
	expectations     []PathExpectation
	outputs          []RenderedOutput
	removals         []PlannedRemoval
	conflicts        []InstallConflict
	managedFragments []ManagedFragment
	managedCarriers  []ManagedCarrierInstallPlan
	manifestBasis    OwnershipBasis
	recovery         RecoveryOperation
}

type HostAdapterInstallPlanBuilder struct {
	host             HostID
	edition          string
	publication      PublicationIdentity
	projectRoot      string
	projectID        string
	scope            InstallScope
	components       ComponentSet
	targetRoots      []string
	expectations     []PathExpectation
	outputs          []RenderedOutput
	removals         []PlannedRemoval
	conflicts        []InstallConflict
	managedFragments []ManagedFragment
	managedCarriers  []ManagedCarrierInstallPlan
	manifestBasis    OwnershipBasis
	recovery         RecoveryOperation
}

func NewHostAdapterInstallPlanBuilder(host HostID) HostAdapterInstallPlanBuilder {
	return HostAdapterInstallPlanBuilder{host: host}
}

func (builder HostAdapterInstallPlanBuilder) AtEdition(
	edition string,
) HostAdapterInstallPlanBuilder {
	next := builder
	next.edition = edition
	return next
}

func (builder HostAdapterInstallPlanBuilder) PublishedFrom(
	publication PublicationIdentity,
) HostAdapterInstallPlanBuilder {
	next := builder
	next.publication = publication
	return next
}

func (builder HostAdapterInstallPlanBuilder) ForProject(
	root string,
	projectID string,
) HostAdapterInstallPlanBuilder {
	next := builder
	next.projectRoot = root
	next.projectID = projectID
	return next
}

func (builder HostAdapterInstallPlanBuilder) WithSelection(
	scope InstallScope,
	components ComponentSet,
) HostAdapterInstallPlanBuilder {
	next := builder
	next.scope = scope
	next.components = ComponentSet{values: components.Values()}
	return next
}

func (builder HostAdapterInstallPlanBuilder) AddTargetRoot(
	root string,
) HostAdapterInstallPlanBuilder {
	next := builder
	next.targetRoots = appendCopy(builder.targetRoots, root)
	return next
}

func (builder HostAdapterInstallPlanBuilder) AddOutput(
	expectation PathExpectation,
	output RenderedOutput,
) HostAdapterInstallPlanBuilder {
	next := builder
	next.expectations = appendCopy(builder.expectations, expectation)
	next.outputs = appendCopy(builder.outputs, output)
	return next
}

func (builder HostAdapterInstallPlanBuilder) AddRemoval(
	removal PlannedRemoval,
) HostAdapterInstallPlanBuilder {
	next := builder
	next.removals = appendCopy(builder.removals, removal)
	return next
}

func (builder HostAdapterInstallPlanBuilder) AddConflict(
	conflict InstallConflict,
) HostAdapterInstallPlanBuilder {
	next := builder
	next.conflicts = appendCopy(builder.conflicts, conflict)
	return next
}

func (builder HostAdapterInstallPlanBuilder) WithManagedFragments(
	fragments []ManagedFragment,
) HostAdapterInstallPlanBuilder {
	next := builder
	next.managedFragments = cloneManagedFragments(fragments)
	return next
}

func (builder HostAdapterInstallPlanBuilder) AddManagedCarrierPlan(
	plan ManagedCarrierInstallPlan,
) HostAdapterInstallPlanBuilder {
	next := builder
	next.managedCarriers = append(
		cloneManagedCarrierInstallPlans(builder.managedCarriers),
		cloneManagedCarrierInstallPlan(plan),
	)
	return next
}

func (builder HostAdapterInstallPlanBuilder) WithManifestBasis(
	basis OwnershipBasis,
) HostAdapterInstallPlanBuilder {
	next := builder
	next.manifestBasis = basis
	return next
}

func (builder HostAdapterInstallPlanBuilder) RecoverWith(
	operation RecoveryOperation,
) HostAdapterInstallPlanBuilder {
	next := builder
	next.recovery = RecoveryOperation{argv: operation.Argv()}
	return next
}

func (builder HostAdapterInstallPlanBuilder) Build() (HostAdapterInstallPlan, error) {
	if _, known := knownHosts[builder.host]; !known {
		return HostAdapterInstallPlan{}, fmt.Errorf("host adapter plan host is not canonical")
	}
	if !adapterEditionPattern.MatchString(builder.edition) {
		return HostAdapterInstallPlan{}, fmt.Errorf("host adapter plan edition is invalid")
	}
	if !builder.publication.valid() {
		return HostAdapterInstallPlan{}, fmt.Errorf("host adapter plan publication identity is invalid")
	}
	projectRoot, err := parseCanonicalAbsolutePath(builder.projectRoot)
	if err != nil {
		return HostAdapterInstallPlan{}, fmt.Errorf("host adapter project root: %w", err)
	}
	projectID, err := projectidentity.ParseProjectID(builder.projectID)
	if err != nil {
		return HostAdapterInstallPlan{}, fmt.Errorf("host adapter project identity: %w", err)
	}
	if builder.scope != ScopeProject && builder.scope != ScopeUser {
		return HostAdapterInstallPlan{}, fmt.Errorf("host adapter install scope is invalid")
	}
	if len(builder.components.values) == 0 {
		return HostAdapterInstallPlan{}, fmt.Errorf("host adapter component set is empty")
	}
	targetRoots, err := canonicalTargetRoots(builder.targetRoots)
	if err != nil {
		return HostAdapterInstallPlan{}, err
	}
	if len(builder.outputs) != len(builder.expectations) {
		return HostAdapterInstallPlan{}, fmt.Errorf("every rendered output needs one predecessor expectation")
	}
	if len(builder.outputs) == 0 &&
		len(builder.removals) == 0 &&
		len(builder.conflicts) == 0 &&
		len(builder.managedCarriers) == 0 {
		return HostAdapterInstallPlan{}, fmt.Errorf("host adapter plan has no rendered or removal effect")
	}
	if builder.manifestBasis.kind != "" &&
		(!builder.manifestBasis.valid() ||
			builder.manifestBasis.kind != OwnershipManifestReceipt) {
		return HostAdapterInstallPlan{}, fmt.Errorf(
			"host adapter plan manifest basis is invalid",
		)
	}
	managedFragments, err := canonicalDesiredManagedFragments(
		builder.managedFragments,
	)
	if err != nil {
		return HostAdapterInstallPlan{}, fmt.Errorf(
			"host adapter plan managed fragments: %w",
			err,
		)
	}
	managedCarriers := cloneManagedCarrierInstallPlans(
		builder.managedCarriers,
	)
	sort.Slice(managedCarriers, func(left int, right int) bool {
		return managedCarriers[left].Path() < managedCarriers[right].Path()
	})
	validatedBuilder := builder
	validatedBuilder.managedFragments = managedFragments
	validatedBuilder.managedCarriers = managedCarriers
	if len(builder.recovery.argv) == 0 {
		return HostAdapterInstallPlan{}, fmt.Errorf("host adapter plan lacks an idempotent recovery operation")
	}
	if err := validateAdapterPaths(validatedBuilder, targetRoots); err != nil {
		return HostAdapterInstallPlan{}, err
	}
	return HostAdapterInstallPlan{
		host:             builder.host,
		edition:          builder.edition,
		publication:      builder.publication,
		projectRoot:      projectRoot,
		projectID:        projectID,
		scope:            builder.scope,
		components:       ComponentSet{values: builder.components.Values()},
		targetRoots:      slices.Clone(targetRoots),
		expectations:     slices.Clone(builder.expectations),
		outputs:          cloneRenderedOutputs(builder.outputs),
		removals:         slices.Clone(builder.removals),
		conflicts:        slices.Clone(builder.conflicts),
		managedFragments: cloneManagedFragments(managedFragments),
		managedCarriers:  cloneManagedCarrierInstallPlans(managedCarriers),
		manifestBasis:    builder.manifestBasis,
		recovery:         RecoveryOperation{argv: builder.recovery.Argv()},
	}, nil
}

func canonicalTargetRoots(raw []string) ([]string, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("host adapter plan needs at least one target root")
	}
	seen := make(map[string]struct{}, len(raw))
	result := make([]string, 0, len(raw))
	for _, candidate := range raw {
		root, err := parseCanonicalAbsolutePath(candidate)
		if err != nil {
			return nil, fmt.Errorf("host adapter target root: %w", err)
		}
		if _, duplicate := seen[root]; duplicate {
			return nil, fmt.Errorf("host adapter repeats target root %s", root)
		}
		seen[root] = struct{}{}
		result = append(result, root)
	}
	sort.Strings(result)
	return result, nil
}

func validateAdapterPaths(
	builder HostAdapterInstallPlanBuilder,
	targetRoots []string,
) error {
	seen := make(map[string]struct{}, len(builder.outputs)+len(builder.removals))
	conflicts := make(map[string]ConflictKind, len(builder.conflicts))
	for _, conflict := range builder.conflicts {
		if !pathWithinAnyRoot(conflict.path, targetRoots) {
			return fmt.Errorf("adapter conflict %s is outside target roots", conflict.path)
		}
		if _, duplicate := conflicts[conflict.path]; duplicate {
			return fmt.Errorf("adapter plan repeats conflict path %s", conflict.path)
		}
		conflicts[conflict.path] = conflict.kind
	}
	for index, output := range builder.outputs {
		expectation := builder.expectations[index]
		if output.path == "" || !expectation.valid() || expectation.path != output.path {
			return fmt.Errorf("rendered output %d lacks its exact predecessor", index)
		}
		component, singleton := output.components.single()
		if !singleton {
			return fmt.Errorf(
				"rendered whole-file output %s must name exactly one component",
				output.path,
			)
		}
		if expectation.kind == PredecessorOrphanedOwned {
			return fmt.Errorf("rendered output %s cannot use orphan-removal ownership", output.path)
		}
		if expectation.kind == PredecessorLocallyModifiedOwned &&
			conflicts[output.path] != ConflictLocallyModifiedOwned {
			return fmt.Errorf("locally-modified output %s lacks its preserving conflict", output.path)
		}
		if expectation.kind == PredecessorForeign &&
			conflicts[output.path] != ConflictForeign {
			return fmt.Errorf("foreign output %s lacks its preserving conflict", output.path)
		}
		if !builder.components.contains(component) {
			return fmt.Errorf(
				"rendered output %s uses unselected component %s",
				output.path,
				component,
			)
		}
		if !pathWithinAnyRoot(output.path, targetRoots) {
			return fmt.Errorf("rendered output %s is outside adapter target roots", output.path)
		}
		if _, duplicate := seen[output.path]; duplicate {
			return fmt.Errorf("adapter plan repeats path %s", output.path)
		}
		seen[output.path] = struct{}{}
	}
	for _, removal := range builder.removals {
		path := removal.expectation.path
		removable := removal.expectation.kind == PredecessorOrphanedOwned ||
			removal.expectation.kind == PredecessorKnownLegacyExact
		if !removable || !removal.expectation.valid() {
			return fmt.Errorf("adapter removal lacks exact manifest or legacy evidence")
		}
		if !builder.components.contains(removal.component) {
			return fmt.Errorf(
				"adapter removal %s uses unselected component %s",
				path,
				removal.component,
			)
		}
		if !pathWithinAnyRoot(path, targetRoots) {
			return fmt.Errorf("adapter removal %s is outside target roots", path)
		}
		if _, duplicate := seen[path]; duplicate {
			return fmt.Errorf("adapter plan repeats path %s", path)
		}
		seen[path] = struct{}{}
	}
	carriers := make(map[string]ManagedCarrierInstallPlan)
	for _, carrier := range builder.managedCarriers {
		path := carrier.Path()
		carrierComponents := carrier.Components()
		if path == "" ||
			validateComponentSet(carrierComponents) != nil {
			return fmt.Errorf(
				"managed carrier plan has an invalid component or path",
			)
		}
		for _, component := range carrierComponents.Values() {
			if !builder.components.contains(component) {
				return fmt.Errorf(
					"managed carrier %s uses unselected component %s",
					path,
					component,
				)
			}
		}
		if !pathWithinAnyRoot(path, targetRoots) {
			return fmt.Errorf(
				"managed carrier %s is outside target roots",
				path,
			)
		}
		if _, duplicate := seen[path]; duplicate {
			return fmt.Errorf("adapter plan repeats path %s", path)
		}
		if _, duplicate := carriers[path]; duplicate {
			return fmt.Errorf(
				"adapter plan repeats managed carrier %s",
				path,
			)
		}
		basis := carrier.ManifestBasis()
		if basis.kind != "" &&
			(!basis.valid() ||
				basis.kind != OwnershipManifestReceipt) {
			return fmt.Errorf(
				"managed carrier %s has an invalid manifest basis",
				path,
			)
		}
		if basis.valid() &&
			(builder.manifestBasis.ref != basis.ref ||
				builder.manifestBasis.digest != basis.digest) {
			return fmt.Errorf(
				"managed carrier %s belongs to another manifest basis",
				path,
			)
		}
		if carrier.Readiness() == ManagedCarrierReady {
			result, available := carrier.MutationResult()
			if !available || result.Path() != path {
				return fmt.Errorf(
					"ready managed carrier %s lacks its exact result",
					path,
				)
			}
		}
		if carrier.Readiness() == ManagedCarrierBlocked &&
			len(carrier.Conflicts()) == 0 {
			return fmt.Errorf(
				"blocked managed carrier %s has no preserving conflict",
				path,
			)
		}
		if carrier.Readiness() != ManagedCarrierReady &&
			carrier.Readiness() != ManagedCarrierBlocked {
			return fmt.Errorf(
				"managed carrier %s readiness is invalid",
				path,
			)
		}
		carriers[path] = cloneManagedCarrierInstallPlan(carrier)
		seen[path] = struct{}{}
	}
	for _, fragment := range builder.managedFragments {
		path := fragment.coordinate.carrierPath
		if !builder.components.contains(fragment.component) {
			return fmt.Errorf(
				"managed fragment %s uses unselected component %s",
				fragment.coordinate.selector,
				fragment.component,
			)
		}
		if !pathWithinAnyRoot(path, targetRoots) {
			return fmt.Errorf(
				"managed fragment carrier %s is outside target roots",
				path,
			)
		}
		carrier, exists := carriers[path]
		if !exists ||
			!carrier.Components().contains(fragment.component) {
			return fmt.Errorf(
				"managed fragment %s lacks its carrier plan",
				fragment.coordinate.selector,
			)
		}
	}
	return nil
}

func pathWithinAnyRoot(path string, roots []string) bool {
	for _, root := range roots {
		relative, err := filepath.Rel(root, path)
		if err != nil {
			continue
		}
		if relative == "." {
			return true
		}
		outside := relative == ".."
		outside = outside || strings.HasPrefix(relative, ".."+string(filepath.Separator))
		if !outside {
			return true
		}
	}
	return false
}

func cloneRenderedOutputs(source []RenderedOutput) []RenderedOutput {
	result := make([]RenderedOutput, len(source))
	for index, output := range source {
		result[index] = RenderedOutput{
			path:       output.path,
			components: output.Components(),
			digest:     output.digest,
			content:    slices.Clone(output.content),
			mode:       output.mode,
		}
	}
	return result
}

func (plan HostAdapterInstallPlan) Host() HostID {
	return plan.host
}

func (plan HostAdapterInstallPlan) BindingID() HostBindingID {
	return HostBindingID{
		host:  plan.host,
		scope: plan.scope,
	}
}

func (plan HostAdapterInstallPlan) Edition() string {
	return plan.edition
}

func (plan HostAdapterInstallPlan) Publication() PublicationIdentity {
	return plan.publication
}

func (plan HostAdapterInstallPlan) ProjectRoot() string {
	return plan.projectRoot
}

func (plan HostAdapterInstallPlan) ProjectID() projectidentity.ProjectID {
	return plan.projectID
}

func (plan HostAdapterInstallPlan) Scope() InstallScope {
	return plan.scope
}

func (plan HostAdapterInstallPlan) Components() ComponentSet {
	return ComponentSet{values: plan.components.Values()}
}

func (plan HostAdapterInstallPlan) TargetRoots() []string {
	return slices.Clone(plan.targetRoots)
}

func (plan HostAdapterInstallPlan) Expectations() []PathExpectation {
	return slices.Clone(plan.expectations)
}

func (plan HostAdapterInstallPlan) Outputs() []RenderedOutput {
	return cloneRenderedOutputs(plan.outputs)
}

func (plan HostAdapterInstallPlan) Removals() []PlannedRemoval {
	return slices.Clone(plan.removals)
}

func (plan HostAdapterInstallPlan) Conflicts() []InstallConflict {
	return slices.Clone(plan.conflicts)
}

func (plan HostAdapterInstallPlan) ManagedFragments() []ManagedFragment {
	return cloneManagedFragments(plan.managedFragments)
}

func (plan HostAdapterInstallPlan) ManagedCarrierPlans() []ManagedCarrierInstallPlan {
	return cloneManagedCarrierInstallPlans(plan.managedCarriers)
}

func (plan HostAdapterInstallPlan) ManagedFragmentConflicts() []ManagedFragmentConflict {
	conflicts := []ManagedFragmentConflict{}
	for _, carrier := range plan.managedCarriers {
		conflicts = append(conflicts, carrier.Conflicts()...)
	}
	sort.Slice(conflicts, func(left int, right int) bool {
		return managedFragmentCoordinateKey(conflicts[left].coordinate) <
			managedFragmentCoordinateKey(conflicts[right].coordinate)
	})
	return cloneManagedFragmentConflicts(conflicts)
}

func (plan HostAdapterInstallPlan) ManifestBasis() OwnershipBasis {
	return plan.manifestBasis
}

func (plan HostAdapterInstallPlan) Recovery() RecoveryOperation {
	return RecoveryOperation{argv: plan.recovery.Argv()}
}
