package cli

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"io/fs"
	"math"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"charm.land/bubbles/v2/table"
	"github.com/go-pdf/fpdf"
	"github.com/rwcarlsen/goexif/exif"
	"gopkg.in/yaml.v3"

	"study-guide/src/internal/store"
	"study-guide/src/internal/util"
)

var (
	cmdInitRunner                 = cmdInit
	cmdSessionsRunner             = cmdSessions
	runFormRunner                 = runForm
	runProtocolTitlesPromptRunner = runProtocolTitlesPrompt
	runEntrWatchFn                = runEntrWatch
	managedAssetNamePattern       = regexp.MustCompile(`^(?:\d+-)?(\d{8}-\d{6})_([0-9a-f]{8})(\.[^.]+)$`)
)

func Run(args []string) int {
	if len(args) == 0 {
		if err := cmdDefault(); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}
		return 0
	}
	var err error
	switch args[0] {
	case "init":
		err = cmdInit()
	case "protocol":
		err = cmdProtocol(args[1:])
	case "subject":
		err = cmdSubject(args[1:])
	case "session":
		err = cmdSession(args[1:])
	case "sessions":
		err = cmdSessionsWithArgs(args[1:])
	case "status":
		err = cmdStatus(true)
	case "publish":
		err = cmdPublish(args[1:])
	case "export":
		err = cmdExport(args[1:])
	case "__publish-once-at-root":
		err = cmdPublishOnceAtRootArgs(args[1:])
	case "__export-once-at-root":
		err = cmdExportOnceAtRootArgs(args[1:])
	case "data":
		err = cmdData(args[1:])
	case "help", "-h", "--help":
		printHelp()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", args[0])
		printHelp()
		return 2
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	return 0
}

func cmdDefault() error {
	// Inside a study: jump directly into session switchboard.
	if _, err := util.StudyRootFromCwd(); err == nil {
		return cmdSessionsRunner()
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	studyPath := filepath.Join(cwd, "study.sg.md")
	// Missing study file in cwd: scaffold first, then continue into sessions.
	if _, err := os.Stat(studyPath); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		if err := cmdInitRunner(); err != nil {
			return err
		}
		if _, err := os.Stat(studyPath); err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		clearTerminalScreen()
		return cmdSessionsRunner()
	}
	printHelp()
	return nil
}

func printHelp() {
	fmt.Println(`sg - Study Guide CLI

Commands:
  init
  protocol reconcile
  subject create|edit|search|print|ls|rm
  session [advance|reverse [--session <slug>]]
  sessions [print]
  data ingest [--assets-dir <path>]
  data ls
  data clean
  status
  export [--once] [--imgsize=x[,y,...]] [<destination-dir>]
  publish [--once] [--with-subject-names] [<destination-dir>]`)
}

func cmdInit() error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	defaultStudyName := deriveStudyNameFromDir(cwd)
	vals, canceled, err := runFormRunner("Initialize Study", []formField{
		{Name: "study_name", Label: "Study Name", Required: true, Value: defaultStudyName},
	})
	if err != nil {
		return err
	}
	if canceled {
		fmt.Println("canceled")
		return nil
	}
	studyName := strings.TrimSpace(vals["study_name"])
	if studyName == "" {
		studyName = defaultStudyName
	}
	if studyName == "" {
		return errors.New("study name is required")
	}
	protocolSteps, canceled, err := runProtocolTitlesPromptRunner()
	if err != nil {
		return err
	}
	if canceled {
		fmt.Println("canceled")
		return nil
	}
	if len(protocolSteps) == 0 {
		return errors.New("at least one protocol step is required")
	}
	if err := ensureStudyFile(filepath.Join(cwd, "study.sg.md"), studyName, protocolSteps); err != nil {
		return err
	}
	if err := ensureFile(filepath.Join(cwd, "subject-requirements.yaml"), "type: person\n"); err != nil {
		return err
	}
	if err := util.EnsureDir(filepath.Join(cwd, "session")); err != nil {
		return err
	}
	fmt.Println("initialized study scaffold")
	return nil
}

func deriveStudyNameFromDir(cwd string) string {
	dir := strings.TrimSpace(filepath.Base(cwd))
	if dir == "" || dir == "." {
		return ""
	}
	dir = strings.NewReplacer("-", " ", "_", " ").Replace(dir)
	parts := strings.Fields(dir)
	for i, part := range parts {
		parts[i] = titleWord(part)
	}
	return strings.Join(parts, " ")
}

func titleWord(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(strings.ToLower(s))
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

func ensureFile(path, content string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func ensureStudyFile(path, studyName string, steps []string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	fm := map[string]any{
		"status":     "WIP",
		"created_on": util.NowTimestamp(),
	}
	var protocol strings.Builder
	protocol.WriteString("Describe the protocol.\n\n## Protocol")
	for _, step := range steps {
		if strings.TrimSpace(step) == "" {
			continue
		}
		protocol.WriteString("\n\n### ")
		protocol.WriteString(strings.TrimSpace(step))
	}
	body := fmt.Sprintf("# %s\n\n# Introduction\n\n# Methods\n\n%s\n\n# Results\n\n# Discussion\n\n# Conclusion\n\n# Special Thanks\n", studyName, protocol.String())
	return util.WriteFrontmatterFile(path, fm, body)
}

func clearTerminalScreen() {
	fmt.Print("\x1b[2J\x1b[H")
}

func cmdSubject(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: sg subject create|edit|search|print|ls|rm")
	}
	switch args[0] {
	case "create":
		return subjectCreate()
	case "edit":
		if len(args) < 2 {
			return errors.New("usage: sg subject edit <id-or-name>")
		}
		return subjectEdit(strings.Join(args[1:], " "))
	case "ls":
		return subjectList()
	case "search":
		if len(args) < 2 {
			return errors.New("usage: sg subject search <name>")
		}
		return subjectSearch(strings.Join(args[1:], " "))
	case "print":
		return subjectPrint(args[1:])
	case "rm":
		if len(args) < 2 {
			return errors.New("usage: sg subject rm <id>")
		}
		return store.RemoveSubject(args[1])
	default:
		return fmt.Errorf("unknown subject subcommand: %s", args[0])
	}
}

func cmdProtocol(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: sg protocol reconcile")
	}
	switch args[0] {
	case "reconcile":
		if len(args) > 1 {
			return fmt.Errorf("unknown argument: %s", args[1])
		}
		root, err := util.StudyRootFromCwd()
		if err != nil {
			return err
		}
		protocol, err := store.ParseProtocol(root)
		if err != nil {
			return err
		}
		if len(protocol.Steps) == 0 {
			return errors.New("no protocol steps found")
		}
		fmt.Println("reconciled protocol step directories")
		return nil
	default:
		return fmt.Errorf("unknown protocol subcommand: %s", args[0])
	}
}

func subjectCreate() error {
	return subjectCreateWithStudyRoot("")
}

func subjectCreateWithStudyRoot(studyRoot string) error {
	subject, canceled, path, err := createSubjectWithStudyRoot(studyRoot)
	if err != nil {
		return err
	}
	if canceled {
		fmt.Println("canceled")
		return nil
	}
	_ = subject
	fmt.Println("created", path)
	return nil
}

func createSubjectWithStudyRoot(studyRoot string) (store.Subject, bool, string, error) {
	form, err := newSubjectCreateFormModel(studyRoot)
	if err != nil {
		return store.Subject{}, false, "", err
	}
	vals, canceled, err := runForm(form.title, form.fields)
	if err != nil {
		return store.Subject{}, false, "", err
	}
	if canceled {
		return store.Subject{}, true, "", nil
	}
	subject, path, err := saveCreatedSubjectRecord(vals)
	if err != nil {
		return store.Subject{}, false, "", err
	}
	return subject, false, path, nil
}

func subjectCreateRequirements(studyRoot string) (store.SubjectRequirements, error) {
	req := store.SubjectRequirements{
		RequiredFields: []string{"name"},
		FixedFields:    map[string]string{},
	}
	root := strings.TrimSpace(studyRoot)
	if root == "" {
		detectedRoot, err := util.StudyRootFromCwd()
		if err == nil {
			root = detectedRoot
		}
	}
	if root != "" {
		studyReq, err := store.ReadSubjectRequirements(root)
		if err != nil {
			return store.SubjectRequirements{}, err
		}
		req = studyReq
		if len(req.RequiredFields) == 0 {
			req.RequiredFields = []string{"name"}
		}
	}
	return req, nil
}

func subjectCreateFormFieldsFromRequirements(req store.SubjectRequirements) []formField {
	requiredMap := map[string]bool{}
	for _, f := range req.RequiredFields {
		key := strings.ToLower(strings.TrimSpace(f))
		if key != "" {
			requiredMap[key] = true
		}
	}
	fixedMap := map[string]string{}
	for k, v := range req.FixedFields {
		key := strings.ToLower(strings.TrimSpace(k))
		if key != "" {
			fixedMap[key] = strings.TrimSpace(v)
		}
	}
	if len(requiredMap) == 0 {
		requiredMap["name"] = true
	}

	builtins := []string{"name", "type", "email", "phone", "age", "sex"}
	fields := make([]formField, 0, len(builtins)+len(requiredMap)+len(fixedMap)+1)
	seen := map[string]bool{}
	for _, key := range builtins {
		f := formField{
			Name:     key,
			Label:    humanizeFieldName(key),
			Required: requiredMap[key],
		}
		if fixed, ok := fixedMap[key]; ok {
			f.Required = true
			f.ReadOnly = true
			f.Value = fixed
			f.Label += " (fixed)"
		}
		fields = append(fields, f)
		seen[key] = true
	}

	for _, raw := range req.RequiredFields {
		key := strings.ToLower(strings.TrimSpace(raw))
		if key == "" || seen[key] {
			continue
		}
		f := formField{
			Name:     key,
			Label:    humanizeFieldName(key),
			Required: true,
		}
		if fixed, ok := fixedMap[key]; ok {
			f.ReadOnly = true
			f.Value = fixed
			f.Label += " (fixed)"
		}
		fields = append(fields, f)
		seen[key] = true
	}

	remainingFixed := make([]string, 0, len(fixedMap))
	for key := range fixedMap {
		if seen[key] {
			continue
		}
		remainingFixed = append(remainingFixed, key)
	}
	sort.Strings(remainingFixed)
	for _, key := range remainingFixed {
		fields = append(fields, formField{
			Name:     key,
			Label:    humanizeFieldName(key) + " (fixed)",
			Required: true,
			ReadOnly: true,
			Value:    fixedMap[key],
		})
	}

	fields = append(fields, formField{Name: "notes", Label: "Notes", Required: false})
	return fields
}

func humanizeFieldName(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ""
	}
	parts := strings.FieldsFunc(trimmed, func(r rune) bool {
		return r == '_' || r == '-' || r == ' '
	})
	for i := range parts {
		if parts[i] == "" {
			continue
		}
		parts[i] = strings.ToUpper(parts[i][:1]) + strings.ToLower(parts[i][1:])
	}
	return strings.Join(parts, " ")
}

func subjectList() error {
	subs, err := store.ListSubjects()
	if err != nil {
		return err
	}
	if len(subs) == 0 {
		fmt.Println("no subjects")
		return nil
	}
	for _, s := range subs {
		fmt.Printf("- %s (%s)\n", s.Name, s.UUID)
	}
	return nil
}

func subjectSearch(q string) error {
	matches, err := store.FindSubject(q)
	if err != nil {
		return err
	}
	for _, s := range matches {
		fmt.Printf("- %s (%s)\n", s.Name, s.UUID)
	}
	if len(matches) == 0 {
		fmt.Println("no matches")
	}
	return nil
}

func subjectPrint(args []string) error {
	all, q := parseSubjectPrintArgs(args)
	if q == "" {
		subjects, err := subjectPrintList(all)
		if err != nil {
			return err
		}
		if len(subjects) == 0 {
			fmt.Println("no subjects")
			return nil
		}
		for _, s := range subjects {
			fmt.Printf("- %s (%s)\n", s.Name, s.UUID)
		}
		return nil
	}
	matches, err := store.FindSubject(q)
	if err != nil {
		return err
	}
	if len(matches) == 0 {
		return errors.New("not found")
	}
	if len(matches) > 1 {
		return errors.New("ambiguous")
	}
	s := matches[0]
	md := fmt.Sprintf("# Subject\n\n- Name: %s\n- UUID: %s\n- Email: %s\n- Phone: %s\n- Age: %s\n- Sex: %s\n", s.Name, s.UUID, s.Email, s.Phone, s.Age, s.Sex)
	util.PrintMarkdown(md)
	return nil
}

func parseSubjectPrintArgs(args []string) (bool, string) {
	all := false
	queryParts := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "--all" {
			all = true
			continue
		}
		queryParts = append(queryParts, arg)
	}
	return all, strings.TrimSpace(strings.Join(queryParts, " "))
}

func subjectPrintList(all bool) ([]store.Subject, error) {
	subjects, err := store.ListSubjects()
	if err != nil {
		return nil, err
	}
	if all {
		return subjects, nil
	}
	root, err := util.StudyRootFromCwd()
	if err != nil {
		return subjects, nil
	}
	return currentStudySubjects(root, subjects)
}

func currentStudySubjects(root string, subjects []store.Subject) ([]store.Subject, error) {
	slugs, err := listSessionSlugs(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	ids := map[string]bool{}
	names := map[string]bool{}
	for _, slug := range slugs {
		_, body, err := util.ReadFrontmatterFile(filepath.Join(root, "session", slug, "session.sg.md"))
		if err != nil {
			return nil, err
		}
		for _, ref := range parseSessionSubjects(body) {
			if id := strings.TrimSpace(ref.ID); id != "" {
				ids[id] = true
			}
			if name := strings.TrimSpace(ref.Name); name != "" {
				names[strings.ToLower(name)] = true
			}
		}
	}
	filtered := make([]store.Subject, 0, len(subjects))
	seen := map[string]bool{}
	for _, s := range subjects {
		if seen[s.UUID] {
			continue
		}
		if ids[s.UUID] || names[strings.ToLower(strings.TrimSpace(s.Name))] {
			filtered = append(filtered, s)
			seen[s.UUID] = true
		}
	}
	return filtered, nil
}

func subjectEdit(q string) error {
	s, err := store.ResolveSubject(q)
	if err != nil {
		return err
	}
	fields := []formField{
		{Name: "name", Label: "Name", Required: true, Value: s.Name},
		{Name: "email", Label: "Email", Value: s.Email},
		{Name: "phone", Label: "Phone", Value: s.Phone},
		{Name: "age", Label: "Age", Value: s.Age},
		{Name: "sex", Label: "Sex", Value: s.Sex},
		{Name: "notes", Label: "Notes", Value: s.Notes},
	}
	vals, canceled, err := runForm("Edit Subject", fields)
	if err != nil {
		return err
	}
	if canceled {
		fmt.Println("canceled")
		return nil
	}
	s.Name = vals["name"]
	s.Email = vals["email"]
	s.Phone = vals["phone"]
	s.Age = vals["age"]
	s.Sex = vals["sex"]
	s.Notes = vals["notes"]
	path, err := store.SaveSubject(s)
	if err != nil {
		return err
	}
	fmt.Println("updated", path)
	return nil
}

func cmdSession(args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "advance":
			return cmdSessionAdvance(args[1:])
		case "reverse":
			return cmdSessionReverse(args[1:])
		default:
			return fmt.Errorf("unknown session subcommand: %s", args[0])
		}
	}

	root, err := util.StudyRootFromCwd()
	if err != nil {
		return err
	}
	selected, err := selectSubjectsForSession(root)
	if err != nil {
		return err
	}
	protocol, err := store.ParseProtocol(root)
	if err != nil {
		return err
	}
	if len(protocol.Steps) == 0 {
		return errors.New("no protocol steps found")
	}
	_, sessionDir, err := createSessionScaffold(root, selected, protocol)
	if err != nil {
		return err
	}
	var prevStepPath string
	for i, step := range protocol.Steps {
		if prevStepPath != "" {
			if _, canceled, err := runSelect("Advance to next step", []string{"Continue", "Cancel"}); err != nil {
				return err
			} else if canceled {
				return nil
			}
			now := util.NowTimestamp()
			if err := setFrontmatterField(prevStepPath, "time_finished", now); err != nil {
				return err
			}
			if err := closeOpenFocusWindow(prevStepPath, now); err != nil {
				return err
			}
		}
		stepDir := filepath.Join(sessionDir, "step", step.Slug)
		if err := util.EnsureDir(filepath.Join(stepDir, "asset")); err != nil {
			return err
		}
		stepPath := filepath.Join(stepDir, "step.sg.md")
		now := util.NowTimestamp()
		fm := map[string]any{
			"time_started": now,
			"focus_windows": []map[string]any{
				{"time_started": now},
			},
		}
		if err := util.WriteFrontmatterFile(stepPath, fm, ""); err != nil {
			return err
		}
		prevStepPath = stepPath
		fmt.Printf("started step %d/%d: %s\n", i+1, len(protocol.Steps), step.Name)
	}
	if prevStepPath != "" {
		if _, canceled, err := runSelect("Finish session?", []string{"Finish", "Cancel"}); err != nil {
			return err
		} else if canceled {
			return nil
		}
		now := util.NowTimestamp()
		if err := setFrontmatterField(prevStepPath, "time_finished", now); err != nil {
			return err
		}
		if err := closeOpenFocusWindow(prevStepPath, now); err != nil {
			return err
		}
	}
	fmt.Println("session complete:", sessionDir)
	return nil
}

func cmdSessions() error {
	return cmdSessionsWithArgs(nil)
}

func cmdSessionsWithArgs(args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "print":
			if len(args) > 1 {
				return fmt.Errorf("unknown argument: %s", args[1])
			}
			return cmdSessionsPrint()
		default:
			return fmt.Errorf("unknown sessions subcommand: %s", args[0])
		}
	}

	root, err := util.StudyRootFromCwd()
	if err != nil {
		return err
	}
	protocol, err := store.ParseProtocol(root)
	if err != nil {
		return err
	}
	if len(protocol.Steps) == 0 {
		return errors.New("no protocol steps found")
	}
	return runSessionsSwitchboard(root, protocol)
}

func cmdData(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: sg data <ingest [--assets-dir <path>] | ls | clean>")
	}
	switch args[0] {
	case "ingest":
		return cmdIngestPhotos(args[1:])
	case "ls":
		if len(args) > 1 {
			return fmt.Errorf("unknown argument: %s", args[1])
		}
		return cmdDataLs()
	case "clean":
		return cmdDataClean(args[1:])
	default:
		return fmt.Errorf("unknown data subcommand: %s", args[0])
	}
}

func cmdDataLs() error {
	root, err := util.StudyRootFromCwd()
	if err != nil {
		return err
	}
	if err := reconcileProtocolStepSlugs(root); err != nil {
		return err
	}
	sessionRoot := filepath.Join(root, "session")
	type dataAssetRow struct {
		Session string
		Step    string
		File    string
	}
	rows := make([]dataAssetRow, 0)
	walkErr := filepath.WalkDir(sessionRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(sessionRoot, path)
		if err != nil {
			return err
		}
		parts := strings.Split(filepath.ToSlash(rel), "/")
		if len(parts) < 5 {
			return nil
		}
		if parts[1] != "step" || parts[3] != "asset" {
			return nil
		}
		if strings.EqualFold(filepath.Base(path), ".DS_Store") {
			return nil
		}
		rows = append(rows, dataAssetRow{
			Session: parts[0],
			Step:    parts[2],
			File:    strings.Join(parts[4:], "/"),
		})
		return nil
	})
	if walkErr != nil && !os.IsNotExist(walkErr) {
		return walkErr
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Session != rows[j].Session {
			return rows[i].Session < rows[j].Session
		}
		if rows[i].Step != rows[j].Step {
			return rows[i].Step < rows[j].Step
		}
		return rows[i].File < rows[j].File
	})

	tblRows := make([]table.Row, 0, len(rows))
	sessionWidth := len("SESSION")
	stepWidth := len("STEP")
	fileWidth := len("FILE")
	for _, r := range rows {
		tblRows = append(tblRows, table.Row{r.Session, r.Step, r.File})
		if len(r.Session) > sessionWidth {
			sessionWidth = len(r.Session)
		}
		if len(r.Step) > stepWidth {
			stepWidth = len(r.Step)
		}
		if len(r.File) > fileWidth {
			fileWidth = len(r.File)
		}
	}
	tbl := table.New(
		table.WithColumns([]table.Column{
			{Title: "SESSION", Width: max(12, sessionWidth)},
			{Title: "STEP", Width: max(12, stepWidth)},
			{Title: "FILE", Width: max(12, fileWidth)},
		}),
		table.WithWidth(max(80, sessionWidth+stepWidth+fileWidth+6)),
		table.WithFocused(false),
		table.WithHeight(max(2, len(tblRows)+1)),
	)
	tbl.SetRows(tblRows)
	tbl.SetStyles(table.DefaultStyles())
	fmt.Println(tbl.View())
	fmt.Printf("assets total: %d\n", len(rows))
	return nil
}

func cmdSessionsPrint() error {
	root, err := util.StudyRootFromCwd()
	if err != nil {
		return err
	}
	protocol, err := store.ParseProtocol(root)
	if err != nil {
		return err
	}
	if len(protocol.Steps) == 0 {
		return errors.New("no protocol steps found")
	}
	slugs, err := listSessionSlugs(root)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		slugs = nil
	}

	type sessionsPrintRow struct {
		Session string
		Step    string
		Start   string
		End     string
	}
	rows := make([]sessionsPrintRow, 0, len(slugs)*len(protocol.Steps))
	for _, slug := range slugs {
		for _, step := range protocol.Steps {
			start := ""
			end := ""
			stepPath := filepath.Join(root, "session", slug, "step", step.Slug, "step.sg.md")
			fm, _, err := util.ReadFrontmatterFile(stepPath)
			if err != nil {
				if !os.IsNotExist(err) {
					return err
				}
			} else {
				start = strings.TrimSpace(asString(fm["time_started"]))
				end = strings.TrimSpace(asString(fm["time_finished"]))
			}
			rows = append(rows, sessionsPrintRow{
				Session: slug,
				Step:    step.Slug,
				Start:   start,
				End:     end,
			})
		}
	}
	tblRows := make([]table.Row, 0, len(rows))
	sessionWidth := len("SESSION")
	stepWidth := len("STEP")
	startWidth := len("START")
	endWidth := len("END")
	for _, r := range rows {
		tblRows = append(tblRows, table.Row{r.Session, r.Step, r.Start, r.End})
		if len(r.Session) > sessionWidth {
			sessionWidth = len(r.Session)
		}
		if len(r.Step) > stepWidth {
			stepWidth = len(r.Step)
		}
		if len(r.Start) > startWidth {
			startWidth = len(r.Start)
		}
		if len(r.End) > endWidth {
			endWidth = len(r.End)
		}
	}
	tbl := table.New(
		table.WithColumns([]table.Column{
			{Title: "SESSION", Width: max(12, sessionWidth)},
			{Title: "STEP", Width: max(12, stepWidth)},
			{Title: "START", Width: max(19, startWidth)},
			{Title: "END", Width: max(19, endWidth)},
		}),
		table.WithWidth(max(80, sessionWidth+stepWidth+startWidth+endWidth+8)),
		table.WithFocused(false),
		table.WithHeight(max(2, len(tblRows)+1)),
	)
	tbl.SetRows(tblRows)
	tbl.SetStyles(table.DefaultStyles())
	fmt.Println(tbl.View())
	return nil
}

func cmdSessionAdvance(args []string) error {
	root, err := util.StudyRootFromCwd()
	if err != nil {
		return err
	}
	protocol, err := store.ParseProtocol(root)
	if err != nil {
		return err
	}
	if len(protocol.Steps) == 0 {
		return errors.New("no protocol steps found")
	}

	sessionSlug, err := parseSessionTargetArgs(args)
	if err != nil {
		return err
	}
	if sessionSlug == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		inferred, ok := inferSessionSlugFromCwd(root, cwd)
		if !ok {
			return errors.New("could not infer session from current directory; pass --session <slug>")
		}
		sessionSlug = inferred
	}

	res, err := advanceSessionOnce(root, sessionSlug, protocol)
	if err != nil {
		return err
	}
	fmt.Printf("session=%s state=%s step=%s\n", sessionSlug, res.State, res.StepSlug)
	return nil
}

func cmdSessionReverse(args []string) error {
	root, err := util.StudyRootFromCwd()
	if err != nil {
		return err
	}
	protocol, err := store.ParseProtocol(root)
	if err != nil {
		return err
	}
	if len(protocol.Steps) == 0 {
		return errors.New("no protocol steps found")
	}

	sessionSlug, err := parseSessionTargetArgs(args)
	if err != nil {
		return err
	}
	if sessionSlug == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		inferred, ok := inferSessionSlugFromCwd(root, cwd)
		if !ok {
			return errors.New("could not infer session from current directory; pass --session <slug>")
		}
		sessionSlug = inferred
	}

	res, err := reverseSessionOnce(root, sessionSlug, protocol)
	if err != nil {
		return err
	}
	fmt.Printf("session=%s state=%s step=%s\n", sessionSlug, res.State, res.StepSlug)
	return nil
}

func parseSessionTargetArgs(args []string) (string, error) {
	var sessionSlug string
	for i := 0; i < len(args); i++ {
		arg := strings.TrimSpace(args[i])
		if arg == "" {
			continue
		}
		switch {
		case arg == "--session":
			if i+1 >= len(args) || strings.TrimSpace(args[i+1]) == "" {
				return "", errors.New("missing value for --session")
			}
			sessionSlug = strings.TrimSpace(args[i+1])
			i++
		case strings.HasPrefix(arg, "--session="):
			sessionSlug = strings.TrimSpace(strings.TrimPrefix(arg, "--session="))
			if sessionSlug == "" {
				return "", errors.New("missing value for --session")
			}
		default:
			return "", fmt.Errorf("unknown argument: %s", arg)
		}
	}
	return sessionSlug, nil
}

func inferSessionSlugFromCwd(root, cwd string) (string, bool) {
	sessionRoot := filepath.Join(root, "session")
	rel, err := filepath.Rel(sessionRoot, cwd)
	if err != nil {
		return "", false
	}
	if rel == "." || strings.HasPrefix(rel, "..") {
		return "", false
	}
	parts := strings.Split(filepath.Clean(rel), string(filepath.Separator))
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" || parts[0] == "." {
		return "", false
	}
	slug := parts[0]
	info, err := os.Stat(filepath.Join(sessionRoot, slug))
	if err != nil || !info.IsDir() {
		return "", false
	}
	return slug, true
}

type sessionRecord struct {
	Slug           string
	SubjectNames   []string
	CurrentStep    string
	CurrentStepIdx int
	ProgressSteps  int
	StepCount      int
	NextStep       string
	Complete       bool
	Active         bool
	NextAction     string
	InvalidReason  string
}

func asBool(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		switch strings.ToLower(strings.TrimSpace(x)) {
		case "true", "yes", "1":
			return true
		}
	}
	return false
}

func stepIsUnfocusable(stepPath string) (bool, error) {
	fm, _, err := util.ReadFrontmatterFile(stepPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return asBool(fm["unfocusable"]), nil
}

func loadSessionRecords(root string, protocol store.Protocol, subjectByID map[string]store.Subject) ([]sessionRecord, error) {
	slugs, err := listSessionSlugs(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	incomplete := make([]sessionRecord, 0, len(slugs))
	complete := make([]sessionRecord, 0, len(slugs))
	for _, slug := range slugs {
		sessionPath := filepath.Join(root, "session", slug, "session.sg.md")
		_, body, err := util.ReadFrontmatterFile(sessionPath)
		if err != nil {
			continue
		}
		subjectNames := make([]string, 0)
		for _, subj := range parseSessionSubjects(body) {
			name := subj.Name
			if s, ok := subjectByID[subj.ID]; ok {
				name = s.Name
			}
			if strings.TrimSpace(name) != "" {
				subjectNames = append(subjectNames, name)
			}
		}
		rec := sessionRecord{
			Slug:           slug,
			SubjectNames:   subjectNames,
			CurrentStepIdx: -1,
			ProgressSteps:  0,
			StepCount:      countFocusableProtocolSteps(filepath.Join(root, "session", slug), protocol),
		}
		progress, err := inspectSessionProgress(filepath.Join(root, "session", slug), protocol)
		if err != nil {
			rec.NextAction = "invalid"
			rec.InvalidReason = err.Error()
		} else {
			rec.ProgressSteps = progress.ProgressSteps
			if progress.CurrentStepIdx >= 0 && progress.CurrentStepIdx < len(protocol.Steps) {
				rec.CurrentStep = protocol.Steps[progress.CurrentStepIdx].Name
				rec.CurrentStepIdx = progress.CurrentStepIdx
			}
			rec.NextAction = progress.NextAction
			switch progress.NextAction {
			case "start":
				if progress.NextStepIdx >= 0 && progress.NextStepIdx < len(protocol.Steps) {
					rec.NextStep = protocol.Steps[progress.NextStepIdx].Name
				}
			case "advance":
				if progress.NextStepIdx >= 0 && progress.NextStepIdx < len(protocol.Steps) {
					rec.NextStep = protocol.Steps[progress.NextStepIdx].Name
				}
			case "finish":
				rec.NextStep = "conclude"
			}
			rec.Complete = progress.SessionFinished && progress.NextAction == "none"
			if progress.SessionFinished && progress.NextAction != "none" {
				rec.InvalidReason = "session marked finished but protocol steps are incomplete"
			}
		}
		if rec.Complete {
			complete = append(complete, rec)
		} else {
			incomplete = append(incomplete, rec)
		}
	}
	return append(incomplete, complete...), nil
}

func sessionActionLabel(nextAction string) string {
	switch nextAction {
	case "start":
		return "Start first step"
	case "advance":
		return "Advance to next step"
	case "finish":
		return "Finish session"
	case "invalid":
		return "Invalid session state"
	default:
		return "No action"
	}
}

type sessionProgress struct {
	ActiveStepIdx   int
	CurrentStepIdx  int
	FirstUnstarted  int
	ProgressSteps   int
	StepCount       int
	NextStepIdx     int
	AnyStepStarted  bool
	SessionFinished bool
	NextAction      string
}

func inspectSessionProgress(sessionDir string, protocol store.Protocol) (sessionProgress, error) {
	if len(protocol.Steps) == 0 {
		return sessionProgress{}, errors.New("protocol has no steps")
	}
	p := sessionProgress{
		ActiveStepIdx:   -1,
		CurrentStepIdx:  -1,
		FirstUnstarted:  -1,
		NextStepIdx:     -1,
		ProgressSteps:   0,
		SessionFinished: false,
	}
	startedFlags := make([]bool, len(protocol.Steps))
	finishedFlags := make([]bool, len(protocol.Steps))
	unfocusableFlags := make([]bool, len(protocol.Steps))
	for i, st := range protocol.Steps {
		stepPath := filepath.Join(sessionDir, "step", st.Slug, "step.sg.md")
		fm, _, err := util.ReadFrontmatterFile(stepPath)
		if err != nil {
			if os.IsNotExist(err) {
				if p.FirstUnstarted == -1 {
					p.FirstUnstarted = i
				}
				continue
			}
			return sessionProgress{}, err
		}
		unfocusableFlags[i] = asBool(fm["unfocusable"])
		if !unfocusableFlags[i] {
			p.StepCount++
		}
		started := strings.TrimSpace(asString(fm["time_started"])) != ""
		finished := strings.TrimSpace(asString(fm["time_finished"])) != ""
		if !started {
			if p.FirstUnstarted == -1 {
				p.FirstUnstarted = i
			}
			if finished {
				return sessionProgress{}, fmt.Errorf("invalid step timing (time_finished without time_started): %s", stepPath)
			}
			continue
		}
		p.AnyStepStarted = true
		startedFlags[i] = true
		finishedFlags[i] = finished
	}

	seenUnstarted := false
	for i := range protocol.Steps {
		if !startedFlags[i] {
			seenUnstarted = true
			continue
		}
		if seenUnstarted {
			return sessionProgress{}, fmt.Errorf("non-contiguous protocol progress: step %q started before earlier missing step", protocol.Steps[i].Slug)
		}
	}

	// Treat an earlier started step as effectively finished when a later step has started.
	lastProgressedFocusable := -1
	for i := range protocol.Steps {
		if !startedFlags[i] {
			if p.FirstUnstarted == -1 {
				p.FirstUnstarted = i
			}
			continue
		}
		effectiveFinished := finishedFlags[i]
		if !effectiveFinished {
			for j := i + 1; j < len(protocol.Steps); j++ {
				if startedFlags[j] {
					effectiveFinished = true
					break
				}
			}
		}
		if !effectiveFinished {
			if p.ActiveStepIdx != -1 {
				return sessionProgress{}, errors.New("multiple active steps found")
			}
			p.ActiveStepIdx = i
			if !unfocusableFlags[i] {
				p.ProgressSteps++
				p.CurrentStepIdx = i
			}
			continue
		}
		if !unfocusableFlags[i] {
			p.ProgressSteps++
			lastProgressedFocusable = i
		}
	}
	if p.CurrentStepIdx == -1 {
		p.CurrentStepIdx = lastProgressedFocusable
	}
	nextFocusableIdx := func(start int) int {
		if start < 0 {
			start = 0
		}
		for i := start; i < len(protocol.Steps); i++ {
			if !unfocusableFlags[i] {
				return i
			}
		}
		return -1
	}
	p.SessionFinished = p.FirstUnstarted == -1 && p.ActiveStepIdx == -1 && p.AnyStepStarted
	switch {
	case p.SessionFinished:
		p.NextAction = "none"
	case p.ActiveStepIdx >= 0:
		p.NextStepIdx = nextFocusableIdx(p.ActiveStepIdx + 1)
		if p.NextStepIdx >= 0 {
			p.NextAction = "advance"
		} else {
			p.NextAction = "finish"
		}
	case !p.AnyStepStarted:
		p.NextStepIdx = nextFocusableIdx(0)
		if p.NextStepIdx >= 0 {
			p.NextAction = "start"
		} else {
			p.NextAction = "finish"
		}
	case p.FirstUnstarted >= 0:
		p.NextStepIdx = nextFocusableIdx(p.FirstUnstarted)
		if p.NextStepIdx >= 0 {
			p.NextAction = "start"
		} else {
			p.NextAction = "finish"
		}
	default:
		p.NextAction = "finish"
	}
	return p, nil
}

func countFocusableProtocolSteps(sessionDir string, protocol store.Protocol) int {
	count := 0
	for _, step := range protocol.Steps {
		stepPath := filepath.Join(sessionDir, "step", step.Slug, "step.sg.md")
		unfocusable, err := stepIsUnfocusable(stepPath)
		if err != nil {
			count++
			continue
		}
		if !unfocusable {
			count++
		}
	}
	return count
}

type sessionAdvanceResult struct {
	State    string
	StepSlug string
}

func advanceSessionOnce(root, sessionSlug string, protocol store.Protocol) (sessionAdvanceResult, error) {
	sessionDir := filepath.Join(root, "session", sessionSlug)
	if info, err := os.Stat(sessionDir); err != nil || !info.IsDir() {
		return sessionAdvanceResult{}, fmt.Errorf("session not found: %s", sessionSlug)
	}
	sessionPath := filepath.Join(sessionDir, "session.sg.md")
	if _, err := os.Stat(sessionPath); err != nil {
		return sessionAdvanceResult{}, fmt.Errorf("session missing file: %s", sessionPath)
	}

	progress, err := inspectSessionProgress(sessionDir, protocol)
	if err != nil {
		return sessionAdvanceResult{}, err
	}
	if progress.SessionFinished {
		return sessionAdvanceResult{}, errors.New("session already finished")
	}
	now := util.NowTimestamp()
	focusedSlug, err := readStudyActiveSessionSlug(root)
	if err != nil {
		return sessionAdvanceResult{}, err
	}
	isFocused := strings.TrimSpace(focusedSlug) == sessionSlug
	switch progress.NextAction {
	case "start":
		target := 0
		if progress.FirstUnstarted >= 0 {
			target = progress.FirstUnstarted
		}
		landedIdx, finished, err := startSkippingUnfocusableSteps(sessionDir, protocol, target, now)
		if err != nil {
			return sessionAdvanceResult{}, err
		}
		if isFocused {
			if err := syncFocusedSessionWindows(root, sessionSlug, protocol, now); err != nil {
				return sessionAdvanceResult{}, err
			}
		}
		if finished {
			lastStep := protocol.Steps[len(protocol.Steps)-1]
			return sessionAdvanceResult{State: "finished", StepSlug: lastStep.Slug}, nil
		}
		return sessionAdvanceResult{State: "started", StepSlug: protocol.Steps[landedIdx].Slug}, nil
	case "advance":
		if progress.ActiveStepIdx < 0 || progress.ActiveStepIdx >= len(protocol.Steps)-1 {
			return sessionAdvanceResult{}, errors.New("cannot advance: no active step")
		}
		active := protocol.Steps[progress.ActiveStepIdx]
		if err := finishSessionStep(sessionDir, active.Slug, now); err != nil {
			return sessionAdvanceResult{}, err
		}
		landedIdx, finished, err := startSkippingUnfocusableSteps(sessionDir, protocol, progress.ActiveStepIdx+1, now)
		if err != nil {
			return sessionAdvanceResult{}, err
		}
		if isFocused {
			if err := syncFocusedSessionWindows(root, sessionSlug, protocol, now); err != nil {
				return sessionAdvanceResult{}, err
			}
		}
		if finished {
			lastStep := protocol.Steps[len(protocol.Steps)-1]
			return sessionAdvanceResult{State: "finished", StepSlug: lastStep.Slug}, nil
		}
		return sessionAdvanceResult{State: "advanced", StepSlug: protocol.Steps[landedIdx].Slug}, nil
	case "finish":
		if progress.ActiveStepIdx >= 0 {
			active := protocol.Steps[progress.ActiveStepIdx]
			if err := finishSessionStep(sessionDir, active.Slug, now); err != nil {
				return sessionAdvanceResult{}, err
			}
		}
		if progress.FirstUnstarted >= 0 {
			if _, _, err := startSkippingUnfocusableSteps(sessionDir, protocol, progress.FirstUnstarted, now); err != nil {
				return sessionAdvanceResult{}, err
			}
		}
		lastStep := protocol.Steps[len(protocol.Steps)-1]
		if isFocused {
			if err := syncFocusedSessionWindows(root, sessionSlug, protocol, now); err != nil {
				return sessionAdvanceResult{}, err
			}
		}
		return sessionAdvanceResult{State: "finished", StepSlug: lastStep.Slug}, nil
	default:
		return sessionAdvanceResult{}, fmt.Errorf("session cannot transition from current state")
	}
}

func startSkippingUnfocusableSteps(sessionDir string, protocol store.Protocol, startIdx int, now string) (int, bool, error) {
	for i := startIdx; i < len(protocol.Steps); i++ {
		step := protocol.Steps[i]
		stepPath := filepath.Join(sessionDir, "step", step.Slug, "step.sg.md")
		unfocusable, err := stepIsUnfocusable(stepPath)
		if err != nil {
			return -1, false, err
		}
		if err := startSessionStep(sessionDir, step, now); err != nil {
			return -1, false, err
		}
		if unfocusable {
			if err := finishSessionStep(sessionDir, step.Slug, now); err != nil {
				return -1, false, err
			}
			continue
		}
		return i, false, nil
	}
	return -1, true, nil
}

func reverseSessionOnce(root, sessionSlug string, protocol store.Protocol) (sessionAdvanceResult, error) {
	sessionDir := filepath.Join(root, "session", sessionSlug)
	if info, err := os.Stat(sessionDir); err != nil || !info.IsDir() {
		return sessionAdvanceResult{}, fmt.Errorf("session not found: %s", sessionSlug)
	}
	sessionPath := filepath.Join(sessionDir, "session.sg.md")
	if _, err := os.Stat(sessionPath); err != nil {
		return sessionAdvanceResult{}, fmt.Errorf("session missing file: %s", sessionPath)
	}

	progress, err := inspectSessionProgress(sessionDir, protocol)
	if err != nil {
		return sessionAdvanceResult{}, err
	}
	if progress.ActiveStepIdx < 0 || progress.ActiveStepIdx >= len(protocol.Steps) {
		return sessionAdvanceResult{}, errors.New("cannot reverse: no active step")
	}
	active := protocol.Steps[progress.ActiveStepIdx]
	if err := clearSessionStepStart(sessionDir, active.Slug); err != nil {
		return sessionAdvanceResult{}, err
	}
	now := util.NowTimestamp()
	focusedSlug, err := readStudyActiveSessionSlug(root)
	if err != nil {
		return sessionAdvanceResult{}, err
	}
	if strings.TrimSpace(focusedSlug) == sessionSlug {
		if err := syncFocusedSessionWindows(root, sessionSlug, protocol, now); err != nil {
			return sessionAdvanceResult{}, err
		}
	}
	return sessionAdvanceResult{State: "reversed", StepSlug: active.Slug}, nil
}

func startSessionStep(sessionDir string, step store.ProtocolStep, now string) error {
	stepDir := filepath.Join(sessionDir, "step", step.Slug)
	if err := util.EnsureDir(filepath.Join(stepDir, "asset")); err != nil {
		return err
	}
	stepPath := filepath.Join(stepDir, "step.sg.md")
	fm, body, err := util.ReadFrontmatterFile(stepPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		fm = map[string]any{}
		body = ""
	}
	if strings.TrimSpace(asString(fm["time_started"])) != "" {
		return fmt.Errorf("step already started: %s", step.Slug)
	}
	fm["time_started"] = now
	delete(fm, "time_finished")
	return util.WriteFrontmatterFile(stepPath, fm, body)
}

func finishSessionStep(sessionDir, stepSlug, now string) error {
	stepPath := filepath.Join(sessionDir, "step", stepSlug, "step.sg.md")
	fm, body, err := util.ReadFrontmatterFile(stepPath)
	if err != nil {
		return err
	}
	if strings.TrimSpace(asString(fm["time_started"])) == "" {
		return fmt.Errorf("cannot finish step without time_started: %s", stepSlug)
	}
	fm["time_finished"] = now
	if windows := decodeFocusWindows(fm["focus_windows"]); len(windows) > 0 {
		last := windows[len(windows)-1]
		if strings.TrimSpace(last.TimeFinished) == "" {
			if strings.TrimSpace(last.TimeStarted) != "" {
				if endTS, closeErr := clampFocusWindowEnd(last.TimeStarted, now); closeErr == nil {
					windows[len(windows)-1].TimeFinished = endTS
				} else {
					windows[len(windows)-1].TimeFinished = now
				}
			} else {
				windows[len(windows)-1].TimeFinished = now
			}
			fm["focus_windows"] = encodeFocusWindows(windows)
		}
	}
	return util.WriteFrontmatterFile(stepPath, fm, body)
}

func clearSessionStepStart(sessionDir, stepSlug string) error {
	stepPath := filepath.Join(sessionDir, "step", stepSlug, "step.sg.md")
	fm, body, err := util.ReadFrontmatterFile(stepPath)
	if err != nil {
		return err
	}
	delete(fm, "time_started")
	delete(fm, "time_finished")
	delete(fm, "focus_windows")
	return util.WriteFrontmatterFile(stepPath, fm, body)
}

type focusWindow struct {
	TimeStarted  string `yaml:"time_started"`
	TimeFinished string `yaml:"time_finished,omitempty"`
}

func decodeFocusWindows(v any) []focusWindow {
	rawList, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]focusWindow, 0, len(rawList))
	for _, item := range rawList {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		started := strings.TrimSpace(asString(m["time_started"]))
		finished := strings.TrimSpace(asString(m["time_finished"]))
		if started == "" {
			continue
		}
		out = append(out, focusWindow{TimeStarted: started, TimeFinished: finished})
	}
	return out
}

func encodeFocusWindows(windows []focusWindow) []map[string]any {
	out := make([]map[string]any, 0, len(windows))
	for _, w := range windows {
		row := map[string]any{"time_started": w.TimeStarted}
		if strings.TrimSpace(w.TimeFinished) != "" {
			row["time_finished"] = w.TimeFinished
		}
		out = append(out, row)
	}
	return out
}

func openFocusWindow(stepPath, now string) error {
	fm, body, err := util.ReadFrontmatterFile(stepPath)
	if err != nil {
		return err
	}
	if strings.TrimSpace(asString(fm["time_started"])) == "" {
		return nil
	}
	windows := decodeFocusWindows(fm["focus_windows"])
	if len(windows) > 0 {
		last := windows[len(windows)-1]
		if strings.TrimSpace(last.TimeFinished) == "" {
			return nil
		}
	}
	windows = append(windows, focusWindow{TimeStarted: now})
	fm["focus_windows"] = encodeFocusWindows(windows)
	return util.WriteFrontmatterFile(stepPath, fm, body)
}

func closeOpenFocusWindow(stepPath, now string) error {
	fm, body, err := util.ReadFrontmatterFile(stepPath)
	if err != nil {
		return err
	}
	windows := decodeFocusWindows(fm["focus_windows"])
	if len(windows) == 0 {
		return nil
	}
	last := windows[len(windows)-1]
	if strings.TrimSpace(last.TimeFinished) != "" {
		return nil
	}
	endTS, err := clampFocusWindowEnd(last.TimeStarted, now)
	if err != nil {
		return err
	}
	windows[len(windows)-1].TimeFinished = endTS
	fm["focus_windows"] = encodeFocusWindows(windows)
	return util.WriteFrontmatterFile(stepPath, fm, body)
}

func clampFocusWindowEnd(startTS, endTS string) (string, error) {
	start, err := util.ParseTimestamp(startTS)
	if err != nil {
		return "", fmt.Errorf("invalid focus window time_started: %s", startTS)
	}
	end, err := util.ParseTimestamp(endTS)
	if err != nil {
		return "", fmt.Errorf("invalid focus window time_finished: %s", endTS)
	}
	if end.Before(start) {
		return startTS, nil
	}
	return endTS, nil
}

func syncFocusedSessionWindows(root, sessionSlug string, protocol store.Protocol, now string) error {
	sessionDir := filepath.Join(root, "session", sessionSlug)
	progress, err := inspectSessionProgress(sessionDir, protocol)
	if err != nil {
		return err
	}
	activeSlug := ""
	if progress.ActiveStepIdx >= 0 && progress.ActiveStepIdx < len(protocol.Steps) {
		activeSlug = protocol.Steps[progress.ActiveStepIdx].Slug
	}
	for _, st := range protocol.Steps {
		stepPath := filepath.Join(sessionDir, "step", st.Slug, "step.sg.md")
		if _, err := os.Stat(stepPath); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		if st.Slug == activeSlug {
			if err := openFocusWindow(stepPath, now); err != nil {
				return err
			}
			continue
		}
		if err := closeOpenFocusWindow(stepPath, now); err != nil {
			return err
		}
	}
	return nil
}

func closeFocusedSessionWindows(root, sessionSlug string, protocol store.Protocol, now string) error {
	if strings.TrimSpace(sessionSlug) == "" {
		return nil
	}
	sessionDir := filepath.Join(root, "session", sessionSlug)
	for _, st := range protocol.Steps {
		stepPath := filepath.Join(sessionDir, "step", st.Slug, "step.sg.md")
		if _, err := os.Stat(stepPath); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		if err := closeOpenFocusWindow(stepPath, now); err != nil {
			return err
		}
	}
	return nil
}

func readStudyActiveSessionSlug(root string) (string, error) {
	studyPath := filepath.Join(root, "study.sg.md")
	fm, _, err := util.ReadFrontmatterFile(studyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(asString(fm["active_session_slug"])), nil
}

func createSessionScaffold(root string, selected []store.Subject, protocol store.Protocol) (string, string, error) {
	if len(selected) == 0 {
		return "", "", errors.New("select at least one subject")
	}
	if len(protocol.Steps) == 0 {
		return "", "", errors.New("no protocol steps found")
	}
	today := time.Now().Format("02-01-2006")
	surnames := make([]string, 0, len(selected))
	for _, s := range selected {
		surnames = append(surnames, util.Slugify(lastToken(s.Name)))
	}
	baseSlug := fmt.Sprintf("%s-%s", today, strings.Join(surnames, "-"))
	sessionSlug := uniqueSessionSlug(root, baseSlug)
	sessionDir := filepath.Join(root, "session", sessionSlug)
	if err := util.EnsureDir(filepath.Join(sessionDir, "step")); err != nil {
		return "", "", err
	}
	subjectLines := make([]string, 0, len(selected))
	for _, s := range selected {
		subjectLines = append(subjectLines, fmt.Sprintf("- %s (%s)", s.Name, s.UUID))
	}
	sfm := map[string]any{}
	if err := util.WriteFrontmatterFile(
		filepath.Join(sessionDir, "session.sg.md"),
		sfm,
		"# Subjects\n\n"+strings.Join(subjectLines, "\n")+"\n\n# Notes\n",
	); err != nil {
		return "", "", err
	}
	for _, step := range protocol.Steps {
		stepDir := filepath.Join(sessionDir, "step", step.Slug)
		if err := util.EnsureDir(filepath.Join(stepDir, "asset")); err != nil {
			return "", "", err
		}
		if err := util.WriteFrontmatterFile(filepath.Join(stepDir, "step.sg.md"), map[string]any{}, ""); err != nil {
			return "", "", err
		}
	}
	return sessionSlug, sessionDir, nil
}

func uniqueSessionSlug(root, baseSlug string) string {
	slug := baseSlug
	for i := 2; ; i++ {
		if _, err := os.Stat(filepath.Join(root, "session", slug)); os.IsNotExist(err) {
			return slug
		}
		slug = fmt.Sprintf("%s-%d", baseSlug, i)
	}
}

func selectSubjectsForSession(studyRoot string) ([]store.Subject, error) {
	selected, canceled, err := runSessionCreatePicker(studyRoot)
	if err != nil {
		return nil, err
	}
	if canceled {
		return nil, errors.New("session canceled")
	}
	return selected, nil
}

func cmdStatus(render bool) error {
	root, err := util.StudyRootFromCwd()
	if err != nil {
		return err
	}
	issues, err := collectStatusIssues(root)
	if err != nil {
		return err
	}
	if len(issues) == 0 {
		fmt.Println("status: complete")
		return nil
	}
	md := "# Status\n\n## Issues\n"
	for _, i := range issues {
		md += "- " + i + "\n"
	}
	md += "\nOverall completeness: false\n"
	if render {
		util.PrintMarkdown(md)
	} else {
		fmt.Print(md)
	}
	return nil
}

func collectStatusIssues(root string) ([]string, error) {
	var issues []string
	studyPath := filepath.Join(root, "study.sg.md")
	fm, body, err := util.ReadFrontmatterFile(studyPath)
	if err != nil {
		return nil, err
	}
	for _, key := range []string{"status", "created_on"} {
		if strings.TrimSpace(asString(fm[key])) == "" {
			issues = append(issues, "study.sg.md missing required field: "+key)
		}
	}
	if strings.TrimSpace(store.ExtractStudyTitle(body)) == "" {
		issues = append(issues, "study.sg.md missing title H1")
	}
	for _, sec := range []string{"# Introduction", "# Methods", "# Results", "# Discussion", "# Conclusion"} {
		if !strings.Contains(body, sec) {
			issues = append(issues, "study.sg.md missing section: "+sec)
		}
	}

	protocol, err := store.ParseProtocol(root)
	if err != nil {
		issues = append(issues, "protocol invalid: "+err.Error())
	}

	sessionRoot := filepath.Join(root, "session")
	entries, err := os.ReadDir(sessionRoot)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		slug := e.Name()
		sdir := filepath.Join(sessionRoot, slug)
		spath := filepath.Join(sdir, "session.sg.md")
		_, body, err := util.ReadFrontmatterFile(spath)
		if err != nil {
			issues = append(issues, "invalid session file: "+spath)
			continue
		}
		if len(parseSessionSubjects(body)) == 0 {
			issues = append(issues, "session missing subjects in # Subjects section: "+slug)
		}

		if err == nil {
			for i, step := range protocol.Steps {
				stepPath := filepath.Join(sdir, "step", step.Slug, "step.sg.md")
				stepFM, _, readErr := util.ReadFrontmatterFile(stepPath)
				if readErr != nil {
					issues = append(issues, "session missing step file for protocol step "+step.Slug+": "+slug)
					continue
				}
				if strings.TrimSpace(asString(stepFM["time_started"])) == "" {
					issues = append(issues, "step missing time_started: "+stepPath)
				}
				if i == len(protocol.Steps)-1 && strings.TrimSpace(asString(stepFM["time_finished"])) == "" {
					issues = append(issues, "step missing time_finished: "+stepPath)
				}
			}
		}
	}
	sort.Strings(issues)
	return issues, nil
}

type publishOptions struct {
	WithSubjectNames bool
	DestinationDir   string
	Once             bool
}

type exportOptions struct {
	DestinationDir string
	ThumbnailSizes []int
	Once           bool
}

func cmdExport(args []string) error {
	opts, err := parseExportArgs(args)
	if err != nil {
		return err
	}
	root, err := util.StudyRootFromCwd()
	if err != nil {
		return err
	}
	dest := opts.DestinationDir
	if strings.TrimSpace(dest) == "" {
		dest = filepath.Join(root, "export")
	}
	dest = resolveStudyDestinationDir(root, dest)
	opts.DestinationDir = dest
	if len(opts.ThumbnailSizes) == 0 {
		opts.ThumbnailSizes = []int{144}
	}
	if !opts.Once {
		return watchExportWithEntr(root, opts)
	}
	return cmdExportAtRoot(root, dest, opts)
}

func cmdExportAtRoot(root, dest string, opts exportOptions) error {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	destAbs, err := filepath.Abs(dest)
	if err != nil {
		return err
	}
	if samePath(rootAbs, destAbs) {
		return errors.New("export destination cannot be the study root")
	}
	if containsPath(destAbs, rootAbs) {
		return errors.New("export destination cannot contain the study root")
	}
	cache, err := stageExportRebuildCache(destAbs)
	if err != nil {
		return err
	}
	restoreCache := cache.root != ""
	defer func() {
		if !restoreCache {
			return
		}
		_ = os.RemoveAll(destAbs)
		_ = os.Rename(cache.root, destAbs)
		_ = os.RemoveAll(cache.parentDir)
	}()
	if err := os.RemoveAll(destAbs); err != nil {
		return err
	}
	if err := util.EnsureDir(destAbs); err != nil {
		return err
	}

	subjectLabels, err := exportSubjectLabels(rootAbs)
	if err != nil {
		return err
	}
	subjectMap, err := subjectMapForStudy(rootAbs)
	if err != nil {
		return err
	}
	if err := writePublishSubjectMap(rootAbs, subjectMap, false); err != nil {
		return err
	}
	sessionSlugMap, err := exportSessionSlugMap(rootAbs)
	if err != nil {
		return err
	}
	methodsAppendix, err := buildExportMethodsAppendix(rootAbs)
	if err != nil {
		return err
	}
	resultsAppendix, err := buildExportResultsAppendix(rootAbs, sessionSlugMap, subjectLabels, opts.ThumbnailSizes)
	if err != nil {
		return err
	}

	err = filepath.WalkDir(rootAbs, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if samePath(path, destAbs) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if containsPath(destAbs, path) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		rel, err := filepath.Rel(rootAbs, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if shouldSkipExportRelativePath(rel) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		exportRel := exportRelativePath(rel, sessionSlugMap)
		destPath := filepath.Join(destAbs, exportRel)
		if d.IsDir() {
			return util.EnsureDir(destPath)
		}
		switch {
		case filepath.ToSlash(rel) == "study.sg.md":
			return writeExportStudyFile(path, destPath, sessionSlugMap, methodsAppendix, resultsAppendix)
		case isSessionMarkdownPath(rel):
			return writeExportSessionFile(path, destPath, subjectLabels)
		default:
			if !shouldSkipExportRawAsset(rel) {
				if err := copyFile(path, destPath); err != nil {
					return err
				}
			}
			return writeExportThumbnails(path, rel, destAbs, cache.root, sessionSlugMap, opts.ThumbnailSizes)
		}
	})
	if err != nil {
		return err
	}
	if cache.root != "" {
		if err := os.RemoveAll(cache.parentDir); err != nil {
			return err
		}
		restoreCache = false
	}
	fmt.Printf("exported at %s to %s\n", time.Now().Format("15:04:05 02-01-2006"), destAbs)
	return nil
}

func cmdPublish(args []string) error {
	opts, err := parsePublishArgs(args)
	if err != nil {
		return err
	}
	root, err := util.StudyRootFromCwd()
	if err != nil {
		return err
	}
	dest := opts.DestinationDir
	if strings.TrimSpace(dest) == "" {
		dest = filepath.Join(root, "publish")
	}
	opts.DestinationDir = resolveStudyDestinationDir(root, dest)
	if !opts.Once {
		return watchPublishWithEntr(root, opts)
	}
	return cmdPublishAtRoot(root, opts)
}

func cmdPublishAtRoot(root string, opts publishOptions) error {
	issues, err := collectStatusIssues(root)
	if err != nil {
		return err
	}
	incomplete := len(issues) > 0
	studyPath := filepath.Join(root, "study.sg.md")
	if incomplete {
		if err := setFrontmatterField(studyPath, "status", "WIP"); err != nil {
			return err
		}
	}

	studyFM, studyBody, err := util.ReadFrontmatterFile(studyPath)
	if err != nil {
		return err
	}
	studyDoc := store.ParseStudyDocumentMarkdown(studyBody)
	protocol, err := store.ParseProtocol(root)
	if err != nil {
		return err
	}
	title := studyDoc.Title
	if title == "" {
		title = "Untitled Study"
	}

	subjects, err := store.ListSubjects()
	if err != nil {
		return err
	}
	subjectByID := map[string]store.Subject{}
	for _, s := range subjects {
		subjectByID[s.UUID] = s
	}

	sessions, subjectMap, err := loadSessionsForPublish(root, protocol, subjectByID, opts)
	if err != nil {
		return err
	}

	outDir := opts.DestinationDir
	siteDir := filepath.Join(outDir, "site")
	if err := util.EnsureDir(siteDir); err != nil {
		return err
	}
	if err := util.EnsureDir(filepath.Join(siteDir, "assets")); err != nil {
		return err
	}

	heroComparison, err := resolvePublishHeroComparison(studyFM, sessions)
	if err != nil {
		return err
	}
	if err := writePublishHeroAssets(siteDir, heroComparison); err != nil {
		return err
	}
	if err := writePublishSessionPages(siteDir, title, sessions, incomplete); err != nil {
		return err
	}
	if err := writePublishIndexThumbnails(siteDir, sessions); err != nil {
		return err
	}
	html, err := renderPublishHTML(siteDir, studyDoc, studyFM, protocol, sessions, heroComparison, incomplete)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(siteDir, "index.html"), []byte(html), 0o644); err != nil {
		return err
	}
	if err := writePublishSubjectMap(root, subjectMap, opts.WithSubjectNames); err != nil {
		return err
	}

	pdfBody := renderPublishText(studyDoc, studyFM, protocol, sessions)
	if err := writePDF(filepath.Join(outDir, "study.pdf"), title, pdfBody, incomplete); err != nil {
		return err
	}
	fmt.Printf("published to %s\n", outDir)
	return nil
}

func writePublishSessionPages(siteDir, title string, sessions []publishSession, incomplete bool) error {
	tasks := make([]func() error, 0)
	for _, s := range sessions {
		sessionDir := filepath.Join(siteDir, "session", s.PublishSlug)
		if err := util.EnsureDir(sessionDir); err != nil {
			return err
		}
		tasks = append(tasks, publishSessionAssetTasks(sessionDir, s)...)
	}
	if err := runPublishTasks(tasks); err != nil {
		return err
	}
	for i, s := range sessions {
		sessionDir := filepath.Join(siteDir, "session", s.PublishSlug)
		var prevSession *publishSession
		var nextSession *publishSession
		if i > 0 {
			prevSession = &sessions[i-1]
		}
		if i+1 < len(sessions) {
			nextSession = &sessions[i+1]
		}
		html, err := renderPublishSessionHTML(sessionDir, title, s, prevSession, nextSession, incomplete)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(sessionDir, "index.html"), []byte(html), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func writePublishIndexThumbnails(siteDir string, sessions []publishSession) error {
	return runPublishTasks(publishIndexThumbnailTasks(siteDir, sessions))
}

func writePublishHeroAssets(siteDir string, hero *publishHeroComparison) error {
	if hero == nil {
		return nil
	}
	for _, img := range []publishHeroImage{hero.Left, hero.Right} {
		if strings.TrimSpace(img.Src) == "" || strings.TrimSpace(img.URL) == "" {
			continue
		}
		dest := filepath.Join(siteDir, filepath.FromSlash(img.URL))
		if err := util.EnsureDir(filepath.Dir(dest)); err != nil {
			return err
		}
		if err := publishAssetForHTML(img.Src, dest); err != nil {
			return err
		}
	}
	return nil
}

func publishSessionAssetTasks(sessionDir string, s publishSession) []func() error {
	tasks := make([]func() error, 0)
	for _, step := range s.Steps {
		step := step
		for _, img := range step.ImageRefs {
			img := img
			tasks = append(tasks, func() error {
				_, dest, err := publishAssetOutputPaths(sessionDir, step.Slug, img)
				if err != nil {
					return err
				}
				if err := util.EnsureDir(filepath.Dir(dest)); err != nil {
					return err
				}
				return publishAssetForHTML(img, dest)
			})
		}
	}
	return tasks
}

func publishIndexThumbnailTasks(siteDir string, sessions []publishSession) []func() error {
	tasks := make([]func() error, 0)
	for _, session := range sessions {
		session := session
		for _, step := range session.Steps {
			step := step
			for _, img := range step.ImageRefs {
				img := img
				tasks = append(tasks, func() error {
					sessionAssetPath := filepath.Join(siteDir, "session", session.PublishSlug, filepath.FromSlash(publishAssetRelativeURL(step.Slug, img)))
					_, thumbDest, err := publishIndexThumbnailOutputPaths(siteDir, session.PublishSlug, step.Slug, img)
					if err != nil {
						return err
					}
					if err := util.EnsureDir(filepath.Dir(thumbDest)); err != nil {
						return err
					}
					return publishImageThumbnailFn(sessionAssetPath, thumbDest)
				})
			}
		}
	}
	return tasks
}

func runPublishTasks(tasks []func() error) error {
	if len(tasks) == 0 {
		return nil
	}
	workerCount := runtime.GOMAXPROCS(0)
	if workerCount < 1 {
		workerCount = 1
	}
	if workerCount > len(tasks) {
		workerCount = len(tasks)
	}

	jobs := make(chan func() error)
	done := make(chan struct{})
	errCh := make(chan error, 1)
	var once sync.Once
	var wg sync.WaitGroup

	worker := func() {
		defer wg.Done()
		for {
			select {
			case <-done:
				return
			case task, ok := <-jobs:
				if !ok {
					return
				}
				if err := task(); err != nil {
					once.Do(func() {
						errCh <- err
						close(done)
					})
				}
			}
		}
	}

	wg.Add(workerCount)
	for i := 0; i < workerCount; i++ {
		go worker()
	}

enqueue:
	for _, task := range tasks {
		select {
		case <-done:
			break enqueue
		case jobs <- task:
		}
	}
	close(jobs)
	wg.Wait()

	select {
	case err := <-errCh:
		return err
	default:
		return nil
	}
}

var publishImagePreviewFn = publishImagePreview
var publishImageThumbnailFn = publishImageThumbnail
var exportImageThumbnailFn = exportImageThumbnail

type publishStep struct {
	Name            string
	Slug            string
	Started         string
	Finished        string
	SourceImageRefs []string
	ImageRefs       []string
}

type publishSession struct {
	Slug        string
	PublishSlug string
	Started     string
	Finished    string
	SubjectIDs  []string
	Subjects    []string
	Steps       []publishStep
}

type publishHeroAssetRef struct {
	SessionSlug string
	StepSlug    string
	AssetIndex  int
}

type publishHeroImage struct {
	Ref publishHeroAssetRef
	Src string
	URL string
}

type publishHeroComparison struct {
	Left  publishHeroImage
	Right publishHeroImage
}

type publishSubjectRef struct {
	key  string
	name string
}

func loadSessionsForPublish(root string, protocol store.Protocol, subjectByID map[string]store.Subject, opts publishOptions) ([]publishSession, map[string]string, error) {
	slugList, err := listSessionSlugs(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	sessions := make([]publishSession, 0, len(slugList))
	sessionSubjectRefs := map[string][]publishSubjectRef{}
	subjectKeys := map[string]struct{}{}
	for _, slug := range slugList {
		sdir := filepath.Join(root, "session", slug)
		_, body, err := util.ReadFrontmatterFile(filepath.Join(sdir, "session.sg.md"))
		if err != nil {
			continue
		}
		refs := parseSessionSubjects(body)
		ids := make([]string, 0, len(refs))
		names := make([]string, 0, len(refs))
		resolvedRefs := make([]publishSubjectRef, 0, len(refs))
		for _, ref := range refs {
			if strings.TrimSpace(ref.ID) != "" {
				ids = append(ids, ref.ID)
			}
			name := ref.Name
			if s, ok := subjectByID[ref.ID]; ok && strings.TrimSpace(s.Name) != "" {
				name = s.Name
			}
			if strings.TrimSpace(name) != "" {
				names = append(names, name)
			}
			key := publishSubjectKey(ref, name)
			if key != "" {
				subjectKeys[key] = struct{}{}
				resolvedRefs = append(resolvedRefs, publishSubjectRef{key: key, name: name})
			}
		}
		sessionSubjectRefs[slug] = resolvedRefs
		steps := make([]publishStep, 0, len(protocol.Steps))
		for _, st := range protocol.Steps {
			stepPath := filepath.Join(sdir, "step", st.Slug, "step.sg.md")
			stepFM, _, err := util.ReadFrontmatterFile(stepPath)
			if err != nil {
				continue
			}
			if asBool(stepFM["unfocusable"]) {
				continue
			}
			imgRefs := []string{}
			assetDir := filepath.Join(sdir, "step", st.Slug, "asset")
			_ = filepath.WalkDir(assetDir, func(path string, d fs.DirEntry, walkErr error) error {
				if walkErr != nil || d.IsDir() {
					return nil
				}
				imgRefs = append(imgRefs, path)
				return nil
			})
			sort.Strings(imgRefs)
			sourceRefs := append([]string(nil), imgRefs...)
			imgRefs, err = resolvePublishStepImageRefs(imgRefs, stepFM)
			if err != nil {
				return nil, nil, err
			}
			steps = append(steps, publishStep{
				Name:            st.Name,
				Slug:            st.Slug,
				Started:         asString(stepFM["time_started"]),
				Finished:        asString(stepFM["time_finished"]),
				SourceImageRefs: sourceRefs,
				ImageRefs:       imgRefs,
			})
		}
		sessions = append(sessions, publishSession{
			Slug:        slug,
			PublishSlug: slug,
			Started:     earliestPublishStepStart(steps),
			Finished:    latestPublishStepFinish(steps),
			SubjectIDs:  ids,
			Subjects:    names,
			Steps:       steps,
		})
	}
	subjectMap := map[string]string{}
	if !opts.WithSubjectNames {
		labelByKey := anonymizedPublishLabels(subjectKeys)
		for _, refs := range sessionSubjectRefs {
			for _, ref := range refs {
				label := strings.TrimSpace(labelByKey[ref.key])
				name := strings.TrimSpace(ref.name)
				if label == "" || name == "" {
					continue
				}
				subjectMap[label] = name
			}
		}
		for i := range sessions {
			refs := sessionSubjectRefs[sessions[i].Slug]
			labels := make([]string, 0, len(refs))
			for _, ref := range refs {
				label := strings.TrimSpace(labelByKey[ref.key])
				if label == "" {
					label = ref.name
				}
				if strings.TrimSpace(label) != "" {
					labels = append(labels, label)
				}
			}
			sessions[i].Subjects = labels
		}
	}
	sort.Slice(sessions, func(i, j int) bool {
		ti, ei := util.ParseTimestamp(sessions[i].Started)
		tj, ej := util.ParseTimestamp(sessions[j].Started)
		if ei == nil && ej == nil {
			return ti.Before(tj)
		}
		return sessions[i].Slug < sessions[j].Slug
	})
	if !opts.WithSubjectNames {
		for i := range sessions {
			sessions[i].PublishSlug = fmt.Sprintf("session-%d", i+1)
		}
	}
	return sessions, subjectMap, nil
}

func writePublishSubjectMap(studyRoot string, subjectMap map[string]string, withSubjectNames bool) error {
	path := filepath.Join(studyRoot, "subject-map.txt")
	if withSubjectNames || len(subjectMap) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	labels := make([]string, 0, len(subjectMap))
	for label := range subjectMap {
		if strings.TrimSpace(label) != "" {
			labels = append(labels, label)
		}
	}
	sort.Slice(labels, func(i, j int) bool {
		return publishSubjectLabelOrder(labels[i]) < publishSubjectLabelOrder(labels[j])
	})
	lines := make([]string, 0, len(labels))
	for _, label := range labels {
		name := strings.TrimSpace(subjectMap[label])
		if name == "" {
			continue
		}
		lines = append(lines, label+": "+name)
	}
	if len(lines) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}

func publishSubjectLabelOrder(label string) int {
	n, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(label, "Subject ")))
	if err != nil || n < 1 {
		return math.MaxInt
	}
	return n
}

func parsePublishArgs(args []string) (publishOptions, error) {
	opts := publishOptions{}
	for _, raw := range args {
		arg := strings.TrimSpace(raw)
		if arg == "" {
			continue
		}
		switch arg {
		case "--once":
			opts.Once = true
		case "--with-subject-names":
			opts.WithSubjectNames = true
		default:
			if strings.HasPrefix(arg, "-") {
				return opts, fmt.Errorf("unknown flag: %s", arg)
			}
			if strings.TrimSpace(opts.DestinationDir) != "" {
				return opts, errors.New("usage: sg publish [--once] [--with-subject-names] [<destination-dir>]")
			}
			opts.DestinationDir = arg
		}
	}
	return opts, nil
}

func parseExportArgs(args []string) (exportOptions, error) {
	opts := exportOptions{}
	for i := 0; i < len(args); i++ {
		arg := strings.TrimSpace(args[i])
		if arg == "" {
			return opts, errors.New("usage: sg export [--once] [--imgsize=x[,y,...]] [<destination-dir>]")
		}
		if arg == "--once" {
			opts.Once = true
			continue
		}
		if strings.HasPrefix(arg, "--imgsize=") {
			rawSizes := strings.TrimSpace(strings.TrimPrefix(arg, "--imgsize="))
			if rawSizes == "" {
				return opts, errors.New("usage: sg export [--once] [--imgsize=x[,y,...]] [<destination-dir>]")
			}
			for _, part := range strings.Split(rawSizes, ",") {
				size, err := strconv.Atoi(strings.TrimSpace(part))
				if err != nil || size <= 0 {
					return opts, fmt.Errorf("invalid imgsize: %s", part)
				}
				opts.ThumbnailSizes = append(opts.ThumbnailSizes, size)
			}
			continue
		}
		if strings.HasPrefix(arg, "-") {
			return opts, fmt.Errorf("unknown flag: %s", arg)
		}
		if strings.TrimSpace(opts.DestinationDir) != "" {
			return opts, errors.New("usage: sg export [--once] [--imgsize=x[,y,...]] [<destination-dir>]")
		}
		opts.DestinationDir = arg
	}
	return opts, nil
}

func cmdPublishOnceAtRootArgs(args []string) error {
	if len(args) < 2 {
		return errors.New("internal usage: __publish-once-at-root <study-root> <destination-dir> [--with-subject-names]")
	}
	opts := publishOptions{DestinationDir: args[1]}
	for _, arg := range args[2:] {
		switch strings.TrimSpace(arg) {
		case "":
			continue
		case "--with-subject-names":
			opts.WithSubjectNames = true
		default:
			return fmt.Errorf("unknown internal flag: %s", arg)
		}
	}
	return cmdPublishAtRoot(args[0], opts)
}

func cmdExportOnceAtRootArgs(args []string) error {
	if len(args) < 3 {
		return errors.New("internal usage: __export-once-at-root <study-root> <destination-dir> <imgsize[,imgsize...]>")
	}
	sizes, err := parseThumbnailSizeList(args[2])
	if err != nil {
		return err
	}
	return cmdExportAtRoot(args[0], args[1], exportOptions{
		DestinationDir: args[1],
		ThumbnailSizes: sizes,
		Once:           true,
	})
}

func watchPublishWithEntr(root string, opts publishOptions) error {
	paths, err := collectStudyWatchPaths(root, []string{opts.DestinationDir})
	if err != nil {
		return err
	}
	utilityArgs := []string{"__publish-once-at-root", root, opts.DestinationDir}
	if opts.WithSubjectNames {
		utilityArgs = append(utilityArgs, "--with-subject-names")
	}
	return runEntrWatchFn(paths, utilityArgs)
}

func watchExportWithEntr(root string, opts exportOptions) error {
	paths, err := collectStudyWatchPaths(root, []string{opts.DestinationDir})
	if err != nil {
		return err
	}
	utilityArgs := []string{"__export-once-at-root", root, opts.DestinationDir, formatThumbnailSizeList(opts.ThumbnailSizes)}
	return runEntrWatchFn(paths, utilityArgs)
}

func runEntrWatch(paths []string, utilityArgs []string) error {
	if len(paths) == 0 {
		return errors.New("no study files to watch")
	}
	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	cmdArgs := append([]string{"-d", exePath}, utilityArgs...)
	cmd := exec.Command("entr", cmdArgs...)
	cmd.Stdin = strings.NewReader(strings.Join(paths, "\n") + "\n")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func collectStudyWatchPaths(root string, extraExcludes []string) ([]string, error) {
	root = filepath.Clean(root)
	excluded := []string{
		filepath.Join(root, "publish"),
		filepath.Join(root, "export"),
	}
	for _, path := range extraExcludes {
		trimmed := strings.TrimSpace(path)
		if trimmed == "" {
			continue
		}
		if !filepath.IsAbs(trimmed) {
			absPath, err := filepath.Abs(trimmed)
			if err != nil {
				return nil, err
			}
			trimmed = absPath
		}
		excluded = append(excluded, filepath.Clean(trimmed))
	}
	seen := map[string]struct{}{}
	var paths []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if shouldSkipWatchPath(root, path, d, excluded) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		clean := filepath.Clean(path)
		if _, ok := seen[clean]; ok {
			return nil
		}
		seen[clean] = struct{}{}
		paths = append(paths, clean)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	return paths, nil
}

func shouldSkipWatchPath(root, candidate string, d fs.DirEntry, excluded []string) bool {
	for _, path := range excluded {
		if samePath(candidate, path) || containsPath(path, candidate) {
			return true
		}
	}
	rel, err := filepath.Rel(root, candidate)
	if err != nil || rel == "." {
		return false
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	for _, part := range parts {
		if part == "" {
			continue
		}
		if strings.HasPrefix(part, ".") {
			return true
		}
		if d.IsDir() && (part == "publish" || part == "export") {
			return true
		}
	}
	return false
}

func formatThumbnailSizeList(sizes []int) string {
	parts := make([]string, 0, len(sizes))
	for _, size := range sizes {
		parts = append(parts, strconv.Itoa(size))
	}
	return strings.Join(parts, ",")
}

func parseThumbnailSizeList(raw string) ([]int, error) {
	parts := strings.Split(strings.TrimSpace(raw), ",")
	sizes := make([]int, 0, len(parts))
	for _, part := range parts {
		size, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || size <= 0 {
			return nil, fmt.Errorf("invalid imgsize: %s", part)
		}
		sizes = append(sizes, size)
	}
	if len(sizes) == 0 {
		return nil, errors.New("missing imgsize")
	}
	return sizes, nil
}

func publishSubjectKey(ref sessionSubjectRef, resolvedName string) string {
	if id := strings.TrimSpace(ref.ID); id != "" {
		return "id:" + id
	}
	name := strings.TrimSpace(resolvedName)
	if name == "" {
		name = strings.TrimSpace(ref.Name)
	}
	if name == "" {
		return ""
	}
	return "name:" + strings.ToLower(name)
}

func anonymizedPublishLabels(keys map[string]struct{}) map[string]string {
	ordered := make([]string, 0, len(keys))
	for key := range keys {
		if strings.TrimSpace(key) != "" {
			ordered = append(ordered, key)
		}
	}
	sort.Strings(ordered)
	labels := make(map[string]string, len(ordered))
	for i, key := range ordered {
		labels[key] = fmt.Sprintf("Subject %d", i+1)
	}
	return labels
}

func earliestPublishStepStart(steps []publishStep) string {
	earliest := ""
	for _, step := range steps {
		if strings.TrimSpace(step.Started) == "" {
			continue
		}
		if earliest == "" {
			earliest = step.Started
			continue
		}
		stepTS, stepErr := util.ParseTimestamp(step.Started)
		curTS, curErr := util.ParseTimestamp(earliest)
		if stepErr == nil && curErr == nil && stepTS.Before(curTS) {
			earliest = step.Started
		}
	}
	return earliest
}

func latestPublishStepFinish(steps []publishStep) string {
	latest := ""
	for _, step := range steps {
		if strings.TrimSpace(step.Finished) == "" {
			continue
		}
		if latest == "" {
			latest = step.Finished
			continue
		}
		stepTS, stepErr := util.ParseTimestamp(step.Finished)
		curTS, curErr := util.ParseTimestamp(latest)
		if stepErr == nil && curErr == nil && stepTS.After(curTS) {
			latest = step.Finished
		}
	}
	return latest
}

func parseSubjectIDs(v any) []string {
	switch ids := v.(type) {
	case []any:
		out := make([]string, 0, len(ids))
		for _, id := range ids {
			if s, ok := id.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, s)
			}
		}
		return out
	case []string:
		out := make([]string, 0, len(ids))
		for _, id := range ids {
			if strings.TrimSpace(id) != "" {
				out = append(out, id)
			}
		}
		return out
	default:
		return nil
	}
}

type sessionSubjectRef struct {
	Name string
	ID   string
}

func parseSessionSubjects(sessionBody string) []sessionSubjectRef {
	section := extractSection(sessionBody, "Subjects")
	if strings.TrimSpace(section) == "" {
		return nil
	}
	lines := strings.Split(section, "\n")
	out := make([]sessionSubjectRef, 0, len(lines))
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "- ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "- "))
		}
		if strings.HasPrefix(line, "* ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "* "))
		}
		name := line
		id := ""
		open := strings.LastIndex(line, "(")
		close := strings.LastIndex(line, ")")
		if open >= 0 && close > open {
			parsedID := strings.TrimSpace(line[open+1 : close])
			parsedName := strings.TrimSpace(line[:open])
			if parsedID != "" {
				id = parsedID
			}
			if parsedName != "" {
				name = parsedName
			}
		}
		if strings.TrimSpace(name) == "" && strings.TrimSpace(id) == "" {
			continue
		}
		if strings.TrimSpace(name) == "" {
			name = id
		}
		out = append(out, sessionSubjectRef{Name: name, ID: id})
	}
	return out
}

func exportSubjectLabels(root string) (map[string]string, error) {
	labelByKey, _, err := subjectMappingsForStudy(root)
	if err != nil {
		return nil, err
	}
	return labelByKey, nil
}

func subjectMapForStudy(root string) (map[string]string, error) {
	_, subjectMap, err := subjectMappingsForStudy(root)
	if err != nil {
		return nil, err
	}
	return subjectMap, nil
}

func subjectMappingsForStudy(root string) (map[string]string, map[string]string, error) {
	subjects, err := store.ListSubjects()
	if err != nil {
		return nil, nil, err
	}
	subjectByID := map[string]store.Subject{}
	for _, subject := range subjects {
		subjectByID[subject.UUID] = subject
	}
	slugs, err := listSessionSlugs(root)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, map[string]string{}, nil
		}
		return nil, nil, err
	}
	keys := map[string]struct{}{}
	nameByKey := map[string]string{}
	for _, slug := range slugs {
		_, body, err := util.ReadFrontmatterFile(filepath.Join(root, "session", slug, "session.sg.md"))
		if err != nil {
			return nil, nil, err
		}
		for _, ref := range parseSessionSubjects(body) {
			name := ref.Name
			if subject, ok := subjectByID[ref.ID]; ok && strings.TrimSpace(subject.Name) != "" {
				name = subject.Name
			}
			key := publishSubjectKey(ref, name)
			if strings.TrimSpace(key) != "" {
				keys[key] = struct{}{}
				if strings.TrimSpace(name) != "" {
					nameByKey[key] = name
				}
			}
		}
	}
	labelByKey := anonymizedPublishLabels(keys)
	subjectMap := map[string]string{}
	for key, label := range labelByKey {
		name := strings.TrimSpace(nameByKey[key])
		if strings.TrimSpace(label) == "" || name == "" {
			continue
		}
		subjectMap[label] = name
	}
	return labelByKey, subjectMap, nil
}

func exportSessionSlugMap(root string) (map[string]string, error) {
	slugs, err := listSessionSlugs(root)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, err
	}
	out := make(map[string]string, len(slugs))
	for i, slug := range slugs {
		out[slug] = fmt.Sprintf("session-%d", i+1)
	}
	return out, nil
}

func shouldSkipExportRelativePath(rel string) bool {
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) == 0 {
		return false
	}
	for _, part := range parts {
		if part == "" {
			continue
		}
		if strings.HasPrefix(part, ".") {
			return true
		}
	}
	switch parts[0] {
	case "publish", "export", "bin":
		return true
	}
	return false
}

func exportRelativePath(rel string, sessionSlugMap map[string]string) string {
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) >= 2 && parts[0] == "session" {
		if mapped := strings.TrimSpace(sessionSlugMap[parts[1]]); mapped != "" {
			parts[1] = mapped
		}
	}
	return filepath.FromSlash(path.Join(parts...))
}

func isSessionMarkdownPath(rel string) bool {
	parts := strings.Split(filepath.ToSlash(rel), "/")
	return len(parts) == 3 && parts[0] == "session" && parts[2] == "session.sg.md"
}

func isSessionAssetPath(rel string) (string, string, string, bool) {
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) != 6 || parts[0] != "session" || parts[2] != "step" || parts[4] != "asset" {
		return "", "", "", false
	}
	return parts[1], parts[3], parts[5], true
}

type exportRebuildCache struct {
	parentDir string
	root      string
}

const exportRebuildCacheTempRoot = "/tmp"

func stageExportRebuildCache(destAbs string) (exportRebuildCache, error) {
	info, err := os.Stat(destAbs)
	if err != nil {
		if os.IsNotExist(err) {
			return exportRebuildCache{}, nil
		}
		return exportRebuildCache{}, err
	}
	if !info.IsDir() {
		if err := os.RemoveAll(destAbs); err != nil {
			return exportRebuildCache{}, err
		}
		return exportRebuildCache{}, nil
	}
	parentDir, err := os.MkdirTemp(exportRebuildCacheTempRoot, "."+filepath.Base(destAbs)+".cache-")
	if err != nil {
		return exportRebuildCache{}, err
	}
	cacheRoot := filepath.Join(parentDir, "previous-export")
	if err := os.Rename(destAbs, cacheRoot); err != nil {
		_ = os.RemoveAll(parentDir)
		return exportRebuildCache{}, err
	}
	return exportRebuildCache{parentDir: parentDir, root: cacheRoot}, nil
}

func writeExportThumbnails(src, rel, destRoot, cacheRoot string, sessionSlugMap map[string]string, sizes []int) error {
	sessionSlug, stepSlug, _, ok := isSessionAssetPath(rel)
	if !ok {
		return nil
	}
	exportSessionSlug := strings.TrimSpace(sessionSlugMap[sessionSlug])
	if exportSessionSlug == "" {
		exportSessionSlug = sessionSlug
	}
	for _, size := range sizes {
		imgRel := exportThumbnailRelativePath(size, exportSessionSlug, stepSlug, src)
		dest := filepath.Join(destRoot, imgRel)
		if err := util.EnsureDir(filepath.Dir(dest)); err != nil {
			return err
		}
		if cacheRoot != "" {
			cachePath := filepath.Join(cacheRoot, imgRel)
			upToDate, err := publishAssetUpToDate(src, cachePath)
			if err != nil && !os.IsNotExist(err) {
				return err
			}
			if err == nil && upToDate {
				if err := copyFile(cachePath, dest); err != nil {
					return err
				}
				continue
			}
		}
		if err := exportImageThumbnailFn(src, dest, size); err != nil {
			return err
		}
	}
	return nil
}

func exportThumbnailRelativePath(size int, sessionSlug, stepSlug, src string) string {
	name := filepath.Base(src)
	name = strings.TrimSuffix(name, filepath.Ext(name)) + ".jpg"
	return path.Join("session", sessionSlug, "step", stepSlug, "asset", "img", strconv.Itoa(size), name)
}

func buildExportResultsAppendix(root string, sessionSlugMap map[string]string, subjectLabels map[string]string, sizes []int) (string, error) {
	slugs, err := listSessionSlugs(root)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	var b strings.Builder
	for _, slug := range slugs {
		exportSlug := strings.TrimSpace(sessionSlugMap[slug])
		if exportSlug == "" {
			exportSlug = slug
		}
		_, sessionBody, err := util.ReadFrontmatterFile(filepath.Join(root, "session", slug, "session.sg.md"))
		if err != nil {
			return "", err
		}
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString("## Session ")
		b.WriteString(exportSlug)
		b.WriteString("\n\n")
		labels := make([]string, 0)
		for _, ref := range parseSessionSubjects(sessionBody) {
			label := strings.TrimSpace(subjectLabels[publishSubjectKey(ref, ref.Name)])
			if label == "" {
				label = "Subject"
			}
			labels = append(labels, label)
		}
		if len(labels) > 0 {
			b.WriteString("Subjects: ")
			b.WriteString(strings.Join(labels, ", "))
			b.WriteString("\n\n")
		}
		stepRoot := filepath.Join(root, "session", slug, "step")
		stepEntries, err := os.ReadDir(stepRoot)
		if err != nil && !os.IsNotExist(err) {
			return "", err
		}
		for _, entry := range stepEntries {
			if !entry.IsDir() {
				continue
			}
			stepSlug := entry.Name()
			stepPath := filepath.Join(stepRoot, stepSlug, "step.sg.md")
			stepFM, _, err := util.ReadFrontmatterFile(stepPath)
			if err != nil {
				continue
			}
			b.WriteString("### Step ")
			b.WriteString(stepSlug)
			b.WriteString("\n\n")
			if started := strings.TrimSpace(asString(stepFM["time_started"])); started != "" {
				b.WriteString("Started: ")
				b.WriteString(started)
				b.WriteString("\n")
			}
			if finished := strings.TrimSpace(asString(stepFM["time_finished"])); finished != "" {
				b.WriteString("Finished: ")
				b.WriteString(finished)
				b.WriteString("\n")
			}
			assetDir := filepath.Join(stepRoot, stepSlug, "asset")
			assetFiles, err := collectImageFiles(assetDir)
			if err != nil && !os.IsNotExist(err) {
				return "", err
			}
			if len(assetFiles) > 0 {
				b.WriteString("\nAssets:\n")
				for _, asset := range assetFiles {
					relAsset := exportResultsAssetRelativePath(exportSlug, stepSlug, asset, sizes)
					b.WriteString("- ")
					b.WriteString(relAsset)
					b.WriteString("\n")
				}
			}
			b.WriteString("\n")
		}
	}
	return strings.TrimSpace(b.String()), nil
}

func shouldSkipExportRawAsset(rel string) bool {
	_, _, assetName, ok := isSessionAssetPath(rel)
	if !ok {
		return false
	}
	ext := strings.ToLower(filepath.Ext(assetName))
	return ext == ".heic" || ext == ".heif"
}

func exportResultsAssetRelativePath(sessionSlug, stepSlug, src string, sizes []int) string {
	ext := strings.ToLower(filepath.Ext(src))
	if (ext == ".heic" || ext == ".heif") && len(sizes) > 0 {
		return exportThumbnailRelativePath(sizes[0], sessionSlug, stepSlug, src)
	}
	return path.Join("session", sessionSlug, "step", stepSlug, "asset", filepath.Base(src))
}

func buildExportMethodsAppendix(root string) (string, error) {
	_, err := store.ParseProtocol(root)
	if err != nil {
		return "", err
	}
	return "", nil
}

func writeExportStudyFile(src, dst string, sessionSlugMap map[string]string, methodsAppendix, resultsAppendix string) error {
	fm, body, err := util.ReadFrontmatterFile(src)
	if err != nil {
		return err
	}
	if slug := strings.TrimSpace(asString(fm["active_session_slug"])); slug != "" {
		if mapped := strings.TrimSpace(sessionSlugMap[slug]); mapped != "" {
			fm["active_session_slug"] = mapped
		}
	}
	rewriteExportHeroComparisonSessionRefs(fm, sessionSlugMap)
	body = replaceSection(body, "Methods", appendSectionContent(extractSection(body, "Methods"), methodsAppendix))
	body = replaceSection(body, "Results", appendSectionContent(extractSection(body, "Results"), resultsAppendix))
	return util.WriteFrontmatterFile(dst, fm, body)
}

func rewriteExportHeroComparisonSessionRefs(fm map[string]any, sessionSlugMap map[string]string) {
	hero, ok := asMap(fm["hero_comparison"])
	if !ok {
		return
	}
	for _, side := range []string{"left", "right"} {
		rawRef, ok := asMap(hero[side])
		if !ok {
			continue
		}
		slug := strings.TrimSpace(asString(rawRef["session"]))
		if slug == "" {
			continue
		}
		if mapped := strings.TrimSpace(sessionSlugMap[slug]); mapped != "" {
			rawRef["session"] = mapped
		}
	}
}

func writeExportSessionFile(src, dst string, subjectLabels map[string]string) error {
	fm, body, err := util.ReadFrontmatterFile(src)
	if err != nil {
		return err
	}
	rewritten := rewriteExportSessionSubjects(body, subjectLabels)
	return util.WriteFrontmatterFile(dst, fm, rewritten)
}

func rewriteExportSessionSubjects(body string, subjectLabels map[string]string) string {
	refs := parseSessionSubjects(body)
	if len(refs) == 0 {
		return body
	}
	lines := make([]string, 0, len(refs))
	for _, ref := range refs {
		label := strings.TrimSpace(subjectLabels[publishSubjectKey(ref, ref.Name)])
		if label == "" {
			label = "Subject"
		}
		lines = append(lines, label)
	}
	return replaceSection(body, "Subjects", strings.Join(lines, "\n"))
}

func replaceSection(md, name, content string) string {
	lines := strings.Split(md, "\n")
	head := "# " + name
	var out []string
	replaced := false
	for i := 0; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == head {
			replaced = true
			out = append(out, lines[i], "")
			trimmedContent := strings.TrimSpace(content)
			if trimmedContent != "" {
				out = append(out, strings.Split(trimmedContent, "\n")...)
			}
			for i = i + 1; i < len(lines); i++ {
				next := strings.TrimSpace(lines[i])
				if strings.HasPrefix(next, "# ") {
					i--
					break
				}
			}
			continue
		}
		out = append(out, lines[i])
	}
	if !replaced {
		return md
	}
	return strings.TrimRight(strings.Join(out, "\n"), "\n") + "\n"
}

func appendSectionContent(existing, appended string) string {
	existing = strings.TrimSpace(existing)
	appended = strings.TrimSpace(appended)
	switch {
	case existing == "":
		return appended
	case appended == "":
		return existing
	default:
		return existing + "\n\n" + appended
	}
}

func samePath(a, b string) bool {
	return filepath.Clean(a) == filepath.Clean(b)
}

func containsPath(root, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

func resolveStudyDestinationDir(root, baseDest string) string {
	return filepath.Join(baseDest, filepath.Base(filepath.Clean(root)))
}

func renderPublishHTML(siteDir string, studyDoc store.StudyDocument, studyFM map[string]any, protocol store.Protocol, sessions []publishSession, heroComparison *publishHeroComparison, incomplete bool) (string, error) {
	title := studyDoc.Title
	studyMeta := fmt.Sprintf("<p>Status: %s<br/>Created: %s</p>", escapeHTML(asString(studyFM["status"])), escapeHTML(asString(studyFM["created_on"])))
	wipBadge := ""
	if incomplete {
		wipBadge = ` <span style="display:inline-block;padding:2px 8px;border:1px solid #b94a48;border-radius:999px;font-size:12px;vertical-align:middle;color:#7a1f1c;background:#ffe9e9;">WIP</span>`
	}
	heroHTML := ""
	if heroComparison != nil {
		heroHTML = `<section class="hero-comparison" style="display:flex;gap:12px;align-items:flex-start;margin:16px 0;">` +
			`<img class="hero-comparison-image" src="` + escapeHTML(heroComparison.Left.URL) + `" alt="Comparison left" style="display:block;width:min(40vw, 360px);height:auto;border:1px solid #d2c5b3;background:#efe6da;" />` +
			`<img class="hero-comparison-image" src="` + escapeHTML(heroComparison.Right.URL) + `" alt="Comparison right" style="display:block;width:min(40vw, 360px);height:auto;border:1px solid #d2c5b3;background:#efe6da;" />` +
			`</section>`
	}
	protocolSteps := ""
	for _, step := range protocol.Steps {
		protocolSteps += "<li>" + escapeHTML(step.Name) + "</li>"
	}
	if protocolSteps == "" {
		protocolSteps = "<li>No protocol steps found</li>"
	}
	protocolHeading := ""
	if protocolSteps != "" {
		protocolHeading = "<h3>Protocol</h3>"
	}

	sessionHTML := ""
	for _, s := range sessions {
		sessionHTML += `<section><h3><a href="session/` + escapeHTML(s.PublishSlug) + `/index.html">` + escapeHTML(sessionDisplayName(s)) + `</a></h3>`
		sessionHTML += `<div style="display:flex;gap:8px;flex-wrap:wrap;margin:12px 0;">`
		for _, st := range s.Steps {
			for _, img := range st.ImageRefs {
				thumbURL, _, err := publishIndexThumbnailOutputPaths(siteDir, s.PublishSlug, st.Slug, img)
				if err != nil {
					return "", err
				}
				sessionHTML += `<a href="session/` + escapeHTML(s.PublishSlug) + `/index.html"><img src="` + escapeHTML(thumbURL) + `" alt="` + escapeHTML(st.Name) + `" style="display:block;width:72px;height:72px;object-fit:cover;border:1px solid #d2c5b3;background:#efe6da;" /></a>`
			}
		}
		sessionHTML += `</div></section>`
	}
	if sessionHTML == "" {
		sessionHTML = "<p>No sessions.</p>"
	}
	resultsHTML := "<pre>" + escapeHTML(studyDoc.SectionContent("Results")) + "</pre>" + sessionHTML

	methodsHTML := "<pre>" + escapeHTML(protocol.Summary) + "</pre>" + protocolHeading + "<ol>" + protocolSteps + "</ol>"
	preambleHTML := ""
	if preamble := strings.TrimSpace(studyDoc.Lead); preamble != "" {
		preambleHTML = "<pre>" + escapeHTML(preamble) + "</pre>"
	}

	return "<html><body><h1>" + escapeHTML(title) + wipBadge + "</h1>" +
		studyMeta +
		heroHTML +
		preambleHTML +
		"<h2>Introduction</h2><pre>" + escapeHTML(studyDoc.SectionContent("Introduction")) + "</pre>" +
		"<h2>Methods</h2>" + methodsHTML +
		"<h2>Results</h2>" + resultsHTML +
		"<h2>Discussion</h2><pre>" + escapeHTML(studyDoc.SectionContent("Discussion")) + "</pre>" +
		"<h2>Conclusion</h2><pre>" + escapeHTML(studyDoc.SectionContent("Conclusion")) + "</pre>" +
		"</body></html>", nil
}

func resolvePublishHeroComparison(studyFM map[string]any, sessions []publishSession) (*publishHeroComparison, error) {
	rawHero, ok := asMap(studyFM["hero_comparison"])
	if !ok || len(rawHero) == 0 {
		return nil, nil
	}
	leftRaw, ok := asMap(rawHero["left"])
	if !ok {
		return nil, errors.New("study.sg.md hero_comparison.left must be an object")
	}
	rightRaw, ok := asMap(rawHero["right"])
	if !ok {
		return nil, errors.New("study.sg.md hero_comparison.right must be an object")
	}
	leftRef, err := parsePublishHeroAssetRef(leftRaw, "left")
	if err != nil {
		return nil, err
	}
	rightRef, err := parsePublishHeroAssetRef(rightRaw, "right")
	if err != nil {
		return nil, err
	}
	left, err := resolvePublishHeroImage(leftRef, sessions)
	if err != nil {
		return nil, err
	}
	right, err := resolvePublishHeroImage(rightRef, sessions)
	if err != nil {
		return nil, err
	}
	return &publishHeroComparison{Left: left, Right: right}, nil
}

func parsePublishHeroAssetRef(raw map[string]any, side string) (publishHeroAssetRef, error) {
	sessionSlug := strings.TrimSpace(asString(raw["session"]))
	if sessionSlug == "" {
		return publishHeroAssetRef{}, fmt.Errorf("study.sg.md hero_comparison.%s.session is required", side)
	}
	stepSlug := strings.TrimSpace(asString(raw["step"]))
	if stepSlug == "" {
		return publishHeroAssetRef{}, fmt.Errorf("study.sg.md hero_comparison.%s.step is required", side)
	}
	assetIndex, ok := asInt(raw["asset_index"])
	if !ok || assetIndex < 0 {
		return publishHeroAssetRef{}, fmt.Errorf("study.sg.md hero_comparison.%s.asset_index must be a non-negative integer", side)
	}
	return publishHeroAssetRef{
		SessionSlug: sessionSlug,
		StepSlug:    stepSlug,
		AssetIndex:  assetIndex,
	}, nil
}

func resolvePublishHeroImage(ref publishHeroAssetRef, sessions []publishSession) (publishHeroImage, error) {
	for _, session := range sessions {
		if session.Slug != ref.SessionSlug && session.PublishSlug != ref.SessionSlug {
			continue
		}
		for _, step := range session.Steps {
			if step.Slug != ref.StepSlug {
				continue
			}
			if ref.AssetIndex >= len(step.SourceImageRefs) {
				return publishHeroImage{}, fmt.Errorf("study.sg.md hero_comparison %s/%s asset_index %d out of range", ref.SessionSlug, ref.StepSlug, ref.AssetIndex)
			}
			src := step.SourceImageRefs[ref.AssetIndex]
			return publishHeroImage{
				Ref: ref,
				Src: src,
				URL: publishHeroAssetRelativeURL(ref, src),
			}, nil
		}
		return publishHeroImage{}, fmt.Errorf("study.sg.md hero_comparison step not found: %s/%s", ref.SessionSlug, ref.StepSlug)
	}
	return publishHeroImage{}, fmt.Errorf("study.sg.md hero_comparison session not found: %s", ref.SessionSlug)
}

func publishHeroAssetRelativeURL(ref publishHeroAssetRef, src string) string {
	name := filepath.Base(src)
	ext := strings.ToLower(filepath.Ext(name))
	if ext == ".heic" || ext == ".heif" {
		name = strings.TrimSuffix(name, filepath.Ext(name)) + ".jpg"
	}
	return path.Join("hero", fmt.Sprintf("%s-%s-%d-%s", ref.SessionSlug, ref.StepSlug, ref.AssetIndex, name))
}

func resolvePublishStepImageRefs(sortedRefs []string, stepFM map[string]any) ([]string, error) {
	renderIndices, ok := asIntSlice(stepFM["render_asset_indices"])
	if !ok {
		return sortedRefs, nil
	}
	ordered := make([]string, 0, len(renderIndices))
	for _, idx := range renderIndices {
		if idx < 0 || idx >= len(sortedRefs) {
			return nil, fmt.Errorf("step.sg.md render_asset_indices index %d out of range", idx)
		}
		ordered = append(ordered, sortedRefs[idx])
	}
	return ordered, nil
}

func renderPublishSessionHTML(sessionDir, title string, s publishSession, prevSession, nextSession *publishSession, incomplete bool) (string, error) {
	columns := ""
	for _, st := range s.Steps {
		column := `<section class="step-column"><h2>` + escapeHTML(st.Name) + `</h2><div class="step-images">`
		for _, img := range st.ImageRefs {
			relURL, _, err := publishAssetOutputPaths(sessionDir, st.Slug, img)
			if err != nil {
				return "", err
			}
			column += `<img src="` + escapeHTML(relURL) + `" alt="` + escapeHTML(st.Name) + `" />`
		}
		column += `</div></section>`
		columns += column
	}
	if columns == "" {
		columns = `<p>No step images.</p>`
	}
	_ = incomplete
	sessionNav := `<span class="session-nav">`
	if prevSession != nil {
		sessionNav += `<span class="header-sep">|</span>`
		sessionNav += `<a class="session-link prev-session-link" href="../` + escapeHTML(prevSession.PublishSlug) + `/index.html">Prev</a>`
	}
	if nextSession != nil {
		sessionNav += `<span class="header-sep">|</span>`
		sessionNav += `<a class="session-link next-session-link" href="../` + escapeHTML(nextSession.PublishSlug) + `/index.html">Next</a>`
	}
	sessionNav += `</span>`
	return `<!doctype html><html><head><meta charset="utf-8"><title>` + escapeHTML(title) + `</title><style>
body{margin:0;font-family:Helvetica,Arial,sans-serif;background:#f6f1e8;color:#1f1b16;--image-size:40vw;}
.comparison-page{min-height:100vh;}
.session-header{position:sticky;top:0;padding:6px 10px 0;background:rgba(246,241,232,.96);backdrop-filter:blur(8px);}
.header-line{display:flex;align-items:center;gap:6px;font-size:13px;line-height:1.1;white-space:nowrap;overflow:hidden;}
.header-link,.header-date,.header-subject{display:block;overflow:hidden;text-overflow:ellipsis;}
.header-link{color:inherit;text-decoration:none;flex:0 0 auto;}
.session-link{color:inherit;text-decoration:none;flex:0 0 auto;}
.session-nav{display:flex;align-items:center;gap:6px;flex:0 0 auto;}
.header-sep{flex:0 0 auto;color:#8a7f72;}
.header-date{flex:0 0 auto;}
.header-subject{flex:1 1 auto;min-width:0;font-weight:600;}
.orientation-controls{display:flex;align-items:center;gap:2px;flex:0 0 auto;}
.orientation-controls label{display:flex;align-items:center;gap:3px;padding:0 2px;height:24px;box-sizing:border-box;}
.orientation-controls input[type="radio"]{margin:0;}
.image-size-controls{display:flex;align-items:center;gap:4px;flex:0 0 auto;margin-left:auto;}
.image-size-controls button{border:1px solid #b7aa98;background:#fffaf2;padding:6px 10px;font:inherit;cursor:pointer;}
.image-size-controls button,.image-size-controls input[type="range"]{height:24px;box-sizing:border-box;}
.image-size-controls input[type="range"]{width:110px;margin:0;}
.comparison-columns{display:flex;gap:0;align-items:flex-start;overflow-x:auto;padding:0 0 0;margin-top:2px;}
.comparison-columns.rows{flex-direction:column;overflow-x:visible;overflow-y:auto;}
.step-column{flex:0 0 min(var(--image-size), 40vw);width:min(var(--image-size), 40vw);min-width:50px;max-height:calc(100vh - 52px);padding:0;border:1px solid #d2c5b3;background:#fffaf2;overflow-y:auto;}
.step-column + .step-column{border-left:0;}
.comparison-columns.rows .step-column{display:grid;grid-template-columns:120px 1fr;width:100%;max-width:none;max-height:none;overflow:visible;}
.comparison-columns.rows .step-column + .step-column{border-left:1px solid #d2c5b3;border-top:0;}
.comparison-columns.rows .step-column h2{position:static;display:flex;align-items:center;min-height:100%;padding:8px;border-right:1px solid #e4d8c8;border-bottom:0;}
.comparison-columns.rows .step-images{display:flex;flex-direction:row;gap:6px;overflow-x:auto;overflow-y:visible;padding:6px;}
.comparison-columns.rows .step-images img{width:var(--image-size);max-width:40vw;height:auto;object-fit:contain;align-self:flex-start;flex:0 0 auto;}
.step-column h2{position:sticky;top:0;margin:0;padding:8px 10px;font-size:13px;border-bottom:1px solid #e4d8c8;background:#fffaf2;}
.step-images{display:flex;flex-direction:column;gap:6px;}
.step-images img{display:block;width:100%;height:auto;background:#efe6da;}
@media (max-width: 900px){.session-header{padding:6px 8px 0;}.header-line{gap:4px;font-size:12px;}.image-size-controls input[type="range"]{width:84px;}.comparison-columns{flex-direction:column;margin-top:2px;}.step-column{width:min(var(--image-size), calc(100vw - 8px));max-height:none;}.step-column + .step-column{border-left:1px solid #d2c5b3;}.orientation-controls label{padding:0 2px;}.comparison-columns.rows .step-column{grid-template-columns:88px 1fr;}.comparison-columns.rows .step-images img{max-width:calc(100vw - 120px);}}
</style><script>
function setImageSize(size){document.body.style.setProperty('--image-size', size);}
function applyStoredImageSize(){
const storedSize = localStorage.getItem('sg_publish_image_size');
if (!storedSize) {
return;
}
setImageSize(storedSize);
const slider = document.getElementById('image-size-slider');
if (!slider) {
return;
}
if (storedSize.endsWith('px')) {
slider.value = storedSize.slice(0, -2);
return;
}
if (storedSize.endsWith('vw')) {
slider.value = String(Number(storedSize.slice(0, -2)) * 10);
}
}
function persistImageSize(size){
localStorage.setItem('sg_publish_image_size', size);
}
function applyStoredOrientation(){
const storedOrientation = localStorage.getItem('sg_publish_orientation');
if (!storedOrientation) {
return;
}
setOrientation(storedOrientation);
const radio = document.querySelector('input[name="layout-orientation"][value="' + storedOrientation + '"]');
if (radio) {
radio.checked = true;
}
}
function setOrientation(value){
const layout = document.getElementById('comparison-columns');
layout.dataset.orientation = value;
layout.className = value === 'rows' ? 'comparison-columns rows' : 'comparison-columns';
localStorage.setItem('sg_publish_orientation', value);
}
function setImageSizeFromSlider(value){
const sliderValue = Number(value);
if (sliderValue <= 100) {
const size = sliderValue + 'px';
setImageSize(size);
persistImageSize(size);
return;
}
const size = (sliderValue / 10) + 'vw';
setImageSize(size);
persistImageSize(size);
}
function syncImageSizeSlider(value){
document.getElementById('image-size-slider').value = value;
}
</script></head><body onload="applyStoredImageSize();applyStoredOrientation()"><div class="comparison-page"><header class="session-header"><div class="header-line"><a class="header-link" href="../../index.html">Up</a>` + sessionNav + `<span class="header-sep">|</span><span class="header-date">` + escapeHTML(sessionStartDate(s.Started)) + `</span><span class="header-sep">|</span><span class="header-subject">` + escapeHTML(sessionDisplayName(s)) + `</span><span class="orientation-controls"><label><input type="radio" name="layout-orientation" value="columns" checked onchange="setOrientation('columns')" />Cols</label><label><input type="radio" name="layout-orientation" value="rows" onchange="setOrientation('rows')" />Rows</label></span><span class="image-size-controls"><button type="button" onclick="syncImageSizeSlider(50);setImageSize('50px');persistImageSize('50px')">Small</button><input id="image-size-slider" type="range" min="50" max="400" value="400" oninput="setImageSizeFromSlider(this.value)" aria-label="Image size" /><button type="button" onclick="syncImageSizeSlider(400);setImageSize('40vw');persistImageSize('40vw')">Large</button></span></div></header><main id="comparison-columns" class="comparison-columns" data-orientation="columns">` + columns + `</main></div></body></html>`, nil
}

func sessionStartDate(started string) string {
	parts := strings.Fields(strings.TrimSpace(started))
	if len(parts) >= 2 {
		return parts[1]
	}
	return started
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func sessionDisplayName(s publishSession) string {
	subjects := nonEmptySubjects(s.Subjects)
	switch len(subjects) {
	case 0:
		return firstNonEmpty(s.Slug, s.PublishSlug)
	case 1:
		return subjects[0]
	default:
		if allAnonymizedPublishSubjects(subjects) {
			return strings.Join(subjects, ", ")
		}
		lastNames := make([]string, 0, len(subjects))
		for _, subject := range subjects {
			lastNames = append(lastNames, lastToken(subject))
		}
		return strings.Join(lastNames, ", ")
	}
}

func allAnonymizedPublishSubjects(subjects []string) bool {
	if len(subjects) == 0 {
		return false
	}
	for _, subject := range subjects {
		if !strings.HasPrefix(strings.TrimSpace(subject), "Subject ") {
			return false
		}
	}
	return true
}

func nonEmptySubjects(subjects []string) []string {
	out := make([]string, 0, len(subjects))
	for _, subject := range subjects {
		if strings.TrimSpace(subject) != "" {
			out = append(out, subject)
		}
	}
	return out
}

func publishAssetOutputPaths(sessionDir, stepSlug, src string) (string, string, error) {
	relURL := publishAssetRelativeURL(stepSlug, src)
	dest := filepath.Join(sessionDir, filepath.FromSlash(relURL))
	return relURL, dest, nil
}

func publishIndexThumbnailOutputPaths(siteDir, sessionSlug, stepSlug, src string) (string, string, error) {
	relURL := publishIndexThumbnailRelativeURL(sessionSlug, stepSlug, src)
	dest := filepath.Join(siteDir, filepath.FromSlash(relURL))
	return relURL, dest, nil
}

func publishAssetRelativeURL(stepSlug, src string) string {
	name := filepath.Base(src)
	ext := strings.ToLower(filepath.Ext(name))
	if ext == ".heic" || ext == ".heif" {
		name = strings.TrimSuffix(name, filepath.Ext(name)) + ".jpg"
	}
	return path.Join("assets", stepSlug, name)
}

func publishIndexThumbnailRelativeURL(sessionSlug, stepSlug, src string) string {
	name := filepath.Base(src)
	name = strings.TrimSuffix(name, filepath.Ext(name)) + ".jpg"
	return path.Join("thumbs", sessionSlug, stepSlug, name)
}

func publishAssetForHTML(src, dest string) error {
	upToDate, err := publishAssetUpToDate(src, dest)
	if err != nil {
		return err
	}
	if upToDate {
		return nil
	}
	ext := strings.ToLower(filepath.Ext(src))
	if ext == ".heic" || ext == ".heif" {
		return publishImagePreviewFn(src, dest)
	}
	return copyFile(src, dest)
}

func publishAssetUpToDate(src, dest string) (bool, error) {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return false, err
	}
	destInfo, err := os.Stat(dest)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return !destInfo.ModTime().Before(srcInfo.ModTime()), nil
}

func publishImagePreview(src, dst string) error {
	cmd := exec.Command("sips", "-s", "format", "jpeg", src, "--out", dst)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("render publish preview for %s: %w: %s", src, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func publishImageThumbnail(src, dst string) error {
	return imageThumbnail(src, dst, 144)
}

func exportImageThumbnail(src, dst string, maxDim int) error {
	return imageThumbnail(src, dst, maxDim)
}

func imageThumbnail(src, dst string, maxDim int) error {
	upToDate, err := publishAssetUpToDate(src, dst)
	if err != nil {
		return err
	}
	if upToDate {
		return nil
	}
	file, err := os.Open(src)
	if err != nil {
		return err
	}
	img, _, decodeErr := image.Decode(file)
	_ = file.Close()
	if decodeErr == nil {
		thumb := resizeImageToFit(img, maxDim)
		out, err := os.Create(dst)
		if err != nil {
			return err
		}
		encodeErr := jpeg.Encode(out, thumb, &jpeg.Options{Quality: 80})
		closeErr := out.Close()
		if encodeErr != nil {
			return encodeErr
		}
		return closeErr
	}
	cmd := exec.Command("sips", "-Z", strconv.Itoa(maxDim), "-s", "format", "jpeg", src, "--out", dst)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("render publish thumbnail for %s: %w: %s", src, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func resizeImageToFit(src image.Image, maxDim int) image.Image {
	bounds := src.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()
	if srcW <= 0 || srcH <= 0 {
		return image.NewRGBA(image.Rect(0, 0, 1, 1))
	}
	scale := math.Min(1, float64(maxDim)/float64(max(srcW, srcH)))
	dstW := max(1, int(math.Round(float64(srcW)*scale)))
	dstH := max(1, int(math.Round(float64(srcH)*scale)))
	if dstW == srcW && dstH == srcH {
		dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
		for y := 0; y < dstH; y++ {
			for x := 0; x < dstW; x++ {
				dst.Set(x, y, src.At(bounds.Min.X+x, bounds.Min.Y+y))
			}
		}
		return dst
	}
	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	for y := 0; y < dstH; y++ {
		srcY := bounds.Min.Y + y*srcH/dstH
		for x := 0; x < dstW; x++ {
			srcX := bounds.Min.X + x*srcW/dstW
			dst.Set(x, y, src.At(srcX, srcY))
		}
	}
	return dst
}

func renderPublishText(studyDoc store.StudyDocument, studyFM map[string]any, protocol store.Protocol, sessions []publishSession) string {
	var b strings.Builder
	b.WriteString("Study: ")
	b.WriteString(studyDoc.Title)
	b.WriteString("\nStatus: ")
	b.WriteString(asString(studyFM["status"]))
	b.WriteString("\nCreated: ")
	b.WriteString(asString(studyFM["created_on"]))
	if preamble := strings.TrimSpace(studyDoc.Lead); preamble != "" {
		b.WriteString("\n\n")
		b.WriteString(preamble)
	}
	b.WriteString("\n\nIntroduction\n")
	b.WriteString(studyDoc.SectionContent("Introduction"))
	b.WriteString("\n\nMethods\n")
	b.WriteString(protocol.Summary)
	if len(protocol.Steps) > 0 {
		b.WriteString("\n\nProtocol\n")
		for _, step := range protocol.Steps {
			b.WriteString("\n- ")
			b.WriteString(step.Name)
		}
	}
	b.WriteString("\n\nResults\n")
	b.WriteString(studyDoc.SectionContent("Results"))
	sessionResults := renderPublishSessionSummaryText(sessions)
	if strings.TrimSpace(sessionResults) != "" {
		b.WriteString("\n\n")
		b.WriteString(sessionResults)
	}
	b.WriteString("\n\nDiscussion\n")
	b.WriteString(studyDoc.SectionContent("Discussion"))
	b.WriteString("\n\nConclusion\n")
	b.WriteString(studyDoc.SectionContent("Conclusion"))
	return b.String()
}

func renderPublishSessionSummaryText(sessions []publishSession) string {
	if len(sessions) == 0 {
		return ""
	}
	var b strings.Builder
	for i, s := range sessions {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString("Session ")
		b.WriteString(firstNonEmpty(s.PublishSlug, s.Slug))
		b.WriteString(" | subjects: ")
		b.WriteString(strings.Join(s.Subjects, ", "))
		b.WriteString("\n")
		for _, st := range s.Steps {
			b.WriteString("- ")
			b.WriteString(st.Name)
			b.WriteString(" [")
			b.WriteString(st.Started)
			b.WriteString(" - ")
			b.WriteString(st.Finished)
			b.WriteString("] images: ")
			b.WriteString(fmt.Sprintf("%d", len(st.ImageRefs)))
			b.WriteString("\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func extractSection(md, name string) string {
	lines := strings.Split(md, "\n")
	head := "# " + name
	collecting := false
	var out []string
	for _, raw := range lines {
		trimmed := strings.TrimSpace(raw)
		if strings.HasPrefix(trimmed, "# ") {
			if trimmed == head {
				collecting = true
				continue
			}
			if collecting {
				break
			}
		}
		if collecting {
			out = append(out, raw)
		}
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

func cmdIngestPhotos(args []string) error {
	opts, err := parseIngestPhotosArgs(args)
	if err != nil {
		return err
	}
	if opts.AssetsDir == "" && runtime.GOOS != "darwin" {
		return errors.New("sg data ingest is supported only on macOS in v1")
	}
	root, err := util.StudyRootFromCwd()
	if err != nil {
		return err
	}
	protocol, err := store.ParseProtocol(root)
	if err != nil {
		return err
	}
	if len(protocol.Steps) == 0 {
		return errors.New("protocol has no steps")
	}

	sessions, err := listSessionSlugs(root)
	if err != nil {
		return err
	}
	if len(sessions) == 0 {
		return errors.New("no sessions found")
	}

	plans := make([]sessionIngestPlan, 0, len(sessions))
	var globalStart time.Time
	var globalEnd time.Time
	for _, session := range sessions {
		sessionDir := filepath.Join(root, "session", session)
		windows, err := loadStepWindows(sessionDir, protocol)
		if err != nil {
			fmt.Printf("warning: skipping %s session %s: %v\n", classifySessionIngestSkip(err), session, err)
			continue
		}
		plans = append(plans, sessionIngestPlan{
			slug:       session,
			sessionDir: sessionDir,
			windows:    windows,
		})
		for _, w := range windows {
			if globalStart.IsZero() || w.Start.Before(globalStart) {
				globalStart = w.Start
			}
			if globalEnd.IsZero() || w.End.After(globalEnd) {
				globalEnd = w.End
			}
		}
	}
	if len(plans) == 0 {
		return errors.New("no ingestible sessions found")
	}

	var exported []string
	if opts.AssetsDir != "" {
		exported, err = collectIngestSourceFiles(opts.AssetsDir)
		if err != nil {
			return err
		}
		if len(exported) == 0 {
			fmt.Println("no assets found in assets dir", opts.AssetsDir)
			return nil
		}
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		sourceDir, err := defaultPhotosLibrarySourceDir(home)
		if err != nil {
			return err
		}
		dbPath, originalsRoot, hasSQLite := photosLibrarySQLiteContextFromSourceDir(sourceDir)
		if !hasSQLite {
			return fmt.Errorf("Photos Library metadata database not found for source directory: %s", sourceDir)
		}
		fmt.Printf("scanning Photos Library assets from %s using SQLite window %s to %s\n", dbPath, globalStart.Format(time.RFC3339), globalEnd.Format(time.RFC3339))
		exported, err = collectPhotosLibraryIngestFilesBySQLite(dbPath, originalsRoot, globalStart, globalEnd, photosLibraryMTimeSkew)
		if err != nil {
			return err
		}
		if len(exported) == 0 {
			fmt.Println("no assets found in Photos Library SQLite metadata window", dbPath)
			return nil
		}
	}

	total := ingestStats{}
	capturedAssets, err := buildCapturedAssets(exported, exifCaptureTimeFn)
	if err != nil {
		return err
	}
	if latestCapture, ok := latestCapturedAssetTime(capturedAssets); ok && latestCapture.Before(globalEnd) {
		fmt.Printf(
			"warning: latest available asset capture time %s is older than latest focus window end %s; source sync may be incomplete\n",
			latestCapture.Format(util.TimestampLayout),
			globalEnd.Format(util.TimestampLayout),
		)
	}
	for _, plan := range plans {
		stats, err := ingestCapturedAssetsForSession(plan.sessionDir, plan.windows, capturedAssets, func(msg string, a ...any) {
			fmt.Printf("session=%s ", plan.slug)
			fmt.Printf(msg, a...)
		})
		if err != nil {
			return fmt.Errorf("session %s: %w", plan.slug, err)
		}
		total.Copied += stats.Copied
		total.SkippedDup += stats.SkippedDup
		total.SkippedNoEXIF += stats.SkippedNoEXIF
		total.SkippedWindow += stats.SkippedWindow
		fmt.Printf("session %s: copied=%d skipped_duplicate=%d skipped_no_exif=%d skipped_outside_windows=%d\n", plan.slug, stats.Copied, stats.SkippedDup, stats.SkippedNoEXIF, stats.SkippedWindow)
	}
	fmt.Printf("ingest complete: sessions=%d copied=%d skipped_duplicate=%d skipped_no_exif=%d skipped_outside_windows=%d\n", len(plans), total.Copied, total.SkippedDup, total.SkippedNoEXIF, total.SkippedWindow)
	return nil
}

func classifySessionIngestSkip(err error) string {
	msg := err.Error()
	switch {
	case strings.HasPrefix(msg, "missing step file:"):
		return "incomplete"
	case msg == "last step is missing time_finished":
		return "incomplete"
	case strings.HasPrefix(msg, "open focus_windows interval for step "):
		return "incomplete"
	default:
		return "invalid"
	}
}

func cmdDataClean(args []string) error {
	if len(args) > 0 {
		return errors.New("usage: sg data clean")
	}
	root, err := util.StudyRootFromCwd()
	if err != nil {
		return err
	}
	if err := reconcileProtocolStepSlugs(root); err != nil {
		return err
	}
	sessionRoot := filepath.Join(root, "session")
	removed := 0
	err = filepath.WalkDir(sessionRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) {
				return nil
			}
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !strings.Contains(filepath.ToSlash(path), "/step/") || !strings.Contains(filepath.ToSlash(path), "/asset/") {
			return nil
		}
		if err := os.Remove(path); err != nil {
			return err
		}
		removed++
		return nil
	})
	if err != nil {
		return err
	}
	fmt.Printf("removed asset files: %d\n", removed)
	return nil
}

func reconcileProtocolStepSlugs(root string) error {
	_, err := store.ParseProtocol(root)
	return err
}

func parseIngestPhotosArgs(args []string) (ingestPhotosOptions, error) {
	opts := ingestPhotosOptions{}
	for i := 0; i < len(args); i++ {
		arg := strings.TrimSpace(args[i])
		if arg == "" {
			continue
		}
		switch {
		case arg == "--assets-dir":
			if i+1 >= len(args) || strings.TrimSpace(args[i+1]) == "" {
				return opts, errors.New("missing value for --assets-dir")
			}
			opts.AssetsDir = strings.TrimSpace(args[i+1])
			i++
		case strings.HasPrefix(arg, "--assets-dir="):
			opts.AssetsDir = strings.TrimSpace(strings.TrimPrefix(arg, "--assets-dir="))
			if opts.AssetsDir == "" {
				return opts, errors.New("missing value for --assets-dir")
			}
		case strings.HasPrefix(arg, "-"):
			return opts, fmt.Errorf("unknown flag: %s", arg)
		default:
			return opts, errors.New("album name positional arguments are no longer supported; default mode scans Photos Library files directly or use sg data ingest --assets-dir <path>")
		}
	}
	return opts, nil
}

func collectImageFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".jpg", ".jpeg", ".png", ".heic", ".heif", ".tif", ".tiff":
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func collectIngestSourceFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if isSupportedIngestExt(path) {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func photosLibrarySQLiteContextFromSourceDir(sourceDir string) (string, string, bool) {
	clean := filepath.Clean(sourceDir)
	// Common case: sourceDir points to .../Photos Library.photoslibrary/originals
	base := strings.ToLower(filepath.Base(clean))
	if base == "originals" || base == "masters" {
		libRoot := filepath.Dir(clean)
		dbPath := filepath.Join(libRoot, "database", "Photos.sqlite")
		if info, err := os.Stat(dbPath); err == nil && !info.IsDir() {
			return dbPath, clean, true
		}
	}
	// Alternate case: sourceDir points to package root.
	dbPath := filepath.Join(clean, "database", "Photos.sqlite")
	origRoot := filepath.Join(clean, "originals")
	if info, err := os.Stat(dbPath); err == nil && !info.IsDir() {
		if oinfo, oerr := os.Stat(origRoot); oerr == nil && oinfo.IsDir() {
			return dbPath, origRoot, true
		}
	}
	return "", "", false
}

func collectPhotosLibraryIngestFilesBySQLite(dbPath, originalsRoot string, start, end time.Time, skew time.Duration) ([]string, error) {
	minTime := start.Add(-skew)
	maxTime := end.Add(skew)
	minUnix := minTime.Unix()
	maxUnix := maxTime.Unix()
	minExif := minTime.Local().Format("2006:01:02 15:04:05")
	maxExif := maxTime.Local().Format("2006:01:02 15:04:05")
	query := fmt.Sprintf(`
WITH filtered AS (
  SELECT
    COALESCE(a.ZDIRECTORY, '') AS zdirectory,
    a.ZFILENAME AS zfilename,
    COALESCE(NULLIF(a.ZMASTER, 0), a.Z_PK) AS logical_asset_key,
    COALESCE(a.ZMODIFICATIONDATE, a.ZDATECREATED, -1) AS sort_modified,
    a.Z_PK AS sort_pk
  FROM ZASSET a
  LEFT JOIN ZADDITIONALASSETATTRIBUTES aa ON aa.ZASSET = a.Z_PK
  WHERE a.ZFILENAME IS NOT NULL
    AND (
      lower(a.ZFILENAME) LIKE '%%.jpg'
      OR lower(a.ZFILENAME) LIKE '%%.jpeg'
      OR lower(a.ZFILENAME) LIKE '%%.png'
      OR lower(a.ZFILENAME) LIKE '%%.heic'
      OR lower(a.ZFILENAME) LIKE '%%.heif'
      OR lower(a.ZFILENAME) LIKE '%%.mov'
      OR lower(a.ZFILENAME) LIKE '%%.mp4'
      OR lower(a.ZFILENAME) LIKE '%%.m4v'
      OR lower(a.ZFILENAME) LIKE '%%.tif'
      OR lower(a.ZFILENAME) LIKE '%%.tiff'
    )
    AND (
      (a.ZDATECREATED IS NOT NULL AND (a.ZDATECREATED + strftime('%%s','2001-01-01 00:00:00')) BETWEEN %d AND %d)
      OR (aa.ZEXIFTIMESTAMPSTRING IS NOT NULL AND aa.ZEXIFTIMESTAMPSTRING >= '%s' AND aa.ZEXIFTIMESTAMPSTRING <= '%s')
    )
),
ranked AS (
  SELECT
    zdirectory,
    zfilename,
    logical_asset_key,
    ROW_NUMBER() OVER (
      PARTITION BY logical_asset_key
      ORDER BY sort_modified DESC, sort_pk DESC
    ) AS rn
  FROM filtered
)
SELECT ranked.zdirectory, ranked.zfilename
FROM ranked
WHERE ranked.rn = 1;`, minUnix, maxUnix, minExif, maxExif)
	out, err := runSQLiteQueryFn(dbPath, query)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	files := []string{}
	for _, raw := range strings.Split(out, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 2)
		if len(parts) != 2 {
			continue
		}
		dir := strings.TrimSpace(parts[0])
		name := strings.TrimSpace(parts[1])
		if name == "" || !isSupportedIngestExt(name) {
			continue
		}
		path := filepath.Join(originalsRoot, dir, name)
		info, statErr := os.Stat(path)
		if statErr != nil || info.IsDir() {
			continue
		}
		if seen[path] {
			continue
		}
		seen[path] = true
		files = append(files, path)
	}
	sort.Strings(files)
	return files, nil
}

func collectPhotosLibraryImageFilesBySQLite(dbPath, originalsRoot string, start, end time.Time, skew time.Duration) ([]string, error) {
	files, err := collectPhotosLibraryIngestFilesBySQLite(dbPath, originalsRoot, start, end, skew)
	if err != nil {
		return nil, err
	}
	filtered := files[:0]
	for _, path := range files {
		if isSupportedImageExt(path) {
			filtered = append(filtered, path)
		}
	}
	return filtered, nil
}

func isSupportedImageExt(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg", ".png", ".heic", ".heif", ".tif", ".tiff":
		return true
	default:
		return false
	}
}

func isSupportedIngestExt(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg", ".png", ".heic", ".heif", ".tif", ".tiff", ".mov", ".mp4", ".m4v":
		return true
	default:
		return false
	}
}

var runSQLiteQueryFn = runSQLiteQuery

func runSQLiteQuery(dbPath, query string) (string, error) {
	cmd := exec.Command("sqlite3", "-readonly", "-noheader", "-separator", "|", dbPath, query)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed querying Photos sqlite db %s: %v (%s)", dbPath, err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func collectImageFilesByMTime(root string, start, end time.Time, skew time.Duration) ([]string, error) {
	minTime := start.Add(-skew)
	maxTime := end.Add(skew)
	minStr := minTime.Format("2006-01-02 15:04:05")
	maxStr := maxTime.Format("2006-01-02 15:04:05")
	cmd := exec.Command(
		"find", root,
		"-type", "f",
		"(",
		"-iname", "*.jpg",
		"-o", "-iname", "*.jpeg",
		"-o", "-iname", "*.png",
		"-o", "-iname", "*.heic",
		"-o", "-iname", "*.heif",
		"-o", "-iname", "*.mov",
		"-o", "-iname", "*.mp4",
		"-o", "-iname", "*.m4v",
		"-o", "-iname", "*.tif",
		"-o", "-iname", "*.tiff",
		")",
		"-newermt", minStr,
		"!", "-newermt", maxStr,
	)
	out, err := cmd.Output()
	if err == nil {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		files := make([]string, 0, len(lines))
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if isIgnoredPhotosCandidatePath(line) {
				continue
			}
			files = append(files, line)
		}
		sort.Strings(files)
		return files, nil
	}

	var files []string
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if isIgnoredPhotosCandidatePath(path) {
				return filepath.SkipDir
			}
			return nil
		}
		if isIgnoredPhotosCandidatePath(path) {
			return nil
		}
		if !isSupportedIngestExt(path) {
			return nil
		}
		info, infoErr := d.Info()
		if infoErr != nil {
			return infoErr
		}
		mod := info.ModTime()
		if mod.Before(minTime) || mod.After(maxTime) {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func isIgnoredPhotosCandidatePath(path string) bool {
	p := strings.ToLower(filepath.Clean(path))
	parts := strings.Split(p, string(filepath.Separator))
	for _, part := range parts {
		if part == "derivatives" || part == "previews" {
			return true
		}
	}
	return false
}

type capturedAsset struct {
	path        string
	captureTime time.Time
	exifErr     error
}

func buildCapturedAssets(sources []string, captureTimeFn func(string) (time.Time, error)) ([]capturedAsset, error) {
	out := make([]capturedAsset, 0, len(sources))
	for _, src := range sources {
		captureTime, exifErr := captureTimeFn(src)
		out = append(out, capturedAsset{
			path:        src,
			captureTime: captureTime,
			exifErr:     exifErr,
		})
	}
	return dedupeCapturedAssetsByCaptureInstant(out), nil
}

func latestCapturedAssetTime(assets []capturedAsset) (time.Time, bool) {
	var latest time.Time
	for _, asset := range assets {
		if asset.exifErr != nil {
			continue
		}
		if latest.IsZero() || asset.captureTime.After(latest) {
			latest = asset.captureTime
		}
	}
	if latest.IsZero() {
		return time.Time{}, false
	}
	return latest, true
}

func dedupeCapturedAssetsByCaptureInstant(assets []capturedAsset) []capturedAsset {
	type pick struct {
		asset capturedAsset
		mod   time.Time
	}
	byKey := map[string]pick{}
	ordered := make([]string, 0, len(assets))
	keepDirect := make([]capturedAsset, 0, len(assets))

	for _, asset := range assets {
		if asset.exifErr != nil || asset.captureTime.Nanosecond() == 0 {
			keepDirect = append(keepDirect, asset)
			continue
		}
		key := asset.captureTime.In(time.UTC).Format(time.RFC3339Nano)
		mod := fileModTime(asset.path)
		existing, ok := byKey[key]
		if !ok {
			byKey[key] = pick{asset: asset, mod: mod}
			ordered = append(ordered, key)
			continue
		}
		if shouldReplaceCapturedAssetPick(asset.path, mod, existing.asset.path, existing.mod) {
			byKey[key] = pick{asset: asset, mod: mod}
		}
	}

	out := make([]capturedAsset, 0, len(keepDirect)+len(ordered))
	out = append(out, keepDirect...)
	for _, key := range ordered {
		out = append(out, byKey[key].asset)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].path < out[j].path
	})
	return out
}

func shouldReplaceCapturedAssetPick(candidatePath string, candidateMod time.Time, existingPath string, existingMod time.Time) bool {
	candidateRender := isPhotosRenderPath(candidatePath)
	existingRender := isPhotosRenderPath(existingPath)
	if candidateRender != existingRender {
		return candidateRender
	}
	if candidateMod.After(existingMod) {
		return true
	}
	if candidateMod.Equal(existingMod) && candidatePath < existingPath {
		return true
	}
	return false
}

func isPhotosRenderPath(path string) bool {
	p := strings.ToLower(filepath.ToSlash(path))
	return strings.Contains(p, "/resources/renders/")
}

func ingestCapturedAssetsForSession(sessionDir string, windows []stepWindow, assets []capturedAsset, warnf func(string, ...any)) (ingestStats, error) {
	stats := ingestStats{}
	existingHashes, err := collectSessionAssetHashes(sessionDir)
	if err != nil {
		return stats, err
	}

	for _, asset := range assets {
		if asset.exifErr != nil {
			stats.SkippedNoEXIF++
			if warnf != nil {
				warnf("warning: skipped (missing capture timestamp metadata): %s\n", asset.path)
			}
			continue
		}
		idx := findStepWindowIndex(asset.captureTime, windows)
		if idx < 0 {
			stats.SkippedWindow++
			continue
		}
		hash8, err := fileSHA8(asset.path)
		if err != nil {
			return stats, err
		}
		if existingHashes[hash8] {
			stats.SkippedDup++
			continue
		}
		ext := strings.ToLower(filepath.Ext(asset.path))
		if ext == "" {
			ext = ".bin"
		}
		destName := fmt.Sprintf("%s_%s%s", asset.captureTime.Local().Format("20060102-150405"), hash8, ext)
		destDir := filepath.Join(sessionDir, "step", windows[idx].StepSlug, "asset")
		if err := util.EnsureDir(destDir); err != nil {
			return stats, err
		}
		dest := filepath.Join(destDir, destName)
		if _, err := os.Stat(dest); err == nil {
			stats.SkippedDup++
			continue
		}
		if err := copyFile(asset.path, dest); err != nil {
			return stats, err
		}
		existingHashes[hash8] = true
		stats.Copied++
	}
	if err := renumberSessionAssetFiles(sessionDir); err != nil {
		return stats, err
	}
	return stats, nil
}

type sessionAssetFile struct {
	path        string
	dir         string
	name        string
	captureTime time.Time
	hash8       string
	ext         string
}

func renumberSessionAssetFiles(sessionDir string) error {
	assets, err := collectManagedSessionAssetFiles(sessionDir)
	if err != nil {
		return err
	}
	if len(assets) == 0 {
		return nil
	}
	assetsByDir := map[string][]sessionAssetFile{}
	for _, asset := range assets {
		assetsByDir[asset.dir] = append(assetsByDir[asset.dir], asset)
	}

	type renamePlan struct {
		from  string
		temp  string
		final string
	}
	plans := make([]renamePlan, 0, len(assets))
	for dir, group := range assetsByDir {
		sort.Slice(group, func(i, j int) bool {
			if !group[i].captureTime.Equal(group[j].captureTime) {
				return group[i].captureTime.Before(group[j].captureTime)
			}
			if group[i].hash8 != group[j].hash8 {
				return group[i].hash8 < group[j].hash8
			}
			return group[i].path < group[j].path
		})
		width := len(strconv.Itoa(len(group)))
		if width < 2 {
			width = 2
		}
		for i, asset := range group {
			finalName := fmt.Sprintf("%0*d-%s_%s%s", width, i, asset.captureTime.Local().Format("20060102-150405"), asset.hash8, asset.ext)
			finalPath := filepath.Join(dir, finalName)
			if finalPath == asset.path {
				continue
			}
			tempPath := filepath.Join(dir, fmt.Sprintf(".renaming-%02d-%s", i, asset.name))
			plans = append(plans, renamePlan{from: asset.path, temp: tempPath, final: finalPath})
		}
	}
	for _, plan := range plans {
		if err := os.Rename(plan.from, plan.temp); err != nil {
			return err
		}
	}
	for _, plan := range plans {
		if err := os.Rename(plan.temp, plan.final); err != nil {
			return err
		}
	}
	return nil
}

func collectManagedSessionAssetFiles(sessionDir string) ([]sessionAssetFile, error) {
	assetRoot := filepath.Join(sessionDir, "step")
	var out []sessionAssetFile
	err := filepath.WalkDir(assetRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) {
				return nil
			}
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !strings.Contains(filepath.ToSlash(path), "/asset/") {
			return nil
		}
		if strings.EqualFold(filepath.Base(path), ".DS_Store") {
			return nil
		}
		asset, ok, err := parseManagedSessionAssetFile(path)
		if err != nil {
			return err
		}
		if ok {
			out = append(out, asset)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func parseManagedSessionAssetFile(path string) (sessionAssetFile, bool, error) {
	name := filepath.Base(path)
	matches := managedAssetNamePattern.FindStringSubmatch(name)
	if len(matches) != 4 {
		return sessionAssetFile{}, false, nil
	}
	captureTime, err := time.ParseInLocation("20060102-150405", matches[1], time.Local)
	if err != nil {
		return sessionAssetFile{}, false, fmt.Errorf("parse managed asset timestamp %q: %w", name, err)
	}
	return sessionAssetFile{
		path:        path,
		dir:         filepath.Dir(path),
		name:        name,
		captureTime: captureTime,
		hash8:       matches[2],
		ext:         strings.ToLower(matches[3]),
	}, true, nil
}

type stepWindow struct {
	StepSlug string
	Start    time.Time
	End      time.Time
	Last     bool
}

type ingestPhotosOptions struct {
	AssetsDir string
}

type ingestStats struct {
	Copied        int
	SkippedDup    int
	SkippedNoEXIF int
	SkippedWindow int
}

var exifCaptureTimeFn = exifCaptureTime

const photosLibraryMTimeSkew = 6 * time.Hour

type sessionIngestPlan struct {
	slug       string
	sessionDir string
	windows    []stepWindow
}

func defaultPhotosLibrarySourceDir(home string) (string, error) {
	if strings.TrimSpace(home) == "" {
		return "", errors.New("could not resolve user home directory for Photos Library lookup")
	}
	cfgPath := filepath.Join(home, ".study-guide", "config")
	cfg, warnings, err := readStudyGuideConfig(cfgPath)
	if err != nil {
		return "", err
	}
	for _, w := range warnings {
		fmt.Fprintln(os.Stderr, w)
	}
	if strings.TrimSpace(cfg.PhotosLibraryPath) != "" {
		configured := expandHomePath(strings.TrimSpace(cfg.PhotosLibraryPath), home)
		info, err := os.Stat(configured)
		if err == nil && info.IsDir() {
			if resolved, ok := resolvePhotosLibraryAssetSubdir(configured); ok {
				return resolved, nil
			}
			return "", fmt.Errorf("configured photos_library_path is not a Photos Library package with originals/: %s", configured)
		}
		if err != nil && os.IsNotExist(err) {
			return "", fmt.Errorf("configured photos_library_path not found: %s", configured)
		}
		if err != nil {
			return "", fmt.Errorf("failed checking configured photos_library_path %s: %w", configured, err)
		}
		return "", fmt.Errorf("configured photos_library_path is not a directory: %s", configured)
	}
	candidates := []string{
		filepath.Join(home, "Pictures", "Photos Library.photoslibrary", "originals"),
		filepath.Join(home, "Pictures", "Photos Library.photoslibrary", "Masters"),
	}
	checked := make([]string, 0, len(candidates))
	for _, path := range candidates {
		checked = append(checked, path)
		info, err := os.Stat(path)
		if err == nil && info.IsDir() {
			return path, nil
		}
		if err != nil && !os.IsNotExist(err) {
			return "", fmt.Errorf("failed checking Photos Library path %s: %w", path, err)
		}
	}
	return "", fmt.Errorf("Photos Library source directory not found; checked: %s", strings.Join(checked, ", "))
}

func resolvePhotosLibraryAssetSubdir(path string) (string, bool) {
	candidates := []string{
		filepath.Join(path, "originals"),
		filepath.Join(path, "Masters"),
	}
	for _, c := range candidates {
		info, err := os.Stat(c)
		if err == nil && info.IsDir() {
			return c, true
		}
	}
	return "", false
}

type studyGuideConfig struct {
	PhotosLibraryPath      string `yaml:"photos_library_path"`
	PhotoLibraryPathLegacy string `yaml:"photo_library_path"`
}

func readStudyGuideConfig(path string) (studyGuideConfig, []string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return studyGuideConfig{}, nil, nil
		}
		return studyGuideConfig{}, nil, fmt.Errorf("failed reading config file %s: %w", path, err)
	}
	var cfg studyGuideConfig
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return studyGuideConfig{}, nil, fmt.Errorf("invalid config file %s: %w", path, err)
	}
	var rawMap map[string]any
	if err := yaml.Unmarshal(raw, &rawMap); err != nil {
		return studyGuideConfig{}, nil, fmt.Errorf("invalid config file %s: %w", path, err)
	}
	warnings := []string{}
	known := map[string]bool{
		"photos_library_path": true,
		"photo_library_path":  true,
	}
	for key := range rawMap {
		if !known[key] {
			warnings = append(warnings, fmt.Sprintf("warning: unrecognized config key in %s: %s", path, key))
		}
	}
	if strings.TrimSpace(cfg.PhotosLibraryPath) == "" && strings.TrimSpace(cfg.PhotoLibraryPathLegacy) != "" {
		cfg.PhotosLibraryPath = cfg.PhotoLibraryPathLegacy
		warnings = append(warnings, fmt.Sprintf("warning: deprecated config key in %s: photo_library_path (use photos_library_path)", path))
	}
	sort.Strings(warnings)
	return cfg, warnings, nil
}

func expandHomePath(path, home string) string {
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return path
}

func loadStepWindows(sessionDir string, protocol store.Protocol) ([]stepWindow, error) {
	focusableIdxs := make([]int, 0, len(protocol.Steps))
	starts := make([]time.Time, len(protocol.Steps))
	finishes := make([]time.Time, len(protocol.Steps))
	hasExplicitFinish := make([]bool, len(protocol.Steps))
	stepFocusWindows := make([][]focusWindow, len(protocol.Steps))
	for i, st := range protocol.Steps {
		stepPath := filepath.Join(sessionDir, "step", st.Slug, "step.sg.md")
		fm, _, err := util.ReadFrontmatterFile(stepPath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("missing step file: %s", stepPath)
			}
			return nil, fmt.Errorf("step file read error: %s: %w", stepPath, err)
		}
		if asBool(fm["unfocusable"]) {
			continue
		}
		focusableIdxs = append(focusableIdxs, i)
		started := asString(fm["time_started"])
		if strings.TrimSpace(started) == "" {
			return nil, fmt.Errorf("step missing time_started: %s", stepPath)
		}
		t, err := util.ParseTimestamp(started)
		if err != nil {
			return nil, fmt.Errorf("invalid step time_started: %s", stepPath)
		}
		starts[i] = t
		if finished := asString(fm["time_finished"]); strings.TrimSpace(finished) != "" {
			ft, err := util.ParseTimestamp(finished)
			if err != nil {
				return nil, fmt.Errorf("invalid step time_finished: %s", stepPath)
			}
			finishes[i] = ft
			hasExplicitFinish[i] = true
		}
		windows := decodeFocusWindows(fm["focus_windows"])
		if len(windows) == 0 {
			return nil, fmt.Errorf("step missing focus_windows: %s", stepPath)
		}
		stepFocusWindows[i] = windows
	}
	if len(focusableIdxs) == 0 {
		return nil, nil
	}
	last := focusableIdxs[len(focusableIdxs)-1]
	if finishes[last].IsZero() {
		return nil, errors.New("last step is missing time_finished")
	}
	for j := 0; j < len(focusableIdxs)-1; j++ {
		i := focusableIdxs[j]
		nextStart := starts[focusableIdxs[j+1]]
		if !hasExplicitFinish[i] {
			finishes[i] = nextStart.Add(-1 * time.Second)
			continue
		}
		if finishes[i].After(nextStart) {
			return nil, fmt.Errorf("step time_finished exceeds next step start for step %s", protocol.Steps[i].Slug)
		}
	}

	windows := make([]stepWindow, 0, len(focusableIdxs))
	for _, i := range focusableIdxs {
		st := protocol.Steps[i]
		for _, fw := range stepFocusWindows[i] {
			fwStart, err := util.ParseTimestamp(fw.TimeStarted)
			if err != nil {
				return nil, fmt.Errorf("invalid focus_windows.time_started for step %s", st.Slug)
			}
			if strings.TrimSpace(fw.TimeFinished) == "" {
				return nil, fmt.Errorf("open focus_windows interval for step %s", st.Slug)
			}
			fwEnd, err := util.ParseTimestamp(fw.TimeFinished)
			if err != nil {
				return nil, fmt.Errorf("invalid focus_windows.time_finished for step %s", st.Slug)
			}
			if fwEnd.Before(fwStart) {
				return nil, fmt.Errorf("focus_windows interval ends before it starts for step %s", st.Slug)
			}
			if fwStart.Before(starts[i]) || fwEnd.After(finishes[i]) {
				return nil, fmt.Errorf("focus_windows interval is outside step envelope for step %s", st.Slug)
			}
			w := stepWindow{StepSlug: st.Slug, Start: fwStart, End: fwEnd, Last: i == last}
			windows = append(windows, w)
		}
	}
	sort.SliceStable(windows, func(i, j int) bool {
		if windows[i].Start.Equal(windows[j].Start) {
			if windows[i].End.Equal(windows[j].End) {
				return windows[i].StepSlug < windows[j].StepSlug
			}
			return windows[i].End.Before(windows[j].End)
		}
		return windows[i].Start.Before(windows[j].Start)
	})
	return windows, nil
}

func findStepWindowIndex(captured time.Time, windows []stepWindow) int {
	captured = captured.Truncate(time.Second)
	for i, w := range windows {
		if (captured.Equal(w.Start) || captured.After(w.Start)) && (captured.Equal(w.End) || captured.Before(w.End)) {
			return i
		}
	}
	return -1
}

func listSessionSlugs(root string) ([]string, error) {
	sessionRoot := filepath.Join(root, "session")
	entries, err := os.ReadDir(sessionRoot)
	if err != nil {
		return nil, err
	}
	out := []string{}
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out, nil
}

func exifCaptureTime(path string) (time.Time, error) {
	if _, err := exec.LookPath("exiftool"); err == nil {
		if t, err := exifToolCaptureTimeFromReader(func(tag string) (string, error) {
			cmd := exec.Command("exiftool", "-s3", "-"+tag, path)
			out, err := cmd.Output()
			if err != nil {
				return "", err
			}
			return strings.TrimSpace(string(out)), nil
		}); err == nil {
			return t, nil
		}
	}
	f, err := os.Open(path)
	if err != nil {
		return time.Time{}, err
	}
	defer f.Close()
	x, err := exif.Decode(f)
	if err != nil {
		return time.Time{}, err
	}
	tm, err := x.DateTime()
	if err != nil {
		return time.Time{}, err
	}
	return tm.In(time.Local), nil
}

func exifToolCaptureTimeFromReader(readTag func(tag string) (string, error)) (time.Time, error) {
	for _, tag := range []string{
		"SubSecDateTimeOriginal",
		"DateTimeOriginal",
		"CreationDate",
		"CreateDate",
		"MediaCreateDate",
		"TrackCreateDate",
		"SubSecCreateDate",
	} {
		v, err := readTag(tag)
		if err != nil || v == "" {
			continue
		}
		if t, parseErr := parseExifToolTimestamp(v); parseErr == nil {
			return t, nil
		}
	}
	return time.Time{}, errors.New("no exiftool capture timestamp found")
}

func parseExifToolTimestamp(v string) (time.Time, error) {
	layouts := []string{
		"2006:01:02 15:04:05.999999999-07:00",
		"2006:01:02 15:04:05.999999999",
		"2006:01:02 15:04:05-07:00",
		"2006:01:02 15:04:05",
	}
	var lastErr error
	for _, layout := range layouts {
		var (
			t   time.Time
			err error
		)
		if strings.Contains(layout, "-07:00") {
			t, err = time.Parse(layout, v)
		} else {
			t, err = time.ParseInLocation(layout, v, time.Local)
		}
		if err == nil {
			return t.In(time.Local), nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = errors.New("unsupported EXIF timestamp format")
	}
	return time.Time{}, lastErr
}

func fileModTime(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

func collectSessionAssetHashes(sessionDir string) (map[string]bool, error) {
	hashes := map[string]bool{}
	err := filepath.WalkDir(sessionDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if !strings.Contains(path, string(filepath.Separator)+"asset"+string(filepath.Separator)) {
			return nil
		}
		h, hashErr := fileSHA8(path)
		if hashErr != nil {
			return hashErr
		}
		hashes[h] = true
		return nil
	})
	if err != nil {
		return nil, err
	}
	return hashes, nil
}

func fileSHA8(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	sum := fmt.Sprintf("%x", h.Sum(nil))
	return sum[:8], nil
}

func setFrontmatterField(path, key, value string) error {
	fm, body, err := util.ReadFrontmatterFile(path)
	if err != nil {
		return err
	}
	fm[key] = value
	return util.WriteFrontmatterFile(path, fm, body)
}

func lastToken(s string) string {
	parts := strings.Fields(strings.TrimSpace(s))
	if len(parts) == 0 {
		return "subject"
	}
	return parts[len(parts)-1]
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}

func asMap(v any) (map[string]any, bool) {
	m, ok := v.(map[string]any)
	return m, ok
}

func asInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int8:
		return int(n), true
	case int16:
		return int(n), true
	case int32:
		return int(n), true
	case int64:
		return int(n), true
	case uint:
		return int(n), true
	case uint8:
		return int(n), true
	case uint16:
		return int(n), true
	case uint32:
		return int(n), true
	case uint64:
		return int(n), true
	case float64:
		if n == math.Trunc(n) {
			return int(n), true
		}
	}
	return 0, false
}

func asIntSlice(v any) ([]int, bool) {
	switch vals := v.(type) {
	case []int:
		out := make([]int, 0, len(vals))
		out = append(out, vals...)
		return out, true
	case []any:
		out := make([]int, 0, len(vals))
		for _, raw := range vals {
			n, ok := asInt(raw)
			if !ok {
				return nil, false
			}
			out = append(out, n)
		}
		return out, true
	default:
		return nil, false
	}
}

func escapeHTML(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return r.Replace(s)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := util.EnsureDir(filepath.Dir(dst)); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func writePDF(path, title, body string, incomplete bool) error {
	p := fpdf.New("P", "mm", "A4", "")
	p.AddPage()
	p.SetFont("Arial", "B", 16)
	titleLine := title
	if incomplete {
		titleLine += " (WIP)"
	}
	p.CellFormat(0, 10, titleLine, "", 1, "L", false, 0, "")
	p.SetFont("Arial", "", 11)
	p.Ln(2)
	p.MultiCell(0, 5, body, "", "L", false)
	return p.OutputFileAndClose(path)
}
