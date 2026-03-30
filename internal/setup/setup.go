// Package setup provides the interactive first-run setup TUI.
//
// Flow: ModelPicker → AuthInput → Verify → [Add another?] → Save
// Supports configuring multiple providers in one session.
package setup

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/m0n0x41d/haft/internal/config"
	"github.com/m0n0x41d/haft/internal/provider"
)

// ---------------------------------------------------------------------------
// State machine
// ---------------------------------------------------------------------------

type setupState int

const (
	stateModelPicker  setupState = iota
	stateAuthChoice              // OpenAI: choose API key vs ChatGPT OAuth
	stateAuthInput               // API key input
	stateOAuthWaiting            // showing device code, polling in background
	stateVerifying               // verifying API key
	stateVerified
	stateAddAnother // ask "configure another provider?"
	stateDone
	stateError
)

// Result is returned after setup completes.
type Result struct {
	Config *config.Config
}

// Model is the BubbleTea model for the setup flow.
type Model struct {
	state  setupState
	config *config.Config // accumulates across multiple provider setups
	err    error
	width  int
	height int

	// Model picker
	registry   *provider.Registry
	providers  []provider.ProviderInfo
	flatModels []modelEntry
	filtered   []modelEntry
	cursor     int
	filterInput textinput.Model

	// Current selection
	selectedModel    provider.ModelInfo
	selectedProvider string
	isDefaultModel   bool // true for first model pick (becomes default)

	// Auth input
	authInput textinput.Model
	spinner   spinner.Model

	// OAuth device flow display
	oauthURL  string
	oauthCode string

	// Styles
	accent   lipgloss.Style
	dim      lipgloss.Style
	errStyle lipgloss.Style
	success  lipgloss.Style
}

type modelEntry struct {
	model    provider.ModelInfo
	provider string
	provName string
	isHeader bool
	header   string
}

// New creates a setup model. Loads existing config to preserve already-configured providers.
func New(existing *config.Config) Model {
	reg := provider.DefaultRegistry()

	fi := textinput.New()
	fi.Placeholder = "type to filter..."
	fi.Focus()
	fi.CharLimit = 50

	ai := textinput.New()
	ai.Placeholder = "paste API key here"
	ai.EchoMode = textinput.EchoPassword
	ai.CharLimit = 200

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))

	cfg := existing
	if cfg == nil {
		cfg = &config.Config{Providers: make(map[string]config.ProviderAuth)}
	}

	m := Model{
		state:       stateModelPicker,
		config:      cfg,
		registry:    reg,
		providers:   provider.EmbeddedProviders(), // curated list, not remote catwalk
		filterInput: fi,
		authInput:   ai,
		spinner:     sp,
		isDefaultModel: cfg.Model == "", // first pick if no default set
		accent:      lipgloss.NewStyle().Foreground(lipgloss.Color("39")),
		dim:         lipgloss.NewStyle().Foreground(lipgloss.Color("242")),
		errStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("196")),
		success:     lipgloss.NewStyle().Foreground(lipgloss.Color("42")),
	}
	m.buildModelList()
	m.applyFilter("")
	return m
}

func (m *Model) buildModelList() {
	m.flatModels = nil
	for _, p := range m.providers {
		if len(p.Models) == 0 {
			continue
		}
		m.flatModels = append(m.flatModels, modelEntry{isHeader: true, header: p.Name})
		for _, model := range p.Models {
			m.flatModels = append(m.flatModels, modelEntry{
				model: model, provider: p.ID, provName: p.Name,
			})
		}
	}
}

func (m *Model) applyFilter(filter string) {
	if filter == "" {
		m.filtered = m.flatModels
		m.cursor = 0
		for m.cursor < len(m.filtered) && m.filtered[m.cursor].isHeader {
			m.cursor++
		}
		return
	}
	lower := strings.ToLower(filter)
	m.filtered = nil
	lastHeader := modelEntry{}
	headerAdded := false
	for _, e := range m.flatModels {
		if e.isHeader {
			lastHeader = e
			headerAdded = false
			continue
		}
		if strings.Contains(strings.ToLower(e.model.ID), lower) ||
			strings.Contains(strings.ToLower(e.model.Name), lower) ||
			strings.Contains(strings.ToLower(e.provName), lower) {
			if !headerAdded {
				m.filtered = append(m.filtered, lastHeader)
				headerAdded = true
			}
			m.filtered = append(m.filtered, e)
		}
	}
	m.cursor = 0
	for m.cursor < len(m.filtered) && m.filtered[m.cursor].isHeader {
		m.cursor++
	}
}

// Completed returns true when setup is done.
func (m Model) Completed() bool { return m.state == stateDone }

// GetResult returns the final config.
func (m Model) GetResult() Result { return Result{Config: m.config} }

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.filterInput.Focus(), m.spinner.Tick)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch m.state {
		case stateModelPicker:
			return m.updateModelPicker(msg)
		case stateAuthChoice:
			return m.updateAuthChoice(msg)
		case stateAuthInput:
			return m.updateAuthInput(msg)
		case stateVerified:
			return m.updateVerified(msg)
		case stateAddAnother:
			return m.updateAddAnother(msg)
		case stateError:
			m.state = stateAuthInput
			m.authInput.SetValue("")
			return m, m.authInput.Focus()
		}

	case oauthDeviceCodeMsg:
		if msg.err != nil {
			m.state = stateError
			m.err = msg.err
			return m, nil
		}
		m.oauthURL = msg.url
		m.oauthCode = msg.code
		m.state = stateOAuthWaiting
		return m, tea.Batch(m.spinner.Tick, pollCodexOAuth(msg.deviceID, msg.code))

	case oauthResultMsg:
		if msg.err != nil {
			m.state = stateError
			m.err = msg.err
		} else {
			m.config.SetAuth(m.selectedProvider, config.ProviderAuth{
				AuthType:     "codex_oauth",
				AccessToken:  msg.accessToken,
				RefreshToken: msg.refreshToken,
			})
			if m.isDefaultModel {
				m.config.Model = m.selectedModel.ID
				m.isDefaultModel = false
			}
			m.state = stateVerified
		}
		return m, nil

	case verifyResultMsg:
		if msg.err != nil {
			m.state = stateError
			m.err = msg.err
		} else {
			// Save provider auth
			m.config.SetAuth(m.selectedProvider, config.ProviderAuth{
				AuthType: "api_key",
				APIKey:   m.authInput.Value(),
			})
			if m.isDefaultModel {
				m.config.Model = m.selectedModel.ID
				m.isDefaultModel = false
			}
			m.state = stateVerified
		}
		return m, nil

	case spinner.TickMsg:
		if m.state == stateVerifying || m.state == stateOAuthWaiting {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m Model) updateModelPicker(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "ctrl+c", "esc":
		if m.config.IsConfigured() {
			// Already have at least one provider — finish
			m.state = stateDone
			return m, tea.Quit
		}
		return m, tea.Quit

	case "up", "k":
		m.cursor--
		for m.cursor >= 0 && m.filtered[m.cursor].isHeader {
			m.cursor--
		}
		if m.cursor < 0 {
			m.cursor = len(m.filtered) - 1
			for m.cursor >= 0 && m.filtered[m.cursor].isHeader {
				m.cursor--
			}
		}

	case "down", "j":
		m.cursor++
		for m.cursor < len(m.filtered) && m.filtered[m.cursor].isHeader {
			m.cursor++
		}
		if m.cursor >= len(m.filtered) {
			m.cursor = 0
			for m.cursor < len(m.filtered) && m.filtered[m.cursor].isHeader {
				m.cursor++
			}
		}

	case "enter":
		if m.cursor >= 0 && m.cursor < len(m.filtered) && !m.filtered[m.cursor].isHeader {
			entry := m.filtered[m.cursor]
			m.selectedModel = entry.model
			m.selectedProvider = entry.provider

			// If provider already has auth, skip to verified
			if existing := m.config.GetAuth(entry.provider); existing.APIKey != "" || existing.AccessToken != "" {
				if m.isDefaultModel {
					m.config.Model = m.selectedModel.ID
					m.isDefaultModel = false
				}
				m.state = stateVerified
				return m, nil
			}

			// OpenAI: offer choice between API key and ChatGPT OAuth
			if entry.provider == "openai" {
				m.state = stateAuthChoice
				return m, nil
			}

			m.state = stateAuthInput
			m.authInput.SetValue("")
			return m, m.authInput.Focus()
		}

	default:
		var cmd tea.Cmd
		m.filterInput, cmd = m.filterInput.Update(key)
		m.applyFilter(m.filterInput.Value())
		return m, cmd
	}
	return m, nil
}

func (m Model) updateAuthChoice(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "1", "a": // API key
		m.state = stateAuthInput
		m.authInput.SetValue("")
		return m, m.authInput.Focus()
	case "2", "o": // OAuth (ChatGPT Plus/Pro)
		m.state = stateVerifying
		return m, runCodexOAuth()
	case "esc":
		m.state = stateModelPicker
		return m, m.filterInput.Focus()
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) updateAuthInput(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.state = stateModelPicker
		return m, m.filterInput.Focus()
	case "enter":
		val := m.authInput.Value()
		if val == "" {
			return m, nil
		}
		m.state = stateVerifying
		return m, tea.Batch(m.spinner.Tick, verifyAPIKey(m.selectedProvider, val))
	default:
		var cmd tea.Cmd
		m.authInput, cmd = m.authInput.Update(key)
		return m, cmd
	}
}

func (m Model) updateVerified(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	// After verification, ask if they want to add another provider
	m.state = stateAddAnother
	return m, nil
}

func (m Model) updateAddAnother(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "y", "Y":
		// Back to model picker for another provider
		m.state = stateModelPicker
		m.filterInput.SetValue("")
		m.applyFilter("")
		return m, m.filterInput.Focus()
	case "n", "N", "enter", "esc":
		m.state = stateDone
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) View() tea.View {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(m.accent.Bold(true).Render("  Haft Setup"))

	// Show configured providers
	configured := m.config.ConfiguredProviders()
	if len(configured) > 0 {
		b.WriteString(m.dim.Render(fmt.Sprintf("  [configured: %s]", strings.Join(configured, ", "))))
	}
	b.WriteString("\n\n")

	switch m.state {
	case stateModelPicker:
		if m.isDefaultModel {
			b.WriteString("  Pick your default model:\n\n")
		} else {
			b.WriteString("  Pick a model (adding another provider):\n\n")
		}
		b.WriteString("  " + m.filterInput.View() + "\n\n")
		m.renderModelList(&b)
		b.WriteString("\n")
		hint := "  ↑/↓ navigate · type to filter · enter select"
		if m.config.IsConfigured() {
			hint += " · esc done"
		}
		b.WriteString(m.dim.Render(hint))

	case stateAuthChoice:
		b.WriteString(fmt.Sprintf("  Authentication for %s (%s):\n\n", m.accent.Render(m.selectedProvider), m.selectedModel.ID))
		b.WriteString("  " + m.accent.Render("[1]") + " API key (paste OpenAI key)\n")
		b.WriteString("  " + m.accent.Render("[2]") + " ChatGPT Plus/Pro login (OAuth device flow)\n\n")
		b.WriteString(m.dim.Render("  1 or 2 · esc back"))

	case stateAuthInput:
		b.WriteString(fmt.Sprintf("  Authentication for %s:\n\n", m.accent.Render(m.selectedProvider)))
		b.WriteString(fmt.Sprintf("  Model: %s\n\n", m.selectedModel.ID))
		b.WriteString("  API Key: " + m.authInput.View() + "\n\n")
		b.WriteString(m.dim.Render("  enter verify · esc back"))

	case stateOAuthWaiting:
		b.WriteString("  ChatGPT Plus/Pro Login\n\n")
		b.WriteString(fmt.Sprintf("  Open:  %s\n", m.accent.Render(m.oauthURL)))
		b.WriteString(fmt.Sprintf("  Code:  %s\n\n", m.accent.Bold(true).Render(m.oauthCode)))
		b.WriteString(fmt.Sprintf("  Waiting for authorization... %s\n\n", m.spinner.View()))
		b.WriteString(m.dim.Render("  Open the URL above, paste the code, and authorize."))

	case stateVerifying:
		b.WriteString(fmt.Sprintf("  Verifying API key... %s\n", m.spinner.View()))

	case stateVerified:
		b.WriteString(m.success.Render(fmt.Sprintf("  ✓ %s connected!", m.selectedProvider)) + "\n\n")
		b.WriteString(fmt.Sprintf("  Model: %s\n", m.selectedModel.ID))
		if m.config.Model == m.selectedModel.ID {
			b.WriteString(m.dim.Render("  (set as default)") + "\n")
		}
		b.WriteString("\n")
		b.WriteString(m.dim.Render("  Press any key to continue..."))

	case stateAddAnother:
		b.WriteString(m.success.Render("  ✓ Setup complete.") + "\n\n")
		b.WriteString(fmt.Sprintf("  Default model: %s\n", m.accent.Render(m.config.Model)))
		b.WriteString(fmt.Sprintf("  Providers: %s\n\n", strings.Join(m.config.ConfiguredProviders(), ", ")))
		b.WriteString("  Configure another provider? " + m.accent.Render("[y/N]"))

	case stateError:
		b.WriteString(m.errStyle.Render(fmt.Sprintf("  ✗ Verification failed: %s", m.err)) + "\n\n")
		b.WriteString(m.dim.Render("  Press any key to retry..."))
	}

	return tea.NewView(b.String())
}

func (m Model) renderModelList(b *strings.Builder) {
	visibleRows := m.height - 12
	if visibleRows < 10 {
		visibleRows = 15
	}

	start := 0
	if m.cursor > visibleRows/2 {
		start = m.cursor - visibleRows/2
	}
	end := start + visibleRows
	if end > len(m.filtered) {
		end = len(m.filtered)
		start = end - visibleRows
		if start < 0 {
			start = 0
		}
	}

	for i := start; i < end; i++ {
		entry := m.filtered[i]
		if entry.isHeader {
			// Mark if provider is already configured
			mark := ""
			for _, p := range m.providers {
				if p.Name == entry.header {
					if auth := m.config.GetAuth(p.ID); auth.APIKey != "" || auth.AccessToken != "" {
						mark = m.success.Render(" ✓")
					}
					break
				}
			}
			b.WriteString(m.dim.Render(fmt.Sprintf("    ─── %s%s ───\n", entry.header, mark)))
			continue
		}

		ctx := formatCtx(entry.model.ContextWindow)
		reason := ""
		if entry.model.CanReason {
			reason = " [reason]"
		}
		cost := ""
		if entry.model.CostPer1MIn > 0 {
			cost = fmt.Sprintf(" $%.2f/$%.2f", entry.model.CostPer1MIn, entry.model.CostPer1MOut)
		}

		info := fmt.Sprintf("%-24s  %5s ctx%s%s", entry.model.ID, ctx, cost, reason)
		if i == m.cursor {
			b.WriteString(m.accent.Bold(true).Render("  > "+info) + "\n")
		} else {
			b.WriteString("    " + info + "\n")
		}
	}
}

// ---------------------------------------------------------------------------
// Verification
// ---------------------------------------------------------------------------

type verifyResultMsg struct{ err error }

func verifyAPIKey(providerID, apiKey string) tea.Cmd {
	return func() tea.Msg {
		start := time.Now()
		var err error
		switch providerID {
		case "openai":
			err = verifyOpenAIKey(apiKey)
		case "anthropic":
			err = verifyAnthropicKey(apiKey)
		default:
			err = nil // accept unknown providers
		}
		if elapsed := time.Since(start); elapsed < 750*time.Millisecond {
			time.Sleep(750*time.Millisecond - elapsed)
		}
		return verifyResultMsg{err: err}
	}
}

func verifyOpenAIKey(key string) error {
	req, _ := http.NewRequest("GET", "https://api.openai.com/v1/models", nil)
	req.Header.Set("Authorization", "Bearer "+key)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 401 {
		return fmt.Errorf("invalid API key")
	}
	return nil
}

func verifyAnthropicKey(key string) error {
	req, _ := http.NewRequest("GET", "https://api.anthropic.com/v1/models", nil)
	req.Header.Set("x-api-key", key)
	req.Header.Set("anthropic-version", "2023-06-01")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 401 {
		return fmt.Errorf("invalid API key")
	}
	return nil
}

// ---------------------------------------------------------------------------
// Codex OAuth device flow (ChatGPT Plus/Pro authentication)
// ---------------------------------------------------------------------------

const (
	codexClientID = "app_EMoamEEZ73f0CkXaXp7hrann"
	codexIssuer   = "https://auth.openai.com"
)

// oauthDeviceCodeMsg carries the device code for display in the TUI.
type oauthDeviceCodeMsg struct {
	url      string
	code     string
	deviceID string
	err      error
}

// oauthResultMsg carries the final OAuth tokens.
type oauthResultMsg struct {
	accessToken  string
	refreshToken string
	err          error
}

// runCodexOAuth step 1: request device code (fast, returns immediately).
func runCodexOAuth() tea.Cmd {
	return func() tea.Msg {
		resp, err := http.Post(
			codexIssuer+"/api/accounts/deviceauth/usercode",
			"application/json",
			strings.NewReader(fmt.Sprintf(`{"client_id":"%s"}`, codexClientID)),
		)
		if err != nil {
			return oauthDeviceCodeMsg{err: fmt.Errorf("device auth request: %w", err)}
		}
		defer resp.Body.Close()

		var deviceAuth struct {
			DeviceAuthID string `json:"device_auth_id"`
			UserCode     string `json:"user_code"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&deviceAuth); err != nil {
			return oauthDeviceCodeMsg{err: fmt.Errorf("parse device auth: %w", err)}
		}

		return oauthDeviceCodeMsg{
			url:      codexIssuer + "/codex/device",
			code:     deviceAuth.UserCode,
			deviceID: deviceAuth.DeviceAuthID,
		}
	}
}

// pollCodexOAuth step 2: poll for token completion (runs in background).
// Must send both device_auth_id AND user_code (matching login.go protocol).
func pollCodexOAuth(deviceID, userCode string) tea.Cmd {
	return func() tea.Msg {
		for i := 0; i < 60; i++ { // 5 min max
			time.Sleep(5 * time.Second)

			// Poll with device_auth_id + user_code (not client_id)
			tokenResp, err := http.Post(
				codexIssuer+"/api/accounts/deviceauth/token",
				"application/json",
				strings.NewReader(fmt.Sprintf(`{"device_auth_id":"%s","user_code":"%s"}`,
					deviceID, userCode)),
			)
			if err != nil {
				continue
			}

			if tokenResp.StatusCode != http.StatusOK {
				tokenResp.Body.Close()
				continue
			}

			var token struct {
				AuthorizationCode string `json:"authorization_code"`
				CodeVerifier      string `json:"code_verifier"`
			}
			_ = json.NewDecoder(tokenResp.Body).Decode(&token)
			tokenResp.Body.Close()

			if token.AuthorizationCode == "" {
				continue
			}

			// Exchange code for OAuth tokens (must include redirect_uri)
			exchangeResp, err := http.PostForm(codexIssuer+"/oauth/token", url.Values{
				"grant_type":    {"authorization_code"},
				"code":          {token.AuthorizationCode},
				"redirect_uri":  {codexIssuer + "/deviceauth/callback"},
				"client_id":     {codexClientID},
				"code_verifier": {token.CodeVerifier},
			})
			if err != nil {
				return oauthResultMsg{err: err}
			}
			var oauthToken struct {
				AccessToken  string `json:"access_token"`
				RefreshToken string `json:"refresh_token"`
			}
			_ = json.NewDecoder(exchangeResp.Body).Decode(&oauthToken)
			exchangeResp.Body.Close()

			if oauthToken.AccessToken != "" {
				return oauthResultMsg{
					accessToken:  oauthToken.AccessToken,
					refreshToken: oauthToken.RefreshToken,
				}
			}
		}
		return oauthResultMsg{err: fmt.Errorf("OAuth timed out — try again")}
	}
}

// primaryProviderIDs lists the providers shown by default in setup.
// Other providers are available via catwalk but would overwhelm the picker.
var primaryProviderIDs = map[string]bool{
	"openai": true, "anthropic": true, "google": true,
	"deepseek": true, "groq": true,
}

// filterPrimaryProviders returns only the main providers for the setup picker.
// Also caps each provider to 10 most relevant models (sorted by context window desc).
func filterPrimaryProviders(all []provider.ProviderInfo) []provider.ProviderInfo {
	var result []provider.ProviderInfo
	for _, p := range all {
		if !primaryProviderIDs[p.ID] {
			continue
		}
		filtered := p
		if len(filtered.Models) > 10 {
			filtered.Models = filtered.Models[:10]
		}
		result = append(result, filtered)
	}
	return result
}

func formatCtx(tokens int) string {
	switch {
	case tokens >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(tokens)/1_000_000)
	case tokens >= 1_000:
		return fmt.Sprintf("%dk", tokens/1_000)
	default:
		return fmt.Sprintf("%d", tokens)
	}
}

// ---------------------------------------------------------------------------
// Run — standalone entry point
// ---------------------------------------------------------------------------

// Run launches the setup TUI. Loads existing config to preserve providers.
// Saves to disk on success.
func Run() (*Result, error) {
	existing, _ := config.Load()
	m := New(existing)
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}

	final, ok := finalModel.(Model)
	if !ok {
		return nil, fmt.Errorf("unexpected model type from setup TUI")
	}
	if !final.Completed() {
		return nil, fmt.Errorf("setup cancelled")
	}

	result := final.GetResult()
	if err := config.Save(result.Config); err != nil {
		return nil, fmt.Errorf("save config: %w", err)
	}

	return &result, nil
}
