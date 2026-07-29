package sqlite

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/m0n0x41d/haft/internal/projectprofile"
)

const (
	classifierVersionBinding = "classifier_version"
	policyVersionBinding     = "policy_version"
	projectRootBinding       = "project_root"
	sessionRefBinding        = "session_ref"
)

func validateProjectRoot(
	projectRoot projectprofile.ProjectRootV1,
) (projectprofile.ProjectRootV1, error) {
	raw := projectRoot.String()
	canonical, err := projectprofile.NewProjectRootV1(raw)
	if err != nil {
		return projectprofile.ProjectRootV1{}, fmt.Errorf("validate project root: %w", err)
	}
	rootPath := canonical.String()
	info, err := os.Stat(rootPath)
	if err != nil {
		return projectprofile.ProjectRootV1{}, fmt.Errorf("inspect physical project root: %w", err)
	}
	if !info.IsDir() {
		return projectprofile.ProjectRootV1{}, fmt.Errorf("physical project root must be a directory")
	}
	physicalPath, err := filepath.EvalSymlinks(rootPath)
	if err != nil {
		return projectprofile.ProjectRootV1{}, fmt.Errorf("resolve physical project root: %w", err)
	}
	physicalPath = filepath.Clean(physicalPath)
	if physicalPath != rootPath {
		return projectprofile.ProjectRootV1{}, fmt.Errorf("project root must use canonical physical form; symlink aliases are not admitted")
	}
	return canonical, nil
}

func validateMethodBindings(
	bindings projectprofile.MethodParameterBindings,
	projectRoot projectprofile.ProjectRootV1,
) error {
	classifierRaw, err := requiredMethodBinding(bindings, classifierVersionBinding)
	if err != nil {
		return err
	}
	_, err = projectprofile.NewClassifierVersion(classifierRaw)
	if err != nil {
		return fmt.Errorf("validate classifier_version binding: %w", err)
	}
	policyRaw, err := requiredMethodBinding(bindings, policyVersionBinding)
	if err != nil {
		return err
	}
	_, err = projectprofile.NewPolicyVersion(policyRaw)
	if err != nil {
		return fmt.Errorf("validate policy_version binding: %w", err)
	}
	rootRaw, err := requiredMethodBinding(bindings, projectRootBinding)
	if err != nil {
		return err
	}
	boundRoot, err := projectprofile.NewProjectRootV1(rootRaw)
	if err != nil {
		return fmt.Errorf("validate project_root binding: %w", err)
	}
	if boundRoot != projectRoot {
		return fmt.Errorf(
			"project_root binding %q does not match requested root %q",
			boundRoot.String(),
			projectRoot.String(),
		)
	}
	sessionRaw, err := requiredMethodBinding(bindings, sessionRefBinding)
	if err != nil {
		return err
	}
	_, err = projectprofile.NewSessionRef(sessionRaw)
	if err != nil {
		return fmt.Errorf("validate session_ref binding: %w", err)
	}
	return nil
}

func requiredMethodBinding(
	bindings projectprofile.MethodParameterBindings,
	name string,
) (string, error) {
	value, found := bindings.ValueFor(name)
	if !found {
		return "", fmt.Errorf("profile-onboarding Work is missing %q binding", name)
	}
	return value, nil
}
