package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/list"

	"study-guide/src/internal/store"
	"study-guide/src/internal/util"
)

func buildCreateSessionItems(subjects []store.Subject, selectedBySubject map[string]bool) ([]list.Item, map[string]string) {
	items := make([]list.Item, 0, len(subjects)+2)
	createLookup := map[string]string{}
	orderedSubjects := orderCreateSessionSubjects(subjects)
	if len(subjects) == 0 {
		items = append(items, listItem(sessionsCreateItemLabel("No subjects available")))
	} else {
		for _, s := range orderedSubjects {
			label := sessionsCreateItemLabel(fmt.Sprintf("%s (%s)", s.Name, shortSubjectID(s.UUID)))
			items = append(items, labeledListItem{title: label, filter: s.Name})
			createLookup[label] = "subject:" + s.UUID
		}
	}
	createSubjectLabel := sessionsCreateItemLabel(sessionsCreateActionCreateSubject)
	items = append(items, listItem(createSubjectLabel))
	createLookup[createSubjectLabel] = "create-subject"
	createLabel := sessionsCreateItemLabel(createSessionActionLabel(orderedSubjects, selectedBySubject))
	items = append(items, listItem(createLabel))
	createLookup[createLabel] = "create"
	return items, createLookup
}

func orderCreateSessionSubjects(subjects []store.Subject) []store.Subject {
	ordered := append([]store.Subject(nil), subjects...)
	sort.SliceStable(ordered, func(i, j int) bool {
		left, leftOK := parseSubjectCreatedOn(ordered[i])
		right, rightOK := parseSubjectCreatedOn(ordered[j])
		switch {
		case leftOK && rightOK && !left.Equal(right):
			return left.After(right)
		case leftOK != rightOK:
			return leftOK
		}
		leftName := strings.ToLower(strings.TrimSpace(ordered[i].Name))
		rightName := strings.ToLower(strings.TrimSpace(ordered[j].Name))
		if leftName != rightName {
			return leftName < rightName
		}
		return ordered[i].UUID < ordered[j].UUID
	})
	return ordered
}

func parseSubjectCreatedOn(subject store.Subject) (time.Time, bool) {
	t, err := util.ParseTimestamp(strings.TrimSpace(subject.CreatedOn))
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

func shortSubjectID(uuid string) string {
	for i := 0; i < len(uuid); i++ {
		if uuid[i] == '-' {
			return uuid[:i]
		}
	}
	return uuid
}

func createSessionActionLabel(subjects []store.Subject, selectedBySubject map[string]bool) string {
	selected, ok := firstSelectedSubject(subjects, selectedBySubject)
	if !ok {
		return sessionsCreateActionCreateSession
	}
	return sessionsCreateActionCreateSession + " with " + selected.Name
}

func firstSelectedSubject(subjects []store.Subject, selectedBySubject map[string]bool) (store.Subject, bool) {
	for _, s := range subjects {
		if selectedBySubject[s.UUID] {
			return s, true
		}
	}
	return store.Subject{}, false
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
