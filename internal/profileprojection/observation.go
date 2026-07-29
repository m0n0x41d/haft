package profileprojection

import (
	"errors"
	"fmt"
	"os"

	"github.com/m0n0x41d/haft/internal/projectprofile"
)

type observationKind string

const (
	observationMissing    observationKind = "missing"
	observationMatched    observationKind = "matched"
	observationDrifted    observationKind = "drifted"
	observationUnreadable observationKind = "unreadable"
)

type projectionObservation struct {
	kind   observationKind
	digest projectprofile.ContentDigest
	detail string
}

func unreadableProjectionObservation(err error) projectionObservation {
	return projectionObservation{
		kind:   observationUnreadable,
		detail: fmt.Sprintf("profile projection cannot be accessed safely: %v", err),
	}
}

func projectionExists(path string) (bool, string, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, "", nil
	}
	if err != nil {
		return false, "", err
	}
	mode := info.Mode()
	if mode.IsRegular() {
		return true, "projection carrier exists without a canonical ledger revision", nil
	}
	return true, "non-regular projection path exists without a canonical ledger revision", nil
}

func debtReason(observation projectionObservation) string {
	switch observation.kind {
	case observationMissing:
		return "projection_missing"
	case observationDrifted:
		return "projection_drift"
	case observationUnreadable:
		return "projection_unreadable"
	default:
		return "projection_not_synchronized"
	}
}
