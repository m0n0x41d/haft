package cli

import (
	"github.com/m0n0x41d/haft/internal/agent"
	"github.com/m0n0x41d/haft/internal/protocol"
)

func applyModeUpdate(sess *agent.Session, update protocol.ModeUpdate) bool {
	mode, ok := agent.ParseExecutionMode(update.Mode)
	if !ok {
		return false
	}

	sess.SetExecutionMode(mode)
	return true
}

func sessionInfo(sess *agent.Session) protocol.SessionInfo {
	mode := string(sess.ExecutionMode())

	return protocol.SessionInfo{
		ID:          sess.ID,
		Title:       sess.Title,
		Model:       sess.Model,
		Mode:        mode,
		Interaction: mode,
		Yolo:        sess.Yolo,
	}
}
