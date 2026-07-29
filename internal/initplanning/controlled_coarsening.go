package initplanning

import (
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"
)

const controlledCoarseningSchema = "haft.controlled-coarsening/v1"

type SourceLossMode string

const (
	SourceLossOmittedDetail        SourceLossMode = "omitted-detail"
	SourceLossQualifier            SourceLossMode = "qualifier-loss"
	SourceLossRedaction            SourceLossMode = "redaction"
	SourceLossAggregation          SourceLossMode = "aggregation"
	SourceLossScopeNarrowing       SourceLossMode = "scope-narrowing"
	SourceLossRecoverability       SourceLossMode = "recoverability-loss"
	SourceLossRepresentationFactor SourceLossMode = "representation-factor-loss"
	SourceLossCoarsening           SourceLossMode = "coarsening-loss"
)

var sourceLossModes = map[SourceLossMode]struct{}{
	SourceLossOmittedDetail:        {},
	SourceLossQualifier:            {},
	SourceLossRedaction:            {},
	SourceLossAggregation:          {},
	SourceLossScopeNarrowing:       {},
	SourceLossRecoverability:       {},
	SourceLossRepresentationFactor: {},
	SourceLossCoarsening:           {},
}

type ControlledCoarseningInput struct {
	SourceRef             string
	SourceDigest          string
	RenderingRef          string
	RenderingDigest       string
	NarrowerAdmissibleUse string
	SourceLossModes       []SourceLossMode
	NonAdmissibleUses     []string
	ReopenTriggers        []string
}

type controlledCoarseningWire struct {
	Schema                 string           `json:"schema"`
	SourceBearingSideRef   string           `json:"source_bearing_side_ref"`
	SourceBearingDigest    string           `json:"source_bearing_digest"`
	CoarsenedRenderingRef  string           `json:"coarsened_rendering_ref"`
	CoarsenedRenderingHash string           `json:"coarsened_rendering_digest"`
	NarrowerAdmissibleUse  string           `json:"narrower_admissible_use"`
	SourceLossModes        []SourceLossMode `json:"source_loss_modes"`
	NonAdmissibleUses      []string         `json:"non_admissible_downstream_uses"`
	ReopenTriggers         []string         `json:"reopen_triggers"`
}

type ControlledCoarseningDeclaration struct {
	wire      controlledCoarseningWire
	canonical []byte
	digest    string
}

func NewControlledCoarseningDeclaration(
	input ControlledCoarseningInput,
) (ControlledCoarseningDeclaration, error) {
	sourceRef, err := validateReason(
		input.SourceRef,
		"controlled-coarsening source-bearing side ref",
	)
	if err != nil {
		return ControlledCoarseningDeclaration{}, err
	}
	renderingRef, err := validateReason(
		input.RenderingRef,
		"controlled-coarsening rendering ref",
	)
	if err != nil {
		return ControlledCoarseningDeclaration{}, err
	}
	admissibleUse, err := validateReason(
		input.NarrowerAdmissibleUse,
		"controlled-coarsening narrower admissible use",
	)
	if err != nil {
		return ControlledCoarseningDeclaration{}, err
	}
	for label, digest := range map[string]string{
		"source-bearing": input.SourceDigest,
		"rendering":      input.RenderingDigest,
	} {
		if !sha256DigestPattern.MatchString(digest) {
			return ControlledCoarseningDeclaration{}, fmt.Errorf(
				"controlled-coarsening %s digest is invalid",
				label,
			)
		}
	}
	lossModes, err := canonicalSourceLossModes(input.SourceLossModes)
	if err != nil {
		return ControlledCoarseningDeclaration{}, err
	}
	nonAdmissible, err := canonicalBoundaryValues(
		input.NonAdmissibleUses,
		"controlled-coarsening non-admissible downstream use",
	)
	if err != nil {
		return ControlledCoarseningDeclaration{}, err
	}
	reopen, err := canonicalBoundaryValues(
		input.ReopenTriggers,
		"controlled-coarsening reopen trigger",
	)
	if err != nil {
		return ControlledCoarseningDeclaration{}, err
	}
	wire := controlledCoarseningWire{
		Schema:                 controlledCoarseningSchema,
		SourceBearingSideRef:   sourceRef,
		SourceBearingDigest:    input.SourceDigest,
		CoarsenedRenderingRef:  renderingRef,
		CoarsenedRenderingHash: input.RenderingDigest,
		NarrowerAdmissibleUse:  admissibleUse,
		SourceLossModes:        lossModes,
		NonAdmissibleUses:      nonAdmissible,
		ReopenTriggers:         reopen,
	}
	canonical, err := json.Marshal(wire)
	if err != nil {
		return ControlledCoarseningDeclaration{}, fmt.Errorf(
			"encode controlled-coarsening declaration: %w",
			err,
		)
	}
	return ControlledCoarseningDeclaration{
		wire:      cloneControlledCoarseningWire(wire),
		canonical: canonical,
		digest:    digestBytesForManifest(canonical),
	}, nil
}

func canonicalSourceLossModes(
	raw []SourceLossMode,
) ([]SourceLossMode, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf(
			"controlled-coarsening source-loss mode is required",
		)
	}
	seen := make(map[SourceLossMode]struct{}, len(raw))
	result := slices.Clone(raw)
	for _, mode := range result {
		if _, known := sourceLossModes[mode]; !known {
			return nil, fmt.Errorf(
				"controlled-coarsening source-loss mode %q is invalid",
				mode,
			)
		}
		if _, duplicate := seen[mode]; duplicate {
			return nil, fmt.Errorf(
				"controlled-coarsening source-loss mode %q is repeated",
				mode,
			)
		}
		seen[mode] = struct{}{}
	}
	sort.Slice(result, func(left int, right int) bool {
		return result[left] < result[right]
	})
	return result, nil
}

func canonicalBoundaryValues(
	raw []string,
	label string,
) ([]string, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("%s is required", label)
	}
	result := slices.Clone(raw)
	seen := make(map[string]struct{}, len(result))
	for _, value := range result {
		if value == "" || value != strings.TrimSpace(value) {
			return nil, fmt.Errorf("%s is required in exact form", label)
		}
		if _, duplicate := seen[value]; duplicate {
			return nil, fmt.Errorf("%s %q is repeated", label, value)
		}
		seen[value] = struct{}{}
	}
	sort.Strings(result)
	return result, nil
}

func cloneControlledCoarseningWire(
	wire controlledCoarseningWire,
) controlledCoarseningWire {
	return controlledCoarseningWire{
		Schema:                 wire.Schema,
		SourceBearingSideRef:   wire.SourceBearingSideRef,
		SourceBearingDigest:    wire.SourceBearingDigest,
		CoarsenedRenderingRef:  wire.CoarsenedRenderingRef,
		CoarsenedRenderingHash: wire.CoarsenedRenderingHash,
		NarrowerAdmissibleUse:  wire.NarrowerAdmissibleUse,
		SourceLossModes:        slices.Clone(wire.SourceLossModes),
		NonAdmissibleUses:      slices.Clone(wire.NonAdmissibleUses),
		ReopenTriggers:         slices.Clone(wire.ReopenTriggers),
	}
}

func (declaration ControlledCoarseningDeclaration) Valid() bool {
	if declaration.wire.Schema != controlledCoarseningSchema {
		return false
	}
	if !sha256DigestPattern.MatchString(declaration.digest) ||
		!sha256DigestPattern.MatchString(
			declaration.wire.SourceBearingDigest,
		) ||
		!sha256DigestPattern.MatchString(
			declaration.wire.CoarsenedRenderingHash,
		) {
		return false
	}
	return len(declaration.canonical) > 0 &&
		len(declaration.wire.SourceLossModes) > 0 &&
		len(declaration.wire.NonAdmissibleUses) > 0 &&
		len(declaration.wire.ReopenTriggers) > 0
}

func (declaration ControlledCoarseningDeclaration) Ref() string {
	return "controlled-coarsening:" +
		strings.TrimPrefix(declaration.digest, "sha256:")
}

func (declaration ControlledCoarseningDeclaration) Digest() string {
	return declaration.digest
}

func (
	declaration ControlledCoarseningDeclaration,
) CanonicalBytes() []byte {
	return slices.Clone(declaration.canonical)
}

func (
	declaration ControlledCoarseningDeclaration,
) SourceBearingSideRef() string {
	return declaration.wire.SourceBearingSideRef
}

func (
	declaration ControlledCoarseningDeclaration,
) SourceBearingDigest() string {
	return declaration.wire.SourceBearingDigest
}

func (
	declaration ControlledCoarseningDeclaration,
) CoarsenedRenderingRef() string {
	return declaration.wire.CoarsenedRenderingRef
}

func (
	declaration ControlledCoarseningDeclaration,
) CoarsenedRenderingDigest() string {
	return declaration.wire.CoarsenedRenderingHash
}

func (
	declaration ControlledCoarseningDeclaration,
) NarrowerAdmissibleUse() string {
	return declaration.wire.NarrowerAdmissibleUse
}

func (
	declaration ControlledCoarseningDeclaration,
) SourceLossModes() []SourceLossMode {
	return slices.Clone(declaration.wire.SourceLossModes)
}

func (
	declaration ControlledCoarseningDeclaration,
) NonAdmissibleUses() []string {
	return slices.Clone(declaration.wire.NonAdmissibleUses)
}

func (
	declaration ControlledCoarseningDeclaration,
) ReopenTriggers() []string {
	return slices.Clone(declaration.wire.ReopenTriggers)
}
