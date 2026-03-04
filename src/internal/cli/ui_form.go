package cli

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type formField struct {
	Name     string
	Label    string
	Required bool
	Value    string
}

type formModel struct {
	title    string
	fields   []formField
	inputs   []textinput.Model
	index    int
	done     bool
	canceled bool
	err      string
}

func newFormModel(title string, fields []formField) formModel {
	inputs := make([]textinput.Model, len(fields))
	for i, f := range fields {
		ti := textinput.New()
		ti.Prompt = f.Label + ": "
		ti.Placeholder = f.Name
		ti.SetValue(f.Value)
		ti.Focus()
		if i != 0 {
			ti.Blur()
		}
		inputs[i] = ti
	}
	return formModel{title: title, fields: fields, inputs: inputs}
}

func (m formModel) Init() tea.Cmd { return textinput.Blink }

func (m formModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+c", "esc"))):
			m.canceled = true
			m.done = true
			return m, tea.Quit
		case msg.Type == tea.KeyEnter:
			if m.fields[m.index].Required && strings.TrimSpace(m.inputs[m.index].Value()) == "" {
				m.err = fmt.Sprintf("%s is required", m.fields[m.index].Label)
				return m, nil
			}
			m.err = ""
			if m.index == len(m.inputs)-1 {
				m.done = true
				return m, tea.Quit
			}
			m.inputs[m.index].Blur()
			m.index++
			m.inputs[m.index].Focus()
			return m, nil
		case msg.Type == tea.KeyShiftTab || msg.String() == "up":
			if m.index > 0 {
				m.inputs[m.index].Blur()
				m.index--
				m.inputs[m.index].Focus()
			}
			return m, nil
		case msg.Type == tea.KeyTab || msg.String() == "down":
			if m.index < len(m.inputs)-1 {
				if m.fields[m.index].Required && strings.TrimSpace(m.inputs[m.index].Value()) == "" {
					m.err = fmt.Sprintf("%s is required", m.fields[m.index].Label)
					return m, nil
				}
				m.err = ""
				m.inputs[m.index].Blur()
				m.index++
				m.inputs[m.index].Focus()
			}
			return m, nil
		}
	}

	cmds := make([]tea.Cmd, len(m.inputs))
	for i := range m.inputs {
		m.inputs[i], cmds[i] = m.inputs[i].Update(msg)
	}
	return m, tea.Batch(cmds...)
}

func (m formModel) View() string {
	var b strings.Builder
	b.WriteString(m.title + "\n\n")
	for _, in := range m.inputs {
		b.WriteString(in.View())
		b.WriteString("\n")
	}
	if m.err != "" {
		b.WriteString("\nError: " + m.err + "\n")
	}
	b.WriteString("\nEnter to continue. Tab/Shift+Tab to move. Esc to cancel.\n")
	return b.String()
}

func runForm(title string, fields []formField) (map[string]string, bool, error) {
	m := newFormModel(title, fields)
	p := tea.NewProgram(m)
	res, err := p.Run()
	if err != nil {
		return nil, false, err
	}
	fm := res.(formModel)
	if fm.canceled {
		return nil, true, nil
	}
	vals := map[string]string{}
	for i, f := range fields {
		vals[f.Name] = strings.TrimSpace(fm.inputs[i].Value())
	}
	return vals, false, nil
}
