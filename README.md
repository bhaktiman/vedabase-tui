# vedabase-tui

[![Latest release](https://img.shields.io/github/v/release/bhaktiman/vedabase-tui?sort=semver)](https://github.com/bhaktiman/vedabase-tui/releases/latest)
[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)](go.mod)
[![Platforms](https://img.shields.io/badge/platforms-Linux%20%7C%20macOS%20%7C%20Windows-555)](https://github.com/bhaktiman/vedabase-tui/releases/latest)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

![vedabase-tui showing Śrīmad-Bhāgavatam 1.1.6 with the contents tree on the left](https://github.com/user-attachments/assets/53adf385-9a20-4e78-a7fe-fc713652455e)

A terminal reader for Śrīla Prabhupāda's books, letters and lectures, built on
the public [vedabase.cc API](https://vedabase.cc/en/api/).

## What it does

- **Read straight through.** `h` and `l` step to the previous and next item and
  carry on across chapter and canto boundaries, so you can read a whole book
  without going back to the contents.
- **Contents tree** beside the text, with a live filter.
- **Full-text search** across everything, one book, the letters or the lectures.
- **Go to a reference** by typing it: `bg 2.13`, `sb 1.2.6`.
- **Collections:** save verses, chapters, letters and lectures into named lists.
- **Markdown export** of one item or a whole collection.
- **Themes**, a centred reading column and a distraction-free mode.
- **Offline cache and resume.** It reopens where you stopped, and anything you
  have already read works without a network.

## Install

Linux, macOS and Windows (WSL):

```sh
curl -fsSL https://github.com/bhaktiman/vedabase-tui/releases/latest/download/install.sh | sh
```

It installs to `~/.local/bin`; then run `vedabase`. When a new version is out,
the app tells you: press `U` to update.

Windows without WSL: download `vedabase_windows_amd64.zip` from the
[latest release](https://github.com/bhaktiman/vedabase-tui/releases/latest),
unzip it and run `vedabase.exe` in Windows Terminal.

## What's in it

Everything is fetched from vedabase.cc (English). The titles below are the
ones the app shows and searches on. People often type Bhagavad Gita, Srimad
Bhagavatam or Chaitanya Charitamrita; in the app these are the first three
books below, and `:` takes their short codes (`bg`, `sb`, `cc`).

**Scriptures with verses**

| Book | Code |
|---|---|
| Bhagavad-gītā As It Is | `bg` |
| Śrīmad-Bhāgavatam (by canto) | `sb` |
| Śrī Caitanya-caritāmṛta (Ādi, Madhya, Antya) | `cc` |
| Śrī Īśopaniṣad | `iso` |
| Nectar of Instruction | `noi` |

**Other books**

Beyond Birth and Death, Easy Journey to Other Planets, Elevation to Kṛṣṇa
Consciousness, Kṛṣṇa, the Supreme Personality of Godhead, Kṛṣṇa, the Reservoir
of Pleasure, Life Comes from Life, Light of the Bhāgavata, The Nectar of
Devotion, On the Way to Kṛṣṇa, The Path of Perfection, Perfection of Yoga,
Perfect Questions, Perfect Answers, Rāja-Vidyā: The King of Knowledge, The
Science of Self-Realization, Teachings of Lord Caitanya, Teachings of Queen
Kuntī and The Topmost Yoga System.

**Letters** (about 6,600). Browse them by year or by recipient.

**Lectures and conversations** (about 3,700). Browse them by year.

## Reading

Pick an item in the tree and it opens in the reader. A verse has up to five
parts, each with a number key that shows or hides it:

| Key | Part |
|---|---|
| `1` | Devanagari (Bengali in Śrī Caitanya-caritāmṛta) |
| `2` | Transliterated verse |
| `3` | Word-for-word synonyms |
| `4` | Translation |
| `5` | Purport |

The choice is remembered between sessions. Clicking a part label does the same.

`j` / `k` scroll a line, `Ctrl-d` / `Ctrl-u` a half page, `gg` / `G` jump to the
top or bottom. `h` / `l` go to the previous or next item. In a book this crosses
chapters and cantos; in letters, lectures and collections it stays inside the
current group.

## Layout

| Key | What it does |
|---|---|
| `t` | Hide or show the contents sidebar. |
| `w` | Switch between a centred reading column and the full width of the pane. The column width is the `reading_width` setting (default 80). |
| `H` | Hide or show the key hints in the bar at the bottom. |
| `F` | Distraction-free mode: only the text, no frame, tree or bars. `F` or `Esc` leaves it. |
| `Tab` | Move focus between the tree and the reader. |

With the mouse you can click a pane to focus it, click a tree row to select or
open it, and use the wheel to scroll.

## Contents tree

The tree lists your Collections first, then the books, Letters and Lectures. `j` / `k` move, `l` or `Enter` expand, `h` collapses, `Enter` on an
item opens it, and `gg` / `G` jump to the first or last row. `f` filters the
tree as you type, and `z` collapses everything. Items you have saved to a
collection are marked with a star.

Letters are grouped "By year" and "By recipient"; lectures and conversations
are grouped by year. Letter years load when you first expand them.

## Search

Press `/` and type. Press `Enter` to run it; more results load as you reach the end of the list.

`Tab` (or `Shift-Tab`, or a click) cycles the scope: **All**, **This book**
(the book you are reading), **Letters** or **Lectures**. From the box,
`Ctrl-n` / `Ctrl-p` or the arrow keys move into the results; `Enter` or a click
opens one. After you close the search, `n` and `N` open the next and previous
result, loading further pages as needed.

The client allows 25 searches a minute. If you hit that, the box tells you how
many seconds to wait.

## Go to a reference

`:` opens a one-line prompt. Type a reference and press `Enter`:

```
bg 2.13
sb 1.2.6
cc madhya 9.74
```

`Esc` cancels.

## Collections

A collection is a named list of verses, chapters, letters or lectures.

- `s` adds the current item. Choose an existing collection, or choose "+ New
  collection…" and type a name. Adding the same item twice is ignored.
- `'` opens the collections list. `Enter` on a collection shows its items;
  `Enter` on an item opens it. `Esc` goes back.
- Inside the list: `d` removes the selected item, `R` renames a collection,
  `D` deletes it (both `d` and `D` ask for `y` to confirm), and `e` exports it.
- The same collections also appear in the tree under Collections.

Collections work offline. They are saved in `collections.json` (see below); the file is written
atomically, and one that cannot be parsed is set aside, never overwritten.

## Export

`e` writes the current item to a Markdown file. In the tree, `e` on a
collection exports the whole collection to one file; `e` in the collections
list does the same. A collection export runs in the background and shows its
progress in the bottom bar. Items that cannot be fetched are listed as
unavailable instead of stopping the export.

Files go to `~/vedabase-exports/` (change it with `export_dir`). An item is
named like `bg-2-13.md`, a collection `collection-<name>.md`. Exporting again
overwrites the file, and the message says so.

Each file starts with front matter (`ref`, `book`, `book_title`, `chapter`,
`url`, the export date; letters and lectures also get `date`, `year` and
`location`; collections get `collection` and `count`). Then comes a heading,
the parts you currently have switched on (1 to 5), and a
`[Read on vedabase.cc](...)` link back to the page.

## Open on the web

`o` opens the current item on vedabase.cc in your browser. You can also click
`↗ vedabase.cc` in the reader's frame, or the `vedabase.cc` tag at the bottom of
the sidebar.

## Themes

`T` opens a theme picker: Vedabase (the default), Terminal, Dracula, Monokai,
Solarized Dark, Solarized Light, Gruvbox Dark, Nord, Tokyo Night and
Catppuccin Mocha. Moving the cursor previews a theme live; Enter keeps and
remembers it, Esc goes back. Themes set the terminal's background while the app
runs and restore it on exit; a terminal without that support keeps its own
background. `Terminal` has the same warm colours but keeps your own background.

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
| `o` | Open the current item on vedabase.cc |
| `Enter` | Open item / expand (tree) |
| `f` | Filter the tree |
| `z` | Collapse all in the tree |
| `/` | Full-text search (`Ctrl-n` / `Ctrl-p` or arrows from the box into the results) |
| `n` / `N` | Next / previous search result |
| `:` | Go to a reference |
| `s` | Add the current item to a collection |
| `'` | Open collections |
| `e` | Export the current item (or the selected collection) |
| `T` | Choose a colour theme |
| `U` | Update to the new version (shown when one is out) |
| `?` | Help |
| `q` / `Esc` | Close overlay; quit |
| Mouse | Click a pane to focus it, a tree row to select or open it, a part label to toggle it, `↗ vedabase.cc` (reader) or the `vedabase.cc` tag (sidebar) to open the website; the wheel scrolls |

While mouse mode is on, selecting text in most terminals (Windows Terminal,
iTerm2, GNOME Terminal) needs Shift+drag, or set `mouse = false`.

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

To install elsewhere or pin a version, set `INSTALL_DIR` or `VERSION` before
`sh` in the install command. To uninstall, delete the binary.

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

vedabase.cc is a free service, so the app keeps its requests modest (at most
100 pages and 25 searches a minute), waits when asked to, and caches
everything it downloads so each item is fetched only once.

## License

The code is under the [MIT License](LICENSE). The texts it displays come from
vedabase.cc and remain under their own copyright.
