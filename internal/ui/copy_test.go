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
	a.item = verseItem()
	drive(a, key("y"))
	v := plain(a)
	if !strings.Contains(v, "Copy") || !strings.Contains(v, "Press y or Enter again to copy") ||
		strings.Index(v, "Press y or Enter again") > strings.Index(v, "1 [x] Devanagari") {
		t.Fatalf("copy line not at the top:\n%s", v)
	}
	for _, w := range []string{"Copy", "1 [x] Devanagari", "2 [x] Verse", "3 [x] Translation", "4 [x] Purport", "5 [x] Link"} {
		if !strings.Contains(v, w) {
			t.Fatalf("missing %q:\n%s", w, v)
		}
	}
	if strings.Contains(v, "Word") || strings.Contains(v, "ynonym") {
		t.Fatal("synonyms offered")
	}
	drive(a, key("esc"))
	if a.copyMenu.visible || strings.Contains(plain(a), "[x] Link") {
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

func verseItem() api.Item {
	return api.Item{Ref: "bg/2/13", URL: "https://vedabase.cc/en/library/bg/2/13/",
		Devanagari: "> देहिनोऽस्मिन्\n> यथा देहे", VerseText: "> dehino 'smin\n> yathā dehe", Synonyms: "dehinaḥ—of the embodied",
		Translation: "As the **embodied** soul…", Purport: "Since every living entity…"}
}

const (
	tDeva  = "देहिनोऽस्मिन्\nयथा देहे"
	tVerse = "dehino 'smin\nyathā dehe"
	tURL   = "https://vedabase.cc/en/library/bg/2/13/"
)

// y y copies everything ticked: the reference, each part under its label, the link last.
func TestCopyEverythingByDefault(t *testing.T) {
	for _, k := range []string{"y", "enter"} {
		a := copyApp(t)
		calls := stubClip(a, "linux", map[string]string{"WAYLAND_DISPLAY": "w"}, "wl-copy")
		a.item = verseItem()
		drive(a, key("y"), key(k))
		want := "Bg 2.13\n\nDevanagari\n" + tDeva + "\n\nVerse\n" + tVerse +
			"\n\nTranslation\nAs the embodied soul…\n\nPurport\nSince every living entity…\n\n" + tURL
		if a.copyMenu.visible || len(*calls) != 1 || string((*calls)[0].input) != want {
			t.Fatalf("%s: copied %q", k, (*calls)[0].input)
		}
		if !strings.Contains(plain(a), "Copied Devanagari, verse, translation, purport and link") {
			t.Fatalf("%s: no toast", k)
		}
	}
}

func TestCopyTicksAndRemembers(t *testing.T) {
	a := copyApp(t)
	calls := stubClip(a, "darwin", nil, "pbcopy")
	a.item = verseItem()
	var saved store.State
	a.save = func(s store.State) error { saved = s; return nil }
	// 1 unticks Devanagari, 4 unticks Purport.
	drive(a, key("y"), key("1"), key("4"), key("y"))
	want := "Bg 2.13\n\nVerse\n" + tVerse + "\n\nTranslation\nAs the embodied soul…\n\n" + tURL
	if got := string((*calls)[0].input); got != want {
		t.Fatalf("copied %q", got)
	}
	if saved.CopyOff != "devanagari,purport" || a.State().CopyOff != "devanagari,purport" {
		t.Fatalf("saved %q", saved.CopyOff)
	}
	// Next time the same boxes are ticked; space on the cursor row ticks it back.
	drive(a, key("y"))
	if a.copyMenu.rows[0].on || a.copyMenu.rows[3].on || !a.copyMenu.rows[1].on {
		t.Fatalf("rows %+v", a.copyMenu.rows)
	}
	drive(a, tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}, key("esc"))
	if a.state.CopyOff != "purport" {
		t.Fatalf("after space: %q", a.state.CopyOff)
	}
}

func TestCopyOnePartHasNoLabelAndLinkAloneIsTheURL(t *testing.T) {
	a := copyApp(t)
	a.item = verseItem()
	calls := stubClip(a, "darwin", nil, "pbcopy")
	drive(a, key("y"), key("1"), key("2"), key("4"), key("5"), key("y")) // translation only
	if got := string((*calls)[0].input); got != "Bg 2.13\n\nAs the embodied soul…" {
		t.Fatalf("translation only: %q", got)
	}
	drive(a, key("y"), key("3"), key("5"), key("y")) // link only
	if got := string((*calls)[1].input); got != tURL {
		t.Fatalf("link only: %q", got)
	}
	drive(a, key("y"), key("5"), key("y")) // nothing ticked
	if len(*calls) != 2 || !a.copyMenu.visible { // stays open, copies nothing
		t.Fatalf("nothing ticked: calls %d visible %v", len(*calls), a.copyMenu.visible)
	}
}

// Rows follow the item: no Devanagari, no row; letters and prose have the
// whole text, unlabelled.
func TestCopyRowsFollowTheItem(t *testing.T) {
	a := copyApp(t)
	stubClip(a, "linux", nil)
	labels := func() string {
		var out []string
		for _, r := range a.copyMenu.rows {
			out = append(out, r.label)
		}
		return strings.Join(out, "|")
	}
	a.item = verseItem()
	a.item.Devanagari, a.item.Purport = "", ""
	drive(a, key("y"))
	if got := labels(); got != "Verse|Translation|Link" {
		t.Fatalf("verse: %s", got)
	}
	drive(a, key("esc"))
	a.item = api.Item{Ref: "letters/x", URL: "u", Translation: "Dear son,", Purport: "Your servant"}
	drive(a, key("y"))
	if got := labels(); got != "Whole text|Link" || a.copyMenu.rows[0].text != "Dear son,\n\nYour servant" {
		t.Fatalf("letter: %s %q", got, a.copyMenu.rows[0].text)
	}
	text, _ := a.copyMenu.copyBody()
	if text != "Dear son,\n\nYour servant\n\nu" {
		t.Fatalf("letter copy %q", text)
	}
}

func TestCopyWSLClipExeGetsUTF16(t *testing.T) {
	a := copyApp(t)
	calls := stubClip(a, "linux", map[string]string{"WSL_DISTRO_NAME": "Ubuntu"}, "clip.exe", "xclip")
	a.item = api.Item{Ref: "bg/2/13", Translation: "Śrīmad-Bhāgavatam ॐ"}
	drive(a, key("y"), key("2"), key("y")) // untick the link
	if len(*calls) != 1 || (*calls)[0].name != "clip.exe" {
		t.Fatalf("calls = %+v", *calls)
	}
	var want []byte // no byte order mark: clip.exe would keep it as a character
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
	drive(a, key("y"), key("2"), key("y")) // untick the link
	if len(*calls) != 1 || (*calls)[0].name != "clip.exe" {
		t.Fatalf("calls = %+v", *calls)
	}
}

func TestCopyUTF8ForOthersAndXToolOrder(t *testing.T) {
	a := copyApp(t)
	a.item = api.Item{Ref: "bg/2/13", Translation: "Śrīmad ॐ"}
	calls := stubClip(a, "linux", map[string]string{"DISPLAY": ":0"}, "xsel", "xclip")
	drive(a, key("y"), key("2"), key("y")) // untick the link
	c := (*calls)[0]
	if c.name != "xclip" || strings.Join(c.args, " ") != "-selection clipboard" || string(c.input) != "Śrīmad ॐ" {
		t.Fatalf("%+v", c)
	}
	calls = stubClip(a, "linux", map[string]string{"DISPLAY": ":0"}, "xsel")
	drive(a, key("y"), key("y")) // the link stays unticked: remembered
	if c := (*calls)[0]; c.name != "xsel" || strings.Join(c.args, " ") != "--clipboard --input" {
		t.Fatalf("%+v", c)
	}
}

func TestCopyNoNativeToolStillOSC52(t *testing.T) {
	a := copyApp(t)
	calls := stubClip(a, "linux", nil)
	_, _ = a.Update(key("y"))
	_, cmd := a.Update(key("y"))
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
	a.item = verseItem()
	drive(a, key("y"))
	r := a.overlayRect(a.overlayBox())
	click := func(x, y int) {
		drive(a, tea.MouseClickMsg{Button: tea.MouseLeft, X: x, Y: y})
	}
	click(r.x+3, r.y+2) // the "press y" line: nothing
	if a.copyMenu.cursor != 0 || !a.copyMenu.rows[0].on {
		t.Fatal("click on the top line ticked a row")
	}
	click(r.x+3, r.y+4) // row 2: unticks Verse
	if a.copyMenu.cursor != 1 || a.copyMenu.rows[1].on || len(*calls) != 0 {
		t.Fatalf("cursor %d rows %+v calls %v", a.copyMenu.cursor, a.copyMenu.rows, *calls)
	}
	click(r.x+3, r.y+4) // again: ticks it back
	if !a.copyMenu.rows[1].on || !a.copyMenu.visible {
		t.Fatal("second click")
	}
	click(0, 0)
	if a.copyMenu.visible || len(*calls) != 0 {
		t.Fatal("outside click did not close without copying")
	}
}

func TestHelpMentionsCopy(t *testing.T) {
	a := copyApp(t)
	drive(a, key("?"))
	if !strings.Contains(plain(a), "Copy (tick parts") {
		t.Fatal("help lacks y")
	}
}

func typeName(m tea.Msg) string { return fmt.Sprintf("%T", m) }

// Copied text is plain: the site's [[ref|label]] links become their labels
// and ** bold markers go.
func TestCopyStripsMarkup(t *testing.T) {
	a := copyApp(t)
	calls := stubClip(a, "linux", map[string]string{"DISPLAY": ":0"}, "xclip")
	a.item = api.Item{Ref: "bg/2/13", Translation: "As **Kṛṣṇa** says in [[bg/2/12|Bg 2.12]] and [[sb/1/1/1]]."}
	drive(a, key("y"), key("2"), key("y")) // untick the link
	if len(*calls) != 1 {
		t.Fatalf("calls = %+v", *calls)
	}
	if got, want := string((*calls)[0].input), "As Kṛṣṇa says in Bg 2.12 and sb/1/1/1."; got != want {
		t.Fatalf("copied %q, want %q", got, want)
	}
}

// Tests never start a real clipboard tool, whatever constructor they use.
func TestDefaultClipEnvInertUnderTest(t *testing.T) {
	if defaultClipEnv().run != nil {
		t.Fatal("defaultClipEnv would run real processes under go test")
	}
}
