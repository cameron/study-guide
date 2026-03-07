package cli

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"

	"study-guide/src/internal/store"
)

type sessionCreatePickerModel struct {
	subjects          []store.Subject
	selectedBySubject map[string]bool
	createLookup      map[string]string
	list              list.Model
	message           string
	requestCreateSubject bool
	canceled          bool
	done              bool
}

func newSessionCreatePickerModel(subjects []store.Subject, selectedBySubject map[string]bool) sessionCreatePickerModel {
	if selectedBySubject == nil {
		selectedBySubject = map[string]bool{}
	}
	m := sessionCreatePickerModel{
		subjects:          append([]store.Subject(nil), subjects...),
		selectedBySubject: map[string]bool{},
		createLookup:      map[string]string{},
	}
	for k, v := range selectedBySubject {
		m.selectedBySubject[k] = v
	}

	delegate := newCreateListDelegate()
	m.list = list.New([]list.Item{}, delegate, 100, 18)
	m.list.Title = "Create Session"
	m.list.SetShowTitle(false)
	m.list.SetShowHelp(false)
	m.list.SetShowStatusBar(false)
	m.list.SetShowPagination(false)
	m.refreshList()
	return m
}

func (m *sessionCreatePickerModel) refreshList() {
	items := make([]list.Item, 0, len(m.subjects)+1)
	m.createLookup = map[string]string{}
	if len(m.subjects) == 0 {
		items = append(items, listItem(sessionsCreateItemLabel("No subjects available")))
	} else {
		for _, s := range m.subjects {
			marker := "[ ]"
			if m.selectedBySubject[s.UUID] {
				marker = "[x]"
			}
			label := sessionsCreateItemLabel(fmt.Sprintf("%s %s (%s)", marker, s.Name, strings.Split(s.UUID, "-")[0]))
			items = append(items, listItem(label))
			m.createLookup[label] = "subject:" + s.UUID
		}
	}
	createSubjectLabel := sessionsCreateItemLabel(sessionsCreateActionCreateSubject)
	items = append(items, listItem(createSubjectLabel))
	m.createLookup[createSubjectLabel] = "create-subject"
	createLabel := sessionsCreateItemLabel(sessionsCreateActionCreateSession)
	items = append(items, listItem(createLabel))
	m.createLookup[createLabel] = "create"
	m.list.SetItems(items)
}

func (m sessionCreatePickerModel) Init() tea.Cmd { return nil }

func (m sessionCreatePickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc", "ctrl+c":
			m.canceled = true
			m.done = true
			return m, tea.Quit
		case "enter":
			it, ok := m.list.SelectedItem().(listItem)
			if !ok {
				return m, nil
			}
			choice := string(it)
			switch token := m.createLookup[choice]; token {
			case "create-subject":
				m.requestCreateSubject = true
				m.done = true
				return m, tea.Quit
			case "create":
				if len(m.SelectedSubjects()) == 0 {
					m.message = "select at least one subject before Create"
					return m, nil
				}
				m.done = true
				return m, tea.Quit
			case "":
				return m, nil
			default:
				uid := strings.TrimPrefix(token, "subject:")
				m.selectedBySubject[uid] = !m.selectedBySubject[uid]
				m.refreshList()
				m.message = ""
				return m, nil
			}
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m sessionCreatePickerModel) View() tea.View {
	var b strings.Builder
	header := m.list.Styles.TitleBar.Render(m.list.Styles.Title.Render("Create Session"))
	b.WriteString(header)
	b.WriteString("\n")
	b.WriteString(subtleTextStyle.Render(sessionsCreateItemLabel(sessionsCreateInfoText)))
	b.WriteString("\n")
	b.WriteString(m.list.View())
	if strings.TrimSpace(m.message) != "" {
		b.WriteString("\n")
		b.WriteString(subtleTextStyle.Render(m.message))
	}
	return tea.NewView(b.String())
}

func (m sessionCreatePickerModel) SelectedSubjects() []store.Subject {
	out := make([]store.Subject, 0, len(m.subjects))
	for _, s := range m.subjects {
		if m.selectedBySubject[s.UUID] {
			out = append(out, s)
		}
	}
	return out
}

func runSessionCreatePicker(studyRoot string) ([]store.Subject, bool, error) {
	selectedBySubject := map[string]bool{}
	for {
		subs, err := store.ListSubjects()
		if err != nil {
			return nil, false, err
		}
		model := newSessionCreatePickerModel(subs, selectedBySubject)
		res, err := runInteractiveProgram(model)
		if err != nil {
			return nil, false, err
		}
		out := res.(sessionCreatePickerModel)
		if out.canceled {
			return nil, true, nil
		}
		if out.requestCreateSubject {
			if err := subjectCreateWithStudyRoot(studyRoot); err != nil {
				return nil, false, err
			}
			selectedBySubject = out.selectedBySubject
			continue
		}
		if !out.done {
			return nil, false, fmt.Errorf("no selection")
		}
		return out.SelectedSubjects(), false, nil
	}
}
