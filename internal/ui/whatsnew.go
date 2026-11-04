package ui

import (
	"fmt"
	"regexp"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// whatsNewVersions is how many changelog entries the dialog shows: the
// running version's and the ones before it.
const whatsNewVersions = 4

// whatsNew is the dialog shown once after an update: the changelog from the
// running version back a few, scrolled with j/k, closed with Esc.
type whatsNew struct {
	visible bool
	title   string // the version in the title
	md      string // the entries shown, as Markdown
	top     int    // first body line shown
}

// changelogSince returns the entry for version and up to n-1 older ones from
// the changelog md ("## vX.Y.Z — date" headings, newest first). ok is false
// when version has no entry.
func changelogSince(md, version string, n int) (string, bool) {
	var secs []string
	for _, s := range strings.Split("\n"+md, "\n## ")[1:] {
		secs = append(secs, "## "+strings.TrimSpace(s))
	}
	for i, s := range secs {
		head := strings.Fields(strings.TrimPrefix(s, "## "))
		if len(head) > 0 && head[0] == version {
			return strings.Join(secs[i:min(i+n, len(secs))], "\n\n"), true
		}
	}
	return "", false
}

// isRelease says whether v is a released version ("v1.2.3"), not "dev".
func isRelease(v string) bool {
	return len(v) > 1 && v[0] == 'v' && v[1] >= '0' && v[1] <= '9'
}

// maybeShowWhatsNew opens the dialog when this release is new to the user:
// state.json saw another version before, or is from before versions were
// recorded but has been used. A first install shows nothing. Either way the
// version is recorded, so the dialog opens once.
func (a *App) maybeShowWhatsNew(changelog string) {
	if !isRelease(a.version) || a.state.SeenVersion == a.version {
		return
	}
	upgraded := a.state.SeenVersion != "" || a.state.LastRef != ""
	a.state.SeenVersion = a.version
	if !upgraded {
		return
	}
	if md, ok := changelogSince(changelog, a.version, whatsNewVersions); ok {
		a.whatsNew = whatsNew{visible: true, title: a.version, md: md}
	}
}

// openWhatsNew is `W`: the same dialog at any time. A build that is not in
// the changelog (dev) shows the newest entries.
func (a *App) openWhatsNew() tea.Cmd {
	v := a.version
	md, ok := changelogSince(a.changelog, v, whatsNewVersions)
	if !ok {
		v = newestVersion(a.changelog)
		md, ok = changelogSince(a.changelog, v, whatsNewVersions)
	}
	if !ok {
		return a.Toast("No changelog in this build", false)
	}
	a.whatsNew = whatsNew{visible: true, title: v, md: md}
	return nil
}

// newestVersion is the first "## vX.Y.Z" heading of the changelog.
func newestVersion(md string) string {
	for _, l := range strings.Split(md, "\n") {
		if f := strings.Fields(strings.TrimPrefix(l, "## ")); strings.HasPrefix(l, "## ") && len(f) > 0 {
			return f[0]
		}
	}
	return ""
}

var (
	mdBold = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	mdCode = regexp.MustCompile("`([^`]+)`")
)

// whatsNewLines renders the entries to lines of at most w cells: headings in
// gold, bullets with a hanging indent, **bold** and `code` styled.
func (a *App) whatsNewLines(w int) []string {
	gold := lipgloss.NewStyle().Foreground(a.pal.Gold).Bold(true)
	bold := lipgloss.NewStyle().Bold(true)
	code := lipgloss.NewStyle().Foreground(a.pal.Teal)
	inline := func(s string) string {
		s = mdCode.ReplaceAllStringFunc(s, func(m string) string { return code.Render(m[1 : len(m)-1]) })
		return mdBold.ReplaceAllStringFunc(s, func(m string) string { return bold.Render(m[2 : len(m)-2]) })
	}
	var out []string
	for _, l := range strings.Split(a.whatsNew.md, "\n") {
		l = strings.TrimRight(l, " ")
		switch {
		case strings.HasPrefix(l, "## "):
			if len(out) > 0 {
				out = append(out, "")
			}
			out = append(out, gold.Render(truncate(strings.TrimPrefix(l, "## "), w)))
		case strings.HasPrefix(l, "- "):
			for i, wl := range strings.Split(ansi.Wrap(inline(strings.TrimPrefix(l, "- ")), w-2, ""), "\n") {
				if i == 0 {
					out = append(out, "• "+wl)
				} else {
					out = append(out, "  "+wl)
				}
			}
		case strings.TrimSpace(l) != "":
			out = append(out, strings.Split(ansi.Wrap(inline(l), w, ""), "\n")...)
		}
	}
	return out
}

// whatsNewGeom is the box's inner width and the number of body lines that fit.
func (a *App) whatsNewGeom() (inner, vis int, lines []string) {
	bw := max(min(a.w-4, 80), 24)
	inner = bw - 4
	lines = a.whatsNewLines(inner)
	vis = max(min(len(lines), a.h-6), 1) // border 2, title, blank, hint
	return
}

func (a *App) whatsNewScroll(d int) {
	_, vis, lines := a.whatsNewGeom()
	a.whatsNew.top = max(min(a.whatsNew.top+d, len(lines)-vis), 0)
}

func (a *App) whatsNewKey(kp tea.KeyPressMsg) tea.Cmd {
	_, vis, _ := a.whatsNewGeom()
	switch kp.String() {
	case "esc", "q", "enter":
		a.whatsNew.visible = false
		a.persist()
	case "j", "down":
		a.whatsNewScroll(1)
	case "k", "up":
		a.whatsNewScroll(-1)
	case "ctrl+d", "pgdown", "space", " ":
		a.whatsNewScroll(max(vis-1, 1))
	case "ctrl+u", "pgup":
		a.whatsNewScroll(-max(vis-1, 1))
	case "g", "home":
		a.whatsNew.top = 0
	case "G", "end":
		a.whatsNewScroll(1 << 20)
	}
	return nil
}

func (a *App) whatsNewView() string {
	inner, vis, lines := a.whatsNewGeom()
	top := min(a.whatsNew.top, max(len(lines)-vis, 0))
	dim := lipgloss.NewStyle().Foreground(a.pal.Dim)
	body := []string{gradientBold("What's new in "+a.whatsNew.title, stops(a.pal)...), ""}
	body = append(body, lines[top:min(top+vis, len(lines))]...)
	hint := "esc close"
	if len(lines) > vis {
		hint = fmt.Sprintf("j/k scroll · esc close · %d–%d of %d", top+1, min(top+vis, len(lines)), len(lines))
	}
	body = append(body, dim.Render(truncate(hint, inner)))
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(a.pal.Dim).
		Padding(0, 1).Width(inner + 4).Render(strings.Join(body, "\n"))
}
