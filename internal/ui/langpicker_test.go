package ui

import (
	"strings"
	"testing"

	"vedabase-tui/internal/store"
)

func TestLKeyOpensPickerAndPicksSpanish(t *testing.T) {
	be := langFake(t)
	a, sv := boot(t, be, store.State{LastRef: "bg/2/13"})
	drive(a, key("L"))
	if !a.langs.visible {
		t.Fatal("L did not open the picker")
	}
	v := plain(a)
	for _, n := range []string{"English", "Español", "Português", "Hindī", "Русский", "current"} {
		if !strings.Contains(v, n) {
			t.Fatalf("picker lacks %q:\n%s", n, v)
		}
	}
	if cmd := a.langKey(key("j")); cmd != nil {
		t.Fatal("moving started something")
	}
	if cmd := a.langKey(key("enter")); cmd != nil {
		drive(a, cmd())
	}
	if a.langs.visible || a.lang != "es" {
		t.Fatalf("visible=%v lang=%q", a.langs.visible, a.lang)
	}
	if got := sv.calls[len(sv.calls)-1].Lang; got != "es" {
		t.Fatalf("saved lang %q", got)
	}
	if r, l := be.lastItemReq(); r != "bg/2/13" || l != "es" {
		t.Fatalf("last request %s %s", r, l)
	}
	if row := strings.Split(plain(a), "\n")[1]; !strings.Contains(row, "ES ▾") {
		t.Fatalf("top border %q", row)
	}
}

func TestLangPickerCancelAndCurrent(t *testing.T) {
	a, sv := boot(t, langFake(t), store.State{LastRef: "bg/2/13"})
	n := len(sv.calls)
	drive(a, key("L"), key("j"), key("esc"))
	if a.langs.visible || a.lang != "en" {
		t.Fatalf("esc: visible=%v lang=%q", a.langs.visible, a.lang)
	}
	drive(a, key("L"), key("j"), key("q"))
	if a.langs.visible || a.lang != "en" {
		t.Fatal("q changed the language")
	}
	drive(a, key("L"), key("enter")) // the current one
	if a.langs.visible || a.lang != "en" || len(sv.calls) != n {
		t.Fatalf("current: visible=%v lang=%q saves=%d", a.langs.visible, a.lang, len(sv.calls)-n)
	}
}

func TestLangPickerClickRow(t *testing.T) {
	a, _ := boot(t, langFake(t), store.State{LastRef: "bg/2/13"})
	drive(a, key("L"))
	if cmd := a.langClick(2, 2+2); cmd != nil { // third row: Português
		drive(a, cmd())
	}
	if a.lang != "pt" || a.langs.visible {
		t.Fatalf("lang=%q visible=%v", a.lang, a.langs.visible)
	}
}

func TestLangTagClickOpensPicker(t *testing.T) {
	a, _ := boot(t, langFake(t), store.State{LastRef: "bg/2/13"})
	r, ok := a.langTagRect()
	if !ok {
		t.Fatal("no tag rect")
	}
	row := []rune(strings.Split(plain(a), "\n")[r.y])
	if string(row[r.x:r.x+r.w]) != "EN ▾" {
		t.Fatalf("drawn %q at rect", string(row[r.x:r.x+r.w]))
	}
	drive(a, click(r.x-1, r.y))
	if a.langs.visible {
		t.Fatal("border click opened the picker")
	}
	drive(a, click(r.x, r.y))
	if !a.langs.visible {
		t.Fatal("click on the tag did not open the picker")
	}
}

func TestNotInTagOnUnavailableBooks(t *testing.T) {
	a, _ := boot(t, langFake(t), store.State{LastRef: "bg/2/13", Lang: "hi"})
	drive(a, tea_size(120, 40))
	v := plain(a)
	if !strings.Contains(v, "· not in HI") {
		t.Fatalf("no suffix under hi:\n%s", v)
	}
	if strings.Contains(v, "Letters · not in") || strings.Contains(v, "Lectures · not in") {
		t.Fatalf("letters/lectures got a suffix:\n%s", v)
	}
	setLang(a, "en")
	if strings.Contains(plain(a), "not in") {
		t.Fatal("suffix under English")
	}
	setLang(a, "es")
	for _, l := range strings.Split(plain(a), "\n") {
		if strings.Contains(l, "Bhagavad") && strings.Contains(l, "not in") {
			t.Fatalf("bg is in es: %q", l)
		}
	}
}

func TestEnglishTagInReaderHeader(t *testing.T) {
	a, _ := boot(t, langFake(t), store.State{LastRef: "bg/2/13", Lang: "es"})
	if strings.Contains(strings.Split(plain(a), "\n")[1], " EN ") {
		t.Fatal("EN tag on a book")
	}
	drive(a, a.Open("letters/letter-to-aatie")())
	reader := strings.Split(plain(a), "\n")[1]
	if i := strings.LastIndex(reader, "╭"); i < 0 || !strings.Contains(reader[i:], "letter-to-aatie EN ") {
		t.Fatalf("no EN tag: %q", reader)
	}
	setLang(a, "en")
	if reader = strings.Split(plain(a), "\n")[1]; strings.Contains(reader[strings.LastIndex(reader, "╭"):], " EN ") {
		t.Fatalf("EN tag under English: %q", reader)
	}
}
