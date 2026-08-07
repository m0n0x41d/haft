package agenthostrestart

import "time"

// LiveMCPReceipt is emitted only by the bound live haft serve process while it
// handles a real status request through the host transport.
type LiveMCPReceipt struct {
	RestartID             string
	CheckpointBasisDigest string
	Nonce                 string
	PID                   int
	ExecutablePath        string
	ExecutableDigest      string
	ProjectRoot           string
	ProcessStartedAt      time.Time
	FulfilledAt           time.Time
}

// ResumeFallbackReceipt is emitted by the detached launchd-owned supervisor
// after exactly one resumed turn has acquired the checkpoint writer lease.
type ResumeFallbackReceipt struct {
	RestartID             string
	CheckpointBasisDigest string
	Nonce                 string
	WakeCount             uint8
	ClearedAt             time.Time
}
