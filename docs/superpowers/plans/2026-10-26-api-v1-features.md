# Plan: new API features

Spec: `docs/superpowers/specs/2026-10-26-api-v1-features-design.md` (cited as §n).
Branch: `feat/api-features` (from `feat/vedabase-tui` @ b74c959).
Each task is one implementer brief, sized for ~30 turns. Tasks run in order; each
ends with `make test`, `go vet ./...` and the race suite green, then one commit
(or a few small ones).

Fixtures: record once with curl into the package's `testdata/` (the only network
use allowed). Live API base `https://vedabase.cc/api/v1`.

---

## T1 — Phase 0: fields, language, new cache dir, letter guard (§6.1)

- `api.Item`: `Prev`, `Next string`; `References []Reference` (`Text`, `Ref`,
  `Field`; JSON null → ""). `api.Document`: `IsoDate`, `Recipients []string`,
  `AudioURL`, `Address`. `Clean` sanitizes all new strings.
- `Client.Verse(ctx, ref, lang)`, `Client.Books(ctx, lang)`,
  `Client.Search(ctx, q, book, page, lang)`; the URL carries `lang=<lang>`.
- Service methods take `lang` as an argument (`Item(ctx, ref, lang)`,
  `Cached(ref, lang)`, `Search(ctx, q, book, page, lang)`, `Books`); no mutable
  language field. `letters/…`/`transcripts/…` refs always use `en` (helper
  `langFor(ref, lang)`). The UI passes `"en"` everywhere for now (T7 adds the
  setting); `itemLoadedMsg` and search result messages carry the `lang` they
  were made with.
- Item cache: `<cache>/items-v2/<lang>/<ref>.json` stores the **raw response
  bytes** (decode on read; the stored copy's `ref` is the requested ref — keep
  today's "store under the ref we asked for" behaviour, e.g. by keying the file
  on the requested ref without rewriting the bytes). Offline fallback: miss in
  `items-v2/en/` + failed fetch → read old `<cache>/items/<ref>.json`
  (English only, read-only). Wire where the ItemCache is built (probably
  `cmd/vedabase/main.go`).
- `cleanLetter` runs only on the old header shape (§6.1).
- **Acceptance tests (write first):**
  - decode a recorded `bg/2/13` (has prev/next, empty refs), a recorded lecture
    `transcripts/680317bgsf` (3 wiki refs, audio_url, iso_date) and a recorded
    new letter `letters/letter-to-lokanatha-2` (recipients, address); assert the
    fields.
  - `Clean` on the new letter returns the purport unchanged; on a new letter
    whose body contains `68-10-13` in the first 400 chars, unchanged; existing
    old-shape tests still pass.
  - client sends `lang=es` when asked (httptest captures the query).
  - service: `Item(ctx,"bg/2/13")` with Lang `es` stores under
    `items-v2/es/bg/2/13.json`; a letter with Lang `es` requests `lang=en` and
    stores under `items-v2/en/…`; an item present only in the old `items/` dir
    is fetched fresh when online, and served from `items/` when the fetch fails
    with `ErrOffline` (en only; with lang `es` it is not served); a cached
    item with an unknown extra JSON field keeps it on disk (raw bytes).
- Commit: `feat(api): read prev/next, references and new document fields; per-language item cache`

## T2 — Phase 1 + smoke fixes (§6.2, §9)

- `h`/`l`: use the shown item's `Prev`/`Next` when non-empty, else today's
  `library.Prev/Next`; inside a collection keep today's behaviour. Tree cursor
  follows like `:` goto. Prefetch uses `Next` the same way.
- `render.DisplayMarkdown`: `[[ref|label]]` → `label`, `[[ref]]` → `ref`.
- O4: Tab/Shift-Tab in search re-runs a non-empty query in the new scope.
- O1: measure first (§9), then transport timeouts only (dial, TLS handshake,
  response header) in the default `http.Client` of `api.NewClient`. No circuit
  breaker.
- **Acceptance tests:** `l` on an item with `Next: "bg/2/14"` opens bg/2/14 even
  when the library's Next differs; with empty `Next`, falls back; wiki-link
  rendering (`[[bg/9/4|*Bg.* 9.4]]` → `*Bg.* 9.4` in the Markdown); Tab with a
  query issues a new search command; the default client's transport has the
  dial/TLS/header timeouts set (unit test on the constructed transport), and a
  request to a listener that accepts but never answers fails within the header
  timeout (use a short override in the test).
- Commits: `feat(ui): h/l follow the site's prev/next`, `feat(render): show inline links as their label`, `fix(search): tab re-runs the query in the new scope`, `fix(api): give up quickly when the network is down`

## T3 — Phase 2a: TOC + listings in api/service/library (§6.3)

- `api.TOC` / `api.TOCNode` (`Ref`, `Title`, `Label`, `Children`; null → "",
  null children → nil); `api.Listing` (`Total, Page, PerPage, HasMore, Undated,
  Years []YearCount, Items []ListingEntry{Ref, Title, IsoDate, Year, Location,
  Recipients}`), sanitized.
- `Client.TOC(ctx, code, lang)` with its own limiter (`rate.Every(7*time.Second)`,
  burst 2); `Client.Letters(ctx, year, page, perPage)` (and `Transcripts` same
  shape) on the verse limiter.
- Service: `TOC(ctx, code)` cached as blob `toc/<lang>/<code>.json`, fresh 24 h,
  stale used on network error; `CachedTOC(code) (api.TOC, fresh bool, ok bool)`
  disk-only; `LetterYears(ctx)` / `LetterYearPage(ctx, year, page)` cached as
  `list/letters/<year>-<page>.json` (years from page 1 of any query, e.g.
  `list/letters/years.json`), fresh 7 days, stale on error.
- `library.(*Library).ApplyTOC(code string, t api.TOC) bool` per §6.3: replace
  the root's children, keep order, titles/labels, rebuild index + flat list for
  that book. Old `*Node`s of that book are discarded: callers re-find by ref. Node labels: a node
  with `Title` shows it; a leaf with only `Label` shows the label as today's
  leaf labels look (check `order.go`; e.g. `1.1` → today's style).
- **Acceptance tests:** recorded `toc/bg` (en) and `toc/iso`, a trimmed `toc/sb`
  fixture (one canto, two chapters, front matter incl. a null-ref node) and a
  `toc/cc` slice; ApplyTOC: order equals TOC order, `Find("bg/1/16-18")` is a
  leaf, chapter titles, front-matter leaf titles, Next across chapter boundary
  uses TOC order; service TOC freshness (fresh hit = no
  request; stale + network error = stale returned; stale + success = refreshed);
  limiter for TOC is separate (a TOC call doesn't consume the verse limiter —
  test via counting or by construction).
- Commit: `feat(api): table of contents and letter listings`

## T4a — Phase 2b: book contents from the API in the UI (§6.3)

- At startup apply every cached TOC for the current language before the first
  frame; after it, a background queue fetches missing/stale TOCs one at a time
  (current book first, expanded books, then `/books` order). Expanding an
  un-TOC'd book moves it to the front. Replacing a book's children keeps the
  cursor on the same ref (else the book root) and keeps expansion state where
  refs still exist.
- Every holder of a `*Node` of the replaced book is re-found by ref after
  ApplyTOC: tree cursor/rows, current item node, filter results, collections
  view (collections nodes are separate — check), reading-order position.
- Chapter-title scraping is no longer called for books that have a TOC.
- **Acceptance tests:** cached TOC applied on launch without network (fake
  backend); queue order (current book first; expand bumps); cursor and current
  node preserved across ApplyTOC; a TOC result for another language is dropped.
- Commit: `feat(ui): book contents from the API`

## T4b — Phase 2b: letters by year from the API; remove scraping (§6.3)

- Letters › By year from `LetterYears`/`LetterYearPage` (per_page 100), paging as
  today; labels from `title` + `iso_date` + `location`.
- Delete `internal/scrape`, the chapter-title fetching and the scraped year
  plumbing in `internal/ui/scraped.go` (keep whatever item-title code is still
  used), `Service.Page` if unused. Fix O3 if it reproduces; add a test that
  opening a By-year letter leaves the cursor under By year.
- **Acceptance tests:** By year shows years from the listing and loads page 2
  near the end; O3 test; no code imports `internal/scrape`.
- Commits: `feat(ui): letters by year from the API`, `refactor: remove HTML scraping`

## T5a — Phase 3a: link spans in the rendered text (§6.4)

- Link spans from `references` per §6.4 (boundary rules, wiki links, parts
  toggles). Sentinel marking → Glamour → strip + record hit regions + underline
  in accent. Cached with the render. `render` returns the hit regions with the
  output; the reader keeps them in screen-independent form (rendered line, col
  range, target ref).
- **Acceptance tests:** matching (`Bg. 4.7` not in `Bg. 4.70`; null-ref skipped;
  wiki link matched and shown as label; link in a hidden part absent); hit
  regions correct for a link wrapped across two lines at width 40; rendered
  text equals the no-link render once sentinels and styling are stripped
  (widths unchanged); a 100k-char lecture fixture renders with links in under
  2× the time without.
- Commit: `feat(render): underline references and record where they are`

## T5b — Phase 3a: click to follow, back and forward (§6.4)

- Click on a link opens it; history back stack (`Backspace`, `[`) / forward
  (`]`), scroll restored, max 100, forward cleared on non-link opens; bottom bar
  `⌫ back` hint.
  Screen mapping accounts for the sidebar width, reader padding, centring
  (`reading_width`), scroll offset and zen mode.
- **Acceptance tests:** click opens (with sidebar shown and hidden, centred
  and full width); back restores ref+offset; forward; forward cleared after `l`;
  `⌫ back` hint shown only with history.
- Commit: `feat(reader): clickable references with back and forward`

## T6 — Phase 3b: `f` hints (§6.4)

- Reader-focus `f` → hint mode over visible links; labels per §6.4; prefix
  narrowing; Esc/non-label cancels; "No links on screen" toast; help + README.
- **Acceptance tests:** labels assigned in order to visible links only; 20
  links → two-letter labels; typing a label opens it and pushes history; tree
  focus `f` still filters; Esc cancels.
- Commit: `feat(reader): f shows key labels on links`

## T7 — Phase 4a: language plumbing (§6.5)

- `state.json` `lang` (default `en`, validated against en/es/pt/hi/ru).
- Service language switch (`SetLang`), TOCs/items per language; books'
  `languages` exposed; reader note for books without the language and for a
  404 in a language (no English fallback); search sends lang (en for
  letters/transcripts scope); export front matter `lang:`.
- **Acceptance tests:** switching to `es` reopens the current item with
  `lang=es`; a book without `hi` shows the note, never an English request for
  that book; letters still `en`; export front matter has `lang: es`.
- Commit: `feat: read the library in another language`

## T8 — Phase 4b: language picker in the sidebar (§6.5)

- `EN ▾` tag in the sidebar's top border next to the version; click or `L`
  opens a picker like the theme picker; tree dims books without the language
  with `· not in <CODE>`; reader header `EN` tag on letters/transcripts under
  another language; help + README.
- **Acceptance tests:** `L` opens picker; picking `es` calls the switch and
  persists; click on the tag opens it; dimmed suffix present for `ej` under
  `hi`.
- Commit: `feat(ui): language picker in the sidebar`

## T9 — Branch review and release prep

- Coordinator: full `make test`, race, `make build`, a tmux smoke run (links,
  `f`, back, language switch, letters by year, offline start).
- One Opus review of the branch; one fix round if needed.
- Release v0.2.0 only with the user's go-ahead.
