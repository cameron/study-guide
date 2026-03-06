# Study Guide v1 Spec

## Purpose
Build a simple local CLI tool (`sg`) for:
- defining a research study structure
- running session workflows with timed protocol steps
- ingesting microscope photos from Apple Photos into the correct step folders
- publishing the study as a website and PDF

Primary use case: live blood analysis with structured metadata and low-friction photo organization.

## Scope
This spec defines v1 behavior and data contracts.

In scope:
- filesystem-based persistence
- interactive CLI workflows
- one protocol per study
- multi-session progression from a single terminal (parallel participant sessions)
- direct Apple Photos ingestion on macOS
- local filesystem asset ingestion mode for development/testing
- best-effort publish (HTML + PDF)

Implementation references (preferred libraries):
- TUI framework: https://charm.land/bubbletea/v2
- TUI components: https://charm.land/bubbles/v2
- terminal markdown rendering: https://github.com/charmbracelet/glow

## Architecture and Implementation Notes
- Interactive command workflows should run in a single long-lived Bubble Tea program per command invocation.
- Within an interactive command, transitions between screens/actions should be internal model state changes, not nested `tea.NewProgram` launches.
- Bubble Tea models use `View() tea.View` and return text content via `tea.NewView(...)`.

Out of scope for v1:
- full non-interactive coverage for all commands except where explicitly defined
- multi-protocol studies
- cloud sync

## Canonical Persistence Model

### Global subject store
- directory: `~/.study-guide/subject/`
- file format: `.sg.md` only

### Study directory layout
```text
<study-root>/
  study.sg.md
  protocol.sg.md
  subject-requirements.yaml
  session/
    <session-slug>/
      session.sg.md
      step/
        <step-slug>/
          step.sg.md
          asset/
            <ingested-files>
```

Notes:
- v1 has exactly one protocol: `<study-root>/protocol.sg.md`
- sessions are not nested under protocol
- `fixtures/study-eg/` is the canonical sample tree name (not `sample-eg/`)

## Naming, IDs, and Time Format

### IDs and slugs
- `uuid`: UUIDv4 string
- slugs: lowercase kebab-case
- session slug format: `<DD-MM-YYYY>-<subject-surname[-surname...]>`

### Time format
- use local time zone
- timestamp string format for session/step files: `HH:MM:SS DD-MM-YYYY`
- no explicit timezone field in v1

## File Schema Contracts

All `.sg.md` files use YAML frontmatter plus markdown body sections.

## `~/.study-guide/subject/<subject>.sg.md`
Required frontmatter:
- `uuid`
- `type` (enum: `person`)
- `name`

Optional frontmatter:
- `email`
- `phone`
- `age`
- `sex`
- `created_on` (timestamp format above)
- `updated_on` (timestamp format above)

Optional markdown sections:
- `# Notes`

## `<study-root>/study.sg.md`
Required frontmatter:
- `status` (enum: `WIP`, `concluded`)
- `created_on`

Optional frontmatter:
- `result_status` (enum: `positive`, `negative`, `null`, `uncertain`)
- `pi_subject_ids` (array of subject UUIDs)

Required markdown title:
- first H1 heading is the study name/title

Expected markdown sections:
- `# Hypotheses`
- `# Discussion`
- `# Conclusion`
- `# Special Thanks`

## `<study-root>/protocol.sg.md`
Required markdown sections:
- `# Protocol Summary`
- `# Steps`

`# Steps` contract:
- each step is an H2 heading under `# Steps`
- heading text is step display name
- step slug is an ordered prefix plus normalized heading: `<NN>-<kebab-step-name>` where `NN` is 1-based, zero-padded to at least two digits (`01`, `02`, ...)
- optional step description is free-form markdown directly below the step heading until the next H2 (or next H1 section)

Optional markdown sections:
- `# Actions`

## `<study-root>/subject-requirements.yaml`
Required keys: none

Optionally, may specify fixed key/value pairs that then MUST exist on the subject for the subject to be
elligible for the study. E.g., to indicate that all study subjects must be people:
- `type: person`
Though any key/value pair may be fixed.

When creating a new subject from inside the study (regardless whether via non/interactive
mode), the fixed key/value pairs should be displayed but not editable, and the required
fields should be enforced

Optionally, may specify a list of required fields for subject creation flow:
- `required_fields` (array: any of `name`, `email`, `phone`, `age`, `sex`)

These requirements must be enforced at session creation time when subject selection takes
place. Subjects not meeting the criteria should not be presented as selectable.

## `<study-root>/session/<slug>/session.sg.md`
Required frontmatter:
- none

Optional frontmatter:
- `notes`

Optional markdown sections:
- `# Subjects`
- `# Notes`
Rule: `# Subjects` section is authoritative for session subjects; each non-empty line under `# Subjects` is one subject entry.

## `<study-root>/session/<slug>/step/<step-slug>/step.sg.md`
Required frontmatter:
- `time_started`

Required for final protocol step at session completion:
- `time_finished`

Optional for non-final protocol steps:
- `time_finished`
  - if omitted, implied value is `next_step.time_started - 1 second`

Optional markdown body:
- free-form notes

## CLI Contracts

`sg` is the executable.
`sg init`, `sg subject create/edit`, `sg session`, and `sg sessions` are interactive.
`sg session advance`, `sg ingest-photos`, `sg rm-assets`, `sg status`, and `sg publish` are non-interactive.

### `sg` (no args)
DWIM entrypoint behavior:
- if run inside a study root (or nested path), launch `sg sessions`
- if run in a directory missing `study.sg.md`, run `sg init` and then launch `sg sessions`
- otherwise, print help text

### `sg init`
Interactive prompt:
- asks for study name
- asks for protocol outline as brief step definitions, one step per line (`<step name> | <optional description>`)

Creates:
- `study.sg.md`
- `protocol.sg.md`
- `subject-requirements.yaml`
- `session/`

`study.sg.md` scaffold rules:
- include `created_on` with current local timestamp
- do not include `name` in frontmatter
- do not include `updated_on` in frontmatter
- write study title as first H1 heading

`protocol.sg.md` scaffold rules:
- create `# Steps` from protocol outline collected during init
- each outline entry becomes one H2 step heading in the entered order; when description is provided, write it directly below that H2
- if no outline is provided, create one placeholder step `## First Step`

### `sg subject create`
- creates a subject in `~/.study-guide/subject/`
- enforces `subject-requirements.yaml` required fields when run from a study directory
- collects optional fields after required fields

### `sg subject search <name>`
Search global subjects by name.

### `sg subject print <id-or-name>`
Print one subject.

### `sg subject ls`
List subjects by name.

### `sg subject rm <id>`
Delete one subject file from global subject store.

### `sg subject edit <id-or-name>`
- resolves a single subject (errors if not found or ambiguous)
- opens an interactive edit form prefilled with current values
- persists updated frontmatter/body to the same subject file

### `sg session`
Interactive session flow:
1. Select subject(s) from global store using the same `Create Session` picker UI used by `sg sessions` create mode.
   That shared picker includes `(+) New subject` and `-> Create Session` actions.
2. Create `session/<session-slug>/session.sg.md`.
3. Parse `protocol.sg.md` steps.
4. On step start: create step folder + `step.sg.md` and write `time_started`.
5. On step advance: write previous step `time_finished`, then start next step.
6. On finish: write current step `time_finished`.

Rule: session command is authoritative for step timing. Step timestamps are never derived from photos.
Note: for session progression, a step may be treated as effectively finished when a later protocol step has `time_started`, even if the earlier step has no explicit `time_finished`.

### `sg sessions`
Interactive session switchboard for running multiple sessions in parallel from one terminal.

Behavior:
1. Shows only incomplete sessions.
2. Provides an autocomplete query over subject name and session slug.
3. Browse table columns are ordered: `SLUG | SUBJECT | ACTIVE | STEP | NEXT`.
4. `ACTIVE` and `NEXT` columns are actionable cells.
5. The selected row (default top row) always has an active action cursor.
6. Arrow key behavior:
- up/down: move selected row
- left/right: move action cursor between `ACTIVE` and `NEXT` in the selected row
7. Press `Enter` to execute the action under the active action cursor:
- `ACTIVE`: mark that session as the single active session in study frontmatter (`study.sg.md` key `active_session_slug`); if the session has not started any protocol step yet, also auto-start its first step
- `NEXT`: perform exactly one timing transition (`start`, `advance`, or `finish`) based on current session progress
8. Press `Esc` to quit browse view.
9. Includes an action to create a new session without leaving the switchboard.
10. The session list view uses compact single-line rows (no blank description line).
   Unarmed row format includes step progress: `<slug> | <subject> | <X>/<Y> <current step>`.
   The browse view is implemented with a table component (column headers visible).
   Step progress is rendered as `[X/Y]`.
   `X` is the count of protocol steps progressed so far (implicitly-finished earlier steps count, plus the currently active step when present).
11. The list control/help legend is hidden on this screen.
12. Replace generic item-count status text with `current step: <step-name|->` status text.
13. In selected row, the focused actionable cell is visually emphasized (high-contrast and bracketed).
14. `ACTIVE` column text is:
- always `active` when row is the currently active session
- `activate` when row is selected and not active
- empty for non-selected, non-active rows
  When the active action cursor is on `ACTIVE`, the visible text is bracketed (for example `{active}` or `{activate}`).
15. `NEXT` column shows the next transition label (or `conclude` when progress action is `finish`).
   Actionable cells (`ACTIVE` and `NEXT`) use a subtle default background tint to indicate CTA affordance even when unfocused.
16. Browse table column sizing should be responsive to viewport width while prioritizing `STEP` readability; on wide viewports use preferred widths `SLUG=35`, `SUBJECT=35`, `ACTIVE=24`, `STEP=48`, and assign remaining width to `NEXT` (minimum `16`).
   Unfocused action text and `NEXT` text should use a brighter grey than footer/helper text (target color: ANSI 256 color `246`).
17. Filter prompt text is ` filter: ` (one leading space; no separate `Sessions` heading line).
   Filter placeholder text is exactly `by subject or slug`.
18. Browse table does not include `Create new session` or `Exit` rows.
   When there are no incomplete sessions, the table shows a single empty-state row: `no active sessions`.
19. Browse footer key hint is: `ctrl+n to create new; esc to quit`.
20. Row selection highlight must be terminal-adaptive and use a subtle tint approximately 15% away from terminal background luminance (lighter on dark terminals, darker on light terminals) to preserve readability across themes.
   The selected-row tint should include a slight blue hue with exact adaptive colors: light `#d9dcef`, dark `#262b3a`.
21. In create mode, selecting `Create` returns to the browse sessions table (showing the created session when applicable).
22. Create mode header text is exactly `Create Session`; instructional copy (`select one or more subjects, then choose Create; esc to cancel`) is shown as subtle/grey text directly below the header (above list items), not inside the header.
23. The shared create-session picker (used by both `sg session` and `sg sessions`) includes a `(+) New subject` action above `-> Create Session`.
23. Session completion/listing is derived from protocol step progress only (not `session.sg.md` timing fields).
24. In create mode, toggling subject selection must not emit transient per-toggle status text (for example `selected subjects: N`), so the view height remains stable while selecting.
25. Create-mode list item labels are uniformly indented with exactly two leading spaces.
26. Create-mode list selection must not change horizontal alignment; selected and unselected rows use the same left inset (no extra selected-state border offset).
27. Create-mode instructional info line is horizontally aligned with list items using the same two-space inset.
28. `p` triggers publish from browse view (keyboard action; not a table row). When there is at least one finished session and zero in-progress sessions, browse footer also includes a bright hint: `p publish with <X> sessions` where `X` is the finished-session count.
29. When `sg` runs with no args in a directory missing `study.sg.md`, the init UI must be visually cleared before transitioning into `sg sessions`.

Rule: this command enables switching among concurrent sessions without changing directories.
Rule: any number of sessions may be in-progress concurrently.

### `sg session advance`
Non-interactive "advance once" command for scriptable/session-directory usage.

Behavior:
- when run inside `<study-root>/session/<slug>/`, target that session
- otherwise require explicit `--session <slug>`
- performs one transition only:
  - start first step, or
  - advance to next step, or
  - finish the session if final step is active
- prints resulting state (`started`, `advanced`, or `finished`) with session slug and active step slug

### `sg ingest-photos`
Purpose: copy photo assets into matching step `asset/` folders by capture time.

Input source:
- default: query Photos Library SQLite metadata (`database/Photos.sqlite`) on macOS, then resolve matched assets from `originals/` on disk
- optional: `--assets-dir <path>` recursively reads image files from a local directory (used for tests/dev; also valid on non-macOS)

Session targeting:
- non-interactive
- always processes all sessions under `<study-root>/session/`
- if any session is missing required timing fields, command fails with a clear session-scoped error

Timestamp source precedence:
1. EXIF capture time
2. skip asset with warning if EXIF missing

Step windows:
- non-last step: `[step.time_started, implied_or_explicit_step.time_finished]` where implied `time_finished = next_step.time_started - 1 second` when omitted
- last step: `[last_step.time_started, last_step.time_finished]`

Rules:
- requires all step `time_started`; requires `time_finished` on last step
- deterministic output filename: `<YYYYMMDD-HHMMSS>_<sha8>.<ext>`
- duplicate handling: skip if same content already exists in target session
- idempotent: re-running ingestion should not duplicate copied assets
- prints per-session counts and one aggregate summary line

### `sg status`
Reports missing/invalid data that affects publication:
- missing expected study sections (`Hypotheses`, `Discussion`, `Conclusion`)
- missing required frontmatter fields
- sessions with missing step files for protocol steps
- steps missing `time_started`
- final protocol steps missing `time_finished`

Outputs:
- issue list
- overall completeness flag

### `sg rm-assets`
Purpose: remove all ingested step assets from the current study.

Behavior:
- non-interactive
- scans all sessions under `<study-root>/session/`
- deletes regular files under `step/<step-slug>/asset/` directories
- preserves study/session/step markdown files and directory structure
- prints a summary count of removed asset files

### `sg publish`
Generates both site and PDF from study files.

Default outputs:
- `<study-root>/publish/site/index.html`
- `<study-root>/publish/study.pdf`

Behavior:
- best effort (do not fail just because data is incomplete)
- run status checks first
- if required sections/steps/fields are missing, set `study.sg.md` frontmatter `status: WIP`
- include visible `WIP` indicator in generated outputs when incomplete

## Publication Structure (v1)

Generated outputs should include:
- study title and metadata
- hypotheses, discussion, conclusion
- protocol summary and steps
- sessions in chronological order
- for each session: subjects, step timeline, and associated images

## Web Subject Signup (Optional in v1)

May be implemented as a minimal Flask server:
- mobile-friendly form
- writes subject `.sg.md` files into `~/.study-guide/subject/`
- can read required fields from a selected study's `subject-requirements.yaml`

This is optional and should not block CLI implementation.

## Acceptance Criteria for v1

All criteria below are pass/fail requirements for v1.

### A. Scaffold and Layout
1. Running `sg` with no args in a directory missing `study.sg.md` runs init flow and then opens `sg sessions`.
2. Running `sg` with no args from a study root (or nested path) opens `sg sessions`.
3. Running `sg init` in an empty directory creates:
- `study.sg.md`
- `protocol.sg.md`
- `subject-requirements.yaml`
- `session/`
4. Re-running `sg init` does not destroy existing data files.
5. The generated scaffold matches the canonical layout defined in this spec.
6. `sg init` is interactive and requires a study name.
7. `study.sg.md` frontmatter includes `created_on` and does not include `name` or `updated_on`.
8. The first H1 in `study.sg.md` equals the study name entered during `sg init`.
9. `sg init` accepts a protocol outline with one step per line (`<step name> | <optional description>`) and writes each outline item as an H2 step under `# Steps` in `protocol.sg.md`.
10. If protocol outline is left blank, `protocol.sg.md` includes a placeholder `## First Step`.

### B. Subject Store and Subject Commands
1. `sg subject create` writes a `.sg.md` file under `~/.study-guide/subject/`.
2. Created subject file includes required fields: `uuid`, `type`, `name`.
3. `uuid` is valid UUIDv4 format.
4. When invoked from a study directory, required fields from `subject-requirements.yaml` are enforced.
5. `sg subject search <name>` returns matching subjects by name.
6. `sg subject print <id-or-name>` prints exactly one matching subject or a clear not-found/ambiguous error.
7. `sg subject ls` lists subjects in a human-readable format.
8. `sg subject rm <id>` deletes only the targeted subject file and leaves others unchanged.
9. `sg subject edit <id-or-name>` updates a single subject interactively and preserves UUID/path.

### C. Protocol Parsing
1. `protocol.sg.md` is accepted only when `# Protocol Summary` and `# Steps` exist.
2. Step definitions are parsed from H2 headings under `# Steps`.
3. Step slugs are `<NN>-<kebab-step-name>` using protocol order (`NN` is zero-padded 1-based index).
4. Optional step descriptions are parsed from markdown content directly under each H2 step heading.
5. Step order in parsed output matches source order in `protocol.sg.md`.

### D. Session Workflow and Timing
1. `sg session` creates `session/<session-slug>/session.sg.md`.
2. Session slug follows `<DD-MM-YYYY>-<subject-surname[-surname...]>`.
3. Session subjects are derived from the `# Subjects` section lines in `session.sg.md` (one subject per line).
4. Starting each step creates `step/<step-slug>/step.sg.md` with `time_started`.
5. Advancing from one step to the next writes `time_finished` to the previous step.
6. Finishing a session writes `time_finished` on the active/final step.
7. Step times are written by `sg session` and never inferred from ingested media.
8. All step timestamps use `HH:MM:SS DD-MM-YYYY`.
10. `sg sessions` supports autocomplete session lookup by subject name and session slug.
11. In `sg sessions`, `Enter` executes the currently focused action cell: `ACTIVE` sets `active_session_slug` and auto-starts the first step when the session has not started any step yet; `NEXT` performs one transition (`start`, `advance`, or `finish`).
12. `sg sessions` allows creating a new session and then managing it in the same interactive flow.
13. `sg session advance` works from within a session directory without requiring `cd` to other sessions.
14. `sg session advance --session <slug>` advances a specific session from study root (or any path within the study).
15. `sg sessions` uses one list view for arm-and-confirm (no separate confirm screen and no `Back` option); `Esc` cancels armed actions.
16. `sg sessions` view hides list control/help context and shows `current step: ...` status text instead of generic item-count status text.
17. In `sg sessions`, arming an action updates that same session row inline with `<X>/<Y>` progress and `enter to ...?` copy (no floating confirmation block below the list).
18. `sg sessions` shows `esc to cancel` helper text in subtle/grey style while an action is armed.
19. `sg sessions` progress numerator `X` in `[X/Y]` reflects progressed steps, not only active-step index; when no step is currently active but later protocol steps remain, `X` equals the number of completed steps.
20. In `sg sessions`, pressing `p` triggers publish from browse view. Footer hint text (`p publish with <X> sessions`) is shown only when `finished_sessions > 0` and `in_progress_sessions == 0`.

### E. Photo Ingestion
1. `sg ingest-photos` is non-interactive and runs against all sessions in the study.
2. Input source modes:
- default mode reads assets from Apple Photos on macOS.
- `--assets-dir <path>` mode reads image files recursively from local filesystem (supported on all OSes).
3. `--assets-dir` is optional.
4. Default mode scans expected Photos Library package subdirectories on disk and fails loudly with the checked paths when none are found.
   The default source path is configurable via `~/.study-guide/config` key `photos_library_path`.
   If configured path points at `Photos Library.photoslibrary` package root, ingestion resolves to `originals/` (or `Masters/` fallback) for file copy targets.
   When `database/Photos.sqlite` is available, candidate assets are selected via SQL metadata query (not filesystem `mtime` walk), then resolved to `originals/<ZDIRECTORY>/<ZFILENAME>`.
   If configured source is not a Photos Library package (no `database/Photos.sqlite`), ingestion falls back to filesystem scanning.
   Unrecognized keys in `~/.study-guide/config` are ignored but emitted as warnings.
5. EXIF capture time is used for matching; assets without EXIF are skipped with a warning.
6. In default mode with Photos Library package input, candidate files are discovered from SQLite metadata time fields (windowed by session step envelopes) before EXIF reads.
6. Time-window matching rule is enforced:
- non-last step: `[step.time_started, implied_or_explicit_step.time_finished]` where implied `time_finished = next_step.time_started - 1 second` when omitted
- last step: `[last_step.time_started, last_step.time_finished]`
7. Assets are copied to the correct `step/<step-slug>/asset/` directory.
8. Output names follow `<YYYYMMDD-HHMMSS>_<sha8>.<ext>`.
9. Duplicate files are skipped based on content identity within each session.
10. Re-running ingestion on unchanged inputs produces no duplicate copies (idempotent behavior).
10. Ingestion refuses to run when required timing fields for matching are missing in any targeted session.
11. Output includes per-session ingest counts and aggregate totals.
12. `sg ingest-photos --assets-dir <path>` is validated with a repository fixture asset set derived from `study-complete` images, with deterministic per-step placement assertions.
    The canonical fixture directory for that asset set is `fixtures/study-complete-assets/`.

### F. Status Reporting
1. `sg status` reports missing required frontmatter fields across study/session/step files.
2. `sg status` reports missing study markdown sections: `Hypotheses`, `Discussion`, `Conclusion`.
3. `sg status` reports sessions missing required step instances from protocol definition.
4. `sg status` reports steps missing `time_started`, and reports missing `time_finished` only for final protocol steps.
5. `sg status` outputs:
- a human-readable issue list
- an overall completeness result

### H. Asset Cleanup
1. `sg rm-assets` removes all files under every `session/*/step/*/asset/` directory in the current study.
2. `sg rm-assets` does not delete `study.sg.md`, `protocol.sg.md`, `session.sg.md`, or `step.sg.md`.
3. `sg rm-assets` outputs a deterministic summary count of removed files.

### G. Publish
1. `sg publish` always attempts generation (best effort), even if data is incomplete.
2. Default outputs are:
- `<study-root>/publish/site/index.html`
- `<study-root>/publish/study.pdf`
3. Generated outputs include:
- study title + metadata
- hypotheses, discussion, conclusion
- protocol summary + step list
- sessions in chronological order
- per-session subject list, step timeline, and associated images
4. `sg publish` runs status checks before rendering outputs.
5. If required sections/steps/fields are missing, `study.sg.md` is updated to `status: WIP`.
6. If incomplete, both HTML and PDF outputs visibly indicate `WIP`.
7. If complete, study status is not downgraded to `WIP`.

### H. Data Integrity and Safety
1. Commands modify only files they are responsible for.
2. No command deletes session assets unless explicitly requested by that command's contract.
3. Frontmatter remains parseable YAML after every command.
4. Existing user-authored markdown body content is preserved unless the command is explicitly responsible for that section.

### I. Automated Testing
1. The repository includes real Go unit tests (`*_test.go`) for core behavior.
2. At minimum, tests cover:
- frontmatter read/write and key ordering guarantees
- protocol step parsing and title extraction
- subject store create/edit/remove and subject resolution behavior
- status issue detection for missing required fields/sections
- ingest photo window matching and boundary behavior
- ingest duplicate/idempotency behavior
- ingest command behavior using `--assets-dir` fixtures against `fixtures/study-eg`
3. `go test ./...` passes in a clean checkout.
4. Tests must not read from or write to the real global subject directory (`~/.study-guide/`); tests must use isolated temporary directories.
5. TUI behavior tests should prefer stable contract and snapshot-style assertions (rendered text states and key layout invariants) over many micro-assertions of individual style properties.
6. Tests should not rely on mutable repository fixture state (for example pre-populated asset counts in `fixtures/`); setup should generate required runtime state within the test (for example by running `sg ingest-photos` in a temp study).
