package tui

import "github.com/charmbracelet/lipgloss"

// Adaptive palette: lipgloss auto-detects the terminal background (via termenv)
// and picks the Light or Dark variant, so output stays legible in both themes.
//
// Warm dark-terminal aesthetic from the bench design system: violet section
// headers, amber accents (★, badges), score values graded green/amber/red,
// dim gray metadata.
var (
	cAccent = lipgloss.AdaptiveColor{Light: "#6C4FE0", Dark: "#8F7FF7"} // section headers (violet)
	cStar   = lipgloss.AdaptiveColor{Light: "#B26A00", Dark: "#E8A87C"} // ★ winner, accent chip (amber)
	cPick   = lipgloss.AdaptiveColor{Light: "#B26A00", Dark: "#E8A87C"} // cursor/selection
	cGood   = lipgloss.AdaptiveColor{Light: "#2E7D32", Dark: "#86EFAC"} // success / high score
	cWarn   = lipgloss.AdaptiveColor{Light: "#B26A00", Dark: "#F2C14E"} // warning / mid score
	cErr    = lipgloss.AdaptiveColor{Light: "#C62828", Dark: "#F28B82"} // error / low score
	cDim    = lipgloss.AdaptiveColor{Light: "#6E6E6E", Dark: "#8A8F98"} // muted
	cWinBg  = lipgloss.AdaptiveColor{Light: "#E7F3E9", Dark: "#222B22"} // winner-row tint

	stTitle    = lipgloss.NewStyle().Bold(true).Foreground(cAccent)
	stPick     = lipgloss.NewStyle().Foreground(cPick)
	stSelected = lipgloss.NewStyle().Foreground(cGood)
	stStar     = lipgloss.NewStyle().Foreground(cStar)
	stGood     = lipgloss.NewStyle().Foreground(cGood)
	stWarn     = lipgloss.NewStyle().Foreground(cWarn)
	stErr      = lipgloss.NewStyle().Foreground(cErr)
	stDim      = lipgloss.NewStyle().Foreground(cDim)
	stHead     = lipgloss.NewStyle().Bold(true).Foreground(cDim)
	stHelp     = lipgloss.NewStyle().Foreground(cDim).MarginTop(1)
	stWinRow   = lipgloss.NewStyle().Background(cWinBg)

	stChip = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(cDim).
		Foreground(cDim).
		Padding(0, 1)
	stChipAccent = stChip.
			BorderForeground(cStar).
			Foreground(cStar)
)

// scoreStyle grades a score: green ≥ 80, amber ≥ 60, red below.
func scoreStyle(v float64) lipgloss.Style {
	switch {
	case v >= 80:
		return stGood
	case v >= 60:
		return stWarn
	default:
		return stErr
	}
}

// chips renders rounded-border badges side by side. The last chip gets the
// amber accent (mirrors the "config vN" chip in the design).
func chips(labels ...string) string {
	if len(labels) == 0 {
		return ""
	}
	rendered := make([]string, 0, 2*len(labels)-1)
	for i, l := range labels {
		if i > 0 {
			rendered = append(rendered, " ")
		}
		st := stChip
		if i == len(labels)-1 {
			st = stChipAccent
		}
		rendered = append(rendered, st.Render(l))
	}
	return lipgloss.JoinHorizontal(lipgloss.Center, rendered...)
}
