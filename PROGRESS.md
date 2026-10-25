# Progress

## Resume here — 2026-10-25 15:30

**Published:** public at https://github.com/bhaktiman/vedabase-tui. `main` and `feat/vedabase-tui` point at the same commit and both are pushed. Latest release is **v0.1.1**, which has the curl installer, binaries for Linux, macOS and Windows, and `checksums.txt`. Race suite green; working tree clean.

**This session:**
- **History:** rewritten so all commits are by bhaktiman.
- **Repo setup:** MIT license, an in-depth README with a screenshot and badges, and a one-line curl install (works in WSL too).
- **Release tooling:** `make release VERSION=vX.Y.Z`, then `gh release create` (steps in `PROGRESS.local.md`).
- **In-app updates:** the sidebar tag shows the version, the app checks GitHub on launch, and `U` swaps the binary and restarts. The update code passed an Opus review after one fix round, and I tested it live from v0.1.0 to v0.1.1.

**Identity, release steps, backup:** in git-ignored `PROGRESS.local.md` (read it too).

**Next action:** ask the user the three still-open questions below, then build what they approve.

**Waiting on the user** (carried over, not raised this session):
1. **Letter footer junk.** API letters end with the site's prev/next links glued on, e.g. `…ACBS/hsLetter to: G. L. Kapoor…` (`letters/letter-to-lokanatha-2`). Proposed: strip the trailing `Letter to: …` and the `ACBS/xx` initials in `api.Clean`.
2. **Collapse button.** Proposed `⊟ collapse (z)` in the sidebar's top border, plus a `z collapse` hint. Awaiting yes.
3. **Five smoke-run fixes:**
   - O5: overlays over the dimmed app
   - O3: Letters › By year jumps to By recipient
   - O7: duplicate search snippet
   - O4: Tab in search doesn't re-run the query
   - O1: offline uncached load waits 8–30 s

**Deferred minors:**
- **Update review:** 9 (the 3-minute timeout covers the whole download), 11 (`hash *file` checksum lines), 12 (the toast queue is cleared, the progress pair isn't atomic).
- **Earlier:** listed in the ledger `.superpowers/sdd/2026-10-25-vedabase-tui/progress.md`, which is git-ignored and also holds the briefs and reports.
- **Stale hashes:** the commit hashes in that ledger predate the history rewrite.

**Docs:** spec `docs/superpowers/specs/2026-10-25-vedabase-tui-design.md`, plan `docs/superpowers/plans/2026-10-25-vedabase-tui.md`. Implementer briefs must say: tests first, no `git stash`, stub the App opener in UI tests, and the commit trailer is Claude Opus 5.5.
