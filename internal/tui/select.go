// Package tui holds the interactive selection screens for `bench run`: pick the
// evals, pick the models (filtered to installed agents), pick the judge.
package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// Item is one selectable row.
type Item struct {
	Label string
	Desc  string
}

// maxVisible caps the window even on tall terminals: scanning beats scrolling
// a wall of options.
const maxVisible = 12

type selectModel struct {
	title    string
	items    []Item
	cursor   int
	offset   int // first visible row
	visible  int // rows shown at once
	width    int // terminal width; rows are clipped so they never wrap
	chosen   map[int]bool
	multi    bool
	done     bool
	canceled bool
}

func (m selectModel) Init() tea.Cmd { return nil }

// clampWindow keeps the cursor inside the visible window.
func (m *selectModel) clampWindow() {
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+m.visible {
		m.offset = m.cursor - m.visible + 1
	}
	if max := len(m.items) - m.visible; m.offset > max {
		m.offset = max
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

func (m selectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		// leave room for title, indicators, and help text
		m.visible = msg.Height - 6
		if m.visible > maxVisible {
			m.visible = maxVisible
		}
		if m.visible < 3 {
			m.visible = 3
		}
		m.clampWindow()
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
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
		case "pgup":
			m.cursor -= m.visible
			if m.cursor < 0 {
				m.cursor = 0
			}
		case "pgdown":
			m.cursor += m.visible
			if m.cursor > len(m.items)-1 {
				m.cursor = len(m.items) - 1
			}
		case "home", "g":
			m.cursor = 0
		case "end", "G":
			m.cursor = len(m.items) - 1
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
				if !all {
					m.chosen = map[int]bool{}
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
		m.clampWindow()
	}
	return m, nil
}

func (m selectModel) View() string {
	if m.done || m.canceled {
		return ""
	}
	var b strings.Builder
	title := m.title
	if m.multi {
		n := 0
		for _, v := range m.chosen {
			if v {
				n++
			}
		}
		title += stDim.Render(fmt.Sprintf("  (%d/%d selected)", n, len(m.items)))
	}
	b.WriteString(stTitle.Render(title) + "\n\n")

	end := m.offset + m.visible
	if end > len(m.items) {
		end = len(m.items)
	}
	if m.offset > 0 {
		b.WriteString(stDim.Render(fmt.Sprintf("  ↑ %d more", m.offset)) + "\n")
	}
	for i := m.offset; i < end; i++ {
		it := m.items[i]
		cursor := "  "
		if i == m.cursor {
			cursor = stPick.Render("▸ ")
		}
		mark := " "
		if m.multi {
			if m.chosen[i] {
				mark = stSelected.Render("◉")
			} else {
				mark = "◯"
			}
		} else if i == m.cursor {
			mark = stPick.Render("•")
		}
		line := fmt.Sprintf("%s%s %s", cursor, mark, it.Label)
		if it.Desc != "" {
			line += "  " + stDim.Render(it.Desc)
		}
		b.WriteString(clip(line, m.width) + "\n")
	}
	if rest := len(m.items) - end; rest > 0 {
		b.WriteString(stDim.Render(fmt.Sprintf("  ↓ %d more", rest)) + "\n")
	}

	hint := "↑/↓ move · pgup/pgdn jump · enter confirm · q cancel"
	if m.multi {
		hint = "↑/↓ move · space toggle · a all · pgup/pgdn jump · enter confirm · q cancel"
	}
	b.WriteString(stHelp.Render(hint))
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
	m := selectModel{
		title:   title,
		items:   items,
		chosen:  map[int]bool{},
		multi:   multi,
		visible: maxVisible,
	}
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
