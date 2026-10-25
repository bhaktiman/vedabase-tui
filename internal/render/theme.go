// Package render turns items into styled terminal text.
package render

import (
	"fmt"
	"image/color"

	"charm.land/glamour/v2/ansi"
	"charm.land/glamour/v2/styles"
	"charm.land/lipgloss/v2"
)

// Palette is the warm, dark colour scheme shared by the whole UI.
type Palette struct{ Bg, Fg, Dim, Gold, Rose, Teal, Violet, Bright color.Color }

// Default is the palette from the design mockup.
var Default = Palette{
	Bg:     lipgloss.Color("#1b1a17"),
	Fg:     lipgloss.Color("#d8d2c4"),
	Dim:    lipgloss.Color("#6d665a"),
	Gold:   lipgloss.Color("#e8a95b"),
	Rose:   lipgloss.Color("#e07a8f"),
	Teal:   lipgloss.Color("#6cc4b4"),
	Violet: lipgloss.Color("#b48ee8"),
	Bright: lipgloss.Color("#f2ece0"),
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
