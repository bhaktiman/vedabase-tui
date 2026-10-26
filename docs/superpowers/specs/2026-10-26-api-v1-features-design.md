# vedabase-tui: new API features — design

Date: 2026-10-26. Builds on `2026-10-25-vedabase-tui-design.md` (the "base
spec"); everything not changed here stays as the base spec says.

## 1. Intent

The vedabase.cc maintainer answered our feature request: seven additions are
live on `https://vedabase.cc/api/v1/` (docs: `https://vedabase.cc/en/api/`).
This round puts them into the app so that:

- quoted verses in purports, letters and lectures can be followed and come back from;
- the whole library can be read in Spanish, Portuguese, Hindi or Russian;
- the sidebar's contents, titles and letter years come from the API instead of
  scraped HTML;
- reading straight through follows the site's own order.

The user approved all of it (phases 0–4 below) plus two leftover smoke fixes,
and asked that the work go ahead without further questions.

## 2. What the API now gives (verified live 2026-10-26)

- **`/verse/{ref}`** adds `prev`, `next` (ref or null) and `references`:
  `[{text, ref, field}]`. `text` is the exact string as it appears in `field`
  (`translation` or `purport`); `ref` is a `/verse` ref or null (work not on the
  site). No offsets. Some texts carry inline links `[[bg/7/1|Bg. 7.1]]` (label can
  contain `*…*`); for those, `text` is the whole `[[…]]` string. Up to 100
  distinct references per answer.
- **`document`** adds `iso_date` (`1968-03-17`, `1968-03` or `1966`), `year`
  (now filled for lectures too), `recipients` (letters, list), `audio_url`
  (lectures), `address` (letters).
- **Letters' `purport`** is now just the letter, with paragraph breaks, from the
  salutation to the signature; `ACBS/hs` on its own line. About 270 memos keep
  their old opening. Sampled 60: all start "My Dear…"/"Dear…" with newlines;
  one has a glued place (`DallasDear Ekendra dasa;`).
- **`GET /books/{code}/toc?lang=`**: `{book, lang, title, children:[{ref, title,
  label, children}]}` in site order; leaves carry `label` (`1.1`,
  `madhya.1.1`, `0a`); combined verses as `/verse` expects. sb is ~336 KB.
  `Cache-Control: max-age=86400`, **10 requests/minute**. Letters and lectures
  have no TOC. Every node has a `ref` (checked bg and sb: no nulls); leaves may
  carry `title` as well as `label` (front matter: `{"ref":"sb/0a","title":"Preface","label":"0a"}`).
- **`GET /letters`, `GET /transcripts`**: `year`, `recipient` (letters), `page`,
  `per_page` (≤100). Answer: `{total, page, per_page, has_more, years:[{year,
  count}], undated, items:[{ref, title, iso_date, year, location, recipients}]}`.
  No filter by verse (`covers=` is ignored).
- **Languages**: `lang` ∈ `en es pt hi ru`. `/books` lists each book's
  `languages` (ej, ssr lack `hi`; lcfl lacks `es`). A book with no text in a
  language → 404, never English. Letters and transcripts are English only.
  `/books?lang=es` still gives English book titles; the TOC's `title` is
  localised ("El Bhagavad-gītā Tal Como Es").
- Limits unchanged: 120 verse requests and 30 searches a minute.
- Terms: cite vedabase.cc, don't alter the text, no commercial use.

## 3. Goals

1. **Phase 0 — groundwork.** Decode the new fields; old cached items (which
   lack them) are refetched; the client takes a language; the old letter
   heuristics no longer touch new-format letters.
2. **Phase 1 — order and inline links.** `h`/`l` and prefetch follow the item's
   own `prev`/`next`; `[[ref|label]]` shows as just the label.
3. **Phase 2 — contents from the API.** Book trees, titles and order come from
   the TOC; Letters › By year comes from `/letters`. HTML scraping is removed.
4. **Phase 3 — clickable references** with back/forward history and Vimium-style
   `f` hints.
5. **Phase 4 — language switcher** in the sidebar.
6. **Smoke fixes O4, O1** (see §9).

## 4. Non-goals

- **Audio.** No player, no `a` key (user: "skip audio, we will do it in the
  future"). `audio_url` is decoded and kept, nothing shows it.
- "Lectures on this verse" (needs the audio work; future).
- Translating the app's own strings (hints, help, toasts). They stay English.
- Arabic (the server doesn't offer it).
- The collapse button (user didn't pick it) and a README credit line (not picked;
  mention again at release time).
- Letter footer cleanup ("Q1"): the server fixed it.
- Lecture text cleanup beyond what `Clean` does today (the metadata first line
  stays; anchors are already stripped).

## 5. Anti-goals: it fails if…

- A reference opens the wrong verse, or underlines the wrong words.
- Switching language shows English text labelled as another language.
- The app hits the TOC limit (10/min) in normal use, or refetches TOCs every
  launch.
- Startup gets slower: the tree must still show at once from disk.
- Going offline breaks anything that works offline today.
- Collections, state or settings are lost or rewritten wrongly (the base spec's
  one safety boundary still holds).

## 6. Design

### 6.1 Phase 0 — fields, language, cache

- `api.Item` gains `Prev, Next string` (null → ""), `References []Reference`
  (`Text, Ref, Field`; null ref → ""). `api.Document` gains `IsoDate`,
  `Recipients []string`, `AudioURL`, `Address`. `Clean` sanitizes the new strings
  too (`SanitizeText`), including reference `Text`/`Ref`.
- **Language in the client.** `Verse`, `Books`, `Search` take a `lang`
  argument instead of the hard-coded `en`.
- **Language is an argument, never shared mutable state.** Service methods take
  `lang` (`Item(ctx, ref, lang)`, later `TOC(ctx, code, lang)`, `Search(…, lang)`);
  each `tea.Cmd` captures the language when it is created, and its result
  message carries that `lang`. The UI drops results whose `lang` no longer
  matches the current one (as it already drops renders from an old theme).
  `letters/…` and `transcripts/…` always use `en` (service helper).
- **Cache break (call).** Items move to a new directory,
  `<cache>/items-v2/<lang>/<ref>.json`, holding the **raw response bytes** as the
  server sent them (decoded on read), so fields added by the server later are
  never lost again. Old items lack `prev`/`next`/`references`, so every item is
  fetched again once. **Offline fallback:** on a miss in `items-v2/en/` *and* a
  failed fetch, the old `<cache>/items/<ref>.json` is served (English only,
  read-only, never copied forward), so an upgraded user who is offline keeps
  every verse they had. The old dir is not deleted. Collections and export keep
  working: they store refs, not items.
- **Letters.** `cleanLetter` only runs on the old header shape: the purport
  starts with `Letter to:` (or the dd-dd-dd stamp is preceded by `Letter to:` /
  `Dated:` within the header window). New-format letters pass through
  untouched. Keep the existing tests for old-shape input (cached raw items in
  `items/` are no longer read, but memos may still arrive old-shaped).
- Search results and listing entries are sanitized as today.

### 6.2 Phase 1 — order and inline links

- **`h`/`l`:** if the shown item has `Prev`/`Next`, open that ref; otherwise use
  `library.Prev/Next` as today. The tree cursor follows (same as `:` goto).
  Prefetch uses `Next` the same way. Inside a collection, `h`/`l` keep moving
  through the collection (existing behaviour wins there).
- **Inline links:** `render.DisplayMarkdown` turns `[[ref|label]]` into `label`
  (label's own `*…*` kept). `[[ref]]` with no label shows `ref`. Phase 3 makes
  them clickable; until then they're plain text.

### 6.3 Phase 2 — contents from the API

- **Client:** `TOC(ctx, code, lang)` and `Listing(ctx, kind, q)` (kind
  `letters`/`transcripts`; q = year, recipient, page, per_page). The TOC gets its
  own limiter under 10/min (e.g. `rate.Every(7s)`, burst 2). Listings use the
  verse limiter (they count against the verse budget, per the docs' grouping;
  the builder confirms with one 429 test if unsure — not required).
- **Service:** TOC cached as a blob `toc/<lang>/<code>.json`; fresh for 24 h;
  stale copies are still used when the network fails (offline works). Listing
  pages cached as blobs `list/<kind>/<year>-<page>.json`, fresh for 7 days
  (call; letters don't change often).
- **Library:** keep the sitemap. It still provides letters (by recipient, from
  slugs) and transcripts (by year, from slugs), and a fallback book tree when no
  TOC is available yet (first launch, offline). New
  `(*Library).ApplyTOC(code string, toc api.TOC)` replaces that book root's
  children with the TOC's tree: titles from `title`, leaf labels from `label`,
  order as given (no re-sorting), `Leaf` = no children. The book's flat reading list and the ref index are rebuilt for that
  book; `Find` keeps working for every ref in the TOC.
- **When TOCs load (call):**
  - At launch, every TOC already on disk for the current language is applied
    before the first frame (disk only, no network).
  - After the first frame, missing or stale TOCs are fetched in the background,
    one at a time under the TOC limiter: the current item's book first, then
    expanded books, then the rest in `/books` order. 24 books ≈ 3 minutes on
    a first launch; later launches fetch only what is over 24 h old.
  - Expanding a book whose TOC isn't loaded jumps it to the front of the queue.
    The sitemap tree shows meanwhile, then is replaced in place (cursor kept on
    the same ref, or on the book root if the ref vanished).
- **Titles:** the TOC replaces chapter-title scraping. Front-matter titles come
  from the TOC too. Item `translation` titles still update leaves as today.
- **Letters › By year:** years and counts from `/letters` (`years`, plus an
  "Undated" group from `undated` if the API offers a way to list them — if not,
  omit Undated). Opening a year loads page 1 (`per_page=100`); further pages load
  as the cursor nears the end, as today. Entry labels: `title`, with `iso_date`
  and `location` shown as today's scraped date/place were. `AddYearGroup` is
  reused.
- **Remove** `internal/scrape` and the UI code that only served it
  (`titlesCmd`, `ancestorTitlesCmd`, the scraped year-page plumbing), and
  `Service.Page` if nothing else uses it. O3 ("By year jumps to By recipient")
  must not reproduce after this; add a test for it.
- Lectures stay grouped by slug date (no change). Using `/transcripts` titles is
  a later nicety, not this round.

### 6.4 Phase 3 — clickable references

- **Which spans are links:** for the shown item, every `references` entry with
  a non-empty `ref`, in its `field` (`translation`, `purport`; for letters and
  lectures the purport is the body). Matching is on the raw Markdown before
  Glamour, by exact `text`. Plain-text matches need boundaries: the character
  after the match must not be a digit, `.`+digit, `-`+digit or `–`+digit (so
  `Bg. 4.7` doesn't match inside `Bg. 4.70`), and the one before must not be a
  letter or digit. Every occurrence of a matching text is a link (the API lists
  distinct texts). Entries with `ref: null` stay plain. Wiki links are matched
  by their full `[[…]]` text and displayed as their label (6.2).
- **Display:** links underlined in the theme accent. The hidden parts toggles
  (`1`–`5`) apply as usual; links in hidden parts don't exist.
- **Mechanism (builder's choice within these constraints):** mark each link in
  the Markdown with sentinels that don't change wrapping widths (zero-width
  characters or similar), render with Glamour as today, then find and strip the
  sentinels in the rendered lines, recording `(line, startCol, endCol, linkIdx)`
  and applying the underline. A link that Glamour wraps across lines becomes
  two hit regions with the same target. The result is cached with the render
  (same cache key). Must hold for 100,000-character lectures without a visible
  slowdown (one pass over the output).
- **Mouse:** a left click on a link opens its ref (only when mouse support is
  on). Clicks elsewhere behave as today.
- **`f` hints (Vimium style):** in the reader (not tree focus — there `f` keeps
  filtering), `f` puts a small label over the start of every link visible on
  screen. Labels from home-row letters `a s d f g h j k l ; e r u i o w` in
  that order; if there are more links than letters, use two-letter labels
  (`aa`, `as`, …) for all of them. Typing a label opens that link; a prefix
  narrows the shown labels; Esc or any non-label key cancels. Labels are drawn in
  reverse video in the accent colour. If no link is visible: a toast "No links on
  screen".
- **History:** following a link (click or hint; call: only link follows)
  pushes the current place (ref + scroll offset) on a back stack. `Backspace` or `[` goes back (restores the
  scroll), `]` goes forward. Opening anything by other means (tree, search, goto,
  h/l) clears the forward stack but keeps the back stack. History is in memory
  only, max 100 entries. The bottom bar shows `⌫ back` when the back stack isn't
  empty.
- **Where the jump lands:** the reader opens the target; the tree cursor follows
  as for `:` goto (book TOC loaded on demand, sitemap fallback otherwise).
- Help (`?`) and README gain `f`, `Backspace`/`[`, `]`.

### 6.5 Phase 4 — language switcher

- **State:** current language saved in `state.json` (`lang`, default `en`).
- **Sidebar tag:** the sidebar's top border shows `EN ▾` (code in capitals) next
  to the version tag. Clicking it, or `L` anywhere outside text inputs, opens a
  picker overlay styled like the theme picker: English, Español, Português,
  हिन्दी, Русский, with the current one marked. Enter/click picks, Esc cancels.
- **Switching:** applies at once: TOCs on disk for the new language are applied
  (others queued, current book first), the current item reloads in the new
  language at the top, search uses the new language, `o` opens the item's
  `url` (already per language). Letters and transcripts are unaffected.
- **Books without the language:** from `/books` `languages`. In the tree they're
  shown dimmed with the suffix `· not in <CODE>` (call), e.g. `· not in HI`. Opening an item in one shows a short note in the reader
  ("This book isn't available in Español on vedabase.cc.") instead of an error
  toast. A 404 for an item that should exist in the language shows the same
  note. Never fall back to English silently.
- **Letters and transcripts** keep working under any language, with an `EN` tag
  in the reader header (call: small dim tag).
- **Collections:** store refs only (unchanged), so an entry opens in the current
  language.
- **Search:** sends `lang`; when the scope is letters or transcripts, `en`.
- **Hindi/Devanagari width:** shown as the API gives it (base spec §11).
- **Export:** exports what's shown, in the current language; front matter gains
  `lang:`.

## 7. Constraints

- Go + Bubble Tea v2, `CGO_ENABLED=0`; no new heavy dependencies.
- Tests offline (`make test`), recorded fixtures under `testdata`; race suite
  green (`CGO_ENABLED=1 go test -race ./...`).
- Rate limits: verse ≤ 100/min, search ≤ 25/min (existing), TOC ≤ ~8/min.
- Tree must render from disk at startup with no network.

## 8. Calls made by the coordinator (user didn't decide these)

New cache dir instead of field sniffing; sitemap kept as the fallback and for
letters/transcripts; all TOCs prefetched slowly in the background; scraping
removed; history only on link follows, `Backspace`/`[` back, `]` forward;
hint letters; `L` for language; `· not in <CODE>` marking; listing cache 7 days.

## 9. Smoke fixes

- **O4:** Tab / Shift-Tab in search changes the scope; if there's a query,
  re-run it in the new scope.
- **O1:** opening an uncached item offline waits 8–30 s before failing. First
  find where the time goes (Go `httptrace` against an unreachable host /
  failing resolver; on WSL resolver retries are the likely cause). Fix with
  transport timeouts only: dial ~5 s, TLS handshake ~5 s, response header
  ~15 s; the overall 30 s request timeout stays for large answers. No
  "offline for N seconds" circuit breaker (it would stall the TOC queue and
  prefetch after one blip).

## 10. Order of work

Phases 0 → 1 → 2 → 3 → 4, smoke fixes with phase 1. Each phase ends green and
committed. One Opus review of the branch before release; the release (v0.2.0)
needs the user's go-ahead.
