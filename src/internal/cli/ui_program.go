package cli

import tea "charm.land/bubbletea/v2"

type altScreenModel struct {
	inner tea.Model
}

func (m *altScreenModel) Init() tea.Cmd {
	return m.inner.Init()
}

func (m *altScreenModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := m.inner.Update(msg)
	m.inner = next
	return m, cmd
}

func (m *altScreenModel) View() tea.View {
	v := m.inner.View()
	v.AltScreen = true
	return v
}

func runInteractiveProgram(model tea.Model) (tea.Model, error) {
	out, err := tea.NewProgram(&altScreenModel{inner: model}).Run()
	if err != nil {
		return nil, err
	}
	if wrapped, ok := out.(*altScreenModel); ok {
		return wrapped.inner, nil
	}
	return out, nil
}
