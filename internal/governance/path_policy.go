package governance

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/m0n0x41d/haft/internal/projectpath"
)

const (
	governanceModeModule = "module"
	governanceModeExact  = "exact"
)

// DecisionPathPolicy is the pure, read-side interpretation of the path scope
// carried by a DecisionRecord. It deliberately does not infer authority from
// an implementation footprint.
type DecisionPathPolicy struct {
	mode                     string
	legacyPathScope          bool
	typedModuleRootPathScope bool
	exactFileTargets         []projectpath.Path
	moduleTargets            []projectpath.ModulePath
}

type decisionPathPolicyJSON struct {
	GovernanceMode          string `json:"governance_mode"`
	ImplementationFootprint struct {
		Files []string `json:"files"`
	} `json:"implementation_footprint"`
	GovernanceTargets []governanceTargetJSON `json:"governance_targets"`
	DriftWatchTargets []driftWatchTargetJSON `json:"drift_watch_targets"`
	BindingTargets    []bindingTargetJSON    `json:"binding_targets"`
}

type governanceTargetJSON struct {
	Kind          string             `json:"kind"`
	Ref           string             `json:"ref"`
	BindingTarget *bindingTargetJSON `json:"binding_target"`
}

type driftWatchTargetJSON struct {
	TargetRef     string             `json:"target_ref"`
	Trigger       string             `json:"trigger"`
	BindingTarget *bindingTargetJSON `json:"binding_target"`
}

type bindingTargetJSON struct {
	Kind       string `json:"kind"`
	FilePath   string `json:"file_path"`
	ModulePath string `json:"module_path"`
}

// ParseDecisionPathPolicy decodes the minimum path-authority projection from a
// DecisionRecord. Invalid modes and malformed JSON fail closed.
func ParseDecisionPathPolicy(structuredData string) (DecisionPathPolicy, error) {
	raw := strings.TrimSpace(structuredData)
	if raw == "" {
		raw = "{}"
	}

	var decoded decisionPathPolicyJSON
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return DecisionPathPolicy{}, fmt.Errorf(
			"decode decision path policy: %w",
			err,
		)
	}

	mode := strings.TrimSpace(decoded.GovernanceMode)
	if mode == "" {
		mode = governanceModeModule
	}
	if mode != governanceModeModule && mode != governanceModeExact {
		return DecisionPathPolicy{}, fmt.Errorf(
			"unsupported governance_mode %q",
			decoded.GovernanceMode,
		)
	}

	targets := decisionBindingTargets(decoded)
	exactFileTargets, moduleTargets, err := classifyBindingTargets(targets)
	if err != nil {
		return DecisionPathPolicy{}, err
	}
	hasFootprint := hasNonBlankString(decoded.ImplementationFootprint.Files)
	hasSemanticTargets := decisionHasSemanticTargets(decoded)

	return DecisionPathPolicy{
		mode:                     mode,
		legacyPathScope:          !hasFootprint && !hasSemanticTargets,
		typedModuleRootPathScope: !hasFootprint && hasSemanticTargets,
		exactFileTargets:         exactFileTargets,
		moduleTargets:            moduleTargets,
	}, nil
}

// HasBindingInFile reports whether at least one explicit non-module binding
// target is physically located in this exact project file.
func (p DecisionPathPolicy) HasBindingInFile(
	filePath projectpath.Path,
) bool {
	for _, target := range p.exactFileTargets {
		if target.String() == filePath.String() {
			return true
		}
	}
	return false
}

// UsesLegacyModulePathScope reports that module context must be recovered from
// historical affected_files rows. Typed decisions never use those rows as
// module authority.
func (p DecisionPathPolicy) UsesLegacyModulePathScope() bool {
	return p.legacyPathScope && p.mode == governanceModeModule
}

// UsesTypedModuleRootPathScope reports that an otherwise typed decision may
// recover module context only from an affected_files entry that is exactly the
// indexed module path. A sibling file or nested directory never qualifies, and
// implementation_footprint remains provenance-only.
func (p DecisionPathPolicy) UsesTypedModuleRootPathScope() bool {
	return p.typedModuleRootPathScope && p.mode == governanceModeModule
}

// AllowsAffectedPathModuleContext applies the shared recursive module-context
// boundary for one stored affected_files row. Exact affected-path readers may
// canonicalize legacy rows for compatibility, but recursive context requires
// that the stored row was canonical already.
func (p DecisionPathPolicy) AllowsAffectedPathModuleContext(
	rawPath string,
	affectedPath projectpath.Path,
	modulePath projectpath.ModulePath,
	indexedConcreteFile bool,
) bool {
	if rawPath != affectedPath.String() ||
		p.mode != governanceModeModule {
		return false
	}
	exactModuleRoot := affectedPath.String() == modulePath.String()
	if p.typedModuleRootPathScope {
		return exactModuleRoot
	}
	if p.legacyPathScope {
		return exactModuleRoot || indexedConcreteFile
	}
	return false
}

// ModuleTargets returns the explicit typed module bindings. The returned slice
// is detached from the policy.
func (p DecisionPathPolicy) ModuleTargets() []projectpath.ModulePath {
	return append([]projectpath.ModulePath{}, p.moduleTargets...)
}

// AllowsModuleContext reports whether a decision has current context for the
// exact canonical module. Legacy rows use governance_mode; typed rows require
// an explicit module BindingTarget for this module.
func (p DecisionPathPolicy) AllowsModuleContext(
	modulePath projectpath.ModulePath,
) bool {
	if p.legacyPathScope {
		return p.mode == governanceModeModule
	}
	for _, target := range p.moduleTargets {
		if target.String() == modulePath.String() {
			return true
		}
	}
	return false
}

func hasNonBlankString(values []string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}

func decisionBindingTargets(
	decoded decisionPathPolicyJSON,
) []bindingTargetJSON {
	driftTargets := make([]bindingTargetJSON, 0)
	for _, target := range decoded.DriftWatchTargets {
		if target.BindingTarget != nil {
			driftTargets = append(driftTargets, *target.BindingTarget)
		}
	}
	if len(driftTargets) > 0 {
		return driftTargets
	}
	governanceTargets := make([]bindingTargetJSON, 0)
	for _, target := range decoded.GovernanceTargets {
		if target.BindingTarget != nil {
			governanceTargets = append(
				governanceTargets,
				*target.BindingTarget,
			)
		}
	}
	if len(governanceTargets) > 0 {
		return governanceTargets
	}
	return append([]bindingTargetJSON{}, decoded.BindingTargets...)
}

func classifyBindingTargets(
	targets []bindingTargetJSON,
) ([]projectpath.Path, []projectpath.ModulePath, error) {
	exactFileTargets := make([]projectpath.Path, 0)
	moduleTargets := make([]projectpath.ModulePath, 0)
	for _, target := range targets {
		if strings.TrimSpace(target.Kind) == "module" {
			modulePath, err := projectpath.ParseModule(target.ModulePath)
			if err != nil {
				return nil, nil, fmt.Errorf(
					"invalid module binding target: %w",
					err,
				)
			}
			moduleTargets = append(moduleTargets, modulePath)
			continue
		}
		if strings.TrimSpace(target.FilePath) == "" {
			continue
		}
		filePath, err := projectpath.Parse(target.FilePath)
		if err != nil {
			return nil, nil, fmt.Errorf(
				"invalid file binding target: %w",
				err,
			)
		}
		exactFileTargets = append(exactFileTargets, filePath)
	}
	return exactFileTargets, moduleTargets, nil
}

func decisionHasSemanticTargets(decoded decisionPathPolicyJSON) bool {
	if len(decoded.BindingTargets) > 0 {
		return true
	}
	for _, target := range decoded.GovernanceTargets {
		if strings.TrimSpace(target.Kind) != "" ||
			strings.TrimSpace(target.Ref) != "" ||
			target.BindingTarget != nil {
			return true
		}
	}
	for _, target := range decoded.DriftWatchTargets {
		if strings.TrimSpace(target.TargetRef) != "" ||
			strings.TrimSpace(target.Trigger) != "" ||
			target.BindingTarget != nil {
			return true
		}
	}
	return false
}
