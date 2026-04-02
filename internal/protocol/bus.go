package protocol

import (
	"encoding/json"

	"github.com/m0n0x41d/haft/internal/jsonrpc"
	"github.com/m0n0x41d/haft/logger"
)

// Bus wraps a JSON-RPC server and provides typed send methods
// for all protocol events. This is the coordinator's interface
// to the TUI — replaces the old view.Bus channel.
type Bus struct {
	rpc *jsonrpc.Server
}

// NewBus creates a protocol bus wrapping a JSON-RPC server.
func NewBus(rpc *jsonrpc.Server) *Bus {
	return &Bus{rpc: rpc}
}

// RPC returns the underlying JSON-RPC server for direct access.
func (b *Bus) RPC() *jsonrpc.Server {
	return b.rpc
}

// --- Notifications (fire-and-forget) ---

func (b *Bus) SendInit(p Init) error {
	logger.Debug().Str("component", "bus").Str("method", "init").Str("session", p.Session.ID).Msg("bus.send")
	return b.rpc.Send(MethodInit, p)
}

func (b *Bus) SendMsgUpdate(p MsgUpdate) error {
	logger.Debug().Str("component", "bus").Str("method", "msg.update").Str("msg_id", p.ID).Bool("streaming", p.Streaming).Int("text_len", len(p.Text)).Int("tools", len(p.Tools)).Msg("bus.send")
	return b.rpc.Send(MethodMsgUpdate, p)
}

func (b *Bus) SendToolStart(p ToolStart) error {
	logger.Info().Str("component", "bus").Str("method", "tool.start").Str("call_id", p.CallID).Str("name", p.Name).Str("subagent", p.SubagentID).Msg("bus.send")
	return b.rpc.Send(MethodToolStart, p)
}

func (b *Bus) SendToolProgress(p ToolProgress) error {
	return b.rpc.Send(MethodToolProgress, p)
}

func (b *Bus) SendToolDone(p ToolDone) error {
	logger.Info().Str("component", "bus").Str("method", "tool.done").Str("call_id", p.CallID).Str("name", p.Name).Bool("error", p.IsError).Int("output_len", len(p.Output)).Msg("bus.send")
	return b.rpc.Send(MethodToolDone, p)
}

func (b *Bus) SendTokenUpdate(p TokenUpdate) error {
	return b.rpc.Send(MethodTokenUpdate, p)
}

func (b *Bus) SendSessionTitle(p SessionTitle) error {
	logger.Debug().Str("component", "bus").Str("method", "session.title").Str("title", p.Title).Msg("bus.send")
	return b.rpc.Send(MethodSessionTitle, p)
}

func (b *Bus) SendCycleUpdate(p CycleUpdate) error {
	logger.Debug().Str("component", "bus").Str("method", "cycle.update").Str("cycle_id", p.CycleID).Str("phase", p.Phase).Msg("bus.send")
	return b.rpc.Send(MethodCycleUpdate, p)
}

func (b *Bus) SendSubagentStart(p SubagentStart) error {
	logger.Info().Str("component", "bus").Str("method", "subagent.start").Str("subagent_id", p.SubagentID).Str("name", p.Name).Msg("bus.send")
	return b.rpc.Send(MethodSubagentStart, p)
}

func (b *Bus) SendSubagentDone(p SubagentDone) error {
	logger.Info().Str("component", "bus").Str("method", "subagent.done").Str("subagent_id", p.SubagentID).Bool("error", p.IsError).Msg("bus.send")
	return b.rpc.Send(MethodSubagentDone, p)
}

func (b *Bus) SendOverseerAlert(p OverseerAlert) error {
	logger.Info().Str("component", "bus").Str("method", "overseer.alert").Int("count", len(p.Alerts)).Msg("bus.send")
	return b.rpc.Send(MethodOverseerAlert, p)
}

func (b *Bus) SendDriftUpdate(p DriftUpdate) error {
	logger.Debug().Str("component", "bus").Str("method", "drift.update").Int("drifted", p.Drifted).Msg("bus.send")
	return b.rpc.Send(MethodDriftUpdate, p)
}

func (b *Bus) SendLSPUpdate(p LSPUpdate) error {
	return b.rpc.Send(MethodLSPUpdate, p)
}

func (b *Bus) SendError(msg string) error {
	logger.Error().Str("component", "bus").Str("method", "error").Str("message", msg).Msg("bus.send")
	return b.rpc.Send(MethodError, Error{Message: msg})
}

func (b *Bus) SendCoordDone() error {
	logger.Info().Str("component", "bus").Str("method", "coord.done").Msg("bus.send")
	return b.rpc.Send(MethodCoordDone, CoordDone{})
}

// --- Requests (block until TUI responds) ---

// AskPermission sends a permission request and blocks until the TUI responds.
func (b *Bus) AskPermission(p PermissionAsk) (PermissionReply, error) {
	logger.Info().Str("component", "bus").Str("method", "permission.ask").Str("tool", p.ToolName).Msg("bus.request")
	raw, err := b.rpc.Request(MethodPermissionAsk, p)
	if err != nil {
		logger.Error().Str("component", "bus").Err(err).Msg("bus.request_error")
		return PermissionReply{}, err
	}
	var reply PermissionReply
	err = json.Unmarshal(raw, &reply)
	logger.Info().Str("component", "bus").Str("method", "permission.reply").Str("action", reply.Action).Msg("bus.response")
	return reply, err
}

// AskQuestion sends a question to the user and blocks until they answer.
func (b *Bus) AskQuestion(p QuestionAsk) (QuestionReply, error) {
	logger.Info().Str("component", "bus").Str("method", "question.ask").Msg("bus.request")
	raw, err := b.rpc.Request(MethodQuestionAsk, p)
	if err != nil {
		logger.Error().Str("component", "bus").Err(err).Msg("bus.request_error")
		return QuestionReply{}, err
	}
	var reply QuestionReply
	err = json.Unmarshal(raw, &reply)
	logger.Info().Str("component", "bus").Str("method", "question.reply").Msg("bus.response")
	return reply, err
}
