package initplanning

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"sort"
	"strings"

	"github.com/m0n0x41d/haft/internal/projectidentity"
)

const knownLegacyDigestRegistrySchema = "haft.known-legacy-digest-registry/v1"

type KnownLegacyPath struct {
	Path      string    `json:"path"`
	Component Component `json:"component"`
	Digest    string    `json:"digest"`
}

type KnownLegacyDigestRegistryInput struct {
	Edition     string
	ProjectRoot string
	ProjectID   string
	Host        HostID
	Scope       InstallScope
	TargetRoots []string
	Paths       []KnownLegacyPath
}

type knownLegacyDigestRegistryWire struct {
	Schema      string            `json:"schema"`
	Edition     string            `json:"edition"`
	ProjectRoot string            `json:"project_root"`
	ProjectID   string            `json:"project_id"`
	Host        HostID            `json:"host"`
	Scope       InstallScope      `json:"scope"`
	TargetRoots []string          `json:"target_roots"`
	Paths       []KnownLegacyPath `json:"paths"`
}

type KnownLegacyDigestRegistry struct {
	wire      knownLegacyDigestRegistryWire
	canonical []byte
	digest    string
}

func BuildKnownLegacyDigestRegistry(
	input KnownLegacyDigestRegistryInput,
) (KnownLegacyDigestRegistry, error) {
	paths := slices.Clone(input.Paths)
	sort.Slice(paths, func(left int, right int) bool {
		return paths[left].Path < paths[right].Path
	})
	targetRoots, err := canonicalTargetRoots(input.TargetRoots)
	if err != nil {
		return KnownLegacyDigestRegistry{}, fmt.Errorf("legacy registry target roots: %w", err)
	}
	wire := knownLegacyDigestRegistryWire{
		Schema:      knownLegacyDigestRegistrySchema,
		Edition:     input.Edition,
		ProjectRoot: input.ProjectRoot,
		ProjectID:   input.ProjectID,
		Host:        input.Host,
		Scope:       input.Scope,
		TargetRoots: targetRoots,
		Paths:       paths,
	}
	return newKnownLegacyDigestRegistry(wire)
}

func ParseKnownLegacyDigestRegistry(
	raw []byte,
) (KnownLegacyDigestRegistry, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var wire knownLegacyDigestRegistryWire
	if err := decoder.Decode(&wire); err != nil {
		return KnownLegacyDigestRegistry{}, fmt.Errorf("decode known-legacy registry: %w", err)
	}
	var trailing any
	err := decoder.Decode(&trailing)
	if err != io.EOF {
		return KnownLegacyDigestRegistry{}, fmt.Errorf("known-legacy registry has trailing JSON")
	}
	registry, err := newKnownLegacyDigestRegistry(wire)
	if err != nil {
		return KnownLegacyDigestRegistry{}, err
	}
	if !bytes.Equal(raw, registry.canonical) {
		return KnownLegacyDigestRegistry{}, fmt.Errorf("known-legacy registry is not canonical JSON")
	}
	return registry, nil
}

func newKnownLegacyDigestRegistry(
	wire knownLegacyDigestRegistryWire,
) (KnownLegacyDigestRegistry, error) {
	if err := validateKnownLegacyDigestRegistryWire(wire); err != nil {
		return KnownLegacyDigestRegistry{}, err
	}
	canonical, err := json.Marshal(wire)
	if err != nil {
		return KnownLegacyDigestRegistry{}, fmt.Errorf("encode known-legacy registry: %w", err)
	}
	digest := digestBytesForManifest(canonical)
	return KnownLegacyDigestRegistry{
		wire:      cloneKnownLegacyDigestRegistryWire(wire),
		canonical: canonical,
		digest:    digest,
	}, nil
}

func validateKnownLegacyDigestRegistryWire(
	wire knownLegacyDigestRegistryWire,
) error {
	if wire.Schema != knownLegacyDigestRegistrySchema {
		return fmt.Errorf("known-legacy registry schema is not current")
	}
	if !adapterEditionPattern.MatchString(wire.Edition) {
		return fmt.Errorf("known-legacy registry edition is invalid")
	}
	projectRoot, err := parseCanonicalAbsolutePath(wire.ProjectRoot)
	if err != nil || projectRoot != wire.ProjectRoot {
		return fmt.Errorf("known-legacy registry project root is invalid")
	}
	if _, err := projectidentity.ParseProjectID(wire.ProjectID); err != nil {
		return fmt.Errorf("known-legacy registry project identity is invalid")
	}
	if _, known := knownHosts[wire.Host]; !known {
		return fmt.Errorf("known-legacy registry host is invalid")
	}
	if wire.Scope != ScopeProject && wire.Scope != ScopeUser {
		return fmt.Errorf("known-legacy registry scope is invalid")
	}
	targetRoots, err := canonicalTargetRoots(wire.TargetRoots)
	if err != nil || !slices.Equal(targetRoots, wire.TargetRoots) {
		return fmt.Errorf("known-legacy registry target roots are not canonical")
	}
	if len(wire.Paths) == 0 {
		return fmt.Errorf("known-legacy registry has no paths")
	}
	previous := ""
	for _, path := range wire.Paths {
		if path.Path <= previous {
			return fmt.Errorf("known-legacy registry paths are not unique and sorted")
		}
		canonical, err := parseCanonicalAbsolutePath(path.Path)
		if err != nil || canonical != path.Path {
			return fmt.Errorf("known-legacy registry path is invalid")
		}
		if !pathWithinAnyRoot(path.Path, targetRoots) {
			return fmt.Errorf("known-legacy registry path is outside target roots")
		}
		if _, known := knownComponents[path.Component]; !known {
			return fmt.Errorf("known-legacy registry path component is invalid")
		}
		if !sha256DigestPattern.MatchString(path.Digest) {
			return fmt.Errorf("known-legacy registry path digest is invalid")
		}
		previous = path.Path
	}
	return nil
}

func cloneKnownLegacyDigestRegistryWire(
	wire knownLegacyDigestRegistryWire,
) knownLegacyDigestRegistryWire {
	return knownLegacyDigestRegistryWire{
		Schema:      wire.Schema,
		Edition:     wire.Edition,
		ProjectRoot: wire.ProjectRoot,
		ProjectID:   wire.ProjectID,
		Host:        wire.Host,
		Scope:       wire.Scope,
		TargetRoots: slices.Clone(wire.TargetRoots),
		Paths:       slices.Clone(wire.Paths),
	}
}

func (registry KnownLegacyDigestRegistry) CanonicalBytes() []byte {
	return slices.Clone(registry.canonical)
}

func (registry KnownLegacyDigestRegistry) Digest() string {
	return registry.digest
}

func (registry KnownLegacyDigestRegistry) Ref() string {
	digest := strings.TrimPrefix(registry.digest, "sha256:")
	return "known-legacy-digest-registry:" + digest
}

func (registry KnownLegacyDigestRegistry) Edition() string {
	return registry.wire.Edition
}

func (registry KnownLegacyDigestRegistry) Paths() []KnownLegacyPath {
	return slices.Clone(registry.wire.Paths)
}

func (registry KnownLegacyDigestRegistry) OwnershipBasis() OwnershipBasis {
	basis, err := NewOwnershipBasis(
		OwnershipLegacyRegistry,
		registry.Ref(),
		registry.Digest(),
	)
	if err != nil {
		return OwnershipBasis{}
	}
	return basis
}

type LegacyRegistrySelectionKind string

const (
	LegacyRegistryNotSelected LegacyRegistrySelectionKind = "not_selected"
	LegacyRegistrySelected    LegacyRegistrySelectionKind = "selected"
)

type LegacyRegistrySelection struct {
	kind     LegacyRegistrySelectionKind
	registry KnownLegacyDigestRegistry
}

func WithoutKnownLegacyRegistry() LegacyRegistrySelection {
	return LegacyRegistrySelection{kind: LegacyRegistryNotSelected}
}

func WithKnownLegacyRegistry(
	registry KnownLegacyDigestRegistry,
) (LegacyRegistrySelection, error) {
	if len(registry.canonical) == 0 || !sha256DigestPattern.MatchString(registry.digest) {
		return LegacyRegistrySelection{}, fmt.Errorf("known-legacy registry is invalid")
	}
	return LegacyRegistrySelection{
		kind:     LegacyRegistrySelected,
		registry: registry,
	}, nil
}
