package initexecution

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/initfs"
	"github.com/m0n0x41d/haft/internal/initplanning"
)

func BindCanonicalPublicationResources(
	plan initplanning.InitPlan,
	userHomeRoot string,
	maxManifestBytes int64,
) (
	HostManifestRegistry,
	initfs.PublicationCoordinator,
	error,
) {
	core := plan.Core()
	layout, err := initplanning.NewPublicationLayout(
		initplanning.PublicationLayoutInput{
			ProjectRoot:  core.ProjectRoot(),
			ProjectID:    core.ProjectID().String(),
			UserHomeRoot: userHomeRoot,
		},
	)
	if err != nil {
		return HostManifestRegistry{}, initfs.PublicationCoordinator{}, err
	}
	hostPlans := plan.Hosts()
	bindings := make([]HostManifestBinding, 0, len(hostPlans))
	for _, hostPlan := range hostPlans {
		location, locationErr := layout.ManifestLocation(
			hostPlan.Host(),
			hostPlan.Scope(),
		)
		if locationErr != nil {
			return HostManifestRegistry{},
				initfs.PublicationCoordinator{},
				locationErr
		}
		for _, targetRoot := range hostPlan.TargetRoots() {
			if !pathInsideRoot(targetRoot, location.Root()) {
				return HostManifestRegistry{},
					initfs.PublicationCoordinator{},
					fmt.Errorf(
						"host %s target root %s is outside canonical %s scope",
						hostPlan.Host(),
						targetRoot,
						hostPlan.Scope(),
					)
			}
		}
		store, storeErr := initfs.NewManifestStore(
			location.Root(),
			location.Path(),
			maxManifestBytes,
		)
		if storeErr != nil {
			return HostManifestRegistry{},
				initfs.PublicationCoordinator{},
				storeErr
		}
		if store.LockPath() != location.ManifestLockPath() ||
			store.JournalPath() != location.JournalPath() {
			return HostManifestRegistry{},
				initfs.PublicationCoordinator{},
				fmt.Errorf(
					"host %s publication carriers differ from canonical layout",
					hostPlan.Host(),
				)
		}
		binding, bindingErr := NewHostManifestBinding(
			hostPlan.BindingID(),
			store,
		)
		if bindingErr != nil {
			return HostManifestRegistry{},
				initfs.PublicationCoordinator{},
				bindingErr
		}
		bindings = append(bindings, binding)
	}
	registry, err := NewHostManifestRegistry(bindings)
	if err != nil {
		return HostManifestRegistry{}, initfs.PublicationCoordinator{}, err
	}
	coordination := layout.CoordinationLocation()
	coordinator, err := initfs.NewPublicationCoordinator(
		coordination.Root(),
		coordination.LockPath(),
	)
	if err != nil {
		return HostManifestRegistry{}, initfs.PublicationCoordinator{}, err
	}
	return registry, coordinator, nil
}
