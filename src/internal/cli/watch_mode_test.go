package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCmdPublish_DefaultsToEntrWatch(t *testing.T) {
	root := filepath.Join(t.TempDir(), "study")
	mustWriteFile(t, filepath.Join(root, "study.sg.md"), injectProtocolIntoStudy("---\nstatus: WIP\ncreated_on: 09:00:00 01-01-2026\n---\n\n# Watch Study\n\n# Introduction\n\n\n# Methods\n\n\n# Results\n\n\n# Discussion\n\n\n# Conclusion\n", "One step.", "Step One"))
	if err := os.MkdirAll(filepath.Join(root, "session"), 0o755); err != nil {
		t.Fatalf("MkdirAll session error: %v", err)
	}

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd error: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("Chdir error: %v", err)
	}
	defer func() { _ = os.Chdir(oldwd) }()

	orig := runEntrWatchFn
	defer func() { runEntrWatchFn = orig }()

	var gotPaths []string
	var gotArgs []string
	runEntrWatchFn = func(paths []string, utilityArgs []string) error {
		gotPaths = append([]string(nil), paths...)
		gotArgs = append([]string(nil), utilityArgs...)
		return nil
	}

	if err := cmdPublish(nil); err != nil {
		t.Fatalf("cmdPublish error: %v", err)
	}

	resolvedRoot, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd error: %v", err)
	}
	wantArgs := []string{"__publish-once-at-root", resolvedRoot, filepath.Join(resolvedRoot, "publish", filepath.Base(resolvedRoot))}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("watch args=%#v want %#v", gotArgs, wantArgs)
	}
	for _, wantPath := range []string{
		resolvedRoot,
		filepath.Join(resolvedRoot, "study.sg.md"),
		filepath.Join(resolvedRoot, "session"),
	} {
		if !containsString(gotPaths, wantPath) {
			t.Fatalf("expected watch paths to contain %q, got %#v", wantPath, gotPaths)
		}
	}
	if containsString(gotPaths, filepath.Join(resolvedRoot, "publish")) {
		t.Fatalf("did not expect watch paths to include publish output tree, got %#v", gotPaths)
	}
}

func TestCmdExport_DefaultsToEntrWatch(t *testing.T) {
	root := filepath.Join(t.TempDir(), "study")
	mustWriteFile(t, filepath.Join(root, "study.sg.md"), injectProtocolIntoStudy("---\nstatus: WIP\ncreated_on: 09:00:00 01-01-2026\n---\n\n# Watch Study\n\n# Introduction\n\n\n# Methods\n\n\n# Results\n\n\n# Discussion\n\n\n# Conclusion\n", "One step.", "Step One"))
	if err := os.MkdirAll(filepath.Join(root, "session"), 0o755); err != nil {
		t.Fatalf("MkdirAll session error: %v", err)
	}

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd error: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("Chdir error: %v", err)
	}
	defer func() { _ = os.Chdir(oldwd) }()

	orig := runEntrWatchFn
	defer func() { runEntrWatchFn = orig }()

	var gotArgs []string
	runEntrWatchFn = func(paths []string, utilityArgs []string) error {
		gotArgs = append([]string(nil), utilityArgs...)
		return nil
	}

	if err := cmdExport(nil); err != nil {
		t.Fatalf("cmdExport error: %v", err)
	}

	resolvedRoot, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd error: %v", err)
	}
	wantArgs := []string{"__export-once-at-root", resolvedRoot, filepath.Join(resolvedRoot, "export", filepath.Base(resolvedRoot)), "144"}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("watch args=%#v want %#v", gotArgs, wantArgs)
	}
}

func TestParsePublishArgs_SupportsOnce(t *testing.T) {
	opts, err := parsePublishArgs([]string{"--once", "--with-subject-names", "out"})
	if err != nil {
		t.Fatalf("parsePublishArgs error: %v", err)
	}
	if !opts.Once {
		t.Fatalf("expected Once to be true")
	}
	if !opts.WithSubjectNames {
		t.Fatalf("expected WithSubjectNames to be true")
	}
	if got := opts.DestinationDir; got != "out" {
		t.Fatalf("DestinationDir=%q want out", got)
	}
}

func TestParseExportArgs_SupportsOnce(t *testing.T) {
	opts, err := parseExportArgs([]string{"--once", "--imgsize=96,240", "out"})
	if err != nil {
		t.Fatalf("parseExportArgs error: %v", err)
	}
	if !opts.Once {
		t.Fatalf("expected Once to be true")
	}
	if got := opts.DestinationDir; got != "out" {
		t.Fatalf("DestinationDir=%q want out", got)
	}
	if !reflect.DeepEqual(opts.ThumbnailSizes, []int{96, 240}) {
		t.Fatalf("ThumbnailSizes=%v want [96 240]", opts.ThumbnailSizes)
	}
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
