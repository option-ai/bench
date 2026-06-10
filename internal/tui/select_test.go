package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func key(s string) tea.KeyMsg {
	switch s {
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "pgdown":
		return tea.KeyMsg{Type: tea.KeyPgDown}
	case "end":
		return tea.KeyMsg{Type: tea.KeyEnd}
	case "space":
		return tea.KeyMsg{Type: tea.KeySpace}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func newModel(n int, multi bool) selectModel {
	items := make([]Item, n)
	for i := range items {
		items[i] = Item{Label: fmt.Sprintf("item-%02d", i)}
	}
	return selectModel{title: "t", items: items, chosen: map[int]bool{}, multi: multi, visible: maxVisible}
}

func TestWindowCapsVisibleRows(t *testing.T) {
	m := newModel(50, true)
	v := m.View()
	if strings.Contains(v, "item-13") {
		t.Fatalf("expected window cap at %d rows, but item-13 is visible:\n%s", maxVisible, v)
	}
	if !strings.Contains(v, "↓ 38 more") {
		t.Fatalf("expected bottom scroll indicator, got:\n%s", v)
	}
}

func TestScrollFollowsCursor(t *testing.T) {
	m := newModel(50, true)
	var mod tea.Model = m
	mod, _ = mod.(selectModel).Update(key("end")) // jump to last item
	v := mod.(selectModel).View()
	if !strings.Contains(v, "item-49") {
		t.Fatalf("expected last item visible after end, got:\n%s", v)
	}
	if !strings.Contains(v, "↑ 38 more") {
		t.Fatalf("expected top scroll indicator, got:\n%s", v)
	}
	if strings.Contains(v, "↓") && strings.Contains(v, "more\n") && strings.Contains(v, "↓ 0 more") {
		t.Fatalf("bottom indicator should be gone at end:\n%s", v)
	}
}

func TestPgDownMovesByWindow(t *testing.T) {
	m := newModel(50, false)
	var mod tea.Model = m
	mod, _ = mod.(selectModel).Update(key("pgdown"))
	got := mod.(selectModel).cursor
	if got != maxVisible {
		t.Fatalf("pgdown: want cursor %d, got %d", maxVisible, got)
	}
}

func TestSelectAllToggleAndCount(t *testing.T) {
	m := newModel(50, true)
	var mod tea.Model = m
	mod, _ = mod.(selectModel).Update(key("a"))
	v := mod.(selectModel).View()
	if !strings.Contains(v, "(50/50 selected)") {
		t.Fatalf("expected all selected, got title line:\n%s", strings.SplitN(v, "\n", 2)[0])
	}
	mod, _ = mod.(selectModel).Update(key("a")) // toggle off
	v = mod.(selectModel).View()
	if !strings.Contains(v, "(0/50 selected)") {
		t.Fatalf("expected none selected after second 'a', got:\n%s", strings.SplitN(v, "\n", 2)[0])
	}
}

func TestSmallTerminalShrinksWindow(t *testing.T) {
	m := newModel(50, true)
	var mod tea.Model = m
	mod, _ = mod.(selectModel).Update(tea.WindowSizeMsg{Width: 80, Height: 10})
	sm := mod.(selectModel)
	if sm.visible != 4 { // 10 - 6 chrome
		t.Fatalf("want visible=4 on height 10, got %d", sm.visible)
	}
}
