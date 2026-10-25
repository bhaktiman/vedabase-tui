package ui

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"vedabase-tui/internal/api"
	"vedabase-tui/internal/store"
)

type fakeUpdater struct {
	mu       sync.Mutex
	tag      string
	applyErr error
	applied  []string
}

func (f *fakeUpdater) Available(context.Context) (string, error) { return f.tag, nil }

func (f *fakeUpdater) Apply(_ context.Context, tag string, progress func(done, total int64)) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.applied = append(f.applied, tag)
	progress(1, 2)
	return f.applyErr
}

func (f *fakeUpdater) calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.applied...)
}

func bootUpd(t *testing.T, u Updater, w int) (*App, *saved) {
	t.Helper()
	sv := &saved{}
	a := New(Options{Backend: newFake(t), State: store.State{LastRef: "bg/2/13", Parts: api.AllParts()},
		SaveState: sv.save, Version: "0.1.0", Updater: u})
	a.tick = func(time.Duration, tea.Msg) tea.Cmd { return nil }
	drive(a, tea.WindowSizeMsg{Width: w, Height: 40}, a.Init()())
	return a, sv
}

func tagRow(a *App) string {
	tr, _ := a.paneRects()
	return strings.Split(plain(a), "\n")[tr.y+tr.h-1]
}

func TestUpdateFound(t *testing.T) {
	a, _ := bootUpd(t, &fakeUpdater{tag: "v0.2.0"}, 140)
	if got := toastText(a); got != "vedabase v0.2.0 is available — press U to update" {
		t.Fatalf("toast = %q", got)
	}
	clearToast(a)
	if row := tagRow(a); !strings.Contains(row, "vedabase.cc · v0.2.0 available · U") {
		t.Fatalf("tag row %q", row)
	}
	if strings.Contains(plain(a), "U update to") {
		t.Fatal("bar hint shown with the sidebar tag on screen")
	}
	// the sidebar tops out at 40 cells: at 120 columns the version and U still fit
	b, _ := bootUpd(t, &fakeUpdater{tag: "v0.2.0"}, 120)
	if row := tagRow(b); !strings.Contains(row, " vedabase.cc · v0.2.0 · U ") {
		t.Fatalf("narrow tag row %q", row)
	}
}

func TestNoUpdateShowsVersion(t *testing.T) {
	for name, u := range map[string]Updater{"empty": &fakeUpdater{}, "nil": nil} {
		a, _ := bootUpd(t, u, 120)
		if row := tagRow(a); !strings.Contains(row, " vedabase.cc · v0.1.0 ") {
			t.Fatalf("%s: tag row %q", name, row)
		}
		if toastText(a) != "" {
			t.Fatalf("%s: toast %q", name, toastText(a))
		}
	}
}

func TestUWithNothingAvailable(t *testing.T) {
	u := &fakeUpdater{}
	a, _ := bootUpd(t, u, 120)
	drive(a, key("U"))
	if len(u.calls()) != 0 || a.updating {
		t.Fatalf("applied %v", u.calls())
	}
}

func TestUApplyRestarts(t *testing.T) {
	u := &fakeUpdater{tag: "v0.2.0"}
	a, sv := bootUpd(t, u, 120)
	clearToast(a)
	_, cmd := a.Update(key("U"))
	if !a.updating || !strings.Contains(toastText(a), "Downloading") {
		t.Fatalf("updating=%v toast=%q", a.updating, toastText(a))
	}
	var done tea.Msg
	msgs := []tea.Msg{cmd()}
	if b, ok := msgs[0].(tea.BatchMsg); ok { // timers are stubbed, so it may be just the one Cmd
		msgs = nil
		for _, c := range b {
			if c != nil {
				msgs = append(msgs, c())
			}
		}
	}
	for _, m := range msgs {
		if d, ok := m.(updateDoneMsg); ok {
			done = d
		}
	}
	if !reflect.DeepEqual(u.calls(), []string{"v0.2.0"}) || done == nil {
		t.Fatalf("applied %v done=%v", u.calls(), done)
	}
	n := len(sv.calls)
	_, q := a.Update(done)
	if !a.RestartRequested() || q == nil {
		t.Fatal("no restart")
	}
	if _, ok := q().(tea.QuitMsg); !ok {
		t.Fatalf("cmd gave %T", q())
	}
	if len(sv.calls) != n+1 || sv.calls[n].LastRef != "bg/2/13" {
		t.Fatalf("state saves %+v", sv.calls)
	}
	// a second U while updating does nothing
	a.updating = true
	drive(a, key("U"))
	if len(u.calls()) != 1 {
		t.Fatal("applied twice")
	}
}

func TestUpdatePermissionError(t *testing.T) {
	u := &fakeUpdater{tag: "v0.2.0", applyErr: fmt.Errorf("rename: %w", fs.ErrPermission)}
	a, _ := bootUpd(t, u, 200)
	clearToast(a)
	drive(a, key("U"))
	if a.RestartRequested() {
		t.Fatal("restart after a failure")
	}
	if !strings.Contains(toastText(a), "permission") {
		t.Fatalf("toast %q", toastText(a))
	}
	if !strings.Contains(plain(a), installCmd) {
		t.Fatal("install command not on screen")
	}
	drive(a, key("j"))
	if strings.Contains(plain(a), installCmd) {
		t.Fatal("install command stayed after a key")
	}
}

func TestUpdateOtherError(t *testing.T) {
	u := &fakeUpdater{tag: "v0.2.0", applyErr: errors.New("checksum mismatch")}
	a, _ := bootUpd(t, u, 120)
	clearToast(a)
	drive(a, key("U"))
	if got := toastText(a); got != "Update failed: checksum mismatch — nothing changed" {
		t.Fatalf("toast %q", got)
	}
	if a.RestartRequested() || strings.Contains(plain(a), installCmd) {
		t.Fatal("restart or command after a plain failure")
	}
	clearToast(a)
	drive(a, key("U"))
	if len(u.calls()) != 2 {
		t.Fatalf("U did not work again: %v", u.calls())
	}
}

func TestUInSearchBoxIsALetter(t *testing.T) {
	u := &fakeUpdater{tag: "v0.2.0"}
	a, _ := bootUpd(t, u, 120)
	drive(a, key("/"), key("U"))
	if got := a.search.input.Value(); got != "U" {
		t.Fatalf("query %q", got)
	}
	if len(u.calls()) != 0 || a.updating {
		t.Fatal("update started from the search box")
	}
}

func TestUpdateHintWithSidebarHidden(t *testing.T) {
	a, _ := bootUpd(t, &fakeUpdater{tag: "v0.2.0"}, 120)
	drive(a, key("t"))
	lines := strings.Split(plain(a), "\n")
	if bar := lines[len(lines)-1]; !strings.Contains(bar, "U update to v0.2.0") {
		t.Fatalf("bar %q", bar)
	}
	drive(a, key("H")) // hints hidden: the offer stays
	lines = strings.Split(plain(a), "\n")
	if bar := lines[len(lines)-1]; !strings.Contains(bar, "U update to v0.2.0") || strings.Contains(bar, "/ search") {
		t.Fatalf("bar %q", bar)
	}
	drive(a, key("F")) // distraction-free: nothing extra
	if strings.Contains(plain(a), "U update to") {
		t.Fatal("offer shown in zen")
	}
}

func TestSidebarTagNarrowFallback(t *testing.T) {
	a, _ := bootUpd(t, &fakeUpdater{tag: "v0.2.0"}, 120)
	for _, c := range []struct {
		iw   int
		want string
	}{{40, "vedabase.cc · v0.2.0 available · U"}, {30, "vedabase.cc · v0.2.0 · U"}, {20, "vedabase.cc · U"}, {16, "vedabase.cc"}} {
		got := ansiRe.ReplaceAllString(a.siteTagView(c.iw), "")
		if got != c.want {
			t.Errorf("iw %d: %q, want %q", c.iw, got, c.want)
		}
	}
	// unframed (no sidebar tag at all): the bottom bar carries the offer
	a.h = 8
	a.layout()
	if a.sideShowsUpdate() || !strings.Contains(a.bottomBar(), "U update to v0.2.0") {
		t.Fatal("offer not in the bottom bar")
	}
}

func TestSiteTagStillClickableWithVersion(t *testing.T) {
	a, _ := bootUpd(t, &fakeUpdater{tag: "v0.2.0"}, 140)
	got := fakeOpener(a)
	tag, ok := a.siteTagRect()
	if !ok {
		t.Fatal("no tag rect")
	}
	if s := string([]rune(tagRow(a))[tag.x : tag.x+tag.w]); s != " vedabase.cc " {
		t.Fatalf("rect covers %q", s)
	}
	drive(a, click(tag.x+tag.w+3, tag.y)) // on the version: only focuses the tree
	if len(*got) != 0 {
		t.Fatalf("version click opened %v", *got)
	}
	drive(a, click(tag.x+3, tag.y))
	if !reflect.DeepEqual(*got, []string{"https://vedabase.cc/"}) {
		t.Fatalf("opened %v", *got)
	}
}
