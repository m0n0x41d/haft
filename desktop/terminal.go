package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"
)

type TerminalSession struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	ProjectPath string `json:"project_path"`
	Cwd         string `json:"cwd"`
	Shell       string `json:"shell"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type TerminalOutputEvent struct {
	ID   string `json:"id"`
	Data string `json:"data"`
}

type terminalManager struct {
	app *App

	mu       sync.Mutex
	seq      int
	sessions map[string]*runningTerminal
}

type runningTerminal struct {
	state TerminalSession
	cmd   *exec.Cmd
	pty   *os.File

	closeOnce sync.Once
}

func newTerminalManager(app *App) *terminalManager {
	return &terminalManager{
		app:      app,
		sessions: make(map[string]*runningTerminal),
	}
}

func (m *terminalManager) nextID() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.seq++
	return fmt.Sprintf("term-%d-%d", time.Now().Unix(), m.seq)
}

func (m *terminalManager) list() []TerminalSession {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]TerminalSession, 0, len(m.sessions))
	for _, session := range m.sessions {
		result = append(result, session.state)
	}

	return result
}

func (m *terminalManager) shutdown() {
	if m == nil {
		return
	}

	m.mu.Lock()
	sessions := make([]*runningTerminal, 0, len(m.sessions))
	for _, session := range m.sessions {
		sessions = append(sessions, session)
	}
	m.mu.Unlock()

	for _, session := range sessions {
		m.closeSession(session, "closed")
	}
}

func (m *terminalManager) create(projectPath string, cwd string) (*TerminalSession, error) {
	if m == nil {
		return nil, fmt.Errorf("terminal manager is not initialized")
	}

	workspacePath := firstNonEmpty(strings.TrimSpace(cwd), strings.TrimSpace(projectPath), m.app.projectRoot)
	if workspacePath == "" {
		return nil, fmt.Errorf("no active project workspace")
	}

	absPath, err := filepath.Abs(workspacePath)
	if err != nil {
		return nil, fmt.Errorf("resolve terminal path: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("open terminal path %s: %w", absPath, err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("terminal path is not a directory: %s", absPath)
	}

	shellPath, err := defaultShellPath()
	if err != nil {
		return nil, err
	}

	dlog.Info().Str("shell", shellPath).Str("cwd", absPath).Msg("creating terminal session")

	cmd := exec.Command(shellPath, "-l")
	cmd.Dir = absPath
	cmd.Env = append(m.app.userEnv, "TERM=xterm-256color")

	ptyFile, err := pty.StartWithSize(cmd, &pty.Winsize{
		Rows: 32,
		Cols: 120,
	})
	if err != nil {
		return nil, fmt.Errorf("start PTY shell: %w", err)
	}

	state := TerminalSession{
		ID:          m.nextID(),
		Title:       filepath.Base(absPath),
		ProjectPath: firstNonEmpty(strings.TrimSpace(projectPath), m.app.projectRoot),
		Cwd:         absPath,
		Shell:       shellPath,
		Status:      "running",
		CreatedAt:   nowRFC3339(),
		UpdatedAt:   nowRFC3339(),
	}

	session := &runningTerminal{
		state: state,
		cmd:   cmd,
		pty:   ptyFile,
	}

	m.mu.Lock()
	m.sessions[state.ID] = session
	m.mu.Unlock()

	m.emitStatus(state)

	go m.readLoop(session)
	go m.waitLoop(session)

	result := state
	return &result, nil
}

func (m *terminalManager) write(id string, data string) error {
	session, err := m.session(id)
	if err != nil {
		return err
	}

	_, err = io.WriteString(session.pty, data)
	return err
}

func (m *terminalManager) resize(id string, cols int, rows int) error {
	session, err := m.session(id)
	if err != nil {
		return err
	}

	if cols <= 0 || rows <= 0 {
		return fmt.Errorf("terminal size must be positive")
	}

	return pty.Setsize(session.pty, &pty.Winsize{
		Rows: uint16(rows),
		Cols: uint16(cols),
	})
}

func (m *terminalManager) close(id string) error {
	session, err := m.session(id)
	if err != nil {
		return err
	}

	m.closeSession(session, "closed")
	return nil
}

func (m *terminalManager) session(id string) (*runningTerminal, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[strings.TrimSpace(id)]
	if !ok {
		return nil, fmt.Errorf("terminal session not found: %s", id)
	}

	return session, nil
}

func (m *terminalManager) readLoop(session *runningTerminal) {
	buffer := make([]byte, 4096)

	for {
		count, err := session.pty.Read(buffer)
		if count > 0 {
			m.emitOutput(session.state.ID, string(buffer[:count]))
		}

		if err == nil {
			continue
		}

		if err != io.EOF && m.app != nil {
			m.app.emitAppError("terminal output", err)
		}

		return
	}
}

func (m *terminalManager) waitLoop(session *runningTerminal) {
	waitErr := session.cmd.Wait()

	// Read status under lock — closeSession may be modifying it concurrently.
	m.mu.Lock()
	status := session.state.Status
	m.mu.Unlock()

	if waitErr != nil && m.app != nil && status != "closed" {
		m.app.emitAppError("terminal session", waitErr)
	}

	m.closeSession(session, "exited")
}

func (m *terminalManager) emitOutput(id string, data string) {
	if m == nil || m.app == nil {
		return
	}

	m.app.emitEvent("terminal.output", TerminalOutputEvent{
		ID:   id,
		Data: data,
	})
}

func (m *terminalManager) emitStatus(state TerminalSession) {
	if m == nil || m.app == nil {
		return
	}

	m.app.emitEvent("terminal.status", state)
}

func (m *terminalManager) closeSession(session *runningTerminal, status string) {
	if m == nil || session == nil {
		return
	}

	session.closeOnce.Do(func() {
		if session.pty != nil {
			_ = session.pty.Close()
		}

		if status == "closed" && session.cmd != nil && session.cmd.Process != nil {
			_ = session.cmd.Process.Kill()
		}

		session.state.Status = status
		session.state.UpdatedAt = nowRFC3339()

		m.mu.Lock()
		delete(m.sessions, session.state.ID)
		m.mu.Unlock()

		m.emitStatus(session.state)
	})
}

func defaultShellPath() (string, error) {
	shell := strings.TrimSpace(os.Getenv("SHELL"))
	if shell != "" {
		if _, err := os.Stat(shell); err == nil {
			return shell, nil
		}
	}

	for _, candidate := range []string{"zsh", "bash", "sh"} {
		path, err := exec.LookPath(candidate)
		if err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("no supported shell found in PATH")
}

func (a *App) ensureTerminalManager() {
	if a == nil {
		return
	}

	if a.terminals != nil {
		return
	}

	a.terminals = newTerminalManager(a)
}

func (a *App) ListTerminalSessions() ([]TerminalSession, error) {
	a.ensureTerminalManager()
	return a.terminals.list(), nil
}

func (a *App) CreateTerminalSession(cwd string) (*TerminalSession, error) {
	a.ensureTerminalManager()
	return a.terminals.create(a.projectRoot, cwd)
}

func (a *App) WriteTerminalInput(id string, data string) error {
	a.ensureTerminalManager()
	return a.terminals.write(id, data)
}

func (a *App) ResizeTerminalSession(id string, cols int, rows int) error {
	a.ensureTerminalManager()
	return a.terminals.resize(id, cols, rows)
}

func (a *App) CloseTerminalSession(id string) error {
	a.ensureTerminalManager()
	return a.terminals.close(id)
}
