// Package render turns items into styled terminal text.
package render

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/glamour/v2/ansi"
	"charm.land/glamour/v2/styles"
	"charm.land/lipgloss/v2"
)

// Palette is a colour scheme shared by the whole UI. Gold is the main accent
// (headings, focus, Sanskrit), Rose errors, Teal labels and "on" toggles,
// Violet links, Bright the strongest text.
type Palette struct {
	Name                                          string
	Bg, Fg, Dim, Gold, Rose, Teal, Violet, Bright color.Color
	Light                                         bool // a light background: Glamour's light base
	Paint                                         bool // the app sets the terminal's background to Bg
}

// Themes are the built-in palettes, in picker order.
var Themes = []Palette{
	{Name: "Vedabase", Bg: lipgloss.Color("#1b1a17"), Fg: lipgloss.Color("#d8d2c4"), Dim: lipgloss.Color("#6d665a"),
		Gold: lipgloss.Color("#e8a95b"), Rose: lipgloss.Color("#e07a8f"), Teal: lipgloss.Color("#6cc4b4"), Violet: lipgloss.Color("#b48ee8"), Bright: lipgloss.Color("#f2ece0"), Paint: true},
	{Name: "Terminal", Bg: lipgloss.Color("#1b1a17"), Fg: lipgloss.Color("#d8d2c4"), Dim: lipgloss.Color("#6d665a"),
		Gold: lipgloss.Color("#e8a95b"), Rose: lipgloss.Color("#e07a8f"), Teal: lipgloss.Color("#6cc4b4"), Violet: lipgloss.Color("#b48ee8"), Bright: lipgloss.Color("#f2ece0")},
	{Name: "Dracula", Bg: lipgloss.Color("#282a36"), Fg: lipgloss.Color("#f8f8f2"), Dim: lipgloss.Color("#6272a4"),
		Gold: lipgloss.Color("#ffb86c"), Rose: lipgloss.Color("#ff79c6"), Teal: lipgloss.Color("#8be9fd"), Violet: lipgloss.Color("#bd93f9"), Bright: lipgloss.Color("#ffffff"), Paint: true},
	{Name: "Monokai", Bg: lipgloss.Color("#272822"), Fg: lipgloss.Color("#f8f8f2"), Dim: lipgloss.Color("#75715e"),
		Gold: lipgloss.Color("#e6db74"), Rose: lipgloss.Color("#f92672"), Teal: lipgloss.Color("#66d9ef"), Violet: lipgloss.Color("#ae81ff"), Bright: lipgloss.Color("#ffffff"), Paint: true},
	{Name: "Solarized Dark", Bg: lipgloss.Color("#002b36"), Fg: lipgloss.Color("#839496"), Dim: lipgloss.Color("#586e75"),
		Gold: lipgloss.Color("#b58900"), Rose: lipgloss.Color("#d33682"), Teal: lipgloss.Color("#2aa198"), Violet: lipgloss.Color("#6c71c4"), Bright: lipgloss.Color("#eee8d5"), Paint: true},
	{Name: "Solarized Light", Bg: lipgloss.Color("#fdf6e3"), Fg: lipgloss.Color("#657b83"), Dim: lipgloss.Color("#93a1a1"),
		Gold: lipgloss.Color("#b58900"), Rose: lipgloss.Color("#d33682"), Teal: lipgloss.Color("#2aa198"), Violet: lipgloss.Color("#6c71c4"), Bright: lipgloss.Color("#073642"), Light: true, Paint: true},
	{Name: "Gruvbox Dark", Bg: lipgloss.Color("#282828"), Fg: lipgloss.Color("#ebdbb2"), Dim: lipgloss.Color("#928374"),
		Gold: lipgloss.Color("#fabd2f"), Rose: lipgloss.Color("#fb4934"), Teal: lipgloss.Color("#8ec07c"), Violet: lipgloss.Color("#d3869b"), Bright: lipgloss.Color("#fbf1c7"), Paint: true},
	{Name: "Nord", Bg: lipgloss.Color("#2e3440"), Fg: lipgloss.Color("#d8dee9"), Dim: lipgloss.Color("#616e88"),
		Gold: lipgloss.Color("#ebcb8b"), Rose: lipgloss.Color("#bf616a"), Teal: lipgloss.Color("#88c0d0"), Violet: lipgloss.Color("#b48ead"), Bright: lipgloss.Color("#eceff4"), Paint: true},
	{Name: "Tokyo Night", Bg: lipgloss.Color("#1a1b26"), Fg: lipgloss.Color("#c0caf5"), Dim: lipgloss.Color("#565f89"),
		Gold: lipgloss.Color("#e0af68"), Rose: lipgloss.Color("#f7768e"), Teal: lipgloss.Color("#7dcfff"), Violet: lipgloss.Color("#bb9af7"), Bright: lipgloss.Color("#ffffff"), Paint: true},
	{Name: "Catppuccin Mocha", Bg: lipgloss.Color("#1e1e2e"), Fg: lipgloss.Color("#cdd6f4"), Dim: lipgloss.Color("#6c7086"),
		Gold: lipgloss.Color("#f9e2af"), Rose: lipgloss.Color("#f38ba8"), Teal: lipgloss.Color("#94e2d5"), Violet: lipgloss.Color("#cba6f7"), Bright: lipgloss.Color("#ffffff"), Paint: true},
}

// Default is the palette from the design mockup.
var Default = Themes[0]

// ThemeByName finds a theme by name, ignoring case.
func ThemeByName(name string) (Palette, bool) {
	for _, p := range Themes {
		if strings.EqualFold(p.Name, name) {
			return p, true
		}
	}
	return Palette{}, false
}

func hex(c color.Color) *string {
	r, g, b, _ := c.RGBA()
	s := fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, b>>8)
	return &s
}

func ptr[T any](v T) *T { return &v }

// GlamourStyle is glamour's dark style recoloured from p: Sanskrit emphasis in
// gold, translations (strong) bright and bold, verse lines (block quotes) gold
// italic without a bar, and teal section labels.
func GlamourStyle(p Palette) ansi.StyleConfig {
	s := styles.DarkStyleConfig
	if p.Light {
		s = styles.LightStyleConfig
	}
	s.Document.Color = hex(p.Fg)
	s.Document.Margin = ptr(uint(0))
	s.Emph = ansi.StylePrimitive{Italic: ptr(true), Color: hex(p.Gold)}
	s.Strong = ansi.StylePrimitive{Bold: ptr(true), Color: hex(p.Bright)}
	s.BlockQuote = ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{Italic: ptr(true), Color: hex(p.Gold)},
		Indent:         ptr(uint(2)),
		IndentToken:    ptr(""),
	}
	s.Heading = ansi.StyleBlock{StylePrimitive: ansi.StylePrimitive{
		BlockSuffix: "\n", Color: hex(p.Gold), Bold: ptr(true)}}
	s.H1 = s.Heading
	s.H2 = s.Heading
	s.H3 = s.Heading
	s.H4 = ansi.StyleBlock{StylePrimitive: ansi.StylePrimitive{
		Color: hex(p.Teal), Bold: ptr(true), Upper: ptr(true)}}
	s.Link = ansi.StylePrimitive{Color: hex(p.Violet), Underline: ptr(true)}
	s.LinkText = ansi.StylePrimitive{Color: hex(p.Violet), Bold: ptr(true)}
	s.HorizontalRule = ansi.StylePrimitive{Color: hex(p.Dim), Format: "\n--------\n"}
	return s
}
