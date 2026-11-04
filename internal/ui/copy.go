package ui

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"unicode/utf16"

	tea "charm.land/bubbletea/v2"

	"vedabase-tui/internal/ref"
	"vedabase-tui/internal/render"
)

// copyRow is one choice of the `y` menu: what it is called, what is copied,
// and the word for the toast.
type copyRow struct{ label, noun, text string }

type copyMenu struct {
	visible bool
	rows    []copyRow
	cursor  int
}

// clipEnv is everything the native clipboard call depends on, so tests can
// stub it and never start a process.
type clipEnv struct {
	goos     string
	getenv   func(string) string
	readFile func(string) ([]byte, error)
	lookPath func(string) (string, error)
	run      func(name string, args []string, input []byte) error
}

// defaultClipEnv is the real platform. Under go test it cannot run anything,
// so no test, whichever constructor it uses, starts a clipboard tool.
func defaultClipEnv() clipEnv {
	e := clipEnv{
		goos: runtime.GOOS, getenv: os.Getenv, readFile: os.ReadFile, lookPath: exec.LookPath,
		run: func(name string, args []string, input []byte) error {
			c := exec.Command(name, args...)
			c.Stdin = bytes.NewReader(input)
			return c.Run()
		},
	}
	if testing.Testing() {
		e.run = nil
	}
	return e
}

func (e clipEnv) wsl() bool {
	if e.getenv("WSL_DISTRO_NAME") != "" {
		return true
	}
	b, err := e.readFile("/proc/version")
	return err == nil && strings.Contains(strings.ToLower(string(b)), "microsoft")
}

// utf16LE is s as UTF-16LE, which clip.exe needs to read anything beyond
// ASCII. No byte order mark: clip.exe detects UTF-16 without one and keeps a
// mark as an invisible first character, which breaks a pasted URL.
func utf16LE(s string) []byte {
	u := utf16.Encode([]rune(s))
	out := make([]byte, 0, 2*len(u))
	for _, c := range u {
		out = append(out, byte(c), byte(c>>8))
	}
	return out
}

// nativeCopy picks the platform's clipboard tool for s: its name, arguments
// and input. ok is false when none applies or none is installed.
func (e clipEnv) nativeCopy(s string) (name string, args []string, input []byte, ok bool) {
	has := func(n string) bool { _, err := e.lookPath(n); return err == nil }
	switch {
	case e.goos == "darwin":
		if has("pbcopy") {
			return "pbcopy", nil, []byte(s), true
		}
	case e.wsl():
		if has("clip.exe") {
			return "clip.exe", nil, utf16LE(s), true
		}
	}
	if e.goos == "darwin" {
		return
	}
	if e.getenv("WAYLAND_DISPLAY") != "" && has("wl-copy") {
		return "wl-copy", nil, []byte(s), true
	}
	if e.getenv("DISPLAY") != "" {
		if has("xclip") {
			return "xclip", []string{"-selection", "clipboard"}, []byte(s), true
		}
		if has("xsel") {
			return "xsel", []string{"--clipboard", "--input"}, []byte(s), true
		}
	}
	return
}

// copyText puts s on the clipboard: OSC 52 through the terminal, and the
// native tool when there is one. The tool runs in a Cmd, off the UI goroutine;
// its failure is not reported, since OSC 52 may have worked.
func (a *App) copyText(s string) tea.Cmd {
	cmds := []tea.Cmd{tea.SetClipboard(s)}
	if a.clip.run != nil {
		if name, args, in, ok := a.clip.nativeCopy(s); ok {
			run := a.clip.run
			cmds = append(cmds, func() tea.Msg { _ = run(name, args, in); return nil })
		}
	}
	return tea.Batch(cmds...)
}

// openCopyMenu opens the `y` menu for the item shown.
func (a *App) openCopyMenu() tea.Cmd {
	if !a.hasItem {
		return a.Toast("Nothing to copy", false)
	}
	it := a.item
	r := a.ref
	if r == "" {
		r = it.Ref
	}
	link := it.URL
	if link == "" {
		link = ref.Display(r)
	}
	rows := []copyRow{{"Link", "link", link}}
	// The item's own text as plain text: no [[ref|label]] markup, no ** bold.
	part := func(s string) string { return strings.TrimSpace(render.PlainText(s)) }
	join := func(parts ...string) string {
		var out []string
		for _, p := range parts {
			if p != "" {
				out = append(out, p)
			}
		}
		return strings.Join(out, "\n\n")
	}
	deva, verse, trans, purport := part(unquote(it.Devanagari)), part(unquote(it.VerseText)), part(it.Translation), part(it.Purport)
	if verse == "" {
		// Letters, lectures and prose chapters: one row with the whole text.
		if t := join(trans, purport); t != "" {
			rows = append(rows, copyRow{"Text", "text", t})
		}
	} else {
		// A verse copy starts with its reference; when it has more than one
		// part, each is labelled as on the page.
		type sec struct{ label, text string }
		sDeva, sVerse, sTrans, sPurp := sec{"Devanagari", deva}, sec{"Verse", verse}, sec{"Translation", trans}, sec{"Purport", purport}
		quote := func(secs ...sec) string {
			var have []sec
			for _, x := range secs {
				if x.text != "" {
					have = append(have, x)
				}
			}
			out := []string{ref.Display(r)}
			for _, x := range have {
				if len(have) > 1 {
					out = append(out, x.label+"\n"+x.text)
				} else {
					out = append(out, x.text)
				}
			}
			return strings.Join(out, "\n\n")
		}
		// Fixed numbers for every verse: 3 is the same as 2 when there is no purport.
		rows = append(rows,
			copyRow{"Verse + translation", "verse and translation", quote(sVerse, sTrans)},
			copyRow{"Verse + translation + purport", "verse, translation and purport", quote(sVerse, sTrans, sPurp)})
		if deva != "" {
			rows = append(rows,
				copyRow{"Devanagari + verse + translation + purport", "Devanagari, verse, translation and purport", quote(sDeva, sVerse, sTrans, sPurp)},
				copyRow{"Devanagari only", "Devanagari", quote(sDeva)})
		}
	}
	a.copyMenu = copyMenu{visible: true, rows: rows}
	return nil
}

// unquote drops the "> " the site puts before each line of a verse.
func unquote(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		l = strings.TrimPrefix(l, ">")
		lines[i] = strings.TrimPrefix(l, " ")
	}
	return strings.Join(lines, "\n")
}

func (a *App) copyChoose(i int) tea.Cmd {
	if i < 0 || i >= len(a.copyMenu.rows) {
		return nil
	}
	row := a.copyMenu.rows[i]
	a.copyMenu.visible = false
	return tea.Batch(a.copyText(row.text), a.Toast("Copied "+row.noun, false))
}

func (a *App) copyKey(kp tea.KeyPressMsg) tea.Cmd {
	m := &a.copyMenu
	switch k := kp.String(); k {
	case "esc", "q":
		m.visible = false
	case "j", "down":
		m.cursor = min(m.cursor+1, len(m.rows)-1)
	case "k", "up":
		m.cursor = max(m.cursor-1, 0)
	case "enter":
		return a.copyChoose(m.cursor)
	default:
		if len(k) == 1 && k[0] >= '1' && k[0] <= '9' {
			return a.copyChoose(int(k[0] - '1'))
		}
	}
	return nil
}

// copyClick selects the row under the click, and chooses it when it already
// was selected.
func (a *App) copyClick(x, y int) tea.Cmd {
	i := y - 2
	vis, top := listView(a.h, len(a.copyMenu.rows), a.copyMenu.cursor)
	if y < 2 || y-2 >= vis || top+i >= len(a.copyMenu.rows) {
		return nil
	}
	i += top
	if i == a.copyMenu.cursor {
		return a.copyChoose(i)
	}
	a.copyMenu.cursor = i
	return nil
}

func (a *App) copyView() string {
	rows := make([]string, len(a.copyMenu.rows))
	for i, r := range a.copyMenu.rows {
		rows[i] = fmt.Sprintf("%d  %s", i+1, r.label)
	}
	return listBox(a.pal, a.w, a.h, "Copy", rows, a.copyMenu.cursor, "", "1-"+fmt.Sprint(len(rows))+" or enter copy · j/k move · esc close")
}
