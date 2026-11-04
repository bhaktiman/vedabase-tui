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
	"charm.land/lipgloss/v2"

	"vedabase-tui/internal/ref"
	"vedabase-tui/internal/render"
)

// copyRow is one part the `y` menu can copy: its key in State.CopyOff, its
// name, its text, and whether it is ticked.
type copyRow struct {
	key, label, text string
	on               bool
}

type copyMenu struct {
	visible bool
	ref     string // the reference heading a verse copy starts with
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

// openCopyMenu opens the `y` menu for the item shown: a tick box per part the
// item has, all ticked unless the user turned one off before (remembered).
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
	// The item's own text as plain text: no [[ref|label]] markup, no ** bold.
	part := func(s string) string { return strings.TrimSpace(render.PlainText(s)) }
	off := map[string]bool{}
	for _, k := range strings.Split(a.state.CopyOff, ",") {
		off[k] = true
	}
	var rows []copyRow
	add := func(key, label, text string) {
		if text != "" {
			rows = append(rows, copyRow{key: key, label: label, text: text, on: !off[key]})
		}
	}
	verse := part(unquote(it.VerseText))
	m := copyMenu{visible: true}
	if verse == "" {
		// Letters, lectures and prose chapters: the whole text, unlabelled.
		var t []string
		for _, x := range []string{part(it.Translation), part(it.Purport)} {
			if x != "" {
				t = append(t, x)
			}
		}
		add("text", "Whole text", strings.Join(t, "\n\n"))
	} else {
		m.ref = ref.Display(r)
		add("devanagari", "Devanagari", part(unquote(it.Devanagari)))
		add("verse", "Verse", verse)
		add("translation", "Translation", part(it.Translation))
		add("purport", "Purport", part(it.Purport))
	}
	add("link", "Link", link)
	m.rows = rows
	a.copyMenu = m
	return nil
}

// copyBody is what the ticked parts copy. A verse starts with its reference
// and labels each part when there is more than one; the link comes last. The
// link alone is just the URL.
func (m copyMenu) copyBody() (text string, names []string) {
	var on []copyRow
	for _, r := range m.rows {
		if r.on {
			on = append(on, r)
			names = append(names, strings.ToLower(r.label))
		}
	}
	var parts []string
	var body []copyRow
	for _, r := range on {
		if r.key != "link" {
			body = append(body, r)
		}
	}
	if len(body) > 0 && m.ref != "" {
		parts = append(parts, m.ref)
	}
	for _, r := range on {
		if len(body) > 1 && r.key != "link" {
			parts = append(parts, r.label+"\n"+r.text)
		} else {
			parts = append(parts, r.text)
		}
	}
	return strings.Join(parts, "\n\n"), names
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

func (a *App) copyToggle(i int) {
	m := &a.copyMenu
	if i < 0 || i >= len(m.rows) {
		return
	}
	m.cursor = i
	m.rows[i].on = !m.rows[i].on
	// Remember: CopyOff lists the parts turned off, so the default is all on.
	off := map[string]bool{}
	for _, k := range strings.Split(a.state.CopyOff, ",") {
		if k != "" {
			off[k] = true
		}
	}
	off[m.rows[i].key] = !m.rows[i].on
	var keys []string
	for _, k := range []string{"devanagari", "verse", "translation", "purport", "text", "link"} {
		if off[k] {
			keys = append(keys, k)
		}
	}
	a.state.CopyOff = strings.Join(keys, ",")
	a.persist()
}

func (a *App) copyDo() tea.Cmd {
	text, names := a.copyMenu.copyBody()
	if text == "" {
		return a.Toast("Nothing ticked — space ticks a part", false)
	}
	a.copyMenu.visible = false
	noun := strings.Join(names, ", ")
	if n := len(names); n > 1 {
		noun = strings.Join(names[:n-1], ", ") + " and " + names[n-1]
	}
	noun = strings.ReplaceAll(noun, "devanagari", "Devanagari")
	return tea.Batch(a.copyText(text), a.Toast("Copied "+noun, false))
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
	case "space", " ", "x":
		a.copyToggle(m.cursor)
	case "enter", "y":
		return a.copyDo()
	default:
		if len(k) == 1 && k[0] >= '1' && k[0] <= '9' {
			a.copyToggle(int(k[0] - '1'))
		}
	}
	return nil
}

// copyClick ticks or unticks the row under the click. Rows start on line 3
// of the box: border, title, the "press y" line.
func (a *App) copyClick(x, y int) tea.Cmd {
	i := y - 3
	vis, top := listView(a.h, len(a.copyMenu.rows), a.copyMenu.cursor)
	if y < 3 || i >= vis || top+i >= len(a.copyMenu.rows) {
		return nil
	}
	a.copyToggle(top + i)
	return nil
}

func (a *App) copyView() string {
	rows := make([]string, len(a.copyMenu.rows))
	for i, r := range a.copyMenu.rows {
		box := "[ ]"
		if r.on {
			box = lipgloss.NewStyle().Foreground(a.pal.Accent).Render("[x]")
		}
		rows[i] = fmt.Sprintf("%d %s %s", i+1, box, r.label)
	}
	// How to copy goes first, under the title: it is the one key that matters.
	top := lipgloss.NewStyle().Foreground(a.pal.Accent).Bold(true).Render("Press y again or Enter to copy")
	return listBox(a.pal, a.w, a.h, "Copy", rows, a.copyMenu.cursor, top,
		"space or 1-"+fmt.Sprint(len(rows))+" tick · esc close")
}
