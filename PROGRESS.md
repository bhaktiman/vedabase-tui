# Progress

## Resume here — 2026-10-25 13:43

**Branch:** `feat/vedabase-tui` (local, not pushed; `main` has only the docs). HEAD `6f9b9bd` plus this handoff commit. Working tree clean. Race suite green and `./vedabase` built.

**Status:** plan Tasks 1–17 are done, including the manual smoke run (12/12 pass) and the whole-branch Opus review. The review found 1 blocking issue (numeric entities that decode to ESC). It's fixed, including the int32-wrap case, and re-checked by a fresh Opus agent: closed. The user's mid-build requests are done as Tasks 18–28: mouse and focus, collections in the sidebar plus `z`, themes with a live `T` picker, per-theme accent and gradient, centred column `w`, spinner, no fade, `o` and the vedabase.cc button and tag, gradients (the scroll marker is plain), full book name and link in exports, bottom-bar hints, `H` hints and `F` zen, and search box → results with ctrl-n/p.

**Next action:** ask the user the three open questions below, then build whatever they say yes to (one subagent at a time; these all touch internal/ui).

**Waiting on the user:**
1. **Letter footer junk.** API letters end with the site's prev/next links glued on, e.g. `…ACBS/hsLetter to: G. L. KapoorLetter to: Sri Paramanand Patel` (seen in `letters/letter-to-lokanatha-2`). The export link itself works (checked: 200, right page). Proposed: strip the trailing `Letter to: …` nav and the `ACBS/xx` initials in `api.Clean`. I asked whether "the URL doesn't work" meant this line or the link itself, and in which app. No answer yet.
2. **Collapse button.** `⊟ collapse (z)` in the sidebar's top border (clickable, shrinks to `⊟ (z)`), plus `z collapse` in the bottom-bar hints. The user rejected shift-tab for this; I recommended against it. Awaiting yes.
3. **Five smoke-run fixes.** Proposed, awaiting yes:
   - O5: overlays drawn over the dimmed app, so the theme preview is visible behind the picker
   - O3: Letters › By year jumps the tree to By recipient (same ref in two groups; match the group the way SetLibrary does for collections)
   - O7: search snippet shown twice
   - O4: Tab in search doesn't re-run the query
   - O1: offline uncached load waits 8–30 s (fail fast on connection refused)

**Then:** superpowers:finishing-a-development-branch (merge to main or PR, the user's choice), and tell the user every `Ruling:` line in the ledger.

**Where things are:**
- Ledger, the source of truth for done items, rulings and minors (git-ignored): `.superpowers/sdd/2026-10-25-vedabase-tui/progress.md`. Briefs and reports are in the same folder (`task-N-brief.md` / `task-N-report.md`, `branch-review.md`, `review-fix-*.md`, `task-17-smoke-report.md`, `implementer-contract.md`).
- Spec: `docs/superpowers/specs/2026-10-25-vedabase-tui-design.md`. Plan: `docs/superpowers/plans/2026-10-25-vedabase-tui.md`.
- Implementer briefs must say: tests first with RED shown, no `git stash`, stub the App opener in UI tests (a review test once opened real browser tabs), and the commit trailer is Claude Opus 5.5.

**Known minors still open (deferred):** listed in the ledger. Among them: Bg-on-Accent cursor contrast low on Solarized Light, Monokai and Solarized Dark; persist fsyncs inside Update; scroll within an item isn't saved on SIGHUP; review minors 7, 9, 11 and 12.
