# Refactor Audit Plan

## Why this document exists

The current CLI works, but the interactive implementation has drifted away from the spec in a few important places. The most significant drift is architectural: several commands compose multiple Bubble Tea programs, while the spec requires a single long-lived program per interactive command with internal state transitions.

This document is a handoff plan for another agent. It is intentionally scoped to clear, high-leverage refactors rather than broad cleanup.

## Primary divergences and smells

### 1. Multiple interactive commands violate the single-program architecture

Spec:
- `spec.md` says interactive workflows should run in a single long-lived Bubble Tea program per command invocation.
- It also says transitions should be internal model state changes, not nested `tea.NewProgram` launches.

Current implementation:
- `src/internal/cli/run.go`
  - `cmdInit()` runs `runFormRunner(...)` and then `runProtocolTitlesPromptRunner()`.
  - `cmdSession()` runs `selectSubjectsForSession(...)` and then repeatedly runs `runSelect(...)` inside the protocol loop.
- `src/internal/cli/ui_session_create_picker.go`
  - `runSessionCreatePicker()` loops and launches `runInteractiveProgram(...)` repeatedly.
  - It then calls `subjectCreateWithStudyRoot(...)`, which launches another interactive form.
- `src/internal/cli/ui_sessions.go`
  - `handleCreateEnter()` calls `subjectCreateWithStudyRoot(...)` from inside the switchboard.

Why this matters:
- It creates redraw/terminal handoff bugs.
- It makes alt-screen ownership ambiguous.
- It pushes command flow into nested control flow instead of explicit state.
- It makes tests rely on stubbing global runners instead of exercising command-level behavior.

### 2. The create-session picker exists twice

Current implementation:
- `src/internal/cli/ui_session_create_picker.go`
- `src/internal/cli/ui_sessions.go`

Both implement:
- subject selection state
- filter behavior
- `(+) New subject`
- `-> Create Session`
- create shortcut logic
- create-mode rendering

Why this matters:
- Behavior can drift between `sg session` and `sg sessions`.
- Bugs need to be fixed twice.
- The shared spec behavior is harder to enforce because the implementation is not actually shared.

### 3. `run.go` mixes command routing, orchestration, persistence, and terminal output

Current implementation:
- `src/internal/cli/run.go` contains:
  - CLI dispatch
  - subject creation/edit orchestration
  - session progression orchestration
  - direct `fmt.Println`/`fmt.Printf`
  - filesystem writes
  - interactive prompts

Why this matters:
- It is hard to reuse logic from interactive models without calling command handlers.
- UI models end up calling command helpers that print and launch programs.
- Testing is forced toward global function replacement instead of narrow dependency seams.

### 4. Interactive flow boundaries are implemented with command functions instead of pure domain operations

Example:
- `subjectCreateWithStudyRoot(...)` both:
  - derives requirements
  - runs the form
  - saves the subject
  - prints output

The UI only needs "create a subject from collected values" and "load subject-create requirements", but the available helper launches an entire command flow.

Why this matters:
- The `sg sessions` create flow cannot embed subject creation cleanly.
- Behavior like success messages and screen clearing becomes a side effect problem.

### 5. Terminal clearing is split between Bubble Tea and raw ANSI

Current implementation:
- `src/internal/cli/ui_program.go` owns the Bubble Tea program wrapper and alt-screen behavior.
- `src/internal/cli/run.go` has `clearTerminalScreen()` which writes raw ANSI to stdout.

Why this matters:
- Terminal behavior is not centralized.
- Transitional fixes tend to become more ad hoc.
- The codebase already has one place that is supposed to own interactive program behavior.

### 6. Some tests enforce source-shape instead of behavior

Current implementation:
- `src/internal/cli/ui_program_test.go` asserts there is exactly one textual `tea.NewProgram(` call in the package.

Why this matters:
- This catches one class of drift, but it is brittle and does not prove command-level single-program behavior.
- It can still pass while commands create nested flows through helper wrappers.

## Recommended refactor direction

Do not try to rewrite every interactive command in one pass. Refactor one command family at a time, starting with the highest-value path.

### Phase 1: Fix the session/create-subject architecture

Goal:
- Make the `sg sessions` create flow a true single-program flow.

Steps:
1. Introduce a command-local state machine for the switchboard with explicit substates for:
   - browse
   - create session
   - create subject
2. Stop calling `subjectCreateWithStudyRoot(...)` from `sessionsSwitchboardModel`.
3. Extract pure helpers from the subject-create command:
   - load requirements
   - build form fields
   - save a subject from collected values
4. Reuse the existing `formModel` as a child model inside the switchboard rather than launching a nested program.
5. On successful subject creation:
   - reload subjects
   - return to create-session mode
   - preserve selected subjects
   - explicitly reset the visible create list state in-model

Acceptance criteria:
- No nested interactive program call from `src/internal/cli/ui_sessions.go`.
- Creating a subject from the create-session flow never leaks stale rows.
- The interaction path is covered by a behavior test, not just a view snapshot.

### Phase 2: Remove the duplicate create-session picker

Goal:
- Have exactly one implementation of the shared create-session picker behavior.

Steps:
1. Extract the shared selection/create-subject/create-session behavior into one model or one reducer-like state object.
2. Make `sg session` and `sg sessions` use the same implementation.
3. Keep only the view-shell differences that are genuinely command-specific.

Preferred shape:
- A reusable child model, not a helper that runs its own Bubble Tea program.

Acceptance criteria:
- Filtering, selection, shortcut keys, and new-subject action are tested once against the shared implementation.
- `src/internal/cli/ui_session_create_picker.go` either disappears or becomes a thin wrapper around shared state/view code.

### Phase 3: Convert `sg session` into one long-lived interactive command

Goal:
- Bring `cmdSession()` into alignment with the spec.

Current problem:
- It chains subject selection and repeated confirm prompts as separate interactive programs.

Steps:
1. Build a single `sessionRunModel` with states for:
   - subject selection
   - step confirmation
   - completion confirmation
   - done/canceled
2. Reuse the shared create-session picker from Phase 2 for subject selection.
3. Replace `runSelect(...)` confirm prompts with in-model confirmation screens.
4. Keep filesystem operations in pure helpers called from the model.

Acceptance criteria:
- `cmdSession()` launches one Bubble Tea program.
- Protocol progression no longer depends on repeated `runSelect(...)` calls.

### Phase 4: Convert `sg init` into one long-lived interactive command

Goal:
- Bring `cmdInit()` into alignment with the spec.

Current problem:
- It launches a form program and then a protocol-title program sequentially.

Steps:
1. Build an `initModel` with states for:
   - study metadata
   - protocol steps
   - done/canceled
2. Reuse `formModel` and `protocolTitlesModel` as child states instead of separate top-level programs.
3. Keep file writing at the edge after the model completes.

Acceptance criteria:
- `cmdInit()` launches one Bubble Tea program.
- Existing init behavior and defaults remain intact.

### Phase 5: Pull orchestration out of `run.go`

Goal:
- Make command handlers thin and make interactive models depend on narrow services/helpers.

Suggested extraction targets:
- `src/internal/cli/subject_service.go`
  - subject requirements loading
  - field shaping
  - subject persistence from collected values
- `src/internal/cli/session_service.go`
  - create session scaffold
  - advance/reverse helpers
  - focus window sync helpers
- `src/internal/cli/terminal.go`
  - any raw terminal clearing that still remains after the UI refactor

Acceptance criteria:
- `run.go` mostly dispatches commands and calls small orchestration functions.
- Interactive models do not call command handlers.

### Phase 6: Tighten tests around behavior, not implementation trivia

Goal:
- Make the tests protect the architecture that matters.

Steps:
1. Replace or supplement `ui_program_test.go` string-count assertions with behavior tests proving that commands do not relaunch top-level programs within one flow.
2. Add regression tests around parent-child screen transitions:
   - create session -> create subject -> back to create session
   - init study-name step -> protocol drafting step
   - session picker -> step confirm -> next step
3. Prefer tests around model state transitions and visible outputs over source scanning.

## Execution order

Recommended order:
1. Phase 1
2. Phase 2
3. Phase 3
4. Phase 4
5. Phase 5
6. Phase 6

This order reduces immediate bug risk first, then removes duplication, then aligns the remaining interactive commands with the spec.

## Guardrails for the follow-on agent

- Do not add compatibility shims for both architectures unless a transition absolutely requires it.
- Keep the shared picker genuinely shared; do not fork it again for `sg session`.
- Prefer adding small pure helpers over adding more global runner variables.
- When adding tests, center them on expected behavior and state transitions.
- Preserve the existing visual contracts already codified in `spec.md` and the UI tests unless the spec is explicitly updated first.
