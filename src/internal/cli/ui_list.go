package cli

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

type listItem string

func (i listItem) FilterValue() string { return string(i) }
func (i listItem) Title() string       { return string(i) }
func (i listItem) Description() string { return "" }

type listModel struct {
	list     list.Model
	selected string
	canceled bool
}

func (m listModel) Init() tea.Cmd { return nil }

func (m listModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if it, ok := m.list.SelectedItem().(listItem); ok {
				m.selected = string(it)
			}
			return m, tea.Quit
		case "esc", "ctrl+c":
			m.canceled = true
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m listModel) View() string { return m.list.View() }

func runSelect(title string, items []string) (string, bool, error) {
	lis := make([]list.Item, 0, len(items))
	for _, item := range items {
		lis = append(lis, listItem(item))
	}
	l := list.New(lis, list.NewDefaultDelegate(), 80, 16)
	l.Title = title
	l.SetShowHelp(true)
	m := listModel{list: l}
	res, err := tea.NewProgram(m).Run()
	if err != nil {
		return "", false, err
	}
	out := res.(listModel)
	if out.canceled {
		return "", true, nil
	}
	if out.selected == "" {
		return "", false, fmt.Errorf("no selection")
	}
	return out.selected, false, nil
}
