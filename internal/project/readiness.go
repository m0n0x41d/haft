package project

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/m0n0x41d/haft/internal/projectprofile"
)

type Readiness string

const (
	ReadinessReady        Readiness = "ready"
	ReadinessNeedsInit    Readiness = "needs_init"
	ReadinessNeedsOnboard Readiness = "needs_onboard"
	ReadinessMissing      Readiness = "missing"
)

type ReadinessFacts struct {
	Status   Readiness
	Exists   bool
	HasHaft  bool
	HasSpecs bool
}

type specRequirement struct {
	path    string
	markers []string
}

var readinessSpecRequirements = []specRequirement{
	{path: filepath.Join(".haft", "workflow.md"), markers: []string{"## Defaults"}},
}

type activeSpecKindRequirement struct {
	documentKind string
	sectionKinds []string
}

var readinessActiveSpecKindRequirements = []activeSpecKindRequirement{
	{
		documentKind: string(SpecDocumentKindTargetSystem),
		sectionKinds: []string{"target.environment", "target.role", "target.boundary"},
	},
	{
		documentKind: string(SpecDocumentKindSoftwareSystem),
		sectionKinds: []string{"software.role", "software.functional_behavior", "software.interfaces", "software.constraints"},
	},
}

func InspectReadiness(projectRoot string) (ReadinessFacts, error) {
	return inspectReadiness(projectRoot, hasMinimumSpecificationSet)
}

// InspectReadinessForScope applies a canonical scope-local applicability
// projection. NotApplicable capabilities are omitted from readiness
// requirements; Underdetermined capabilities keep readiness fail-closed.
func InspectReadinessForScope(
	projectRoot string,
	applicability ProjectSpecificationSetApplicability,
) (ReadinessFacts, error) {
	if !applicability.Valid() {
		return ReadinessFacts{}, fmt.Errorf(
			"project specification applicability is invalid",
		)
	}
	minimumSpecificationSet := func(root string) bool {
		return hasMinimumSpecificationSetForScope(root, applicability)
	}
	return inspectReadiness(projectRoot, minimumSpecificationSet)
}

func inspectReadiness(
	projectRoot string,
	hasMinimumSpecifications func(string) bool,
) (ReadinessFacts, error) {
	root := strings.TrimSpace(projectRoot)
	if root == "" {
		return ReadinessFacts{Status: ReadinessMissing}, nil
	}

	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return ReadinessFacts{Status: ReadinessMissing}, nil
		}
		return ReadinessFacts{}, err
	}
	if !info.IsDir() {
		return ReadinessFacts{Status: ReadinessMissing}, nil
	}

	hasHaft := fileExists(filepath.Join(root, ".haft", configFile))
	if !hasHaft {
		return ReadinessFacts{
			Status: ReadinessNeedsInit,
			Exists: true,
		}, nil
	}

	hasSpecs := hasMinimumSpecifications(root)
	if !hasSpecs {
		return ReadinessFacts{
			Status:  ReadinessNeedsOnboard,
			Exists:  true,
			HasHaft: true,
		}, nil
	}

	return ReadinessFacts{
		Status:   ReadinessReady,
		Exists:   true,
		HasHaft:  true,
		HasSpecs: true,
	}, nil
}

func hasMinimumSpecificationSet(projectRoot string) bool {
	for _, requirement := range readinessSpecRequirements {
		if !fileContainsAll(filepath.Join(projectRoot, requirement.path), requirement.markers) {
			return false
		}
	}

	report, err := CheckSpecificationSet(projectRoot)
	if err != nil {
		return false
	}
	if report.HasFindings() {
		return false
	}

	sections, err := LoadSpecSections(projectRoot)
	if err != nil {
		return false
	}

	return hasRequiredActiveSpecKinds(sections, readinessActiveSpecKindRequirements)
}

func hasMinimumSpecificationSetForScope(
	projectRoot string,
	applicability ProjectSpecificationSetApplicability,
) bool {
	for _, requirement := range readinessSpecRequirements {
		path := filepath.Join(projectRoot, requirement.path)
		if !fileContainsAll(path, requirement.markers) {
			return false
		}
	}

	report, err := CheckSpecificationSetForScope(projectRoot, applicability)
	if err != nil || report.HasFindings() {
		return false
	}

	sections, err := LoadSpecSectionsForScope(projectRoot, applicability)
	if err != nil {
		return false
	}
	requirements, err := readinessActiveSpecKindRequirementsFor(applicability)
	if err != nil {
		return false
	}

	return hasRequiredActiveSpecKinds(sections, requirements)
}

func readinessActiveSpecKindRequirementsFor(
	applicability ProjectSpecificationSetApplicability,
) ([]activeSpecKindRequirement, error) {
	if !applicability.Valid() {
		return nil, fmt.Errorf(
			"project specification applicability is invalid",
		)
	}
	requirements := append(
		[]activeSpecKindRequirement{},
		readinessActiveSpecKindRequirements...,
	)
	selected := slices.DeleteFunc(
		requirements,
		func(requirement activeSpecKindRequirement) bool {
			documentKind := SpecDocumentKind(requirement.documentKind)
			member, found := applicability.Member(documentKind)
			return !found || member.Kind() != projectprofile.CapabilityRequired
		},
	)
	return selected, nil
}

func hasRequiredActiveSpecKinds(
	sections []SpecSection,
	requirements []activeSpecKindRequirement,
) bool {
	for _, requirement := range requirements {
		for _, sectionKind := range requirement.sectionKinds {
			if !hasActiveSpecKind(sections, requirement.documentKind, sectionKind) {
				return false
			}
		}
	}
	return true
}

func hasActiveSpecKind(sections []SpecSection, documentKind string, sectionKind string) bool {
	for _, section := range sections {
		if section.DocumentKind != documentKind || section.Kind != sectionKind {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(section.Status), string(SpecSectionStateActive)) {
			return true
		}
	}
	return false
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func fileContainsAll(path string, markers []string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}

	content := string(data)
	for _, marker := range markers {
		if !strings.Contains(content, marker) {
			return false
		}
	}

	return true
}
