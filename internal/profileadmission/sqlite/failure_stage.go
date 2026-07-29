package sqlite

import (
	"slices"
	"strings"
)

type effectFailureStage string

const (
	failureStageFailureContract            effectFailureStage = "failure-contract"
	failureStageResultContract             effectFailureStage = "result-contract"
	failureStageDenialContract             effectFailureStage = "denial-contract"
	failureStageAdapterContract            effectFailureStage = "adapter-contract"
	failureStageBeginImmediate             effectFailureStage = "begin-immediate"
	failureStageAuthorityRead              effectFailureStage = "authority-read"
	failureStageAuthorityContract          effectFailureStage = "authority-contract"
	failureStageSupportContract            effectFailureStage = "support-contract"
	failureStageReplayReread               effectFailureStage = "replay-reread"
	failureStageLedgerHeadIntegrity        effectFailureStage = "ledger-head-integrity"
	failureStageRequestDigestContract      effectFailureStage = "request-digest-contract"
	failureStageAuthorityUseContract       effectFailureStage = "authority-use-contract"
	failureStageAdmissionWrite             effectFailureStage = "admission-write"
	failureStageAuthorityUseWrite          effectFailureStage = "authority-use-write"
	failureStageRevisionWrite              effectFailureStage = "revision-write"
	failureStagePrecommitReread            effectFailureStage = "precommit-reread"
	failureStageContextBeforeCommit        effectFailureStage = "context-before-commit"
	failureStageAmbiguousCommit            effectFailureStage = "ambiguous-commit"
	failureStageCommitCleanup              effectFailureStage = "commit-cleanup"
	failureStagePostcommitClose            effectFailureStage = "postcommit-close"
	failureStagePostcommitReread           effectFailureStage = "postcommit-reread"
	failureStageRollback                   effectFailureStage = "rollback"
	failureStageAuthorityDenialContract    effectFailureStage = "authority-denial-contract"
	failureStageAuthorityUseDenialContract effectFailureStage = "authority-use-denial-contract"
	failureStageServiceContract            effectFailureStage = "service-contract"
	failureStageAdapterResultContract      effectFailureStage = "adapter-result-contract"
	failureStageRestartReread              effectFailureStage = "restart-reread"
	failureStageRestartTokenContract       effectFailureStage = "restart-token-contract"
	failureStageTokenReread                effectFailureStage = "token-reread"
	failureStageTokenContract              effectFailureStage = "token-contract"
)

func (stage effectFailureStage) valid() bool {
	stages := exactFailureStages()
	exact := slices.Contains(stages, stage)
	if exact {
		return true
	}
	raw := string(stage)
	base, rollback := strings.CutSuffix(raw, "-rollback")
	if !rollback {
		return false
	}
	baseStage := effectFailureStage(base)
	return slices.Contains(stages, baseStage)
}

func (stage effectFailureStage) failureRef() string {
	if !stage.valid() {
		stage = failureStageFailureContract
	}
	return "effect-failure:profile-admission:sqlite:" + string(stage)
}

func rollbackStage(stage effectFailureStage) effectFailureStage {
	if !stage.valid() {
		return failureStageFailureContract
	}
	raw := string(stage)
	return effectFailureStage(raw + "-rollback")
}

func exactFailureStages() []effectFailureStage {
	return []effectFailureStage{
		failureStageFailureContract,
		failureStageResultContract,
		failureStageDenialContract,
		failureStageAdapterContract,
		failureStageBeginImmediate,
		failureStageAuthorityRead,
		failureStageAuthorityContract,
		failureStageSupportContract,
		failureStageReplayReread,
		failureStageLedgerHeadIntegrity,
		failureStageRequestDigestContract,
		failureStageAuthorityUseContract,
		failureStageAdmissionWrite,
		failureStageAuthorityUseWrite,
		failureStageRevisionWrite,
		failureStagePrecommitReread,
		failureStageContextBeforeCommit,
		failureStageAmbiguousCommit,
		failureStageCommitCleanup,
		failureStagePostcommitClose,
		failureStagePostcommitReread,
		failureStageRollback,
		failureStageAuthorityDenialContract,
		failureStageAuthorityUseDenialContract,
		failureStageServiceContract,
		failureStageAdapterResultContract,
		failureStageRestartReread,
		failureStageRestartTokenContract,
		failureStageTokenReread,
		failureStageTokenContract,
	}
}
