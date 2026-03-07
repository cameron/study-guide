package cli

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/table"
	"github.com/go-pdf/fpdf"
	"github.com/rwcarlsen/goexif/exif"
	"gopkg.in/yaml.v3"

	"study-guide/src/internal/store"
	"study-guide/src/internal/util"
)

var (
	cmdInitRunner     = cmdInit
	cmdSessionsRunner = cmdSessions
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
	case "subject":
		err = cmdSubject(args[1:])
	case "session":
		err = cmdSession(args[1:])
	case "sessions":
		err = cmdSessionsWithArgs(args[1:])
	case "status":
		err = cmdStatus(true)
	case "publish":
		err = cmdPublish()
	case "data":
		err = cmdData(args[1:])
	case "rm-assets":
		err = cmdRmAssets(args[1:])
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
  subject create|edit|search|print|ls|rm
  session [advance|reverse [--session <slug>]]
  sessions [print]
  data ingest [--assets-dir <path>]
  data ls
  status
  publish
  rm-assets`)
}

func cmdInit() error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	vals, canceled, err := runForm("Initialize Study", []formField{
		{Name: "study_name", Label: "Study Name", Required: true},
		{Name: "protocol_outline", Label: "Protocol Steps (one per line, optional: name | description)", Required: false},
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
		return errors.New("study name is required")
	}
	outlineSteps := parseOutlineSteps(vals["protocol_outline"])
	if len(outlineSteps) == 0 {
		outlineSteps = []protocolOutlineStep{{Name: "First Step"}}
	}
	if err := ensureStudyFile(filepath.Join(cwd, "study.sg.md"), studyName); err != nil {
		return err
	}
	if err := ensureProtocolFile(filepath.Join(cwd, "protocol.sg.md"), outlineSteps); err != nil {
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

type protocolOutlineStep struct {
	Name        string
	Description string
}

func parseOutlineSteps(raw string) []protocolOutlineStep {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	lines := strings.Split(raw, "\n")
	steps := make([]protocolOutlineStep, 0, len(lines))
	for _, line := range lines {
		for _, part := range strings.Split(line, ",") {
			step, ok := parseOutlineStep(part)
			if ok {
				steps = append(steps, step)
			}
		}
	}
	return steps
}

func parseOutlineStep(raw string) (protocolOutlineStep, bool) {
	part := strings.TrimSpace(raw)
	if part == "" {
		return protocolOutlineStep{}, false
	}
	name := part
	desc := ""
	if strings.Contains(part, "|") {
		p := strings.SplitN(part, "|", 2)
		name = strings.TrimSpace(p[0])
		desc = strings.TrimSpace(p[1])
	}
	if name == "" {
		return protocolOutlineStep{}, false
	}
	return protocolOutlineStep{Name: name, Description: desc}, true
}

func ensureFile(path, content string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func ensureStudyFile(path, studyName string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	fm := map[string]any{
		"status":     "WIP",
		"created_on": util.NowTimestamp(),
	}
	body := fmt.Sprintf("# %s\n\n# Hypotheses\n\n# Discussion\n\n# Conclusion\n\n# Special Thanks\n", studyName)
	return util.WriteFrontmatterFile(path, fm, body)
}

func ensureProtocolFile(path string, steps []protocolOutlineStep) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	var b strings.Builder
	b.WriteString("# Protocol Summary\n\nDescribe the protocol.\n\n# Steps\n\n")
	for _, s := range steps {
		if strings.TrimSpace(s.Name) == "" {
			continue
		}
		b.WriteString("## ")
		b.WriteString(s.Name)
		b.WriteString("\n\n")
		if strings.TrimSpace(s.Description) != "" {
			b.WriteString(s.Description)
			b.WriteString("\n\n")
		}
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
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
		if len(args) < 2 {
			return errors.New("usage: sg subject print <id-or-name>")
		}
		return subjectPrint(strings.Join(args[1:], " "))
	case "rm":
		if len(args) < 2 {
			return errors.New("usage: sg subject rm <id>")
		}
		return store.RemoveSubject(args[1])
	default:
		return fmt.Errorf("unknown subject subcommand: %s", args[0])
	}
}

func subjectCreate() error {
	return subjectCreateWithStudyRoot("")
}

func subjectCreateWithStudyRoot(studyRoot string) error {
	req, err := subjectCreateRequirements(studyRoot)
	if err != nil {
		return err
	}
	fields := subjectCreateFormFieldsFromRequirements(req)
	vals, canceled, err := runForm("Create Subject", fields)
	if err != nil {
		return err
	}
	if canceled {
		fmt.Println("canceled")
		return nil
	}
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
	s := store.Subject{
		Name:  vals["name"],
		Type:  subjectType,
		Email: vals["email"],
		Phone: vals["phone"],
		Age:   vals["age"],
		Sex:   vals["sex"],
		Notes: vals["notes"],
		Extra: extra,
	}
	path, err := store.SaveSubject(s)
	if err != nil {
		return err
	}
	fmt.Println("created", path)
	return nil
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

func subjectPrint(q string) error {
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
	_, sessionDir, err := createSessionScaffold(root, selected)
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
		return errors.New("usage: sg data ingest [--assets-dir <path>] | ls")
	}
	switch args[0] {
	case "ingest":
		return cmdIngestPhotos(args[1:])
	case "ls":
		if len(args) > 1 {
			return fmt.Errorf("unknown argument: %s", args[1])
		}
		return cmdDataLs()
	default:
		return fmt.Errorf("unknown data subcommand: %s", args[0])
	}
}

func cmdDataLs() error {
	root, err := util.StudyRootFromCwd()
	if err != nil {
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
			StepCount:      len(protocol.Steps),
		}
		progress, err := inspectSessionProgress(filepath.Join(root, "session", slug), protocol)
		if err != nil {
			rec.NextAction = "invalid"
			rec.InvalidReason = err.Error()
		} else {
			rec.ProgressSteps = progress.ProgressSteps
			if progress.ActiveStepIdx >= 0 && progress.ActiveStepIdx < len(protocol.Steps) {
				rec.CurrentStep = protocol.Steps[progress.ActiveStepIdx].Name
				rec.CurrentStepIdx = progress.ActiveStepIdx
			} else if progress.ProgressSteps > 0 {
				// When no step is currently active, show the last progressed step.
				lastProgressed := progress.ProgressSteps - 1
				if lastProgressed >= len(protocol.Steps) {
					lastProgressed = len(protocol.Steps) - 1
				}
				if lastProgressed >= 0 {
					rec.CurrentStep = protocol.Steps[lastProgressed].Name
					rec.CurrentStepIdx = lastProgressed
				}
			}
			rec.NextAction = progress.NextAction
			switch progress.NextAction {
			case "start":
				target := progress.FirstUnstarted
				if target < 0 {
					target = 0
				}
				if target >= 0 && target < len(protocol.Steps) {
					rec.NextStep = protocol.Steps[target].Name
				}
			case "advance":
				target := progress.ActiveStepIdx + 1
				if target >= 0 && target < len(protocol.Steps) {
					rec.NextStep = protocol.Steps[target].Name
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
	FirstUnstarted  int
	ProgressSteps   int
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
		FirstUnstarted:  -1,
		ProgressSteps:   0,
		SessionFinished: false,
	}
	startedFlags := make([]bool, len(protocol.Steps))
	finishedFlags := make([]bool, len(protocol.Steps))
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
			p.ProgressSteps++
			continue
		}
		p.ProgressSteps++
	}
	lastIdx := len(protocol.Steps) - 1
	p.SessionFinished = p.FirstUnstarted == -1 && p.ActiveStepIdx == -1 && p.AnyStepStarted
	switch {
	case p.SessionFinished:
		p.NextAction = "none"
	case p.ActiveStepIdx >= 0 && p.ActiveStepIdx < lastIdx:
		p.NextAction = "advance"
	case p.ActiveStepIdx == lastIdx:
		p.NextAction = "finish"
	case !p.AnyStepStarted:
		p.NextAction = "start"
	case p.FirstUnstarted >= 0:
		p.NextAction = "start"
	default:
		p.NextAction = "finish"
	}
	return p, nil
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
		if err := startSessionStep(sessionDir, protocol.Steps[target], now); err != nil {
			return sessionAdvanceResult{}, err
		}
			if isFocused {
				if err := syncFocusedSessionWindows(root, sessionSlug, protocol, now); err != nil {
					return sessionAdvanceResult{}, err
				}
			}
			return sessionAdvanceResult{State: "started", StepSlug: protocol.Steps[target].Slug}, nil
	case "advance":
		if progress.ActiveStepIdx < 0 || progress.ActiveStepIdx >= len(protocol.Steps)-1 {
			return sessionAdvanceResult{}, errors.New("cannot advance: no active step")
		}
		active := protocol.Steps[progress.ActiveStepIdx]
		if err := finishSessionStep(sessionDir, active.Slug, now); err != nil {
			return sessionAdvanceResult{}, err
		}
		next := protocol.Steps[progress.ActiveStepIdx+1]
		if err := startSessionStep(sessionDir, next, now); err != nil {
			return sessionAdvanceResult{}, err
		}
			if isFocused {
				if err := syncFocusedSessionWindows(root, sessionSlug, protocol, now); err != nil {
					return sessionAdvanceResult{}, err
				}
			}
			return sessionAdvanceResult{State: "advanced", StepSlug: next.Slug}, nil
	case "finish":
		if progress.ActiveStepIdx >= 0 {
			active := protocol.Steps[progress.ActiveStepIdx]
			if err := finishSessionStep(sessionDir, active.Slug, now); err != nil {
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

func createSessionScaffold(root string, selected []store.Subject) (string, string, error) {
	if len(selected) == 0 {
		return "", "", errors.New("select at least one subject")
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
	for _, sec := range []string{"# Hypotheses", "# Discussion", "# Conclusion"} {
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

func cmdPublish() error {
	root, err := util.StudyRootFromCwd()
	if err != nil {
		return err
	}
	return cmdPublishAtRoot(root)
}

func cmdPublishAtRoot(root string) error {
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
	protocol, _ := store.ParseProtocol(root)
	title := store.ExtractStudyTitle(studyBody)
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

	sessions, err := loadSessionsForPublish(root, protocol, subjectByID)
	if err != nil {
		return err
	}

	outDir := filepath.Join(root, "publish")
	siteDir := filepath.Join(outDir, "site")
	if err := util.EnsureDir(siteDir); err != nil {
		return err
	}
	if err := util.EnsureDir(filepath.Join(siteDir, "assets")); err != nil {
		return err
	}

	html := renderPublishHTML(root, title, studyFM, studyBody, protocol, sessions, incomplete)
	if err := os.WriteFile(filepath.Join(siteDir, "index.html"), []byte(html), 0o644); err != nil {
		return err
	}

	pdfBody := renderPublishText(title, studyFM, studyBody, protocol, sessions)
	if err := writePDF(filepath.Join(outDir, "study.pdf"), title, pdfBody, incomplete); err != nil {
		return err
	}
	fmt.Printf("published to %s\n", outDir)
	return nil
}

type publishStep struct {
	Name      string
	Slug      string
	Started   string
	Finished  string
	ImageRefs []string
}

type publishSession struct {
	Slug       string
	Started    string
	Finished   string
	SubjectIDs []string
	Subjects   []string
	Steps      []publishStep
}

func loadSessionsForPublish(root string, protocol store.Protocol, subjectByID map[string]store.Subject) ([]publishSession, error) {
	sessionRoot := filepath.Join(root, "session")
	entries, err := os.ReadDir(sessionRoot)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	sessions := make([]publishSession, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		slug := e.Name()
		sdir := filepath.Join(sessionRoot, slug)
		_, body, err := util.ReadFrontmatterFile(filepath.Join(sdir, "session.sg.md"))
		if err != nil {
			continue
		}
		refs := parseSessionSubjects(body)
		ids := make([]string, 0, len(refs))
		names := make([]string, 0, len(ids))
		for _, ref := range refs {
			if strings.TrimSpace(ref.ID) != "" {
				ids = append(ids, ref.ID)
			}
			name := ref.Name
			if s, ok := subjectByID[ref.ID]; ok {
				name = s.Name
			}
			if strings.TrimSpace(name) != "" {
				names = append(names, name)
			}
		}
		steps := make([]publishStep, 0, len(protocol.Steps))
		for _, st := range protocol.Steps {
			stepPath := filepath.Join(sdir, "step", st.Slug, "step.sg.md")
			stepFM, _, err := util.ReadFrontmatterFile(stepPath)
			if err != nil {
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
			steps = append(steps, publishStep{
				Name:      st.Name,
				Slug:      st.Slug,
				Started:   asString(stepFM["time_started"]),
				Finished:  asString(stepFM["time_finished"]),
				ImageRefs: imgRefs,
			})
		}
			started := ""
			finished := ""
			for _, step := range steps {
				if strings.TrimSpace(step.Started) != "" {
					if started == "" {
						started = step.Started
					} else if st, errSt := util.ParseTimestamp(step.Started); errSt == nil {
						if cur, errCur := util.ParseTimestamp(started); errCur == nil && st.Before(cur) {
							started = step.Started
						}
					}
				}
				if strings.TrimSpace(step.Finished) != "" {
					if finished == "" {
						finished = step.Finished
					} else if st, errSt := util.ParseTimestamp(step.Finished); errSt == nil {
						if cur, errCur := util.ParseTimestamp(finished); errCur == nil && st.After(cur) {
							finished = step.Finished
						}
					}
				}
			}
			sessions = append(sessions, publishSession{
				Slug:       slug,
				Started:    started,
				Finished:   finished,
				SubjectIDs: ids,
				Subjects:   names,
				Steps:      steps,
		})
	}
	sort.Slice(sessions, func(i, j int) bool {
		ti, ei := util.ParseTimestamp(sessions[i].Started)
		tj, ej := util.ParseTimestamp(sessions[j].Started)
		if ei == nil && ej == nil {
			return ti.Before(tj)
		}
		return sessions[i].Slug < sessions[j].Slug
	})
	return sessions, nil
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

func renderPublishHTML(root, title string, studyFM map[string]any, studyBody string, protocol store.Protocol, sessions []publishSession, incomplete bool) string {
	studyMeta := fmt.Sprintf("<p>Status: %s<br/>Created: %s</p>", escapeHTML(asString(studyFM["status"])), escapeHTML(asString(studyFM["created_on"])))
	wip := ""
	if incomplete {
		wip = `<div style="padding:8px;background:#ffe9e9;border:1px solid #d66"><strong>WIP</strong> Incomplete study data</div>`
	}
	protocolSteps := ""
	for _, step := range protocol.Steps {
		protocolSteps += "<li>" + escapeHTML(step.Name) + "</li>"
	}
	if protocolSteps == "" {
		protocolSteps = "<li>No protocol steps found</li>"
	}

	assetOut := filepath.Join(root, "publish", "site", "assets")
	sessionHTML := ""
	for _, s := range sessions {
		sessionHTML += "<section><h3>Session " + escapeHTML(s.Slug) + "</h3>"
		sessionHTML += "<p>Started: " + escapeHTML(s.Started) + "<br/>Finished: " + escapeHTML(s.Finished) + "</p>"
		sessionHTML += "<p>Subjects: " + escapeHTML(strings.Join(s.Subjects, ", ")) + "</p>"
		sessionHTML += "<ul>"
		for _, st := range s.Steps {
			sessionHTML += "<li><strong>" + escapeHTML(st.Name) + "</strong> [" + escapeHTML(st.Started) + " - " + escapeHTML(st.Finished) + "]"
			if len(st.ImageRefs) > 0 {
				sessionHTML += "<div>"
				for _, img := range st.ImageRefs {
					relDest := filepath.Join("assets", s.Slug, st.Slug, filepath.Base(img))
					dest := filepath.Join(assetOut, s.Slug, st.Slug, filepath.Base(img))
					_ = util.EnsureDir(filepath.Dir(dest))
					_ = copyFile(img, dest)
					sessionHTML += `<img src="` + escapeHTML(relDest) + `" style="max-width:220px;max-height:220px;margin:4px"/>`
				}
				sessionHTML += "</div>"
			}
			sessionHTML += "</li>"
		}
		sessionHTML += "</ul></section>"
	}
	if sessionHTML == "" {
		sessionHTML = "<p>No sessions.</p>"
	}

	return "<html><body>" + wip + "<h1>" + escapeHTML(title) + "</h1>" +
		studyMeta +
		"<h2>Hypotheses</h2><pre>" + escapeHTML(extractSection(studyBody, "Hypotheses")) + "</pre>" +
		"<h2>Discussion</h2><pre>" + escapeHTML(extractSection(studyBody, "Discussion")) + "</pre>" +
		"<h2>Conclusion</h2><pre>" + escapeHTML(extractSection(studyBody, "Conclusion")) + "</pre>" +
		"<h2>Protocol Summary</h2><pre>" + escapeHTML(protocol.Summary) + "</pre>" +
		"<h2>Protocol Steps</h2><ol>" + protocolSteps + "</ol>" +
		"<h2>Sessions</h2>" + sessionHTML +
		"</body></html>"
}

func renderPublishText(title string, studyFM map[string]any, studyBody string, protocol store.Protocol, sessions []publishSession) string {
	var b strings.Builder
	b.WriteString("Study: ")
	b.WriteString(title)
	b.WriteString("\nStatus: ")
	b.WriteString(asString(studyFM["status"]))
	b.WriteString("\nCreated: ")
	b.WriteString(asString(studyFM["created_on"]))
	b.WriteString("\n\nHypotheses\n")
	b.WriteString(extractSection(studyBody, "Hypotheses"))
	b.WriteString("\n\nDiscussion\n")
	b.WriteString(extractSection(studyBody, "Discussion"))
	b.WriteString("\n\nConclusion\n")
	b.WriteString(extractSection(studyBody, "Conclusion"))
	b.WriteString("\n\nProtocol Summary\n")
	b.WriteString(protocol.Summary)
	b.WriteString("\n\nProtocol Steps\n")
	for _, step := range protocol.Steps {
		b.WriteString("- ")
		b.WriteString(step.Name)
		b.WriteString("\n")
	}
	b.WriteString("\nSessions\n")
	for _, s := range sessions {
		b.WriteString("- ")
		b.WriteString(s.Slug)
		b.WriteString(" | subjects: ")
		b.WriteString(strings.Join(s.Subjects, ", "))
		b.WriteString("\n")
		for _, st := range s.Steps {
			b.WriteString("  * ")
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
	return b.String()
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
			return fmt.Errorf("session %s: %w", session, err)
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

	var exported []string
	if opts.AssetsDir != "" {
		exported, err = collectImageFiles(opts.AssetsDir)
		if err != nil {
			return err
		}
		if len(exported) == 0 {
			fmt.Println("no photos found in assets dir", opts.AssetsDir)
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
		if hasSQLite {
			fmt.Printf("scanning Photos Library assets from %s using SQLite window %s to %s\n", dbPath, globalStart.Format(time.RFC3339), globalEnd.Format(time.RFC3339))
			exported, err = collectPhotosLibraryImageFilesBySQLite(dbPath, originalsRoot, globalStart, globalEnd, photosLibraryMTimeSkew)
			if err != nil {
				return err
			}
			if len(exported) == 0 {
				fmt.Println("no photos found in Photos Library SQLite metadata window", dbPath)
				return nil
			}
		} else {
			fmt.Printf("scanning Photos Library files from %s (mtime envelope %s to %s)\n", sourceDir, globalStart.Format(time.RFC3339), globalEnd.Format(time.RFC3339))
			exported, err = collectImageFilesByMTime(sourceDir, globalStart, globalEnd, photosLibraryMTimeSkew)
			if err != nil {
				return err
			}
			if len(exported) == 0 {
				fmt.Println("no photos found in Photos Library source directory within mtime envelope", sourceDir)
				return nil
			}
		}
	}

	total := ingestStats{}
	capturedAssets, err := buildCapturedAssets(exported, exifCaptureTimeFn)
	if err != nil {
		return err
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
	fmt.Printf("ingest complete: sessions=%d copied=%d skipped_duplicate=%d skipped_no_exif=%d skipped_outside_windows=%d\n", len(sessions), total.Copied, total.SkippedDup, total.SkippedNoEXIF, total.SkippedWindow)
	return nil
}

func cmdRmAssets(args []string) error {
	if len(args) > 0 {
		return errors.New("usage: sg rm-assets")
	}
	root, err := util.StudyRootFromCwd()
	if err != nil {
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
		case ".jpg", ".jpeg", ".png", ".heic", ".tif", ".tiff":
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

func collectPhotosLibraryImageFilesBySQLite(dbPath, originalsRoot string, start, end time.Time, skew time.Duration) ([]string, error) {
	minTime := start.Add(-skew)
	maxTime := end.Add(skew)
	minUnix := minTime.Unix()
	maxUnix := maxTime.Unix()
	minExif := minTime.Local().Format("2006:01:02 15:04:05")
	maxExif := maxTime.Local().Format("2006:01:02 15:04:05")
	query := fmt.Sprintf(`
SELECT DISTINCT
  COALESCE(a.ZDIRECTORY, ''),
  a.ZFILENAME
FROM ZASSET a
LEFT JOIN ZADDITIONALASSETATTRIBUTES aa ON aa.ZASSET = a.Z_PK
WHERE a.ZFILENAME IS NOT NULL
  AND (
    lower(a.ZFILENAME) LIKE '%%.jpg'
    OR lower(a.ZFILENAME) LIKE '%%.jpeg'
    OR lower(a.ZFILENAME) LIKE '%%.png'
    OR lower(a.ZFILENAME) LIKE '%%.heic'
    OR lower(a.ZFILENAME) LIKE '%%.tif'
    OR lower(a.ZFILENAME) LIKE '%%.tiff'
  )
  AND (
    (a.ZDATECREATED IS NOT NULL AND (a.ZDATECREATED + strftime('%%s','2001-01-01 00:00:00')) BETWEEN %d AND %d)
    OR (aa.ZEXIFTIMESTAMPSTRING IS NOT NULL AND aa.ZEXIFTIMESTAMPSTRING >= '%s' AND aa.ZEXIFTIMESTAMPSTRING <= '%s')
  );`, minUnix, maxUnix, minExif, maxExif)
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
		if name == "" || !isSupportedImageExt(name) {
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

func isSupportedImageExt(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg", ".png", ".heic", ".tif", ".tiff":
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
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".jpg", ".jpeg", ".png", ".heic", ".tif", ".tiff":
		default:
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
	return out, nil
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
				warnf("warning: skipped (missing EXIF capture time): %s\n", asset.path)
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
	return stats, nil
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
			return configured, nil
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
	PhotosLibraryPath       string `yaml:"photos_library_path"`
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
	starts := make([]time.Time, len(protocol.Steps))
	finishes := make([]time.Time, len(protocol.Steps))
	hasExplicitFinish := make([]bool, len(protocol.Steps))
	stepFocusWindows := make([][]focusWindow, len(protocol.Steps))
	for i, st := range protocol.Steps {
		stepPath := filepath.Join(sessionDir, "step", st.Slug, "step.sg.md")
		fm, _, err := util.ReadFrontmatterFile(stepPath)
		if err != nil {
			return nil, fmt.Errorf("missing step file: %s", stepPath)
		}
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
	last := len(protocol.Steps) - 1
	if finishes[last].IsZero() {
		return nil, errors.New("last step is missing time_finished")
	}
	for i := 0; i < last; i++ {
		nextStart := starts[i+1]
		if !hasExplicitFinish[i] || !finishes[i].Before(nextStart) {
			finishes[i] = nextStart.Add(-1 * time.Second)
		}
	}

	windows := make([]stepWindow, 0, len(protocol.Steps))
	for i, st := range protocol.Steps {
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
		cmd := exec.Command("exiftool", "-s3", "-DateTimeOriginal", "-d", "%Y-%m-%d %H:%M:%S", path)
		out, err := cmd.Output()
		if err == nil {
			v := strings.TrimSpace(string(out))
			if v != "" {
				if t, parseErr := time.ParseInLocation("2006-01-02 15:04:05", v, time.Local); parseErr == nil {
					return t, nil
				}
			}
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
	p.CellFormat(0, 10, title, "", 1, "L", false, 0, "")
	p.SetFont("Arial", "", 11)
	if incomplete {
		p.SetTextColor(170, 40, 40)
		p.CellFormat(0, 8, "WIP", "", 1, "L", false, 0, "")
		p.SetTextColor(0, 0, 0)
	}
	p.Ln(2)
	p.MultiCell(0, 5, body, "", "L", false)
	return p.OutputFileAndClose(path)
}
