package ui

import (
	"image/color"
	"os"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/term"
)

// Theme holds the dashboard color palette, adaptive to terminal background.
type Theme struct {
	Green   color.Color
	Yellow  color.Color
	Red     color.Color
	Blue    color.Color
	Cyan    color.Color
	Magenta color.Color
	Dim     color.Color
	Text    color.Color
	Bold    color.Color
	BgSub   color.Color // subtle background for header/selected
	Border  color.Color
}

func detectTheme() Theme {
	dark := lipgloss.HasDarkBackground(os.Stdin, os.Stdout)
	if dark {
		return Theme{
			Green:   lipgloss.Color("#22c55e"),
			Yellow:  lipgloss.Color("#eab308"),
			Red:     lipgloss.Color("#ef4444"),
			Blue:    lipgloss.Color("#60a5fa"),
			Cyan:    lipgloss.Color("#22d3ee"),
			Magenta: lipgloss.Color("#c084fc"),
			Dim:     lipgloss.Color("#6b7280"),
			Text:    lipgloss.Color("#d1d5db"),
			Bold:    lipgloss.Color("#f3f4f6"),
			BgSub:   lipgloss.Color("#1f2937"),
			Border:  lipgloss.Color("#4b5563"),
		}
	}
	return Theme{
		Green:   lipgloss.Color("#16a34a"),
		Yellow:  lipgloss.Color("#ca8a04"),
		Red:     lipgloss.Color("#dc2626"),
		Blue:    lipgloss.Color("#2563eb"),
		Cyan:    lipgloss.Color("#0891b2"),
		Magenta: lipgloss.Color("#9333ea"),
		Dim:     lipgloss.Color("#9ca3af"),
		Text:    lipgloss.Color("#374151"),
		Bold:    lipgloss.Color("#111827"),
		BgSub:   lipgloss.Color("#f3f4f6"),
		Border:  lipgloss.Color("#d1d5db"),
	}
}

// Styles holds reusable styles for the dashboard.
type Styles struct {
	Theme Theme

	// Tabs
	ActiveTab   lipgloss.Style
	InactiveTab lipgloss.Style

	// Content
	Title    lipgloss.Style
	Subtitle lipgloss.Style
	Label    lipgloss.Style
	Value    lipgloss.Style
	DimText  lipgloss.Style
	Section  lipgloss.Style

	// Status indicators
	OK      lipgloss.Style
	Warning lipgloss.Style
	Error   lipgloss.Style

	// List items
	SelectedItem lipgloss.Style
	NormalItem   lipgloss.Style
	DimRow       lipgloss.Style

	// Header / help
	Header  lipgloss.Style
	HelpBar lipgloss.Style
	HelpKey lipgloss.Style
}

// NewStyles creates the style set for a given terminal width.
func NewStyles(width int) Styles {
	t := detectTheme()

	return Styles{
		Theme: t,

		ActiveTab: lipgloss.NewStyle().
			Bold(true).
			Foreground(t.Bold).
			Background(t.BgSub).
			Padding(0, 2),
		InactiveTab: lipgloss.NewStyle().
			Foreground(t.Dim).
			Padding(0, 2),

		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(t.Bold),
		Subtitle: lipgloss.NewStyle().
			Foreground(t.Cyan),
		Label: lipgloss.NewStyle().
			Foreground(t.Dim),
		Value: lipgloss.NewStyle().
			Foreground(t.Text),
		DimText: lipgloss.NewStyle().
			Foreground(t.Dim),
		Section: lipgloss.NewStyle().
			Bold(true).
			Foreground(t.Blue).
			MarginTop(1),

		OK: lipgloss.NewStyle().
			Foreground(t.Green),
		Warning: lipgloss.NewStyle().
			Foreground(t.Yellow),
		Error: lipgloss.NewStyle().
			Foreground(t.Red),

		SelectedItem: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffffff")).
			Background(lipgloss.Color("#3b82f6")).
			Bold(true),
		NormalItem: lipgloss.NewStyle().
			Foreground(t.Text),
		DimRow: lipgloss.NewStyle().
			Foreground(t.Dim),

		Header: lipgloss.NewStyle().
			Background(t.BgSub).
			Foreground(t.Text).
			Padding(0, 1).
			Width(width).
			BorderBottom(true).
			BorderStyle(lipgloss.ThickBorder()).
			BorderForeground(t.Blue),

		HelpBar: lipgloss.NewStyle().
			Foreground(t.Dim).
			Background(t.BgSub).
			Width(width).
			Padding(0, 1),
		HelpKey: lipgloss.NewStyle().
			Foreground(t.Text).
			Bold(true),
	}
}

// REffStyle returns the appropriate style for an R_eff value.
func (s Styles) REffStyle(reff float64) lipgloss.Style {
	switch {
	case reff >= 0.7:
		return lipgloss.NewStyle().Foreground(s.Theme.Green)
	case reff >= 0.3:
		return lipgloss.NewStyle().Foreground(s.Theme.Yellow)
	default:
		return lipgloss.NewStyle().Foreground(s.Theme.Red)
	}
}

// IsTerminal checks if stdout is a TTY.
func IsTerminal() bool {
	return term.IsTerminal(os.Stdout.Fd())
}
