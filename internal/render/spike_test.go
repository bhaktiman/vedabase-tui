package render

import (
	"encoding/json"
	"os"
	"testing"
	"time"
	"unicode/utf8"

	"charm.land/bubbles/v2/viewport"
	"charm.land/glamour/v2"
)

func loadPurport(tb testing.TB) string {
	tb.Helper()
	b, err := os.ReadFile("testdata/660219bg-new-york.json")
	if err != nil {
		tb.Fatal(err)
	}
	var v struct {
		Purport string `json:"purport"`
	}
	if err := json.Unmarshal(b, &v); err != nil {
		tb.Fatal(err)
	}
	return v.Purport
}

func renderAt(tb testing.TB, md string, w int) string {
	tb.Helper()
	r, err := glamour.NewTermRenderer(glamour.WithStandardStyle("dark"), glamour.WithWordWrap(w))
	if err != nil {
		tb.Fatal(err)
	}
	out, err := r.Render(md)
	if err != nil {
		tb.Fatal(err)
	}
	return out
}

func BenchmarkGlamourLong(b *testing.B) {
	md := loadPurport(b)
	for _, w := range []int{80, 200} {
		b.Run(map[int]string{80: "w80", 200: "w200"}[w], func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				renderAt(b, md, w)
			}
		})
	}
}

func TestViewportLong(t *testing.T) {
	md := loadPurport(t)
	if n := utf8.RuneCountInString(md); n != 114801 {
		t.Fatalf("purport length = %d runes, want 114801", n)
	}
	start := time.Now()
	out := renderAt(t, md, 80)
	t.Logf("render w80: %v (%d bytes)", time.Since(start), len(out))

	vp := viewport.New(viewport.WithWidth(100), viewport.WithHeight(40))
	start = time.Now()
	vp.SetContent(out)
	t.Logf("SetContent: %v", time.Since(start))

	start = time.Now()
	for i := 0; i < 500; i++ {
		vp.ScrollDown(1)
		_ = vp.View()
	}
	t.Logf("500x ScrollDown+View: %v", time.Since(start))
}
