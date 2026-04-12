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
- v1 has exactly one protocol stored inside `<study-root>/study.sg.md`
- sessions are not nested under protocol
- `fixtures/study-eg/` is the canonical sample tree name (not `sample-eg/`)
- shipped fixtures used by tests must use anonymized study and subject names

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
- `hero_comparison`
  - optional object describing a study-level side-by-side comparison image pair for publish output
  - shape:
    - `left`
      - `session` (source session slug)
      - `step` (source step slug)
      - `asset_index` (0-based index into that step's sorted asset list)
    - `right`
      - `session` (source session slug)
      - `step` (source step slug)
      - `asset_index` (0-based index into that step's sorted asset list)

Required markdown title:
- first H1 heading is the study name/title

Optional study preamble:
- markdown between the title H1 and the next top-level H1 section is preserved as authored study preamble content
- study preamble may contain subtitle, abstract, or editorial notes before `# Introduction`
- study parsing preserves title, study preamble, and top-level H1 sections in source order from a single pass over `study.sg.md`

Expected markdown sections:
- `# Introduction`
- `# Methods`
- `# Results`
- `# Discussion`
- `# Conclusion`
- `# Special Thanks`

`# Methods` contract:
- markdown directly under `# Methods` before `## Protocol` is the protocol summary / authored methods text
- `## Protocol` is required under `# Methods`
- each protocol step is an H3 heading under `## Protocol`
- heading text is step display name
- protocol step titles must be unique after slug normalization within a study
- step slug is an ordered prefix plus normalized heading: `<NN>-<kebab-step-name>` where `NN` is 1-based, zero-padded to at least two digits (`01`, `02`, ...)
- when a step heading changes but its protocol position stays the same, existing session step directories for that ordinal are renamed to the regenerated step slug and keep their `step.sg.md` plus `asset/` contents
- when protocol steps are reordered without renaming, existing session step directories are reassigned by normalized step title and then renamed to the regenerated ordered step slugs
- when a new protocol step is added, existing session step directories continue following their historical step identities rather than being reassigned to fill the inserted ordinal; newly added steps do not gain synthesized historical session directories
- v1 supports rename-only, reorder-only, add-only, and add+rename protocol edits; simultaneous rename+reorder edits are out of scope
- optional step description is free-form markdown directly below the step heading until the next H3 (or next H2 / H1 section)

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
- `required_fields` (array: supports known fields `name`, `email`, `phone`, `age`, `sex` plus custom fields)

These requirements must be enforced at session creation time when subject selection takes
place. Subjects not meeting the criteria should not be presented as selectable.

Any scalar key in `subject-requirements.yaml` other than `required_fields` is treated as a fixed
subject field/value for in-study subject creation. Fixed fields:
- are shown in the subject-create form
- are pre-populated with the fixed value from `subject-requirements.yaml`
- are marked fixed (not editable)
- are always persisted onto the created subject frontmatter

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
  - if explicitly present, that explicit value remains authoritative for timing/ownership validation, including when it is equal to the next step's `time_started`

Optional frontmatter:
- `focus_windows` (array of `{time_started, time_finished}` pairs)
  - records the periods when this step was focused in `sg sessions`
  - each entry must use the standard timestamp format (`HH:MM:SS DD-MM-YYYY`)
  - `time_finished` may be omitted only while the step is currently focused; it must be present for closed/completed sessions
  - ingestion ownership uses `focus_windows` only (no fallback to step-envelope ownership)
- `unfocusable` (boolean)
  - when `true`, `sg sessions` and `sg session advance` skip over that step within the same advance action
  - the step still exists for publication/export/history, but is excluded from session-board progress counts/labels
  - photo ingest ignores that step's focus-window requirements and never assigns assets to it
- `render_asset_indices` (array of 0-based integers)
  - controls publish image selection/order for this step
  - indices are resolved against that step's sorted source asset list
  - when present, publish renders only the listed assets, in the listed order

Optional markdown body:
- free-form notes

## CLI Contracts

`sg` is the executable.
`sg init`, `sg subject create/edit`, `sg session`, and `sg sessions` are interactive.

All interactive Bubble Tea flows must be launched through a single shared program runner configured with alternate-screen mode so `sg` takes over and restores the terminal cleanly.
All interactive screens render their title/header using a shared in-palette title bar style with background fill (not plain unstyled text headers).
The shared title bar uses a brighter, bluer turquoise adaptive background tint: light `#78f0ff`, dark `#1490a0`.
Filter prompt accent blue reuses that same adaptive blue values for palette consistency.
`sg session advance`, `sg sessions print`, `sg data ingest`, `sg data ls`, `sg data clean`, `sg status`, `sg publish`, and `sg export` are non-interactive.
`sg protocol reconcile` is non-interactive.
CLI help output lists `sg protocol reconcile`, `sg data ingest`, `sg data ls`, `sg data clean`, and `sg export` as separate command entries.

### `sg` (no args)
DWIM entrypoint behavior:
- if run inside a study root (or nested path), launch `sg sessions`
- if run in a directory missing `study.sg.md`, run `sg init` and then launch `sg sessions`
- otherwise, print help text

### `sg init`
Interactive prompt:
- asks for study name (pre-filled from current folder name, converting `-`/`_` to spaces and title-casing words)
- asks for protocol step titles (one title per entry)
- while entering protocol step titles: `Enter` adds the typed title and keeps init in step-entry mode; at least one step is required before `Enter` on an empty input can finish step entry

Creates:
- `study.sg.md`
- `subject-requirements.yaml`
- `session/`

`study.sg.md` scaffold rules:
- include `created_on` with current local timestamp
- do not include `name` in frontmatter
- do not include `updated_on` in frontmatter
- write study title as first H1 heading
- include empty `# Introduction`, `# Methods`, `# Results`, `# Discussion`, `# Conclusion`, and `# Special Thanks` sections in that order
- under `# Methods`, create a `## Protocol` subsection from protocol outline collected during init
- each outline entry becomes one H3 step heading in the entered order
- init requires at least one outline step; no placeholder step is synthesized

### `sg subject create`
- creates a subject in `~/.study-guide/subject/`
- enforces `subject-requirements.yaml` required fields when run from a study directory
- when launched from `sg session`/`sg sessions` create mode, reads requirements from that active study root
- collects optional fields after required fields

### `sg subject search <name>`
Search global subjects by name.

### `sg subject print [<id-or-name>] [--all]`
Print subject information.

Behavior:
- when `<id-or-name>` is provided, print exactly one matching subject
- when no `<id-or-name>` is provided and the current working directory is inside a study, print the distinct subjects referenced by session `# Subjects` sections in that study
- when no `<id-or-name>` is provided and the current working directory is not inside a study, print all subjects
- `--all` forces printing all subjects even when inside a study

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
   That shared picker includes `(+) New subject` and `-> Create Session` actions; choosing `(+) New subject` completes subject creation and then immediately creates the session with that new subject.
2. Create `session/<session-slug>/session.sg.md`.
3. Parse protocol steps from `study.sg.md` `# Methods` → `## Protocol`.
4. On step start: create step folder + `step.sg.md` and write `time_started`.
5. On step advance: write previous step `time_finished`, then start next step.
6. On finish: write current step `time_finished`.

Rule: session command is authoritative for step timing. Step timestamps are never derived from photos.
Note: for session progression, a step may be treated as effectively finished when a later protocol step has `time_started`, even if the earlier step has no explicit `time_finished`.

### `sg sessions`
Interactive session switchboard for running multiple sessions in parallel from one terminal.

Behavior:
1. Shows both incomplete and completed sessions.
   Completed sessions remain selectable, are rendered in grey text, and are sorted below all incomplete sessions.
2. Provides an autocomplete query over subject name and session slug.
3. Browse table columns are ordered: `SUBJECT | CURRENT STEP | NEXT STEP`.
4. The selected row (default top row) is the row that keyboard actions apply to.
5. Arrow key behavior:
- up/down: move selected row
- typing into the browse filter is ordinary text entry and must not trigger any session action
6. Press `Enter` to perform the selected row's next transition:
- if that row is not focused, first mark it as the single focused session in study frontmatter (`study.sg.md` key `active_session_slug`)
- if another session was focused, close that previous focused session's open `focus_windows` interval (if any active step exists)
- then perform exactly one timing transition (`start`, `advance`, or `finish`) for the selected row based on current session progress
- if the session has not started any protocol step yet, this starts its first step as part of that same `Enter`
   Focus tracking contract:
   - switching focus closes the previous focused session's open `focus_windows` interval (if any active step exists) and opens a new interval on the newly focused session's active step (if any)
   - when a focused session advances/reverses/finishes, `focus_windows` follow the active step so there is never more than one open focus interval per session
7. Press `ctrl+a` in browse view to open the selected session's current step asset folder in Finder.
   If the current step asset folder does not exist yet, create it first.
8. Press `Esc` in browse view to:
- unfocus the active session, clear `active_session_slug`, and close that session's open `focus_windows` interval when any session is currently focused
- quit browse view when no session is currently focused
9. Includes an action to create a new session without leaving the switchboard.
10. The session list view uses compact single-line rows (no blank description line).
   The browse view is implemented with a table component (column headers visible).
   `CURRENT STEP` is rendered as `[X/Y] <current step>`.
   `NEXT STEP` shows the next transition target label (or `conclude` when the next action is `finish`).
   `X` is the count of protocol steps progressed so far (implicitly-finished earlier steps count, plus the currently active step when present).
11. The list control/help legend is hidden on this screen.
12. Browse view does not render extra status summary lines above or below the table (except the key-hint footer line). This includes per-action session state/result text (for example `session=<slug> state=...`) which must not be shown in the browse layout.
13. Browse table selected-row styling uses Bubble's default pink selected text treatment (foreground emphasis).
   This pink selected styling applies to the sessions browse table and create-mode subject list.
14. The focused session row is visually separated from the other rows with a very light green background wash that does not rely on inserting a selectable blank row.
15. Browse table column sizing should be responsive to viewport width while prioritizing `CURRENT STEP` readability; on wide viewports use preferred widths `SUBJECT=35`, `CURRENT STEP=48`, and assign remaining width to `NEXT STEP` (minimum `16`).
16. Browse layout order is:
- `Sessions` title line with inline key legend rendered on the same line in the format `[key] description // [key] description // ...`
- filter input (` filter: ` prompt)
- browse body with the open-sessions table on the left and a `Focus History` panel on the right
  The `Focus History` panel lists recent step `focus_windows` across all sessions, including concluded sessions, newest first, and each entry includes the session name, protocol step number/name, and the window start/stop times.
   Filter inputs (browse + shared create picker) use a distinct accent style:
   prompt color uses the same adaptive blue as title bars (`#78f0ff` light, `#1490a0` dark), query text ANSI `212` (pink), placeholder ANSI `245` (dim).
17. Focused session row is always pinned to the top of the open-sessions list.
18. After focusing a session from browse view, the selected row moves to the top focused row.
19. Filter placeholder text is exactly `by subject or slug`.
20. Browse table does not include `Create new session` or `Exit` rows.
   When there are no incomplete sessions, the table shows a single empty-state row: `no open sessions`.
20. Browse title-line key hint is: `[enter] next step // [ctrl+b] step backwards // [ctrl+n] create session // [p] publish // [esc] unfocus/quit`.
21. In browse view, only the literal `Sessions` title text uses the shared highlighted title bar styling; the inline key hint remains plain text on the same line after the title.
22. In create mode, selecting `Create` returns to the browse sessions table (showing the created session when applicable).
23. Create mode header text is exactly `Create Session`; instructional copy (`select a subject, then confirm Create; esc to cancel`) is shown as subtle/grey text directly below the header (above list items), not inside the header.
24. The shared create-session picker (used by both `sg session` and `sg sessions`) includes a `(+) New subject` action above the create-confirmation row.
25. Session completion/listing is derived from protocol step progress only (not `session.sg.md` timing fields).
26. In create mode, toggling subject selection must not emit transient per-toggle status text (for example `selected subjects: N`), so the view height remains stable while selecting.
27. Create-mode list item labels are uniformly indented with exactly two leading spaces.
27. Create-mode list selection must not change horizontal alignment; selected and unselected rows use the same left inset (no extra selected-state border offset).
28. Create-mode instructional info line is horizontally aligned with list items using the same two-space inset.
29. `p` triggers publish from browse view (keyboard action; not a table row).
30. When `sg` runs with no args in a directory missing `study.sg.md`, the init UI must be visually cleared before transitioning into `sg sessions`.
31. Choosing `(+) New subject` from the shared create-session picker (used by both `sg session` and `sg sessions`) must transition into an isolated subject-create screen within that same long-lived interactive program, then immediately create a session with that new subject and return to browse view without stale rows leaking between the two screens.
32. In the shared create-session picker (used by both `sg session` and `sg sessions`), typing immediately starts fuzzy autocomplete filtering over subject names (without requiring `/`).
33. In create mode of that same shared picker, `shift+enter` is a keyboard shortcut for the current create-confirmation row.
34. In create mode of that same shared picker, the subject filter/search input is always visible before typing.
35. In create mode of that same shared picker, clearing the subject filter query (for example backspacing from one character to empty) must not duplicate the `Filter:` line, and subject rows remain selectable.
36. In create mode of that same shared picker, when the subject filter query changes, the filtered results must auto-select the top visible entry.
37. Create-mode selected subject row styling uses Bubble's default pink selected color, and this selected styling must apply to the auto-selected top entry while filtering.
38. In create mode of the shared create-session picker, pressing `enter` on a subject chooses exactly that subject, rewrites the create-confirmation row label to `-> Create Session with <subject>`, and moves selection to that row instead of building a multi-subject selection list.
39. In `sg sessions` create mode and the shared `sg session` picker, subject rows are ordered by most recently created first, so a subject created from `(+) New subject` returns at the top of the picker.
40. In create mode of the shared create-session picker, the subject list height must expand with the terminal viewport instead of staying at a short fixed height, so available subjects can fill the screen.

Rule: this command enables switching among concurrent sessions without changing directories.
Rule: any number of sessions may be in-progress concurrently.

### `sg sessions print`
Non-interactive session timing report.

Behavior:
- prints a table with columns: `SESSION | STEP | START | END`
- `STEP` values use protocol step slugs
- includes one row per protocol step for each session under `<study-root>/session/`
- session rows are grouped by session slug (ascending) and step rows follow protocol order
- `START`/`END` values come from each step's `time_started`/`time_finished` frontmatter (empty when missing)
- rendering uses a Bubble Tea table component so columns align visually in terminal output (not markdown-pipe rows)

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

### `sg session reverse`
Non-interactive "step backwards once" command for scriptable/session-directory usage.

Behavior:
- when run inside `<study-root>/session/<slug>/`, target that session
- otherwise require explicit `--session <slug>`
- finds the current active step and removes only its `time_started` value
- keeps step folders/files intact
- if no active step exists, returns an error
- prints resulting state (`reversed`) with session slug and step slug

### `sg protocol reconcile`
Purpose: explicitly reconcile session step directory slugs to the current protocol in `study.sg.md`.

Behavior:
- non-interactive
- parses `study.sg.md`
- renames existing session step directories to the current ordinal-based step slugs when the step ordinal still matches
- preserves `step.sg.md` and `asset/` contents within renamed step directories
- prints a success message when reconciliation completes

### `sg data ingest`
Purpose: copy captured media assets into matching step `asset/` folders by capture time.

Input source:
- default: query Photos Library SQLite metadata (`database/Photos.sqlite`) on macOS, then resolve matched assets from `originals/` on disk
- optional: `--assets-dir <path>` recursively reads supported media files from a local directory (used for tests/dev; also valid on non-macOS)

Session targeting:
- non-interactive
- always processes all sessions under `<study-root>/session/`
- if any session is missing required timing fields, command fails with a clear session-scoped error

Timestamp source precedence:
1. embedded capture timestamp metadata (EXIF for images; QuickTime/video creation metadata for supported video formats)
2. skip asset with warning if capture timestamp metadata is missing

Step windows:
- non-last step: `[step.time_started, implied_or_explicit_step.time_finished]` where implied `time_finished = next_step.time_started - 1 second` when omitted
- last step: `[last_step.time_started, last_step.time_finished]`

Ownership windows:
- ingest matches assets to `focus_windows` intervals (per entry) for each step
- if any step is missing `focus_windows`, or has an open/invalid `focus_windows` interval, ingestion fails with a clear session-scoped error

Rules:
- requires all step `time_started`; requires `time_finished` on last step
- deterministic output filename: `<NN>-<YYYYMMDD-HHMMSS>_<sha8>.<ext>` where `NN` is the 0-based, zero-padded chronological index within that step
- after each ingest run, step asset filenames are renumbered so the numeric prefix stays in capture-time order within each step; ties break by content hash
- supported ingest media types are `.jpg`, `.jpeg`, `.png`, `.heic`, `.heif`, `.tif`, `.tiff`, `.mov`, `.mp4`, and `.m4v`
- in default Photos SQLite mode, candidate selection groups variants by logical Photos master asset id and keeps only the newest row by metadata modification/create time (so edited/rotated variants win over older originals)
- before step matching, captured candidates that share the same sub-second EXIF capture instant are deduplicated; Photos render candidates (`resources/renders`) are preferred over non-render variants, then newest file modification time breaks ties
- if the latest available asset capture time is older than the study's latest focus-window `time_finished`, ingestion prints an incomplete-sync warning and continues
- duplicate handling: skip if same content already exists in target session
- idempotent: re-running ingestion should not duplicate copied assets
- prints per-session counts and one aggregate summary line

### `sg data ls`
Purpose: list ingested step assets for all sessions in the current study.

Behavior:
- non-interactive
- reconciles session step directory slugs to the current protocol before scanning assets
- scans all sessions under `<study-root>/session/`
- prints one row per asset with columns `SESSION | STEP | FILE`
- ignores filesystem metadata files in asset directories (for example `.DS_Store`)
- rows are sorted by `SESSION`, then `STEP`, then `FILE`
- prints aggregate summary line with total assets

### `sg data clean`
Purpose: remove all ingested step assets from the current study.

Behavior:
- non-interactive
- reconciles session step directory slugs to the current protocol before deleting assets
- scans all sessions under `<study-root>/session/`
- deletes regular files under `step/<step-slug>/asset/` directories
- does not delete study/session/step metadata files
- prints a summary count of removed asset files

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

### `sg publish`
Generates both site and PDF from study files.

Default outputs:
- `<study-root>/publish/site/index.html`
- `<study-root>/publish/study.pdf`

Flags:
- `--once` runs a single publish pass and exits
- `--with-subject-names` preserves real subject names in published outputs

Behavior:
- defaults to a continuous mode that watches study files/directories and re-runs publish when they change
- best effort (do not fail just because data is incomplete)
- run status checks first
- if required sections/steps/fields are missing, set `study.sg.md` frontmatter `status: WIP`
- include visible `WIP` indicator in generated outputs when incomplete
- default published outputs anonymize session subjects using deterministic study-wide placeholders (`Subject 1`, `Subject 2`, ...)
- publish also writes a local subject lookup file in the study root so the operator can map anonymized labels back to real names without exposing those names in the HTML/PDF outputs

## Publication Structure (v1)

Generated outputs should include:
- study title and metadata
- hypotheses, discussion, conclusion
- protocol summary and steps
- sessions in chronological order
- for each session: anonymized subjects by default, step timeline, and associated images
- the web session list shows only a linked session title and thumbnails; by default the title is a comma-separated list of anonymized subject labels, and with `--with-subject-names` it uses the full single-subject name or comma-separated subject last names

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
- `subject-requirements.yaml`
- `session/`
4. Re-running `sg init` does not destroy existing data files.
5. The generated scaffold matches the canonical layout defined in this spec.
6. `sg init` is interactive and pre-fills study name from the current folder name (converting `-`/`_` to spaces and title-casing words).
7. `study.sg.md` frontmatter includes `created_on` and does not include `name` or `updated_on`.
8. The first H1 in `study.sg.md` equals the resolved study name from `sg init` (user-provided value, or derived folder-name default when left unchanged/blank).
9. `sg init` accepts protocol step titles and writes each title as an H3 step under `study.sg.md` `# Methods` → `## Protocol`.
10. `sg init` requires at least one protocol step before finishing the protocol-title prompt.

### B. Subject Store and Subject Commands
1. `sg subject create` writes a `.sg.md` file under `~/.study-guide/subject/`.
2. Created subject file includes required fields: `uuid`, `type`, `name`.
3. `uuid` is valid UUIDv4 format.
4. When invoked from a study directory, required fields from `subject-requirements.yaml` are enforced.
5. Custom required fields (non-built-in keys) are prompted in subject creation and persisted to subject frontmatter.
6. Fixed subject-requirement key/value pairs are shown in the create form as fixed and written to the subject.
7. `sg subject search <name>` returns matching subjects by name.
8. `sg subject print [<id-or-name>] [--all]` prints exactly one matching subject for explicit queries; with no query it prints current-study subjects, or all subjects when outside a study or when `--all` is set.
9. `sg subject ls` lists subjects in a human-readable format.
10. `sg subject rm <id>` deletes only the targeted subject file and leaves others unchanged.
11. `sg subject edit <id-or-name>` updates a single subject interactively and preserves UUID/path.

### C. Protocol Parsing
1. `study.sg.md` is accepted only when `# Methods` contains a `## Protocol` subsection.
2. Step definitions are parsed from H3 headings under `# Methods` → `## Protocol`.
3. Step slugs are `<NN>-<kebab-step-name>` using protocol order (`NN` is zero-padded 1-based index).
4. Parsing the study protocol reconciles existing session step directory names to the current ordinal-based step slugs.
5. Optional protocol summary text is parsed from markdown content directly under `# Methods` before `## Protocol`.
6. Optional step descriptions are parsed from markdown content directly under each H3 step heading.
7. Step order in parsed output matches source order in `study.sg.md`.
7. `sg protocol reconcile` explicitly triggers that same step-directory reconciliation and prints a success message.
8. Publish output renders the authored `# Methods` summary content and protocol step list from `study.sg.md` inside the `Methods` section positioned between study `Introduction` and `Results`.
9. Publish output appends session result data beneath the `Results` section instead of rendering a separate top-level `Sessions` section.
10. Publish output preserves authored study preamble content from between the title H1 and `# Introduction` before the `Introduction` section, without inventing a synthetic heading for it.

### D. Session Workflow and Timing
1. `sg session` creates `session/<session-slug>/session.sg.md`.
2. Session slug follows `<DD-MM-YYYY>-<subject-surname[-surname...]>`.
3. Session subjects are derived from the `# Subjects` section lines in `session.sg.md` (one subject per line).
4. Creating a session precreates `step/<step-slug>/step.sg.md` plus `asset/` for every protocol step, with empty frontmatter until that step is started.
5. Starting a step writes `time_started` into that existing `step/<step-slug>/step.sg.md`.
6. Advancing from one step to the next writes `time_finished` to the previous step.
7. Finishing a session writes `time_finished` on the active/final step.
8. Step times are written by `sg session` and never inferred from ingested media.
9. All step timestamps use `HH:MM:SS DD-MM-YYYY`.
10. `sg sessions` supports autocomplete session lookup by subject name and session slug.
11. In `sg sessions`, `Enter` applies to the selected row: it focuses that session if needed, then performs exactly one transition (`start`, `advance`, or `finish`) for that row.
11a. `sg sessions` records focus ownership per step via `focus_windows` in step frontmatter; focus switches close the previous focused interval and open the next focused interval.
11b. Protocol steps with `unfocusable: true` are excluded from the session-board progression experience: starting/advancing skips past them within the same action, and board progress counts/labels use only focusable steps.
12. `sg sessions` allows creating a new session and then managing it in the same interactive flow.
13. `sg session advance` works from within a session directory without requiring `cd` to other sessions.
14. `sg session advance --session <slug>` advances a specific session from study root (or any path within the study).
15. In `sg sessions`, `Esc` unfocuses the active session when one is focused; otherwise it exits the browse view.
16. `sg sessions` view hides list control/help context and does not show extra browse status summary lines; it keeps the browse key-hint footer.
17. `sg sessions` progress numerator `X` in `[X/Y]` reflects progressed steps, not only active-step index; when no step is currently active but later protocol steps remain, `X` equals the number of completed steps.
18. `sg sessions` keeps separate selected-row and focused-session concepts: selection follows the cursor, while focus is the session named by `active_session_slug`.
19. `sg sessions` visually distinguishes the focused row without inserting a selectable spacer row.
20. In `sg sessions`, pressing `p` triggers publish from browse view.
21. `sg session reverse` clears `time_started` on the active step only and keeps step files/folders intact.
22. In `sg sessions`, pressing `ctrl+b` performs the same single-step reverse transition as `sg session reverse` for the selected row.
23. In `sg sessions`, pressing `ctrl+a` opens the selected session's current step asset folder, creating the `asset/` directory first when needed.
23. `sg sessions print` outputs one timing row per protocol step per session in an aligned Bubble Tea table with `SESSION | STEP | START | END` columns.
24. In `sg sessions` create mode and the shared `sg session` picker, clearing the subject filter query preserves single-line filter rendering and keeps subject rows selectable.
25. In `sg sessions` create mode and the shared `sg session` picker, typing/changing a subject filter auto-selects the top filtered subject entry.
27. In `sg sessions` create mode and the shared `sg session` picker, subject rows are ordered by most recently created first, so a subject created from `(+) New subject` returns at the top of the picker.

### E. Photo Ingestion
1. `sg data ingest` is non-interactive and runs against all sessions in the study.
2. Input source modes:
- default mode reads assets from Apple Photos on macOS.
- `--assets-dir <path>` mode reads supported media files recursively from local filesystem (supported on all OSes).
3. `--assets-dir` is optional.
4. Default mode requires an Apple Photos library package on disk and fails loudly with the checked paths when none are found.
   The default source path is configurable via `~/.study-guide/config` key `photos_library_path`.
   If configured path points at `Photos Library.photoslibrary` package root, ingestion resolves to `originals/` (or `Masters/` fallback) for file copy targets.
   Candidate assets are selected via SQL metadata query from `database/Photos.sqlite`, then resolved to `originals/<ZDIRECTORY>/<ZFILENAME>`.
   That SQL selection deduplicates duplicate/edited metadata rows for the same logical master asset (using Photos master linkage) by keeping only the most recent metadata row.
   If configured source is not a Photos Library package with `database/Photos.sqlite`, default-mode ingestion fails; recursive filesystem ingestion is available only via `--assets-dir`.
   Unrecognized keys in `~/.study-guide/config` are ignored but emitted as warnings.
5. Embedded capture timestamp metadata is used for matching; assets without supported image/video capture metadata are skipped with a warning.
6. In default mode with Photos Library package input, candidate files are discovered from SQLite metadata time fields (windowed by session step envelopes) before EXIF reads.
6. Time-window matching rule is enforced:
- non-last step: `[step.time_started, implied_or_explicit_step.time_finished]` where implied `time_finished = next_step.time_started - 1 second` when omitted
- last step: `[last_step.time_started, last_step.time_finished]`
   Effective ownership for ingest uses `focus_windows` only; there is no ownership fallback to whole-step time windows.
   Protocol steps authored with `unfocusable: true` are excluded from ingest ownership matching and do not require `focus_windows`.
7. Assets are copied to the correct `step/<step-slug>/asset/` directory.
8. Output names follow `<YYYYMMDD-HHMMSS>_<sha8>.<ext>`.
9. Duplicate files are skipped based on content identity within each session.
10. Re-running ingestion on unchanged inputs produces no duplicate copies (idempotent behavior).
10. If a session is incomplete for ingest matching (for example a later protocol step file has not been created yet, or the final step has not been finished yet), ingestion warns that it skipped an incomplete session and continues processing every other session with valid timing windows.
11. If a session is invalid for ingest matching (for example malformed frontmatter/YAML or nonsensical timestamps/window bounds), ingestion warns that it skipped an invalid session and continues processing every other session with valid timing windows.
12. If a required step file exists but cannot be parsed, the invalid-session warning reports a step-file read error for that concrete path instead of claiming the file is missing.
13. Output includes per-session ingest counts and aggregate totals.
14. `sg data ingest --assets-dir <path>` is validated with a repository fixture asset set derived from `study-complete` images, with deterministic per-step placement assertions.
    The canonical fixture directory for that asset set is `fixtures/study-complete-assets/`.
15. `sg data ls` outputs one sorted row per ingested asset (`SESSION | STEP | FILE`) and an aggregate asset total.
16. `sg data ls` and `sg data clean` reconcile renamed ordinal step directory slugs from the current protocol before scanning session assets.
17. The repository concurrent-ingest fixture keeps study state and source media separate:
- study fixture: `fixtures/four-concurrently/` (session step `asset/` dirs empty before ingest)
- source media fixture: `fixtures/four-concurrently-data/`
- each source photo embeds metadata describing expected destination (`subject`, `step`) so tests can assert ingestion placement by metadata, not only by filename.
18. `sg data clean` removes all files under every `session/*/step/*/asset/` directory in the current study and prints a deterministic removed-file count.

### F. Status Reporting
1. `sg status` reports missing required frontmatter fields across study/session/step files.
2. `sg status` reports missing study markdown sections: `Hypotheses`, `Discussion`, `Conclusion`.
3. `sg status` reports sessions missing required step instances from protocol definition.
4. `sg status` reports steps missing `time_started`, and reports missing `time_finished` only for final protocol steps.
5. `sg status` outputs:
- a human-readable issue list
- an overall completeness result

### G. Publish
1. `sg publish` always attempts generation (best effort), even if data is incomplete.
2. Default outputs are:
- `<study-root>/publish/<study-folder-name>/site/index.html`
- `<study-root>/publish/<study-folder-name>/study.pdf`
- `<study-root>/subject-map.txt`
3. `sg publish <destination-dir>` writes publish output beneath `<destination-dir>/<study-folder-name>/`.
4. Generated outputs include:
- study title + metadata
- authored study preamble content from between the title H1 and `# Introduction`, preserved as authored before the `Introduction` section
- optional study hero comparison rendered as two side-by-side images when `study.sg.md` frontmatter `hero_comparison` is present
  - each side is resolved from `session` + `step` + `asset_index`
  - `asset_index` is resolved against that step's sorted source asset list, not by filename
- hypotheses, discussion, conclusion
- protocol summary + step list
- sessions in chronological order, including chrono-ordered thumbnails of photos
- subject labels are anonymized by default in HTML and PDF outputs using deterministic study-wide placeholders (`Subject 1`, `Subject 2`, ...)
- when subject names are anonymized, published web session paths/folder names must also be anonymized and must not reuse subject-derived study session slugs
- when subject names are anonymized, publish also writes `<study-root>/subject-map.txt` listing each deterministic placeholder and its real subject name; this file is not part of the published site and is not linked from it
- index-page thumbnails are published as separate derived image files rather than linking directly to full-size session assets
- HTML-published images must be browser-displayable; HEIC/HEIF assets are published as rendered preview images rather than raw HEIC references
- when a step sets `render_asset_indices`, publish renders only those step assets and in that explicit index order
- page per session
  - compact single-line toolbar with link back to index, session start date (unlabeled), subject label, and image-size controls
    - navigation controls are normal links labeled `Up`, `Prev`, and `Next`
    - toolbar includes previous/next session links when adjacent sessions exist
    - do not show `WIP` on the session page
    - js controls that scale the size of the images: working slider plus easy min/max buttons
      - slider spans the same range as the buttons: min 50x50; max 40vw
      - chosen image size persists across session pages
  - body supports toggling between step columns and step rows
    - default orientation is columns
    - toolbar includes radio-button controls to switch orientation
    - layout radios are plain controls, not button-styled boxes
    - chosen orientation persists across session pages
  - in column mode, body is a series of columns, one per protocol step, each with a small header (step name)
    - columns have no padding, minimal border, and no gutter between columns
      - enables photos to be compared side-by-side
  - in row mode, body stacks steps vertically as rows with the same compact styling
    - each row has a short step-name cell on the left and an image strip on the right
    - row-mode image width follows the same image-size control rather than stretching to full row width
    - row-mode images keep their aspect ratio and are not distorted by the size control
    - row-mode images are fully visible and are not clipped vertically
  - only a single small separation between the toolbar and the column area
    - do not use both a header/content border and extra padding to create that separation
  - small vertical padding between photos in columns;
5. `sg publish` runs status checks before rendering outputs.
6. If required sections/steps/fields are missing, `study.sg.md` is updated to `status: WIP`.
7. If incomplete, both HTML and PDF outputs visibly indicate `WIP` (but not as a global header)
8. If complete, study status is not downgraded to `WIP`.
9. If `study.sg.md` protocol content cannot be parsed, `sg publish` fails instead of silently rendering an empty protocol.
10. `sg publish` reuses previously rendered HTML image assets when the publish output is already up to date, and does not re-render unchanged HEIC/HEIF previews.
11. `sg publish` processes publish-image derivations concurrently so multiple HTML asset renders and index thumbnail renders can make progress in the same run.
12. `sg publish --with-subject-names` preserves the real subject-name behavior in both HTML and PDF outputs.
13. `sg publish` defaults to continuous watch mode and re-runs on study file/directory changes; `--once` preserves the one-off publish behavior.

### `sg export [<destination-dir>]`
Exports an anonymized study snapshot by copying the study hierarchy into a destination directory.

Behavior:
- defaults to a continuous mode that watches study files/directories and re-runs export when they change
- `--once` runs a single export pass and exits
- defaults destination to `<study-root>/export`
- writes the exported study snapshot beneath `<destination-dir>/<study-folder-name>/`
- generates derived thumbnails in the exported snapshot
- writes `<study-root>/subject-map.txt` listing each deterministic anonymized subject label and its real subject name
- default thumbnail size is `144`
- supports `--imgsize=x[,y,...]`; each requested size produces a derived image tree under each exported step's asset directory at `asset/img/<size>/`
- copies the study hierarchy as files/directories rather than rendering HTML/PDF
- preserves `study.sg.md`, `subject-requirements.yaml`, `session/`, step files, and non-HEIC assets
- excludes raw `.heic`/`.heif` files from the exported snapshot while still generating derived JPEG images for them under each exported step's asset directory at `asset/img/<size>/`
- does not copy derived output directories such as `publish/` or a pre-existing `export/`
- does not copy unrelated repo/tooling directories or metadata such as `.git/`, nested `.git/`, `bin/`, or `.tmux.workspace`
- anonymizes structured session subject references in exported `session.sg.md` files using deterministic study-wide labels (`Subject 1`, `Subject 2`, ...)
- removes subject UUIDs from exported session subject lines
- anonymizes exported session directory names to deterministic labels (`session-1`, `session-2`, ...) and rewrites `study.sg.md` frontmatter `active_session_slug` to the exported session slug when present
- preserves protocol content inside exported `study.sg.md` `# Methods` → `## Protocol`
- appends each exported session's structured data under `study.sg.md` section `# Results`, after any pre-existing results content
- preserves existing non-`# Results` sections in `study.sg.md`, including authored `# Methods` content
- when rebuilding into an existing export destination, temporarily moves the previous export aside into `/tmp` and reuses unchanged derived thumbnails from it while constructing the fresh snapshot
- prints `exported at <HH:MM:SS DD-MM-YYYY> to <destination>` after each successful export pass, including continuous watch re-runs
- leaves the source study unchanged

### H. Data Integrity and Safety
1. Commands modify only files they are responsible for.
2. No command deletes session assets unless explicitly requested by that command's contract.
3. Frontmatter remains parseable YAML after every command.
4. Existing user-authored markdown body content is preserved unless the command is explicitly responsible for that section.

### I. Export
1. `sg export` defaults output to `<study-root>/export/<study-folder-name>/`.
2. `sg export <destination-dir>` writes the exported study snapshot beneath `<destination-dir>/<study-folder-name>/`.
3. Export output preserves the study hierarchy and copies source markdown and assets rather than rendering HTML or PDF.
4. Export output excludes derived directories `publish/` and `export/`.
5. Export output excludes unrelated repo/tooling directories or metadata such as `.git/`, nested `.git/`, `bin/`, and `.tmux.workspace`.
6. Export anonymizes structured session subject references to deterministic study-wide labels and strips subject UUIDs from exported `session.sg.md`.
7. Export anonymizes exported session directory names to deterministic `session-N` labels.
8. Export rewrites `study.sg.md` frontmatter `active_session_slug` to the anonymized exported session slug when present.
9. Exported `study.sg.md` preserves the protocol inside `# Methods` → `## Protocol`.
10. Export appends each session's structured data beneath `study.sg.md` section `# Results`, after any pre-existing results content.
11. Export preserves existing non-`# Results` sections in `study.sg.md`, including authored `# Methods` content.
12. Export preserves authored study preamble markdown before `# Introduction`, including multiple paragraphs or optional italic notes when present.
13. Export writes derived images under each exported step's asset directory at `asset/img/<size>/`.
13. Export excludes raw `.heic`/`.heif` files from copied exported assets while still writing derived JPEG images for them under each exported step's asset directory at `asset/img/<size>/`.
14. Re-running `sg export` against an existing destination may reuse unchanged derived images from the prior export while still replacing the exported snapshot with a fresh copy.
15. `sg export` does not modify the source study files.
16. `sg export` defaults to continuous watch mode and re-runs on study file/directory changes; `--once` preserves the one-off export behavior.
17. Export writes `<study-root>/subject-map.txt` using the same deterministic anonymized subject labels used in exported session content.
18. Each successful export pass prints `exported at <HH:MM:SS DD-MM-YYYY> to <destination>` using local time.

### J. Automated Testing
1. The repository includes real Go unit tests (`*_test.go`) for core behavior.
2. At minimum, tests cover:
- frontmatter read/write and key ordering guarantees
- protocol step parsing and title extraction
- subject store create/edit/remove and subject resolution behavior
- status issue detection for missing required fields/sections
- ingest photo window matching and boundary behavior
- ingest duplicate/idempotency behavior
- data ingest command behavior using `--assets-dir` fixtures against `fixtures/study-eg`
- data ls output ordering and totals
3. `go test ./...` passes in a clean checkout.
4. Tests must not read from or write to the real global subject directory (`~/.study-guide/`); tests must use isolated temporary directories.
5. TUI behavior tests should prefer stable contract and snapshot-style assertions (rendered text states and key layout invariants) over many micro-assertions of individual style properties.
6. Tests should not rely on mutable repository fixture state (for example pre-populated asset counts in `fixtures/`); setup should generate required runtime state within the test (for example by running `sg data ingest` in a temp study).
