package agenthostrestart

import "time"

// VerifiedRuntimeSnapshot is the secret-free projection of one completed
// agent-host restart. It contains safe coordinates and digests only; private
// nonces and receipt bodies never cross this package boundary.
type VerifiedRuntimeSnapshot struct {
	RestartID                      string
	ThreadID                       string
	ResumeIntentDigest             string
	CheckpointState                string
	CheckpointAttempt              uint8
	CheckpointCreatedAt            time.Time
	RestartCheckpointDigest        string
	PreparedTaskRuntimePID         int
	PreparedTaskRuntimeStartedAt   time.Time
	PreparedTaskRuntimeExecutable  string
	PreparedTaskRuntimeArgsDigest  string
	InstalledExecutablePath        string
	InstalledExecutableDigest      string
	ProjectRoot                    string
	LiveMCPPID                     int
	LiveMCPStartedAt               time.Time
	LiveMCPFulfilledAt             time.Time
	LiveMCPExecutablePath          string
	LiveMCPExecutableDigest        string
	LiveMCPProjectRoot             string
	LiveMCPReceiptDigest           string
	FallbackReceiptDigest          string
	FallbackWakeCount              uint8
	FallbackClearedAt              time.Time
	ExactTaskResumeCount           uint8
	SingleWriterObserved           bool
	LaunchdRemovalObserved         bool
	PrivateStateGitignoredObserved bool
	CandidateDigestReserved        bool
	TemporaryStagesAbsent          bool
}
