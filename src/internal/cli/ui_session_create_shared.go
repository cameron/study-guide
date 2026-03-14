package cli

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/list"

	"study-guide/src/internal/store"
)

func buildCreateSessionItems(subjects []store.Subject, selectedBySubject map[string]bool) ([]list.Item, map[string]string) {
	items := make([]list.Item, 0, len(subjects)+2)
	createLookup := map[string]string{}
	if len(subjects) == 0 {
		items = append(items, listItem(sessionsCreateItemLabel("No subjects available")))
	} else {
		for _, s := range subjects {
			marker := "[ ]"
			if selectedBySubject[s.UUID] {
				marker = "[x]"
			}
			label := sessionsCreateItemLabel(fmt.Sprintf("%s %s (%s)", marker, s.Name, strings.Split(s.UUID, "-")[0]))
			items = append(items, labeledListItem{title: label, filter: s.Name})
			createLookup[label] = "subject:" + s.UUID
		}
	}
	createSubjectLabel := sessionsCreateItemLabel(sessionsCreateActionCreateSubject)
	items = append(items, listItem(createSubjectLabel))
	createLookup[createSubjectLabel] = "create-subject"
	createLabel := sessionsCreateItemLabel(sessionsCreateActionCreateSession)
	items = append(items, listItem(createLabel))
	createLookup[createLabel] = "create"
	return items, createLookup
}

func newCreateSessionListModel(items []list.Item) list.Model {
	delegate := newCreateListDelegate()
	createList := list.New(items, delegate, 100, 18)
	createList.Title = "Create Session"
	createList.SetShowTitle(false)
	createList.SetShowFilter(false)
	createList.SetShowHelp(false)
	createList.SetShowStatusBar(false)
	createList.SetShowPagination(false)
	createList.FilterInput.Prompt = "Filter: "
	createList.FilterInput.Placeholder = sessionsCreateFilterPlaceholder
	createList.FilterInput.CharLimit = 120
	createList.FilterInput.Focus()
	applyFilterInputAccentStyle(&createList.FilterInput)
	return createList
}
