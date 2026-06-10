// Package tui holds the interactive selection screens for `bench run`: pick the
// evals, pick the models (filtered to installed agents), pick the judge.
package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
	cursorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("84"))
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).MarginTop(1)
)

// Item is one selectable row.
type Item struct {
	Label string
	Desc  string
}

type selectModel struct {
	title    string
	items    []Item
	cursor   int
	chosen   map[int]bool
	multi    bool
	done     bool
	canceled bool
}

func (m selectModel) Init() tea.Cmd { return nil }

func (m selectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "ctrl+c", "q", "esc":
		m.canceled = true
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.items)-1 {
			m.cursor++
		}
	case " ":
		if m.multi {
			m.chosen[m.cursor] = !m.chosen[m.cursor]
		}
	case "a":
		if m.multi {
			all := len(m.chosen) < len(m.items)
			for i := range m.items {
				m.chosen[i] = all
			}
		}
	case "enter":
		if !m.multi {
			m.chosen = map[int]bool{m.cursor: true}
		}
		if len(m.chosen) == 0 {
			return m, nil // require at least one
		}
		m.done = true
		return m, tea.Quit
	}
	return m, nil
}

func (m selectModel) View() string {
	if m.done || m.canceled {
		return ""
	}
	var b strings.Builder
	b.WriteString(titleStyle.Render(m.title) + "\n\n")
	for i, it := range m.items {
		cursor := "  "
		if i == m.cursor {
			cursor = cursorStyle.Render("▸ ")
		}
		mark := " "
		if m.multi {
			if m.chosen[i] {
				mark = selectedStyle.Render("◉")
			} else {
				mark = "◯"
			}
		} else if i == m.cursor {
			mark = cursorStyle.Render("•")
		}
		line := fmt.Sprintf("%s%s %s", cursor, mark, it.Label)
		if it.Desc != "" {
			line += "  " + dimStyle.Render(it.Desc)
		}
		b.WriteString(line + "\n")
	}
	hint := "↑/↓ move · enter confirm · q cancel"
	if m.multi {
		hint = "↑/↓ move · space toggle · a all · enter confirm · q cancel"
	}
	b.WriteString(helpStyle.Render(hint))
	return b.String()
}

// PickMany shows a multi-select and returns the chosen indices.
func PickMany(title string, items []Item) ([]int, error) {
	return runSelect(title, items, true)
}

// PickOne shows a single-select and returns the chosen index.
func PickOne(title string, items []Item) (int, error) {
	idxs, err := runSelect(title, items, false)
	if err != nil {
		return -1, err
	}
	return idxs[0], nil
}

func runSelect(title string, items []Item, multi bool) ([]int, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("nothing to select")
	}
	m := selectModel{title: title, items: items, chosen: map[int]bool{}, multi: multi}
	out, err := tea.NewProgram(m).Run()
	if err != nil {
		return nil, err
	}
	fm := out.(selectModel)
	if fm.canceled {
		return nil, fmt.Errorf("canceled")
	}
	var idxs []int
	for i := range items {
		if fm.chosen[i] {
			idxs = append(idxs, i)
		}
	}
	return idxs, nil
}
