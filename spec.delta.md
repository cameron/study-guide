# Spec Delta Notes

- Smoke test finding: `sg status` reported `complete`, but `study.sg.md` still had frontmatter `status: WIP` after `sg publish`.
- Decision needed: define canonical behavior for study status transitions when data is complete.
  - Option A: `sg publish` (or `sg status`) updates `study.sg.md status` to `concluded` when no issues remain.
  - Option B: keep `status` manual-only and ensure `sg status` output clearly distinguishes computed completeness vs declared study status.
