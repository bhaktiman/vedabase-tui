# What's new

<!-- Newest first. Each version is a "## vX.Y.Z — date" heading followed by
bullets. The app shows the running version's entry and a few before it once
after an update; the release notes are copied from here. -->

## v0.3.3 — 2026-11-03

- **What's new.** After an update, this list opens once so you can see what changed. `Esc` closes it.
- **Copy what you tick.** `y` opens a tick box for each part: Devanagari, Verse, Translation, Purport and Link. All are ticked at first, and your choice is remembered. Press `y` or `Enter` again to copy. A verse copy starts with its reference and labels each part.
- **Pasting on Windows (WSL) works.** Copied text no longer starts with an invisible character, which broke pasted links in the browser.
- **One key for collections.** `'` is gone: `c` jumps to a collection or anything in one, and the sidebar renames, deletes and reorders them.

## v0.3.2 — 2026-11-03

- **Notes.** `a` writes a note on any verse, letter or lecture. It shows at the top of the page as "Your note", a ✎ marks noted items in your collections, and notes are included in exports.
- **Find on the page.** `Ctrl-f` highlights every match as you type, ignoring case and diacritics (`krsna` finds Kṛṣṇa); `n` / `N` step through them.
- **Copy** a reference, translation or verse with `y`.
- **Manage collections.** `c` jumps to any collection or item in one. In the sidebar, `R` renames and `D` deletes a collection, `d` removes an item at once, and `J` / `K` reorder items.
- **Choose where your data lives.** The `data_dir` setting in `config.toml` keeps collections and notes in a folder of your choice, for example a synced folder shared between computers.

## v0.3.1 — 2026-10-27

- **Items refresh.** A downloaded verse is trusted for 30 days; after that it is shown at once and re-checked against vedabase.cc in the background.
- **`R` re-fetches** the verse you're reading and tells you whether it changed.
- **The update offer stays put** in the bottom bar until you install it.

## v0.2.1 — 2026-10-26

- **Languages fixed after upgrading.** Books show in Español, Português, Hindī and Русский again instead of "not available".

## v0.2.0 — 2026-10-26

- **Clickable references.** Quoted verses such as "(Bg. 4.7)" are underlined: click one, or press `f` and type its label. `Backspace` or `[` goes back, `]` goes forward.
- **Five languages.** Press `L` to read the books in Español, Português, Hindī or Русский.
- **Reading straight through.** `h` / `l` follow the site's own order across chapters and cantos; letters and lectures go in date order.

## v0.1.3 — 2026-10-25

- **Resize the sidebar** by dragging its edge or with `<` and `>`; the width is remembered.
- **Help fits on screen** in two columns on wide terminals.

## v0.1.2 — 2026-10-25

- **`U` checks for updates** at any time.
- **Popups open over what you are reading** instead of blanking the screen.
- **Recently used collections come first** when you press `s`.
