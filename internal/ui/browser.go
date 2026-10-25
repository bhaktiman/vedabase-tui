package ui

import (
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const (
	webHome     = "https://vedabase.cc/"
	noWebPage   = "No web page for this item"
	webBtnLabel = "↗ vedabase.cc (o)"
	siteTag     = "vedabase.cc"
	releasesURL = "https://github.com/bhaktiman/vedabase-tui/releases/latest"
	installCmd  = "curl -fsSL " + releasesURL + "/download/install.sh | sh"
)

// openedMsg is the result of starting the browser.
type openedMsg struct{ err error }

// webURLOK reports whether u is a page of the site we pull from. The address
// is remote data, so nothing else is ever handed to the system opener.
func webURLOK(u string) bool {
	p, err := url.Parse(u)
	if err != nil || p.Scheme != "https" {
		return false
	}
	return p.Host == "vedabase.cc" || p.Host == "www.vedabase.cc"
}

// browserCommand is the program and arguments that open u on goos. wsl says we
// are inside WSL, where the Windows browser is reached through explorer.exe.
func browserCommand(goos string, wsl bool, u string) (string, []string) {
	switch {
	case wsl:
		return "explorer.exe", []string{u}
	case goos == "darwin":
		return "open", []string{u}
	case goos == "windows":
		return "rundll32", []string{"url.dll,FileProtocolHandler", u}
	}
	return "xdg-open", []string{u}
}

func inWSL() bool {
	if os.Getenv("WSL_DISTRO_NAME") != "" {
		return true
	}
	b, err := os.ReadFile("/proc/sys/kernel/osrelease")
	return err == nil && strings.Contains(strings.ToLower(string(b)), "microsoft")
}

// openURL starts the system browser on u without waiting for it. Output is
// discarded so nothing scribbles over the screen. explorer.exe exits 1 even on
// success, so only a failure to start counts.
func openURL(u string) error {
	prog, args := browserCommand(runtime.GOOS, inWSL(), u)
	c := exec.Command(prog, args...)
	if err := c.Start(); err != nil {
		return err
	}
	go c.Wait() // reap it
	return nil
}

// openWeb opens u in the browser, or toasts why it can't.
func (a *App) openWeb(u string) tea.Cmd {
	if !webURLOK(u) {
		return a.Toast(noWebPage, false)
	}
	op := a.opener
	if op == nil {
		op = openURL
	}
	return func() tea.Msg { return openedMsg{op(u)} }
}

func (a *App) onOpened(m openedMsg) tea.Cmd {
	if m.err != nil {
		return a.Toast("Couldn't open the browser", true)
	}
	return a.Toast("Opened on vedabase.cc", false)
}

// openItemWeb opens the reader's open item on the website.
func (a *App) openItemWeb() tea.Cmd {
	if !a.hasItem {
		return a.Toast(noWebPage, false)
	}
	return a.openWeb(a.item.URL)
}

// webButtonOn reports whether the reader's top border carries the button.
func (a *App) webButtonOn() bool {
	if !a.framed() || !a.hasItem || !webURLOK(a.item.URL) {
		return false
	}
	_, ok := titleRoom(a.readerOuterW(), lipgloss.Width(a.readerTitle()))
	return ok
}

// titleRoom is how many cells the title may use when the button is drawn, and
// whether the pane is wide enough to draw it at all.
func titleRoom(ow, titleW int) (int, bool) {
	room := ow - 3 - 1 - 1 - btnSpan
	if room < min(titleW, minTitle) {
		return 0, false
	}
	return min(room, titleW), true
}

const (
	minTitle = 12
)

// btnSpan is the button with the space before it and " ─╮" after.
var btnSpan = lipgloss.Width(webBtnLabel) + 1 + 3

// webButtonRect is where the button sits on the screen.
func (a *App) webButtonRect() rect {
	_, r := a.paneRects()
	w := lipgloss.Width(webBtnLabel)
	return rect{r.x + r.w - 3 - w, r.y, w, 1}
}

// siteTagRect is where the sidebar's bottom-edge tag sits on the screen.
// Only the site name is clickable, not the version after it.
func (a *App) siteTagRect() (rect, bool) {
	t, _ := a.paneRects()
	if !a.framed() || !a.treeVisible() || t.w < len(siteTag)+4 {
		return rect{}, false
	}
	iw := t.w - 2
	left := (iw - (a.siteLabel(iw).width() + 2)) / 2
	return rect{t.x + 1 + left, t.y + t.h - 1, len(siteTag) + 2, 1}, true
}

// tagLabel is what follows "vedabase.cc" in the sidebar tag: dim is the
// version, acc the update offer.
type tagLabel struct{ dim, acc string }

func (l tagLabel) width() int {
	return lipgloss.Width(siteTag) + lipgloss.Width(l.dim) + lipgloss.Width(l.acc)
}

// siteLabel is the longest tag that fits an inner width of iw.
func (a *App) siteLabel(iw int) tagLabel {
	var opts []tagLabel
	switch {
	case a.avail != "":
		opts = []tagLabel{{" · " + a.avail, " available · U"}, {" · " + a.avail, " · U"}, {"", " · U"}}
	case a.version != "":
		opts = []tagLabel{{" · " + a.version, ""}}
	}
	for _, o := range opts {
		if o.width()+4 <= iw {
			return o
		}
	}
	return tagLabel{}
}

// siteTagView is the styled sidebar tag for an inner width of iw.
func (a *App) siteTagView(iw int) string {
	l := a.siteLabel(iw)
	return gradientText(siteTag, stops(a.pal)...) +
		lipgloss.NewStyle().Foreground(a.pal.Dim).Render(l.dim) +
		lipgloss.NewStyle().Foreground(a.pal.Accent).Render(l.acc)
}

// sideShowsUpdate reports whether the sidebar tag is on screen with the
// update offer in it.
func (a *App) sideShowsUpdate() bool {
	t, _ := a.paneRects()
	return a.framed() && a.treeVisible() && !a.state.Zen && a.siteLabel(t.w-2).acc != ""
}
