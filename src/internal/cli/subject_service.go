package cli

import (
	"strings"

	"study-guide/src/internal/store"
	"study-guide/src/internal/util"
)

func subjectFromCreateValues(vals map[string]string) store.Subject {
	extra := map[string]string{}
	for k, v := range vals {
		switch k {
		case "name", "type", "email", "phone", "age", "sex", "notes":
			continue
		default:
			if strings.TrimSpace(v) != "" {
				extra[k] = strings.TrimSpace(v)
			}
		}
	}
	subjectType := strings.TrimSpace(vals["type"])
	if subjectType == "" {
		subjectType = "person"
	}
	return store.Subject{
		Name:  strings.TrimSpace(vals["name"]),
		Type:  subjectType,
		Email: strings.TrimSpace(vals["email"]),
		Phone: strings.TrimSpace(vals["phone"]),
		Age:   strings.TrimSpace(vals["age"]),
		Sex:   strings.TrimSpace(vals["sex"]),
		Notes: strings.TrimSpace(vals["notes"]),
		Extra: extra,
	}
}

func saveCreatedSubjectRecord(vals map[string]string) (store.Subject, string, error) {
	subject := subjectFromCreateValues(vals)
	path, err := store.SaveSubject(subject)
	if err != nil {
		return store.Subject{}, "", err
	}
	fm, body, err := util.ReadFrontmatterFile(path)
	if err != nil {
		return store.Subject{}, "", err
	}
	return store.SubjectFromFM(path, fm, body), path, nil
}

func saveCreatedSubject(vals map[string]string) (string, error) {
	_, path, err := saveCreatedSubjectRecord(vals)
	return path, err
}

func formValues(m formModel) map[string]string {
	vals := map[string]string{}
	for i, f := range m.fields {
		vals[f.Name] = strings.TrimSpace(m.inputs[i].Value())
	}
	return vals
}

func newSubjectCreateFormModel(studyRoot string) (formModel, error) {
	req, err := subjectCreateRequirements(studyRoot)
	if err != nil {
		return formModel{}, err
	}
	return newFormModel("Create Subject", subjectCreateFormFieldsFromRequirements(req)), nil
}
