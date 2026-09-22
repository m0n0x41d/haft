package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/m0n0x41d/haft/internal/initfs"
	"github.com/m0n0x41d/haft/internal/initplanning"
)

// HostRepairProbe compiles one ordinary host request and publishes only
// host files. Its core is an inert plan fixture; it never opens a database.
func HostRepairProbe(projectRoot string, homeRoot string, hostName string, mode string) ([]byte, error) {
	request, err := compilePublicInitRequest(weakPublicInitRequest{
		invocation:  initplanning.InvocationExplicit,
		projectRoot: projectRoot,
		projectID:   "qnt_e3149c17",
		hosts:       initHostOptions{codex: hostName == "codex", air: hostName == "air"},
		overseer:    publicOverseerWeakDisabled(),
	})
	if err != nil {
		return nil, err
	}
	runtime, err := currentHostPublicationRuntimeFromProcess()
	if err != nil {
		return nil, err
	}
	runtime.userHomeRoot = homeRoot
	executableRoot := filepath.Dir(runtime.executablePath)
	runtime.executablePath = filepath.Join(executableRoot, "haft")
	basis, err := initplanning.NewUnavailableBasis("isolated host publication probe; core is not executed")
	if err != nil {
		return nil, err
	}
	databasePath := filepath.Join(homeRoot, ".haft", "projects", request.projectID, "haft.db")
	core, err := initplanning.NewCoreProjectPlanBuilder().
		ForProject(projectRoot, request.projectID).
		AtDatabase(databasePath).
		WithSchemaTransition(initplanning.CoreInitialize, 0, 60).
		WithBasis(basis).
		Build()
	if err != nil {
		return nil, err
	}
	plan, err := compilePublicHostInitPlan(request, core, runtime, publicInitMaxCarrierBytes)
	if err != nil {
		return nil, err
	}
	if mode == "plan" {
		return json.MarshalIndent(plan.Preview(), "", "  ")
	}
	if plan.Readiness() != initplanning.PlanReady {
		preview := typedPublicInitPreview{Base: plan.Preview()}
		return nil, typedPublicInitBlockedError(preview)
	}
	if mode == "race" {
		path := filepath.Join(projectRoot, ".codex", "config.toml")
		original, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil, readErr
		}
		changed := append(original, []byte("\n# Concurrent operator edit.\n")...)
		if writeErr := os.WriteFile(path, changed, 0644); writeErr != nil {
			return nil, writeErr
		}
	}
	layout, err := initplanning.NewPublicationLayout(initplanning.PublicationLayoutInput{
		ProjectRoot: projectRoot, ProjectID: request.projectID, UserHomeRoot: homeRoot,
	})
	if err != nil {
		return nil, err
	}
	publisher, err := initfs.NewHostPublisher(publicInitMaxCarrierBytes)
	if err != nil {
		return nil, err
	}
	results := make(map[string]string)
	for _, host := range plan.Hosts() {
		if mode == "race" && host.Scope() != initplanning.ScopeProject {
			continue
		}
		location, locationErr := layout.ManifestLocation(host.Host(), host.Scope())
		if locationErr != nil {
			return nil, locationErr
		}
		store, storeErr := initfs.NewManifestStore(location.Root(), location.Path(), publicInitMaxCarrierBytes)
		if storeErr != nil {
			return nil, storeErr
		}
		batch, batchErr := initplanning.BuildHostPublicationBatch(host)
		if batchErr != nil {
			return nil, batchErr
		}
		outcome, publishErr := publisher.Publish(batch, store)
		if publishErr != nil {
			return nil, publishErr
		}
		results[host.BindingID().String()] = string(outcome.Kind())
		if failure, failed := outcome.Failure(); failed {
			return nil, fmt.Errorf("publication: %v", failure)
		}
	}
	return json.MarshalIndent(results, "", "  ")
}
