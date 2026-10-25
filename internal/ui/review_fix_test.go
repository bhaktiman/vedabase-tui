package ui

import (
	"strings"
	"testing"

	"vedabase-tui/internal/api"
	"vedabase-tui/internal/store"
)

// Remote text with numeric character references that decode to ESC or BEL must
// not reach the terminal: Glamour decodes them, so the service cleans them first.
func TestNumericEntityEscapesNeverReachView(t *testing.T) {
	be := newFake(t)
	bad := "x &#27;]0;PWNED&#7; &#x1b;]52;c;Y2xpcA==&#X07; &#155;2J &#127; y"
	it := be.items["bg/2/13"]
	it.Translation, it.Purport, it.Synonyms = bad, bad, bad
	be.items["bg/2/13"] = api.Clean(it) // the real service cleans on every read
	a, _ := boot(t, be, store.State{LastRef: "bg/2/13"})
	v := a.View().Content
	for _, s := range []string{"\x1b]", "\x07", "\u009b", "\x7f"} {
		if strings.Contains(v, s) {
			t.Errorf("view contains %q", s)
		}
	}
	if !strings.Contains(plain(a), "PWNED") {
		t.Errorf("text missing:\n%s", plain(a))
	}
}
