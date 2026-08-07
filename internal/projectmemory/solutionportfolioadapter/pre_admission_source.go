package solutionportfolioadapter

import (
	"github.com/m0n0x41d/haft/internal/projectmemory/recordatconcern"
	"github.com/m0n0x41d/haft/internal/typedmemorystore"
)

var (
	ErrPreAdmissionSourceStageInvalid      = recordatconcern.ErrPreAdmissionSourceStageInvalid
	ErrPreAdmissionSourceUnavailable       = recordatconcern.ErrPreAdmissionSourceUnavailable
	ErrPreAdmissionFallbackProviderMissing = recordatconcern.ErrPreAdmissionFallbackProviderMissing
)

type PreAdmissionSourceStage = recordatconcern.PreAdmissionSourceStage
type PreAdmissionObservableInputProvider = recordatconcern.PreAdmissionObservableInputProvider

func SealPreAdmissionSourceStage(
	candidate ValidCandidate,
) (PreAdmissionSourceStage, error) {
	return recordatconcern.SealPreAdmissionSourceStage(candidate)
}

func NewPreAdmissionObservableInputProvider(
	stage PreAdmissionSourceStage,
	fallback typedmemorystore.ObservableInputContentProvider,
) (PreAdmissionObservableInputProvider, error) {
	return recordatconcern.NewPreAdmissionObservableInputProvider(
		stage,
		fallback,
	)
}
