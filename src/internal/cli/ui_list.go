package cli

import (
	"fmt"
	"unicode"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
)

type listItem string

func (i listItem) FilterValue() string { return string(i) }
func (i listItem) Title() string       { return string(i) }
func (i listItem) Description() string { return "" }

type labeledListItem struct {
	title  string
	filter string
}

func (i labeledListItem) FilterValue() string { return i.filter }
func (i labeledListItem) Title() string       { return i.title }
func (i labeledListItem) Description() string { return "" }

type listModel struct {
	list     list.Model
	selected string
	canceled bool
}

func (m listModel) Init() tea.Cmd { return nil }

func (m listModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		// Start filtering immediately when typing, without requiring "/".
		startListFilteringOnTextInput(&m.list, msg)
		switch msg.String() {
		case "enter":
			if m.list.SettingFilter() {
				break
			}
			if title, ok := selectedListItemTitle(m.list.SelectedItem()); ok {
				m.selected = title
			}
			return m, tea.Quit
		case "esc", "ctrl+c":
			if m.list.SettingFilter() && msg.String() == "esc" {
				break
			}
			m.canceled = true
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m listModel) View() tea.View { return tea.NewView(m.list.View()) }

func runSelect(title string, items []string) (string, bool, error) {
	lis := make([]list.Item, 0, len(items))
	for _, item := range items {
		lis = append(lis, listItem(item))
	}
	l := list.New(lis, list.NewDefaultDelegate(), 80, 16)
	l.Title = title
	l.SetShowHelp(true)
	m := listModel{list: l}
	res, err := runInteractiveProgram(m)
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

func hasNonSpaceText(text string) bool {
	for _, r := range text {
		if !unicode.IsSpace(r) {
			return true
		}
	}
	return false
}

func startListFilteringOnTextInput(l *list.Model, key tea.KeyPressMsg) {
	if l.SettingFilter() || key.Text == "" || !hasNonSpaceText(key.Text) {
		return
	}
	l.SetShowFilter(true)
	l.SetFilteringEnabled(true)
	l.SetFilterState(list.Filtering)
}

func startListFilteringOnTextInputWithoutInlineFilter(l *list.Model, key tea.KeyPressMsg) {
	if l.SettingFilter() || key.Text == "" || !hasNonSpaceText(key.Text) {
		return
	}
	l.SetFilteringEnabled(true)
	l.SetFilterState(list.Filtering)
}

func resetListFilterIfEmpty(l *list.Model) {
	if l.SettingFilter() && l.FilterValue() == "" {
		l.ResetFilter()
	}
}

func autoSelectTopEntryInFilteredList(l *list.Model, prevFilter string) {
	if !l.SettingFilter() {
		return
	}
	if l.FilterValue() != prevFilter {
		l.ResetSelected()
		return
	}
	if l.SelectedItem() == nil && len(l.VisibleItems()) > 0 {
		l.ResetSelected()
	}
}

func selectedListItemTitle(it list.Item) (string, bool) {
	switch item := it.(type) {
	case listItem:
		return string(item), true
	case labeledListItem:
		return item.title, true
	default:
		return "", false
	}
}
