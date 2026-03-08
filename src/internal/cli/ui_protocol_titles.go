package cli

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

type protocolTitlesModel struct {
	input    textinput.Model
	steps    []string
	done     bool
	canceled bool
}

func newProtocolTitlesModel() protocolTitlesModel {
	in := textinput.New()
	in.Prompt = "Step title: "
	in.Placeholder = "Enter adds steps; empty enter finishes after first step; esc to cancel"
	in.SetWidth(max(len(in.Placeholder), 40))
	in.Focus()
	return protocolTitlesModel{input: in, steps: []string{}}
}

func (m protocolTitlesModel) Init() tea.Cmd { return textinput.Blink }

func (m protocolTitlesModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+c", "esc"))):
			m.canceled = true
			m.done = true
			return m, tea.Quit
		case msg.Code == tea.KeyEnter:
			title := strings.TrimSpace(m.input.Value())
			if title == "" {
				if len(m.steps) == 0 {
					return m, nil
				}
				m.done = true
				return m, tea.Quit
			}
			m.steps = append(m.steps, title)
			m.input.SetValue("")
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m protocolTitlesModel) View() tea.View {
	var b strings.Builder
	b.WriteString(renderScreenTitle("Draft Protocol"))
	b.WriteString("\n\n")
	b.WriteString(m.input.View())
	b.WriteString("\n\n")
	if len(m.steps) > 0 {
		b.WriteString("Steps:\n")
		for i, step := range m.steps {
			b.WriteString("- ")
			b.WriteString(step)
			if i < len(m.steps)-1 {
				b.WriteString("\n")
			}
		}
		b.WriteString("\n\n")
	}
	return tea.NewView(b.String())
}

func runProtocolTitlesPrompt() ([]string, bool, error) {
	res, err := runInteractiveProgram(newProtocolTitlesModel())
	if err != nil {
		return nil, false, err
	}
	m := res.(protocolTitlesModel)
	if m.canceled {
		return nil, true, nil
	}
	return m.steps, false, nil
}
