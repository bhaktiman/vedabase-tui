package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

type helpRow struct{ key, what string }

var helpGroups = []struct {
	name string
	rows []helpRow
}{
	{"View", []helpRow{
		{"tab", "Switch focus"}, {"t", "Hide / show sidebar"}, {"w", "Centred / full width"},
		{"H", "Hide / show key hints"}, {"F", "Distraction-free reading"},
	}},
	{"Reader", []helpRow{
		{"j / k", "Scroll line"}, {"h / l", "Previous / next item"},
		{"Ctrl-d / Ctrl-u", "Half page"}, {"gg / G", "Top / bottom"},
		{"1 – 5", "Toggle parts"}, {"o", "Open on vedabase.cc"},
	}},
	{"Tree", []helpRow{
		{"j / k", "Move down / up"}, {"h / l", "Collapse / expand"},
		{"Enter", "Open item / expand"}, {"gg / G", "First / last"},
		{"f", "Filter the tree"}, {"z", "Collapse all"},
	}},
	{"Overlays", []helpRow{
		{"/", "Full-text search"}, {"n / N", "Next / previous result"},
		{":", "Go to reference"}, {"s", "Add to collection"},
		{"'", "Open collections"}, {"e", "Export item"},
		{"T", "Choose theme"}, {"?", "This help"}, {"q / Esc", "Close overlay; quit"},
	}},
}

func (a *App) helpView() string {
	gold := lipgloss.NewStyle().Foreground(a.pal.Gold).Bold(true)
	key := lipgloss.NewStyle().Foreground(a.pal.Teal)
	dim := lipgloss.NewStyle().Foreground(a.pal.Dim)
	var lines []string
	lines = append(lines, gradientBold("Keys", stops(a.pal)...))
	for _, g := range helpGroups {
		lines = append(lines, "", gold.Render(g.name))
		for _, r := range g.rows {
			lines = append(lines, "  "+key.Render(r.key)+strings.Repeat(" ", max(17-lipgloss.Width(r.key), 1))+r.what)
		}
	}
	lines = append(lines, "", dim.Render("any key closes"))
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(a.pal.Dim).
		Padding(0, 2).Render(strings.Join(lines, "\n"))
}
