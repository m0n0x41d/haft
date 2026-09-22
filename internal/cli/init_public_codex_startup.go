package cli

import (
	"bytes"

	"github.com/m0n0x41d/haft/internal/initplanning"
)

// Retain exact recognition of the previous generated startup allowance.
// Additional keys in these tables are operator customizations, not legacy
// defaults. Nested tool-approval tables remain outside the managed table set.
func publicPreviousCodexStartupRecords(
	projection initplanning.HostAdapterProjection,
) ([]initplanning.ManagedFragmentRecord, error) {
	if projection.Host() != initplanning.HostCodex && projection.Host() != initplanning.HostAir {
		return nil, nil
	}
	records := make([]initplanning.ManagedFragmentRecord, 0, 1)
	for _, fragment := range projection.ManagedFragments() {
		coordinate := fragment.Coordinate()
		if coordinate.Kind() != initplanning.ManagedTOMLTableSet || coordinate.Selector() != "mcp_servers.haft" {
			continue
		}
		content := fragment.Content()
		previous := bytes.Replace(content, []byte("startup_timeout_sec = 20\n"), []byte("startup_timeout_sec = 10\n"), 1)
		record, err := initplanning.NewKnownLegacyManagedFragmentRecord(fragment, previous)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, nil
}
