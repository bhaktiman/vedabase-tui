# vedabase-tui Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A colourful two-pane terminal reader for Prabhupāda's books, letters and lectures over the vedabase.cc API, with show/hide verse parts, vim keys, search, named collections and Markdown export.

**Architecture:** Pure, well-tested packages (`ref`, `api`, `library`, `store`, `collections`, `export`, `render`) sit under a thin `service` layer that combines cache and network, and a Bubble Tea v2 UI (`internal/ui`) that talks to the service only through a `Backend` interface, so UI tests run against a fake. The contents tree is built from the site's sitemap; item text comes from the API; titles that only exist in HTML are scraped lazily.

**Tech Stack:** Go 1.26 · `charm.land/bubbletea/v2` v2.1.0 · `charm.land/bubbles/v2` v2.2.1 · `charm.land/lipgloss/v2` v2.0.6 · `charm.land/glamour/v2` v2.0.1 · `golang.org/x/net/html` · `golang.org/x/text` · `golang.org/x/time/rate` · `github.com/BurntSushi/toml`

**Spec:** `docs/superpowers/specs/2026-10-25-vedabase-tui-design.md`. Read it before any task; section numbers below (§n) refer to it.

## Global Constraints

- Module path `vedabase-tui`, binary `vedabase`, built with `CGO_ENABLED=0 go build -o vedabase ./cmd/vedabase`.
- Pin exactly the versions listed in Tech Stack. No other third-party dependencies without a ledger note.
- **Tests never touch the network.** Fixtures are recorded once into `testdata/` by the step that says so. `go test ./...` must pass offline.
- Every API call sends `lang=en` and `User-Agent: vedabase-tui/<version>` (`version` is a `var` in `cmd/vedabase`, default `"dev"`, overridable by `-ldflags`).
- Client-side limits: **100 verse/page requests per minute and 25 searches per minute**. HTML page and sitemap fetches count against the verse budget.
- Storage paths (§9): items under `<cache>/vedabase-tui/items/`, blobs under `<cache>/vedabase-tui/`, collections at `<data>/vedabase-tui/collections.json`, settings at `<config>/vedabase-tui/config.toml`, state at `<state>/vedabase-tui/state.json`. `<cache>`/`<config>` come from `os.UserCacheDir`/`os.UserConfigDir`. `<data>` is `$XDG_DATA_HOME` or `~/.local/share`, and `<state>` is `$XDG_STATE_HOME` or `~/.local/state`. Default export dir is `~/vedabase-exports`.
- **The one safety boundary:** `collections.json` is only ever written by temp file in the same dir → fsync → rename → fsync dir. It is never truncated in place, and a file that won't parse is renamed aside, never overwritten.
- Animations never block input. Any key press finishes running effects instantly, and `animations = false` disables all motion.
- Item cache entries never expire. Scraped HTML expires after 30 days. The sitemap and `/books` refresh at most weekly. The sitemap server sends no `Last-Modified` (checked 2026-10-25), so the refresh is a plain GET.
- Sizes:
  - Recorded transcript `transcripts/660219bg-new-york`: 114,801 characters of purport. It is the long-document fixture.

## Review Focus

1. **Tiny or very narrow terminal** (e.g. 30×8): no panic, the tree auto-hides below 70 columns, and the reader still scrolls. Test in Task 11.
2. **First launch offline, no cached sitemap:** the startup screen shows "Can't reach vedabase.cc" with retry (`r`) instead of an endless bar. Launching with a cache but no network works. Tests in Tasks 9 and 11.
3. **Holding `l` (key repeat) through uncached verses:** a slow response for an item the user already left must not replace the current reader, and renders for a stale width/parts are dropped. Test in Task 11.
4. **Two app instances editing collections:** each mutation re-reads the file before applying, so the second instance doesn't erase the first one's additions. Test in Task 6.
5. **Unexpected data:**
   - a book code in the sitemap that isn't in `/books` is still listed, after the known books;
   - an unseen path segment sorts after numbers without panicking;
   - API `null`s and unknown JSON fields are tolerated.

   Tests in Tasks 3 and 4.

---

## File Structure

```
cmd/vedabase/main.go           entry: paths, settings, service, tea.Program
internal/ref/ref.go            path↔ref, natural order, display labels, slugs
internal/ref/goto.go           ":" go-to parser
internal/api/types.go          Item, Document, Book, SearchPage, Parts
internal/api/client.go         HTTP client, errors, retry, limiter
internal/library/library.go    Node, Library, sitemap → tree, lookups
internal/library/order.go      reading order (Next/Prev), Position
internal/store/paths.go        XDG paths
internal/store/cache.go        ItemCache, BlobCache
internal/store/settings.go     Settings (TOML), State (JSON)
internal/collections/store.go  collections store (atomic)
internal/export/export.go      Markdown for items and collections, file writing
internal/render/render.go      item → display Markdown, glamour render cache, <mark>
internal/render/theme.go       palette + glamour StyleConfig
internal/scrape/scrape.go      chapter titles, letter year facet + year pages
internal/service/service.go    cache+network: Item, Library, Search, Page, prefetch
internal/ui/…                  app, tree, reader, search, collections, goto, help, toast, effects, keys
```

Each package has its `_test.go` beside it and `testdata/` where noted.

---

### Task 1: Scaffold and long-document render spike

**Files:**
- Create: `go.mod`, `Makefile`, `cmd/vedabase/main.go` (prints version, exits), `internal/render/spike_test.go`, `internal/render/testdata/660219bg-new-york.json`, `docs/spike-render.md`

**Interfaces:**
- Produces: module `vedabase-tui`; `make build` (static binary `./vedabase`), `make test` (`go test ./...`).

- [ ] **Step 1:** `go mod init vedabase-tui`, then `go get` each Tech Stack module at its pinned version. Write the `Makefile` with targets `build` (`CGO_ENABLED=0 go build -ldflags "-X main.version=$(VERSION)" -o vedabase ./cmd/vedabase`) and `test`. Write `main.go` so that `./vedabase --version` prints `vedabase dev`.
- [ ] **Step 2:** Record the fixture. Its `purport` length must be 114801:
  `curl -s https://vedabase.cc/api/v1/verse/transcripts/660219bg-new-york?lang=en > internal/render/testdata/660219bg-new-york.json`
- [ ] **Step 3:** Write `BenchmarkGlamourLong` in `spike_test.go`. Load the fixture's `purport`, then for widths 80 and 200 create `glamour.NewTermRenderer(glamour.WithStandardStyle("dark"), glamour.WithWordWrap(w))` and `Render` it. Add `TestViewportLong`: put the rendered 80-col string into `viewport.New()` sized 100×40 with `SetContent`, then time 500 `ScrollDown(1)` calls plus `View()`.
- [ ] **Step 4:** Run `go test ./internal/render -run TestViewportLong -bench GlamourLong -benchtime 5x -v`. Write the numbers into `docs/spike-render.md` with the decision, using this rule:
  - render ≤ 1 s → Task 11 renders the whole document in a background `tea.Cmd`, showing the loader if it takes over 100 ms;
  - render > 1 s → Task 11 renders the first 20k characters (cut at a paragraph boundary) first, then the rest.

  Either way, rendering is never on the `Update` path.
- [ ] **Step 5:** Commit: `git add -A && git commit -m "chore: scaffold module and long-document render spike"`

---

### Task 2: `ref` — references, natural order, labels, slugs

**Files:**
- Create: `internal/ref/ref.go`, `internal/ref/goto.go`, `internal/ref/ref_test.go`, `internal/ref/goto_test.go`

**Interfaces:**
- Produces:
  - `FromPath(p string) (string, bool)`: accepts a full URL or a path.
  - `Less(a, b string) bool`: natural order of whole refs, compared segment by segment.
  - `CompareSegment(a, b string) int`
  - `Display(r string) string`
  - `Recipient(slug string) string`
  - `TranscriptDate(slug string) (time.Time, bool)`
  - `Filename(r string) string`: `/` → `-`.
  - `Slug(name string) string`: lowercase ASCII, diacritics folded via `golang.org/x/text`, runs of non-alphanumerics → `-`, trimmed.
  - `ParseGoto(in string) (string, error)`
  - `Lilas = []string{"adi","madhya","antya"}`

- [ ] **Step 1: Write the failing tests.**

```go
func TestFromPath(t *testing.T) {
	cases := map[string]string{
		"https://vedabase.cc/bg/2/13/":                     "bg/2/13",
		"/cc/adi/0a/":                                      "cc/adi/0a",
		"https://vedabase.cc/en/library/letters/letter-to-aatie/": "letters/letter-to-aatie",
		"/en/library/transcripts/660219bg-new-york/":       "transcripts/660219bg-new-york",
	}
	for in, want := range cases {
		got, ok := FromPath(in)
		if !ok || got != want { t.Errorf("FromPath(%q)=%q,%v want %q", in, got, ok, want) }
	}
	if _, ok := FromPath("https://vedabase.cc/"); ok { t.Error("root must not be a ref") }
}

func TestLess(t *testing.T) {
	sorted := []string{"bg/0a", "bg/0b", "bg/1", "bg/1/1", "bg/1/16-18", "bg/1/19", "bg/2", "bg/10",
		"cc/adi", "cc/madhya", "cc/antya", "sb/4/29/1a-2a", "sb/4/29/1b", "sb/4/29/2b", "sb/10/1/notes", "sb/10/1/summary"}
	for i := 0; i+1 < len(sorted); i++ {
		if !Less(sorted[i], sorted[i+1]) || Less(sorted[i+1], sorted[i]) {
			t.Errorf("want %q < %q", sorted[i], sorted[i+1])
		}
	}
}

func TestDisplay(t *testing.T) {
	cases := map[string]string{"bg/2/13": "Bg 2.13", "sb/1/2/6": "SB 1.2.6", "cc/madhya/9/74": "CC Madhya 9.74",
		"bg/1/16-18": "Bg 1.16-18", "iso/3": "Iso 3", "lcfl/1": "Lcfl 1"}
	for in, want := range cases { if got := Display(in); got != want { t.Errorf("Display(%q)=%q want %q", in, got, want) } }
}

func TestRecipientAndDate(t *testing.T) {
	if got := Recipient("letter-to-abhirama-12"); got != "abhirama" { t.Error(got) }
	if got := Recipient("letter-to-damodara-abhirama"); got != "damodara-abhirama" { t.Error(got) }
	d, ok := TranscriptDate("660219bg-new-york")
	if !ok || d.Format("2006-01-02") != "1966-02-19" { t.Error(d, ok) }
	if _, ok := TranscriptDate("bg-new-york"); ok { t.Error("no date") }
}

func TestFilenameSlug(t *testing.T) {
	if Filename("bg/2/13") != "bg-2-13" { t.Fail() }
	if Slug("On the Soul — Kṛṣṇa's") != "on-the-soul-krsna-s" { t.Error(Slug("On the Soul — Kṛṣṇa's")) }
}
```

```go
func TestParseGoto(t *testing.T) {
	ok := map[string]string{"bg 2.13": "bg/2/13", "bg/2/13": "bg/2/13", "BG 2.13": "bg/2/13",
		"sb 1.2.6": "sb/1/2/6", "cc madhya 9.74": "cc/madhya/9/74", "cc m 9.74": "cc/madhya/9/74",
		"cc a 1.1": "cc/adi/1/1", "cc an 2.3": "cc/antya/2/3", "iso 3": "iso/3", " bg 1.16-18 ": "bg/1/16-18"}
	for in, want := range ok {
		got, err := ParseGoto(in)
		if err != nil || got != want { t.Errorf("ParseGoto(%q)=%q,%v want %q", in, got, err, want) }
	}
	for _, bad := range []string{"", "   ", "2.13", "bg 2..13"} {
		if _, err := ParseGoto(bad); err == nil { t.Errorf("ParseGoto(%q) want error", bad) }
	}
}
```

- [ ] **Step 2:** Run `go test ./internal/ref/`. Expected: FAIL (undefined).
- [ ] **Step 3:** Implement. Segment order inside `CompareSegment`:
  1. līlā names in `Lilas` order, before everything else;
  2. then segments with a leading number, by that number, then by the remaining suffix as a string (so `0a < 0b < 1 < 1a-2a < 1b < 16-18`);
  3. then other words, alphabetically.

  When one ref is a prefix of the other, the shorter sorts first (`bg/1 < bg/1/1`).

  `Display`:
  - book codes `sb` and `cc` are upper-cased; any other code gets only its first letter capitalised;
  - a līlā is title-cased;
  - numeric parts after the book (and līlā) are joined with `.`.

  `ParseGoto`:
  - lower-case and trim the input;
  - if it contains `/`, return it as is when it has 2–5 non-empty segments;
  - otherwise split on spaces: the book first, then for `cc` an optional līlā (accept a unique prefix of `adi`/`madhya`/`antya`; `a` means `adi`), then dotted numbers, the last of which may be a range `n-m`.
- [ ] **Step 4:** Run `go test ./internal/ref/`. Expected: PASS.
- [ ] **Step 5:** Commit: `git commit -am "feat(ref): refs, natural order, labels, go-to parser"` (add the new files first).

---

### Task 3: `api` — client, types, errors, limits

**Files:**
- Create: `internal/api/types.go`, `internal/api/client.go`, `internal/api/client_test.go`, `internal/api/testdata/{verse-bg-2-13,verse-lcfl-1,verse-letter-aatie,books,search-soul-body,err-not-found}.json`

**Interfaces:**
- Produces:

```go
type Item struct {
	Ref, Lang, Book, URL                                   string
	Devanagari, VerseText, Synonyms, Translation, Purport string // JSON null → ""
	Format   string
	Document *Document `json:"document"`
}
type Document struct {
	Type     string          `json:"type"`
	Date     string          `json:"date"`
	Year     int             `json:"year"` // null → 0
	Location string          `json:"location"`
	Covers   json.RawMessage `json:"covers"`
}
type Book struct{ Code, Title, Type string; Languages []string }
type SearchResult struct{ Ref, Book, BookTitle, URL, Translation, SnippetTranslation, SnippetPurport string } // json tags snake_case
type SearchPage struct{ Query, Book string; Page, PerPage int; HasMore bool; Results []SearchResult }
type Parts struct{ Devanagari, Verse, Synonyms, Translation, Purport bool } // json snake_case
func AllParts() Parts

type APIError struct{ Status int; Code, Message string }       // Error() = Message
type RateLimitedError struct{ RetryAfter time.Duration }        // 429
var ErrOffline = errors.New("offline")                           // wraps any transport error
func IsNotFound(err error) bool

type Client struct{ /* base URL, http.Client, UA, verse+search limiters, RetryDelay time.Duration */ }
func NewClient(base, userAgent string, hc *http.Client) *Client // base "https://vedabase.cc"; limiters 100/min & 25/min, burst 5
func (c *Client) Verse(ctx context.Context, ref string) (Item, error)
func (c *Client) Books(ctx context.Context) ([]Book, error)
func (c *Client) Search(ctx context.Context, q, book string, page int) (SearchPage, error) // per_page=20; book "" = all
func (c *Client) Fetch(ctx context.Context, url string, progress func(done, total int64)) ([]byte, error) // site pages & sitemap; verse budget; progress may be nil
```

- [ ] **Step 1:** Record the fixtures once with `curl -s`:
  - `/api/v1/verse/bg/2/13`
  - `/api/v1/verse/lcfl/1`
  - `/api/v1/verse/letters/letter-to-aatie`
  - `/api/v1/books`
  - `/api/v1/search?q=soul+body&book=bg&per_page=2`
  - `/api/v1/verse/bg/99/1` → `err-not-found.json`

  Add `?lang=en` to every request (`&lang=en` on the search, which already has a query string).
- [ ] **Step 2: Write the failing tests.** Every test uses `httptest.NewServer` to serve the fixtures, and sets `RetryDelay` to 1 ms.

```go
func TestVerseDecodes(t *testing.T) // bg/2/13: Translation starts "**As the embodied soul", Document == nil; lcfl/1: Devanagari=="" , Translation=="The First Morning Walk: April 18, 1973"
func TestDocumentDecodes(t *testing.T) // letters: Document.Date=="October 28th 1967", Location=="Navadvipa"
func TestUnknownFieldsAndNulls(t *testing.T) // server returns {"ref":"x/1","devanagari":null,"new_field":{"a":1},"document":{"year":null}} → no error, Document.Year==0
func TestRequestShape(t *testing.T) // handler asserts path "/api/v1/verse/bg/2/13", query lang=en, User-Agent "vedabase-tui/test"; search asserts q, book, page, per_page=20
func TestNotFound(t *testing.T) // 404 fixture → IsNotFound(err)==true, err.Error() contains "No text for bg/99/1"
func TestRateLimited(t *testing.T) // 429 + "Retry-After: 7" → errors.As RateLimitedError with RetryAfter==7*time.Second; exactly 1 request made
func TestRetryOnceOn5xx(t *testing.T) // 500 then 200 → success, 2 requests; 500,500 → APIError{Status:500}, 2 requests
func TestOffline(t *testing.T) // closed server URL → errors.Is(err, ErrOffline)
func TestFetchProgress(t *testing.T) // 10 KB body with Content-Length → progress called, last call (10240,10240)
func TestLimiterBlocks(t *testing.T) // search limiter: 25/min burst 5 → the 6th call within the test waits; use ctx with 50 ms timeout → context.DeadlineExceeded
```

- [ ] **Step 3:** Run `go test ./internal/api/`. Expected: FAIL.
- [ ] **Step 4:** Implement with `golang.org/x/time/rate` (`rate.NewLimiter(rate.Every(time.Minute/100), 5)` and `rate.Every(time.Minute/25), 5`).
  - Decode errors from the `{"error":{code,message}}` body.
  - A 429 is not retried; the UI handles it.
  - Wrap transport errors (`*url.Error`) with `ErrOffline`.
- [ ] **Step 5:** Run `go test ./internal/api/`. Expected: PASS.
- [ ] **Step 6:** Commit: `feat(api): vedabase API client with limits and typed errors`

---

### Task 4: `library` — sitemap → tree, reading order

**Files:**
- Create: `internal/library/library.go`, `internal/library/order.go`, `internal/library/library_test.go`, `internal/library/testdata/sitemap-sample.xml`

**Interfaces:**
- Consumes: `ref.FromPath`, `ref.Less`, `ref.Recipient`, `ref.TranscriptDate`, `api.Book`
- Produces:

```go
type Node struct {
	Ref      string  // "bg", "bg/2", "bg/2/13"; virtual nodes: "letters#year", "letters#to", "letters#to/abhirama", "transcripts#1966"
	Seg      string  // last segment ("13", "adi", "0a"); for virtual nodes the group key
	Title    string  // set by scraping / fetched items; empty until known
	Leaf     bool    // readable item
	Lazy     bool    // children loaded on demand (letters by year)
	Children []*Node
	Parent   *Node
	Book     string  // book code of the root this node is under
}
func (n *Node) Label() string // Title if set; else defaults below
type Library struct{ Roots []*Node /* + index */ }
func Parse(sitemap io.Reader, books []api.Book) (*Library, error)
func (l *Library) Find(ref string) *Node              // the primary node for a ref (book tree, or By recipient for letters, year group for transcripts)
func (l *Library) Next(n *Node) *Node                 // nil at the end
func (l *Library) Prev(n *Node) *Node
func Position(n *Node) (idx, total int)               // 1-based among leaf siblings
func (l *Library) AddYearGroup(year int, refs []string, titles map[string]string) *Node // used by Task 16 under "letters#year"
```

Default `Label()` values:
- book root → the book title;
- chapter-level container → `Chapter N`;
- `sb` level 1 → `Canto N`;
- `cc` līlā → `Ādi-līlā` / `Madhya-līlā` / `Antya-līlā`;
- leaf → its path after the book, joined with `.` (e.g. `2.13`; for prose books, `Chapter N`);
- front matter `0x` → `Front matter (0x)`;
- `summary`/`notes`/`concluding_words` → title-cased words;
- `letters#to/<name>` → the name with `-` → `, ` and title-cased (`Damodara, Abhirama`);
- transcript year group → the year;
- a letter or transcript leaf → its slug until a Title is set.

- [ ] **Step 1:** Write `sitemap-sample.xml` in the real `<urlset><url><loc>…` format with these locs:
  - `/bg/`, `/bg/0a/`, `/bg/1/`, `/bg/1/1/`, `/bg/1/16-18/`, `/bg/1/19/`, `/bg/2/`, `/bg/2/1/`, `/bg/2/13/`, `/bg/10/`, `/bg/10/1/`
  - `/sb/1/1/1/`, `/sb/1/1/2/`, `/sb/1/2/1/`, `/sb/4/29/1a-2a/`, `/sb/4/29/1b/`
  - `/cc/antya/1/1/`, `/cc/adi/0a/`, `/cc/adi/1/1/`, `/cc/madhya/9/74/`
  - `/lcfl/1/`, `/lcfl/2/`, `/iso/1/`, `/zzz/1/` (unknown book)
  - `/en/library/letters/letter-to-abhirama/`, `/en/library/letters/letter-to-abhirama-2/`, `/en/library/letters/letter-to-aatie/`
  - `/en/library/transcripts/660219bg-new-york/`, `/en/library/transcripts/670101sb-sf/`

  Write them deliberately out of order. `books` = bg, sb, cc, iso, lcfl, letters, transcripts, in that order.
- [ ] **Step 2: Write the failing tests.**

```go
func TestRootsOrder(t *testing.T)  // root Refs == [bg sb cc iso lcfl zzz letters transcripts] (unknown book after known books, before letters)
func TestLeavesAndContainers(t *testing.T) {
	// Find("bg/2").Leaf==false; Find("bg/2/13").Leaf==true; Find("lcfl/1").Leaf==true; Find("bg/0a").Leaf==true
	// children of bg == [0a 1 2 10] in that order; children of cc == [adi madhya antya]
	// children of bg/1 Segs == [1 16-18 19]
}
func TestReadingOrderCrossesChapters(t *testing.T) {
	// Next(Find("bg/1/19")).Ref == "bg/2/1"; Prev(Find("bg/2/1")).Ref == "bg/1/19"
	// Next(Find("bg/0a")).Ref == "bg/1/1"
	// Next(Find("bg/10/1")) == nil  (end of book, does not enter sb)
	// Next(Find("cc/adi/1/1")).Ref == "cc/madhya/9/74"
}
func TestPosition(t *testing.T)    // Position(Find("bg/1/16-18")) == (2,3)
func TestLettersByRecipient(t *testing.T) {
	// letters root children: [letters#year (Lazy, no children), letters#to]
	// letters#to children labels == ["Aatie","Abhirama"]; Abhirama children Refs == [letters/letter-to-abhirama letters/letter-to-abhirama-2]
	// Find("letters/letter-to-aatie").Parent.Ref == "letters#to/aatie"
	// Next within the group: Next(Find("letters/letter-to-abhirama")).Ref == "letters/letter-to-abhirama-2"; Next of the last == nil
}
func TestTranscriptsByYear(t *testing.T) // year groups "1966","1967"; Next of 660219bg-new-york == nil (end of its year group)
func TestLabels(t *testing.T)      // Find("bg/2").Label()=="Chapter 2"; Find("sb/1").Label()=="Canto 1"; Find("cc/adi").Label()=="Ādi-līlā"; Find("bg/2/13").Label()=="2.13"; Find("bg/0a").Label()=="Front matter (0a)"; Title overrides
func TestParseRealScale(t *testing.T) // skip unless testdata/sitemap-en-books.xml exists (not committed): parses in < 1s, > 30000 nodes
```

- [ ] **Step 3:** Run `go test ./internal/library/`. Expected: FAIL.
- [ ] **Step 4:** Implement.
  - Only books that appear in the sitemap get a root.
  - Parse with `encoding/xml`, streaming over `<loc>`. Build the tree by path segments and sort children with `ref.CompareSegment`.
  - Transcripts in a year group are ordered by date, then slug.
  - `Next`/`Prev` under a book root walk a cached, flattened leaf list for that book. Under letters/transcripts they move to the next leaf sibling in the parent.
- [ ] **Step 5:** Run `go test ./internal/library/`. Expected: PASS.
- [ ] **Step 6:** Commit: `feat(library): build contents tree from sitemap with reading order`

---

### Task 5: `store` — paths, caches, settings, state

**Files:**
- Create: `internal/store/paths.go`, `internal/store/cache.go`, `internal/store/settings.go`, `internal/store/store_test.go`

**Interfaces:**
- Consumes: `api.Item`, `api.Parts`
- Produces:

```go
type Paths struct{ Cache, Data, Config, State string } // each already ends in "vedabase-tui"
func DefaultPaths() (Paths, error)
func WriteFileAtomic(path string, data []byte) error   // temp in same dir, fsync, rename, fsync dir; also used by collections and export
type ItemCache struct{ Dir string }                      // <Cache>/items/<ref>.json (ref's "/" as subdirs)
func (c ItemCache) Get(ref string) (api.Item, bool)
func (c ItemCache) Has(ref string) bool
func (c ItemCache) Put(it api.Item) error
type BlobCache struct{ Dir string }
func (c BlobCache) Get(name string) (data []byte, saved time.Time, ok bool)
func (c BlobCache) Put(name string, data []byte) error
type Settings struct {
	ExportDir  string `toml:"export_dir"`  // "~/" expanded on load
	Animations bool   `toml:"animations"`  // default true
}
func LoadSettings(path string) (Settings, error)         // missing file → defaults and writes a commented default file
type State struct {
	LastRef     string    `json:"last_ref"`
	Scroll      int       `json:"scroll"`
	Parts       api.Parts `json:"parts"`
	TreeHidden  bool      `json:"tree_hidden"`
}
func LoadState(path string) State                        // missing/corrupt → State{Parts: api.AllParts()}
func SaveState(path string, s State) error
```

- [ ] **Step 1: Write the failing tests** (use `t.TempDir()` and `t.Setenv`).

```go
func TestDefaultPathsXDG(t *testing.T)   // XDG_CACHE_HOME=/x/c, XDG_CONFIG_HOME=/x/k, XDG_DATA_HOME=/x/d, XDG_STATE_HOME=/x/s → /x/c/vedabase-tui etc.; unset DATA/STATE with HOME=/h → /h/.local/share/vedabase-tui, /h/.local/state/vedabase-tui
func TestItemCacheRoundTrip(t *testing.T) // Put bg/2/13 then Get equal; Has("cc/madhya/9/74")==false; file exists at items/bg/2/13.json
func TestBlobCacheSavedTime(t *testing.T) // Put then Get → saved within 1s of now
func TestWriteFileAtomicNoPartial(t *testing.T) // writing over existing file leaves no *.tmp files in dir; content replaced
func TestSettingsDefaultsAndTilde(t *testing.T) // missing → ExportDir == $HOME/vedabase-exports, Animations true, file created; file with export_dir="~/notes/vb" → $HOME/notes/vb; animations=false honoured
func TestStateRoundTripAndCorrupt(t *testing.T) // save/load equal; corrupt JSON → defaults with AllParts
```

- [ ] **Step 2:** Run `go test ./internal/store/`. Expected: FAIL.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** Run `go test ./internal/store/`. Expected: PASS.
- [ ] **Step 5:** Commit: `feat(store): XDG paths, item/blob caches, settings and state`

---

### Task 6: `collections` — the user-data store (high risk: own Opus review)

**Files:**
- Create: `internal/collections/store.go`, `internal/collections/store_test.go`

**Interfaces:**
- Consumes: `store.WriteFileAtomic`
- Produces:

```go
type Item struct {
	Ref   string    `json:"ref"`
	Book  string    `json:"book"`
	Label string    `json:"label"`
	Added time.Time `json:"added"`
}
type Collection struct {
	Name    string    `json:"name"`
	Created time.Time `json:"created"`
	Items   []Item    `json:"items"`
}
// file: {"version":1,"collections":[...]}
type Store struct{ /* path, in-memory copy */ }
func Open(path string) (s *Store, warning string, err error) // missing → empty, no file written; unparsable → renamed to "<path>.bad-<20060102-150405>", warning names it, empty store
func (s *Store) List() []Collection                           // copy, in creation order
func (s *Store) Get(name string) (Collection, bool)
func (s *Store) Add(name string, it Item) (added bool, err error) // creates collection if needed; exact Ref already present → false, nil
func (s *Store) Remove(name, ref string) error
func (s *Store) Rename(old, new string) error                  // error if new exists or is blank
func (s *Store) Delete(name string) error
func (s *Store) Contains(ref string) bool                      // any collection
```

Every mutation reloads the file (if its mtime or size changed since the last load), applies the change, writes atomically and only then updates memory. If the write fails, memory is unchanged and the error is returned. Names are trimmed and compared case-insensitively; the stored spelling is kept.

- [ ] **Step 1: Write the failing tests.**

```go
func TestAddCreatesAndPersists(t *testing.T)   // Add("On the soul", bg/2/13) → added; reopen → one collection, one item, Label kept
func TestAddDuplicateIsNoop(t *testing.T)      // second Add same ref → added==false; bg/1/16-18 and bg/1/17 both kept (exact ref identity)
func TestContains(t *testing.T)
func TestRenameDeleteRemove(t *testing.T)      // rename to existing name → error; rename "on the soul"→"Soul" case-insensitive lookup works; Remove; Delete
func TestMissingFileWritesNothing(t *testing.T) // Open on missing path → no file created until first Add
func TestCorruptFileMovedAside(t *testing.T) {
	// write "{not json" → Open returns warning containing ".bad-", original bytes preserved in the .bad- file, store empty,
	// and the next Add writes a fresh valid file without touching the .bad- file
}
func TestWriteFailureKeepsMemory(t *testing.T) // make dir read-only (skip if root) → Add returns error; List() unchanged; original file intact
func TestTwoInstances(t *testing.T) {
	// a,_,_ := Open(p); b,_,_ := Open(p); a.Add("X", bg/2/13); b.Add("X", bg/2/14)
	// reopen → collection X has both items
}
func TestNoTempFilesLeft(t *testing.T)         // after several mutations dir holds only collections.json
```

- [ ] **Step 2:** Run `go test ./internal/collections/`. Expected: FAIL.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** Run `go test -race ./internal/collections/`. Expected: PASS.
- [ ] **Step 5:** Commit: `feat(collections): atomic named-collections store`
- [ ] **Step 6:** Run the high-risk review. Dispatch one Opus reviewer on this task's diff alone, with the bar stated: blocking means data loss or corruption, or a broken test; everything else is a ledgered minor. Then do one fix round if needed and have the same reviewer check it.

---

### Task 7: `export` — Markdown for items and collections

**Files:**
- Create: `internal/export/export.go`, `internal/export/export_test.go`, `internal/export/testdata/{bg-2-13-all.md,bg-2-13-no-dev-syn.md,letter-aatie.md,collection.md}`

**Interfaces:**
- Consumes: `api.Item`, `api.Parts`, `ref.Display`, `ref.Filename`, `ref.Slug`, `store.WriteFileAtomic`
- Produces:

```go
type Meta struct{ BookTitle, Chapter string }               // Chapter "" → omitted
func ItemMarkdown(it api.Item, m Meta, p api.Parts, today time.Time) string
type Entry struct{ Ref string; Item *api.Item; Meta Meta } // Item nil → "(unavailable: <ref>)"
func CollectionMarkdown(name string, entries []Entry, p api.Parts, today time.Time) string
func ItemFilename(ref string) string                        // "bg-2-13.md"
func CollectionFilename(name string) string                 // "collection-" + ref.Slug(name) + ".md"
func Write(dir, filename, content string) (path string, existed bool, err error) // MkdirAll + WriteFileAtomic
```

Format (§7 Export):
- Front matter with every string value double-quoted and escaped: `ref`, `book`, `book_title`, `chapter` (when set), `url`, `exported` (`2006-01-02`). Letters and lectures add `date`, `year` (when not 0) and `location` from `Document`.
- Heading `# <ref.Display>`, or for documents `# <Translation>`.
- Then each visible, non-empty part in the order Devanagari, Verse, Synonyms, Translation, Purport, as is, separated by blank lines.
- A collection file:
  - front matter `collection`, `exported`, `count`;
  - `# <name>`;
  - per entry: `## <Display or title>`, then `<BookTitle> · [vedabase.cc](<url>)`, then the parts.

- [ ] **Step 1:** Write golden tests that use the Task 3 fixtures (copy them into `internal/export/testdata/`) and `today = 2026-10-25`:
  - `TestItemAllParts`
  - `TestItemHiddenParts` (Devanagari and Synonyms off: neither text appears)
  - `TestLetterFrontMatter` (`date: "October 28th 1967"`, `location: "Navadvipa"`)
  - `TestCollectionWithMissing` (one present entry and one with `Item == nil` → contains `(unavailable: sb/1/1/1)`)
  - `TestFilenames` (`collection-on-the-soul.md`)
  - `TestWriteReportsExisted` (second write → `existed == true`)

  Golden files are compared byte for byte, and `-update` regenerates them.
- [ ] **Step 2:** Run `go test ./internal/export/`. Expected: FAIL.
- [ ] **Step 3:** Implement. Generate the goldens with `-update`, then read them to check they match §7, including a blank line after the front matter.
- [ ] **Step 4:** Run `go test ./internal/export/`. Expected: PASS.
- [ ] **Step 5:** Commit: `feat(export): Markdown export for items and collections`

---

### Task 8: `render` — theme, display Markdown, render cache, `<mark>`

**Files:**
- Create: `internal/render/theme.go`, `internal/render/render.go`, `internal/render/render_test.go`

**Interfaces:**
- Consumes: `api.Item`, `api.Parts`, `ref.Display`
- Produces:

```go
type Palette struct{ Bg, Fg, Dim, Gold, Rose, Teal, Violet, Bright color.Color } // image/color; values from the mockup: Bg #1b1a17, Fg #d8d2c4, Dim #6d665a, Gold #e8a95b, Rose #e07a8f, Teal #6cc4b4, Violet #b48ee8, Bright #f2ece0
var Default Palette
func GlamourStyle(p Palette) ansi.StyleConfig // emphasis (Sanskrit italics) gold, strong (translation) bright bold, block quote (verse lines) gold italic with no bar indent clutter, h4 section labels teal small caps-ish
func DisplayMarkdown(it api.Item, p api.Parts) string // "#### Devanagari\n\n<text>\n\n#### Verse …"; skips hidden/empty parts; documents: "## <title>\n\n<date — location>\n\n<purport>"
type Renderer struct{ /* style, LRU of 32 entries keyed {ref,width,parts} */ }
func NewRenderer(p Palette) *Renderer
func (r *Renderer) Render(ref, md string, width int, p api.Parts) (string, error)
func Marks(snippet string, hl lipgloss.Style) string // <mark>x</mark> → hl.Render("x"); other tags stripped; html entities unescaped
```

- [ ] **Step 1: Write the failing tests.**

```go
func TestDisplayMarkdownParts(t *testing.T) // all parts → contains "#### Word for word" and "dehinaḥ"; Synonyms off → no "dehinaḥ"; lcfl/1 → no "#### Devanagari" heading (empty part)
func TestDisplayMarkdownDocument(t *testing.T) // letter → starts "## Letter to: Aatie", contains "October 28th 1967 — Navadvipa"
func TestRenderCaches(t *testing.T)         // same key twice → second call < 1ms and identical; different width → re-render
func TestRenderNoANSIWidthOverflow(t *testing.T) // every line of Render(..., 60, ...) has lipgloss.Width <= 60
func TestMarks(t *testing.T)                // "a <mark>soul</mark> &amp; b<br>" with hl=plain style → "a soul & b"; with hl bold → contains the ANSI bold sequence around "soul"
```

- [ ] **Step 2:** Run `go test ./internal/render/`. Expected: FAIL.
- [ ] **Step 3:** Implement. Base `GlamourStyle` on glamour's dark style config, overriding colours from `Palette`.
- [ ] **Step 4:** Run `go test ./internal/render/`. Expected: PASS.
- [ ] **Step 5:** Commit: `feat(render): palette, glamour style, display Markdown and render cache`

---

### Task 9: `service` — cache plus network

**Files:**
- Create: `internal/service/service.go`, `internal/service/service_test.go`

**Interfaces:**
- Consumes: `api.Client`, `store.ItemCache`, `store.BlobCache`, `library.Parse`
- Produces:

```go
type Service struct {
	Client *api.Client
	Items  store.ItemCache
	Blobs  store.BlobCache
	Now    func() time.Time
}
func (s *Service) Item(ctx context.Context, ref string) (it api.Item, cached bool, err error) // cache first; on fetch, Put
func (s *Service) Cached(ref string) bool
func (s *Service) Books(ctx context.Context) ([]api.Book, error)                              // blob "books.json", weekly refresh, falls back to stale cache on error
func (s *Service) Library(ctx context.Context, progress func(done, total int64)) (lib *library.Library, stale bool, err error)
	// blob "sitemap-en-books.xml": cached → parse it, stale = older than 7 days; none cached → Fetch with progress, Put, parse
func (s *Service) RefreshLibrary(ctx context.Context) (*library.Library, error)                // background re-download when stale
func (s *Service) Search(ctx context.Context, q, book string, page int) (api.SearchPage, error) // never cached
func (s *Service) Page(ctx context.Context, url string) ([]byte, error)                        // HTML pages, blob keyed by url path, 30-day expiry, stale fallback on error
```

- [ ] **Step 1: Write the failing tests** against `httptest` (reuse the Task 3 and Task 4 fixtures by copying them into `testdata/`) and a fake `Now`.

```go
func TestItemCachesAfterFetch(t *testing.T)   // first call cached==false and 1 request; second cached==true and still 1 request
func TestLibraryFirstLaunch(t *testing.T)     // no blob → progress called, lib has bg root, stale==false
func TestLibraryStale(t *testing.T)           // blob saved 8 days ago (Now shifted) → no request made, stale==true
func TestLibraryOfflineNoCache(t *testing.T)  // closed server, empty cache → errors.Is(err, api.ErrOffline)
func TestLibraryOfflineWithCache(t *testing.T) // closed server, cached blob → lib ok
func TestPageExpiry(t *testing.T)             // fresh blob → no request; 31 days old → refetch; refetch fails offline → stale bytes returned, nil error
```

- [ ] **Step 2:** Run `go test ./internal/service/`. Expected: FAIL.
- [ ] **Step 3:** Implement. `Library` needs `Books` for root order; if `Books` fails and nothing is cached, return its error.
- [ ] **Step 4:** Run `go test ./internal/service/`. Expected: PASS.
- [ ] **Step 5:** Commit: `feat(service): cache-first items, library and page loading`

---

### Task 10: UI — contents tree component

**Files:**
- Create: `internal/ui/keys.go`, `internal/ui/tree.go`, `internal/ui/tree_test.go`, `internal/ui/testutil_test.go`

**Interfaces:**
- Consumes: `library.Library`, `library.Node`, `render.Palette`
- Produces:

```go
// keys.go
type keySeq struct{ pendingG bool }
func (k *keySeq) Resolve(msg tea.KeyPressMsg) string // returns logical key: "j","k","h","l","top"(gg),"bottom"(G),"halfdown"(ctrl+d / pgdown),"halfup","enter", or msg.String(); "g" alone returns "" and arms pendingG
// tree.go
type Tree struct{ /* lib, expanded set, rows, cursor, offset, w, h, filter, starred func(string) bool, pal */ }
func NewTree(lib *library.Library, starred func(ref string) bool, pal render.Palette) Tree
func (t Tree) Update(msg tea.Msg) (Tree, tea.Cmd)
func (t Tree) View() string
func (t *Tree) SetSize(w, h int)
func (t *Tree) Reveal(n *library.Node)      // expand ancestors, put cursor on n, scroll it into view
func (t Tree) Selected() *library.Node
func (t *Tree) SetFilter(s string)          // case-insensitive, diacritics folded; shows matches plus their ancestors, expanded
func (t *Tree) SetLibrary(lib *library.Library) // keep expansion and cursor by Ref
type OpenMsg struct{ Node *library.Node }   // Enter or l on a leaf
type ExpandLazyMsg struct{ Node *library.Node } // expanding a Lazy node with no children
```

Keys in the tree:
- `j`/`k` and arrows: move.
- `l`, `→` or Enter on a container: expand. `l` or Enter on a leaf: emit `OpenMsg`.
- `h` or `←`: collapse, or jump to the parent if already collapsed.
- `gg`/`G`: first/last row.
- `Ctrl-d`/`Ctrl-u`: half a page.

Rows show `▸`/`▾` for containers, indented 2 spaces per depth, `★` before starred leaves, the label truncated to the width with `…`, and the cursor row in gold reverse.

- [ ] **Step 1:** In `testutil_test.go`, write `key(s string) tea.KeyPressMsg`. A single rune `r` → `{Code: r, Text: s}`; `"ctrl+d"` → `{Code: 'd', Mod: tea.ModCtrl}`; `"enter"`, `"tab"`, `"esc"`, `"up"`, `"down"` → the matching `tea.Key*` code. Also write `sampleLibrary(t)`, which parses `../library/testdata/sitemap-sample.xml` with the books decoded from `../api/testdata/books.json` (real titles, so the transcripts root is "Lectures and conversations").
- [ ] **Step 2: Write the failing tests.**

```go
func TestTreeExpandAndOpen(t *testing.T)   // start on "Bhagavad-gītā…" root; l → expanded, rows include "Chapter 1"; j,j to "Chapter 1" (row after "Front matter (0a)"), l → expanded, j → "1.1"; l → cmd yields OpenMsg{Node.Ref=="bg/1/1"}
func TestTreeCollapseToParent(t *testing.T) // on bg/1/1: h → cursor on bg/1 (still expanded); h → collapsed
func TestTreeGGAndG(t *testing.T)          // G → last root ("Lectures and conversations"); g,g → first root
func TestTreeReveal(t *testing.T)          // Reveal(Find("cc/madhya/9/74")) → Selected().Ref == "cc/madhya/9/74"; its ancestors expanded
func TestTreeStarred(t *testing.T)         // starred returns true for bg/2/13 → that row's View contains "★"
func TestTreeFilter(t *testing.T)          // SetFilter("abhi") → visible leaf rows only under "Abhirama"; SetFilter("") restores
func TestTreeLazy(t *testing.T)            // expanding letters#year → cmd yields ExpandLazyMsg
func TestTreeNarrow(t *testing.T)          // SetSize(10,3) then View() → no panic, every line lipgloss.Width <= 10
```

- [ ] **Step 3:** Run `go test ./internal/ui/ -run Tree`. Expected: FAIL.
- [ ] **Step 4:** Implement.
- [ ] **Step 5:** Run `go test ./internal/ui/ -run Tree`. Expected: PASS.
- [ ] **Step 6:** Commit: `feat(ui): contents tree component with vim keys and filter`

---

### Task 11: UI — app shell, reader, first runnable app

**Files:**
- Create: `internal/ui/app.go`, `internal/ui/reader.go`, `internal/ui/toast.go`, `internal/ui/msgs.go`, `internal/ui/app_test.go`
- Modify: `cmd/vedabase/main.go` (wire paths → settings, state, service, collections → `ui.New` → `tea.NewProgram(...).Run()`; on exit save state)

**Interfaces:**
- Consumes: `Tree`, `keySeq`, `render.Renderer`, `render.DisplayMarkdown`, `library.*`, `store.State`, `store.Settings`, `collections.Store`
- Produces:

```go
type Backend interface {
	Item(ctx context.Context, ref string) (api.Item, bool, error)
	Cached(ref string) bool
	Library(ctx context.Context, progress func(done, total int64)) (*library.Library, bool, error)
	RefreshLibrary(ctx context.Context) (*library.Library, error)
	Search(ctx context.Context, q, book string, page int) (api.SearchPage, error)
	Page(ctx context.Context, url string) ([]byte, error)
}
type Options struct {
	Backend     Backend
	Collections *collections.Store
	Settings    store.Settings
	State       store.State
	SaveState   func(store.State) error
	Now         func() time.Time
}
func New(o Options) *App          // App implements tea.Model (v2: View() tea.View, alt screen on)
// msgs.go — messages later tasks also use
type itemLoadedMsg struct{ ref string; item api.Item; err error }
type renderedMsg struct{ ref string; width int; parts api.Parts; out string }
type libraryMsg struct{ lib *library.Library; stale bool; err error }
type progressMsg struct{ done, total int64 }
type ToastMsg struct{ Text string; Error bool }      // exported so overlays can emit it
func (a *App) Toast(text string, isErr bool) tea.Cmd
func (a *App) Open(ref string) tea.Cmd                // used by search, collections, goto
```

Behaviour:
1. **Startup screen** while the library loads: a plain text progress line (Task 15 makes it animated). On `libraryMsg` with an error and no library, show `Can't reach vedabase.cc — press r to retry, q to quit`. If `stale`, run `RefreshLibrary` in the background and swap the library in with `Tree.SetLibrary` when it arrives.
2. **Layout:**
   - top bar: breadcrumb of ancestors' `Label()` joined with ` › `, and `Position` as `idx / total` on the right;
   - the tree takes 30% of the width, clamped to 24–40 columns;
   - a bottom key-hint bar shows the parts state: `1 dev 2 verse 3 words 4 trans 5 purport`, hidden parts struck through or dim.

   `t` toggles the tree, Tab switches focus, and below 70 columns the tree is hidden automatically.
3. **Reader:**
   - `Open(ref)` sets the current ref, then:
     - cached → load it;
     - not cached → show `Loading…` and fetch via a Cmd.
   - Then render in a Cmd at the reader width with the current parts.
   - Drop any `itemLoadedMsg` or `renderedMsg` whose ref, width or parts don't match the current ones.
   - After display, prefetch `lib.Next(node)` if it isn't cached; the result is ignored except for caching.
   - On a resize, re-render after 100 ms of quiet.
   - Keys in the reader: `j`/`k` scroll, `Ctrl-d`/`Ctrl-u` half page, `gg`/`G`, `h`/`l` previous/next item (at a book's end: toast `End of <book title>`), `1`–`5` toggle parts (re-render, persist into state).
   - The tree follows: `Reveal(node)` on every open.
4. **Errors in the reader:**
   - `ErrOffline` + not cached → `You're offline and this hasn't been downloaded yet.`
   - `RateLimitedError` → `vedabase.cc asked us to wait <n>s` (Task 15 adds the countdown and auto-retry).
   - Anything else → toast in rose plus the message in the reader.
5. **State:**
   - On start, open `State.LastRef` if set, else the first leaf of `bg`, restoring the scroll offset after the first render.
   - Persist `LastRef`, `Scroll`, `Parts` and `TreeHidden` on quit (`q` at top level, Ctrl-C).
6. **Toast** (`toast.go`): a queue of one visible toast, bottom-right, gone after 2.5 s via `tea.Tick`. Plain for now; Task 15 animates it.

- [ ] **Step 1:** Write `fakeBackend` in `app_test.go`. It wraps the sample library and a map of items taken from the Task 3 fixtures, plus synthetic ones. `Item` can be made slow for a chosen ref, and it records calls. Also write a helper `drive(a *App, msgs ...tea.Msg) *App` that runs `Update` and executes the returned Cmds synchronously, feeding their messages back in (skipping `tea.Tick` cmds unless the test asks for them).
- [ ] **Step 2: Write the failing tests.**

```go
func TestStartOpensLastRef(t *testing.T)     // State.LastRef "bg/2/13" → after drive: reader shows "As the embodied soul"; top bar contains "Chapter 2" and "2 / 2"
func TestNextPrevCrossesChapter(t *testing.T) // at bg/1/19, l → current ref bg/2/1; h → bg/1/19
func TestEndOfBookToast(t *testing.T)        // at bg/10/1, l → current ref unchanged, toast "End of Bhagavad-gītā As It Is"
func TestTogglePartsPersist(t *testing.T)    // press 3 → View lacks "dehinaḥ"; quit → SaveState called with Parts.Synonyms==false
func TestStaleResponseDropped(t *testing.T)  // open slow ref A, then open B (fast); deliver A's itemLoadedMsg late → reader still shows B
func TestStaleRenderDropped(t *testing.T)    // renderedMsg with old width → ignored
func TestPrefetchNext(t *testing.T)          // open bg/1/16-18 → backend.Item called for bg/1/19 afterwards
func TestOfflineMessages(t *testing.T)       // Library returns ErrOffline → View contains "Can't reach vedabase.cc"; r retries (second Library call); Item ErrOffline → reader shows "You're offline"
func TestNarrowTerminal(t *testing.T)        // WindowSizeMsg{30,8} → no panic, tree hidden, j scrolls
func TestTreeToggleAndFocus(t *testing.T)    // t hides tree (View has no "▸"); tab moves focus: with focus on tree, j moves tree cursor not reader
```

- [ ] **Step 3:** Run `go test ./internal/ui/`. Expected: FAIL.
- [ ] **Step 4:** Implement `App`, `Reader`, toast and `main.go`. In `main.go`, show the `collections.Open` warning as the first toast. Follow the render path from `docs/spike-render.md`.
- [ ] **Step 5:** Run `go test ./internal/ui/`. Expected: PASS. Then `make build && ./vedabase` by hand: browse the BG tree, read 2.13, press `l` a few times, toggle `3`, quit and relaunch. It should reopen at the same verse with word-for-word still hidden.
- [ ] **Step 6:** Commit: `feat(ui): two-pane app shell and reader`

---

### Task 12: UI — search overlay

**Files:**
- Create: `internal/ui/search.go`, `internal/ui/search_test.go`
- Modify: `internal/ui/app.go` (route `/`, `n`, `N`, overlay stack)

**Interfaces:**
- Consumes: `Backend.Search`, `render.Marks`, `App.Open`
- Produces: `type Search struct{…}`, `func NewSearch(pal render.Palette) Search`, `type searchResultsMsg struct{ q, scope string; page api.SearchPage; err error }`

Behaviour (§7 Search):
- `/` opens a centred overlay with a `textinput` and a scope line `All · This book · Letters · Lectures`, cycled by Tab. "This book" is the current item's `Book`, and the line names it.
- Enter runs page 1; typing alone never searches.
- The results list shows `Display(ref)` with the book title, the translation trimmed to one line, and `snippet_purport` (else `snippet_translation`) through `Marks` in gold-bold highlight.
- `j`/`k` move. Reaching the last row while `has_more` loads the next page; a `Loading more…` row is shown meanwhile.
- Enter opens the result and hides the overlay but keeps its state. Esc in the reader with a live search reopens it, scroll kept.
- `n`/`N` anywhere open the next/previous result, loading the next page when needed.
- Search errors: `RateLimitedError` → the inline line `Wait <n>s before searching again`; others → a toast.

- [ ] **Step 1: Write the failing tests** with `fakeBackend.Search` returning a 2-page result set.

```go
func TestSearchRunsOnEnterOnly(t *testing.T) // type "soul" → 0 Search calls; enter → 1 call with q "soul", book ""
func TestSearchScope(t *testing.T)           // current item bg/2/13, tab → scope "This book (Bhagavad-gītā As It Is)", enter → book "bg"; tab,tab → "letters"
func TestSearchPaging(t *testing.T)          // page 1 has_more → moving to the last row triggers page 2 call; results appended
func TestSearchOpenAndNext(t *testing.T)     // enter on result 1 → reader ref == result1.Ref, overlay hidden; n → result2; N → result1; esc → overlay visible with cursor on result1
func TestSearchMarksHighlighted(t *testing.T) // View contains "soul" and no "<mark>"
```

- [ ] **Step 2:** Run `go test ./internal/ui/ -run Search`. Expected: FAIL.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** Run `go test ./internal/ui/`. Expected: PASS.
- [ ] **Step 5:** Commit: `feat(ui): full-text search overlay`

---

### Task 13: UI — collections and export

**Files:**
- Create: `internal/ui/collections.go`, `internal/ui/export.go`, `internal/ui/collections_test.go`
- Modify: `internal/ui/app.go` (keys `s`, `'`, `e`; star check passed to the tree)

**Interfaces:**
- Consumes: `collections.Store`, `export.*`, `Backend.Item`, `Settings.ExportDir`, `App.Toast`, `App.Open`
- Produces:
  - `type Picker struct{…}`: the `s` overlay.
  - `type CollectionsView struct{…}`: the `'` overlay.
  - `type exportDoneMsg struct{ path string; existed bool; missing int; err error }`
  - `type exportProgressMsg struct{ done, total int }`

Behaviour (§7 Collections, Export):
- **`s`** opens the picker on the current reader item, or the selected tree leaf when the tree has focus. It lists existing collections, plus a `+ New collection…` row that turns into a text input. Choosing one calls `Store.Add`:
  - the label is `Display(ref) — <first line of translation, ** stripped, ≤ 60 chars>`, or the title for documents;
  - added → toast `★ Added to <name>`;
  - already there → toast `Already in <name>`;
  - error → rose toast.

  Afterwards the tree's ★ reflects `Store.Contains`.
- **`'`** opens the collections view:
  - the list shows name and count;
  - Enter shows the items;
  - Enter on an item opens it;
  - `d` on an item removes it, after confirming with `y`;
  - `R` on a collection renames it (inline input);
  - `D` deletes it, after confirming with `y`;
  - `e` on a collection exports it;
  - Esc goes back or closes.
- **`e`** in the reader or tree exports the current item with the current parts:
  - it calls `export.ItemMarkdown` with Meta from the book title and the chapter ancestor's `Label()`, then `export.Write`;
  - the toast is `✓ Exported <path with $HOME as ~>`, or `✓ Overwrote …` when the file existed.
- **Collection export** runs in a Cmd goroutine:
  - for each item, use the cache if available, else `Backend.Item`; the limiter applies;
  - it sends `exportProgressMsg` through a channel listened to by a Cmd, and the status bar shows `Exporting <name> 3/12`;
  - at the end it writes the file and sends `exportDoneMsg`;
  - failed items become `Entry{Item: nil}`, and the toast adds `(<n> unavailable)`;
  - the user can keep reading meanwhile.

- [ ] **Step 1: Write the failing tests** (collections store in `t.TempDir()`, `ExportDir` in a temp dir).

```go
func TestStarAddsToNewCollection(t *testing.T) // at bg/2/13: s, select "+ New collection…", type "On the soul", enter → store has it; toast "★ Added to On the soul"; tree row shows ★
func TestStarDuplicateToast(t *testing.T)      // second time → toast "Already in On the soul"
func TestCollectionsViewOpenItem(t *testing.T) // ' → enter on collection → enter on item → reader ref == bg/2/13
func TestRemoveRenameDelete(t *testing.T)      // d,y removes; R renames; D,y deletes; D,n keeps
func TestExportItem(t *testing.T)              // e → file bg-2-13.md in ExportDir; with Synonyms hidden it lacks "dehinaḥ"; second e → toast starts "✓ Overwrote"
func TestExportCollectionFetchesMissing(t *testing.T) // collection with cached bg/2/13 and uncached bg/2/1 and failing sb/1/1/1 → backend.Item called for bg/2/1 and sb/1/1/1; file contains both texts and "(unavailable: sb/1/1/1)"; toast mentions "1 unavailable"
```

- [ ] **Step 2:** Run `go test ./internal/ui/ -run 'Star|Collection|Export|Remove'`. Expected: FAIL.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** Run `go test ./internal/ui/`. Expected: PASS.
- [ ] **Step 5:** Commit: `feat(ui): named collections and Markdown export`

---

### Task 14: UI — go-to, tree filter, help

**Files:**
- Create: `internal/ui/goto.go`, `internal/ui/help.go`, `internal/ui/goto_test.go`
- Modify: `internal/ui/app.go`

**Interfaces:**
- Consumes: `ref.ParseGoto`, `library.Find`, `Tree.SetFilter`, `App.Open`

Behaviour:
- **`:`** opens a one-line prompt at the bottom with the placeholder `bg 2.13 · sb 1.2.6 · cc madhya 9.74`. On Enter it runs `ParseGoto`:
  - a parse error → toast with the error;
  - `lib.Find(ref)` is nil → toast `No <input> in the library`;
  - otherwise `Open`.

  Esc cancels.
- **`f`** with the tree focused opens a filter line under the tree. Each keystroke calls `SetFilter`. Enter keeps the filter and returns to the tree; Esc clears it.
- **`?`** toggles a help overlay listing every key in §7's table, grouped Reader / Tree / Overlays. Any key closes it.

- [ ] **Step 1: Write the failing tests.**

```go
func TestGotoOpens(t *testing.T)       // ":", type "cc m 9.74", enter → reader ref "cc/madhya/9/74"
func TestGotoUnknown(t *testing.T)     // ":bg 99.1" enter → toast "No bg 99.1 in the library", ref unchanged
func TestFilterKey(t *testing.T)       // focus tree, "f", type "abhi" → tree View contains "Abhirama" and not "Bhagavad"; esc → filter cleared
func TestHelpOverlay(t *testing.T)     // "?" → View contains "Add to collection"; any key closes
```

- [ ] **Step 2:** Run `go test ./internal/ui/ -run 'Goto|Filter|Help'`. Expected: FAIL.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** Run `go test ./internal/ui/`. Expected: PASS.
- [ ] **Step 5:** Commit: `feat(ui): go-to reference, tree filter and help`

---

### Task 15: UI — colour and motion

**Files:**
- Create: `internal/ui/effects.go`, `internal/ui/effects_test.go`
- Modify: `internal/ui/app.go`, `internal/ui/reader.go`, `internal/ui/toast.go`, `internal/ui/collections.go`

**Interfaces:**
- Consumes: Bubbles `progress`, `spinner`; `Settings.Animations`; `render.Palette`
- Produces:

```go
type Effects struct{ /* active effects, frame counter, enabled bool */ }
type frameMsg struct{ id int }   // tick at 60 ms while any effect is live; no ticks when idle
func (e *Effects) Start(kind EffectKind, now time.Time) tea.Cmd
func (e *Effects) FinishAll()    // called on every key press before handling it
type EffectKind int              // Sparkle, Burst, FadeIn, Toast, Countdown
```

The six effects (§8):
1. **Startup:** the Bubbles `progress` bar with a gradient from Rose → Gold → Teal → Violet, fed by `progressMsg`, labelled `2.1 / 3.7 MB`, and the title `ॐ vedabase`.
2. **Sparkle + shimmer:** while a reader load is in flight, `✦ ✧` cycle through Gold/Rose/Teal/Violet next to the heading, and 3 placeholder lines of `░` shimmer with a moving brighter band. It starts only after 120 ms of loading, so cached items never flash it.
3. **Star burst:** on a successful `Add`, the tree's ★ and the toast's ★ pulse while 4 sparkles move outwards over about 500 ms.
4. **Toast:** slides in from the right over 3 frames and fades over its last 300 ms. Errors are Rose.
5. **Fade-in:** a new item's rendered text goes from Dim to full colour over ≤ 150 ms. While fading, it is the same string re-rendered with a dimming style. If that proves too costly at 100k characters, fade only the visible viewport lines.
6. **Countdown:** on `RateLimitedError`, the reader or search line shows `Slow down a moment ☕` with a Rose bar shrinking over `RetryAfter`. At zero, it retries the request itself.

Rules:
- With `animations = false`, effects render their final frame immediately and no `frameMsg` ticks are scheduled.
- The countdown still waits; it just shows a static `Retrying in <n>s` that updates once a second.
- Any key press calls `FinishAll` first.

- [ ] **Step 1: Write the failing tests.**

```go
func TestNoTicksWhenIdle(t *testing.T)            // fresh Effects: no Cmd scheduled after Update(frameMsg) with nothing live
func TestKeyFinishesEffects(t *testing.T)         // start FadeIn, press j → effect gone, reader View equals the final render
func TestAnimationsOff(t *testing.T)              // Settings.Animations=false: open uncached slow item → no frameMsg Cmd; star add → toast shown, no burst ticks
func TestSparkleOnlyAfterDelay(t *testing.T)      // cached item open → View never contains "✦"; slow uncached → after 120 ms tick, View contains "✦"
func TestCountdownRetries(t *testing.T)           // Item returns RateLimitedError{2s} once → View contains "Slow down a moment"; after advancing 2s of ticks → Item called again, text shown
func TestStartupProgressBar(t *testing.T)         // progressMsg{2.1MB,3.7MB} → View contains "2.1 / 3.7 MB"
```

- [ ] **Step 2:** Run `go test ./internal/ui/ -run 'Ticks|Finishes|AnimationsOff|Sparkle|Countdown|Startup'`. Expected: FAIL.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** Run `go test ./internal/ui/`. Expected: PASS. Then run `make build && ./vedabase` by hand with `rm -rf ~/.cache/vedabase-tui` first: watch the progress bar, open uncached verses (sparkles), star something (burst), export (toast). Set `animations = false` in the config and check that it's calm.
- [ ] **Step 5:** Commit: `feat(ui): colour and motion effects`

---

### Task 16: Scraped titles and letters by year

**Files:**
- Create: `internal/scrape/scrape.go`, `internal/scrape/scrape_test.go`, `internal/scrape/testdata/{bg.html,sb-1.html,cc-adi.html,letters.html,letters-1967-p1.html}`
- Modify: `internal/ui/app.go` (expanding containers triggers title fetch; `ExpandLazyMsg` loads years), `internal/library/library.go` only if `AddYearGroup` needs a fix

**Interfaces:**
- Consumes: `Backend.Page`, `library.Node.Title`, `library.Library.AddYearGroup`, `ref.FromPath`
- Produces:

```go
func ChapterTitles(page []byte, parentRef string) map[string]string // child ref → title, e.g. "bg/2" → "Chapter 2: Contents of the Gītā Summarized"; front matter refs too if listed
func LetterYears(page []byte) []YearCount                          // from the year facet on /en/library/letters/
type YearCount struct{ Year, Count int }
func LetterYearPage(page []byte) (entries []LetterEntry, hasNext bool) // /en/library/letters/?year=Y&page=N
type LetterEntry struct{ Ref, Title, Date, Place string }           // "letters/letter-to-aatie", "Letter to: Aatie", "October 28th 1967", "Navadvipa"
```

- [ ] **Step 1:** Record the fixtures once with `curl -s`:
  - `https://vedabase.cc/bg/`, `/sb/1/` and `/cc/adi/`;
  - `/en/library/letters/`;
  - `/en/library/letters/?year=1967`.

  Confirm the page-2 URL form from the "Next →" link in the last fixture, and write it in a comment in `scrape.go`.
- [ ] **Step 2: Write the failing tests.**

```go
func TestChapterTitlesBG(t *testing.T)   // titles["bg/2"] == "Chapter 2: Contents of the Gītā Summarized"; len(titles) >= 18
func TestChapterTitlesSBAndCC(t *testing.T) // non-empty, keys under "sb/1/" and "cc/adi/"
func TestLetterYears(t *testing.T)       // contains {1967,264} and {1969,760}; sorted by year
func TestLetterYearPage(t *testing.T)    // 20 entries; first has Title starting "Letter to:", Date contains "1967"; hasNext == true
func TestGarbageHTML(t *testing.T)       // []byte("<html><body>nope") → empty results, no panic
```

- [ ] **Step 3:** Run `go test ./internal/scrape/`. Expected: FAIL.
- [ ] **Step 4:** Implement with `golang.org/x/net/html`.
  - Match anchors by `href` using `ref.FromPath`, and take their text with whitespace collapsed.
  - For letter entries, take the date and place from the line that follows the title, split on ` — `.
- [ ] **Step 5:** Wire it into the UI:
  - **Expanding a book or container** whose children have no Title calls `Page(<site URL of the container>)`, applies `ChapterTitles` to the children, and re-renders the tree. Failures are silent and the default labels stay.
  - **Expanding `letters#year`** fetches `/en/library/letters/`, then calls `AddYearGroup` for each year with no refs yet, marking each year node Lazy.
  - **Expanding a year** loads page 1. Moving the tree cursor to the last loaded row of a year with more pages loads the next page.
  - **Letter titles from year pages** are also copied onto the nodes under By recipient, keyed by ref.
  - **After opening any item**, set the node's Title from `Translation` when it's a front-matter page or a document.
- [ ] **Step 6: Write the failing UI tests**, add them to `app_test.go` with `fakeBackend.Page` serving the fixtures, then make them pass:

```go
func TestChapterTitlesApplied(t *testing.T) // expand bg → row "Chapter 2: Contents of the Gītā Summarized"
func TestLettersByYearLazy(t *testing.T)    // expand Letters › By year → row "1967 (264)"; expand 1967 → 20 rows; cursor to last → second Page call for page 2
func TestTitleFailureKeepsDefaults(t *testing.T) // Page returns error → row "Chapter 2"
```

- [ ] **Step 7:** Run `go test ./...`. Expected: PASS.
- [ ] **Step 8:** Commit: `feat: scraped chapter titles and letters by year`

---

### Task 17: Finish — README, static build, branch review

**Files:**
- Create: `README.md`
- Modify: anything the review asks for

- [ ] **Step 1:** Write `README.md` covering:
  - what it is;
  - build (`make build`) and install (`go install ./cmd/vedabase`);
  - the key table from §7;
  - where files live (§9);
  - the settings keys `export_dir` and `animations`;
  - being a good neighbour: rate limits and caching;
  - a link to https://vedabase.cc/en/api/.
- [ ] **Step 2:** Run `make test && make build && file vedabase`. Expected: all tests pass, and the output contains `statically linked`. Run `go vet ./...`. Expected: no output.
- [ ] **Step 3:** Manual smoke run through all of §2's goals:
  - read in order across a chapter boundary in BG and SB, and across a līlā in CC;
  - toggle each part;
  - search in each scope and use `n`/`N`;
  - add items to two collections, then export one verse and one collection;
  - browse Letters by year and by recipient, and Lectures by year;
  - restart: it resumes at the last verse;
  - disconnect the network: cached reading works and uncached reading shows the offline message.

  Note any failures in the ledger.
- [ ] **Step 4:** Commit: `docs: README`
- [ ] **Step 5:** Run one Opus review of the whole branch, with the bar stated (blocking = breaks behaviour, tests, or the collections safety boundary). Then one fix round if needed, checked by the same reviewer.

---

## Execution notes (from the user's global CLAUDE.md)

- There is no per-task review. After each task the coordinator reads the whole diff and runs `make test` plus the task's commands.
- Task 6 gets its own Opus review. The whole branch gets one Opus review at the end (Task 17).
- Subagent briefs should each fit in about 30 turns. Tasks 11, 13 and 15 are the largest: if a brief runs long, split it into implement-and-test, then the manual run.
