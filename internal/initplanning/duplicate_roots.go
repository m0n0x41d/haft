package initplanning

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
)

var skillNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

type SkillExposure struct {
	name string
	path string
}

func NewSkillExposure(name string, path string) (SkillExposure, error) {
	if !skillNamePattern.MatchString(name) {
		return SkillExposure{}, fmt.Errorf("skill exposure name is invalid")
	}
	canonical, err := parseCanonicalAbsolutePath(path)
	if err != nil {
		return SkillExposure{}, fmt.Errorf("skill exposure path: %w", err)
	}
	return SkillExposure{name: name, path: canonical}, nil
}

func (exposure SkillExposure) Name() string {
	return exposure.name
}

func (exposure SkillExposure) Path() string {
	return exposure.path
}

type SkillRootOrigin string

const (
	SkillRootManifestOwned SkillRootOrigin = "installation_manifest"
	SkillRootDiscovered    SkillRootOrigin = "filesystem_discovery"
)

type ActiveSkillRoot struct {
	root           string
	host           HostID
	scope          InstallScope
	origin         SkillRootOrigin
	evidenceRef    string
	evidenceDigest string
	exposures      []SkillExposure
}

func NewManifestSkillRoot(
	root string,
	host HostID,
	scope InstallScope,
	manifest InstallationManifest,
	exposures []SkillExposure,
) (ActiveSkillRoot, error) {
	canonical, err := validateSkillRootBinding(root, host, scope, exposures)
	if err != nil {
		return ActiveSkillRoot{}, err
	}
	if manifest.wire.Host != host || manifest.wire.InstallScope != scope {
		return ActiveSkillRoot{}, fmt.Errorf("skill root manifest belongs to another host or scope")
	}
	if !pathWithinAnyRoot(canonical, manifest.wire.TargetRoots) {
		return ActiveSkillRoot{}, fmt.Errorf("skill root is outside manifest target roots")
	}
	manifestSkills := make(map[string]struct{}, len(manifest.wire.RenderedPaths))
	for _, path := range manifest.wire.RenderedPaths {
		if path.Component == ComponentSkills {
			manifestSkills[path.Path] = struct{}{}
		}
	}
	for _, exposure := range exposures {
		if _, recorded := manifestSkills[exposure.path]; !recorded {
			return ActiveSkillRoot{}, fmt.Errorf(
				"skill exposure %s is not a manifest-owned skill carrier",
				exposure.path,
			)
		}
	}
	return ActiveSkillRoot{
		root:           canonical,
		host:           host,
		scope:          scope,
		origin:         SkillRootManifestOwned,
		evidenceRef:    manifest.Ref(),
		evidenceDigest: manifest.Digest(),
		exposures:      slices.Clone(exposures),
	}, nil
}

func NewDiscoveredSkillRoot(
	root string,
	host HostID,
	scope InstallScope,
	observationRef string,
	observationDigest string,
	exposures []SkillExposure,
) (ActiveSkillRoot, error) {
	canonical, err := validateSkillRootBinding(root, host, scope, exposures)
	if err != nil {
		return ActiveSkillRoot{}, err
	}
	if observationRef == "" || observationRef != strings.TrimSpace(observationRef) {
		return ActiveSkillRoot{}, fmt.Errorf("skill-root discovery reference is invalid")
	}
	if !sha256DigestPattern.MatchString(observationDigest) {
		return ActiveSkillRoot{}, fmt.Errorf("skill-root discovery digest is invalid")
	}
	return ActiveSkillRoot{
		root:           canonical,
		host:           host,
		scope:          scope,
		origin:         SkillRootDiscovered,
		evidenceRef:    observationRef,
		evidenceDigest: observationDigest,
		exposures:      slices.Clone(exposures),
	}, nil
}

func validateSkillRootBinding(
	root string,
	host HostID,
	scope InstallScope,
	exposures []SkillExposure,
) (string, error) {
	canonical, err := parseCanonicalAbsolutePath(root)
	if err != nil {
		return "", fmt.Errorf("skill root: %w", err)
	}
	if _, known := knownHosts[host]; !known {
		return "", fmt.Errorf("skill root host is invalid")
	}
	if scope != ScopeProject && scope != ScopeUser {
		return "", fmt.Errorf("skill root scope is invalid")
	}
	if len(exposures) == 0 {
		return "", fmt.Errorf("skill root has no exposures")
	}
	seenNames := make(map[string]struct{}, len(exposures))
	for _, exposure := range exposures {
		if !skillNamePattern.MatchString(exposure.name) {
			return "", fmt.Errorf("skill root contains an invalid exposure")
		}
		if !pathWithinAnyRoot(exposure.path, []string{canonical}) {
			return "", fmt.Errorf("skill exposure %s is outside root %s", exposure.path, canonical)
		}
		if _, duplicate := seenNames[exposure.name]; duplicate {
			return "", fmt.Errorf("skill root repeats exposure name %s", exposure.name)
		}
		seenNames[exposure.name] = struct{}{}
	}
	return canonical, nil
}

func (root ActiveSkillRoot) Root() string {
	return root.root
}

func (root ActiveSkillRoot) Host() HostID {
	return root.host
}

func (root ActiveSkillRoot) Scope() InstallScope {
	return root.scope
}

func (root ActiveSkillRoot) Origin() SkillRootOrigin {
	return root.origin
}

func (root ActiveSkillRoot) EvidenceRef() string {
	return root.evidenceRef
}

func (root ActiveSkillRoot) EvidenceDigest() string {
	return root.evidenceDigest
}

func (root ActiveSkillRoot) Exposures() []SkillExposure {
	return slices.Clone(root.exposures)
}

type DuplicateSkillRoot struct {
	SkillName           string
	LeftRoot            string
	LeftHost            HostID
	LeftScope           InstallScope
	LeftOrigin          SkillRootOrigin
	LeftEvidenceRef     string
	LeftEvidenceDigest  string
	RightRoot           string
	RightHost           HostID
	RightScope          InstallScope
	RightOrigin         SkillRootOrigin
	RightEvidenceRef    string
	RightEvidenceDigest string
}

func FindDuplicateSkillRoots(
	roots []ActiveSkillRoot,
) ([]DuplicateSkillRoot, error) {
	canonical := slices.Clone(roots)
	sort.Slice(canonical, func(left int, right int) bool {
		return skillRootKey(canonical[left]) < skillRootKey(canonical[right])
	})
	previous := ""
	for _, root := range canonical {
		key := skillRootKey(root)
		if key == previous || !validActiveSkillRoot(root) {
			return nil, fmt.Errorf("active skill-root set is invalid or duplicated")
		}
		previous = key
	}
	bySkill := make(map[string][]ActiveSkillRoot)
	for _, root := range canonical {
		for _, exposure := range root.exposures {
			bySkill[exposure.name] = append(bySkill[exposure.name], root)
		}
	}
	duplicates := make([]DuplicateSkillRoot, 0)
	for skillName, exposingRoots := range bySkill {
		pairs := distinctSkillRootPairs(skillName, exposingRoots)
		duplicates = append(duplicates, pairs...)
	}
	sort.Slice(duplicates, func(left int, right int) bool {
		return duplicateSkillRootKey(duplicates[left]) < duplicateSkillRootKey(duplicates[right])
	})
	return duplicates, nil
}

func distinctSkillRootPairs(
	skillName string,
	roots []ActiveSkillRoot,
) []DuplicateSkillRoot {
	if len(roots) < 2 {
		return nil
	}
	head := roots[0]
	pairs := make([]DuplicateSkillRoot, 0, len(roots)-1)
	for _, candidate := range roots[1:] {
		if head.root != candidate.root {
			pairs = append(pairs, duplicateSkillRoot(skillName, head, candidate))
		}
	}
	tail := distinctSkillRootPairs(skillName, roots[1:])
	return append(pairs, tail...)
}

func validActiveSkillRoot(root ActiveSkillRoot) bool {
	if root.root == "" || root.evidenceRef == "" ||
		!sha256DigestPattern.MatchString(root.evidenceDigest) ||
		len(root.exposures) == 0 {
		return false
	}
	if root.origin != SkillRootManifestOwned && root.origin != SkillRootDiscovered {
		return false
	}
	_, err := validateSkillRootBinding(
		root.root,
		root.host,
		root.scope,
		root.exposures,
	)
	return err == nil
}

func skillRootKey(root ActiveSkillRoot) string {
	return root.root + "\x00" + string(root.host) + "\x00" + string(root.scope) + "\x00" + string(root.origin)
}

func duplicateSkillRoot(
	skillName string,
	left ActiveSkillRoot,
	right ActiveSkillRoot,
) DuplicateSkillRoot {
	if skillRootKey(right) < skillRootKey(left) {
		left, right = right, left
	}
	return DuplicateSkillRoot{
		SkillName:           skillName,
		LeftRoot:            left.root,
		LeftHost:            left.host,
		LeftScope:           left.scope,
		LeftOrigin:          left.origin,
		LeftEvidenceRef:     left.evidenceRef,
		LeftEvidenceDigest:  left.evidenceDigest,
		RightRoot:           right.root,
		RightHost:           right.host,
		RightScope:          right.scope,
		RightOrigin:         right.origin,
		RightEvidenceRef:    right.evidenceRef,
		RightEvidenceDigest: right.evidenceDigest,
	}
}

func duplicateSkillRootKey(duplicate DuplicateSkillRoot) string {
	return duplicate.SkillName + "\x00" + duplicate.LeftRoot + "\x00" + duplicate.RightRoot
}
