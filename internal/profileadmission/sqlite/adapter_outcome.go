package sqlite

type adapterOutcome struct {
	kind      AdmissionResultKind
	admission canonicalAdmissionMaterial
	delivery  CanonicalAdmissionDelivery
	denials   []AdmissionDenial
	failure   AdmissionFailure
}

func newAdapterAdmitted(
	admission canonicalAdmissionMaterial,
	delivery CanonicalAdmissionDelivery,
) adapterOutcome {
	if !delivery.valid() || validateCanonicalAdmissionMaterial(admission) != nil {
		return newAdapterFailed(
			AdmissionCommitOutcomeUnknown,
			failureStageResultContract,
		)
	}
	return adapterOutcome{
		kind:      AdmissionResultAdmitted,
		admission: admission,
		delivery:  delivery,
	}
}

func newAdapterDenied(denials []AdmissionDenial) adapterOutcome {
	if len(denials) == 0 {
		return newAdapterFailed(
			AdmissionDefinitelyNotCommitted,
			failureStageDenialContract,
		)
	}
	return adapterOutcome{
		kind:    AdmissionResultNotAdmitted,
		denials: append([]AdmissionDenial{}, denials...),
	}
}

func newAdapterFailed(
	posture AdmissionCommitPosture,
	stage effectFailureStage,
) adapterOutcome {
	failure := AdmissionFailure{
		commitPosture: posture,
		failureRef:    stage.failureRef(),
	}
	if !failure.valid() {
		failure = AdmissionFailure{
			commitPosture: AdmissionCommitOutcomeUnknown,
			failureRef:    failureStageFailureContract.failureRef(),
		}
	}
	return adapterOutcome{
		kind:    AdmissionResultWriteFailed,
		failure: failure,
	}
}

func (outcome adapterOutcome) DeliveryPosture() CanonicalAdmissionDelivery {
	return outcome.delivery
}
