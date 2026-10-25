# vedabase-tui

[![Latest release](https://img.shields.io/github/v/release/bhaktiman/vedabase-tui?sort=semver)](https://github.com/bhaktiman/vedabase-tui/releases/latest)
[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)](go.mod)
[![Platforms](https://img.shields.io/badge/platforms-Linux%20%7C%20macOS%20%7C%20Windows-555)](https://github.com/bhaktiman/vedabase-tui/releases/latest)

A terminal reader for Śrīla Prabhupāda's books, letters and lectures, built on
the public [vedabase.cc API](https://vedabase.cc/en/api/). Written in Go with
Bubble Tea v2; it ships as a single static binary.

You get a book tree beside the text, the five parts of a verse (Devanagari,
verse, synonyms, translation, purport) that you can toggle, full-text search,
collections you can save verses into, and Markdown export. It resumes where you
left off, and anything you have already read works offline.

## Install

Linux and macOS (Intel or Apple Silicon):

```sh
curl -fsSL https://github.com/bhaktiman/vedabase-tui/releases/latest/download/install.sh | sh
```

The script downloads the right binary from the
[latest release](https://github.com/bhaktiman/vedabase-tui/releases/latest),
checks it against `checksums.txt` and puts it in `~/.local/bin`; no sudo needed.
Then run `vedabase`. To install elsewhere or pin a version:

```sh
curl -fsSL https://github.com/bhaktiman/vedabase-tui/releases/latest/download/install.sh | INSTALL_DIR=~/bin VERSION=v0.1.0 sh
```

Windows: download `vedabase_windows_amd64.zip` from the
[latest release](https://github.com/bhaktiman/vedabase-tui/releases/latest),
unzip it and run `vedabase.exe` in Windows Terminal.

To update, run the install command again. To uninstall, delete the binary
(`rm ~/.local/bin/vedabase`).

## Build from source

Needs Go 1.26.

```
git clone https://github.com/bhaktiman/vedabase-tui.git && cd vedabase-tui
make build        # CGO_ENABLED=0, writes ./vedabase
./vedabase
```

Install into `$GOBIN` instead:

```
go install ./cmd/vedabase
```

`make release VERSION=v0.1.0` cross-builds every platform into `dist/` with
`checksums.txt`, ready to upload to a GitHub release.

## Keys

Press `?` in the app for the same list.

| Key | Action |
|---|---|
| `Tab` | Switch focus between tree and reader |
| `t` | Hide / show the sidebar |
| `w` | Centred / full-width text |
| `H` | Hide / show the key hints |
| `F` | Distraction-free reading (only the text; `F` or `Esc` leaves) |
| `j` / `k` | Scroll a line (reader); move down / up (tree) |
| `h` / `l` | Previous / next item (reader); collapse / expand (tree) |
| `Ctrl-d` / `Ctrl-u` | Half page down / up |
| `gg` / `G` | Top / bottom (first / last in the tree) |
| `1` – `5` | Toggle Devanagari, verse, synonyms, translation, purport |
| `Enter` | Open item / expand (tree) |
| `f` | Filter the tree |
| `z` | Collapse all in the tree |
| `/` | Full-text search (`Ctrl-n` / `Ctrl-p` or arrows from the box into the results) |
| `n` / `N` | Next / previous search result |
| `:` | Go to a reference |
| `s` | Add the current item to a collection |
| `'` | Open collections |
| `o` | Open the current item on vedabase.cc |
| `e` | Export the current item |
| `T` | Choose a colour theme |
| `?` | Help |
| `q` / `Esc` | Close overlay; quit |
| Mouse | Click a pane to focus it, a tree row to select or open it, a part label to toggle it, `↗ vedabase.cc` (reader) or the `vedabase.cc` tag (sidebar) to open the website; the wheel scrolls |

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

# Widest line of verse text, in columns; the text is centred. 0 = use the whole pane.
# reading_width = 80
```

## Being a good neighbour

vedabase.cc is a free service. The client limits itself to 100 verse requests
and 25 searches per minute (page fetches count against the verse budget),
honours `Retry-After` on a 429, and caches everything it downloads, so each
item is fetched at most once. It sends `User-Agent: vedabase-tui/<version>`.

The API has no listing for letters and lectures, so those are listed by
scraping the site's pages. That may break if the site changes; the app then
falls back to search.
