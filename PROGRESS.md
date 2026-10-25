# Progress

## Resume here — 2026-10-25 11:57

**Branch:** `feat/vedabase-tui` (local, not pushed; `main` has only the docs). **Status:** plan Tasks 1–16 done and checked by hand in tmux; Task 17's README and static build are done. The user asked for three more things mid-build, which became Tasks 18 and 19. Task 18 was **running in a background subagent at handoff**.

**Next action:** check whether Task 18 landed. Run `git log --oneline -3` and look for `feat(ui): mouse support and clear pane focus`.
- **It landed:** read its report, run `CGO_ENABLED=1 go test -race ./...`, try the mouse and the focus border in tmux, then dispatch Task 19.
- **It didn't:** the working tree holds its partial edits (README.md and internal/store/settings.go were modified at handoff). Re-dispatch Task 18 with its brief and report path. Don't commit the half-finished edits as they are.

**Remaining after Task 18:**
1. **Task 19:** collections as the first sidebar root (★ Collections), and `z` to collapse all. The brief is ready.
2. **Task 17, Step 3:** the manual smoke run in tmux, using isolated XDG dirs under the session scratchpad. The plan lists the checks.
3. **Final Opus review of the whole branch:** run `review-package` over the range from `c99a5e6` to HEAD, and point the reviewer at the ledger's "minor (deferred)" lines. Then one fix wave and one scoped re-review.
4. Use superpowers:finishing-a-development-branch, then tell the user every `Ruling:` line in the ledger.

**Where things are:**
- Spec: `docs/superpowers/specs/2026-10-25-vedabase-tui-design.md`
- Plan: `docs/superpowers/plans/2026-10-25-vedabase-tui.md`
- The ledger, git-ignored but on disk, is the source of truth for done/rulings/minors: `.superpowers/sdd/2026-10-25-vedabase-tui/progress.md`.
- That same folder holds the briefs and reports: `task-N-brief.md`, `task-N-report.md`, plus the extra `task-18-brief.md` and `task-19-brief.md`, and `implementer-contract.md`, which every dispatch references.
- Review policy (user's global CLAUDE.md): no per-task reviewer. The coordinator reads each diff and runs the tests, plus one Opus review per branch. Task 6 (collections) already had its own Opus review and passed after one fix round.

**User-visible decisions to report at the end** (the full list is in the ledger):
- Letters from the API arrive with their line breaks stripped. `api.Clean` rebuilds paragraphs; the cache keeps the raw text.
- Terminal control characters are stripped from all remote text.
- Mouse mode means Shift+drag to select text; `mouse = false` turns it off.
- A fresh launch opens at Bg Dedication (0a).
