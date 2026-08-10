package initplanning

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"slices"
	"sort"
	"strings"

	"github.com/m0n0x41d/haft/internal/projectidentity"
)

const (
	installationManifestSchemaV1 = "haft.host-installation-manifest/v1"
	installationManifestSchemaV2 = "haft.host-installation-manifest/v2"
	installationManifestSchema   = installationManifestSchemaV1
)

type PublicationIdentity struct {
	haftVersion         string
	executablePath      string
	executableDigest    string
	skillBundleDigest   string
	kernelCatalogDigest string
}

type PublicationIdentityInput struct {
	HaftVersion         string
	ExecutablePath      string
	ExecutableDigest    string
	SkillBundleDigest   string
	KernelCatalogDigest string
}

func NewPublicationIdentity(
	input PublicationIdentityInput,
) (PublicationIdentity, error) {
	version, err := validateReason(input.HaftVersion, "Haft version")
	if err != nil {
		return PublicationIdentity{}, err
	}
	executablePath, err := parseCanonicalAbsolutePath(input.ExecutablePath)
	if err != nil {
		return PublicationIdentity{}, fmt.Errorf("executable path: %w", err)
	}
	for label, digest := range map[string]string{
		"executable":     input.ExecutableDigest,
		"skill bundle":   input.SkillBundleDigest,
		"kernel catalog": input.KernelCatalogDigest,
	} {
		if !sha256DigestPattern.MatchString(digest) {
			return PublicationIdentity{}, fmt.Errorf("%s digest is invalid", label)
		}
	}
	return PublicationIdentity{
		haftVersion:         version,
		executablePath:      executablePath,
		executableDigest:    input.ExecutableDigest,
		skillBundleDigest:   input.SkillBundleDigest,
		kernelCatalogDigest: input.KernelCatalogDigest,
	}, nil
}

func (identity PublicationIdentity) valid() bool {
	if identity.haftVersion == "" || identity.haftVersion != strings.TrimSpace(identity.haftVersion) {
		return false
	}
	canonical, err := parseCanonicalAbsolutePath(identity.executablePath)
	if err != nil || canonical != identity.executablePath {
		return false
	}
	return sha256DigestPattern.MatchString(identity.executableDigest) &&
		sha256DigestPattern.MatchString(identity.skillBundleDigest) &&
		sha256DigestPattern.MatchString(identity.kernelCatalogDigest)
}

func (identity PublicationIdentity) HaftVersion() string {
	return identity.haftVersion
}

func (identity PublicationIdentity) ExecutablePath() string {
	return identity.executablePath
}

func (identity PublicationIdentity) ExecutableDigest() string {
	return identity.executableDigest
}

func (identity PublicationIdentity) SkillBundleDigest() string {
	return identity.skillBundleDigest
}

func (identity PublicationIdentity) KernelCatalogDigest() string {
	return identity.kernelCatalogDigest
}

type ManifestPath struct {
	Path      string    `json:"path"`
	Component Component `json:"component"`
	Digest    string    `json:"digest"`
	Mode      uint32    `json:"mode"`
}

// ManifestFragment is an ownership receipt for one exact semantic coordinate
// inside a shared carrier. It deliberately records neither the whole carrier
// digest nor its mode: those remain concurrency preconditions, not ownership.
type ManifestFragment struct {
	CarrierPath  string              `json:"carrier_path"`
	Component    Component           `json:"component"`
	Kind         ManagedFragmentKind `json:"kind"`
	Selector     string              `json:"selector"`
	MemberID     string              `json:"member_id,omitempty"`
	TOMLTables   []string            `json:"toml_tables,omitempty"`
	MergeEdition string              `json:"merge_edition"`
	Digest       string              `json:"digest"`
}

type installationManifestWire struct {
	Schema              string             `json:"schema"`
	ProjectRoot         string             `json:"project_root"`
	ProjectID           string             `json:"project_id"`
	Host                HostID             `json:"host"`
	AdapterEdition      string             `json:"adapter_edition"`
	InstallScope        InstallScope       `json:"install_scope"`
	Components          []Component        `json:"components"`
	TargetRoots         []string           `json:"target_roots"`
	HaftVersion         string             `json:"haft_version"`
	ExecutablePath      string             `json:"executable_path"`
	ExecutableDigest    string             `json:"executable_digest"`
	SkillBundleDigest   string             `json:"skill_bundle_digest"`
	KernelCatalogDigest string             `json:"kernel_catalog_digest"`
	RenderedPaths       []ManifestPath     `json:"rendered_paths"`
	ManagedFragments    []ManifestFragment `json:"managed_fragments,omitempty"`
}

type InstallationManifest struct {
	wire      installationManifestWire
	canonical []byte
	digest    string
}

func BuildInstallationManifest(
	plan HostAdapterInstallPlan,
) (InstallationManifest, error) {
	if len(plan.conflicts) != 0 ||
		len(plan.ManagedFragmentConflicts()) != 0 {
		return InstallationManifest{}, fmt.Errorf("cannot manifest a blocked host adapter plan")
	}
	publication := plan.publication
	if !publication.valid() {
		return InstallationManifest{}, fmt.Errorf("cannot manifest a plan without publication identity")
	}
	paths := make([]ManifestPath, len(plan.outputs))
	for index, output := range plan.outputs {
		paths[index] = ManifestPath{
			Path:      output.path,
			Component: output.Component(),
			Digest:    output.digest,
			Mode:      uint32(output.mode.Perm()),
		}
	}
	sort.Slice(paths, func(left int, right int) bool {
		return paths[left].Path < paths[right].Path
	})
	managed := manifestFragmentsFromDesired(plan.managedFragments)
	schema := installationManifestSchemaV1
	if len(managed) > 0 {
		schema = installationManifestSchemaV2
	}
	wire := installationManifestWire{
		Schema:              schema,
		ProjectRoot:         plan.projectRoot,
		ProjectID:           plan.projectID.String(),
		Host:                plan.host,
		AdapterEdition:      plan.edition,
		InstallScope:        plan.scope,
		Components:          plan.components.Values(),
		TargetRoots:         slices.Clone(plan.targetRoots),
		HaftVersion:         publication.haftVersion,
		ExecutablePath:      publication.executablePath,
		ExecutableDigest:    publication.executableDigest,
		SkillBundleDigest:   publication.skillBundleDigest,
		KernelCatalogDigest: publication.kernelCatalogDigest,
		RenderedPaths:       paths,
		ManagedFragments:    managed,
	}
	return newInstallationManifest(wire)
}

func BuildProjectionInstallationManifest(
	projection HostAdapterProjection,
) (InstallationManifest, error) {
	if len(projection.retainedManagedFragmentComponents) != 0 {
		return InstallationManifest{}, fmt.Errorf(
			"cannot manifest a projection with unresolved retained managed fragments",
		)
	}
	if !projection.publication.valid() {
		return InstallationManifest{}, fmt.Errorf(
			"cannot manifest a projection without publication identity",
		)
	}
	targetRoots, err := canonicalTargetRoots(projection.targetRoots)
	if err != nil || !slices.Equal(targetRoots, projection.targetRoots) {
		return InstallationManifest{}, fmt.Errorf(
			"cannot manifest a projection with non-canonical target roots",
		)
	}
	outputs, err := validateProjectionOutputs(
		projection.outputs,
		projection.components,
		targetRoots,
	)
	if err != nil {
		return InstallationManifest{}, err
	}
	fragments, err := validateProjectionManagedFragments(
		projection.fragments,
		projection.components,
		targetRoots,
		outputs,
	)
	if err != nil {
		return InstallationManifest{}, err
	}
	if len(outputs) == 0 && len(fragments) == 0 {
		return InstallationManifest{}, fmt.Errorf(
			"cannot manifest an empty host adapter projection",
		)
	}
	paths := manifestPathsFromOutputs(outputs)
	managed := manifestFragmentsFromDesired(fragments)
	schema := installationManifestSchemaV1
	if len(managed) > 0 {
		schema = installationManifestSchemaV2
	}
	publication := projection.publication
	wire := installationManifestWire{
		Schema:              schema,
		ProjectRoot:         projection.projectRoot,
		ProjectID:           projection.projectID.String(),
		Host:                projection.host,
		AdapterEdition:      projection.edition,
		InstallScope:        projection.scope,
		Components:          projection.components.Values(),
		TargetRoots:         slices.Clone(targetRoots),
		HaftVersion:         publication.haftVersion,
		ExecutablePath:      publication.executablePath,
		ExecutableDigest:    publication.executableDigest,
		SkillBundleDigest:   publication.skillBundleDigest,
		KernelCatalogDigest: publication.kernelCatalogDigest,
		RenderedPaths:       paths,
		ManagedFragments:    managed,
	}
	return newInstallationManifest(wire)
}

func manifestPathsFromOutputs(outputs []RenderedOutput) []ManifestPath {
	paths := make([]ManifestPath, len(outputs))
	for index, output := range outputs {
		paths[index] = ManifestPath{
			Path:      output.path,
			Component: output.Component(),
			Digest:    output.digest,
			Mode:      uint32(output.mode.Perm()),
		}
	}
	sort.Slice(paths, func(left int, right int) bool {
		return paths[left].Path < paths[right].Path
	})
	return paths
}

func manifestFragmentsFromDesired(
	fragments []ManagedFragment,
) []ManifestFragment {
	result := make([]ManifestFragment, len(fragments))
	for index, fragment := range fragments {
		coordinate := fragment.coordinate
		result[index] = ManifestFragment{
			CarrierPath:  coordinate.carrierPath,
			Component:    fragment.component,
			Kind:         coordinate.kind,
			Selector:     coordinate.selector,
			MemberID:     coordinate.memberID,
			TOMLTables:   slices.Clone(coordinate.tomlTables),
			MergeEdition: coordinate.mergeEdition,
			Digest:       fragment.digest,
		}
	}
	sort.Slice(result, func(left int, right int) bool {
		return manifestFragmentCoordinateKey(result[left]) <
			manifestFragmentCoordinateKey(result[right])
	})
	return result
}

func ParseInstallationManifest(raw []byte) (InstallationManifest, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var wire installationManifestWire
	if err := decoder.Decode(&wire); err != nil {
		return InstallationManifest{}, fmt.Errorf("decode installation manifest: %w", err)
	}
	var trailing any
	err := decoder.Decode(&trailing)
	if err != io.EOF {
		return InstallationManifest{}, fmt.Errorf("installation manifest has trailing JSON")
	}
	manifest, err := newInstallationManifest(wire)
	if err != nil {
		return InstallationManifest{}, err
	}
	if !bytes.Equal(raw, manifest.canonical) {
		return InstallationManifest{}, fmt.Errorf("installation manifest is not canonical JSON")
	}
	return manifest, nil
}

func newInstallationManifest(
	wire installationManifestWire,
) (InstallationManifest, error) {
	if err := validateInstallationManifestWire(wire); err != nil {
		return InstallationManifest{}, err
	}
	canonical, err := json.Marshal(wire)
	if err != nil {
		return InstallationManifest{}, fmt.Errorf("encode installation manifest: %w", err)
	}
	digest := digestBytesForManifest(canonical)
	return InstallationManifest{
		wire:      cloneInstallationManifestWire(wire),
		canonical: canonical,
		digest:    digest,
	}, nil
}

func validateInstallationManifestWire(wire installationManifestWire) error {
	if wire.Schema != installationManifestSchemaV1 &&
		wire.Schema != installationManifestSchemaV2 {
		return fmt.Errorf("installation manifest schema is not current")
	}
	projectRoot, err := parseCanonicalAbsolutePath(wire.ProjectRoot)
	if err != nil || projectRoot != wire.ProjectRoot {
		return fmt.Errorf("installation manifest project root is invalid")
	}
	if _, err := projectidentity.ParseProjectID(wire.ProjectID); err != nil {
		return fmt.Errorf("installation manifest project identity is invalid")
	}
	if _, known := knownHosts[wire.Host]; !known {
		return fmt.Errorf("installation manifest host is invalid")
	}
	if !adapterEditionPattern.MatchString(wire.AdapterEdition) {
		return fmt.Errorf("installation manifest adapter edition is invalid")
	}
	if wire.InstallScope != ScopeProject && wire.InstallScope != ScopeUser {
		return fmt.Errorf("installation manifest scope is invalid")
	}
	components, err := validateCanonicalComponents(wire.Components)
	if err != nil {
		return err
	}
	targetRoots, err := canonicalTargetRoots(wire.TargetRoots)
	if err != nil || !slices.Equal(targetRoots, wire.TargetRoots) {
		return fmt.Errorf("installation manifest target roots are not canonical")
	}
	if _, err := validateReason(wire.HaftVersion, "manifest Haft version"); err != nil {
		return err
	}
	if _, err := parseCanonicalAbsolutePath(wire.ExecutablePath); err != nil {
		return fmt.Errorf("installation manifest executable path is invalid")
	}
	for label, digest := range map[string]string{
		"executable":     wire.ExecutableDigest,
		"skill bundle":   wire.SkillBundleDigest,
		"kernel catalog": wire.KernelCatalogDigest,
	} {
		if !sha256DigestPattern.MatchString(digest) {
			return fmt.Errorf("installation manifest %s digest is invalid", label)
		}
	}
	if wire.RenderedPaths == nil {
		return fmt.Errorf("installation manifest rendered paths are required")
	}
	if wire.Schema == installationManifestSchemaV1 &&
		len(wire.ManagedFragments) != 0 {
		return fmt.Errorf("v1 installation manifest cannot own managed fragments")
	}
	if wire.Schema == installationManifestSchemaV1 &&
		len(wire.RenderedPaths) == 0 {
		return fmt.Errorf("v1 installation manifest has no rendered paths")
	}
	if wire.Schema == installationManifestSchemaV2 &&
		len(wire.ManagedFragments) == 0 {
		return fmt.Errorf("v2 installation manifest has no managed fragments")
	}
	previous := ""
	wholePaths := make(map[string]struct{}, len(wire.RenderedPaths))
	for _, path := range wire.RenderedPaths {
		if path.Path <= previous {
			return fmt.Errorf("installation manifest rendered paths are not unique and sorted")
		}
		if _, err := parseCanonicalAbsolutePath(path.Path); err != nil {
			return fmt.Errorf("installation manifest rendered path is invalid")
		}
		if !pathWithinAnyRoot(path.Path, targetRoots) {
			return fmt.Errorf("installation manifest rendered path is outside target roots")
		}
		if !components.contains(path.Component) {
			return fmt.Errorf("installation manifest rendered path uses an unselected component")
		}
		if !sha256DigestPattern.MatchString(path.Digest) {
			return fmt.Errorf("installation manifest rendered path digest is invalid")
		}
		mode := fs.FileMode(path.Mode)
		if mode == 0 || mode&^fs.FileMode(0o777) != 0 {
			return fmt.Errorf("installation manifest rendered path mode is invalid")
		}
		wholePaths[path.Path] = struct{}{}
		previous = path.Path
	}
	previousFragment := ""
	groups := make(map[string][]ManagedFragmentRecord)
	for _, fragment := range wire.ManagedFragments {
		record, err := managedFragmentRecordFromManifest(fragment)
		if err != nil {
			return err
		}
		key := manifestFragmentCoordinateKey(fragment)
		if key <= previousFragment {
			return fmt.Errorf(
				"installation manifest managed fragments are not unique and sorted",
			)
		}
		if !components.contains(fragment.Component) {
			return fmt.Errorf(
				"installation manifest managed fragment uses an unselected component",
			)
		}
		if !pathWithinAnyRoot(fragment.CarrierPath, targetRoots) {
			return fmt.Errorf(
				"installation manifest managed fragment carrier is outside target roots",
			)
		}
		if _, wholeOwned := wholePaths[fragment.CarrierPath]; wholeOwned {
			return fmt.Errorf(
				"installation manifest cannot own a whole carrier and its fragment",
			)
		}
		groups[fragment.CarrierPath] = append(
			groups[fragment.CarrierPath],
			record,
		)
		previousFragment = key
	}
	for _, records := range groups {
		if _, _, _, _, _, err := validateManagedFragmentGroup(
			nil,
			records,
			nil,
		); err != nil {
			return fmt.Errorf(
				"installation manifest managed fragment group: %w",
				err,
			)
		}
	}
	return nil
}

func managedFragmentRecordFromManifest(
	fragment ManifestFragment,
) (ManagedFragmentRecord, error) {
	if !sha256DigestPattern.MatchString(fragment.Digest) {
		return ManagedFragmentRecord{}, fmt.Errorf(
			"installation manifest managed fragment digest is invalid",
		)
	}
	var coordinate ManagedFragmentCoordinate
	var err error
	switch fragment.Kind {
	case ManagedJSONObjectEntry, ManagedJSONArrayMember:
		selector, selectorErr := parseCanonicalJSONPointer(fragment.Selector)
		if selectorErr != nil {
			return ManagedFragmentRecord{}, fmt.Errorf(
				"installation manifest managed JSON selector: %w",
				selectorErr,
			)
		}
		coordinate, err = newJSONManagedFragmentCoordinate(
			fragment.CarrierPath,
			fragment.Kind,
			selector,
			fragment.MemberID,
			fragment.MergeEdition,
		)
	case ManagedYAMLMappingEntry, ManagedYAMLSequenceMember:
		selector, selectorErr := parseCanonicalJSONPointer(fragment.Selector)
		if selectorErr != nil {
			return ManagedFragmentRecord{}, fmt.Errorf(
				"installation manifest managed YAML selector: %w",
				selectorErr,
			)
		}
		coordinate, err = newYAMLManagedFragmentCoordinate(
			fragment.CarrierPath,
			fragment.Kind,
			selector,
			fragment.MemberID,
			fragment.MergeEdition,
		)
	case ManagedTOMLTableFamily, ManagedTOMLTableSet:
		if fragment.MemberID != "" {
			return ManagedFragmentRecord{}, fmt.Errorf(
				"installation manifest TOML fragment has a member identity",
			)
		}
		prefix, prefixErr := validateTOMLTablePrefix(fragment.Selector)
		if prefixErr != nil {
			return ManagedFragmentRecord{}, prefixErr
		}
		path, pathErr := parseCanonicalAbsolutePath(fragment.CarrierPath)
		if pathErr != nil {
			return ManagedFragmentRecord{}, pathErr
		}
		edition, editionErr := validateManagedMergeEdition(
			fragment.MergeEdition,
		)
		if editionErr != nil {
			return ManagedFragmentRecord{}, editionErr
		}
		tables := []string(nil)
		if fragment.Kind == ManagedTOMLTableFamily &&
			len(fragment.TOMLTables) != 0 {
			return ManagedFragmentRecord{}, fmt.Errorf(
				"installation manifest TOML table family has exact tables",
			)
		}
		if fragment.Kind == ManagedTOMLTableSet {
			tables, err = canonicalTOMLTableNames(
				prefix,
				fragment.TOMLTables,
			)
			if err != nil {
				return ManagedFragmentRecord{}, err
			}
			if !slices.Equal(tables, fragment.TOMLTables) {
				return ManagedFragmentRecord{}, fmt.Errorf(
					"installation manifest TOML table set is not canonical",
				)
			}
		}
		coordinate = ManagedFragmentCoordinate{
			carrierPath:  path,
			kind:         fragment.Kind,
			selector:     prefix,
			mergeEdition: edition,
			tomlPrefix:   prefix,
			tomlTables:   tables,
		}
	case ManagedHTMLCommentSection:
		if fragment.MemberID != "" {
			return ManagedFragmentRecord{}, fmt.Errorf(
				"installation manifest HTML-comment section has a member identity",
			)
		}
		coordinate, err = newHTMLCommentSectionCoordinate(
			fragment.CarrierPath,
			fragment.Selector,
			fragment.MergeEdition,
		)
	default:
		return ManagedFragmentRecord{}, fmt.Errorf(
			"installation manifest managed fragment kind is invalid",
		)
	}
	if err != nil {
		return ManagedFragmentRecord{}, err
	}
	record := ManagedFragmentRecord{
		coordinate: coordinate,
		component:  fragment.Component,
		digest:     fragment.Digest,
	}
	if !record.valid() {
		return ManagedFragmentRecord{}, fmt.Errorf(
			"installation manifest managed fragment coordinate is invalid",
		)
	}
	return record, nil
}

func parseCanonicalJSONPointer(raw string) ([]string, error) {
	if !strings.HasPrefix(raw, "/") {
		return nil, fmt.Errorf("JSON pointer is not absolute")
	}
	encoded := strings.Split(strings.TrimPrefix(raw, "/"), "/")
	decoded := make([]string, len(encoded))
	for index, token := range encoded {
		value, err := decodeJSONPointerToken(token)
		if err != nil {
			return nil, err
		}
		decoded[index] = value
	}
	values, canonical, err := canonicalJSONPointer(decoded)
	if err != nil {
		return nil, err
	}
	if canonical != raw {
		return nil, fmt.Errorf("JSON pointer is not canonical")
	}
	return values, nil
}

func decodeJSONPointerToken(raw string) (string, error) {
	var result strings.Builder
	for index := 0; index < len(raw); index++ {
		if raw[index] != '~' {
			result.WriteByte(raw[index])
			continue
		}
		if index+1 >= len(raw) {
			return "", fmt.Errorf("JSON pointer has an incomplete escape")
		}
		index++
		switch raw[index] {
		case '0':
			result.WriteByte('~')
		case '1':
			result.WriteByte('/')
		default:
			return "", fmt.Errorf("JSON pointer has an invalid escape")
		}
	}
	return result.String(), nil
}

func manifestFragmentCoordinateKey(
	fragment ManifestFragment,
) string {
	return strings.Join(
		[]string{
			fragment.CarrierPath,
			string(fragment.Kind),
			fragment.Selector,
			fragment.MemberID,
			fragment.MergeEdition,
		},
		"\x00",
	)
}

func validateCanonicalComponents(raw []Component) (ComponentSet, error) {
	stringsRaw := make([]string, len(raw))
	for index, component := range raw {
		stringsRaw[index] = string(component)
	}
	components, err := ParseComponentSet(stringsRaw)
	if err != nil {
		return ComponentSet{}, fmt.Errorf("installation manifest components: %w", err)
	}
	if !slices.Equal(components.values, raw) {
		return ComponentSet{}, fmt.Errorf("installation manifest components are not canonical")
	}
	return components, nil
}

func cloneInstallationManifestWire(
	wire installationManifestWire,
) installationManifestWire {
	fragments := make([]ManifestFragment, len(wire.ManagedFragments))
	for index, fragment := range wire.ManagedFragments {
		fragments[index] = fragment
		fragments[index].TOMLTables = slices.Clone(fragment.TOMLTables)
	}
	return installationManifestWire{
		Schema:              wire.Schema,
		ProjectRoot:         wire.ProjectRoot,
		ProjectID:           wire.ProjectID,
		Host:                wire.Host,
		AdapterEdition:      wire.AdapterEdition,
		InstallScope:        wire.InstallScope,
		Components:          slices.Clone(wire.Components),
		TargetRoots:         slices.Clone(wire.TargetRoots),
		HaftVersion:         wire.HaftVersion,
		ExecutablePath:      wire.ExecutablePath,
		ExecutableDigest:    wire.ExecutableDigest,
		SkillBundleDigest:   wire.SkillBundleDigest,
		KernelCatalogDigest: wire.KernelCatalogDigest,
		RenderedPaths:       slices.Clone(wire.RenderedPaths),
		ManagedFragments:    fragments,
	}
}

func cloneInstallationManifest(
	manifest InstallationManifest,
) InstallationManifest {
	return InstallationManifest{
		wire:      cloneInstallationManifestWire(manifest.wire),
		canonical: slices.Clone(manifest.canonical),
		digest:    manifest.digest,
	}
}

func digestBytesForManifest(value []byte) string {
	digest := sha256Digest(value)
	return "sha256:" + digest
}

func sha256Digest(value []byte) string {
	digest := sha256.Sum256(value)
	return fmt.Sprintf("%x", digest)
}

func (manifest InstallationManifest) CanonicalBytes() []byte {
	return slices.Clone(manifest.canonical)
}

func (manifest InstallationManifest) Schema() string {
	return manifest.wire.Schema
}

func (manifest InstallationManifest) Digest() string {
	return manifest.digest
}

func (manifest InstallationManifest) Ref() string {
	return "host-installation-manifest:" + strings.TrimPrefix(manifest.digest, "sha256:")
}

func (manifest InstallationManifest) Host() HostID {
	return manifest.wire.Host
}

func (manifest InstallationManifest) ProjectRoot() string {
	return manifest.wire.ProjectRoot
}

func (manifest InstallationManifest) ProjectID() string {
	return manifest.wire.ProjectID
}

func (manifest InstallationManifest) AdapterEdition() string {
	return manifest.wire.AdapterEdition
}

func (manifest InstallationManifest) Scope() InstallScope {
	return manifest.wire.InstallScope
}

func (manifest InstallationManifest) Components() []Component {
	return slices.Clone(manifest.wire.Components)
}

func (manifest InstallationManifest) TargetRoots() []string {
	return slices.Clone(manifest.wire.TargetRoots)
}

func (manifest InstallationManifest) HaftVersion() string {
	return manifest.wire.HaftVersion
}

func (manifest InstallationManifest) ExecutablePath() string {
	return manifest.wire.ExecutablePath
}

func (manifest InstallationManifest) ExecutableDigest() string {
	return manifest.wire.ExecutableDigest
}

func (manifest InstallationManifest) SkillBundleDigest() string {
	return manifest.wire.SkillBundleDigest
}

func (manifest InstallationManifest) KernelCatalogDigest() string {
	return manifest.wire.KernelCatalogDigest
}

func (manifest InstallationManifest) RenderedPaths() []ManifestPath {
	return slices.Clone(manifest.wire.RenderedPaths)
}

func (manifest InstallationManifest) ManagedFragments() []ManifestFragment {
	fragments := make(
		[]ManifestFragment,
		len(manifest.wire.ManagedFragments),
	)
	for index, fragment := range manifest.wire.ManagedFragments {
		fragments[index] = fragment
		fragments[index].TOMLTables = slices.Clone(fragment.TOMLTables)
	}
	return fragments
}

func (manifest InstallationManifest) ManagedFragmentRecords() (
	[]ManagedFragmentRecord,
	error,
) {
	records := make(
		[]ManagedFragmentRecord,
		len(manifest.wire.ManagedFragments),
	)
	for index, fragment := range manifest.wire.ManagedFragments {
		record, err := managedFragmentRecordFromManifest(fragment)
		if err != nil {
			return nil, err
		}
		records[index] = record
	}
	return canonicalManagedFragmentRecords(records)
}

func (manifest InstallationManifest) ManagedFragmentBaseline() (
	ManagedFragmentBaseline,
	error,
) {
	records, err := manifest.ManagedFragmentRecords()
	if err != nil {
		return ManagedFragmentBaseline{}, err
	}
	if len(records) == 0 {
		return ManagedFragmentBaseline{}, fmt.Errorf(
			"installation manifest has no managed fragment ownership",
		)
	}
	return NewManagedFragmentManifestBaseline(
		records,
		manifest.OwnershipBasis(),
	)
}

func (manifest InstallationManifest) OwnershipBasis() OwnershipBasis {
	basis, err := NewOwnershipBasis(
		OwnershipManifestReceipt,
		manifest.Ref(),
		manifest.Digest(),
	)
	if err != nil {
		return OwnershipBasis{}
	}
	return basis
}
