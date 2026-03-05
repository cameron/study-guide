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

	"github.com/go-pdf/fpdf"
	"github.com/rwcarlsen/goexif/exif"

	"study-guide/src/internal/store"
	"study-guide/src/internal/util"
)

func Run(args []string) int {
	if len(args) == 0 {
		printHelp()
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
		err = cmdSessions()
	case "status":
		err = cmdStatus(true)
	case "publish":
		err = cmdPublish()
	case "ingest-photos":
		err = cmdIngestPhotos(args[1:])
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

func printHelp() {
	fmt.Println(`sg - Study Guide CLI

Commands:
  init
  subject create|edit|search|print|ls|rm
  session [advance [--session <slug>]]
  sessions
  status
  publish
  ingest-photos`)
}

func cmdInit() error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	vals, canceled, err := runForm("Initialize Study", []formField{
		{Name: "study_name", Label: "Study Name", Required: true},
		{Name: "protocol_outline", Label: "Protocol Steps (comma-separated)", Required: false},
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
		outlineSteps = []string{"First Step"}
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

func parseOutlineSteps(raw string) []string {
	parts := strings.Split(raw, ",")
	steps := make([]string, 0, len(parts))
	for _, p := range parts {
		step := strings.TrimSpace(p)
		if step != "" {
			steps = append(steps, step)
		}
	}
	return steps
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

func ensureProtocolFile(path string, steps []string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	var b strings.Builder
	b.WriteString("# Protocol Summary\n\nDescribe the protocol.\n\n# Steps\n\n")
	for _, s := range steps {
		b.WriteString("## ")
		b.WriteString(s)
		b.WriteString("\n\n")
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
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
	requiredMap := map[string]bool{"name": true}
	if root, err := util.StudyRootFromCwd(); err == nil {
		fields, err := store.ReadRequiredSubjectFields(root)
		if err == nil {
			for _, f := range fields {
				requiredMap[f] = true
			}
		}
	}
	fields := []formField{
		{Name: "name", Label: "Name", Required: requiredMap["name"]},
		{Name: "email", Label: "Email", Required: requiredMap["email"]},
		{Name: "phone", Label: "Phone", Required: requiredMap["phone"]},
		{Name: "age", Label: "Age", Required: requiredMap["age"]},
		{Name: "sex", Label: "Sex", Required: requiredMap["sex"]},
		{Name: "notes", Label: "Notes", Required: false},
	}
	vals, canceled, err := runForm("Create Subject", fields)
	if err != nil {
		return err
	}
	if canceled {
		fmt.Println("canceled")
		return nil
	}
	s := store.Subject{
		Name:  vals["name"],
		Type:  "person",
		Email: vals["email"],
		Phone: vals["phone"],
		Age:   vals["age"],
		Sex:   vals["sex"],
		Notes: vals["notes"],
	}
	path, err := store.SaveSubject(s)
	if err != nil {
		return err
	}
	fmt.Println("created", path)
	return nil
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
		default:
			return fmt.Errorf("unknown session subcommand: %s", args[0])
		}
	}

	root, err := util.StudyRootFromCwd()
	if err != nil {
		return err
	}
	selected, err := selectSubjectsForSession()
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
	sessionPath := filepath.Join(sessionDir, "session.sg.md")

	var prevStepPath string
	for i, step := range protocol.Steps {
		if prevStepPath != "" {
			if _, canceled, err := runSelect("Advance to next step", []string{"Continue", "Cancel"}); err != nil {
				return err
			} else if canceled {
				return nil
			}
			if err := setFrontmatterField(prevStepPath, "time_finished", util.NowTimestamp()); err != nil {
				return err
			}
		}
		stepDir := filepath.Join(sessionDir, "step", step.Slug)
		if err := util.EnsureDir(filepath.Join(stepDir, "asset")); err != nil {
			return err
		}
		stepPath := filepath.Join(stepDir, "step.sg.md")
		fm := map[string]any{
			"time_started": util.NowTimestamp(),
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
		if err := setFrontmatterField(prevStepPath, "time_finished", util.NowTimestamp()); err != nil {
			return err
		}
	}
	if err := setFrontmatterField(sessionPath, "time_finished", util.NowTimestamp()); err != nil {
		return err
	}
	fmt.Println("session complete:", sessionDir)
	return nil
}

func cmdSessions() error {
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

	sessionSlug, err := parseCurrentSessionAdvanceArgs(args)
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

func parseCurrentSessionAdvanceArgs(args []string) (string, error) {
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
		sfm, _, err := util.ReadFrontmatterFile(sessionPath)
		if err != nil {
			continue
		}
		subjectNames := make([]string, 0)
		for _, id := range parseSubjectIDs(sfm["subject_ids"]) {
			if s, ok := subjectByID[id]; ok {
				subjectNames = append(subjectNames, s.Name)
				continue
			}
			subjectNames = append(subjectNames, id)
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
	sfm, _, err := util.ReadFrontmatterFile(filepath.Join(sessionDir, "session.sg.md"))
	if err != nil {
		return sessionProgress{}, err
	}
	p := sessionProgress{
		ActiveStepIdx:   -1,
		FirstUnstarted:  -1,
		ProgressSteps:   0,
		SessionFinished: strings.TrimSpace(asString(sfm["time_finished"])) != "",
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
	switch {
	case p.SessionFinished && p.FirstUnstarted >= 0:
		p.NextAction = "invalid"
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
	switch progress.NextAction {
	case "start":
		target := 0
		if progress.FirstUnstarted >= 0 {
			target = progress.FirstUnstarted
		}
		if err := startSessionStep(sessionDir, protocol.Steps[target], now); err != nil {
			return sessionAdvanceResult{}, err
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
		return sessionAdvanceResult{State: "advanced", StepSlug: next.Slug}, nil
	case "finish":
		if progress.ActiveStepIdx >= 0 {
			active := protocol.Steps[progress.ActiveStepIdx]
			if err := finishSessionStep(sessionDir, active.Slug, now); err != nil {
				return sessionAdvanceResult{}, err
			}
		}
		if err := setFrontmatterField(sessionPath, "time_finished", now); err != nil {
			return sessionAdvanceResult{}, err
		}
		lastStep := protocol.Steps[len(protocol.Steps)-1]
		return sessionAdvanceResult{State: "finished", StepSlug: lastStep.Slug}, nil
	default:
		return sessionAdvanceResult{}, fmt.Errorf("session cannot transition from current state")
	}
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
	return util.WriteFrontmatterFile(stepPath, fm, body)
}

func createSessionScaffold(root string, selected []store.Subject) (string, string, error) {
	if len(selected) == 0 {
		return "", "", errors.New("select at least one subject")
	}
	today := time.Now().Format("02-01-2006")
	surnames := make([]string, 0, len(selected))
	subjectIDs := make([]string, 0, len(selected))
	for _, s := range selected {
		surnames = append(surnames, util.Slugify(lastToken(s.Name)))
		subjectIDs = append(subjectIDs, s.UUID)
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
	sfm := map[string]any{
		"time_started": util.NowTimestamp(),
		"subject_ids":  subjectIDs,
	}
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

func selectSubjectsForSession() ([]store.Subject, error) {
	selected, canceled, err := runSessionCreatePicker()
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
		sfm, _, err := util.ReadFrontmatterFile(spath)
		if err != nil {
			issues = append(issues, "invalid session file: "+spath)
			continue
		}
		if strings.TrimSpace(asString(sfm["time_started"])) == "" {
			issues = append(issues, "session missing required field time_started: "+slug)
		}
		if !hasNonEmptySubjectIDs(sfm["subject_ids"]) {
			issues = append(issues, "session missing required field subject_ids: "+slug)
		}

		if err == nil {
			for _, step := range protocol.Steps {
				stepPath := filepath.Join(sdir, "step", step.Slug, "step.sg.md")
				stepFM, _, readErr := util.ReadFrontmatterFile(stepPath)
				if readErr != nil {
					issues = append(issues, "session missing step file for protocol step "+step.Slug+": "+slug)
					continue
				}
				if strings.TrimSpace(asString(stepFM["time_started"])) == "" {
					issues = append(issues, "step missing time_started: "+stepPath)
				}
				if strings.TrimSpace(asString(stepFM["time_finished"])) == "" {
					issues = append(issues, "step missing time_finished: "+stepPath)
				}
			}
		}
	}
	sort.Strings(issues)
	return issues, nil
}

func hasNonEmptySubjectIDs(v any) bool {
	switch ids := v.(type) {
	case []any:
		return len(ids) > 0
	case []string:
		return len(ids) > 0
	default:
		return false
	}
}

func cmdPublish() error {
	root, err := util.StudyRootFromCwd()
	if err != nil {
		return err
	}
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
		sfm, _, err := util.ReadFrontmatterFile(filepath.Join(sdir, "session.sg.md"))
		if err != nil {
			continue
		}
		ids := parseSubjectIDs(sfm["subject_ids"])
		names := make([]string, 0, len(ids))
		for _, id := range ids {
			if s, ok := subjectByID[id]; ok {
				names = append(names, s.Name)
			} else {
				names = append(names, id)
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
		sessions = append(sessions, publishSession{
			Slug:       slug,
			Started:    asString(sfm["time_started"]),
			Finished:   asString(sfm["time_finished"]),
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
		return errors.New("sg ingest-photos is supported only on macOS in v1")
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
		tmpDir, err := os.MkdirTemp("", "sg-photos-*")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmpDir)

		exported, err = exportPhotosFromApplePhotosFn(opts.AlbumName, tmpDir)
		if err != nil {
			return err
		}
		if len(exported) == 0 {
			fmt.Println("no photos exported from album", opts.AlbumName)
			return nil
		}
	}

	total := ingestStats{}
	for _, session := range sessions {
		sessionDir := filepath.Join(root, "session", session)
		windows, err := loadStepWindows(sessionDir, protocol)
		if err != nil {
			return fmt.Errorf("session %s: %w", session, err)
		}
		stats, err := ingestPhotosForSession(sessionDir, windows, exported, exifCaptureTimeFn, func(msg string, a ...any) {
			fmt.Printf("session=%s ", session)
			fmt.Printf(msg, a...)
		})
		if err != nil {
			return fmt.Errorf("session %s: %w", session, err)
		}
		total.Copied += stats.Copied
		total.SkippedDup += stats.SkippedDup
		total.SkippedNoEXIF += stats.SkippedNoEXIF
		total.SkippedWindow += stats.SkippedWindow
		fmt.Printf("session %s: copied=%d skipped_duplicate=%d skipped_no_exif=%d skipped_outside_windows=%d\n", session, stats.Copied, stats.SkippedDup, stats.SkippedNoEXIF, stats.SkippedWindow)
	}
	fmt.Printf("ingest complete: sessions=%d copied=%d skipped_duplicate=%d skipped_no_exif=%d skipped_outside_windows=%d\n", len(sessions), total.Copied, total.SkippedDup, total.SkippedNoEXIF, total.SkippedWindow)
	return nil
}

func parseIngestPhotosArgs(args []string) (ingestPhotosOptions, error) {
	opts := ingestPhotosOptions{AlbumName: "SG Ingest"}
	seenAlbum := false
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
			if seenAlbum {
				return opts, errors.New("only one album name positional argument is allowed")
			}
			opts.AlbumName = arg
			seenAlbum = true
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

func ingestPhotosForSession(sessionDir string, windows []stepWindow, sources []string, captureTimeFn func(string) (time.Time, error), warnf func(string, ...any)) (ingestStats, error) {
	stats := ingestStats{}
	existingHashes, err := collectSessionAssetHashes(sessionDir)
	if err != nil {
		return stats, err
	}

	for _, src := range sources {
		captureTime, exifErr := captureTimeFn(src)
		if exifErr != nil {
			stats.SkippedNoEXIF++
			if warnf != nil {
				warnf("warning: skipped (missing EXIF capture time): %s\n", src)
			}
			continue
		}
		idx := findStepWindowIndex(captureTime, windows)
		if idx < 0 {
			stats.SkippedWindow++
			continue
		}
		hash8, err := fileSHA8(src)
		if err != nil {
			return stats, err
		}
		if existingHashes[hash8] {
			stats.SkippedDup++
			continue
		}
		ext := strings.ToLower(filepath.Ext(src))
		if ext == "" {
			ext = ".bin"
		}
		destName := fmt.Sprintf("%s_%s%s", captureTime.Local().Format("20060102-150405"), hash8, ext)
		destDir := filepath.Join(sessionDir, "step", windows[idx].StepSlug, "asset")
		if err := util.EnsureDir(destDir); err != nil {
			return stats, err
		}
		dest := filepath.Join(destDir, destName)
		if _, err := os.Stat(dest); err == nil {
			stats.SkippedDup++
			continue
		}
		if err := copyFile(src, dest); err != nil {
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
	AlbumName string
	AssetsDir string
}

type ingestStats struct {
	Copied        int
	SkippedDup    int
	SkippedNoEXIF int
	SkippedWindow int
}

var exportPhotosFromApplePhotosFn = exportPhotosFromApplePhotos
var exifCaptureTimeFn = exifCaptureTime

func loadStepWindows(sessionDir string, protocol store.Protocol) ([]stepWindow, error) {
	starts := make([]time.Time, len(protocol.Steps))
	finishes := make([]time.Time, len(protocol.Steps))
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
		}
	}
	last := len(protocol.Steps) - 1
	if finishes[last].IsZero() {
		return nil, errors.New("last step is missing time_finished")
	}

	windows := make([]stepWindow, 0, len(protocol.Steps))
	for i, st := range protocol.Steps {
		w := stepWindow{StepSlug: st.Slug, Start: starts[i], Last: i == last}
		if i == last {
			w.End = finishes[i]
		} else {
			w.End = starts[i+1]
		}
		windows = append(windows, w)
	}
	return windows, nil
}

func findStepWindowIndex(captured time.Time, windows []stepWindow) int {
	for i, w := range windows {
		if w.Last {
			if (captured.Equal(w.Start) || captured.After(w.Start)) && (captured.Equal(w.End) || captured.Before(w.End)) {
				return i
			}
			continue
		}
		if (captured.Equal(w.Start) || captured.After(w.Start)) && captured.Before(w.End) {
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

func exportPhotosFromApplePhotos(albumName, outDir string) ([]string, error) {
	script := fmt.Sprintf(`set exportFolder to POSIX file "%s"
tell application "Photos"
	if not (exists album "%s") then
		error "album not found: %s"
	end if
	set mediaItems to media items of album "%s"
	if (count of mediaItems) is 0 then
		return
	end if
	export mediaItems to exportFolder with using originals
end tell`, escapeAppleScriptString(outDir), escapeAppleScriptString(albumName), escapeAppleScriptString(albumName), escapeAppleScriptString(albumName))
	cmd := exec.Command("osascript", "-e", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("failed exporting from Apple Photos: %v (%s)", err, strings.TrimSpace(string(out)))
	}
	var files []string
	_ = filepath.WalkDir(outDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".jpg", ".jpeg", ".png", ".heic", ".tif", ".tiff":
			files = append(files, path)
		}
		return nil
	})
	sort.Strings(files)
	return files, nil
}

func escapeAppleScriptString(s string) string {
	return strings.ReplaceAll(s, `"`, `\\"`)
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
