package initplanning

import (
	"fmt"
	"path/filepath"

	"github.com/m0n0x41d/haft/internal/projectidentity"
)

const hostInstallationDirectory = "host-installations"

type PublicationLayoutInput struct {
	ProjectRoot  string
	ProjectID    string
	UserHomeRoot string
}

type PublicationLayout struct {
	projectRoot  string
	projectID    projectidentity.ProjectID
	userHomeRoot string
	coordination PublicationCoordinationLocation
}

func NewPublicationLayout(
	input PublicationLayoutInput,
) (PublicationLayout, error) {
	projectRoot, err := parseCanonicalAbsolutePath(input.ProjectRoot)
	if err != nil {
		return PublicationLayout{}, fmt.Errorf(
			"publication project root: %w",
			err,
		)
	}
	projectID, err := projectidentity.ParseProjectID(input.ProjectID)
	if err != nil {
		return PublicationLayout{}, fmt.Errorf(
			"publication project identity: %w",
			err,
		)
	}
	userHomeRoot, err := parseCanonicalAbsolutePath(input.UserHomeRoot)
	if err != nil {
		return PublicationLayout{}, fmt.Errorf(
			"publication user home root: %w",
			err,
		)
	}
	userHaftRoot := filepath.Join(userHomeRoot, ".haft")
	coordinationDirectory := filepath.Join(
		userHaftRoot,
		hostInstallationDirectory,
	)
	coordinationLock := filepath.Join(
		coordinationDirectory,
		"publication.lock",
	)
	return PublicationLayout{
		projectRoot:  projectRoot,
		projectID:    projectID,
		userHomeRoot: userHomeRoot,
		coordination: PublicationCoordinationLocation{
			root:     userHomeRoot,
			lockPath: coordinationLock,
		},
	}, nil
}

type ManifestLocation struct {
	root        string
	path        string
	host        HostID
	scope       InstallScope
	lockPath    string
	journalPath string
}

func (layout PublicationLayout) ManifestLocation(
	host HostID,
	scope InstallScope,
) (ManifestLocation, error) {
	if _, known := knownHosts[host]; !known {
		return ManifestLocation{}, fmt.Errorf(
			"manifest location host is not canonical",
		)
	}
	scopeRoots := map[InstallScope]string{
		ScopeProject: layout.projectRoot,
		ScopeUser:    layout.userHomeRoot,
	}
	root, known := scopeRoots[scope]
	if !known {
		return ManifestLocation{}, fmt.Errorf(
			"manifest location scope is not canonical",
		)
	}
	if root == "" || layout.projectID.String() == "" {
		return ManifestLocation{}, fmt.Errorf(
			"publication layout is invalid",
		)
	}
	directory := filepath.Join(
		root,
		".haft",
		hostInstallationDirectory,
	)
	fileName := string(host) + "." + string(scope) + ".json"
	path := filepath.Join(directory, fileName)
	return ManifestLocation{
		root:        root,
		path:        path,
		host:        host,
		scope:       scope,
		lockPath:    path + ".lock",
		journalPath: path + ".pending",
	}, nil
}

func (location ManifestLocation) Root() string {
	return location.root
}

func (location ManifestLocation) Path() string {
	return location.path
}

func (location ManifestLocation) Host() HostID {
	return location.host
}

func (location ManifestLocation) Scope() InstallScope {
	return location.scope
}

func (location ManifestLocation) ManifestLockPath() string {
	return location.lockPath
}

func (location ManifestLocation) JournalPath() string {
	return location.journalPath
}

type PublicationCoordinationLocation struct {
	root     string
	lockPath string
}

func (layout PublicationLayout) CoordinationLocation() PublicationCoordinationLocation {
	return layout.coordination
}

func (location PublicationCoordinationLocation) Root() string {
	return location.root
}

func (location PublicationCoordinationLocation) LockPath() string {
	return location.lockPath
}
