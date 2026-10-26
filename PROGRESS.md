# Progress

## Resume here — 2026-10-26 13:55

**Branch `feat/api-features`** (merged into `feat/vedabase-tui` and `main`): uses the seven new vedabase.cc API features (maintainer's reply 2026-10-26). Race suite green; both Opus reviews approved (phases 0–2 after one fix round; phases 3–4 with minors); tmux smoke run passed.

**What's in it:**
- Phase 0: new fields (prev/next, references, iso_date, recipients, audio_url, address); raw per-language item cache `items-v2/<lang>/`, old `items/` served read-only when a fetch fails; letter cleanup only for old-format letters.
- Phase 1: `h`/`l` follow the site's prev/next; `[[ref|label]]` shows as the label; O4 (Tab re-runs the query) and O1 (transport timeouts) fixed.
- Phase 2: book contents from `/books/{code}/toc` (cached a day, background queue under 10/min); Letters › By year from `/letters`; HTML scraping removed; O3 fixed.
- Phase 3: references underlined and clickable; `f` shows Vimium-style key labels; Backspace/`[` back, `]` forward.
- Phase 4: languages en/es/pt/hi/ru via the `EN ▾` tag in the sidebar's top edge or `L`; books missing a language are dimmed `· not in XX` and show a note, never English.

**Released:** v0.2.0 on 2026-10-26 (7 assets; `main`, `feat/vedabase-tui` and `feat/api-features` pushed). README credits vedabase.cc as the source.

**Next action:** none pending; ask the user what's next (the audio player is the parked idea).

**Out of scope, future:** a terminal audio player for lectures (via mpv) and "Lectures on this verse" (index built from lecture titles; maybe ask the maintainer for a verse → lectures field). Collapse button not wanted.

**Deferred minors:** ledger `.superpowers/sdd/2026-10-26-api-features/progress.md` (review 1 #7, #10; review 2 #3, #5, #8, #9, #10). Older ones: `.superpowers/sdd/2026-10-25-vedabase-tui/progress.md`.

**Identity, release steps, backup:** in git-ignored `PROGRESS.local.md` (read it too).

**Older deferred minors:**
- **Update review:** 9 (the 3-minute timeout covers the whole download), 11 (`hash *file` checksum lines), 12 (the toast queue is cleared, the progress pair isn't atomic).
- **Earlier:** listed in the ledger `.superpowers/sdd/2026-10-25-vedabase-tui/progress.md`, which is git-ignored and also holds the briefs and reports.
- **Stale hashes:** the commit hashes in that ledger predate the history rewrite.

**Docs:** specs `docs/superpowers/specs/2026-10-25-vedabase-tui-design.md` and `…/2026-10-26-api-v1-features-design.md`; plans in `docs/superpowers/plans/`. Implementer briefs must say: tests first, no `git stash`, stub the App opener in UI tests, and the commit trailer is Claude Opus 5.5.
