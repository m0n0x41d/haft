package project

import (
	"os"
	"path/filepath"
	"strings"
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

func InspectReadiness(projectRoot string) (ReadinessFacts, error) {
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

	hasSpecs := hasMinimumSpecificationSet(root)
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

	return hasActiveSpecCarrier(report, "target-system") &&
		hasActiveSpecCarrier(report, "enabling-system")
}

func hasActiveSpecCarrier(report SpecCheckReport, kind string) bool {
	for _, document := range report.Documents {
		if document.Kind != kind {
			continue
		}

		return document.ActiveSpecSections > 0
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
