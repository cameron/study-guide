# TUI Table Refactor Review

## Findings (ordered by severity)

1. Spec mismatch: selected-row tint is not implemented.
- Spec requires adaptive selected-row tint with exact colors (`spec.md` lines 258-259).
- Implementation clears selected row styling entirely (`src/internal/cli/ui_sessions.go` lines 117-123).
- Tint constants exist but are unused (`src/internal/cli/ui_sessions.go` lines 111-115).
- This likely blocks clarity for multi-row interaction and future visual features.

2. Rendering pipeline is fragile due to string-token plus regex post-processing.
- Action/focus styling is done by injecting control markers in cell text, rendering table, then regex-rewriting full output (`src/internal/cli/ui_sessions.go` lines 653-665).
- `focusedTokenPattern` can match real content containing `{...}` anywhere in rendered output (`src/internal/cli/ui_sessions.go` line 65).
- This gets harder to maintain as cell content and states become richer.

3. `renderEntryRow` has hidden state coupling.
- Row rendering queries current selection from table cursor/model state on each row render (`src/internal/cli/ui_sessions.go` lines 557-585).
- Row output depends on mutable external state, not just row data plus explicit selection context.
- This increases bug surface for features like row-level modes and additional actions.

4. Create-mode list rebuild discards list model state on every toggle.
- `refreshCreateList` recreates the entire list model each time (`src/internal/cli/ui_sessions.go` line 609).
- This risks cursor/scroll jitter and makes advanced create-flow behavior harder.

5. UI model owns too much domain/IO behavior.
- `handleBrowseEnter` and refresh paths perform filesystem/domain operations directly (`src/internal/cli/ui_sessions.go` lines 302-409).
- This tightly couples TUI rendering, orchestration, and persistence, slowing feature iteration and testing.

## Refactor opportunities

1. Introduce a `BrowseRowVM` builder.
- Pure function: `sessionRecord + selection/action state -> render model`.
- No table cursor access inside row rendering.

2. Replace post-render regex styling with explicit cell styling.
- Keep focus/action state as structured fields and style at cell construction time.
- Remove delimiter-token protocol (`\x1e...\x1f`) entirely.

3. Split switchboard into layers.
- Add a `sessionsService` for load/activate/advance/create workflows.
- TUI model dispatches intents and renders returned state.

4. Stabilize create list updates.
- Update items in-place or preserve cursor/scroll index when rebuilding.

5. Align tests to contract snapshots over style internals.
- Add browse-view snapshots for key states: empty, invalid, active, next-step focus, filtered.
