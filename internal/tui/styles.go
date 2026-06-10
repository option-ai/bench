package tui

import "github.com/charmbracelet/lipgloss"

// Adaptive palette: lipgloss auto-detects the terminal background (via termenv)
// and picks the Light or Dark variant, so output stays legible in both themes.
var (
	cAccent = lipgloss.AdaptiveColor{Light: "#6C4FE0", Dark: "#9D8CFF"} // titles
	cPick   = lipgloss.AdaptiveColor{Light: "#C2185B", Dark: "#FF79C6"} // cursor/selection
	cGood   = lipgloss.AdaptiveColor{Light: "#2E7D32", Dark: "#73F59F"} // success
	cWarn   = lipgloss.AdaptiveColor{Light: "#B26A00", Dark: "#FFB454"} // warning
	cErr    = lipgloss.AdaptiveColor{Light: "#C62828", Dark: "#FF6B6B"} // error
	cDim    = lipgloss.AdaptiveColor{Light: "#6E6E6E", Dark: "#9AA0A6"} // muted

	stTitle    = lipgloss.NewStyle().Bold(true).Foreground(cAccent)
	stPick     = lipgloss.NewStyle().Foreground(cPick)
	stSelected = lipgloss.NewStyle().Foreground(cGood)
	stGood     = lipgloss.NewStyle().Foreground(cGood)
	stWarn     = lipgloss.NewStyle().Foreground(cWarn)
	stErr      = lipgloss.NewStyle().Foreground(cErr)
	stDim      = lipgloss.NewStyle().Foreground(cDim)
	stHead     = lipgloss.NewStyle().Bold(true).Foreground(cDim)
	stHelp     = lipgloss.NewStyle().Foreground(cDim).MarginTop(1)
)
