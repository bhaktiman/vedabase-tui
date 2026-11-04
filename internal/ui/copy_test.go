package ui

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vedabase-tui/internal/api"
	"vedabase-tui/internal/render"
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
	a.item.Devanagari = ""
	drive(a, key("y"))
	v := plain(a)
	// No Devanagari: most first, named by what it copies, link last.
	for _, w := range []string{"Copy", "1  Verse + translation + purport", "2  Verse + translation", "3  Translation only", "4  Link"} {
		if !strings.Contains(v, w) {
			t.Fatalf("missing %q:\n%s", w, v)
		}
	}
	drive(a, key("esc"))
	if a.copyMenu.visible || strings.Contains(plain(a), "4  Link") {
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
	drive(a, key("y"), key("3")) // the shown verse has Devanagari: 3 is verse + translation
	if a.copyMenu.visible {
		t.Fatal("menu still open")
	}
	want := "Bg 2.13\n\nVerse\n" + strings.TrimSpace(render.PlainText(unquote(a.item.VerseText))) + "\n\nTranslation\n" + strings.TrimSpace(render.PlainText(a.item.Translation))
	if len(*calls) != 1 || (*calls)[0].name != "wl-copy" || string((*calls)[0].input) != want {
		t.Fatalf("calls = %+v", *calls)
	}
	if strings.Contains(want, "> ") {
		t.Fatalf("quote markers copied: %q", want)
	}
	if !strings.Contains(plain(a), "Copied verse and translation") { // toast says what was copied
		t.Fatal("no toast")
	}
}

func TestCopyLinkIsJustTheURLAndLast(t *testing.T) {
	a := copyApp(t)
	calls := stubClip(a, "darwin", nil, "pbcopy")
	drive(a, key("y"))
	n := len(a.copyMenu.rows)
	drive(a, key(fmt.Sprint(n)))
	if len(*calls) != 1 || (*calls)[0].name != "pbcopy" || string((*calls)[0].input) != a.item.URL || !strings.HasPrefix(a.item.URL, "https://") {
		t.Fatalf("calls = %+v", *calls)
	}
}

func verseItem() api.Item {
	return api.Item{Ref: "bg/2/13", URL: "https://vedabase.cc/en/library/bg/2/13/",
		Devanagari: "> देहिनोऽस्मिन्\n> यथा देहे", VerseText: "> dehino 'smin\n> yathā dehe", Synonyms: "dehinaḥ—of the embodied",
		Translation: "As the **embodied** soul…", Purport: "Since every living entity…"}
}

func TestCopyVerseRows(t *testing.T) {
	a := copyApp(t)
	deva, verse := "देहिनोऽस्मिन्\nयथा देहे", "dehino 'smin\nyathā dehe"
	for k, want := range map[string]string{
		"1": "Bg 2.13\n\nDevanagari\n" + deva + "\n\nVerse\n" + verse + "\n\nTranslation\nAs the embodied soul…\n\nPurport\nSince every living entity…",
		"2": "Bg 2.13\n\nDevanagari\n" + deva + "\n\nVerse\n" + verse + "\n\nTranslation\nAs the embodied soul…",
		"3": "Bg 2.13\n\nVerse\n" + verse + "\n\nTranslation\nAs the embodied soul…",
		"4": "Bg 2.13\n\nAs the embodied soul…",
		"5": "Bg 2.13\n\n" + deva,
		"6": "https://vedabase.cc/en/library/bg/2/13/",
	} {
		calls := stubClip(a, "darwin", nil, "pbcopy")
		a.item = verseItem()
		drive(a, key("y"), key(k))
		if len(*calls) != 1 || string((*calls)[0].input) != want {
			t.Fatalf("%s: copied %q", k, (*calls)[0].input)
		}
		if strings.Contains(string((*calls)[0].input), "dehinaḥ") {
			t.Fatalf("%s: synonyms copied", k)
		}
	}
}

func TestCopyJKEnter(t *testing.T) {
	a := copyApp(t)
	calls := stubClip(a, "darwin", nil, "pbcopy")
	a.item = verseItem()
	drive(a, key("y"), key("j"), key("j"), key("enter"))
	if got := string((*calls)[0].input); len(*calls) != 1 || !strings.HasPrefix(got, "Bg 2.13\n\nVerse\n") || strings.Contains(got, "Purport") {
		t.Fatalf("calls = %+v", *calls)
	}
}

// Rows are named by what they copy and follow the item: a row that would
// repeat one above, or copy nothing, is left out.
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
	drive(a, key("y"))
	if got := labels(); got != "Devanagari + verse + translation + purport|Devanagari + verse + translation|Verse + translation|Translation only|Devanagari only|Link" {
		t.Fatalf("full verse: %s", got)
	}
	drive(a, key("esc"))
	a.item.Purport = "" // rows 1 and 2 would be the same
	drive(a, key("y"))
	if got := labels(); got != "Devanagari + verse + translation|Verse + translation|Translation only|Devanagari only|Link" {
		t.Fatalf("no purport: %s", got)
	}
	drive(a, key("esc"))
	a.item = api.Item{Ref: "letters/x", URL: "u", Translation: "Dear son,", Purport: "Your servant"}
	drive(a, key("y"))
	if got := labels(); got != "Whole text|Link" || a.copyMenu.rows[0].text != "Dear son,\n\nYour servant" {
		t.Fatalf("letter: %s %q", got, a.copyMenu.rows[0].text)
	}
}

func TestCopyWSLClipExeGetsUTF16(t *testing.T) {
	a := copyApp(t)
	calls := stubClip(a, "linux", map[string]string{"WSL_DISTRO_NAME": "Ubuntu"}, "clip.exe", "xclip")
	a.item = api.Item{Ref: "bg/2/13", Translation: "Śrīmad-Bhāgavatam ॐ"}
	drive(a, key("y"), key("1"))
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
	drive(a, key("y"), key("1"))
	if len(*calls) != 1 || (*calls)[0].name != "clip.exe" {
		t.Fatalf("calls = %+v", *calls)
	}
}

func TestCopyUTF8ForOthersAndXToolOrder(t *testing.T) {
	a := copyApp(t)
	a.item = api.Item{Ref: "bg/2/13", Translation: "Śrīmad ॐ"}
	calls := stubClip(a, "linux", map[string]string{"DISPLAY": ":0"}, "xsel", "xclip")
	drive(a, key("y"), key("1"))
	c := (*calls)[0]
	if c.name != "xclip" || strings.Join(c.args, " ") != "-selection clipboard" || string(c.input) != "Śrīmad ॐ" {
		t.Fatalf("%+v", c)
	}
	calls = stubClip(a, "linux", map[string]string{"DISPLAY": ":0"}, "xsel")
	drive(a, key("y"), key("1"))
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
	if !strings.Contains(plain(a), "Copy verse / translation") {
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
	drive(a, key("y"), key("1"))
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
