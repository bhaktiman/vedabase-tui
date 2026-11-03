package ui

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"unicode/utf16"

	tea "charm.land/bubbletea/v2"

	"vedabase-tui/internal/export"
	"vedabase-tui/internal/ref"
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

func defaultClipEnv() clipEnv {
	return clipEnv{
		goos: runtime.GOOS, getenv: os.Getenv, readFile: os.ReadFile, lookPath: exec.LookPath,
		run: func(name string, args []string, input []byte) error {
			c := exec.Command(name, args...)
			c.Stdin = bytes.NewReader(input)
			return c.Run()
		},
	}
}

func (e clipEnv) wsl() bool {
	if e.getenv("WSL_DISTRO_NAME") != "" {
		return true
	}
	b, err := e.readFile("/proc/version")
	return err == nil && strings.Contains(strings.ToLower(string(b)), "microsoft")
}

// utf16LE is s as UTF-16LE with a byte order mark, which clip.exe needs to
// read anything beyond ASCII.
func utf16LE(s string) []byte {
	u := utf16.Encode([]rune(s))
	out := make([]byte, 0, 2+2*len(u))
	out = append(out, 0xFF, 0xFE)
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
	head := ref.Display(r)
	if it.URL != "" {
		head += " — " + it.URL
	}
	rows := []copyRow{{"Reference + link", "reference", head}}
	if s := strings.TrimSpace(it.Translation); s != "" {
		rows = append(rows, copyRow{"Translation", "translation", s})
	}
	var verse []string
	for _, s := range []string{it.Devanagari, it.VerseText} {
		if s = strings.TrimSpace(s); s != "" {
			verse = append(verse, s)
		}
	}
	if len(verse) > 0 {
		rows = append(rows, copyRow{"Verse", "verse", strings.Join(verse, "\n\n")})
	}
	rows = append(rows, copyRow{"Everything", "text", export.ItemText(it, a.metaFor(r), a.state.Parts)})
	a.copyMenu = copyMenu{visible: true, rows: rows}
	return nil
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
