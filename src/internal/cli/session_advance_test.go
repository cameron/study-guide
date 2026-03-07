package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"study-guide/src/internal/store"
	"study-guide/src/internal/util"
)

func TestAdvanceSessionOnce_StartsFirstStep(t *testing.T) {
	root := t.TempDir()
	protocol := testProtocol()
	slug := "01-01-2026-alpha"
	mustWriteSessionFile(t, root, slug, map[string]any{
		"subject_ids": []string{"sub-1"},
	})

	res, err := advanceSessionOnce(root, slug, protocol)
	if err != nil {
		t.Fatalf("advanceSessionOnce returned error: %v", err)
	}
	if res.State != "started" || res.StepSlug != "first-step" {
		t.Fatalf("unexpected result: %#v", res)
	}

	stepFM, _, err := util.ReadFrontmatterFile(filepath.Join(root, "session", slug, "step", "first-step", "step.sg.md"))
	if err != nil {
		t.Fatalf("read first step failed: %v", err)
	}
	if asString(stepFM["time_started"]) == "" {
		t.Fatalf("expected first step time_started")
	}
	if asString(stepFM["time_finished"]) != "" {
		t.Fatalf("did not expect first step time_finished on start")
	}
}

func TestAdvanceSessionOnce_AdvancesToNextStep(t *testing.T) {
	root := t.TempDir()
	protocol := testProtocol()
	slug := "01-01-2026-beta"
	mustWriteSessionFile(t, root, slug, map[string]any{
		"time_started": "10:00:00 01-01-2026",
		"subject_ids":  []string{"sub-1"},
	})
	mustWriteStepFile(t, filepath.Join(root, "session", slug, "step", "first-step", "step.sg.md"), map[string]any{
		"time_started": "10:01:00 01-01-2026",
	}, "")

	res, err := advanceSessionOnce(root, slug, protocol)
	if err != nil {
		t.Fatalf("advanceSessionOnce returned error: %v", err)
	}
	if res.State != "advanced" || res.StepSlug != "second-step" {
		t.Fatalf("unexpected result: %#v", res)
	}

	firstFM, _, err := util.ReadFrontmatterFile(filepath.Join(root, "session", slug, "step", "first-step", "step.sg.md"))
	if err != nil {
		t.Fatalf("read first step failed: %v", err)
	}
	if asString(firstFM["time_finished"]) == "" {
		t.Fatalf("expected first step time_finished after advance")
	}
	secondFM, _, err := util.ReadFrontmatterFile(filepath.Join(root, "session", slug, "step", "second-step", "step.sg.md"))
	if err != nil {
		t.Fatalf("read second step failed: %v", err)
	}
	if asString(secondFM["time_started"]) == "" {
		t.Fatalf("expected second step time_started after advance")
	}
}

func TestAdvanceSessionOnce_FinishesSessionAtFinalStep(t *testing.T) {
	root := t.TempDir()
	protocol := testProtocol()
	slug := "01-01-2026-gamma"
	mustWriteSessionFile(t, root, slug, map[string]any{
		"subject_ids": []string{"sub-1"},
	})
	mustWriteStepFile(t, filepath.Join(root, "session", slug, "step", "first-step", "step.sg.md"), map[string]any{
		"time_started":  "10:01:00 01-01-2026",
		"time_finished": "10:05:00 01-01-2026",
	}, "")
	mustWriteStepFile(t, filepath.Join(root, "session", slug, "step", "second-step", "step.sg.md"), map[string]any{
		"time_started": "10:06:00 01-01-2026",
	}, "")

	res, err := advanceSessionOnce(root, slug, protocol)
	if err != nil {
		t.Fatalf("advanceSessionOnce returned error: %v", err)
	}
	if res.State != "finished" || res.StepSlug != "second-step" {
		t.Fatalf("unexpected result: %#v", res)
	}

	secondFM, _, err := util.ReadFrontmatterFile(filepath.Join(root, "session", slug, "step", "second-step", "step.sg.md"))
	if err != nil {
		t.Fatalf("read second step failed: %v", err)
	}
	if asString(secondFM["time_finished"]) == "" {
		t.Fatalf("expected final step time_finished")
	}
	sessionFM, _, err := util.ReadFrontmatterFile(filepath.Join(root, "session", slug, "session.sg.md"))
	if err != nil {
		t.Fatalf("read session failed: %v", err)
	}
	if strings.TrimSpace(asString(sessionFM["time_started"])) != "" || strings.TrimSpace(asString(sessionFM["time_finished"])) != "" {
		t.Fatalf("expected session timing fields to be omitted; got time_started=%q time_finished=%q", asString(sessionFM["time_started"]), asString(sessionFM["time_finished"]))
	}
}

func TestReverseSessionOnce_ClearsCurrentStepAndAllowsRestart(t *testing.T) {
	root := t.TempDir()
	protocol := testProtocol()
	slug := "01-01-2026-theta"
	mustWriteSessionFile(t, root, slug, map[string]any{
		"subject_ids": []string{"sub-1"},
	})
	mustWriteStepFile(t, filepath.Join(root, "session", slug, "step", "first-step", "step.sg.md"), map[string]any{
		"time_started":  "10:00:00 01-01-2026",
		"time_finished": "10:01:00 01-01-2026",
	}, "")
	mustWriteStepFile(t, filepath.Join(root, "session", slug, "step", "second-step", "step.sg.md"), map[string]any{
		"time_started": "10:02:00 01-01-2026",
	}, "")

	res, err := reverseSessionOnce(root, slug, protocol)
	if err != nil {
		t.Fatalf("reverseSessionOnce returned error: %v", err)
	}
	if res.State != "reversed" || res.StepSlug != "second-step" {
		t.Fatalf("unexpected reverse result: %#v", res)
	}

	secondPath := filepath.Join(root, "session", slug, "step", "second-step", "step.sg.md")
	secondFM, _, err := util.ReadFrontmatterFile(secondPath)
	if err != nil {
		t.Fatalf("read second step failed: %v", err)
	}
	if strings.TrimSpace(asString(secondFM["time_started"])) != "" {
		t.Fatalf("expected second step time_started cleared, got %q", asString(secondFM["time_started"]))
	}

	adv, err := advanceSessionOnce(root, slug, protocol)
	if err != nil {
		t.Fatalf("advanceSessionOnce after reverse returned error: %v", err)
	}
	if adv.State != "started" || adv.StepSlug != "second-step" {
		t.Fatalf("unexpected advance result after reverse: %#v", adv)
	}
	secondFM, _, err = util.ReadFrontmatterFile(secondPath)
	if err != nil {
		t.Fatalf("read second step failed: %v", err)
	}
	if strings.TrimSpace(asString(secondFM["time_started"])) == "" {
		t.Fatalf("expected second step to be started again")
	}
}

func TestInferSessionSlugFromCwd(t *testing.T) {
	root := t.TempDir()
	slug := "01-01-2026-delta"
	if err := util.EnsureDir(filepath.Join(root, "session", slug, "step", "first-step")); err != nil {
		t.Fatalf("EnsureDir failed: %v", err)
	}

	got, ok := inferSessionSlugFromCwd(root, filepath.Join(root, "session", slug, "step", "first-step"))
	if !ok || got != slug {
		t.Fatalf("expected inferred slug %q, got %q ok=%v", slug, got, ok)
	}
	if _, ok := inferSessionSlugFromCwd(root, root); ok {
		t.Fatalf("did not expect slug inference at study root")
	}
}

func TestCmdSessionAdvance_InfersSessionFromDirectory(t *testing.T) {
	root := t.TempDir()
	slug := "01-01-2026-epsilon"
	mustWriteFile(t, filepath.Join(root, "study.sg.md"), "---\nstatus: WIP\ncreated_on: 10:00:00 01-01-2026\n---\n\n# Study\n")
	mustWriteFile(t, filepath.Join(root, "protocol.sg.md"), "# Protocol Summary\n\nSummary\n\n# Steps\n\n## First Step\n\n")
	mustWriteSessionFile(t, root, slug, map[string]any{
		"time_started": "10:00:00 01-01-2026",
		"subject_ids":  []string{"sub-1"},
	})

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd error: %v", err)
	}
	if err := os.Chdir(filepath.Join(root, "session", slug)); err != nil {
		t.Fatalf("Chdir error: %v", err)
	}
	defer func() { _ = os.Chdir(oldwd) }()

	if err := cmdSession([]string{"advance"}); err != nil {
		t.Fatalf("cmdSession advance returned error: %v", err)
	}
}

func TestLoadSessionRecords_ToleratesImplicitStepCompletion(t *testing.T) {
	root := t.TempDir()
	protocol := testProtocol()
	slug := "01-01-2026-zeta"
	mustWriteSessionFile(t, root, slug, map[string]any{
		"time_started": "10:00:00 01-01-2026",
		"subject_ids":  []string{"sub-1"},
	})
	mustWriteStepFile(t, filepath.Join(root, "session", slug, "step", "first-step", "step.sg.md"), map[string]any{
		"time_started": "10:01:00 01-01-2026",
	}, "")
	mustWriteStepFile(t, filepath.Join(root, "session", slug, "step", "second-step", "step.sg.md"), map[string]any{
		"time_started": "10:02:00 01-01-2026",
	}, "")

	records, err := loadSessionRecords(root, protocol, map[string]store.Subject{})
	if err != nil {
		t.Fatalf("loadSessionRecords returned error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].NextAction != "finish" {
		t.Fatalf("expected finish next action, got %q", records[0].NextAction)
	}
	if records[0].NextStep != "conclude" {
		t.Fatalf("expected conclude next step label, got %q", records[0].NextStep)
	}
	if records[0].InvalidReason != "" {
		t.Fatalf("did not expect invalid reason, got %q", records[0].InvalidReason)
	}
}

func TestLoadSessionRecords_IgnoresSessionFinishedFlagAndUsesStepProgress(t *testing.T) {
	root := t.TempDir()
	protocol := testProtocol()
	slug := "01-01-2026-eta"
	mustWriteSessionFile(t, root, slug, map[string]any{
		"time_started":  "10:00:00 01-01-2026",
		"time_finished": "10:10:00 01-01-2026",
		"subject_ids":   []string{"sub-1"},
	})
	mustWriteStepFile(t, filepath.Join(root, "session", slug, "step", "first-step", "step.sg.md"), map[string]any{
		"time_started": "10:01:00 01-01-2026",
	}, "")

	records, err := loadSessionRecords(root, protocol, map[string]store.Subject{})
	if err != nil {
		t.Fatalf("loadSessionRecords returned error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].Complete {
		t.Fatalf("expected session to be incomplete")
	}
	if records[0].NextAction != "advance" {
		t.Fatalf("expected derived next action from steps to be advance, got %q", records[0].NextAction)
	}
	if records[0].InvalidReason != "" {
		t.Fatalf("did not expect invalid reason when deriving from steps, got %q", records[0].InvalidReason)
	}
}

func TestRenderEntryRow_ProgressUsesCompletedStepsWhenNoActiveStep(t *testing.T) {
	root := t.TempDir()
	protocol := testProtocolThreeSteps()
	slug := "18-02-2026-boehmer"
	mustWriteSessionFile(t, root, slug, map[string]any{
		"time_started": "23:24:10 18-02-2026",
		"subject_ids":  []string{"sub-1"},
	})
	mustWriteStepFile(t, filepath.Join(root, "session", slug, "step", "first-step", "step.sg.md"), map[string]any{
		"time_started":  "23:24:10 18-02-2026",
		"time_finished": "15:13:05 04-03-2026",
	}, "")
	mustWriteStepFile(t, filepath.Join(root, "session", slug, "step", "second-step", "step.sg.md"), map[string]any{
		"time_started":  "15:13:05 04-03-2026",
		"time_finished": "16:04:15 04-03-2026",
	}, "")

	records, err := loadSessionRecords(root, protocol, map[string]store.Subject{})
	if err != nil {
		t.Fatalf("loadSessionRecords returned error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	model := sessionsSwitchboardModel{protocol: protocol}
	_, _, _, stepText, _ := model.renderEntryRow(browseEntry{kind: browseEntrySession, record: records[0]})
	if !strings.HasPrefix(stepText, "[2/3]") {
		t.Fatalf("expected [2/3] step progress, got %q", stepText)
	}
}

func TestLoadSessionRecords_ShowsLastProgressedStepWhenNoActiveStep(t *testing.T) {
	root := t.TempDir()
	protocol := testProtocolThreeSteps()
	slug := "18-02-2026-boehmer"
	mustWriteSessionFile(t, root, slug, map[string]any{
		"time_started": "23:24:10 18-02-2026",
		"subject_ids":  []string{"sub-1"},
	})
	mustWriteStepFile(t, filepath.Join(root, "session", slug, "step", "first-step", "step.sg.md"), map[string]any{
		"time_started":  "23:24:10 18-02-2026",
		"time_finished": "15:13:05 04-03-2026",
	}, "")
	mustWriteStepFile(t, filepath.Join(root, "session", slug, "step", "second-step", "step.sg.md"), map[string]any{
		"time_started":  "15:13:05 04-03-2026",
		"time_finished": "16:04:15 04-03-2026",
	}, "")
	mustWriteStepFile(t, filepath.Join(root, "session", slug, "step", "third-step", "step.sg.md"), map[string]any{
		"time_started":  "16:04:15 04-03-2026",
		"time_finished": "16:15:15 04-03-2026",
	}, "")

	records, err := loadSessionRecords(root, protocol, map[string]store.Subject{})
	if err != nil {
		t.Fatalf("loadSessionRecords returned error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].CurrentStep != "Third Step" {
		t.Fatalf("expected current step to remain on last progressed step, got %q", records[0].CurrentStep)
	}
	model := sessionsSwitchboardModel{protocol: protocol}
	_, _, _, stepText, _ := model.renderEntryRow(browseEntry{kind: browseEntrySession, record: records[0]})
	if !strings.Contains(stepText, "Third Step") {
		t.Fatalf("expected step text to include Third Step, got %q", stepText)
	}
}

func TestLoadSessionRecords_DerivesSubjectNamesFromSubjectsSection(t *testing.T) {
	root := t.TempDir()
	protocol := testProtocol()
	slug := "01-01-2026-theta"
	path := filepath.Join(root, "session", slug, "session.sg.md")
	if err := util.EnsureDir(filepath.Dir(path)); err != nil {
		t.Fatalf("EnsureDir failed: %v", err)
	}
	if err := util.WriteFrontmatterFile(path, map[string]any{}, "# Subjects\n\n- Alpha Subject (sub-1)\nBeta Subject (sub-2)\n\n# Notes\n"); err != nil {
		t.Fatalf("WriteFrontmatterFile failed: %v", err)
	}

	records, err := loadSessionRecords(root, protocol, map[string]store.Subject{})
	if err != nil {
		t.Fatalf("loadSessionRecords returned error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if len(records[0].SubjectNames) != 2 {
		t.Fatalf("expected 2 derived subject names, got %#v", records[0].SubjectNames)
	}
	if records[0].SubjectNames[0] != "Alpha Subject" || records[0].SubjectNames[1] != "Beta Subject" {
		t.Fatalf("unexpected derived subject names: %#v", records[0].SubjectNames)
	}
}

func TestAdvanceSessionOnce_RejectsNonContiguousProgress(t *testing.T) {
	root := t.TempDir()
	protocol := testProtocolThreeSteps()
	slug := "01-01-2026-noncontiguous"
	mustWriteSessionFile(t, root, slug, map[string]any{
		"subject_ids": []string{"sub-1"},
	})
	// first-step and third-step started while second-step is missing.
	mustWriteStepFile(t, filepath.Join(root, "session", slug, "step", "first-step", "step.sg.md"), map[string]any{
		"time_started":  "10:01:00 01-01-2026",
		"time_finished": "10:02:00 01-01-2026",
	}, "")
	mustWriteStepFile(t, filepath.Join(root, "session", slug, "step", "third-step", "step.sg.md"), map[string]any{
		"time_started": "10:03:00 01-01-2026",
	}, "")

	_, err := advanceSessionOnce(root, slug, protocol)
	if err == nil {
		t.Fatalf("expected error for non-contiguous progress")
	}
	if !strings.Contains(err.Error(), "non-contiguous") {
		t.Fatalf("expected non-contiguous error, got: %v", err)
	}
}

func mustWriteSessionFile(t *testing.T, root, slug string, fm map[string]any) {
	t.Helper()
	path := filepath.Join(root, "session", slug, "session.sg.md")
	if err := util.EnsureDir(filepath.Dir(path)); err != nil {
		t.Fatalf("EnsureDir failed: %v", err)
	}
	if err := util.WriteFrontmatterFile(path, fm, ""); err != nil {
		t.Fatalf("WriteFrontmatterFile failed: %v", err)
	}
}

func testProtocol() store.Protocol {
	return store.Protocol{
		Steps: []store.ProtocolStep{
			{Name: "First Step", Slug: "first-step"},
			{Name: "Second Step", Slug: "second-step"},
		},
	}
}

func testProtocolThreeSteps() store.Protocol {
	return store.Protocol{
		Steps: []store.ProtocolStep{
			{Name: "First Step", Slug: "first-step"},
			{Name: "Second Step", Slug: "second-step"},
			{Name: "Third Step", Slug: "third-step"},
		},
	}
}
