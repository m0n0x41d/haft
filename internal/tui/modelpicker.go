package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/m0n0x41d/haft/internal/config"
	"github.com/m0n0x41d/haft/internal/provider"
)

// ---------------------------------------------------------------------------
// Inline model picker — modal overlay inside the agent TUI.
// Opened via Ctrl+M or /model. Supports model selection + inline auth.
// ---------------------------------------------------------------------------

type pickerState int

const (
	pickerBrowse pickerState = iota
	pickerAuth
	pickerVerifying
	pickerDone
	pickerError
)

// ModelPicker is the modal overlay for mid-session model switching.
type ModelPicker struct {
	state    pickerState
	cfg      *config.Config
	registry *provider.Registry

	// Browser
	entries  []pickerEntry
	filtered []pickerEntry
	cursor   int
	filter   textinput.Model

	// Auth
	selectedModel    provider.ModelInfo
	selectedProvider string
	authInput        textinput.Model
	spinner          spinner.Model
	err              error

	// Result
	result *ModelSwitchMsg

	width, height int
}

type pickerEntry struct {
	model    provider.ModelInfo
	provider string
	provName string
	isHeader bool
	header   string
}

// NewModelPicker creates a model picker overlay.
func NewModelPicker(cfg *config.Config, width, height int) *ModelPicker {
	reg := provider.DefaultRegistry()

	fi := textinput.New()
	fi.Placeholder = "filter..."
	fi.Focus()
	fi.CharLimit = 40

	ai := textinput.New()
	ai.Placeholder = "paste API key"
	ai.EchoMode = textinput.EchoPassword
	ai.CharLimit = 200

	sp := spinner.New(spinner.WithSpinner(spinner.Dot))

	p := &ModelPicker{
		state:    pickerBrowse,
		cfg:      cfg,
		registry: reg,
		filter:   fi,
		authInput: ai,
		spinner:  sp,
		width:    width,
		height:   height,
	}
	p.buildEntries(reg.Providers())
	p.applyFilter("")
	return p
}

func (p *ModelPicker) buildEntries(providers []provider.ProviderInfo) {
	for _, prov := range providers {
		if len(prov.Models) == 0 {
			continue
		}
		p.entries = append(p.entries, pickerEntry{isHeader: true, header: prov.Name, provider: prov.ID})
		for _, m := range prov.Models {
			p.entries = append(p.entries, pickerEntry{
				model: m, provider: prov.ID, provName: prov.Name,
			})
		}
	}
}

func (p *ModelPicker) applyFilter(filter string) {
	if filter == "" {
		p.filtered = p.entries
		p.cursor = 0
		for p.cursor < len(p.filtered) && p.filtered[p.cursor].isHeader {
			p.cursor++
		}
		return
	}
	lower := strings.ToLower(filter)
	p.filtered = nil
	lastHdr := pickerEntry{}
	hdrAdded := false
	for _, e := range p.entries {
		if e.isHeader {
			lastHdr = e
			hdrAdded = false
			continue
		}
		if strings.Contains(strings.ToLower(e.model.ID), lower) ||
			strings.Contains(strings.ToLower(e.provName), lower) {
			if !hdrAdded {
				p.filtered = append(p.filtered, lastHdr)
				hdrAdded = true
			}
			p.filtered = append(p.filtered, e)
		}
	}
	p.cursor = 0
	for p.cursor < len(p.filtered) && p.filtered[p.cursor].isHeader {
		p.cursor++
	}
}

// Done returns true when the picker has a result or was cancelled.
func (p *ModelPicker) Done() bool { return p.state == pickerDone }

// Result returns the switch message, or nil if cancelled.
func (p *ModelPicker) Result() *ModelSwitchMsg { return p.result }

// Update handles a key press. Returns a tea.Cmd if needed (e.g., spinner tick).
func (p *ModelPicker) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return p.handleKey(msg)
	case verifyKeyResultMsg:
		if msg.err != nil {
			p.state = pickerError
			p.err = msg.err
		} else {
			p.state = pickerDone
			p.result = &ModelSwitchMsg{
				Model:    p.selectedModel.ID,
				Provider: p.selectedProvider,
				APIKey:   p.authInput.Value(),
			}
		}
		return nil
	case spinner.TickMsg:
		if p.state == pickerVerifying {
			var cmd tea.Cmd
			p.spinner, cmd = p.spinner.Update(msg)
			return cmd
		}
	}
	return nil
}

func (p *ModelPicker) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	key := msg.Key()

	// Esc always closes
	if key.Code == tea.KeyEscape {
		p.state = pickerDone
		return nil
	}

	switch p.state {
	case pickerBrowse:
		return p.handleBrowseKey(msg)
	case pickerAuth:
		return p.handleAuthKey(msg)
	case pickerError:
		p.state = pickerAuth
		p.authInput.SetValue("")
		return p.authInput.Focus()
	}
	return nil
}

func (p *ModelPicker) handleBrowseKey(msg tea.KeyPressMsg) tea.Cmd {
	key := msg.Key()
	switch {
	case key.Code == tea.KeyUp || (key.Code == 'k' && key.Mod == 0):
		p.cursor--
		for p.cursor >= 0 && p.filtered[p.cursor].isHeader {
			p.cursor--
		}
		if p.cursor < 0 {
			p.cursor = len(p.filtered) - 1
			for p.cursor >= 0 && p.filtered[p.cursor].isHeader {
				p.cursor--
			}
		}
	case key.Code == tea.KeyDown || (key.Code == 'j' && key.Mod == 0):
		p.cursor++
		for p.cursor < len(p.filtered) && p.filtered[p.cursor].isHeader {
			p.cursor++
		}
		if p.cursor >= len(p.filtered) {
			p.cursor = 0
			for p.cursor < len(p.filtered) && p.filtered[p.cursor].isHeader {
				p.cursor++
			}
		}
	case key.Code == tea.KeyEnter:
		if p.cursor >= 0 && p.cursor < len(p.filtered) && !p.filtered[p.cursor].isHeader {
			entry := p.filtered[p.cursor]
			p.selectedModel = entry.model
			p.selectedProvider = entry.provider

			// Provider already has auth → switch immediately
			if auth := p.cfg.GetAuth(entry.provider); auth.APIKey != "" || auth.AccessToken != "" {
				p.state = pickerDone
				p.result = &ModelSwitchMsg{
					Model:    entry.model.ID,
					Provider: entry.provider,
				}
				return nil
			}

			// Need auth
			p.state = pickerAuth
			p.authInput.SetValue("")
			return p.authInput.Focus()
		}
	default:
		var cmd tea.Cmd
		p.filter, cmd = p.filter.Update(msg)
		p.applyFilter(p.filter.Value())
		return cmd
	}
	return nil
}

func (p *ModelPicker) handleAuthKey(msg tea.KeyPressMsg) tea.Cmd {
	key := msg.Key()
	switch key.Code {
	case tea.KeyEnter:
		val := p.authInput.Value()
		if val == "" {
			return nil
		}
		p.state = pickerVerifying
		return tea.Batch(p.spinner.Tick, verifyKey(p.selectedProvider, val))
	default:
		var cmd tea.Cmd
		p.authInput, cmd = p.authInput.Update(msg)
		return cmd
	}
}

// Render draws the modal overlay.
func (p *ModelPicker) Render(styles Styles) string {
	accent := lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("242"))
	errSt := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	okSt := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))

	var b strings.Builder
	b.WriteString(accent.Bold(true).Render("Switch Model") + "\n\n")

	switch p.state {
	case pickerBrowse:
		b.WriteString(p.filter.View() + "\n\n")

		rows := p.height - 10
		if rows < 8 {
			rows = 12
		}
		start := max(0, p.cursor-rows/2)
		end := min(len(p.filtered), start+rows)
		if end-start < rows {
			start = max(0, end-rows)
		}

		for i := start; i < end; i++ {
			e := p.filtered[i]
			if e.isHeader {
				mark := ""
				if auth := p.cfg.GetAuth(e.provider); auth.APIKey != "" || auth.AccessToken != "" {
					mark = okSt.Render(" ✓")
				}
				b.WriteString(dim.Render(fmt.Sprintf("── %s%s ──\n", e.header, mark)))
				continue
			}
			prefix := "  "
			st := lipgloss.NewStyle()
			if i == p.cursor {
				prefix = "> "
				st = accent.Bold(true)
			}
			ctx := formatTokens(e.model.ContextWindow)
			reason := ""
			if e.model.CanReason {
				reason = " [r]"
			}
			b.WriteString(st.Render(fmt.Sprintf("%s%-26s %5s%s", prefix, e.model.ID, ctx, reason)) + "\n")
		}
		b.WriteString("\n" + dim.Render("↑/↓ select · enter pick · esc cancel"))

	case pickerAuth:
		b.WriteString(fmt.Sprintf("API key for %s:\n\n", accent.Render(p.selectedProvider)))
		b.WriteString(p.authInput.View() + "\n\n")
		b.WriteString(dim.Render("enter verify · esc cancel"))

	case pickerVerifying:
		b.WriteString(fmt.Sprintf("Verifying... %s", p.spinner.View()))

	case pickerError:
		b.WriteString(errSt.Render(fmt.Sprintf("✗ %s", p.err)) + "\n\n")
		b.WriteString(dim.Render("press any key to retry"))
	}

	boxWidth := min(p.width-4, 50)
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("39")).
		Padding(1, 2).
		Width(boxWidth).
		Render(b.String())
}

func formatTokens(n int) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%dk", n/1_000)
	}
	return fmt.Sprintf("%d", n)
}

// verifyKeyResultMsg is the result of async API key verification.
type verifyKeyResultMsg struct{ err error }

func verifyKey(providerID, apiKey string) tea.Cmd {
	return func() tea.Msg {
		var err error
		switch providerID {
		case "openai":
			err = verifyOpenAI(apiKey)
		case "anthropic":
			err = verifyAnthropic(apiKey)
		default:
			err = nil
		}
		return verifyKeyResultMsg{err: err}
	}
}

func verifyOpenAI(key string) error {
	return quickHTTPCheck("https://api.openai.com/v1/models", "Authorization", "Bearer "+key)
}

func verifyAnthropic(key string) error {
	return quickHTTPCheck("https://api.anthropic.com/v1/models", "x-api-key", key)
}

func quickHTTPCheck(url, headerKey, headerVal string) error {
	req, _ := newHTTPRequest("GET", url)
	if req == nil {
		return fmt.Errorf("failed to create request")
	}
	req.Header.Set(headerKey, headerVal)
	if headerKey == "x-api-key" {
		req.Header.Set("anthropic-version", "2023-06-01")
	}
	resp, err := httpClient().Do(req)
	if err != nil {
		return fmt.Errorf("connection failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode == 401 {
		return fmt.Errorf("invalid API key")
	}
	return nil
}
