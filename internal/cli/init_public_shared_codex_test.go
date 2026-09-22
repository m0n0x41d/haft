package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/m0n0x41d/haft/internal/initfs"
	"github.com/m0n0x41d/haft/internal/initplanning"
)

func TestPublicCodexAirSequentialInitSharesGeneratedTables(t *testing.T) {
	orders := [][]initplanning.HostID{
		{initplanning.HostCodex, initplanning.HostAir},
		{initplanning.HostAir, initplanning.HostCodex},
	}
	for _, order := range orders {
		t.Run(string(order[0]), func(t *testing.T) {
			request, runtime, core := discoveredCodexFixture(t)
			config, err := currentCodexTOMLFragmentWithStartup(currentCoherentHostContext{projectID: request.projectID}, 10)
			if err != nil {
				t.Fatal(err)
			}
			custom := []byte("\n[mcp_servers.haft.tools.haft_method]\napproval_mode = \"prompt\"\n")
			config = append(config, custom...)
			configPath := filepath.Join(request.projectRoot, ".codex", "config.toml")
			writeDiscoveredCodexFixture(t, configPath, config)
			for _, host := range order {
				selected := sharedCodexHostRequest(t, request, host)
				plan, err := compilePublicHostInitPlan(selected, core, runtime, publicInitMaxCarrierBytes)
				if err != nil {
					t.Fatal(err)
				}
				publishDiscoveredCodexFixture(t, plan, runtime, initfs.HostPublicationApplied)
			}
			updated, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(updated, custom) || !bytes.Contains(updated, []byte("startup_timeout_sec = 20")) {
				t.Fatalf("shared timeout or approvals differ: %s", updated)
			}
			for _, host := range order {
				selected := sharedCodexHostRequest(t, request, host)
				plan, err := compilePublicHostInitPlan(selected, core, runtime, publicInitMaxCarrierBytes)
				if err != nil {
					t.Fatal(err)
				}
				publishDiscoveredCodexFixture(t, plan, runtime, initfs.HostPublicationAlreadyCurrent)
			}
		})
	}
}

func sharedCodexHostRequest(t *testing.T, request publicInitRequest, host initplanning.HostID) publicInitRequest {
	t.Helper()
	selected, err := compilePublicInitRequest(weakPublicInitRequest{
		invocation:  initplanning.InvocationExplicit,
		projectRoot: request.projectRoot,
		projectID:   request.projectID,
		hosts:       initHostOptions{codex: host == initplanning.HostCodex, air: host == initplanning.HostAir},
		overseer:    publicOverseerWeakDisabled(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return selected
}
