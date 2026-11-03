package ui

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vedabase-tui/internal/api"
	"vedabase-tui/internal/store"
)

type clipCall struct {
	name  string
	args  []string
	input []byte
}

// stubClip makes the app think it runs on env's platform with the given tools
// installed, and records the native calls instead of making them.
func stubClip(a *App, goos string, env map[string]string, tools ...string) *[]clipCall {
	var calls []clipCall
	a.clip = clipEnv{
		goos:     goos,
		getenv:   func(k string) string { return env[k] },
		readFile: func(string) ([]byte, error) { return nil, errors.New("none") },
		lookPath: func(n string) (string, error) {
			for _, t := range tools {
				if t == n {
					return "/bin/" + n, nil
				}
			}
			return "", errors.New("not found")
		},
		run: func(n string, args []string, in []byte) error {
			calls = append(calls, clipCall{n, args, in})
			return nil
		},
	}
	return &calls
}

func copyApp(t *testing.T) *App {
	a, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
	a.opener = func(string) error { return nil }
	return a
}

func TestCopyMenuOpensWithRows(t *testing.T) {
	a := copyApp(t)
	stubClip(a, "linux", nil)
	drive(a, key("y"))
	v := plain(a)
	for _, w := range []string{"Copy", "1  Reference + link", "2  Translation", "Everything"} {
		if !strings.Contains(v, w) {
			t.Fatalf("missing %q:\n%s", w, v)
		}
	}
	drive(a, key("esc"))
	if a.copyMenu.visible || strings.Contains(plain(a), "Reference + link") {
		t.Fatal("esc did not close")
	}
}

func TestCopyNoItemNoMenu(t *testing.T) {
	a, _ := boot(t, newFake(t), store.State{})
	a.hasItem = false
	drive(a, key("y"))
	if a.copyMenu.visible {
		t.Fatal("menu opened without an item")
	}
}

func TestCopyNumberCopiesAtOnce(t *testing.T) {
	a := copyApp(t)
	calls := stubClip(a, "linux", map[string]string{"WAYLAND_DISPLAY": "w"}, "wl-copy")
	drive(a, key("y"), key("2"))
	if a.copyMenu.visible {
		t.Fatal("menu still open")
	}
	if len(*calls) != 1 || (*calls)[0].name != "wl-copy" || string((*calls)[0].input) != strings.TrimSpace(a.item.Translation) {
		t.Fatalf("calls = %+v", *calls)
	}
	if !strings.Contains(plain(a), "Copied translation") {
		t.Fatal("no toast")
	}
}

func TestCopyReferenceHasLink(t *testing.T) {
	a := copyApp(t)
	calls := stubClip(a, "darwin", nil, "pbcopy")
	drive(a, key("y"), key("enter"))
	if len(*calls) != 1 || (*calls)[0].name != "pbcopy" {
		t.Fatalf("calls = %+v", *calls)
	}
	got := string((*calls)[0].input)
	if !strings.HasPrefix(got, "Bg 2.13 — ") || !strings.Contains(got, "https://vedabase.cc/") {
		t.Fatalf("got %q", got)
	}
}

func TestCopyJKEnter(t *testing.T) {
	a := copyApp(t)
	calls := stubClip(a, "darwin", nil, "pbcopy")
	drive(a, key("y"), key("j"), key("enter"))
	if len(*calls) != 1 || string((*calls)[0].input) != strings.TrimSpace(a.item.Translation) {
		t.Fatalf("calls = %+v", *calls)
	}
}

func TestCopyEverythingHasNoFrontMatter(t *testing.T) {
	a := copyApp(t)
	calls := stubClip(a, "darwin", nil, "pbcopy")
	drive(a, key("y"), key("4"))
	got := string((*calls)[0].input)
	if len(a.copyMenu.rows) != 4 {
		t.Skip("item has no verse row")
	}
	if strings.HasPrefix(got, "---") || strings.Contains(got, "exported:") {
		t.Fatalf("front matter: %q", got)
	}
	if !strings.HasPrefix(got, "# ") || !strings.Contains(got, "[Read on vedabase.cc](") {
		t.Fatalf("got %q", got)
	}
}

func TestCopyVerseRowHiddenWhenNone(t *testing.T) {
	a := copyApp(t)
	stubClip(a, "linux", nil)
	a.item.Devanagari, a.item.VerseText = "", ""
	drive(a, key("y"))
	for _, r := range a.copyMenu.rows {
		if r.label == "Verse" {
			t.Fatal("verse row shown")
		}
	}
}

func TestCopyWSLClipExeGetsUTF16(t *testing.T) {
	a := copyApp(t)
	calls := stubClip(a, "linux", map[string]string{"WSL_DISTRO_NAME": "Ubuntu"}, "clip.exe", "xclip")
	a.item = api.Item{Ref: "bg/2/13", Translation: "Śrīmad-Bhāgavatam ॐ"}
	drive(a, key("y"), key("2"))
	if len(*calls) != 1 || (*calls)[0].name != "clip.exe" {
		t.Fatalf("calls = %+v", *calls)
	}
	want := []byte{0xFF, 0xFE}
	for _, r := range "Śrīmad-Bhāgavatam ॐ" {
		want = append(want, byte(r), byte(r>>8)) // all in the BMP
	}
	if !bytes.Equal((*calls)[0].input, want) {
		t.Fatalf("bytes = % x", (*calls)[0].input)
	}
}

func TestCopyWSLDetectedFromProcVersion(t *testing.T) {
	a := copyApp(t)
	calls := stubClip(a, "linux", nil, "clip.exe")
	a.clip.readFile = func(string) ([]byte, error) { return []byte("Linux ... Microsoft ..."), nil }
	drive(a, key("y"), key("1"))
	if len(*calls) != 1 || (*calls)[0].name != "clip.exe" {
		t.Fatalf("calls = %+v", *calls)
	}
}

func TestCopyUTF8ForOthersAndXToolOrder(t *testing.T) {
	a := copyApp(t)
	a.item = api.Item{Ref: "bg/2/13", Translation: "Śrīmad ॐ"}
	calls := stubClip(a, "linux", map[string]string{"DISPLAY": ":0"}, "xsel", "xclip")
	drive(a, key("y"), key("2"))
	c := (*calls)[0]
	if c.name != "xclip" || strings.Join(c.args, " ") != "-selection clipboard" || string(c.input) != "Śrīmad ॐ" {
		t.Fatalf("%+v", c)
	}
	calls = stubClip(a, "linux", map[string]string{"DISPLAY": ":0"}, "xsel")
	drive(a, key("y"), key("2"))
	if c := (*calls)[0]; c.name != "xsel" || strings.Join(c.args, " ") != "--clipboard --input" {
		t.Fatalf("%+v", c)
	}
}

func TestCopyNoNativeToolStillOSC52(t *testing.T) {
	a := copyApp(t)
	calls := stubClip(a, "linux", nil)
	_, _ = a.Update(key("y"))
	_, cmd := a.Update(key("2"))
	if len(*calls) != 0 {
		t.Fatal("ran a tool")
	}
	found := false
	var walk func(m tea.Msg)
	walk = func(m tea.Msg) {
		if b, ok := m.(tea.BatchMsg); ok {
			for _, c := range b {
				if c != nil {
					walk(c())
				}
			}
			return
		}
		if m != nil && strings.Contains(strings.ToLower(strings.TrimPrefix(fmtT(m), "tea.")), "clipboard") {
			found = true
		}
	}
	walk(cmd())
	if !found {
		t.Fatal("no OSC 52 clipboard message")
	}
}

func fmtT(m tea.Msg) string { return strings.ToLower(typeName(m)) }

func TestCopyMenuClickAndOutsideClick(t *testing.T) {
	a := copyApp(t)
	calls := stubClip(a, "darwin", nil, "pbcopy")
	drive(a, key("y"))
	r := a.overlayRect(a.overlayBox())
	click := func(x, y int) {
		drive(a, tea.MouseClickMsg{Button: tea.MouseLeft, X: x, Y: y})
	}
	click(r.x+3, r.y+3) // row 2: selects
	if a.copyMenu.cursor != 1 || len(*calls) != 0 {
		t.Fatalf("cursor %d calls %v", a.copyMenu.cursor, *calls)
	}
	click(r.x+3, r.y+3) // again: copies
	if len(*calls) != 1 || a.copyMenu.visible {
		t.Fatalf("calls %v", *calls)
	}
	drive(a, key("y"))
	click(0, 0)
	if a.copyMenu.visible {
		t.Fatal("outside click did not close")
	}
}

func TestHelpMentionsCopy(t *testing.T) {
	a := copyApp(t)
	drive(a, key("?"))
	if !strings.Contains(plain(a), "Copy reference") {
		t.Fatal("help lacks y")
	}
}

func typeName(m tea.Msg) string { return fmt.Sprintf("%T", m) }
