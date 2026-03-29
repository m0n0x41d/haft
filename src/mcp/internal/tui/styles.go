package tui

import "charm.land/lipgloss/v2"

// Styles holds all TUI styling.
type Styles struct {
	// Layout
	HeaderBar    lipgloss.Style
	StatusBar    lipgloss.Style
	BlockDivider lipgloss.Style

	// Messages
	UserText       lipgloss.Style
	AssistantMark  lipgloss.Style
	AssistantLabel lipgloss.Style
	AssistantText  lipgloss.Style
	ThinkingText   lipgloss.Style

	// Tools
	ToolName    lipgloss.Style
	ToolParam   lipgloss.Style
	ToolBody    lipgloss.Style
	ToolError   lipgloss.Style
	ToolRunning lipgloss.Style
	ToolDone    lipgloss.Style

	// Status
	StatusModel  lipgloss.Style
	StatusState  lipgloss.Style
	StatusDim    lipgloss.Style
	GliderLive   lipgloss.Style
	GliderDead   lipgloss.Style
	StatusAccent lipgloss.Style

	// Permission
	PermTitle lipgloss.Style
	PermKey   lipgloss.Style
	PermDeny  lipgloss.Style

	// Error
	ErrorText lipgloss.Style

	// Input frame
	InputBorder lipgloss.Style

	// General
	Dim    lipgloss.Style
	Cursor lipgloss.Style
}

// DefaultStyles returns the agent TUI theme.
func DefaultStyles() Styles {
	subtle := lipgloss.Color("241")
	accent := lipgloss.Color("39")  // blue
	green := lipgloss.Color("42")   // assistant border / success
	red := lipgloss.Color("196")    // error
	yellow := lipgloss.Color("214") // permission / running
	white := lipgloss.Color("255")  // bright text
	dimWhite := lipgloss.Color("250")

	return Styles{
		// Layout
		HeaderBar:    lipgloss.NewStyle().Bold(true).Foreground(accent).PaddingLeft(1),
		StatusBar:    lipgloss.NewStyle().Foreground(subtle).PaddingLeft(1),
		BlockDivider: lipgloss.NewStyle().Foreground(lipgloss.Color("239")),

		// Messages
		UserText:       lipgloss.NewStyle().Foreground(white),
		AssistantMark:  lipgloss.NewStyle().Bold(true).Foreground(green),
		AssistantLabel: lipgloss.NewStyle().Foreground(subtle),
		AssistantText:  lipgloss.NewStyle().Foreground(dimWhite),
		ThinkingText:   lipgloss.NewStyle().Foreground(lipgloss.Color("243")),

		// Tools
		ToolName:    lipgloss.NewStyle().Bold(true).Foreground(accent),
		ToolParam:   lipgloss.NewStyle().Foreground(subtle),
		ToolBody:    lipgloss.NewStyle().Foreground(subtle),
		ToolError:   lipgloss.NewStyle().Foreground(red),
		ToolRunning: lipgloss.NewStyle().Foreground(yellow),
		ToolDone:    lipgloss.NewStyle().Foreground(green),

		// Status
		StatusModel:  lipgloss.NewStyle().Bold(true).Foreground(accent),
		StatusState:  lipgloss.NewStyle().Foreground(dimWhite),
		StatusDim:    lipgloss.NewStyle().Foreground(subtle),
		GliderLive:   lipgloss.NewStyle().Bold(true).Foreground(accent),
		GliderDead:   lipgloss.NewStyle().Foreground(lipgloss.Color("238")),
		StatusAccent: lipgloss.NewStyle().Foreground(accent),

		// Permission
		PermTitle: lipgloss.NewStyle().Bold(true).Foreground(yellow),
		PermKey:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("16")).Background(green),
		PermDeny:  lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("16")).Background(red),

		// Error
		ErrorText: lipgloss.NewStyle().Foreground(red),

		// Input frame — muted teal, thick lines
		InputBorder: lipgloss.NewStyle().Foreground(lipgloss.Color("30")), // dark teal

		// General
		Dim:    lipgloss.NewStyle().Foreground(subtle),
		Cursor: lipgloss.NewStyle().Foreground(white).Bold(true),
	}
}
