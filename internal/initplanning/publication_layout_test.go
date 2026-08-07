package initplanning

import (
	"path/filepath"
	"testing"
)

func TestPublicationLayoutDerivesOneCanonicalProjectAndUserBinding(
	t *testing.T,
) {
	projectRoot := filepath.Join(t.TempDir(), "project")
	userHomeRoot := t.TempDir()
	layout, err := NewPublicationLayout(PublicationLayoutInput{
		ProjectRoot:  projectRoot,
		ProjectID:    "qnt_e3149c17",
		UserHomeRoot: userHomeRoot,
	})
	if err != nil {
		t.Fatalf("NewPublicationLayout: %v", err)
	}

	project, err := layout.ManifestLocation(HostCodex, ScopeProject)
	if err != nil {
		t.Fatalf("project ManifestLocation: %v", err)
	}
	projectPath := filepath.Join(
		projectRoot,
		".haft",
		"host-installations",
		"codex.project.json",
	)
	assertManifestLocation(
		t,
		project,
		projectRoot,
		projectPath,
		HostCodex,
		ScopeProject,
	)

	user, err := layout.ManifestLocation(HostCodex, ScopeUser)
	if err != nil {
		t.Fatalf("user ManifestLocation: %v", err)
	}
	userPath := filepath.Join(
		userHomeRoot,
		".haft",
		"host-installations",
		"codex.user.json",
	)
	assertManifestLocation(
		t,
		user,
		userHomeRoot,
		userPath,
		HostCodex,
		ScopeUser,
	)

	coordination := layout.CoordinationLocation()
	if coordination.Root() != userHomeRoot ||
		coordination.LockPath() != filepath.Join(
			userHomeRoot,
			".haft",
			"host-installations",
			"publication.lock",
		) {
		t.Fatalf("coordination location = %#v", coordination)
	}
}

func TestPublicationLayoutMakesUserBindingAndCoordinationCrossProjectGlobal(
	t *testing.T,
) {
	root := t.TempDir()
	userHomeRoot := filepath.Join(root, "user-home")
	left := mustPublicationLayout(
		t,
		filepath.Join(root, "left"),
		"qnt_e3149c17",
		userHomeRoot,
	)
	right := mustPublicationLayout(
		t,
		filepath.Join(root, "right"),
		"qnt_34f7b96f",
		userHomeRoot,
	)

	leftUser, err := left.ManifestLocation(HostCodex, ScopeUser)
	if err != nil {
		t.Fatalf("left user ManifestLocation: %v", err)
	}
	rightUser, err := right.ManifestLocation(HostCodex, ScopeUser)
	if err != nil {
		t.Fatalf("right user ManifestLocation: %v", err)
	}
	if leftUser.Path() != rightUser.Path() {
		t.Fatalf(
			"user bindings do not expose one conflict path: %s != %s",
			leftUser.Path(),
			rightUser.Path(),
		)
	}
	if left.CoordinationLocation().LockPath() !=
		right.CoordinationLocation().LockPath() {
		t.Fatal("projects under one Haft home do not share coordination")
	}

	leftProject, err := left.ManifestLocation(HostCodex, ScopeProject)
	if err != nil {
		t.Fatalf("left project ManifestLocation: %v", err)
	}
	rightProject, err := right.ManifestLocation(HostCodex, ScopeProject)
	if err != nil {
		t.Fatalf("right project ManifestLocation: %v", err)
	}
	if leftProject.Path() == rightProject.Path() {
		t.Fatal("distinct project bindings share a project manifest")
	}
}

func TestPublicationLayoutRejectsWeakOrUnknownCoordinates(t *testing.T) {
	root := t.TempDir()
	valid := PublicationLayoutInput{
		ProjectRoot:  filepath.Join(root, "project"),
		ProjectID:    "qnt_e3149c17",
		UserHomeRoot: filepath.Join(root, "home"),
	}
	cases := map[string]PublicationLayoutInput{
		"relative project root": {
			ProjectRoot:  "project",
			ProjectID:    valid.ProjectID,
			UserHomeRoot: valid.UserHomeRoot,
		},
		"filesystem project root": {
			ProjectRoot:  string(filepath.Separator),
			ProjectID:    valid.ProjectID,
			UserHomeRoot: valid.UserHomeRoot,
		},
		"invalid project ID": {
			ProjectRoot:  valid.ProjectRoot,
			ProjectID:    "project",
			UserHomeRoot: valid.UserHomeRoot,
		},
		"relative Haft root": {
			ProjectRoot:  valid.ProjectRoot,
			ProjectID:    valid.ProjectID,
			UserHomeRoot: ".",
		},
		"filesystem Haft root": {
			ProjectRoot:  valid.ProjectRoot,
			ProjectID:    valid.ProjectID,
			UserHomeRoot: string(filepath.Separator),
		},
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := NewPublicationLayout(input); err == nil {
				t.Fatal("weak publication layout was accepted")
			}
		})
	}

	layout, err := NewPublicationLayout(valid)
	if err != nil {
		t.Fatalf("NewPublicationLayout(valid): %v", err)
	}
	if _, err := layout.ManifestLocation(
		HostID("unknown"),
		ScopeProject,
	); err == nil {
		t.Fatal("unknown host received a manifest location")
	}
	if _, err := layout.ManifestLocation(
		HostCodex,
		InstallScope("unknown"),
	); err == nil {
		t.Fatal("unknown scope received a manifest location")
	}
}

func assertManifestLocation(
	t *testing.T,
	location ManifestLocation,
	root string,
	path string,
	host HostID,
	scope InstallScope,
) {
	t.Helper()
	if location.Root() != root ||
		location.Path() != path ||
		location.ManifestLockPath() != path+".lock" ||
		location.JournalPath() != path+".pending" ||
		location.Host() != host ||
		location.Scope() != scope {
		t.Fatalf("manifest location = %#v", location)
	}
}

func mustPublicationLayout(
	t *testing.T,
	projectRoot string,
	projectID string,
	userHomeRoot string,
) PublicationLayout {
	t.Helper()
	layout, err := NewPublicationLayout(PublicationLayoutInput{
		ProjectRoot:  projectRoot,
		ProjectID:    projectID,
		UserHomeRoot: userHomeRoot,
	})
	if err != nil {
		t.Fatalf("NewPublicationLayout: %v", err)
	}
	return layout
}
