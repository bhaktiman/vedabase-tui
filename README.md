# vedabase-tui

A terminal reader for Śrīla Prabhupāda's books, letters and lectures, built on
the public [vedabase.cc API](https://vedabase.cc/en/api/). Written in Go with
Bubble Tea v2; it ships as a single static binary.

You get a book tree beside the text, the five parts of a verse (Devanagari,
verse, synonyms, translation, purport) that you can toggle, full-text search,
collections you can save verses into, and Markdown export. It resumes where you
left off, and anything you have already read works offline.

## Build and run

```
make build        # CGO_ENABLED=0, writes ./vedabase
./vedabase
```

Install into `$GOBIN` instead:

```
go install ./cmd/vedabase
```

## Keys

Press `?` in the app for the same list.

| Key | Action |
|---|---|
| `j` / `k` | Scroll a line (reader); move down / up (tree) |
| `h` / `l` | Previous / next item (reader); collapse / expand (tree) |
| `Ctrl-d` / `Ctrl-u` | Half page down / up |
| `gg` / `G` | Top / bottom (first / last in the tree) |
| `1` – `5` | Toggle Devanagari, verse, synonyms, translation, purport |
| `Enter` | Open item / expand (tree) |
| `f` | Filter the tree |
| `Tab` | Switch focus between tree and reader |
| `t` | Hide / show the tree |
| `/` | Full-text search |
| `n` / `N` | Next / previous search result |
| `:` | Go to a reference |
| `s` | Add the current item to a collection |
| `'` | Open collections |
| `e` | Export the current item |
| `T` | Choose a colour theme |
| `?` | Help |
| `q` / `Esc` | Close overlay; quit |
| Mouse | Click a pane to focus it, a tree row to select or open it, a part label to toggle it; the wheel scrolls |

While mouse mode is on, selecting text in most terminals (Windows Terminal,
iTerm2, GNOME Terminal) needs Shift+drag, or set `mouse = false`.

## Themes

`T` opens a theme picker: Vedabase (the default), Terminal, Dracula, Monokai,
Solarized Dark, Solarized Light, Gruvbox Dark, Nord, Tokyo Night and
Catppuccin Mocha. Moving the cursor previews a theme live; Enter keeps and
remembers it, Esc goes back. Themes set the terminal's background while the app
runs and restore it on exit; a terminal without that support keeps its own
background. `Terminal` has the same warm colours but keeps your own background.

## Where files live

Paths follow the XDG variables when they are set.

| What | Where |
|---|---|
| Downloaded items | `~/.cache/vedabase-tui/items/` (safe to delete) |
| Sitemap, index pages, year lists | `~/.cache/vedabase-tui/` |
| Collections | `~/.local/share/vedabase-tui/collections.json` |
| Settings | `~/.config/vedabase-tui/config.toml` |
| Last item, scroll, toggles | `~/.local/state/vedabase-tui/state.json` |
| Exports | `~/vedabase-exports/` (configurable) |

Collections are your own data. They are written atomically (temp file, fsync,
rename), and a file that fails to parse is renamed aside to
`collections.json.bad-<timestamp>` rather than overwritten.

## Settings

On first run a commented `config.toml` is written for you:

```toml
# Where Markdown exports are written. A leading "~/" means your home directory.
# export_dir = "~/vedabase-exports"

# Set to false to turn off all motion (fades, highlights, slides).
# animations = true

# Set to false to turn off mouse support, so your terminal's own text
# selection works without holding Shift.
# mouse = true
```

## Being a good neighbour

vedabase.cc is a free service. The client limits itself to 100 verse requests
and 25 searches per minute (page fetches count against the verse budget),
honours `Retry-After` on a 429, and caches everything it downloads, so each
item is fetched at most once. It sends `User-Agent: vedabase-tui/<version>`.

The API has no listing for letters and lectures, so those are listed by
scraping the site's pages. That may break if the site changes; the app then
falls back to search.
