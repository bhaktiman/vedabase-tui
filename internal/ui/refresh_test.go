package ui

import (
	"errors"
	"reflect"
	"testing"

	"vedabase-tui/internal/api"
	"vedabase-tui/internal/store"
)

func TestRRefetchesShownItem(t *testing.T) {
	f := newFake(t)
	a, _ := boot(t, f, store.State{LastRef: "bg/2/13"})
	newer := f.item("bg/2/13")
	newer.Translation = "**Corrected translation**"
	f.refreshItem = &newer
	_, cmd := a.Update(key("R"))
	if toastText(a) != "Checking vedabase.cc…" {
		t.Fatalf("toast %q", toastText(a))
	}
	a.Update(key("R")) // a second R while one is in flight does nothing
	drive(a, cmd())
	if !reflect.DeepEqual(f.refreshCalls, []string{"bg/2/13"}) {
		t.Fatalf("refresh calls %v", f.refreshCalls)
	}
	if a.item.Translation != "**Corrected translation**" || toastText(a) != "Updated from vedabase.cc" {
		t.Fatalf("translation %q toast %q", a.item.Translation, toastText(a))
	}
}

func TestRUnchangedAndFailures(t *testing.T) {
	f := newFake(t)
	a, _ := boot(t, f, store.State{LastRef: "bg/2/13"})
	_, cmd := a.Update(key("R"))
	drive(a, cmd())
	if toastText(a) != "Already up to date" {
		t.Fatalf("toast %q", toastText(a))
	}
	before := a.item
	f.refreshErr = api.ErrOffline
	clearToast(a)
	_, cmd = a.Update(key("R"))
	drive(a, cmd())
	if toastText(a) != "Offline: kept the saved copy" || !reflect.DeepEqual(a.item, before) || a.refreshing {
		t.Fatalf("toast %q refreshing %v", toastText(a), a.refreshing)
	}
	f.refreshErr = errors.New("boom")
	clearToast(a)
	_, cmd = a.Update(key("R"))
	drive(a, cmd())
	if toastText(a) != "Refresh failed: boom" {
		t.Fatalf("toast %q", toastText(a))
	}
}

func TestRWithoutItemDoesNothing(t *testing.T) {
	f := newFake(t)
	a, _ := boot(t, f, store.State{})
	a.hasItem = false
	if _, cmd := a.Update(key("R")); cmd != nil || len(f.refreshCalls) != 0 {
		t.Fatal("refreshed with nothing open")
	}
}

// An expired copy shows at once; the API is asked behind it, and only a
// changed text says anything.
func TestExpiredItemShownThenRevalidated(t *testing.T) {
	f := newFake(t)
	f.expired = map[string]bool{"bg/2/13": true}
	newer := f.item("bg/2/13")
	newer.Translation = "**Corrected translation**"
	f.refreshItem = &newer
	a, _ := boot(t, f, store.State{LastRef: "bg/2/13"})
	if len(f.refreshCalls) != 1 || f.refreshCalls[0] != "bg/2/13" {
		t.Fatalf("refresh calls %v", f.refreshCalls)
	}
	if a.item.Translation != "**Corrected translation**" || toastText(a) != "Updated from vedabase.cc" {
		t.Fatalf("translation %q toast %q", a.item.Translation, toastText(a))
	}
}

func TestRevalidateQuietWhenUnchangedOrFailed(t *testing.T) {
	for name, err := range map[string]error{"unchanged": nil, "offline": api.ErrOffline} {
		f := newFake(t)
		f.expired = map[string]bool{"bg/2/13": true}
		f.refreshErr = err
		a, _ := boot(t, f, store.State{LastRef: "bg/2/13"})
		if len(f.refreshCalls) != 1 || toastText(a) != "" || !a.hasItem {
			t.Fatalf("%s: calls %v toast %q", name, f.refreshCalls, toastText(a))
		}
	}
}
