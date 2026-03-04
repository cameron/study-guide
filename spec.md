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
- TUI framework: https://github.com/charmbracelet/bubbletea
- TUI components: https://github.com/charmbracelet/bubbles
- terminal markdown rendering: https://github.com/charmbracelet/glow

## Architecture and Implementation Notes
- Interactive command workflows should run in a single long-lived Bubble Tea program per command invocation.
- Within an interactive command, transitions between screens/actions should be internal model state changes, not nested `tea.NewProgram` launches.

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
- `study-eg/` is the canonical sample tree name (not `sample-eg/`)

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
- step slug is kebab-case transform of heading unless explicitly overridden later (future extension)

Optional markdown sections:
- `# Actions`

## `<study-root>/subject-requirements.yaml`
Required keys:
- `type: person`

Optional keys specify required fields for subject creation flow:
- `required_fields` (array: any of `name`, `email`, `phone`, `age`, `sex`)

## `<study-root>/session/<slug>/session.sg.md`
Required frontmatter:
- `time_started`
- `subject_ids` (array of subject UUIDs, minimum length 1)

Optional frontmatter:
- `time_finished`
- `notes`

Optional markdown sections:
- `# Subjects`
- `# Notes`

## `<study-root>/session/<slug>/step/<step-slug>/step.sg.md`
Required frontmatter:
- `time_started`

Required at session completion:
- `time_finished`

Optional markdown body:
- free-form notes

## CLI Contracts

`sg` is the executable.
`sg init`, `sg subject create/edit`, `sg session`, and `sg sessions` are interactive.
`sg current-session advance`, `sg ingest-photos`, `sg status`, and `sg publish` are non-interactive.

### `sg init`
Interactive prompt:
- asks for study name
- asks for protocol outline as brief step titles (zero or more)

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
- each outline entry becomes one H2 step heading in the entered order
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
1. Select subject(s) from global store (or create a new subject).
2. Create `session/<session-slug>/session.sg.md`.
3. Parse `protocol.sg.md` steps.
4. On step start: create step folder + `step.sg.md` and write `time_started`.
5. On step advance: write previous step `time_finished`, then start next step.
6. On finish: write current step `time_finished` and `session.sg.md time_finished`.

Rule: session command is authoritative for step timing. Step timestamps are never derived from photos.
Note: for session progression, a step may be treated as effectively finished when a later protocol step has `time_started`, even if the earlier step has no explicit `time_finished`.

### `sg sessions`
Interactive session switchboard for running multiple sessions in parallel from one terminal.

Behavior:
1. Shows only incomplete sessions.
2. Provides an autocomplete query over subject name and session slug.
3. In the same list view, press `Enter` once to arm the highlighted session/action.
4. Press `Enter` again (on the same row) to execute the default action:
- start first protocol step (if no step has started yet)
- advance to next step (if currently between first and last step)
- finish session (if currently on final step)
5. Press `Esc` to cancel an armed action (no dedicated `Back` item/screen).
6. Includes an action to create a new session without leaving the switchboard.
7. The session list view uses compact single-line rows (no blank description line).
   Unarmed row format includes step progress: `<slug> | <subject> | <X>/<Y> <current step>`.
   The browse view is implemented with a table component (column headers visible).
   Step progress is rendered as `[X/Y]`.
   `X` is the count of protocol steps progressed so far (implicitly-finished earlier steps count, plus the currently active step when present).
8. The list control/help legend is hidden on this screen.
9. Replace generic item-count status text with `current step: <step-name|->` status text.
10. When an action is armed, update that same session row inline (not below the list), for example:
- `<slug> | <subject> | <X>/<Y> <current step> "enter to advance to <next step>?"`
11. Show `esc to cancel` as subtle/grey helper text.
12. The browse view includes a `Next Step` column:
- unarmed rows: next step name rendered in subtle/grey style
- armed row: same next-step name rendered with high-contrast emphasis and suffix ` (enter to advance)`
   Browse table base column widths are absolute: `SLUG=35`, `SUBJECT=35`, `STEP=48`; `NEXT STEP` gets the remaining width with minimum `32`.
13. Filter prompt text is ` filter: ` (one leading space; no separate `Sessions` heading line).
14. Browse table does not include `Create new session` or `Exit` rows.
15. Browse footer key hint is: `ctrl+n to create new; esc to quit`.
16. Row selection highlight must be terminal-adaptive and use a subtle tint approximately 15% away from terminal background luminance (lighter on dark terminals, darker on light terminals) to preserve readability across themes.
17. In create mode, selecting `Done` returns to the browse sessions table (showing the created session when applicable).
18. A session with `session.sg.md time_finished` but missing required protocol step progress is treated as incomplete/invalid and remains listed.

Rule: this command enables switching among concurrent sessions without changing directories.
Rule: any number of sessions may be in-progress concurrently.

### `sg current-session advance`
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
- default: direct Apple Photos album export on macOS
- optional: `--assets-dir <path>` recursively reads image files from a local directory (used for tests/dev; also valid on non-macOS)

Session targeting:
- non-interactive
- always processes all sessions under `<study-root>/session/`
- if any session is missing required timing fields, command fails with a clear session-scoped error

Timestamp source precedence:
1. EXIF capture time
2. skip asset with warning if EXIF missing

Step windows:
- non-last step: `[step.time_started, next_step.time_started)`
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
- steps missing `time_started`/`time_finished`

Outputs:
- issue list
- overall completeness flag

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
1. Running `sg init` in an empty directory creates:
- `study.sg.md`
- `protocol.sg.md`
- `subject-requirements.yaml`
- `session/`
2. Re-running `sg init` does not destroy existing data files.
3. The generated scaffold matches the canonical layout defined in this spec.
4. `sg init` is interactive and requires a study name.
5. `study.sg.md` frontmatter includes `created_on` and does not include `name` or `updated_on`.
6. The first H1 in `study.sg.md` equals the study name entered during `sg init`.
7. `sg init` accepts a protocol outline and writes each outline item as an H2 step under `# Steps` in `protocol.sg.md`.
8. If protocol outline is left blank, `protocol.sg.md` includes a placeholder `## First Step`.

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
3. Step slugs are kebab-case transforms of step headings.
4. Step order in parsed output matches source order in `protocol.sg.md`.

### D. Session Workflow and Timing
1. `sg session` creates `session/<session-slug>/session.sg.md`.
2. Session slug follows `<DD-MM-YYYY>-<subject-surname[-surname...]>`.
3. Session file contains required fields: `time_started`, `subject_ids`.
4. Starting each step creates `step/<step-slug>/step.sg.md` with `time_started`.
5. Advancing from one step to the next writes `time_finished` to the previous step.
6. Finishing a session writes:
- `time_finished` on the active/final step
- `time_finished` in `session.sg.md`
7. Step times are written by `sg session` and never inferred from ingested media.
8. All timestamps in session and step files use `HH:MM:SS DD-MM-YYYY`.
9. `session.sg.md` frontmatter key order writes `time_started` before `time_finished` when both exist.
10. `sg sessions` supports autocomplete session lookup by subject name and session slug.
11. In `sg sessions`, a second `Enter` after selection executes exactly one transition (`start`, `advance`, or `finish`) based on current session progress.
12. `sg sessions` allows creating a new session and then managing it in the same interactive flow.
13. `sg current-session advance` works from within a session directory without requiring `cd` to other sessions.
14. `sg current-session advance --session <slug>` advances a specific session from study root (or any path within the study).
15. `sg sessions` uses one list view for arm-and-confirm (no separate confirm screen and no `Back` option); `Esc` cancels armed actions.
16. `sg sessions` view hides list control/help context and shows `current step: ...` status text instead of generic item-count status text.
17. In `sg sessions`, arming an action updates that same session row inline with `<X>/<Y>` progress and `enter to ...?` copy (no floating confirmation block below the list).
18. `sg sessions` shows `esc to cancel` helper text in subtle/grey style while an action is armed.
19. `sg sessions` progress numerator `X` in `[X/Y]` reflects progressed steps, not only active-step index; when no step is currently active but later protocol steps remain, `X` equals the number of completed steps.

### E. Photo Ingestion
1. `sg ingest-photos` is non-interactive and runs against all sessions in the study.
2. Input source modes:
- default mode reads assets from Apple Photos on macOS.
- `--assets-dir <path>` mode reads image files recursively from local filesystem (supported on all OSes).
3. `--assets-dir` is optional.
4. EXIF capture time is used for matching; assets without EXIF are skipped with a warning.
5. Time-window matching rule is enforced:
- non-last step: `[step.time_started, next_step.time_started)`
- last step: `[last_step.time_started, last_step.time_finished]`
6. Assets are copied to the correct `step/<step-slug>/asset/` directory.
7. Output names follow `<YYYYMMDD-HHMMSS>_<sha8>.<ext>`.
8. Duplicate files are skipped based on content identity within each session.
9. Re-running ingestion on unchanged inputs produces no duplicate copies (idempotent behavior).
10. Ingestion refuses to run when required timing fields for matching are missing in any targeted session.
11. Output includes per-session ingest counts and aggregate totals.

### F. Status Reporting
1. `sg status` reports missing required frontmatter fields across study/session/step files.
2. `sg status` reports missing study markdown sections: `Hypotheses`, `Discussion`, `Conclusion`.
3. `sg status` reports sessions missing required step instances from protocol definition.
4. `sg status` reports steps missing `time_started` or `time_finished`.
5. `sg status` outputs:
- a human-readable issue list
- an overall completeness result

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
- ingest command behavior using `--assets-dir` fixtures against `study-eg`
3. `go test ./...` passes in a clean checkout.
4. Tests must not read from or write to the real global subject directory (`~/.study-guide/`); tests must use isolated temporary directories.
