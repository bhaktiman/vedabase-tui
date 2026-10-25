# vedabase-tui — design

Date: 2026-10-25
Status: draft for approval

A colourful terminal reader for Śrīla Prabhupāda's books, letters and lectures,
built on the open JSON API at https://vedabase.cc/en/api/.

Throughout, **(user)** marks a decision the user made; **(call)** marks a
decision the coordinator made on the user's behalf, which the user can still
overturn. Unmarked items are facts about the API or the environment.

## 1. Intent

The user wants to **read the books straight through**, verse after verse, in a
terminal, and also **search** for and jump to specific passages (user). Along the
way they collect verses and passages they find into **named collections** and
**export** single verses or whole collections to Markdown (user).

It should feel good to use: **colourful, with small Bubble Tea animations**
(loading bars, sparkles while waiting, etc.) (user), and **vim keys** for
navigation (user).

## 2. Goals

1. Browse every book, plus letters and lectures, in a contents tree.
2. Read a verse with each part shown or hidden independently: Devanagari/Bengali,
   transliterated verse, word-for-word, translation, purport (user).
3. Read in order without touching the tree: next/previous verse runs across
   chapter (and canto/līlā) boundaries (call).
4. Full-text search across the books, letters and lectures, with result
   snippets, jumping into any result.
5. Several named collections holding any item: a verse, a prose chapter, a
   letter, a lecture (user).
6. Export a single verse, or a whole collection, to Markdown including the
   reference, book and vedabase.cc URL (user).
7. Letters browsable by year and by recipient; lectures by year (user).
8. Colour and motion as in §8 (user approved all six effects).

## 3. Non-goals

- Languages other than English. The API serves only `en` today; the code may
  pass `lang=en` everywhere but needs no language switcher.
- Personal notes or annotations on collection items. Collections are lists of
  references only (user: "just a way to add verses and things we find as a
  collection").
- Sync, accounts, sharing collections between machines. Single local user (call:
  the user said "we" but never confirmed a second user; nothing here should block
  adding sync later).
- Mirroring the whole library for offline use. About 20k verses at the API's
  120 requests per minute is roughly 3 hours; the cache fills lazily as you read.
- Audio, Q&A, word index, BTG and the site's other sections.
- A release pipeline. A static binary that builds with one command is enough.

## 4. Anti-goals: it fails if…

- **An animation ever makes the reader wait.** Every effect is non-blocking; a
  keypress during an animation jumps it to its end state. Loaders appear only
  while something is really downloading.
- **Collections are ever lost or corrupted.** This is the only user data. It
  lives apart from the cache and is written atomically (§9).
- **It hammers vedabase.cc.** It stays well under the published limits, caches
  everything it downloads, and fetches HTML pages only on demand.

## 5. Stack (user, from the stack discussion)

- **Go** (1.26 is installed), built with `CGO_ENABLED=0` as a single static binary.
- **Bubble Tea v2** (`charm.land/bubbletea/v2`, currently v2.1.0)
- **Bubbles v2** (`charm.land/bubbles/v2`, v2.2.1): viewport, text input,
  spinner, progress, list
- **Lip Gloss v2** (`charm.land/lipgloss/v2`, v2.0.6): colours, borders, layout
- **Glamour v2** (`charm.land/glamour/v2`, v2.0.1): renders the API's Markdown,
  styled to match the Lip Gloss palette
- The standard library for HTTP and JSON. `golang.org/x/net/html` for the few
  HTML pages scraped (call). A small TOML library for the settings file (call;
  any maintained one).

Binary name: `vedabase` (call).

## 6. What the API provides (verified 2026-10-25)

Base URL `https://vedabase.cc/api/v1/`. Read-only GET, JSON, no key.

- `GET /books` returns 24 codes: 22 books (`bg`, `sb`, `cc`, `iso`, `noi`, `kb`,
  `nod`, …) plus `letters` and `transcripts`. Each has `code`, `title`, `type`
  (`chapters` / `cantos` / `sections` / `flat` / `documents`) and `languages`.
- `GET /verse/{ref}` returns `ref, lang, book, url, devanagari, verse_text,
  synonyms, translation, purport, format`, with missing parts as `null`. All text
  is Markdown: `*italic*` Sanskrit, `**bold**` translation, and `> ` at the start
  of each verse line. Letters and transcripts also carry
  `document: {type, date, year, location, covers}`. In them, `translation` is the
  title and `purport` is the whole text, sometimes over 100,000 characters.
- `GET /search?q=&book=&page=&per_page=`: all words must match, and quotes give
  an exact phrase. `page` runs 1–50 and `per_page` 1–50 (at most 2,500 results).
  There is no total count, only `has_more`. Results carry `ref, book, book_title,
  url, translation, snippet_translation, snippet_purport`; the snippets contain
  `<mark>…</mark>` (the only HTML in the API).
- Errors look like `{"error":{"code","message"}}`, with codes `bad_ref`,
  `unsupported_lang`, `bad_query`, `bad_param`, `not_found`, `rate_limited`
  (429 + `Retry-After`) and `lookup_failed`.
- **Limits: 120 verse requests and 30 searches per minute per address.**

**There is no table-of-contents endpoint.** Probing `books/bg`, `toc/bg` and
similar paths returns 404. A chapter ref such as `bg/2` returns 404, yet `lcfl/1`
(a prose chapter) returns the whole chapter. So a book's `type` does **not**
tell you how deep its tree goes.

### Where the tree comes from

`https://vedabase.cc/sitemap-en-books.xml` (3.7 MB, ~31k `<loc>` URLs) lists
every page of every book, plus `/en/library/letters/<slug>/` (6,582) and
`/en/library/transcripts/<slug>/` (3,705).

- Path → API ref: strip the host and trailing slash (`/bg/2/13/` → `bg/2/13`).
  For the library, strip `en/library/` (`/en/library/letters/letter-to-aatie/`
  → `letters/letter-to-aatie`).
- **A sitemap node with no children is a readable item.** Interior nodes
  (`bg/2`, `sb/1`, `cc/adi`) are containers only.
- Segments seen besides plain numbers: the CC līlās `adi`, `madhya`, `antya`;
  front matter `0a`–`0d` (e.g. `bg/0a` = Dedication and `sb/0a` = Preface, both
  verified readable, with the title in `translation`); ranges `16-18`;
  `concluding_words`, `summary`, `notes`, `1a-2a`, `1b`, `2b`.
- The sitemap is sorted as text (`bg/1, bg/10, bg/11`). The tree must use
  **natural order**: numeric parts by number, front matter `0x` first, and a
  range sorted by its first number.

### Labels the sitemap lacks

- **Chapter, canto and līlā titles** exist only in the site's HTML index pages,
  e.g. `/bg/` lists "Chapter 2: Contents of the Gītā Summarized", and `/sb/1/`
  and `/cc/adi/` list theirs. Fetch each index page the first time its node is
  expanded, cache it, and fall back to "Chapter N" if parsing fails. This depends
  on the site's markup and may break.
- **Front-matter titles** (Dedication, Preface…): the item's own `translation`
  holds the title. Show a placeholder until it's fetched, or take it from the
  index page if listed there (builder's choice).
- **Letters by year:** the library page `/en/library/letters/?year=YYYY` lists
  that year's letters with recipient, date and place, 20 per page (the year
  facet shows counts, e.g. 1969: 760). Scrape it when a year node opens, then
  cache. This is also HTML scraping and may break. Year grouping needs this
  scrape (the sitemap has no dates). **Fetch lazily (call):** page 1 when the
  year opens, further pages as the list scrolls near its end, never all 38 up
  front.
- **Scraping is in v1 but not on the critical path (call):** the tree works
  first with "Chapter N" and raw labels, and title scraping is added later.
- **Letters by recipient:** taken from the slug with no fetching:
  `letter-to-<name>[-N]` → `<name>`, giving about 1,886 groups. Slug names have no
  diacritics ("abhirama"); once the proper title ("Letter to: Abhirāma") is known
  from a year page or a fetched letter, show that instead. A joint letter
  (`letter-to-damodara-abhirama`) is its own group (call).
- **Lectures by year:** the slug starts with the date (`660219bg-new-york` →
  1966-02-19), so group by year with no fetching. `/en/library/transcripts/?year=`
  also exists for nicer titles if wanted.

## 7. Screens and interaction

### Layout (user chose "B: two panes")

```
┌ Bhagavad-gītā As It Is › Ch 2 › Text 13 ─────────────────── 13 / 72 ┐
│ ▸ Dedication           │ Bg 2.13                                     │
│ ▸ 1 Observing the…     │ DEVANAGARI  देहिनोऽस्मिन्यथा देहे …         │
│ ▾ 2 Contents of the…   │ VERSE       dehino 'smin yathā dehe …       │
│     2.12               │ WORD FOR WORD  dehinaḥ—of the embodied; …   │
│   ★ 2.13               │ TRANSLATION As the embodied soul …          │
│     2.14               │ PURPORT     Since every living entity …     │
├────────────────────────┴─────────────────────────────────────────────┤
│ tab focus · h/l verse · / search · ' collections · 1 2 3 4 5 parts   │
└──────────────────────────────────────────────────────────────────────┘
```

- A contents tree on the left and a reader on the right. A top bar shows the
  breadcrumb and the position within the chapter. A bottom bar shows key hints
  and which parts are showing.
- `t` hides or shows the tree for a full-width reader. Tab switches focus.
- The tree follows the reader, highlighting and revealing the current item. ★
  marks items that are in any collection.
- Top-level tree nodes: the 22 books in `/books` order, then **Letters** (with
  **By year** and **By recipient** subtrees) and **Lectures & conversations**
  (by year).
- Prose books and documents (e.g. `lcfl/1`, a letter, a lecture) render as one
  long scrolling page with the same keys.
- On launch it reopens the last item read, at its scroll position (call).
- The tree is a **custom component**; Bubbles has no tree widget. It needs lazy
  children (for year lists), expand/collapse, ★ markers and the `f` filter.

### Reading order (call)

- `h`/`l` visit **every leaf of the current book in natural order**: front
  matter, every verse or range, `summary`, `notes`, `concluding_words`,
  crossing chapter, canto and līlā boundaries.
- They **stop at the ends of a book** (toast "End of Bhagavad-gītā") rather than
  running into the next book.
- In Letters and Lectures, `h`/`l` follow the order of the tree group the item
  was opened from: by date in a year group, by date within a recipient group
  once dates are known, and by slug until then.
- The top bar's "13 / 72" is the item's position among its **sitemap siblings**
  (the leaves under the same parent node), not an API value.

### Show / hide parts (user)

`1` Devanagari/Bengali · `2` transliterated verse · `3` word-for-word ·
`4` translation · `5` purport (numbering is the call). The choice persists across
restarts (call). A hidden part disappears entirely, and the bottom bar shows it
dimmed or struck through. Parts that are `null` for an item are simply absent.

### Keys (user asked for vim keys; mapping is the call)

Arrow keys and PgUp/PgDn work alongside these.

| Key | Reader | Tree |
|---|---|---|
| `j` / `k` | scroll line | move down/up |
| `h` / `l` | previous/next item (crosses chapters) | collapse/expand |
| `Ctrl-d` / `Ctrl-u` | half page | half page |
| `gg` / `G` | top/bottom | first/last |
| `Enter` | — | open item / expand |
| `/` | full-text search | full-text search |
| `f` | — | filter the tree as you type (e.g. 1,886 recipients) |
| `n` / `N` | next/previous search result | same |
| `:` | go to reference | same |
| `s` | add to a collection | add selected item |
| `'` | open collections | same |
| `e` | export current item | export selected item |
| `t` | hide/show tree | same |
| `1`–`5` | toggle parts | same |
| `?` | help overlay | same |
| `q` / `Esc` | close overlay; quit at top level | same |

`/` always means full-text search, and `f` filters the tree (call; this replaces
an earlier aside that `/` would filter the tree).

### Go to reference (`:`) (call)

Accepts the site's ref form (`bg/2/13`, `sb/1/2/6`, `cc/madhya/9/74`) and a
friendly form with spaces and dots (`bg 2.13`, `sb 1.2.6`, `cc madhya 9.74`).
Short līlā names (`cc m 9.74`) are nice to have. An unknown ref shows an error
toast. The builder decides the exact parser.

### Search (`/`)

- A search box with a scope selector: all, the current book, letters, or lectures
  (Tab cycles; call). The query runs on **Enter only**, never as you type,
  because of the 30/min limit.
- Results show book and ref, the translation line, and a snippet, with `<mark>`
  turned into a highlight colour. Scrolling to the end loads the next page while
  `has_more` is true.
- Enter opens a result in the reader. From there `h`/`l` keep reading the book
  normally, and `n`/`N` move to the next/previous result. Esc returns to the
  results list with its scroll position kept (call).
- `n` past the last loaded result fetches the next page if `has_more` (call).
  In the tree pane, `n`/`N` do the same as in the reader.

### Collections (user: several named collections; details are the call)

- `s` opens a small picker: choose an existing collection or type a new name.
  Adding an item already in that collection is a no-op with a toast.
- `'` opens the collections view: a list of collections (name, item count). Enter
  shows the items in the order added (ref, book, first line of translation or the
  title); Enter on an item opens it in the reader.
- Remove an item; rename or delete a collection (with confirmation for delete).
  No manual reordering in v1.
- An item stores its ref, plus the book code and a display label captured when
  added, so the list renders offline.
- Items are identified by **exact ref** (call). `bg/1/16-18` and `bg/1/17` are
  different items, and ★ shows only on the exact ref.

### Export (user: Markdown, single verses and whole collections)

- `e` on an item writes one file: `<export_dir>/<ref with / → ->.md`, e.g.
  `bg-2-13.md`, `letters-letter-to-aatie.md`.
- `e` in the collections view on a collection writes **one** file for the whole
  collection, `<export_dir>/collection-<name-slug>.md` (call).
- **Contents:** only the parts currently showing (call). A file that already
  exists is overwritten, and a toast says so (call).
- **A collection export with uncached items** runs as a background job (call).
  It fetches the missing items within the rate limit and shows the progress bar
  (effect 1) in the status area while you keep reading. The file is written only
  when every item is in. If an item fails, it still writes the file, with
  "(unavailable: <ref>)" in that item's place, and says so in the toast.
- Default `export_dir` is `~/vedabase-exports/`, settable in the config (call;
  the user didn't say where, and an Obsidian vault folder works).

Single item:

```markdown
---
ref: bg/2/13
book: bg
book_title: Bhagavad-gītā As It Is
chapter: "2: Contents of the Gītā Summarized"   # when known
url: https://vedabase.cc/bg/2/13/
exported: 2026-10-25
---
# Bg 2.13

> देहिनोऽस्मिन्यथा देहे …

> dehino 'smin yathā dehe
> …

*dehinaḥ*—of the embodied; …

**As the embodied soul continually passes, …**

Since every living entity is an individual soul, …
```

For letters and lectures, front matter adds `date`, `year` and `location` from
the `document` object. A collection file has front matter (`collection`,
`exported`, `count`), then one `## <label>` section per item: a line with the
book title and `[vedabase.cc](url)`, then that item's parts.

## 8. Colour and motion (user approved all six)

The palette is warm and dark: gold for headings and Sanskrit, with rose, teal and
violet accents. Truecolor via Lip Gloss, degrading gracefully on 256-colour
terminals. Glamour is given a custom style matching the palette, so Sanskrit
italics render gold and translations bold and bright.

1. **Startup progress bar:** a gradient bar (Bubbles `progress`, animated) while
   the sitemap downloads on first launch, showing MB done/total.
2. **Sparkle loader + shimmer:** while an uncached item downloads, twinkling
   multicolour ✦✧ next to the heading and a shimmering placeholder where the text
   will be.
3. **Star burst:** adding to a collection pops the ★ with a few sparkles
   scattering for about half a second.
4. **Toasts:** short confirmations that slide in and fade out ("✓ Exported
   ~/vedabase-exports/bg-2-13.md", "★ Added to On the soul"). Errors use the same
   mechanism in rose.
5. **Quick fade-in on the next item:** at most about 150 ms of colour ramp when
   the reader changes item.
6. **Friendly rate-limit wait:** on a 429, a countdown bar for the
   `Retry-After` seconds ("Slow down a moment ☕"), then the request retries
   itself.

Rules: every animation is driven by Bubble Tea tick messages and never blocks
input; any key finishes it instantly; `animations = false` in the config turns
all motion off (static equivalents remain).

## 9. Storage (call; told to the user)

| What | Where | Notes |
|---|---|---|
| Downloaded items | `~/.cache/vedabase-tui/items/` | one JSON file per ref, safe to delete |
| Sitemap, index pages, year lists | `~/.cache/vedabase-tui/` | refreshed as below |
| Collections | `~/.local/share/vedabase-tui/collections.json` | user data |
| Settings + last position | `~/.config/vedabase-tui/config.toml` (settings), `~/.local/state/vedabase-tui/state.json` (last item, scroll, toggles) | the split is the builder's choice |
| Exports | `~/vedabase-exports/` | configurable |

Respect `XDG_*_HOME` when set (`os.UserCacheDir` / `os.UserConfigDir` are fine
for cache and config).

**Collections writes are atomic:** write a temp file in the same directory,
fsync, rename. Never truncate in place. If the file fails to parse, don't
overwrite it: rename it aside (`collections.json.bad-<timestamp>`), tell the user
in a toast, and start empty. This is the one safety boundary of the project.

## 10. Network behaviour

- **Client-side limiter** sitting under the published limits (e.g. 100 verse
  requests and 25 searches per minute, call). HTML page fetches count against
  the verse budget.
- **429:** honour `Retry-After` and show effect 6. **5xx/`lookup_failed`:** one
  retry after a short delay, then an error toast.
- **Prefetch:** after an item displays, fetch the next item in reading order
  if it isn't cached. Only one item ahead.
- **Cache:** items never expire (the docs say a verse doesn't change).
  Scraped HTML (titles, year lists) refreshes after 30 days (call). Re-download
  the sitemap in the background at most once a week, using a conditional GET if
  the server supports it. Show the cached tree immediately on launch.
- **Offline:** cached items and the cached tree work with no network. Uncached
  items show a clear "offline" message instead of an endless loader.
- Send a descriptive `User-Agent` (e.g. `vedabase-tui/<version>`) (call).

## 11. Rendering long documents

Some lectures exceed 100,000 characters. Render Markdown with Glamour **once**
per (item, width, visible parts) and keep the result; the viewport only scrolls a
prepared string. Re-render on resize, debounced.

**First spike:** before building the UI around it, render the longest transcript
at 80 and 200 columns and measure the Glamour render time and viewport scroll
responsiveness. If rendering is slow (say, over 200 ms), render in the background
behind the sparkle loader, or render the visible region first.

Devanagari and Bengali are shown as the API gives them. Terminal width handling
for conjuncts is imperfect, notably in Windows Terminal on WSL2 (the user's
setup). That's acceptable, and toggle `1` hides them.

## 12. Testing (builder's choice of detail)

- Unit tests: sitemap → tree (natural sort, leaf detection, līlās, front matter,
  ranges), path ↔ ref mapping, letter slug → recipient, transcript slug → date,
  go-to-ref parsing, `<mark>` conversion, export Markdown (golden files),
  collections store (atomic write, corrupt-file handling).
- An HTTP client tested against `httptest` with recorded API responses,
  including a 429 with `Retry-After`. Tests never hit vedabase.cc.
- UI flows are optional (`teatest` or model-level tests of `Update`).

## 13. Left to the builder

Settings are read at startup only (no live reload). Package layout, exact palette values, Glamour style JSON, the go-to-ref grammar
beyond the examples, the help overlay contents, how scraped HTML is parsed,
settings-file layout, the module path, and the order of work, except that the
§11 spike and the sitemap → tree parser come first.
