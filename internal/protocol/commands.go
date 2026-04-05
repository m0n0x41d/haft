package protocol

import "encoding/json"

// ---------------------------------------------------------------------------
// TUI → Backend notifications
// ---------------------------------------------------------------------------

// Submit sends a user message to the coordinator.
type Submit struct {
	Text        string       `json:"text"`
	DisplayText string       `json:"displayText,omitempty"`
	Attachments []Attachment `json:"attachments,omitempty"`
}

type Attachment struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	MIMEType string `json:"mimeType,omitempty"`
	IsImage  bool   `json:"isImage"`
	Content  string `json:"content,omitempty"` // file content (text)
	Data     string `json:"data,omitempty"`    // base64 (image)
}

// Cancel requests cancellation of the current coordinator run.
type Cancel struct{}

// ModeUpdate updates session-level execution modes.
type ModeUpdate struct {
	Mode string `json:"mode,omitempty"`
	Yolo bool   `json:"yolo,omitempty"`
}

func (m *ModeUpdate) UnmarshalJSON(data []byte) error {
	type modeUpdatePayload struct {
		Mode       string `json:"mode,omitempty"`
		Autonomous *bool  `json:"autonomous,omitempty"`
		Yolo       bool   `json:"yolo,omitempty"`
	}

	var payload modeUpdatePayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}

	m.Mode = payload.Mode
	m.Yolo = payload.Yolo

	if payload.Mode != "" {
		return nil
	}
	if payload.Autonomous == nil {
		return nil
	}
	if *payload.Autonomous {
		m.Mode = "autonomous"
		return nil
	}
	m.Mode = "symbiotic"
	return nil
}

// Resize notifies the backend of terminal size change.
type Resize struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// ---------------------------------------------------------------------------
// TUI → Backend requests (expect response)
// ---------------------------------------------------------------------------

// SessionListRequest asks for recent sessions.
type SessionListRequest struct {
	Limit int `json:"limit,omitempty"`
}

// SessionListResponse returns recent sessions.
type SessionListResponse struct {
	Sessions []SessionInfo `json:"sessions"`
}

// SessionResumeRequest asks to load a previous session.
type SessionResumeRequest struct {
	SessionID string `json:"sessionId"`
}

// SessionResumeResponse returns the session and its messages.
type SessionResumeResponse struct {
	Session  SessionInfo `json:"session"`
	Messages []MsgInfo   `json:"messages"`
}

// ModelListRequest asks for available models.
type ModelListRequest struct{}

// ModelListResponse returns available models.
type ModelListResponse struct {
	Models []ModelInfo `json:"models"`
}

type ModelInfo struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Provider      string `json:"provider"`
	ContextWindow int    `json:"contextWindow"`
	CanReason     bool   `json:"canReason"`
}

// ModelSwitchRequest asks to change the active model.
type ModelSwitchRequest struct {
	Model    string `json:"model"`
	Provider string `json:"provider,omitempty"`
	APIKey   string `json:"apiKey,omitempty"`
}

// ModelSwitchResponse confirms the switch.
type ModelSwitchResponse struct {
	OK bool `json:"ok"`
}

// CompactRequest triggers context compaction.
type CompactRequest struct{}

// CompactResponse returns compaction results.
type CompactResponse struct {
	Before int `json:"before"`
	After  int `json:"after"`
}

// FileListRequest asks for project files (for @mention picker).
type FileListRequest struct {
	Filter string `json:"filter,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

// FileListResponse returns project files.
type FileListResponse struct {
	Files []FileInfo `json:"files"`
}

type FileInfo struct {
	Path string `json:"path"` // relative to project root
	Size int64  `json:"size"`
}

// ---------------------------------------------------------------------------
// Method name constants — single source of truth
// ---------------------------------------------------------------------------

const (
	// Backend → TUI
	MethodInit          = "init"
	MethodMsgUpdate     = "msg.update"
	MethodToolStart     = "tool.start"
	MethodToolProgress  = "tool.progress"
	MethodToolDone      = "tool.done"
	MethodTokenUpdate   = "token.update"
	MethodSessionTitle  = "session.title"
	MethodCycleUpdate   = "cycle.update"
	MethodSubagentStart = "subagent.start"
	MethodSubagentDone  = "subagent.done"
	MethodOverseerAlert = "overseer.alert"
	MethodDriftUpdate   = "drift.update"
	MethodLSPUpdate     = "lsp.update"
	MethodError         = "error"
	MethodCoordDone     = "coord.done"

	// Backend → TUI requests
	MethodPermissionAsk = "permission.ask"
	MethodQuestionAsk   = "question.ask"

	// TUI → Backend
	MethodSubmit         = "submit"
	MethodCancel         = "cancel"
	MethodAutonomyToggle = "autonomy.toggle"
	MethodYoloToggle     = "yolo.toggle"
	MethodResize         = "resize"

	// TUI → Backend requests
	MethodFileList      = "file.list"
	MethodSessionList   = "session.list"
	MethodSessionResume = "session.resume"
	MethodModelList     = "model.list"
	MethodModelSwitch   = "model.switch"
	MethodCompact       = "compact"
)
