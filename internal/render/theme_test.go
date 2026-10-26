package render

import (
	"testing"

	"vedabase-tui/internal/api"
)

func TestThemesComplete(t *testing.T) {
	want := []string{"Vedabase", "Terminal", "Dracula", "Monokai", "Solarized Dark",
		"Solarized Light", "Gruvbox Dark", "Nord", "Tokyo Night", "Catppuccin Mocha"}
	if len(Themes) != len(want) {
		t.Fatalf("%d themes, want %d", len(Themes), len(want))
	}
	for i, p := range Themes {
		if p.Name != want[i] {
			t.Errorf("theme %d = %q, want %q", i, p.Name, want[i])
		}
		for n, c := range map[string]any{"Bg": p.Bg, "Fg": p.Fg, "Dim": p.Dim, "Gold": p.Gold,
			"Rose": p.Rose, "Teal": p.Teal, "Violet": p.Violet, "Bright": p.Bright} {
			if c == nil {
				t.Errorf("%s: %s is nil", p.Name, n)
			}
		}
		if p.Light != (p.Name == "Solarized Light") || p.Paint != (p.Name != "Terminal") {
			t.Errorf("%s: Light=%v Paint=%v", p.Name, p.Light, p.Paint)
		}
	}
	if Themes[0] != Default {
		t.Fatal("Themes[0] is not Default")
	}
	if p, ok := ThemeByName("dracula"); !ok || p.Name != "Dracula" {
		t.Fatalf("ThemeByName(dracula) = %v, %v", p.Name, ok)
	}
	if _, ok := ThemeByName("nope"); ok {
		t.Fatal("unknown theme found")
	}
}

func TestGlamourStyleLight(t *testing.T) {
	p, _ := ThemeByName("Solarized Light")
	s := GlamourStyle(p)
	if s.Document.Color == nil || *s.Document.Color != "#657b83" {
		t.Fatalf("document colour = %v", s.Document.Color)
	}
	// The light base differs from the dark one in the parts the palette leaves alone.
	d := GlamourStyle(Default)
	if s.CodeBlock.Chroma != nil && d.CodeBlock.Chroma != nil && *s.CodeBlock.Chroma == *d.CodeBlock.Chroma {
		t.Fatal("light style still uses the dark base")
	}
}

func TestRendererSetPaletteDropsCache(t *testing.T) {
	r := NewRenderer(Default)
	a, _, _ := r.Render("x", "en", "## hi", nil, 40, api.AllParts())
	d, _ := ThemeByName("Dracula")
	r.SetPalette(d)
	b, _, _ := r.Render("x", "en", "## hi", nil, 40, api.AllParts())
	if a == b || r.renders != 2 {
		t.Fatalf("palette swap did not re-render: renders=%d", r.renders)
	}
}

func TestThemesHaveAccentAndGrad(t *testing.T) {
	for _, p := range Themes {
		if p.Accent == nil {
			t.Errorf("%s: no Accent", p.Name)
		}
		for i, c := range p.Grad {
			if c == nil {
				t.Errorf("%s: Grad[%d] is nil", p.Name, i)
			}
		}
	}
	want := map[string]struct {
		accent string
		grad   [3]string
		gold   string
	}{
		"Dracula": {"#bd93f9", [3]string{"#ff79c6", "#bd93f9", "#8be9fd"}, "#ffb86c"},
		"Monokai": {"#f92672", [3]string{"#f92672", "#fd971f", "#e6db74"}, "#fd971f"},
	}
	for _, p := range Themes {
		w, ok := want[p.Name]
		if !ok || p.Accent == nil {
			continue
		}
		if got := *hex(p.Accent); got != w.accent {
			t.Errorf("%s accent %s, want %s", p.Name, got, w.accent)
		}
		if got := *hex(p.Gold); got != w.gold {
			t.Errorf("%s gold %s, want %s", p.Name, got, w.gold)
		}
		for i := range w.grad {
			if p.Grad[i] != nil && *hex(p.Grad[i]) != w.grad[i] {
				t.Errorf("%s grad[%d] %s, want %s", p.Name, i, *hex(p.Grad[i]), w.grad[i])
			}
		}
	}
}
