package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vedabase-tui/internal/store"
)

func TestGotoOpens(t *testing.T) {
	a, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
	drive(a, key(":"))
	if v := plain(a); !strings.Contains(v, ": ") {
		t.Fatalf("no prompt:\n%s", v)
	}
	typeKeys(a, "cc m 9.74") // letters like q/j/s/e/n must not leak to global keys
	drive(a, key("enter"))
	if a.ref != "cc/madhya/9/74" {
		t.Fatalf("ref = %q", a.ref)
	}
	if a.gotoBar.visible {
		t.Fatal("prompt still open")
	}
}

func TestGotoPromptTakesEveryKey(t *testing.T) {
	a, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
	drive(a, key(":"))
	typeKeys(a, "qsen/jk")
	if a.search.visible || a.picker.visible || a.exporting || a.gotoBar.input.Value() != "qsen/jk" {
		t.Fatalf("keys leaked: %q", a.gotoBar.input.Value())
	}
	drive(a, key("esc"))
	if a.gotoBar.visible || a.ref != "bg/2/13" {
		t.Fatal("esc did not cancel")
	}
}

func TestGotoUnknown(t *testing.T) {
	a, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
	drive(a, key(":"))
	typeKeys(a, "bg 99.1")
	drive(a, key("enter"))
	if a.toasts.cur == nil || a.toasts.cur.text != "No bg 99.1 in the library" {
		t.Fatalf("toast = %+v", a.toasts.cur)
	}
	if a.ref != "bg/2/13" {
		t.Fatalf("ref = %q", a.ref)
	}
	drive(a, key(":"))
	typeKeys(a, "zzz")
	drive(a, key("enter"))
	if a.toasts.cur == nil || a.ref != "bg/2/13" {
		t.Fatalf("parse error not toasted: %+v", a.toasts.cur)
	}
}

func TestFilterKey(t *testing.T) {
	a, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
	drive(a, key("tab"), key("f"))
	typeKeys(a, "abhi")
	v := ansiRe.ReplaceAllString(a.tree.View(), "") // the reader's top bar also says Bhagavad
	if !strings.Contains(v, "Abhirama") || strings.Contains(v, "Bhagavad") {
		t.Fatalf("filtered tree wrong:\n%s", v)
	}
	drive(a, key("esc"))
	if a.tree.filter != "" || a.filtering || !strings.Contains(a.tree.View(), "Bhagavad") {
		t.Fatal("esc did not clear the filter")
	}
}

func TestFilterTakesEveryKeyAndEnterKeeps(t *testing.T) {
	a, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
	drive(a, key("tab"), key("f"))
	typeKeys(a, "jknqse/")
	if a.search.visible || a.tree.filter != "jknqse/" || !a.filtering {
		t.Fatalf("keys leaked; filter %q", a.tree.filter)
	}
	drive(a, tea.KeyPressMsg{Code: tea.KeyBackspace})
	if a.tree.filter != "jknqse" {
		t.Fatalf("backspace: %q", a.tree.filter)
	}
	drive(a, key("esc"), key("f"))
	typeKeys(a, "abhi")
	drive(a, key("enter"))
	if a.filtering || a.tree.filter != "abhi" || !a.focusTree {
		t.Fatalf("enter should keep filter and the tree: %q", a.tree.filter)
	}
	if !strings.Contains(plain(a), "f abhi") {
		t.Fatal("filter line not shown")
	}
}

func TestFilterNeedsTreeFocus(t *testing.T) {
	a, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
	drive(a, key("f"))
	if a.filtering {
		t.Fatal("f opened the filter with the reader focused")
	}
}

func TestHelpOverlay(t *testing.T) {
	a, _ := boot(t, newFake(t), store.State{LastRef: "bg/2/13"})
	drive(a, key("?"))
	v := plain(a)
	for _, w := range []string{"Add to collection", "Reader", "Tree", "Overlays", "Go to reference"} {
		if !strings.Contains(v, w) {
			t.Errorf("help lacks %q", w)
		}
	}
	drive(a, key("q")) // closes, does not quit
	if a.help || strings.Contains(plain(a), "Add to collection") {
		t.Fatal("help still open")
	}
}
