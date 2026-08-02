package cli

import (
	kerneldb "github.com/m0n0x41d/haft/db"
	"github.com/m0n0x41d/haft/internal/testsupport/kerneldbfixture"
)

func openCurrentKernelTestStore(
	databasePath string,
) (*kerneldb.Store, error) {
	return kerneldbfixture.OpenCurrentStore(databasePath)
}
